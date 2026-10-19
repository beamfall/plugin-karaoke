package karaoke

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	sdk "github.com/beamfall/plugin-sdk"
)

// PluginID must match the bundle's ID — scan resolves the parser through the
// Library's plugin binding.
const PluginID = "karaoke"

// Media classes: the conventional "class" Media field values, one per kind of
// karaoke song. A song's class says where its words come from.
const (
	// ClassUltraStar is audio (or video) with an UltraStar .txt: timed,
	// pitched syllables.
	ClassUltraStar = "ultrastar"
	// ClassCDG is MP3+G: audio with a .cdg graphics sidecar.
	ClassCDG = "cdg"
	// ClassVideo is a karaoke video with the words in the picture.
	ClassVideo = "video"
	// ClassLRC is audio with LRC timed lyrics, as a sidecar or embedded in
	// the file's lyrics tag.
	ClassLRC = "lrc"
	// ClassBacking is an instrumental with no timed words found.
	ClassBacking = "backing"
)

// Media field values for "vocals" and "singers".
const (
	VocalsGuide = "guide"
	SingersDuet = "duet"
)

// Parser implements the scan seams for karaoke libraries.
type Parser struct{}

var (
	_ sdk.FilenameParser   = (*Parser)(nil)
	_ sdk.MediaFieldParser = (*Parser)(nil)
)

// New returns a ready Parser.
func New() *Parser { return &Parser{} }

// videoExtensions are containers whose video stream is a picture to show. An
// MP3 or M4A with cover art also probes HasVideo, so the stream alone does not
// make a song a karaoke video.
var videoExtensions = map[string]bool{
	"mp4": true, "m4v": true, "mkv": true, "webm": true, "avi": true, "mov": true,
	"mpg": true, "mpeg": true, "vob": true, "ts": true, "wmv": true, "flv": true,
}

// sidecarExtensions are never songs themselves.
var sidecarExtensions = map[string]bool{extCDG: true, extLRC: true, extTXT: true}

// knownBrands maps the disc-ID prefixes of widely distributed karaoke labels
// to the label name. An unknown prefix still yields a disc-id, just no brand.
var knownBrands = map[string]string{
	"SC":  "Sound Choice",
	"SF":  "Sunfly",
	"CB":  "Chartbuster",
	"PHM": "Pop Hits Monthly",
	"THM": "Top Hits Monthly",
	"SBI": "SBI Karaoke",
}

var (
	// "SC8125-03 - Artist - Title": a disc ID (2–6 capital letters, digits,
	// optional letter suffix) and a track, then " - ". The track is required
	// so a band name like "UB40 - Red Red Wine" is not read as a disc.
	reDiscTrackPrefix = regexp.MustCompile(`^([A-Z]{2,6}) ?(\d{2,6}[A-Z]?)[-. ](\d{1,3}) *- +(.+)$`)
	// A bare disc ID: a folder name or an album tag ("SC8125", "SF 001").
	reDiscID = regexp.MustCompile(`^([A-Z]{2,6}) ?-?(\d{2,6}[A-Z]?)$`)
	// "03 - Artist - Title" inside a disc folder.
	reLeadingTrack = regexp.MustCompile(`^(\d{1,3}) *[-.] +(.+)$`)
	// Bracketed karaoke markers in a title or file name.
	reMarker = regexp.MustCompile(`(?i)\s*[\(\[]\s*(karaoke(?: version)?|karaoke video|instrumental(?: version)?|with (?:lead |guide )?vocals?|vocal(?: guide)?|guide vocals?|duet(?: version)?|mp3\+?g|cdg|hd|in the style of ([^\)\]]+)|originally performed by ([^\)\]]+)|as made famous by ([^\)\]]+))\s*[\)\]]`)
	// A trailing " - Karaoke" / " - Karaoke Version".
	reKaraokeSuffix = regexp.MustCompile(`(?i)\s+-\s+karaoke(?: version)?$`)
)

// song is everything one parse derives about a file, computed once per hook
// call and shared by Parse and ParseMediaFields.
type song struct {
	title, artist, discID, brand string
	class, sync, language, genre string
	vocals, singers              string
	track, year                  int
	// lyrics is the text timeline for the ultrastar and lrc classes.
	lyrics *Lyrics
	// graphicsPath is the .cdg sidecar for the cdg class.
	graphicsPath string
}

// Parse derives the best-guess ParsedMedia for one karaoke file. Sources, most
// trusted first: an UltraStar .txt header, the file's own tags, LRC [ti]/[ar]
// tags, then the file name. ok is false for a sidecar file itself and for a
// file no title can be derived for.
//
// Mapping onto the domain-agnostic seam: Title is the song title, Class the
// song's karaoke class, Year the release year when known, Index the track on
// its karaoke disc when a disc ID was found. Each song is its own Media, so
// PartGroup is never set.
func (p *Parser) Parse(path string, probe sdk.ParseProbe) (sdk.ParsedMedia, bool) {
	s, ok := analyze(path, probe)
	if !ok {
		return sdk.ParsedMedia{}, false
	}
	return sdk.ParsedMedia{Title: s.title, Class: s.class, Year: s.year, Index: s.track}, true
}

// ParseMediaFields derives the karaoke text Media fields declared in the
// bundle: artist, disc-id, brand, language, genre, vocals, singers, and
// lyrics-sync.
func (p *Parser) ParseMediaFields(path string, probe sdk.ParseProbe) []sdk.ParsedTextField {
	s, ok := analyze(path, probe)
	if !ok {
		return nil
	}
	var fields []sdk.ParsedTextField
	add := func(name, value string) {
		if value = strings.TrimSpace(value); value != "" {
			fields = append(fields, sdk.ParsedTextField{Name: name, Text: value})
		}
	}
	add("artist", s.artist)
	add("disc-id", s.discID)
	add("brand", s.brand)
	add("language", s.language)
	add("genre", s.genre)
	add("vocals", s.vocals)
	add("singers", s.singers)
	add("lyrics-sync", s.sync)
	return fields
}

// TimedLyrics returns the song's timed words for on-screen highlighting: the
// UltraStar notes, else the LRC sidecar, else LRC text embedded in the file's
// lyrics tag. ok is false for CD+G, video, and backing tracks, whose words
// are not text. It is an optional parser method a Core consumer discovers
// through its own interface, keeping lyric formats inside the plugin.
func (p *Parser) TimedLyrics(path string, probe sdk.ParseProbe) (Lyrics, bool) {
	s, ok := analyze(path, probe)
	if !ok || s.lyrics == nil {
		return Lyrics{}, false
	}
	return *s.lyrics, true
}

// ComposedPicture says how a server can compose a picture for a song that has
// none of its own, so every client plays it as an ordinary video with the
// words on screen:
//
//   - CD+G: graphicsPath is the .cdg sidecar, which the server decodes as the
//     picture.
//   - UltraStar or LRC: timedText is an ASS document (RenderASS) the server
//     burns over a plain background.
//
// ok is false for a song with no sidecar words (a backing track, or a
// karaoke video, which already has its picture) and for a sidecar file. It
// reads sidecars only: a server composes ahead of playback with no probe, so
// LRC embedded in tags is not composed. It is an optional parser method a Core
// consumer discovers through its own interface, so it uses only basic types.
func (p *Parser) ComposedPicture(path string) (graphicsPath, timedText string, ok bool) {
	s, ok := analyze(path, sdk.ParseProbe{})
	if !ok {
		return "", "", false
	}
	switch {
	case s.class == ClassCDG && s.graphicsPath != "":
		return s.graphicsPath, "", true
	case s.lyrics != nil:
		return "", RenderASS(*s.lyrics), true
	}
	return "", "", false
}

func analyze(path string, probe sdk.ParseProbe) (song, bool) {
	ext := extension(path)
	if sidecarExtensions[ext] {
		return song{}, false
	}
	found := findSidecars(path)
	if found.ultraStarVideo {
		// The #VIDEO of an UltraStar song is part of that song, which its
		// audio file already represents.
		return song{}, false
	}
	tags := probe.Tags
	var s song

	// Class and highlight shape, from where the words live.
	lrc := found.lrc
	if lrc == nil {
		if parsed, ok := embeddedLRC(tags); ok {
			lrc = &parsed
		}
	}
	switch {
	case found.ultraStar != nil:
		s.class, s.sync = ClassUltraStar, SyncNote
		s.lyrics = &found.ultraStar.Lyrics
	case found.cdg:
		s.class, s.sync = ClassCDG, SyncGraphics
		s.graphicsPath = found.cdgPath
	case probe.HasVideo && videoExtensions[ext]:
		s.class, s.sync = ClassVideo, SyncVideo
	case lrc != nil:
		s.class, s.sync = ClassLRC, lrc.Lyrics.Sync
		s.lyrics = &lrc.Lyrics
	default:
		s.class = ClassBacking
	}

	// The file name, read for disc, track, artist, and title.
	name := fromFileName(path)

	if us := found.ultraStar; us != nil {
		s.title, s.artist = us.Title, us.Artist
		s.language, s.genre, s.year = us.Language, us.Genre, us.Year
		if us.Duet {
			s.singers = SingersDuet
		}
	}
	s.title = firstNonEmpty(s.title, tags["title"])
	s.artist = firstNonEmpty(s.artist, tags["artist"], tags["album_artist"])
	if lrc != nil {
		s.title = firstNonEmpty(s.title, lrc.Title)
		s.artist = firstNonEmpty(s.artist, lrc.Artist)
	}
	s.title = firstNonEmpty(s.title, name.title)
	s.artist = firstNonEmpty(s.artist, name.artist)

	// Markers in the chosen title ("(In the Style of X)", "[Vocal Guide]")
	// clean it and carry facts of their own.
	title, marks := stripMarkers(s.title)
	s.title = title
	s.artist = firstNonEmpty(s.artist, marks.styleOf, name.marks.styleOf)
	if artist, artistMarks := stripMarkers(s.artist); artist != "" {
		s.artist = artist
		marks.merge(artistMarks)
	}
	marks.merge(name.marks)
	if marks.vocals {
		s.vocals = VocalsGuide
	}
	if marks.duet {
		s.singers = SingersDuet
	}

	// A track number means something only on a disc: the album tag names
	// the disc when the file name does not.
	s.discID, s.track = name.discID, name.track
	if s.discID == "" {
		s.discID = discID(tags["album"])
	}
	if n, ok := tagTrack(tags["track"]); ok && s.track == 0 {
		s.track = n
	}
	if s.discID == "" {
		s.track = 0
	}
	if prefix := discPrefix(s.discID); prefix != "" {
		s.brand = knownBrands[prefix]
	}

	if s.year == 0 {
		s.year = tagYear(firstNonEmpty(tags["date"], tags["year"], tags["originaldate"]))
	}
	s.language = firstNonEmpty(s.language, tags["language"])
	s.genre = firstNonEmpty(s.genre, tags["genre"])

	if s.title == "" {
		return song{}, false
	}
	return s, true
}

// nameFacts is what a file name (and its disc folder) says.
type nameFacts struct {
	title, artist, discID string
	track                 int
	marks                 markers
}

// fromFileName reads "SC8125-03 - Artist - Title", "03 - Artist - Title" in a
// disc folder ("SC8125/"), "Artist - Title", or a bare title. Underscores
// count as spaces. A name with no " - " is a title. Known limitation: a file
// named "Title - Artist" reads backwards — the order is not knowable from the
// name alone, and "Artist - Title" is the dominant convention. Tags and
// sidecars win when present.
func fromFileName(path string) nameFacts {
	var facts nameFacts
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	stem = strings.Join(strings.Fields(strings.ReplaceAll(stem, "_", " ")), " ")
	stem = reKaraokeSuffix.ReplaceAllString(stem, "")

	if m := reDiscTrackPrefix.FindStringSubmatch(stem); m != nil {
		facts.discID = m[1] + m[2]
		facts.track = atoi(m[3])
		stem = m[4]
	} else if folder := discID(filepath.Base(filepath.Dir(path))); folder != "" {
		if m := reLeadingTrack.FindStringSubmatch(stem); m != nil {
			facts.discID = folder
			facts.track = atoi(m[1])
			stem = m[2]
		}
	}

	stem, facts.marks = stripMarkers(stem)
	if artist, title, ok := strings.Cut(stem, " - "); ok {
		facts.artist, facts.title = strings.TrimSpace(artist), strings.TrimSpace(title)
	} else {
		facts.title = strings.TrimSpace(stem)
	}
	return facts
}

// markers are the facts bracketed karaoke markers carry.
type markers struct {
	vocals, duet bool
	styleOf      string
}

func (m *markers) merge(other markers) {
	m.vocals = m.vocals || other.vocals
	m.duet = m.duet || other.duet
	if m.styleOf == "" {
		m.styleOf = other.styleOf
	}
}

// stripMarkers removes bracketed karaoke markers from s and reports them.
func stripMarkers(s string) (string, markers) {
	var m markers
	cleaned := reMarker.ReplaceAllStringFunc(s, func(match string) string {
		sub := reMarker.FindStringSubmatch(match)
		label := strings.ToLower(sub[1])
		switch {
		case strings.Contains(label, "vocal"):
			m.vocals = true
		case strings.HasPrefix(label, "duet"):
			m.duet = true
		}
		if m.styleOf == "" {
			m.styleOf = strings.TrimSpace(firstNonEmpty(sub[2], sub[3], sub[4]))
		}
		return ""
	})
	cleaned = reKaraokeSuffix.ReplaceAllString(cleaned, "")
	return strings.Join(strings.Fields(cleaned), " "), m
}

// discID normalizes a bare disc ID ("SC 8125", "SC-8125" → "SC8125"); "" when
// s is not one.
func discID(s string) string {
	m := reDiscID.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return ""
	}
	return m[1] + m[2]
}

// discPrefix is the leading capital letters of a normalized disc ID
// ("SC8125" → "SC").
func discPrefix(id string) string {
	for i := 0; i < len(id); i++ {
		if id[i] < 'A' || id[i] > 'Z' {
			return id[:i]
		}
	}
	return id
}

// embeddedLRC reads LRC text from a lyrics tag (FFmpeg exposes ID3 USLT as
// "lyrics" or "lyrics-<lang>"). Plain unsynchronized lyrics are not LRC. Keys
// are tried in sorted order so the choice is deterministic.
func embeddedLRC(tags map[string]string) (LRC, bool) {
	var keys []string
	for key := range tags {
		if key == "lyrics" || strings.HasPrefix(key, "lyrics-") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if lrc, err := ParseLRC([]byte(tags[key])); err == nil {
			lrc.Lyrics.Source = LyricsSourceEmbeddedLRC
			return lrc, true
		}
	}
	return LRC{}, false
}

// tagTrack reads "5" or "5/12".
func tagTrack(raw string) (int, bool) {
	track, _, _ := strings.Cut(strings.TrimSpace(raw), "/")
	n, err := strconv.Atoi(strings.TrimSpace(track))
	return n, err == nil && n > 0
}

// tagYear reads the year from "1999", "1999-05-01", or "1999-05".
func tagYear(raw string) int {
	raw = strings.TrimSpace(raw)
	if len(raw) < 4 {
		return 0
	}
	year, err := strconv.Atoi(raw[:4])
	if err != nil || year < 1000 {
		return 0
	}
	return year
}

func extension(path string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

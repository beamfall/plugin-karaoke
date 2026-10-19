package karaoke

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// UltraStar parse failures.
var (
	// ErrNotUltraStar reports a .txt with no #TITLE header: a song folder
	// holds readme files too.
	ErrNotUltraStar = errors.New("karaoke: not an UltraStar file (no #TITLE)")
	// ErrUltraStarBPM reports a missing, non-numeric, or non-positive #BPM.
	ErrUltraStarBPM = errors.New("karaoke: UltraStar #BPM is missing or not a positive number")
)

// UltraStar is an UltraStar song file (the .txt singing-game format) reduced
// to its header metadata and its timed, pitched notes.
type UltraStar struct {
	Title    string
	Artist   string
	Language string
	Genre    string
	Edition  string
	Year     int
	// Audio is the audio file the song names (#AUDIO, else #MP3), relative to
	// the .txt.
	Audio string
	Video string
	Cover string
	// Version is #VERSION as written; empty for legacy (pre-1.0) files.
	Version string
	BPM     float64
	Gap     time.Duration
	// Duet reports a two-voice file (P1/P2 sections). Singers carries the
	// voice names (#P1/#P2 or #DUETSINGERP1/#DUETSINGERP2) where given.
	Duet    bool
	Singers [2]string
	Lyrics  Lyrics
}

// ParseUltraStar reads an UltraStar .txt: legacy (unversioned, optionally
// #RELATIVE:yes and #ENCODING-declared) and version 1.x files, solo and duet.
// Every note keeps its type and pitch so a client can draw a pitch guide and
// score. A "~" note is a sustained vowel carried to another pitch: it stays its
// own syllable, with its pitch and type, and empty text so the words read
// unchanged.
func ParseUltraStar(data []byte) (UltraStar, error) {
	// The whole file decides its encoding once. Header names are ASCII, so
	// #VERSION and #ENCODING are read from the raw bytes first and steer the
	// decode: only an unversioned file may declare a legacy code page.
	rawTags, _ := ultraStarHeaders(strings.Split(string(data), "\n"))
	declared := ""
	if strings.TrimSpace(rawTags["VERSION"]) == "" || strings.HasPrefix(strings.TrimSpace(rawTags["VERSION"]), "0") {
		declared = rawTags["ENCODING"]
	}
	lines := strings.Split(strings.ReplaceAll(decodeText(data, declared), "\r\n", "\n"), "\n")
	headerTags, body := ultraStarHeaders(lines)
	if headerTags["TITLE"] == "" {
		return UltraStar{}, ErrNotUltraStar
	}
	version := headerTags["VERSION"]
	versioned := false
	if version != "" {
		major, _, _ := strings.Cut(version, ".")
		switch strings.TrimSpace(major) {
		case "0":
		case "1":
			versioned = true
		default:
			return UltraStar{}, fmt.Errorf("karaoke: UltraStar version %q is not supported", version)
		}
	}
	text := strings.TrimSpace

	bpm, ok := ultraStarNumber(headerTags["BPM"])
	if !ok || bpm <= 0 {
		return UltraStar{}, ErrUltraStarBPM
	}
	gapMS, _ := ultraStarNumber(headerTags["GAP"])
	relative := !versioned && strings.EqualFold(headerTags["RELATIVE"], "yes")

	notes, duet := readUltraStarNotes(lines[body:], relative)
	toTime := func(beat int64) time.Duration {
		ms := gapMS + float64(beat)*60_000/(bpm*4)
		if ms < 0 || math.IsNaN(ms) {
			ms = 0
		}
		return time.Duration(math.Min(math.Round(ms), float64(math.MaxInt64/int64(time.Millisecond)))) * time.Millisecond
	}

	var raws []rawSyllable
	for _, note := range notes {
		syllable := note.text
		start, end := toTime(note.start), toTime(note.end)
		if strings.TrimSpace(syllable) == "~" {
			// A "~ " that ends a word moves the boundary onto the syllable
			// the vowel belongs to.
			if n := len(raws); n > 0 && !note.lineBreak && raws[n-1].singer == note.singer &&
				strings.HasSuffix(syllable, " ") && !strings.HasSuffix(raws[n-1].text, " ") {
				raws[n-1].text += " "
			}
			syllable = ""
		}
		raws = append(raws, rawSyllable{
			start: start, end: end, text: syllable, pitch: note.pitch, kind: note.kind,
			lineBreak: note.lineBreak, singer: note.singer,
		})
	}
	trimLineEdges(raws)
	if !hasWords(raws) {
		return UltraStar{}, ErrNoLyrics
	}
	lyricLines := buildLines(raws, lastSyllableHold)
	// Duet files list each voice's whole part in turn; present the lines in
	// the order they are sung.
	sort.SliceStable(lyricLines, func(i, j int) bool { return lyricLines[i].Start < lyricLines[j].Start })

	song := UltraStar{
		Title:    text(headerTags["TITLE"]),
		Artist:   text(headerTags["ARTIST"]),
		Language: text(headerTags["LANGUAGE"]),
		Genre:    text(headerTags["GENRE"]),
		Edition:  text(headerTags["EDITION"]),
		Audio:    text(firstNonEmpty(headerTags["AUDIO"], headerTags["MP3"])),
		Video:    text(headerTags["VIDEO"]),
		Cover:    text(headerTags["COVER"]),
		Version:  version,
		BPM:      bpm,
		Gap:      toTime(0),
		Duet:     duet,
		Singers: [2]string{
			text(firstNonEmpty(headerTags["P1"], headerTags["DUETSINGERP1"])),
			text(firstNonEmpty(headerTags["P2"], headerTags["DUETSINGERP2"])),
		},
	}
	if year, err := strconv.Atoi(strings.TrimSpace(headerTags["YEAR"])); err == nil && year > 0 && year < 10000 {
		song.Year = year
	}
	song.Lyrics = Lyrics{
		Source:   LyricsSourceUltraStar,
		Sync:     SyncNote,
		Title:    song.Title,
		Artist:   song.Artist,
		Language: song.Language,
		Lines:    lyricLines,
	}
	return song, nil
}

// ultraStarHeaders reads the leading "#NAME:value" lines (first value wins,
// names upper-cased) and returns them with the index of the first body line.
func ultraStarHeaders(lines []string) (map[string]string, int) {
	tags := map[string]string{}
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		tag, ok := strings.CutPrefix(trimmed, "#")
		if !ok {
			return tags, index
		}
		name, value, ok := strings.Cut(tag, ":")
		if !ok {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(name))
		if _, seen := tags[key]; !seen && strings.TrimSpace(value) != "" {
			tags[key] = strings.TrimSpace(value)
		}
	}
	return tags, len(lines)
}

type ultraStarNote struct {
	start, end int64
	pitch      int
	kind       NoteKind
	text       string
	lineBreak  bool
	singer     int
}

// readUltraStarNotes reads the body: notes, line breaks ("- beat [beat]"),
// voice switches ("P1", "P2", "P3" = both, also "P 1"), up to "E".
func readUltraStarNotes(lines []string, relative bool) ([]ultraStarNote, bool) {
	var (
		notes        []ultraStarNote
		offset       int64
		pendingBreak bool
		singer       = SingerOne
		duet         bool
	)
	for _, line := range lines {
		line = strings.TrimRight(strings.TrimLeft(line, " \t"), "\r")
		if line == "" {
			continue
		}
		switch kind := line[0]; kind {
		case ':', '*', 'F', 'R', 'G':
			start, length, pitch, text, ok := ultraStarNoteFields(line[1:])
			if !ok {
				continue
			}
			start += offset
			notes = append(notes, ultraStarNote{
				start: start, end: start + max(length, 0), pitch: pitch, kind: ultraStarKind(kind),
				text: text, lineBreak: pendingBreak, singer: singer,
			})
			pendingBreak = false
		case '-':
			pendingBreak = true
			if relative {
				fields := strings.Fields(line[1:])
				shiftText := ""
				switch {
				case len(fields) >= 2:
					shiftText = fields[1]
				case len(fields) == 1:
					shiftText = fields[0]
				}
				if shift, err := strconv.ParseInt(shiftText, 10, 32); err == nil {
					offset += shift
				}
			}
		case 'P':
			voice, err := strconv.Atoi(strings.TrimSpace(line[1:]))
			if err != nil {
				continue
			}
			duet = true
			switch voice {
			case 1:
				singer = SingerOne
			case 2:
				singer = SingerTwo
			default:
				singer = SingerBoth
			}
			offset, pendingBreak = 0, true
		case 'E':
			if strings.TrimSpace(line[1:]) == "" {
				return notes, duet
			}
		}
	}
	return notes, duet
}

// ultraStarNoteFields reads "start length pitch text". The text is everything
// after the single separator following the pitch, so a leading space in it —
// a word boundary — survives.
func ultraStarNoteFields(rest string) (start, length int64, pitch int, text string, ok bool) {
	var numbers [3]int64
	for i := range numbers {
		rest = strings.TrimLeft(rest, " \t")
		end := strings.IndexAny(rest, " \t")
		if end < 0 {
			end = len(rest)
		}
		n, err := strconv.ParseInt(rest[:end], 10, 32)
		if err != nil {
			return 0, 0, 0, "", false
		}
		numbers[i] = n
		rest = rest[end:]
	}
	if len(rest) > 0 {
		rest = rest[1:]
	}
	return numbers[0], numbers[1], int(numbers[2]), rest, true
}

func ultraStarKind(c byte) NoteKind {
	switch c {
	case '*':
		return NoteGolden
	case 'F':
		return NoteFreestyle
	case 'R':
		return NoteRap
	case 'G':
		return NoteGoldenRap
	default:
		return NoteNormal
	}
}

// ultraStarNumber parses a header number; UltraStar files from comma-decimal
// locales write "300,5".
func ultraStarNumber(value string) (float64, bool) {
	n, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", "."), 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return n, true
}

// trimLineEdges drops the word-boundary space before a line's first word and
// after its last, so a line's text is exactly its words.
func trimLineEdges(raws []rawSyllable) {
	starts := []int{}
	for i := range raws {
		if i == 0 || raws[i].lineBreak || raws[i].singer != raws[i-1].singer {
			starts = append(starts, i)
		}
	}
	for n, first := range starts {
		end := len(raws)
		if n+1 < len(starts) {
			end = starts[n+1]
		}
		for i := first; i < end; i++ {
			raws[i].text = strings.TrimLeft(raws[i].text, " \t")
			if raws[i].text != "" {
				break
			}
		}
		for i := end - 1; i >= first; i-- {
			raws[i].text = strings.TrimRight(raws[i].text, " \t")
			if raws[i].text != "" {
				break
			}
		}
	}
}

func hasWords(raws []rawSyllable) bool {
	for _, raw := range raws {
		if strings.TrimSpace(raw.text) != "" {
			return true
		}
	}
	return false
}

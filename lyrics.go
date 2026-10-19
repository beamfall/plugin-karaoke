package karaoke

import (
	"errors"
	"strings"
	"time"
)

// Lyric highlight shapes a client renders. They are the values of the
// "lyrics-sync" Media field and Lyrics.Sync.
const (
	// SyncLine lights one whole line at a time (plain LRC).
	SyncLine = "line"
	// SyncWord wipes each word or syllable as it is sung (enhanced LRC).
	SyncWord = "word"
	// SyncNote wipes each syllable and also carries the sung pitch, so a
	// client can draw a pitch guide and score the singer (UltraStar).
	SyncNote = "note"
	// SyncGraphics means the words are CD+G graphics the client draws from
	// the .cdg sidecar; there is no text timeline.
	SyncGraphics = "graphics"
	// SyncVideo means the words are burned into the picture.
	SyncVideo = "video"
)

// Lyrics sources: where a Lyrics timeline was read from.
const (
	LyricsSourceUltraStar   = "ultrastar"
	LyricsSourceLRC         = "lrc"
	LyricsSourceEmbeddedLRC = "embedded-lrc"
)

// NoteKind is an UltraStar note type. LRC syllables carry NoteNone.
type NoteKind string

const (
	NoteNone      NoteKind = ""
	NoteNormal    NoteKind = "normal"
	NoteGolden    NoteKind = "golden"
	NoteFreestyle NoteKind = "freestyle"
	NoteRap       NoteKind = "rap"
	NoteGoldenRap NoteKind = "golden-rap"
)

// Singer values for LyricLine.Singer. A solo song is all SingerOne.
const (
	SingerOne  = 1
	SingerTwo  = 2
	SingerBoth = 3
)

// lastLineHold is how long the last line of a line-timed file stays lit when
// nothing bounds it: an LRC line says when it starts, never when it stops.
const lastLineHold = 3 * time.Second

// lastSyllableHold bounds the final word of a word-timed file the same way.
const lastSyllableHold = time.Second

// ErrNoLyrics reports a file that parsed but carries no sung words.
var ErrNoLyrics = errors.New("karaoke: no timed lyrics")

// Lyrics is one song's timed words, reduced to the neutral shape a client
// needs to highlight them: lines in display order, each with its syllables.
// Times are offsets from the start of the audio, with any file-level offset
// already applied.
type Lyrics struct {
	// Source is one of the LyricsSource* constants.
	Source string
	// Sync is SyncLine, SyncWord, or SyncNote.
	Sync     string
	Title    string
	Artist   string
	Language string
	Lines    []LyricLine
}

// LyricLine is one displayed line. End is when it stops being sung; a client
// clears or advances the line then.
type LyricLine struct {
	Start  time.Duration
	End    time.Duration
	Singer int
	// Text is the line's words joined, trimmed.
	Text      string
	Syllables []Syllable
}

// Syllable is one highlight step. A leading space in Text marks a word
// boundary, so concatenating a line's syllables yields its words. Pitch is the
// UltraStar note (semitones relative to C4) and is meaningful only when Kind
// is not NoteNone.
type Syllable struct {
	Start time.Duration
	End   time.Duration
	Text  string
	Pitch int
	Kind  NoteKind
}

// rawSyllable is the pre-timeline shape both readers produce. end < 0 means
// "unknown — bound it by what follows".
type rawSyllable struct {
	start     time.Duration
	end       time.Duration
	text      string
	pitch     int
	kind      NoteKind
	lineBreak bool
	singer    int
}

// buildLines groups raw syllables into lines (a lineBreak syllable opens a
// new line, as does a singer change) and fills unknown end times: a syllable
// ends where the next one in its line starts, and a line's last syllable ends
// at the line's end. A line ends at its last explicit end, else where the
// next line of the same singer starts, else after hold.
func buildLines(raws []rawSyllable, hold time.Duration) []LyricLine {
	var lines []LyricLine
	for _, raw := range raws {
		if len(lines) == 0 || raw.lineBreak || lines[len(lines)-1].Singer != raw.singer {
			lines = append(lines, LyricLine{Start: raw.start, End: -1, Singer: raw.singer})
		}
		line := &lines[len(lines)-1]
		line.Syllables = append(line.Syllables, Syllable{
			Start: raw.start, End: raw.end, Text: raw.text, Pitch: raw.pitch, Kind: raw.kind,
		})
	}
	for i := range lines {
		line := &lines[i]
		next := time.Duration(-1)
		for j := i + 1; j < len(lines); j++ {
			if lines[j].Singer == line.Singer {
				next = lines[j].Start
				break
			}
		}
		var builder strings.Builder
		for k := range line.Syllables {
			syllable := &line.Syllables[k]
			builder.WriteString(syllable.Text)
			if syllable.End >= 0 {
				continue
			}
			switch {
			case k+1 < len(line.Syllables):
				syllable.End = line.Syllables[k+1].Start
			case next >= 0:
				syllable.End = next
			default:
				syllable.End = syllable.Start + hold
			}
			if syllable.End < syllable.Start {
				syllable.End = syllable.Start
			}
		}
		line.Text = strings.Join(strings.Fields(builder.String()), " ")
		line.End = line.Syllables[len(line.Syllables)-1].End
		for _, syllable := range line.Syllables {
			line.End = max(line.End, syllable.End)
		}
	}
	return lines
}

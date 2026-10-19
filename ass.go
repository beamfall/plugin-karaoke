package karaoke

import (
	"fmt"
	"strings"
	"time"
)

// ASS layout constants. The canvas matches the 1280x720 picture a server
// composes the words onto.
const (
	assWidth  = 1280
	assHeight = 720
	// assLead is how long a line is on screen before it is sung, so the
	// singer can read ahead.
	assLead = 1500 * time.Millisecond
	// assTitleHold is the longest the title card stays up before the first
	// line; it only shows when the song opens with at least assTitleMin of
	// intro.
	assTitleHold = 5 * time.Second
	assTitleMin  = 2 * time.Second
	// Lines alternate between two rows, measured up from the bottom edge.
	assUpperRowMargin = 330
	assLowerRowMargin = 210
)

// assSingerStyles names the ASS style for each singer. The sung fill color
// differs per voice so duet singers can tell their parts apart.
var assSingerStyles = map[int]string{SingerOne: "One", SingerTwo: "Two", SingerBoth: "Both"}

// RenderASS writes the timed lyrics as an Advanced SubStation Alpha document
// for a 1280x720 picture, the shape a server burns onto a plain background to
// make a karaoke video:
//
//   - Each line appears assLead before it is sung, alternating between an
//     upper and a lower row, so the next line is always visible.
//   - Word- and note-timed syllables fill left to right as they are sung
//     ({\kf}); a line-timed line lights all at once when it starts ({\k}).
//   - Duet voices fill in different colors.
//   - A title card shows over a long enough intro.
//
// The output is deterministic for the same input.
func RenderASS(l Lyrics) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nWrapStyle: 0\nScaledBorderAndShadow: yes\n\n", assWidth, assHeight)
	b.WriteString("[V4+ Styles]\n")
	b.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	// Colors are &HAABBGGRR. Primary is the sung fill, Secondary the unsung text.
	b.WriteString("Style: One,Sans,64,&H0000B4FF,&H00FFFFFF,&H00000000,&H80000000,-1,0,0,0,100,100,0,0,1,4,2,2,60,60,0,1\n")
	b.WriteString("Style: Two,Sans,64,&H00FFC83C,&H00FFFFFF,&H00000000,&H80000000,-1,0,0,0,100,100,0,0,1,4,2,2,60,60,0,1\n")
	b.WriteString("Style: Both,Sans,64,&H005AE65A,&H00FFFFFF,&H00000000,&H80000000,-1,0,0,0,100,100,0,0,1,4,2,2,60,60,0,1\n")
	b.WriteString("Style: Title,Sans,56,&H00FFFFFF,&H00FFFFFF,&H00000000,&H80000000,-1,0,0,0,100,100,0,0,1,3,2,5,60,60,0,1\n\n")
	b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	if len(l.Lines) > 0 {
		first := l.Lines[0].Start
		if title := assTitleCard(l); title != "" && first >= assTitleMin {
			end := min(first-assLead/2, assTitleHold)
			fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Title,,0,0,0,,%s\n", assTime(0), assTime(end), title)
		}
	}

	rowFree := [2]time.Duration{}
	for i, line := range l.Lines {
		row := i % 2
		show := max(line.Start-assLead, rowFree[row], 0)
		end := max(line.End, line.Start)
		rowFree[row] = end
		margin := assUpperRowMargin
		if row == 1 {
			margin = assLowerRowMargin
		}
		style := assSingerStyles[line.Singer]
		if style == "" {
			style = assSingerStyles[SingerOne]
		}
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,%s,,0,0,%d,,%s\n",
			assTime(show), assTime(end), style, margin, assLineText(line, show, l.Sync == SyncLine))
	}
	return b.String()
}

// assLineText builds one line's karaoke-timed text. Every \k duration is in
// centiseconds from the moment the line is shown.
func assLineText(line LyricLine, show time.Duration, wholeLine bool) string {
	var b strings.Builder
	if wholeLine {
		fmt.Fprintf(&b, "{\\k%d}{\\k%d}%s", centis(line.Start-show), centis(line.End-max(line.Start, show)), assEscape(line.Text))
		return b.String()
	}
	cursor := show
	for _, syllable := range line.Syllables {
		if gap := syllable.Start - cursor; gap > 0 {
			fmt.Fprintf(&b, "{\\k%d}", centis(gap))
			cursor = syllable.Start
		}
		duration := max(syllable.End-cursor, 0)
		fmt.Fprintf(&b, "{\\kf%d}%s", centis(duration), assEscape(syllable.Text))
		cursor += time.Duration(centis(duration)) * 10 * time.Millisecond
	}
	return strings.TrimSpace(b.String())
}

func assTitleCard(l Lyrics) string {
	switch {
	case l.Title != "" && l.Artist != "":
		return assEscape(l.Title) + "\\N{\\fs40}" + assEscape(l.Artist)
	default:
		return assEscape(l.Title)
	}
}

// assEscape keeps lyric text from being read as ASS markup: override blocks
// open with "{", and "\" starts an escape such as \N.
var assEscaper = strings.NewReplacer("\\", "＼", "{", "｛", "}", "｝", "\n", " ", "\r", " ")

func assEscape(s string) string { return assEscaper.Replace(s) }

// assTime formats H:MM:SS.cc.
func assTime(d time.Duration) string {
	cs := centis(d)
	return fmt.Sprintf("%d:%02d:%02d.%02d", cs/360000, cs/6000%60, cs/100%60, cs%100)
}

// centis rounds a non-negative duration to whole centiseconds.
func centis(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	return int64((d + 5*time.Millisecond) / (10 * time.Millisecond))
}

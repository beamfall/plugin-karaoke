package karaoke

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// LRC is an LRC lyrics file reduced to what karaoke needs: its tags and its
// timed words.
type LRC struct {
	Title  string
	Artist string
	Album  string
	// Offset is the [offset:] tag, already applied to every time in Lyrics.
	Offset time.Duration
	// WordTimed reports whether any line carries <mm:ss.xx> word tags
	// (the "enhanced" LRC form).
	WordTimed bool
	Lyrics    Lyrics
}

// ParseLRC reads an LRC file: plain, line-timed LRC and the enhanced form with
// <mm:ss.xx> tags before each word. Conventions honored:
//
//   - A line may open with several timestamps ("[00:12.00][01:15.00]chorus");
//     it is sung once per timestamp, and its word tags move with each repeat.
//   - [offset:+N] makes the words come N ms sooner; times clamp at zero.
//   - A timestamp with no words ends the previous line (an instrumental break).
//   - A second line at the same time as the previous one is a translation and
//     is dropped; the first is the one sung.
//   - A word tag with no words after it ends the previous word.
//   - The duet part prefixes "M:", "F:", "D:" are stripped from a line's start.
//
// It returns ErrNoLyrics when no line carries both a timestamp and words.
func ParseLRC(data []byte) (LRC, error) {
	text := decodeText(data, "")
	type entry struct {
		stamps   []time.Duration
		segments []lrcSegment
	}
	var (
		tags    = map[string]string{}
		entries []entry
	)
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		rest := strings.TrimSpace(line)
		var stamps []time.Duration
		for strings.HasPrefix(rest, "[") {
			closing := strings.IndexByte(rest, ']')
			if closing < 0 {
				break
			}
			group := rest[1:closing]
			if at, ok := lrcTimestamp(group); ok {
				stamps = append(stamps, at)
				rest = strings.TrimLeft(rest[closing+1:], " \t")
				continue
			}
			if name, value, isTag := strings.Cut(group, ":"); isTag && len(stamps) == 0 {
				key := strings.ToLower(strings.TrimSpace(name))
				if _, seen := tags[key]; !seen {
					tags[key] = strings.TrimSpace(value)
				}
			}
			break
		}
		if len(stamps) > 0 {
			entries = append(entries, entry{stamps: stamps, segments: lrcSegments(rest)})
		}
	}

	offset := time.Duration(0)
	if raw := strings.TrimSpace(tags["offset"]); raw != "" {
		if ms, err := strconv.Atoi(strings.TrimPrefix(raw, "+")); err == nil {
			offset = time.Duration(ms) * time.Millisecond
		}
	}
	shift := func(at time.Duration) time.Duration { return max(at-offset, 0) }

	wordTimed := false
	// written is the time as the file writes it. Lines are ordered and
	// translations detected on it, not on the shifted start: an offset that
	// clamps several early lines to zero must not make them look like
	// translations of one another.
	type timedLine struct {
		written time.Duration
		start   time.Duration
		words   []rawSyllable
	}
	var lines []timedLine
	for _, e := range entries {
		for _, segment := range e.segments {
			wordTimed = wordTimed || segment.timed
		}
		first := e.stamps[0]
		for _, stamp := range e.stamps {
			start := shift(stamp)
			moved := func(at time.Duration) time.Duration { return shift(at - first + stamp) }
			lines = append(lines, timedLine{written: stamp, start: start, words: lrcWords(e.segments, start, moved)})
		}
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].written < lines[j].written })

	var raws []rawSyllable
	lastWritten := time.Duration(-1)
	for _, line := range lines {
		switch {
		case len(line.words) == 0:
			if n := len(raws); n > 0 && line.start > raws[n-1].start {
				last := &raws[n-1]
				if last.end < 0 || last.end > line.start {
					last.end = line.start
				}
			}
		case line.written == lastWritten:
			// A translation line; keep the first.
		default:
			lastWritten = line.written
			raws = append(raws, line.words...)
		}
	}
	if len(raws) == 0 {
		return LRC{}, ErrNoLyrics
	}
	sync, hold := SyncLine, lastLineHold
	if wordTimed {
		sync, hold = SyncWord, lastSyllableHold
	}
	return LRC{
		Title:     tags["ti"],
		Artist:    tags["ar"],
		Album:     tags["al"],
		Offset:    offset,
		WordTimed: wordTimed,
		Lyrics: Lyrics{
			Source: LyricsSourceLRC,
			Sync:   sync,
			Title:  tags["ti"],
			Artist: tags["ar"],
			Lines:  buildLines(raws, hold),
		},
	}, nil
}

// lrcSegment is a run of line text and the word tag in front of it.
type lrcSegment struct {
	at    time.Duration
	timed bool
	text  string
}

// lrcSegments cuts a line's text at each <mm:ss.xx> word tag. A "<" that opens
// no timestamp is text.
func lrcSegments(text string) []lrcSegment {
	segments := []lrcSegment{{text: text}}
	search := 0
	for {
		current := &segments[len(segments)-1]
		open := strings.IndexByte(current.text[search:], '<')
		if open < 0 {
			return segments
		}
		open += search
		closing := strings.IndexByte(current.text[open+1:], '>')
		if closing < 0 {
			return segments
		}
		closing += open + 1
		at, ok := lrcTimestamp(current.text[open+1 : closing])
		if !ok {
			search = open + 1
			continue
		}
		whole := current.text
		current.text = whole[:open]
		segments = append(segments, lrcSegment{at: at, timed: true, text: whole[closing+1:]})
		search = 0
	}
}

// lrcWords turns one timestamped line into syllables; nil for a line with no
// words. The first syllable carries the line break, and a word boundary is a
// leading space on the syllable after it.
func lrcWords(segments []lrcSegment, start time.Duration, moved func(time.Duration) time.Duration) []rawSyllable {
	var out []rawSyllable
	spacePending := false
	for index, segment := range segments {
		at := start
		if segment.timed {
			at = moved(segment.at)
		}
		decoded := segment.text
		if index == 0 || len(out) == 0 {
			decoded = stripPartPrefix(strings.TrimLeft(decoded, " \t"))
		}
		body := strings.TrimSpace(decoded)
		if body == "" {
			if n := len(out); n > 0 && segment.timed && at > out[n-1].start {
				out[n-1].end = at
			}
			spacePending = spacePending || decoded != ""
			continue
		}
		boundary := spacePending || strings.HasPrefix(decoded, " ") || strings.HasPrefix(decoded, "\t")
		if len(out) > 0 && boundary {
			body = " " + body
		}
		out = append(out, rawSyllable{
			start: at, end: -1, text: body, lineBreak: len(out) == 0, singer: SingerOne,
		})
		spacePending = strings.HasSuffix(decoded, " ") || strings.HasSuffix(decoded, "\t")
	}
	return out
}

func stripPartPrefix(line string) string {
	for _, prefix := range []string{"M:", "F:", "D:"} {
		if rest, ok := strings.CutPrefix(line, prefix); ok {
			return strings.TrimLeft(rest, " \t")
		}
	}
	return line
}

// lrcTimestamp reads "mm:ss", "mm:ss.x", "mm:ss.xx", "mm:ss.xxx" (or ":" for
// the fraction separator). Seconds must be below 60.
func lrcTimestamp(written string) (time.Duration, bool) {
	written = strings.TrimSpace(written)
	minutesText, rest, ok := strings.Cut(written, ":")
	if !ok {
		return 0, false
	}
	minutes, ok := digits(minutesText)
	if !ok {
		return 0, false
	}
	secondsText, fraction, hasFraction := rest, "", false
	if at := strings.IndexAny(rest, ".:"); at >= 0 {
		secondsText, fraction, hasFraction = rest[:at], rest[at+1:], true
	}
	seconds, ok := digits(secondsText)
	if !ok || seconds >= 60 {
		return 0, false
	}
	millis := 0
	if hasFraction {
		if _, ok := digits(fraction); !ok {
			return 0, false
		}
		padded := (fraction + "000")[:3]
		millis, _ = strconv.Atoi(padded)
	}
	if minutes > 100_000 {
		return 0, false
	}
	return time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second + time.Duration(millis)*time.Millisecond, true
}

// digits parses a non-empty run of ASCII digits (at most 9, so it never
// overflows).
func digits(s string) (int, bool) {
	if s == "" || len(s) > 9 {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

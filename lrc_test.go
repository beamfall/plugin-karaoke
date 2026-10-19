package karaoke

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func lineSummary(lines []LyricLine) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = line.Start.String() + "-" + line.End.String() + " " + line.Text
	}
	return out
}

func TestParseLRCLineTimed(t *testing.T) {
	lrc, err := ParseLRC([]byte("\xEF\xBB\xBF[ar: Adele ]\r\n[ti:Hello]\r\n[offset:+500]\r\n" +
		"[00:05.00]Hello, it's me\r\n" +
		"[00:05.00]Bonjour, c'est moi\r\n" + // translation at the same time: dropped
		"[00:09.5]I was wondering\r\n" +
		"[00:20.00]\r\n" + // blank: ends the previous line (instrumental break)
		"[00:30.000][01:00.00]M: Chorus line\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if lrc.Artist != "Adele" || lrc.Title != "Hello" || lrc.Offset != ms(500) || lrc.WordTimed {
		t.Fatalf("tags = %+v", lrc)
	}
	if lrc.Lyrics.Sync != SyncLine || lrc.Lyrics.Source != LyricsSourceLRC {
		t.Fatalf("sync/source = %q/%q", lrc.Lyrics.Sync, lrc.Lyrics.Source)
	}
	want := []string{
		"4.5s-9s Hello, it's me",
		"9s-19.5s I was wondering",
		"29.5s-59.5s Chorus line",
		"59.5s-1m2.5s Chorus line",
	}
	if got := lineSummary(lrc.Lyrics.Lines); !reflect.DeepEqual(got, want) {
		t.Fatalf("lines =\n%q\nwant\n%q", got, want)
	}
}

func TestParseLRCWordTimed(t *testing.T) {
	lrc, err := ParseLRC([]byte("[00:01.00]<00:01.00>Twin<00:01.50>kle <00:02.00>twin<00:02.50>kle<00:03.20>\n" +
		"[00:04.00]<00:04.00>lit<00:04.40>tle <00:05.00>star\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !lrc.WordTimed || lrc.Lyrics.Sync != SyncWord {
		t.Fatalf("WordTimed = %v sync %q", lrc.WordTimed, lrc.Lyrics.Sync)
	}
	first := lrc.Lyrics.Lines[0]
	if first.Text != "Twinkle twinkle" {
		t.Fatalf("text = %q", first.Text)
	}
	gotTexts := []string{}
	for _, s := range first.Syllables {
		gotTexts = append(gotTexts, s.Text)
	}
	if !reflect.DeepEqual(gotTexts, []string{"Twin", "kle", " twin", "kle"}) {
		t.Fatalf("syllables = %q", gotTexts)
	}
	// A trailing empty word tag ends the last word.
	if last := first.Syllables[3]; last.Start != ms(2500) || last.End != ms(3200) || first.End != ms(3200) {
		t.Fatalf("last syllable = %+v line end %v", last, first.End)
	}
	if mid := first.Syllables[1]; mid.End != ms(2000) {
		t.Fatalf("syllable end = %v, want next start", mid.End)
	}
	// The final word of the file is held lastSyllableHold.
	second := lrc.Lyrics.Lines[1]
	if end := second.Syllables[2].End; end != ms(5000)+lastSyllableHold {
		t.Fatalf("final hold = %v", end)
	}
}

func TestParseLRCRepeatMovesWordTags(t *testing.T) {
	lrc, err := ParseLRC([]byte("[00:10.00][00:40.00]<00:10.00>Na <00:10.50>na\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := lrc.Lyrics.Lines[1].Syllables[1].Start; got != ms(40500) {
		t.Fatalf("repeated word tag = %v, want 40.5s", got)
	}
}

func TestParseLRCRejects(t *testing.T) {
	for name, input := range map[string]string{
		"empty":            "",
		"tags only":        "[ar:Adele]\n[ti:Hello]\n",
		"plain lyrics":     "Hello, it's me\nI was wondering\n",
		"seconds over 59":  "[00:61.00]nope\n",
		"blank lines only": "[00:01.00]\n[00:02.00]  \n",
	} {
		if _, err := ParseLRC([]byte(input)); !errors.Is(err, ErrNoLyrics) {
			t.Errorf("%s: err = %v, want ErrNoLyrics", name, err)
		}
	}
}

func TestParseLRCNegativeOffsetAndLegacyEncoding(t *testing.T) {
	// "Café" in Windows-1252, and an offset that pushes the start below zero.
	lrc, err := ParseLRC([]byte("[offset:-250]\n[00:00.00]Caf\xe9\n"))
	if err != nil {
		t.Fatal(err)
	}
	line := lrc.Lyrics.Lines[0]
	if line.Text != "Café" || line.Start != ms(250) {
		t.Fatalf("line = %+v", line)
	}
	lrc, err = ParseLRC([]byte("[offset:5000]\n[00:01.00]early\n"))
	if err != nil || lrc.Lyrics.Lines[0].Start != 0 {
		t.Fatalf("clamped start = %v err %v", lrc.Lyrics.Lines[0].Start, err)
	}
}

func TestParseLRCUTF16(t *testing.T) {
	// "[00:01.00]Hi" as UTF-16LE with BOM.
	data := []byte{0xFF, 0xFE}
	for _, r := range "[00:01.00]Hi\n" {
		data = append(data, byte(r), 0)
	}
	lrc, err := ParseLRC(data)
	if err != nil || lrc.Lyrics.Lines[0].Text != "Hi" {
		t.Fatalf("lrc = %+v err %v", lrc, err)
	}
}

func TestLRCTimestamp(t *testing.T) {
	for input, want := range map[string]time.Duration{
		"01:02":     ms(62000),
		"01:02.5":   ms(62500),
		"01:02.05":  ms(62050),
		"01:02.005": ms(62005),
		"01:02:50":  ms(62500),
		" 00:00.00": 0,
	} {
		if got, ok := lrcTimestamp(input); !ok || got != want {
			t.Errorf("lrcTimestamp(%q) = %v, %v; want %v", input, got, ok, want)
		}
	}
	for _, input := range []string{"", "ar", "1:x", "00:60", "a:00", "00:00.x", "1234567890:00"} {
		if _, ok := lrcTimestamp(input); ok {
			t.Errorf("lrcTimestamp(%q) accepted", input)
		}
	}
}

func TestParseLRCOffsetClampKeepsLines(t *testing.T) {
	lrc, err := ParseLRC([]byte("[offset:+5000]\n[00:01.00]one\n[00:02.00]two\n[00:03.00]three\n[00:10.00]four\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, line := range lrc.Lyrics.Lines {
		got = append(got, line.Text)
	}
	if !reflect.DeepEqual(got, []string{"one", "two", "three", "four"}) {
		t.Fatalf("lines = %q: clamped lines must not read as translations", got)
	}
}

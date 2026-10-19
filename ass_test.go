package karaoke

import (
	"strings"
	"testing"
	"time"
)

func dialogues(doc string) []string {
	var out []string
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "Dialogue: ") {
			out = append(out, line)
		}
	}
	return out
}

func TestRenderASSWordTimed(t *testing.T) {
	lyrics := Lyrics{
		Sync: SyncWord, Title: "Twinkle", Artist: "Trad.",
		Lines: []LyricLine{
			{Start: ms(3000), End: ms(4000), Singer: SingerOne, Text: "Twinkle", Syllables: []Syllable{
				{Start: ms(3000), End: ms(3500), Text: "Twin"},
				{Start: ms(3600), End: ms(4000), Text: "kle"},
			}},
			{Start: ms(4200), End: ms(5000), Singer: SingerTwo, Text: "lit{tle}", Syllables: []Syllable{
				{Start: ms(4200), End: ms(5000), Text: `lit{tle}\N`},
			}},
		},
	}
	doc := RenderASS(lyrics)
	got := dialogues(doc)
	want := []string{
		// Title card over the intro, ending half a lead before the first line.
		`Dialogue: 0,0:00:00.00,0:00:02.25,Title,,0,0,0,,Twinkle\N{\fs40}Trad.`,
		// Shown 1.5s early on the upper row: 1.5s unsung, then fill-wipes with
		// the 0.1s gap between syllables kept.
		`Dialogue: 0,0:00:01.50,0:00:04.00,One,,0,0,330,,{\k150}{\kf50}Twin{\k10}{\kf40}kle`,
		// Second voice on the lower row; markup in the words is neutralized.
		"Dialogue: 0,0:00:02.70,0:00:05.00,Two,,0,0,210,,{\\k150}{\\kf80}lit｛tle｝＼N",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("dialogues =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !strings.Contains(doc, "PlayResX: 1280\nPlayResY: 720") || RenderASS(lyrics) != doc {
		t.Fatal("canvas missing or output not deterministic")
	}
}

func TestRenderASSLineTimedAndRows(t *testing.T) {
	lines := []LyricLine{}
	for i := range 3 {
		start := time.Duration(i) * time.Second
		lines = append(lines, LyricLine{Start: start, End: start + 4*time.Second, Singer: SingerBoth, Text: "la",
			Syllables: []Syllable{{Start: start, End: start + 4*time.Second, Text: "la"}}})
	}
	got := dialogues(RenderASS(Lyrics{Sync: SyncLine, Title: "No intro", Lines: lines}))
	want := []string{
		// No title card: the song starts at once. Line-timed lines light whole.
		`Dialogue: 0,0:00:00.00,0:00:04.00,Both,,0,0,330,,{\k0}{\k400}la`,
		`Dialogue: 0,0:00:00.00,0:00:05.00,Both,,0,0,210,,{\k100}{\k400}la`,
		// The third line waits for its row to clear rather than overlap.
		`Dialogue: 0,0:00:04.00,0:00:06.00,Both,,0,0,330,,{\k0}{\k200}la`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("dialogues =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestAssTime(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                                    "0:00:00.00",
		ms(1234):                             "0:00:01.23",
		ms(1235):                             "0:00:01.24",
		time.Hour + 2*time.Minute + ms(3040): "1:02:03.04",
		-time.Second:                         "0:00:00.00",
	} {
		if got := assTime(d); got != want {
			t.Errorf("assTime(%v) = %s, want %s", d, got, want)
		}
	}
}

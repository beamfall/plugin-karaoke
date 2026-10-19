package karaoke

import (
	"reflect"
	"testing"
	"unicode/utf8"

	sdk "github.com/beamfall/plugin-sdk"
)

func FuzzParseFilename(f *testing.F) {
	for _, seed := range []string{
		"/lib/SC8125-03 - Toto - Africa.mp3",
		"/lib/SF001/07 - Oasis - Wonderwall.mp3",
		"/lib/Wonderwall [Vocal Guide] (In the Style of Oasis).mp4",
		"/lib/UB40 - Red Red Wine.mp3",
		"/lib/[Karaoke].mp3",
		"",
	} {
		f.Add(seed, "Title (Karaoke)", "SC 8001", "4/15")
	}
	p := New()
	f.Fuzz(func(t *testing.T, path, title, album, track string) {
		probe := sdk.ParseProbe{Tags: map[string]string{"title": title, "album": album, "track": track}}
		first, ok := p.Parse(path, probe)
		second, ok2 := p.Parse(path, probe)
		if !reflect.DeepEqual(first, second) || ok != ok2 {
			t.Fatalf("not deterministic: %+v/%v then %+v/%v", first, ok, second, ok2)
		}
		if ok && first.Title == "" {
			t.Fatal("ok with an empty title")
		}
		if !ok && !reflect.DeepEqual(first, sdk.ParsedMedia{}) {
			t.Fatalf("refusal carried data: %+v", first)
		}
	})
}

func FuzzParseLRC(f *testing.F) {
	for _, seed := range []string{
		"[ar:A]\n[offset:+500]\n[00:05.00]Hello\n[00:06.00]\n[00:07.00][00:30.00]x\n",
		"[00:01.00]<00:01.00>Twin<00:01.50>kle <00:02.00>",
		"[00:01.00]M: a <bad> <00:99.00>b",
		"\xff\xfe[\x000\x000\x00",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		lrc, err := ParseLRC(data)
		if err != nil {
			return
		}
		checkLyrics(t, lrc.Lyrics)
	})
}

func FuzzParseUltraStar(f *testing.F) {
	for _, seed := range []string{
		"#TITLE:T\n#BPM:300\n#GAP:1000\n: 0 4 5 Is\n: 4 4 7  this\n- 10\n: 12 4 7 real\nE\n",
		"#TITLE:R\n#BPM:60\n#RELATIVE:yes\n: 0 4 0 one\n- 8 10\n: 0 4 0 ~\n",
		"#VERSION:1.1.0\n#TITLE:D\n#BPM:60\nP1\n: 0 4 0 hi\nP2\n: 8 4 0 yo\nP3\n* 30 -4 0 both\n",
		"#TITLE:x\n#BPM:0,0001\n#GAP:-99999999999\n: 2147483647 2147483647 0 x\n",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		song, err := ParseUltraStar(data)
		if err != nil {
			return
		}
		checkLyrics(t, song.Lyrics)
	})
}

// checkLyrics asserts the invariants every parsed timeline keeps.
func checkLyrics(t *testing.T, lyrics Lyrics) {
	t.Helper()
	if len(lyrics.Lines) == 0 {
		t.Fatal("parsed lyrics with no lines")
	}
	for i, line := range lyrics.Lines {
		if !utf8.ValidString(line.Text) {
			t.Fatalf("line %d text is not UTF-8: %q", i, line.Text)
		}
		if line.Start < 0 || line.End < line.Start || len(line.Syllables) == 0 {
			t.Fatalf("line %d malformed: %+v", i, line)
		}
		if line.Singer < SingerOne || line.Singer > SingerBoth {
			t.Fatalf("line %d singer %d", i, line.Singer)
		}
		if i > 0 && line.Start < lyrics.Lines[i-1].Start {
			t.Fatalf("line %d starts before line %d", i, i-1)
		}
		for j, s := range line.Syllables {
			if s.Start < 0 || s.End < s.Start {
				t.Fatalf("line %d syllable %d malformed: %+v", i, j, s)
			}
		}
	}
}

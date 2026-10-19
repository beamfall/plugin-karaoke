package karaoke

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseUltraStarSolo(t *testing.T) {
	song, err := ParseUltraStar([]byte("#TITLE:Twinkle\n#ARTIST:Traditional\n#LANGUAGE:English\n" +
		"#EDITION:Kids\n#YEAR:1806\n#MP3:twinkle.mp3\n#VIDEO:twinkle.mp4\n#COVER:cover.jpg\n" +
		"#BPM:150,5\n#GAP:2000\n" +
		": 0 2 0 Twin\n" +
		"* 2 2 0 kle \n" +
		": 4 2 7 twin\n" +
		": 6 2 9 ~\n" + // sustain carried to a new pitch: its own syllable
		"F 8 2 9 kle\n" +
		"- 12\n" +
		"R 14 2 9  lit\n" +
		"G 16 2 7 tle\n" +
		"E\n" +
		": 99 1 0 ignored after E\n"))
	if err != nil {
		t.Fatal(err)
	}
	if song.Title != "Twinkle" || song.Artist != "Traditional" || song.Language != "English" ||
		song.Edition != "Kids" || song.Year != 1806 || song.Audio != "twinkle.mp3" ||
		song.Video != "twinkle.mp4" || song.Cover != "cover.jpg" || song.BPM != 150.5 || song.Gap != ms(2000) || song.Duet {
		t.Fatalf("header = %+v", song)
	}
	lyrics := song.Lyrics
	if lyrics.Sync != SyncNote || len(lyrics.Lines) != 2 {
		t.Fatalf("lyrics = %+v", lyrics)
	}
	first := lyrics.Lines[0]
	if first.Text != "Twinkle twinkle" {
		t.Fatalf("first line = %q", first.Text)
	}
	beat := func(b float64) int { return int(2000 + b*60000/(150.5*4) + 0.5) }
	twin, held := first.Syllables[2], first.Syllables[3]
	if twin.Text != "twin" || twin.Start != ms(beat(4)) || twin.End != ms(beat(6)) || twin.Pitch != 7 {
		t.Fatalf("syllable before the sustain = %+v", twin)
	}
	if held.Text != "" || held.Start != ms(beat(6)) || held.End != ms(beat(8)) || held.Pitch != 9 || held.Kind != NoteNormal {
		t.Fatalf("sustain keeps its own pitch = %+v", held)
	}
	kinds := []NoteKind{}
	for _, s := range first.Syllables {
		kinds = append(kinds, s.Kind)
	}
	if !reflect.DeepEqual(kinds, []NoteKind{NoteNormal, NoteGolden, NoteNormal, NoteNormal, NoteFreestyle}) {
		t.Fatalf("kinds = %v", kinds)
	}
	second := lyrics.Lines[1]
	if second.Text != "little" || second.Syllables[0].Text != "lit" || second.Syllables[0].Kind != NoteRap ||
		second.Syllables[1].Kind != NoteGoldenRap {
		t.Fatalf("second line = %+v", second)
	}
}

func TestParseUltraStarRelative(t *testing.T) {
	song, err := ParseUltraStar([]byte("#TITLE:Rel\n#BPM:60\n#RELATIVE:yes\n" +
		": 0 4 0 one\n- 8 10\n: 0 4 0 two\n"))
	if err != nil {
		t.Fatal(err)
	}
	// 60 BPM ⇒ a beat is 250ms; the second line's beats shift by 10.
	if got := song.Lyrics.Lines[1].Start; got != ms(2500) {
		t.Fatalf("relative line start = %v", got)
	}
}

func TestParseUltraStarDuet(t *testing.T) {
	song, err := ParseUltraStar([]byte("#VERSION:1.1.0\n#TITLE:Duet\n#ARTIST:Two\n#AUDIO:duet.ogg\n" +
		"#P1:Alice\n#P2:Bob\n#BPM:60\n" +
		"P1\n: 0 4 0 hi\n- 6\n: 20 4 0 bye\n" +
		"P2\n: 8 4 0 yo\n" +
		"P3\n: 30 4 0 both\nE\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !song.Duet || song.Singers != [2]string{"Alice", "Bob"} || song.Audio != "duet.ogg" {
		t.Fatalf("duet header = %+v", song)
	}
	got := []string{}
	for _, line := range song.Lyrics.Lines {
		got = append(got, line.Text+"/"+string(rune('0'+line.Singer)))
	}
	if !reflect.DeepEqual(got, []string{"hi/1", "yo/2", "bye/1", "both/3"}) {
		t.Fatalf("lines in sung order = %v", got)
	}
}

func TestParseUltraStarLegacyEncoding(t *testing.T) {
	// "Liebe für dich" in Windows-1252 under a declared encoding.
	song, err := ParseUltraStar([]byte("#TITLE:F\xfcr dich\n#ENCODING:CP1252\n#BPM:100\n: 0 1 0 f\xfcr\nE\n"))
	if err != nil {
		t.Fatal(err)
	}
	if song.Title != "Für dich" || song.Lyrics.Lines[0].Text != "für" {
		t.Fatalf("decoded = %q / %q", song.Title, song.Lyrics.Lines[0].Text)
	}
}

func TestParseUltraStarRejects(t *testing.T) {
	cases := map[string]struct {
		input string
		want  error
	}{
		"readme":          {"This is a readme.\n", ErrNotUltraStar},
		"no bpm":          {"#TITLE:x\n: 0 1 0 a\n", ErrUltraStarBPM},
		"zero bpm":        {"#TITLE:x\n#BPM:0\n: 0 1 0 a\n", ErrUltraStarBPM},
		"bad bpm":         {"#TITLE:x\n#BPM:fast\n: 0 1 0 a\n", ErrUltraStarBPM},
		"no notes":        {"#TITLE:x\n#BPM:100\nE\n", ErrNoLyrics},
		"malformed notes": {"#TITLE:x\n#BPM:100\n: a b c d\n: 1\n", ErrNoLyrics},
	}
	for name, c := range cases {
		if _, err := ParseUltraStar([]byte(c.input)); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	if _, err := ParseUltraStar([]byte("#TITLE:x\n#VERSION:2.0.0\n#BPM:100\n: 0 1 0 a\n")); err == nil ||
		!strings.Contains(err.Error(), "not supported") {
		t.Errorf("version 2: err = %v", err)
	}
}

func TestParseUltraStarSustainEndsWord(t *testing.T) {
	song, err := ParseUltraStar([]byte("#TITLE:x\n#BPM:60\n: 0 1 0 Oh\n: 1 1 2 ~ \n: 2 1 0 yes\nE\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := song.Lyrics.Lines[0].Text; got != "Oh yes" {
		t.Fatalf("text = %q, want the word boundary kept", got)
	}
}

func TestParseUltraStarClampsHostileGap(t *testing.T) {
	for _, gap := range []string{"10000000000000", "1e300", "-5000"} {
		song, err := ParseUltraStar([]byte("#TITLE:x\n#BPM:60\n#GAP:" + gap + "\n: 0 1 0 a\nE\n"))
		if err != nil {
			t.Fatal(err)
		}
		if song.Gap < 0 || song.Lyrics.Lines[0].Start < 0 {
			t.Errorf("#GAP:%s → Gap %v, first line %v", gap, song.Gap, song.Lyrics.Lines[0].Start)
		}
	}
}

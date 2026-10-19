package karaoke_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"

	karaoke "github.com/beamfall/plugin-karaoke"
)

func fieldMap(fields []sdk.ParsedTextField) map[string]string {
	out := map[string]string{}
	for _, f := range fields {
		out[f.Name] = f.Text
	}
	return out
}

func TestParseMediaFields(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"SC8125-03 - Toto - Africa.mp3":                        "",
		"SC8125-03 - Toto - Africa.CDG":                        cdgBytes,
		"duet/song.mp3":                                        "",
		"duet/song.txt":                                        "#TITLE:Islands\n#ARTIST:Kenny & Dolly\n#LANGUAGE:English\n#GENRE:Country\n#BPM:100\nP1\n: 0 1 0 a\nP2\n: 2 1 0 b\nE\n",
		"Wonderwall [Vocal Guide] (In the Style of Oasis).mp4": "",
	})
	cases := []struct {
		name  string
		path  string
		probe sdk.ParseProbe
		want  map[string]string
	}{
		{
			name: "MP3+G with upper-case .CDG and a known label",
			path: filepath.Join(dir, "SC8125-03 - Toto - Africa.mp3"),
			want: map[string]string{"artist": "Toto", "disc-id": "SC8125", "brand": "Sound Choice", "lyrics-sync": karaoke.SyncGraphics},
		},
		{
			name: "UltraStar duet: header metadata and singers",
			path: filepath.Join(dir, "duet/song.mp3"),
			want: map[string]string{"artist": "Kenny & Dolly", "language": "English", "genre": "Country",
				"singers": karaoke.SingersDuet, "lyrics-sync": karaoke.SyncNote},
		},
		{
			name:  "video with markers: style-of artist and guide vocals",
			path:  filepath.Join(dir, "Wonderwall [Vocal Guide] (In the Style of Oasis).mp4"),
			probe: sdk.ParseProbe{HasVideo: true},
			want:  map[string]string{"artist": "Oasis", "vocals": karaoke.VocalsGuide, "lyrics-sync": karaoke.SyncVideo},
		},
		{
			name: "duet marker and unknown label prefix",
			path: "/lib/ZZK123-01 - Elton John & Kiki Dee - Don't Go Breaking My Heart (Duet).mp3",
			probe: sdk.ParseProbe{Tags: map[string]string{
				"language": "en", "genre": "Pop",
				"lyrics-eng": "[00:01.00]Don't go breaking my heart",
			}},
			want: map[string]string{"artist": "Elton John & Kiki Dee", "disc-id": "ZZK123", "language": "en",
				"genre": "Pop", "singers": karaoke.SingersDuet, "lyrics-sync": karaoke.SyncLine},
		},
		{
			name: "sidecar file yields nothing",
			path: filepath.Join(dir, "SC8125-03 - Toto - Africa.CDG"),
			want: map[string]string{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := fieldMap(karaoke.New().ParseMediaFields(c.path, c.probe))
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("fields = %v, want %v", got, c.want)
			}
		})
	}
}

func TestParseClassFromEmbeddedLRC(t *testing.T) {
	parsed, ok := karaoke.New().Parse("/lib/Adele - Hello.m4a", sdk.ParseProbe{Tags: map[string]string{
		"lyrics": "[00:01.00]<00:01.00>Hel<00:01.50>lo",
	}})
	if !ok || parsed.Class != karaoke.ClassLRC {
		t.Fatalf("parsed = %+v ok %v", parsed, ok)
	}
	fields := fieldMap(karaoke.New().ParseMediaFields("/lib/Adele - Hello.m4a", sdk.ParseProbe{Tags: map[string]string{
		"lyrics": "[00:01.00]<00:01.00>Hel<00:01.50>lo",
	}}))
	if fields["lyrics-sync"] != karaoke.SyncWord {
		t.Fatalf("sync = %q", fields["lyrics-sync"])
	}
	// Plain, unsynchronized lyrics do not make a karaoke track.
	parsed, _ = karaoke.New().Parse("/lib/Adele - Hello.m4a", sdk.ParseProbe{Tags: map[string]string{
		"lyrics": "Hello, it's me",
	}})
	if parsed.Class != karaoke.ClassBacking {
		t.Fatalf("plain lyrics class = %q", parsed.Class)
	}
}

func TestTimedLyrics(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"us/a.mp3":     "",
		"us/a.txt":     strings.Replace(ultraStarSong, "queen.mp3", "a.mp3", 1),
		"us/a.lrc":     "[00:01.00]loses to UltraStar\n",
		"lrc/b.mp3":    "",
		"lrc/b.lrc":    "[00:01.00]from the sidecar\n",
		"cdg/c.mp3":    "",
		"cdg/c.cdg":    cdgBytes,
		"broken/d.mp3": "",
		"broken/d.lrc": "not lyrics",
		"cdglrc/f.mp3": "",
		"cdglrc/f.cdg": cdgBytes,
		"cdglrc/f.lrc": "[00:01.00]the graphics are the words\n",
		"badcdg/e.mp3": "",
		"badcdg/e.cdg": "not packets",
	})
	p := karaoke.New()
	cases := []struct {
		path   string
		probe  sdk.ParseProbe
		source string
		first  string
		ok     bool
	}{
		{path: "us/a.mp3", source: karaoke.LyricsSourceUltraStar, first: "Is this", ok: true},
		{path: "lrc/b.mp3", source: karaoke.LyricsSourceLRC, first: "from the sidecar", ok: true},
		{path: "broken/d.mp3", probe: sdk.ParseProbe{Tags: map[string]string{"lyrics": "[00:02.00]embedded"}},
			source: karaoke.LyricsSourceEmbeddedLRC, first: "embedded", ok: true},
		{path: "cdg/c.mp3"},
		{path: "cdglrc/f.mp3"},
		{path: "cdglrc/f.mp4", probe: sdk.ParseProbe{HasVideo: true, Tags: map[string]string{"lyrics": "[00:01.00]burned in"}}},
		{path: "us/a.txt"},
	}
	for _, c := range cases {
		lyrics, ok := p.TimedLyrics(filepath.Join(dir, c.path), c.probe)
		if ok != c.ok || lyrics.Source != c.source || (ok && lyrics.Lines[0].Text != c.first) {
			t.Errorf("%s: lyrics = %+v ok %v", c.path, lyrics, ok)
		}
	}
	// A .cdg that is not whole 24-byte packets is not MP3+G.
	if parsed, _ := p.Parse(filepath.Join(dir, "badcdg/e.mp3"), sdk.ParseProbe{}); parsed.Class != karaoke.ClassBacking {
		t.Errorf("bad cdg class = %q", parsed.Class)
	}
}

func TestUltraStarFolderScanIsBounded(t *testing.T) {
	files := map[string]string{
		"big/song.mp3":  "",
		"big/other.txt": strings.Replace(ultraStarSong, "queen.mp3", "song.mp3", 1),
	}
	for i := range 70 {
		files["big/filler"+string(rune('a'+i%26))+string(rune('a'+i/26))+".ogg"] = ""
	}
	dir := writeFiles(t, files)
	parsed, _ := karaoke.New().Parse(filepath.Join(dir, "big/song.mp3"), sdk.ParseProbe{})
	if parsed.Class != karaoke.ClassBacking {
		t.Fatalf("large folder was scanned: class = %q", parsed.Class)
	}
}

func TestUltraStarVideoIsPartOfTheSong(t *testing.T) {
	txt := strings.Replace(ultraStarSong, "#MP3:queen.mp3", "#MP3:Queen - X.mp3\n#VIDEO:Queen - X.mp4", 1)
	for name, files := range map[string]map[string]string{
		"same stem": {"Queen - X.mp3": "", "Queen - X.mp4": "", "Queen - X.txt": txt},
		"folder":    {"Queen - X.mp3": "", "Queen - X.mp4": "", "notes.txt": txt},
	} {
		dir := writeFiles(t, files)
		if parsed, ok := karaoke.New().Parse(filepath.Join(dir, "Queen - X.mp3"), sdk.ParseProbe{}); !ok || parsed.Class != karaoke.ClassUltraStar {
			t.Errorf("%s: audio = %+v ok %v", name, parsed, ok)
		}
		if parsed, ok := karaoke.New().Parse(filepath.Join(dir, "Queen - X.mp4"), sdk.ParseProbe{HasVideo: true}); ok {
			t.Errorf("%s: the song's video parsed as its own song: %+v", name, parsed)
		}
	}
}

func TestUltraStarFolderScanIsOrderIndependent(t *testing.T) {
	for _, order := range [][]string{{"a", "b"}, {"b", "a"}} {
		dir := t.TempDir()
		for _, name := range order {
			content := strings.Replace(strings.Replace(ultraStarSong, "queen.mp3", "song.mp3", 1), "Bohemian Rhapsody", "Song "+name, 1)
			if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "song.mp3"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if parsed, _ := karaoke.New().Parse(filepath.Join(dir, "song.mp3"), sdk.ParseProbe{}); parsed.Title != "Song a" {
			t.Errorf("created %v: title = %q, want the first .txt by name", order, parsed.Title)
		}
	}
}

func TestComposedPicture(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"cdg/a.mp3":   "",
		"cdg/a.cdg":   cdgBytes,
		"us/b.mp3":    "",
		"us/b.txt":    strings.Replace(ultraStarSong, "queen.mp3", "b.mp3", 1),
		"lrc/c.ogg":   "",
		"lrc/c.lrc":   "[00:01.00]sidecar words\n",
		"plain/d.mp3": "",
		"video/e.mp4": "",
	})
	p := karaoke.New()
	graphics, text, ok := p.ComposedPicture(filepath.Join(dir, "cdg/a.mp3"))
	if !ok || graphics != filepath.Join(dir, "cdg/a.cdg") || text != "" {
		t.Errorf("cdg = %q %q %v", graphics, text, ok)
	}
	graphics, text, ok = p.ComposedPicture(filepath.Join(dir, "us/b.mp3"))
	if !ok || graphics != "" || !strings.Contains(text, `{\kf`) || !strings.Contains(text, "this") {
		t.Errorf("ultrastar = %q %v", graphics, ok)
	}
	if _, text, ok = p.ComposedPicture(filepath.Join(dir, "lrc/c.ogg")); !ok || !strings.Contains(text, "sidecar words") {
		t.Errorf("lrc = %v", ok)
	}
	for _, path := range []string{"plain/d.mp3", "video/e.mp4", "cdg/a.cdg", "us/b.txt"} {
		if g, x, ok := p.ComposedPicture(filepath.Join(dir, path)); ok {
			t.Errorf("%s composed: %q %q", path, g, x)
		}
	}
}

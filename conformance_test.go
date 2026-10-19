package karaoke_test

import (
	"os"
	"path/filepath"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"

	karaoke "github.com/beamfall/plugin-karaoke"
)

func TestBundleConformance(t *testing.T) {
	sdktest.RunBundleConformance(t, karaoke.Bundle())
	// The bundle is Media-field-first: no Entity Kinds, so it can never
	// collide with another plugin's kind names.
	sdktest.RequireEntityKindVocab(t, karaoke.Bundle(), nil)
}

// writeFiles creates files (name → content) in a fresh directory and returns
// the directory.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const ultraStarSong = `#TITLE:Bohemian Rhapsody
#ARTIST:Queen
#LANGUAGE:English
#GENRE:Rock
#YEAR:1975
#MP3:queen.mp3
#BPM:300
#GAP:1000
: 0 4 5 Is
: 4 4 7  this
- 10
: 12 4 7 real
E
`

// cdgBytes is a whole number of 24-byte CD+G packets.
var cdgBytes = string(make([]byte, 24*4))

func TestParseConformance(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"Queen - Bohemian Rhapsody/queen.mp3":                     "",
		"Queen - Bohemian Rhapsody/Queen - Bohemian Rhapsody.txt": ultraStarSong,
		"mp3g/SC8125-03 - Toto - Africa.mp3":                      "",
		"mp3g/SC8125-03 - Toto - Africa.cdg":                      cdgBytes,
		"lrc/Adele - Hello.mp3":                                   "",
		"lrc/Adele - Hello.lrc":                                   "[ar:Adele]\n[ti:Hello]\n[00:05.00]Hello, it's me\n",
	})
	sdktest.RunConformance(t, karaoke.New(), []sdktest.ParseVector{
		{
			Name:   "disc-prefixed MP3+G: disc track, title, CD+G class",
			Path:   filepath.Join(dir, "mp3g/SC8125-03 - Toto - Africa.mp3"),
			Probe:  sdk.ParseProbe{HasAudio: true, HasVideo: true},
			Want:   sdk.ParsedMedia{Title: "Africa", Class: karaoke.ClassCDG, Index: 3},
			WantOK: true,
		},
		{
			Name:   "UltraStar song folder: header names the audio, header wins",
			Path:   filepath.Join(dir, "Queen - Bohemian Rhapsody/queen.mp3"),
			Probe:  sdk.ParseProbe{HasAudio: true},
			Want:   sdk.ParsedMedia{Title: "Bohemian Rhapsody", Class: karaoke.ClassUltraStar, Year: 1975},
			WantOK: true,
		},
		{
			Name:   "LRC sidecar: lrc class",
			Path:   filepath.Join(dir, "lrc/Adele - Hello.mp3"),
			Probe:  sdk.ParseProbe{HasAudio: true},
			Want:   sdk.ParsedMedia{Title: "Hello", Class: karaoke.ClassLRC},
			WantOK: true,
		},
		{
			Name:   "karaoke video: markers stripped from the title",
			Path:   "/lib/karaoke/Whitney Houston - I Will Always Love You (Karaoke Version).mp4",
			Probe:  sdk.ParseProbe{HasAudio: true, HasVideo: true},
			Want:   sdk.ParsedMedia{Title: "I Will Always Love You", Class: karaoke.ClassVideo},
			WantOK: true,
		},
		{
			Name:   "MP3 cover art is not a karaoke video",
			Path:   "/lib/karaoke/ABBA - Waterloo.mp3",
			Probe:  sdk.ParseProbe{HasAudio: true, HasVideo: true},
			Want:   sdk.ParsedMedia{Title: "Waterloo", Class: karaoke.ClassBacking},
			WantOK: true,
		},
		{
			Name:   "disc folder: leading track number inside it",
			Path:   "/lib/karaoke/SF001/07 - Oasis - Wonderwall.mp3",
			Want:   sdk.ParsedMedia{Title: "Wonderwall", Class: karaoke.ClassBacking, Index: 7},
			WantOK: true,
		},
		{
			Name: "tags win over the file name; year from date tag",
			Path: "/lib/karaoke/track01.mp3",
			Probe: sdk.ParseProbe{Tags: map[string]string{
				"title": "Dancing Queen (Karaoke)", "artist": "ABBA", "date": "1976-08-16",
				"album": "SC 8001", "track": "4/15",
			}},
			Want:   sdk.ParsedMedia{Title: "Dancing Queen", Class: karaoke.ClassBacking, Year: 1976, Index: 4},
			WantOK: true,
		},
		{
			Name:   "a band name with digits is not a disc ID",
			Path:   "/lib/karaoke/UB40 - Red Red Wine.mp3",
			Want:   sdk.ParsedMedia{Title: "Red Red Wine", Class: karaoke.ClassBacking},
			WantOK: true,
		},
		{
			Name:   "the CD+G sidecar itself is not a song",
			Path:   filepath.Join(dir, "mp3g/SC8125-03 - Toto - Africa.cdg"),
			WantOK: false,
		},
		{
			Name:   "an LRC sidecar is not a song",
			Path:   "/lib/karaoke/Adele - Hello.LRC",
			WantOK: false,
		},
		{
			Name:   "no title derivable",
			Path:   "/lib/karaoke/[Karaoke].mp3",
			WantOK: false,
		},
	})
}

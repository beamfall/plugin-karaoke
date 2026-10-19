//go:build unix

package karaoke_test

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	sdk "github.com/beamfall/plugin-sdk"

	karaoke "github.com/beamfall/plugin-karaoke"
)

// A FIFO beside a song must be skipped, never opened: opening one for reading
// blocks until a writer appears, which would hang the scan.
func TestSidecarFIFOIsSkipped(t *testing.T) {
	dir := writeFiles(t, map[string]string{"song.mp3": ""})
	for _, name := range []string{"song.lrc", "song.txt", "other.txt"} {
		if err := syscall.Mkfifo(filepath.Join(dir, name), 0o644); err != nil {
			t.Skipf("mkfifo: %v", err)
		}
	}
	done := make(chan sdk.ParsedMedia, 1)
	go func() {
		parsed, _ := karaoke.New().Parse(filepath.Join(dir, "song.mp3"), sdk.ParseProbe{})
		done <- parsed
	}()
	select {
	case parsed := <-done:
		if parsed.Class != karaoke.ClassBacking {
			t.Fatalf("class = %q", parsed.Class)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Parse hung on a FIFO sidecar")
	}
}

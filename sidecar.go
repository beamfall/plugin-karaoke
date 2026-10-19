package karaoke

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// maxSidecarBytes bounds every sidecar read: a real LRC or UltraStar file
	// is a few KiB, so anything past 1 MiB is not one.
	maxSidecarBytes = 1 << 20
	// maxFolderTxtBytes bounds each .txt the folder scan reads, and
	// maxFolderScanBytes the scan's total: an UltraStar file is tens of KiB,
	// and the scan runs for every file of a folder on every hook call.
	maxFolderTxtBytes  = 256 << 10
	maxFolderScanBytes = 2 << 20
	// maxFolderEntries bounds the UltraStar folder scan. An UltraStar song
	// folder holds a handful of files; a large flat folder is not one, and is
	// not scanned per file.
	maxFolderEntries = 64
	// cdgPacketBytes is the CD+G subcode packet size: a .cdg is a whole
	// number of 24-byte packets.
	cdgPacketBytes = 24
)

// Sidecar extensions, lowercase without the dot. These are the values the
// bundle declares as InteractiveSidecars.
const (
	extCDG = "cdg"
	extLRC = "lrc"
	extTXT = "txt"
)

// sidecars is what sits beside one media file.
type sidecars struct {
	cdg bool
	// cdgPath is the .cdg sidecar's path when cdg is true.
	cdgPath   string
	lrc       *LRC
	ultraStar *UltraStar
	// ultraStarVideo reports that the file is the #VIDEO of an UltraStar
	// song whose #AUDIO is another file: a part of that song, not a song.
	ultraStarVideo bool
}

// findSidecars looks beside path for the karaoke companions of the same stem:
// a .cdg (MP3+G), a .lrc, and an UltraStar .txt. An UltraStar .txt is also
// recognized under another name in a small song folder when its #AUDIO/#MP3
// header names this file. Unreadable or invalid sidecars are absent, never an
// error: a parse hook must not fail a scan.
func findSidecars(path string) sidecars {
	var found sidecars
	stem := strings.TrimSuffix(path, filepath.Ext(path))
	if name, ok := siblingPath(stem, extCDG); ok {
		if info, err := os.Stat(name); err == nil && info.Size() > 0 && info.Size()%cdgPacketBytes == 0 {
			found.cdg, found.cdgPath = true, name
		}
	}
	if name, ok := siblingPath(stem, extLRC); ok {
		if data, ok := readBounded(name, maxSidecarBytes); ok {
			if lrc, err := ParseLRC(data); err == nil {
				found.lrc = &lrc
			}
		}
	}
	base := filepath.Base(path)
	if name, ok := siblingPath(stem, extTXT); ok {
		if data, ok := readBounded(name, maxSidecarBytes); ok {
			if song, err := ParseUltraStar(data); err == nil {
				found.claim(&song, base, true)
			}
		}
	}
	if found.ultraStar == nil && !found.ultraStarVideo {
		ultraStarNaming(path, &found)
	}
	return found
}

// claim records song against the file named base: as its UltraStar song when
// the song's audio is this file (or, for a same-stem .txt, when the song names
// no audio), or as the song's video when #VIDEO names this file and #AUDIO
// another. It reports whether the song named the file at all.
func (found *sidecars) claim(song *UltraStar, base string, sameStem bool) bool {
	switch {
	case sameName(song.Audio, base), sameStem && song.Audio == "":
		found.ultraStar = song
		return true
	case song.Video != "" && sameName(song.Video, base):
		found.ultraStarVideo = true
		return true
	}
	return false
}

func sameName(header, base string) bool {
	return strings.EqualFold(filepath.Base(filepath.FromSlash(header)), base)
}

// ultraStarNaming scans a small folder, in name order, for an UltraStar .txt
// whose #AUDIO or #VIDEO names path's file, and claims the first one found.
func ultraStarNaming(path string, found *sidecars) {
	dir, base := filepath.Dir(path), filepath.Base(path)
	folder, err := os.Open(dir)
	if err != nil {
		return
	}
	defer func() { _ = folder.Close() }()
	entries, err := folder.ReadDir(maxFolderEntries + 1)
	if err != nil && err != io.EOF || len(entries) > maxFolderEntries {
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	budget := maxFolderScanBytes
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.EqualFold(strings.TrimPrefix(filepath.Ext(entry.Name()), "."), extTXT) {
			continue
		}
		data, ok := readBounded(filepath.Join(dir, entry.Name()), min(maxFolderTxtBytes, budget))
		budget -= len(data)
		if !ok {
			continue
		}
		if song, err := ParseUltraStar(data); err == nil && found.claim(&song, base, false) {
			return
		}
		if budget <= 0 {
			return
		}
	}
}

// siblingPath returns stem+"."+ext in lower or upper case, whichever exists as
// a regular file.
func siblingPath(stem, ext string) (string, bool) {
	for _, candidate := range []string{stem + "." + ext, stem + "." + strings.ToUpper(ext)} {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate, true
		}
	}
	return "", false
}

// readBounded reads a whole regular file of at most limit bytes. Anything
// else (a FIFO, a device) is refused before it is opened: opening a FIFO for
// reading blocks until a writer appears.
func readBounded(name string, limit int) ([]byte, bool) {
	if info, err := os.Stat(name); err != nil || !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return nil, false
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(data) > limit {
		return nil, false
	}
	return data, true
}

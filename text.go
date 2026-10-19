package karaoke

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// decodeText turns a sidecar's bytes into UTF-8 text. Karaoke sidecars are
// often old and not Unicode, so it honors a byte-order mark (UTF-8 or
// UTF-16), keeps valid UTF-8 as is, and otherwise reads the bytes in the
// declared legacy code page, defaulting to Windows-1252 — the code page the
// bulk of legacy LRC and UltraStar files were written in. Double-byte East
// Asian legacy encodings (Shift-JIS, GBK, Big5) are not detected; such a file
// decodes as Windows-1252 mojibake rather than failing.
func decodeText(data []byte, declared string) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return strings.ToValidUTF8(string(data[3:]), "�")
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}), bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		decoded, err := unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder().Bytes(data)
		if err == nil {
			return string(decoded)
		}
	}
	if utf8.Valid(data) {
		return string(data)
	}
	decoded, err := legacyEncoding(declared).NewDecoder().Bytes(data)
	if err != nil {
		return strings.ToValidUTF8(string(data), "�")
	}
	return string(decoded)
}

// legacyEncoding maps an UltraStar #ENCODING value to a decoder. Unknown or
// absent declarations fall back to Windows-1252.
func legacyEncoding(declared string) encoding.Encoding {
	switch strings.ToUpper(strings.NewReplacer("-", "", " ", "", "_", "").Replace(declared)) {
	case "CP1250", "WINDOWS1250":
		return charmap.Windows1250
	case "CP1251", "WINDOWS1251":
		return charmap.Windows1251
	case "LATIN1", "ISO88591":
		return charmap.ISO8859_1
	default:
		return charmap.Windows1252
	}
}

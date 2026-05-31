package finger

import (
	"strings"
	"unicode/utf8"
)

// sanitize makes a finger response body safe to print to a terminal, per
// RFC 1288 §3.3 ("filter any unprintable data"). It visualizes rather than
// deletes control data, so no information is lost:
//
//   - tab, newline, and every printable rune (including all valid multibyte
//     UTF-8) are kept verbatim;
//   - C0 controls (U+0000–U+001F except tab/newline) and DEL (U+007F) become
//     caret notation (ESC -> "^[", BEL -> "^G", DEL -> "^?");
//   - C1 controls (U+0080–U+009F, even when validly UTF-8-encoded) and any
//     invalid UTF-8 byte become lowercase "\xXX" hex.
//
// We deliberately keep UTF-8 rather than honoring §3.3's literal "7-bit"
// wording: stripping bytes >= 0x80 would delete legitimate modern content,
// while the genuine terminal-control threat is the control ranges above.
func sanitize(body []byte) []byte {
	// Fast path: if there is nothing to defang, return the input unchanged.
	if isClean(body) {
		return body
	}
	var b strings.Builder
	b.Grow(len(body))
	for i := 0; i < len(body); {
		r, size := utf8.DecodeRune(body[i:])
		if r == utf8.RuneError && size == 1 {
			// Invalid UTF-8 byte.
			writeHex(&b, body[i])
			i++
			continue
		}
		switch {
		case r == '\t' || r == '\n':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			// C0 control (except tab/newline already handled) or DEL.
			writeCaret(&b, r)
		case r >= 0x80 && r <= 0x9f:
			// C1 control, however it was encoded.
			writeHex(&b, byte(r))
		default:
			b.WriteRune(r)
		}
		i += size
	}
	return []byte(b.String())
}

// isClean reports whether body contains only bytes sanitize would keep
// verbatim, allowing the common case to skip allocation. It is conservative:
// any byte that could need defanging (control, DEL, or >= 0x80) forces the
// slow path, where DecodeRune does the precise classification.
func isClean(body []byte) bool {
	for _, c := range body {
		if c == '\t' || c == '\n' {
			continue
		}
		if c < 0x20 || c >= 0x7f {
			return false
		}
	}
	return true
}

// writeCaret writes a C0 control or DEL in caret notation: the control's
// printable counterpart is the code point XOR 0x40 (NUL -> '@', US -> '_',
// DEL -> '?').
func writeCaret(b *strings.Builder, r rune) {
	b.WriteByte('^')
	b.WriteByte(byte(r) ^ 0x40)
}

// writeHex writes a single byte as lowercase `\xXX`.
func writeHex(b *strings.Builder, c byte) {
	const hex = "0123456789abcdef"
	b.WriteByte('\\')
	b.WriteByte('x')
	b.WriteByte(hex[c>>4])
	b.WriteByte(hex[c&0xf])
}

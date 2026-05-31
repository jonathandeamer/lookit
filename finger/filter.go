package finger

import "unicode/utf8"

// filterControlChars strips non-printable control characters per RFC 1288 §3.3,
// while preserving valid UTF-8 multibyte sequences. Allowed: printable ASCII
// (0x20–0x7E), CR, LF, HT, and valid UTF-8 ≥ 0x80. Everything else is removed.
func filterControlChars(body []byte) []byte {
	out := make([]byte, 0, len(body))
	for i := 0; i < len(body); {
		b := body[i]
		if b < utf8.RuneSelf { // ASCII range
			if b == '\t' || b == '\n' || b == '\r' || (b >= 0x20 && b <= 0x7e) {
				out = append(out, b)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(body[i:])
		if r == utf8.RuneError && size == 1 {
			i++ // invalid byte, skip
			continue
		}
		out = append(out, body[i:i+size]...)
		i += size
	}
	return out
}

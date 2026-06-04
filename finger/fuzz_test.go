package finger

import (
	"testing"
	"unicode"
	"unicode/utf8"
)

// FuzzSanitize asserts sanitize's core security property: whatever bytes a
// hostile finger server sends, the output is valid UTF-8 and carries no raw
// terminal-control data — C0 controls (except tab/newline), DEL, C1 controls,
// or non-printing Unicode format controls (Cf/Zl/Zp). A failure here is a
// sanitize bypass: a byte that would reach the terminal unfiltered.
func FuzzSanitize(f *testing.F) {
	f.Add([]byte("Plan: just a normal plan\n"))
	f.Add([]byte("\x1b[31mred\x1b[0m and \x07 bell"))
	f.Add([]byte("\x00\x7f mixed \xc2\x85 NEL"))
	f.Add([]byte("café résumé naïve 日本語")) // legitimate multibyte UTF-8
	f.Add([]byte("\xe2\x80\xaboverride"))  // U+202B RTL embedding (Cf)
	f.Add([]byte{0xff, 0xfe, 0x00})        // invalid UTF-8 + NUL

	f.Fuzz(func(t *testing.T, body []byte) {
		out := string(sanitize(body))
		if !utf8.ValidString(out) {
			t.Fatalf("sanitize produced invalid UTF-8 for %q", body)
		}
		for _, r := range out {
			switch {
			case r == '\t' || r == '\n':
				// kept verbatim
			case r < 0x20 || r == 0x7f:
				t.Fatalf("control rune %U survived sanitize of %q", r, body)
			case r >= 0x80 && r <= 0x9f:
				t.Fatalf("C1 control %U survived sanitize of %q", r, body)
			case unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp):
				t.Fatalf("format control %U survived sanitize of %q", r, body)
			}
		}
	})
}

// FuzzParseTarget asserts ParseTarget never panics and that any target it
// accepts is free of control characters in the user/host token — the egress
// guard that stops a hostile argument from smuggling extra RFC 1288 query
// lines onto the wire.
func FuzzParseTarget(f *testing.F) {
	f.Add("alice@plan.cat")
	f.Add("@tilde.team")
	f.Add("user@host:79")
	f.Add("ring@thebackupbox.net")
	f.Add("")
	f.Add("bad\x00@host")

	f.Fuzz(func(t *testing.T, arg string) {
		target, err := ParseTarget(arg)
		if err != nil {
			return // rejected inputs are fine
		}
		if hasControl(target.User) || hasControl(target.HostPort) {
			t.Fatalf("ParseTarget accepted %q but target carries control bytes: %+v", arg, target)
		}
	})
}

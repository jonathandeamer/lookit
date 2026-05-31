package finger

import "testing"

func TestFilterControlChars(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"printable ASCII unchanged", "hello world", "hello world"},
		{"LF preserved", "line1\nline2", "line1\nline2"},
		{"HT preserved", "col1\tcol2", "col1\tcol2"},
		{"ESC stripped, rest kept", "a\x1b[31mred\x1b[0m", "a[31mred[0m"},
		{"BEL stripped", "bel\x07here", "belhere"},
		{"NUL stripped", "null\x00byte", "nullbyte"},
		{"DEL stripped", "del\x7fhere", "delhere"},
		{"valid UTF-8 preserved", "café", "café"},
		{"box-drawing preserved", "box┌─┐", "box┌─┐"},
		{"4-byte UTF-8 preserved", "emoji😀", "emoji😀"},
		{"lone C1 bytes stripped", "\x80\x81lone", "lone"},
		{"invalid UTF-8: strip bad byte, keep valid", "bad\xc3\x28seq", "bad(seq"},
		{"UTF-8 surrogate-half rejected", "\xed\xa0\x80", ""},
		{"truncated multibyte at EOF stripped", "lone\xc3", "lone"},
		{"empty input, no panic", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(filterControlChars([]byte(tt.in)))
			if got != tt.want {
				t.Errorf("filterControlChars(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

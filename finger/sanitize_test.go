package finger

import "testing"

func TestSanitize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"clean unchanged", "Hello, world!\n", "Hello, world!\n"},
		{"tab and newline preserved", "a\tb\nc", "a\tb\nc"},
		{"esc to caret", "\x1b[31m", "^[[31m"},
		{"bel to caret", "ding\x07", "ding^G"},
		{"nul to caret", "a\x00b", "a^@b"},
		{"unit separator to caret", "a\x1fb", "a^_b"},
		{"del to caret", "a\x7fb", "a^?b"},
		{"stray cr to caret", "a\rb", "a^Mb"},
		{"valid multibyte utf8 preserved", "café ünïcödé", "café ünïcödé"},
		{"raw high c1 byte", "a\x9bb", "a\\x9bb"},
		{"utf8 encoded c1", "ab", "a\\x85b"},
		{"invalid utf8 byte", "a\xffb", "a\\xffb"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(sanitize([]byte(tt.in)))
			if got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

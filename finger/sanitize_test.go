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
		{"valid multibyte utf8 preserved", "café ünïcodé", "café ünïcodé"},
		{"raw high c1 byte", "a\x9bb", "a\\x9bb"},
		{"utf8 encoded c1", "ab", "a\\x85b"},
		{"invalid utf8 byte", "a\xffb", "a\\xffb"},
		{"empty", "", ""},
		{"rlo override to unicode escape", "a‮b", "a\\u{202e}b"},
		{"zero-width space to unicode escape", "a​b", "a\\u{200b}b"},
		{"bom to unicode escape", "\ufeffhi", "\\u{feff}hi"},
		{"zwj between emoji escaped", "\U0001f468‍\U0001f469", "\U0001f468\\u{200d}\U0001f469"},
		{"line separator to unicode escape", "a b", "a\\u{2028}b"},
		{"soft hyphen to unicode escape", "a­b", "a\\u{ad}b"},
		{"plain emoji preserved", "hi \U0001f600", "hi \U0001f600"},
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

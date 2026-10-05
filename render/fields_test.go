package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestHighlightFieldsSentenceMarkersAreWholeLine(t *testing.T) {
	theme := NewThemeWithBackground(colorprofile.TrueColor, true)
	body := []byte("No mail.\nNo mail. (on leave)\nNever logged in.\nNever logged in. last week\nNo Plan.\nNo Plan. yet\nLogin: alice\n")
	got := highlightFields(theme, body, nil)

	// Strip ANSI to assert structure; count Field colour on genuine markers only.
	plain := stripANSIForTest(got)
	wantPlain := "No mail.\nNo mail. (on leave)\nNever logged in.\nNever logged in. last week\nNo Plan.\nNo Plan. yet\nLogin: alice\n"
	if plain != wantPlain {
		t.Fatalf("plain text changed:\n got %q\nwant %q", plain, wantPlain)
	}

	fieldColour := "\x1b[38;2;255;95;162m"
	// Genuine whole-line markers + Login: = 4 styled prefixes.
	if got := strings.Count(got, fieldColour); got != 4 {
		t.Fatalf("field colour appears %d times, want 4 (3 sentences + Login:): %q", got, got)
	}
	for _, prose := range []string{"No mail. (on leave)", "Never logged in. last week", "No Plan. yet"} {
		// Prose line must not start with the field colour wrapping only the marker.
		idx := strings.Index(got, prose)
		if idx < 0 {
			t.Fatalf("missing prose line %q in %q", prose, got)
		}
		// The characters immediately before the prose should not be a render of just the marker.
		if strings.Contains(got, fieldColour+strings.Split(prose, " ")[0]) && strings.HasPrefix(prose, "No mail.") {
			// Check specifically: "No mail." coloured then " (on leave)" uncoloured would mean prefix match.
			if strings.Contains(got, fieldColour+"No mail."+"\x1b[0m"+" (on leave)") ||
				strings.Contains(got, fieldColour+"No mail."+"\x1b[39m"+" (on leave)") {
				t.Fatalf("prose line incorrectly treated as prefix: %q", got)
			}
		}
	}
	// Stronger check: after colouring, the prose lines appear with no escape before the marker text at line start.
	for _, line := range strings.Split(got, "\n") {
		stripped := stripANSIForTest(line)
		if stripped == "No mail. (on leave)" || stripped == "Never logged in. last week" || stripped == "No Plan. yet" {
			if strings.Contains(line, "\x1b[") {
				t.Fatalf("prose line should be unstyled, got %q", line)
			}
		}
		if stripped == "No mail." || stripped == "Never logged in." || stripped == "No Plan." {
			if !strings.Contains(line, fieldColour) {
				t.Fatalf("status sentence should be styled, got %q", line)
			}
		}
	}
}

func stripANSIForTest(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEsc = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

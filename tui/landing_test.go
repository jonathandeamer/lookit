package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// foregroundSequences returns the set of distinct truecolor foreground SGR
// payloads (e.g. "38;2;255;95;162") present in s. Sequences may have other
// attributes (e.g. "1;38;2;...") — we extract just the 38;2;R;G;B part.
func foregroundSequences(s string) map[string]bool {
	out := map[string]bool{}
	i := 0
	for i < len(s) {
		// Find the next escape sequence
		idx := strings.Index(s[i:], "\x1b[")
		if idx == -1 {
			break
		}
		i += idx + 2 // skip to after "\x1b["
		// Find the end of the sequence (the 'm')
		if j := strings.IndexByte(s[i:], 'm'); j >= 0 {
			seq := s[i : i+j]
			// Extract 38;2;R;G;B from sequences that may include other attributes
			if start := strings.Index(seq, "38;2;"); start >= 0 {
				// Skip "38;2;" (5 chars) and scan R;G;B
				pos := start + 5
				for component := 0; component < 3 && pos < len(seq); component++ {
					// Skip the digits of this component
					for pos < len(seq) && seq[pos] >= '0' && seq[pos] <= '9' {
						pos++
					}
					// If not the last component, expect a semicolon
					if component < 2 && pos < len(seq) && seq[pos] == ';' {
						pos++
					}
				}
				// pos now points just after B; extract "38;2;R;G;B"
				out[seq[start:pos]] = true
			}
			i += j + 1
		} else {
			break
		}
	}
	return out
}

func stripANSIForLandingTest(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestHeaderMarkRendersFingerAndWordmark(t *testing.T) {
	st := newStyles(true)
	out := headerMark(st, colorprofile.TrueColor)
	plain := stripANSIForLandingTest(out)
	if plain != heroManicule+" "+heroWordmark {
		t.Fatalf("header mark = %q, want %q", plain, heroManicule+" "+heroWordmark)
	}
	if got := len(foregroundSequences(out)); got < 3 {
		t.Fatalf("expected per-rune colour sweep, got %d distinct colours:\n%q", got, out)
	}
}

func TestHeaderMarkAnsiFallsBackToSolid(t *testing.T) {
	st := newStyles(true)
	out := headerMark(st, colorprofile.ANSI)
	if plain := stripANSIForLandingTest(out); plain != heroManicule+" "+heroWordmark {
		t.Fatalf("header mark = %q, want %q", plain, heroManicule+" "+heroWordmark)
	}
	if got := len(foregroundSequences(out)); got > 2 {
		t.Fatalf("ANSI should not use the truecolor sweep, got %d distinct colours:\n%q", got, out)
	}
}

func TestHeaderMarkANSI256VariesPerRune(t *testing.T) {
	st := newStyles(true)
	out := headerMark(st, colorprofile.ANSI256)
	if !strings.Contains(out, heroManicule) {
		t.Fatalf("missing manicule:\n%q", out)
	}
	// ANSI256 takes the same per-rune gradient path as TrueColor.
	if got := len(foregroundSequences(out)); got < 3 {
		t.Fatalf("expected a per-rune sweep on ANSI256, got %d distinct colours:\n%q", got, out)
	}
}

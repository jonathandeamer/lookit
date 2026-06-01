package tui

import (
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestWordmarkColorsSweepsPalette(t *testing.T) {
	p := paletteFor(true)
	colors := wordmarkColors(p, 6)
	if len(colors) != 6 {
		t.Fatalf("len = %d, want 6", len(colors))
	}
	assertSameColor(t, "first stop", colors[0], p.AccentPink)
	assertSameColor(t, "last stop", colors[5], p.AccentMint)
	if sameColor(colors[0], colors[5]) {
		t.Fatal("gradient endpoints should differ")
	}
	if sameColor(colors[1], colors[4]) {
		t.Fatal("interior gradient colours should differ")
	}
}

func TestWordmarkColorsSingleRune(t *testing.T) {
	p := paletteFor(true)
	colors := wordmarkColors(p, 1)
	if len(colors) != 1 {
		t.Fatalf("len = %d, want 1", len(colors))
	}
	assertSameColor(t, "single", colors[0], p.AccentPink)
}

func TestLerpColor(t *testing.T) {
	black := color.RGBA{R: 0, G: 0, B: 0, A: 0xff}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 0xff}
	cases := []struct {
		name  string
		t     float64
		wantR uint8
	}{
		{"t=0 is a", 0, 0},
		{"t=1 is b", 1, 255},
		{"midpoint rounds up", 0.5, 128},
		{"clamp below 0", -1, 0},
		{"clamp above 1", 2, 255},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lerpColor(black, white, tc.t)
			r, _, _, _ := got.RGBA()
			if uint8(r>>8) != tc.wantR {
				t.Fatalf("R = %d, want %d", uint8(r>>8), tc.wantR)
			}
		})
	}
}

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

func TestGradientWordmarkTrueColorVariesPerRune(t *testing.T) {
	st := newStyles(true)
	out := gradientWordmark(st, colorprofile.TrueColor)
	if !strings.Contains(out, heroManicule) {
		t.Fatalf("missing manicule:\n%q", out)
	}
	if got := len(foregroundSequences(out)); got < 3 {
		t.Fatalf("expected a per-rune sweep, got %d distinct colours:\n%q", got, out)
	}
}

func TestGradientWordmarkAnsiFallsBackToSolid(t *testing.T) {
	st := newStyles(true)
	out := gradientWordmark(st, colorprofile.ANSI)
	if !strings.Contains(out, heroManicule) {
		t.Fatalf("missing manicule:\n%q", out)
	}
	if got := len(foregroundSequences(out)); got > 2 {
		t.Fatalf("ANSI should be solid, got %d distinct colours:\n%q", got, out)
	}
}

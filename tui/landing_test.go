package tui

import (
	"image/color"
	"testing"
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

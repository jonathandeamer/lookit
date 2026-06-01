package tui

import (
	"image/color"
	"math"
)

// lerpColor linearly interpolates between two colours in 8-bit RGB. t is
// clamped to [0,1].
func lerpColor(a, b color.Color, t float64) color.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	return color.RGBA{
		R: uint8(math.Round(float64(ar>>8)*(1-t) + float64(br>>8)*t)),
		G: uint8(math.Round(float64(ag>>8)*(1-t) + float64(bg>>8)*t)),
		B: uint8(math.Round(float64(ab>>8)*(1-t) + float64(bb>>8)*t)),
		A: 0xff,
	}
}

// wordmarkColors returns n colours sweeping AccentPink -> AccentViolet ->
// AccentMint across the palette. The endpoints are the exact palette stops;
// interior positions are interpolated. n <= 1 returns the first stop only.
func wordmarkColors(p palette, n int) []color.Color {
	stops := []color.Color{p.AccentPink, p.AccentViolet, p.AccentMint}
	if n <= 1 {
		return []color.Color{stops[0]}
	}
	out := make([]color.Color, n)
	for i := 0; i < n; i++ {
		switch i {
		case 0:
			out[i] = stops[0]
		case n - 1:
			out[i] = stops[len(stops)-1]
		default:
			seg := float64(i) / float64(n-1) * float64(len(stops)-1)
			lo := int(seg)
			out[i] = lerpColor(stops[lo], stops[lo+1], seg-float64(lo))
		}
	}
	return out
}

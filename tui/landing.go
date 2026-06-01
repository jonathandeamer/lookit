package tui

import (
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

const (
	heroManicule = "☞"
	heroWordmark = "lookit"
	heroTagline  = "a finger client for the modern terminal"
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

// gradientWordmark renders "☞ lookit": the manicule in AccentPink, and the
// wordmark with a per-rune pink->violet->mint gradient on truecolor/ANSI256.
// On ANSI (16-colour) and below the gradient muddies, so it falls back to a
// solid AccentViolet wordmark. The gradient is decorative; the wordmark is
// always legible.
func gradientWordmark(st styles, profile colorprofile.Profile) string {
	manicule := lipgloss.NewStyle().Foreground(st.palette.AccentPink).Render(heroManicule)
	if profile < colorprofile.ANSI256 {
		word := lipgloss.NewStyle().Foreground(st.palette.AccentViolet).Bold(true).Render(heroWordmark)
		return manicule + " " + word
	}
	runes := []rune(heroWordmark)
	colors := wordmarkColors(st.palette, len(runes))
	var b strings.Builder
	for i, r := range runes {
		b.WriteString(lipgloss.NewStyle().Foreground(colors[i]).Bold(true).Render(string(r)))
	}
	return manicule + " " + b.String()
}

// heroInputWidth bounds the centred landing input so it reads as a tidy box
// rather than a full-width bar: ~40 columns, clamped to the terminal width
// (leaving a small margin) and never below 12.
func heroInputWidth(totalWidth int) int {
	w := 40
	if max := totalWidth - 4; w > max {
		w = max
	}
	if w < 12 {
		w = 12
	}
	return w
}

// heroView composes the centred landing hero — wordmark, tagline (hidden under
// 40 columns), a spacer, and the already-rendered input — and places it in the
// centre of a width x height box. It is the sole renderer of the input on the
// landing screen. Pure: string in, string out.
func heroView(st styles, profile colorprofile.Profile, width, height int, input string) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	parts := []string{gradientWordmark(st, profile)}
	// The tagline only fits comfortably beside/below the wordmark at 40+ columns.
	if width >= 40 {
		parts = append(parts, lipgloss.NewStyle().Foreground(st.palette.Dim).Render(heroTagline))
	}
	parts = append(parts, "", input) // blank spacer line before the input
	block := lipgloss.JoinVertical(lipgloss.Center, parts...)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, block)
}

package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

const (
	heroManicule = "☞"
	heroWordmark = "lookit"
	heroTagline  = "a finger client for the modern terminal"
)

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
	colors := gradientColors(st.palette, len(runes))
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

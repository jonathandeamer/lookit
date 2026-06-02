package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

const (
	heroManicule = "☞"
	heroWordmark = "lookit"
)

// headerMark renders "☞ lookit": the manicule in AccentPink, and the
// wordmark with a per-rune pink->violet->mint gradient on truecolor/ANSI256.
// On ANSI (16-colour) and below the gradient muddies, so it falls back to a
// solid AccentViolet wordmark. The gradient is decorative; the wordmark is
// always legible.
func headerMark(st styles, profile colorprofile.Profile) string {
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

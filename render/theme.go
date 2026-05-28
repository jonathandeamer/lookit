// Package render formats finger responses for terminal display.
package render

import (
	"image/color"
	"io"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Theme holds the styles used by Render. Build one per profile.
type Theme struct {
	Profile colorprofile.Profile

	Arrow   lipgloss.Style // "➜"
	Target  lipgloss.Style // "alice@plan.cat"
	Latency lipgloss.Style // "123ms"
	Sparkle lipgloss.Style // success indicator
	Footer  lipgloss.Style // bytes / elapsed line (dim)
	Warning lipgloss.Style // yellow notices like "truncated"
	Field   lipgloss.Style // "Login:" labels
	ErrLine lipgloss.Style // red error lines

	NoColor bool // true if the profile is Ascii or NoTTY
}

// pink, cyan, gold, red, dim — base palette (truecolor source values).
var (
	colPink = color.RGBA{0xff, 0x6f, 0xd5, 0xff}
	colCyan = color.RGBA{0x6b, 0xe1, 0xff, 0xff}
	colGold = color.RGBA{0xff, 0xd0, 0x6b, 0xff}
	colRed  = color.RGBA{0xff, 0x6b, 0x6b, 0xff}
	colDim  = color.RGBA{0x80, 0x80, 0x80, 0xff}
)

// NewTheme builds a Theme appropriate for the given profile. On Ascii/NoTTY
// profiles, returns a no-color theme that still preserves spacing.
func NewTheme(p colorprofile.Profile) Theme {
	noColor := p <= colorprofile.Ascii
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termProfile(p))

	style := func(c color.Color, bold bool) lipgloss.Style {
		if noColor {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().
			Renderer(renderer).
			Foreground(lipgloss.Color(toHex(p.Convert(c)))).
			Bold(bold)
	}
	return Theme{
		Profile: p,
		NoColor: noColor,
		Arrow:   style(colPink, true),
		Target:  style(colPink, true),
		Latency: style(colDim, false),
		Sparkle: style(colGold, false),
		Footer:  style(colDim, false),
		Warning: style(colGold, false),
		Field:   style(colCyan, true),
		ErrLine: style(colRed, false),
	}
}

func termProfile(p colorprofile.Profile) termenv.Profile {
	switch p {
	case colorprofile.TrueColor:
		return termenv.TrueColor
	case colorprofile.ANSI256:
		return termenv.ANSI256
	case colorprofile.ANSI:
		return termenv.ANSI
	default:
		return termenv.Ascii
	}
}

// toHex formats a color.Color as "#RRGGBB" for lipgloss.Color.
func toHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	const hex = "0123456789abcdef"
	out := []byte{'#', 0, 0, 0, 0, 0, 0}
	rb, gb, bb := byte(r>>8), byte(g>>8), byte(b>>8)
	out[1] = hex[rb>>4]
	out[2] = hex[rb&0xf]
	out[3] = hex[gb>>4]
	out[4] = hex[gb&0xf]
	out[5] = hex[bb>>4]
	out[6] = hex[bb&0xf]
	return string(out)
}

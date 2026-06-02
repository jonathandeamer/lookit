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
	Footer  lipgloss.Style // dim style for subdued notices and CLI help text
	Warning lipgloss.Style // yellow notices like "truncated"
	Field   lipgloss.Style // "Login:" labels
	ErrLine lipgloss.Style // red error lines

	NoColor bool // true if the profile is Ascii or NoTTY
}

type renderPalette struct {
	Text, Dim, AccentPink, AccentViolet color.Color
	AccentMint, AccentGold, AccentRed   color.Color
	BaseBg                              color.Color
}

func renderPaletteFor(darkBackground bool) renderPalette {
	if darkBackground {
		return renderPalette{
			Text:         hexColor("#f0edf5"),
			Dim:          hexColor("#8c8792"),
			AccentPink:   hexColor("#ff5fa2"),
			AccentViolet: hexColor("#9878ff"),
			AccentMint:   hexColor("#38e7ad"),
			AccentGold:   hexColor("#eed76d"),
			AccentRed:    hexColor("#ff6f87"),
			BaseBg:       hexColor("#171719"),
		}
	}
	return renderPalette{
		Text:         hexColor("#25222a"),
		Dim:          hexColor("#766f7d"),
		AccentPink:   hexColor("#c92870"),
		AccentViolet: hexColor("#6d43d6"),
		AccentMint:   hexColor("#007f62"),
		AccentGold:   hexColor("#765f00"),
		AccentRed:    hexColor("#c82f4d"),
		BaseBg:       hexColor("#fbfafc"),
	}
}

func hexColor(s string) color.RGBA {
	if len(s) != 7 || s[0] != '#' {
		panic("invalid colour literal: " + s)
	}
	return color.RGBA{
		R: fromHexByte(s[1], s[2]),
		G: fromHexByte(s[3], s[4]),
		B: fromHexByte(s[5], s[6]),
		A: 0xff,
	}
}

func fromHexByte(hi, lo byte) byte {
	return fromHexNibble(hi)<<4 | fromHexNibble(lo)
}

func fromHexNibble(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	default:
		panic("invalid colour literal")
	}
}

// NewTheme builds a Theme for the given profile using Lip Gloss v1's detected
// terminal background.
func NewTheme(p colorprofile.Profile) Theme {
	return NewThemeWithBackground(p, lipgloss.HasDarkBackground())
}

// NewThemeWithBackground builds a Theme for the given profile and terminal
// background. On Ascii/NoTTY profiles, it returns a no-color theme that still
// preserves spacing.
func NewThemeWithBackground(p colorprofile.Profile, darkBackground bool) Theme {
	noColor := p <= colorprofile.Ascii
	pal := renderPaletteFor(darkBackground)
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termProfile(p))
	renderer.SetHasDarkBackground(darkBackground)

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
		Arrow:   style(pal.AccentViolet, true),
		Target:  style(pal.AccentPink, true),
		Latency: style(pal.Dim, false),
		Sparkle: style(pal.AccentGold, false),
		Footer:  style(pal.Dim, false),
		Warning: style(pal.AccentGold, false),
		Field:   style(pal.AccentPink, false),
		ErrLine: style(pal.AccentRed, false),
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

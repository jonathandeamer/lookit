package render

import (
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
	"github.com/jonathandeamer/lookit/finger"
)

// Render formats a finger query result for the requested terminal color
// profile, using Lip Gloss v1's standalone background detection.
func Render(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile) string {
	return RenderWithBackground(t, body, meta, queryErr, profile, lipgloss.HasDarkBackground())
}

// RenderWithBackground formats a finger query result for a known terminal
// background mode. The TUI uses this so tea.BackgroundColorMsg can restyle a
// live session deterministically.
func RenderWithBackground(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile, darkBackground bool) string {
	theme := NewThemeWithBackground(profile, darkBackground)
	var sb strings.Builder

	success := queryErr == nil
	sb.WriteString(renderHeader(theme, t, meta, success))

	if len(body) == 0 && success {
		sb.WriteString(theme.Footer.Render("(no response body)"))
		sb.WriteByte('\n')
	} else {
		if isTildeTeam(t) {
			body = reflowPronouns(body)
		}
		sb.WriteString(highlightFields(theme, body, extraFieldPrefixes(t)))
		if len(body) > 0 && body[len(body)-1] != '\n' {
			sb.WriteByte('\n')
		}
	}

	notice := ""
	if meta.Truncated {
		notice = "truncated"
	}
	sb.WriteString(renderFooter(theme, meta, notice))

	if queryErr != nil {
		sb.WriteString(theme.ErrLine.Render(queryErr.Error()))
		sb.WriteByte('\n')
	}

	return sb.String()
}

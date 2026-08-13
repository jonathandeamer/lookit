package render

import (
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/jonathandeamer/lookit/finger"
)

// Render formats a finger query result for the requested terminal color
// profile, using Lip Gloss v1's standalone background detection.
func Render(t finger.Target, body []byte, queryErr error, profile colorprofile.Profile) string {
	return RenderWithBackground(t, body, queryErr, profile, lipgloss.HasDarkBackground())
}

// RenderWithBackground formats a finger query result for a known terminal
// background mode. The TUI uses this so tea.BackgroundColorMsg can restyle a
// live session deterministically. It adds no receipt or metadata chrome; the
// TUI owns byte count, elapsed time, and truncation in its status bar.
func RenderWithBackground(t finger.Target, body []byte, queryErr error, profile colorprofile.Profile, darkBackground bool) string {
	return RenderWithWidth(t, body, queryErr, profile, darkBackground, 0)
}

// RenderWithWidth renders like RenderWithBackground but wraps the error line at
// width cells so a long dial failure stays readable in a narrow terminal. Only
// the error line — which lookit and net generate — is wrapped; the response body
// is never reflowed, so ASCII art and column layouts keep their exact bytes. A
// width of 0 or less means no wrapping.
func RenderWithWidth(t finger.Target, body []byte, queryErr error, profile colorprofile.Profile, darkBackground bool, width int) string {
	theme := NewThemeWithBackground(profile, darkBackground)
	var sb strings.Builder

	if len(body) == 0 && queryErr == nil {
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

	if queryErr != nil {
		text := queryErr.Error()
		if width > 0 {
			// ansi.Wrap prefers word boundaries and breaks mid-word only when a
			// token is longer than the line, so no error text is ever clipped.
			text = ansi.Wrap(text, width, "")
		}
		for i, line := range strings.Split(text, "\n") {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(theme.ErrLine.Render(line))
		}
		sb.WriteByte('\n')
	}

	return sb.String()
}

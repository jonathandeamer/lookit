package render

import (
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
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
	return RenderLayout(t, body, queryErr, profile, darkBackground, LayoutOptions{
		ErrorWidth: width,
	}).Text
}

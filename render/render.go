package render

import (
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
	"github.com/jonathandeamer/lookit/finger"
)

// Option tunes RenderWithBackground. Defaults match the one-shot CLI, which
// owns the full chrome; the TUI opts out of the parts its own bars duplicate.
type Option func(*options)

type options struct {
	footer bool
}

// WithoutFooter suppresses the trailing "bytes · elapsed" stats line. The TUI
// pins the page size in its status bar and the elapsed time in the header, so
// the footer would only duplicate facts already on screen.
func WithoutFooter() Option {
	return func(o *options) { o.footer = false }
}

// Render formats a finger query result for the requested terminal color
// profile, using Lip Gloss v1's standalone background detection.
func Render(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile) string {
	return RenderWithBackground(t, body, meta, queryErr, profile, lipgloss.HasDarkBackground())
}

// RenderWithBackground formats a finger query result for a known terminal
// background mode. The TUI uses this so tea.BackgroundColorMsg can restyle a
// live session deterministically.
func RenderWithBackground(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile, darkBackground bool, opts ...Option) string {
	o := options{footer: true}
	for _, opt := range opts {
		opt(&o)
	}

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

	if o.footer {
		notice := ""
		if meta.Truncated {
			notice = "truncated"
		}
		sb.WriteString(renderFooter(theme, meta, notice))
	}

	if queryErr != nil {
		sb.WriteString(theme.ErrLine.Render(queryErr.Error()))
		sb.WriteByte('\n')
	}

	return sb.String()
}

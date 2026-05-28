package render

import (
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

// Render formats a finger query result for the requested terminal color profile.
func Render(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile) string {
	theme := NewTheme(profile)
	var sb strings.Builder

	success := queryErr == nil
	sb.WriteString(renderHeader(theme, t, meta, success))

	if len(body) == 0 && success {
		sb.WriteString(theme.Footer.Render("(no response body)"))
		sb.WriteByte('\n')
	} else {
		sb.WriteString(highlightFields(theme, body))
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

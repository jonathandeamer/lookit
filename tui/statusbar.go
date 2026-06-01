package tui

import (
	"fmt"
	"net"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jonathandeamer/lookit/finger"
)

// statusBar is a pure description of the one-line bottom chrome. It holds no
// Bubble Tea state; appModel.View builds and renders it each frame. The
// breadcrumb's shape — "@host" alone vs "@host / user" — is the honest signal
// of directory-vs-profile, derived from the real target (no asserted "kind").
type statusBar struct {
	host      string   // "@tilde.team" ("" only on the landing screen)
	user      string   // "jonathan" ("" for a host directory)
	escTarget string   // previous history node's target.Raw ("" at the root)
	flags     []string // honesty flags, e.g. {"auto-detected"}, {"partial (truncated)"}
	page      string   // "page 2/4" when list has multiple pages; "" otherwise
	scroll    string   // "42%" when reader is scrollable; "" otherwise
	meta      string   // "1.2 KB", "3 users", …
	hints     string   // contextual keys, e.g. "↵ go · / filter · ? help"
	width     int
	styles    styles
}

// landingBar is the bar shown before anything is fetched.
func landingBar(width int, st styles) statusBar {
	return statusBar{hints: "type a target and press ↵ · ? help", width: width, styles: st}
}

func (b statusBar) render() string {
	if b.width <= 0 {
		return ""
	}
	st := b.styles

	// Right group: "◂ esc: X · page N/M · 42% · meta · hints", dim, truncated whole if needed.
	var right []string
	if b.escTarget != "" {
		right = append(right, "◂ esc: "+b.escTarget)
	}
	if b.page != "" {
		right = append(right, b.page)
	}
	if b.scroll != "" {
		right = append(right, b.scroll)
	}
	if b.meta != "" {
		right = append(right, b.meta)
	}
	if b.hints != "" {
		right = append(right, b.hints)
	}
	rightText := ansi.Truncate(strings.Join(right, " · "), b.width, "…")
	rightW := lipgloss.Width(rightText)

	// Left group: breadcrumb + flags. Flags are kept whole when they fit; the
	// breadcrumb truncates first because it is the most expendable content.
	avail := b.width - rightW - 1
	if avail < 0 {
		avail = 0
	}
	plainFlags, styledFlags := b.flagsWithin(avail)
	crumbBudget := avail - lipgloss.Width(plainFlags)
	if crumbBudget < 0 {
		crumbBudget = 0
	}

	left := b.styleCrumb(crumbBudget) + styledFlags
	leftW := lipgloss.Width(left)

	gap := b.width - leftW - rightW
	if gap < 0 {
		gap = 0
	}
	line := left + st.barFill.Render(strings.Repeat(" ", gap)) + st.barDim.Render(rightText)
	return st.barFill.Width(b.width).MaxWidth(b.width).Render(line)
}

func (b statusBar) flagsWithin(width int) (plain, styled string) {
	if width <= 0 {
		return "", ""
	}
	for _, f := range b.flags {
		nextPlain := plain + "  " + f
		if lipgloss.Width(nextPlain) > width {
			break
		}
		fs := b.styles.barFlag
		if strings.HasPrefix(f, "partial") {
			fs = b.styles.barWarn
		}
		plain = nextPlain
		styled += "  " + fs.Render(f)
	}
	return plain, styled
}

// styleCrumb renders the breadcrumb within budget: host dim + user bold when it
// fits; collapsed to a single truncated dim string when it does not (mixed
// styling can't survive a mid-run cut cleanly).
func (b statusBar) styleCrumb(budget int) string {
	st := b.styles
	full := b.host
	if b.user != "" {
		full += " / " + b.user
	}
	if lipgloss.Width(full) > budget {
		return st.barHost.Render(ansi.Truncate(full, budget, "…"))
	}
	if b.user == "" {
		return st.barHost.Render(b.host)
	}
	return st.barHost.Render(b.host) + st.barSep.Render(" / ") + gradientString(st.barUser, st.palette, b.user)
}

// breadcrumbParts splits a target into the bar's host ("@host") and user halves.
func breadcrumbParts(t finger.Target) (host, user string) {
	h, _, err := net.SplitHostPort(t.HostPort)
	if err != nil {
		h = t.HostPort
	}
	return "@" + h, t.User
}

// formatBytes renders a byte count compactly: "512 B", "1.2 KB", "3.4 MB".
func formatBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

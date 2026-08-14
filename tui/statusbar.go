package tui

import (
	"fmt"
	"net"
	"strings"
	"time"

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
	escShort  bool     // render "◂ esc" without its destination (state ladder rung 4)
	flags     []string // honesty flags, e.g. {"auto-detected"}, {"partial (truncated)"}
	page      string   // "page 2/4" when list has multiple pages; "" otherwise
	scroll    string   // "42%" when reader is scrollable; "" otherwise
	latency   string   // "123ms" for a landed response; "" otherwise
	meta      string   // "1.2 KB", "3 users", …
	hints     string   // contextual keys, e.g. "↵ go · / filter · ? help"
	priority  *priorityStatus
	width     int
	styles    styles
}

// priorityStatus keeps an expendable detail between the status context and
// its consequence/actions. When space runs out, detail is truncated before
// the suffix so the controls and stale-response warning remain visible.
type priorityStatus struct {
	prefix string
	detail string
	suffix string
}

func (s priorityStatus) text() string {
	return s.prefix + s.detail + s.suffix
}

func (s priorityStatus) render(width int) string {
	if width <= 0 {
		return ""
	}
	if full := s.text(); lipgloss.Width(full) <= width {
		return full
	}

	suffixWidth := lipgloss.Width(s.suffix)
	if suffixWidth >= width {
		return ansi.Cut(s.suffix, suffixWidth-width, suffixWidth)
	}
	prefixWidth := lipgloss.Width(s.prefix)
	detailWidth := width - prefixWidth - suffixWidth
	if detailWidth > 0 {
		return s.prefix + ansi.Truncate(s.detail, detailWidth, "…") + s.suffix
	}
	return ansi.Truncate(s.prefix, width-suffixWidth, "…") + s.suffix
}

// hintSeparator joins every hint list in the app. It is the only delimiter, so
// splitting on it recovers the individual hints losslessly.
const hintSeparator = " · "

// helpHint is the pointer to the overlay. hintsWithin keeps it after the
// first hint, then drops it if the two cannot both fit.
const helpHint = "? help"

// hintsFloor is the least hintsWithin will ever return without giving up: the
// first hint, which is the one that matters. On a failed request that is
// "r retry"; the state ladder may not shed below the width this needs.
func (b statusBar) hintsFloor() string {
	if b.hints == "" {
		return ""
	}
	return strings.Split(b.hints, hintSeparator)[0]
}

// hintsWithin reduces the hint list to the whole hints that fit budget cells,
// in three descending stages:
//
//  1. Drop hints from the end, keeping the first and helpHint. Lists are built
//     most-useful-first, so the tail loses least.
//  2. If the first and helpHint still do not fit together, keep the first.
//     helpHint is the pointer to the overlay, but it is not an action; on a
//     failed request the first hint is "r retry", which is the only thing worth
//     doing on that screen. Issue #76 exists because the bar spent its width on
//     less useful facts than the retry, and stage 2 is what stops this
//     function from reintroducing that.
//  3. Give up and return "". render falls back to its ordinary ellipsis
//     truncation, so a very narrow bar is no worse than before.
func (b statusBar) hintsWithin(budget int) string {
	if b.hints == "" || lipgloss.Width(b.hints) <= budget {
		return b.hints
	}
	if budget <= 0 {
		return ""
	}
	parts := strings.Split(b.hints, hintSeparator)
	for {
		drop := -1
		for i := len(parts) - 1; i > 0; i-- {
			if parts[i] == helpHint {
				continue
			}
			drop = i
			break
		}
		if drop < 0 {
			break
		}
		parts = append(parts[:drop], parts[drop+1:]...)
		if joined := strings.Join(parts, hintSeparator); lipgloss.Width(joined) <= budget {
			return joined
		}
	}
	if lipgloss.Width(parts[0]) <= budget {
		return parts[0]
	}
	return ""
}

// stateLadder returns the bar at each rung of the state-shedding order,
// richest first. render walks the rungs twice: first for a line that fits
// with its hints intact, then for one that still fits the address plus the
// hints' irreducible floor.
//
// The order is value, cheapest concession first: how long the request took,
// then how big it was, then where in it you are, then where esc goes, then
// that esc goes anywhere at all. render already applied exactly this test to
// latency alone; this generalises that line rather than adding a mechanism
// beside it.
func (b statusBar) stateLadder() []statusBar {
	rungs := []statusBar{b}
	next := b
	for _, reduce := range []func(*statusBar){
		func(s *statusBar) { s.latency = "" },
		func(s *statusBar) { s.meta = "" },
		func(s *statusBar) { s.page, s.scroll = "", "" },
		func(s *statusBar) { s.escShort = true },
		func(s *statusBar) { s.escTarget, s.escShort = "", false },
	} {
		reduce(&next)
		rungs = append(rungs, next)
	}
	return rungs
}

func (b statusBar) render() string {
	if b.width <= 0 {
		return ""
	}
	if b.priority != nil {
		return b.renderPriority()
	}
	st := b.styles

	// Right group: "◂ esc: X · page N/M · 42% · latency · meta · hints", dim, truncated whole if needed.
	allFlags, _ := b.flagsWithin(b.width)
	// Walk the state ladder. Default to rung 1 (latency dropped) when neither
	// pass finds a fit — the all-or-nothing test this generalises. Descending
	// further without buying a whole address would surrender state for nothing.
	rungs := b.stateLadder()
	chosen, crumbFits := rungs[1], false
	// Pass one: the richest rung at which the whole line fits with its hints
	// intact. This is the original all-or-nothing latency test, widened to
	// every state segment, and it keeps state from ever being bought with
	// hints — the wrong currency, since latency is the cheapest thing here.
	for _, rung := range rungs {
		if b.fullWidth(rung.rightParts(true), allFlags) <= b.width {
			chosen, crumbFits = rung, true
			break
		}
	}
	// Pass two: nothing fits whole, so find the richest rung whose state
	// coexists with the address and at least the hints' irreducible floor.
	// Below, the hints shed down to that floor to pay for it.
	if !crumbFits {
		for _, rung := range rungs {
			probe := rung
			probe.hints = b.hintsFloor()
			if b.fullWidth(probe.rightParts(true), allFlags) <= b.width {
				chosen, crumbFits = rung, true
				break
			}
		}
	}
	// b is a value receiver, so this shapes only the line being rendered.
	b.latency, b.meta, b.page = chosen.latency, chosen.meta, chosen.page
	b.scroll, b.escTarget, b.escShort = chosen.scroll, chosen.escTarget, chosen.escShort
	right := b.rightParts(true)
	// Honesty flags take precedence over contextual hints. Reserve their room
	// before truncating the right group so a new hint cannot hide a partial or
	// auto-detected marker on a narrow terminal.
	separator := 0
	if len(right) > 0 && (b.host != "" || b.user != "" || allFlags != "") {
		separator = 1
	}
	rightBudget := b.width - lipgloss.Width(allFlags) - separator
	if rightBudget < 0 {
		rightBudget = 0
	}
	rightJoined := strings.Join(right, " · ")
	// Shed whole hints before falling back to cutting a word. Their budget is
	// whatever the chosen state rung and the *whole* breadcrumb leave, so the
	// hints give way to the address rather than the address to the hints —
	// which is the defect review item 20 describes. rightParts appends hints
	// last when they exist, so the state is everything ahead of them.
	// crumbFits gates this: hints give way only when doing so actually buys the
	// whole address. Where the address cannot be whole at any rung — a bar
	// narrower than the breadcrumb itself — surrendering hints would cost them
	// for nothing, so the group is left alone and the breadcrumb collapses
	// instead, which is what TestStatusBarDoesNotReserveSpaceForHiddenBreadcrumb
	// has always asserted.
	if over := b.fullWidth(right, allFlags) - b.width; crumbFits && b.hints != "" && over > 0 {
		state := right[:len(right)-1]
		if kept := b.hintsWithin(lipgloss.Width(b.hints) - over); kept != b.hints {
			trimmed := append([]string{}, state...)
			if kept != "" {
				trimmed = append(trimmed, kept)
			}
			rightJoined = strings.Join(trimmed, " · ")
		}
	}
	rightText := ""
	if rightBudget > 0 {
		rightText = ansi.Truncate(rightJoined, rightBudget, "…")
	}
	rightW := lipgloss.Width(rightText)
	if rightW == 0 {
		separator = 0
	}

	// Left group: breadcrumb + flags. Flags are kept whole when they fit; the
	// breadcrumb truncates first because it is the most expendable content.
	avail := b.width - rightW - separator
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
	if leftW == 0 && rightW < b.width {
		rightText = ansi.Truncate(rightJoined, b.width, "…")
		rightW = lipgloss.Width(rightText)
	}

	gap := b.width - leftW - rightW
	if gap < 0 {
		gap = 0
	}
	line := left + st.barFill.Render(strings.Repeat(" ", gap)) + st.barDim.Render(rightText)
	return st.barFill.Width(b.width).MaxWidth(b.width).Render(line)
}

func (b statusBar) rightParts(includeLatency bool) []string {
	var right []string
	if b.escTarget != "" {
		// The affordance is what the user needs; the destination is a
		// courtesy. Rung 4 of the state ladder keeps the first and drops the
		// second, which buys 12-22 cells on a narrow bar.
		if b.escShort {
			right = append(right, "◂ esc")
		} else {
			right = append(right, "◂ esc: "+b.escTarget)
		}
	}
	if b.page != "" {
		right = append(right, b.page)
	}
	if b.scroll != "" {
		right = append(right, b.scroll)
	}
	if includeLatency && b.latency != "" {
		right = append(right, b.latency)
	}
	if b.meta != "" {
		right = append(right, b.meta)
	}
	if b.hints != "" {
		right = append(right, b.hints)
	}
	return right
}

func (b statusBar) fullWidth(right []string, flags string) int {
	crumb := b.host
	if b.user != "" {
		crumb += " / " + b.user
	}
	leftWidth := lipgloss.Width(crumb) + lipgloss.Width(flags)
	rightWidth := lipgloss.Width(strings.Join(right, " · "))
	if leftWidth > 0 && rightWidth > 0 {
		return leftWidth + 1 + rightWidth
	}
	return leftWidth + rightWidth
}

func (b statusBar) renderPriority() string {
	st := b.styles
	full := b.priority.text()
	if priorityWidth := lipgloss.Width(full); priorityWidth < b.width {
		ordinaryWidth := b.width - priorityWidth - lipgloss.Width(" · ")
		if ordinaryWidth > 0 && b.hasOrdinaryStatus() {
			ordinary := b
			ordinary.priority = nil
			ordinary.hints = ""
			ordinary.width = ordinaryWidth
			line := ordinary.render() + st.barDim.Render(" · "+full)
			return st.barFill.Width(b.width).MaxWidth(b.width).Render(line)
		}
	}

	priority := b.priority.render(b.width)
	gap := b.width - lipgloss.Width(priority)
	if gap < 0 {
		gap = 0
	}
	line := st.barFill.Render(strings.Repeat(" ", gap)) + st.barDim.Render(priority)
	return st.barFill.Width(b.width).MaxWidth(b.width).Render(line)
}

func (b statusBar) hasOrdinaryStatus() bool {
	return b.host != "" || b.user != "" || b.escTarget != "" || len(b.flags) > 0 ||
		b.page != "" || b.scroll != "" || b.latency != "" || b.meta != ""
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
	return st.barHost.Render(b.host) + st.barSep.Render(" / ") + st.barUser.Render(b.user)
}

// breadcrumbParts splits a target into the bar's host ("@host") and user halves.
func breadcrumbParts(t finger.Target) (host, user string) {
	h, _, err := net.SplitHostPort(t.HostPort)
	if err != nil {
		h = t.HostPort
	}
	return "@" + h, t.QueryLine()
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

func formatElapsed(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

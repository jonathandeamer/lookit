package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

const (
	aboutTagline      = "A modern TUI browser for the Finger protocol"
	aboutRepo         = "github.com/jonathandeamer/lookit"
	aboutFingerAuthor = "jonathan@tilde.team"
	aboutIssuesURL    = "https://github.com/jonathandeamer/lookit/issues"
)

// aboutView composes the centered about block. Pure: string in, string out, so
// it is golden-testable. The identity lines are centered relative to each other;
// the personality and action groups are left-aligned internally and centered as
// blocks, matching the approved layout.
func aboutView(st styles, profile colorprofile.Profile, version, builtAt string, width, height int) string {
	dim := lipgloss.NewStyle().Foreground(st.palette.Dim)
	text := lipgloss.NewStyle().Foreground(st.palette.Text)
	spark := lipgloss.NewStyle().Foreground(st.palette.AccentMint)
	arrow := lipgloss.NewStyle().Foreground(st.palette.AccentViolet)

	identity := []string{
		headerMark(st, profile),
		dim.Render(aboutTagline),
		"",
		dim.Render("lookit " + version + " · MIT license"),
	}
	if builtAt != "" && builtAt != "unknown" {
		identity = append(identity, dim.Render("built "+builtAt))
	}
	identity = append(identity, dim.Render(aboutRepo))
	identityBlock := lipgloss.JoinVertical(lipgloss.Center, identity...)

	bullets := lipgloss.JoinVertical(
		lipgloss.Left,
		spark.Render("✦ ")+text.Render("Built with Charm · charm.sh"),
		spark.Render("✦ ")+text.Render("young software — bug reports & ideas welcome"),
	)

	// Right-pad the shorter action so both key hints align in a column.
	left1 := arrow.Render("➜ ") + text.Render("finger "+aboutFingerAuthor)
	left2 := arrow.Render("➜ ") + text.Render("report a bug or idea")
	leftW := lipgloss.Width(left1)
	if w := lipgloss.Width(left2); w > leftW {
		leftW = w
	}
	const hintGap = 6
	pad := func(s string) string {
		return s + strings.Repeat(" ", leftW-lipgloss.Width(s)+hintGap)
	}
	actions := lipgloss.JoinVertical(
		lipgloss.Left,
		pad(left1)+dim.Render("↵ go"),
		pad(left2)+dim.Render("y copy"),
	)

	block := lipgloss.JoinVertical(
		lipgloss.Center,
		identityBlock,
		"",
		bullets,
		"",
		actions,
		"",
		dim.Render("thanks for supporting the small internet"),
	)

	// Per-line truncation so long lines (tagline, repo URL) degrade on narrow
	// terminals instead of overflowing the placed width.
	if width > 0 {
		lines := strings.Split(block, "\n")
		for i, ln := range lines {
			lines[i] = ansi.Truncate(ln, width, "…")
		}
		block = strings.Join(lines, "\n")
	}

	if width <= 0 || height <= 0 {
		return block
	}
	vPos := lipgloss.Center
	if lipgloss.Height(block) >= height {
		vPos = lipgloss.Top // very short terminal: top-align rather than clip
	}
	return lipgloss.Place(width, height, lipgloss.Center, vPos, block)
}

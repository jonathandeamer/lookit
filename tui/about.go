package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

const (
	aboutTagline          = "A modern TUI browser for the finger protocol"
	aboutRepo             = "https://github.com/jonathandeamer/lookit"
	aboutFingerAuthor     = "jonathan@tilde.team"
	aboutIssuesURL        = "https://github.com/jonathandeamer/lookit/issues"
	aboutCharmURL         = "https://charm.sh"
	aboutCatalogCreditURL = "https://640kb.neocities.org/fingerverse/"
)

// aboutModel renders the full-screen about view. Like readerModel it owns no
// lifecycle or quit; appModel drives it via setSize/setProfile/setBackground.
type aboutModel struct {
	profile colorprofile.Profile
	styles  styles
	version string // bare version, e.g. "v0.0.1" or "dev"
	builtAt string // build date, e.g. "2026-06-03"; "" / "unknown" hides the row
	width   int
	height  int
}

func newAbout(profile colorprofile.Profile, version, builtAt string) aboutModel {
	return aboutModel{
		profile: profile,
		styles:  newStyles(true),
		version: version,
		builtAt: builtAt,
	}
}

func (m *aboutModel) setSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *aboutModel) setProfile(p colorprofile.Profile) { m.profile = p }

func (m *aboutModel) setBackground(dark bool) { m.styles = newStyles(dark) }

func (m aboutModel) View() string {
	return aboutView(m.styles, m.profile, m.version, m.builtAt, m.width, m.height)
}

// aboutView composes the centered about block. Pure: string in, string out, so
// it is golden-testable. The identity lines are centered relative to each other;
// the personality and action groups are left-aligned internally and centered as
// blocks, matching the approved layout.
func aboutView(st styles, profile colorprofile.Profile, version, builtAt string, width, height int) string {
	dim := lipgloss.NewStyle().Foreground(st.palette.Dim)
	text := lipgloss.NewStyle().Foreground(st.palette.Text)
	spark := lipgloss.NewStyle().Foreground(st.palette.AccentMint)
	arrow := lipgloss.NewStyle().Foreground(st.palette.AccentViolet)

	// Long lines (tagline, repo URL, the catalog credit) degrade by truncation on
	// narrow terminals. Fit each line to the terminal *before* any JoinVertical,
	// so a group is centered against lines that already fit: centering first and
	// truncating after chops JoinVertical's own right-hand padding, which paints
	// an ellipsis onto lines that were never too long and shifts the whole block
	// off-centre once its widest line exceeds the terminal.
	fit := func(lines ...string) []string {
		if width <= 0 {
			return lines
		}
		out := make([]string, len(lines))
		for i, ln := range lines {
			out[i] = ansi.Truncate(ln, width, "…")
		}
		return out
	}

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
	identityBlock := lipgloss.JoinVertical(lipgloss.Center, fit(identity...)...)

	// The two credits are peers — one for the toolkit, one for the catalog — so
	// they share the bullets' weight rather than one hiding in the dim identity
	// block. The catalog credit goes last: its URL is the longest line here, and
	// the ragged edge is least visible at the bottom of a left-aligned group.
	bullet := func(label, url string) string {
		return spark.Render("✦ ") + text.Render(label+" ") + text.Hyperlink(url).Render(url)
	}
	bullets := lipgloss.JoinVertical(lipgloss.Left, fit(
		bullet("Built with Charm", aboutCharmURL),
		spark.Render("✦ ")+text.Render("Young software; bug reports & ideas welcome"),
		bullet("Catalog inspired by", aboutCatalogCreditURL),
	)...)

	// Right-pad the shorter action so both key hints align in a column.
	left1 := arrow.Render("➜ ") + text.Render("finger "+aboutFingerAuthor)
	left2 := arrow.Render("➜ ") + text.Render("Report a bug or idea")
	leftW := lipgloss.Width(left1)
	if w := lipgloss.Width(left2); w > leftW {
		leftW = w
	}
	const hintGap = 6
	pad := func(s string) string {
		return s + strings.Repeat(" ", leftW-lipgloss.Width(s)+hintGap)
	}
	actions := lipgloss.JoinVertical(lipgloss.Left, fit(
		pad(left1)+dim.Render("↵ go"),
		pad(left2)+dim.Render("y copy issues URL"),
	)...)

	closing := fit(dim.Render("Thanks for supporting the small internet"))[0]

	block := lipgloss.JoinVertical(
		lipgloss.Center,
		identityBlock,
		"",
		bullets,
		"",
		actions,
		"",
		closing,
	)

	if width <= 0 || height <= 0 {
		return block
	}
	vPos := lipgloss.Center
	if lipgloss.Height(block) >= height {
		vPos = lipgloss.Top // very short terminal: top-align rather than clip
	}
	return lipgloss.Place(width, height, lipgloss.Center, vPos, block)
}

package tui

import "charm.land/lipgloss/v2"

type styles struct {
	// bottom status bar
	barFill lipgloss.Style // full-width background
	barHost lipgloss.Style // "@host" (dim)
	barSep  lipgloss.Style // " / " separator
	barUser lipgloss.Style // "user" (bold/bright)
	barFlag lipgloss.Style // neutral flag, e.g. "auto-detected"
	barWarn lipgloss.Style // caution flag, e.g. "partial (truncated)"
	barDim  lipgloss.Style // right-aligned context (esc/meta/hints)
}

func newStyles() styles {
	barBg := lipgloss.Color("#242424")
	seg := lipgloss.NewStyle().Background(barBg)
	return styles{
		barFill: seg,
		barHost: seg.Foreground(lipgloss.Color("#9a9a9a")),
		barSep:  seg.Foreground(lipgloss.Color("#6a6a6a")),
		barUser: seg.Foreground(lipgloss.Color("#ffffff")).Bold(true),
		barFlag: seg.Foreground(lipgloss.Color("#9a9a9a")),
		barWarn: seg.Foreground(lipgloss.Color("#c9a227")),
		barDim:  seg.Foreground(lipgloss.Color("#808080")),
	}
}

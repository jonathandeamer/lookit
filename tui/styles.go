package tui

import "charm.land/lipgloss/v2"

type styles struct {
	title  lipgloss.Style
	status lipgloss.Style
	error  lipgloss.Style
	hint   lipgloss.Style
}

func newStyles() styles {
	return styles{
		title:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff6fd5")),
		status: lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")),
		error:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b6b")),
		hint:   lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")),
	}
}

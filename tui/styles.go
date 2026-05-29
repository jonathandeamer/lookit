package tui

import "charm.land/lipgloss/v2"

type styles struct {
	title    lipgloss.Style
	status   lipgloss.Style
	error    lipgloss.Style
	hint     lipgloss.Style
	listName lipgloss.Style // dim real-name column in list rows
	selected lipgloss.Style // highlighted list row
}

func newStyles() styles {
	return styles{
		title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff6fd5")),
		status:   lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")),
		error:    lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b6b")),
		hint:     lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")),
		listName: lipgloss.NewStyle().Foreground(lipgloss.Color("#8fb7ff")),
		selected: lipgloss.NewStyle().Foreground(lipgloss.Color("#8affc1")).Bold(true),
	}
}

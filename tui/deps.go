package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	lipgloss "charm.land/lipgloss/v2"
)

// Dependency anchors keep planned TUI libraries in the module graph until the
// real TUI implementation imports them in later tasks.
var (
	_ tea.ColorProfileMsg
	_ tea.Model = dependencyModel{}
	_           = tea.NewProgram
	_           = tea.NewView
	_           = textinput.Model{}
	_           = viewport.Model{}
	_           = lipgloss.NewStyle
)

type dependencyModel struct{}

func (dependencyModel) Init() tea.Cmd {
	return nil
}

func (dependencyModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return dependencyModel{}, nil
}

func (dependencyModel) View() tea.View {
	return tea.NewView("")
}

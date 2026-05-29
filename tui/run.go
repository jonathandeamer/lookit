package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// Run starts the interactive TUI and blocks until the user quits.
//
// Bubble Tea v2's Program.Run does not take a context. The ctx parameter is
// accepted now so cancellation can be wired in later without changing main.go;
// this implementation does not yet use it.
func Run(ctx context.Context, profile colorprofile.Profile) error {
	_ = ctx
	program := tea.NewProgram(newApp(defaultFetch, profile))
	_, err := program.Run()
	return err
}

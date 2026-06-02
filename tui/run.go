package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// Options configures a TUI session launched from the command line.
type Options struct {
	// InitialQuery is the raw positional argument, replayed through the landing
	// input's submit path on startup (the same path a typed target takes). It is
	// only meaningful when Seed is true.
	InitialQuery string
	// Seed reports whether a positional argument was supplied at all. It is
	// tracked separately from InitialQuery because a supplied-but-blank argument
	// (lookit "" / lookit "   ") has an empty InitialQuery yet must still be
	// replayed, so the user gets the same parse-error-in-place behaviour as a
	// malformed target rather than a silent landing.
	Seed bool
	// Version is the build version line surfaced in the ? help panel.
	Version string
}

// Run starts the interactive TUI and blocks until the user quits.
//
// Bubble Tea v2's Program.Run does not take a context. The ctx parameter is
// accepted now so cancellation can be wired in later without changing main.go;
// this implementation does not yet use it.
func Run(ctx context.Context, profile colorprofile.Profile, opts Options) error {
	_ = ctx
	program := tea.NewProgram(newAppWithOptions(defaultFetch, profile, opts))
	_, err := program.Run()
	return err
}

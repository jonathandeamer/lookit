package tui

import "charm.land/bubbles/v2/key"

// keyMap holds lookit's app-level key bindings. It is matched with key.Matches
// and doubles as the help.KeyMap that drives the bubbles/help panel. Scroll,
// page, and jump bindings are owned by the bubbles viewport/list at runtime;
// they appear here only so the help panel advertises them (we disable the
// list's built-in help).
type keyMap struct {
	FocusInput key.Binding
	Back       key.Binding
	Open       key.Binding
	Filter     key.Binding
	Raw        key.Binding
	Copy       key.Binding
	Help       key.Binding
	Quit       key.Binding
	ForceQuit  key.Binding

	// display-only (handled by sub-models)
	Move key.Binding
	Page key.Binding
	Jump key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		FocusInput: key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "target")),
		Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Open:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "open")),
		Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Raw:        key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "raw")),
		Copy:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:       key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		ForceQuit:  key.NewBinding(key.WithKeys("ctrl+c")),
		Move:       key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓", "move")),
		Page:       key.NewBinding(key.WithKeys("left", "right", "h", "l", "pgup", "pgdown"), key.WithHelp("←/→", "page")),
		Jump:       key.NewBinding(key.WithKeys("g", "G"), key.WithHelp("g/G", "top/bottom")),
	}
}

// ShortHelp implements help.KeyMap. (Unused by the bar today, which renders its
// own focus-aware hints, but required by the interface.)
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.FocusInput, k.Back, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap — the expanded panel toggled by '?'.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Open, k.FocusInput, k.Copy, k.Raw},
		{k.Move, k.Page, k.Jump, k.Filter},
		{k.Back, k.Help, k.Quit},
	}
}

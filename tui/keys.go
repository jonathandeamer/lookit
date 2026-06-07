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
	About      key.Binding
	Quit       key.Binding
	ForceQuit  key.Binding

	// display-only (handled by sub-models)
	Move key.Binding
	Page key.Binding
	Jump key.Binding

	// link navigation (reader only)
	LinkNext   key.Binding
	LinkPrev   key.Binding
	LinkFinger key.Binding
	LinkPanel  key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		FocusInput: key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "target")),
		Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Open:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "go")),
		Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Raw:        key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view source")),
		Copy:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		About:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "about lookit")),
		Quit:       key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		ForceQuit:  key.NewBinding(key.WithKeys("ctrl+c")),
		Move:       key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓", "move")),
		Page:       key.NewBinding(key.WithKeys("left", "right", "h", "l", "pgup", "pgdown"), key.WithHelp("←/→", "page")),
		Jump:       key.NewBinding(key.WithKeys("g", "G"), key.WithHelp("g/G", "top/bottom")),
		LinkNext:   key.NewBinding(key.WithKeys("tab", "n"), key.WithHelp("tab/n", "next link")),
		LinkPrev:   key.NewBinding(key.WithKeys("shift+tab", "N"), key.WithHelp("shift+tab/N", "prev link")),
		LinkFinger: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "finger link")),
		LinkPanel:  key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "links panel")),
	}
}

// ShortHelp implements help.KeyMap. (Unused by the bar today, which renders its
// own focus-aware hints, but required by the interface.)
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.FocusInput, k.Back, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap — the expanded panel toggled by '?'. Help
// itself is intentionally omitted: the bottom bar always advertises "? help", so
// listing it inside the open panel (where '?' actually closes) is redundant.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Open, k.FocusInput, k.Copy, k.Raw},
		{k.Move, k.Page, k.Jump, k.Filter},
		{k.Back, k.About, k.Quit},
	}
}

package tui

import "charm.land/bubbles/v2/key"

// keyMap holds lookit's app-level key bindings. It is matched with key.Matches.
// Scroll, page, and jump bindings are owned by the bubbles viewport/list at
// runtime. Move and Page appear here so the help panel can advertise them; Jump
// mirrors the working advanced shortcut but is intentionally omitted from help.
type keyMap struct {
	FocusInput key.Binding
	Back       key.Binding
	Open       key.Binding
	Filter     key.Binding
	Raw        key.Binding
	Wrap       key.Binding
	Refresh    key.Binding
	Copy       key.Binding
	Help       key.Binding
	About      key.Binding
	Bookmark   key.Binding
	Home       key.Binding
	Browse     key.Binding
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
		Wrap:       key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "wrap")),
		Refresh:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Copy:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		About:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "about lookit")),
		// Both keys take a letter bubbles binds to paging: 'b' is PrevPage in the
		// list AND the viewport, 'h' is PrevPage in the list and CursorLeft in the
		// viewport. handleKey matches before delegating, so letter back-paging goes
		// away in the reader and every user list. Accepted: 'b'/bookmark and
		// 'h'/home are the stronger mnemonics and both need a bare letter. ←/pgup
		// still page back, and 'l' still pages forward.
		Bookmark: key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "bookmark")),
		Home:     key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "home")),
		// Browse is the omnibox gesture: from the target input, drop into the
		// startpage list. Both keys are free in textinput (↓ has no binding, and tab
		// only matters to the reader's LinkNext, which the input never reaches).
		Browse:    key.NewBinding(key.WithKeys("down", "tab"), key.WithHelp("↓", "browse")),
		Quit:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		ForceQuit: key.NewBinding(key.WithKeys("ctrl+c")),
		Move:      key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓", "move")),
		// 'h' only. keyMap.Page is display-only (it advertises what the viewport
		// and list bind at runtime), so it must drop the key we intercept and keep
		// the ones we don't — 'l' still pages.
		Page:       key.NewBinding(key.WithKeys("left", "right", "l", "pgup", "pgdown"), key.WithHelp("←/→", "page")),
		Jump:       key.NewBinding(key.WithKeys("g", "G"), key.WithHelp("g/G", "top/bottom")),
		LinkNext:   key.NewBinding(key.WithKeys("tab", "n"), key.WithHelp("tab", "next link")),
		LinkPrev:   key.NewBinding(key.WithKeys("shift+tab", "N"), key.WithHelp("shift+tab", "previous link")),
		LinkFinger: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "finger link")),
		LinkPanel:  key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "browse links")),
	}
}

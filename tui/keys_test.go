package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
)

func TestKeyMapBindings(t *testing.T) {
	k := newKeyMap()
	// Sanity: the keys we rely on are bound to the expected runes.
	cases := map[string]key.Binding{
		"i":   k.FocusInput,
		"y":   k.Copy,
		"r":   k.Refresh,
		"v":   k.Raw,
		"q":   k.Quit,
		"?":   k.Help,
		"esc": k.Back,
	}
	for want, b := range cases {
		if got := b.Keys(); len(got) == 0 || !contains(got, want) {
			t.Fatalf("binding %v keys = %v, want to contain %q", b.Help(), got, want)
		}
	}
}

func TestRefreshKeyHelp(t *testing.T) {
	got := newKeyMap().Refresh.Help()
	if got != (key.Help{Key: "r", Desc: "refresh"}) {
		t.Fatalf("Refresh help = %+v", got)
	}
}

func TestLinkKeyHelpSimplifiesDisplayWithoutRemovingAliases(t *testing.T) {
	k := newKeyMap()
	tests := []struct {
		name     string
		binding  key.Binding
		wantHelp key.Help
		wantKeys []string
	}{
		{
			name:     "next",
			binding:  k.LinkNext,
			wantHelp: key.Help{Key: "tab", Desc: "next link"},
			wantKeys: []string{"tab", "n"},
		},
		{
			name:     "previous",
			binding:  k.LinkPrev,
			wantHelp: key.Help{Key: "shift+tab", Desc: "previous link"},
			wantKeys: []string{"shift+tab", "N"},
		},
		{
			name:     "panel",
			binding:  k.LinkPanel,
			wantHelp: key.Help{Key: "L", Desc: "browse links"},
			wantKeys: []string{"L"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.binding.Help(); got != tt.wantHelp {
				t.Fatalf("link help = %+v, want %+v", got, tt.wantHelp)
			}
			for _, want := range tt.wantKeys {
				if got := tt.binding.Keys(); !contains(got, want) {
					t.Errorf("keys = %v, want to retain %q", got, want)
				}
			}
		})
	}
}

func TestKeyMapFullHelpOmitsJumpButKeepsBinding(t *testing.T) {
	k := newKeyMap()
	var all []string
	for _, group := range k.FullHelp() {
		for _, b := range group {
			all = append(all, strings.Join(b.Keys(), ","))
		}
	}
	joined := strings.Join(all, " ")
	for _, want := range []string{"i", "y", "r", "esc", "q", "left,right,l,pgup,pgdown"} {
		if !contains(all, want) {
			t.Fatalf("FullHelp missing %q; got %s", want, joined)
		}
	}
	// '?' is intentionally absent from the panel: the bottom bar always shows
	// "? help", and inside the open panel '?' actually closes it.
	if strings.Contains(joined, "?") {
		t.Fatalf("FullHelp should omit '?' (it lives in the bottom bar); got %s", joined)
	}
	if contains(all, "g,G") {
		t.Fatalf("FullHelp should omit Jump; got %s", joined)
	}
	for _, want := range []string{"g", "G"} {
		if got := k.Jump.Keys(); !contains(got, want) {
			t.Fatalf("Jump keys = %v, want to retain %q", got, want)
		}
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func TestKeyMapAboutBinding(t *testing.T) {
	k := newKeyMap()
	if got := k.About.Keys(); len(got) == 0 || !contains(got, "a") {
		t.Fatalf("About keys = %v, want to contain 'a'", got)
	}
	if h := k.About.Help(); h.Key != "a" || h.Desc != "about lookit" {
		t.Fatalf("About help = %+v, want {a, about lookit}", h)
	}
	var all []string
	for _, group := range k.FullHelp() {
		for _, b := range group {
			all = append(all, strings.Join(b.Keys(), ","))
		}
	}
	if !strings.Contains(strings.Join(all, " "), "a") {
		t.Fatalf("FullHelp should advertise the about key 'a': %v", all)
	}
}

func TestBookmarkAndHomeKeysBound(t *testing.T) {
	k := newKeyMap()
	if got := k.Bookmark.Keys(); !contains(got, "b") {
		t.Fatalf("Bookmark keys = %v, want b", got)
	}
	if got := k.Home.Keys(); !contains(got, "h") {
		t.Fatalf("Home keys = %v, want h", got)
	}
	if got, want := k.Home.Help(), (key.Help{Key: "h", Desc: "home"}); got != want {
		t.Fatalf("Home help = %+v, want %+v", got, want)
	}
	// h moved from Page to Home, so the help must stop claiming it.
	if got := k.Page.Keys(); contains(got, "h") {
		t.Fatalf("Page keys = %v, must not still claim h", got)
	}
	// ...but l is untouched: keyMap.Page is display-only, and the viewport and
	// list both still bind l to page forward. Dropping it would advertise LESS
	// than lookit does, which is the opposite of the honesty this list exists for.
	if got := k.Page.Keys(); !contains(got, "l") {
		t.Fatalf("Page keys = %v, want l — it still pages", got)
	}
	if got := k.Page.Keys(); !contains(got, "left") || !contains(got, "right") {
		t.Fatalf("Page keys = %v, want the arrows", got)
	}
}

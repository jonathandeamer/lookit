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

func TestKeyMapFullHelpIncludesPageAndMoveKeys(t *testing.T) {
	k := newKeyMap()
	var all []string
	for _, group := range k.FullHelp() {
		for _, b := range group {
			all = append(all, strings.Join(b.Keys(), ","))
		}
	}
	joined := strings.Join(all, " ")
	for _, want := range []string{"i", "y", "esc", "q"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("FullHelp missing %q; got %s", want, joined)
		}
	}
	// '?' is intentionally absent from the panel: the bottom bar always shows
	// "? help", and inside the open panel '?' actually closes it.
	if strings.Contains(joined, "?") {
		t.Fatalf("FullHelp should omit '?' (it lives in the bottom bar); got %s", joined)
	}
	// Page/move discoverability (owed because we disable the list's own help).
	if !strings.Contains(joined, "left") || !strings.Contains(joined, "g") {
		t.Fatalf("FullHelp must advertise page/move keys; got %s", joined)
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

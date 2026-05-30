package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jonathandeamer/lookit/finger"
)

func testCommon() *commonModel {
	return &commonModel{width: 80, height: 24}
}

func hostTarget(t *testing.T, raw string) finger.Target {
	t.Helper()
	target, err := finger.ParseTarget(raw)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestNewListSelectsFirstUser(t *testing.T) {
	users := []User{{Login: "alrs"}, {Login: "dtracker", Name: "DT"}}
	m := newList(testCommon(), hostTarget(t, "@tilde.team"), users)

	sel, ok := m.selected()
	if !ok {
		t.Fatal("selected ok = false, want true")
	}
	if sel.login != "alrs" {
		t.Fatalf("selected login = %q, want alrs", sel.login)
	}
}

func TestListMoveDownChangesSelection(t *testing.T) {
	users := []User{{Login: "alrs"}, {Login: "dtracker"}}
	m := newList(testCommon(), hostTarget(t, "@tilde.team"), users)

	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyDown})

	sel, ok := m.selected()
	if !ok {
		t.Fatal("selected ok = false after move, want true")
	}
	if sel.login != "dtracker" {
		t.Fatalf("after down, selected = %q, want dtracker", sel.login)
	}
}

func TestListViewShowsLoginAndName(t *testing.T) {
	users := []User{{Login: "geurimja", Name: "Geurimja"}}
	m := newList(testCommon(), hostTarget(t, "@plan.cat"), users)

	view := m.View()
	if !strings.Contains(view, "geurimja") {
		t.Fatalf("view missing login: %q", view)
	}
	if !strings.Contains(view, "Geurimja") {
		t.Fatalf("view missing name: %q", view)
	}
}

func TestListNotFilteringByDefault(t *testing.T) {
	m := newList(testCommon(), hostTarget(t, "@tilde.team"), []User{{Login: "alrs"}})
	if m.filtering() {
		t.Fatal("filtering = true, want false on a fresh list")
	}
}

func TestGenericListFlaggedGeneric(t *testing.T) {
	users := []User{{Login: "betsy"}, {Login: "oleander"}}
	body := []byte("betsy\noleander\n")
	m := newListWithPreamble(testCommon(), hostTarget(t, "@unknown.host"), users, body, false, true)
	// Flags are now in the status bar, not appended to the list Title.
	wantTitle := "@unknown.host — 2 users"
	if m.list.Title != wantTitle {
		t.Fatalf("title = %q, want plain %q (flags moved to status bar)", m.list.Title, wantTitle)
	}
	if !m.generic {
		t.Fatal("listModel.generic = false, want true")
	}
}

func TestGenericListPreambleHasViewRawNote(t *testing.T) {
	users := []User{{Login: "betsy"}, {Login: "oleander"}}
	body := []byte("betsy\noleander\n")
	m := newListWithPreamble(testCommon(), hostTarget(t, "@unknown.host"), users, body, false, true)
	if !strings.Contains(m.preamble, "press r") {
		t.Fatalf("preamble = %q, want it to mention the view-raw key", m.preamble)
	}
}

func TestRecognizedListNotFlagged(t *testing.T) {
	users := []User{{Login: "alrs"}, {Login: "dtracker"}}
	body := []byte(hostListBody())
	m := newListWithPreamble(testCommon(), hostTarget(t, "@tilde.team"), users, body, false, false)
	// Title is a plain "host — N users" string; no flag suffixes (flags are in the bar).
	wantTitle := "@tilde.team — 2 users"
	if m.list.Title != wantTitle {
		t.Fatalf("title = %q, want plain %q", m.list.Title, wantTitle)
	}
	if m.generic {
		t.Fatal("listModel.generic = true, want false for a recognized list")
	}
}

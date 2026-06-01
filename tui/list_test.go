package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jonathandeamer/lookit/finger"
)

func testCommon() *commonModel {
	return &commonModel{width: 80, height: 24, darkBackground: true, styles: newStyles(true)}
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
	m := newListWithPreamble(testCommon(), hostTarget(t, "@unknown.host"), users, body, true)
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
	m := newListWithPreamble(testCommon(), hostTarget(t, "@unknown.host"), users, body, true)
	if !strings.Contains(m.preamble, "press r") {
		t.Fatalf("preamble = %q, want it to mention the view-raw key", m.preamble)
	}
}

func TestRecognizedListNotFlagged(t *testing.T) {
	users := []User{{Login: "alrs"}, {Login: "dtracker"}}
	body := []byte(hostListBody())
	m := newListWithPreamble(testCommon(), hostTarget(t, "@tilde.team"), users, body, false)
	// Title is a plain "host — N users" string; no flag suffixes (flags are in the bar).
	wantTitle := "@tilde.team — 2 users"
	if m.list.Title != wantTitle {
		t.Fatalf("title = %q, want plain %q", m.list.Title, wantTitle)
	}
	if m.generic {
		t.Fatal("listModel.generic = true, want false for a recognized list")
	}
}

func TestUserItemImplementsDefaultItem(t *testing.T) {
	it := userItem{login: "alrs", name: "Alvaro", target: "alrs@tilde.team"}
	if it.Title() != "alrs" {
		t.Fatalf("Title = %q, want alrs", it.Title())
	}
	desc := it.Description()
	if !strings.Contains(desc, "Alvaro") || !strings.Contains(desc, "alrs@tilde.team") {
		t.Fatalf("Description = %q, want name + target", desc)
	}
}

func TestUserItemDescription(t *testing.T) {
	tests := []struct {
		name string
		item userItem
		want string
	}{
		{
			name: "name and target",
			item: userItem{login: "alrs", name: "Alvaro", target: "alrs@tilde.team"},
			want: "Alvaro · alrs@tilde.team",
		},
		{
			name: "name only",
			item: userItem{login: "alrs", name: "Alvaro"},
			want: "Alvaro",
		},
		{
			name: "target only",
			item: userItem{login: "alrs", target: "alrs@tilde.team"},
			want: "alrs@tilde.team",
		},
		{
			name: "neither",
			item: userItem{login: "alrs"},
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.item.Description()
			if got != tc.want {
				t.Fatalf("Description() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDefaultDelegateRendersLoginAndName(t *testing.T) {
	common := &commonModel{width: 80, height: 20, darkBackground: true, styles: newStyles(true)}
	m := newList(common, hostTarget(t, "@tilde.team"), []User{{Login: "alrs", Name: "Alvaro"}})
	m.setSize(80, 18)
	view := m.View()
	if !strings.Contains(view, "alrs") || !strings.Contains(view, "Alvaro") {
		t.Fatalf("list view missing login/name:\n%s", view)
	}
}

func TestNewListCapsEntries(t *testing.T) {
	users := make([]User, 5000)
	for i := range users {
		users[i] = User{Login: fmt.Sprintf("u%d", i)}
	}
	common := testCommon()
	m := newList(common, finger.Target{Raw: "@big.example"}, users)
	if got := len(m.list.Items()); got != maxListEntries {
		t.Fatalf("newList kept %d items, want exactly %d", got, maxListEntries)
	}
}

func TestNewListWithPreambleNoNoteAtCap(t *testing.T) {
	users := make([]User, maxListEntries)
	for i := range users {
		users[i] = User{Login: fmt.Sprintf("u%d", i)}
	}
	common := testCommon()
	m := newListWithPreamble(common, finger.Target{Raw: "@big.example"}, users, nil, false)
	if got := len(m.list.Items()); got != maxListEntries {
		t.Fatalf("at cap: kept %d items, want %d", got, maxListEntries)
	}
	if strings.Contains(m.preamble, "truncated") {
		t.Fatalf("at cap: preamble = %q, want no truncation note", m.preamble)
	}
}

func TestNewListWithPreambleNotesTruncation(t *testing.T) {
	users := make([]User, 5000)
	for i := range users {
		users[i] = User{Login: fmt.Sprintf("u%d", i)}
	}
	common := testCommon()
	m := newListWithPreamble(common, finger.Target{Raw: "@big.example"}, users, nil, false)
	if !strings.Contains(m.preamble, "truncated") {
		t.Fatalf("preamble = %q, want a truncation note", m.preamble)
	}
}

func TestNewListUsesSharedStyles(t *testing.T) {
	common := testCommon()
	common.styles = newStyles(false)
	common.darkBackground = false
	m := newList(common, hostTarget(t, "@tilde.team"), []User{{Login: "alrs", Name: "Alvaro"}})

	if !sameColor(m.list.Styles.Filter.Focused.Prompt.GetForeground(), common.styles.input.Focused.Prompt.GetForeground()) {
		t.Fatal("list filter prompt should use shared input prompt colour")
	}
	if !sameColor(m.list.Styles.Spinner.GetForeground(), common.styles.spinner.GetForeground()) {
		t.Fatal("list spinner should use shared spinner colour")
	}
	if !strings.Contains(m.View(), "\x1b[38;2;168;31;98") {
		t.Fatalf("light selected row should contain selected login colour:\n%s", m.View())
	}
}

func TestSelectedListItemShelfSpansFullWidth(t *testing.T) {
	common := testCommon()
	common.width = 32
	m := newList(common, hostTarget(t, "@tilde.team"), []User{{Login: "alrs", Name: "Alvaro"}})

	view := m.View()
	assertFullWidthStyledLine(t, "selected title", lineContaining(t, view, "alrs"), m.list.Width(), common.styles.palette.SelectionBg)
	assertFullWidthStyledLine(t, "selected description", lineContaining(t, view, "Alvaro"), m.list.Width(), common.styles.palette.SelectionBg)
}

func TestSelectedListItemShelfIncludesBlankDescriptionLine(t *testing.T) {
	common := testCommon()
	common.width = 32
	m := newList(common, hostTarget(t, "@tilde.team"), []User{{Login: "alrs"}})

	lines := strings.Split(m.View(), "\n")
	titleIndex := lineIndexContaining(t, m.View(), "alrs")
	if len(lines) <= titleIndex+1 {
		t.Fatalf("list view has %d lines, want selected title and description rows:\n%s", len(lines), m.View())
	}
	assertFullWidthStyledLine(t, "selected blank description", lines[titleIndex+1], m.list.Width(), common.styles.palette.SelectionBg)
}

func TestListApplyStylesUpdatesExistingList(t *testing.T) {
	common := testCommon()
	common.styles = newStyles(true)
	m := newList(common, hostTarget(t, "@tilde.team"), []User{{Login: "alrs", Name: "Alvaro"}})

	m.applyStyles(newStyles(false))
	if !sameColor(m.list.Styles.Filter.Focused.Prompt.GetForeground(), newStyles(false).input.Focused.Prompt.GetForeground()) {
		t.Fatal("applyStyles should update list filter prompt")
	}
	if !strings.Contains(m.View(), "\x1b[38;2;168;31;98") {
		t.Fatalf("applyStyles should update selected row render:\n%s", m.View())
	}
}

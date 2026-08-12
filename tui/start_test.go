package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

// testCommon is shared with list_test.go; do not redeclare it here.

func twoSections() []startSection {
	return []startSection{
		{title: "BOOKMARKS", entries: []startEntry{
			{target: "@tilde.team", kind: kindCommunity, note: "Small public access unix", source: sourceBookmark},
		}},
		{title: "COMMUNITIES", entries: []startEntry{
			{target: "@plan.cat", kind: kindCommunity, note: "Classic finger, polished for the present", source: sourceCatalog},
			{target: "@happynetbox.com", kind: kindCommunity, note: "Finger server of user profiles, run by Ben Brown", source: sourceCatalog},
		}},
	}
}

// The cursor must never rest on a header, including at construction.
func TestStartSelectionSkipsLeadingHeader(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	got, ok := m.selected()
	if !ok {
		t.Fatal("selected() ok = false, want an entry")
	}
	if got.target != "@tilde.team" {
		t.Fatalf("selected = %q, want @tilde.team", got.target)
	}
}

func TestStartSelectTargetPreservesIdentity(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	if !m.selectTarget("@happynetbox.com") {
		t.Fatal("selectTarget returned false for an existing row")
	}
	got, ok := m.selected()
	if !ok || got.target != "@happynetbox.com" {
		t.Fatalf("selected = %+v, %v; want @happynetbox.com", got, ok)
	}
}

// Moving down out of one section must land on the next section's first entry,
// stepping over its header rather than selecting it.
func TestStartCursorStepsOverInteriorHeader(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	got, ok := m.selected()
	if !ok {
		t.Fatal("selected() ok = false")
	}
	if got.target != "@plan.cat" {
		t.Fatalf("selected = %q, want @plan.cat", got.target)
	}
}

// Moving up must step over the header in the other direction too.
func TestStartCursorStepsOverHeaderUpward(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m, _ = m.update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	got, _ := m.selected()
	if got.target != "@tilde.team" {
		t.Fatalf("selected = %q, want @tilde.team", got.target)
	}
}

func TestStartCursorSkipsHeaderAtPageBoundary(t *testing.T) {
	common := testCommon()
	common.height = 8 // force pagination with the two-row delegate
	m := newStart(common, twoSections(), "", "")
	m.list.Select(2) // the COMMUNITIES header, on a later page
	m.skipNonEntry(1)
	got, ok := m.selected()
	if !ok || got.target != "@plan.cat" {
		t.Fatalf("selected = %+v, %v; want @plan.cat after boundary header", got, ok)
	}
}

func TestStartHasNoCatalogCreditRow(t *testing.T) {
	tests := []struct {
		name     string
		sections []startSection
	}{
		{name: "catalog on with bookmark", sections: twoSections()},
		{name: "catalog on without bookmarks", sections: twoSections()[1:]},
		{name: "catalog off with borrowed catalog note", sections: []startSection{{
			title: "BOOKMARKS",
			entries: []startEntry{{
				target: "@tilde.team",
				note:   twoSections()[1].entries[0].note,
				source: sourceBookmark,
			}},
		}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newStart(testCommon(), tt.sections, "", "")
			items := m.list.Items()
			if len(items) == 0 {
				t.Fatal("startpage has no items")
			}
			last, ok := items[len(items)-1].(startItem)
			if !ok || !last.selectable() {
				t.Fatalf("last item = %#v, want a selectable entry", items[len(items)-1])
			}
			for _, item := range items {
				row, ok := item.(startItem)
				if !ok {
					t.Fatalf("item = %#v, want startItem", item)
				}
				if !row.selectable() && row.header == "" {
					t.Fatalf("non-selectable item = %#v, want a header", row)
				}
			}
			if strings.Contains(stripANSIForLandingTest(m.View()), "Catalog inspired by") {
				t.Fatalf("startpage still renders catalog attribution:\n%s", m.View())
			}
		})
	}
}

func TestStartFilterSelectsFirstMatchAfterHeadersDisappear(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	var cmd tea.Cmd
	for _, r := range "plan" {
		m, cmd = m.update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	msg, ok := findFilterMatches(cmd)
	if !ok {
		t.Fatal("filter command produced no list.FilterMatchesMsg")
	}
	m, _ = m.update(msg)
	got, ok := m.selected()
	if !ok || got.target != "@plan.cat" {
		t.Fatalf("selected = %+v, %v; want first filtered row @plan.cat", got, ok)
	}
}

func findFilterMatches(cmd tea.Cmd) (tea.Msg, bool) {
	if cmd == nil {
		return nil, false
	}
	msg := cmd()
	if _, ok := msg.(list.FilterMatchesMsg); ok {
		return msg, true
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, child := range batch {
			if msg, ok := findFilterMatches(child); ok {
				return msg, true
			}
		}
	}
	return nil, false
}

func TestStartEmptyStateHasNoSelection(t *testing.T) {
	m := newStart(testCommon(), nil, "", "No bookmarks yet.")
	if _, ok := m.selected(); ok {
		t.Fatal("selected() ok = true on an empty startpage")
	}
	if got := m.View(); !strings.Contains(got, "No bookmarks yet.") {
		t.Fatalf("View() = %q, want the empty state to explain itself", got)
	}
}

// A filter matching nothing empties the VISIBLE set, not the list. Showing the
// file-level empty state there would claim something false and hide the filter
// input mid-keystroke.
func TestStartZeroMatchFilterKeepsTheList(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "No bookmarks yet. The catalog is off.")
	// Drive the filter the way a user does. list.SetFilterText would leave the
	// state at FilterApplied, where bubbles hides the input by design; the case
	// this guards is the input still being visible mid-keystroke.
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	var cmd tea.Cmd
	for _, r := range "zzzznomatch" {
		m, cmd = m.update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if msg, ok := findFilterMatches(cmd); ok {
		m, _ = m.update(msg)
	}

	// Mid-keystroke: the filter input stays visible. bubbles deliberately draws
	// no body while filtering with zero matches (list.go populatedView), so the
	// point here is the file-level empty state staying away.
	got := m.View()
	if strings.Contains(got, "No bookmarks yet.") {
		t.Fatalf("zero-match filter showed the file-level empty state:\n%s", got)
	}
	if !strings.Contains(got, "zzzznomatch") {
		t.Errorf("View() missing the filter input mid-keystroke:\n%s", got)
	}

	// Enter on a zero-match filter clears it (bubbles' own behaviour), restoring
	// the full list — still never the file-level empty state, and never leaving
	// the cursor stranded on a section header.
	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got = m.View()
	if strings.Contains(got, "No bookmarks yet.") {
		t.Fatalf("cleared zero-match filter showed the file-level empty state:\n%s", got)
	}
	if !strings.Contains(got, "@plan.cat") {
		t.Errorf("View() lost the list after the filter cleared:\n%s", got)
	}
	if _, ok := m.selected(); !ok {
		t.Error("cursor is not on a selectable row after the filter cleared")
	}
}

// "No entries." is the list's own empty line, shown when the startpage has rows
// configured but none to draw. It must never be replaced by the file-level
// empty state, which would assert something false.
func TestStartNoItemsLineIsTheListsOwnNoun(t *testing.T) {
	m := newStart(testCommon(), nil, "", "")
	if got := m.View(); !strings.Contains(got, "No entries.") {
		t.Fatalf("View() = %q, want the list's own no-match noun", got)
	}
}

func TestStartViewShowsNoticeAndHeaders(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "2 unreadable lines in ~/.config/lookit/bookmarks", "")
	got := m.View()
	for _, want := range []string{"unreadable lines", "BOOKMARKS", "COMMUNITIES", "@tilde.team"} {
		if !strings.Contains(got, want) {
			t.Errorf("View() missing %q:\n%s", want, got)
		}
	}
}

func TestStartFilterStateReporters(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	if m.filtering() || m.filterApplied() {
		t.Fatal("a fresh startpage reports a filter")
	}
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.filtering() {
		t.Error("filtering() = false while typing a filter")
	}
	var cmd tea.Cmd
	for _, r := range "plan" {
		m, cmd = m.update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if msg, ok := findFilterMatches(cmd); ok {
		m, _ = m.update(msg)
	}
	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.filtering() {
		t.Error("filtering() = true after the filter was accepted")
	}
	if !m.filterApplied() {
		t.Error("filterApplied() = false after the filter was accepted")
	}
}

// applyStyles must reinstate the startpage delegate: applyListStyles ends by
// installing the plain user delegate, which would stop drawing section headers.
func TestStartApplyStylesKeepsSectionHeaders(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	m.applyStyles(newStyles(false))
	if got := m.View(); !strings.Contains(got, "COMMUNITIES") {
		t.Fatalf("View() lost its section headers after applyStyles:\n%s", got)
	}
}

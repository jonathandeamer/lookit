package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// testCommon is shared with list_test.go; do not redeclare it here.

func twoSections() []startSection {
	return []startSection{
		{id: sectionBookmarks, title: "BOOKMARKS", entries: []startEntry{
			{target: "@tilde.team", kind: kindCommunity, note: "Small public access unix", source: sourceBookmark},
		}},
		{id: sectionCommunities, title: "COMMUNITIES", entries: []startEntry{
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
	common.width = 40
	common.height = 8 // force pagination at a width that uses the two-row delegate
	m := newStart(common, twoSections(), "", "")
	m.list.Select(2) // the COMMUNITIES header, on a later page
	m.skipNonEntry(1)
	got, ok := m.selected()
	if !ok || got.target != "@plan.cat" {
		t.Fatalf("selected = %+v, %v; want @plan.cat after boundary header", got, ok)
	}
}

func TestStartDelegateResponsiveHeight(t *testing.T) {
	common := testCommon()
	common.width = startWideMinWidth
	wide := newStart(common, twoSections(), "", "")
	if got := wide.list.Paginator.PerPage; got < 2 {
		t.Fatalf("wide PerPage = %d, want multiple one-row items", got)
	}
	if got := newStartDelegate(common, common.ensureStyles()).Height(); got != 1 {
		t.Fatalf("wide delegate height = %d, want 1", got)
	}

	common.width = startWideMinWidth - 1
	narrow := newStart(common, twoSections(), "", "")
	if got := newStartDelegate(common, common.ensureStyles()).Height(); got != 2 {
		t.Fatalf("narrow delegate height = %d, want 2", got)
	}
	if narrow.list.Paginator.PerPage >= wide.list.Paginator.PerPage {
		t.Fatalf("narrow PerPage = %d, wide = %d", narrow.list.Paginator.PerPage, wide.list.Paginator.PerPage)
	}
}

func TestStartWideRowKeepsLongestCatalogTarget(t *testing.T) {
	common := testCommon()
	common.width = 80
	sections := []startSection{{
		id: sectionServices, title: "SERVICES",
		entries: []startEntry{{
			target: "wordsearch:today@bbs.airandwave.net",
			note:   "Daily word search puzzle", source: sourceCatalog,
		}},
	}}
	m := newStart(common, sections, "", "")
	line := lineContaining(t, stripANSIForLandingTest(m.View()), "wordsearch:today@bbs.airandwave.net")
	if !strings.Contains(line, "wordsearch:today@bbs.airandwave.net") {
		t.Fatalf("target truncated at 80 columns: %q", line)
	}
}

func TestStartDelegateWidthVariants(t *testing.T) {
	tests := []struct {
		width int
		wide  bool
	}{
		{width: 80, wide: true},
		{width: 72, wide: true},
		{width: 71, wide: false},
		{width: 24, wide: false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("width_%d", tt.width), func(t *testing.T) {
			common := testCommon()
			common.width = tt.width
			m := newStart(common, twoSections(), "", "")
			view := m.View()
			plain := stripANSIForLandingTest(view)

			for i, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > m.list.Width() {
					t.Fatalf("line %d width = %d, list width = %d:\n%q", i, got, m.list.Width(), line)
				}
			}

			targetLine := lineIndexContaining(t, plain, "@tilde.team")
			noteLine := lineIndexContaining(t, plain, "Small public")
			if tt.wide && targetLine != noteLine {
				t.Fatalf("wide target line = %d, note line = %d; want one physical row:\n%s", targetLine, noteLine, plain)
			}
			if !tt.wide && noteLine != targetLine+1 {
				t.Fatalf("narrow target line = %d, note line = %d; want stacked rows:\n%s", targetLine, noteLine, plain)
			}

			if tt.wide {
				assertFullWidthStyledLine(t, "wide selected shelf", lineContaining(t, view, "@tilde.team"), m.list.Width(), common.styles.palette.SelectionBg)
			}

			d := newStartDelegate(common, common.ensureStyles())
			var header strings.Builder
			d.Render(&header, m.list, 0, m.list.Items()[0])
			if got := len(strings.Split(header.String(), "\n")); got != d.Height() {
				t.Fatalf("header rows = %d, delegate height = %d: %q", got, d.Height(), header.String())
			}
		})
	}
}

func TestStartDelegatePreservesFilterMatches(t *testing.T) {
	for _, query := range []string{"tilde", "public"} {
		t.Run(query, func(t *testing.T) {
			common := testCommon()
			common.width = 80
			m := newStart(common, twoSections(), "", "")
			m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
			var cmd tea.Cmd
			for _, r := range query {
				m, cmd = m.update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			msg, ok := findFilterMatches(cmd)
			if !ok {
				t.Fatal("filter command produced no list.FilterMatchesMsg")
			}
			m, _ = m.update(msg)

			view := m.View()
			plain := stripANSIForLandingTest(view)
			lineIndex := lineIndexContaining(t, plain, "@tilde.team")
			line := strings.Split(view, "\n")[lineIndex]
			if !strings.Contains(line, "\x1b[4") {
				t.Fatalf("matched %q runes are not underlined:\n%q", query, line)
			}
			if strings.Contains(line, backgroundSequence(common.styles.palette.SelectionBg)) {
				t.Fatalf("filtering row has a selected shelf:\n%q", line)
			}
		})
	}
}

func TestStartDelegateDropsMatchBeyondTruncation(t *testing.T) {
	st := testCommon().ensureStyles()
	got := renderStartField("abcdef", 3, []int{2}, st.listItem.NormalTitle, st.listItem.FilterMatch)
	if plain := stripANSIForLandingTest(got); plain != "ab…" {
		t.Fatalf("truncated field = %q, want %q", plain, "ab…")
	}
	if strings.Contains(got, "\x1b[4") {
		t.Fatalf("discarded match styled the ellipsis: %q", got)
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

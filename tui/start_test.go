package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

func threeSections() []startSection {
	return []startSection{
		{id: sectionBookmarks, title: "BOOKMARKS", entries: []startEntry{
			{target: "@tilde.team", source: sourceBookmark},
		}},
		{id: sectionCommunities, title: "COMMUNITIES", entries: []startEntry{
			{target: "@plan.cat", kind: kindCommunity, source: sourceCatalog},
			{target: "@happynetbox.com", kind: kindCommunity, source: sourceCatalog},
		}},
		{id: sectionServices, title: "SERVICES", entries: []startEntry{
			{target: "quake@bbs.airandwave.net", kind: kindService, source: sourceCatalog},
			{target: "dict@bbs.airandwave.net", kind: kindService, source: sourceCatalog},
		}},
	}
}

func TestStartOverviewWideCountsAssembledRows(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")
	got := stripANSIForLandingTest(m.overviewView())
	want := "BOOKMARKS  1 │ CATALOG  2 communities · 2 services"
	if got != want {
		t.Fatalf("overview = %q, want %q", got, want)
	}
	assertStartOverviewFits(t, m)
}

func TestStartOverviewNarrowStacksOwnershipAndCatalog(t *testing.T) {
	common := testCommon()
	common.width = 40
	m := newStart(common, threeSections(), "", "")
	got := strings.Split(stripANSIForLandingTest(m.overviewView()), "\n")
	want := []string{"BOOKMARKS  1", "CATALOG    2 communities · 2 services"}
	if !slices.Equal(got, want) {
		t.Fatalf("overview = %#v, want %#v", got, want)
	}
	assertStartOverviewFits(t, m)
}

func TestStartOverviewUnfilteredGroupRules(t *testing.T) {
	tests := []struct {
		name     string
		sections []startSection
		empty    string
		want     string
		noHeader bool
	}{
		{
			name:     "no bookmarks",
			sections: threeSections()[1:],
			want:     "BOOKMARKS  none yet │ CATALOG  2 communities · 2 services",
			noHeader: true,
		},
		{
			name:     "catalog off",
			sections: threeSections()[:1],
			want:     "BOOKMARKS  1",
		},
		{
			name:  "file empty",
			empty: "No bookmarks yet. The catalog is off.",
			want:  "",
		},
		{
			name: "singular catalog labels",
			sections: []startSection{
				{id: sectionCommunities, title: "COMMUNITIES", entries: []startEntry{{target: "@plan.cat", kind: kindCommunity}}},
				{id: sectionServices, title: "SERVICES", entries: []startEntry{{target: "date@example.com", kind: kindService}}},
			},
			want: "BOOKMARKS  none yet │ CATALOG  1 community · 1 service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			common := testCommon()
			common.width = 100
			m := newStart(common, tt.sections, "", tt.empty)
			if got := stripANSIForLandingTest(m.overviewView()); got != tt.want {
				t.Fatalf("overview = %q, want %q", got, tt.want)
			}
			if tt.noHeader && strings.Contains(stripANSIForLandingTest(m.list.View()), "BOOKMARKS") {
				t.Fatal("catalog-only startpage gained an empty bookmark section")
			}
			assertStartOverviewFits(t, m)
		})
	}
}

func TestStartOverviewPinnedCatalogRowsMoveToBookmarks(t *testing.T) {
	catalog := []startEntry{
		{target: "@plan.cat", kind: kindCommunity, source: sourceCatalog},
		{target: "@tilde.team", kind: kindCommunity, source: sourceCatalog},
		{target: "date@example.com", kind: kindService, source: sourceCatalog},
	}
	sections := buildSections(catalog, bookmarkFile{targets: []string{"@plan.cat"}})
	common := testCommon()
	common.width = 80
	m := newStart(common, sections, "", "")
	got := stripANSIForLandingTest(m.overviewView())
	want := "BOOKMARKS  1 │ CATALOG  1 community · 1 service"
	if got != want {
		t.Fatalf("overview = %q, want %q", got, want)
	}
	counts := m.overviewCounts()
	if counts.bookmarks+counts.communities+counts.services != 3 {
		t.Fatalf("overview duplicated a pinned catalog row: %#v", counts)
	}
}

func TestStartOverviewAppliedFilterUsesOnlyMatchingGroups(t *testing.T) {
	sections := threeSections()
	sections[0].entries[0].note = "shared-match"
	sections[2].entries[0].note = "shared-match"
	common := testCommon()
	common.width = 80
	m := newStart(common, sections, "", "")
	m.list.SetFilterText("shared-match")

	got := stripANSIForLandingTest(m.overviewView())
	want := "BOOKMARKS  1 │ CATALOG  1 service"
	if got != want {
		t.Fatalf("filtered overview = %q, want %q", got, want)
	}
	counts := m.overviewCounts()
	if total := counts.bookmarks + counts.communities + counts.services; total != 2 {
		t.Fatalf("filtered overview total = %d, want 2", total)
	}
	app := appModel{start: m}
	if got := app.startBar(80, common.styles).meta; got != "2 entries" {
		t.Fatalf("filtered start bar meta = %q, want %q", got, "2 entries")
	}
	plainList := stripANSIForLandingTest(m.list.View())
	bookmarkLine := lineContaining(t, plainList, "@tilde.team")
	if strings.Contains(bookmarkLine, "◆") || strings.Contains(bookmarkLine, "BOOKMARK ") {
		t.Fatalf("filtered bookmark row gained an ownership prefix: %q", bookmarkLine)
	}
	assertStartOverviewFits(t, m)

	m.list.ResetFilter()
	if got := stripANSIForLandingTest(m.overviewView()); got != "BOOKMARKS  1 │ CATALOG  2 communities · 2 services" {
		t.Fatalf("cleared overview = %q", got)
	}
}

func TestStartOverviewHidesWhileFilteringAndWithZeroAppliedMatches(t *testing.T) {
	m := newStart(testCommon(), threeSections(), "", "")
	m.list.SetFilterState(list.Filtering)
	if got := m.overviewView(); got != "" || m.overviewHeight() != 0 {
		t.Fatalf("filtering overview = %q, height %d; want hidden", got, m.overviewHeight())
	}
	m.list.SetFilterText("zzzz-no-match")
	if got := m.overviewView(); got != "" || m.overviewHeight() != 0 {
		t.Fatalf("zero-match applied overview = %q, height %d; want hidden", got, m.overviewHeight())
	}
}

func TestStartOverviewHighlightsSelectedSectionOnly(t *testing.T) {
	common := testCommon()
	common.width = 80
	common.contentFocused = true
	m := newStart(common, threeSections(), "", "")

	tests := []struct {
		name   string
		index  int
		gold   string
		others []string
	}{
		{name: "bookmark", index: 1, gold: "BOOKMARKS", others: []string{"2 communities", "2 services"}},
		{name: "community", index: 3, gold: "2 communities", others: []string{"BOOKMARKS", "2 services"}},
		{name: "service", index: 6, gold: "2 services", others: []string{"BOOKMARKS", "2 communities"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.list.Select(tt.index)
			assertStartOverviewGoldSegment(t, m.overviewView(), common, tt.gold, tt.others...)
		})
	}
}

func TestStartOverviewHighlightSurvivesContinuationPage(t *testing.T) {
	common := testCommon()
	common.width = 40
	common.height = 10
	common.contentFocused = true
	sections := []startSection{{id: sectionCommunities, title: "COMMUNITIES"}}
	for i := range 8 {
		sections[0].entries = append(sections[0].entries, startEntry{target: fmt.Sprintf("user-%d@example.com", i), kind: kindCommunity})
	}
	m := newStart(common, sections, "", "")
	m.list.Select(len(m.list.Items()) - 1)
	if m.list.Paginator.Page == 0 {
		t.Fatal("test setup did not select a continuation page")
	}
	if strings.Contains(stripANSIForLandingTest(m.list.View()), "COMMUNITIES") {
		t.Fatal("continuation page unexpectedly retained the inline section header")
	}
	assertStartOverviewGoldSegment(t, m.overviewView(), common, "8 communities", "BOOKMARKS")
}

func TestStartOverviewFilteredBookmarkKeepsOwnershipHighlight(t *testing.T) {
	sections := threeSections()
	sections[0].entries[0].note = "flat-match"
	sections[2].entries[0].note = "flat-match"
	common := testCommon()
	common.width = 80
	common.contentFocused = true
	m := newStart(common, sections, "", "")
	m.list.SetFilterText("flat-match")
	m.list.Select(0)
	if strings.Contains(stripANSIForLandingTest(m.list.View()), "BOOKMARKS") {
		t.Fatal("flat filtered results unexpectedly retained the inline bookmark header")
	}
	assertStartOverviewGoldSegment(t, m.overviewView(), common, "BOOKMARKS", "1 service")
}

func TestStartOverviewInputFocusHasNoSelectedSegment(t *testing.T) {
	common := testCommon()
	common.width = 80
	common.contentFocused = false
	m := newStart(common, threeSections(), "", "")
	view := m.overviewView()
	if count := strings.Count(view, overviewGoldSequence(common)); count != 0 {
		t.Fatalf("input-focused overview contains %d gold foreground sequences: %q", count, view)
	}
	if count := countSGRParam(view, "1"); count != 0 {
		t.Fatalf("input-focused overview contains %d bold sequences: %q", count, view)
	}
}

func TestStartSizingIncludesNoticeAndOverview(t *testing.T) {
	common := testCommon()
	common.width = 40
	m := newStart(common, threeSections(), "first warning\nsecond warning", "")
	m.setSize(40, 20)
	want := 20 - startChromeRows - m.noticeHeight() - m.overviewHeight()
	if got := m.list.Height(); got != want {
		t.Fatalf("list height = %d, want %d (body minus chrome, notice, and overview)", got, want)
	}
	plain := stripANSIForLandingTest(m.View())
	wantPrefix := "first warning\nsecond warning\n\nBOOKMARKS  1\nCATALOG    2 communities · 2 services\n"
	if !strings.HasPrefix(plain, wantPrefix) {
		t.Fatalf("view order =\n%q\nwant prefix\n%q", plain, wantPrefix)
	}
}

func assertStartOverviewFits(t *testing.T, m startModel) {
	t.Helper()
	for i, line := range strings.Split(m.overviewView(), "\n") {
		if got := ansi.StringWidth(line); got > m.list.Width() {
			t.Fatalf("overview line %d width = %d, list width = %d: %q", i, got, m.list.Width(), line)
		}
	}
}

func assertStartOverviewGoldSegment(t *testing.T, view string, common *commonModel, gold string, others ...string) {
	t.Helper()
	style := lipgloss.NewStyle().Foreground(common.styles.palette.AccentGold).Bold(true)
	if want := style.Render(gold); !strings.Contains(view, want) {
		t.Fatalf("overview does not highlight %q with gold+bold: %q", gold, view)
	}
	if count := strings.Count(view, overviewGoldSequence(common)); count != 1 {
		t.Fatalf("overview gold foreground sequence count = %d, want exactly 1: %q", count, view)
	}
	if count := countSGRParam(view, "1"); count != 1 {
		t.Fatalf("overview bold sequence count = %d, want exactly 1: %q", count, view)
	}
	for _, other := range others {
		if forbidden := style.Render(other); strings.Contains(view, forbidden) {
			t.Fatalf("overview also highlights %q with gold+bold: %q", other, view)
		}
	}
}

func overviewGoldSequence(common *commonModel) string {
	r, g, b, _ := common.styles.palette.AccentGold.RGBA()
	return fmt.Sprintf("38;2;%d;%d;%d", r>>8, g>>8, b>>8)
}

func countSGRParam(s, want string) int {
	count := 0
	for {
		start := strings.Index(s, "\x1b[")
		if start < 0 {
			return count
		}
		s = s[start+2:]
		end := strings.IndexByte(s, 'm')
		if end < 0 {
			return count
		}
		for _, param := range strings.Split(s[:end], ";") {
			if param == want {
				count++
			}
		}
		s = s[end+1:]
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

func TestStartSelectionShelfFollowsContentFocus(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, twoSections(), "", "")

	common.contentFocused = true
	active := lineContaining(t, m.View(), "@tilde.team")
	assertFullWidthStyledLine(t, "active start selection", active, m.list.Width(), common.styles.palette.SelectionBg)

	common.contentFocused = false
	inactive := lineContaining(t, m.View(), "@tilde.team")
	assertFullWidthStyledLine(t, "inactive start selection", inactive, m.list.Width(), common.styles.palette.SubtleBg)
}

func TestStartFilterTransitionsReclaimOverviewHeight(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T, startModel, int)
	}{
		{
			name: "enter and apply",
			run: func(t *testing.T, m startModel, unfilteredHeight int) {
				m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
				if got, want := m.list.Height(), unfilteredHeight+2; got != want {
					t.Fatalf("filtering list height = %d, want %d", got, want)
				}

				m = typeStartFilter(t, m, "plan")
				m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
				wantHeight := m.common.bodyHeight() - startChromeRows - m.noticeHeight() - m.overviewHeight()
				if m.list.FilterState() != list.FilterApplied || m.list.Height() != wantHeight {
					t.Fatalf("applied state=%v height=%d, want applied/%d", m.list.FilterState(), m.list.Height(), wantHeight)
				}
			},
		},
		{
			name: "cancel live filter",
			run: func(t *testing.T, m startModel, unfilteredHeight int) {
				m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
				m = typeStartFilter(t, m, "plan")
				m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEsc})
				assertStartFilterRestored(t, m, unfilteredHeight)
			},
		},
		{
			name: "clear applied filter",
			run: func(t *testing.T, m startModel, unfilteredHeight int) {
				m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
				m = typeStartFilter(t, m, "plan")
				m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
				m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEsc})
				assertStartFilterRestored(t, m, unfilteredHeight)
			},
		},
		{
			name: "accept zero matches",
			run: func(t *testing.T, m startModel, unfilteredHeight int) {
				m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
				m = typeStartFilter(t, m, "zzzz-no-match")
				m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
				assertStartFilterRestored(t, m, unfilteredHeight)
				if _, ok := m.selected(); !ok {
					t.Fatal("zero-match clear did not leave a selectable row")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			common := testCommon()
			common.width = 40
			m := newStart(common, threeSections(), "", "")
			if got := m.overviewHeight(); got != 2 {
				t.Fatalf("narrow overview height = %d, want 2", got)
			}
			tt.run(t, m, m.list.Height())
		})
	}
}

func assertStartFilterRestored(t *testing.T, m startModel, wantHeight int) {
	t.Helper()
	if m.list.FilterState() != list.Unfiltered || m.list.FilterValue() != "" || m.list.Height() != wantHeight {
		t.Fatalf("restored state=%v filter=%q height=%d, want unfiltered/empty/%d", m.list.FilterState(), m.list.FilterValue(), m.list.Height(), wantHeight)
	}
}

func TestStartAppliedFilterSurvivesResponsiveResize(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = typeStartFilter(t, m, "plan")
	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEnter})

	wantFilter := m.list.FilterValue()
	wantCounts := m.overviewCounts()
	wantSelected, ok := m.selected()
	if !ok {
		t.Fatal("applied filter has no selected entry")
	}
	widePerPage := m.list.Paginator.PerPage
	if got := newStartDelegate(common, common.ensureStyles()).Height(); got != 1 {
		t.Fatalf("wide delegate height = %d, want 1", got)
	}

	m.setSize(71, common.bodyHeight())
	assertStartAppliedFilterState(t, m, wantFilter, wantCounts, wantSelected.target)
	if got := newStartDelegate(common, common.ensureStyles()).Height(); got != 2 {
		t.Fatalf("narrow delegate height = %d, want 2", got)
	}
	if got := m.list.Paginator.PerPage; got >= widePerPage {
		t.Fatalf("narrow PerPage = %d, want less than wide %d", got, widePerPage)
	}

	m.setSize(72, common.bodyHeight())
	assertStartAppliedFilterState(t, m, wantFilter, wantCounts, wantSelected.target)
	if got := newStartDelegate(common, common.ensureStyles()).Height(); got != 1 {
		t.Fatalf("restored wide delegate height = %d, want 1", got)
	}
	if got := m.list.Paginator.PerPage; got != widePerPage {
		t.Fatalf("restored wide PerPage = %d, want %d", got, widePerPage)
	}
}

func assertStartAppliedFilterState(t *testing.T, m startModel, wantFilter string, wantCounts startOverviewCounts, wantTarget string) {
	t.Helper()
	if m.list.FilterState() != list.FilterApplied || m.list.FilterValue() != wantFilter {
		t.Fatalf("filter state=%v value=%q, want applied/%q", m.list.FilterState(), m.list.FilterValue(), wantFilter)
	}
	if got := m.overviewCounts(); got != wantCounts {
		t.Fatalf("filtered counts = %#v, want %#v", got, wantCounts)
	}
	selected, ok := m.selected()
	if !ok || selected.target != wantTarget {
		t.Fatalf("selected = %+v, %v; want %q", selected, ok, wantTarget)
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
			common.contentFocused = true
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

func TestStartDelegateMatchesUTF8NoteByByteOffset(t *testing.T) {
	common := testCommon()
	common.width = 80
	sections := []startSection{{
		id: sectionServices, title: "SERVICES",
		entries: []startEntry{{
			target: "date@example.com",
			note:   "Today’s date, across the years",
			source: sourceCatalog,
		}},
	}}
	m := newStart(common, sections, "", "")
	m.list.SetFilterText("years")
	m.list.SetFilterState(list.Filtering)

	view := m.View()
	plain := stripANSIForLandingTest(view)
	lineIndex := lineIndexContaining(t, plain, "date@example.com")
	line := strings.Split(view, "\n")[lineIndex]
	if got := underlinedText(line); got != "years" {
		t.Fatalf("underlined text = %q, want %q\n%q", got, "years", line)
	}
}

func underlinedText(s string) string {
	var out strings.Builder
	underlined := false
	for len(s) > 0 {
		if strings.HasPrefix(s, "\x1b[") {
			end := strings.IndexByte(s, 'm')
			if end >= 0 {
				params := strings.Split(s[2:end], ";")
				if len(params) == 1 && params[0] == "" {
					underlined = false
				}
				for _, param := range params {
					switch param {
					case "0", "24":
						underlined = false
					case "4":
						underlined = true
					}
				}
				s = s[end+1:]
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s)
		if underlined {
			out.WriteRune(r)
		}
		s = s[size:]
	}
	return out.String()
}

func TestStartDelegateFrameStarvedWidthsNeverOverflow(t *testing.T) {
	for _, width := range []int{1, 2} {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			common := testCommon()
			common.width = width

			t.Run("unselected", func(t *testing.T) {
				m := newStart(common, twoSections(), "", "")
				assertStartDelegateItemFits(t, m, 3, m.list.Items()[3])
			})

			t.Run("filtering_empty", func(t *testing.T) {
				m := newStart(common, twoSections(), "", "")
				m.list.SetFilterState(list.Filtering)
				assertStartDelegateItemFits(t, m, 3, m.list.Items()[3])
			})

			t.Run("filtering_text", func(t *testing.T) {
				m := newStart(common, twoSections(), "", "")
				m.list.SetFilterText("plan")
				m.list.SetFilterState(list.Filtering)
				assertStartDelegateItemFits(t, m, 0, m.list.VisibleItems()[0])
			})
		})
	}
}

func assertStartDelegateItemFits(t *testing.T, m startModel, index int, item list.Item) {
	t.Helper()
	var rendered strings.Builder
	newStartDelegate(m.common, m.common.ensureStyles()).Render(&rendered, m.list, index, item)
	for lineIndex, line := range strings.Split(rendered.String(), "\n") {
		if got := lipgloss.Width(line); got > m.list.Width() {
			t.Fatalf("line %d width = %d, list width = %d: %q", lineIndex, got, m.list.Width(), line)
		}
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

func typeStartFilter(t *testing.T, m startModel, filter string) startModel {
	t.Helper()
	for _, r := range filter {
		var cmd tea.Cmd
		m, cmd = m.update(tea.KeyPressMsg{Code: r, Text: string(r)})
		msg, ok := findFilterMatches(cmd)
		if !ok {
			t.Fatalf("typing %q produced no list.FilterMatchesMsg", r)
		}
		m, _ = m.update(msg)
	}
	return m
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

func TestStartRowTargetShortensChildrenUnlessFiltered(t *testing.T) {
	child := startEntry{target: "dict@bbs.airandwave.net", child: true}
	if got, want := startRowTarget(child, false), "  dict"; got != want {
		t.Errorf("unfiltered child = %q, want %q", got, want)
	}
	// Filtering removes the parent that supplies the host, so the row must
	// carry its full address again.
	if got, want := startRowTarget(child, true), "dict@bbs.airandwave.net"; got != want {
		t.Errorf("filtered child = %q, want %q", got, want)
	}
	parent := startEntry{target: "@bbs.airandwave.net"}
	if got, want := startRowTarget(parent, false), "@bbs.airandwave.net"; got != want {
		t.Errorf("parent = %q, want %q", got, want)
	}
}

// In the narrow two-line layout the note sits under the target, so an
// unindented note would hang left of its own row.
func TestNarrowChildRowIndentsBothLines(t *testing.T) {
	common := testCommon()
	common.width = 40
	st := common.ensureStyles()
	d := newStartDelegate(common, st)
	if d.Height() != 2 {
		t.Fatalf("delegate height = %d at width 40, want the two-line layout", d.Height())
	}
	l := list.New([]list.Item{
		startItem{entry: startEntry{target: "dict@bbs.airandwave.net", note: "Dictionary lookup", child: true}, section: sectionServices},
		startItem{entry: startEntry{target: "other@example.com", note: "Other row"}, section: sectionServices},
		startItem{entry: startEntry{target: "third@example.com", note: "Third row"}, section: sectionServices},
	}, d, 40, 4)
	// Render both rows unselected so the selection shelf's border and padding do
	// not get mistaken for the child's own two-space indent. The base list style
	// already left-pads every row by two spaces (tui/styles.go NormalTitle/
	// NormalDesc), so asserting a bare "starts with two spaces" would pass even
	// if the child-specific indent were dropped entirely. Compare the child's
	// leading-space count against its non-child sibling's instead, so only the
	// feature under test — the extra indent — can satisfy the assertion.
	l.Select(2) // a third row, so neither rendered row below carries the selection shelf
	var childBuf, siblingBuf strings.Builder
	d.Render(&childBuf, l, 0, l.Items()[0])
	d.Render(&siblingBuf, l, 1, l.Items()[1])
	childLines := strings.Split(ansi.Strip(childBuf.String()), "\n")
	siblingLines := strings.Split(ansi.Strip(siblingBuf.String()), "\n")
	if len(childLines) != len(siblingLines) {
		t.Fatalf("child rendered %d lines, sibling %d; want equal", len(childLines), len(siblingLines))
	}
	for i := range childLines {
		childIndent := len(childLines[i]) - len(strings.TrimLeft(childLines[i], " "))
		siblingIndent := len(siblingLines[i]) - len(strings.TrimLeft(siblingLines[i], " "))
		if got, want := childIndent, siblingIndent+2; got != want {
			t.Errorf("line %d indent = %d, want %d (sibling %d + the child's own 2): child %q, sibling %q",
				i, got, want, siblingIndent, childLines[i], siblingLines[i])
		}
	}
}

func TestFilteredChildRendersFullTargetWithMatchHighlight(t *testing.T) {
	for _, query := range []string{"dict", "airandwave"} {
		t.Run(query, func(t *testing.T) {
			common := testCommon()
			common.width = 80
			sections := []startSection{{
				id: sectionServices, title: "SERVICES",
				entries: []startEntry{{
					target: "dict@bbs.airandwave.net", note: "Dictionary lookup",
					source: sourceCatalog, child: true,
				}},
			}}
			m := newStart(common, sections, "", "")
			m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
			m = typeStartFilter(t, m, query)
			m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEnter})

			view := m.View()
			plain := ansi.Strip(view)
			lineIndex := lineIndexContaining(t, plain, "dict@bbs.airandwave.net")
			line := strings.Split(view, "\n")[lineIndex]
			if got := underlinedText(line); got != query {
				t.Fatalf("underlined text = %q, want %q\n%q", got, query, line)
			}
		})
	}
}

// A structural row duplicates a target listed elsewhere. Filtering drops the
// headers that tell the two copies apart, so the duplicate must drop out too.
func TestStructuralRowsDoNotMatchFilters(t *testing.T) {
	structural := startItem{entry: startEntry{target: "@happynetbox.com", structural: true}}
	if got := structural.FilterValue(); got != "" {
		t.Fatalf("FilterValue() = %q, want empty so the copy is filtered out", got)
	}
	listing := startItem{entry: startEntry{target: "@happynetbox.com", note: "n"}}
	if got, want := listing.FilterValue(), "@happynetbox.com n"; got != want {
		t.Fatalf("FilterValue() = %q, want %q", got, want)
	}
}

// Counts describe displayed listings after bookmark/catalog suppression. A
// structural parent is navigation structure, not another listing, so it must
// not raise either total.
func TestCountsIgnoreStructuralRows(t *testing.T) {
	items := []list.Item{
		startItem{header: "SERVICES", section: sectionServices},
		startItem{entry: startEntry{target: "@happynetbox.com", structural: true}, section: sectionServices},
		startItem{entry: startEntry{target: "bot@happynetbox.com", child: true}, section: sectionServices},
	}
	if got := startCounts(items); got.services != 1 {
		t.Fatalf("services = %d, want 1 — the structural copy is not a listing", got.services)
	}
}

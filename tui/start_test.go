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

func sectionGapSections() []startSection {
	return []startSection{
		{id: sectionCommunities, title: "COMMUNITIES", entries: []startEntry{
			{target: "@plan.cat", kind: kindCommunity, note: "Classic finger, polished for the present", source: sourceCatalog},
		}},
		{id: sectionServices, title: "SERVICES", entries: []startEntry{
			{target: "date@example.com", kind: kindService, note: "Today’s date, across the years", source: sourceCatalog},
		}},
	}
}

// startHeaderLineIndex finds the rendered section header for title. Match the
// header's trailing rule rather than the word alone, so this cannot be fooled
// by the same word appearing in a row or in the overview above.
func startHeaderLineIndex(t *testing.T, lines []string, title string) int {
	t.Helper()
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), title+" ─") {
			return i
		}
	}
	t.Fatalf("no %s header line in:\n%s", title, strings.Join(lines, "\n"))
	return -1
}

// TestStartSectionSpacingIsUniformWhenWide asserts the decision (one blank row
// above every header) rather than the mechanism (how many spacer items exist),
// so it stays honest if the assembly changes again.
func TestStartSectionSpacingIsUniformWhenWide(t *testing.T) {
	common := testCommon()
	common.width = 100
	common.height = 40
	m := newStart(common, threeSections(), "", "")

	plain := stripANSIForLandingTest(m.View())
	lines := strings.Split(plain, "\n")

	for _, title := range []string{"BOOKMARKS", "COMMUNITIES", "SERVICES"} {
		header := startHeaderLineIndex(t, lines, title)
		if header < 2 {
			t.Fatalf("%s header at line %d, want room for a gap above it:\n%s", title, header, plain)
		}
		if got := strings.TrimSpace(lines[header-1]); got != "" {
			t.Errorf("line above %s = %q, want one blank row:\n%s", title, got, plain)
		}
		if got := strings.TrimSpace(lines[header-2]); got == "" {
			t.Errorf("two blank rows above %s, want exactly one:\n%s", title, plain)
		}
	}
}

// TestStartSectionSpacingIsUniformWhenNarrow covers the two-row layout, where
// the gap comes from the header's own first row rather than a spacer item.
//
// Both fixtures matter. threeSections' entries carry no note, so their second
// row renders empty; twoSections' entries fill both rows. Before
// headerNeedsBlank the first case produced two blank rows above a header and
// the second produced one, which made section spacing depend on whether the
// last entry of a section happened to be described.
func TestStartSectionSpacingIsUniformWhenNarrow(t *testing.T) {
	for _, tt := range []struct {
		name     string
		sections []startSection
		titles   []string
	}{
		{"entries without notes", threeSections(), []string{"BOOKMARKS", "COMMUNITIES", "SERVICES"}},
		{"entries with notes", twoSections(), []string{"BOOKMARKS", "COMMUNITIES"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			common := testCommon()
			common.width = 45
			common.height = 40
			m := newStart(common, tt.sections, "", "")

			plain := stripANSIForLandingTest(m.View())
			lines := strings.Split(plain, "\n")

			for _, title := range tt.titles {
				header := startHeaderLineIndex(t, lines, title)
				if header < 2 {
					t.Fatalf("%s header at line %d, want room for a gap above it:\n%s", title, header, plain)
				}
				if got := strings.TrimSpace(lines[header-1]); got != "" {
					t.Errorf("line above %s = %q, want one blank row:\n%s", title, got, plain)
				}
				if got := strings.TrimSpace(lines[header-2]); got == "" {
					t.Errorf("two blank rows above %s, want exactly one:\n%s", title, plain)
				}
			}
		})
	}
}

// TestStartUniformSpacingResizeKeepsFirstRowOfSection crosses the 72-column
// boundary with the selection on a section's first row — the position most
// exposed to a spacer being inserted directly above it. Three sections mean
// two spacers appear and disappear together.
func TestStartUniformSpacingResizeKeepsFirstRowOfSection(t *testing.T) {
	const target = "quake@bbs.airandwave.net" // the first SERVICES row

	common := testCommon()
	common.width = 100
	common.height = 40
	m := newStart(common, threeSections(), "", "")
	if !m.selectTarget(target) {
		t.Fatalf("could not select %s", target)
	}

	m.setSize(71, common.bodyHeight())
	if got, ok := m.selected(); !ok || got.target != target {
		t.Fatalf("after narrowing, selected = %+v, %v; want %s", got, ok, target)
	}

	m.setSize(100, common.bodyHeight())
	if got, ok := m.selected(); !ok || got.target != target {
		t.Fatalf("after widening, selected = %+v, %v; want %s", got, ok, target)
	}
}

func TestStartSingleSectionAssemblesNoSpacer(t *testing.T) {
	sections := []startSection{{
		id: sectionBookmarks, title: "BOOKMARKS",
		entries: []startEntry{{target: "@tilde.team", source: sourceBookmark}},
	}}
	for _, width := range []int{45, 80, 100} {
		items := startItems(sections, width)
		for i, item := range items {
			row, ok := item.(startItem)
			if ok && row.spacer {
				t.Errorf("width %d: item %d is a spacer, want none in a single-section page", width, i)
			}
		}
	}
}

func TestStartItemsBeginsWithTheFirstHeader(t *testing.T) {
	items := startItems(sectionGapSections(), 80)
	if len(items) == 0 {
		t.Fatal("got 0 items, want at least a header")
	}
	first, ok := items[0].(startItem)
	if !ok || first.header == "" {
		t.Fatalf("first item = %+v, want a header: bubbles' reserved filter row supplies the gap above it", items[0])
	}
}

func TestStartSectionGapRendersExactlyOneBlankRow(t *testing.T) {
	for _, tt := range []struct {
		name  string
		width int
	}{
		{name: "wide spacer item", width: 80},
		{name: "narrow header row", width: 40},
	} {
		t.Run(tt.name, func(t *testing.T) {
			common := testCommon()
			common.width = tt.width
			m := newStart(common, sectionGapSections(), "", "")
			lines := strings.Split(stripANSIForLandingTest(m.View()), "\n")
			servicesLine := lineIndexContaining(t, strings.Join(lines, "\n"), "SERVICES")
			if servicesLine < 2 {
				t.Fatalf("SERVICES line = %d, want room for content and a gap:\n%s", servicesLine, strings.Join(lines, "\n"))
			}
			if got := strings.TrimSpace(lines[servicesLine-1]); got != "" {
				t.Fatalf("line before SERVICES = %q, want blank:\n%s", got, strings.Join(lines, "\n"))
			}
			if got := strings.TrimSpace(lines[servicesLine-2]); got == "" {
				t.Fatalf("two blank lines before SERVICES, want exactly one:\n%s", strings.Join(lines, "\n"))
			}
		})
	}
}

func TestStartSpacerItemCounts(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		sections []startSection
		want     int
	}{
		{name: "wide both", width: 80, sections: sectionGapSections(), want: 5},
		{name: "narrow both", width: 40, sections: sectionGapSections(), want: 4},
		{name: "wide communities only", width: 80, sections: sectionGapSections()[:1], want: 2},
		{name: "wide services only", width: 80, sections: sectionGapSections()[1:], want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			common := testCommon()
			common.width = tt.width
			m := newStart(common, tt.sections, "", "")
			if got := len(m.list.Items()); got != tt.want {
				t.Fatalf("item count = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestStartFilterDropsSectionGap(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, sectionGapSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = typeStartFilter(t, m, "date")

	if got := len(m.list.VisibleItems()); got != 1 {
		t.Fatalf("filtered item count = %d, want 1 service match", got)
	}
	plain := stripANSIForLandingTest(m.View())
	if strings.Contains(plain, "COMMUNITIES") || strings.Contains(plain, "SERVICES") {
		t.Fatalf("filtered view retained section chrome:\n%s", plain)
	}
}

// startRowColumns finds the rendered row carrying target and reports the
// columns its target and note begin at. Both come from the same line: a
// selected row draws a shelf that shifts its content right, so comparing a
// column taken from one row with a column taken from another measures the
// shelf as well as the layout.
func startRowColumns(t *testing.T, view, target, note string) (int, int) {
	t.Helper()
	for _, line := range strings.Split(stripANSIForLandingTest(view), "\n") {
		targetColumn := strings.Index(line, target)
		noteColumn := strings.Index(line, note)
		if targetColumn >= 0 && noteColumn >= 0 {
			return targetColumn, noteColumn
		}
	}
	t.Fatalf("no rendered row carries both %q and %q in:\n%s", target, note, stripANSIForLandingTest(view))
	return -1, -1
}

// TestStartNoteColumnTracksTheLongestTarget: the gutter was pinned at half the
// width, so a nine-cell target had roughly forty blank cells before its note
// while the whole composition hung on the left third of the screen. It is
// measured from the longest target on the page instead.
func TestStartNoteColumnTracksTheLongestTarget(t *testing.T) {
	common := testCommon()
	common.width = 100
	longest := "@zaibatsu.circumlunar.space"
	m := newStart(common, []startSection{{id: sectionCommunities, title: "COMMUNITIES", entries: []startEntry{
		{target: "@graph.no", kind: kindCommunity, note: "Weather worldwide by place name"},
		{target: longest, kind: kindCommunity, note: "Sundogs seeking asylum"},
	}}}, "", "")

	targetColumn, noteColumn := startRowColumns(t, m.View(), longest, "Sundogs seeking asylum")
	if got, want := noteColumn-targetColumn, lipgloss.Width(longest)+startTargetColumnGap; got != want {
		t.Fatalf("note column starts %d cells after the target column, want %d (longest target plus its gap)", got, want)
	}
}

// TestStartNoteColumnMeasuresTheRowsAsDrawn: a grouped child draws as a token
// under its connector and only regains its full address once a filter flattens
// the view. Measuring the column against the address it is not currently
// showing would pad the gutter for text that is not on the screen — which is
// the walk this issue is about.
func TestStartNoteColumnMeasuresTheRowsAsDrawn(t *testing.T) {
	common := testCommon()
	common.width = 100
	parent := "@bbs.airandwave.net"
	m := newStart(common, []startSection{{id: sectionServices, title: "SERVICES", entries: []startEntry{
		{target: parent, kind: kindService, note: "Over two dozen services"},
		{target: "dict@bbs.airandwave.net", kind: kindService, note: "Dictionary lookup", child: true, lastChild: true},
	}}}, "", "")

	targetColumn, noteColumn := startRowColumns(t, m.View(), parent, "Over two dozen services")
	if got, want := noteColumn-targetColumn, lipgloss.Width(parent)+startTargetColumnGap; got != want {
		t.Fatalf("note column starts %d cells after the target column, want %d (the widest row as drawn, plus its gap)", got, want)
	}
}

// TestStartNoteColumnNeverTakesMoreThanHalfTheWidth: measuring the column from
// the longest target hands the page's layout to its longest row, so one very
// long address could otherwise push every note off the screen. The old fixed
// half becomes the ceiling.
func TestStartNoteColumnNeverTakesMoreThanHalfTheWidth(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, []startSection{{id: sectionCommunities, title: "COMMUNITIES", entries: []startEntry{
		{target: "@a.example", kind: kindCommunity, note: "a short note"},
		{target: "someone@" + strings.Repeat("long", 15) + ".example", kind: kindCommunity, note: "a very wide row"},
	}}}, "", "")

	targetColumn, noteColumn := startRowColumns(t, m.View(), "@a.example", "a short note")
	if got := noteColumn - targetColumn; got > m.list.Width()/2 {
		t.Fatalf("note column starts %d cells after the target column, want no more than half of %d", got, m.list.Width())
	}
}

func TestStartOverviewWideCountsAssembledRows(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")
	got := stripANSIForLandingTest(m.overviewView())
	want := "YOURS  1 │ CATALOG  2 communities · 2 services"
	if got != want {
		t.Fatalf("overview = %q, want %q", got, want)
	}
	assertStartOverviewFits(t, m)
}

// TestStartOverviewDoesNotRepeatTheSectionHeader: the overview line and the
// BOOKMARKS header sit three lines apart at the very top of the page, where a
// reader is orienting, so the first two things they read must not be the same
// word. The overview names whose the section is; the header names the section.
func TestStartOverviewDoesNotRepeatTheSectionHeader(t *testing.T) {
	for _, width := range []int{100, 80, 45} {
		common := testCommon()
		common.width = width
		m := newStart(common, threeSections(), "", "")
		overview := stripANSIForLandingTest(m.overviewView())
		if strings.Contains(overview, "BOOKMARKS") {
			t.Fatalf("width %d: overview = %q, want it not to repeat the BOOKMARKS header", width, overview)
		}
		if !strings.Contains(stripANSIForLandingTest(m.list.View()), "BOOKMARKS") {
			t.Fatalf("width %d: the BOOKMARKS section header should still be on the page", width)
		}
	}
}

// TestStartOverviewNarrowAlignsValues: stacked, the two labels are different
// lengths, and their values still belong in one column.
func TestStartOverviewNarrowAlignsValues(t *testing.T) {
	common := testCommon()
	common.width = 40
	m := newStart(common, threeSections(), "", "")
	lines := strings.Split(stripANSIForLandingTest(m.overviewView()), "\n")
	if len(lines) != 2 {
		t.Fatalf("overview = %#v, want two stacked lines", lines)
	}
	first, second := strings.Index(lines[0], "1"), strings.Index(lines[1], "2")
	if first != second {
		t.Fatalf("values start at columns %d and %d, want one column:\n%q\n%q", first, second, lines[0], lines[1])
	}
}

func TestStartOverviewNarrowStacksOwnershipAndCatalog(t *testing.T) {
	common := testCommon()
	common.width = 40
	m := newStart(common, threeSections(), "", "")
	got := strings.Split(stripANSIForLandingTest(m.overviewView()), "\n")
	want := []string{"YOURS    1", "CATALOG  2 communities · 2 services"}
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
			want:     "YOURS  none yet │ CATALOG  2 communities · 2 services",
			noHeader: true,
		},
		{
			name:     "catalog off",
			sections: threeSections()[:1],
			want:     "YOURS  1",
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
			want: "YOURS  none yet │ CATALOG  1 community · 1 service",
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
	want := "YOURS  1 │ CATALOG  1 community · 1 service"
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
	want := "YOURS  1 │ CATALOG  1 service"
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
	if got := stripANSIForLandingTest(m.overviewView()); got != "YOURS  1 │ CATALOG  2 communities · 2 services" {
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
		target string
		gold   string
		others []string
	}{
		{name: "bookmark", target: "@tilde.team", gold: "YOURS", others: []string{"2 communities", "2 services"}},
		{name: "community", target: "@plan.cat", gold: "2 communities", others: []string{"YOURS", "2 services"}},
		{name: "service", target: "quake@bbs.airandwave.net", gold: "2 services", others: []string{"YOURS", "2 communities"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !m.selectTarget(tt.target) {
				t.Fatalf("could not select fixture target %q", tt.target)
			}
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
	assertStartOverviewGoldSegment(t, m.overviewView(), common, "8 communities", "YOURS")
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
	assertStartOverviewGoldSegment(t, m.overviewView(), common, "YOURS", "1 service")
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
	wantPrefix := "first warning\nsecond warning\n\nYOURS    1\nCATALOG  2 communities · 2 services\n"
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

func TestStartCursorSkipsSectionGapAndHeader(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, sectionGapSections(), "", "")

	m, _ = m.update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	got, ok := m.selected()
	if !ok || got.target != "date@example.com" {
		t.Fatalf("down selected = %+v, %v; want first service", got, ok)
	}

	m, _ = m.update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	got, ok = m.selected()
	if !ok || got.target != "@plan.cat" {
		t.Fatalf("up selected = %+v, %v; want last community", got, ok)
	}
}

func TestStartCursorSkipsSectionGapAtPageBoundary(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, sectionGapSections(), "", "")
	m.list.SetShowPagination(false)
	m.list.SetSize(common.width, 3)
	if got := m.list.Paginator.PerPage; got != 2 {
		t.Fatalf("PerPage = %d, want 2 so the spacer starts the next page", got)
	}

	m.list.Select(2) // the community, immediately before the spacer's page
	m, _ = m.update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if got, ok := m.selected(); !ok || got.target != "date@example.com" {
		t.Fatalf("down selected = %+v, %v; want first service across the page boundary", got, ok)
	}

	m, _ = m.update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if got, ok := m.selected(); !ok || got.target != "@plan.cat" {
		t.Fatalf("up selected = %+v, %v; want community across the page boundary", got, ok)
	}
}

func TestStartSectionGapResponsiveResizePreservesSelection(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, sectionGapSections(), "", "")
	if !m.selectTarget("date@example.com") {
		t.Fatal("could not select the service fixture")
	}

	m.setSize(71, common.bodyHeight())
	if got := len(m.list.Items()); got != 4 {
		t.Fatalf("narrow item count = %d, want 4 without a section spacer", got)
	}
	if got, ok := m.selected(); !ok || got.target != "date@example.com" {
		t.Fatalf("narrow selected = %+v, %v; want date@example.com", got, ok)
	}

	m.setSize(72, common.bodyHeight())
	if got := len(m.list.Items()); got != 5 {
		t.Fatalf("wide item count = %d, want 5 with a section spacer", got)
	}
	if got, ok := m.selected(); !ok || got.target != "date@example.com" {
		t.Fatalf("wide selected = %+v, %v; want date@example.com", got, ok)
	}
}

func TestStartSectionGapResponsiveResizePreservesDuplicateOccurrence(t *testing.T) {
	common := testCommon()
	common.width = 80
	sections := sectionGapSections()
	sections = append([]startSection{{
		id: sectionBookmarks, title: "BOOKMARKS",
		entries: []startEntry{
			{target: "@tilde.team", source: sourceBookmark},
			{target: "@tilde.team", source: sourceBookmark},
		},
	}}, sections...)
	m := newStart(common, sections, "", "")

	seen := 0
	for index, item := range m.list.Items() {
		row, ok := item.(startItem)
		if !ok || !row.selectable() || row.entry.target != "@tilde.team" {
			continue
		}
		if seen == 1 {
			m.list.Select(index)
			break
		}
		seen++
	}
	position, ok := m.captureTogglePosition()
	if !ok || position.full != (startSectionPosition{section: sectionBookmarks, ordinal: 1}) {
		t.Fatalf("initial position = %+v, %v; want second bookmark occurrence", position.full, ok)
	}

	m.setSize(71, common.bodyHeight())
	position, ok = m.captureTogglePosition()
	if !ok || position.full != (startSectionPosition{section: sectionBookmarks, ordinal: 1}) {
		t.Fatalf("resized position = %+v, %v; want second bookmark occurrence", position.full, ok)
	}
}

func TestStartSectionGapResponsiveResizeSynchronizesAfterFilterClears(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, sectionGapSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = typeStartFilter(t, m, "date")
	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEnter})

	m.setSize(71, common.bodyHeight())
	if got := len(m.list.Items()); got != 5 {
		t.Fatalf("filtered resize changed underlying item count to %d, want 5 until the filter clears", got)
	}

	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if got := len(m.list.Items()); got != 4 {
		t.Fatalf("cleared narrow item count = %d, want 4 without a section spacer", got)
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

func TestStartSetSizeFromZeroWidthCountsOverview(t *testing.T) {
	common := testCommon()
	common.width = 0
	common.height = 36
	m := newStart(common, twoSections(), "", "")
	if got := m.list.Width(); got != 0 {
		t.Fatalf("precondition: list width = %d, want 0 so the first wide layout is the first SetSize", got)
	}

	m.setSize(100, 34)
	if got := m.overviewHeight(); got != 1 {
		t.Fatalf("overview height = %d, want 1 at 100 columns", got)
	}
	want := 34 - startChromeRows - m.noticeHeight() - m.overviewHeight()
	if got := m.list.Height(); got != want {
		t.Fatalf("first wide setSize list height = %d, want %d", got, want)
	}

	m.setSize(100, 34)
	if got := m.list.Height(); got != want {
		t.Fatalf("second setSize list height = %d, want %d (must be idempotent)", got, want)
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
			d.Render(&header, m.list, 1, m.list.Items()[1])
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
		{name: "catalog on with bookmark", sections: threeSections()},
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
				if !row.selectable() && row.header == "" && !row.spacer {
					t.Fatalf("non-selectable item = %#v, want a header or section spacer", row)
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

func TestNarrowChildRowAlignsSelectedNoteUnderToken(t *testing.T) {
	common := testCommon()
	common.width = 40
	common.contentFocused = true
	st := common.ensureStyles()
	d := newStartDelegate(common, st)
	if d.Height() != 2 {
		t.Fatalf("delegate height = %d at width 40, want the two-line layout", d.Height())
	}
	items := []list.Item{
		startItem{entry: startEntry{
			target: "dict@bbs.airandwave.net", note: "Dictionary lookup",
			child: true, lastChild: true,
		}, section: sectionServices},
	}
	l := list.New(items, d, 40, 4)
	l.Select(0)

	var buf strings.Builder
	d.Render(&buf, l, 0, items[0])
	lines := strings.Split(ansi.Strip(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("selected child rendered %d lines, want 2: %q", len(lines), lines)
	}
	targetByte := strings.Index(lines[0], "dict")
	noteByte := strings.Index(lines[1], "Dictionary lookup")
	if targetByte < 0 || noteByte < 0 {
		t.Fatalf("rendered lines = %q, want token and note", lines)
	}
	targetColumn := lipgloss.Width(lines[0][:targetByte])
	noteColumn := lipgloss.Width(lines[1][:noteByte])
	if noteColumn != targetColumn {
		t.Fatalf("note column = %d, token column = %d: %q", noteColumn, targetColumn, lines)
	}
}

// bubbles/list derives pagination from one fixed delegate height, so the
// narrow two-line layout can't grow only for a selected row: an unselected
// child's second line is dead space rather than tighter spacing.
func TestNarrowChildRowLeavesTheNoteLineBlankWhenUnselected(t *testing.T) {
	common := testCommon()
	common.width = 46
	common.contentFocused = true
	st := common.ensureStyles()
	d := newStartDelegate(common, st)
	if d.Height() != 2 {
		t.Fatalf("delegate height = %d at width 46, want the two-line layout", d.Height())
	}
	items := []list.Item{
		startItem{entry: startEntry{
			target: "dict@bbs.airandwave.net", note: "Dictionary lookup",
			child: true, lastChild: true,
		}, section: sectionServices},
		startItem{entry: startEntry{
			target: "wtr@bbs.airandwave.net", note: "Weather report",
			child: true, lastChild: true,
		}, section: sectionServices},
	}
	l := list.New(items, d, 46, 4)
	l.Select(1) // select the other row so item 0 renders unselected

	var buf strings.Builder
	d.Render(&buf, l, 0, items[0])
	lines := strings.Split(ansi.Strip(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("unselected child rendered %d lines, want 2: %q", len(lines), lines)
	}
	if !strings.Contains(lines[0], "dict") {
		t.Fatalf("first line = %q, want the connector and token", lines[0])
	}
	if strings.TrimSpace(lines[1]) != "" {
		t.Fatalf("second line = %q, want blank when unselected", lines[1])
	}
}

// The wide single-line layout is the macOS default and was once revertible with
// the suite still green, so it is asserted on rendered output rather than on
// startRowNote alone.
func TestWideChildRowShowsItsNoteOnlyWhenSelected(t *testing.T) {
	const note = "Latest earthquakes, M2.5+ past day"
	common := testCommon()
	common.width = 100
	st := common.ensureStyles()
	d := newStartDelegate(common, st)
	if d.Height() != 1 {
		t.Fatalf("delegate height = %d at width 100, want the one-line layout", d.Height())
	}
	items := []list.Item{
		startItem{entry: startEntry{
			target: "quake@bbs.airandwave.net", note: note,
			child: true,
		}, section: sectionServices},
		startItem{entry: startEntry{
			target: "urban@bbs.airandwave.net", note: note,
			child: true, lastChild: true,
		}, section: sectionServices},
	}
	l := list.New(items, d, 100, 6)

	l.Select(1) // row 0 unselected
	var unselected strings.Builder
	d.Render(&unselected, l, 0, items[0])
	unselectedLine := ansi.Strip(unselected.String())
	if strings.Contains(unselectedLine, note) {
		t.Errorf("unselected child row = %q, want an empty note column", unselectedLine)
	}
	if !strings.Contains(unselectedLine, "quake") {
		t.Errorf("unselected child row = %q, want its token", unselectedLine)
	}

	l.Select(0) // row 0 selected
	var selected strings.Builder
	d.Render(&selected, l, 0, items[0])
	if got := ansi.Strip(selected.String()); !strings.Contains(got, note) {
		t.Errorf("selected child row = %q, want its full note", got)
	}
}

// Selection is the cursor's row, not which pane takes keys. A selected child
// keeps its note while the target input is focused and the inactive shelf is
// drawn — otherwise the note would blink out every time focus moved.
func TestSelectedChildKeepsItsNoteWithoutContentFocus(t *testing.T) {
	const note = "Latest earthquakes, M2.5+ past day"
	common := testCommon()
	common.width = 100
	common.contentFocused = false
	st := common.ensureStyles()
	d := newStartDelegate(common, st)
	items := []list.Item{
		startItem{entry: startEntry{
			target: "quake@bbs.airandwave.net", note: note,
			child: true, lastChild: true,
		}, section: sectionServices},
	}
	l := list.New(items, d, 100, 4)
	l.Select(0)

	var buf strings.Builder
	d.Render(&buf, l, 0, items[0])
	if got := ansi.Strip(buf.String()); !strings.Contains(got, note) {
		t.Fatalf("selected child without content focus = %q, want its full note", got)
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

// Pressing "/" with nothing typed does not flatten anything: bubbles still
// returns every item, headers included, so a child is still sitting under its
// parent and must keep its token. Only a non-empty query collapses the view and
// leaves a child needing its full address.
func TestEmptyFilterKeepsChildTokensAndQueryExpandsThem(t *testing.T) {
	newStartWithGroup := func(t *testing.T) startModel {
		t.Helper()
		common := testCommon()
		common.width = 80
		return newStart(common, []startSection{{
			id: sectionServices, title: "SERVICES",
			entries: []startEntry{
				{target: "@bbs.airandwave.net", note: "Over two dozen services", source: sourceCatalog},
				{target: "dict@bbs.airandwave.net", note: "Dictionary lookup", source: sourceCatalog, child: true},
			},
		}}, "", "")
	}

	t.Run("empty query keeps the token", func(t *testing.T) {
		m := newStartWithGroup(t)
		m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
		if got := m.list.FilterState(); got != list.Filtering {
			t.Fatalf("filter state = %v, want Filtering", got)
		}
		plain := ansi.Strip(m.View())
		if strings.Contains(plain, "dict@bbs.airandwave.net") {
			t.Errorf("child expanded to its full address while its group is still on screen:\n%s", plain)
		}
		if !strings.Contains(plain, "├ dict") {
			t.Errorf("child token missing:\n%s", plain)
		}
	})

	t.Run("a typed query expands it", func(t *testing.T) {
		m := newStartWithGroup(t)
		m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
		m = typeStartFilter(t, m, "dict")
		plain := ansi.Strip(m.View())
		if !strings.Contains(plain, "dict@bbs.airandwave.net") {
			t.Errorf("flattened child kept its bare token:\n%s", plain)
		}
	})
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

func TestStartRowTargetDrawsConnectors(t *testing.T) {
	mid := startEntry{target: "dict@bbs.airandwave.net", child: true}
	last := startEntry{target: "wordsearch:today@bbs.airandwave.net", child: true, lastChild: true}
	root := startEntry{target: "@bbs.airandwave.net"}
	if got, want := startRowTarget(mid, false), "   ├ dict"; got != want {
		t.Errorf("mid child = %q, want %q", got, want)
	}
	if got, want := startRowTarget(last, false), "   └ wordsearch:today"; got != want {
		t.Errorf("last child = %q, want %q", got, want)
	}
	if got, want := startRowTarget(root, false), "@bbs.airandwave.net"; got != want {
		t.Errorf("root = %q, want %q", got, want)
	}
	// Flattened: no parent above it, so the address returns and the connector
	// goes with it.
	if got, want := startRowTarget(mid, true), "dict@bbs.airandwave.net"; got != want {
		t.Errorf("flattened child = %q, want %q", got, want)
	}
}

func TestStartRowTargetRendersFullAddressForNumericTokens(t *testing.T) {
	// Issue #93: `1@happynetbox.com` (or any all-digit token) renders as a
	// bare numeral under the connector, which reads as a list marker rather
	// than a service name. The fix keeps the connector so the row still
	// reads as a child, but shows the full address in place of the bare
	// token.
	single := startEntry{target: "1@happynetbox.com", child: true}
	multi := startEntry{target: "12@happynetbox.com", child: true, lastChild: true}
	letter := startEntry{target: "bot@happynetbox.com", child: true}
	mixed := startEntry{target: "r2d2@happynetbox.com", child: true}
	if got, want := startRowTarget(single, false), "   ├ 1@happynetbox.com"; got != want {
		t.Errorf("single-digit child = %q, want %q", got, want)
	}
	if got, want := startRowTarget(multi, false), "   └ 12@happynetbox.com"; got != want {
		t.Errorf("multi-digit child = %q, want %q", got, want)
	}
	// Non-numeric tokens keep the original token-only rendering.
	if got, want := startRowTarget(letter, false), "   ├ bot"; got != want {
		t.Errorf("letter child = %q, want %q", got, want)
	}
	if got, want := startRowTarget(mixed, false), "   ├ r2d2"; got != want {
		t.Errorf("mixed child = %q, want %q", got, want)
	}
	// Flattened child still returns the bare address (parent is off screen).
	if got, want := startRowTarget(single, true), "1@happynetbox.com"; got != want {
		t.Errorf("flattened single-digit child = %q, want %q", got, want)
	}
}

func TestStartRowNotePerState(t *testing.T) {
	const note = "Saturday Morning Gemzine — back issues"
	child := startEntry{target: "smog@typed-hole.org", note: note, child: true}
	root := startEntry{target: "@typed-hole.org", note: "A small menu of fingers, from lobste.rs to smog"}

	tests := []struct {
		name      string
		entry     startEntry
		selected  bool
		flattened bool
		want      string
	}{
		{name: "unselected child shows nothing", entry: child, want: ""},
		{name: "selected child shows its note", entry: child, selected: true, want: note},
		{name: "flattened child shows its note", entry: child, flattened: true, want: note},
		{name: "root always shows its note", entry: root, want: root.note},
		{name: "selected root is unchanged", entry: root, selected: true, want: root.note},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := startRowNote(tt.entry, tt.selected, tt.flattened); got != tt.want {
				t.Fatalf("startRowNote = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterValueIsTargetAndNote(t *testing.T) {
	entry := startEntry{target: "cyoa@typed-hole.org", note: "Choose your own adventure", child: true}
	item := startItem{entry: entry}
	if got, want := item.FilterValue(), "cyoa@typed-hole.org Choose your own adventure"; got != want {
		t.Errorf("child FilterValue = %q, want %q", got, want)
	}
	structural := startItem{entry: startEntry{target: "@happynetbox.com", structural: true}}
	if got := structural.FilterValue(); got != "" {
		t.Errorf("structural FilterValue = %q, want empty", got)
	}
}

func TestSplitStartMatchesMapsTargetAndNote(t *testing.T) {
	target := "cyoa@typed-hole.org"
	targetMatches, noteMatches := splitStartMatches([]int{0, len(target) + 1}, target)
	if len(targetMatches) != 1 || targetMatches[0] != 0 {
		t.Errorf("targetMatches = %v, want [0]", targetMatches)
	}
	if len(noteMatches) != 1 || noteMatches[0] != 0 {
		t.Errorf("noteMatches = %v, want [0]", noteMatches)
	}
}

func TestStartFilterNoMatchNamesTheQuery(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = typeStartFilter(t, m, "zzzzzz")

	if got := len(m.list.VisibleItems()); got != 0 {
		t.Fatalf("filter matched %d rows, want a zero-match filter for this test", got)
	}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "no match for “zzzzzz”") {
		t.Fatalf("no-match message missing from view:\n%s", view)
	}
	if !strings.Contains(view, "zzzzzz") || !strings.Contains(view, "Filter:") {
		t.Fatalf("filter prompt must survive alongside the message:\n%s", view)
	}
}

func TestStartFilterNoMatchSitsBelowThePrompt(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = typeStartFilter(t, m, "zzzzzz")

	lines := strings.Split(ansi.Strip(m.View()), "\n")
	offset := m.filterPromptHeight()
	if offset < 1 || offset >= len(lines) {
		t.Fatalf("filter prompt height %d is outside the %d-line view", offset, len(lines))
	}
	if got := strings.TrimSpace(lines[offset]); got != "no match for “zzzzzz”" {
		t.Fatalf("line %d = %q, want the no-match message", offset, got)
	}
	if !strings.Contains(lines[0], "Filter:") {
		t.Fatalf("line 0 = %q, want the filter prompt", lines[0])
	}
}

func TestStartFilterWithMatchesHasNoMessage(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = typeStartFilter(t, m, "plan")

	if len(m.list.VisibleItems()) == 0 {
		t.Fatal("expected at least one match for this test")
	}
	if strings.Contains(ansi.Strip(m.View()), "no match for") {
		t.Fatal("a matching filter must not show the no-match message")
	}
}

func TestStartEmptyFilterHasNoMessage(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})

	if strings.Contains(ansi.Strip(m.View()), "no match for") {
		t.Fatal("pressing / with nothing typed must not show the no-match message")
	}
}

func TestStartUnfilteredHasNoMessage(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")

	if strings.Contains(ansi.Strip(m.View()), "no match for") {
		t.Fatal("an unfiltered startpage must not show the no-match message")
	}
}

func TestStartFilterNoMatchTruncatesAtNarrowWidth(t *testing.T) {
	common := testCommon()
	common.width = 20
	m := newStart(common, threeSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = typeStartFilter(t, m, "zzzzzzzzzzzzzzzzzzzz")

	message := ansi.Strip(m.noMatchMessage())
	if message == "" {
		t.Fatal("expected a no-match message at 20 columns")
	}
	if strings.Contains(message, "\n") {
		t.Fatalf("message must stay one row tall, got %q", message)
	}
	if w := lipgloss.Width(message); w > m.list.Width() {
		t.Fatalf("message width %d exceeds list width %d: %q", w, m.list.Width(), message)
	}
	if !strings.HasSuffix(message, "…") {
		t.Fatalf("message should truncate with an ellipsis at this width, got %q", message)
	}
}

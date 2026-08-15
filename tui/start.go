package tui

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// startChromeRows matches listChromeRows: space the bubbles list reserves once
// its own title and help are hidden.
const (
	startChromeRows      = 1
	startWideMinWidth    = 72
	startTargetColumnPct = 50
	// Blank cells between the widest target and the note column.
	startTargetColumnGap = 2
	// startBookmarkMarker precedes a row the user pinned themselves.
	startBookmarkMarker = "★"
)

// startItem is one row: an entry, section header, or wide-layout spacer.
// Non-entry rows occupy normal item slots so the list's uniform-height
// pagination holds.
type startItem struct {
	entry   startEntry
	header  string // non-empty => this row is a section heading
	section startSectionID
	spacer  bool // one blank row above a section header, in the one-line layout
}

func (i startItem) selectable() bool {
	return i.header == "" && i.entry.target != ""
}

// countsAsListing reports whether this row should count toward the overview
// and status-bar totals. The rule is what each section is showing: every
// selectable row counts in the section that drew it, headers and spacers count
// nowhere. startCounts and appModel.startBar each tally rows independently, and
// both must call this rather than re-encode the rule, or the overview and the
// status bar can silently disagree on screen.
//
// A structural parent counts, though it is a second copy of a target listed
// elsewhere. Excluding it made the count contradict the section it described:
// pinning a service host that keeps its children leaves the host heading
// SERVICES and dropped the services count anyway (#123). The price is that a
// host on screen twice is counted twice — YOURS and CATALOG can overlap by a
// pinned group parent, and @happynetbox.com is both a community and the header
// of its services group. A filter is exempt for free: structural rows return ""
// from FilterValue, so a flattened view has already dropped them.
func (i startItem) countsAsListing() bool {
	return i.selectable()
}

// FilterValue drives "/". Non-entry rows return "" so they drop out while
// filtering, which flattens the view to matches — the behaviour we want.
// Structural rows return "" for a second reason: filtering removes the section
// headers that distinguish a parent copy from the listing it duplicates, so
// without this a filter would show two identical selectable rows.
func (i startItem) FilterValue() string {
	if !i.selectable() || i.entry.structural {
		return ""
	}
	return i.entry.target + " " + i.entry.note
}

func (i startItem) Title() string {
	if i.header != "" {
		return i.header
	}
	return i.entry.target
}

func (i startItem) Description() string {
	if i.header != "" {
		return ""
	}
	return i.entry.note
}

// startModel is the launch screen: an embedded catalog plus the user's
// bookmarks, rendered as one sectioned list.
type startModel struct {
	common          *commonModel
	list            list.Model
	sections        []startSection
	assembledCounts startOverviewCounts
	height          int
	notice          string // parse problems, surfaced rather than swallowed
	empty           string // shown instead of the list when there is nothing to show
}

type startOverviewCounts struct {
	bookmarks   int
	communities int
	services    int
}

type startSectionPosition struct {
	section startSectionID
	ordinal int
}

type startTogglePosition struct {
	full     startSectionPosition
	filtered *startSectionPosition
	filter   string
}

var startSectionOrder = [...]startSectionID{
	sectionBookmarks,
	sectionCommunities,
	sectionServices,
}

func startCounts(items []list.Item) startOverviewCounts {
	var counts startOverviewCounts
	for _, item := range items {
		it, ok := item.(startItem)
		if !ok || !it.countsAsListing() {
			continue
		}
		switch it.section {
		case sectionBookmarks:
			counts.bookmarks++
		case sectionCommunities:
			counts.communities++
		case sectionServices:
			counts.services++
		}
	}
	return counts
}

// A row in a section with no overview identity (today only the dormant
// PEOPLE/kindPerson group, which carries sectionUnknown) is counted nowhere. The
// shipped catalog contains no person entries, so the overview total equals
// startBar's visible count; adding one would silently break that invariant.
func countLabel(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func startItems(sections []startSection, width int) []list.Item {
	var items []list.Item
	for _, s := range sections {
		// Exactly one blank row above every section header. The wide one-row
		// layout needs a spacer item to produce it — except above the first
		// header, where bubbles' reserved filter row (rendered whenever
		// filtering is enabled, even with SetShowTitle(false)) already
		// supplies one. The narrow two-row layout needs no spacer at any
		// boundary, because renderHeader spends its first row on a blank.
		if width >= startWideMinWidth && len(items) > 0 {
			items = append(items, startItem{spacer: true})
		}
		items = append(items, startItem{header: s.title, section: s.id})
		for _, e := range s.entries {
			items = append(items, startItem{entry: e, section: s.id})
		}
	}
	return items
}

func newStart(common *commonModel, sections []startSection, notice, empty string) startModel {
	items := startItems(sections, common.width)

	st := common.ensureStyles()
	height := common.bodyHeight() - startChromeRows
	if height < 1 {
		height = 1
	}
	l := list.New(items, newStartDelegate(common, st), common.width, height)
	applyListStyles(&l, st)
	// NOT redundant with the delegate passed to list.New: applyListStyles ends
	// with SetDelegate(defaultUserDelegate(st)) (tui/list.go:141), which clobbers
	// it. The startpage delegate must be reinstated after every style pass — see
	// applyStyles below, which orders it the same way for the same reason.
	l.SetDelegate(newStartDelegate(common, st))
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	// Only reaches Styles.NoItems ("No entries.") — the status bar is hidden.
	// Matches the noun our own bar uses, instead of the default "No items.".
	l.SetStatusBarItemName("entry", "entries")

	m := startModel{common: common, list: l, sections: sections, assembledCounts: startCounts(items), notice: notice, empty: empty}
	m.skipNonEntry(1) // never rest on the leading header
	m.setSize(common.width, common.bodyHeight())
	return m
}

type startDelegate struct {
	common *commonModel
	st     styles
}

func newStartDelegate(common *commonModel, st styles) startDelegate {
	return startDelegate{common: common, st: st}
}

// startTargetColumn is the width the target column holds: the widest target on
// the page as currently drawn, plus a gap. Measured over every visible row
// rather than per section, so the note column's left edge stays put down the
// whole page instead of stepping with each section.
//
// It takes flattened rather than measuring both of a child's forms, because a
// child draws as a token under its connector and only regains its full address
// once a filter flattens the view. Reserving room for the address it is not
// showing would pad the gutter for text that is not on the screen, which is
// the walk this measurement exists to close.
func startTargetColumn(items []list.Item, flattened bool) int {
	widest := 0
	for _, item := range items {
		it, ok := item.(startItem)
		if !ok || !it.selectable() {
			continue
		}
		if w := lipgloss.Width(startRowTarget(it.entry, flattened)); w > widest {
			widest = w
		}
	}
	if widest == 0 {
		return 0
	}
	return widest + startTargetColumnGap
}

func (d startDelegate) Height() int {
	if d.common.width >= startWideMinWidth {
		return 1
	}
	return 2
}

func (d startDelegate) Spacing() int { return 0 }

func (d startDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d startDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(startItem)
	if !ok || m.Width() <= 0 {
		return
	}
	if it.spacer {
		return
	}
	if it.header != "" {
		d.renderHeader(w, m.Width(), it.header, d.headerNeedsBlank(m, index))
		return
	}
	if !it.selectable() {
		return
	}
	d.renderEntry(w, m, index, it)
}

// headerNeedsBlank reports whether a two-row header must spend its first row on
// a blank to satisfy the one-blank-row-above-every-header rule. It must not
// when the row above already renders empty, or the gap doubles.
//
// Two cases produce an already-blank row. At the top of a page bubbles' own
// title row sits directly above, and it is empty unless the filter prompt is
// in it. And a two-row entry whose note is absent — an unclassified bookmark,
// or a grouped child whose note stays hidden until selected — leaves its
// second row empty, which is why the gap used to depend on whether the last
// entry of a section happened to carry a note.
//
// The one-row layout never needs this: its headers occupy a single row and the
// gap is a spacer item instead.
func (d startDelegate) headerNeedsBlank(m list.Model, index int) bool {
	if d.Height() == 1 {
		return false
	}
	items := m.VisibleItems()
	start, _ := m.Paginator.GetSliceBounds(len(items))
	if index <= start {
		return m.FilterState() == list.Filtering
	}
	if index-1 >= len(items) {
		return true
	}
	prev, ok := items[index-1].(startItem)
	if !ok {
		return true
	}
	return !d.rowEndsBlank(m, index-1, prev)
}

// rowEndsBlank reports whether a two-row row's second terminal row renders as
// empty space. A selected row never does: the selection shelf draws its border
// down both rows.
func (d startDelegate) rowEndsBlank(m list.Model, index int, it startItem) bool {
	if it.spacer {
		return true
	}
	if it.header != "" || !it.selectable() {
		return false
	}
	isSelected := index == m.Index()
	if isSelected && m.FilterState() != list.Filtering {
		return false
	}
	emptyFilter := m.FilterState() == list.Filtering && m.FilterValue() == ""
	flattened := (m.FilterState() == list.Filtering || m.FilterState() == list.FilterApplied) && !emptyFilter
	return startRowNote(it.entry, isSelected, flattened) == ""
}

func (d startDelegate) renderHeader(w io.Writer, width int, header string, needsBlank bool) {
	if needsBlank {
		fmt.Fprint(w, "\n") //nolint:errcheck
	}
	label := ansi.Truncate(header+" ", width, "…")
	ruleWidth := width - lipgloss.Width(label)
	if ruleWidth < 0 {
		ruleWidth = 0
	}
	fmt.Fprintf(w, "%s%s", d.st.barFlag.Bold(true).Render(label), lipgloss.NewStyle().Foreground(d.st.palette.Rule).Render(strings.Repeat("─", ruleWidth))) //nolint:errcheck
}

func (d startDelegate) renderEntry(w io.Writer, m list.Model, index int, item startItem) {
	isSelected := index == m.Index()
	emptyFilter := m.FilterState() == list.Filtering && m.FilterValue() == ""
	isFiltered := m.FilterState() == list.Filtering || m.FilterState() == list.FilterApplied
	// Flattened means the view has actually collapsed: headers and structural
	// rows are gone and a child has no parent above it to supply its host. That
	// needs a non-empty query, not merely an active filter — pressing "/" alone
	// leaves every group and header on screen, where a child is still a child.
	flattened := isFiltered && !emptyFilter
	showShelf := isSelected && m.FilterState() != list.Filtering
	// Only the active shelf is painted across the row; the inactive one is a
	// left rule with nothing behind it, so it renders down the ordinary path.
	fillShelf := showShelf && d.common.contentFocused

	titleStyle, descStyle := d.st.listItem.NormalTitle, d.st.listItem.NormalDesc
	if emptyFilter {
		titleStyle, descStyle = d.st.listItem.DimmedTitle, d.st.listItem.DimmedDesc
	} else if showShelf {
		if d.common.contentFocused {
			titleStyle, descStyle = d.st.listItem.SelectedTitle, d.st.listItem.SelectedDesc
		} else {
			// No fill while the input holds focus: a tinted full-width shelf
			// reads as an active selection, so it competes with the input's
			// cursor for the one focus signal on screen. The left rule alone
			// keeps the place browsing will resume from — in Dim rather than
			// Rule, which at #ded8e8 on #fbfafc all but vanishes in a light
			// terminal, losing the resume point along with the fill.
			inactiveShelf := lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(d.st.palette.Dim).
				Padding(0, 0, 0, 1)
			titleStyle = inactiveShelf.Foreground(d.st.palette.Text)
			descStyle = inactiveShelf.Foreground(d.st.palette.Dim)
		}
	}
	titleStyle = startStyleWithinWidth(titleStyle, m.Width())
	descStyle = startStyleWithinWidth(descStyle, m.Width())

	rowTarget := startRowTarget(item.entry, flattened)
	rowNote := startRowNote(item.entry, isSelected, flattened)

	var targetMatches, noteMatches []int
	if flattened {
		targetMatches, noteMatches = splitStartMatches(m.MatchesForItem(index), item.entry.target)
	}

	if d.Height() == 1 {
		frame := titleStyle.GetHorizontalFrameSize()
		targetWidth, noteWidth := startColumnWidths(m.Width(), frame, startTargetColumn(m.VisibleItems(), flattened))
		target := renderStartField(rowTarget, targetWidth, targetMatches, titleStyle, d.st.listItem.FilterMatch)
		note := renderStartField(rowNote, noteWidth, noteMatches, descStyle, d.st.listItem.FilterMatch)
		target = padStartField(target, targetWidth, startInlineStyle(titleStyle))
		row := target + note
		if fillShelf {
			fmt.Fprint(w, renderSelectedShelfLine(row, titleStyle, m.Width())) //nolint:errcheck
			return
		}
		fmt.Fprint(w, titleStyle.Render(row)) //nolint:errcheck
		return
	}

	titleWidth := m.Width() - titleStyle.GetHorizontalFrameSize()
	if titleWidth < 0 {
		titleWidth = 0
	}
	descWidth := m.Width() - descStyle.GetHorizontalFrameSize()
	if descWidth < 0 {
		descWidth = 0
	}
	if item.entry.child && !flattened && rowNote != "" {
		rowNote = "     " + rowNote
	}
	target := renderStartField(rowTarget, titleWidth, targetMatches, titleStyle, d.st.listItem.FilterMatch)
	note := renderStartField(rowNote, descWidth, noteMatches, descStyle, d.st.listItem.FilterMatch)
	if fillShelf {
		fmt.Fprintf(w, "%s\n%s", renderSelectedShelfLine(target, titleStyle, m.Width()), renderSelectedShelfLine(note, descStyle, m.Width())) //nolint:errcheck
		return
	}
	fmt.Fprintf(w, "%s\n%s", titleStyle.Render(target), descStyle.Render(note)) //nolint:errcheck
}

func startColumnWidths(width, frame, measured int) (int, int) {
	available := width - frame
	if available < 0 {
		available = 0
	}
	// The measured column hands the layout to the page's longest row, so the
	// old fixed half survives as its ceiling: one very long address can crowd
	// the notes, but it can never push them off the screen.
	target := min(measured, available*startTargetColumnPct/100)
	return target, available - target
}

// startRowTarget is the target column's text. A child shows only its query
// token, prefixed by a connector that gives the group its shape, because the
// parent row above it states the host — but once the view flattens, the parent
// may be off screen, so the full address returns and the connector goes with
// it. Flattened is not the same as "a filter is active": see renderEntry.
//
// A child whose token reads as a numeral (`1`, `12`) would otherwise be
// indistinguishable from a list numeral under the connector; in that case the
// full address is rendered in its place so the row stays legible. The
// connector is kept — it is what signals "this is a child of the row above" —
// so the trade is purely about the label, not the tree shape. See issue #93.
//
// A row the user pinned themselves carries a marker, because BOOKMARKS
// otherwise renders exactly like the sections lookit ships and reads as a third
// catalog section (issue #97). Two things about it:
//
// It keys on source, not on the bookmarked flag. They differ: a catalog parent
// retained to head a SERVICES group is stamped bookmarked when its target is
// pinned (sections.go), so the flag would mark a group header outside
// BOOKMARKS. Only source names the list that built the row.
//
// It goes in the target text rather than the row's left padding, so the
// measured gutter counts it and pinned notes keep the column the rest of the
// page uses. The cost is that BOOKMARKS targets sit two cells right of the
// catalog's — read as an indented, marked block, the same idiom the child
// connector already uses. Like the connector, it drops when the view flattens:
// match highlighting is computed against entry.target, so a prefix there would
// shift every offset, and a flattened view has dropped its section headers
// anyway.
func startRowTarget(entry startEntry, flattened bool) string {
	if flattened {
		return entry.target
	}
	if !entry.child {
		if entry.source == sourceBookmark {
			return startBookmarkMarker + " " + entry.target
		}
		return entry.target
	}
	token := entryToken(entry.target)
	connector := "├"
	if entry.lastChild {
		connector = "└"
	}
	if tokenAllNumeric(token) {
		return "   " + connector + " " + entry.target
	}
	return "   " + connector + " " + token
}

// tokenAllNumeric reports whether s is non-empty and every rune is an ASCII
// digit — the rule keyed on "looks like a numeral" from issue #93.
func tokenAllNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// startRowNote is the note column's text. Two rows are silent, for different
// reasons.
//
// A child is silent by default, so the section reads as a list of tokens rather
// than a wall of prose; its note returns on the row the cursor is on, and
// wherever the child renders as a listing rather than a member of a visible
// group.
//
// A pinned row's note column holds its last-visited date. The row is silent
// only while the date is unknown — the cursor never lifts an unvisited row's
// blank. Suppression keys on source, not on the bookmarked flag, for the
// reason the ownership marker does: a structural parent retained to head a
// SERVICES group is stamped bookmarked but was built from the catalog, and
// blanking it would silently strip a group header the user never pinned.
//
// Both silences lift when a filter flattens the page, and a flattening filter
// always restores the catalog note so "/" stays honest. The entry keeps its
// note throughout — suppression is a property of where the row renders, which
// is what makes unpinning restore the description with no state to unwind, and
// what keeps FilterValue honest: "/" still matches a pinned row on its note,
// and the flattened view it produces is exactly where that note is on screen.
func startRowNote(entry startEntry, selected, flattened bool) string {
	if flattened {
		return entry.note
	}
	if entry.source == sourceBookmark {
		if entry.visited.IsZero() {
			return ""
		}
		// The prefix is what makes the value self-describing. This column holds
		// prose for catalog rows, so a bare "today" changes register with
		// nothing to announce it — and a filter that flattens the sections
		// interleaves the two kinds of row in the same column, which is exactly
		// where the reader has least context to supply the missing word.
		return "visited " + relativeDay(entry.visited, nowFn())
	}
	if entry.child && !selected {
		return ""
	}
	return entry.note
}

// relativeDay renders a last-visited instant relative to now, fuzzier the
// further back it goes. Buckets are calendar-day differences in the user's
// local zone, not divisions of an elapsed duration: bucketing in UTC would
// tell a user in UTC-8 that an evening visit happened "today" all through the
// following morning. A future stamp (clock skew, hand-edit) clamps to today.
func relativeDay(ts, now time.Time) string {
	t, n := ts.In(time.Local), now.In(time.Local)
	if t.After(n) {
		return "today"
	}
	// Walk local midnights with AddDate. Dividing elapsed hours by 24 is
	// wrong across DST: a spring-forward Saturday→Monday is 47 hours and
	// would land in the "yesterday" bucket.
	ty, tm, td := t.Date()
	ny, nm, nd := n.Date()
	cursor := time.Date(ty, tm, td, 0, 0, 0, 0, t.Location())
	end := time.Date(ny, nm, nd, 0, 0, 0, 0, n.Location())
	days := 0
	// Stop at the last bucket boundary: every value from 365 up renders the
	// same, and walking them one day at a time is unbounded work on every
	// render for a stamp an epoch-era clock or a hand-edit can supply.
	for days < 365 && cursor.Before(end) {
		cursor = cursor.AddDate(0, 0, 1)
		days++
	}
	switch {
	case days == 0:
		return "today"
	case days == 1:
		return "yesterday"
	case days < 30:
		return fmt.Sprintf("%d days ago", days)
	case days < 365:
		months := days / 30
		if months > 11 {
			months = 11 // 360–364 days would otherwise say "12 months ago"
		}
		return ago(months, "month")
	default:
		return "over 1 year ago"
	}
}

func ago(n int, unit string) string {
	if n == 1 {
		return "1 " + unit + " ago"
	}
	return fmt.Sprintf("%d %ss ago", n, unit)
}

// splitStartMatches maps filter-match offsets in FilterValue onto the target
// and note fields.
func splitStartMatches(matches []int, target string) (targetMatches, noteMatches []int) {
	noteOffset := len(target) + 1
	for _, match := range matches {
		if match < noteOffset-1 {
			targetMatches = append(targetMatches, match)
		} else if match >= noteOffset {
			noteMatches = append(noteMatches, match-noteOffset)
		}
	}
	return targetMatches, noteMatches
}

func startStyleWithinWidth(st lipgloss.Style, width int) lipgloss.Style {
	overflow := st.GetHorizontalFrameSize() - width
	if overflow <= 0 {
		return st
	}
	left := st.GetPaddingLeft()
	if overflow > left {
		overflow = left
	}
	return st.PaddingLeft(left - overflow)
}

func renderStartField(value string, width int, matches []int, base, match lipgloss.Style) string {
	truncated := ansi.Truncate(value, width, "…")
	if len(matches) == 0 {
		return startInlineStyle(base).Render(truncated)
	}
	limit := len([]rune(truncated))
	if lipgloss.Width(value) > width && limit > 0 {
		limit-- // the ellipsis is not one of the original field's matched runes
	}
	kept := make([]int, 0, len(matches))
	for _, byteIndex := range matches {
		if byteIndex < 0 || byteIndex >= len(value) || !utf8.RuneStart(value[byteIndex]) {
			continue
		}
		runeIndex := utf8.RuneCountInString(value[:byteIndex])
		if runeIndex < limit {
			kept = append(kept, runeIndex)
		}
	}
	unmatched := startInlineStyle(base).Inline(true)
	matched := unmatched.Inherit(match)
	return lipgloss.StyleRunes(truncated, kept, matched, unmatched)
}

func startInlineStyle(st lipgloss.Style) lipgloss.Style {
	return st.UnsetPadding().UnsetBorderStyle()
}

func padStartField(field string, width int, fill lipgloss.Style) string {
	if pad := width - lipgloss.Width(field); pad > 0 {
		return field + fill.Render(strings.Repeat(" ", pad))
	}
	return field
}

func (m startModel) update(msg tea.Msg) (startModel, tea.Cmd) {
	beforeState := m.list.FilterState()
	beforeIndex := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.list.FilterState() != beforeState {
		m.setSize(m.common.width, m.common.bodyHeight())
	}
	if _, ok := msg.(list.FilterMatchesMsg); ok {
		// Filtering removes headers. The unfiltered cursor starts
		// at 1 to skip the leading header, so reset it to the first filtered row.
		m.list.Select(0)
		m.skipNonEntry(1)
		return m, cmd
	}
	// Skip whenever the cursor is left on a non-entry row, not only when the
	// index moved: clearing a zero-match filter resets to index 0 — the leading
	// header — without changing the index at all.
	dir := 1
	if after := m.list.Index(); after < beforeIndex {
		dir = -1
	}
	if _, ok := m.selected(); !ok {
		m.skipNonEntry(dir)
	}
	return m, cmd
}

// skipNonEntry advances past a header in the direction of travel, reversing at
// the ends so the cursor can never rest on a non-entry row.
func (m *startModel) skipNonEntry(dir int) {
	items := m.list.VisibleItems()
	if len(items) == 0 {
		return
	}
	idx := m.list.Index()
	for range len(items) {
		it, ok := items[idx].(startItem)
		if !ok || it.selectable() {
			m.list.Select(idx)
			return
		}
		idx += dir
		if idx < 0 || idx >= len(items) {
			dir = -dir
			idx += 2 * dir // step back inside, then continue the other way
			if idx < 0 || idx >= len(items) {
				return
			}
		}
	}
}

// noMatchMessage is the first content row while a typed filter matches nothing.
// Bubbles leaves that region blank; name the query, not the catalog.
func (m startModel) noMatchMessage() string {
	if m.list.FilterState() != list.Filtering || len(m.list.VisibleItems()) > 0 {
		return ""
	}
	query := strings.TrimSpace(m.list.FilterValue())
	if query == "" {
		return "" // "/" pressed with nothing typed: every row is still on screen
	}
	// PaddingLeft(2) puts the message on the same left edge as the entry rows it
	// replaces and the filter prompt above it.
	style := m.list.Styles.NoItems.PaddingLeft(2)
	return ansi.Truncate(style.Render("no match for “"+query+"”"), m.list.Width(), "…")
}

// filterPromptHeight is the TitleBar-wrapped filter input. Derive the offset so
// the message stays under the prompt; do not hardcode 2.
func (m startModel) filterPromptHeight() int {
	return lipgloss.Height(m.list.Styles.TitleBar.Render(m.list.FilterInput.View()))
}

// replaceLine overwrites line n of view, appending when the view is shorter so
// the caller's line can never be silently dropped.
func replaceLine(view string, n int, line string) string {
	lines := strings.Split(view, "\n")
	if n < 0 || n >= len(lines) {
		return view + "\n" + line
	}
	lines[n] = line
	return strings.Join(lines, "\n")
}

// View shows the file-level empty state only when the startpage has no rows AT
// ALL. Gating on VisibleItems() instead would fire on a zero-match filter, where
// it would assert something false ("no bookmarks yet") and swallow the filter
// input the user is still typing into.
func (m startModel) View() string {
	body := m.empty
	if len(m.list.Items()) > 0 || m.empty == "" {
		body = m.list.View()
		if message := m.noMatchMessage(); message != "" {
			body = replaceLine(body, m.filterPromptHeight(), message)
		}
		if overview := m.overviewView(); overview != "" {
			body = overview + "\n" + body
		}
	}
	if m.notice != "" {
		body = m.notice + "\n\n" + body
	}
	if missing := m.height - lipgloss.Height(body); missing > 0 {
		body += strings.Repeat("\n", missing)
	}
	return body
}

func (m *startModel) setSize(width, height int) {
	m.common.width = width
	if height < 1 {
		height = 1
	}
	m.height = height
	if m.list.FilterState() == list.Unfiltered {
		position, hasSelection := m.captureTogglePosition()
		m.list.SetItems(startItems(m.sections, width))
		if hasSelection {
			m.selectSectionPosition(position.full)
		}
	}
	h := height - startChromeRows - m.noticeHeight() - m.overviewHeight()
	if h < 1 {
		h = 1
	}
	m.list.SetDelegate(newStartDelegate(m.common, m.common.ensureStyles()))
	m.list.SetSize(width, h)
}

// The overview's two labels. The ownership one is deliberately not the word
// "BOOKMARKS": the section header three lines below already says that, and at
// the top of the page, where a reader is orienting, the repetition was the
// loudest thing on screen. Naming whose the rows are also says something the
// header does not.
const (
	overviewOwnershipLabel = "YOURS"
	overviewCatalogLabel   = "CATALOG"
)

// overviewLabel renders a label with the gap that follows it. Stacked (narrow),
// the labels are different lengths and their values belong in one column, so it
// pads to the longer label; on one line the groups are separated by │ and
// alignment means nothing, so it pads to a plain two spaces.
func (m startModel) overviewLabel(label string, style lipgloss.Style) string {
	column := lipgloss.Width(label)
	if m.common.width < startWideMinWidth {
		column = max(lipgloss.Width(overviewOwnershipLabel), lipgloss.Width(overviewCatalogLabel))
	}
	return style.Render(label) + strings.Repeat(" ", column-lipgloss.Width(label)+2)
}

func (m startModel) overviewCounts() startOverviewCounts {
	if m.list.FilterState() == list.FilterApplied {
		return startCounts(m.list.VisibleItems())
	}
	return m.assembledCounts
}

func (m startModel) overviewView() string {
	if m.list.FilterState() == list.Filtering || len(m.list.Items()) == 0 {
		return ""
	}

	counts := m.overviewCounts()
	filtered := m.list.FilterState() == list.FilterApplied
	st := m.common.ensureStyles()
	labelStyle := lipgloss.NewStyle().Foreground(st.palette.Dim)
	valueStyle := lipgloss.NewStyle().Foreground(st.palette.Text)
	selectedSection := sectionUnknown
	if m.common.contentFocused {
		if selected, ok := m.list.SelectedItem().(startItem); ok && selected.selectable() {
			selectedSection = selected.section
		}
	}
	gold := lipgloss.NewStyle().Foreground(st.palette.AccentGold).Bold(true)

	var groups []string
	if !filtered || counts.bookmarks > 0 {
		bookmarksStyle := labelStyle
		if selectedSection == sectionBookmarks {
			bookmarksStyle = gold
		}
		value := "none yet"
		if counts.bookmarks > 0 {
			value = fmt.Sprintf("%d", counts.bookmarks)
		}
		groups = append(groups, m.overviewLabel(overviewOwnershipLabel, bookmarksStyle)+valueStyle.Render(value))
	}

	var catalogValues []string
	if counts.communities > 0 {
		style := valueStyle
		if selectedSection == sectionCommunities {
			style = gold
		}
		catalogValues = append(catalogValues, style.Render(countLabel(counts.communities, "community", "communities")))
	}
	if counts.services > 0 {
		style := valueStyle
		if selectedSection == sectionServices {
			style = gold
		}
		catalogValues = append(catalogValues, style.Render(countLabel(counts.services, "service", "services")))
	}
	if len(catalogValues) > 0 {
		groups = append(groups, m.overviewLabel(overviewCatalogLabel, labelStyle)+strings.Join(catalogValues, " · "))
	}
	if len(groups) == 0 {
		return ""
	}

	separator := " │ "
	if m.common.width < startWideMinWidth {
		separator = "\n"
	}
	lines := strings.Split(strings.Join(groups, separator), "\n")
	for i, line := range lines {
		// common.width, not list.Width(): setSize measures the overview
		// before the list has been given this frame's width, and a zero
		// list width collapses the truncated line to empty so the first
		// layout forgets to reserve the overview row.
		lines[i] = ansi.Truncate(line, m.common.width, "…")
	}
	return strings.Join(lines, "\n")
}

func (m startModel) overviewHeight() int {
	view := m.overviewView()
	if view == "" {
		return 0
	}
	return len(strings.Split(view, "\n"))
}

func (m startModel) noticeHeight() int {
	if m.notice == "" {
		return 0
	}
	return len(strings.Split(m.notice, "\n")) + 1
}

// selected returns the highlighted entry. A non-entry row or empty list yields false.
func (m startModel) selected() (startEntry, bool) {
	it, ok := m.list.SelectedItem().(startItem)
	if !ok || !it.selectable() {
		return startEntry{}, false
	}
	return it.entry, true
}

// selectTarget restores selection by stable identity after a startpage reload.
// Bookmark toggles deliberately restore by section and ordinal instead, because
// toggling moves the acted-on target between BOOKMARKS and the catalog.
func (m *startModel) selectTarget(target string) bool {
	for i, item := range m.list.VisibleItems() {
		entry, ok := item.(startItem)
		if ok && entry.selectable() && entry.entry.target == target {
			m.list.Select(i)
			return true
		}
	}
	return false
}

// captureTogglePosition records the selected row's ordinal within its section
// in both the assembled page and, when applied, bubbles' fuzzy-ranked results.
func (m startModel) captureTogglePosition() (startTogglePosition, bool) {
	selected, ok := m.list.SelectedItem().(startItem)
	if !ok || !selected.selectable() {
		return startTogglePosition{}, false
	}

	position := startTogglePosition{
		full: startSectionPosition{section: selected.section},
	}
	globalIndex := m.list.GlobalIndex()
	for index, item := range m.list.Items() {
		if index >= globalIndex {
			break
		}
		candidate, ok := item.(startItem)
		if !ok || !candidate.selectable() || candidate.section != selected.section {
			continue
		}
		position.full.ordinal++
	}

	if m.list.FilterState() == list.FilterApplied {
		filtered := startSectionPosition{section: selected.section}
		visibleIndex := m.list.Index()
		for index, item := range m.list.VisibleItems() {
			if index >= visibleIndex {
				break
			}
			candidate, ok := item.(startItem)
			if !ok || !candidate.selectable() || candidate.section != selected.section {
				continue
			}
			filtered.ordinal++
		}
		position.filtered = &filtered
		position.filter = m.list.FilterValue()
	}
	return position, true
}

// selectSectionPosition restores a section-relative ordinal, preferring the
// next section when that section disappears and the nearest previous one when
// there is no later selectable section.
func (m *startModel) selectSectionPosition(position startSectionPosition) bool {
	indexes := make(map[startSectionID][]int, len(startSectionOrder))
	for index, item := range m.list.VisibleItems() {
		candidate, ok := item.(startItem)
		if !ok || !candidate.selectable() {
			continue
		}
		indexes[candidate.section] = append(indexes[candidate.section], index)
	}

	requested := -1
	for i, section := range startSectionOrder {
		if section == position.section {
			requested = i
			break
		}
	}
	if requested < 0 {
		return false
	}

	section := position.section
	if len(indexes[section]) == 0 {
		section = sectionUnknown
		for _, candidate := range startSectionOrder[requested+1:] {
			if len(indexes[candidate]) > 0 {
				section = candidate
				break
			}
		}
		if section == sectionUnknown {
			for i := requested - 1; i >= 0; i-- {
				candidate := startSectionOrder[i]
				if len(indexes[candidate]) > 0 {
					section = candidate
					break
				}
			}
		}
	}

	sectionIndexes := indexes[section]
	if len(sectionIndexes) == 0 {
		return false
	}
	ordinal := position.ordinal
	if ordinal < 0 {
		ordinal = 0
	}
	if ordinal >= len(sectionIndexes) {
		ordinal = len(sectionIndexes) - 1
	}
	m.list.Select(sectionIndexes[ordinal])
	return true
}

func (m startModel) filtering() bool {
	return m.list.FilterState() == list.Filtering
}

func (m startModel) filterApplied() bool {
	return m.list.FilterState() == list.FilterApplied
}

func (m *startModel) applyStyles(st styles) {
	applyListStyles(&m.list, st)
	m.list.SetDelegate(newStartDelegate(m.common, st))
}

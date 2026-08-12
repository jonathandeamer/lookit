package tui

import (
	"fmt"
	"io"
	"strings"
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
)

// startItem is one row: an entry or section header. Non-entry rows occupy
// normal item slots so the list's uniform-height pagination holds.
type startItem struct {
	entry   startEntry
	header  string // non-empty => this row is a section heading
	section startSectionID
}

func (i startItem) selectable() bool {
	return i.header == "" && i.entry.target != ""
}

// FilterValue drives "/". Non-entry rows return "" so they drop out while
// filtering, which flattens the view to matches — the behaviour we want.
func (i startItem) FilterValue() string {
	if !i.selectable() {
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
	common *commonModel
	list   list.Model
	notice string // parse problems, surfaced rather than swallowed
	empty  string // shown instead of the list when there is nothing to show
}

func newStart(common *commonModel, sections []startSection, notice, empty string) startModel {
	var items []list.Item
	for _, s := range sections {
		items = append(items, startItem{header: s.title, section: s.id})
		for _, e := range s.entries {
			items = append(items, startItem{entry: e, section: s.id})
		}
	}

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

	m := startModel{common: common, list: l, notice: notice, empty: empty}
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
	if it.header != "" {
		d.renderHeader(w, m.Width(), it.header)
		return
	}
	if !it.selectable() {
		return
	}
	d.renderEntry(w, m, index, it)
}

func (d startDelegate) renderHeader(w io.Writer, width int, header string) {
	if d.Height() == 2 {
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

	titleStyle, descStyle := d.st.listItem.NormalTitle, d.st.listItem.NormalDesc
	if emptyFilter {
		titleStyle, descStyle = d.st.listItem.DimmedTitle, d.st.listItem.DimmedDesc
	} else if isSelected && m.FilterState() != list.Filtering {
		titleStyle, descStyle = d.st.listItem.SelectedTitle, d.st.listItem.SelectedDesc
	}
	titleStyle = startStyleWithinWidth(titleStyle, m.Width())
	descStyle = startStyleWithinWidth(descStyle, m.Width())

	var targetMatches, noteMatches []int
	if isFiltered && !emptyFilter {
		targetMatches, noteMatches = splitStartMatches(m.MatchesForItem(index), item.entry.target)
	}

	if d.Height() == 1 {
		frame := titleStyle.GetHorizontalFrameSize()
		targetWidth, noteWidth := startColumnWidths(m.Width(), frame)
		target := renderStartField(item.entry.target, targetWidth, targetMatches, titleStyle, d.st.listItem.FilterMatch)
		note := renderStartField(item.entry.note, noteWidth, noteMatches, descStyle, d.st.listItem.FilterMatch)
		target = padStartField(target, targetWidth, startInlineStyle(titleStyle))
		row := target + note
		if isSelected && m.FilterState() != list.Filtering {
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
	target := renderStartField(item.entry.target, titleWidth, targetMatches, titleStyle, d.st.listItem.FilterMatch)
	note := renderStartField(item.entry.note, descWidth, noteMatches, descStyle, d.st.listItem.FilterMatch)
	if isSelected && m.FilterState() != list.Filtering {
		fmt.Fprintf(w, "%s\n%s", renderSelectedShelfLine(target, titleStyle, m.Width()), renderSelectedShelfLine(note, descStyle, m.Width())) //nolint:errcheck
		return
	}
	fmt.Fprintf(w, "%s\n%s", titleStyle.Render(target), descStyle.Render(note)) //nolint:errcheck
}

func startColumnWidths(width, frame int) (int, int) {
	available := width - frame
	if available < 0 {
		available = 0
	}
	target := available * startTargetColumnPct / 100
	return target, available - target
}

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
	before := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
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
	if after := m.list.Index(); after < before {
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

// View shows the file-level empty state only when the startpage has no rows AT
// ALL. Gating on VisibleItems() instead would fire on a zero-match filter, where
// it would assert something false ("no bookmarks yet") and swallow the filter
// input the user is still typing into. A filter that matches nothing keeps the
// list, which renders the filter input plus its own "No entries." (Styles.NoItems).
func (m startModel) View() string {
	body := m.empty
	if len(m.list.Items()) > 0 || m.empty == "" {
		body = m.list.View()
	}
	if m.notice != "" {
		return m.notice + "\n\n" + body
	}
	return body
}

func (m *startModel) setSize(width, height int) {
	m.common.width = width
	h := height - startChromeRows - m.noticeHeight()
	if h < 1 {
		h = 1
	}
	m.list.SetDelegate(newStartDelegate(m.common, m.common.ensureStyles()))
	m.list.SetSize(width, h)
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

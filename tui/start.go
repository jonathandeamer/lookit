package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// startChromeRows matches listChromeRows: space the bubbles list reserves once
// its own title and help are hidden.
const (
	startChromeRows  = 1
	catalogCreditURL = "https://640kb.neocities.org/fingerverse/"
)

// startItem is one row: an entry, section header, or catalog credit. Non-entry
// rows occupy normal item slots so the list's uniform-height pagination holds.
type startItem struct {
	entry  startEntry
	header string // non-empty => this row is a section heading
	credit bool
}

func (i startItem) selectable() bool {
	return i.header == "" && !i.credit && i.entry.target != ""
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
	if i.credit {
		return "Catalog inspired by"
	}
	return i.entry.target
}

func (i startItem) Description() string {
	if i.header != "" {
		return ""
	}
	if i.credit {
		return catalogCreditURL
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
	hasCatalogRow := false
	for _, s := range sections {
		items = append(items, startItem{header: s.title})
		for _, e := range s.entries {
			items = append(items, startItem{entry: e})
			if e.source == sourceCatalog {
				hasCatalogRow = true
			}
		}
	}
	if hasCatalogRow {
		items = append(items, startItem{credit: true})
	}

	st := common.ensureStyles()
	height := common.bodyHeight() - startChromeRows
	if height < 1 {
		height = 1
	}
	l := list.New(items, startDelegate{userDelegate: defaultUserDelegate(st), st: st}, common.width, height)
	applyListStyles(&l, st)
	// NOT redundant with the delegate passed to list.New: applyListStyles ends
	// with SetDelegate(defaultUserDelegate(st)) (tui/list.go:141), which clobbers
	// it. The startpage delegate must be reinstated after every style pass — see
	// applyStyles below, which orders it the same way for the same reason.
	l.SetDelegate(startDelegate{userDelegate: defaultUserDelegate(st), st: st})
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

// startDelegate renders headers itself and defers entries to the existing user
// delegate, so startpage rows look exactly like user-list rows.
type startDelegate struct {
	userDelegate
	st styles
}

func (d startDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	if it, ok := item.(startItem); ok && it.header != "" {
		// Two rows, matching the entry cell height that pagination assumes.
		fmt.Fprintf(w, "\n%s", d.st.barFlag.Render(it.header)) //nolint:errcheck
		return
	}
	if it, ok := item.(startItem); ok && it.credit {
		dim := lipgloss.NewStyle().Foreground(d.st.palette.Dim)
		url := lipgloss.NewStyle().Hyperlink(catalogCreditURL).Render(catalogCreditURL)
		fmt.Fprintf(w, "%s\n%s", dim.Render("Catalog inspired by"), dim.Render(url)) //nolint:errcheck
		return
	}
	d.userDelegate.Render(w, m, index, item)
}

func (m startModel) update(msg tea.Msg) (startModel, tea.Cmd) {
	before := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if _, ok := msg.(list.FilterMatchesMsg); ok {
		// Filtering removes headers and the credit. The unfiltered cursor starts
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

// skipNonEntry advances past a header or credit in the direction of travel,
// reversing at the ends so the cursor can never rest on a non-entry row.
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
	h := height - startChromeRows - m.noticeHeight()
	if h < 1 {
		h = 1
	}
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
	m.list.SetDelegate(startDelegate{userDelegate: defaultUserDelegate(st), st: st})
}

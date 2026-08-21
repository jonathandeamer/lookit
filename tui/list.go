package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jonathandeamer/lookit/finger"
)

// listChromeRows reserves space for list internals after title/help are hidden.
const listChromeRows = 1
const maxPreambleRows = 12

// maxListEntries bounds how many parsed users we turn into Bubble list items.
// A hostile host can pack a 1 MiB response with tens of thousands of distinct
// logins; capping keeps list/filter state bounded so the TUI can't be frozen.
const maxListEntries = 2000

// filterPrompt is the filter prompt used by lookit.
const filterPrompt = "filter: "

// userItem is one selectable user in the list.
type userItem struct {
	login  string
	name   string
	target string
}

// FilterValue lets the list filter by login as the user types "/", plus the
// target where there is one, so a cross-host listing can be filtered by host.
// The login comes first and unchanged: list.DefaultDelegate maps the matched
// rune indices straight onto Title(), so leading with the login keeps a login
// match highlighted on the row, and a host match simply falls past the end of
// the title rather than colouring the wrong characters.
func (i userItem) FilterValue() string {
	if i.target != "" {
		return i.login + " " + i.target
	}
	return i.login
}

// Title satisfies list.DefaultItem — the primary line is the login.
func (i userItem) Title() string { return i.login }

// Description satisfies list.DefaultItem — shows name and/or target if present.
func (i userItem) Description() string {
	var parts []string
	if i.name != "" {
		parts = append(parts, i.name)
	}
	if i.target != "" {
		parts = append(parts, i.target)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}

// listModel wraps a bubbles list of a host's users.
type listModel struct {
	common   *commonModel
	list     list.Model
	host     finger.Target
	preamble string
	generic  bool
}

func newList(common *commonModel, host finger.Target, users []User) listModel {
	if len(users) > maxListEntries {
		users = users[:maxListEntries]
	}
	items := make([]list.Item, len(users))
	for i, u := range users {
		items[i] = userItem{login: u.Login, name: u.Name, target: u.Target}
	}

	width := common.width
	height := common.bodyHeight() - listChromeRows
	if height < 1 {
		height = 1
	}

	st := common.ensureStyles()
	d := defaultUserDelegate(st)
	l := list.New(items, d, width, height)
	applyListStyles(&l, st)
	l.Title = fmt.Sprintf("%s — %s", host.Raw, countLabel(len(users), "user", "users"))
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)

	return listModel{common: common, list: l, host: host}
}

type userDelegate struct {
	list.DefaultDelegate
}

func defaultUserDelegate(st styles) userDelegate {
	d := list.NewDefaultDelegate()
	d.Styles = st.listItem
	d.SetSpacing(0) // drop the blank line between items: 3 rows/item -> 2 (tighter)
	return userDelegate{DefaultDelegate: d}
}

func (d userDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	if index != m.Index() || m.FilterState() == list.Filtering {
		d.DefaultDelegate.Render(w, m, index, item)
		return
	}

	i, ok := item.(list.DefaultItem)
	if !ok || m.Width() <= 0 {
		return
	}

	title := renderSelectedShelfLine(i.Title(), d.Styles.SelectedTitle, m.Width())
	if !d.ShowDescription {
		fmt.Fprint(w, title) //nolint:errcheck
		return
	}

	desc := firstDescriptionLine(i.Description())
	fmt.Fprintf(w, "%s\n%s", title, renderSelectedShelfLine(desc, d.Styles.SelectedDesc, m.Width())) //nolint:errcheck
}

func firstDescriptionLine(desc string) string {
	if desc == "" {
		return ""
	}
	return strings.Split(desc, "\n")[0]
}

func renderSelectedShelfLine(text string, st lipgloss.Style, width int) string {
	contentWidth := width - st.GetHorizontalFrameSize()
	if contentWidth < 0 {
		contentWidth = 0
	}
	return st.Width(width).MaxWidth(width).Render(ansi.Truncate(text, contentWidth, "..."))
}

func applyListStyles(l *list.Model, st styles) {
	l.Styles = st.list
	l.FilterInput.SetStyles(st.list.Filter)
	l.Help.Styles = st.help
	l.FilterInput.Prompt = filterPrompt
	l.Paginator.ActiveDot = st.list.ActivePaginationDot.String()
	l.Paginator.InactiveDot = st.list.InactivePaginationDot.String()
	l.SetDelegate(defaultUserDelegate(st))
}

func (m *listModel) applyStyles(st styles) {
	applyListStyles(&m.list, st)
}

func newListWithPreamble(common *commonModel, host finger.Target, users []User, body []byte, generic bool) listModel {
	preamble := ""
	if parsed, ok := parseUserList(body, host.HostPort); ok {
		preamble = parsed.preamble
	} else {
		preamble = extractListPreamble(body)
	}
	return newListWithPreambleText(common, host, users, preamble, generic)
}

func newListFromParsed(common *commonModel, host finger.Target, parsed parsedUserList) listModel {
	return newListWithPreambleText(common, host, parsed.users, parsed.preamble, parsed.generic)
}

func newListWithPreambleText(common *commonModel, host finger.Target, users []User, preamble string, generic bool) listModel {
	total := len(users)
	m := newList(common, host, users)
	m.generic = generic
	m.preamble = preamble
	if generic {
		note := "Auto-detected user list from an unrecognized response — press v to view source."
		if m.preamble != "" {
			m.preamble = note + "\n\n" + m.preamble
		} else {
			m.preamble = note
		}
	}
	if total > maxListEntries {
		note := fmt.Sprintf("List truncated — showing first %d of %d", maxListEntries, total)
		if m.preamble != "" {
			m.preamble = note + "\n\n" + m.preamble
		} else {
			m.preamble = note
		}
	}
	m.setSize(common.width, common.bodyHeight())
	return m
}

func (m listModel) update(msg tea.Msg) (listModel, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m listModel) View() string {
	if m.preamble != "" {
		return m.visiblePreamble() + "\n\n" + m.list.View()
	}
	return m.list.View()
}

func (m *listModel) setSize(width, height int) {
	h := height - listChromeRows - m.preambleHeight()
	if h < 1 {
		h = 1
	}
	m.list.SetSize(width, h)
}

func (m listModel) visiblePreamble() string {
	lines := strings.Split(m.preamble, "\n")
	limit := maxPreambleRows
	if m.common != nil && m.common.height > 0 {
		limit = m.common.height / 2
		if limit < 3 {
			limit = 3
		}
		if limit > maxPreambleRows {
			limit = maxPreambleRows
		}
	}
	if len(lines) <= limit {
		return m.preamble
	}
	out := append([]string{}, lines[:limit-1]...)
	out = append(out, "...")
	return strings.Join(out, "\n")
}

func (m listModel) preambleHeight() int {
	if m.preamble == "" {
		return 0
	}
	return len(strings.Split(m.visiblePreamble(), "\n")) + 1
}

// selected returns the highlighted user, if any.
func (m listModel) selected() (userItem, bool) {
	it, ok := m.list.SelectedItem().(userItem)
	return it, ok
}

func sameUserIdentity(want, candidate userItem) bool {
	if want.target != "" {
		return candidate.target == want.target
	}
	return candidate.login == want.login
}

func (m *listModel) selectIdentity(want userItem) {
	for i, raw := range m.list.VisibleItems() {
		candidate, ok := raw.(userItem)
		if ok && sameUserIdentity(want, candidate) {
			m.list.Select(i)
			return
		}
	}
	m.list.Select(0)
}

// filtering reports whether the user is actively typing a filter.
func (m listModel) filtering() bool {
	return m.list.FilterState() == list.Filtering
}

// acceptFilter applies a bubbles list filter synchronously, and reports whether
// the list ended up with a filter applied.
//
// bubbles computes matches in a tea.Cmd, and its own accept path
// (list.handleFiltering) flips the list to FilterApplied using whatever match
// set is installed at that moment. When the accept key arrives before the
// FilterMatchesMsg for the query already on screen — a terminal delivering
// "/bo\r\r" in a single read — the applied selection is the *pre-filter* row,
// and the next key acts on it: Enter fingers the wrong account, b bookmarks a
// target the user never highlighted (issue #129). The late message then lands
// and the list visibly settles on the right row, which is what makes the
// mismatch so hard to read as a bug.
//
// SetFilterText runs the same filter synchronously, so re-applying the query
// that is already in the input settles the matches before anything can act on
// the selection. The two cases bubbles special-cases on accept are reproduced
// here: an empty query and a query matching nothing both clear the filter
// rather than applying it.
func acceptFilter(l *list.Model) bool {
	if l.FilterValue() == "" {
		l.ResetFilter()
		return false
	}
	l.SetFilterText(l.FilterValue())
	if len(l.VisibleItems()) == 0 {
		l.ResetFilter()
		return false
	}
	l.FilterInput.Blur()
	return true
}

func extractListPreamble(body []byte) string {
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	if preamble, ok := columnarPreamble(lines); ok {
		return preamble
	}
	if preamble, ok := gridPreamble(lines); ok {
		return preamble
	}
	if preamble, ok := markerPreamble(lines); ok {
		return preamble
	}
	return ""
}

func columnarPreamble(lines []string) (string, bool) {
	for i, ln := range lines {
		fields := strings.Fields(ln)
		if len(fields) > 0 && strings.EqualFold(fields[0], "Login") {
			return trimPreamble(lines[:i]), true
		}
	}
	return "", false
}

func gridPreamble(lines []string) (string, bool) {
	for i, ln := range lines {
		if !gridCueRe.MatchString(ln) {
			continue
		}
		end := i + 1
		if end < len(lines) && strings.TrimSpace(lines[end]) == "" {
			end++
		}
		return trimPreamble(lines[:end]), true
	}
	return "", false
}

func markerPreamble(lines []string) (string, bool) {
	for i, ln := range lines {
		if markerRe.MatchString(ln) {
			return trimPreamble(lines[:i]), true
		}
	}
	return "", false
}

func trimPreamble(lines []string) string {
	text := strings.TrimRight(strings.Join(lines, "\n"), "\n")
	return strings.TrimSpace(text)
}

package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/jonathandeamer/lookit/finger"
)

// listChromeRows reserves space for list internals after title/help are hidden.
const listChromeRows = 1
const maxPreambleRows = 12

// userItem is one selectable user in the list.
type userItem struct {
	login  string
	name   string
	target string
}

// FilterValue lets the list filter by login as the user types "/".
func (i userItem) FilterValue() string { return i.login }

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
	items := make([]list.Item, len(users))
	for i, u := range users {
		items[i] = userItem{login: u.Login, name: u.Name, target: u.Target}
	}

	width := common.width
	height := common.bodyHeight() - listChromeRows
	if height < 1 {
		height = 1
	}

	st := common.styles
	d := defaultUserDelegate(st)
	l := list.New(items, d, width, height)
	applyListStyles(&l, st)
	l.Title = fmt.Sprintf("%s — %d users", host.Raw, len(users))
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)

	return listModel{common: common, list: l, host: host}
}

func defaultUserDelegate(st styles) list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles = st.listItem
	d.SetSpacing(0) // drop the blank line between items: 3 rows/item -> 2 (tighter)
	return d
}

func applyListStyles(l *list.Model, st styles) {
	l.Styles = st.list
	l.FilterInput.SetStyles(st.list.Filter)
	l.Help.Styles = st.help
	l.Paginator.ActiveDot = st.list.ActivePaginationDot.String()
	l.Paginator.InactiveDot = st.list.InactivePaginationDot.String()
	l.SetDelegate(defaultUserDelegate(st))
}

func (m *listModel) applyStyles(st styles) {
	applyListStyles(&m.list, st)
}

func newListWithPreamble(common *commonModel, host finger.Target, users []User, body []byte, generic bool) listModel {
	m := newList(common, host, users)
	m.generic = generic
	if parsed, ok := parseUserList(body); ok {
		m.preamble = parsed.preamble
	} else {
		m.preamble = extractListPreamble(body)
	}
	if generic {
		note := "Auto-detected user list from an unrecognized response — press r to view raw."
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

// filtering reports whether the user is actively typing a filter.
func (m listModel) filtering() bool {
	return m.list.FilterState() == list.Filtering
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

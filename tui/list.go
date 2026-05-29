package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/jonathandeamer/lookit/finger"
)

// listChromeRows reserves space for the list title and footer when sizing.
const listChromeRows = 4
const maxPreambleRows = 12

// userItem is one selectable user in the list.
type userItem struct {
	login string
	name  string
}

// FilterValue lets the list filter by login as the user types "/".
func (i userItem) FilterValue() string { return i.login }

// userDelegate renders one user per line: "> login   name".
type userDelegate struct {
	styles styles
}

func (d userDelegate) Height() int                         { return 1 }
func (d userDelegate) Spacing() int                        { return 0 }
func (d userDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d userDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(userItem)
	if !ok {
		return
	}
	cursor := "  "
	login := it.login
	if index == m.Index() {
		cursor = "> "
		login = d.styles.selected.Render(it.login)
	}
	line := login
	if it.name != "" {
		line += "  " + d.styles.listName.Render(it.name)
	}
	fmt.Fprint(w, cursor+line)
}

// listModel wraps a bubbles list of a host's users.
type listModel struct {
	common   *commonModel
	list     list.Model
	host     finger.Target
	preamble string
}

func newList(common *commonModel, host finger.Target, users []User) listModel {
	items := make([]list.Item, len(users))
	for i, u := range users {
		items[i] = userItem{login: u.Login, name: u.Name}
	}

	width := common.width
	height := common.height - listChromeRows
	if height < 1 {
		height = 1
	}

	l := list.New(items, userDelegate{styles: newStyles()}, width, height)
	l.Title = fmt.Sprintf("%s — %d users", host.Raw, len(users))
	l.SetShowStatusBar(false)
	l.SetShowHelp(true)

	return listModel{common: common, list: l, host: host}
}

func newListWithPreamble(common *commonModel, host finger.Target, users []User, body []byte) listModel {
	m := newList(common, host, users)
	m.preamble = extractListPreamble(body)
	m.setSize(common.width, common.height)
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

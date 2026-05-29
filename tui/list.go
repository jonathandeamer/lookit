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
	line := it.login
	if it.name != "" {
		line += "  " + d.styles.listName.Render(it.name)
	}
	cursor := "  "
	if index == m.Index() {
		cursor = "> "
		line = d.styles.selected.Render(it.login)
		if it.name != "" {
			line += "  " + d.styles.listName.Render(it.name)
		}
	}
	fmt.Fprint(w, cursor+line)
}

// listModel wraps a bubbles list of a host's users.
type listModel struct {
	common *commonModel
	list   list.Model
	host   finger.Target
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

func (m listModel) update(msg tea.Msg) (listModel, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m listModel) View() string {
	var b strings.Builder
	b.WriteString(m.list.View())
	return b.String()
}

func (m *listModel) setSize(width, height int) {
	h := height - listChromeRows
	if h < 1 {
		h = 1
	}
	m.list.SetSize(width, h)
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

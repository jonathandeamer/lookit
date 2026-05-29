package tui

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

// appState selects which sub-model is active.
type appState int

const (
	stateReader appState = iota
	stateList
)

// commonModel is state shared across sub-models.
type commonModel struct {
	width   int
	height  int
	profile colorprofile.Profile
	fetch   FetchFunc
}

// appModel is the top-level state machine. It routes input and fetch results
// between the reader and the list, and owns quit/back behavior.
type appModel struct {
	common *commonModel
	state  appState
	reader readerModel
	list   listModel

	// hostList caches the most recent host response so Back from a drilled
	// user is instant; fromList is true when the reader shows a drilled user.
	hostList *Entry
	fromList bool
}

func newApp(fetch FetchFunc, profile colorprofile.Profile) appModel {
	if fetch == nil {
		fetch = defaultFetch
	}
	common := &commonModel{profile: profile, fetch: fetch}
	return appModel{
		common: common,
		state:  stateReader,
		reader: newReader(fetch, profile),
	}
}

func (m appModel) Init() tea.Cmd {
	return tea.Batch(
		m.reader.Init(),
		tea.RequestCapability("RGB"),
		tea.RequestCapability("Tc"),
	)
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.common.width = msg.Width
		m.common.height = msg.Height
		m.reader.setSize(msg.Width, msg.Height)
		// Resize the list only once it exists; a freshly-opened list is sized
		// from common in newList.
		if m.hostList != nil {
			m.list.setSize(msg.Width, msg.Height)
		}
		return m, nil

	case tea.ColorProfileMsg:
		m.common.profile = msg.Profile
		m.reader.setProfile(msg.Profile)
		return m, nil

	case tea.KeyPressMsg:
		// handleKey may mutate the model (e.g. clearing fromList on a fresh
		// Enter) even when it does not fully handle the key, so adopt its
		// returned model before deciding whether to delegate.
		handled, model, cmd := m.handleKey(msg)
		m = model.(appModel)
		if handled {
			return m, cmd
		}

	case fetchResultMsg:
		return m.routeFetch(msg.entry), nil
	}

	// Delegate to the active sub-model.
	var cmd tea.Cmd
	switch m.state {
	case stateList:
		m.list, cmd = m.list.update(msg)
	default:
		m.reader, cmd = m.reader.update(msg)
	}
	return m, cmd
}

// handleKey processes cross-screen keys (quit, back, drill). It returns
// handled=false to let the active sub-model handle the key.
func (m appModel) handleKey(msg tea.KeyPressMsg) (bool, tea.Model, tea.Cmd) {
	key := msg.Key()

	// Ctrl+C always quits.
	if key.Code == 'c' && key.Mod == tea.ModCtrl {
		return true, m, tea.Quit
	}

	switch m.state {
	case stateList:
		// While typing a filter, let the list own Esc/Enter.
		if m.list.filtering() {
			return false, m, nil
		}
		switch key.Code {
		case tea.KeyEsc:
			// Esc clears an applied filter first; only backs out when unfiltered.
			if m.list.list.FilterState() != list.Unfiltered {
				return false, m, nil
			}
			m.state = stateReader
			m.fromList = false
			return true, m, nil
		case tea.KeyEnter:
			return m.drill()
		}

	case stateReader:
		if key.Code == tea.KeyEsc {
			if m.fromList {
				m.state = stateList
				m.fromList = false
				return true, m, nil
			}
			return true, m, tea.Quit
		}
		if key.Code == tea.KeyEnter {
			// A fresh manual fetch from the input clears any drill context.
			m.fromList = false
		}
	}

	return false, m, nil
}

// drill fingers the highlighted user as login@host and switches to the reader.
func (m appModel) drill() (bool, tea.Model, tea.Cmd) {
	sel, ok := m.list.selected()
	if !ok {
		return true, m, nil
	}
	// Build login@host from the host's original argument (minus the leading
	// "@"), preserving any explicit :port the user typed.
	host := strings.TrimPrefix(m.list.host.Raw, "@")
	target, err := finger.ParseTarget(sel.login + "@" + host)
	if err != nil {
		return true, m, nil
	}
	m.reader.setLoading(target)
	m.state = stateReader
	m.fromList = true
	return true, m, fetchCmd(context.Background(), m.common.fetch, target)
}

// routeFetch is the single decision point for a completed fetch: a host
// response that parses opens the list; everything else renders in the reader.
func (m appModel) routeFetch(entry Entry) appModel {
	m.reader.loading = false
	if entry.Err == nil && entry.Target.User == "" {
		if users, ok := ParseUsers(entry.Body); ok {
			cached := entry
			m.hostList = &cached
			m.list = newList(m.common, entry.Target, users)
			m.state = stateList
			m.fromList = false
			return m
		}
	}
	m.reader.setEntry(entry)
	m.state = stateReader
	return m
}

func (m appModel) View() tea.View {
	var content string
	switch m.state {
	case stateList:
		content = m.list.View()
	default:
		content = m.reader.View()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

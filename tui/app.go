package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
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
		return m, nil

	case tea.ColorProfileMsg:
		m.common.profile = msg.Profile
		m.reader.setProfile(msg.Profile)
		return m, nil

	case tea.KeyPressMsg:
		key := msg.Key()
		// Ctrl+C always quits.
		if key.Code == 'c' && key.Mod == tea.ModCtrl {
			return m, tea.Quit
		}
		// Reader home: Esc quits (Phase 2 behavior).
		if m.state == stateReader && key.Code == tea.KeyEsc && !m.fromList {
			return m, tea.Quit
		}

	case fetchResultMsg:
		return m.routeFetch(msg.entry), nil
	}

	// Delegate to the active sub-model.
	var cmd tea.Cmd
	m.reader, cmd = m.reader.update(msg)
	return m, cmd
}

// routeFetch is the single decision point for a completed fetch. In this task
// every result goes to the reader; Task 4 adds the host-list branch.
func (m appModel) routeFetch(entry Entry) appModel {
	m.reader.setEntry(entry)
	m.state = stateReader
	return m
}

func (m appModel) View() tea.View {
	v := tea.NewView(m.reader.View())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

package tui

import (
	"context"
	"net"
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
	hostList  *Entry
	fromList  bool
	listReady bool
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
		if m.listReady {
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
		handled, updated, cmd := m.handleKey(msg)
		m = updated
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
func (m appModel) handleKey(msg tea.KeyPressMsg) (bool, appModel, tea.Cmd) {
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
		case 'r':
			// On a generic ("best guess") list only, show the cached raw host
			// body so the user can read the actual response when the heuristic
			// parse looks wrong. fromList=true so Esc returns to the list.
			if m.list.generic && m.hostList != nil {
				m.reader.setEntry(*m.hostList)
				m.state = stateReader
				m.fromList = true
				return true, m, nil
			}
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
func (m appModel) drill() (bool, appModel, tea.Cmd) {
	sel, ok := m.list.selected()
	if !ok {
		return true, m, nil
	}
	raw := sel.target
	serverSupplied := raw != ""
	if !serverSupplied {
		// Build login@host from the host's original argument (minus the leading
		// "@"), preserving any explicit :port the user typed.
		host := strings.TrimPrefix(m.list.host.Raw, "@")
		raw = sel.login + "@" + host
	}
	target, err := finger.ParseTarget(raw)
	if err != nil {
		return true, m, nil
	}
	if serverSupplied {
		// A target extracted from the server's own response (a finger:// link
		// or "finger user@host" command) could point at an arbitrary host:port.
		// Finger always lives on port 79; pin server-supplied targets to it so a
		// malicious response can't steer lookit at another service (e.g.
		// host:22). User-typed targets keep their explicit port.
		target = pinFingerPort(target)
	}
	m.reader.setLoading(target)
	m.state = stateReader
	m.fromList = true
	return true, m, fetchCmd(context.Background(), m.common.fetch, target)
}

// routeFetch is the single decision point for a completed fetch: a host
// response that parses opens the list (flagged "(best guess)" when only the
// generic fallback recognized it); everything else renders in the reader.
func (m appModel) routeFetch(entry Entry) appModel {
	// Any fetch result means loading is done; the list branch never calls setEntry, so clear it here.
	m.reader.loading = false
	if len(entry.Body) > 0 && shouldOpenList(entry) {
		if parsed, ok := parseUserList(entry.Body); ok {
			cached := entry
			m.hostList = &cached
			incomplete := entry.Err != nil || entry.Meta.Truncated
			m.list = newListWithPreamble(m.common, entry.Target, parsed.users, entry.Body, incomplete, parsed.generic)
			m.listReady = true
			m.state = stateList
			m.fromList = false
			return m
		}
	}
	m.reader.setEntry(entry)
	m.state = stateReader
	return m
}

// shouldOpenList reports whether a fetch result is a host-style listing that
// should open the selectable list rather than the plain reader. Host queries
// (no user) qualify; "ring@thebackupbox.net" is special-cased because that
// pseudo-user returns the Finger Ring directory rather than a single profile.
func shouldOpenList(entry Entry) bool {
	return entry.Target.User == "" ||
		(entry.Target.User == "ring" && strings.HasPrefix(entry.Target.HostPort, "thebackupbox.net:"))
}

// pinFingerPort rewrites a target's port to 79, the finger well-known port.
// It is applied to targets lifted from a server's response so a hostile entry
// cannot direct lookit at a non-finger service. The host is preserved (the
// Finger Ring is legitimately cross-host).
func pinFingerPort(t finger.Target) finger.Target {
	if host, _, err := net.SplitHostPort(t.HostPort); err == nil {
		t.HostPort = net.JoinHostPort(host, "79")
	}
	return t
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

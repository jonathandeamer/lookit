package tui

import (
	"context"
	"fmt"
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

// bodyHeight is the height available to a sub-model after reserving one row
// for the bottom status bar.
func (c *commonModel) bodyHeight() int {
	if c.height > 1 {
		return c.height - 1
	}
	return 1
}

// histNode snapshots a landed screen so back/forward restore instead of
// re-fetching. listUsers/listGeneric are cached so View needn't re-parse.
type histNode struct {
	entry       Entry
	state       appState
	scrollY     int    // reader viewport offset
	listIdx     int    // list selected index
	listFltr    string // applied list filter
	listUsers   int
	listGeneric bool
}

// appModel is the top-level state machine. It routes input and fetch results
// between the reader and the list, and owns quit/back behavior.
type appModel struct {
	common *commonModel
	state  appState
	reader readerModel
	list   listModel

	history    []histNode
	pos        int  // -1 == landing (nothing fetched yet)
	showingRaw bool // r-toggled raw view of the current generic list node
	help       bool // help overlay open
	listReady  bool
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
		pos:    -1,
	}
}

// push records a newly-landed screen, truncating any forward tail first.
func (m *appModel) push(node histNode) {
	if m.pos+1 < len(m.history) {
		m.history = m.history[:m.pos+1]
	}
	m.history = append(m.history, node)
	m.pos = len(m.history) - 1
}

// snapshot captures live view state into the current node before navigating.
func (m *appModel) snapshot() {
	if m.pos < 0 || m.pos >= len(m.history) {
		return
	}
	n := &m.history[m.pos]
	if n.state == stateReader {
		n.scrollY = m.reader.viewport.YOffset()
	} else {
		n.listIdx = m.list.list.Index()
		n.listFltr = m.list.list.FilterValue()
	}
}

// restore rebuilds the active sub-model from a node (no network).
func (m *appModel) restore(n histNode) {
	if n.state == stateReader {
		m.state = stateReader
		m.reader.setEntry(n.entry)
		m.reader.viewport.SetYOffset(n.scrollY)
		return
	}
	if parsed, ok := parseUserList(n.entry.Body); ok {
		m.state = stateList
		m.list = newListWithPreamble(m.common, n.entry.Target, parsed.users, n.entry.Body, parsed.generic)
		m.listReady = true
		m.list.list.Select(n.listIdx)
		if n.listFltr != "" {
			m.list.list.SetFilterText(n.listFltr)
		}
		return
	}
	// Defensive: a previously-listed body no longer parses; show it in the
	// reader rather than leaving a stale list on screen. Unreachable in
	// practice (parseUserList is deterministic on the same bytes).
	m.state = stateReader
	m.reader.setEntry(n.entry)
}

// gotoLanding returns the reader to its empty pre-fetch state.
func (m *appModel) gotoLanding() {
	m.state = stateReader
	m.reader.current = nil
	m.reader.loading = false
	m.reader.viewport.SetContent("No response yet.")
}

// stepBack moves one step toward history root, or to the landing from pos 0.
func (m *appModel) stepBack() {
	m.showingRaw = false
	if m.pos < 0 {
		return
	}
	m.snapshot()
	m.pos--
	if m.pos < 0 {
		m.gotoLanding()
		return
	}
	m.restore(m.history[m.pos])
}

// back is Esc semantics: step back, or quit when already at the landing.
func (m *appModel) back() tea.Cmd {
	if m.pos < 0 {
		return tea.Quit
	}
	m.stepBack()
	return nil
}

// forward re-applies a previously-popped node.
func (m *appModel) forward() {
	m.showingRaw = false
	if m.pos >= len(m.history)-1 {
		return
	}
	m.snapshot()
	m.pos++
	m.restore(m.history[m.pos])
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
		m.reader.setSize(msg.Width, m.common.bodyHeight())
		// Resize the list only once it exists; a freshly-opened list is sized
		// from common in newList.
		if m.listReady {
			m.list.setSize(msg.Width, m.common.bodyHeight())
		}
		return m, nil

	case tea.ColorProfileMsg:
		m.common.profile = msg.Profile
		m.reader.setProfile(msg.Profile)
		return m, nil

	case tea.KeyPressMsg:
		// handleKey may mutate the model even when it does not fully handle
		// the key, so adopt its returned model before deciding whether to delegate.
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

	// Help overlay: any key closes it; '?' opens it (except while filtering).
	if m.help {
		m.help = false
		return true, m, nil
	}
	if key.Code == '?' && (m.state != stateList || !m.list.filtering()) {
		m.help = true
		return true, m, nil
	}

	switch m.state {
	case stateList:
		if m.list.filtering() {
			return false, m, nil
		}
		switch {
		case key.Code == tea.KeyEsc:
			// Let the list clear an active or applied filter before backing out.
			if m.list.list.FilterState() != list.Unfiltered {
				return false, m, nil
			}
			cmd := m.back()
			return true, m, cmd
		case key.Code == tea.KeyLeft && key.Mod == tea.ModAlt:
			m.stepBack()
			return true, m, nil
		case key.Code == tea.KeyRight && key.Mod == tea.ModAlt:
			m.forward()
			return true, m, nil
		case key.Code == tea.KeyEnter:
			return m.drill()
		case key.Code == 'r':
			if m.list.generic && m.pos >= 0 {
				m.reader.setEntry(m.history[m.pos].entry)
				m.state = stateReader
				m.showingRaw = true
				return true, m, nil
			}
		}

	case stateReader:
		if m.showingRaw && key.Code == tea.KeyEsc {
			m.showingRaw = false
			m.state = stateList
			return true, m, nil
		}
		switch {
		case key.Code == tea.KeyEsc:
			cmd := m.back()
			return true, m, cmd
		case key.Code == tea.KeyLeft && key.Mod == tea.ModAlt:
			m.stepBack()
			return true, m, nil
		case key.Code == tea.KeyRight && key.Mod == tea.ModAlt:
			m.forward()
			return true, m, nil
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
	return true, m, fetchCmd(context.Background(), m.common.fetch, target)
}

// routeFetch is the single decision point for a completed fetch: a host
// response that parses opens the list; everything else renders in the reader.
// Either way it pushes a history node.
func (m appModel) routeFetch(entry Entry) appModel {
	m.reader.loading = false
	m.showingRaw = false
	node := histNode{entry: entry, state: stateReader}
	if len(entry.Body) > 0 && shouldOpenList(entry) {
		if parsed, ok := parseUserList(entry.Body); ok {
			m.list = newListWithPreamble(m.common, entry.Target, parsed.users, entry.Body, parsed.generic)
			m.listReady = true
			node.state = stateList
			node.listUsers = len(parsed.users)
			node.listGeneric = parsed.generic
		}
	}
	if node.state == stateReader {
		m.reader.setEntry(entry)
	}
	m.state = node.state
	m.snapshot() // save current position's scroll/selection before pushing
	m.push(node)
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

// statusBarModel assembles the bottom bar from the current node + history.
func (m appModel) statusBarModel() statusBar {
	st := newStyles()
	w := m.common.width
	if m.pos < 0 {
		return landingBar(w, st)
	}
	node := m.history[m.pos]
	bar := statusBar{width: w, styles: st}
	bar.host, bar.user = breadcrumbParts(node.entry.Target)
	if m.pos >= 1 {
		bar.escTarget = m.history[m.pos-1].entry.Target.Raw
	}

	if m.showingRaw {
		// Esc here returns to the list at the same history position (it does
		// not pop history), so don't show a back-to-previous-target hint.
		bar.escTarget = ""
		bar.meta = formatBytes(len(node.entry.Body))
		bar.hints = "esc back · ? help"
		return bar
	}

	switch node.state {
	case stateList:
		bar.meta = fmt.Sprintf("%d users", node.listUsers)
		bar.hints = "↵ open · / filter · esc back · ? help"
		if node.listGeneric {
			bar.flags = append(bar.flags, "auto-detected")
			bar.hints = "↵ open · / filter · r raw · esc back · ? help"
		}
		if node.entry.Err != nil {
			bar.flags = append(bar.flags, "partial (error)")
		} else if node.entry.Meta.Truncated {
			bar.flags = append(bar.flags, "partial (truncated)")
		}
	default: // stateReader
		bar.meta = formatBytes(len(node.entry.Body))
		bar.hints = "↑↓ scroll · esc back · ? help"
	}
	return bar
}

func (m appModel) helpView() string {
	st := newStyles()
	lines := []string{
		st.title.Render("lookit — keys"),
		"",
		"  Enter        open / fetch the highlighted target",
		"  Esc          back (quit at the top)",
		"  Alt+←        back        Alt+→   forward",
		"  ↑ ↓          scroll / move selection",
		"  /            filter a list      r   raw view (auto-detected lists)",
		"  ?            toggle this help    Ctrl+C   quit",
		"",
		st.hint.Render("press any key to close"),
	}
	return strings.Join(lines, "\n")
}

func (m appModel) View() tea.View {
	var content string
	switch {
	case m.help:
		content = m.helpView()
	case m.state == stateList:
		content = m.list.View()
	default:
		content = m.reader.View()
	}
	content += "\n" + m.statusBarModel().render()

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

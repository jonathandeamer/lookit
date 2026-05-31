package tui

import (
	"context"
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

// setClipboard is a seam for testing: it defaults to tea.SetClipboard.
var setClipboard = tea.SetClipboard

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

// bodyHeight is the height available to a sub-model after reserving the top
// input row and the bottom status-bar row.
func (c *commonModel) bodyHeight() int {
	if c.height > 2 {
		return c.height - 2
	}
	return 1
}

// histNode snapshots a landed screen so back restores instead of re-fetching.
// listUsers/listGeneric are cached so View needn't re-parse.
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

	input        textinput.Model
	inputFocused bool
	keys         keyMap

	loading       bool
	loadingTarget finger.Target
	spin          spinner.Model

	flash string

	history    []histNode
	pos        int  // -1 == landing (nothing fetched yet)
	showingRaw bool // r-toggled "view source" of the current node's raw body
	help       bool // help panel open
	helpModel  help.Model
	listReady  bool
}

func newApp(fetch FetchFunc, profile colorprofile.Profile) appModel {
	if fetch == nil {
		fetch = defaultFetch
	}
	common := &commonModel{profile: profile, fetch: fetch}
	in := textinput.New()
	in.Placeholder = "alice@plan.cat"
	in.Prompt = "target: "
	in.CharLimit = 256
	in.SetWidth(40)
	in.Focus() // landing starts focused
	app := appModel{
		common:       common,
		state:        stateReader,
		reader:       newReader(profile),
		input:        in,
		inputFocused: true,
		keys:         newKeyMap(),
		helpModel:    help.New(),
		spin:         spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		pos:          -1,
	}
	app.updateKeymap() // first frame reflects the landing's enabled set
	return app
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
	m.reader.viewport.SetContent("No response yet.")
	m.inputFocused = true
	m.input.SetValue("")
	m.input.Focus() // discard the blink cmd; the cursor still shows
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

// focusInput gives the keyboard to the target input, pre-filled with the
// current target for browser-style editing.
func (m *appModel) focusInput() tea.Cmd {
	if m.pos >= 0 {
		m.input.SetValue(m.history[m.pos].entry.Target.Raw)
	}
	m.inputFocused = true
	m.input.CursorEnd()
	return m.input.Focus()
}

// blurInput returns the keyboard to the content.
func (m *appModel) blurInput() {
	m.inputFocused = false
	m.input.Blur()
}

// enterRaw shows the current node's unprocessed body ("view source") in the
// reader viewport. It works over any node (list or profile); the underlying
// node.state is preserved in history so exitRaw can return to it.
func (m *appModel) enterRaw() {
	if m.pos < 0 {
		return
	}
	m.reader.setRaw(m.history[m.pos].entry.Body)
	m.state = stateReader
	m.showingRaw = true
}

// exitRaw returns from raw view to the node's normal view (list or profile).
func (m *appModel) exitRaw() {
	m.showingRaw = false
	if m.pos < 0 {
		return
	}
	node := m.history[m.pos]
	m.state = node.state
	if node.state == stateReader {
		m.reader.setEntry(node.entry) // re-render the profile
	}
}

// submit parses the input and starts a fetch, blurring to content. On a parse
// error it keeps the input focused and flashes the error.
func (m *appModel) submit() tea.Cmd {
	target, err := finger.ParseTarget(strings.TrimSpace(m.input.Value()))
	if err != nil {
		m.flash = "error: " + err.Error()
		return nil
	}
	m.flash = "" // clear any stale parse-error flash from a prior failed submit
	m.blurInput()
	m.loading = true
	m.loadingTarget = target
	return tea.Batch(fetchCmd(context.Background(), m.common.fetch, target), m.spin.Tick)
}

func (m appModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		tea.RequestCapability("RGB"),
		tea.RequestCapability("Tc"),
	)
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Sync the enabled set to the current state before handleKey: key.Matches
	// ignores a disabled binding, so a stale enabled set would drop keys (e.g.
	// 'i'/'?' after a fetch left the landing's enablement in place).
	(&m).updateKeymap()
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.common.width = msg.Width
		m.common.height = msg.Height
		iw := msg.Width - lipgloss.Width(m.input.Prompt)
		if iw < 20 {
			iw = 20
		}
		m.input.SetWidth(iw)
		m.helpModel.SetWidth(msg.Width)
		m.resizeForHelp()
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

	case clearFlashMsg:
		m.flash = ""
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// Delegate unhandled messages: to the input when focused, else to content.
	var cmd tea.Cmd
	if m.inputFocused {
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	switch m.state {
	case stateList:
		m.list, cmd = m.list.update(msg)
	default:
		m.reader, cmd = m.reader.update(msg)
	}
	return m, cmd
}

// handleKey processes app-level keys and focus routing. handled=false lets the
// caller delegate the key to the active sub-model (content) or the input.
func (m appModel) handleKey(msg tea.KeyPressMsg) (bool, appModel, tea.Cmd) {
	if key.Matches(msg, m.keys.ForceQuit) {
		return true, m, tea.Quit
	}

	// Help panel: any key closes it.
	if m.help {
		m.help = false
		m.helpModel.ShowAll = false
		m.resizeForHelp()
		return true, m, nil
	}

	// Input focused: Enter/Esc/? are commands; everything else types. '?' opens
	// help (it can't appear in a finger address, and the landing — input focused
	// — is exactly where a first-time user reaches for help).
	if m.inputFocused {
		switch {
		case key.Matches(msg, m.keys.Help): // ?
			m.help = true
			m.helpModel.ShowAll = true
			m.resizeForHelp()
			return true, m, nil
		case key.Matches(msg, m.keys.Open): // Enter
			cmd := m.submit()
			return true, m, cmd
		case key.Matches(msg, m.keys.Back): // Esc
			if m.pos < 0 {
				return true, m, tea.Quit
			}
			m.blurInput()
			return true, m, nil
		}
		return false, m, nil // fall through: type into the input
	}

	// Content focused.
	if m.state == stateList && m.list.filtering() {
		return false, m, nil // list owns its filter keys
	}
	switch {
	case key.Matches(msg, m.keys.Help):
		m.help = true
		m.helpModel.ShowAll = true
		m.resizeForHelp()
		return true, m, nil
	case key.Matches(msg, m.keys.Quit):
		return true, m, tea.Quit
	case key.Matches(msg, m.keys.FocusInput):
		cmd := m.focusInput()
		return true, m, cmd
	case key.Matches(msg, m.keys.Back):
		if m.state == stateList && m.list.list.FilterState() != list.Unfiltered {
			return false, m, nil // clear an applied filter first
		}
		if m.showingRaw {
			m.exitRaw()
			return true, m, nil
		}
		cmd := m.back()
		return true, m, cmd
	case key.Matches(msg, m.keys.Copy):
		cmd := m.copyAddress()
		return true, m, cmd
	case key.Matches(msg, m.keys.Open) && m.state == stateList:
		return m.drill()
	case key.Matches(msg, m.keys.Raw) && m.pos >= 0:
		if m.showingRaw {
			m.exitRaw()
		} else {
			m.enterRaw()
		}
		return true, m, nil
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
	m.loading = true
	m.loadingTarget = target
	// Keep the current view (the list) on screen while loading; routeFetch sets
	// the final state when the result lands. Switching to the reader eagerly here
	// flashed the previous profile for a frame before the new one arrived.
	return true, m, tea.Batch(fetchCmd(context.Background(), m.common.fetch, target), m.spin.Tick)
}

// routeFetch is the single decision point for a completed fetch: a host
// response that parses opens the list; everything else renders in the reader.
// Either way it pushes a history node.
func (m appModel) routeFetch(entry Entry) appModel {
	m.loading = false
	m.showingRaw = false
	m.inputFocused = false
	m.input.Blur()
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

// clearFlashMsg is sent after a flash timer fires to clear m.flash.
type clearFlashMsg struct{}

// clearFlashCmd returns a command that fires clearFlashMsg after 2 seconds.
func (m *appModel) clearFlashCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })
}

// copyAddress copies the relevant address to the clipboard and flashes it.
func (m *appModel) copyAddress() tea.Cmd {
	var addr string
	if m.state == stateList {
		if sel, ok := m.list.selected(); ok {
			addr = sel.target
			if addr == "" {
				addr = sel.login + "@" + strings.TrimPrefix(m.list.host.Raw, "@")
			}
		}
	} else if m.pos >= 0 {
		addr = m.history[m.pos].entry.Target.Raw
	}
	if addr == "" {
		return nil
	}
	m.flash = "copied " + addr
	return tea.Batch(setClipboard(addr), m.clearFlashCmd())
}

// statusBarModel assembles the bottom bar from the current node + history.
// updateKeymap enables only the bindings usable in the current state. It is the
// single source of truth with two effects: the expanded '?' help panel skips
// disabled bindings (bubbles/help), and key.Matches treats a disabled binding as
// no-match — so a content key is inert (types literally) while the input is
// focused. It must run before both handleKey (routing) and the render path
// (help panel); Update and View call it. Pattern: pop's updateKeymap
// (~/pop/keymap.go).
func (m *appModel) updateKeymap() {
	content := !m.inputFocused
	hasResult := m.pos >= 0
	inList := content && m.state == stateList && !m.showingRaw

	// Dual-mode commands — handleKey matches them in BOTH the input-focused and
	// content branches, so they must stay live while typing: Open=Enter (submit
	// a target / drill a list row), Back=Esc (cancel the edit / history back /
	// quit at the bare landing), Help='?'.
	m.keys.Help.SetEnabled(true)
	m.keys.Open.SetEnabled(m.inputFocused || inList)
	m.keys.Back.SetEnabled(m.inputFocused || (content && hasResult))

	// Content-only keys — inert while the input is focused (they type literally).
	m.keys.FocusInput.SetEnabled(content)
	m.keys.Quit.SetEnabled(content)
	m.keys.Copy.SetEnabled(content && hasResult)
	m.keys.Raw.SetEnabled(content && hasResult)
	m.keys.Filter.SetEnabled(inList)
	m.keys.Move.SetEnabled(content)
	m.keys.Page.SetEnabled(content)
	m.keys.Jump.SetEnabled(content)
}

func (m appModel) statusBarModel() statusBar {
	st := newStyles()
	w := m.common.width
	if m.loading {
		bar := statusBar{width: w, styles: st}
		bar.hints = m.spin.View() + " loading " + m.loadingTarget.Raw
		return bar
	}
	if m.pos < 0 {
		bar := landingBar(w, st)
		if m.flash != "" {
			bar.hints = m.flash
		}
		return bar
	}
	node := m.history[m.pos]
	bar := statusBar{width: w, styles: st}
	bar.host, bar.user = breadcrumbParts(node.entry.Target)
	if m.pos >= 1 {
		bar.escTarget = m.history[m.pos-1].entry.Target.Raw
	}

	if m.inputFocused {
		// Editing the address over existing content: Enter fetches, Esc cancels
		// the edit (it does not navigate), so don't offer a back-to-previous
		// target hint here.
		bar.escTarget = ""
		bar.hints = "↵ fetch · esc cancel"
		if m.flash != "" {
			bar.hints = m.flash
		}
		return bar
	}

	if m.showingRaw {
		// Esc here returns to the list at the same history position (it does
		// not pop history), so don't show a back-to-previous-target hint.
		bar.escTarget = ""
		bar.meta = formatBytes(len(node.entry.Body))
		bar.hints = "esc back · ? help"
		if m.flash != "" {
			bar.hints = m.flash
		}
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
		if tp := m.list.list.Paginator.TotalPages; tp > 1 {
			bar.page = fmt.Sprintf("page %d/%d", m.list.list.Paginator.Page+1, tp)
		}
	default: // stateReader
		bar.meta = formatBytes(len(node.entry.Body))
		bar.hints = "↑↓ scroll · esc back · ? help"
		if m.reader.viewport.TotalLineCount() > m.reader.viewport.Height() {
			bar.scroll = fmt.Sprintf("%d%%", int(math.Round(m.reader.viewport.ScrollPercent()*100)))
		}
	}
	if m.flash != "" {
		bar.hints = m.flash
	}
	return bar
}

// helpHeight returns the number of rows the help panel occupies when open,
// or 0 when the panel is closed.
func (m *appModel) helpHeight() int {
	if !m.help {
		return 0
	}
	m.updateKeymap() // measure the same enabled set the View will render
	return lipgloss.Height(m.helpModel.View(m.keys))
}

// resizeForHelp re-sizes the active sub-model to leave room for the help block.
// Called after toggling m.help so sub-models can fill the available height.
func (m *appModel) resizeForHelp() {
	h := m.common.bodyHeight() - m.helpHeight()
	if h < 1 {
		h = 1
	}
	m.reader.setSize(m.common.width, h)
	if m.listReady {
		m.list.setSize(m.common.width, h)
	}
}

func (m appModel) View() tea.View {
	(&m).updateKeymap() // sync the help panel's enabled set to current state
	var content string
	switch m.state {
	case stateList:
		content = m.list.View()
	default:
		content = m.reader.View()
	}
	bottom := m.statusBarModel().render()
	if m.help {
		bottom = m.helpModel.View(m.keys) + "\n" + bottom
	}
	full := m.input.View() + "\n" + content + "\n" + bottom

	v := tea.NewView(full)
	v.AltScreen = true
	return v
}

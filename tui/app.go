package tui

import (
	"context"
	"fmt"
	"math"
	"math/rand"
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
	"github.com/charmbracelet/x/ansi"
	"github.com/jonathandeamer/lookit/finger"
)

// setClipboard is a seam for testing: it defaults to tea.SetClipboard.
var setClipboard = tea.SetClipboard

// sampleTargets are the rotating greyed-out hints shown in the empty target
// input. The mix of "@host" directory shapes and "user@host" profile shapes
// teaches both input forms. They are hint text only, never auto-submitted.
var sampleTargets = []string{
	"ring@thebackupbox.net",
	"@happynetbox.com",
	"@plan.cat",
	"@tilde.team",
	"jonathan@tilde.team",
}

// pickSample returns a uniformly random sample target for the placeholder.
func pickSample() string {
	return sampleTargets[rand.Intn(len(sampleTargets))]
}

// appState selects which sub-model is active.
type appState int

const (
	stateReader appState = iota
	stateList
)

// commonModel is state shared across sub-models.
type commonModel struct {
	width          int
	height         int
	profile        colorprofile.Profile
	darkBackground bool
	styles         styles
	fetch          FetchFunc
	version        string
}

// bodyHeight is the height available to a sub-model after reserving the top
// input row and the bottom status-bar row.
func (c *commonModel) bodyHeight() int {
	if c.height > 2 {
		return c.height - 2
	}
	return 1
}

func (c *commonModel) ensureStyles() styles {
	if c.styles.palette.BaseBg == nil {
		c.styles = newStyles(c.darkBackground)
	}
	return c.styles
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
	seeded       bool // a CLI positional arg was supplied; replay it on Init
	keys         keyMap

	loading       bool
	loadingTarget finger.Target
	reqSeq        uint64 // monotonic id of the most recently started fetch
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
	return newAppWithOptions(fetch, profile, Options{})
}

func newAppWithOptions(fetch FetchFunc, profile colorprofile.Profile, opts Options) appModel {
	if fetch == nil {
		fetch = defaultFetch
	}
	st := newStyles(true)
	common := &commonModel{
		profile:        profile,
		darkBackground: true,
		styles:         st,
		fetch:          fetch,
		version:        opts.Version,
	}
	in := textinput.New()
	in.Placeholder = pickSample()
	in.Prompt = "target: "
	in.CharLimit = 256
	in.SetWidth(40)
	in.SetStyles(st.input)
	if opts.Seed {
		in.SetValue(opts.InitialQuery) // replayed via seedSubmitMsg in Init/Update
	}
	in.Focus() // landing starts focused
	app := appModel{
		common:       common,
		state:        stateReader,
		reader:       newReader(profile),
		input:        in,
		inputFocused: true,
		seeded:       opts.Seed,
		keys:         newKeyMap(),
		helpModel:    help.New(),
		spin:         spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(st.spinner)),
		pos:          -1,
	}
	app.reader.setBackground(common.darkBackground)
	app.reader.styles = st
	app.helpModel.Styles = st.help
	app.updateKeymap() // first frame reflects the landing's enabled set
	return app
}

func (m *appModel) setBackground(dark bool) {
	m.common.darkBackground = dark
	m.common.styles = newStyles(dark)
	m.applyStyles()
}

func (m *appModel) applyStyles() {
	st := m.common.ensureStyles()
	m.input.SetStyles(st.input)
	m.helpModel.Styles = st.help
	m.spin.Style = st.spinner
	m.reader.styles = st
	if m.showingRaw {
		m.reader.darkBackground = m.common.darkBackground
	} else {
		m.reader.setBackground(m.common.darkBackground)
	}
	if m.listReady {
		m.list.applyStyles(st)
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
		if n.listFltr != "" {
			m.list.list.SetFilterText(n.listFltr)
		}
		m.list.list.Select(n.listIdx)
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
	m.resizeForHelp()
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
	m.flash = ""
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
	m.resizeForHelp()
	return m.input.Focus()
}

// startFetch marks loading for target, advances the request id so any
// still-in-flight earlier fetch's result will be discarded on arrival, and
// returns the command batch that performs the fetch and ticks the spinner.
func (m *appModel) startFetch(target finger.Target) tea.Cmd {
	m.loading = true
	m.loadingTarget = target
	m.reqSeq++
	return tea.Batch(fetchCmd(context.Background(), m.common.fetch, target, m.reqSeq), m.spin.Tick)
}

// blurInput returns the keyboard to the content.
func (m *appModel) blurInput() {
	m.inputFocused = false
	m.input.Blur()
	m.resizeForHelp()
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
	return m.startFetch(target)
}

// seedSubmitMsg replays a command-line initial query through submit() on
// startup, so a seeded target takes the exact path a typed one does.
type seedSubmitMsg struct{}

func (m appModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		textinput.Blink,
		tea.RequestBackgroundColor,
		tea.RequestCapability("RGB"),
		tea.RequestCapability("Tc"),
	}
	if m.seeded {
		// Replay the supplied positional arg through submit(), even when blank:
		// a blank arg yields the same parse-error flash as Enter-on-empty does
		// interactively, rather than silently landing.
		cmds = append(cmds, func() tea.Msg { return seedSubmitMsg{} })
	}
	return tea.Batch(cmds...)
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

	case tea.BackgroundColorMsg:
		m.setBackground(msg.IsDark())
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
		if msg.reqID != m.reqSeq {
			// A superseded in-flight fetch finished late; drop it so it cannot
			// replace the current view/history with stale (or hostile) output.
			return m, nil
		}
		return m.routeFetch(msg.entry), nil

	case clearFlashMsg:
		m.flash = ""
		return m, nil

	case seedSubmitMsg:
		cmd := m.submit()
		return m, cmd

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
	m.flash = ""
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
	// Keep the current view (the list) on screen while loading; routeFetch sets
	// the final state when the result lands. Switching to the reader eagerly here
	// flashed the previous profile for a frame before the new one arrived.
	cmd := m.startFetch(target)
	return true, m, cmd
}

// routeFetch is the single decision point for a completed fetch: a host
// response that parses opens the list; everything else renders in the reader.
// Either way it pushes a history node.
func (m appModel) routeFetch(entry Entry) appModel {
	m.loading = false
	m.inputFocused = false
	m.input.Blur()
	m.snapshot() // save current position's scroll/selection before replacing it
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
		pinned := net.JoinHostPort(host, "79")
		if t.HostPort != pinned {
			t.HostPort = pinned
			t.Raw = rawFromTarget(t)
		} else {
			t.HostPort = pinned
		}
	}
	return t
}

func rawFromTarget(t finger.Target) string {
	return t.User + "@" + t.HostPort
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
			if sel.target != "" {
				// Mirror drill's safety: a server-supplied target could point at
				// an arbitrary host:port. Pin to finger's port 79 before copying
				// so a pasted-back address can't be steered at another service.
				// sel.target is pre-validated by ParseUsers, so a parse error here
				// is effectively unreachable; on error we simply copy nothing.
				if t, err := finger.ParseTarget(sel.target); err == nil {
					addr = rawFromTarget(pinFingerPort(t))
				}
			} else {
				addr = sel.login + "@" + strings.TrimPrefix(m.list.host.Raw, "@")
			}
		}
	} else if m.pos >= 0 {
		addr = m.history[m.pos].entry.Target.Raw
	}
	if addr == "" {
		m.flash = "nothing to copy"
		return m.clearFlashCmd()
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

// joinHints assembles the bar's hint string. "esc back" is included only when
// there is no "◂ esc: <target>" breadcrumb segment (escTarget == ""): when that
// segment is present it already shows esc-goes-back (and where to), so repeating
// it in the hints is redundant. "? help" always closes the list — the bottom bar
// is help's permanent home, so the '?' panel itself omits it.
func joinHints(parts []string, escTarget string) string {
	if escTarget == "" {
		parts = append(parts, "esc back")
	}
	parts = append(parts, "? help")
	return strings.Join(parts, " · ")
}

func (m appModel) statusBarModel() statusBar {
	st := m.common.styles
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
		// Editing the address over existing content: Enter goes (fetches the
		// typed target), Esc cancels the edit (it does not navigate), so don't
		// offer a back-to-previous target hint here.
		bar.escTarget = ""
		bar.hints = "↵ go · esc cancel"
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
		parts := []string{"↵ go", "/ filter"}
		if node.listGeneric {
			bar.flags = append(bar.flags, "auto-detected")
			parts = append(parts, "r raw")
		}
		if node.entry.Err != nil {
			bar.flags = append(bar.flags, "partial (error)")
		} else if node.entry.Meta.Truncated {
			bar.flags = append(bar.flags, "partial (truncated)")
		}
		bar.hints = joinHints(parts, bar.escTarget)
		if tp := m.list.list.Paginator.TotalPages; tp > 1 {
			bar.page = fmt.Sprintf("page %d/%d", m.list.list.Paginator.Page+1, tp)
		}
	default: // stateReader
		bar.meta = formatBytes(len(node.entry.Body))
		// The render footer (which carried the truncation notice) is suppressed
		// in the TUI. The error message still renders in the viewport via the
		// ErrLine, but truncation had no other home, so surface it here.
		if node.entry.Meta.Truncated {
			bar.flags = append(bar.flags, "partial (truncated)")
		}
		bar.hints = joinHints([]string{"↑↓ scroll"}, bar.escTarget)
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
	return lipgloss.Height(m.helpView())
}

// resizeForHelp re-sizes the active sub-model to leave room for the help block.
// Called after toggling m.help so sub-models can fill the available height.
func (m *appModel) resizeForHelp() {
	h := m.common.height - m.topChromeHeight() - 1 - m.helpHeight()
	if h < 1 {
		h = 1
	}
	m.reader.setSize(m.common.width, h)
	if m.listReady {
		m.list.setSize(m.common.width, h)
	}
}

func (m appModel) helpView() string {
	return fullWidthHelpView(m.keys.FullHelp(), m.common.styles, m.common.width, m.helpModel.FullSeparator)
}

func (m appModel) topChromeHeight() int {
	if m.inputFocused {
		return 2 // header mark + target row (see inputChromeView)
	}
	return 1
}

func (m appModel) inputChromeView() string {
	if !m.inputFocused {
		return m.input.View()
	}
	return headerMark(m.common.styles, m.common.profile) + "\n" + m.input.View()
}

func fullWidthHelpView(groups [][]key.Binding, st styles, width int, separator string) string {
	var columns [][]string
	var widths []int
	maxRows := 0
	for _, group := range groups {
		rows := helpColumnRows(group, st)
		if len(rows) == 0 {
			continue
		}
		columnWidth := maxLineWidth(rows)
		for i, row := range rows {
			rows[i] = padStyledLine(row, columnWidth, st.helpBand)
		}
		columns = append(columns, rows)
		widths = append(widths, columnWidth)
		if len(rows) > maxRows {
			maxRows = len(rows)
		}
	}
	if maxRows == 0 {
		return ""
	}

	lines := make([]string, maxRows)
	sep := st.help.FullSeparator.Render(separator)
	for row := range maxRows {
		var line strings.Builder
		for col, rows := range columns {
			if col > 0 {
				line.WriteString(sep)
			}
			if row < len(rows) {
				line.WriteString(rows[row])
				continue
			}
			line.WriteString(st.helpBand.Render(strings.Repeat(" ", widths[col])))
		}
		out := line.String()
		if width > 0 && lipgloss.Width(out) > width {
			out = ansi.Truncate(out, width, "...")
		}
		lines[row] = padStyledLine(out, width, st.helpBand)
	}
	return strings.Join(lines, "\n")
}

func helpColumnRows(group []key.Binding, st styles) []string {
	keyWidth := 0
	for _, binding := range group {
		if !binding.Enabled() {
			continue
		}
		if w := lipgloss.Width(binding.Help().Key); w > keyWidth {
			keyWidth = w
		}
	}
	if keyWidth == 0 {
		return nil
	}

	var rows []string
	for _, binding := range group {
		if !binding.Enabled() {
			continue
		}
		help := binding.Help()
		key := st.help.FullKey.Render(help.Key + strings.Repeat(" ", keyWidth-lipgloss.Width(help.Key)))
		rows = append(rows, key+st.helpBand.Render(" ")+st.help.FullDesc.Render(help.Desc))
	}
	return rows
}

func maxLineWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > width {
			width = w
		}
	}
	return width
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
		bottom = m.helpView() + "\n" + bottom
	}
	full := m.inputChromeView() + "\n" + content + "\n" + bottom

	v := tea.NewView(full)
	v.AltScreen = true
	return v
}

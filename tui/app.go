package tui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
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
	stateAbout
)

// commonModel is state shared across sub-models.
type commonModel struct {
	width          int
	height         int
	profile        colorprofile.Profile
	darkBackground bool
	styles         styles
	fetch          FetchFunc
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
	links       []Link // cached detected links for the reader
	linkIdx     int    // focused link index (-1 == none)
}

// appModel is the top-level state machine. It routes input and fetch results
// between the reader and the list, and owns quit/back behavior.
type appModel struct {
	common *commonModel
	state  appState
	reader readerModel
	list   listModel
	about  aboutModel

	aboutFromState appState // state to restore when the about screen closes

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
		about:        newAbout(profile, opts.Version, opts.BuiltAt),
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
	app.about.setBackground(common.darkBackground)
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
	m.about.setBackground(m.common.darkBackground)
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
		n.links = m.reader.links
		n.linkIdx = m.reader.focusedLink
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
		m.reader.links = n.links
		m.reader.focusedLink = n.linkIdx
		return
	}
	if parsed, ok := parseUserList(n.entry.Body, n.entry.Target.HostPort); ok {
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
	m.resize()
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
	m.resize()
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
	m.resize()
}

// openAbout switches to the full-screen about view, remembering the current
// state so closeAbout can restore it without a re-fetch. About is transient: it
// is not pushed onto history.
func (m *appModel) openAbout() {
	m.flash = ""
	m.aboutFromState = m.state
	m.state = stateAbout
	m.resize()
}

// closeAbout returns from the about view to the screen it was opened from.
func (m *appModel) closeAbout() {
	m.state = m.aboutFromState
	m.resize()
}

// openHelp shows the full-height help panel.
func (m *appModel) openHelp() {
	m.help = true
	m.helpModel.ShowAll = true
	m.resize()
}

// closeHelp hides the help panel. The caller re-sizes (or opens the about
// screen, which sizes itself) depending on where it lands next.
func (m *appModel) closeHelp() {
	m.help = false
	m.helpModel.ShowAll = false
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
		m.resize()
		return m, nil

	case tea.ColorProfileMsg:
		m.common.profile = msg.Profile
		m.reader.setProfile(msg.Profile)
		m.about.setProfile(msg.Profile)
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

	// Help panel: any key closes it — except 'a', which opens the about screen.
	if m.help {
		m.closeHelp()
		if key.Matches(msg, m.keys.About) {
			m.openAbout()
			return true, m, nil
		}
		m.resize()
		return true, m, nil
	}

	// About screen: its own keys, ahead of the input-focus branch.
	if m.state == stateAbout {
		switch {
		case key.Matches(msg, m.keys.Open): // ↵ finger the author
			m.closeAbout()
			target, err := finger.ParseTarget(aboutFingerAuthor)
			if err != nil {
				return true, m, nil
			}
			return true, m, m.startFetch(target)
		case key.Matches(msg, m.keys.Copy): // y copy the issues URL
			// setFlash mutates m, so sequence it before the return reads m by
			// value (handleKey returns m, not *m): operand order is unspecified.
			flash := m.setFlash("copied " + aboutIssuesURL)
			return true, m, tea.Batch(setClipboard(aboutIssuesURL), flash)
		case key.Matches(msg, m.keys.About), key.Matches(msg, m.keys.Back): // a / esc close
			m.closeAbout()
			return true, m, nil
		case key.Matches(msg, m.keys.Quit): // q quit
			return true, m, tea.Quit
		}
		return true, m, nil // swallow any other key on the about screen
	}

	// Input focused: Enter/Esc/? are commands; everything else types. '?' opens
	// help (it can't appear in a finger address, and the landing — input focused
	// — is exactly where a first-time user reaches for help).
	if m.inputFocused {
		switch {
		case key.Matches(msg, m.keys.Help): // ?
			m.openHelp()
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
		m.openHelp()
		return true, m, nil
	case key.Matches(msg, m.keys.About):
		m.openAbout()
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
	var target finger.Target
	var err error
	if sel.target != "" {
		// A target extracted from the server's own response (a finger:// link or
		// "finger user@host" command) could point at an arbitrary host:port.
		// ParseTargetPinned forces port 79 so a malicious response can't steer
		// lookit at another service (e.g. host:22), discarding the response's
		// port rather than letting a bad one block the drill.
		target, err = finger.ParseTargetPinned(sel.target)
	} else {
		// Build login@host from the host's original argument (minus the leading
		// "@"), preserving any explicit :port the user typed.
		host := strings.TrimPrefix(m.list.host.Raw, "@")
		target, err = finger.ParseTarget(sel.login + "@" + host)
	}
	if err != nil {
		if errors.Is(err, finger.ErrServerForwarding) {
			return true, m, m.setFlash(err.Error())
		}
		return true, m, nil
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
		if parsed, ok := parseUserList(entry.Body, entry.Target.HostPort); ok {
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
	return entry.Target.HostQuery() ||
		(entry.Target.QueryLine() == "ring" && strings.HasPrefix(entry.Target.HostPort, "thebackupbox.net:"))
}

// clearFlashMsg is sent after a flash timer fires to clear m.flash.
type clearFlashMsg struct{}

// clearFlashCmd returns a command that fires clearFlashMsg after 2 seconds.
func (m *appModel) clearFlashCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })
}

// setFlash shows a transient status message and returns the command that clears
// it. Use it for self-expiring flashes (copies, "nothing to copy"); the parse
// error in submit() is deliberately persistent and sets m.flash directly.
func (m *appModel) setFlash(msg string) tea.Cmd {
	m.flash = msg
	return m.clearFlashCmd()
}

// copyAddress copies the relevant address to the clipboard and flashes it.
func (m *appModel) copyAddress() tea.Cmd {
	var addr string
	if m.state == stateList {
		if sel, ok := m.list.selected(); ok {
			if sel.target != "" {
				// Mirror drill's safety: a server-supplied target could point at
				// an arbitrary host:port, so pin to finger's port 79 before copying
				// so a pasted-back address can't be steered at another service.
				// Forwarded targets are refused explicitly; other parse errors
				// still copy nothing.
				if t, err := finger.ParseTargetPinned(sel.target); err == nil {
					addr = t.Raw
				} else if errors.Is(err, finger.ErrServerForwarding) {
					return m.setFlash(err.Error())
				}
			} else {
				addr = sel.login + "@" + strings.TrimPrefix(m.list.host.Raw, "@")
			}
		}
	} else if m.pos >= 0 {
		addr = m.history[m.pos].entry.Target.Raw
	}
	if addr == "" {
		return m.setFlash("nothing to copy")
	}
	return tea.Batch(setClipboard(addr), m.setFlash("copied "+addr))
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
	m.keys.About.SetEnabled(true)
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

	if m.state == stateAbout {
		// The about screen's own actions are live regardless of input focus.
		m.keys.Open.SetEnabled(true)
		m.keys.Copy.SetEnabled(true)
		m.keys.Back.SetEnabled(true)
		m.keys.Quit.SetEnabled(true)
	}
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
	if m.loading {
		bar := statusBar{width: m.common.width, styles: m.common.styles}
		bar.hints = m.spin.View() + " loading " + m.loadingTarget.Raw
		return bar // flash is intentionally suppressed while loading
	}
	bar := m.buildStatusBar()
	if m.flash != "" {
		bar.hints = m.flash // a transient flash message overrides the resting hints
	}
	return bar
}

// buildStatusBar assembles the bar for the current (non-loading) screen. The
// flash override is applied once by statusBarModel, so each branch here sets
// bar.hints to its resting value without repeating the check.
func (m appModel) buildStatusBar() statusBar {
	st := m.common.styles
	w := m.common.width
	if m.state == stateAbout {
		bar := statusBar{width: w, styles: st}
		if m.pos >= 0 {
			bar.escTarget = m.history[m.pos].entry.Target.Raw
		} else {
			bar.host = "about lookit"
		}
		parts := []string{"↵ go", "y copy"}
		if bar.escTarget == "" {
			parts = append(parts, "esc back")
		}
		parts = append(parts, "q quit")
		bar.hints = strings.Join(parts, " · ")
		return bar
	}
	if m.pos < 0 {
		return landingBar(w, st)
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
		return bar
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
		parts := []string{"↵ go", "/ filter"}
		if node.listGeneric {
			bar.flags = append(bar.flags, "auto-detected")
			parts = append(parts, "v view source")
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
	return bar
}

// resize re-sizes the active sub-models to the available body height (the screen
// minus the target row and the status bar). The help panel is drawn as an
// overlay (see View and overlayHelp), so it deliberately does NOT affect
// sub-model sizing: toggling help must not re-paginate the list.
func (m *appModel) resize() {
	h := m.common.height - m.topChromeHeight() - 1
	if h < 1 {
		h = 1
	}
	m.reader.setSize(m.common.width, h)
	if m.listReady {
		m.list.setSize(m.common.width, h)
	}
	ah := m.common.height - 1
	if ah < 1 {
		ah = 1
	}
	m.about.setSize(m.common.width, ah)
}

func (m appModel) helpView() string {
	st := m.common.styles
	w := m.common.width
	return fullWidthHelpView(m.keys.FullHelp(), st, w, m.helpModel.FullSeparator)
}

func (m appModel) topChromeHeight() int {
	return 1 // one target row; the wordmark now lives only on the about screen
}

func (m appModel) inputChromeView() string {
	return m.input.View()
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

	if m.state == stateAbout {
		bottom := m.statusBarModel().render()
		v := tea.NewView(m.about.View() + "\n" + bottom)
		v.AltScreen = true
		return v
	}

	var content string
	switch m.state {
	case stateList:
		content = m.list.View()
	default:
		content = m.reader.View()
	}
	body := m.inputChromeView() + "\n" + content
	if m.help {
		// Draw help over the bottom rows of the content rather than below it, so
		// the content keeps its height and a paginated list does not re-paginate
		// when the panel toggles.
		body = overlayHelp(body, m.helpView())
	}
	full := body + "\n" + m.statusBarModel().render()

	v := tea.NewView(full)
	v.AltScreen = true
	return v
}

// overlayHelp draws the help panel over the bottom rows of body, replacing those
// lines rather than pushing them down. Help lines are full-width opaque bands
// (see fullWidthHelpView), so a line-level replace suffices — no alpha
// compositing — and the content underneath keeps its height.
func overlayHelp(body, help string) string {
	if help == "" {
		return body
	}
	bodyLines := strings.Split(body, "\n")
	helpLines := strings.Split(help, "\n")
	if n := len(helpLines); n > len(bodyLines) {
		helpLines = helpLines[n-len(bodyLines):]
	}
	copy(bodyLines[len(bodyLines)-len(helpLines):], helpLines)
	return strings.Join(bodyLines, "\n")
}

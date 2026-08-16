package tui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

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

// targetPlaceholder is the greyed-out hint in the empty target input. It
// teaches the two input shapes and deliberately names no destination:
// suggesting somewhere to go is the startpage's job, and its catalog and
// bookmark rows render directly below this one. It also promises nothing about
// what a bare "@host" returns, because the protocol doesn't.
const targetPlaceholder = "user@host or @host"

// appState selects which sub-model is active.
type appState int

const (
	stateReader appState = iota
	stateList
	stateAbout
	stateStart // the launch screen; used only at pos == -1, never in a histNode
)

// commonModel is state shared across sub-models.
type commonModel struct {
	ctx            context.Context
	width          int
	height         int
	profile        colorprofile.Profile
	darkBackground bool
	contentFocused bool
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

type refreshViewState struct {
	state      appState
	scrollY    int
	linkRaw    string
	listFilter string
	selected   userItem
}

// appModel is the top-level state machine. It routes input and fetch results
// between the reader and the list, and owns quit/back behavior.
type appModel struct {
	common *commonModel
	state  appState
	reader readerModel
	list   listModel
	about  aboutModel
	start  startModel

	aboutFromState appState // state to restore when the about screen closes

	input        textinput.Model
	inputFocused bool
	seeded       bool // a CLI positional arg was supplied; replay it on Init
	keys         keyMap

	pending        *pendingRequest
	requestFailure *requestFailure
	reqSeq         uint64 // monotonic id of the most recently started fetch
	spin           spinner.Model

	flash string

	history      []histNode
	pos          int  // -1 == landing (nothing fetched yet)
	showingRaw   bool // v-toggled "view source" of the current node's raw body
	showingLinks bool // L-toggled links panel overlay
	linksPanel   linksPanel
	help         bool // help panel open
	listReady    bool
}

func newApp(fetch FetchFunc, profile colorprofile.Profile) appModel {
	return newAppWithContext(context.Background(), fetch, profile, Options{})
}

func newAppWithOptions(fetch FetchFunc, profile colorprofile.Profile, opts Options) appModel {
	return newAppWithContext(context.Background(), fetch, profile, opts)
}

func newAppWithContext(ctx context.Context, fetch FetchFunc, profile colorprofile.Profile, opts Options) appModel {
	if ctx == nil {
		ctx = context.Background()
	}
	if fetch == nil {
		fetch = defaultFetch
	}
	st := newStyles(true)
	common := &commonModel{
		ctx:            ctx,
		profile:        profile,
		darkBackground: true,
		contentFocused: false,
		styles:         st,
		fetch:          fetch,
	}
	in := textinput.New()
	in.Placeholder = targetPlaceholder
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
		spin:         spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(st.spinner)),
		pos:          -1,
	}
	app.reader.setBackground(common.darkBackground)
	app.reader.styles = st
	app.about.setBackground(common.darkBackground)
	app.state = stateStart
	app.reloadStart()
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
	m.spin.Style = st.spinner
	m.reader.styles = st
	m.about.setBackground(m.common.darkBackground)
	m.start.applyStyles(st)
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

func (m appModel) captureRefreshView() refreshViewState {
	view := refreshViewState{state: m.state}
	switch m.state {
	case stateList:
		view.listFilter = m.list.list.FilterValue()
		view.selected, _ = m.list.selected()
	case stateReader:
		view.scrollY = m.reader.viewport.YOffset()
		if m.reader.focusedLink >= 0 && m.reader.focusedLink < len(m.reader.links) {
			view.linkRaw = m.reader.links[m.reader.focusedLink].Raw
		}
	}
	return view
}

func (m *appModel) restoreRefreshView(view refreshViewState) {
	if view.state != m.state {
		return
	}
	if m.state == stateList {
		if view.listFilter != "" {
			m.list.list.SetFilterText(view.listFilter)
		}
		m.list.selectIdentity(view.selected)
		m.snapshot()
		return
	}
	m.reader.focusedLink = -1
	for i, link := range m.reader.links {
		if view.linkRaw != "" && link.Raw == view.linkRaw {
			m.reader.focusedLink = i
			break
		}
	}
	// Re-render: showRouted already set the entry, but it did so before the
	// link focus above was restored, so the refreshed body carries no
	// highlight until the entry goes through the reader a second time.
	node := m.history[m.pos]
	m.reader.setEntryWithLinks(node.entry, node.links)
	m.reader.viewport.SetYOffset(view.scrollY)
	m.snapshot()
}

// restore rebuilds the active sub-model from a node (no network).
func (m *appModel) restore(n histNode) {
	m.setAddress(n.entry.Target.Raw)
	if n.state == stateReader {
		m.state = stateReader
		m.reader.links = n.links
		m.reader.focusedLink = n.linkIdx
		m.reader.setEntryWithLinks(n.entry, n.links)
		m.reader.viewport.SetYOffset(n.scrollY)
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
	m.reader.links = n.links
	m.reader.focusedLink = n.linkIdx
	m.reader.setEntryWithLinks(n.entry, n.links)
}

// gotoStart returns to the launch screen, reloading it so a bookmark added while
// browsing is present when you walk back.
//
// It deliberately does NOT touch focus. Both callers — stepBack's root
// fall-through and goHome — arrive with content focused (Esc in the input branch
// at pos >= 0 blurs rather than stepping back), and focus should follow how you
// arrived: only launch focuses the input, the way a new browser tab focuses the
// address bar while navigating Home focuses the document. newAppWithContext
// already focuses the input at launch, so nothing is lost by dropping it here.
//
// The exception is an unusable startpage: with `catalog off` and no bookmarks
// there is nothing selectable, so content focus is a dead end. The caller
// returns the Focus cmd in that case.
func (m *appModel) gotoStart() tea.Cmd {
	m.state = stateStart
	m.reader.current = nil
	m.reloadStart()
	m.setAddress("") // drop the stale target; 'i' should open on an empty row
	m.resize()
	if _, ok := m.start.selected(); !ok {
		m.setInputFocused(true)
		return m.input.Focus()
	}
	return nil
}

// bookmarkTarget reports what 'b' acts on for the current screen. On a list it
// is the host, not the highlighted user: 'b' on @tilde.team means "come back to
// this directory". To bookmark a person, drill in and press b there.
func (m appModel) bookmarkTarget() (string, bool) {
	if m.state == stateStart {
		entry, ok := m.start.selected()
		return entry.target, ok
	}
	if m.pos < 0 || m.pos >= len(m.history) {
		return "", false
	}
	return m.history[m.pos].entry.Target.Raw, true
}

// bookmarkVisit reports the visit a new bookmark should record. Bookmarking
// from a landed screen stamps now: that response is on screen, so the visit is
// a fact, and the row would otherwise read as unvisited until the next fetch.
// The startpage stamps nothing — there the target is a row the cursor happens
// to rest on, never a place we went. An errored landing stamps nothing either,
// matching stampVisit: a dial failure is not a visit.
func (m appModel) bookmarkVisit() time.Time {
	if m.state == stateStart || m.pos < 0 || m.pos >= len(m.history) {
		return time.Time{}
	}
	if m.history[m.pos].entry.Err != nil {
		return time.Time{}
	}
	return nowFn()
}

// toggleBookmark adds or removes the current target, then reloads the startpage
// so it reflects the file. Bookmark records contain only the target: the
// protocol cannot establish a kind, and routing remains response-derived.
func (m *appModel) toggleBookmark() tea.Cmd {
	var position startTogglePosition
	hasPosition := false
	if m.state == stateStart {
		position, hasPosition = m.start.captureTogglePosition()
	}

	target, ok := m.bookmarkTarget()
	if !ok {
		return nil
	}
	if err := validateBookmarkRecordTarget(target); err != nil {
		return m.setFlash("error: cannot bookmark: " + err.Error())
	}
	path, err := bookmarksPathFn()
	if err != nil {
		return m.setFlash("error: " + err.Error())
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return m.setFlash("error: " + err.Error())
	}

	file := parseBookmarks(data)
	already := false
	for _, saved := range file.targets {
		if saved == target {
			already = true
			break
		}
	}

	var updated []byte
	var msg string
	if already {
		updated = deleteBookmarkLine(data, target)
		msg = "✓ removed " + target
	} else {
		updated = appendBookmarkLine(data, target, m.bookmarkVisit())
		msg = "✓ bookmarked " + target
	}
	if err := saveBookmarkData(path, updated); err != nil {
		return m.setFlash("error: " + err.Error())
	}
	if m.state == stateStart {
		m.reloadStart()
		restored := false
		if hasPosition && position.filtered != nil {
			m.start.list.SetFilterText(position.filter)
			if len(m.start.list.VisibleItems()) > 0 {
				restored = m.start.selectSectionPosition(*position.filtered)
			} else {
				m.start.list.ResetFilter()
				restored = m.start.selectSectionPosition(position.full)
			}
		} else if hasPosition {
			restored = m.start.selectSectionPosition(position.full)
		}
		m.resize()
		if !restored {
			return tea.Batch(m.focusInput(), m.setFlash(msg))
		}
	}
	return m.setFlash(msg)
}

// goHome is exactly equivalent to holding Esc — focus included: return to the
// startpage, drop the trail, and stay on the content with a row selected. The
// startpage is not a history node, so there is nothing to push.
func (m *appModel) goHome() tea.Cmd {
	m.clearRequestFailure()
	m.showingRaw = false
	m.showingLinks = false
	m.flash = ""
	m.history = nil
	m.pos = -1
	return m.gotoStart() // nil unless the startpage is empty, which focuses the input
}

// reloadStart rebuilds the startpage from disk. Called at construction and
// after every bookmark write, so the screen always reflects the file.
func (m *appModel) reloadStart() {
	file, path := loadBookmarks()
	sections := buildSections(loadCatalog(), file)
	m.start = newStart(m.common, sections, startNotice(file, path), startEmptyMessage(file, path))
}

// startNotice surfaces parse problems rather than swallowing them, naming the
// file actually in use so the user edits the one that has an effect.
//
// Several problems report the first in full and count the rest. The startpage
// budgets one unwrapped row for the notice; one described problem is the
// actable one, and fixing it usually explains its neighbours.
func startNotice(file bookmarkFile, path string) string {
	if len(file.problems) == 0 {
		return ""
	}
	shown := shortenHome(path)
	first := file.problems[0]
	where := shown + ":"
	if first.line > 0 {
		where = fmt.Sprintf("%s line %d:", shown, first.line)
	}
	notice := fmt.Sprintf("%s %s", where, first.reason)
	if rest := len(file.problems) - 1; rest > 0 {
		notice += fmt.Sprintf(" (+%d)", rest)
	}
	return notice
}

// startEmptyMessage explains a blank startpage instead of letting it look
// broken. It quotes the resolved path: with $XDG_CONFIG_HOME set, the
// ~/.config fallback would send the user to edit a file with no effect.
func startEmptyMessage(file bookmarkFile, path string) string {
	if file.catalogHidden {
		return fmt.Sprintf("No bookmarks yet. The catalog is off — remove `catalog off` from %s to see it.", shortenHome(path))
	}
	return "No bookmarks yet."
}

// stepBack moves one step toward history root, or to the startpage from pos 0.
func (m *appModel) stepBack() tea.Cmd {
	m.clearRequestFailure()
	m.showingRaw = false
	m.showingLinks = false
	if m.pos < 0 {
		return nil
	}
	m.snapshot()
	m.pos--
	if m.pos < 0 {
		return m.gotoStart()
	}
	m.restore(m.history[m.pos])
	return nil
}

// back is Esc semantics: step back, or quit when already at the landing.
func (m *appModel) back() tea.Cmd {
	m.flash = ""
	if m.pos < 0 {
		return tea.Quit
	}
	return m.stepBack()
}

// focusInput gives the keyboard to the target input, pre-filled with the
// current target for browser-style editing.
func (m *appModel) setInputFocused(focused bool) {
	m.inputFocused = focused
	m.common.contentFocused = !focused
}

func (m *appModel) setAddress(raw string) {
	m.input.SetValue(raw)
}

func (m *appModel) restoreVisibleAddress() {
	raw := ""
	if m.pos >= 0 && m.pos < len(m.history) {
		raw = m.history[m.pos].entry.Target.Raw
	}
	m.setAddress(raw)
}

func (m *appModel) focusInput() tea.Cmd {
	m.clearRequestFailure()
	m.restoreVisibleAddress()
	m.setInputFocused(true)
	m.input.CursorEnd()
	m.resize()
	return m.input.Focus()
}

// blurInput returns the keyboard to the content.
func (m *appModel) blurInput() {
	m.setInputFocused(false)
	m.input.Blur()
	m.resize()
}

// openAbout switches to the full-screen about view, remembering the current
// state so closeAbout can restore it without a re-fetch. About is transient: it
// is not pushed onto history.
func (m *appModel) openAbout() {
	m.clearRequestFailure()
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

// openHelp shows the help panel.
func (m *appModel) openHelp() {
	m.help = true
}

// closeHelp hides the help panel.
func (m *appModel) closeHelp() {
	m.help = false
}

func replayKey(msg tea.KeyPressMsg) tea.Cmd {
	return func() tea.Msg { return msg }
}

// enterRaw shows the current node's unprocessed body ("view source") in the
// reader viewport. It works over any node (list or profile); the underlying
// node.state is preserved in history so exitRaw can return to it.
func (m *appModel) enterRaw() {
	if m.pos < 0 {
		return
	}
	m.clearRequestFailure()
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
		m.reader.setEntryWithLinks(node.entry, node.links) // re-render the profile
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
	cmd := m.startRequest(target, requestNavigate, false)
	m.blurInput()
	return cmd
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
	if m.common.ctx.Done() != nil {
		cmds = append(cmds, waitForSessionCancel(m.common.ctx))
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

	case list.FilterMatchesMsg:
		// Bubbles does not identify which query produced a filter result. Once
		// the active list has accepted or reset its filter, any queued result is
		// stale and must not replace the synchronously settled selection.
		if !m.helpFilterActive() {
			return m, nil
		}

	case sessionCanceledMsg:
		_ = m.cancelRequest()
		return m, tea.Quit

	case fetchResultMsg:
		if m.common.ctx != nil && m.common.ctx.Err() != nil {
			_ = m.cancelRequest()
			return m, tea.Quit
		}
		request, ok := m.finishRequest(msg.reqID)
		if !ok {
			return m, nil
		}
		if request.intent == requestRefresh {
			return m.landRefresh(msg.entry, request)
		}
		return m.landNavigation(msg.entry)

	case clearFlashMsg:
		m.flash = ""
		return m, nil

	case seedSubmitMsg:
		cmd := m.submit()
		return m, cmd

	case spinner.TickMsg:
		if m.pending != nil {
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
	// The links panel is an overlay that owns the body while it is open, so it
	// takes the delegated messages too. Routing on m.state alone sent them to
	// the view underneath — for a panel over a profile, the reader — and the
	// panel's own list.FilterMatchesMsg never arrived, so its filter accepted a
	// query, showed the prompt, and silently never narrowed anything.
	if m.showingLinks {
		m.linksPanel, cmd = m.linksPanel.update(msg)
		return m, cmd
	}
	switch m.state {
	case stateList:
		m.list, cmd = m.list.update(msg)
	case stateStart:
		m.start, cmd = m.start.update(msg)
	default:
		m.reader, cmd = m.reader.update(msg)
	}
	return m, cmd
}

// handleKey processes app-level keys and focus routing. handled=false lets the
// caller delegate the key to the active sub-model (content) or the input.
func (m appModel) handleKey(msg tea.KeyPressMsg) (bool, appModel, tea.Cmd) {
	if m.pending != nil {
		switch {
		case key.Matches(msg, m.keys.ForceQuit), key.Matches(msg, m.keys.Quit):
			_ = m.cancelRequest()
			return true, m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			return true, m, m.cancelRequest()
		default:
			return true, m, nil
		}
	}

	if key.Matches(msg, m.keys.ForceQuit) {
		return true, m, tea.Quit
	}

	if m.help {
		switch {
		case key.Matches(msg, m.keys.Help), key.Matches(msg, m.keys.Back):
			m.closeHelp()
			m.resize()
			return true, m, nil
		}

		layout := m.helpLayout()
		if !layout.matches(msg) {
			return true, m, nil
		}
		m.closeHelp()
		if key.Matches(msg, m.keys.About) {
			m.openAbout()
			return true, m, nil
		}
		m.resize()
		return true, m, replayKey(msg)
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
			return true, m, m.startRequest(target, requestNavigate, false)
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
		case key.Matches(msg, m.keys.Browse) && m.state == stateStart:
			if _, ok := m.start.selected(); ok {
				m.blurInput()
				return true, m, nil
			}
			return false, m, nil // nothing to browse: let ↓ fall through to the input
		case key.Matches(msg, m.keys.Open): // Enter
			cmd := m.submit()
			return true, m, cmd
		case key.Matches(msg, m.keys.Back): // Esc
			if m.pos < 0 {
				return true, m, tea.Quit
			}
			m.restoreVisibleAddress()
			m.blurInput()
			return true, m, nil
		}
		return false, m, nil // fall through: type into the input
	}

	// Content focused.
	// The accept key is intercepted so the filter is applied synchronously;
	// bubbles' own accept can apply a stale match set, leaving the following
	// key to act on the pre-filter row (issue #129). Every other filter key
	// belongs to the list.
	if m.state == stateList && m.list.filtering() {
		if key.Matches(msg, m.list.list.KeyMap.AcceptWhileFiltering) {
			acceptFilter(&m.list.list)
			return true, m, nil
		}
		return false, m, nil // list owns its filter keys
	}
	if m.state == stateStart && m.start.filtering() {
		if key.Matches(msg, m.start.list.KeyMap.AcceptWhileFiltering) {
			m.start.acceptFilter()
			return true, m, nil
		}
		return false, m, nil
	}
	if m.state == stateStart && m.start.filterApplied() && key.Matches(msg, m.keys.Back) {
		return false, m, nil
	}
	if m.showingLinks && m.linksPanel.filtering() {
		if key.Matches(msg, m.linksPanel.list.KeyMap.AcceptWhileFiltering) {
			acceptFilter(&m.linksPanel.list)
			return true, m, nil
		}
		var cmd tea.Cmd
		m.linksPanel, cmd = m.linksPanel.update(msg)
		return true, m, cmd
	}
	if m.showingLinks && m.linksPanel.filterApplied() && key.Matches(msg, m.keys.Back) {
		var cmd tea.Cmd
		m.linksPanel, cmd = m.linksPanel.update(msg)
		return true, m, cmd
	}

	// Links panel: when open, panel-mode keys are handled before the main switch.
	if m.showingLinks {
		switch {
		case key.Matches(msg, m.keys.Help):
			m.openHelp()
			return true, m, nil
		case key.Matches(msg, m.keys.Back) || key.Matches(msg, m.keys.LinkPanel):
			m.showingLinks = false
			if m.pos >= 0 {
				node := m.history[m.pos]
				if sel, ok := m.linksPanel.selected(); ok {
					for i, l := range node.links {
						if l.Raw == sel.Raw {
							m.reader.focusedLink = i
							break
						}
					}
				}
				m.reader.setEntryWithLinks(node.entry, node.links)
			}
			return true, m, nil
		case key.Matches(msg, m.keys.Open) && m.pos >= 0:
			node := &m.history[m.pos]
			if sel, ok := m.linksPanel.selected(); ok {
				for i, l := range node.links {
					if l.Raw == sel.Raw {
						m.reader.focusedLink = i
						break
					}
				}
				switch actionsForLink(sel).enter {
				case linkEnterGo:
					m.showingLinks = false
					return true, m, m.startRequest(sel.Target, requestNavigate, false)
				case linkEnterRefuse:
					return true, m, m.setFlash(sel.Blocked)
				default:
					return true, m, nil
				}
			}
			return true, m, nil
		case key.Matches(msg, m.keys.LinkFinger):
			if sel, ok := m.linksPanel.selected(); ok {
				if actionsForLink(sel).finger {
					m.showingLinks = false
					return true, m, m.startRequest(sel.Target, requestNavigate, false)
				}
			}
			return true, m, nil
		case key.Matches(msg, m.keys.Copy):
			if sel, ok := m.linksPanel.selected(); ok && actionsForLink(sel).copy {
				flash := m.setFlash("copied " + sel.Raw)
				return true, m, tea.Batch(setClipboard(sel.Raw), flash)
			}
			return true, m, nil
		case key.Matches(msg, m.keys.Quit):
			return true, m, tea.Quit
		}
		// Delegate remaining keys to the panel list.
		var cmd tea.Cmd
		m.linksPanel, cmd = m.linksPanel.update(msg)
		return true, m, cmd
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
	// Ahead of the generic Back below, which calls m.back() — that quits at
	// pos < 0, which is exactly the startpage. Here Esc backs out one level,
	// into the target input.
	case key.Matches(msg, m.keys.Back) && m.state == stateStart:
		return true, m, m.focusInput()
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
	case key.Matches(msg, m.keys.Refresh):
		cmd := m.refreshCurrent()
		return true, m, cmd
	case key.Matches(msg, m.keys.Bookmark):
		return true, m, m.toggleBookmark()
	case key.Matches(msg, m.keys.Home):
		return true, m, m.goHome()
	case key.Matches(msg, m.keys.Open) && m.state == stateStart:
		entry, ok := m.start.selected()
		if !ok {
			return true, m, nil
		}
		target, err := finger.ParseTarget(entry.target)
		if err != nil {
			return true, m, m.setFlash("error: " + err.Error())
		}
		return true, m, m.startRequest(target, requestNavigate, false)
	case key.Matches(msg, m.keys.Open) && m.state == stateList:
		return m.drill()
	case key.Matches(msg, m.keys.Open) && m.state == stateReader && m.pos >= 0:
		node := &m.history[m.pos]
		if m.reader.focusedLink >= 0 && m.reader.focusedLink < len(node.links) {
			return m.activateFocusedLink(node)
		}
		return false, m, nil // no focused link — fall through
	case key.Matches(msg, m.keys.Raw) && m.pos >= 0:
		if m.showingRaw {
			m.exitRaw()
		} else {
			m.enterRaw()
		}
		return true, m, nil
	case key.Matches(msg, m.keys.LinkNext) && m.pos >= 0:
		node := &m.history[m.pos]
		m.reader.nextLink(len(node.links))
		node.linkIdx = m.reader.focusedLink
		m.reader.setEntryWithLinks(node.entry, node.links)
		return true, m, nil
	case key.Matches(msg, m.keys.LinkPrev) && m.pos >= 0:
		node := &m.history[m.pos]
		m.reader.prevLink(len(node.links))
		node.linkIdx = m.reader.focusedLink
		m.reader.setEntryWithLinks(node.entry, node.links)
		return true, m, nil
	case key.Matches(msg, m.keys.LinkFinger) && m.pos >= 0:
		node := &m.history[m.pos]
		if m.reader.focusedLink >= 0 && m.reader.focusedLink < len(node.links) {
			link := node.links[m.reader.focusedLink]
			if actionsForLink(link).finger {
				cmd := m.startRequest(link.Target, requestNavigate, false)
				return true, m, cmd
			}
		}
		return true, m, nil
	case key.Matches(msg, m.keys.LinkPanel) && m.pos >= 0:
		m.clearRequestFailure()
		node := m.history[m.pos]
		m.showingLinks = true
		m.linksPanel = newLinksPanel(m.common, node.links)
		m.linksPanel.setSize(m.common.width, m.common.bodyHeight())
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
	// Keep the current view (the list) on screen while loading; landNavigation sets
	// the final state when the result lands. Switching to the reader eagerly here
	// flashed the previous profile for a frame before the new one arrived.
	cmd := m.startRequest(target, requestNavigate, false)
	return true, m, cmd
}

type routedEntry struct {
	node   histNode
	parsed *parsedUserList
}

func routeEntry(entry Entry) routedEntry {
	routed := routedEntry{node: histNode{entry: entry, state: stateReader, linkIdx: -1}}
	routed.node.links = DetectLinks(entry.Body, entry.Target.HostPort)
	if len(entry.Body) == 0 || !shouldOpenList(entry) {
		return routed
	}
	parsed, ok := parseUserList(entry.Body, entry.Target.HostPort)
	if !ok {
		return routed
	}
	routed.node.state = stateList
	routed.node.listUsers = len(parsed.users)
	routed.node.listGeneric = parsed.generic
	routed.parsed = &parsed
	return routed
}

func (m *appModel) showRouted(routed routedEntry) {
	m.setAddress(routed.node.entry.Target.Raw)
	m.setInputFocused(false)
	m.input.Blur()
	m.showingRaw = false
	m.showingLinks = false
	m.state = routed.node.state
	if routed.node.state == stateList {
		m.list = newListWithPreamble(m.common, routed.node.entry.Target, routed.parsed.users, routed.node.entry.Body, routed.node.listGeneric)
		m.listReady = true
		return
	}
	m.reader.focusedLink = -1
	m.reader.setEntryWithLinks(routed.node.entry, routed.node.links)
}

func (m appModel) landNavigation(entry Entry) (appModel, tea.Cmd) {
	m.snapshot()
	var cmd tea.Cmd
	if entry.Err == nil {
		cmd = stampVisitCmd(entry.Target.Raw)
	}
	routed := routeEntry(entry)
	m.showRouted(routed)
	m.push(routed.node)
	m.requestFailure = nil
	return m, cmd
}

func (m appModel) landRefresh(entry Entry, request pendingRequest) (appModel, tea.Cmd) {
	if m.pos < 0 || m.pos >= len(m.history) {
		return m, nil
	}
	if entry.failed() {
		m.requestFailure = &requestFailure{retry: request.retry, err: entry.Err}
		return m, nil
	}
	var cmd tea.Cmd
	if entry.Err == nil {
		cmd = stampVisitCmd(entry.Target.Raw)
	}
	view := refreshViewState{}
	if request.view != nil {
		view = *request.view
	}
	routed := routeEntry(entry)
	m.history[m.pos] = routed.node
	m.showRouted(routed)
	m.restoreRefreshView(view)
	m.requestFailure = nil
	return m, cmd
}

func (m appModel) shouldRetry() bool {
	if m.requestFailure != nil {
		return true
	}
	if m.pos < 0 || m.pos >= len(m.history) {
		return false
	}
	return m.history[m.pos].entry.failed()
}

func (m appModel) refreshHelp() key.Help {
	if m.shouldRetry() {
		return key.Help{Key: "r", Desc: "retry"}
	}
	return key.Help{Key: "r", Desc: "refresh"}
}

func (m appModel) refreshHint() string {
	help := m.refreshHelp()
	return help.Key + " " + help.Desc
}

func (m *appModel) clearRequestFailure() {
	m.requestFailure = nil
}

func (m *appModel) refreshCurrent() tea.Cmd {
	if m.pos < 0 || m.pos >= len(m.history) {
		return nil
	}
	m.snapshot()
	view := m.captureRefreshView()
	cmd := m.startRequest(m.history[m.pos].entry.Target, requestRefresh, m.shouldRetry())
	m.pending.view = &view
	return cmd
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

// linkKindLabel returns a short human-readable label for the kind of link.
func linkKindLabel(l Link) string {
	if l.Blocked != "" {
		return "forwarded finger"
	}
	switch l.Kind {
	case LinkFinger:
		if l.Ambiguous {
			return "address (ambiguous)"
		}
		return "finger"
	case LinkURL:
		return "url"
	case LinkEmail:
		return "email"
	case LinkSocial:
		return "social"
	}
	return "link"
}

func (m appModel) focusedReaderLink() (Link, bool) {
	if m.state != stateReader || m.pos < 0 || m.pos >= len(m.history) {
		return Link{}, false
	}
	node := m.history[m.pos]
	if m.reader.focusedLink < 0 || m.reader.focusedLink >= len(node.links) {
		return Link{}, false
	}
	return node.links[m.reader.focusedLink], true
}

// activateFocusedLink dispatches the default action for the currently focused link.
func (m appModel) activateFocusedLink(node *histNode) (bool, appModel, tea.Cmd) {
	link := node.links[m.reader.focusedLink]
	switch actionsForLink(link).enter {
	case linkEnterGo:
		return true, m, m.startRequest(link.Target, requestNavigate, false)
	case linkEnterRefuse:
		return true, m, m.setFlash(link.Blocked)
	default:
		return true, m, nil
	}
}

// copyAddress copies the relevant address to the clipboard and flashes it.
func (m *appModel) copyAddress() tea.Cmd {
	if m.state == stateReader && m.pos >= 0 {
		node := m.history[m.pos]
		if m.reader.focusedLink >= 0 && m.reader.focusedLink < len(node.links) {
			raw := node.links[m.reader.focusedLink].Raw
			return tea.Batch(setClipboard(raw), m.setFlash("copied "+raw))
		}
	}
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
// updateKeymap models binding availability in the active interaction layer. It
// is the single source of truth with two effects: the expanded '?' help panel
// skips disabled bindings (bubbles/help), and key.Matches treats a disabled
// binding as no-match — so a content key is inert (types literally) while the
// input is focused. The Help layout's retained set is the final
// execute-through-Help gate. It must run before both handleKey (routing) and
// the render path (help panel); Update and View call it. Pattern: pop's
// updateKeymap (~/pop/keymap.go).
func (m appModel) helpFilterActive() bool {
	return (m.state == stateList && m.list.filtering()) ||
		(m.state == stateStart && m.start.filtering()) ||
		(m.showingLinks && m.linksPanel.filtering())
}

func (m *appModel) updateKeymap() {
	refreshHelp := m.refreshHelp()
	m.keys.Refresh.SetHelp(refreshHelp.Key, refreshHelp.Desc)
	if m.state == stateStart {
		m.keys.Bookmark.SetHelp("b", m.startBookmarkAction())
	} else {
		m.keys.Bookmark.SetHelp("b", "bookmark")
	}
	if m.pending != nil {
		for _, binding := range []*key.Binding{
			&m.keys.Open, &m.keys.FocusInput, &m.keys.Filter, &m.keys.Raw,
			&m.keys.Copy, &m.keys.Help, &m.keys.About, &m.keys.Move,
			&m.keys.Page, &m.keys.Jump, &m.keys.LinkNext, &m.keys.LinkPrev,
			&m.keys.LinkFinger, &m.keys.LinkPanel, &m.keys.Refresh,
			&m.keys.Bookmark, &m.keys.Home,
		} {
			binding.SetEnabled(false)
		}
		m.keys.Back.SetEnabled(true)
		m.keys.Quit.SetEnabled(true)
		m.keys.ForceQuit.SetEnabled(true)
		return
	}
	content := !m.inputFocused
	hasResult := m.pos >= 0
	inList := content && m.state == stateList && !m.showingRaw
	canRefresh := content && hasResult &&
		(m.state == stateReader || m.state == stateList) &&
		!m.showingRaw && !m.showingLinks &&
		(m.state != stateList || !m.list.filtering())

	// Dual-mode commands — handleKey matches them in BOTH the input-focused and
	// content branches, so they must stay live while typing: Open=Enter (submit
	// a target / drill a list row), Back=Esc (cancel the edit / history back /
	// quit at the bare landing), Help='?'.
	filtering := m.helpFilterActive()
	m.keys.Help.SetEnabled(m.state != stateAbout && !filtering)
	m.keys.About.SetEnabled(
		m.state == stateAbout ||
			m.help ||
			(content && !filtering && !m.showingLinks),
	)
	inStart := content && m.state == stateStart
	_, startHasSelection := m.start.selected()
	startHasSelection = inStart && startHasSelection

	m.keys.Open.SetEnabled(m.inputFocused || inList || startHasSelection)
	m.keys.Back.SetEnabled(m.inputFocused || (content && hasResult) || m.state == stateStart)
	_, browsable := m.start.selected()
	m.keys.Browse.SetEnabled(m.inputFocused && m.state == stateStart && browsable)

	// Content-only keys — inert while the input is focused (they type literally).
	m.keys.FocusInput.SetEnabled(content)
	m.keys.Quit.SetEnabled(content)
	m.keys.Copy.SetEnabled(content && hasResult)
	m.keys.Raw.SetEnabled(content && hasResult)
	m.keys.Filter.SetEnabled(inList || startHasSelection)
	m.keys.Move.SetEnabled(content)
	m.keys.Page.SetEnabled(content)
	m.keys.Jump.SetEnabled(content)
	m.keys.Refresh.SetEnabled(canRefresh)
	_, canBookmark := m.bookmarkTarget()
	m.keys.Bookmark.SetEnabled(content && canBookmark && !m.showingLinks)
	m.keys.Home.SetEnabled(content && (m.pos >= 0 || m.state != stateStart))

	inReader := content && m.state == stateReader && !m.showingRaw
	hasReaderLinks := false
	if m.state == stateReader && m.pos >= 0 && m.pos < len(m.history) {
		hasReaderLinks = len(m.history[m.pos].links) > 0
	}
	m.keys.LinkNext.SetEnabled(inReader && hasReaderLinks && !m.showingLinks)
	m.keys.LinkPrev.SetEnabled(inReader && hasReaderLinks && !m.showingLinks)
	m.keys.LinkFinger.SetEnabled(false)
	m.keys.LinkPanel.SetEnabled((inReader && hasReaderLinks) || m.showingLinks)
	if inReader {
		if link, ok := m.focusedReaderLink(); ok {
			actions := actionsForLink(link)
			m.keys.Open.SetEnabled(actions.enter != linkEnterNone)
			m.keys.LinkFinger.SetEnabled(actions.finger)
		} else {
			m.keys.Open.SetEnabled(false)
		}
	}
	if m.showingLinks {
		m.keys.Back.SetEnabled(true)
		m.keys.LinkPanel.SetEnabled(true)
		m.keys.Filter.SetEnabled(!m.linksPanel.filtering())
		m.keys.Open.SetEnabled(false)
		m.keys.LinkFinger.SetEnabled(false)
		if link, ok := m.linksPanel.selected(); ok {
			actions := actionsForLink(link)
			m.keys.Open.SetEnabled(actions.enter != linkEnterNone)
			m.keys.LinkFinger.SetEnabled(actions.finger)
		}
	}

	if m.state == stateAbout {
		// The about screen's own actions are live regardless of input focus.
		m.keys.Open.SetEnabled(true)
		m.keys.Copy.SetEnabled(true)
		m.keys.Back.SetEnabled(true)
		m.keys.Quit.SetEnabled(true)
		m.keys.About.SetEnabled(true)
	}
}

// filterModeHints names what the keys do while a bubbles filter is open, for
// any of the three lists that have one. The links panel established this
// vocabulary; the startpage and user list use the same words rather than
// showing resting hints ("/ filter" while already filtering, "↵ go" with
// nothing to go to) that are not true mid-filter.
//
// matches gates the "enter apply" promise. Bubbles refuses to apply a filter
// that matched nothing — Enter drops back to the unfiltered list, exactly what
// Esc does — so on a zero-match query Esc is the only key worth naming.
func filterModeHints(value string, matches int) string {
	switch {
	case value == "":
		return "type to filter · esc cancel"
	case matches == 0:
		return "esc cancel"
	default:
		return "enter apply · esc cancel"
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
	if m.pending != nil {
		priority := m.pendingPriorityStatus(time.Now())
		return statusBar{width: m.common.width, styles: m.common.styles, priority: &priority}
	}
	bar := m.buildStatusBar()
	if m.flash != "" {
		bar.hints = m.flash // a transient flash message overrides the resting hints
		return bar
	}
	if m.help {
		// The overlay is showing these same commands two lines up, laid out in
		// columns that fit. State stays: the address, byte count, page, scroll
		// percentage, latency and flags appear nowhere else.
		bar.hints = ""
	}
	if m.requestFailure != nil && (m.state != stateList || !m.list.filtering()) {
		priority := m.requestFailure.priorityStatus()
		bar.hints = ""
		bar.priority = &priority
	}
	return bar
}

// buildStatusBar assembles the bar for the current (non-loading) screen.
// statusBarModel applies transient flash and refresh-failure overrides, so each
// branch here sets bar.hints to its resting value without repeating the check.
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
		parts := []string{"↵ go to author", "y copy issues URL"}
		if bar.escTarget == "" {
			parts = append(parts, "esc back")
		}
		parts = append(parts, "q quit")
		bar.hints = strings.Join(parts, " · ")
		return bar
	}
	if m.pos < 0 {
		return m.startBar(w, st)
	}
	node := m.history[m.pos]
	bar := statusBar{width: w, styles: st, latency: formatElapsed(node.entry.Meta.Elapsed)}
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

	if m.showingLinks {
		// Esc in this overlay acts on the panel — closing it, or clearing its
		// filter — and returns to the same reader at the same history position.
		// It does not pop history, so don't promise a back-to-previous-target
		// breadcrumb (the same rule view-source follows above). Clearing it here
		// rather than per-case also keeps joinHints' invariant by hand: every
		// branch below names esc exactly once.
		bar.escTarget = ""
		var parts []string
		switch {
		case m.linksPanel.filtering() && m.linksPanel.filterValue() == "":
			bar.hints = "type to filter · esc cancel"
			return bar
		case m.linksPanel.filtering():
			bar.hints = "enter apply · esc cancel"
			return bar
		case m.linksPanel.filterApplied():
			parts = []string{"↑/↓ move", "esc clear filter"}
		default:
			parts = []string{"↑/↓ move", "/ filter", "esc back"}
		}
		if selected, ok := m.linksPanel.selected(); ok {
			parts = append(parts, linkActionHints(selected)...)
		}
		bar.hints = strings.Join(parts, " · ")
		return bar
	}

	switch node.state {
	case stateList:
		bar.meta = countLabel(node.listUsers, "user", "users")
		if m.list.filtering() {
			// Esc cancels the filter here; it does not walk history, so the
			// breadcrumb would name a step this key won't take.
			bar.escTarget = ""
			bar.hints = filterModeHints(m.list.list.FilterValue(), len(m.list.list.VisibleItems()))
			return bar
		}
		parts := []string{"↵ go", "/ filter", m.refreshHint()}
		if node.entry.Err != nil {
			bar.flags = append(bar.flags, "partial (error)")
		} else if node.entry.Meta.Truncated {
			bar.flags = append(bar.flags, "partial (truncated)")
		}
		if node.listGeneric {
			bar.flags = append(bar.flags, "auto-detected")
			parts = append(parts, "v view source")
		}
		bar.hints = joinHints(parts, bar.escTarget)
		if tp := m.list.list.Paginator.TotalPages; tp > 1 {
			bar.page = fmt.Sprintf("page %d/%d", m.list.list.Paginator.Page+1, tp)
		}
	default: // stateReader
		if node.entry.failed() {
			// A read deadline can set Meta.Truncated with an empty body, so skip
			// the truncation flag here as well as meta/scroll.
			bar.hints = joinHints([]string{m.refreshHint()}, bar.escTarget)
			return bar
		}
		bar.meta = formatBytes(len(node.entry.Body))
		// The render footer (which carried the truncation notice) is suppressed
		// in the TUI. The error message still renders in the viewport via the
		// ErrLine, but truncation had no other home, so surface it here.
		if node.entry.Meta.Truncated {
			bar.flags = append(bar.flags, "partial (truncated)")
		}
		// Focused-link mode overrides the resting hints.
		if m.reader.focusedLink >= 0 && m.reader.focusedLink < len(node.links) {
			link := node.links[m.reader.focusedLink]
			n := m.reader.focusedLink + 1
			total := len(node.links)
			label := linkKindLabel(link)
			extra := linkActionHints(link)
			if link.Blocked != "" {
				extra = append(extra, link.Blocked)
			}
			extra = append(extra, "tab next", m.refreshHint())
			bar.hints = fmt.Sprintf("link %d/%d · %s · %s", n, total, label, strings.Join(extra, " · "))
			return bar
		}
		bar.hints = joinHints([]string{"↑↓ scroll", m.refreshHint()}, bar.escTarget)
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
	m.start.setSize(m.common.width, h)
	if m.showingLinks {
		m.linksPanel.setSize(m.common.width, m.common.bodyHeight())
	}
	ah := m.common.height - 1
	if ah < 1 {
		ah = 1
	}
	m.about.setSize(m.common.width, ah)
}

// startBar is the launch screen's bottom bar. It replaces landingBar, which
// had nothing to advertise but typing.
func (m appModel) startBar(width int, st styles) statusBar {
	bar := statusBar{width: width, styles: st}
	// The window into the catalog is small and was barely signposted: at 80x24
	// roughly a third of it is on screen, and bubbles' pagination dots in the
	// corner were the only sign the rest existed. Report position in the words
	// listBar already uses, rather than inventing a second vocabulary for it.
	//
	// Set before the input-focused branch: the app launches with the target
	// input focused, so that is the state in which the page's extent most needs
	// stating. The state ladder sheds it on a narrow bar (statusbar.go), after
	// latency and meta and before any hint is cut.
	if tp := m.start.list.Paginator.TotalPages; tp > 1 {
		bar.page = fmt.Sprintf("page %d/%d", m.start.list.Paginator.Page+1, tp)
	}
	if m.inputFocused {
		bar.hints = "type a target and press ↵ · ↓ browse · ? help"
		return bar
	}
	n := 0
	for _, it := range m.start.list.VisibleItems() {
		if si, ok := it.(startItem); ok && si.countsAsListing() {
			n++
		}
	}
	if n > 0 {
		bar.meta = countLabel(n, "entry", "entries")
	}
	if m.start.filtering() {
		bar.hints = filterModeHints(m.start.list.FilterValue(), len(m.start.list.VisibleItems()))
		return bar
	}
	// "↵ go" and "b …" act on the selected row, so they are offered only when
	// there is one. A filter that matches nothing leaves none.
	var parts []string
	if _, ok := m.start.selected(); ok {
		parts = append(parts, "↵ go", "b "+m.startBookmarkAction())
	}
	if m.start.filterApplied() {
		parts = append(parts, "esc clear filter")
	} else {
		parts = append(parts, "/ filter")
	}
	parts = append(parts, "i target", "? help")
	bar.hints = strings.Join(parts, " · ")
	return bar
}

// startBookmarkAction names what b will do. It reads bookmarked state, not the
// section that rendered the row: a parent copy survives dedup and is still
// sourceCatalog while pinned, so source would advertise the wrong verb.
func (m appModel) startBookmarkAction() string {
	entry, ok := m.start.selected()
	if ok && entry.bookmarked {
		return "remove"
	}
	return "bookmark"
}

func (m appModel) topChromeHeight() int {
	return 1 // one target row; the wordmark now lives only on the about screen
}

func (m appModel) inputChromeView() string {
	return m.input.View()
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
	if m.showingLinks {
		content = m.linksPanel.View()
	} else {
		switch m.state {
		case stateList:
			content = m.list.View()
		case stateStart:
			content = m.start.View()
		default:
			content = m.reader.View()
		}
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
// compositing — and the content underneath keeps its height. If Help has more
// lines than the body can hold, retain the leading priority rows.
func overlayHelp(body, help string) string {
	if help == "" {
		return body
	}
	bodyLines := strings.Split(body, "\n")
	helpLines := strings.Split(help, "\n")
	if n := len(helpLines); n > len(bodyLines) {
		helpLines = helpLines[:len(bodyLines)]
	}
	copy(bodyLines[len(bodyLines)-len(helpLines):], helpLines)
	return strings.Join(bodyLines, "\n")
}

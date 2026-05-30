# TUI navigation & chrome Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give a lookit TUI session a persistent breadcrumb status bar and browser-style back/forward history across every hop.

**Architecture:** `appModel` gains a `history []histNode` + `pos` cursor that replaces the ad-hoc `fromList`/`hostList` back machinery; a new pure `statusBar` renderer composes the bottom chrome; the reader and list shed their own scattered title/status/hint chrome into that one bar.

**Tech Stack:** Go, Bubble Tea v2 (`charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`), `github.com/charmbracelet/x/ansi` for width-aware truncation.

---

## Background for the implementer

This is the `tui/` package only — do not touch `finger/` or `render/`. Read `docs/superpowers/specs/2026-05-29-tui-navigation-chrome-design.md` first; it is the source of truth for behavior.

Key existing facts:
- `appModel` (app.go) is the top-level state machine with a `state appState` (`stateReader`/`stateList`), a `reader readerModel`, a `list listModel`, and a `*commonModel` holding `width/height/profile/fetch`. Today it also has `hostList *Entry`, `fromList bool`, `listReady bool` — **`hostList` and `fromList` are removed by this plan.**
- `Entry` (fetch.go) = `{Target finger.Target; Body []byte; Meta finger.Meta; Err error}`.
- A completed fetch arrives as `fetchResultMsg{entry}` and is handled by `routeFetch`.
- Tests never hit the network: `newApp(stubFetch(t), colorprofile.NoTTY)` builds a model; `stubFetch(t)` **fails the test if called** (use it to prove back/forward don't re-fetch). `fetchRecorder(body)`/`captureFetch` record targets. Keys are `tea.KeyPressMsg{Code: ..., Mod: ...}`. `isQuit(cmd)` (already in app_test.go) reports whether a cmd yields `tea.QuitMsg`.
- v2 APIs you'll use: `viewport.YOffset() int`, `viewport.SetYOffset(int)`, `viewport.ScrollPercent() float64`, `list.Index() int`, `list.Select(int)`, `list.FilterValue() string`, `list.SetFilterText(string)`, `list.SetShowTitle(bool)`, `list.SetShowHelp(bool)`.

Run a single test with: `go test ./tui/ -run TestName -count=1 -v`. Run the full gate with: `make check`.

## File structure

- **Create** `tui/statusbar.go` — pure `statusBar` struct + `render()`; `landingBar`, `breadcrumbParts`, `formatBytes` helpers. One responsibility: turn already-computed parts into one bottom line.
- **Create** `tui/statusbar_test.go` — pure render tests across states.
- **Modify** `tui/styles.go` — add bar segment styles.
- **Modify** `tui/app.go` — history stack (`histNode`, `history`, `pos`), `push`/`stepBack`/`back`/`forward`/`snapshot`/`restore`/`gotoLanding`, keymap (`Esc`, `Alt+←`, `Alt+→`), `routeFetch` push, `r` raw-view via `showingRaw`, `View` composition, help overlay, sizing.
- **Modify** `tui/reader.go` — drop `status`/`hint` lines from `View`; recompute `chromeRows`.
- **Modify** `tui/list.go` — `SetShowTitle(false)`/`SetShowHelp(false)`; expose `users()`/`generic`; size via `bodyHeight`; move flag copy out of `Title` (Task 4).
- **Modify** `tui/app_test.go`, `tui/reader_test.go`, `tui/list_test.go` — update tests that referenced removed fields / old chrome.

---

## Task 1: The status bar renderer (pure)

**Files:**
- Modify: `tui/styles.go`
- Create: `tui/statusbar.go`
- Create: `tui/statusbar_test.go`
- Add dependency: `github.com/charmbracelet/x/ansi`

- [ ] **Step 1: Add bar styles**

Edit `tui/styles.go` — add fields to the `styles` struct and initialize them in `newStyles`. The bar is a uniform subtle-background line, so every segment shares the background:

```go
type styles struct {
	title    lipgloss.Style
	status   lipgloss.Style
	error    lipgloss.Style
	hint     lipgloss.Style
	listName lipgloss.Style
	selected lipgloss.Style

	// bottom status bar
	barFill lipgloss.Style // full-width background
	barHost lipgloss.Style // "@host" (dim)
	barSep  lipgloss.Style // " / " separator
	barUser lipgloss.Style // "user" (bold/bright)
	barFlag lipgloss.Style // neutral flag, e.g. "auto-detected"
	barWarn lipgloss.Style // caution flag, e.g. "partial (truncated)"
	barDim  lipgloss.Style // right-aligned context (esc/meta/hints)
}

func newStyles() styles {
	barBg := lipgloss.Color("#242424")
	seg := lipgloss.NewStyle().Background(barBg)
	return styles{
		title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff6fd5")),
		status:   lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")),
		error:    lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b6b")),
		hint:     lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")),
		listName: lipgloss.NewStyle().Foreground(lipgloss.Color("#8fb7ff")),
		selected: lipgloss.NewStyle().Foreground(lipgloss.Color("#8affc1")).Bold(true),

		barFill: seg,
		barHost: seg.Foreground(lipgloss.Color("#9a9a9a")),
		barSep:  seg.Foreground(lipgloss.Color("#6a6a6a")),
		barUser: seg.Foreground(lipgloss.Color("#ffffff")).Bold(true),
		barFlag: seg.Foreground(lipgloss.Color("#9a9a9a")),
		barWarn: seg.Foreground(lipgloss.Color("#c9a227")),
		barDim:  seg.Foreground(lipgloss.Color("#808080")),
	}
}
```

- [ ] **Step 2: Write the failing renderer tests**

Create `tui/statusbar_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestStatusBarProfileShowsBreadcrumb(t *testing.T) {
	b := statusBar{host: "@tilde.team", user: "jonathan", escTarget: "@tilde.team",
		meta: "1.2 KB", hints: "esc back · ? help", width: 80, styles: newStyles()}
	out := b.render()
	for _, want := range []string{"@tilde.team", "jonathan", "◂ esc: @tilde.team", "1.2 KB", "? help"} {
		if !strings.Contains(out, want) {
			t.Fatalf("bar %q missing %q", out, want)
		}
	}
	if w := lipgloss.Width(out); w != 80 {
		t.Fatalf("bar width = %d, want 80", w)
	}
}

func TestStatusBarDirectoryHasNoUserHalf(t *testing.T) {
	b := statusBar{host: "@tilde.team", meta: "3 users",
		hints: "↵ open · ? help", width: 80, styles: newStyles()}
	out := b.render()
	if strings.Contains(out, " / ") {
		t.Fatalf("directory bar should have no ' / ' separator: %q", out)
	}
	if !strings.Contains(out, "3 users") {
		t.Fatalf("bar %q missing meta", out)
	}
}

func TestStatusBarLandingShowsHint(t *testing.T) {
	out := landingBar(80, newStyles()).render()
	if !strings.Contains(out, "type a target") {
		t.Fatalf("landing bar %q missing hint", out)
	}
}

func TestStatusBarTruncatesBreadcrumbFirst(t *testing.T) {
	b := statusBar{host: "@an-extremely-long-hostname.example.org", user: "verylonguser",
		meta: "1.2 KB", hints: "esc back · ? help", width: 40, styles: newStyles()}
	out := b.render()
	if w := lipgloss.Width(out); w != 40 {
		t.Fatalf("bar width = %d, want 40 (must clamp)", w)
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis when breadcrumb overflows: %q", out)
	}
	if !strings.Contains(out, "? help") {
		t.Fatalf("right-side hints must survive truncation: %q", out)
	}
}

func TestStatusBarZeroWidthIsEmpty(t *testing.T) {
	if out := (statusBar{width: 0, styles: newStyles()}).render(); out != "" {
		t.Fatalf("zero-width bar = %q, want empty", out)
	}
}

func TestStatusBarWarnFlagRendered(t *testing.T) {
	b := statusBar{host: "@tilde.team", flags: []string{"partial (truncated)"},
		meta: "3 users", hints: "? help", width: 80, styles: newStyles()}
	if !strings.Contains(b.render(), "partial (truncated)") {
		t.Fatalf("bar missing warn flag")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./tui/ -run TestStatusBar -count=1 -v`
Expected: FAIL — `undefined: statusBar` / `landingBar`.

- [ ] **Step 4: Implement the renderer**

Create `tui/statusbar.go`:

```go
package tui

import (
	"fmt"
	"net"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jonathandeamer/lookit/finger"
)

// statusBar is a pure description of the one-line bottom chrome. It holds no
// Bubble Tea state; appModel.View builds and renders it each frame. The
// breadcrumb's shape — "@host" alone vs "@host / user" — is the honest signal
// of directory-vs-profile, derived from the real target (no asserted "kind").
type statusBar struct {
	host      string   // "@tilde.team" ("" only on the landing screen)
	user      string   // "jonathan" ("" for a host directory)
	escTarget string   // previous history node's target.Raw ("" at the root)
	flags     []string // honesty flags, e.g. {"auto-detected"}, {"partial (truncated)"}
	meta      string   // "1.2 KB", "3 users", …
	hints     string   // contextual keys, e.g. "↵ open · / filter · ? help"
	width     int
	styles    styles
}

// landingBar is the bar shown before anything is fetched.
func landingBar(width int, st styles) statusBar {
	return statusBar{hints: "type a target and press ↵ · ? help", width: width, styles: st}
}

func (b statusBar) render() string {
	if b.width <= 0 {
		return ""
	}
	st := b.styles

	// Right group: "◂ esc: X · meta · hints", dim, truncated whole if needed.
	var right []string
	if b.escTarget != "" {
		right = append(right, "◂ esc: "+b.escTarget)
	}
	if b.meta != "" {
		right = append(right, b.meta)
	}
	if b.hints != "" {
		right = append(right, b.hints)
	}
	rightText := ansi.Truncate(strings.Join(right, " · "), b.width, "…")
	rightW := lipgloss.Width(rightText)

	// Left group: breadcrumb + flags. Flags are kept whole; the breadcrumb
	// truncates first (it is the most expendable when space is tight).
	plainFlags, styledFlags := "", ""
	for _, f := range b.flags {
		plainFlags += "  " + f
		fs := st.barFlag
		if strings.HasPrefix(f, "partial") {
			fs = st.barWarn
		}
		styledFlags += "  " + fs.Render(f)
	}

	avail := b.width - rightW - 1
	if avail < 0 {
		avail = 0
	}
	crumbBudget := avail - lipgloss.Width(plainFlags)
	if crumbBudget < 0 {
		crumbBudget = 0
	}

	left := b.styleCrumb(crumbBudget) + styledFlags
	leftW := lipgloss.Width(left)

	gap := b.width - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	line := left + st.barFill.Render(strings.Repeat(" ", gap)) + st.barDim.Render(rightText)
	return st.barFill.Width(b.width).MaxWidth(b.width).Render(line)
}

// styleCrumb renders the breadcrumb within budget: host dim + user bold when it
// fits; collapsed to a single truncated dim string when it does not (mixed
// styling can't survive a mid-run cut cleanly).
func (b statusBar) styleCrumb(budget int) string {
	st := b.styles
	full := b.host
	if b.user != "" {
		full += " / " + b.user
	}
	if lipgloss.Width(full) > budget {
		return st.barHost.Render(ansi.Truncate(full, budget, "…"))
	}
	if b.user == "" {
		return st.barHost.Render(b.host)
	}
	return st.barHost.Render(b.host) + st.barSep.Render(" / ") + st.barUser.Render(b.user)
}

// breadcrumbParts splits a target into the bar's host ("@host") and user halves.
func breadcrumbParts(t finger.Target) (host, user string) {
	h, _, err := net.SplitHostPort(t.HostPort)
	if err != nil {
		h = t.HostPort
	}
	return "@" + h, t.User
}

// formatBytes renders a byte count compactly: "512 B", "1.2 KB", "3.4 MB".
func formatBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}
```

- [ ] **Step 5: Tidy deps, run tests to verify they pass**

Run: `go mod tidy && go test ./tui/ -run TestStatusBar -count=1 -v`
Expected: PASS (all six). `go mod tidy` resolves `github.com/charmbracelet/x/ansi` from the module cache (it is a transitive dep of lipgloss v2; no network needed).

- [ ] **Step 6: Commit**

```bash
git add tui/statusbar.go tui/statusbar_test.go tui/styles.go go.mod go.sum
git commit -m "feat(tui): add pure status-bar renderer"
```

---

## Task 2: Session history stack & keymap

This replaces `fromList`/`hostList` with a real back/forward history. The UI still looks the same after this task (the bar is wired in Task 3); behavior changes are: Esc/Alt+← go back through history, Alt+→ goes forward, back/forward never re-fetch.

**Files:**
- Modify: `tui/app.go`
- Modify: `tui/app_test.go` (update tests referencing removed fields; add history tests)

- [ ] **Step 1: Write failing history tests**

Add to `tui/app_test.go`:

```go
func TestForwardBackDoNotRefetch(t *testing.T) {
	// stubFetch fails if called: proves back/forward restore from cache.
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	userT := hostTarget(t, "bob@tilde.team")

	// Land a host list, then a drilled profile — two pushes.
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	step, _ = m.Update(fetchResultMsg{entry: Entry{Target: userT, Body: []byte("Login: bob\n")}})
	m = step.(appModel)

	if len(m.history) != 2 || m.pos != 1 || m.state != stateReader {
		t.Fatalf("history=%d pos=%d state=%d, want 2/1/reader", len(m.history), m.pos, m.state)
	}

	// Back to the list (no fetch — stubFetch would fail the test).
	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt})
	m = step.(appModel)
	if m.pos != 0 || m.state != stateList {
		t.Fatalf("after back: pos=%d state=%d, want 0/list", m.pos, m.state)
	}

	// Forward to the profile again.
	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModAlt})
	m = step.(appModel)
	if m.pos != 1 || m.state != stateReader {
		t.Fatalf("after forward: pos=%d state=%d, want 1/reader", m.pos, m.state)
	}
}

func TestNewNavigationTruncatesForwardTail(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	a := hostTarget(t, "@a.example")
	b := hostTarget(t, "@b.example")
	c := hostTarget(t, "@c.example")

	for _, tg := range []finger.Target{a, b} {
		step, _ := m.Update(fetchResultMsg{entry: Entry{Target: tg, Body: []byte(hostListBody())}})
		m = step.(appModel)
	}
	// Back to a, then navigate to c — b's forward entry must be discarded.
	step, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt})
	m = step.(appModel)
	step, _ = m.Update(fetchResultMsg{entry: Entry{Target: c, Body: []byte(hostListBody())}})
	m = step.(appModel)

	if len(m.history) != 2 || m.pos != 1 {
		t.Fatalf("history=%d pos=%d, want 2/1 (forward tail truncated)", len(m.history), m.pos)
	}
	if got := m.history[1].entry.Target.Raw; got != c.Raw {
		t.Fatalf("head = %q, want %q", got, c.Raw)
	}
}

func TestAltLeftAtRootIsNoOp(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt})
	if cmd != nil && isQuit(cmd) {
		t.Fatal("Alt+← on landing must not quit")
	}
	if got := step.(appModel); got.pos != -1 {
		t.Fatalf("pos = %d, want -1 (unchanged)", got.pos)
	}
}

func TestRestorePreservesListSelection(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	// Move the list cursor down, then drill, then come back.
	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = step.(appModel)
	wantIdx := m.list.list.Index()
	step, _ = m.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t, "x@tilde.team"), Body: []byte("Login: x\n")}})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = step.(appModel)
	if m.list.list.Index() != wantIdx {
		t.Fatalf("restored list index = %d, want %d", m.list.list.Index(), wantIdx)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./tui/ -run 'TestForwardBack|TestNewNavigationTruncates|TestAltLeftAtRoot|TestRestorePreservesList' -count=1 -v`
Expected: FAIL — `m.history`/`m.pos` undefined.

- [ ] **Step 3: Replace appModel fields and add history methods**

In `tui/app.go`, change the `appModel` struct: remove `hostList`/`fromList`, add `history`/`pos`/`showingRaw`, keep `listReady`:

```go
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
```

In `newApp`, initialize `pos: -1`:

```go
	return appModel{
		common: common,
		state:  stateReader,
		reader: newReader(fetch, profile),
		pos:    -1,
	}
```

Add the history methods (anywhere in app.go):

```go
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
	m.state = n.state
	if n.state == stateReader {
		m.reader.setEntry(n.entry)
		m.reader.viewport.SetYOffset(n.scrollY)
		return
	}
	if parsed, ok := parseUserList(n.entry.Body); ok {
		incomplete := n.entry.Err != nil || n.entry.Meta.Truncated
		m.list = newListWithPreamble(m.common, n.entry.Target, parsed.users, n.entry.Body, incomplete, parsed.generic)
		m.listReady = true
		m.list.list.Select(n.listIdx)
		if n.listFltr != "" {
			m.list.list.SetFilterText(n.listFltr)
		}
	}
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
	m.showingRaw = false
	if m.pos < 0 {
		return tea.Quit
	}
	m.stepBack()
	return nil
}

// forward re-applies a previously-popped node.
func (m *appModel) forward() {
	if m.pos >= len(m.history)-1 {
		return
	}
	m.snapshot()
	m.pos++
	m.restore(m.history[m.pos])
}
```

- [ ] **Step 4: Rewrite routeFetch to push a node**

Replace `routeFetch` in `tui/app.go`:

```go
// routeFetch is the single decision point for a completed fetch: a host
// response that parses opens the list; everything else renders in the reader.
// Either way it pushes a history node.
func (m appModel) routeFetch(entry Entry) appModel {
	m.reader.loading = false
	m.showingRaw = false
	node := histNode{entry: entry, state: stateReader}
	if len(entry.Body) > 0 && shouldOpenList(entry) {
		if parsed, ok := parseUserList(entry.Body); ok {
			incomplete := entry.Err != nil || entry.Meta.Truncated
			m.list = newListWithPreamble(m.common, entry.Target, parsed.users, entry.Body, incomplete, parsed.generic)
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
```

- [ ] **Step 5: Rewire handleKey, drill, and the `r` raw view**

Replace the `handleKey` `switch m.state` body and the `drill` `fromList` line in `tui/app.go`.

`handleKey` (keep the Ctrl+C guard above it unchanged):

```go
	switch m.state {
	case stateList:
		if m.list.filtering() {
			return false, m, nil
		}
		switch {
		case key.Code == tea.KeyEsc:
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
				// In-place raw view of the cached body; no history push.
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
```

In `drill`, delete the `m.fromList = true` line (drilling now just starts a fetch; `routeFetch` pushes the node when the result lands). The final lines become:

```go
	m.reader.setLoading(target)
	m.state = stateReader
	return true, m, fetchCmd(context.Background(), m.common.fetch, target)
```

- [ ] **Step 6: Update existing tests that referenced removed fields**

Apply these exact edits in `tui/app_test.go`:

1. `TestHostFetchThatParsesOpensList` — replace the `hostList` assertion:

```go
	if len(got.history) != 1 || got.pos != 0 || got.history[0].state != stateList {
		t.Fatalf("history=%d pos=%d, want one list node", len(got.history), got.pos)
	}
```

2. `TestEscInDrilledReaderRestoresList` — rebuild via the history stack:

```go
func TestEscInDrilledReaderRestoresList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	user := hostTarget(t, "bob@tilde.team")
	m.history = []histNode{
		{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList},
		{entry: Entry{Target: user, Body: []byte("Login: bob\n")}, state: stateReader},
	}
	m.pos = 1
	m.state = stateReader

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)

	if got.state != stateList || got.pos != 0 {
		t.Fatalf("state=%d pos=%d, want list/0 after Esc", got.state, got.pos)
	}
}
```

3. `TestEscInListReturnsToReaderHome` — a single list node backs out to the landing:

```go
func TestEscInListReturnsToReaderHome(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	m.history = []histNode{{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList}}
	m.pos = 0
	m.state = stateList
	users, _ := ParseUsers([]byte(hostListBody()))
	m.list = newList(m.common, host, users)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)

	if got.state != stateReader || got.pos != -1 {
		t.Fatalf("state=%d pos=%d, want reader/-1 (landing)", got.state, got.pos)
	}
	if cmd != nil && isQuit(cmd) {
		t.Fatal("Esc in list must not quit while history is non-empty")
	}
}
```

4. `TestEscWhileFilteringDelegatesToList` — drop the `m.hostList` line; replace the three setup lines after `users, _ := ...` with:

```go
	m.history = []histNode{{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList}}
	m.pos = 0
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.state = stateList
```

5. `TestWindowSizePropagatesToBothSubModels` — replace the `m.hostList = ...` line with `m.listReady = true` (keep the existing `m.listReady = true` if already present; ensure no `hostList` reference remains).

6. `drillFirstUser` helper — replace its body’s setup and assertion:

```go
func drillFirstUser(t *testing.T, host finger.Target, users []User, fetch FetchFunc) tea.Cmd {
	t.Helper()
	m := newApp(fetch, colorprofile.NoTTY)
	m.history = []histNode{{entry: Entry{Target: host}, state: stateList}}
	m.pos = 0
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.state = stateList
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if got.state != stateReader {
		t.Fatalf("expected a drilled reader (state=%d)", got.state)
	}
	if cmd == nil {
		t.Fatal("expected a fetch command from drilling")
	}
	return cmd
}
```

7. `TestRViewsRawBodyOnGenericList` — replace the `fromList` assertion with `showingRaw`:

```go
	if !got.showingRaw {
		t.Fatal("showingRaw = false, want true after viewing raw")
	}
```

- [ ] **Step 7: Run the whole tui suite**

Run: `go test ./tui/ -count=1`
Expected: PASS. (Title-based tests still pass — `newListWithPreamble` still sets `list.Title`; the bar isn't wired yet.)

- [ ] **Step 8: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): session back/forward history replacing fromList"
```

---

## Task 3: Compose the bar into the view; shed reader/list chrome

Now the bar appears and the sub-models lose their own status/title/hint chrome.

**Files:**
- Modify: `tui/app.go` (View, sizing, `statusBarModel`)
- Modify: `tui/reader.go` (View, chromeRows)
- Modify: `tui/list.go` (hide built-in title/help; `bodyHeight`)
- Modify: `tui/reader_test.go`, `tui/app_test.go`

- [ ] **Step 1: Write failing composition tests**

Add to `tui/app_test.go`:

```go
func TestViewIncludesBreadcrumbBar(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	host := hostTarget(t, "@tilde.team")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)

	view := m.View().Content
	if !strings.Contains(view, "@tilde.team") {
		t.Fatalf("view missing breadcrumb host:\n%s", view)
	}
	if !strings.Contains(view, "? help") {
		t.Fatalf("view missing help hint:\n%s", view)
	}
}

func TestLandingViewShowsLandingBar(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	m.reader.setSize(80, 23)
	if !strings.Contains(m.View().Content, "type a target") {
		t.Fatalf("landing view missing landing hint:\n%s", m.View().Content)
	}
}
```

Add to `tui/reader_test.go`:

```go
func TestReaderViewNoLongerRendersStatusLine(t *testing.T) {
	m := newReader(stubFetch(t), colorprofile.NoTTY)
	m.setSize(80, 20)
	m.status = "loaded something"
	// The status line moved to appModel's bar; the reader must not draw it.
	if strings.Contains(m.View(), "loaded something") {
		t.Fatalf("reader View should no longer render its status line:\n%s", m.View())
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./tui/ -run 'TestViewIncludesBreadcrumbBar|TestLandingViewShows|TestReaderViewNoLonger' -count=1 -v`
Expected: FAIL (status line still present / bar absent).

- [ ] **Step 3: Add `statusBarModel` and compose it in `View`**

In `tui/app.go` add:

```go
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
		// (Task 4 adds the partial-truncated / partial-error flags here.)
	default: // stateReader
		bar.meta = formatBytes(len(node.entry.Body))
		bar.hints = "↑↓ scroll · esc back · ? help"
	}
	return bar
}
```

Add the `"fmt"` import to `app.go` if not present.

Replace `View`:

```go
func (m appModel) View() tea.View {
	var content string
	switch m.state {
	case stateList:
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
```

- [ ] **Step 4: Reserve one row for the bar when sizing**

Add a `bodyHeight` helper to `commonModel` in `tui/app.go`:

```go
// bodyHeight is the height available to a sub-model after reserving one row
// for the bottom status bar.
func (c *commonModel) bodyHeight() int {
	if c.height > 1 {
		return c.height - 1
	}
	return 1
}
```

Update the `WindowSizeMsg` case in `Update` to size sub-models with the reduced height:

```go
	case tea.WindowSizeMsg:
		m.common.width = msg.Width
		m.common.height = msg.Height
		m.reader.setSize(msg.Width, m.common.bodyHeight())
		if m.listReady {
			m.list.setSize(msg.Width, m.common.bodyHeight())
		}
		return m, nil
```

- [ ] **Step 5: Shed the reader's status & hint lines**

In `tui/reader.go`, change `chromeRows` and `View`:

```go
// chromeRows counts the non-viewport lines in the reader view: title + input.
// (The status and hint lines moved to appModel's bottom bar.)
const chromeRows = 2
```

```go
func (m readerModel) View() string {
	var b strings.Builder
	b.WriteString(m.styles.title.Render("lookit"))
	b.WriteByte('\n')
	b.WriteString(m.input.View())
	b.WriteByte('\n')
	b.WriteString(m.viewport.View())
	return b.String()
}
```

(`setSize` already subtracts `chromeRows`; with the bar reserved by `appModel`, the math is correct. Leave `setSize`, `status`, `setEntry`, etc. as-is — `status` is still set, just no longer drawn here.)

- [ ] **Step 6: Hide the list's built-in title & help**

In `tui/list.go`, in `newList`, after `l.SetShowStatusBar(false)` add:

```go
	l.SetShowTitle(false)
	l.SetShowHelp(false)
```

Change `listChromeRows` (the bar is reserved by `appModel`, and title/help are now hidden):

```go
// listChromeRows reserves space for list internals after title/help are hidden.
const listChromeRows = 1
```

In `newList`, size from `bodyHeight` is unnecessary (appModel passes reduced height via setSize), but `newList` computes its own initial height from `common.height`; change that line to use `common.bodyHeight()`:

```go
	height := common.bodyHeight() - listChromeRows
```

And in `newListWithPreamble`, the final `m.setSize(common.width, common.height)` becomes:

```go
	m.setSize(common.width, common.bodyHeight())
```

(`list.Title` is still set in `newListWithPreamble` for now; Task 4 removes the flag copy from it. The hidden title carries no visual weight but keeps the existing `Title`-substring tests green until Task 4 rewrites them.)

- [ ] **Step 7: Run the whole suite**

Run: `make check`
Expected: all gates green. If the reader/list viewport height assertions in existing tests fail, confirm `bodyHeight` is used consistently and `chromeRows`/`listChromeRows` match the new layout.

- [ ] **Step 8: Commit**

```bash
git add tui/app.go tui/reader.go tui/list.go tui/app_test.go tui/reader_test.go
git commit -m "feat(tui): compose status bar; move chrome out of reader/list"
```

---

## Task 4: Honest flag copy

Replace `(best guess)` / `(incomplete)` with the consequence-first copy, split by cause, tiered between the bar (terse) and the preamble note (full sentence).

**Files:**
- Modify: `tui/list.go` (preamble note copy; drop flag text from `Title`)
- Modify: `tui/app.go` (`statusBarModel` list flags)
- Modify: `tui/app_test.go` (the four Title-substring tests → assert bar flags)

- [ ] **Step 1: Write failing flag tests**

In `tui/app_test.go`, replace the bodies of the existing flag tests to assert on the composed view's bar. Replace `TestTruncatedHostFetchMarksListIncomplete`, `TestErroredHostFetchWithBodyMarksListIncomplete`, `TestCompleteHostFetchListNotMarkedIncomplete`, and `TestGenericHostFetchOpensFlaggedList`:

```go
func barFor(t *testing.T, entry Entry) string {
	t.Helper()
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 100, 24
	step, _ := m.Update(fetchResultMsg{entry: entry})
	return step.(appModel).statusBarModel().render()
}

func TestTruncatedHostFetchMarksListIncomplete(t *testing.T) {
	host := hostTarget(t, "@tilde.team")
	bar := barFor(t, Entry{Target: host, Body: []byte(hostListBody()),
		Meta: finger.Meta{Addr: host.HostPort, Truncated: true}})
	if !strings.Contains(bar, "partial (truncated)") {
		t.Fatalf("bar = %q, want partial (truncated)", bar)
	}
}

func TestErroredHostFetchWithBodyMarksListIncomplete(t *testing.T) {
	host := hostTarget(t, "@tilde.team")
	bar := barFor(t, Entry{Target: host, Body: []byte(hostListBody()),
		Meta: finger.Meta{Addr: host.HostPort}, Err: errors.New("connection reset")})
	if !strings.Contains(bar, "partial (error)") {
		t.Fatalf("bar = %q, want partial (error)", bar)
	}
}

func TestCompleteHostFetchListNotMarkedIncomplete(t *testing.T) {
	host := hostTarget(t, "@tilde.team")
	bar := barFor(t, Entry{Target: host, Body: []byte(hostListBody()), Meta: finger.Meta{Addr: host.HostPort}})
	if strings.Contains(bar, "partial") {
		t.Fatalf("bar = %q, should not flag partial", bar)
	}
}

func TestGenericHostFetchOpensFlaggedList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 100, 24
	target := hostTarget(t, "@unknown.host")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: target, Body: []byte(genericListBody()), Meta: finger.Meta{Addr: target.HostPort}}})
	got := step.(appModel)
	if got.state != stateList || !got.list.generic {
		t.Fatalf("state=%d generic=%v, want list/true", got.state, got.list.generic)
	}
	if !strings.Contains(got.statusBarModel().render(), "auto-detected") {
		t.Fatalf("bar missing auto-detected flag")
	}
}
```

Also update `TestGenericListShowsRawHintInPreamble`-style expectations if present: search `tui/` for the old preamble string `"Auto-detected from an unrecognized response"` and the `"(best guess)"`/`"(incomplete)"` literals and update any remaining assertions to the new copy below.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./tui/ -run 'MarksListIncomplete|NotMarkedIncomplete|OpensFlaggedList' -count=1 -v`
Expected: FAIL (bar shows no `partial`/`auto-detected` flags yet; list flags branch is a Task-3 stub).

- [ ] **Step 3: Add the list flags in `statusBarModel`**

In `tui/app.go`, extend the `case stateList:` branch of `statusBarModel` (replace the Task-3 stub comment):

```go
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
```

- [ ] **Step 4: Remove flag text from the list Title; reword the preamble note**

In `tui/list.go` `newListWithPreamble`, delete the two blocks that append `" (best guess)"` and `" (incomplete)"` to `m.list.Title` (the Title is hidden now; flags live in the bar). Keep `m.list.Title` set to the plain `host.Raw — N users` from `newList`.

Reword the generic preamble note (the `note := ...` line) to:

```go
		note := "Auto-detected user list from an unrecognized response — press r to view raw."
```

- [ ] **Step 5: Run the suite**

Run: `make check`
Expected: green. Grep to confirm no stale copy remains: `git grep -n "(best guess)\|(incomplete)\|Auto-detected from an unrecognized"` should return nothing in `tui/` (test or source).

- [ ] **Step 6: Commit**

```bash
git add tui/app.go tui/list.go tui/app_test.go
git commit -m "feat(tui): consequence-first honesty flags in the status bar"
```

---

## Task 5: Help overlay

`?` toggles a full-screen help overlay listing the keymap; any key closes it. It must not open while a list filter is being typed.

**Files:**
- Modify: `tui/app.go` (help toggle in `handleKey`; overlay in `View`)
- Modify: `tui/app_test.go`

- [ ] **Step 1: Write failing help tests**

Add to `tui/app_test.go`:

```go
func TestQuestionMarkTogglesHelpOverlay(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	host := hostTarget(t, "@tilde.team")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)

	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = step.(appModel)
	if !m.help {
		t.Fatal("help should be open after '?'")
	}
	if !strings.Contains(m.View().Content, "Alt+←") {
		t.Fatalf("help overlay missing keymap:\n%s", m.View().Content)
	}

	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if step.(appModel).help {
		t.Fatal("any key should close the help overlay")
	}
}

func TestQuestionMarkWhileFilteringDoesNotOpenHelp(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	users, _ := ParseUsers([]byte(hostListBody()))
	m.history = []histNode{{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList}}
	m.pos = 0
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.state = stateList

	step, _ := m.Update(tea.KeyPressMsg{Code: '/'})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	if step.(appModel).help {
		t.Fatal("'?' must be a literal filter character while filtering, not open help")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./tui/ -run 'TestQuestionMark' -count=1 -v`
Expected: FAIL — `m.help` never set.

- [ ] **Step 3: Handle the toggle and render the overlay**

In `tui/app.go` `handleKey`, immediately after the Ctrl+C guard (and before the `switch m.state`), add the toggle. Closing on any key takes priority when the overlay is open:

```go
	// Help overlay: any key closes it; '?' opens it (except while filtering).
	if m.help {
		m.help = false
		return true, m, nil
	}
	if key.Code == '?' && !(m.state == stateList && m.list.filtering()) {
		m.help = true
		return true, m, nil
	}
```

Add the overlay renderer:

```go
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
```

In `View`, short-circuit to the overlay (still append the bar so layout is stable):

```go
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
```

- [ ] **Step 4: Run help tests, then the full gate**

Run: `go test ./tui/ -run 'TestQuestionMark' -count=1 -v` → PASS
Run: `make check` → all gates green.

- [ ] **Step 5: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): add ? help overlay"
```

---

## Final verification

- [ ] Run `make check` once more — all four gates (`go vet`, `gofmt -l`, `golangci-lint`, `go test -race`) green.
- [ ] `git grep -n "fromList\|hostList" tui/` returns nothing (the old back machinery is fully removed).
- [ ] `git grep -n "(best guess)\|(incomplete)" tui/` returns nothing.
- [ ] Manually build and smoke-test in a real terminal (the TUI can't be tested headlessly): `make build && ./lookit`, then fetch `@tilde.team`, drill a user, confirm the bar breadcrumb, `Esc`/`Alt+←` back, `Alt+→` forward, and `?` help.

## Notes for the executor

- **One package, strict layering:** all edits are in `tui/`; never modify `finger/` or `render/`.
- **Commit messages:** Conventional Commits, and **no `Co-Authored-By` trailer** (repo rule — a `commit-msg` hook enforces the format).
- **TUI can't be smoke-tested headlessly** — rely on the unit tests for behavior; the final manual step is a human check.
- The breadcrumb width-truncation in `statusBar.render` uses `ansi.Truncate`, which is display-width aware; exact multi-width-rune handling is approximate and acceptable for v1 (see spec "Accepted residual scope").

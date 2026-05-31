# Charm-idiomatic blur-by-default navigation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace lookit's always-focused-input + `Alt` history chord with a Charm-idiomatic blur-by-default model: content owns the keyboard, the target input is summoned with `i`, history back is `Esc` (no forward), plus `bubbles/help`, `bubbles/spinner`, scroll-%/page indicators, `y`-to-copy, the default list delegate, and no mouse capture.

**Architecture:** The target input migrates out of `readerModel` into `appModel` as top chrome with an `inputFocused` flag; a `key.Binding` keyMap drives both routing (`key.Matches`) and a `bubbles/help` bottom panel; the reader shrinks to a viewport. The history stack, status-bar renderer, drill+pin, and honesty flags are reused unchanged except that `forward()`/`Alt` are removed.

**Tech Stack:** Go, Bubble Tea v2 (`charm.land/bubbletea/v2`), `charm.land/bubbles/v2` (`textinput`, `viewport`, `list`, `help`, `spinner`, `key`, `paginator`), `charm.land/lipgloss/v2`.

---

## Background for the implementer

Read `docs/superpowers/specs/2026-05-30-tui-idiomatic-navigation-design.md` first — it is the source of truth. All work is in `tui/`; never touch `finger/` or `render/`.

Current shape (post-merge `main`):
- `appModel` (app.go) owns `common`, `state` (`stateReader`/`stateList`), `reader`, `list`, a history stack (`history []histNode`, `pos`, `showingRaw`, `help bool`, `listReady`), and the methods `push/snapshot/restore/gotoLanding/stepBack/back/forward`, `handleKey`, `drill`, `routeFetch`, `statusBarModel`, `helpView`, `View`.
- `readerModel` (reader.go) **currently owns the `textinput`** (always focused), the `viewport`, `current *Entry`, `loading`, `status`, plus `update` (Enter→fetch) and `View` (title+input+viewport). `const chromeRows = 2`.
- `listModel` (list.go) uses a hand-rolled `userDelegate`; `userItem{login,name,target}` implements only `FilterValue()`.
- `statusBar` (statusbar.go) is a pure renderer: fields `host,user,escTarget string; flags []string; meta,hints string; width int; styles styles`; method `render()`; helpers `landingBar`, `breadcrumbParts`, `formatBytes`.

Verified v2 APIs you will use:
- `textinput.Model`: `Focus() tea.Cmd`, `Blur()`, `Focused() bool`, `Value()`, `SetValue(string)`, `CursorEnd()`.
- `help`: `help.New() help.Model`; `m.View(k help.KeyMap) string`; `m.ShowAll bool`; `help.KeyMap` interface = `ShortHelp() []key.Binding` + `FullHelp() [][]key.Binding`.
- `spinner`: `spinner.New(opts ...spinner.Option) spinner.Model`; `m.Update(msg) (Model, tea.Cmd)`; `m.View() string`; `m.Tick` is a `tea.Cmd`; `spinner.TickMsg` is the tick message.
- `key`: `key.NewBinding(key.WithKeys(...), key.WithHelp(k,desc))`; `key.Matches(msg, bindings...) bool` (msg is `tea.KeyPressMsg`).
- `list.Model`: exported field `Paginator paginator.Model` with `Page int`, `TotalPages int`; `list.NewDefaultDelegate() list.DefaultDelegate`; `list.DefaultItem` = `Item` + `Title() string` + `Description() string`; `DefaultDelegate.Styles` is a `list.DefaultItemStyles`.
- `tea.SetClipboard(s string) tea.Cmd` (writes the system clipboard via OSC 52).

Test conventions: `newApp(stubFetch(t), colorprofile.NoTTY)`; `stubFetch(t)` fails if called; `fetchRecorder(body)`/`captureFetch` record targets; keys are `tea.KeyPressMsg{Code: ..., Mod: ...}` (a rune like `'i'` for letters); `isQuit(cmd)` reports `tea.QuitMsg`. Single test: `go test ./tui/ -run Name -count=1 -v`. Gate: `make check` (run `make fmt` if gofmt complains). Commits: Conventional Commits, **no `Co-Authored-By` trailer**.

## File structure

- **Create** `tui/keys.go` — the `keyMap` (`key.Binding`s) + `help.KeyMap` impl.
- **Modify** `tui/app.go` — input ownership, `inputFocused`, focus routing via `key.Matches`, centralized submit, spinner, help model, flash, scroll-%/page wiring, drop mouse mode, `y`-copy, state-driven `updateKeymap()` + input-focused bar hint (Task 8); remove `forward()`/`Alt`/`helpView` string.
- **Modify** `tui/reader.go` — drop the input (and `status`/`loading`/`update`-fetch); reader becomes viewport-only.
- **Modify** `tui/list.go` — default delegate; `userItem` gains `Title()/Description()`.
- **Modify** `tui/statusbar.go` — add `scroll`/`page` segments.
- **Modify** `tui/styles.go` — theme the default delegate; (bar/title styles unchanged).
- **Modify** tests in `tui/app_test.go`, `tui/reader_test.go`, `tui/list_test.go`.

---

## Task 1: key.Binding keymap (foundation)

**Files:**
- Create: `tui/keys.go`
- Create test: `tui/keys_test.go`

- [ ] **Step 1: Write the failing test** — `tui/keys_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
)

func TestKeyMapBindings(t *testing.T) {
	k := newKeyMap()
	// Sanity: the keys we rely on are bound to the expected runes.
	cases := map[string]key.Binding{
		"i":   k.FocusInput,
		"y":   k.Copy,
		"r":   k.Raw,
		"q":   k.Quit,
		"?":   k.Help,
		"esc": k.Back,
	}
	for want, b := range cases {
		if got := b.Keys(); len(got) == 0 || !contains(got, want) {
			t.Fatalf("binding %v keys = %v, want to contain %q", b.Help(), got, want)
		}
	}
}

func TestKeyMapFullHelpIncludesPageAndMoveKeys(t *testing.T) {
	k := newKeyMap()
	var all []string
	for _, group := range k.FullHelp() {
		for _, b := range group {
			all = append(all, strings.Join(b.Keys(), ","))
		}
	}
	joined := strings.Join(all, " ")
	for _, want := range []string{"i", "y", "esc", "?", "q"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("FullHelp missing %q; got %s", want, joined)
		}
	}
	// Page/move discoverability (owed because we disable the list's own help).
	if !strings.Contains(joined, "left") || !strings.Contains(joined, "g") {
		t.Fatalf("FullHelp must advertise page/move keys; got %s", joined)
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run to verify it fails** — `go test ./tui/ -run TestKeyMap -count=1 -v` → FAIL (`newKeyMap` undefined).

- [ ] **Step 3: Implement** — `tui/keys.go`:

```go
package tui

import "charm.land/bubbles/v2/key"

// keyMap holds lookit's app-level key bindings. It is matched with key.Matches
// and doubles as the help.KeyMap that drives the bubbles/help panel. Scroll,
// page, and jump bindings are owned by the bubbles viewport/list at runtime;
// they appear here only so the help panel advertises them (we disable the
// list's built-in help).
type keyMap struct {
	FocusInput key.Binding
	Back       key.Binding
	Open       key.Binding
	Filter     key.Binding
	Raw        key.Binding
	Copy       key.Binding
	Help       key.Binding
	Quit       key.Binding
	ForceQuit  key.Binding

	// display-only (handled by sub-models)
	Move key.Binding
	Page key.Binding
	Jump key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		FocusInput: key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "target")),
		Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Open:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "open")),
		Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Raw:        key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "raw")),
		Copy:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:       key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		ForceQuit:  key.NewBinding(key.WithKeys("ctrl+c")),
		Move:       key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓", "move")),
		Page:       key.NewBinding(key.WithKeys("left", "right", "h", "l", "pgup", "pgdown"), key.WithHelp("←/→", "page")),
		Jump:       key.NewBinding(key.WithKeys("g", "G"), key.WithHelp("g/G", "top/bottom")),
	}
}

// ShortHelp implements help.KeyMap. (Unused by the bar today, which renders its
// own focus-aware hints, but required by the interface.)
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.FocusInput, k.Back, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap — the expanded panel toggled by '?'.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Open, k.FocusInput, k.Copy, k.Raw},
		{k.Move, k.Page, k.Jump, k.Filter},
		{k.Back, k.Help, k.Quit},
	}
}
```

- [ ] **Step 4: Run to verify it passes** — `go test ./tui/ -run TestKeyMap -count=1 -v` → PASS. Then `make check` → green.

- [ ] **Step 5: Commit**

```bash
git add tui/keys.go tui/keys_test.go
git commit -m "feat(tui): add key.Binding keymap and help.KeyMap"
```

---

## Task 2: Input → appModel, blur-by-default focus, routing (core)

The big structural task. After it: content owns the keyboard; `i` focuses the input; `Esc` backs (history) / `q`/`Ctrl+C` quit; typing only reaches the input when focused; `forward()` and `Alt` are gone; the reader is viewport-only; submit is centralized. The help is still the existing full-screen view (its text updated to the new keys); Task 3 swaps the mechanism. The spinner is a plain `loading <target>` string for now (Task 4 adds the animation).

**Files:**
- Modify: `tui/app.go`, `tui/reader.go`
- Modify: `tui/app_test.go`, `tui/reader_test.go`

- [ ] **Step 1: Write failing focus/routing tests** — add to `tui/app_test.go`:

```go
func TestLandingFocusesInput(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	if !m.inputFocused {
		t.Fatal("landing should focus the input")
	}
}

func TestIFocusesInputFromContent(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	if m.inputFocused {
		t.Fatal("after a fetch, content should have focus")
	}
	step, _ = m.Update(tea.KeyPressMsg{Code: 'i'})
	m = step.(appModel)
	if !m.inputFocused {
		t.Fatal("'i' should focus the input")
	}
}

func TestTypingReachesInputOnlyWhenFocused(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY) // landing: input focused
	step, _ := m.Update(tea.KeyPressMsg{Code: 'b'})
	step, _ = step.(appModel).Update(tea.KeyPressMsg{Code: 'o'})
	m = step.(appModel)
	if m.input.Value() != "bo" {
		t.Fatalf("input value = %q, want \"bo\"", m.input.Value())
	}
}

func TestSubmitFetchesParsedTargetAndBlurs(t *testing.T) {
	fetch, seen := fetchRecorder("Plan: hi\n")
	m := newApp(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@plan.cat")
	step, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = step.(appModel)
	if m.inputFocused {
		t.Fatal("submit should blur the input to content")
	}
	if cmd == nil {
		t.Fatal("submit should return a fetch command")
	}
	cmd()
	if len(*seen) != 1 || (*seen)[0] != "alice@plan.cat" {
		t.Fatalf("fetched %v, want [alice@plan.cat]", *seen)
	}
}

func TestQQuitsFromContent(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	if cmd == nil || !isQuit(cmd) {
		t.Fatal("'q' should quit from content")
	}
}

func TestQIsLiteralWhenInputFocused(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY) // input focused
	step, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	m = step.(appModel)
	if cmd != nil && isQuit(cmd) {
		t.Fatal("'q' must be literal while the input is focused")
	}
	if m.input.Value() != "q" {
		t.Fatalf("input value = %q, want \"q\"", m.input.Value())
	}
}

func TestEscFromInputBlursToContentThenQuitsAtLanding(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY) // landing, input focused, pos -1
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil || !isQuit(cmd) {
		t.Fatal("Esc from the bare landing input should quit")
	}

	// With content present, Esc from the input blurs (does not quit).
	m2 := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	step, _ := m2.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m2 = step.(appModel)
	step, _ = m2.Update(tea.KeyPressMsg{Code: 'i'}) // focus input
	m2 = step.(appModel)
	step, cmd2 := m2.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m2 = step.(appModel)
	if cmd2 != nil && isQuit(cmd2) {
		t.Fatal("Esc from input with content present must not quit")
	}
	if m2.inputFocused {
		t.Fatal("Esc from input should blur to content")
	}
}

func TestAltArrowsNoLongerNavigate(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	a := hostTarget(t, "@a.example")
	b := hostTarget(t, "@b.example")
	for _, tg := range []finger.Target{a, b} {
		step, _ := m.Update(fetchResultMsg{entry: Entry{Target: tg, Body: []byte(hostListBody())}})
		m = step.(appModel)
	}
	// Alt+Left used to go back; now it's inert (content key, delegated, no-op for the list).
	step, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt})
	if step.(appModel).pos != 1 {
		t.Fatalf("pos = %d, want 1 (Alt+Left must not navigate)", step.(appModel).pos)
	}
}
```

- [ ] **Step 2: Run to verify failure** — `go test ./tui/ -run 'TestLandingFocuses|TestIFocuses|TestTypingReaches|TestSubmitFetches|TestQQuits|TestQIsLiteral|TestEscFromInput|TestAltArrowsNoLonger' -count=1 -v` → FAIL (fields/behaviour absent).

- [ ] **Step 3: Shrink `readerModel` to a viewport** — replace `tui/reader.go` entirely:

```go
package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/render"
)

// chromeRows counts the reader's own non-viewport lines. The reader is now
// viewport-only; the input and status moved to appModel (top bar / status bar).
const chromeRows = 0

// readerModel shows one rendered finger response in a scrollable viewport. It
// owns scrolling only; appModel owns the input, fetch, quit, and chrome.
type readerModel struct {
	viewport viewport.Model
	current  *Entry
	profile  colorprofile.Profile
	styles   styles
	width    int
	height   int
}

func newReader(_ FetchFunc, profile colorprofile.Profile) readerModel {
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(10))
	vp.SetContent("No response yet.")
	return readerModel{viewport: vp, profile: profile, styles: newStyles()}
}

// Init is a no-op (the input's blink command now lives in appModel.Init).
func (m readerModel) Init() tea.Cmd { return nil }

// update forwards scroll messages to the viewport.
func (m readerModel) update(msg tea.Msg) (readerModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders just the viewport.
func (m readerModel) View() string { return m.viewport.View() }

func (m *readerModel) setSize(width, height int) {
	m.width = width
	m.height = height
	if width <= 0 || height <= 0 {
		return
	}
	m.viewport.SetWidth(width)
	vh := height - chromeRows
	if vh < 1 {
		vh = 1
	}
	m.viewport.SetHeight(vh)
}

func (m *readerModel) setProfile(p colorprofile.Profile) {
	m.profile = p
	if m.current != nil {
		m.viewport.SetContent(renderEntry(m.profile, *m.current))
	}
}

// setEntry displays a fetched result.
func (m *readerModel) setEntry(entry Entry) {
	m.current = &entry
	m.viewport.SetContent(renderEntry(m.profile, entry))
}

func renderEntry(profile colorprofile.Profile, entry Entry) string {
	return render.Render(entry.Target, entry.Body, entry.Meta, entry.Err, profile)
}
```

(reader.go no longer imports `finger` — it only passes `entry.Target` through to `render.Render`.)

- [ ] **Step 4: Add input ownership + focus state to `appModel`** — in `tui/app.go`, add imports `"charm.land/bubbles/v2/textinput"` and (already present) `"charm.land/bubbles/v2/key"` is new; add it. Extend the struct and constructor:

```go
type appModel struct {
	common *commonModel
	state  appState
	reader readerModel
	list   listModel

	input       textinput.Model
	inputFocused bool
	keys        keyMap

	loading       bool
	loadingTarget finger.Target

	history    []histNode
	pos        int
	showingRaw bool
	help       bool
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
	return appModel{
		common:       common,
		state:        stateReader,
		reader:       newReader(fetch, profile),
		input:        in,
		inputFocused: true,
		keys:         newKeyMap(),
		pos:          -1,
	}
}
```

`Init` returns the input blink (the reader no longer blinks):

```go
func (m appModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		tea.RequestCapability("RGB"),
		tea.RequestCapability("Tc"),
	)
}
```

- [ ] **Step 5: Add submit + focus helpers** — add to `tui/app.go`:

```go
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

// submit parses the input and starts a fetch, blurring to content. On a parse
// error it keeps the input focused and flashes the error.
func (m *appModel) submit() tea.Cmd {
	target, err := finger.ParseTarget(strings.TrimSpace(m.input.Value()))
	if err != nil {
		m.flash = "error: " + err.Error()
		return nil
	}
	m.blurInput()
	m.loading = true
	m.loadingTarget = target
	return fetchCmd(context.Background(), m.common.fetch, target)
}
```

Add a `flash string` field to `appModel` (used here and fully wired in Task 6):

```go
	flash string
```

- [ ] **Step 6: Rewrite `handleKey` for focus routing** — replace the whole `handleKey` method in `tui/app.go`:

```go
// handleKey processes app-level keys and focus routing. handled=false lets the
// caller delegate the key to the active sub-model (content) or the input.
func (m appModel) handleKey(msg tea.KeyPressMsg) (bool, appModel, tea.Cmd) {
	if key.Matches(msg, m.keys.ForceQuit) {
		return true, m, tea.Quit
	}

	// Help overlay: any key closes it (Task 3 swaps the rendering).
	if m.help {
		m.help = false
		return true, m, nil
	}

	// Input focused: only Enter/Esc are commands; everything else types.
	if m.inputFocused {
		switch {
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
			m.showingRaw = false
			m.state = stateList
			return true, m, nil
		}
		cmd := m.back()
		return true, m, cmd
	case key.Matches(msg, m.keys.Open) && m.state == stateList:
		return m.drill()
	case key.Matches(msg, m.keys.Raw) && m.state == stateList:
		if m.list.generic && m.pos >= 0 {
			m.reader.setEntry(m.history[m.pos].entry)
			m.state = stateReader
			m.showingRaw = true
			return true, m, nil
		}
	}
	return false, m, nil
}
```

- [ ] **Step 7: Route the fall-through in `Update`** — change the delegation tail of `Update` so unhandled keys go to the input when focused, else to the content sub-model. Replace the bottom delegate block:

```go
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
```

- [ ] **Step 8: Drop `forward()`, the `Alt` cases, the old reader fetch, and update `drill`/`gotoLanding`** —
  - Delete the `forward` method.
  - In `drill`, replace `m.reader.setLoading(target)` with the centralized loading state:
    ```go
    m.loading = true
    m.loadingTarget = target
    m.state = stateReader
    return true, m, fetchCmd(context.Background(), m.common.fetch, target)
    ```
  - In `routeFetch`, at the top (alongside `m.showingRaw = false`): set `m.loading = false`, and **`m.inputFocused = false; m.input.Blur()`** — a landed result always shows content, so focus returns to it (this is what makes `Enter`/`Esc`/`q` behave as content keys after any fetch, in real use and in tests). Delete the existing `m.reader.loading = false` line (the field is gone).
  - In `gotoLanding`, delete `m.reader.loading = false` (field gone). Also **refocus the input** (the landing is the input-focused state): set `m.inputFocused = true`, `m.input.SetValue("")`, and call `m.input.Focus()` (discard the returned blink cmd — the cursor still shows; only the blink animation is skipped on this path). Keep the `m.reader.current = nil` / `"No response yet."` lines.
  - `snapshot`/`restore` reference `m.reader.viewport` — unchanged (viewport still there).

- [ ] **Step 9: Update `helpView` text and `View`** — update `helpView` to the new keymap (no Alt/forward; add i/y/q) and make the input the top row of `View`:

```go
func (m appModel) helpView() string {
	st := newStyles()
	lines := []string{
		st.title.Render("lookit — keys"),
		"",
		"  i            focus the target input      ↵   open / fetch",
		"  esc          back (quit at the top)      q   quit",
		"  ←/→  h/l     page        ↑/↓  j/k  move  g/G  top/bottom",
		"  /            filter a list      r   raw view (auto-detected lists)",
		"  y            copy address       ?   toggle this help   ctrl+c quit",
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
	full := m.input.View() + "\n" + content + "\n" + m.statusBarModel().render()

	v := tea.NewView(full)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion // dropped in Task 6
	return v
}
```

- [ ] **Step 10: Reserve the input row when sizing** — the top input now takes a row in addition to the bar. Change `commonModel.bodyHeight` to reserve **two** rows:

```go
// bodyHeight is the height available to a sub-model after reserving the top
// input row and the bottom status-bar row.
func (c *commonModel) bodyHeight() int {
	if c.height > 2 {
		return c.height - 2
	}
	return 1
}
```

Also, in `Update`'s `WindowSizeMsg` case, size the input width: after setting `m.common.width/height`, add `m.input.SetWidth(msg.Width - len(m.input.Prompt))` (guard `>= 20`). Concretely:

```go
	case tea.WindowSizeMsg:
		m.common.width = msg.Width
		m.common.height = msg.Height
		iw := msg.Width - len(m.input.Prompt)
		if iw < 20 {
			iw = 20
		}
		m.input.SetWidth(iw)
		m.reader.setSize(msg.Width, m.common.bodyHeight())
		if m.listReady {
			m.list.setSize(msg.Width, m.common.bodyHeight())
		}
		return m, nil
```

- [ ] **Step 11: Migrate the existing tests** — three groups:

  **(a) reader_test.go** — the input/fetch tests now belong to `appModel`. Delete `TestNewReaderInitialState`, `TestReaderInvalidEnterSetsStatusError`, `TestReaderValidEnterStartsFetch`, `TestReaderDuplicateEnterWhileLoadingDoesNotFetch` (covered by the new app-level submit/focus tests). `stubFetch(t)` lives here — keep it (it's still referenced widely). Update `TestReaderSetSize` if present: the reader is viewport-only now (`chromeRows == 0`), so the viewport height equals the height you pass; assert that. Remove any reference to `m.input`/`m.status`/`m.loading` on the reader (gone).

  **(b) Manually-constructed content-state tests in app_test.go** — several tests build list/reader state directly (not via a fetch) and then press a key. Because `newApp` now starts with the input focused, they must set **`m.inputFocused = false`** (content focused) right after constructing state, or `Enter`/`Esc`/`q`/`r` would be treated as input keys. Add `m.inputFocused = false` to the setup of: `TestEnterInListDrillsIntoUser`, `TestMenuListKeepsPreambleAndDrillsIntoExplicitTarget`, `TestEscInDrilledReaderRestoresList`, `TestEscInListReturnsToReaderHome`, `TestEscWhileFilteringDelegatesToList`, and the `drillFirstUser` helper. (Tests that drive state via `m.Update(fetchResultMsg{...})` — e.g. `TestRViewsRawBodyOnGenericList`, `TestRInertOnRecognizedList` — need no change, because `routeFetch` now blurs to content.)

  **(c) `?` help tests in app_test.go** (added in the prior feature) — the focus model changes their premises:
  - `TestQuestionMarkTogglesHelpOverlay`: it fetches first (content focused) so `?` still opens help; but it asserts the overlay contains `"Alt+←"`, which no longer exists. Change that assertion to `strings.Contains(view, "move")` (or `"page"`).
  - `TestQuestionMarkFromReaderOpensHelp`: it pressed `?` from a fresh `newApp`, which is now the **input-focused landing** where `?` is literal. Rework it to reach a content-focused reader first — drive a profile fetch (`m.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t,"alice@plan.cat"), Body: []byte("Plan\n")}})`), then press `?` and assert `m.help`.
  - `TestQuestionMarkWhileFilteringDoesNotOpenHelp`: unchanged in behaviour (list filtering still defers `?`); it sets list state manually, so also add `m.inputFocused = false` to its setup.

  After these edits run `go build ./tui/`, then fix any remaining mechanical compile errors (the only removed reader fields are `input`, `loading`, `status`).

- [ ] **Step 12: Run the suite** — `go test ./tui/ -count=1`. Fix compile/asserts until green, then `make check`. Expected: the new Task-2 tests pass; history/drill/bar tests still pass.

- [ ] **Step 13: Commit**

```bash
git add tui/app.go tui/reader.go tui/app_test.go tui/reader_test.go
git commit -m "feat(tui): blur-by-default focus; move input to appModel; drop Alt/forward"
```

---

## Task 3: bubbles/help bottom panel

Replace the full-screen `helpView` with a glow-style expandable bottom panel that shrinks the content.

**Files:** Modify `tui/app.go`, `tui/app_test.go`.

- [ ] **Step 1: Failing test** — add to `tui/app_test.go`:

```go
func TestHelpExpandsAtBottomNotFullScreen(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	host := hostTarget(t, "@tilde.team")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)

	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = step.(appModel)
	view := m.View().Content
	if !strings.Contains(view, "move") || !strings.Contains(view, "page") {
		t.Fatalf("expanded help missing move/page keys:\n%s", view)
	}
	// Not a full-screen takeover: a list user is still visible alongside help.
	if !strings.Contains(view, "alrs") {
		t.Fatalf("help should not blank the content:\n%s", view)
	}
}
```

- [ ] **Step 2: Verify failure** — `go test ./tui/ -run TestHelpExpandsAtBottom -count=1 -v` → FAIL (full-screen swap hides `alrs`).

- [ ] **Step 3: Implement** — add a `help.Model` and render it as a bottom block. In `tui/app.go` add import `"charm.land/bubbles/v2/help"`; add field `helpModel help.Model`; init in `newApp` with `helpModel: help.New()`; in `handleKey` set `m.helpModel.ShowAll = true` when opening and `false` when closing (alongside the `m.help` bool). Replace `View`:

```go
func (m appModel) View() tea.View {
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
	v.MouseMode = tea.MouseModeCellMotion // dropped in Task 6
	return v
}
```

In `handleKey`, the open/close branches become:

```go
	if m.help {
		m.help = false
		m.helpModel.ShowAll = false
		return true, m, nil
	}
	...
	case key.Matches(msg, m.keys.Help):
		m.help = true
		m.helpModel.ShowAll = true
		return true, m, nil
```

Delete the `helpView` method. Shrink the content to make room for the help block: in `View`, when `m.help`, the content must be shorter. Compute the help height and set the active sub-model's size for this frame is overkill; instead reserve rows by trimming content lines. Simplest correct approach: when `m.help`, size the content to `bodyHeight - helpHeight`. Add a helper and call it in `View` before rendering content:

```go
func (m *appModel) helpHeight() int {
	if !m.help {
		return 0
	}
	return lipgloss.Height(m.helpModel.View(m.keys))
}
```

Add import `"charm.land/lipgloss/v2"`. Then in `View`, before rendering content, when `m.help` temporarily shrink: since `View` has a value receiver, build content against a reduced height by calling `m.reader`/`m.list` `View()` after `setSize(width, bodyHeight()-helpHeight())`. To avoid mutating in `View`, instead reserve in the sizing path: handle a `?` toggle by re-running `setSize`. Concretely, in `handleKey` open/close branches, after toggling `m.help`, call:

```go
		m.resizeForHelp()
```

and add:

```go
// resizeForHelp re-sizes the active sub-model to leave room for the help block.
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
```

(When help closes, `helpHeight()` is 0 so it restores full height.)

- [ ] **Step 4: Verify** — `go test ./tui/ -run 'TestHelp|TestQuestionMark' -count=1 -v` → PASS (the merged `TestQuestionMarkTogglesHelpOverlay`/`...WhileFiltering`/`...FromReader` still pass — `m.help` toggles as before; if `TestQuestionMarkTogglesHelpOverlay` asserted the literal `Alt+←`, update it to assert `move`/`page` instead). Then `make check`.

- [ ] **Step 5: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): glow-style bottom help panel via bubbles/help"
```

---

## Task 4: bubbles/spinner loading indicator

**Files:** Modify `tui/app.go`, `tui/app_test.go`.

- [ ] **Step 1: Failing test** — add to `tui/app_test.go`:

```go
func TestLoadingShowsSpinnerTarget(t *testing.T) {
	// A fetch that we drive manually: set loading via submit, render the bar.
	m := newApp(func(_ context.Context, tg finger.Target) ([]byte, finger.Meta, error) {
		return []byte("Plan\n"), finger.Meta{}, nil
	}, colorprofile.NoTTY)
	m.common.width = 80
	m.input.SetValue("bob@sdf.org")
	step, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = step.(appModel)
	if !m.loading {
		t.Fatal("submit should set loading")
	}
	if !strings.Contains(m.statusBarModel().render(), "bob@sdf.org") {
		t.Fatalf("loading bar should name the target:\n%s", m.statusBarModel().render())
	}
}

func TestResultClearsLoading(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.loading = true
	host := hostTarget(t, "@tilde.team")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	if step.(appModel).loading {
		t.Fatal("a fetch result should clear loading")
	}
}
```

- [ ] **Step 2: Verify failure** — `go test ./tui/ -run 'TestLoadingShows|TestResultClears' -count=1 -v` → FAIL.

- [ ] **Step 3: Implement** — add import `"charm.land/bubbles/v2/spinner"`; add field `spin spinner.Model`; init `spin: spinner.New()` in `newApp`. Kick the spinner on submit/drill by batching its tick, and tick it in `Update`:

  - In `submit` and `drill`, return the fetch batched with the spinner tick:
    ```go
    return tea.Batch(fetchCmd(context.Background(), m.common.fetch, target), m.spin.Tick)
    ```
    (Update the two `return ... fetchCmd(...)` sites accordingly; `submit` returns `tea.Cmd`, `drill` returns the triple.)
  - In `Update`, handle the tick (add a case before the delegate tail):
    ```go
    case spinner.TickMsg:
        if m.loading {
            var cmd tea.Cmd
            m.spin, cmd = m.spin.Update(msg)
            return m, cmd
        }
        return m, nil
    ```
  - In `statusBarModel`, when `m.loading`, override meta/hints with the spinner + target (before the `m.pos < 0` landing check, so loading shows even from the landing):
    ```go
    if m.loading {
        bar := statusBar{width: w, styles: st}
        bar.hints = m.spin.View() + " loading " + m.loadingTarget.Raw
        return bar
    }
    ```
    Put this immediately after `w := m.common.width`.

- [ ] **Step 4: Verify** — `go test ./tui/ -run 'TestLoading|TestResultClears' -count=1 -v` → PASS; `make check`.

- [ ] **Step 5: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): bubbles/spinner loading indicator"
```

---

## Task 5: status bar scroll-% and page indicator

**Files:** Modify `tui/statusbar.go`, `tui/app.go`, `tui/statusbar_test.go`, `tui/app_test.go`.

- [ ] **Step 1: Failing tests** — add to `tui/statusbar_test.go`:

```go
func TestStatusBarShowsScrollAndPage(t *testing.T) {
	b := statusBar{host: "@tilde.team", user: "bob", scroll: "42%",
		hints: "? help", width: 80, styles: newStyles()}
	if !strings.Contains(b.render(), "42%") {
		t.Fatalf("bar missing scroll %%: %q", b.render())
	}
	b2 := statusBar{host: "@sdf.org", page: "page 2/4", meta: "42 users",
		hints: "? help", width: 80, styles: newStyles()}
	if !strings.Contains(b2.render(), "page 2/4") {
		t.Fatalf("bar missing page indicator: %q", b2.render())
	}
}
```

- [ ] **Step 2: Verify failure** — `go test ./tui/ -run TestStatusBarShowsScrollAndPage -count=1 -v` → FAIL (`scroll`/`page` fields absent).

- [ ] **Step 3: Implement** — in `tui/statusbar.go` add `scroll string` and `page string` fields to `statusBar`, and include them in the right-hand group of `render()`. Find where the right group is assembled (the `var right []string` block) and add, in order, after `escTarget` and before `meta`:

```go
	if b.page != "" {
		right = append(right, b.page)
	}
	if b.scroll != "" {
		right = append(right, b.scroll)
	}
```

(Keep `meta` and `hints` appends as they are.)

In `tui/app.go` `statusBarModel`, fill them:
- reader branch (`default`): after setting `bar.meta`, add
  ```go
  if m.reader.viewport.TotalLineCount() > m.reader.viewport.Height() {
      bar.scroll = fmt.Sprintf("%d%%", int(m.reader.viewport.ScrollPercent()*100))
  }
  ```
- list branch: after setting `bar.meta`, add
  ```go
  if tp := m.list.list.Paginator.TotalPages; tp > 1 {
      bar.page = fmt.Sprintf("page %d/%d", m.list.list.Paginator.Page+1, tp)
  }
  ```

(`viewport.TotalLineCount()` and `ScrollPercent()` exist in bubbles v2; if `TotalLineCount` is named differently, gate on `ScrollPercent() < 1 || YOffset() > 0` instead.)

Add an app-level assertion to `tui/app_test.go`:

```go
func TestListBarShowsPageIndicatorWhenPaged(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 40, 8 // small height forces multiple pages
	users := make([]User, 40)
	for i := range users {
		users[i] = User{Login: fmt.Sprintf("u%02d", i)}
	}
	body := "Login\n"
	for _, u := range users {
		body += u.Login + "\n"
	}
	step, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 8})
	m = step.(appModel)
	step, _ = m.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t, "@big.host"), Body: []byte(body)}})
	m = step.(appModel)
	if m.state == stateList && m.list.list.Paginator.TotalPages > 1 {
		if !strings.Contains(m.statusBarModel().render(), "page 1/") {
			t.Fatalf("expected page indicator:\n%s", m.statusBarModel().render())
		}
	}
}
```

(Add `"fmt"` to app_test.go imports if missing.)

- [ ] **Step 4: Verify** — `go test ./tui/ -run 'TestStatusBarShowsScrollAndPage|TestListBarShowsPage' -count=1 -v` → PASS; `make check`.

- [ ] **Step 5: Commit**

```bash
git add tui/statusbar.go tui/app.go tui/statusbar_test.go tui/app_test.go
git commit -m "feat(tui): scroll-% and page indicator in the status bar"
```

---

## Task 6: drop mouse capture + `y` yank-copy + flash

**Files:** Modify `tui/app.go`, `tui/app_test.go`.

- [ ] **Step 1: Failing tests** — add to `tui/app_test.go`:

```go
func TestViewSetsNoMouseMode(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	if m.View().MouseMode != tea.MouseModeNone {
		t.Fatalf("MouseMode = %v, want none (native copy preserved)", m.View().MouseMode)
	}
}

func TestYCopiesAddressWithFlash(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel) // list of @tilde.team, content focused

	step, _ = m.Update(tea.KeyPressMsg{Code: 'y'})
	m = step.(appModel)
	if copied != "alrs@tilde.team" {
		t.Fatalf("copied = %q, want alrs@tilde.team", copied)
	}
	if !strings.Contains(m.flash, "alrs@tilde.team") {
		t.Fatalf("flash = %q, want it to mention the copied address", m.flash)
	}
}
```

- [ ] **Step 2: Verify failure** — `go test ./tui/ -run 'TestViewSetsNoMouseMode|TestYCopies' -count=1 -v` → FAIL.

- [ ] **Step 3: Implement** —
  - In `View`, delete the line `v.MouseMode = tea.MouseModeCellMotion` (leave `AltScreen = true`). Default `MouseMode` is `tea.MouseModeNone`.
  - Add a clipboard seam near the top of `app.go`: `var setClipboard = tea.SetClipboard`.
  - Add a copy helper:
    ```go
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
    ```
  - Add the flash-clear timer:
    ```go
    type clearFlashMsg struct{}

    func (m *appModel) clearFlashCmd() tea.Cmd {
        return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })
    }
    ```
    (Add `"time"` to imports.)
  - In `Update`, handle the clear:
    ```go
    case clearFlashMsg:
        m.flash = ""
        return m, nil
    ```
  - In `handleKey`, content-focused branch, add a `Copy` case (before `Open`):
    ```go
    case key.Matches(msg, m.keys.Copy):
        cmd := m.copyAddress()
        return true, m, cmd
    ```
  - In `statusBarModel`, surface the flash: immediately after the `m.loading` block, add
    ```go
    if m.flash != "" && m.pos >= 0 {
        // (fall through to build the bar, then override hints below)
    }
    ```
    Simpler: at the end of `statusBarModel`, just before `return bar`, add `if m.flash != "" { bar.hints = m.flash }`. (The flash replaces the hint text until it clears.)

- [ ] **Step 4: Verify** — `go test ./tui/ -run 'TestViewSetsNoMouseMode|TestYCopies' -count=1 -v` → PASS; `make check`. Also confirm `y` is literal while the input is focused — it is, because the content-focused switch is only reached when `!m.inputFocused`; add a quick assertion if desired.

- [ ] **Step 5: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): drop mouse capture; add y-yank copy with flash"
```

---

## Task 7: default list delegate

**Files:** Modify `tui/list.go`, `tui/styles.go`, `tui/list_test.go`.

- [ ] **Step 1: Failing test** — add to `tui/list_test.go`:

```go
func TestUserItemImplementsDefaultItem(t *testing.T) {
	it := userItem{login: "alrs", name: "Alvaro", target: "alrs@tilde.team"}
	if it.Title() != "alrs" {
		t.Fatalf("Title = %q, want alrs", it.Title())
	}
	desc := it.Description()
	if !strings.Contains(desc, "Alvaro") || !strings.Contains(desc, "alrs@tilde.team") {
		t.Fatalf("Description = %q, want name + target", desc)
	}
}

func TestDefaultDelegateRendersLoginAndName(t *testing.T) {
	common := &commonModel{width: 80, height: 20}
	m := newList(common, hostTarget(t, "@tilde.team"), []User{{Login: "alrs", Name: "Alvaro"}})
	m.setSize(80, 18)
	view := m.View()
	if !strings.Contains(view, "alrs") || !strings.Contains(view, "Alvaro") {
		t.Fatalf("list view missing login/name:\n%s", view)
	}
}
```

- [ ] **Step 2: Verify failure** — `go test ./tui/ -run 'TestUserItemImplements|TestDefaultDelegateRenders' -count=1 -v` → FAIL (`Title`/`Description` undefined).

- [ ] **Step 3: Implement** — in `tui/list.go`:
  - Give `userItem` the `DefaultItem` methods (keep `FilterValue`):
    ```go
    func (i userItem) Title() string { return i.login }
    func (i userItem) Description() string {
        parts := []string{}
        if i.name != "" {
            parts = append(parts, i.name)
        }
        if i.target != "" {
            parts = append(parts, i.target)
        }
        if len(parts) == 0 {
            return ""
        }
        return strings.Join(parts, " · ")
    }
    ```
  - Delete the `userDelegate` type and its methods (and the now-unused `io`/`fmt` imports if they become unused — keep `fmt` only if still referenced; `newList` uses `fmt.Sprintf` for the Title, so keep `fmt`; drop `io`).
  - In `newList`, build the default delegate and theme its `Styles` field in place (`list.NewDefaultDelegate()` already populates `d.Styles`; we just override colours to lookit's palette):
    ```go
    d := list.NewDefaultDelegate()
    d.Styles.SelectedTitle = d.Styles.SelectedTitle.
        Foreground(lipgloss.Color("#8affc1")).BorderForeground(lipgloss.Color("#8affc1"))
    d.Styles.SelectedDesc = d.Styles.SelectedDesc.Foreground(lipgloss.Color("#8fb7ff"))
    d.Styles.NormalDesc = d.Styles.NormalDesc.Foreground(lipgloss.Color("#808080"))
    l := list.New(items, d, width, height)
    ```
    (Add `"charm.land/lipgloss/v2"` to list.go's imports for `lipgloss.Color`.)
  - The `selected`/`listName` style fields in `styles.go` were only used by the deleted `userDelegate` — remove those two fields and their initializers from the `styles` struct and `newStyles()`. (No new import needed in styles.go.) Run `git grep -n "\.selected\|\.listName" tui/` to confirm nothing else references them before deleting.

- [ ] **Step 4: Verify** — `go test ./tui/ -count=1`. The default delegate is ~2 lines/item; any existing list-view test asserting exact single-line layout (`> alrs  Alvaro`) must be updated to substring checks (`alrs`, `Alvaro`). `make check`.

- [ ] **Step 5: Commit**

```bash
git add tui/list.go tui/styles.go tui/list_test.go
git commit -m "feat(tui): use the default list delegate (title + description)"
```

---

## Task 8: state-driven binding enablement (`updateKeymap`) — DONE (commit `2a93e28`)

> **As-built note.** The first draft of this task derived the status-bar short
> hints from `keyMap.ShortHelp()`. During execution that was found to be a
> **regression** (lookit's per-state bar hints — `/ filter`, `r raw`, `↑↓ scroll`
> — are richer than `ShortHelp()`'s fixed set, and `ShortHelp()` is never
> rendered: the collapsed help *is* the bar's own hints). The scope was changed
> (with the user) to **"panel + input-focused bar fix"**: enablement drives the
> `?` help panel and routing; the bar keeps its richer hints and only gains an
> input-focused (`↵ fetch · esc cancel`) hint. This section records what shipped.

Make the `keyMap` the single source of truth for which keys are live in the
current state, mirroring `pop`'s `updateKeymap()` (`~/pop/keymap.go`). It has two
effects, both from `bubbles` behaviour:

1. **Help panel.** `bubbles/help` skips disabled bindings (`help.go:138/203`), so
   the `?` panel advertises only live keys (no `open`/`filter` in a profile
   reader; no content keys while typing).
2. **Routing.** `key.Matches` returns false for a disabled binding, so a
   content-only key is inert (typed literally) while the input is focused.

Because of (2), `updateKeymap()` must run **before `handleKey`** — so it is
called at the top of `Update`, *not* only in the render path. It also runs in
`View`/`helpHeight` so the panel reflects the post-key state. (The earlier
"display only / no stale-state risk" claim was wrong: routing depends on it.)

**Key classes:** `Open`(`Enter`)/`Back`(`Esc`)/`Help`(`?`) are **dual-mode** —
`handleKey`'s input-focused branch matches them (submit / cancel / help), so they
stay enabled while typing; `Open`/`Filter` are gated to the list when
content-focused, `Back` to `pos >= 0`. `FocusInput`/`Copy`/`Raw`/`Quit`/`Filter`
and display-only `Move`/`Page`/`Jump` are **content-only** (disabled while
typing). No `shortHints` helper was added.

**Files:** Modified `tui/app.go`, `tui/app_test.go`.

- [x] **Step 1–2: Failing tests** — added `TestUpdateKeymapGatesByState`,
  `TestHelpPanelHidesInertKeys`, `TestInputFocusedBarShowsFetchCancel` to
  `tui/app_test.go` (no `key` import needed — they assert `.Enabled()` directly
  and inspect `View().Content` / `statusBarModel().render()`).

- [x] **Step 3: Implement** — in `tui/app.go`:

```go
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

	// Dual-mode commands — matched in BOTH the input-focused and content
	// branches, so live while typing: Open=Enter (submit/drill), Back=Esc
	// (cancel/back/quit-at-landing), Help='?'.
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
```

Call sites: `(&m).updateKeymap()` at the top of `Update` (before the message
switch) and of `View`; `m.updateKeymap()` at the top of `helpHeight` (after the
`!m.help` guard) so the measured and rendered panel heights match; `app.updateKeymap()`
at the end of `newApp` for the first frame.

Status bar: an **input-focused branch** was added to `statusBarModel()` (after
the breadcrumb is built, before the `showingRaw`/state switch): it clears
`escTarget` and sets `bar.hints = "↵ fetch · esc cancel"` (honouring a `flash`
override). The existing per-state content hints are unchanged.

- [x] **Step 4: Verify** — the three new tests pass; full `tui` suite green; `make check` green.

- [x] **Step 5: Commit** — `2a93e28` `feat(tui): state-driven key enablement for the help panel`.

---

## Final verification

- [ ] `make check` green (vet, gofmt, golangci-lint, race).
- [ ] `git grep -n "forward\|MouseModeCellMotion\|ModAlt" tui/*.go` returns nothing (forward + Alt + mouse capture all gone). `helpView` may remain only if you kept a fallback — it should be deleted (Task 3).
- [ ] `git grep -n "userDelegate" tui/*.go` returns nothing.
- [ ] `git grep -n "updateKeymap" tui/app.go` shows the method exists and is called from `Update`, `View`, and `helpHeight` (Task 8).
- [ ] `make build && ./lookit` in a real terminal: landing focuses input; type `@tilde.team` ↵ → list; `j/k` move, `←/→` page, `g/G` jump; `i` focuses input pre-filled; `y` copies + flashes; `?` expands a bottom help block (content still visible) that lists **only** the keys live in that state (e.g. no `open`/`filter` in a profile reader, none of the content keys while the input is focused); `Esc` backs to landing then quits; drag-to-select copies natively (no mouse capture).

## Notes for the executor

- One package only (`tui/`); never modify `finger/` or `render/`.
- Conventional Commits, **no `Co-Authored-By`**.
- The TUI can't be smoke-tested headlessly — rely on unit tests; the final manual check is a human step.
- If a bubbles API name differs slightly from this plan (e.g. a viewport line-count getter), adjust to the real signature in `~/bubbles`/the module cache rather than forcing the name — report it as a concern if behaviour would change.

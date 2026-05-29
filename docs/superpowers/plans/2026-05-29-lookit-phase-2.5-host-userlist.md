# lookit Phase 2.5 Host User-List Drill-Down Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When `lookit` fingers a `@host` that returns a recognizable user list, show that list as a selectable screen; pressing Enter on a user fingers `login@host` and shows the result in the reader.

**Architecture:** Refactor the merged Phase 2 single `tui.Model` into a glow-style state machine: an `appModel` routes between a `readerModel` (today's input+viewport reader) and a `listModel` (a `bubbles/v2/list`). A pure `ParseUsers` function decides whether a host response is a recognizable list. `tui.Run` stays the only exported entry point, so `main.go` is untouched.

**Tech Stack:** Go, Bubble Tea v2 (`charm.land/bubbletea/v2`), Bubbles v2 (`charm.land/bubbles/v2` list/textinput/viewport), Lip Gloss v2 (`charm.land/lipgloss/v2`), existing `github.com/charmbracelet/colorprofile`, existing `finger/` and `render/` packages.

**Spec:** `docs/superpowers/specs/2026-05-29-lookit-phase-2.5-host-userlist-design.md`

---

## File Structure

```
tui/
├── userlist.go        # NEW: User type + ParseUsers (pure parser)
├── userlist_test.go   # NEW: golden corpus
├── reader.go          # RENAMED from model.go: readerModel (input+viewport reader)
├── reader_test.go     # RENAMED from model_test.go: reader tests
├── list.go            # NEW: listModel + userDelegate (bubbles list)
├── list_test.go       # NEW: list tests
├── app.go             # NEW: appModel state machine + routing + navigation
├── app_test.go        # NEW: transition tests
├── fetch.go           # unchanged: Entry, FetchFunc, fetchCmd
├── styles.go          # MODIFY: add list/delegate styles
└── run.go             # MODIFY: construct appModel instead of Model
```

## Shared Types (fixed names for all tasks)

```go
// tui/userlist.go
type User struct {
	Login string
	Name  string // "" when unknown
}
func ParseUsers(body []byte) ([]User, bool)

// tui/reader.go
type readerModel struct { ... }
func newReader(fetch FetchFunc, profile colorprofile.Profile) readerModel

// tui/list.go
type userItem struct{ login, name string }
type listModel struct { ... }
func newList(common *commonModel, host finger.Target, users []User) listModel

// tui/app.go
type appState int
const ( stateReader appState = iota; stateList )
type commonModel struct {
	width   int
	height  int
	profile colorprofile.Profile
	fetch   FetchFunc
}
type appModel struct { ... }
func newApp(fetch FetchFunc, profile colorprofile.Profile) appModel
```

`finger.Target` has fields `User`, `HostPort` ("host:79"), `Raw`. `finger.Meta` has `Addr`, `Bytes`, `Truncated`. `render.Render(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile) string`. These are existing and unchanged.

---

## Task 1: ParseUsers parser and golden corpus

**Files:**
- Create: `/Users/jonathan/lookit/tui/userlist.go`
- Create: `/Users/jonathan/lookit/tui/userlist_test.go`

- [ ] **Step 1: Write the failing test file with the golden corpus**

Create `/Users/jonathan/lookit/tui/userlist_test.go`:

```go
package tui

import (
	"reflect"
	"testing"
)

func logins(users []User) []string {
	out := make([]string, len(users))
	for i, u := range users {
		out[i] = u.Login
	}
	return out
}

// --- Hosts that should parse into a user list ---

func TestParseColumnarPlanCat(t *testing.T) {
	body := []byte("Login                Name                 Login Time\n" +
		"jss                                       Fri May 29 05:31 UTC\n" +
		"geurimja             Geurimja             Thu May 28 21:57 UTC\n" +
		"26d0                 Jimenshi             Thu May 28 03:20 UTC\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"jss", "geurimja", "26d0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
	if users[1].Name != "Geurimja" {
		t.Fatalf("users[1].Name = %q, want %q", users[1].Name, "Geurimja")
	}
}

func TestParseColumnarDedupTildePink(t *testing.T) {
	body := []byte("Login       Name                Tty      Idle  Login Time   Where\n" +
		"irek                            pts/15   207d  Sep 13 2025\n" +
		"irek                            pts/16   256d  Sep 14 2025\n" +
		"ghoti                           pts/7      1d  Apr  6 14:59\n" +
		"irek                            pts/17   200d  Sep 14 2025\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"irek", "ghoti"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v (deduped, order preserved)", got, want)
	}
}

func TestParseGridTildeTeam(t *testing.T) {
	body := []byte("welcome to tilde.team\n\n" +
		"hello somehost,\n" +
		"users currently logged in are:\n\n" +
		"alrs\tdtracker\tkapad\n" +
		"anshupati\tenyc\tkneezle\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	want := []string{"alrs", "dtracker", "kapad", "anshupati", "enyc", "kneezle"}
	if got := logins(users); !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
}

func TestParseGridStopsAtSecondBlockCosmicVoyage(t *testing.T) {
	// cosmic.voyage: the "online" block must parse; the separate
	// "Who control these ships:" block (multi-word personas) must NOT.
	body := []byte("Users currently online:\n" +
		"   klu tomasino\n\n" +
		"Who control these ships:\n" +
		"   betsy\n" +
		"   Melvin P Feltersnatch\n" +
		"   Oleander\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"klu", "tomasino"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v (ships block must be excluded)", got, want)
	}
}

func TestParseGridSingleUserZaibatsu(t *testing.T) {
	body := []byte("Currently logged in sundogs:\ndokuja\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"dokuja"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
}

func TestParseMarkerHappyNetBox(t *testing.T) {
	body := []byte("Happy Net Box\n\n25 most recently updated profiles:\n" +
		"> andypiper\n> benbrown\n> goose\n\n" +
		"For a random profile:\n> finger random@happynetbox.com\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"andypiper", "benbrown", "goose"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v (command line excluded)", got, want)
	}
}

// --- Hosts that should NOT parse (decline -> plain reader) ---

func TestDeclineBannerTildeTown(t *testing.T) {
	body := []byte("Hi! we're a little community that exists on a linux server. " +
		"to learn more go to https://tilde.town\n")
	if _, ok := ParseUsers(body); ok {
		t.Fatal("ParseUsers ok = true, want false (banner only)")
	}
}

func TestDeclineEmptyTildeClub(t *testing.T) {
	if _, ok := ParseUsers([]byte("")); ok {
		t.Fatal("ParseUsers ok = true, want false (empty)")
	}
}

func TestDeclineInlineCueTypedHole(t *testing.T) {
	// Users are inline on the cue line ("probably julien"); must NOT be parsed.
	body := []byte("Welcome to the Typed Hole\n" +
		"Users currently logged in: probably julien\n\n" +
		"Available fingers:\n" +
		"weather:\tget weather\nlobsters:\tget stories\n")
	if _, ok := ParseUsers(body); ok {
		t.Fatal("ParseUsers ok = true, want false (inline cue must not parse)")
	}
}

func TestDeclineDaemonHelpDebian(t *testing.T) {
	body := []byte("userdir-ldap finger daemon\n--------------------------\n" +
		"finger <uid>[/<attributes>]@db.debian.org\n  where uid is the user id\n")
	if _, ok := ParseUsers(body); ok {
		t.Fatal("ParseUsers ok = true, want false (daemon help)")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /Users/jonathan/lookit
go test ./tui/ -run TestParse -count=1
```

Expected: build failure — `undefined: ParseUsers` and `undefined: User`.

- [ ] **Step 3: Implement `userlist.go`**

Create `/Users/jonathan/lookit/tui/userlist.go`:

```go
package tui

import (
	"regexp"
	"strings"
)

// User is one entry in a host's finger user list.
type User struct {
	Login string
	Name  string // "" when unknown
}

// loginRe matches a plausible Unix login: leading alphanumeric/underscore,
// then login characters, max 32 runes. Rejects FQDNs, punctuated words, and
// over-long tokens.
var loginRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,31}$`)

// gridCueRe gates Format 1 (token grid). Only a recognized "who is here" cue
// turns following lines into a login block.
var gridCueRe = regexp.MustCompile(`(?i)logged[\s-]?in|online`)

// markerRe matches Format 3 marker rows: "> login" with a single login token.
var markerRe = regexp.MustCompile(`^\s*>\s+(\S+)\s*$`)

// ParseUsers extracts a host's logged-in / listed users from a finger response
// body. It returns (users, true) only when a format is confidently recognized;
// otherwise (nil, false). The caller guarantees this is a host query.
//
// Three gated matchers are tried in order: columnar (Login header), grid
// (cue line), marker ("> login"). Results are deduplicated, order preserved.
func ParseUsers(body []byte) ([]User, bool) {
	lines := strings.Split(string(body), "\n")

	if users, ok := parseColumnar(lines); ok {
		return users, true
	}
	if users, ok := parseGrid(lines); ok {
		return users, true
	}
	if users, ok := parseMarker(lines); ok {
		return users, true
	}
	return nil, false
}

// parseColumnar handles classic fingerd output: a "Login ... Name ..." header
// followed by one row per session. Login is the first whitespace token; Name
// is best-effort (the second token when it looks like a name, else "").
func parseColumnar(lines []string) ([]User, bool) {
	header := -1
	hasName := false
	for i, ln := range lines {
		fields := strings.Fields(ln)
		if len(fields) > 0 && strings.EqualFold(fields[0], "Login") {
			header = i
			for _, f := range fields[1:] {
				if strings.EqualFold(f, "Name") {
					hasName = true
				}
			}
			break
		}
	}
	if header < 0 {
		return nil, false
	}

	var users []User
	seen := map[string]bool{}
	for _, ln := range lines[header+1:] {
		if strings.TrimSpace(ln) == "" {
			break
		}
		fields := strings.Fields(ln)
		if len(fields) == 0 || !loginRe.MatchString(fields[0]) {
			continue
		}
		login := fields[0]
		if seen[login] {
			continue
		}
		seen[login] = true
		name := ""
		// Best-effort name: take a single non-login-looking second token.
		if hasName && len(fields) >= 2 && !looksLikeColumnNoise(fields[1]) {
			name = fields[1]
		}
		users = append(users, User{Login: login, Name: name})
	}
	if len(users) == 0 {
		return nil, false
	}
	return users, true
}

// looksLikeColumnNoise rejects obvious non-name second tokens (tty/idle/date
// fragments) so a bare login row does not get a junk name. Best-effort only.
func looksLikeColumnNoise(s string) bool {
	if s == "" {
		return true
	}
	// Tty/idle columns like "pts/15", "*p1", "t6", or dates "Fri"/"May".
	if strings.ContainsAny(s, "/*:") {
		return true
	}
	return false
}

// parseGrid handles a whitespace/tab grid of bare logins that appears after a
// recognized cue line. It collects only the contiguous block immediately
// following the cue (after up to one blank line); a line containing any
// non-login token ends the block. Cue-line tokens are never parsed.
func parseGrid(lines []string) ([]User, bool) {
	cue := -1
	for i, ln := range lines {
		if gridCueRe.MatchString(ln) {
			cue = i
			break
		}
	}
	if cue < 0 {
		return nil, false
	}

	i := cue + 1
	// Skip up to one blank line between the cue and the block.
	if i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}

	var users []User
	seen := map[string]bool{}
	for ; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			break
		}
		fields := strings.Fields(lines[i])
		allLogins := len(fields) > 0
		for _, f := range fields {
			if !loginRe.MatchString(f) {
				allLogins = false
				break
			}
		}
		if !allLogins {
			break
		}
		for _, f := range fields {
			if seen[f] {
				continue
			}
			seen[f] = true
			users = append(users, User{Login: f})
		}
	}
	if len(users) == 0 {
		return nil, false
	}
	return users, true
}

// parseMarker handles "> login" lists (e.g. happynetbox). Each matching line
// must have exactly one login token after the marker.
func parseMarker(lines []string) ([]User, bool) {
	var users []User
	seen := map[string]bool{}
	for _, ln := range lines {
		m := markerRe.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		login := m[1]
		if !loginRe.MatchString(login) || seen[login] {
			continue
		}
		seen[login] = true
		users = append(users, User{Login: login})
	}
	if len(users) == 0 {
		return nil, false
	}
	return users, true
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
cd /Users/jonathan/lookit
gofmt -w tui/
go test ./tui/ -run 'TestParse|TestDecline' -count=1 -v
```

Expected: all 10 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/jonathan/lookit
git add tui/userlist.go tui/userlist_test.go
git commit -m "feat(tui): parse host finger responses into user lists"
```

---

## Task 2: Extract readerModel and introduce appModel shell

This task renames the Phase 2 `model.go` into `reader.go` as `readerModel`, strips process-level concerns (quit, fetch-result routing, terminal-mode declarations) out of it, and adds an `appModel` that wraps it. At the end of this task the TUI behaves exactly like Phase 2 (reader only); the list state is added in Task 4.

**Files:**
- Create: `/Users/jonathan/lookit/tui/reader.go` (from `model.go`)
- Create: `/Users/jonathan/lookit/tui/reader_test.go` (from `model_test.go`)
- Delete: `/Users/jonathan/lookit/tui/model.go`, `/Users/jonathan/lookit/tui/model_test.go`
- Create: `/Users/jonathan/lookit/tui/app.go`
- Modify: `/Users/jonathan/lookit/tui/run.go`

- [ ] **Step 1: Create `reader.go` with the extracted readerModel**

Create `/Users/jonathan/lookit/tui/reader.go`:

```go
package tui

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
	"github.com/jonathandeamer/lookit/render"
)

const initialStatus = "enter a finger target, then press Enter"

// chromeRows counts the non-viewport lines in the reader view: title, input,
// status, hint.
const chromeRows = 4

// readerModel is the query reader: a target input, a status line, and a
// scrollable viewport showing one rendered finger response. It owns typing,
// scrolling, and starting a fetch on Enter. It does NOT route fetch results,
// quit, or declare terminal modes — appModel owns those.
type readerModel struct {
	input    textinput.Model
	viewport viewport.Model
	current  *Entry
	loading  bool
	status   string
	profile  colorprofile.Profile
	fetch    FetchFunc
	styles   styles
	width    int
	height   int
}

func newReader(fetch FetchFunc, profile colorprofile.Profile) readerModel {
	if fetch == nil {
		fetch = defaultFetch
	}

	input := textinput.New()
	input.Placeholder = "alice@plan.cat"
	input.Prompt = "target: "
	input.Focus()
	input.CharLimit = 256
	input.SetWidth(40)

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(10))
	vp.SetContent("No response yet.")

	return readerModel{
		input:   input,
		viewport: vp,
		status:  initialStatus,
		profile: profile,
		fetch:   fetch,
		styles:  newStyles(),
	}
}

// Init returns the input blink command.
func (m readerModel) Init() tea.Cmd {
	return textinput.Blink
}

// update handles reader-local messages: typing, scrolling, and Enter to start
// a fetch. Enter returns a fetch command (the result is routed by appModel).
// It never quits and never handles fetchResultMsg.
func (m readerModel) update(msg tea.Msg) (readerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.Key()
		if key.Code == tea.KeyEnter {
			if m.loading {
				return m, nil
			}
			target, err := finger.ParseTarget(strings.TrimSpace(m.input.Value()))
			if err != nil {
				m.status = "error: " + err.Error()
				return m, nil
			}
			m.setLoading(target)
			return m, fetchCmd(context.Background(), m.fetch, target)
		}
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// View renders the reader as a plain string. appModel wraps it in a tea.View.
func (m readerModel) View() string {
	var b strings.Builder
	b.WriteString(m.styles.title.Render("lookit"))
	b.WriteByte('\n')
	b.WriteString(m.input.View())
	b.WriteByte('\n')
	if strings.HasPrefix(m.status, "error:") {
		b.WriteString(m.styles.error.Render(m.status))
	} else {
		b.WriteString(m.styles.status.Render(m.status))
	}
	b.WriteByte('\n')
	b.WriteString(m.viewport.View())
	b.WriteByte('\n')
	b.WriteString(m.styles.hint.Render("Enter fetches - arrows/PageUp/PageDown scroll - Esc back/quit"))
	return b.String()
}

// setSize lays out the input and viewport for the given terminal size.
func (m *readerModel) setSize(width, height int) {
	m.width = width
	m.height = height
	if width <= 0 || height <= 0 {
		return
	}
	inputWidth := width - len(m.input.Prompt)
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.input.SetWidth(inputWidth)
	m.viewport.SetWidth(width)
	vh := height - chromeRows
	if vh < 1 {
		vh = 1
	}
	m.viewport.SetHeight(vh)
}

// setProfile updates the color profile and re-renders the current entry.
func (m *readerModel) setProfile(p colorprofile.Profile) {
	m.profile = p
	if m.current != nil {
		m.viewport.SetContent(renderEntry(m.profile, *m.current))
	}
}

// setLoading marks a fetch in progress for the given target.
func (m *readerModel) setLoading(target finger.Target) {
	m.loading = true
	m.status = "loading " + target.Raw + "..."
}

// setEntry displays a fetched result, clearing the loading state.
func (m *readerModel) setEntry(entry Entry) {
	m.loading = false
	m.current = &entry
	m.status = statusForEntry(entry)
	m.viewport.SetContent(renderEntry(m.profile, entry))
}

func statusForEntry(entry Entry) string {
	if entry.Err != nil {
		return "error: " + entry.Err.Error()
	}
	if entry.Meta.Truncated {
		return "loaded " + entry.Target.Raw + " (truncated)"
	}
	return "loaded " + entry.Target.Raw
}

func renderEntry(profile colorprofile.Profile, entry Entry) string {
	return render.Render(entry.Target, entry.Body, entry.Meta, entry.Err, profile)
}
```

- [ ] **Step 2: Create `app.go` with a reader-only appModel**

Create `/Users/jonathan/lookit/tui/app.go`:

```go
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
```

- [ ] **Step 3: Update `run.go` to construct appModel**

Replace the body of `/Users/jonathan/lookit/tui/run.go`'s `Run` so it builds `newApp`:

```go
package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// Run starts the interactive TUI and blocks until the user quits.
//
// Bubble Tea v2's Program.Run does not take a context. The ctx parameter is
// accepted now so cancellation can be wired in later without changing main.go;
// this implementation does not yet use it.
func Run(ctx context.Context, profile colorprofile.Profile) error {
	_ = ctx
	program := tea.NewProgram(newApp(defaultFetch, profile))
	_, err := program.Run()
	return err
}
```

- [ ] **Step 4: Delete the old model files and move the tests**

```bash
cd /Users/jonathan/lookit
git rm tui/model.go tui/model_test.go
```

Create `/Users/jonathan/lookit/tui/reader_test.go` (the Phase 2 reader/sizing tests, retargeted to `newReader`, `update`, and the new accessors; quit and color-profile assertions move to `app_test.go` in Task 4):

```go
package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

func stubFetch(t *testing.T) FetchFunc {
	t.Helper()
	return func(context.Context, finger.Target) ([]byte, finger.Meta, error) {
		t.Fatalf("fetch should not be called")
		return nil, finger.Meta{}, nil
	}
}

func TestNewReaderInitialState(t *testing.T) {
	m := newReader(stubFetch(t), colorprofile.NoTTY)
	if !m.input.Focused() {
		t.Fatalf("input should be focused")
	}
	if m.loading {
		t.Fatalf("loading = true, want false")
	}
	if m.current != nil {
		t.Fatalf("current = %#v, want nil", m.current)
	}
	if m.status == "" {
		t.Fatalf("status should contain an initial hint")
	}
}

func TestReaderInvalidEnterSetsStatusError(t *testing.T) {
	m := newReader(stubFetch(t), colorprofile.NoTTY)
	m.input.SetValue("not-a-target")

	next, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil for invalid input", cmd)
	}
	if next.loading {
		t.Fatalf("loading = true, want false")
	}
	if !strings.Contains(next.status, "error:") {
		t.Fatalf("status = %q, want error", next.status)
	}
}

func TestReaderValidEnterStartsFetch(t *testing.T) {
	calls := 0
	fetch := func(context.Context, finger.Target) ([]byte, finger.Meta, error) {
		calls++
		return []byte("Login: alice\n"), finger.Meta{Addr: "plan.cat:79", Bytes: 13}, nil
	}
	m := newReader(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@plan.cat")

	next, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !next.loading {
		t.Fatalf("loading = false, want true")
	}
	if cmd == nil {
		t.Fatalf("cmd = nil, want fetch command")
	}
	if _, ok := cmd().(fetchResultMsg); !ok {
		t.Fatalf("cmd did not return fetchResultMsg")
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls)
	}
}

func TestReaderDuplicateEnterWhileLoadingDoesNotFetch(t *testing.T) {
	calls := 0
	fetch := func(context.Context, finger.Target) ([]byte, finger.Meta, error) {
		calls++
		return nil, finger.Meta{}, nil
	}
	m := newReader(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@plan.cat")
	m.loading = true

	_, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil while loading", cmd)
	}
	if calls != 0 {
		t.Fatalf("fetch calls = %d, want 0", calls)
	}
}

func TestReaderSetEntryUpdatesViewport(t *testing.T) {
	m := newReader(stubFetch(t), colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	m.setEntry(Entry{
		Target: target,
		Body:   []byte("Login: alice\n"),
		Meta:   finger.Meta{Addr: target.HostPort, Bytes: len("Login: alice\n")},
	})
	if m.loading {
		t.Fatalf("loading = true, want false")
	}
	if m.current == nil || m.current.Target.Raw != "alice@plan.cat" {
		t.Fatalf("current = %#v, want alice entry", m.current)
	}
	if !strings.Contains(m.viewport.View(), "Login: alice") {
		t.Fatalf("viewport content missing body: %q", m.viewport.View())
	}
}

func TestReaderSetEntryError(t *testing.T) {
	m := newReader(stubFetch(t), colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	m.setEntry(Entry{
		Target: target,
		Meta:   finger.Meta{Addr: target.HostPort},
		Err:    errors.New("dial failed"),
	})
	if m.current == nil || m.current.Err == nil {
		t.Fatalf("current = %#v, want error entry", m.current)
	}
	if !strings.Contains(m.viewport.View(), "dial failed") {
		t.Fatalf("viewport content missing error: %q", m.viewport.View())
	}
}

func TestReaderSetSize(t *testing.T) {
	m := newReader(stubFetch(t), colorprofile.NoTTY)
	m.setSize(100, 30)
	if m.viewport.Width() != 100 {
		t.Fatalf("viewport width = %d, want 100", m.viewport.Width())
	}
	if m.viewport.Height() != 26 {
		t.Fatalf("viewport height = %d, want 26", m.viewport.Height())
	}
}
```

- [ ] **Step 5: Run tests and build**

```bash
cd /Users/jonathan/lookit
gofmt -w tui/
go test ./... -count=1
go build -o lookit .
```

Expected: all tests pass; build succeeds. (The TUI still behaves exactly like Phase 2.)

- [ ] **Step 6: Commit**

```bash
cd /Users/jonathan/lookit
git add tui/
git commit -m "refactor(tui): split reader from app state machine"
```

---

## Task 3: listModel and userDelegate

Adds the selectable list screen wrapping `bubbles/v2/list`. Not yet wired into routing; that happens in Task 4.

**Files:**
- Create: `/Users/jonathan/lookit/tui/list.go`
- Create: `/Users/jonathan/lookit/tui/list_test.go`
- Modify: `/Users/jonathan/lookit/tui/styles.go`

- [ ] **Step 1: Add list styles to `styles.go`**

Replace `/Users/jonathan/lookit/tui/styles.go` with:

```go
package tui

import "charm.land/lipgloss/v2"

type styles struct {
	title    lipgloss.Style
	status   lipgloss.Style
	error    lipgloss.Style
	hint     lipgloss.Style
	listName lipgloss.Style // dim real-name column in list rows
	selected lipgloss.Style // highlighted list row
}

func newStyles() styles {
	return styles{
		title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff6fd5")),
		status:   lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")),
		error:    lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b6b")),
		hint:     lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")),
		listName: lipgloss.NewStyle().Foreground(lipgloss.Color("#8fb7ff")),
		selected: lipgloss.NewStyle().Foreground(lipgloss.Color("#8affc1")).Bold(true),
	}
}
```

- [ ] **Step 2: Write the failing list test**

Create `/Users/jonathan/lookit/tui/list_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jonathandeamer/lookit/finger"
)

func testCommon() *commonModel {
	return &commonModel{width: 80, height: 24}
}

func hostTarget(t *testing.T, raw string) finger.Target {
	t.Helper()
	target, err := finger.ParseTarget(raw)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestNewListSelectsFirstUser(t *testing.T) {
	users := []User{{Login: "alrs"}, {Login: "dtracker", Name: "DT"}}
	m := newList(testCommon(), hostTarget(t, "@tilde.team"), users)

	sel, ok := m.selected()
	if !ok {
		t.Fatal("selected ok = false, want true")
	}
	if sel.login != "alrs" {
		t.Fatalf("selected login = %q, want alrs", sel.login)
	}
}

func TestListMoveDownChangesSelection(t *testing.T) {
	users := []User{{Login: "alrs"}, {Login: "dtracker"}}
	m := newList(testCommon(), hostTarget(t, "@tilde.team"), users)

	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyDown})

	sel, _ := m.selected()
	if sel.login != "dtracker" {
		t.Fatalf("after down, selected = %q, want dtracker", sel.login)
	}
}

func TestListViewShowsLoginAndName(t *testing.T) {
	users := []User{{Login: "geurimja", Name: "Geurimja"}}
	m := newList(testCommon(), hostTarget(t, "@plan.cat"), users)

	view := m.View()
	if !strings.Contains(view, "geurimja") {
		t.Fatalf("view missing login: %q", view)
	}
	if !strings.Contains(view, "Geurimja") {
		t.Fatalf("view missing name: %q", view)
	}
}

func TestListNotFilteringByDefault(t *testing.T) {
	m := newList(testCommon(), hostTarget(t, "@tilde.team"), []User{{Login: "alrs"}})
	if m.filtering() {
		t.Fatal("filtering = true, want false on a fresh list")
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

```bash
cd /Users/jonathan/lookit
go test ./tui/ -run TestList -count=1
go test ./tui/ -run TestNewList -count=1
```

Expected: build failure — `undefined: newList`, `undefined: userItem`.

- [ ] **Step 4: Implement `list.go`**

Create `/Users/jonathan/lookit/tui/list.go`:

```go
package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/jonathandeamer/lookit/finger"
)

// listChromeRows reserves space for the list title and footer when sizing.
const listChromeRows = 4

// userItem is one selectable user in the list.
type userItem struct {
	login string
	name  string
}

// FilterValue lets the list filter by login as the user types "/".
func (i userItem) FilterValue() string { return i.login }

// userDelegate renders one user per line: "> login   name".
type userDelegate struct {
	styles styles
}

func (d userDelegate) Height() int                             { return 1 }
func (d userDelegate) Spacing() int                            { return 0 }
func (d userDelegate) Update(tea.Msg, *list.Model) tea.Cmd     { return nil }

func (d userDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(userItem)
	if !ok {
		return
	}
	line := it.login
	if it.name != "" {
		line += "  " + d.styles.listName.Render(it.name)
	}
	cursor := "  "
	if index == m.Index() {
		cursor = "> "
		line = d.styles.selected.Render(it.login)
		if it.name != "" {
			line += "  " + d.styles.listName.Render(it.name)
		}
	}
	fmt.Fprint(w, cursor+line)
}

// listModel wraps a bubbles list of a host's users.
type listModel struct {
	common *commonModel
	list   list.Model
	host   finger.Target
}

func newList(common *commonModel, host finger.Target, users []User) listModel {
	items := make([]list.Item, len(users))
	for i, u := range users {
		items[i] = userItem{login: u.Login, name: u.Name}
	}

	width := common.width
	height := common.height - listChromeRows
	if height < 1 {
		height = 1
	}

	l := list.New(items, userDelegate{styles: newStyles()}, width, height)
	l.Title = fmt.Sprintf("%s — %d users", host.Raw, len(users))
	l.SetShowStatusBar(false)
	l.SetShowHelp(true)

	return listModel{common: common, list: l, host: host}
}

func (m listModel) update(msg tea.Msg) (listModel, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m listModel) View() string {
	var b strings.Builder
	b.WriteString(m.list.View())
	return b.String()
}

func (m *listModel) setSize(width, height int) {
	h := height - listChromeRows
	if h < 1 {
		h = 1
	}
	m.list.SetSize(width, h)
}

// selected returns the highlighted user, if any.
func (m listModel) selected() (userItem, bool) {
	it, ok := m.list.SelectedItem().(userItem)
	return it, ok
}

// filtering reports whether the user is actively typing a filter.
func (m listModel) filtering() bool {
	return m.list.FilterState() == list.Filtering
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /Users/jonathan/lookit
gofmt -w tui/
go test ./tui/ -run 'TestList|TestNewList' -count=1 -v
go build ./...
```

Expected: all four list tests PASS; build succeeds. (`listModel` is not yet referenced by `app.go` — that happens in Task 4. Unused package-level types are valid Go, so the build is clean.)

- [ ] **Step 6: Commit**

```bash
cd /Users/jonathan/lookit
git add tui/
git commit -m "feat(tui): add selectable user list screen"
```

---

## Task 4: Wire routing, navigation, and back-stack

Connects the parser and list into `appModel`: host responses that parse open the list; Enter drills into a user; Esc navigates back.

**Files:**
- Modify: `/Users/jonathan/lookit/tui/app.go`
- Create: `/Users/jonathan/lookit/tui/app_test.go`

- [ ] **Step 1: Write the failing transition tests**

Create `/Users/jonathan/lookit/tui/app_test.go`:

```go
package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

// fetchOnce returns a fetch func yielding fixed bytes and records the targets.
func fetchRecorder(body string) (FetchFunc, *[]string) {
	var seen []string
	f := func(_ context.Context, t finger.Target) ([]byte, finger.Meta, error) {
		seen = append(seen, t.Raw)
		return []byte(body), finger.Meta{Addr: t.HostPort, Bytes: len(body)}, nil
	}
	return f, &seen
}

func hostListBody() string {
	return "users currently logged in are:\n\nalrs\tdtracker\tkapad\n"
}

func TestHostFetchThatParsesOpensList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@tilde.team")
	entry := Entry{Target: target, Body: []byte(hostListBody()), Meta: finger.Meta{Addr: target.HostPort}}

	next, _ := m.Update(fetchResultMsg{entry: entry})
	got := next.(appModel)

	if got.state != stateList {
		t.Fatalf("state = %d, want stateList", got.state)
	}
	if got.hostList == nil {
		t.Fatal("hostList not cached")
	}
	sel, ok := got.list.selected()
	if !ok || sel.login != "alrs" {
		t.Fatalf("list selection = %+v ok=%v, want alrs", sel, ok)
	}
}

func TestHostFetchThatDeclinesStaysInReader(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@tilde.town")
	entry := Entry{Target: target, Body: []byte("just a banner\n"), Meta: finger.Meta{Addr: target.HostPort}}

	next, _ := m.Update(fetchResultMsg{entry: entry})
	got := next.(appModel)

	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader", got.state)
	}
	if !strings.Contains(got.reader.viewport.View(), "just a banner") {
		t.Fatalf("reader viewport missing body: %q", got.reader.viewport.View())
	}
}

func TestUserFetchStaysInReader(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "alice@plan.cat")
	entry := Entry{Target: target, Body: []byte("Plan: hi\n"), Meta: finger.Meta{Addr: target.HostPort}}

	next, _ := m.Update(fetchResultMsg{entry: entry})
	got := next.(appModel)

	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader", got.state)
	}
}

func TestEnterInListDrillsIntoUser(t *testing.T) {
	fetch, seen := fetchRecorder("Plan: hi\n")
	m := newApp(fetch, colorprofile.NoTTY)
	m.common.fetch = fetch
	// Put the app in list state for @tilde.team.
	host := hostTarget(t, "@tilde.team")
	m.hostList = &Entry{Target: host, Body: []byte(hostListBody())}
	users, _ := ParseUsers([]byte(hostListBody()))
	m.list = newList(m.common, host, users)
	m.state = stateList

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)

	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader after drill", got.state)
	}
	if !got.fromList {
		t.Fatal("fromList = false, want true after drill")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want fetch command")
	}
	cmd() // run the fetch command
	if len(*seen) != 1 || (*seen)[0] != "alrs@tilde.team" {
		t.Fatalf("fetched targets = %v, want [alrs@tilde.team]", *seen)
	}
}

func TestEscInDrilledReaderRestoresList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	users, _ := ParseUsers([]byte(hostListBody()))
	m.hostList = &Entry{Target: host, Body: []byte(hostListBody())}
	m.list = newList(m.common, host, users)
	m.state = stateReader
	m.fromList = true

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)

	if got.state != stateList {
		t.Fatalf("state = %d, want stateList after Esc", got.state)
	}
	if got.fromList {
		t.Fatal("fromList = true, want false after returning to list")
	}
}

func TestEscInListReturnsToReaderHome(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	users, _ := ParseUsers([]byte(hostListBody()))
	m.list = newList(m.common, host, users)
	m.state = stateList

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)

	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader", got.state)
	}
	if cmd != nil && isQuit(cmd) {
		t.Fatal("Esc in list must not quit")
	}
}

func TestEscInReaderHomeQuits(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil || !isQuit(cmd) {
		t.Fatal("Esc at reader home should quit")
	}
}

func TestCtrlCQuitsFromList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.state = stateList
	m.list = newList(m.common, hostTarget(t, "@tilde.team"), []User{{Login: "alrs"}})

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil || !isQuit(cmd) {
		t.Fatal("Ctrl+C should quit from list state")
	}
}

func TestWindowSizePropagatesToBothSubModels(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	// hostList set so the guarded list-resize branch runs (must not panic).
	m.hostList = &Entry{Target: host}
	m.state = stateList
	m.list = newList(m.common, host, []User{{Login: "alrs"}})

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	got := next.(appModel)

	if got.common.width != 100 || got.common.height != 30 {
		t.Fatalf("common size = %dx%d, want 100x30", got.common.width, got.common.height)
	}
	if got.reader.viewport.Width() != 100 {
		t.Fatalf("reader viewport width = %d, want 100", got.reader.viewport.Width())
	}
}

// isQuit runs a command and reports whether it produced tea.QuitMsg.
func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
cd /Users/jonathan/lookit
go test ./tui/ -run 'TestHostFetch|TestEnterInList|TestEsc|TestUserFetch' -count=1
```

Expected: failures — the list-routing, drill, and back transitions are not implemented yet (host fetch goes to reader, no `list` field on appModel, etc.).

- [ ] **Step 3: Add the list field and wire routing/navigation in `app.go`**

Replace `/Users/jonathan/lookit/tui/app.go` with:

```go
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
```

- [ ] **Step 4: Run all tests and build**

```bash
cd /Users/jonathan/lookit
gofmt -w tui/
go test ./... -count=1
go build -o lookit .
```

Expected: all tests pass; build succeeds.

- [ ] **Step 5: Commit**

```bash
cd /Users/jonathan/lookit
git add tui/
git commit -m "feat(tui): open user list on host fetch and drill into users"
```

---

## Task 5: Docs and final verification

**Files:**
- Modify: `/Users/jonathan/lookit/README.md`

- [ ] **Step 1: Document the drill-down in the README**

In `/Users/jonathan/lookit/README.md`, find the existing TUI controls paragraph:

```markdown
In the TUI, type a target and press Enter to fetch it. Use arrows or PageUp/PageDown to scroll the response. Press Esc or Ctrl+C to quit.
```

Replace it with:

```markdown
In the TUI, type a target and press Enter to fetch it. Use arrows or PageUp/PageDown to scroll the response. Press Esc or Ctrl+C to quit.

Fingering a server (`@host`) that returns a list of users opens a selectable list: use the arrows to move, `/` to filter, and Enter to finger the highlighted user. Press Esc to go back to the list, and Esc again to return to the input.
```

- [ ] **Step 2: Run the full local CI dry-run**

```bash
cd /Users/jonathan/lookit
go vet ./...
test -z "$(gofmt -l .)" && echo "gofmt clean"
go test ./... -race
go build -o lookit .
```

Expected: `go vet` exits 0; `gofmt clean` prints; all race tests pass; binary builds.

- [ ] **Step 3: Manual TUI smoke test**

```bash
cd /Users/jonathan/lookit
./lookit
```

In the TUI:
1. Type `@tilde.team` and press Enter. Expected: a selectable user list opens with a title like `@tilde.team — N users`.
2. Move with arrows, press `/` and type to filter, press Esc to clear the filter.
3. Highlight a user and press Enter. Expected: the reader shows that user's finger response; status shows `loaded <login>@tilde.team`.
4. Press Esc. Expected: back to the user list.
5. Press Esc. Expected: back to the input (reader home).
6. Type `@tilde.town` and press Enter. Expected: a banner renders in the plain reader (no list — correctly declined).
7. Press Esc or Ctrl+C. Expected: quits cleanly.

- [ ] **Step 4: Commit the docs**

```bash
cd /Users/jonathan/lookit
git add README.md
git commit -m "docs: document TUI user-list drill-down"
```

- [ ] **Step 5: Final git check**

```bash
cd /Users/jonathan/lookit
git log --oneline -6
git status --short
```

Expected: recent commits use Conventional Commits; no commit message mentions Codex or co-author trailers; no tracked changes remain.

---

## Notes for the implementer

- **No co-author or Codex trailers** in commit messages (project convention from Phase 1/2).
- The `~/bubbletea`, `~/bubbles`, `~/lipgloss` clones are reference only; the versions in `go.mod` are authoritative. If a resolved API differs from a code block here (e.g. a list method name), adjust to the resolved module and keep the behavior described.
- `render/` and `finger/` are unchanged by this plan.
- Deferred (do NOT build): no-arg landing screen, in-viewport token-selection fallback, unicode-aware name columns. See the spec's "Deferred work".

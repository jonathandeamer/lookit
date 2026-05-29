# lookit Phase 2 TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Phase 2: `lookit` with no arguments opens a compact query-first TUI reader while existing one-shot target queries remain supported.

**Architecture:** Add a new `tui/` package that consumes `finger/` and `render/` without moving networking or rendering responsibilities. Keep `main.go` as process routing: no args opens the TUI, one target arg uses the existing one-shot path, `version` prints build info, and help flags print usage.

**Tech Stack:** Go, Bubble Tea v2 (`charm.land/bubbletea/v2`), Bubbles v2 (`charm.land/bubbles/v2` textinput/viewport/key), Lip Gloss v2 (`charm.land/lipgloss/v2`) for TUI chrome only, existing `github.com/charmbracelet/colorprofile`, existing Phase 1 Lip Gloss dependency for `render/`.

---

## File Structure

```
lookit/
├── main.go
├── main_test.go
├── finger/
│   └── ... existing unchanged
├── render/
│   ├── theme.go                 # unchanged in Phase 2
│   └── ... existing unchanged
└── tui/
    ├── fetch.go                 # FetchFunc, Entry, fetch command/result message
    ├── model.go                 # Bubble Tea model, update, view
    ├── styles.go                # compact TUI styles
    └── model_test.go            # injected-fetch model tests
```

## Shared Types And Interfaces

These names are fixed for all tasks:

```go
// tui/fetch.go
type Entry struct {
	Target finger.Target
	Body   []byte
	Meta   finger.Meta
	Err    error
}

type FetchFunc func(context.Context, finger.Target) ([]byte, finger.Meta, error)

// tui/model.go
type Model struct {
	input    textinput.Model
	viewport viewport.Model
	current  *Entry
	loading  bool
	status   string
	profile  colorprofile.Profile
	fetch    FetchFunc
	ready    bool
	width    int
	height   int
}
```

Do not add history, persistence, subscriptions, `get`, `--tui`, or a command framework in this phase.

---

## Task 1: Add Bubble Tea/Bubbles v2 dependencies

**Files:**
- Modify: `/Users/jonathan/lookit/go.mod`
- Modify: `/Users/jonathan/lookit/go.sum`

- [ ] **Step 1: Add the v2 Charm dependencies**

Run:

```bash
cd /Users/jonathan/lookit
go get charm.land/bubbletea/v2@latest
go get charm.land/bubbles/v2@latest
go get charm.land/lipgloss/v2@latest
```

Expected: `go.mod` adds the new v2 modules. Exact versions may differ.

- [ ] **Step 2: Tidy and test**

Run:

```bash
cd /Users/jonathan/lookit
go mod tidy
go test ./render/... -count=1
go test ./... -count=1
```

Expected: all tests pass.

Do not migrate `render/theme.go` to `charm.land/lipgloss/v2` in this task. Local review of `~/lipgloss/UPGRADE_GUIDE_V2.md` shows v2 removed `Renderer`, and the Phase 1 renderer currently relies on `lipgloss.NewRenderer(io.Discard)` for explicit profile handling. Keep `render/` stable and use Lip Gloss v2 only inside the new `tui/` package.

- [ ] **Step 3: Commit**

Run:

```bash
cd /Users/jonathan/lookit
git add go.mod go.sum
git commit -m "chore: adopt Charm v2 TUI dependencies"
```

---

## Task 2: Version output and testable CLI routing helper

**Files:**
- Modify: `/Users/jonathan/lookit/main.go`
- Modify: `/Users/jonathan/lookit/main_test.go`

This task adds `lookit version` and refactors process routing into a testable helper. `lookit` with no args still prints usage in this task; it will open the TUI in Task 6.

- [ ] **Step 1: Replace `main.go` with a testable router**

Replace `/Users/jonathan/lookit/main.go` with:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/charmbracelet/colorprofile"

	"github.com/jonathandeamer/lookit/finger"
	"github.com/jonathandeamer/lookit/render"
)

// Exit codes per sysexits.h-ish conventions.
const (
	exitOK      = 0
	exitNetwork = 2
	exitUsage   = 64 // EX_USAGE
)

var (
	version = "dev"
	builtAt = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] == "-h" || args[0] == "--help" {
		printUsage(stderr)
		return exitUsage
	}

	if args[0] == "version" {
		fmt.Fprintln(stdout, versionString())
		return exitOK
	}

	target, err := finger.ParseTarget(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "lookit: %v\n", err)
		return exitUsage
	}

	return runOneShot(context.Background(), target, stdout)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  lookit user@host[:port]")
	fmt.Fprintln(w, "  lookit @host[:port]")
	fmt.Fprintln(w, "  lookit version")
}

func versionString() string {
	return fmt.Sprintf("lookit %s (built %s)", version, builtAt)
}

func runOneShot(ctx context.Context, target finger.Target, stdout io.Writer) int {
	profile := colorprofile.Detect(os.Stdout, os.Environ())
	body, meta, queryErr := finger.Query(ctx, target)
	fmt.Fprint(stdout, render.Render(target, body, meta, queryErr, profile))
	if queryErr != nil {
		return exitCodeFor(queryErr)
	}
	return exitOK
}

// exitCodeFor maps Query errors to process exit codes. Network failures
// (refused, timeout, DNS) return 2; everything else returns 2 as well for now.
func exitCodeFor(err error) int {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return exitNetwork
	}
	return exitNetwork
}
```

- [ ] **Step 2: Replace `main_test.go`**

Replace `/Users/jonathan/lookit/main_test.go` with:

```go
package main

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"testing"
)

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "dns error",
			err:  &net.DNSError{Err: "no such host", Name: "example.invalid"},
			want: exitNetwork,
		},
		{
			name: "generic error",
			err:  errors.New("read failed"),
			want: exitNetwork,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCodeFor(tc.err); got != tc.want {
				t.Fatalf("exitCodeFor(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() {
		version, builtAt = oldVersion, oldBuiltAt
	})

	version = "0.2.0"
	builtAt = "2026-05-29"

	if got, want := versionString(), "lookit 0.2.0 (built 2026-05-29)"; got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}

func TestRunVersion(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() {
		version, builtAt = oldVersion, oldBuiltAt
	})
	version = "dev"
	builtAt = "unknown"

	var stdout, stderr bytes.Buffer
	code := run([]string{"version"}, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if got, want := stdout.String(), "lookit dev (built unknown)\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "lookit version") {
		t.Fatalf("stderr usage missing version command: %q", stderr.String())
	}
}

func TestRunInvalidTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"just-a-name"}, &stdout, &stderr)

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "lookit:") {
		t.Fatalf("stderr = %q, want lookit error", stderr.String())
	}
}
```

- [ ] **Step 3: Run tests**

Run:

```bash
cd /Users/jonathan/lookit
gofmt -w main.go main_test.go
go test ./... -count=1
go build -o lookit .
./lookit version
```

Expected:

```text
lookit dev (built unknown)
```

- [ ] **Step 4: Commit**

Run:

```bash
cd /Users/jonathan/lookit
git add main.go main_test.go
git commit -m "feat(main): add version output and testable routing"
```

---

## Task 3: TUI model skeleton, input, invalid target, and quit keys

**Files:**
- Create: `/Users/jonathan/lookit/tui/fetch.go`
- Create: `/Users/jonathan/lookit/tui/styles.go`
- Create: `/Users/jonathan/lookit/tui/model.go`
- Create: `/Users/jonathan/lookit/tui/model_test.go`

- [ ] **Step 1: Create initial tests**

Create `/Users/jonathan/lookit/tui/model_test.go`:

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

func stubFetch(t *testing.T) FetchFunc {
	t.Helper()
	return func(context.Context, finger.Target) ([]byte, finger.Meta, error) {
		t.Fatalf("fetch should not be called")
		return nil, finger.Meta{}, nil
	}
}

func TestNewModelInitialState(t *testing.T) {
	m := New(stubFetch(t), colorprofile.NoTTY)

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

func TestInvalidEnterSetsStatusError(t *testing.T) {
	m := New(stubFetch(t), colorprofile.NoTTY)
	m.input.SetValue("not-a-target")

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(Model)

	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil for invalid input", cmd)
	}
	if got.loading {
		t.Fatalf("loading = true, want false")
	}
	if !strings.Contains(got.status, "error:") {
		t.Fatalf("status = %q, want error", got.status)
	}
}

func TestQuitKeysReturnCommand(t *testing.T) {
	for _, msg := range []tea.Msg{
		tea.KeyPressMsg{Code: tea.KeyEsc},
		tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl},
	} {
		m := New(stubFetch(t), colorprofile.NoTTY)
		_, cmd := m.Update(msg)
		if cmd == nil {
			t.Fatalf("Update(%#v) returned nil cmd, want quit cmd", msg)
		}
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
cd /Users/jonathan/lookit
go test ./tui/... -count=1
```

Expected: build failure because package `tui` and `New` do not exist yet.

- [ ] **Step 3: Create `fetch.go`**

Create `/Users/jonathan/lookit/tui/fetch.go`:

```go
package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/jonathandeamer/lookit/finger"
)

type Entry struct {
	Target finger.Target
	Body   []byte
	Meta   finger.Meta
	Err    error
}

type FetchFunc func(context.Context, finger.Target) ([]byte, finger.Meta, error)

type fetchResultMsg struct {
	entry Entry
}

func defaultFetch(ctx context.Context, target finger.Target) ([]byte, finger.Meta, error) {
	return finger.Query(ctx, target)
}

func fetchCmd(ctx context.Context, fetch FetchFunc, target finger.Target) tea.Cmd {
	return func() tea.Msg {
		body, meta, err := fetch(ctx, target)
		return fetchResultMsg{
			entry: Entry{
				Target: target,
				Body:   body,
				Meta:   meta,
				Err:    err,
			},
		}
	}
}
```

- [ ] **Step 4: Create `styles.go`**

Create `/Users/jonathan/lookit/tui/styles.go`:

```go
package tui

import "charm.land/lipgloss/v2"

type styles struct {
	title  lipgloss.Style
	status lipgloss.Style
	error  lipgloss.Style
	hint   lipgloss.Style
}

func newStyles() styles {
	return styles{
		title:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff6fd5")),
		status: lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")),
		error:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b6b")),
		hint:   lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")),
	}
}
```

- [ ] **Step 5: Create `model.go`**

Create `/Users/jonathan/lookit/tui/model.go`:

```go
package tui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
	"github.com/jonathandeamer/lookit/render"
)

const initialStatus = "enter a finger target, then press Enter"

type Model struct {
	input    textinput.Model
	viewport viewport.Model
	current  *Entry
	loading  bool
	status   string
	profile  colorprofile.Profile
	fetch    FetchFunc
	ready    bool
	width    int
	height   int
	styles   styles
}

func New(fetch FetchFunc, profile colorprofile.Profile) Model {
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

	return Model{
		input:    input,
		viewport: vp,
		status:   initialStatus,
		profile:  profile,
		fetch:    fetch,
		styles:   newStyles(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		tea.RequestCapability("RGB"),
		tea.RequestCapability("Tc"),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.Key()
		switch {
		case key.Code == tea.KeyEsc || (key.Code == 'c' && key.Mod == tea.ModCtrl):
			return m, tea.Quit
		case key.Code == tea.KeyEnter:
			if m.loading {
				return m, nil
			}
			target, err := finger.ParseTarget(strings.TrimSpace(m.input.Value()))
			if err != nil {
				m.status = "error: " + err.Error()
				return m, nil
			}
			m.loading = true
			m.status = "loading " + target.Raw + "..."
			return m, fetchCmd(context.Background(), m.fetch, target)
		}
	case fetchResultMsg:
		m.loading = false
		m.current = &msg.entry
		m.status = statusForEntry(msg.entry)
		m.viewport.SetContent(renderEntry(m.profile, msg.entry))
		return m, nil
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m Model) View() tea.View {
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
	b.WriteString(m.styles.hint.Render("Enter fetches · arrows/PageUp/PageDown scroll · Esc quits · ? help"))
	v := tea.NewView(b.String())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
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

- [ ] **Step 6: Run tests**

Run:

```bash
cd /Users/jonathan/lookit
gofmt -w tui/
go test ./tui/... -count=1
go test ./... -count=1
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

Run:

```bash
cd /Users/jonathan/lookit
git add tui/
git commit -m "feat(tui): add query reader model skeleton"
```

---

## Task 4: Async fetch success, error, and duplicate submit behavior

**Files:**
- Modify: `/Users/jonathan/lookit/tui/model_test.go`
- Modify: `/Users/jonathan/lookit/tui/model.go`
- Modify: `/Users/jonathan/lookit/tui/fetch.go`

- [ ] **Step 1: Add fetch behavior tests**

Append to `/Users/jonathan/lookit/tui/model_test.go`:

```go
func TestValidEnterStartsFetch(t *testing.T) {
	calls := 0
	fetch := func(context.Context, finger.Target) ([]byte, finger.Meta, error) {
		calls++
		return []byte("Login: alice\n"), finger.Meta{Addr: "plan.cat:79", Bytes: 13}, nil
	}

	m := New(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@plan.cat")

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(Model)

	if !got.loading {
		t.Fatalf("loading = false, want true")
	}
	if cmd == nil {
		t.Fatalf("cmd = nil, want fetch command")
	}
	msg := cmd()
	if _, ok := msg.(fetchResultMsg); !ok {
		t.Fatalf("cmd returned %T, want fetchResultMsg", msg)
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls)
	}
}

func TestDuplicateEnterWhileLoadingDoesNotFetch(t *testing.T) {
	calls := 0
	fetch := func(context.Context, finger.Target) ([]byte, finger.Meta, error) {
		calls++
		return nil, finger.Meta{}, nil
	}

	m := New(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@plan.cat")
	m.loading = true

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil while loading", cmd)
	}
	if calls != 0 {
		t.Fatalf("fetch calls = %d, want 0", calls)
	}
}

func TestFetchSuccessUpdatesCurrentAndViewport(t *testing.T) {
	m := New(stubFetch(t), colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	msg := fetchResultMsg{entry: Entry{
		Target: target,
		Body:   []byte("Login: alice\n"),
		Meta:   finger.Meta{Addr: target.HostPort, Bytes: len("Login: alice\n")},
	}}

	next, cmd := m.Update(msg)
	got := next.(Model)

	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil", cmd)
	}
	if got.loading {
		t.Fatalf("loading = true, want false")
	}
	if got.current == nil || got.current.Target.Raw != "alice@plan.cat" {
		t.Fatalf("current = %#v, want alice entry", got.current)
	}
	if !strings.Contains(got.viewport.View(), "Login: alice") {
		t.Fatalf("viewport content missing body: %q", got.viewport.View())
	}
}

func TestFetchErrorUpdatesCurrentAndViewport(t *testing.T) {
	m := New(stubFetch(t), colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	msg := fetchResultMsg{entry: Entry{
		Target: target,
		Meta:   finger.Meta{Addr: target.HostPort},
		Err:    errors.New("dial failed"),
	}}

	next, _ := m.Update(msg)
	got := next.(Model)

	if got.loading {
		t.Fatalf("loading = true, want false")
	}
	if got.current == nil || got.current.Err == nil {
		t.Fatalf("current = %#v, want error entry", got.current)
	}
	if !strings.Contains(got.viewport.View(), "dial failed") {
		t.Fatalf("viewport content missing error: %q", got.viewport.View())
	}
}
```

Add `errors` to the import block:

```go
	"errors"
```

- [ ] **Step 2: Run tests**

Run:

```bash
cd /Users/jonathan/lookit
go test ./tui/... -count=1
```

Expected: tests may already pass if Task 3 implemented fetch handling exactly. If they fail, continue.

- [ ] **Step 3: Ensure fetch handling exists**

In `/Users/jonathan/lookit/tui/model.go`, verify the Enter case and `fetchResultMsg` case match this behavior:

```go
case key.Code == tea.KeyEnter:
	if m.loading {
		return m, nil
	}
	target, err := finger.ParseTarget(strings.TrimSpace(m.input.Value()))
	if err != nil {
		m.status = "error: " + err.Error()
		return m, nil
	}
	m.loading = true
	m.status = "loading " + target.Raw + "..."
	return m, fetchCmd(context.Background(), m.fetch, target)
case fetchResultMsg:
	m.loading = false
	m.current = &msg.entry
	m.status = statusForEntry(msg.entry)
	m.viewport.SetContent(renderEntry(m.profile, msg.entry))
	return m, nil
```

- [ ] **Step 4: Run all tests**

Run:

```bash
cd /Users/jonathan/lookit
gofmt -w tui/
go test ./tui/... -count=1
go test ./... -count=1
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

Run:

```bash
cd /Users/jonathan/lookit
git add tui/
git commit -m "feat(tui): fetch targets asynchronously"
```

---

## Task 5: Viewport sizing, color profile message, and compact view

**Files:**
- Modify: `/Users/jonathan/lookit/tui/model.go`
- Modify: `/Users/jonathan/lookit/tui/model_test.go`

- [ ] **Step 1: Add layout and color-profile tests**

Append to `/Users/jonathan/lookit/tui/model_test.go`:

```go
func TestWindowSizeUpdatesViewport(t *testing.T) {
	m := New(stubFetch(t), colorprofile.NoTTY)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	got := next.(Model)

	if !got.ready {
		t.Fatalf("ready = false, want true")
	}
	if got.width != 100 || got.height != 30 {
		t.Fatalf("size = %dx%d, want 100x30", got.width, got.height)
	}
	if got.viewport.Width() != 100 {
		t.Fatalf("viewport width = %d, want 100", got.viewport.Width())
	}
	if got.viewport.Height() != 26 {
		t.Fatalf("viewport height = %d, want 26", got.viewport.Height())
	}
}

func TestColorProfileMessageUpdatesProfile(t *testing.T) {
	m := New(stubFetch(t), colorprofile.NoTTY)

	next, _ := m.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	got := next.(Model)

	if got.profile != colorprofile.TrueColor {
		t.Fatalf("profile = %v, want TrueColor", got.profile)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd /Users/jonathan/lookit
go test ./tui/... -count=1
```

Expected: layout test fails until `tea.WindowSizeMsg` handling is added.

- [ ] **Step 3: Add resize and color-profile handling**

In `/Users/jonathan/lookit/tui/model.go`, add these helpers:

```go
const chromeRows = 4

func (m Model) resize(width, height int) Model {
	m.ready = true
	m.width = width
	m.height = height
	m.input.SetWidth(max(20, width-len(m.input.Prompt)))
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(max(1, height-chromeRows))
	return m
}
```

Add these cases to `Update` before the key handling:

```go
case tea.WindowSizeMsg:
	m = m.resize(msg.Width, msg.Height)
	return m, nil
case tea.ColorProfileMsg:
	m.profile = msg.Profile
	if m.current != nil {
		m.viewport.SetContent(renderEntry(m.profile, *m.current))
	}
	return m, nil
```

- [ ] **Step 4: Ensure compact view uses available width**

Update `View()` in `/Users/jonathan/lookit/tui/model.go` so the title and status rows remain compact:

```go
func (m Model) View() tea.View {
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
	b.WriteString(m.styles.hint.Render("Enter fetches · arrows/PageUp/PageDown scroll · Esc quits · ? help"))
	v := tea.NewView(b.String())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
```

No full-screen border, panels, or sidebar should be added. `AltScreen` and `MouseModeCellMotion` are Bubble Tea terminal-mode declarations, not visual chrome. The alt screen keeps the shell clean on exit, and cell-motion mouse mode lets the Bubbles viewport receive mouse-wheel events.

- [ ] **Step 5: Run all tests**

Run:

```bash
cd /Users/jonathan/lookit
gofmt -w tui/
go test ./tui/... -count=1
go test ./... -count=1
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

Run:

```bash
cd /Users/jonathan/lookit
git add tui/
git commit -m "feat(tui): size viewport and track color profile"
```

---

## Task 6: Wire `lookit` no-arg TUI route

**Files:**
- Modify: `/Users/jonathan/lookit/main.go`
- Modify: `/Users/jonathan/lookit/main_test.go`
- Create: `/Users/jonathan/lookit/tui/run.go`

- [ ] **Step 1: Add test coverage for no-arg routing**

Append to `/Users/jonathan/lookit/main_test.go`:

```go
func TestRunNoArgsStartsTUI(t *testing.T) {
	oldStartTUI := startTUI
	t.Cleanup(func() {
		startTUI = oldStartTUI
	})

	called := false
	startTUI = func() error {
		called = true
		return nil
	}

	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !called {
		t.Fatalf("startTUI was not called")
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q, want both empty", stdout.String(), stderr.String())
	}
}

func TestRunNoArgsTUIFailure(t *testing.T) {
	oldStartTUI := startTUI
	t.Cleanup(func() {
		startTUI = oldStartTUI
	})

	startTUI = func() error {
		return errors.New("terminal unavailable")
	}

	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)

	if code != exitNetwork {
		t.Fatalf("exit code = %d, want %d", code, exitNetwork)
	}
	if !strings.Contains(stderr.String(), "terminal unavailable") {
		t.Fatalf("stderr = %q, want TUI error", stderr.String())
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd /Users/jonathan/lookit
go test . -count=1
```

Expected: build failure because `startTUI` does not exist.

- [ ] **Step 3: Create `tui/run.go`**

Create `/Users/jonathan/lookit/tui/run.go`:

```go
package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

func Run(ctx context.Context, profile colorprofile.Profile) error {
	program := tea.NewProgram(New(defaultFetch, profile))
	_ = ctx
	_, err := program.Run()
	return err
}
```

Bubble Tea v2.0.6 `Program.Run` does not take a context. The exported `Run(ctx, profile)` signature still accepts one so future cancellation can be added without changing `main.go`; this first implementation explicitly assigns `_ = ctx`.

- [ ] **Step 4: Wire main no-arg route**

In `/Users/jonathan/lookit/main.go`, add the `tui` import:

```go
	"github.com/jonathandeamer/lookit/tui"
```

Add this package variable near `version`:

```go
var startTUI = func() error {
	profile := colorprofile.Detect(os.Stdout, os.Environ())
	return tui.Run(context.Background(), profile)
}
```

Change the start of `run` to:

```go
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		if err := startTUI(); err != nil {
			fmt.Fprintf(stderr, "lookit: %v\n", err)
			return exitNetwork
		}
		return exitOK
	}
	if len(args) != 1 || args[0] == "-h" || args[0] == "--help" {
		printUsage(stderr)
		return exitUsage
	}
	// rest unchanged
}
```

Update usage text in `printUsage`:

```go
func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  lookit")
	fmt.Fprintln(w, "  lookit user@host[:port]")
	fmt.Fprintln(w, "  lookit @host[:port]")
	fmt.Fprintln(w, "  lookit version")
}
```

- [ ] **Step 5: Run tests and build**

Run:

```bash
cd /Users/jonathan/lookit
gofmt -w main.go main_test.go tui/
go test ./... -count=1
go build -o lookit .
./lookit --help
./lookit version
```

Expected:

- Tests pass.
- Build succeeds.
- Help output includes bare `lookit`.
- Version output remains `lookit dev (built unknown)`.

- [ ] **Step 6: Manual TUI smoke**

Run:

```bash
cd /Users/jonathan/lookit
./lookit
```

Expected: TUI opens with focused input, compact status, empty viewport, and quit works with Esc or Ctrl+C.

- [ ] **Step 7: Commit**

Run:

```bash
cd /Users/jonathan/lookit
git add main.go main_test.go tui/run.go
git commit -m "feat(main): open TUI when no target is provided"
```

---

## Task 7: README Phase 2 usage and controls

**Files:**
- Modify: `/Users/jonathan/lookit/README.md`

- [ ] **Step 1: Update usage section**

In `/Users/jonathan/lookit/README.md`, replace the current usage block with:

```markdown
```bash
lookit                    # open the TUI reader
lookit alice@plan.cat     # finger a user once
lookit @tilde.team        # finger a server once (banner + user list)
lookit alice@example.com:79 # explicit port
lookit version            # print version/build info
```
```

- [ ] **Step 2: Add TUI controls**

Below the usage block, add:

```markdown
In the TUI, type a target and press Enter to fetch it. Use arrows or PageUp/PageDown to scroll the response. Press Esc or Ctrl+C to quit.
```

Do not document `get`, `--tui`, `subscribe`, `refresh`, or `discover` as current behavior.

- [ ] **Step 3: Run checks**

Run:

```bash
cd /Users/jonathan/lookit
go test ./... -count=1
```

Expected: tests pass.

- [ ] **Step 4: Commit**

Run:

```bash
cd /Users/jonathan/lookit
git add README.md
git commit -m "docs: document TUI usage and controls"
```

---

## Final verification

- [ ] **Step 1: Full local CI dry-run**

Run:

```bash
cd /Users/jonathan/lookit
go vet ./...
test -z "$(gofmt -l .)" && echo "gofmt clean"
go test ./... -race -v
go build -o lookit .
```

Expected:

- `go vet` exits 0.
- `gofmt clean` prints.
- all race tests pass.
- binary builds.

- [ ] **Step 2: CLI smoke tests**

Run:

```bash
cd /Users/jonathan/lookit
./lookit version
./lookit --help
./lookit just-a-name; echo "exit code: $?"
```

Expected:

- version prints `lookit dev (built unknown)`.
- help includes bare `lookit`.
- invalid target exits 64.

- [ ] **Step 3: TUI manual smoke**

Run:

```bash
cd /Users/jonathan/lookit
./lookit
```

Expected:

- compact TUI opens.
- input is focused.
- typing a target and pressing Enter starts loading.
- response appears in scrollable viewport.
- Esc and Ctrl+C quit.

- [ ] **Step 4: Git log and status**

Run:

```bash
cd /Users/jonathan/lookit
git log --oneline -12
git status --short
```

Expected:

- recent commits use Conventional Commits.
- no commit message mentions Codex or co-author trailers.
- no tracked changes remain.

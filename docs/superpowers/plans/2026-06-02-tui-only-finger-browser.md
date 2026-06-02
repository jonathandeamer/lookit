# TUI-only Finger Browser Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse lookit from a dual-mode tool (one-shot CLI + TUI) into a single interactive finger browser whose command-line argument seeds the TUI.

**Architecture:** `main.go` becomes a thin launcher (recognises `-h`/`--help` and `-v`/`--version`, otherwise launches the TUI with an optional initial-query string). The raw argument is replayed through the TUI's existing `submit()` path, so "typed" and "seeded" share one code path and one `finger.ParseTarget` chokepoint. The one-shot render path and its footer retire; version info moves to a `-v`/`--version` flag plus a band on the `?` help panel.

**Tech Stack:** Go, Bubble Tea v2 (`charm.land/bubbletea/v2`), Lip Gloss v2 (`charm.land/lipgloss/v2`) in `tui/`, Lip Gloss v1 in `render/`, `colorprofile`.

**Spec:** `docs/superpowers/specs/2026-06-02-tui-only-finger-browser-design.md`

---

## File Structure

- `render/render.go` — drop the footer option/wrapper machinery; `RenderWithBackground` becomes footerless. `Render` (background-detecting wrapper) **stays** (used by tests).
- `render/chrome.go` — delete the now-unused `renderFooter` and `fmtBytes`.
- `render/render_test.go` — delete the footer-specific test; regenerate goldens.
- `render/testdata/*.golden` — regenerated (lose the footer line).
- `tui/reader.go` — drop the `render.WithoutFooter()` argument.
- `render/cli.go` — `Usage` text: replace the `version` subcommand line with `--version`, add a "press ? in lookit for keys" pointer.
- `render/cli_test.go` — update the `Usage` assertions.
- `tui/run.go` — add exported `Options{InitialQuery, Version}`; `Run` takes `Options`; `newAppWithOptions` constructor.
- `tui/app.go` — `commonModel.version` field; `seedSubmitMsg`; `Init` emits it when seeded; `Update` handles it; `helpView` grows a version/tagline band; `newApp` delegates to `newAppWithOptions`.
- `tui/app_test.go` — seeded-valid, seeded-invalid, and help-band tests.
- `main.go` — rewrite `run` (flag scan + seed + `{0,1}` exit codes); delete `runOneShot`/`runOneShotFunc`/`exitCodeFor`/`version` subcommand/`exitNetwork`/`exitUsage`; `startTUI` seam takes `tui.Options`.
- `main_test.go` — rewrite for the new router.

---

## Task 1: Make `render` footerless

**Files:**
- Modify: `render/render.go`
- Modify: `render/chrome.go`
- Modify: `tui/reader.go:86-90`
- Modify: `render/render_test.go:61-75`
- Regenerate: `render/testdata/*.golden`

- [ ] **Step 1: Delete the footer test**

In `render/render_test.go`, delete the entire `TestRenderWithoutFooterOmitsStats` function (lines 61–75, from `func TestRenderWithoutFooterOmitsStats(t *testing.T) {` through its closing `}`). It is the only test referencing `WithoutFooter` and the only remaining caller of `fmtBytes` from outside `chrome.go`.

- [ ] **Step 2: Make `RenderWithBackground` footerless**

In `render/render.go`, replace the `Option`/`options`/`WithoutFooter` block and the `RenderWithBackground` function with the versions below. Keep `Render` (it is used by tests); it now calls `RenderWithBackground` with no options.

Replace lines 11–66 (from `// Option tunes RenderWithBackground.` through the closing of the `if o.footer { ... }` block) so the file reads:

```go
// Render formats a finger query result for the requested terminal color
// profile, using Lip Gloss v1's standalone background detection.
func Render(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile) string {
	return RenderWithBackground(t, body, meta, queryErr, profile, lipgloss.HasDarkBackground())
}

// RenderWithBackground formats a finger query result for a known terminal
// background mode. The TUI uses this so tea.BackgroundColorMsg can restyle a
// live session deterministically. It is footerless: the one-shot CLI that owned
// the "bytes · elapsed" footer is gone, and the TUI surfaces byte count and
// truncation in its own status bar.
func RenderWithBackground(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile, darkBackground bool) string {
	theme := NewThemeWithBackground(profile, darkBackground)
	var sb strings.Builder

	success := queryErr == nil
	sb.WriteString(renderHeader(theme, t, meta, success))

	if len(body) == 0 && success {
		sb.WriteString(theme.Footer.Render("(no response body)"))
		sb.WriteByte('\n')
	} else {
		if isTildeTeam(t) {
			body = reflowPronouns(body)
		}
		sb.WriteString(highlightFields(theme, body, extraFieldPrefixes(t)))
		if len(body) > 0 && body[len(body)-1] != '\n' {
			sb.WriteByte('\n')
		}
	}

	if queryErr != nil {
		sb.WriteString(theme.ErrLine.Render(queryErr.Error()))
		sb.WriteByte('\n')
	}

	return sb.String()
}
```

(The `import` block keeps `strings`, `colorprofile`, `lipgloss`, and `finger` — `lipgloss` is still used by `Render`.)

- [ ] **Step 3: Delete the now-unused footer helpers**

In `render/chrome.go`, delete the `renderFooter` function (lines 23–31, `func renderFooter(...) string { ... }`) and the `fmtBytes` function (lines 42–51, `func fmtBytes(n int) string { ... }`). Keep `renderHeader` and `fmtElapsed` (both still used by `renderHeader`). After deletion, verify `fmt` is still imported/used (it is — `fmtElapsed` uses it).

- [ ] **Step 4: Drop `WithoutFooter` from the reader**

In `tui/reader.go`, change `renderEntry` (lines 86–90) to:

```go
func renderEntry(profile colorprofile.Profile, darkBackground bool, entry Entry) string {
	// The status bar pins the byte count and truncation flag and the header
	// carries the elapsed time, so render itself is footerless.
	return render.RenderWithBackground(entry.Target, entry.Body, entry.Meta, entry.Err, profile, darkBackground)
}
```

- [ ] **Step 5: Verify it compiles and the footer-dependent code is gone**

Run: `go build ./... 2>&1`
Expected: builds with no errors (no "undefined: WithoutFooter", no "declared and not used").

- [ ] **Step 6: Regenerate the golden files**

Run: `go test ./render/ -update`
Expected: PASS, with `--- updated golden ...` log lines.

Then inspect the diff: `git diff render/testdata`
Expected: every changed golden loses exactly its trailing footer line (the `NNN B · NNNms` stats line, and in `truncated.truecolor.golden` the `truncated` notice that lived in the footer). No other lines change.

- [ ] **Step 7: Run the render tests**

Run: `go test ./render/ ./tui/ -count=1`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add render/render.go render/chrome.go render/render_test.go render/testdata tui/reader.go
git commit -m "refactor(render): drop one-shot footer path"
```

---

## Task 2: Update CLI `Usage` text

**Files:**
- Modify: `render/cli.go:14-24`
- Modify: `render/cli_test.go`

- [ ] **Step 1: Update the expected usage string**

`render/cli_test.go` defines a shared `plainUsage` const (lines 11–15) that both `TestUsagePlainIsByteIdentical` and `TestUsageStyledKeepsTextAddsAnsi` compare against. Update that one const to the new block:

```go
const plainUsage = "usage:\n" +
	"  lookit\n" +
	"  lookit user@host[:port]\n" +
	"  lookit @host[:port]\n" +
	"  lookit --version\n" +
	"\n" +
	"press ? in lookit for keys\n"
```

Both existing tests now assert the new text automatically — no other edits to `cli_test.go`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./render/ -run Usage -count=1 -v`
Expected: FAIL (current `Usage` still emits the `version` subcommand line and no pointer line).

- [ ] **Step 3: Update `Usage`**

In `render/cli.go`, replace the `Usage` function (lines 14–24) with:

```go
func Usage(profile colorprofile.Profile) string {
	t := NewTheme(profile)
	cmd := t.Target.Render("lookit")
	var b strings.Builder
	fmt.Fprintln(&b, t.Footer.Render("usage:"))
	fmt.Fprintf(&b, "  %s\n", cmd)
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Field.Render("user@host[:port]"))
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Field.Render("@host[:port]"))
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Footer.Render("--version"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, t.Footer.Render("press ? in lookit for keys"))
	return b.String()
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./render/ -run Usage -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add render/cli.go render/cli_test.go
git commit -m "feat(cli): point usage at --version and the ? help"
```

---

## Task 3: Entry surface — TUI Options, seeding, and CLI router

This task changes `tui.Run`'s signature, so `tui/run.go`, `tui/app.go`, `main.go`, and `main_test.go` are updated together to keep the build green at the single commit.

**Files:**
- Modify: `tui/run.go`
- Modify: `tui/app.go` (`commonModel`, `newApp`, add `newAppWithOptions`, `seedSubmitMsg`, `Init`, `Update`)
- Modify: `tui/app_test.go` (add seed tests)
- Modify: `main.go`
- Modify: `main_test.go`

- [ ] **Step 1: Write the seeded-valid and seeded-invalid TUI tests**

Add to `tui/app_test.go`:

```go
// collectMsgs runs a command (recursing into batches) and returns every
// non-batch message produced. Safe for Init's commands: textinput.Blink and the
// capability requests all return their message immediately (no timers).
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	var out []tea.Msg
	if cmd == nil {
		return out
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			out = append(out, collectMsgs(c)...)
		}
		return out
	}
	if msg != nil {
		out = append(out, msg)
	}
	return out
}

func hasSeedSubmit(msgs []tea.Msg) bool {
	for _, msg := range msgs {
		if _, ok := msg.(seedSubmitMsg); ok {
			return true
		}
	}
	return false
}

func TestSeededInitEmitsSeedSubmit(t *testing.T) {
	m := newAppWithOptions(stubFetch(t), colorprofile.NoTTY, Options{InitialQuery: "alice@plan.cat", Seed: true})
	if !hasSeedSubmit(collectMsgs(m.Init())) {
		t.Fatal("Init() did not emit seedSubmitMsg when a query was seeded")
	}
}

func TestBlankSeedStillEmitsSeedSubmit(t *testing.T) {
	// lookit "" / lookit "   ": an arg was supplied, so it must still be replayed.
	m := newAppWithOptions(stubFetch(t), colorprofile.NoTTY, Options{InitialQuery: "   ", Seed: true})
	if !hasSeedSubmit(collectMsgs(m.Init())) {
		t.Fatal("Init() did not emit seedSubmitMsg for a supplied-but-blank arg")
	}
}

func TestUnseededInitOmitsSeedSubmit(t *testing.T) {
	m := newAppWithOptions(stubFetch(t), colorprofile.NoTTY, Options{})
	if hasSeedSubmit(collectMsgs(m.Init())) {
		t.Fatal("Init() emitted seedSubmitMsg without a seed")
	}
}

func TestSeededValidQueryFetchesAndRoutesToReader(t *testing.T) {
	fetch, seen := fetchRecorder("Plan: hi\n")
	m := newAppWithOptions(fetch, colorprofile.NoTTY, Options{InitialQuery: "alice@plan.cat", Seed: true})

	next, cmd := m.Update(seedSubmitMsg{})
	got := next.(appModel)
	if !got.loading {
		t.Fatalf("after seed submit: loading=false, want true")
	}
	if cmd == nil {
		t.Fatal("seed submit cmd = nil, want a fetch command")
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != "alice@plan.cat" {
		t.Fatalf("fetched targets = %v, want [alice@plan.cat]", *seen)
	}

	landed, _ := got.Update(fetchResultMsg{reqID: got.reqSeq, entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
	if landed.(appModel).state != stateReader {
		t.Fatalf("state = %d, want stateReader", landed.(appModel).state)
	}
}

func TestSeededInvalidQueryShowsErrorOnLanding(t *testing.T) {
	m := newAppWithOptions(stubFetch(t), colorprofile.NoTTY, Options{InitialQuery: "just-a-name", Seed: true})

	next, cmd := m.Update(seedSubmitMsg{})
	got := next.(appModel)

	if got.loading {
		t.Fatalf("invalid seed: loading=true, want false")
	}
	if cmd != nil {
		t.Fatalf("invalid seed: cmd != nil, want nil (no fetch)")
	}
	if !got.inputFocused {
		t.Fatalf("invalid seed: inputFocused=false, want true")
	}
	if !strings.Contains(got.flash, "error") {
		t.Fatalf("invalid seed: flash=%q, want it to contain \"error\"", got.flash)
	}
	if got.input.Value() != "just-a-name" {
		t.Fatalf("invalid seed: input=%q, want it to retain \"just-a-name\"", got.input.Value())
	}
}
```

(`hostTarget`, `fetchRecorder`, `stubFetch`, and `runCmds` already exist in `tui/app_test.go`.)

- [ ] **Step 2: Run the tests to verify they fail (compile error)**

Run: `go test ./tui/ -run Seed -count=1 -v`
Expected: FAIL — build error, `undefined: newAppWithOptions`, `undefined: Options`, `undefined: seedSubmitMsg`.

- [ ] **Step 3: Add `Options`, `newAppWithOptions`, and the new `Run` signature in `tui/run.go`**

Replace the body of `tui/run.go` with:

```go
package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// Options configures a TUI session launched from the command line.
type Options struct {
	// InitialQuery is the raw positional argument, replayed through the landing
	// input's submit path on startup (the same path a typed target takes). It is
	// only meaningful when Seed is true.
	InitialQuery string
	// Seed reports whether a positional argument was supplied at all. It is
	// tracked separately from InitialQuery because a supplied-but-blank argument
	// (lookit "" / lookit "   ") has an empty InitialQuery yet must still be
	// replayed, so the user gets the same parse-error-in-place behaviour as a
	// malformed target rather than a silent landing.
	Seed bool
	// Version is the build version line surfaced in the ? help panel.
	Version string
}

// Run starts the interactive TUI and blocks until the user quits.
//
// Bubble Tea v2's Program.Run does not take a context. The ctx parameter is
// accepted now so cancellation can be wired in later without changing main.go;
// this implementation does not yet use it.
func Run(ctx context.Context, profile colorprofile.Profile, opts Options) error {
	_ = ctx
	program := tea.NewProgram(newAppWithOptions(defaultFetch, profile, opts))
	_, err := program.Run()
	return err
}
```

- [ ] **Step 4: Add the `version`/`seeded` fields and rework the constructors in `tui/app.go`**

In `tui/app.go`, add a `version` field to `commonModel` (after `fetch FetchFunc` at line 58):

```go
	fetch          FetchFunc
	version        string
```

Add a `seeded` field to `appModel` (after `inputFocused bool` near line 98):

```go
	input        textinput.Model
	inputFocused bool
	seeded       bool // a CLI positional arg was supplied; replay it on Init
```

Then replace the existing `newApp` function (lines 116–150) with a delegating wrapper plus the real constructor:

```go
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
```

- [ ] **Step 5: Add `seedSubmitMsg`, emit it from `Init`, and handle it in `Update`**

In `tui/app.go`, add the message type near the other message types (e.g. just above `clearFlashMsg` at line 576):

```go
// seedSubmitMsg replays a command-line initial query through submit() on
// startup, so a seeded target takes the exact path a typed one does.
type seedSubmitMsg struct{}
```

Replace `Init` (lines 325–332) with:

```go
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
```

In `Update`, add a case alongside the others (e.g. after the `clearFlashMsg` case at line 378–380):

```go
	case seedSubmitMsg:
		cmd := m.submit()
		return m, cmd
```

- [ ] **Step 6: Run the TUI seed tests**

Run: `go test ./tui/ -run Seed -count=1 -v`
Expected: PASS.

- [ ] **Step 7: Confirm the rest of the TUI suite still passes**

Run: `go test ./tui/ -count=1`
Expected: PASS (the `newApp(...)` wrapper preserves every existing call site).

- [ ] **Step 8: Write the new `main_test.go`**

Replace the entire contents of `main_test.go` with:

```go
package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/jonathandeamer/lookit/tui"
)

// pinProfile forces detectProfile to a fixed profile for the duration of a
// test, so CLI-text assertions don't depend on the ambient environment.
func pinProfile(t *testing.T, p colorprofile.Profile) {
	t.Helper()
	old := detectProfile
	t.Cleanup(func() { detectProfile = old })
	detectProfile = func(io.Writer, []string) colorprofile.Profile { return p }
}

// stubStartTUI replaces the startTUI seam, recording the options it was called
// with and returning err.
func stubStartTUI(t *testing.T, err error) *tui.Options {
	t.Helper()
	old := startTUI
	t.Cleanup(func() { startTUI = old })
	var got tui.Options
	startTUI = func(opts tui.Options) error {
		got = opts
		return err
	}
	return &got
}

func TestVersionString(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() { version, builtAt = oldVersion, oldBuiltAt })
	version = "0.2.0"
	builtAt = "2026-05-29"
	if got, want := versionString(), "lookit 0.2.0 (built 2026-05-29)"; got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}

func TestRunHelp(t *testing.T) {
	pinProfile(t, colorprofile.NoTTY)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout.String(), "usage:") {
		t.Fatalf("stdout = %q, want usage block", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunVersionFlag(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() { version, builtAt = oldVersion, oldBuiltAt })
	version = "dev"
	builtAt = "unknown"
	pinProfile(t, colorprofile.NoTTY)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--version"}, &stdout, &stderr)
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

func TestRunVersionFlagStyled(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() { version, builtAt = oldVersion, oldBuiltAt })
	version = "dev"
	builtAt = "unknown"
	pinProfile(t, colorprofile.TrueColor)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-v"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("styled version has no ANSI: %q", stdout.String())
	}
	if got := ansi.Strip(stdout.String()); got != "lookit dev (built unknown)\n" {
		t.Fatalf("stripped version = %q, want %q", got, "lookit dev (built unknown)\n")
	}
}

func TestRunNoArgsStartsTUI(t *testing.T) {
	got := stubStartTUI(t, nil)
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if got.InitialQuery != "" {
		t.Fatalf("InitialQuery = %q, want empty", got.InitialQuery)
	}
	if got.Seed {
		t.Fatalf("Seed = true, want false for the no-arg launch")
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q, want both empty", stdout.String(), stderr.String())
	}
}

func TestRunSeedsTUIWithTarget(t *testing.T) {
	got := stubStartTUI(t, nil)
	var stdout, stderr bytes.Buffer
	code := run([]string{"alice@plan.cat"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !got.Seed {
		t.Fatalf("Seed = false, want true when an arg is supplied")
	}
	if got.InitialQuery != "alice@plan.cat" {
		t.Fatalf("InitialQuery = %q, want %q", got.InitialQuery, "alice@plan.cat")
	}
}

func TestRunSeedsTUIWithMalformedTarget(t *testing.T) {
	// A malformed target is NOT rejected at the CLI; it seeds the TUI, which
	// shows the parse error in-app.
	got := stubStartTUI(t, nil)
	var stdout, stderr bytes.Buffer
	code := run([]string{"just-a-name"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !got.Seed || got.InitialQuery != "just-a-name" {
		t.Fatalf("Seed=%v InitialQuery=%q, want true / %q", got.Seed, got.InitialQuery, "just-a-name")
	}
}

func TestRunSeedsTUIWithBlankArg(t *testing.T) {
	// lookit "": an arg was supplied (Seed=true) even though its value is blank,
	// so the TUI replays it and surfaces the parse error in-place.
	got := stubStartTUI(t, nil)
	var stdout, stderr bytes.Buffer
	code := run([]string{""}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !got.Seed {
		t.Fatalf("Seed = false, want true for a supplied-but-blank arg")
	}
	if got.InitialQuery != "" {
		t.Fatalf("InitialQuery = %q, want empty", got.InitialQuery)
	}
}

func TestRunTooManyArgs(t *testing.T) {
	pinProfile(t, colorprofile.NoTTY)
	var stdout, stderr bytes.Buffer
	code := run([]string{"a@b", "c@d"}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr = %q, want usage block", stderr.String())
	}
}

func TestRunTUIFailure(t *testing.T) {
	pinProfile(t, colorprofile.NoTTY)
	stubStartTUI(t, errors.New("terminal unavailable"))
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "terminal unavailable") {
		t.Fatalf("stderr = %q, want TUI error", stderr.String())
	}
}
```

- [ ] **Step 9: Run the main tests to verify they fail (compile error)**

Run: `go test . -count=1 -v 2>&1 | head -30`
Expected: FAIL — build error, because `main.go` still has the old `run`, `startTUI = func() error`, `exitNetwork`/`exitUsage`, etc.

- [ ] **Step 10: Rewrite `main.go`**

Replace the entire contents of `main.go` with:

```go
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/colorprofile"

	"github.com/jonathandeamer/lookit/render"
	"github.com/jonathandeamer/lookit/tui"
)

// Exit codes: lookit is a TUI-only finger browser, so there is no per-result
// network exit code. 0 is a clean session; 1 is any startup/usage failure.
const (
	exitOK    = 0
	exitError = 1
)

var (
	version       = "dev"
	builtAt       = "unknown"
	detectProfile = colorprofile.Detect
	startTUI      = func(opts tui.Options) error {
		profile := colorprofile.Detect(os.Stdout, os.Environ())
		return tui.Run(context.Background(), profile, opts)
	}
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable router. lookit always opens the TUI; a single positional
// argument seeds it. -h/--help and -v/--version are the only flags.
func run(args []string, stdout, stderr io.Writer) int {
	outProfile := detectProfile(stdout, os.Environ())
	errProfile := detectProfile(stderr, os.Environ())

	var positional []string
	for _, a := range args {
		switch a {
		case "-h", "--help":
			fmt.Fprint(stdout, render.Usage(outProfile))
			return exitOK
		case "-v", "--version":
			fmt.Fprintln(stdout, render.Version(versionString(), outProfile))
			return exitOK
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprint(stderr, render.Usage(errProfile))
				return exitError
			}
			positional = append(positional, a)
		}
	}

	if len(positional) > 1 {
		fmt.Fprint(stderr, render.Usage(errProfile))
		return exitError
	}

	// Seed is true whenever a positional arg was supplied, even a blank one
	// (lookit ""): the TUI replays it through submit() so a blank/malformed arg
	// shows its parse error in-place rather than silently landing.
	seed := len(positional) == 1
	query := ""
	if seed {
		query = positional[0]
	}

	if err := startTUI(tui.Options{InitialQuery: query, Seed: seed, Version: versionString()}); err != nil {
		fmt.Fprintln(stderr, render.ErrorLine(err.Error(), errProfile))
		return exitError
	}
	return exitOK
}

func versionString() string {
	return fmt.Sprintf("lookit %s (built %s)", version, builtAt)
}
```

- [ ] **Step 11: Run the main tests to verify they pass**

Run: `go test . -count=1 -v`
Expected: PASS (all `TestRun*` and `TestVersionString`).

- [ ] **Step 12: Build and run the full suite**

Run: `go build ./... && go test ./... -count=1`
Expected: builds cleanly; all packages PASS.

- [ ] **Step 13: Commit**

```bash
git add tui/run.go tui/app.go tui/app_test.go main.go main_test.go
git commit -m "feat(tui): seed the TUI from the CLI argument; retire one-shot mode"
```

---

## Task 4: Version/tagline band on the `?` help panel

**Files:**
- Modify: `tui/app.go:763-765` (`helpView`)
- Modify: `tui/app_test.go` (add a band test)

- [ ] **Step 1: Write the help-band test**

Add to `tui/app_test.go` (it imports `ansi` — see Step 3 if not):

```go
func TestHelpPanelShowsVersionBand(t *testing.T) {
	m := newAppWithOptions(stubFetch(t), colorprofile.NoTTY, Options{Version: "lookit 1.2.3 (built 2026-06-02)"})
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)

	got := ansi.Strip(m.helpView())
	if !strings.Contains(got, "lookit 1.2.3") {
		t.Fatalf("help view missing version: %q", got)
	}
	if !strings.Contains(got, "A modern TUI browser for the Finger protocol") {
		t.Fatalf("help view missing tagline: %q", got)
	}
	if !strings.Contains(got, "RFC 1288") {
		t.Fatalf("help view missing protocol pointer: %q", got)
	}
}

func TestHelpPanelNoBandWithoutVersion(t *testing.T) {
	m := newAppWithOptions(stubFetch(t), colorprofile.NoTTY, Options{})
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)

	got := ansi.Strip(m.helpView())
	if strings.Contains(got, "RFC 1288") {
		t.Fatalf("help view should have no version band when version is empty: %q", got)
	}
}
```

- [ ] **Step 2: Ensure `ansi` is imported in the test file**

`tui/app_test.go` must import `"github.com/charmbracelet/x/ansi"`. If it is not already in the import block, add it.

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./tui/ -run HelpPanelShowsVersionBand -count=1 -v`
Expected: FAIL — the version/pointer text is absent from `helpView()`.

- [ ] **Step 4: Add the band to `helpView`**

In `tui/app.go`, replace `helpView` (lines 763–765) with:

```go
func (m appModel) helpView() string {
	st := m.common.styles
	w := m.common.width
	body := fullWidthHelpView(m.keys.FullHelp(), st, w, m.helpModel.FullSeparator)
	if m.common.version == "" {
		return body
	}
	const tagline = "A modern TUI browser for the Finger protocol"
	p := st.palette
	nameStyle := lipgloss.NewStyle().Foreground(p.AccentViolet).Background(p.SubtleBg).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(p.Dim).Background(p.SubtleBg)
	name, rest, _ := strings.Cut(m.common.version, " ")
	// Title band: version + one-line tagline (the spec requires both).
	titleInner := nameStyle.Render(name)
	if rest != "" {
		titleInner += dimStyle.Render(" " + rest)
	}
	titleInner += dimStyle.Render(" · " + tagline)
	footerInner := dimStyle.Render("finger · RFC 1288 · github.com/jonathandeamer/lookit")
	title := padStyledLine(ansi.Truncate(titleInner, w, "…"), w, st.helpBand)
	footer := padStyledLine(ansi.Truncate(footerInner, w, "…"), w, st.helpBand)
	return title + "\n" + body + "\n" + footer
}
```

(`lipgloss`, `ansi`, and `strings` are already imported in `tui/app.go`; `padStyledLine`, `st.palette`, `st.helpBand`, `AccentViolet`, `Dim`, `SubtleBg` all already exist. At width 80 the title row — version `+ " · " +` tagline — is ~78 columns, so it is not truncated; narrower terminals truncate the tail gracefully.)

- [ ] **Step 5: Run the band tests**

Run: `go test ./tui/ -run HelpPanel -count=1 -v`
Expected: PASS.

- [ ] **Step 6: Confirm the help panel still sizes correctly (no regressions)**

Run: `go test ./tui/ -count=1`
Expected: PASS — `helpHeight`/`resizeForHelp` measure `helpView()` dynamically, so the two extra rows are absorbed automatically.

- [ ] **Step 7: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): show version and finger pointer in the help panel"
```

---

## Task 5: Final verification

**Files:** none (verification + docs only)

- [ ] **Step 1: Run the full CI gate**

Run: `make check`
Expected: PASS — `go vet`, `gofmt -l .` empty, `golangci-lint run ./...` (no `unused` complaints — `renderFooter`/`fmtBytes`/`WithoutFooter` are fully removed and have no callers), and `go test ./... -race`.

- [ ] **Step 2: Smoke-check the CLI surfaces manually**

Run: `go run . --help && go run . --version && go run . -v`
Expected: the new usage block (with `--version` and the `press ? in lookit for keys` line), then the version line twice. None of these open the TUI.

(The seeded-TUI paths — `go run . user@host` and `go run . just-a-name` — require a real TTY and cannot be verified headlessly; they are covered by the `tui`/`main` unit tests.)

- [ ] **Step 3: Tick the spec's testing checklist mentally against the suite**

Confirm each spec "Testing" bullet maps to a passing test: seeded-valid (`TestSeededValidQueryFetchesAndRoutesToReader`), seeded-invalid (`TestSeededInvalidQueryShowsErrorOnLanding`), help band (`TestHelpPanelShowsVersionBand`), and the `main` router cases (`TestRunHelp`, `TestRunVersionFlag`, `TestRunNoArgsStartsTUI`, `TestRunSeedsTUIWithTarget`, `TestRunTooManyArgs`, `TestRunTUIFailure`).

- [ ] **Step 4: Final commit if `make check` produced formatting changes**

```bash
git status --porcelain
# If gofmt changed anything:
git add -A && git commit -m "style: gofmt"
```

# Request Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add cancellable modal finger requests plus in-place refresh/retry with honest stale-response warnings and preserved reader/list context.

**Architecture:** A new `tui/request.go` owns typed pending-request state, request contexts, cancellation, session shutdown, and request-status copy. `appModel` continues to own key routing and history; result routing is split into “classify an entry” and “apply it as navigation or refresh,” while `listModel` gains one identity-based selection helper for refresh restoration.

**Tech Stack:** Go 1.21+ source compatibility with the `go 1.26` toolchain declared by `go.mod`; Bubble Tea v2, Bubbles v2, Lip Gloss v2 (`charm.land/...` imports); standard-library `context` and `time`; Go tests with injected `FetchFunc` fakes.

## Global Constraints

- Read `docs/superpowers/specs/2026-08-02-request-controls-design.md` before implementation; it is the source of truth for behavior and copy.
- Preserve the dependency direction `finger/` → `render/` → `tui/`; this feature changes only TUI lifecycle/presentation plus user documentation.
- Do not change `finger.Query` sanitization, the 1 MiB body cap, timeout values, target parsing, forwarding, or port-79 pinning.
- One request may be pending at a time. Loading is modal: only `Esc`, `q`, and `Ctrl+C` act; non-key terminal capability/resize messages still update.
- Refresh never adds `/W`, changes the target, appends history, changes `pos`, or truncates the forward tail.
- Tests must use injected fakes and must not access the network.
- Preserve macOS-native text selection: do not enable mouse capture.
- Use adaptive existing styles; add no hard-coded colours.
- Do not add reader search, configurable timeouts, automatic retry, background work, persistence, or new screens.
- Run `make check` as the final gate.
- Commit steps below are conditional. Skip them unless the user explicitly authorizes commits for the implementation session. Never add co-author or generated-by trailers.

## File Map

- Create `tui/request.go`: request intent/state, context creation, cancellation, session-cancel message, loading text, and persistent failure text.
- Create `tui/request_test.go`: focused lifecycle, cancellation, refresh disposition, context restoration, modal routing, status, and warning tests.
- Modify `tui/app.go`: session context construction, pending-result dispatch, modal key routing, entry routing, history replacement, refresh context restoration, warning lifecycle, key enablement, status, and help.
- Modify `tui/run.go`: pass the exported `Run` context into `appModel`.
- Modify `tui/keys.go` and `tui/keys_test.go`: add and pin the `r` refresh/retry binding.
- Modify `tui/list.go` and `tui/list_test.go`: restore refreshed selection by target/login identity within visible filtered items.
- Modify `tui/app_test.go`: migrate assertions from removed loading fields and retain existing routing/link/about coverage.
- Modify `README.md` and `docs/user-facing-messages.md`: document the controls and exact copy.

---

### Task 1: Typed cancellable navigation requests

**Files:**
- Create: `tui/request.go`
- Create: `tui/request_test.go`
- Modify: `tui/app.go`
- Modify: `tui/run.go`
- Modify: `tui/app_test.go`
- Modify: `tui/list_test.go`

**Interfaces:**
- Consumes: existing `FetchFunc`, `fetchCmd`, `appModel.input`, `appModel.spin`, and `tea.Quit`.
- Produces: `requestIntent`, `requestNavigate`, `requestRefresh`, `pendingRequest`, `requestFailure`, `sessionCanceledMsg`, `waitForSessionCancel(context.Context) tea.Cmd`, `(*appModel).startRequest(finger.Target, requestIntent, bool) tea.Cmd`, `(*appModel).finishRequest(uint64) (pendingRequest, bool)`, `(*appModel).cancelRequest() tea.Cmd`, and `(appModel).pendingStatus(time.Time) string`.

- [ ] **Step 1: Write failing lifecycle tests**

Create `tui/request_test.go` with these deterministic helpers and tests:

```go
package tui

import (
    "context"
    "image/color"
    "strings"
    "testing"
    "time"

    "charm.land/bubbles/v2/spinner"
    tea "charm.land/bubbletea/v2"
    "github.com/charmbracelet/colorprofile"
    "github.com/jonathandeamer/lookit/finger"
)

func asyncMessages(cmd tea.Cmd) <-chan tea.Msg {
    out := make(chan tea.Msg, 8)
    var run func(tea.Cmd)
    run = func(next tea.Cmd) {
        go func() {
            msg := next()
            if batch, ok := msg.(tea.BatchMsg); ok {
                for _, child := range batch {
                    run(child)
                }
                return
            }
            out <- msg
        }()
    }
    run(cmd)
    return out
}

func nextFetchResult(t *testing.T, msgs <-chan tea.Msg) fetchResultMsg {
    t.Helper()
    deadline := time.After(time.Second)
    for {
        select {
        case msg := <-msgs:
            if result, ok := msg.(fetchResultMsg); ok {
                return result
            }
        case <-deadline:
            t.Fatal("timed out waiting for fetchResultMsg")
        }
    }
}

func deliverNavigation(m appModel, entry Entry) appModel {
    m.reqSeq++
    m.pending = &pendingRequest{
        id: m.reqSeq, target: entry.Target, intent: requestNavigate,
        started: time.Now(), cancel: func() {},
    }
    next, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: entry})
    return next.(appModel)
}

func TestEscapeCancelsPendingRequestAndDropsResult(t *testing.T) {
    started := make(chan struct{})
    fetch := func(ctx context.Context, _ finger.Target) ([]byte, finger.Meta, error) {
        close(started)
        <-ctx.Done()
        return []byte("partial"), finger.Meta{}, ctx.Err()
    }
    m := newApp(fetch, colorprofile.NoTTY)
    m.input.SetValue("alice@plan.cat")
    next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
    m = next.(appModel)
    msgs := asyncMessages(cmd)
    <-started

    next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
    m = next.(appModel)
    if m.pending != nil || !m.inputFocused || m.input.Value() != "alice@plan.cat" {
        t.Fatalf("cancelled request = pending %#v, focused %v, value %q", m.pending, m.inputFocused, m.input.Value())
    }
    result := nextFetchResult(t, msgs)
    next, _ = m.Update(result)
    m = next.(appModel)
    if m.pos != -1 || len(m.history) != 0 {
        t.Fatalf("cancelled result landed: pos=%d history=%d", m.pos, len(m.history))
    }
}

func TestPendingRequestIsModal(t *testing.T) {
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.pending = &pendingRequest{id: 1, target: hostTarget(t, "alice@plan.cat"), intent: requestNavigate, started: time.Now(), cancel: func() {}}
    before := m.input.Value()
    for _, msg := range []tea.KeyPressMsg{{Code: '?'}, {Code: 'i'}, {Code: 'j'}, {Code: tea.KeyEnter}} {
        next, cmd := m.Update(msg)
        m = next.(appModel)
        if cmd != nil || m.help || m.input.Value() != before || m.pending == nil {
            t.Fatalf("key %q escaped modal loading", msg.String())
        }
    }
}

func TestPendingStatusIncludesElapsedAndControls(t *testing.T) {
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.pending = &pendingRequest{target: hostTarget(t, "alice@plan.cat"), started: time.Unix(100, 0), cancel: func() {}}
    got := m.pendingStatus(time.Unix(103, 900_000_000))
    for _, want := range []string{"loading alice@plan.cat", "3s", "esc cancel", "q quit"} {
        if !strings.Contains(got, want) {
            t.Fatalf("pending status %q missing %q", got, want)
        }
    }
}

func TestSessionCancellationCancelsAndQuits(t *testing.T) {
    ctx, cancelSession := context.WithCancel(context.Background())
    m := newAppWithContext(ctx, stubFetch(t), colorprofile.NoTTY, Options{})
    requestCancelled := false
    m.pending = &pendingRequest{cancel: func() { requestCancelled = true }}
    cancelSession()
    next, cmd := m.Update(sessionCanceledMsg{})
    if !requestCancelled || next.(appModel).pending != nil || cmd == nil {
        t.Fatal("session cancellation did not cancel and quit")
    }
    if _, ok := cmd().(tea.QuitMsg); !ok {
        t.Fatalf("session cancellation command = %T", cmd())
    }
}

func TestSubmitRecordsInputOriginBeforeBlur(t *testing.T) {
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.input.SetValue("alice@plan.cat")
    next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
    got := next.(appModel)
    if cmd == nil || got.pending == nil || !got.pending.returnToInput || got.pending.intent != requestNavigate {
        t.Fatalf("submitted pending = %#v, cmd nil=%v", got.pending, cmd == nil)
    }
}

func TestCancelSeededLookupRestoresSeededInput(t *testing.T) {
    m := newAppWithOptions(stubFetch(t), colorprofile.NoTTY, Options{InitialQuery: "alice@plan.cat", Seed: true})
    next, _ := m.Update(seedSubmitMsg{})
    m = next.(appModel)
    next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
    got := next.(appModel)
    if !got.inputFocused || got.input.Value() != "alice@plan.cat" || got.pending != nil || got.pos != -1 {
        t.Fatalf("cancelled seed = focused %v value %q pending %#v pos %d", got.inputFocused, got.input.Value(), got.pending, got.pos)
    }
}

func TestCancelSubmittedTargetOverContentRestoresEditor(t *testing.T) {
    old := Entry{Target: hostTarget(t, "old@plan.cat"), Body: []byte("old\n")}
    m := deliverNavigation(newApp(stubFetch(t), colorprofile.NoTTY), old)
    m.common.fetch = func(ctx context.Context, _ finger.Target) ([]byte, finger.Meta, error) {
        <-ctx.Done()
        return nil, finger.Meta{}, ctx.Err()
    }
    next, _ := m.Update(tea.KeyPressMsg{Code: 'i'})
    m = next.(appModel)
    m.input.SetValue("new@plan.cat")
    next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
    m = next.(appModel)
    next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
    got := next.(appModel)
    if !got.inputFocused || got.input.Value() != "new@plan.cat" || got.pos != 0 || string(got.history[0].entry.Body) != "old\n" {
        t.Fatalf("cancelled edit = focused %v value %q pos %d entry %#v", got.inputFocused, got.input.Value(), got.pos, got.history[0].entry)
    }
}

func TestStartRequestDefensivelyCancelsPriorRequest(t *testing.T) {
    cancelled := false
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.reqSeq = 4
    m.pending = &pendingRequest{id: 4, cancel: func() { cancelled = true }}
    target := hostTarget(t, "alice@plan.cat")
    cmd := m.startRequest(target, requestRefresh, true)
    if !cancelled || cmd == nil || m.pending == nil || m.pending.id != 5 || m.pending.target != target || m.pending.intent != requestRefresh || !m.pending.retry {
        t.Fatalf("replacement pending = %#v, prior cancelled=%v", m.pending, cancelled)
    }
}

func TestPendingQuitKeysCancelBeforeQuit(t *testing.T) {
    for _, keyMsg := range []tea.KeyPressMsg{{Code: 'q'}, {Code: 'c', Mod: tea.ModCtrl}} {
        cancelled := false
        m := newApp(stubFetch(t), colorprofile.NoTTY)
        m.pending = &pendingRequest{id: 1, cancel: func() { cancelled = true }}
        next, cmd := m.Update(keyMsg)
        if !cancelled || next.(appModel).pending != nil || !isQuit(cmd) {
            t.Fatalf("key %q: cancelled=%v pending=%#v quit=%v", keyMsg.String(), cancelled, next.(appModel).pending, isQuit(cmd))
        }
    }
}

func TestPendingStillProcessesTerminalMessages(t *testing.T) {
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.pending = &pendingRequest{id: 1, cancel: func() {}}
    beforeSpinner := m.spin.View()
    next, _ := m.Update(tea.WindowSizeMsg{Width: 91, Height: 27})
    m = next.(appModel)
    next, _ = m.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
    m = next.(appModel)
    next, _ = m.Update(tea.BackgroundColorMsg{Color: color.White})
    m = next.(appModel)
    next, tickCmd := m.Update(spinner.TickMsg{Time: time.Now()})
    got := next.(appModel)
    if got.pending == nil || got.common.width != 91 || got.common.height != 27 || got.common.profile != colorprofile.TrueColor || got.common.darkBackground || got.spin.View() == beforeSpinner || tickCmd == nil {
        t.Fatalf("terminal updates lost while pending: pending=%#v size=%dx%d profile=%v dark=%v spinner=%q", got.pending, got.common.width, got.common.height, got.common.profile, got.common.darkBackground, got.spin.View())
    }
}
```

- [ ] **Step 2: Run the tests to verify RED**

```bash
go test ./tui -run 'Test(EscapeCancelsPendingRequestAndDropsResult|PendingRequestIsModal|PendingStatusIncludesElapsedAndControls|SessionCancellationCancelsAndQuits|SubmitRecordsInputOriginBeforeBlur|CancelSeededLookupRestoresSeededInput|CancelSubmittedTargetOverContentRestoresEditor|StartRequestDefensivelyCancelsPriorRequest|PendingQuitKeysCancelBeforeQuit|PendingStillProcessesTerminalMessages)$' -count=1 -v
```

Expected: build failure because the request types and constructors do not exist.

- [ ] **Step 3: Implement `tui/request.go`**

```go
package tui

import (
    "context"
    "fmt"
    "strings"
    "time"

    tea "charm.land/bubbletea/v2"
    "github.com/jonathandeamer/lookit/finger"
)

type requestIntent uint8

const (
    requestNavigate requestIntent = iota
    requestRefresh
)

type pendingRequest struct {
    id            uint64
    target        finger.Target
    intent        requestIntent
    retry         bool
    returnToInput bool
    started       time.Time
    cancel        context.CancelFunc
}

type requestFailure struct {
    retry  bool
    target finger.Target
    err    error
}

type sessionCanceledMsg struct{}

func waitForSessionCancel(ctx context.Context) tea.Cmd {
    return func() tea.Msg { <-ctx.Done(); return sessionCanceledMsg{} }
}

func (m *appModel) takePending() *pendingRequest {
    pending := m.pending
    m.pending = nil
    return pending
}

func (m *appModel) startRequest(target finger.Target, intent requestIntent, retry bool) tea.Cmd {
    if previous := m.takePending(); previous != nil {
        previous.cancel()
    }
    base := m.common.ctx
    if base == nil {
        base = context.Background()
    }
    ctx, cancel := context.WithCancel(base)
    m.reqSeq++
    m.pending = &pendingRequest{
        id: m.reqSeq, target: target, intent: intent, retry: retry,
        returnToInput: m.inputFocused, started: time.Now(), cancel: cancel,
    }
    return tea.Batch(fetchCmd(ctx, m.common.fetch, target, m.reqSeq), m.spin.Tick)
}

func (m *appModel) finishRequest(id uint64) (pendingRequest, bool) {
    if m.pending == nil || m.pending.id != id {
        return pendingRequest{}, false
    }
    pending := *m.takePending()
    pending.cancel()
    return pending, true
}

func (m *appModel) cancelRequest() tea.Cmd {
    pending := m.takePending()
    if pending == nil {
        return nil
    }
    pending.cancel()
    if !pending.returnToInput {
        return nil
    }
    m.inputFocused = true
    m.input.CursorEnd()
    m.resize()
    return m.input.Focus()
}

func (m appModel) pendingStatus(now time.Time) string {
    if m.pending == nil {
        return ""
    }
    parts := []string{m.spin.View() + " loading " + m.pending.target.Raw}
    if elapsed := now.Sub(m.pending.started).Truncate(time.Second); elapsed >= time.Second {
        parts = append(parts, elapsed.String())
    }
    return strings.Join(append(parts, "esc cancel", "q quit"), " · ")
}

func (f requestFailure) statusText() string {
    operation, consequence := "refresh", " · showing previous response"
    if f.retry {
        operation, consequence = "retry", ""
    }
    return fmt.Sprintf("%s failed: %s%s · r retry", operation, f.err, consequence)
}
```

- [ ] **Step 4: Wire request state into constructors and `Run`**

In `commonModel`, add `ctx context.Context`. Replace `loading`/`loadingTarget` in `appModel` with `pending *pendingRequest` and `requestFailure *requestFailure`.

Use these constructor boundaries:

```go
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
        in.SetValue(opts.InitialQuery)
    }
    in.Focus()
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
    app.updateKeymap()
    return app
}
```

Append the session watcher in `Init` only for cancellable contexts, so existing `collectMsgs` tests remain non-blocking:

```go
if m.common.ctx.Done() != nil {
    cmds = append(cmds, waitForSessionCancel(m.common.ctx))
}
return tea.Batch(cmds...)
```

Replace `Run`'s ignored-context body with:

```go
func Run(ctx context.Context, profile colorprofile.Profile, opts Options) error {
    program := tea.NewProgram(newAppWithContext(ctx, defaultFetch, profile, opts))
    _, err := program.Run()
    return err
}
```

Add `context` to `tui/list_test.go` imports and update its shared common model:

```go
func testCommon() *commonModel {
    return &commonModel{ctx: context.Background(), width: 80, height: 24, darkBackground: true, styles: newStyles(true)}
}
```

- [ ] **Step 5: Migrate navigation and result dispatch**

Replace every `m.startFetch(target)` with `m.startRequest(target, requestNavigate, false)` in About, drill, reader links, ambiguous finger actions, and links-panel actions; remove `startFetch`. In `submit`, start the request **before** blurring so `returnToInput` records the origin correctly:

```go
func (m *appModel) submit() tea.Cmd {
    target, err := finger.ParseTarget(strings.TrimSpace(m.input.Value()))
    if err != nil {
        m.flash = "error: " + err.Error()
        return nil
    }
    m.flash = ""
    cmd := m.startRequest(target, requestNavigate, false)
    m.blurInput()
    return cmd
}
```

In `Update`:

```go
case sessionCanceledMsg:
    _ = m.cancelRequest()
    return m, tea.Quit

case fetchResultMsg:
    if m.common.ctx.Err() != nil {
        _ = m.cancelRequest()
        return m, tea.Quit
    }
    request, ok := m.finishRequest(msg.reqID)
    if !ok { return m, nil }
    _ = request // Task 2 dispatches on this intent.
    return m.routeFetch(msg.entry), nil
```

Change spinner handling to `m.pending != nil`; remove `m.loading = false` from `routeFetch`.

- [ ] **Step 6: Make loading modal and update status**

Before all existing `handleKey` branches:

```go
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
```

Add this leading `updateKeymap` branch (Task 4 adds Refresh to the disabled slice):

```go
if m.pending != nil {
    for _, binding := range []*key.Binding{
        &m.keys.Open, &m.keys.FocusInput, &m.keys.Filter, &m.keys.Raw,
        &m.keys.Copy, &m.keys.Help, &m.keys.About, &m.keys.Move,
        &m.keys.Page, &m.keys.Jump, &m.keys.LinkNext, &m.keys.LinkPrev,
        &m.keys.LinkFinger, &m.keys.LinkPanel,
    } {
        binding.SetEnabled(false)
    }
    m.keys.Back.SetEnabled(true)
    m.keys.Quit.SetEnabled(true)
    m.keys.ForceQuit.SetEnabled(true)
    return
}
```

Replace loading status with:

```go
if m.pending != nil {
    return statusBar{width: m.common.width, styles: m.common.styles, hints: m.pendingStatus(time.Now())}
}
```

- [ ] **Step 7: Migrate all existing loading assertions**

Run `rg -n 'loading|loadingTarget|fetchResultMsg' tui --glob '*_test.go'`. Use the `deliverNavigation` helper from Step 1 for tests that synthesize a landed navigation without first submitting/drilling. Convert direct synthetic landings from:

```go
next, _ := m.Update(fetchResultMsg{entry: entry})
m = next.(appModel)
```

to:

```go
m = deliverNavigation(m, entry)
```

For results that follow a real `submit`, drill, About action, or link action, retain `Update(fetchResultMsg{reqID: m.reqSeq, ...})`; the action has already created `m.pending`. Replace loading assertions exactly as follows:

```go
if got.pending == nil { t.Fatal("request should be pending") }
if got.pending != nil { t.Fatal("request should be settled") }
```

Replace manual `m.loading = true`/`m.reqSeq = id` setup with:

```go
m.reqSeq = id
m.pending = &pendingRequest{id: id, target: target, intent: requestNavigate, cancel: func() {}}
```

Preserve every seeded-query, link, About, spinner, list-visible-during-drill, result-clears-loading, and stale-result assertion. The stale-result test must leave the pending pointer and ID unchanged after ID 1, then clear it and land ID 2.

- [ ] **Step 8: Verify Task 1**

```bash
gofmt -w tui/request.go tui/request_test.go tui/app.go tui/run.go tui/app_test.go tui/list_test.go
go test ./tui -run 'Test(EscapeCancelsPendingRequestAndDropsResult|PendingRequestIsModal|PendingStatusIncludesElapsedAndControls|SessionCancellationCancelsAndQuits|SubmitRecordsInputOriginBeforeBlur|CancelSeededLookupRestoresSeededInput|CancelSubmittedTargetOverContentRestoresEditor|StartRequestDefensivelyCancelsPriorRequest|PendingQuitKeysCancelBeforeQuit|PendingStillProcessesTerminalMessages|StaleFetchResultDropped|LoadingShowsSpinnerTarget|ResultClearsLoading)$' -count=1 -v
go test ./tui -count=1
```

Expected: both pass.

- [ ] **Step 9: Conditional commit checkpoint**

If commits are explicitly authorized:

```bash
git add tui/request.go tui/request_test.go tui/app.go tui/run.go tui/app_test.go tui/list_test.go
git commit -m "feat(tui): add cancellable requests"
```

Otherwise skip the commit.

---

### Task 2: Atomic refresh/retry disposition

**Files:**
- Modify: `tui/app.go`
- Modify: `tui/request_test.go`

**Interfaces:**
- Consumes: Task 1 request intent/state plus existing link detection and list parsing.
- Produces: `routedEntry`, `routeEntry(Entry) routedEntry`, `(*appModel).showRouted(routedEntry)`, `(appModel).landNavigation(Entry) appModel`, `(appModel).landRefresh(Entry, pendingRequest) appModel`, `(appModel).shouldRetry() bool`, and `(*appModel).refreshCurrent() tea.Cmd`.

- [ ] **Step 1: Write failing refresh-disposition tests**

Add imports `errors` and these helpers/tests to `request_test.go`:

```go
func settledReader(t *testing.T, entry Entry) appModel {
    t.Helper()
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.inputFocused = false
    m.input.Blur()
    node := histNode{entry: entry, state: stateReader, links: DetectLinks(entry.Body, entry.Target.HostPort), linkIdx: -1}
    m.history, m.pos, m.state = []histNode{node}, 0, stateReader
    m.reader.focusedLink = -1
    m.reader.setEntryWithLinks(entry, node.links)
    return m
}

func deliverRefresh(m appModel, entry Entry, retry bool) appModel {
    m.reqSeq++
    m.pending = &pendingRequest{id: m.reqSeq, target: entry.Target, intent: requestRefresh, retry: retry, cancel: func() {}}
    next, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: entry})
    return next.(appModel)
}

func TestRefreshReplacesNodeWithoutChangingHistoryShape(t *testing.T) {
    old := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
    m := settledReader(t, old)
    tail := histNode{entry: Entry{Target: hostTarget(t, "bob@plan.cat"), Body: []byte("tail\n")}, state: stateReader}
    m.history = append(m.history, tail)
    got := deliverRefresh(m, Entry{Target: old.Target, Body: []byte("fresh\n")}, false)
    if got.pos != 0 || len(got.history) != 2 || string(got.history[0].entry.Body) != "fresh\n" || string(got.history[1].entry.Body) != "tail\n" {
        t.Fatalf("refresh changed history shape: pos=%d len=%d", got.pos, len(got.history))
    }
}

func TestNavigationFailureStillPushesErrorNode(t *testing.T) {
    old := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
    m := settledReader(t, old)
    failed := Entry{Target: hostTarget(t, "bob@plan.cat"), Err: errors.New("dial failed")}
    got := deliverNavigation(m, failed)
    if got.pos != 1 || len(got.history) != 2 || got.history[1].entry.Err == nil || got.requestFailure != nil {
        t.Fatalf("navigation failure = pos %d history %#v warning %#v", got.pos, got.history, got.requestFailure)
    }
}

func TestEmptyBodyRefreshFailurePreservesEntry(t *testing.T) {
    old := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
    got := deliverRefresh(settledReader(t, old), Entry{Target: old.Target, Err: errors.New("dial failed")}, false)
    if string(got.history[0].entry.Body) != "old\n" || got.requestFailure == nil || got.requestFailure.err.Error() != "dial failed" {
        t.Fatalf("empty failure result = entry %#v failure %#v", got.history[0].entry, got.requestFailure)
    }
}

func TestCleanEmptyRefreshReplacesNode(t *testing.T) {
    old := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
    got := deliverRefresh(settledReader(t, old), Entry{Target: old.Target}, false)
    if len(got.history[0].entry.Body) != 0 || got.history[0].entry.Err != nil {
        t.Fatalf("clean empty refresh did not replace: %#v", got.history[0].entry)
    }
}

func TestBodyBearingRefreshErrorReplacesNode(t *testing.T) {
    old := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
    partial := Entry{Target: old.Target, Body: []byte("partial\n"), Meta: finger.Meta{Truncated: true}, Err: errors.New("read timed out")}
    got := deliverRefresh(settledReader(t, old), partial, false)
    if string(got.history[0].entry.Body) != "partial\n" || got.history[0].entry.Err == nil || got.requestFailure != nil {
        t.Fatalf("partial refresh = entry %#v failure %#v", got.history[0].entry, got.requestFailure)
    }
}

func TestSuccessfulRetryReplacesEmptyErrorNode(t *testing.T) {
    failed := Entry{Target: hostTarget(t, "alice@plan.cat"), Err: errors.New("dial failed")}
    got := deliverRefresh(settledReader(t, failed), Entry{Target: failed.Target, Body: []byte("recovered\n")}, true)
    if got.pos != 0 || len(got.history) != 1 || string(got.history[0].entry.Body) != "recovered\n" || got.history[0].entry.Err != nil || got.requestFailure != nil {
        t.Fatalf("successful retry = history %#v warning %#v", got.history, got.requestFailure)
    }
}
```

- [ ] **Step 2: Run tests to verify RED**

```bash
go test ./tui -run 'Test(RefreshReplacesNode|NavigationFailureStillPushesErrorNode|EmptyBodyRefreshFailure|CleanEmptyRefresh|BodyBearingRefreshError|SuccessfulRetryReplacesEmptyErrorNode)' -count=1 -v
```

Expected: failures because matching results still always push navigation.

- [ ] **Step 3: Split route classification from disposition**

Replace `routeFetch` with:

```go
type routedEntry struct {
    node   histNode
    parsed *parsedUserList
}

func routeEntry(entry Entry) routedEntry {
    routed := routedEntry{node: histNode{entry: entry, state: stateReader, linkIdx: -1}}
    routed.node.links = DetectLinks(entry.Body, entry.Target.HostPort)
    if len(entry.Body) == 0 || !shouldOpenList(entry) { return routed }
    parsed, ok := parseUserList(entry.Body, entry.Target.HostPort)
    if !ok { return routed }
    routed.node.state = stateList
    routed.node.listUsers = len(parsed.users)
    routed.node.listGeneric = parsed.generic
    routed.parsed = &parsed
    return routed
}

func (m *appModel) showRouted(routed routedEntry) {
    m.inputFocused = false
    m.input.Blur()
    m.showingRaw, m.showingLinks = false, false
    m.state = routed.node.state
    if routed.node.state == stateList {
        m.list = newListWithPreamble(m.common, routed.node.entry.Target, routed.parsed.users, routed.node.entry.Body, routed.node.listGeneric)
        m.listReady = true
        return
    }
    m.reader.focusedLink = -1
    m.reader.setEntryWithLinks(routed.node.entry, routed.node.links)
}

func (m appModel) landNavigation(entry Entry) appModel {
    m.snapshot()
    routed := routeEntry(entry)
    m.showRouted(routed)
    m.push(routed.node)
    m.requestFailure = nil
    return m
}

func (m appModel) landRefresh(entry Entry, request pendingRequest) appModel {
    if m.pos < 0 || m.pos >= len(m.history) { return m }
    if entry.Err != nil && len(entry.Body) == 0 {
        m.requestFailure = &requestFailure{retry: request.retry, target: request.target, err: entry.Err}
        return m
    }
    routed := routeEntry(entry)
    m.history[m.pos] = routed.node
    m.showRouted(routed)
    m.requestFailure = nil
    return m
}
```

- [ ] **Step 4: Dispatch by intent and add the refresh engine**

After `finishRequest`, dispatch:

```go
if request.intent == requestRefresh {
    return m.landRefresh(msg.entry, request), nil
}
return m.landNavigation(msg.entry), nil
```

Add:

```go
func (m appModel) shouldRetry() bool {
    if m.requestFailure != nil { return true }
    if m.pos < 0 || m.pos >= len(m.history) { return false }
    entry := m.history[m.pos].entry
    return entry.Err != nil && len(entry.Body) == 0
}

func (m *appModel) refreshCurrent() tea.Cmd {
    if m.pos < 0 || m.pos >= len(m.history) { return nil }
    m.snapshot()
    return m.startRequest(m.history[m.pos].entry.Target, requestRefresh, m.shouldRetry())
}
```

Do not bind `r` until Task 4.

- [ ] **Step 5: Verify Task 2**

```bash
gofmt -w tui/app.go tui/request_test.go
go test ./tui -run 'Test(Refresh|EmptyBodyRefreshFailure|CleanEmptyRefresh|BodyBearingRefreshError|StaleFetchResultDropped)' -count=1 -v
go test ./tui -count=1
```

Expected: all pass.

- [ ] **Step 6: Conditional commit checkpoint**

If commits are explicitly authorized:

```bash
git add tui/app.go tui/request_test.go
git commit -m "feat(tui): refresh results in place"
```

Otherwise skip it.

---

### Task 3: Preserve refresh context by identity

**Files:**
- Modify: `tui/app.go`
- Modify: `tui/request.go`
- Modify: `tui/list.go`
- Modify: `tui/list_test.go`
- Modify: `tui/request_test.go`

**Interfaces:**
- Consumes: Task 2 routed refresh and Bubbles `VisibleItems`/`SetFilterText`.
- Produces: `refreshViewState`, `pendingRequest.view *refreshViewState`, capture/restore helpers, `sameUserIdentity`, and `(*listModel).selectIdentity`.

- [ ] **Step 1: Write failing identity tests**

Append to `list_test.go`:

```go
func TestSelectIdentityUsesTargetThenLogin(t *testing.T) {
    m := newList(testCommon(), hostTarget(t, "@tilde.team"), []User{
        {Login: "same", Target: "alice@one.example"},
        {Login: "same", Target: "alice@two.example"},
        {Login: "plain"},
    })
    m.selectIdentity(userItem{login: "same", target: "alice@two.example"})
    selected, _ := m.selected()
    if selected.target != "alice@two.example" { t.Fatalf("target = %q", selected.target) }
    m.selectIdentity(userItem{login: "plain"})
    selected, _ = m.selected()
    if selected.login != "plain" { t.Fatalf("login = %q", selected.login) }
}

func TestSelectIdentityFallsBackInsideFilter(t *testing.T) {
    m := newList(testCommon(), hostTarget(t, "@tilde.team"), []User{{Login: "alice"}, {Login: "bob"}, {Login: "bobby"}})
    m.list.SetFilterText("bob")
    m.selectIdentity(userItem{login: "missing"})
    selected, ok := m.selected()
    if !ok || selected.login != "bob" { t.Fatalf("fallback = %#v, ok=%v", selected, ok) }
}
```

- [ ] **Step 2: Write failing refresh preservation tests**

Append to `request_test.go`:

```go
func settledList(t *testing.T) appModel {
    t.Helper()
    target := hostTarget(t, "@tilde.team")
    routed := routeEntry(Entry{Target: target, Body: []byte("Users currently online:\n\nalice bob\n")})
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.history, m.pos = []histNode{routed.node}, 0
    m.showRouted(routed)
    return m
}

func TestReaderRefreshPreservesScrollAndLinkRaw(t *testing.T) {
    target := hostTarget(t, "alice@plan.cat")
    old := Entry{Target: target, Body: []byte("top\nhttps://example.com\n" + strings.Repeat("line\n", 40))}
    m := settledReader(t, old)
    m.reader.setSize(40, 6)
    m.reader.focusedLink = 0
    m.reader.setEntryWithLinks(old, m.history[0].links)
    m.reader.viewport.SetYOffset(8)
    fresh := Entry{Target: target, Body: []byte("changed\nhttps://example.com\n" + strings.Repeat("new\n", 40))}
    got := deliverRefresh(m, fresh, false)
    if got.reader.viewport.YOffset() != 8 { t.Fatalf("YOffset = %d", got.reader.viewport.YOffset()) }
    if got.reader.focusedLink < 0 || got.reader.links[got.reader.focusedLink].Raw != "https://example.com" {
        t.Fatalf("focused link = %d, links=%#v", got.reader.focusedLink, got.reader.links)
    }
}

func TestRefreshUsesViewCapturedAtRequestStart(t *testing.T) {
    target := hostTarget(t, "alice@plan.cat")
    old := Entry{Target: target, Body: []byte(strings.Repeat("old\n", 40))}
    m := settledReader(t, old)
    m.reader.setSize(40, 6)
    m.reader.viewport.SetYOffset(8)
    _ = m.refreshCurrent()
    m.reader.viewport.SetYOffset(1) // simulate a live resize/clamp while modal
    fresh := Entry{Target: target, Body: []byte(strings.Repeat("fresh\n", 40))}
    next, _ := m.Update(fetchResultMsg{reqID: m.pending.id, entry: fresh})
    got := next.(appModel)
    if got.reader.viewport.YOffset() != 8 { t.Fatalf("YOffset = %d, want start-time snapshot 8", got.reader.viewport.YOffset()) }
}

func TestReaderRefreshClearsMissingLinkInsteadOfReusingIndex(t *testing.T) {
    target := hostTarget(t, "alice@plan.cat")
    old := Entry{Target: target, Body: []byte("https://old.example\nhttps://keep.example\n")}
    m := settledReader(t, old)
    m.reader.focusedLink = 1
    m.history[0].linkIdx = 1
    fresh := Entry{Target: target, Body: []byte("https://replacement.example\n")}
    got := deliverRefresh(m, fresh, false)
    if got.reader.focusedLink != -1 || got.history[0].linkIdx != -1 {
        t.Fatalf("missing link reused numeric index: reader=%d node=%d", got.reader.focusedLink, got.history[0].linkIdx)
    }
}

func TestListRefreshPreservesFilterAndIdentity(t *testing.T) {
    target := hostTarget(t, "@tilde.team")
    routed := routeEntry(Entry{Target: target, Body: []byte("Users currently online:\n\nalice bob bobby\n")})
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.history, m.pos = []histNode{routed.node}, 0
    m.showRouted(routed)
    m.list.list.SetFilterText("bob")
    m.list.selectIdentity(userItem{login: "bobby"})
    got := deliverRefresh(m, Entry{Target: target, Body: []byte("Users currently online:\n\nbob bobby carol\n")}, false)
    selected, ok := got.list.selected()
    if got.list.list.FilterValue() != "bob" || !ok || selected.login != "bobby" {
        t.Fatalf("filter=%q selected=%#v ok=%v", got.list.list.FilterValue(), selected, ok)
    }
}

func TestRefreshTypeChangeResetsViewState(t *testing.T) {
    target := hostTarget(t, "@tilde.team")
    old := Entry{Target: target, Body: []byte(strings.Repeat("old\n", 30))}
    m := settledReader(t, old)
    m.reader.viewport.SetYOffset(9)
    got := deliverRefresh(m, Entry{Target: target, Body: []byte("Users currently online:\n\nalice bob\n")}, false)
    if got.state != stateList || got.list.list.Index() != 0 || got.list.list.FilterValue() != "" {
        t.Fatal("type-changing refresh retained incompatible state")
    }
}

func TestListToReaderRefreshResetsViewState(t *testing.T) {
    target := hostTarget(t, "@tilde.team")
    routed := routeEntry(Entry{Target: target, Body: []byte("Users currently online:\n\nalice bob\n")})
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.history, m.pos = []histNode{routed.node}, 0
    m.showRouted(routed)
    m.list.list.SetFilterText("bob")
    got := deliverRefresh(m, Entry{Target: target, Body: []byte("plain response\n")}, false)
    if got.state != stateReader || got.reader.viewport.YOffset() != 0 || got.reader.focusedLink != -1 {
        t.Fatalf("list-to-reader state = state %d offset %d link %d", got.state, got.reader.viewport.YOffset(), got.reader.focusedLink)
    }
}

func TestCancelRefreshPreservesReaderView(t *testing.T) {
    entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("https://example.com\n" + strings.Repeat("line\n", 30))}
    m := settledReader(t, entry)
    m.reader.setSize(40, 6)
    m.reader.focusedLink = 0
    m.reader.viewport.SetYOffset(7)
    before := m.history[0].entry
    _ = m.startRequest(entry.Target, requestRefresh, false)
    next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
    got := next.(appModel)
    if got.pending != nil || got.reader.viewport.YOffset() != 7 || got.reader.focusedLink != 0 || string(got.history[0].entry.Body) != string(before.Body) {
        t.Fatalf("cancelled refresh = pending %#v offset %d link %d entry %#v", got.pending, got.reader.viewport.YOffset(), got.reader.focusedLink, got.history[0].entry)
    }
}

func TestCancelListDrillPreservesFilterAndSelection(t *testing.T) {
    m := settledList(t)
    m.list.list.SetFilterText("bob")
    m.list.selectIdentity(userItem{login: "bob"})
    target := hostTarget(t, "bob@tilde.team")
    _ = m.startRequest(target, requestNavigate, false)
    next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
    got := next.(appModel)
    selected, ok := got.list.selected()
    if got.pending != nil || got.list.list.FilterValue() != "bob" || !ok || selected.login != "bob" || got.pos != 0 {
        t.Fatalf("cancelled drill = pending %#v filter %q selected %#v ok=%v pos=%d", got.pending, got.list.list.FilterValue(), selected, ok, got.pos)
    }
}
```

- [ ] **Step 3: Run tests to verify RED**

```bash
go test ./tui -run 'Test(SelectIdentity|ReaderRefreshPreserves|ReaderRefreshClears|RefreshUsesViewCaptured|ListRefreshPreserves|RefreshTypeChange|ListToReaderRefresh|CancelRefreshPreserves|CancelListDrillPreserves)' -count=1 -v
```

Expected: build/behavioral failures.

- [ ] **Step 4: Implement list identity restoration**

Add to `list.go`:

```go
func sameUserIdentity(want, candidate userItem) bool {
    if want.target != "" { return candidate.target == want.target }
    return candidate.login == want.login
}

func (m *listModel) selectIdentity(want userItem) {
    for i, raw := range m.list.VisibleItems() {
        candidate, ok := raw.(userItem)
        if ok && sameUserIdentity(want, candidate) {
            m.list.Select(i)
            return
        }
    }
    m.list.Select(0)
}
```

- [ ] **Step 5: Implement refresh view capture/restore**

Add to `app.go`:

```go
type refreshViewState struct {
    state      appState
    scrollY    int
    linkRaw    string
    listFilter string
    selected   userItem
}

func (m appModel) captureRefreshView() refreshViewState {
    view := refreshViewState{state: m.state}
    if m.state == stateList {
        view.listFilter = m.list.list.FilterValue()
        view.selected, _ = m.list.selected()
    } else if m.state == stateReader {
        view.scrollY = m.reader.viewport.YOffset()
        if m.reader.focusedLink >= 0 && m.reader.focusedLink < len(m.reader.links) {
            view.linkRaw = m.reader.links[m.reader.focusedLink].Raw
        }
    }
    return view
}

func (m *appModel) restoreRefreshView(view refreshViewState) {
    if view.state != m.state { return }
    if m.state == stateList {
        if view.listFilter != "" { m.list.list.SetFilterText(view.listFilter) }
        m.list.selectIdentity(view.selected)
        m.snapshot()
        return
    }
    m.reader.focusedLink = -1
    for i, link := range m.reader.links {
        if view.linkRaw != "" && link.Raw == view.linkRaw { m.reader.focusedLink = i; break }
    }
    node := m.history[m.pos]
    m.reader.setEntryWithLinks(node.entry, node.links)
    m.reader.viewport.SetYOffset(view.scrollY)
    m.snapshot()
}
```

Add the snapshot to `pendingRequest` in `request.go`:

```go
type pendingRequest struct {
    id            uint64
    target        finger.Target
    intent        requestIntent
    retry         bool
    returnToInput bool
    started       time.Time
    cancel        context.CancelFunc
    view          *refreshViewState
}
```

Update `refreshCurrent` so production refreshes capture the live state before loading begins:

```go
func (m *appModel) refreshCurrent() tea.Cmd {
    if m.pos < 0 || m.pos >= len(m.history) { return nil }
    m.snapshot()
    view := m.captureRefreshView()
    cmd := m.startRequest(m.history[m.pos].entry.Target, requestRefresh, m.shouldRetry())
    m.pending.view = &view
    return cmd
}
```

Update the `deliverRefresh` test helper to mirror that production invariant:

```go
func deliverRefresh(m appModel, entry Entry, retry bool) appModel {
    view := m.captureRefreshView()
    m.reqSeq++
    m.pending = &pendingRequest{
        id: m.reqSeq, target: entry.Target, intent: requestRefresh,
        retry: retry, cancel: func() {}, view: &view,
    }
    next, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: entry})
    return next.(appModel)
}
```

In the usable-result branch of `landRefresh`, read the start-time view before replacing the node, then restore it:

```go
view := refreshViewState{}
if request.view != nil { view = *request.view }
routed := routeEntry(entry)
m.history[m.pos] = routed.node
m.showRouted(routed)
m.restoreRefreshView(view)
m.requestFailure = nil
return m
```

Do not touch the empty-body failure branch.

- [ ] **Step 6: Verify Task 3**

```bash
gofmt -w tui/app.go tui/list.go tui/list_test.go tui/request_test.go
go test ./tui -run 'Test(SelectIdentity|ReaderRefreshPreserves|ReaderRefreshClears|RefreshUsesViewCaptured|ListRefreshPreserves|RefreshTypeChange|ListToReaderRefresh|CancelRefreshPreserves|CancelListDrillPreserves)' -count=1 -v
go test ./tui -count=1
```

Expected: all pass.

- [ ] **Step 7: Conditional commit checkpoint**

If commits are explicitly authorized:

```bash
git add tui/app.go tui/request.go tui/list.go tui/list_test.go tui/request_test.go
git commit -m "feat(tui): preserve context across refresh"
```

Otherwise skip it.

---

### Task 4: Refresh key, contextual help, and stale-response warning

**Files:**
- Modify: `tui/keys.go`
- Modify: `tui/keys_test.go`
- Modify: `tui/app.go`
- Modify: `tui/app_test.go`
- Modify: `tui/request_test.go`

**Interfaces:**
- Consumes: `refreshCurrent`, `shouldRetry`, `requestFailure.statusText`, current contextual help/status machinery.
- Produces: `keyMap.Refresh`, `(appModel).refreshHelp() key.Help`, `(appModel).refreshHint() string`, `(*appModel).clearRequestFailure()`, exact `r refresh`/`r retry` hints, and warning lifecycle.

- [ ] **Step 1: Write failing binding/scope tests**

Make these exact additions in `keys_test.go`:

```go
// TestKeyMapBindings cases:
"r": k.Refresh,

// TestKeyMapFullHelpIncludesPageAndMoveKeys expected membership:
for _, want := range []string{"i", "y", "r", "esc", "q"} {
    if !strings.Contains(joined, want) {
        t.Fatalf("FullHelp missing %q; got %s", want, joined)
    }
}
```

Then add:

```go
func TestRefreshKeyHelp(t *testing.T) {
    got := newKeyMap().Refresh.Help()
    if got != (key.Help{Key: "r", Desc: "refresh"}) { t.Fatalf("Refresh help = %+v", got) }
}
```

Append to `request_test.go`:

```go
func TestRefreshKeyScopeAndDynamicHelp(t *testing.T) {
    entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n")}
    m := settledReader(t, entry)
    m.updateKeymap()
    if !m.keys.Refresh.Enabled() || m.keys.Refresh.Help().Desc != "refresh" { t.Fatal("reader refresh disabled") }
    m.inputFocused = true
    m.updateKeymap()
    if m.keys.Refresh.Enabled() { t.Fatal("refresh enabled in input") }
    m.inputFocused, m.showingRaw = false, true
    m.updateKeymap()
    if m.keys.Refresh.Enabled() { t.Fatal("refresh enabled in source") }
    m.showingRaw = false
    m.requestFailure = &requestFailure{err: errors.New("timeout")}
    m.updateKeymap()
    if !m.keys.Refresh.Enabled() || m.keys.Refresh.Help().Desc != "retry" { t.Fatal("retry help not active") }
}

func TestRStartsInPlaceRefresh(t *testing.T) {
    fetch, seen := fetchRecorder("fresh\n")
    m := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")})
    m.common.fetch = fetch
    next, cmd := m.Update(tea.KeyPressMsg{Code: 'r'})
    got := next.(appModel)
    if got.pending == nil || got.pending.intent != requestRefresh || cmd == nil { t.Fatal("r did not start refresh") }
    runCmds(cmd)
    if len(*seen) != 1 || (*seen)[0] != "alice@plan.cat" { t.Fatalf("targets = %v", *seen) }
}
```

- [ ] **Step 2: Write failing warning tests**

```go
func TestFailedRefreshWarningCopy(t *testing.T) {
    entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
    m := settledReader(t, entry)
    m.common.width = 120
    m.requestFailure = &requestFailure{target: entry.Target, err: errors.New("read timed out")}
    bar := m.statusBarModel().render()
    for _, want := range []string{"refresh failed: read timed out", "showing previous response", "r retry"} {
        if !strings.Contains(bar, want) { t.Fatalf("bar %q missing %q", bar, want) }
    }
    m.requestFailure.retry = true
    bar = m.statusBarModel().render()
    if !strings.Contains(bar, "retry failed") || strings.Contains(bar, "showing previous response") { t.Fatalf("retry bar = %q", bar) }
}

func TestCancelledRetryRestoresWarning(t *testing.T) {
    entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
    m := settledReader(t, entry)
    failure := &requestFailure{target: entry.Target, err: errors.New("timeout")}
    m.requestFailure = failure
    m.pending = &pendingRequest{target: entry.Target, retry: true, cancel: func() {}}
    next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
    got := next.(appModel)
    if got.pending != nil || got.requestFailure != failure { t.Fatal("cancelled retry lost warning") }
}
```

- [ ] **Step 3: Run tests to verify RED**

```bash
go test ./tui -run 'Test(RefreshKeyHelp|RefreshKeyScope|RStartsInPlaceRefresh|FailedRefreshWarningCopy|CancelledRetryRestoresWarning)' -count=1 -v
```

Expected: build/behavioral failures.

- [ ] **Step 4: Add Refresh binding and action**

In `keys.go`, add and place the binding exactly as follows:

```go
type keyMap struct {
    Raw     key.Binding
    Refresh key.Binding
    Copy    key.Binding
}

// In newKeyMap:
Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),

// In FullHelp's first group:
{k.Open, k.FocusInput, k.Copy, k.Raw, k.Refresh},
```

Add these copy helpers to `app.go`:

```go
func (m appModel) refreshHelp() key.Help {
    if m.shouldRetry() { return key.Help{Key: "r", Desc: "retry"} }
    return key.Help{Key: "r", Desc: "refresh"}
}

func (m appModel) refreshHint() string {
    help := m.refreshHelp()
    return help.Key + " " + help.Desc
}
```

In `updateKeymap`, reset the help each time and enable it only over refreshable main content:

```go
refreshHelp := m.refreshHelp()
m.keys.Refresh.SetHelp(refreshHelp.Key, refreshHelp.Desc)

if m.pending != nil {
    for _, binding := range []*key.Binding{
        &m.keys.Open, &m.keys.FocusInput, &m.keys.Filter, &m.keys.Raw,
        &m.keys.Copy, &m.keys.Help, &m.keys.About, &m.keys.Move,
        &m.keys.Page, &m.keys.Jump, &m.keys.LinkNext, &m.keys.LinkPrev,
        &m.keys.LinkFinger, &m.keys.LinkPanel, &m.keys.Refresh,
    } {
        binding.SetEnabled(false)
    }
    m.keys.Back.SetEnabled(true)
    m.keys.Quit.SetEnabled(true)
    m.keys.ForceQuit.SetEnabled(true)
    return
}

canRefresh := content && hasResult &&
    (m.state == stateReader || m.state == stateList) &&
    !m.showingRaw && !m.showingLinks &&
    !(m.state == stateList && m.list.filtering())
m.keys.Refresh.SetEnabled(canRefresh)
```

Do not add `!m.help` to `canRefresh`: the help branch in `handleKey` swallows every key, so `r` only closes help and never starts a request, while the expanded help can still advertise the underlying screen's Refresh/Retry action. About, source, links, input, and active filter modes all disable the binding. Ensure the pending branch disables Refresh.

In `handleKey` add:

```go
case key.Matches(msg, m.keys.Refresh):
    return true, m, m.refreshCurrent()
```

- [ ] **Step 5: Add contextual status/help copy**

Use `m.refreshHint()` in the three main-content status branches:

```go
// stateList, before the optional generic-list source hint:
parts := []string{"↵ go", "/ filter", m.refreshHint()}

// focused reader link, after the existing tab hint:
extra = append(extra, "tab next", m.refreshHint())

// ordinary reader:
bar.hints = joinHints([]string{"↑↓ scroll", m.refreshHint()}, bar.escTarget)
```

Preserve `joinHints` breadcrumb behavior. The full help panel filters Refresh through the enabled state, except that it intentionally remains enabled for display while the help overlay itself swallows keys as described in Step 4.

Update `TestReaderFocusedLinkStatus`'s four exact expected strings by appending ` · r refresh`, and add these exact resting-status assertions to `request_test.go`:

```go
func TestMainContentRefreshStatus(t *testing.T) {
    reader := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n")})
    if got, want := reader.buildStatusBar().hints, "↑↓ scroll · r refresh · esc back · ? help"; got != want {
        t.Fatalf("reader hints = %q, want %q", got, want)
    }
    failed := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Err: errors.New("dial failed")})
    if got, want := failed.buildStatusBar().hints, "↑↓ scroll · r retry · esc back · ? help"; got != want {
        t.Fatalf("error hints = %q, want %q", got, want)
    }
    listed := settledList(t)
    if got, want := listed.buildStatusBar().hints, "↵ go · / filter · r refresh · esc back · ? help"; got != want {
        t.Fatalf("list hints = %q, want %q", got, want)
    }
}
```

Keep exact string equality in the neighboring tests; do not weaken them to substring-only assertions.

- [ ] **Step 6: Implement warning precedence and clearing**

Add:

```go
func (m *appModel) clearRequestFailure() { m.requestFailure = nil }
```

Place `m.clearRequestFailure()` at the start of these exact transitions:

```go
func (m *appModel) stepBack() {
    m.clearRequestFailure()
    m.showingRaw = false
    m.showingLinks = false
    if m.pos < 0 { return }
    m.snapshot()
    m.pos--
    if m.pos < 0 {
        m.gotoLanding()
        return
    }
    m.restore(m.history[m.pos])
}

func (m *appModel) focusInput() tea.Cmd {
    m.clearRequestFailure()
    if m.pos >= 0 { m.input.SetValue(m.history[m.pos].entry.Target.Raw) }
    m.inputFocused = true
    m.input.CursorEnd()
    m.resize()
    return m.input.Focus()
}

func (m *appModel) openAbout() {
    m.clearRequestFailure()
    m.flash = ""
    m.aboutFromState = m.state
    m.state = stateAbout
    m.resize()
}

func (m *appModel) enterRaw() {
    if m.pos < 0 { return }
    m.clearRequestFailure()
    m.reader.setRaw(m.history[m.pos].entry.Body)
    m.state = stateReader
    m.showingRaw = true
}

case key.Matches(msg, m.keys.LinkPanel) && m.pos >= 0:
    m.clearRequestFailure()
    node := m.history[m.pos]
    m.showingLinks = true
    m.linksPanel = newLinksPanel(m.common, node.links)
    m.linksPanel.setSize(m.common.width, m.common.bodyHeight())
    return true, m, nil
```

Do not call it from help, scrolling, `startRequest`, or `cancelRequest`. `landNavigation` and the usable-result branch of `landRefresh` already clear it in Task 2; the empty-body failure branch replaces it with the latest error.

Status precedence is pending → flash → request failure → resting:

```go
if m.pending != nil {
    return statusBar{width: m.common.width, styles: m.common.styles, hints: m.pendingStatus(time.Now())}
}
bar := m.buildStatusBar()
if m.flash != "" { bar.hints = m.flash; return bar }
if m.requestFailure != nil { bar.hints = m.requestFailure.statusText() }
return bar
```

- [ ] **Step 7: Complete interaction coverage**

Add these scope and literal-input tests to `request_test.go` (add `charm.land/bubbles/v2/key` and `github.com/charmbracelet/x/ansi` imports):

```go
func helpText(m appModel) string {
    m.updateKeymap()
    return ansi.Strip(m.helpView())
}

func TestRefreshHelpContexts(t *testing.T) {
    reader := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n")})
    if got := helpText(reader); !strings.Contains(got, "r refresh") { t.Fatalf("reader help:\n%s", got) }

    failed := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Err: errors.New("dial failed")})
    if got := helpText(failed); !strings.Contains(got, "r retry") { t.Fatalf("error help:\n%s", got) }

    warning := reader
    warning.requestFailure = &requestFailure{err: errors.New("timeout")}
    if got := helpText(warning); !strings.Contains(got, "r retry") { t.Fatalf("warning help:\n%s", got) }

    raw := reader
    raw.enterRaw()
    about := reader
    about.openAbout()
    linked := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("https://example.com\n")})
    next, _ := linked.Update(tea.KeyPressMsg{Code: 'L'})
    links := next.(appModel)
    filtering := settledList(t)
    next, _ = filtering.Update(tea.KeyPressMsg{Code: '/'})
    filtering = next.(appModel)
    for name, model := range map[string]appModel{"source": raw, "about": about, "links": links, "filter": filtering} {
        got := helpText(model)
        if strings.Contains(got, "r refresh") || strings.Contains(got, "r retry") {
            t.Fatalf("%s help advertises refresh:\n%s", name, got)
        }
    }
}

func TestRIsLiteralInTargetAndFilterInputs(t *testing.T) {
    target := newApp(stubFetch(t), colorprofile.NoTTY)
    next, _ := target.Update(tea.KeyPressMsg{Code: 'r'})
    gotTarget := next.(appModel)
    if gotTarget.input.Value() != "r" || gotTarget.pending != nil { t.Fatalf("target r = %q pending %#v", gotTarget.input.Value(), gotTarget.pending) }

    filtered := settledList(t)
    next, _ = filtered.Update(tea.KeyPressMsg{Code: '/'})
    filtered = next.(appModel)
    next, _ = filtered.Update(tea.KeyPressMsg{Code: 'r'})
    gotFilter := next.(appModel)
    if gotFilter.list.list.FilterValue() != "r" || gotFilter.pending != nil { t.Fatalf("filter r = %q pending %#v", gotFilter.list.list.FilterValue(), gotFilter.pending) }
}

func TestRWhileHelpOpenOnlyClosesHelp(t *testing.T) {
    m := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n")})
    m.help = true
    m.helpModel.ShowAll = true
    next, cmd := m.Update(tea.KeyPressMsg{Code: 'r'})
    got := next.(appModel)
    if got.help || got.pending != nil || cmd != nil { t.Fatalf("r in help = help %v pending %#v cmd %T", got.help, got.pending, cmd) }
}
```

Add these warning lifecycle tests:

```go
func modelWithWarning(t *testing.T) appModel {
    t.Helper()
    m := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte(strings.Repeat("line\n", 30))})
    m.requestFailure = &requestFailure{target: m.history[0].entry.Target, err: errors.New("old error")}
    return m
}

func TestWarningSurvivesScrollAndHelpCycle(t *testing.T) {
    m := modelWithWarning(t)
    failure := m.requestFailure
    next, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
    m = next.(appModel)
    next, _ = m.Update(tea.KeyPressMsg{Code: '?'})
    m = next.(appModel)
    next, _ = m.Update(tea.KeyPressMsg{Code: '?'})
    got := next.(appModel)
    if got.help || got.requestFailure != failure { t.Fatalf("warning after scroll/help = help %v warning %#v", got.help, got.requestFailure) }
}

func TestWarningClearsOnExplicitTransitions(t *testing.T) {
    tests := map[string]func(appModel) appModel{
        "back": func(m appModel) appModel { m.stepBack(); return m },
        "input": func(m appModel) appModel { _ = m.focusInput(); return m },
        "about": func(m appModel) appModel { m.openAbout(); return m },
        "source": func(m appModel) appModel { m.enterRaw(); return m },
    }
    for name, transition := range tests {
        got := transition(modelWithWarning(t))
        if got.requestFailure != nil { t.Fatalf("%s retained warning %#v", name, got.requestFailure) }
    }

    links := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("https://example.com\n")})
    links.requestFailure = &requestFailure{err: errors.New("old error")}
    next, _ := links.Update(tea.KeyPressMsg{Code: 'L'})
    if got := next.(appModel); got.requestFailure != nil { t.Fatalf("links retained warning %#v", got.requestFailure) }

    navigation := deliverNavigation(modelWithWarning(t), Entry{Target: hostTarget(t, "bob@plan.cat"), Err: errors.New("new navigation error")})
    if navigation.requestFailure != nil { t.Fatalf("navigation retained warning %#v", navigation.requestFailure) }

    refreshed := deliverRefresh(modelWithWarning(t), Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("fresh\n")}, true)
    if refreshed.requestFailure != nil { t.Fatalf("usable refresh retained warning %#v", refreshed.requestFailure) }
}

func TestRepeatedRetryFailureUsesLatestError(t *testing.T) {
    m := modelWithWarning(t)
    got := deliverRefresh(m, Entry{Target: m.history[0].entry.Target, Err: errors.New("new error")}, true)
    if got.requestFailure == nil || !got.requestFailure.retry || got.requestFailure.err.Error() != "new error" || string(got.history[0].entry.Body) == "" {
        t.Fatalf("repeated failure = warning %#v entry %#v", got.requestFailure, got.history[0].entry)
    }
}
```

Add a pending-keymap test; include every `keyMap` field so a future enabled binding cannot silently escape the modal:

```go
func TestPendingKeymapEnablesOnlyCancellationAndQuit(t *testing.T) {
    m := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n")})
    m.pending = &pendingRequest{id: 1, cancel: func() {}}
    m.updateKeymap()
    bindings := map[string]key.Binding{
        "focus": m.keys.FocusInput, "back": m.keys.Back, "open": m.keys.Open,
        "filter": m.keys.Filter, "raw": m.keys.Raw, "refresh": m.keys.Refresh,
        "copy": m.keys.Copy, "help": m.keys.Help, "about": m.keys.About,
        "quit": m.keys.Quit, "force-quit": m.keys.ForceQuit, "move": m.keys.Move,
        "page": m.keys.Page, "jump": m.keys.Jump, "link-next": m.keys.LinkNext,
        "link-prev": m.keys.LinkPrev, "link-finger": m.keys.LinkFinger,
        "link-panel": m.keys.LinkPanel,
    }
    for name, binding := range bindings {
        want := name == "back" || name == "quit" || name == "force-quit"
        if binding.Enabled() != want { t.Fatalf("%s enabled=%v, want %v", name, binding.Enabled(), want) }
    }
}
```

The q/Ctrl+C cancel-before-quit test is `TestPendingQuitKeysCancelBeforeQuit` from Task 1; keep it in the Task 4 focused run.

- [ ] **Step 8: Verify Task 4**

```bash
gofmt -w tui/keys.go tui/keys_test.go tui/app.go tui/app_test.go tui/request_test.go
go test ./tui -run 'Test(Refresh|RStarts|RIsLiteral|FailedRefresh|CancelledRetry|Warning|RepeatedRetry|MainContentRefreshStatus|QuestionMark|HelpPanel|KeyMap|Pending)' -count=1 -v
go test ./tui -count=1
```

Expected: all pass.

- [ ] **Step 9: Conditional commit checkpoint**

If commits are explicitly authorized:

```bash
git add tui/keys.go tui/keys_test.go tui/app.go tui/app_test.go tui/request_test.go
git commit -m "feat(tui): add refresh and retry controls"
```

Otherwise skip it.

---

### Task 5: Documentation and full verification

**Files:**
- Modify: `README.md`
- Modify: `docs/user-facing-messages.md`
- Test: repository-wide gates through `Makefile`

**Interfaces:**
- Consumes: final exact copy from Tasks 1–4.
- Produces: accurate usage guidance/message inventory; no Go API.

- [ ] **Step 1: Update README usage**

Replace the TUI usage paragraph with:

```markdown
Type a target and press Enter to fetch it. Finger a bare `@host` and, when it answers with a list of users, lookit opens that list: move with the arrows, `/` to filter, Enter to finger whoever's highlighted. Enter on a user drills in, Esc walks back through where you've been, and `r` refreshes the current response or retries a failed lookup. While a request is loading, Esc cancels it. Ctrl+C always quits.
```

- [ ] **Step 2: Update the message inventory**

In `docs/user-facing-messages.md`:

- replace loading copy with `<spinner> loading <target> · <elapsed> · esc cancel · q quit` and note elapsed starts after one second;
- add status/help rows for `r refresh`, `r retry`, `refresh failed: <error> · showing previous response · r retry`, and `retry failed: <error> · r retry`;
- correct touched legacy `r raw` entries to `v view source`;
- correct the generic-list note to “press v to view source”;
- note that explicit TUI cancellation suppresses `context.Canceled` and discarded partial bodies.

Use source filenames (`tui/request.go`, `tui/app.go`, `tui/keys.go`) rather than inventing new line numbers.

- [ ] **Step 3: Run focused packages**

```bash
go test ./tui -count=1
go test ./finger ./render -count=1
```

Expected: all pass.

- [ ] **Step 4: Run the canonical gate**

```bash
make check
```

Expected: `go vet ./...`, empty `gofmt -l .`, `golangci-lint run ./...`, and `go test ./... -race` all pass.

- [ ] **Step 5: Inspect final scope**

```bash
git diff --check
git status --short
git diff --stat
rg -n 'loadingTarget|loading[[:space:]]+bool|startFetch' tui --glob '*.go'
```

Expected: no whitespace errors; only planned files plus approved spec/plan are changed; the final `rg` prints nothing; no `finger/` or `render/` source changes exist.

- [ ] **Step 6: Conditional documentation commit**

If commits are explicitly authorized:

```bash
git add README.md docs/user-facing-messages.md
git commit -m "docs: document request controls"
```

Otherwise skip it. Do not push, open a PR, or enable auto-merge without separate explicit approval.

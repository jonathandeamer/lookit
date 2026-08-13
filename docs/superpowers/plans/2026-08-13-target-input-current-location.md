# Active Target Address and Response Chrome Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the target row consistently show the active navigation address, remove the duplicate reader receipt and success sparkle, and show landed response latency as low-priority status metadata.

**Architecture:** `appModel` continues to own the input and derives its value from settled, draft, or pending navigation state. `render/` becomes body-only; `readerModel` overlays links directly on that output, while `statusBar` reads `Entry.Meta.Elapsed` and includes it only when the complete ordinary bar fits without truncation.

**Tech Stack:** Go 1.26 toolchain, Bubble Tea/Bubbles/Lip Gloss v2 in `tui/`, Lip Gloss v1 in `render/`, injected-fetch model tests, renderer golden tests.

## Global Constraints

- Preserve `finger/` → `render/` → `tui/`; add no dependency and do not migrate either Lip Gloss major version.
- Keep the target prompt, syntax placeholder, 256-character limit, focus keys, history shape, routing, sanitization, port-79 pinning, bookmarks, and catalog behavior unchanged.
- Settled startpage means an empty address; settled history means its `Target.Raw`; focused input owns its draft; pending navigation means pending `Target.Raw`.
- Content-originated cancellation restores the visible address; input-originated cancellation preserves submitted text as a focused draft.
- Keep the structured breadcrumb, flags, navigation preview, page/scroll values, response size/count, priority warnings, and action hints.
- Omit latency before it causes information that fit without latency to truncate. Do not change the pending request's live elapsed counter.
- Add no success sparkle, `ok`, or success badge. Partial and failed outcomes remain explicit.
- Tests remain offline and use existing fakes. Run `make check` before each implementation commit.
- Use Conventional Commits with no AI/co-author trailers. Commit steps require explicit authorization at execution time; pushing and PR work require separate approval.

---

### Task 1: Make the target row the active navigation address

**Files:**
- Modify: `tui/app.go:278-334,468-518,703-785,1017-1058`
- Modify: `tui/request.go:50-91`
- Test: `tui/app_test.go`
- Test: `tui/request_test.go`

**Interfaces:**
- Consumes: `pendingRequest.target`, `pendingRequest.intent`, `pendingRequest.returnToInput`, `appModel.history`, and `appModel.pos`.
- Produces: `func (m *appModel) setAddress(raw string)` and `func (m *appModel) restoreVisibleAddress()`.
- Preserves: `startRequest`, `cancelRequest`, history nodes, fetch routing, and all focus behavior except draft cancellation now restores the settled address.

- [ ] **Step 1: Add failing request-state tests**

Add to `tui/request_test.go`:

```go
func TestStartRequestPublishesNavigationAddress(t *testing.T) {
	old := Entry{Target: hostTarget(t, "old@plan.cat"), Body: []byte("old\n")}
	m := settledReader(t, old)
	next := hostTarget(t, "new@plan.cat")

	_ = m.startRequest(next, requestNavigate, false)
	t.Cleanup(m.pending.cancel)

	if got := m.input.Value(); got != next.Raw {
		t.Fatalf("input = %q, want pending target %q", got, next.Raw)
	}
}

func TestCancelContentNavigationRestoresVisibleAddress(t *testing.T) {
	old := Entry{Target: hostTarget(t, "old@plan.cat"), Body: []byte("old\n")}
	m := settledReader(t, old)
	_ = m.startRequest(hostTarget(t, "new@plan.cat"), requestNavigate, false)

	if cmd := m.cancelRequest(); cmd != nil {
		t.Fatal("content cancellation unexpectedly focused the input")
	}
	if got := m.input.Value(); got != old.Target.Raw {
		t.Fatalf("input = %q, want visible target %q", got, old.Target.Raw)
	}
}

func TestCancelStartpageNavigationRestoresEmptyAddress(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	_ = m.startRequest(hostTarget(t, "alice@plan.cat"), requestNavigate, false)

	_ = m.cancelRequest()
	if got := m.input.Value(); got != "" {
		t.Fatalf("input = %q, want empty startpage address", got)
	}
}
```

Extend `TestCancelSubmittedTargetOverContentRestoresEditor` after its existing
assertions:

```go
next, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
got = next.(appModel)
if got.inputFocused || got.input.Value() != old.Target.Raw {
	t.Fatalf("cancel draft = focused %v value %q, want blurred %q", got.inputFocused, got.input.Value(), old.Target.Raw)
}
```

- [ ] **Step 2: Add failing route/restoration assertions**

Strengthen these existing `tui/app_test.go` tests:

```go
// TestEnterInListDrillsIntoUser, after the pending assertion:
if got.input.Value() != "alrs@tilde.team" {
	t.Fatalf("pending input = %q, want alrs@tilde.team", got.input.Value())
}

// TestReaderEnterDefiniteFingersFocusedLink and TestAboutEnterFingersAuthor:
if got.input.Value() != got.pending.target.Raw {
	t.Fatalf("input = %q, want pending target %q", got.input.Value(), got.pending.target.Raw)
}

// TestStartEnterRequestsSelectedTarget: retain Update's model and assert:
next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
got := next.(appModel)
if got.input.Value() != selected.target {
	t.Fatalf("input = %q, want selected target %q", got.input.Value(), selected.target)
}

// TestEscInDrilledReaderRestoresList:
if got.input.Value() != host.Raw {
	t.Fatalf("input = %q, want restored list target %q", got.input.Value(), host.Raw)
}

// TestHomeTruncatesHistory:
if m.input.Value() != "" {
	t.Fatalf("home input = %q, want empty", m.input.Value())
}
```

Also require the landed model in `TestEnterInListDrillsIntoUser` to contain
`"alrs@tilde.team"`, separately pinning pending and settled writes.

Add these remaining transition regressions to `tui/request_test.go`:

```go
func TestRefreshKeepsActiveAddress(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	m := deliverNavigation(newApp(stubFetch(t), colorprofile.NoTTY), Entry{Target: target, Body: []byte("old\n")})
	_ = m.refreshCurrent()
	if got := m.input.Value(); got != target.Raw {
		t.Fatalf("pending refresh input = %q, want %q", got, target.Raw)
	}
	next, _ := m.Update(fetchResultMsg{reqID: m.pending.id, entry: Entry{Target: target, Body: []byte("fresh\n")}})
	got := next.(appModel)
	if got.input.Value() != target.Raw {
		t.Fatalf("landed refresh input = %q, want %q", got.input.Value(), target.Raw)
	}
}

func TestTransientViewsDoNotChangeAddress(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	m := deliverNavigation(newApp(stubFetch(t), colorprofile.NoTTY), Entry{Target: target, Body: []byte("Plan: hi\n")})
	m.openHelp()
	m.closeHelp()
	m.openAbout()
	m.closeAbout()
	m.enterRaw()
	m.exitRaw()
	m.showingLinks = true
	m.showingLinks = false
	if got := m.input.Value(); got != target.Raw {
		t.Fatalf("transient views changed input to %q, want %q", got, target.Raw)
	}
}
```

Extend `TestNavigationFailureStillPushesErrorNode` to require the failed
entry's `Target.Raw` in the input. Existing parse-error and placeholder tests
already pin that invalid input stays a draft and an empty draft shows the
syntax hint; retain them unchanged.

- [ ] **Step 3: Verify the tests fail for the old behavior**

Run:

```bash
go test ./tui -run 'Test(StartRequestPublishesNavigationAddress|CancelContentNavigationRestoresVisibleAddress|CancelStartpageNavigationRestoresEmptyAddress|CancelSubmittedTargetOverContentRestoresEditor|EnterInListDrillsIntoUser|ReaderEnterDefiniteFingersFocusedLink|AboutEnterFingersAuthor|StartEnterRequestsSelectedTarget|EscInDrilledReaderRestoresList|HomeTruncatesHistory|RefreshKeepsActiveAddress|TransientViewsDoNotChangeAddress|NavigationFailureStillPushesErrorNode)$' -count=1 -v
```

Expected: FAIL because content navigation does not publish/restore the address,
land/back do not synchronize it, and Esc leaves the edited draft blurred.

- [ ] **Step 4: Add the address helpers**

Place beside `focusInput` in `tui/app.go`:

```go
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
```

Use `restoreVisibleAddress` in `focusInput`. Replace `gotoStart`'s direct
`SetValue("")` with `setAddress("")`.

- [ ] **Step 5: Synchronize request start and cancellation**

In `startRequest`, capture focus origin and publish only navigation targets:

```go
returnToInput := m.inputFocused
if intent == requestNavigate {
	m.setAddress(target.Raw)
}
m.pending = &pendingRequest{
	id: m.reqSeq, target: target, intent: intent, retry: retry,
	returnToInput: returnToInput, started: time.Now(), cancel: cancel,
}
```

In `cancelRequest`, restore only content-originated requests:

```go
pending.cancel()
if !pending.returnToInput {
	m.restoreVisibleAddress()
	return nil
}
m.setInputFocused(true)
m.input.CursorEnd()
m.resize()
return m.input.Focus()
```

The input-origin branch deliberately retains the already-published submitted
value as a retryable draft.

- [ ] **Step 6: Synchronize land, restore, and Esc-from-input**

Write `n.entry.Target.Raw` at the start of `restore`; write
`routed.node.entry.Target.Raw` at the start of `showRouted`. In the focused
input's Esc branch, call `restoreVisibleAddress()` before `blurInput()`:

```go
case key.Matches(msg, m.keys.Back):
	if m.pos < 0 {
		return true, m, tea.Quit
	}
	m.restoreVisibleAddress()
	m.blurInput()
	return true, m, nil
```

Do not change parse-error handling. `showRouted` also covers a newly landed
navigation error entry.

- [ ] **Step 7: Format and verify Task 1**

Run:

```bash
gofmt -w tui/app.go tui/request.go tui/app_test.go tui/request_test.go
go test ./tui -run 'Test(StartRequestPublishesNavigationAddress|CancelContentNavigationRestoresVisibleAddress|CancelStartpageNavigationRestoresEmptyAddress|CancelSubmittedTargetOverContentRestoresEditor|EnterInListDrillsIntoUser|ReaderEnterDefiniteFingersFocusedLink|AboutEnterFingersAuthor|StartEnterRequestsSelectedTarget|EscInDrilledReaderRestoresList|HomeTruncatesHistory|RefreshKeepsActiveAddress|TransientViewsDoNotChangeAddress|NavigationFailureStillPushesErrorNode)$' -count=1 -v
make check
```

Expected: focused tests and all four repository gates pass.

- [ ] **Step 8: Commit Task 1**

After explicit commit authorization:

```bash
git add tui/app.go tui/request.go tui/app_test.go tui/request_test.go
git commit -m "fix(tui): synchronize the active target address"
```

---

### Task 2: Remove the reader receipt and move latency into status

**Files:**
- Delete: `render/chrome.go`
- Modify: `render/render.go`, `render/theme.go`, `render/render_test.go`, `render/theme_test.go`, `render/tildeteam_test.go`
- Modify: `render/testdata/*.golden`
- Modify: `tui/reader.go`, `tui/reader_test.go`, `tui/links.go`
- Modify: `tui/statusbar.go`, `tui/statusbar_test.go`
- Modify: `tui/app.go`, `tui/app_test.go`
- Modify: `CLAUDE.md`

**Interfaces:**
- Consumes: `Entry.Target`, `Entry.Body`, `Entry.Err`, `Entry.Meta.Elapsed`, existing status fields, and `applyLinkOverlay`.
- Produces: `formatElapsed(time.Duration) string`, `statusBar.latency`, `statusBar.rightParts(bool)`, and `statusBar.fullWidth([]string, string)`.
- Changes: `render.Render` becomes `Render(finger.Target, []byte, error, colorprofile.Profile) string`; `RenderWithBackground` becomes `RenderWithBackground(finger.Target, []byte, error, colorprofile.Profile, bool) string` because metadata no longer belongs to body rendering.
- Removes: `renderHeader`, `render.Split`, `Theme.Arrow`, `Theme.Latency`, and `Theme.Sparkle`. Retain `Theme.Target` for CLI help/version.

- [ ] **Step 1: Add failing body-first and link-offset tests**

Replace `TestSplit_HeaderAndBodyRoundtrip` in `render/render_test.go`:

```go
func TestRenderStartsWithResponseBody(t *testing.T) {
	target := finger.Target{HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	got := RenderWithBackground(target, []byte("Login: alice\nPlan:\nHello\n"), finger.Meta{Elapsed: 123 * time.Millisecond}, nil, colorprofile.NoTTY, true)
	if !strings.HasPrefix(got, "Login: alice\n") {
		t.Fatalf("rendered response starts with chrome: %q", got)
	}
	for _, forbidden := range []string{"➜", "✦", "123ms"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("rendered response contains receipt %q: %q", forbidden, got)
		}
	}
}
```

Add to `tui/reader_test.go` (and import `github.com/charmbracelet/x/ansi`):

```go
func TestReaderLinkCanOccupyFirstResponseLine(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil { t.Fatal(err) }
	const raw = "alice@example.com"
	m.focusedLink = 0
	m.setEntryWithLinks(Entry{Target: target, Body: []byte(raw + "\nrest\n")}, []Link{{Kind: LinkFinger, Action: ActionCopy, Raw: raw}})
	first, _, _ := strings.Cut(ansi.Strip(m.viewport.View()), "\n")
	if got := strings.TrimRight(first, " "); got != raw {
		t.Fatalf("reader first line = %q, want body link %q", got, raw)
	}
}

func TestReaderFocusedLinkScrollHasNoHeaderOffset(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil { t.Fatal(err) }
	const raw = "alice@example.com"
	m.setSize(40, 2)
	m.focusedLink = 0
	m.setEntryWithLinks(Entry{Target: target, Body: []byte("zero\none\ntwo\n" + raw + "\ntail\ntail\n")}, []Link{{Kind: LinkFinger, Action: ActionCopy, Raw: raw}})
	if got := m.viewport.YOffset(); got != 2 {
		t.Fatalf("YOffset = %d, want 2 without a header offset", got)
	}
}
```

- [ ] **Step 2: Add failing elapsed/status tests**

Add to `tui/statusbar_test.go` and import `time`. Exercise latency only
through the existing `appModel`/`statusBar.render` surface so the red phase
compiles and fails on missing behavior rather than a not-yet-added field or
helper:

```go
func TestStatusBarFormatsLandedLatency(t *testing.T) {
	tests := []struct{ in time.Duration; want string }{
		{500 * time.Microsecond, "500µs"},
		{42 * time.Millisecond, "42ms"},
		{1500 * time.Millisecond, "1.50s"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			m := settledReader(t, Entry{
				Target: hostTarget(t, "alice@plan.cat"),
				Body:   []byte("Plan: hi\n"),
				Meta:   finger.Meta{Elapsed: tt.in},
			})
			m.common.width = 100
			if got := ansi.Strip(m.buildStatusBar().render()); !strings.Contains(got, tt.want+" · 9 B") {
				t.Errorf("status bar = %q, want formatted latency %q before byte count", got, tt.want)
			}
		})
	}
}

func TestStatusBarShowsLandedLatencyWhenItFits(t *testing.T) {
	m := settledReader(t, Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("Plan: hi\n"),
		Meta:   finger.Meta{Elapsed: 123 * time.Millisecond},
	})
	m.common.width = 100
	if got := ansi.Strip(m.buildStatusBar().render()); !strings.Contains(got, "123ms · 9 B") {
		t.Fatalf("status bar missing latency: %q", got)
	}
}

func TestStatusBarDropsLatencyBeforeExistingInformation(t *testing.T) {
	m := settledReader(t, Entry{
		Target: hostTarget(t, "alice@x.example"),
		Body:   []byte("Plan: hi\n"),
		Meta:   finger.Meta{Elapsed: 123 * time.Millisecond},
	})
	m.common.width = lipgloss.Width("@x.example / alice") + 1 + lipgloss.Width("9 B · ↑↓ scroll · r refresh · ? help")
	got := ansi.Strip(m.buildStatusBar().render())
	if strings.Contains(got, "123ms") || !strings.Contains(got, "9 B") || !strings.Contains(got, "? help") {
		t.Fatalf("latency displaced existing information: %q", got)
	}
}
```

Add to `tui/app_test.go`. Inspect rendered output through the existing API so
these tests also compile before production gains a latency field:

```go
func TestLandedReaderAndListExposeLatencyToStatusBar(t *testing.T) {
	readerTarget := hostTarget(t, "alice@plan.cat")
	reader := deliverNavigation(newApp(stubFetch(t), colorprofile.NoTTY), Entry{
		Target: readerTarget,
		Body:   []byte("Plan: hi\n"),
		Meta:   finger.Meta{Elapsed: 123 * time.Millisecond},
	})
	reader.common.width = 100
	if got := ansi.Strip(reader.buildStatusBar().render()); !strings.Contains(got, "123ms") {
		t.Fatalf("reader status = %q, want latency 123ms", got)
	}

	listTarget := hostTarget(t, "@tilde.team")
	list := deliverNavigation(newApp(stubFetch(t), colorprofile.NoTTY), Entry{
		Target: listTarget,
		Body:   []byte("Users currently online:\n\nalice bob\n"),
		Meta:   finger.Meta{Elapsed: 45 * time.Millisecond},
	})
	list.common.width = 100
	if got := ansi.Strip(list.buildStatusBar().render()); !strings.Contains(got, "45ms") {
		t.Fatalf("list status = %q, want latency 45ms", got)
	}
}

func TestStartAndAboutDoNotExposeResponseLatency(t *testing.T) {
	start := newApp(stubFetch(t), colorprofile.NoTTY)
	if got := ansi.Strip(start.buildStatusBar().render()); strings.Contains(got, "123ms") {
		t.Fatalf("startpage status unexpectedly carries response latency: %q", got)
	}
	target := hostTarget(t, "alice@plan.cat")
	about := deliverNavigation(start, Entry{Target: target, Body: []byte("Plan: hi\n"), Meta: finger.Meta{Elapsed: 123 * time.Millisecond}})
	about.openAbout()
	if got := ansi.Strip(about.buildStatusBar().render()); strings.Contains(got, "123ms") {
		t.Fatalf("about status unexpectedly carries response latency: %q", got)
	}
}
```

- [ ] **Step 3: Verify Task 2 tests fail**

Run:

```bash
go test ./render ./tui -run 'Test(RenderStartsWithResponseBody|ReaderLinkCanOccupyFirstResponseLine|ReaderFocusedLinkScrollHasNoHeaderOffset|StatusBarFormatsLandedLatency|StatusBarShowsLandedLatencyWhenItFits|StatusBarDropsLatencyBeforeExistingInformation|LandedReaderAndListExposeLatencyToStatusBar|StartAndAboutDoNotExposeResponseLatency)$' -count=1 -v
```

Expected: FAIL assertions (not compilation) because responses still have
receipts and header offsets, and the existing rendered status bar does not
show landed latency.

- [ ] **Step 4: Make `render/` body-only**

Use these signatures in `render/render.go` and remove the `renderHeader` call:

```go
func Render(t finger.Target, body []byte, queryErr error, profile colorprofile.Profile) string {
	return RenderWithBackground(t, body, queryErr, profile, lipgloss.HasDarkBackground())
}

func RenderWithBackground(t finger.Target, body []byte, queryErr error, profile colorprofile.Profile, darkBackground bool) string {
	theme := NewThemeWithBackground(profile, darkBackground)
	var sb strings.Builder
	if len(body) == 0 && queryErr == nil {
		sb.WriteString(theme.Footer.Render("(no response body)"))
		sb.WriteByte('\n')
	} else {
		if isTildeTeam(t) { body = reflowPronouns(body) }
		sb.WriteString(highlightFields(theme, body, extraFieldPrefixes(t)))
		if len(body) > 0 && body[len(body)-1] != '\n' { sb.WriteByte('\n') }
	}
	if queryErr != nil {
		sb.WriteString(theme.ErrLine.Render(queryErr.Error()))
		sb.WriteByte('\n')
	}
	return sb.String()
}
```

Update `TestRenderStartsWithResponseBody` to call the new body-only
`RenderWithBackground(target, body, nil, colorprofile.NoTTY, true)` signature
as part of this production change.

Delete `render/chrome.go` and `Split`. Remove `Arrow`, `Latency`, and `Sparkle`
from `Theme`; retain `Target`. Remove `AccentViolet` from `renderPalette` and
the dark/light violet contrast cases because `Arrow` was its only consumer. Update renderer tests,
`tildeteam_test.go`, and all reader calls to omit the `finger.Meta` argument;
remove the now-unused `time` import from `render/render_test.go`.

- [ ] **Step 5: Remove header assumptions from `readerModel`**

Change the link-aware path to:

```go
rendered := render.RenderWithBackground(entry.Target, entry.Body, entry.Err, m.profile, m.darkBackground)
rendered = applyLinkOverlay(rendered, links, m.focusedLink, m.styles)
m.viewport.SetContent(rendered)
m.scrollToFocusedLink(links)
```

Change `scrollToFocusedLink` to accept only `links`, calculate
`offset := bodyLine - m.viewport.Height()/2`, and retain the zero clamp. Update
`setProfile`, `setBackground`, and `renderEntry` for the renderer signature.
Update `tui/links.go` so `applyLinkOverlay` says it operates on the complete
rendered response, while links still originate only from sanitized `Entry.Body`.

- [ ] **Step 6: Add optional landed latency to `statusBar`**

Import `time`, add `latency string` between `scroll` and `meta`, and move the
deleted formatter into `tui/statusbar.go`:

```go
func formatElapsed(d time.Duration) string {
	if d < time.Millisecond { return fmt.Sprintf("%dµs", d.Microseconds()) }
	if d < time.Second { return fmt.Sprintf("%dms", d.Milliseconds()) }
	return fmt.Sprintf("%.2fs", d.Seconds())
}
```

Extract the current right-side assembly into this helper, preserving all
existing segment order while inserting latency before response metadata:

```go
func (b statusBar) rightParts(includeLatency bool) []string {
	var right []string
	if b.escTarget != "" { right = append(right, "◂ esc: "+b.escTarget) }
	if b.page != "" { right = append(right, b.page) }
	if b.scroll != "" { right = append(right, b.scroll) }
	if includeLatency && b.latency != "" { right = append(right, b.latency) }
	if b.meta != "" { right = append(right, b.meta) }
	if b.hints != "" { right = append(right, b.hints) }
	return right
}
```

Add this exact fit helper:

```go
func (b statusBar) fullWidth(right []string, flags string) int {
	crumb := b.host
	if b.user != "" { crumb += " / " + b.user }
	leftWidth := lipgloss.Width(crumb) + lipgloss.Width(flags)
	rightWidth := lipgloss.Width(strings.Join(right, " · "))
	if leftWidth > 0 && rightWidth > 0 { return leftWidth + 1 + rightWidth }
	return leftWidth + rightWidth
}
```

In `render`, keep latency only when the entire candidate fits:

```go
allFlags, _ := b.flagsWithin(b.width)
right := b.rightParts(false)
candidate := b.rightParts(true)
if b.latency != "" && b.fullWidth(candidate, allFlags) <= b.width {
	right = candidate
}
```

Then run the existing reservation/truncation/styling logic with `right`. Include
latency in `hasOrdinaryStatus`. Do not modify `pendingPriorityStatus`.

- [ ] **Step 7: Populate latency for landed screens**

In `buildStatusBar`, immediately after reading the history node:

```go
node := m.history[m.pos]
bar := statusBar{width: w, styles: st, latency: formatElapsed(node.entry.Meta.Elapsed)}
bar.host, bar.user = breadcrumbParts(node.entry.Target)
```

This deliberately makes latency available in reader, list, raw, focused-input,
and links-panel views. Startpage/About return before constructing it; priority
warnings retain their existing override behavior.

- [ ] **Step 8: Update goldens and living architecture documentation**

Run:

```bash
go test ./render -run 'TestRender_' -update -count=1
```

Inspect every changed golden: it must start with body, empty-response treatment,
or error, never a synthetic receipt. Update `CLAUDE.md` to document the new
renderer signatures, the active-address state model, the headerless reader, and
status-owned landed latency. Do not rewrite dated historical specs/decisions.

- [ ] **Step 9: Format and verify Task 2**

Run:

```bash
gofmt -w render/render.go render/render_test.go render/theme.go render/theme_test.go render/tildeteam_test.go tui/app.go tui/app_test.go tui/links.go tui/reader.go tui/reader_test.go tui/statusbar.go tui/statusbar_test.go
go test ./render ./tui -run 'Test(RenderStartsWithResponseBody|ReaderLinkCanOccupyFirstResponseLine|ReaderFocusedLinkScrollHasNoHeaderOffset|FormatElapsed|StatusBarShowsLandedLatencyWhenItFits|StatusBarDropsLatencyBeforeExistingInformation|LandedReaderAndListExposeLatencyToStatusBar|StartAndAboutDoNotExposeResponseLatency)$' -count=1 -v
go test ./render ./tui -count=1
make check
```

Expected: focused regressions, both packages, and all four repository gates pass.

- [ ] **Step 10: Commit Task 2**

After explicit commit authorization:

```bash
git add CLAUDE.md render tui/app.go tui/app_test.go tui/links.go tui/reader.go tui/reader_test.go tui/statusbar.go tui/statusbar_test.go
git commit -m "feat(tui): move response latency to the status bar"
```

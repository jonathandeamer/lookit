# Failed-Request Status Bar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On a request that produced no response, the status bar drops the byte count and the scroll hint and keeps `r retry`, instead of reporting `0 B · ↑↓ scrol…`.

**Architecture:** `Entry` gains a `failed()` predicate — an error with an empty body — which is the rule `appModel.shouldRetry` already spells out inline. `buildStatusBar`'s `stateReader` branch returns early for a failed entry with no `meta` and no `scroll`, so `r retry` inherits the width the two removed fields were spending. No new priority rule.

**Tech Stack:** Go, Bubble Tea v2, Lip Gloss v2.

**Spec:** `docs/superpowers/specs/2026-08-14-failed-request-status-bar-design.md`

## Global Constraints

- Do not touch `finger/` or any security invariant.
- One predicate, two call sites: the status bar and the retry keybinding must never disagree about whether a response landed.
- A response that arrived *and* errored (truncated read, reset after body) is **not** a failure: it keeps its byte count, scroll percentage, and `partial (truncated)` flag.
- `bar.latency` stays: it is a true measurement of the failed attempt and is already lowest priority, included only when the whole line fits.
- Do not change the refresh-failure priority status, the loading status, or any non-reader branch.
- Conventional Commits; no `Co-Authored-By` and no AI attribution trailers. Do not push or open a PR.

---

### Task 1: Extract the "no response landed" predicate

**Files:**
- Modify: `tui/fetch.go` (add the method beside `Entry`)
- Modify: `tui/app.go` (`appModel.shouldRetry`, around line 1082)
- Test: `tui/app_test.go`

**Interfaces:**
- Consumes: `type Entry struct { Target finger.Target; Body []byte; Meta finger.Meta; Err error }` in `tui/fetch.go`.
- Produces: `func (e Entry) failed() bool`.

- [ ] **Step 1: Write the failing test**

Add to `tui/app_test.go`:

```go
func TestEntryFailed(t *testing.T) {
	tests := []struct {
		name  string
		entry Entry
		want  bool
	}{
		{"error with no body", Entry{Err: errors.New("connection refused by 127.0.0.1:1")}, true},
		{"error with a body", Entry{Body: []byte("half a plan\n"), Err: errors.New("read timed out")}, false},
		{"body, no error", Entry{Body: []byte("a plan\n")}, false},
		{"empty success", Entry{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.entry.failed(); got != tc.want {
				t.Errorf("failed() = %v, want %v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and verify RED**

Run: `go test ./tui -run TestEntryFailed -count=1 -v`

Expected: FAIL — `entry.failed` is undefined.

- [ ] **Step 3: Add the predicate and use it in shouldRetry**

In `tui/fetch.go`, below the `Entry` type:

```go
// failed reports a request that produced no response at all. It is the
// difference between "the server said nothing" and "the server said nothing
// useful": an errored response that still carries bytes is a partial success,
// with a body to scroll and a byte count worth reporting.
//
// The status bar and the retry keybinding both key off this, so they cannot
// disagree about whether a response landed.
func (e Entry) failed() bool {
	return e.Err != nil && len(e.Body) == 0
}
```

In `tui/app.go`, `shouldRetry` becomes:

```go
func (m appModel) shouldRetry() bool {
	if m.requestFailure != nil {
		return true
	}
	if m.pos < 0 || m.pos >= len(m.history) {
		return false
	}
	return m.history[m.pos].entry.failed()
}
```

- [ ] **Step 4: Run the tests and verify GREEN**

Run: `go test ./tui -run 'TestEntryFailed|Retry|Refresh' -count=1`

Expected: PASS — `shouldRetry`'s behaviour is unchanged.

- [ ] **Step 5: Commit**

```bash
git add tui/fetch.go tui/app.go tui/app_test.go
git commit -m "refactor(tui): name the no-response-landed condition"
```

---

### Task 2: Drop the byte count and scroll hint on a failed request

**Files:**
- Modify: `tui/app.go` (`buildStatusBar`, the `default: // stateReader` branch)
- Test: `tui/statusbar_test.go`

**Interfaces:**
- Consumes: `Entry.failed()` from Task 1; `appModel.buildStatusBar() statusBar`; `joinHints(parts []string, escTarget string) string`; `appModel.refreshHint() string`; `statusBar.render() string`.
- Produces: no new exported surface.

- [ ] **Step 1: Write the failing tests**

Two package-level helpers already exist and must be reused rather than
reinvented: `settledReader(t *testing.T, entry Entry) appModel`
(`tui/request_test.go:67`) puts an entry on screen as a settled reader node,
and `hostTarget(t *testing.T, raw string) finger.Target` (`tui/list_test.go:17`)
parses a target. `settledReader` builds the model with `newApp`, so the width
is set afterwards via `m.common.width`, which `buildStatusBar` reads into
`bar.width`. Add to `tui/statusbar_test.go` (its imports already include
`strings`, `ansi`, and `finger`; add `errors` and `slices`):

```go
func TestStatusBarFailedRequestDropsBytesAndScroll(t *testing.T) {
	m := settledReader(t, Entry{
		Target: hostTarget(t, "nobody@127.0.0.1:1"),
		Err:    errors.New("connection refused by 127.0.0.1:1"),
	})
	bar := m.buildStatusBar()

	if bar.meta != "" {
		t.Errorf("meta = %q, want empty: no response landed, so there are no bytes to report", bar.meta)
	}
	if bar.scroll != "" {
		t.Errorf("scroll = %q, want empty: there is nothing to scroll", bar.scroll)
	}
	if !strings.Contains(bar.hints, "r retry") {
		t.Errorf("hints = %q, want them to include \"r retry\"", bar.hints)
	}
	if strings.Contains(bar.hints, "scroll") {
		t.Errorf("hints = %q, want no scroll hint", bar.hints)
	}
}

func TestStatusBarFailedRequestKeepsRetryAt45Columns(t *testing.T) {
	m := settledReader(t, Entry{
		Target: hostTarget(t, "nobody@127.0.0.1:1"),
		Err:    errors.New("connection refused by 127.0.0.1:1"),
	})
	m.common.width = 45
	bar := m.buildStatusBar()

	line := ansi.Strip(bar.render())
	if !strings.Contains(line, "r retry") {
		t.Errorf("45-column bar dropped the only useful action:\n%s", line)
	}
	if strings.Contains(line, "0 B") {
		t.Errorf("45-column bar still reports a byte count:\n%s", line)
	}
}

func TestStatusBarErroredResponseWithBodyKeepsBytes(t *testing.T) {
	m := settledReader(t, Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("half a plan\n"),
		Meta:   finger.Meta{Bytes: 12, Truncated: true},
		Err:    errors.New("plan.cat:79 stopped responding after 30s"),
	})
	bar := m.buildStatusBar()

	if bar.meta == "" {
		t.Error("a response with bytes must still report its byte count")
	}
	if !slices.Contains(bar.flags, "partial (truncated)") {
		t.Errorf("flags = %v, want the partial (truncated) flag", bar.flags)
	}
}
```

Keep the assertions exactly as written.

- [ ] **Step 2: Run the tests and verify RED**

Run: `go test ./tui -run 'TestStatusBarFailedRequest|TestStatusBarErroredResponseWithBody' -count=1 -v`

Expected: FAIL — `meta` is `0 B` and the hints still lead with `↑↓ scroll`.

- [ ] **Step 3: Return early for a failed entry**

In `tui/app.go`, at the top of the `default: // stateReader` branch of
`buildStatusBar`, before `bar.meta = formatBytes(...)`:

```go
	default: // stateReader
		if node.entry.failed() {
			// No response landed. A byte count would present the failure as an
			// empty-but-successful response — an outcome lookit is careful to
			// distinguish elsewhere ("partial (truncated)", "auto-detected") —
			// and there is nothing to scroll. Dropping both is also what makes
			// "r retry", the only useful action here, fit a narrow bar; no
			// priority rule is needed.
			bar.hints = joinHints([]string{m.refreshHint()}, bar.escTarget)
			return bar
		}
		bar.meta = formatBytes(len(node.entry.Body))
```

The early return is safe: a failed entry has no links to focus and cannot be
`Meta.Truncated`, which requires bytes.

- [ ] **Step 4: Run the tests and verify GREEN**

Run: `go test ./tui -run 'TestStatusBarFailedRequest|TestStatusBarErroredResponseWithBody' -count=1 -v`

Expected: PASS.

- [ ] **Step 5: Run the whole package**

Run: `go test ./tui -count=1 -race`

Expected: PASS. If an existing test asserted `0 B` on a failed request, that
assertion was encoding the bug — update it to the new expectation and say so in
the commit body.

- [ ] **Step 6: Run the gate and commit**

```bash
make check
git add tui/app.go tui/statusbar_test.go
git commit -m "fix(tui): stop reporting 0 B and a scroll hint on a failed request"
```

---

## Verification

- [ ] `make check` passes.
- [ ] A failed reader bar at 45 columns shows `◂ esc: … · r retry · ? help` with no `0 B` and no scroll hint.
- [ ] A truncated-but-present response is unchanged.

# Flash Consistency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the `y` copy key feedback when there's nothing to copy, and stop transient "copied …" flashes from bleeding across screen changes — without touching copy styling or parse-error persistence.

**Architecture:** Two small edits to `tui/app.go`'s existing `flash` mechanism: `copyAddress` flashes "nothing to copy" instead of silently returning `nil`; `back()` and `drill()` clear `m.flash` so confirmations stay tied to the screen that produced them. `focusInput()` is deliberately left alone (it's on the parse-error recovery path).

**Tech Stack:** Go, Bubble Tea v2 (`charm.land/bubbletea/v2`).

**Spec:** `docs/superpowers/specs/2026-06-01-flash-consistency-design.md`

---

## File Structure

- **Modify `tui/app.go`** — `copyAddress` (the `addr == ""` branch, ~lines 603-604), `back()` (~lines 248-254), `drill()` (~line 485, first statement). No new files; the `flash`/`clearFlashCmd` machinery already exists.
- **Modify `tui/app_test.go`** — append model tests for the new feedback and the clearing/preservation behaviour.

---

### Task 1: "nothing to copy" feedback

**Files:**
- Modify: `tui/app.go` (`copyAddress`, the `if addr == "" { return nil }` branch)
- Test: `tui/app_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `tui/app_test.go` (the file is `package tui` and already imports `colorprofile`, `tea`, `finger`, etc., and defines `stubFetch`/`hostTarget`):

```go
func TestCopyAddressNothingToCopy(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY) // landing: pos == -1, stateReader, no address
	cmd := (&m).copyAddress()
	if m.flash != "nothing to copy" {
		t.Fatalf("flash = %q, want %q", m.flash, "nothing to copy")
	}
	if cmd == nil {
		t.Fatal("copyAddress returned nil cmd; want a clear-flash command")
	}
}

func TestCopyAddressSuccessSetsCopiedFlash(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("Plan: hi\n"),
	}})
	m = step.(appModel)

	_ = (&m).copyAddress()
	if want := "copied alice@plan.cat"; m.flash != want {
		t.Fatalf("flash = %q, want %q", m.flash, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tui/ -run 'TestCopyAddressNothingToCopy|TestCopyAddressSuccessSetsCopiedFlash' -count=1 -v`
Expected: `TestCopyAddressNothingToCopy` FAILS (`flash = "" , want "nothing to copy"` — current code returns `nil` without setting flash). `TestCopyAddressSuccessSetsCopiedFlash` should already PASS (it documents unchanged behaviour).

- [ ] **Step 3: Implement the change**

In `tui/app.go`, in `copyAddress`, replace the silent no-op branch:

```go
	if addr == "" {
		return nil
	}
	m.flash = "copied " + addr
	return tea.Batch(setClipboard(addr), m.clearFlashCmd())
```

with:

```go
	if addr == "" {
		m.flash = "nothing to copy"
		return m.clearFlashCmd()
	}
	m.flash = "copied " + addr
	return tea.Batch(setClipboard(addr), m.clearFlashCmd())
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestCopyAddressNothingToCopy|TestCopyAddressSuccessSetsCopiedFlash' -count=1 -v`
Expected: PASS (both).

- [ ] **Step 5: Commit**

```bash
gofmt -w tui/app.go tui/app_test.go
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): flash 'nothing to copy' instead of a silent no-op"
```

(Conventional Commits; **no `Co-Authored-By` or other trailers** — this repo forbids them.)

---

### Task 2: Clear stale flash on navigation (and preserve errors on refocus)

**Files:**
- Modify: `tui/app.go` (`back()`, `drill()`)
- Test: `tui/app_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `tui/app_test.go`:

```go
func TestBackClearsFlash(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("Plan: hi\n"),
	}})
	m = step.(appModel) // pos == 0 now, so back() steps back rather than quitting

	m.flash = "copied alice@plan.cat"
	(&m).back()
	if m.flash != "" {
		t.Fatalf("flash = %q after back, want empty", m.flash)
	}
}

func TestDrillClearsFlash(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.state = stateList
	m.list = newList(m.common, hostTarget(t, "@tilde.team"), []User{{Login: "alrs"}})
	m.listReady = true

	m.flash = "copied alrs@tilde.team"
	_, got, _ := m.drill() // value receiver: the clear lands on the returned model
	if got.flash != "" {
		t.Fatalf("flash = %q after drill, want empty", got.flash)
	}
}

func TestFocusInputPreservesErrorFlash(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("Plan: hi\n"),
	}})
	m = step.(appModel)

	m.flash = "error: bad target"
	(&m).focusInput() // on the parse-error recovery path: must NOT clear the flash
	if m.flash != "error: bad target" {
		t.Fatalf("flash = %q after focusInput, want it preserved", m.flash)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tui/ -run 'TestBackClearsFlash|TestDrillClearsFlash|TestFocusInputPreservesErrorFlash' -count=1 -v`
Expected: `TestBackClearsFlash` and `TestDrillClearsFlash` FAIL (flash still set — neither clears it yet). `TestFocusInputPreservesErrorFlash` should already PASS (focusInput doesn't touch flash; this test guards against a regression).

- [ ] **Step 3: Clear the flash in `back()`**

In `tui/app.go`, change `back()` to clear the flash as its first statement:

```go
func (m *appModel) back() tea.Cmd {
	m.flash = ""
	if m.pos < 0 {
		return tea.Quit
	}
	m.stepBack()
	return nil
}
```

- [ ] **Step 4: Clear the flash in `drill()`**

In `tui/app.go`, change `drill()` to clear the flash as its first statement (the value receiver means it lands on the returned model):

```go
func (m appModel) drill() (bool, appModel, tea.Cmd) {
	m.flash = ""
	sel, ok := m.list.selected()
	if !ok {
		return true, m, nil
	}
	// ... rest of drill unchanged ...
```

(Insert only the `m.flash = ""` line; leave every other line of `drill` exactly as-is.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestBackClearsFlash|TestDrillClearsFlash|TestFocusInputPreservesErrorFlash' -count=1 -v`
Expected: PASS (all three).

- [ ] **Step 6: Full gate**

Run: `make check`
Expected: `go vet` clean, `gofmt -l` empty, golangci-lint `0 issues`, all tests pass with `-race` (including the existing copy/flash/navigation tests).

- [ ] **Step 7: Commit**

```bash
gofmt -w tui/app.go tui/app_test.go
git add tui/app.go tui/app_test.go
git commit -m "fix(tui): clear stale flash on back/drill, keep errors on refocus"
```

(Conventional Commits; **no trailers**.)

---

## Self-Review

**1. Spec coverage:**
- "nothing to copy" feedback (auto-clears via `clearFlashCmd`) → Task 1.
- Success path unchanged → Task 1 (`TestCopyAddressSuccessSetsCopiedFlash` documents it; the success lines are untouched).
- Clear stale flash on `back()` and `drill()` only → Task 2 Steps 3-4.
- `focusInput()` excluded; parse error survives it → Task 2 (`TestFocusInputPreservesErrorFlash` regression guard; `focusInput` is not modified).
- Error persistence and copy styling untouched → no edits to `submit` or any style.
- Raw toggle excluded → `enterRaw`/`exitRaw` not modified.

**2. Placeholder scan:** None — every step shows the exact code and command.

**3. Type consistency:** `copyAddress`/`focusInput`/`back` have pointer receivers (`*appModel`), called as `(&m).method()`. `drill` has a value receiver returning `(bool, appModel, tea.Cmd)`, so the test reads `got.flash` from the returned model. `clearFlashCmd` returns a `tea.Cmd` (existing). `fetchResultMsg{reqID, entry}`, `Entry{Target, Body}`, `hostTarget`, `newList(common, host, users)`, `User{Login}`, `stateList` all match existing definitions in `package tui`.

**4. Ambiguity check:** `back()` clears the flash unconditionally before the `pos < 0` quit check — harmless on the quit path and simplest. `drill()` clears the flash before the `selected()` guard so it applies on every drill attempt; this is safe because a drill is only reachable with content/list focus, never on the input-focused parse-error path. `TestCopyAddressSuccessSetsCopiedFlash` and `TestFocusInputPreservesErrorFlash` are characterization tests that pass before their task's code change (they pin behaviour that must not regress); this is intentional, and Step 2 notes it explicitly so the engineer isn't surprised by a "failing test" that already passes.

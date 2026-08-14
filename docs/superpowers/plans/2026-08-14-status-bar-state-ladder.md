# Status Bar State Ladder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The status bar's address renders whole down to 45 columns, because state segments give up their width in value order — `latency`, then `meta`, then `page`/`scroll`, then the esc destination, then esc itself — instead of the address paying for all of them.

**Architecture:** `render` already contains one all-or-nothing fit test that includes `latency` only when the whole line fits (`tui/statusbar.go`). The ladder generalises that single test into a sequence: try the bar at each rung, cheapest concession first, and take the first rung whose full width fits. Each rung is the same `statusBar` with fields blanked, so `rightParts` needs no new parameters and `buildStatusBar` is untouched.

**Tech Stack:** Go, Lip Gloss v2, `github.com/charmbracelet/x/ansi`.

**Spec:** `docs/superpowers/specs/2026-08-14-status-bar-state-ladder-design.md`

## Global Constraints

- Conventional Commits. **No `Co-Authored-By` and no AI-attribution trailers.**
- `make check` must pass. The `tui` race tests take ~75s; budget for it.
- **Do not push or open a PR** without an explicit go-ahead.
- Scope is `tui/statusbar.go` and `tui/statusbar_test.go`. **No change to `tui/app.go`** — the ladder reads fields `buildStatusBar` already sets.
- The bar stays **one line**. Settled: spending height was considered and rejected.
- Honesty flags keep their existing reservation ahead of hints. That invariant is untouched.
- Rules 1 and 2 (hint priority) are already implemented in this branch. The ladder governs state segments only; it must not re-derive or re-order hints.
- Out of scope: review finding 21 (the bar's right-alignment).

## Two traps, from the handoff

Both cost the previous session time; neither is obvious.

1. **Never assert "the address survived" with `strings.Contains(out, crumb)`.** The list state gives a false positive, because `@tilde.team` also appears inside `◂ esc: @tilde.team`. Anchor on the left of the line — `strings.HasPrefix(strings.TrimLeft(out, " "), crumb)`.
2. **`render` budgets the right group first and gives the left the remainder**, so a left-side improvement only becomes visible through a right-side reduction. That asymmetry is the defect being fixed; it is easy to mistake for a broken assertion.

---

### Task 1: Let the esc segment degrade to a bare affordance

**Files:**
- Modify: `tui/statusbar.go` (`statusBar` struct, `rightParts`)
- Test: `tui/statusbar_test.go`

**Interfaces:**
- Consumes: `statusBar.escTarget string`.
- Produces: `statusBar.escShort bool` — when true, `rightParts` emits `"◂ esc"` instead of `"◂ esc: <target>"`. Later tasks set it; nothing outside `statusbar.go` reads it.

- [ ] **Step 1: Write the failing test**

```go
func TestStatusBarEscDegradesToBareAffordance(t *testing.T) {
	full := statusBar{escTarget: "trunc@127.0.0.1:2479", width: 80, styles: newStyles(true)}
	if got := stripANSIForLandingTest(full.render()); !strings.Contains(got, "◂ esc: trunc@127.0.0.1:2479") {
		t.Fatalf("precondition: %q, want the full destination", got)
	}

	short := full
	short.escShort = true
	got := stripANSIForLandingTest(short.render())
	if !strings.Contains(got, "◂ esc") {
		t.Errorf("%q dropped the esc affordance entirely", got)
	}
	if strings.Contains(got, "trunc@127.0.0.1:2479") {
		t.Errorf("%q kept the destination, want it dropped", got)
	}
}
```

- [ ] **Step 2: Run it and verify RED**

Run: `go test ./tui/ -run TestStatusBarEscDegradesToBareAffordance -count=1 -v`

Expected: FAIL to compile — `escShort` is not a field of `statusBar`.

- [ ] **Step 3: Add the field and use it**

In the `statusBar` struct, below `escTarget`:

```go
	escShort  bool     // render "◂ esc" without its destination (state ladder rung 4)
```

In `rightParts`, replace:

```go
	if b.escTarget != "" {
		right = append(right, "◂ esc: "+b.escTarget)
	}
```

with:

```go
	if b.escTarget != "" {
		// The affordance is what the user needs; the destination is a
		// courtesy. Rung 4 of the state ladder keeps the first and drops the
		// second, which buys 12-22 cells on a narrow bar.
		if b.escShort {
			right = append(right, "◂ esc")
		} else {
			right = append(right, "◂ esc: "+b.escTarget)
		}
	}
```

- [ ] **Step 4: Run it and verify GREEN**

Run: `go test ./tui/ -run TestStatusBarEscDegradesToBareAffordance -count=1 -v`

- [ ] **Step 5: Commit**

```bash
git add tui/statusbar.go tui/statusbar_test.go
git commit -m "feat(tui): let the status bar's esc segment drop its destination"
```

---

### Task 2: Replace the single latency fit test with the ladder

**Files:**
- Modify: `tui/statusbar.go` (`render`, and a new `stateLadder` method)
- Test: `tui/statusbar_test.go`

**Interfaces:**
- Consumes: `statusBar.fullWidth([]string, string) int`, `rightParts(bool) []string`, `escShort` from Task 1.
- Produces: `func (b statusBar) stateLadder() []statusBar` — the bar at each rung, richest first. `render` picks the first rung that fits.

- [ ] **Step 1: Write the failing test**

`readerBar` mirrors what `buildStatusBar` produces for a landed reader.

```go
func readerBar(width int) statusBar {
	return statusBar{
		host: "@127.0.0.1", user: "alice", escTarget: "@127.0.0.1",
		latency: "2ms", meta: "1.2 KB", scroll: "42%",
		hints: "↑↓ scroll · r refresh · ? help",
		width: width, styles: newStyles(true),
	}
}

// crumbSurvives anchors on the left of the line. Do not use strings.Contains:
// the address also appears inside "◂ esc: <target>".
func crumbSurvives(out, crumb string) bool {
	return strings.HasPrefix(strings.TrimLeft(out, " "), crumb)
}

func TestStatusBarKeepsTheAddressDownTo45(t *testing.T) {
	for _, width := range []int{45, 60, 80, 100} {
		out := stripANSIForLandingTest(readerBar(width).render())
		if !crumbSurvives(out, "@127.0.0.1 / alice") {
			t.Errorf("width %d: %q clipped the address", width, out)
		}
		if lipgloss.Width(out) > width {
			t.Errorf("width %d: rendered %d cells", width, lipgloss.Width(out))
		}
	}
}

// TestStatusBarShedsStateInLadderOrder: a bar still showing a cheaper segment
// must still show every dearer one.
func TestStatusBarShedsStateInLadderOrder(t *testing.T) {
	for width := 30; width <= 110; width++ {
		out := stripANSIForLandingTest(readerBar(width).render())
		if strings.Contains(out, "2ms") && !strings.Contains(out, "1.2 KB") {
			t.Errorf("width %d: %q kept latency but dropped meta", width, out)
		}
		if strings.Contains(out, "1.2 KB") && !strings.Contains(out, "42%") {
			t.Errorf("width %d: %q kept meta but dropped scroll", width, out)
		}
		if strings.Contains(out, "◂ esc: @127.0.0.1") && !strings.Contains(out, "42%") {
			t.Errorf("width %d: %q kept the esc destination but dropped scroll", width, out)
		}
	}
}
```

- [ ] **Step 2: Run and verify RED**

Run: `go test ./tui/ -run 'TestStatusBarKeepsTheAddressDownTo45|TestStatusBarShedsStateInLadderOrder' -count=1 -v`

Expected: `TestStatusBarKeepsTheAddressDownTo45` FAILS at 45 and probably 60 — the address is clipped to `@127.0.0…`.

- [ ] **Step 3: Add the ladder**

In `tui/statusbar.go`, above `render`:

```go
// stateLadder returns the bar at each rung of the state-shedding order,
// richest first. render takes the first rung whose full width fits, so the
// address stops paying for segments the user needs less than it.
//
// The order is value, cheapest concession first: how long the request took,
// then how big it was, then where in it you are, then where esc goes, then
// that esc goes anywhere at all. render already applied exactly this test to
// latency alone; this generalises that line rather than adding a mechanism
// beside it.
func (b statusBar) stateLadder() []statusBar {
	rungs := []statusBar{b}
	drop := []func(*statusBar){
		func(s *statusBar) { s.latency = "" },
		func(s *statusBar) { s.meta = "" },
		func(s *statusBar) { s.page, s.scroll = "", "" },
		func(s *statusBar) { s.escShort = true },
		func(s *statusBar) { s.escTarget, s.escShort = "", false },
	}
	next := b
	for _, reduce := range drop {
		reduce(&next)
		rungs = append(rungs, next)
	}
	return rungs
}
```

- [ ] **Step 4: Use it in `render`**

Replace:

```go
	right := b.rightParts(false)
	candidate := b.rightParts(true)
	if b.latency != "" && b.fullWidth(candidate, allFlags) <= b.width {
		right = candidate
	}
```

with:

```go
	// Walk the state ladder and keep the richest rung that fits whole,
	// falling through to the leanest when none does.
	rungs := b.stateLadder()
	chosen := rungs[len(rungs)-1]
	for _, rung := range rungs {
		if b.fullWidth(rung.rightParts(true), allFlags) <= b.width {
			chosen = rung
			break
		}
	}
	b.latency, b.meta, b.page = chosen.latency, chosen.meta, chosen.page
	b.scroll, b.escTarget, b.escShort = chosen.scroll, chosen.escTarget, chosen.escShort
	right := b.rightParts(true)
```

`b` is a value receiver, so assigning its fields here affects only this render.

- [ ] **Step 5: Run the status bar tests**

Run: `go test ./tui/ -run TestStatusBar -count=1 -v`

Expected: the two new tests PASS. If an existing test fails, read it before editing: an assertion that `latency` is *dropped* when the line is full is still right; one that a *dearer* segment is dropped while a cheaper one survives is now wrong by design.

- [ ] **Step 6: Run the package suite**

Run: `go test ./tui/ -count=1`

- [ ] **Step 7: Commit**

```bash
git add tui/statusbar.go tui/statusbar_test.go
git commit -m "fix(tui): shed status bar state in value order so the address survives"
```

---

### Task 3: Verify in real terminals and record the outcome

**Files:**
- Modify: `docs/superpowers/specs/2026-08-14-status-bar-state-ladder-design.md` (outcome note only)

- [ ] **Step 1: Run the gate set**

Run: `make check`

- [ ] **Step 2: Record the stills**

Run: `make review-tui`

Expected: twelve tapes, 138 stills, all guards passing. ~7 minutes.

- [ ] **Step 3: Read the bottom line of these frames**

- `out/tui-review/responses-45-dark/error.png` — the address should render whole, and `r retry` must still be there. Both #76 and item 20 in one frame.
- `out/tui-review/responses-45-dark/reader.png` — address whole at the narrow floor.
- `out/tui-review/responses-60-dark/reader.png` — the 60-column case the hint rules already improved; confirm the ladder did not undo it.
- `out/tui-review/responses-100-tall/reader-help.png` — at 100 the full address and every state segment should be present.

- [ ] **Step 4: Append the outcome to the spec**

Record the measured before/after at 45, 60 and 100, in the shape of the "Current behaviour" table the spec already carries.

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/specs/2026-08-14-status-bar-state-ladder-design.md
git commit -m "docs(superpowers): record the status bar state ladder outcome"
```

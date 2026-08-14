# Status Bar Hint Priority Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The status bar shows no hints while the help overlay is open, and when hints do not fit it drops whole hints from the end rather than cutting a word — never dropping `? help`.

**Architecture:** Two independent rules. Rule 1 is a case in `appModel.statusBarModel`, beside the existing flash and `requestFailure` cases. Rule 2 is a loop in `statusBar.render` that sheds trailing hint units before the existing `ansi.Truncate` fallback. `statusBar.hints` stays a ` · `-joined string; the renderer recovers its units by splitting on that separator.

**Tech Stack:** Go, Bubble Tea v2 (`charm.land/bubbletea/v2`), Lip Gloss v2, `github.com/charmbracelet/x/ansi`.

**Spec:** `docs/superpowers/specs/2026-08-14-status-bar-hint-priority-design.md`

## Global Constraints

- Conventional Commits. **No `Co-Authored-By` and no AI-attribution trailers** in commits or PR bodies.
- `make check` must pass: `go vet ./...`, `gofmt -l .` empty, `golangci-lint run ./...`, `go test ./... -race`.
- Do not change the shape of any existing `statusBar{…}` literal — the field stays `hints string`.
- Do not touch `finger/`, `render/`, the help overlay, or the keymap.
- The hint separator is the three-character string `" · "` (space, U+00B7, space). It is the only delimiter; every producer uses it.
- `? help` is matched by exact value, never by position.
- Nothing may render worse than today at any width: the existing `ansi.Truncate` stays as the final fallback.
- Run single tests with `go test ./tui/ -run TestName -count=1 -v`.

---

### Task 1: Drop the bar's hints while help is open

**Files:**
- Modify: `tui/app.go` (`appModel.statusBarModel`, around line 1354)
- Test: `tui/app_test.go`

**Interfaces:**
- Consumes: `appModel.help bool` (set by `openHelp`), `appModel.flash string`, `appModel.requestFailure`, `statusBar` from `tui/statusbar.go`.
- Produces: no new exported names. `statusBarModel` keeps its signature `func (m appModel) statusBarModel() statusBar`.

- [ ] **Step 1: Write the failing test**

Add to `tui/app_test.go`. `helpContextModels` lives in `tui/help_test.go`, same package.

```go
// TestStatusBarDropsHintsWhileHelpIsOpen: the overlay lists the same commands,
// better laid out, two lines above. State stays because the overlay never
// shows it.
func TestStatusBarDropsHintsWhileHelpIsOpen(t *testing.T) {
	for name, base := range helpContextModels(t) {
		if name != "reader no links" && name != "user list" && name != "start content" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			m := base
			m.common.width, m.common.height = 100, 24
			(&m).updateKeymap()
			closed := m.statusBarModel()
			if closed.hints == "" {
				t.Fatalf("precondition: %s has no hints with help closed", name)
			}

			m.openHelp()
			(&m).updateKeymap()
			open := m.statusBarModel()
			if open.hints != "" {
				t.Errorf("hints with help open = %q, want empty", open.hints)
			}
			if open.meta != closed.meta {
				t.Errorf("meta = %q, want it kept at %q", open.meta, closed.meta)
			}
			if open.host != closed.host || open.user != closed.user {
				t.Errorf("breadcrumb = %q/%q, want it kept at %q/%q",
					open.host, open.user, closed.host, closed.user)
			}
		})
	}
}

// TestStatusBarFlashSurvivesHelp: a flash reports something that just
// happened and the overlay never shows it, so it outranks rule 1.
func TestStatusBarFlashSurvivesHelp(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 100, 24
	m.blurInput()
	m.openHelp()
	m.flash = "copied jonathan@tilde.team"
	(&m).updateKeymap()

	if got := m.statusBarModel().hints; got != "copied jonathan@tilde.team" {
		t.Errorf("hints = %q, want the flash to survive with help open", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tui/ -run 'TestStatusBarDropsHintsWhileHelpIsOpen|TestStatusBarFlashSurvivesHelp' -count=1 -v`

Expected: `TestStatusBarDropsHintsWhileHelpIsOpen` FAILS on all three subtests with `hints with help open = "…" , want empty`. `TestStatusBarFlashSurvivesHelp` PASSES already — the flash case returns early today, and this test exists to keep it that way.

- [ ] **Step 3: Add the rule**

In `tui/app.go`, in `statusBarModel`, after the flash block and before the `requestFailure` block:

```go
	if m.help {
		// The overlay is showing these same commands two lines up, laid out in
		// columns that fit. State stays: the address, byte count, page, scroll
		// percentage and flags appear nowhere else.
		bar.hints = ""
	}
```

Order matters. The flash block above returns early, so a flash still wins. The `requestFailure` block below sets its own priority status and clears hints for its own reason; running after this is harmless because both want hints gone.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./tui/ -run 'TestStatusBarDropsHintsWhileHelpIsOpen|TestStatusBarFlashSurvivesHelp' -count=1 -v`

Expected: PASS, both.

- [ ] **Step 5: Run the package suite**

Run: `go test ./tui/ -count=1`

Expected: PASS. If a test asserted bar hints while help happened to be open, fix the expectation — hints are gone by design there.

- [ ] **Step 6: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): drop the status bar's hints while help is open"
```

---

### Task 2: Shed whole hints instead of cutting a word

**Files:**
- Modify: `tui/statusbar.go` (`statusBar.render`, around lines 66-100)
- Test: `tui/statusbar_test.go`

**Interfaces:**
- Consumes: `statusBar` fields `hints string`, `width int`; `b.rightParts(bool) []string`; `b.fullWidth([]string, string) int`; `b.flagsWithin(int) (plain, styled string)`.
- Produces: `func (b statusBar) hintsWithin(budget int) string` — returns the hint string reduced to whole units that fit `budget` cells, or `b.hints` unchanged when it already fits, or `""` when not even one unit fits.

- [ ] **Step 1: Write the failing test**

Add to `tui/statusbar_test.go`:

```go
const startHints = "↵ go · b bookmark · / filter · i target · ? help"

// TestStatusBarShedsWholeHints: the joined right group is truncated
// positionally today, so a narrow bar cuts a hint mid-word and loses "? help"
// first — the one hint that stands in for all the others.
func TestStatusBarShedsWholeHints(t *testing.T) {
	for _, width := range []int{45, 50, 60} {
		b := statusBar{meta: "28 entries", hints: startHints, width: width, styles: newStyles(true)}
		out := stripANSIForLandingTest(b.render())

		if !strings.Contains(out, "? help") {
			t.Errorf("width %d: %q dropped \"? help\"", width, out)
		}
		if strings.Contains(out, "…") {
			t.Errorf("width %d: %q cut a hint mid-word, want whole hints dropped", width, out)
		}
		for _, hint := range strings.Split(startHints, " · ") {
			idx := strings.Index(out, hint)
			if idx < 0 {
				continue // dropped whole, which is the point
			}
			if !strings.Contains(out, hint) {
				t.Errorf("width %d: %q contains a partial %q", width, out, hint)
			}
		}
	}
}

// TestStatusBarKeepsEveryHintWhenItFits guards against over-eager shedding.
func TestStatusBarKeepsEveryHintWhenItFits(t *testing.T) {
	b := statusBar{meta: "28 entries", hints: startHints, width: 100, styles: newStyles(true)}
	out := stripANSIForLandingTest(b.render())
	for _, hint := range strings.Split(startHints, " · ") {
		if !strings.Contains(out, hint) {
			t.Errorf("width 100: %q dropped %q, want the full list", out, hint)
		}
	}
}

// TestStatusBarNarrowerThanHelpStillRenders: below the width of "? help"
// itself there is nothing to keep, and the existing ellipsis path takes over.
// The bar must not exceed its width or panic.
func TestStatusBarNarrowerThanHelpStillRenders(t *testing.T) {
	for _, width := range []int{1, 4, 8} {
		b := statusBar{host: "@tilde.team", hints: startHints, width: width, styles: newStyles(true)}
		out := b.render()
		if got := lipgloss.Width(out); got > width {
			t.Errorf("width %d: rendered %d cells, want at most %d", width, got, width)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tui/ -run 'TestStatusBarShedsWholeHints|TestStatusBarKeepsEveryHintWhenItFits|TestStatusBarNarrowerThanHelpStillRenders' -count=1 -v`

Expected: `TestStatusBarShedsWholeHints` FAILS at widths 45 and 50 — the rendered bar contains `…` and lacks `? help`. The other two PASS already and exist as guards.

- [ ] **Step 3: Add the shedding helper**

In `tui/statusbar.go`, above `render`:

```go
// hintSeparator joins every hint list in the app. It is the only delimiter, so
// splitting on it recovers the individual hints losslessly.
const hintSeparator = " · "

// helpHint is pinned: it is the pointer to the overlay that carries every hint
// this function is about to drop, so it is the last one to go.
const helpHint = "? help"

// hintsWithin reduces the hint list to the whole hints that fit budget cells.
// It drops from the end, because hint lists are built most-useful-first, and it
// never drops helpHint while anything remains.
//
// Returning "" means not even one hint fits; render falls back to its ordinary
// ellipsis truncation, so a very narrow bar is no worse than before.
func (b statusBar) hintsWithin(budget int) string {
	if b.hints == "" || budget <= 0 || lipgloss.Width(b.hints) <= budget {
		return b.hints
	}
	parts := strings.Split(b.hints, hintSeparator)
	for len(parts) > 1 {
		drop := len(parts) - 1
		if parts[drop] == helpHint {
			drop--
		}
		parts = append(parts[:drop], parts[drop+1:]...)
		if joined := strings.Join(parts, hintSeparator); lipgloss.Width(joined) <= budget {
			return joined
		}
	}
	if lipgloss.Width(parts[0]) <= budget {
		return parts[0]
	}
	return ""
}
```

- [ ] **Step 4: Use it in `render`**

In `tui/statusbar.go`, in `render`, replace this:

```go
	rightJoined := strings.Join(right, " · ")
	rightText := ""
	if rightBudget > 0 {
		rightText = ansi.Truncate(rightJoined, rightBudget, "…")
	}
```

with this:

```go
	rightJoined := strings.Join(right, " · ")
	// Shed whole hints before falling back to cutting a word. rightParts
	// appends hints last when they exist, so the state is everything ahead of
	// them and their budget is whatever it leaves.
	if b.hints != "" && lipgloss.Width(rightJoined) > rightBudget {
		state := right[:len(right)-1]
		hintBudget := rightBudget - lipgloss.Width(strings.Join(state, " · "))
		if len(state) > 0 {
			hintBudget -= lipgloss.Width(" · ")
		}
		trimmed := append([]string{}, state...)
		if kept := b.hintsWithin(hintBudget); kept != "" {
			trimmed = append(trimmed, kept)
		}
		rightJoined = strings.Join(trimmed, " · ")
	}
	rightText := ""
	if rightBudget > 0 {
		rightText = ansi.Truncate(rightJoined, rightBudget, "…")
	}
```

`right[:len(right)-1]` is safe: this branch only runs when `b.hints != ""`, and `rightParts` appends the hints as its final element in exactly that case. The copy into `trimmed` avoids aliasing `right`, which is still read below for nothing else but is not worth the risk.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./tui/ -run 'TestStatusBar' -count=1 -v`

Expected: PASS, including the pre-existing `TestStatusBar…` cases. If `TestStatusBarTruncatesBreadcrumbBeforeHints` (or whichever existing case asserts the ellipsis) now fails because the bar no longer truncates mid-hint, read it before changing it: an assertion that a hint is *cut* is now wrong by design, but an assertion that hints *survive* is still right and must keep passing.

- [ ] **Step 6: Run the package suite**

Run: `go test ./tui/ -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add tui/statusbar.go tui/statusbar_test.go
git commit -m "fix(tui): drop whole status bar hints instead of cutting a word"
```

---

### Task 3: Confirm it in real terminals and record the outcome

**Files:**
- Modify: `docs/superpowers/specs/2026-08-14-status-bar-hint-priority-design.md` (outcome note only)

**Interfaces:**
- Consumes: the review kit (`make review-tui`).
- Produces: nothing.

- [ ] **Step 1: Run the full gate set**

Run: `make check`

Expected: all four gates pass.

- [ ] **Step 2: Record the stills**

Run: `make review-tui`

Expected: twelve tapes, 138 stills, every guard passing. About 7 minutes.

- [ ] **Step 3: Check the frames**

Open these and read the bottom line:

- `out/tui-review/chrome-45-dark/help.png` — help open at the narrow floor. The bar should carry state only, no hints, and the overlay is unchanged.
- `out/tui-review/chrome-45-dark/start-list.png` — help closed at 45. The bar should end in `? help`, with whole hints dropped and no `…` inside the hint list.
- `out/tui-review/chrome-60-dark/start-list.png` — the reader breadcrumb should have more room than before.
- `out/tui-review/responses-100-tall/reader-help.png` — the frame that showed the duplication. The bar should no longer repeat the overlay's commands.
- `out/tui-review/responses-45-dark/error.png` — a failed request at 45. `r retry` must still be present; #83 made the state drop out here and this change must not regress it.

- [ ] **Step 4: Append the outcome to the spec**

```markdown
## Outcome

Implemented 2026-08-14. Measured after the change: with help open the bar
carries state and no hints in every context; with help closed at 45 columns the
hint list sheds whole entries and retains `? help`, where it previously cut a
word and dropped `? help` first.
```

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/specs/2026-08-14-status-bar-hint-priority-design.md
git commit -m "docs(superpowers): record the status bar hint priority outcome"
```

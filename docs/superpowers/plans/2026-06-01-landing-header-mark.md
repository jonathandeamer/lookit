# Landing Header Mark Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a reusable `☞ lookit` header row above the normal `target:` input now, while keeping the centered launch splash until a later removal pass.

**Architecture:** Split the existing landing wordmark into a reusable pure renderer in `tui/landing.go`. Add an app-level input chrome helper in `tui/app.go` that renders `☞ lookit` on its own row above the Bubbles `textinput` whenever the normal app layout shows an actively focused target input; the existing centered splash branch remains unchanged except for calling the renamed renderer.

**Tech Stack:** Go, Bubble Tea v2, Lip Gloss v2, Charm `colorprofile`, existing `tui` model and pure render tests.

---

## File Structure

- `tui/landing.go` owns the pure `☞ lookit` mark renderer and the centered launch hero.
- `tui/landing_test.go` covers the pure mark renderer and centered hero layout.
- `tui/app.go` owns app-level composition. Add a small `inputChromeView()` helper so the normal top chrome can show either just `target:` or `☞ lookit` above `target:`.
- `tui/app_test.go` covers when the reusable header row appears and, importantly, that the centered splash is still launch-only for now.
- `docs/superpowers/specs/2026-06-01-landing-hero-design.md` records that the header row is the durable direction and the centered splash is transitional.

---

### Task 1: Extract The Reusable Header Mark Renderer

**Files:**
- Modify: `tui/landing.go`
- Modify: `tui/landing_test.go`

- [ ] **Step 1: Write the failing tests**

In `tui/landing_test.go`, add this helper near the existing test helpers:

```go
func stripANSIForLandingTest(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
```

Then add these tests after `TestGradientWordmarkAnsiFallsBackToSolid`:

```go
func TestHeaderMarkRendersFingerAndWordmark(t *testing.T) {
	st := newStyles(true)
	out := headerMark(st, colorprofile.TrueColor)
	plain := stripANSIForLandingTest(out)
	if plain != heroManicule+" "+heroWordmark {
		t.Fatalf("header mark = %q, want %q", plain, heroManicule+" "+heroWordmark)
	}
	if got := len(foregroundSequences(out)); got < 3 {
		t.Fatalf("expected per-rune colour sweep, got %d distinct colours:\n%q", got, out)
	}
}

func TestHeaderMarkAnsiFallsBackToSolid(t *testing.T) {
	st := newStyles(true)
	out := headerMark(st, colorprofile.ANSI)
	if plain := stripANSIForLandingTest(out); plain != heroManicule+" "+heroWordmark {
		t.Fatalf("header mark = %q, want %q", plain, heroManicule+" "+heroWordmark)
	}
	if got := len(foregroundSequences(out)); got > 2 {
		t.Fatalf("ANSI should not use the truecolor sweep, got %d distinct colours:\n%q", got, out)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:

```bash
go test ./tui/ -run 'TestHeaderMark' -count=1
```

Expected: FAIL to compile with `undefined: headerMark`.

- [ ] **Step 3: Implement the helper by renaming the current renderer**

In `tui/landing.go`, replace the `gradientWordmark` function signature and comment with `headerMark`, keeping the body unchanged:

```go
// headerMark renders "☞ lookit": the manicule in AccentPink, and the
// wordmark with a per-rune pink->violet->mint gradient on truecolor/ANSI256.
// On ANSI (16-colour) and below the gradient muddies, so it falls back to a
// solid AccentViolet wordmark. The gradient is decorative; the wordmark is
// always legible.
func headerMark(st styles, profile colorprofile.Profile) string {
	manicule := lipgloss.NewStyle().Foreground(st.palette.AccentPink).Render(heroManicule)
	if profile < colorprofile.ANSI256 {
		word := lipgloss.NewStyle().Foreground(st.palette.AccentViolet).Bold(true).Render(heroWordmark)
		return manicule + " " + word
	}
	runes := []rune(heroWordmark)
	colors := wordmarkColors(st.palette, len(runes))
	var b strings.Builder
	for i, r := range runes {
		b.WriteString(lipgloss.NewStyle().Foreground(colors[i]).Bold(true).Render(string(r)))
	}
	return manicule + " " + b.String()
}
```

Update `heroView` in `tui/landing.go` from:

```go
parts := []string{gradientWordmark(st, profile)}
```

to:

```go
parts := []string{headerMark(st, profile)}
```

- [ ] **Step 4: Update existing gradient tests to use the new helper**

In `tui/landing_test.go`, replace existing `gradientWordmark(...)` calls with `headerMark(...)` in:

- `TestGradientWordmarkTrueColorVariesPerRune`
- `TestGradientWordmarkAnsiFallsBackToSolid`
- `TestGradientWordmarkANSI256VariesPerRune`

Keep the test names unchanged; they still describe the gradient behavior.

- [ ] **Step 5: Run the focused tests**

Run:

```bash
go test ./tui/ -run 'Test(HeaderMark|GradientWordmark)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add tui/landing.go tui/landing_test.go
git commit -m "refactor(tui): extract reusable header mark"
```

---

### Task 2: Add Header Mark Above Focused Normal Target Input

**Files:**
- Modify: `tui/app.go`
- Modify: `tui/app_test.go`

- [ ] **Step 1: Write failing app composition tests**

In `tui/app_test.go`, add these tests near the existing landing hero tests:

```go
func TestFocusedInputChromeShowsHeaderMarkAboveTargetRow(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)

	m.input.SetValue("alice@plan.cat")
	(&m).submit()
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("Login: alice\n"),
	}})
	m = step.(appModel)

	(&m).focusInput()
	view := stripANSIForLandingTest(m.View().Content)
	lines := strings.Split(view, "\n")
	markLine := -1
	targetLine := -1
	for i, line := range lines {
		if strings.Contains(line, heroManicule+" "+heroWordmark) {
			markLine = i
		}
		if strings.Contains(line, "target:") {
			targetLine = i
		}
	}
	if markLine < 0 {
		t.Fatalf("focused input chrome missing header mark:\n%s", view)
	}
	if targetLine < 0 {
		t.Fatalf("focused input chrome missing target row:\n%s", view)
	}
	if targetLine != markLine+1 {
		t.Fatalf("target row should immediately follow header mark, mark=%d target=%d:\n%s", markLine, targetLine, view)
	}
}

func TestBlurredResultChromeDoesNotSpendHeaderRow(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)

	m.input.SetValue("alice@plan.cat")
	(&m).submit()
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("Login: alice\n"),
	}})
	m = step.(appModel)

	view := stripANSIForLandingTest(m.View().Content)
	if strings.Contains(view, heroManicule+" "+heroWordmark) {
		t.Fatalf("blurred result view should not spend a row on the header mark:\n%s", view)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:

```bash
go test ./tui/ -run 'Test(FocusedInputChromeShowsHeaderMarkAboveTargetRow|BlurredResultChromeDoesNotSpendHeaderRow)' -count=1
```

Expected: first test FAILS because the normal view currently renders only `target:` as the top row; second test should PASS or continue passing.

- [ ] **Step 3: Add the input chrome helper**

In `tui/app.go`, add this helper near `helpView`:

```go
func (m appModel) inputChromeView() string {
	if !m.inputFocused {
		return m.input.View()
	}
	return headerMark(m.common.styles, m.common.profile) + "\n" + m.input.View()
}
```

- [ ] **Step 4: Use the helper in the normal app layout**

In `tui/app.go`, update the non-landing `View` composition from:

```go
full := m.input.View() + "\n" + content + "\n" + bottom
```

to:

```go
full := m.inputChromeView() + "\n" + content + "\n" + bottom
```

Do not change the `if m.landing && m.pos < 0` branch. The centered splash stays in place for now.

- [ ] **Step 5: Re-run the focused app tests**

Run:

```bash
go test ./tui/ -run 'Test(FocusedInputChromeShowsHeaderMarkAboveTargetRow|BlurredResultChromeDoesNotSpendHeaderRow)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): show header mark above focused target input"
```

---

### Task 3: Reserve Height For The Extra Editing Header Row

**Files:**
- Modify: `tui/app.go`
- Modify: `tui/app_test.go`

- [ ] **Step 1: Write a failing height accounting test**

In `tui/app_test.go`, add this test near the focused input chrome tests:

```go
func TestFocusedInputHeaderKeepsTotalViewHeightStable(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m = sized.(appModel)

	m.input.SetValue("alice@plan.cat")
	(&m).submit()
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte(strings.Repeat("line\n", 20)),
	}})
	m = step.(appModel)

	(&m).focusInput()
	(&m).resizeForHelp()
	if got := lipgloss.Height(m.View().Content); got != m.common.height {
		t.Fatalf("view height = %d, want terminal height %d:\n%s", got, m.common.height, m.View().Content)
	}
}
```

If `tui/app_test.go` does not already import `charm.land/lipgloss/v2`, add it to the import block.

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./tui/ -run TestFocusedInputHeaderKeepsTotalViewHeightStable -count=1
```

Expected: FAIL because sub-model height currently reserves one top input row, but focused input chrome now consumes two rows.

- [ ] **Step 3: Add a top chrome height helper**

In `tui/app.go`, add this helper near `bodyHeight` or `inputChromeView`:

```go
func (m appModel) topChromeHeight() int {
	if m.inputFocused && !(m.landing && m.pos < 0) {
		return 2
	}
	return 1
}
```

- [ ] **Step 4: Use the helper when resizing sub-models**

In `tui/app.go`, update `resizeForHelp` from:

```go
h := m.common.bodyHeight() - m.helpHeight()
```

to:

```go
h := m.common.height - m.topChromeHeight() - 1 - m.helpHeight()
```

Keep the existing floor:

```go
if h < 1 {
	h = 1
}
```

This reserves `topChromeHeight()` rows above content and one bottom status-bar row.

- [ ] **Step 5: Update `WindowSizeMsg` handling if needed**

Find the `tea.WindowSizeMsg` case in `tui/app.go`. If it uses `m.common.bodyHeight()` directly to size the reader/list, replace that calculation with:

```go
h := m.common.height - m.topChromeHeight() - 1 - m.helpHeight()
if h < 1 {
	h = 1
}
m.reader.setSize(m.common.width, h)
if m.listReady {
	m.list.setSize(m.common.width, h)
}
```

If the `WindowSizeMsg` case already calls `resizeForHelp()`, leave it alone.

- [ ] **Step 6: Re-run the height test**

Run:

```bash
go test ./tui/ -run TestFocusedInputHeaderKeepsTotalViewHeightStable -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "fix(tui): reserve height for focused input header"
```

---

### Task 4: Document The Transitional Direction

**Files:**
- Modify: `docs/superpowers/specs/2026-06-01-landing-hero-design.md`

- [ ] **Step 1: Update the spec with the transitional decision**

In `docs/superpowers/specs/2026-06-01-landing-hero-design.md`, add this section after the existing “Visual design” section:

````markdown
## Transitional header direction

The `☞ lookit` mark is intended to survive a later splash-removal pass. In that
future layout, the mark should become compact top chrome above the `target:` row
on the empty/editing screen:

```text
☞ lookit
target: @tilde.team
```

The target input stays normal terminal text, and the bottom status bar remains
unchanged. The centered launch splash is therefore temporary presentation
chrome; the durable asset is the finger mark plus per-rune `lookit` gradient.
Until the splash is removed, the reusable header row appears in the normal app
layout when the target input is focused.
````

- [ ] **Step 2: Run a markdown sanity check**

Run:

```bash
rg -n "Transitional header direction|☞ lookit|target: @tilde.team|focused" docs/superpowers/specs/2026-06-01-landing-hero-design.md
```

Expected: shows the new section and both text examples.

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/specs/2026-06-01-landing-hero-design.md
git commit -m "docs(tui): record reusable landing header direction"
```

---

### Task 5: Verification

**Files:**
- Verify only

- [ ] **Step 1: Run focused landing and app tests**

Run:

```bash
go test ./tui/ -run 'Test(HeaderMark|GradientWordmark|Hero|FocusedInput|BlurredResult|Landing)' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run the full TUI package tests**

Run:

```bash
go test ./tui/ -count=1
```

Expected: PASS.

- [ ] **Step 3: Run the full gate**

Run:

```bash
make check
```

Expected: PASS.

- [ ] **Step 4: Manual smoke test in a real terminal**

Run:

```bash
make build
./lookit
```

Expected:

- First launch still shows the centered splash.
- Submitting a target dismisses the splash as before.
- Pressing `i` over a result shows `☞ lookit` on its own row above `target:`.
- The `target:` row remains normal-sized.
- The bottom bar remains one regular terminal row.
- Blurred reader/list views do not spend a row on the header mark.

---

## Self-Review

- **Spec coverage:** The plan implements the reusable header mark now, leaves splash removal for later, keeps the bottom bar unchanged, and avoids making the target input faux-large.
- **Placeholder scan:** No TBD/TODO placeholders are present. Each task includes exact paths, concrete code, commands, and expected results.
- **Type consistency:** `headerMark(st styles, profile colorprofile.Profile) string`, `inputChromeView() string`, and `topChromeHeight() int` are used consistently across tasks.

# Landing Hero Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the empty `No response yet.` first frame with a launch-only, centered "hero" splash — the `☞` manicule + a gradient `lookit` wordmark + tagline + the focused target input.

**Architecture:** A new `tui/landing.go` holds pure render functions (gradient colour math, `gradientWordmark`, `heroView`) with no fetch/network dependency. `appModel` gains a `landing bool` (true at launch, cleared on the first `submit`), and `View()` branches to render the hero — which is the **sole owner of the input** while landing — instead of the normal `input \n content \n bottom` stack. After the first lookup the hero never returns. Colour comes entirely from the already-shipped Functional Bright palette in `tui/styles.go`.

**Tech Stack:** Go, Bubble Tea v2 (`charm.land/bubbletea/v2`), Lip Gloss v2 (`charm.land/lipgloss/v2`), `github.com/charmbracelet/colorprofile`, `image/color`.

**Spec:** `docs/superpowers/specs/2026-06-01-landing-hero-design.md`

---

## File Structure

- **Create `tui/landing.go`** — pure landing render helpers: `lerpColor`, `wordmarkColors`, `gradientWordmark`, `heroInputWidth`, `heroView`, and the `heroManicule`/`heroWordmark`/`heroTagline` constants. One responsibility: turning palette + dimensions + a rendered input string into the centered hero block.
- **Create `tui/landing_test.go`** — unit tests for the helpers above (same `tui` package; reuses existing `sameColor`/`assertSameColor` helpers from `styles_test.go`).
- **Modify `tui/app.go`** — add `landing bool` to `appModel`; initialise it in `newApp`; clear it in `submit`; branch in `View()`.
- **Modify `tui/app_test.go`** — landing lifecycle tests (true at launch + hero rendered; cleared on submit; never returns on back-navigation).

No changes to `render/`, `finger/`, `routeFetch`, `ParseUsers`, the status bar, or the data model.

---

### Task 1: Gradient colour helpers

**Files:**
- Create: `tui/landing.go`
- Test: `tui/landing_test.go`

- [ ] **Step 1: Write the failing test**

Create `tui/landing_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

func TestWordmarkColorsSweepsPalette(t *testing.T) {
	p := paletteFor(true)
	colors := wordmarkColors(p, 6)
	if len(colors) != 6 {
		t.Fatalf("len = %d, want 6", len(colors))
	}
	assertSameColor(t, "first stop", colors[0], p.AccentPink)
	assertSameColor(t, "last stop", colors[5], p.AccentMint)
	if sameColor(colors[0], colors[5]) {
		t.Fatal("gradient endpoints should differ")
	}
	if sameColor(colors[1], colors[4]) {
		t.Fatal("interior gradient colours should differ")
	}
}

func TestWordmarkColorsSingleRune(t *testing.T) {
	p := paletteFor(true)
	colors := wordmarkColors(p, 1)
	if len(colors) != 1 {
		t.Fatalf("len = %d, want 1", len(colors))
	}
	assertSameColor(t, "single", colors[0], p.AccentPink)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run 'TestWordmarkColors' -count=1 -v`
Expected: FAIL to compile — `undefined: wordmarkColors`.

- [ ] **Step 3: Write minimal implementation**

Create `tui/landing.go`:

```go
package tui

import (
	"image/color"
)

// lerpColor linearly interpolates between two colours in 8-bit RGB. t is
// clamped to [0,1].
func lerpColor(a, b color.Color, t float64) color.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	return color.RGBA{
		R: uint8(float64(ar>>8)*(1-t) + float64(br>>8)*t),
		G: uint8(float64(ag>>8)*(1-t) + float64(bg>>8)*t),
		B: uint8(float64(ab>>8)*(1-t) + float64(bb>>8)*t),
		A: 0xff,
	}
}

// wordmarkColors returns n colours sweeping AccentPink -> AccentViolet ->
// AccentMint across the palette. The endpoints are the exact palette stops;
// interior positions are interpolated. n <= 1 returns the first stop only.
func wordmarkColors(p palette, n int) []color.Color {
	stops := []color.Color{p.AccentPink, p.AccentViolet, p.AccentMint}
	if n <= 1 {
		return []color.Color{stops[0]}
	}
	out := make([]color.Color, n)
	for i := 0; i < n; i++ {
		switch i {
		case 0:
			out[i] = stops[0]
		case n - 1:
			out[i] = stops[len(stops)-1]
		default:
			seg := float64(i) / float64(n-1) * float64(len(stops)-1)
			lo := int(seg)
			out[i] = lerpColor(stops[lo], stops[lo+1], seg-float64(lo))
		}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tui/ -run 'TestWordmarkColors' -count=1 -v`
Expected: PASS (both subtests).

- [ ] **Step 5: Commit**

```bash
git add tui/landing.go tui/landing_test.go
git commit -m "feat(tui): add gradient colour helpers for landing wordmark"
```

---

### Task 2: Gradient wordmark

**Files:**
- Modify: `tui/landing.go`
- Test: `tui/landing_test.go`

- [ ] **Step 1: Write the failing test**

Append to `tui/landing_test.go`:

```go
// foregroundSequences returns the set of distinct truecolor foreground SGR
// payloads (e.g. "38;2;255;95;162") present in s.
func foregroundSequences(s string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(s, "\x1b[") {
		if strings.HasPrefix(part, "38;2;") {
			if i := strings.IndexByte(part, 'm'); i >= 0 {
				out[part[:i]] = true
			}
		}
	}
	return out
}

func TestGradientWordmarkTrueColorVariesPerRune(t *testing.T) {
	st := newStyles(true)
	out := gradientWordmark(st, colorprofile.TrueColor)
	if !strings.Contains(out, heroManicule) {
		t.Fatalf("missing manicule:\n%q", out)
	}
	// Manicule + 6 gradient runes => several distinct foreground colours.
	if got := len(foregroundSequences(out)); got < 3 {
		t.Fatalf("expected a per-rune sweep, got %d distinct colours:\n%q", got, out)
	}
}

func TestGradientWordmarkAnsiFallsBackToSolid(t *testing.T) {
	st := newStyles(true)
	out := gradientWordmark(st, colorprofile.ANSI)
	if !strings.Contains(out, heroManicule) {
		t.Fatalf("missing manicule:\n%q", out)
	}
	// 16-colour: manicule colour + one solid word colour => at most 2 distinct.
	if got := len(foregroundSequences(out)); got > 2 {
		t.Fatalf("ANSI should be solid, got %d distinct colours:\n%q", got, out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run 'TestGradientWordmark' -count=1 -v`
Expected: FAIL to compile — `undefined: gradientWordmark`, `undefined: heroManicule`.

- [ ] **Step 3: Write minimal implementation**

In `tui/landing.go`, add the imports and the constants + function. The import block becomes:

```go
import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)
```

Add near the top of the file (after the imports):

```go
const (
	heroManicule = "☞"
	heroWordmark = "lookit"
	heroTagline  = "a finger client for the modern terminal"
)
```

Add the function:

```go
// gradientWordmark renders "☞ lookit": the manicule in AccentPink, and the
// wordmark with a per-rune pink->violet->mint gradient on truecolor/ANSI256.
// On ANSI (16-colour) and below the gradient muddies, so it falls back to a
// solid AccentViolet wordmark. The gradient is decorative; the wordmark is
// always legible.
func gradientWordmark(st styles, profile colorprofile.Profile) string {
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

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tui/ -run 'TestGradientWordmark' -count=1 -v`
Expected: PASS (both subtests).

- [ ] **Step 5: Commit**

```bash
git add tui/landing.go tui/landing_test.go
git commit -m "feat(tui): render gradient lookit wordmark"
```

---

### Task 3: Hero composition

**Files:**
- Modify: `tui/landing.go`
- Test: `tui/landing_test.go`

- [ ] **Step 1: Write the failing test**

Append to `tui/landing_test.go`:

```go
func TestHeroViewCentersWordmarkTaglineAndInput(t *testing.T) {
	st := newStyles(true)
	out := heroView(st, colorprofile.TrueColor, 60, 12, "target: ")
	if h := lipgloss.Height(out); h != 12 {
		t.Fatalf("hero height = %d, want 12", h)
	}
	if w := lipgloss.Width(out); w != 60 {
		t.Fatalf("hero width = %d, want 60", w)
	}
	for _, want := range []string{heroManicule, heroTagline, "target:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("hero missing %q:\n%s", want, out)
		}
	}
}

func TestHeroViewHidesTaglineWhenNarrow(t *testing.T) {
	st := newStyles(true)
	out := heroView(st, colorprofile.TrueColor, 30, 12, "target: ")
	if strings.Contains(out, heroTagline) {
		t.Fatalf("tagline should be hidden under 40 cols:\n%s", out)
	}
	if !strings.Contains(out, heroManicule) {
		t.Fatalf("wordmark should still render when narrow:\n%s", out)
	}
}

func TestHeroViewEmptyOnZeroDimensions(t *testing.T) {
	st := newStyles(true)
	if out := heroView(st, colorprofile.TrueColor, 0, 12, "target: "); out != "" {
		t.Fatalf("zero width should render empty, got %q", out)
	}
}

func TestHeroInputWidthBounds(t *testing.T) {
	if got := heroInputWidth(200); got != 40 {
		t.Fatalf("wide terminal width = %d, want 40", got)
	}
	if got := heroInputWidth(20); got != 16 {
		t.Fatalf("narrow terminal width = %d, want 16", got)
	}
	if got := heroInputWidth(4); got != 12 {
		t.Fatalf("tiny terminal width = %d, want floor 12", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run 'TestHero' -count=1 -v`
Expected: FAIL to compile — `undefined: heroView`, `undefined: heroInputWidth`.

- [ ] **Step 3: Write minimal implementation**

In `tui/landing.go`, add both functions:

```go
// heroInputWidth bounds the centred landing input so it reads as a tidy box
// rather than a full-width bar: ~40 columns, clamped to the terminal width
// (leaving a small margin) and never below 12.
func heroInputWidth(totalWidth int) int {
	w := 40
	if max := totalWidth - 4; w > max {
		w = max
	}
	if w < 12 {
		w = 12
	}
	return w
}

// heroView composes the centred landing hero — wordmark, tagline (hidden under
// 40 columns), a spacer, and the already-rendered input — and places it in the
// centre of a width x height box. It is the sole renderer of the input on the
// landing screen. Pure: string in, string out.
func heroView(st styles, profile colorprofile.Profile, width, height int, input string) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	parts := []string{gradientWordmark(st, profile)}
	if width >= 40 {
		parts = append(parts, lipgloss.NewStyle().Foreground(st.palette.Dim).Render(heroTagline))
	}
	parts = append(parts, "", input) // blank spacer line before the input
	block := lipgloss.JoinVertical(lipgloss.Center, parts...)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, block)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tui/ -run 'TestHero' -count=1 -v`
Expected: PASS (all four subtests).

- [ ] **Step 5: Commit**

```bash
git add tui/landing.go tui/landing_test.go
git commit -m "feat(tui): compose centered landing hero"
```

---

### Task 4: Wire the launch-only hero into appModel

**Files:**
- Modify: `tui/app.go` (struct `appModel` ~line 91; `newApp` ~line 134; `submit` ~line 310; `View` ~line 837)
- Test: `tui/app_test.go`

- [ ] **Step 1: Write the failing test**

Append to `tui/app_test.go` (it already imports `context`, `strings`, `tea`, `colorprofile`, `finger`, and defines `stubFetch`/`hostTarget`):

```go
func TestLandingTrueAtLaunchAndHeroRendered(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	if !m.landing {
		t.Fatal("landing should be true at launch")
	}
	content := m.View().Content
	if !strings.Contains(content, heroManicule) {
		t.Fatalf("launch view missing hero manicule:\n%s", content)
	}
	if !strings.Contains(content, heroTagline) {
		t.Fatalf("launch view missing tagline:\n%s", content)
	}
}

func TestSubmitDismissesLandingHero(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)

	m.input.SetValue("@tilde.team")
	(&m).submit() // returns a fetch cmd we deliberately do not run, so stubFetch is never called
	if m.landing {
		t.Fatal("submit should dismiss the landing hero")
	}
	if strings.Contains(m.View().Content, heroManicule) {
		t.Fatalf("hero should be gone after submit:\n%s", m.View().Content)
	}
}

func TestHeroDoesNotReturnOnBackToLanding(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)

	m.input.SetValue("alice@plan.cat")
	(&m).submit() // landing=false, reqSeq=1, loading
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("Login: alice\n"),
	}})
	m = step.(appModel)
	if m.landing {
		t.Fatal("landing should be false after a fetch")
	}

	(&m).back() // pos 0 -> -1, returns to the empty landing
	if m.pos != -1 {
		t.Fatalf("want pos -1 after back-to-landing, got %d", m.pos)
	}
	if m.landing {
		t.Fatal("hero must not return on back-navigation")
	}
	if strings.Contains(m.View().Content, heroManicule) {
		t.Fatalf("hero reappeared on back-to-landing:\n%s", m.View().Content)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run 'TestLandingTrueAtLaunch|TestSubmitDismissesLandingHero|TestHeroDoesNotReturnOnBackToLanding' -count=1 -v`
Expected: FAIL to compile — `m.landing undefined (type appModel has no field or method landing)`.

- [ ] **Step 3: Add the `landing` field to `appModel`**

In `tui/app.go`, in the `appModel` struct (the block ending around line 113 with `listReady  bool`), add the field next to the other top-level flags:

```go
	help       bool // help panel open
	helpModel  help.Model
	listReady  bool
	landing    bool // true until the first fetch is dispatched; gates the hero
```

- [ ] **Step 4: Initialise `landing` in `newApp`**

In `tui/app.go`, in the `appModel{...}` literal inside `newApp` (around line 134-144), add `landing: true,`:

```go
	app := appModel{
		common:       common,
		state:        stateReader,
		reader:       newReader(profile),
		input:        in,
		inputFocused: true,
		keys:         newKeyMap(),
		helpModel:    help.New(),
		spin:         spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(st.spinner)),
		pos:          -1,
		landing:      true,
	}
```

- [ ] **Step 5: Clear `landing` in `submit`**

In `tui/app.go`, in `submit` (around line 310), set `m.landing = false` on the success path, right after the flash is cleared:

```go
func (m *appModel) submit() tea.Cmd {
	target, err := finger.ParseTarget(strings.TrimSpace(m.input.Value()))
	if err != nil {
		m.flash = "error: " + err.Error()
		return nil
	}
	m.flash = "" // clear any stale parse-error flash from a prior failed submit
	m.landing = false
	m.blurInput()
	return m.startFetch(target)
}
```

- [ ] **Step 6: Branch in `View` to render the hero**

In `tui/app.go`, replace the body of `View` (around line 837-855) with a landing branch followed by the existing stack:

```go
func (m appModel) View() tea.View {
	(&m).updateKeymap() // sync the help panel's enabled set to current state

	if m.landing && m.pos < 0 { // pos<0 keeps the hero gone once anything lands, even for results delivered without a prior submit
		bottom := m.statusBarModel().render()
		if m.help {
			bottom = m.helpView() + "\n" + bottom
		}
		heroH := m.common.height - lipgloss.Height(bottom)
		if heroH < 1 {
			heroH = 1
		}
		in := m.input // value copy: bounding its width must not mutate the live input
		in.SetWidth(heroInputWidth(m.common.width))
		hero := heroView(m.common.styles, m.common.profile, m.common.width, heroH, in.View())
		v := tea.NewView(hero + "\n" + bottom)
		v.AltScreen = true
		return v
	}

	var content string
	switch m.state {
	case stateList:
		content = m.list.View()
	default:
		content = m.reader.View()
	}
	bottom := m.statusBarModel().render()
	if m.help {
		bottom = m.helpView() + "\n" + bottom
	}
	full := m.input.View() + "\n" + content + "\n" + bottom

	v := tea.NewView(full)
	v.AltScreen = true
	return v
}
```

- [ ] **Step 7: Run the new tests to verify they pass**

Run: `go test ./tui/ -run 'TestLandingTrueAtLaunch|TestSubmitDismissesLandingHero|TestHeroDoesNotReturnOnBackToLanding' -count=1 -v`
Expected: PASS (all three).

- [ ] **Step 8: Run the full gate**

Run: `make check`
Expected: `go vet` clean, `gofmt -l` empty, golangci-lint `0 issues`, all tests pass with `-race`.

- [ ] **Step 9: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): show launch-only landing hero"
```

---

## Self-Review

**1. Spec coverage:**

- Centered hero composition (manicule + gradient wordmark + tagline + input) → Tasks 2, 3.
- Launch-only behaviour; cleared on first fetch; never returns on back-nav → Task 4 (`landing` field, `submit`, `TestHeroDoesNotReturnOnBackToLanding`).
- Pure, golden-testable render functions; no fetch/network dependency → Task 1-3 (`tui/landing.go`).
- Hero is sole owner of the input; `View()` does not also prepend the top input row → Task 4 Step 6 (landing branch builds `hero + "\n" + bottom`, no `m.input.View()` prefix).
- Status bar reuses existing `landingBar` (`pos < 0`); honest hint unchanged; no `q quit` → no status-bar change made (Task 4 leaves `statusBarModel` untouched; landing keeps `pos == -1`).
- Gradient degradation by profile (truecolor/ANSI256 sweep; ANSI+ solid) → Task 2 + `TestGradientWordmarkAnsiFallsBackToSolid`.
- Background-aware (light/dark) → `newStyles(dark)` palette flows through `m.common.styles`; `heroView`/`gradientWordmark` read `st.palette`.
- Narrow-terminal tagline hidden; bounded centered input → Task 3 (`heroView` < 40 guard, `heroInputWidth`) + tests.
- Help panel open on landing accounted for → Task 4 Step 6 includes `if m.help { bottom = helpView + ... }` so `heroH` reserves its rows.

**2. Placeholder scan:** No TBD/TODO; every code step shows complete code; every test step shows full test bodies.

**3. Type consistency:** `palette` fields are `color.Color` (styles.go); `lerpColor`/`wordmarkColors` consume/produce `color.Color`. `gradientWordmark(st styles, profile colorprofile.Profile)` and `heroView(st styles, profile colorprofile.Profile, width, height int, input string)` match the spec's revised signatures and their call site in `View`. `heroManicule`/`heroWordmark`/`heroTagline` declared in Task 2, used in Tasks 2-4 and tests. `m.View().Content`, `stubFetch(t)`, `hostTarget(t, …)`, `sameColor`/`assertSameColor` are existing seams confirmed in `app_test.go`/`styles_test.go`.

**4. Ambiguity check:** `heroInputWidth(20)` → `40` capped to `20-4=16` → `16` (≥12), matching the test. `heroInputWidth(4)` → `4-4=0`, `w=0<12` → floor `12`. The `View` value-receiver copy (`in := m.input`) bounds width without mutating the live model — relied on by `TestSubmitDismissesLandingHero` which re-renders after submit.

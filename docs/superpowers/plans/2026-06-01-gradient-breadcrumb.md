# Gradient Breadcrumb Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Accent the status-bar breadcrumb's leaf (the user) with the palette's pink→violet→mint gradient, with the gradient maths relocated to a neutral file so the breadcrumb doesn't depend on the splash-bound `landing.go`.

**Architecture:** Move the generic gradient helpers out of `tui/landing.go` into a new `tui/gradient.go` (renaming `wordmarkColors`→`gradientColors`) and add a reusable `gradientString`. Then change one line of `statusBar.styleCrumb` to render the leaf via `gradientString`, and extend the palette contrast test to gate pink/mint on the status-bar background at 3:1.

**Tech Stack:** Go, `charm.land/lipgloss/v2`, `github.com/charmbracelet/colorprofile`, `image/color`, `github.com/charmbracelet/x/ansi` (tests).

**Spec:** `docs/superpowers/specs/2026-06-01-gradient-breadcrumb-design.md`

---

## File Structure

- **Create `tui/gradient.go`** — generic gradient maths: `lerpColor`, `gradientColors` (renamed from `wordmarkColors`), and `gradientString`. Independent of the hero, so it survives the later splash removal.
- **Create `tui/gradient_test.go`** — the relocated `lerpColor`/`gradientColors` tests plus a `gradientString` test.
- **Modify `tui/landing.go`** — delete the two moved helpers; point `gradientWordmark` at `gradientColors`; drop now-unused imports.
- **Modify `tui/landing_test.go`** — delete the three helper tests that moved (the `gradientWordmark` tests stay).
- **Modify `tui/statusbar.go`** — one line in `styleCrumb` (leaf → `gradientString`).
- **Modify `tui/statusbar_test.go`** — breadcrumb leaf-gradient tests.
- **Modify `tui/styles_test.go`** — extend `TestTUIPaletteContrast` (pink/mint on `SubtleBg` ≥3.0).

---

### Task 1: Relocate gradient helpers and add `gradientString`

**Files:**
- Create: `tui/gradient.go`, `tui/gradient_test.go`
- Modify: `tui/landing.go`, `tui/landing_test.go`

- [ ] **Step 1: Create `tui/gradient.go`**

```go
package tui

import (
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
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
		R: uint8(math.Round(float64(ar>>8)*(1-t) + float64(br>>8)*t)),
		G: uint8(math.Round(float64(ag>>8)*(1-t) + float64(bg>>8)*t)),
		B: uint8(math.Round(float64(ab>>8)*(1-t) + float64(bb>>8)*t)),
		A: 0xff,
	}
}

// gradientColors returns n colours sweeping AccentPink -> AccentViolet ->
// AccentMint across the palette. The endpoints are the exact palette stops;
// interior positions are interpolated. n <= 1 returns the first stop only.
func gradientColors(p palette, n int) []color.Color {
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

// gradientString renders s rune-by-rune in base, replacing only base's
// foreground with the gradient colour for each rune's position (so base's
// background and bold are preserved). Empty s renders empty.
func gradientString(base lipgloss.Style, p palette, s string) string {
	runes := []rune(s)
	colors := gradientColors(p, len(runes))
	var b strings.Builder
	for i, r := range runes {
		b.WriteString(base.Foreground(colors[i]).Render(string(r)))
	}
	return b.String()
}
```

- [ ] **Step 2: Remove the moved helpers from `tui/landing.go` and repoint `gradientWordmark`**

In `tui/landing.go`: delete the `lerpColor` function and the `wordmarkColors` function entirely. In `gradientWordmark`, change the one call `wordmarkColors(st.palette, len(runes))` to `gradientColors(st.palette, len(runes))`. Then fix the import block — `image/color` and `math` are no longer used in `landing.go`; it should read:

```go
import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)
```

(`gradientWordmark` still uses `strings`, `lipgloss`, `colorprofile`; `heroView`/`heroInputWidth` need no new imports.)

- [ ] **Step 3: Move the helper tests**

In `tui/landing_test.go`, delete these four test functions (they move to gradient_test.go): `TestLerpColor`, `TestWordmarkColorsSweepsPalette`, `TestWordmarkColorsSingleRune`, and `TestWordmarkColorsLightPaletteSweeps`. **Leave** the `gradientWordmark` and `heroView`/`heroInputWidth` tests and the `foregroundSequences` helper in place (the latter stays in `landing_test.go` and is shared package-wide). If, after deleting those tests, any import in `landing_test.go` becomes unused, remove it (likely `image/color`).

Then create `tui/gradient_test.go`:

```go
package tui

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestLerpColor(t *testing.T) {
	black := color.RGBA{R: 0, G: 0, B: 0, A: 0xff}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 0xff}
	cases := []struct {
		name  string
		t     float64
		wantR uint8
	}{
		{"t=0 is a", 0, 0},
		{"t=1 is b", 1, 255},
		{"midpoint rounds up", 0.5, 128},
		{"clamp below 0", -1, 0},
		{"clamp above 1", 2, 255},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lerpColor(black, white, tc.t)
			r, _, _, _ := got.RGBA()
			if uint8(r>>8) != tc.wantR {
				t.Fatalf("R = %d, want %d", uint8(r>>8), tc.wantR)
			}
		})
	}
}

func TestGradientColorsSweepsPalette(t *testing.T) {
	p := paletteFor(true)
	colors := gradientColors(p, 6)
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

func TestGradientColorsSingleRune(t *testing.T) {
	p := paletteFor(true)
	colors := gradientColors(p, 1)
	if len(colors) != 1 {
		t.Fatalf("len = %d, want 1", len(colors))
	}
	assertSameColor(t, "single", colors[0], p.AccentPink)
}

func TestGradientColorsLightPaletteSweeps(t *testing.T) {
	p := paletteFor(false)
	colors := gradientColors(p, 6)
	if len(colors) != 6 {
		t.Fatalf("len = %d, want 6", len(colors))
	}
	assertSameColor(t, "light first stop", colors[0], p.AccentPink)
	assertSameColor(t, "light last stop", colors[5], p.AccentMint)
	if sameColor(colors[0], colors[5]) {
		t.Fatal("light gradient endpoints should differ")
	}
}

func TestGradientStringVariesPerRune(t *testing.T) {
	p := paletteFor(true)
	out := gradientString(lipgloss.NewStyle(), p, "lookit")
	if got := len(foregroundSequences(out)); got < 3 {
		t.Fatalf("expected a per-rune gradient, got %d distinct colours: %q", got, out)
	}
	if got := ansi.Strip(out); got != "lookit" {
		t.Fatalf("stripped gradientString = %q, want %q", got, "lookit")
	}
}

func TestGradientStringEmpty(t *testing.T) {
	if out := gradientString(lipgloss.NewStyle(), paletteFor(true), ""); out != "" {
		t.Fatalf("gradientString(\"\") = %q, want empty", out)
	}
}
```

- [ ] **Step 4: Build and run the package**

Run: `go build ./tui/ && go test ./tui/ -count=1`
Expected: PASS. The move is behaviour-preserving (`gradientWordmark` and its tests are unchanged apart from the renamed call), and the new `gradientString` tests pass. If the build complains about an unused import in `landing.go` or `landing_test.go`, remove it and re-run.

- [ ] **Step 5: Commit**

```bash
gofmt -w tui/gradient.go tui/gradient_test.go tui/landing.go tui/landing_test.go
git add tui/gradient.go tui/gradient_test.go tui/landing.go tui/landing_test.go
git commit -m "refactor(tui): extract gradient helpers into gradient.go"
```

(Conventional Commits; **no `Co-Authored-By` or other trailers** — this repo forbids them.)

---

### Task 2: Gradient leaf in the breadcrumb + contrast gate

**Files:**
- Modify: `tui/statusbar.go` (`styleCrumb`, the final return)
- Modify: `tui/statusbar_test.go`, `tui/styles_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `tui/statusbar_test.go`. It currently imports `strings`, `testing`, and `charm.land/lipgloss/v2`; add `"github.com/charmbracelet/x/ansi"`. (`foregroundSequences` is defined in `landing_test.go`, same package.)

```go
func TestStatusBarLeafIsGradient(t *testing.T) {
	b := statusBar{host: "@tilde.team", user: "jonathan", width: 80, styles: newStyles(true)}
	crumb := b.styleCrumb(60) // 60 > width of "@tilde.team / jonathan", so it fits
	if got := ansi.Strip(crumb); got != "@tilde.team / jonathan" {
		t.Fatalf("stripped crumb = %q, want %q", got, "@tilde.team / jonathan")
	}
	// host + " / " share the one dim BarText colour; a gradient leaf adds several.
	if got := len(foregroundSequences(crumb)); got < 3 {
		t.Fatalf("expected a gradient leaf (>=3 distinct fg colours), got %d: %q", got, crumb)
	}
}

func TestStatusBarLeafCollapsesToDimWhenOverBudget(t *testing.T) {
	b := statusBar{host: "@a-very-long-hostname.example.org", user: "verylonguser", width: 80, styles: newStyles(true)}
	crumb := b.styleCrumb(10) // narrow budget forces the dim collapse
	if !strings.Contains(crumb, "…") {
		t.Fatalf("expected ellipsis collapse: %q", crumb)
	}
	if got := len(foregroundSequences(crumb)); got != 1 {
		t.Fatalf("collapsed crumb should be a single dim colour, got %d: %q", got, crumb)
	}
}

func TestStatusBarDirectoryLeafHasNoGradient(t *testing.T) {
	b := statusBar{host: "@tilde.team", width: 80, styles: newStyles(true)}
	crumb := b.styleCrumb(60)
	if got := ansi.Strip(crumb); got != "@tilde.team" {
		t.Fatalf("stripped crumb = %q, want %q", got, "@tilde.team")
	}
	if got := len(foregroundSequences(crumb)); got != 1 {
		t.Fatalf("directory crumb should be a single dim colour, got %d: %q", got, crumb)
	}
}
```

Also extend `TestTUIPaletteContrast` in `tui/styles_test.go`: add these two lines after the existing `"help key on subtle bg"` assertion (violet keeps its ≥4.5 gate; pink/mint use the 3:1 decorative-leaf tier):

```go
			assertContrast(t, "breadcrumb leaf pink on subtle bg", tt.p.AccentPink, tt.p.SubtleBg, 3.0)
			assertContrast(t, "breadcrumb leaf mint on subtle bg", tt.p.AccentMint, tt.p.SubtleBg, 3.0)
```

- [ ] **Step 2: Run tests to verify the leaf test fails**

Run: `go test ./tui/ -run 'TestStatusBarLeafIsGradient|TestStatusBarLeafCollapsesToDimWhenOverBudget|TestStatusBarDirectoryLeafHasNoGradient|TestTUIPaletteContrast' -count=1 -v`
Expected: `TestStatusBarLeafIsGradient` FAILS (leaf is currently the flat bold `barUser` → only 2 distinct fg colours). The collapse, directory, and contrast tests PASS already (collapse/directory behaviour is unchanged; pink/mint clear 3.0 — see the spec's measured table).

- [ ] **Step 3: Render the leaf with the gradient**

In `tui/statusbar.go`, `styleCrumb`, change only the final return line:

```go
	return st.barHost.Render(b.host) + st.barSep.Render(" / ") + st.barUser.Render(b.user)
```

to:

```go
	return st.barHost.Render(b.host) + st.barSep.Render(" / ") + gradientString(st.barUser, st.palette, b.user)
```

(`st.barUser` carries the status-bar background + bold; `gradientString` keeps those and varies only the per-rune foreground. The other branches — over-budget collapse and the `b.user == ""` directory case — are unchanged.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestStatusBarLeafIsGradient|TestStatusBarLeafCollapsesToDimWhenOverBudget|TestStatusBarDirectoryLeafHasNoGradient|TestTUIPaletteContrast' -count=1 -v`
Expected: PASS (all).

- [ ] **Step 5: Full gate**

Run: `make check`
Expected: `go vet` clean, `gofmt -l` empty, golangci-lint `0 issues`, all tests pass with `-race` (including the existing breadcrumb tests, which assert on plain/`ansi.Strip` text and so are unaffected by the leaf colouring).

- [ ] **Step 6: Commit**

```bash
gofmt -w tui/statusbar.go tui/statusbar_test.go tui/styles_test.go
git add tui/statusbar.go tui/statusbar_test.go tui/styles_test.go
git commit -m "feat(tui): accent the breadcrumb leaf with the palette gradient"
```

(Conventional Commits; **no trailers**.)

---

## Self-Review

**1. Spec coverage:**
- Leaf rendered with gradient when profile fits → Task 2 Step 3 + `TestStatusBarLeafIsGradient`.
- Host/separator unchanged (dim); directory stays fully dim → Task 2 (`styleCrumb` other branches untouched) + `TestStatusBarDirectoryLeafHasNoGradient`.
- Over-budget collapses to dim truncated string → unchanged branch + `TestStatusBarLeafCollapsesToDimWhenOverBudget`.
- Helpers relocated to `tui/gradient.go`, `wordmarkColors`→`gradientColors`, `gradientString` added, `gradientWordmark` repointed and left in `landing.go` → Task 1.
- Contrast gate: pink/mint on `SubtleBg` ≥3.0 (violet keeps ≥4.5) → Task 2 Step 1.
- No new deps; hero untouched → only the listed files change; `heroView`/`heroInputWidth`/`landing` wiring not modified.

**2. Placeholder scan:** None — full code and exact commands in every step.

**3. Type consistency:** `gradientColors(p palette, n int) []color.Color` and `gradientString(base lipgloss.Style, p palette, s string) string` are defined in Task 1 and called identically in `gradientWordmark` (Task 1 Step 2) and `styleCrumb` (Task 2 Step 3). `st.barUser`, `st.barHost`, `st.barSep`, `st.palette` are existing `styles` fields. `foregroundSequences`, `assertSameColor`, `sameColor`, `assertContrast`, `paletteFor`, `newStyles` are existing test helpers in `package tui`.

**4. Ambiguity check:** `styleCrumb(60)` fits `@tilde.team / jonathan` (width 22) so it takes the gradient branch; `styleCrumb(10)` forces collapse. The leaf-gradient assertion uses `>=3` distinct foreground colours (one dim BarText for host+separator, plus ≥2 gradient colours for an 8-rune leaf) to distinguish gradient from the flat 2-colour baseline. The contrast tests for pink/mint at 3.0 pass on the current palette (dark 5.23/9.32, light 4.18/3.99 — all ≥3.0), so Step 1 introduces no failing contrast assertion; only the leaf-rendering test is red until Step 3.

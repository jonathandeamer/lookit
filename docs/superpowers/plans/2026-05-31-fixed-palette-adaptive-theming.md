# Fixed Palette Adaptive Theming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement one fixed Lip Gloss-inspired Functional Bright palette that adapts to light and dark terminal backgrounds across the TUI and one-shot renderer, with automated contrast gates.

**Architecture:** Add semantic palette/style builders in `render/` and `tui/` while preserving the existing dependency direction and the split between Lip Gloss v1 in `render/` and Lip Gloss v2 in `tui/`. The TUI stores terminal background mode, reapplies styles on `tea.BackgroundColorMsg`, and passes that mode into the shared renderer.

**Tech Stack:** Go, `github.com/charmbracelet/colorprofile`, Lip Gloss v1 (`github.com/charmbracelet/lipgloss`) in `render/`, Bubble Tea v2 (`charm.land/bubbletea/v2`), Bubbles v2 (`charm.land/bubbles/v2`), Lip Gloss v2 (`charm.land/lipgloss/v2`) in `tui/`, Go unit tests, `make check`.

---

## Source Spec

Read first: `docs/superpowers/specs/2026-05-31-fixed-palette-adaptive-theming-design.md`.

The approved visual direction is **Functional Bright** with the **B3 violet selection shelf**.

## File Structure

- Modify `render/theme.go`: render package semantic palette, light/dark theme construction, WCAG helpers for render tests if kept package-local.
- Modify `render/render.go`: preserve `Render`, add `RenderWithBackground`, pass background mode to `NewTheme`.
- Modify `render/render_test.go`: add light/dark truecolor assertions and keep `NoTTY` ANSI-free assertions.
- Create `render/theme_test.go`: render palette contrast tests.
- Modify `tui/styles.go`: central TUI palette and component style builder.
- Create `tui/styles_test.go`: TUI palette contrast and component style assertions.
- Modify `tui/app.go`: store/reapply styles, handle `tea.BackgroundColorMsg`, request background colour in `Init`, use shared styles in `statusBarModel`.
- Modify `tui/reader.go`: pass background mode into `render.RenderWithBackground` and re-render on background changes.
- Modify `tui/list.go`: build the default list delegate from shared styles and expose an `applyStyles` helper.
- Modify `tui/app_test.go`, `tui/list_test.go`, `tui/statusbar_test.go`, `tui/reader_test.go`: update constructor expectations and add background/style tests.
- Keep `docs/superpowers/specs/2026-05-31-fixed-palette-adaptive-theming-design.md` unchanged unless execution discovers a contradiction; if it changes, commit that docs correction separately.

---

## Task 1: Render Package Palette, Contrast Gate, and Background-Aware API

**Files:**
- Modify: `render/theme.go`
- Modify: `render/render.go`
- Create: `render/theme_test.go`
- Modify: `render/render_test.go`

- [ ] **Step 1: Write failing render contrast and background tests**

Create `render/theme_test.go`:

```go
package render

import (
	"image/color"
	"math"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestRenderPaletteContrast(t *testing.T) {
	tests := []struct {
		name  string
		fg    color.Color
		bg    color.Color
		ratio float64
	}{
		{"dark text", renderPaletteFor(true).Text, renderPaletteFor(true).BaseBg, 4.5},
		{"dark dim", renderPaletteFor(true).Dim, renderPaletteFor(true).BaseBg, 4.5},
		{"dark field", renderPaletteFor(true).AccentPink, renderPaletteFor(true).BaseBg, 4.5},
		{"dark target", renderPaletteFor(true).AccentPink, renderPaletteFor(true).BaseBg, 4.5},
		{"dark warning", renderPaletteFor(true).AccentGold, renderPaletteFor(true).BaseBg, 4.5},
		{"dark error", renderPaletteFor(true).AccentRed, renderPaletteFor(true).BaseBg, 4.5},
		{"light text", renderPaletteFor(false).Text, renderPaletteFor(false).BaseBg, 4.5},
		{"light dim", renderPaletteFor(false).Dim, renderPaletteFor(false).BaseBg, 4.5},
		{"light field", renderPaletteFor(false).AccentPink, renderPaletteFor(false).BaseBg, 4.5},
		{"light target", renderPaletteFor(false).AccentPink, renderPaletteFor(false).BaseBg, 4.5},
		{"light warning", renderPaletteFor(false).AccentGold, renderPaletteFor(false).BaseBg, 4.5},
		{"light error", renderPaletteFor(false).AccentRed, renderPaletteFor(false).BaseBg, 4.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := contrastRatio(tc.fg, tc.bg); got < tc.ratio {
				t.Fatalf("contrast %.2f below %.2f", got, tc.ratio)
			}
		})
	}
}

func TestRenderThemeLightDarkColoursDiffer(t *testing.T) {
	dark := NewTheme(colorprofile.TrueColor, true)
	light := NewTheme(colorprofile.TrueColor, false)
	if sameColor(dark.Field.GetForeground(), light.Field.GetForeground()) {
		t.Fatal("field foreground should differ between dark and light backgrounds")
	}
	if sameColor(dark.Warning.GetForeground(), light.Warning.GetForeground()) {
		t.Fatal("warning foreground should differ between dark and light backgrounds")
	}
}

func contrastRatio(a, b color.Color) float64 {
	l1, l2 := relativeLuminance(a), relativeLuminance(b)
	if l2 > l1 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return 0.2126*linear(float64(r)/65535) +
		0.7152*linear(float64(g)/65535) +
		0.0722*linear(float64(b)/65535)
}

func linear(v float64) float64 {
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func sameColor(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
```

Append to `render/render_test.go`:

```go
func TestRenderWithBackgroundUsesLightPalette(t *testing.T) {
	body := []byte("Login: alice\nPlan: hello\n")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 123 * time.Millisecond, Bytes: len(body)}

	got := RenderWithBackground(target, body, meta, nil, colorprofile.TrueColor, false)
	if !strings.Contains(got, "\x1b[38;2;201;40;112mLogin:\x1b[0m") {
		t.Fatalf("light render missing light field colour escape:\n%q", got)
	}
}

func TestRenderWithBackgroundUsesDarkPalette(t *testing.T) {
	body := []byte("Login: alice\nPlan: hello\n")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 123 * time.Millisecond, Bytes: len(body)}

	got := RenderWithBackground(target, body, meta, nil, colorprofile.TrueColor, true)
	if !strings.Contains(got, "\x1b[38;2;255;95;162mLogin:\x1b[0m") {
		t.Fatalf("dark render missing dark field colour escape:\n%q", got)
	}
}
```

- [ ] **Step 2: Run render tests to verify failure**

Run:

```bash
go test ./render -run 'TestRenderPaletteContrast|TestRenderThemeLightDarkColoursDiffer|TestRenderWithBackgroundUses' -count=1 -v
```

Expected: FAIL because `renderPaletteFor`, `NewTheme(profile, darkBackground)`, and `RenderWithBackground` do not exist yet.

- [ ] **Step 3: Implement render palette and API**

In `render/theme.go`, replace the old colour var block and `NewTheme` implementation with:

```go
type renderPalette struct {
	Text, Dim, AccentPink, AccentViolet color.Color
	AccentMint, AccentGold, AccentRed  color.Color
	BaseBg                             color.Color
}

func renderPaletteFor(darkBackground bool) renderPalette {
	if darkBackground {
		return renderPalette{
			Text:         hexColor("#f0edf5"),
			Dim:          hexColor("#8c8792"),
			AccentPink:   hexColor("#ff5fa2"),
			AccentViolet: hexColor("#8a63ff"),
			AccentMint:   hexColor("#38e7ad"),
			AccentGold:   hexColor("#eed76d"),
			AccentRed:    hexColor("#ff6f87"),
			BaseBg:       hexColor("#171719"),
		}
	}
	return renderPalette{
		Text:         hexColor("#25222a"),
		Dim:          hexColor("#766f7d"),
		AccentPink:   hexColor("#c92870"),
		AccentViolet: hexColor("#6d43d6"),
		AccentMint:   hexColor("#007f62"),
		AccentGold:   hexColor("#765f00"),
		AccentRed:    hexColor("#c82f4d"),
		BaseBg:       hexColor("#fbfafc"),
	}
}

func hexColor(s string) color.RGBA {
	if len(s) != 7 || s[0] != '#' {
		panic("invalid colour literal: " + s)
	}
	return color.RGBA{
		R: fromHexByte(s[1], s[2]),
		G: fromHexByte(s[3], s[4]),
		B: fromHexByte(s[5], s[6]),
		A: 0xff,
	}
}

func fromHexByte(hi, lo byte) byte {
	return fromHexNibble(hi)<<4 | fromHexNibble(lo)
}

func fromHexNibble(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	default:
		panic("invalid colour literal")
	}
}

// NewTheme builds a Theme for the given profile and terminal
// background. On Ascii/NoTTY profiles, it returns a no-color theme that still
// preserves spacing.
func NewTheme(p colorprofile.Profile, darkBackground bool) Theme {
	noColor := p <= colorprofile.Ascii
	pal := renderPaletteFor(darkBackground)
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termProfile(p))
	renderer.SetHasDarkBackground(darkBackground)

	style := func(c color.Color, bold bool) lipgloss.Style {
		if noColor {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().
			Renderer(renderer).
			Foreground(lipgloss.Color(toHex(p.Convert(c)))).
			Bold(bold)
	}
	return Theme{
		Profile: p,
		NoColor: noColor,
		Arrow:   style(pal.AccentViolet, true),
		Target:  style(pal.AccentPink, true),
		Latency: style(pal.Dim, false),
		Sparkle: style(pal.AccentGold, false),
		Footer:  style(pal.Dim, false),
		Warning: style(pal.AccentGold, false),
		Field:   style(pal.AccentPink, true),
		ErrLine: style(pal.AccentRed, false),
	}
}
```

In `render/render.go`, add the Lip Gloss v1 import and split `Render`:

```go
import (
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
	"github.com/jonathandeamer/lookit/finger"
)

// Render formats a finger query result for the requested terminal color
// profile, using Lip Gloss v1's standalone background detection.
func Render(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile) string {
	return RenderWithBackground(t, body, meta, queryErr, profile, lipgloss.HasDarkBackground())
}

// RenderWithBackground formats a finger query result for a known terminal
// background mode. The TUI uses this so tea.BackgroundColorMsg can restyle a
// live session deterministically.
func RenderWithBackground(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile, darkBackground bool) string {
	theme := NewTheme(profile, darkBackground)
	var sb strings.Builder

	success := queryErr == nil
	sb.WriteString(renderHeader(theme, t, meta, success))

	if len(body) == 0 && success {
		sb.WriteString(theme.Footer.Render("(no response body)"))
		sb.WriteByte('\n')
	} else {
		sb.WriteString(highlightFields(theme, body))
		if len(body) > 0 && body[len(body)-1] != '\n' {
			sb.WriteByte('\n')
		}
	}

	notice := ""
	if meta.Truncated {
		notice = "truncated"
	}
	sb.WriteString(renderFooter(theme, meta, notice))

	if queryErr != nil {
		sb.WriteString(theme.ErrLine.Render(queryErr.Error()))
		sb.WriteByte('\n')
	}

	return sb.String()
}
```

- [ ] **Step 4: Run render tests to verify pass**

Run:

```bash
go test ./render -count=1
```

Expected: PASS, except existing truecolor golden tests may fail because intentional palette escapes changed. If goldens fail, continue to Step 5.

- [ ] **Step 5: Make render golden tests deterministic and update intentional goldens**

In `render/render_test.go`, change every existing truecolor golden call from:

```go
got := Render(target, body, meta, nil, colorprofile.TrueColor)
```

to:

```go
got := RenderWithBackground(target, body, meta, nil, colorprofile.TrueColor, true)
```

For the timeout test, preserve `queryErr`:

```go
got := RenderWithBackground(target, body, meta, queryErr, colorprofile.TrueColor, true)
```

Leave `TestRender_BasicNoTTY` and `TestRender_NoTTY_HasNoANSI` using `Render`, because the no-colour path is background-independent.

Run:

```bash
go test ./render -run 'TestRender_BasicTrueColor|TestRender_AsciiArtPreserved|TestRender_EmptyResponse|TestRender_Truncated|TestRender_Timeout' -update -count=1
go test ./render -count=1
```

Expected: PASS. Confirm `basic.notty.golden` is unchanged; `NoTTY` output must remain plain.

- [ ] **Step 6: Commit render package changes**

Run:

```bash
git add render/theme.go render/render.go render/theme_test.go render/render_test.go render/testdata
git commit -m "feat(render): add adaptive fixed palette"
```

---

## Task 2: TUI Semantic Palette and Contrast Gate

**Files:**
- Modify: `tui/styles.go`
- Create: `tui/styles_test.go`

- [ ] **Step 1: Write failing TUI palette tests**

Create `tui/styles_test.go`:

```go
package tui

import (
	"image/color"
	"math"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestTUIPaletteContrast(t *testing.T) {
	for _, dark := range []bool{true, false} {
		p := paletteFor(dark)
		tests := []struct {
			name  string
			fg    color.Color
			bg    color.Color
			ratio float64
		}{
			{"text on base", p.Text, p.BaseBg, 4.5},
			{"dim on base", p.Dim, p.BaseBg, 4.5},
			{"pink on base", p.AccentPink, p.BaseBg, 4.5},
			{"violet on base", p.AccentViolet, p.BaseBg, 4.5},
			{"mint on base", p.AccentMint, p.BaseBg, 4.5},
			{"gold on base", p.AccentGold, p.BaseBg, 4.5},
			{"red on base", p.AccentRed, p.BaseBg, 4.5},
			{"status text", p.BarText, p.SubtleBg, 4.5},
			{"help key", p.AccentViolet, p.SubtleBg, 4.5},
			{"help desc", p.BarText, p.SubtleBg, 4.5},
			{"selected login", p.SelectionLogin, p.SelectionBg, 4.5},
			{"selected desc", p.SelectionDesc, p.SelectionBg, 4.5},
			{"selection rail", p.AccentViolet, p.SelectionBg, 3.0},
		}
		for _, tc := range tests {
			t.Run(modeName(dark)+" "+tc.name, func(t *testing.T) {
				if got := contrastRatio(tc.fg, tc.bg); got < tc.ratio {
					t.Fatalf("contrast %.2f below %.2f", got, tc.ratio)
				}
			})
		}
	}
}

func TestNewStylesUsesFunctionalBrightRoles(t *testing.T) {
	st := newStyles(true)
	if !sameColor(st.palette.AccentViolet, st.barBadge.GetBackground()) {
		t.Fatalf("bar badge background should use accent violet")
	}
	if !sameColor(st.palette.AccentMint, st.spinner.GetForeground()) {
		t.Fatalf("spinner should use accent mint")
	}
	if !sameColor(st.palette.AccentViolet, st.listItem.SelectedTitle.GetBorderLeftForeground()) {
		t.Fatalf("selected title rail should use accent violet")
	}
	if !sameColor(st.palette.SelectionBg, st.listItem.SelectedTitle.GetBackground()) {
		t.Fatalf("selected title background should use selection bg")
	}
	if !sameColor(st.palette.AccentPink, st.listItem.SelectedTitle.GetForeground()) {
		t.Fatalf("selected title foreground should use accent pink")
	}
}

func modeName(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}

func contrastRatio(a, b color.Color) float64 {
	l1, l2 := relativeLuminance(a), relativeLuminance(b)
	if l2 > l1 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return 0.2126*linear(float64(r)/65535) +
		0.7152*linear(float64(g)/65535) +
		0.0722*linear(float64(b)/65535)
}

func linear(v float64) float64 {
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func sameColor(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

func TestHexColorAcceptsFunctionalBrightValues(t *testing.T) {
	if !sameColor(hexColor("#8a63ff"), lipgloss.Color("#8a63ff")) {
		t.Fatal("hexColor should produce the same RGBA value as lipgloss.Color")
	}
}
```

- [ ] **Step 2: Run TUI style tests to verify failure**

Run:

```bash
go test ./tui -run 'TestTUIPaletteContrast|TestNewStylesUsesFunctionalBrightRoles|TestHexColorAcceptsFunctionalBrightValues' -count=1 -v
```

Expected: FAIL because `paletteFor`, `hexColor`, and the expanded `styles` fields do not exist yet.

- [ ] **Step 3: Replace `tui/styles.go` with the semantic style builder**

Replace `tui/styles.go` with:

```go
package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type palette struct {
	Text, Dim, AccentPink, AccentViolet lipgloss.Color
	AccentMint, AccentGold, AccentRed   lipgloss.Color
	BaseBg, SubtleBg, SelectionBg, Rule lipgloss.Color
	BarText, SelectionLogin, SelectionDesc lipgloss.Color
}

type styles struct {
	palette palette

	// bottom status bar
	barFill  lipgloss.Style
	barHost  lipgloss.Style
	barSep   lipgloss.Style
	barUser  lipgloss.Style
	barFlag  lipgloss.Style
	barWarn  lipgloss.Style
	barDim   lipgloss.Style
	barBadge lipgloss.Style

	input    textinput.Styles
	help     help.Styles
	spinner  lipgloss.Style
	list     list.Styles
	listItem list.DefaultItemStyles
}

func paletteFor(dark bool) palette {
	if dark {
		return palette{
			Text:           lipgloss.Color("#f0edf5"),
			Dim:            lipgloss.Color("#8c8792"),
			AccentPink:     lipgloss.Color("#ff5fa2"),
			AccentViolet:   lipgloss.Color("#8a63ff"),
			AccentMint:     lipgloss.Color("#38e7ad"),
			AccentGold:     lipgloss.Color("#eed76d"),
			AccentRed:      lipgloss.Color("#ff6f87"),
			BaseBg:         lipgloss.Color("#171719"),
			SubtleBg:       lipgloss.Color("#292631"),
			SelectionBg:    lipgloss.Color("#342747"),
			Rule:           lipgloss.Color("#35313d"),
			BarText:        lipgloss.Color("#bbb3c8"),
			SelectionLogin: lipgloss.Color("#ff86ba"),
			SelectionDesc:  lipgloss.Color("#cbc1dc"),
		}
	}
	return palette{
		Text:           lipgloss.Color("#25222a"),
		Dim:            lipgloss.Color("#766f7d"),
		AccentPink:     lipgloss.Color("#c92870"),
		AccentViolet:   lipgloss.Color("#6d43d6"),
		AccentMint:     lipgloss.Color("#007f62"),
		AccentGold:     lipgloss.Color("#765f00"),
		AccentRed:      lipgloss.Color("#c82f4d"),
		BaseBg:         lipgloss.Color("#fbfafc"),
		SubtleBg:       lipgloss.Color("#e9e4f0"),
		SelectionBg:    lipgloss.Color("#f3e9f4"),
		Rule:           lipgloss.Color("#ded8e8"),
		BarText:        lipgloss.Color("#4c4554"),
		SelectionLogin: lipgloss.Color("#a81f62"),
		SelectionDesc:  lipgloss.Color("#6f6677"),
	}
}

func hexColor(s string) lipgloss.Color {
	return lipgloss.Color(s)
}

func newStyles(dark bool) styles {
	p := paletteFor(dark)
	bar := lipgloss.NewStyle().Background(p.SubtleBg)

	inputStyles := textinput.DefaultStyles(dark)
	inputStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(p.AccentViolet)
	inputStyles.Focused.Text = lipgloss.NewStyle().Foreground(p.Text)
	inputStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(p.Dim)
	inputStyles.Blurred.Prompt = lipgloss.NewStyle().Foreground(p.Dim)
	inputStyles.Blurred.Text = lipgloss.NewStyle().Foreground(p.Text)
	inputStyles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(p.Dim)
	inputStyles.Cursor.Color = p.AccentPink
	inputStyles.Cursor.Shape = tea.CursorBar

	helpStyles := help.Styles{
		ShortKey:       lipgloss.NewStyle().Foreground(p.AccentViolet),
		ShortDesc:      lipgloss.NewStyle().Foreground(p.BarText),
		ShortSeparator: lipgloss.NewStyle().Foreground(p.Rule),
		Ellipsis:       lipgloss.NewStyle().Foreground(p.Rule),
		FullKey:        lipgloss.NewStyle().Foreground(p.AccentViolet),
		FullDesc:       lipgloss.NewStyle().Foreground(p.BarText),
		FullSeparator:  lipgloss.NewStyle().Foreground(p.Rule),
	}

	listStyles := list.DefaultStyles(dark)
	listStyles.Filter = inputStyles
	listStyles.Spinner = lipgloss.NewStyle().Foreground(p.AccentMint)
	listStyles.DefaultFilterCharacterMatch = lipgloss.NewStyle().Foreground(p.AccentPink).Underline(true)
	listStyles.StatusBar = lipgloss.NewStyle().Foreground(p.BarText)
	listStyles.StatusEmpty = lipgloss.NewStyle().Foreground(p.Dim)
	listStyles.StatusBarActiveFilter = lipgloss.NewStyle().Foreground(p.Text)
	listStyles.StatusBarFilterCount = lipgloss.NewStyle().Foreground(p.Dim)
	listStyles.NoItems = lipgloss.NewStyle().Foreground(p.Dim)
	listStyles.PaginationStyle = lipgloss.NewStyle().Foreground(p.BarText).PaddingLeft(2)
	listStyles.HelpStyle = lipgloss.NewStyle().Foreground(p.BarText).Padding(1, 0, 0, 2)
	listStyles.ActivePaginationDot = lipgloss.NewStyle().Foreground(p.AccentViolet).SetString("•")
	listStyles.InactivePaginationDot = lipgloss.NewStyle().Foreground(p.Rule).SetString("•")
	listStyles.ArabicPagination = lipgloss.NewStyle().Foreground(p.BarText)
	listStyles.DividerDot = lipgloss.NewStyle().Foreground(p.Rule).SetString(" • ")

	itemStyles := list.NewDefaultItemStyles(dark)
	itemStyles.NormalTitle = lipgloss.NewStyle().Foreground(p.Text).Padding(0, 0, 0, 2)
	itemStyles.NormalDesc = lipgloss.NewStyle().Foreground(p.Dim).Padding(0, 0, 0, 2)
	itemStyles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(p.AccentViolet).
		Background(p.SelectionBg).
		Foreground(p.SelectionLogin).
		Padding(0, 0, 0, 1)
	itemStyles.SelectedDesc = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(p.AccentViolet).
		Background(p.SelectionBg).
		Foreground(p.SelectionDesc).
		Padding(0, 0, 0, 1)
	itemStyles.DimmedTitle = lipgloss.NewStyle().Foreground(p.Dim).Padding(0, 0, 0, 2)
	itemStyles.DimmedDesc = lipgloss.NewStyle().Foreground(p.Rule).Padding(0, 0, 0, 2)
	itemStyles.FilterMatch = lipgloss.NewStyle().Foreground(p.AccentPink).Underline(true)

	return styles{
		palette: p,
		barFill: bar,
		barHost: bar.Foreground(p.BarText),
		barSep:  bar.Foreground(p.Rule),
		barUser: bar.Foreground(p.Text).Bold(true),
		barFlag: bar.Foreground(p.BarText),
		barWarn: bar.Foreground(p.AccentGold),
		barDim:  bar.Foreground(p.BarText),
		barBadge: lipgloss.NewStyle().
			Background(p.AccentViolet).
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true),
		input:    inputStyles,
		help:     helpStyles,
		spinner:  lipgloss.NewStyle().Foreground(p.AccentMint),
		list:     listStyles,
		listItem: itemStyles,
	}
}
```

- [ ] **Step 4: Run TUI style tests to verify pass**

Run:

```bash
go test ./tui -run 'TestTUIPaletteContrast|TestNewStylesUsesFunctionalBrightRoles|TestHexColorAcceptsFunctionalBrightValues' -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Commit TUI style builder**

Run:

```bash
git add tui/styles.go tui/styles_test.go
git commit -m "feat(tui): add fixed adaptive palette styles"
```

---

## Task 3: Wire Background Mode and Styles Through the App

**Files:**
- Modify: `tui/app.go`
- Modify: `tui/reader.go`
- Modify: `tui/app_test.go`
- Modify: `tui/reader_test.go`

- [ ] **Step 1: Write failing background message tests**

Append to `tui/app_test.go`:

```go
func TestBackgroundColorMsgRestylesTUI(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	oldBg := m.common.styles.palette.BaseBg

	next, _ := m.Update(tea.BackgroundColorMsg{Color: color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}})
	got := next.(appModel)

	if got.common.darkBackground {
		t.Fatal("darkBackground = true after light background message, want false")
	}
	if sameColor(got.common.styles.palette.BaseBg, oldBg) {
		t.Fatal("palette base background did not change")
	}
	if !sameColor(got.helpModel.Styles.FullKey.GetForeground(), got.common.styles.help.FullKey.GetForeground()) {
		t.Fatal("help styles were not reapplied")
	}
	if !sameColor(got.spin.Style.GetForeground(), got.common.styles.spinner.GetForeground()) {
		t.Fatal("spinner style was not reapplied")
	}
	if !sameColor(got.input.Styles().Focused.Prompt.GetForeground(), got.common.styles.input.Focused.Prompt.GetForeground()) {
		t.Fatal("input styles were not reapplied")
	}
}

func TestBackgroundColorMsgRerendersCurrentReader(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	target := hostTarget(t, "alice@plan.cat")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: target, Body: []byte("Login: alice\n")}})
	m = step.(appModel)
	if !strings.Contains(m.reader.viewport.View(), "\x1b[38;2;255;95;162mLogin:\x1b[0m") {
		t.Fatalf("precondition: reader did not render dark field colour:\n%q", m.reader.viewport.View())
	}

	step, _ = m.Update(tea.BackgroundColorMsg{Color: color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}})
	got := step.(appModel)
	if !strings.Contains(got.reader.viewport.View(), "\x1b[38;2;201;40;112mLogin:\x1b[0m") {
		t.Fatalf("reader did not re-render with light field colour:\n%q", got.reader.viewport.View())
	}
}
```

Add `image/color` to the import block in `tui/app_test.go`.

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./tui -run 'TestBackgroundColorMsgRestylesTUI|TestBackgroundColorMsgRerendersCurrentReader' -count=1 -v
```

Expected: FAIL because `common.styles`, `common.darkBackground`, background handling, and reader background rendering are not wired yet.

- [ ] **Step 3: Add shared style state to `commonModel` and `newApp`**

In `tui/app.go`, change `commonModel`:

```go
type commonModel struct {
	width          int
	height         int
	profile        colorprofile.Profile
	darkBackground bool
	styles         styles
	fetch          FetchFunc
}
```

In `newApp`, initialize styles before constructing components:

```go
	st := newStyles(true)
	common := &commonModel{
		profile:        profile,
		darkBackground: true,
		styles:         st,
		fetch:          fetch,
	}
	in := textinput.New()
	in.Placeholder = "alice@plan.cat"
	in.Prompt = "target: "
	in.CharLimit = 256
	in.SetWidth(40)
	in.SetStyles(st.input)
	in.Focus() // landing starts focused
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
	}
	app.reader.setBackground(common.darkBackground)
	app.reader.styles = st
	app.helpModel.Styles = st.help
	app.updateKeymap()
	return app
```

- [ ] **Step 4: Add style application helpers**

In `tui/app.go`, after `newApp`, add:

```go
func (m *appModel) setBackground(dark bool) {
	m.common.darkBackground = dark
	m.common.styles = newStyles(dark)
	m.applyStyles()
}

func (m *appModel) applyStyles() {
	st := m.common.styles
	m.input.SetStyles(st.input)
	m.helpModel.Styles = st.help
	m.spin.Style = st.spinner
	m.reader.styles = st
	m.reader.setBackground(m.common.darkBackground)
	if m.listReady {
		m.list.applyStyles(st)
	}
}
```

- [ ] **Step 5: Request and handle background colour messages**

In `tui/app.go` `Init`, add `tea.RequestBackgroundColor()` to the batch:

```go
func (m appModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		tea.RequestBackgroundColor,
		tea.RequestCapability("RGB"),
		tea.RequestCapability("Tc"),
	)
}
```

In `Update`, add the case after `ColorProfileMsg`:

```go
	case tea.BackgroundColorMsg:
		m.setBackground(msg.IsDark())
		return m, nil
```

In the existing `ColorProfileMsg` case, keep profile update and re-render:

```go
	case tea.ColorProfileMsg:
		m.common.profile = msg.Profile
		m.reader.setProfile(msg.Profile)
		return m, nil
```

- [ ] **Step 6: Pass background mode through the reader renderer**

In `tui/reader.go`, add the `darkBackground` field:

```go
type readerModel struct {
	viewport       viewport.Model
	current        *Entry
	profile        colorprofile.Profile
	darkBackground bool
	styles         styles
	width          int
	height         int
}
```

Change `newReader` return:

```go
return readerModel{viewport: vp, profile: profile, darkBackground: true, styles: newStyles(true)}
```

Add:

```go
func (m *readerModel) setBackground(dark bool) {
	m.darkBackground = dark
	if m.current != nil {
		m.viewport.SetContent(renderEntry(m.profile, m.darkBackground, *m.current))
	}
}
```

Update `setProfile` and `setEntry`:

```go
func (m *readerModel) setProfile(p colorprofile.Profile) {
	m.profile = p
	if m.current != nil {
		m.viewport.SetContent(renderEntry(m.profile, m.darkBackground, *m.current))
	}
}

func (m *readerModel) setEntry(entry Entry) {
	m.current = &entry
	m.viewport.SetContent(renderEntry(m.profile, m.darkBackground, entry))
}

func renderEntry(profile colorprofile.Profile, darkBackground bool, entry Entry) string {
	return render.RenderWithBackground(entry.Target, entry.Body, entry.Meta, entry.Err, profile, darkBackground)
}
```

- [ ] **Step 7: Update reader tests for the new renderEntry signature**

If any test calls `renderEntry`, update the call to pass `true` for the dark-background default:

```go
got := renderEntry(colorprofile.TrueColor, true, entry)
```

Existing `newReader(colorprofile.NoTTY)` tests should still compile because `newReader` keeps the same signature.

- [ ] **Step 8: Run focused app tests**

Run:

```bash
go test ./tui -run 'TestBackgroundColorMsgRestylesTUI|TestBackgroundColorMsgRerendersCurrentReader|TestColorProfileMsgPropagates|TestLoadingShowsSpinnerTarget' -count=1 -v
```

Expected: FAIL only if list style application is not implemented yet; if failure mentions `applyStyles` on `listModel`, continue to Task 4 before rerunning the full package.

- [ ] **Step 9: Commit app/reader background wiring**

Commit only after Task 4 if Task 3 cannot compile without list helpers. If Task 3 compiles independently, run:

```bash
git add tui/app.go tui/reader.go tui/app_test.go tui/reader_test.go
git commit -m "feat(tui): react to terminal background changes"
```

---

## Task 4: Apply Shared Styles to List, Status Bar, Help, Input, and Spinner

**Files:**
- Modify: `tui/list.go`
- Modify: `tui/statusbar.go`
- Modify: `tui/app.go`
- Modify: `tui/list_test.go`
- Modify: `tui/statusbar_test.go`

- [ ] **Step 1: Write failing list/style integration tests**

Append to `tui/list_test.go`:

```go
func TestNewListUsesSharedStyles(t *testing.T) {
	common := testCommon()
	common.styles = newStyles(false)
	common.darkBackground = false
	m := newList(common, hostTarget(t, "@tilde.team"), []User{{Login: "alrs", Name: "Alvaro"}})

	if !sameColor(m.list.Styles.Filter.Focused.Prompt.GetForeground(), common.styles.input.Focused.Prompt.GetForeground()) {
		t.Fatal("list filter prompt should use shared input prompt colour")
	}
	if !sameColor(m.list.Styles.Spinner.GetForeground(), common.styles.spinner.GetForeground()) {
		t.Fatal("list spinner should use shared spinner colour")
	}
	if !strings.Contains(m.View(), "\x1b[38;2;168;31;98m") {
		t.Fatalf("light selected row should contain selected login colour:\n%s", m.View())
	}
}

func TestListApplyStylesUpdatesExistingList(t *testing.T) {
	common := testCommon()
	common.styles = newStyles(true)
	m := newList(common, hostTarget(t, "@tilde.team"), []User{{Login: "alrs", Name: "Alvaro"}})

	m.applyStyles(newStyles(false))
	if !sameColor(m.list.Styles.Filter.Focused.Prompt.GetForeground(), newStyles(false).input.Focused.Prompt.GetForeground()) {
		t.Fatal("applyStyles should update list filter prompt")
	}
	if !strings.Contains(m.View(), "\x1b[38;2;168;31;98m") {
		t.Fatalf("applyStyles should update selected row render:\n%s", m.View())
	}
}
```

- [ ] **Step 2: Write failing status bar badge test**

Append to `tui/statusbar_test.go`:

```go
func TestStatusBarUsesBadgeStyle(t *testing.T) {
	st := newStyles(true)
	out := (statusBar{host: "@tilde.team", hints: "? help", width: 40, styles: st}).render()
	if strings.Contains(out, "STATUS") {
		t.Fatal("status bar should not invent a STATUS badge")
	}
	if !sameColor(st.barBadge.GetBackground(), st.palette.AccentViolet) {
		t.Fatal("bar badge should use accent violet")
	}
}
```

- [ ] **Step 3: Run list/status tests to verify failure**

Run:

```bash
go test ./tui -run 'TestNewListUsesSharedStyles|TestListApplyStylesUpdatesExistingList|TestStatusBarUsesBadgeStyle' -count=1 -v
```

Expected: FAIL because `newList` still overrides local colours, `applyStyles` does not exist, and status styles are not fully centralized.

- [ ] **Step 4: Implement shared list delegate and style application**

In `tui/list.go`, replace the local delegate styling block in `newList` with:

```go
	st := common.styles
	d := defaultUserDelegate(st)
	l := list.New(items, d, width, height)
	applyListStyles(&l, st)
```

Add helpers near `newList`:

```go
func defaultUserDelegate(st styles) list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles = st.listItem
	d.SetSpacing(0)
	return d
}

func applyListStyles(l *list.Model, st styles) {
	l.Styles = st.list
	l.FilterInput.SetStyles(st.list.Filter)
	l.Help.Styles = st.help
	l.Paginator.ActiveDot = st.list.ActivePaginationDot.String()
	l.Paginator.InactiveDot = st.list.InactivePaginationDot.String()
	l.SetDelegate(defaultUserDelegate(st))
}

func (m *listModel) applyStyles(st styles) {
	applyListStyles(&m.list, st)
}
```

Remove the `charm.land/lipgloss/v2` and `charm.land/lipgloss/v2/compat` imports from `tui/list.go` if they are no longer used.

- [ ] **Step 5: Ensure test helpers initialize styles**

In `tui/list_test.go`, update `testCommon`:

```go
func testCommon() *commonModel {
	return &commonModel{width: 80, height: 24, darkBackground: true, styles: newStyles(true)}
}
```

Also update the direct `commonModel` literal in `TestDefaultDelegateRendersLoginAndName`:

```go
common := &commonModel{width: 80, height: 20, darkBackground: true, styles: newStyles(true)}
```

- [ ] **Step 6: Use common styles in status bar rendering**

In `tui/app.go`, change `statusBarModel`:

```go
	st := m.common.styles
```

Remove `st := newStyles()` from `statusBarModel`.

Existing statusbar unit tests can keep calling `newStyles(true)` directly. Update all `newStyles()` calls in tests to `newStyles(true)`.

- [ ] **Step 7: Run full TUI package tests**

Run:

```bash
go test ./tui -count=1
```

Expected: PASS. If old tests fail with `not enough arguments in call to newStyles`, update those calls to `newStyles(true)` for dark-mode structural tests.

- [ ] **Step 8: Commit TUI style integration**

If Task 3 was not committed because it needed list helpers, include those files here:

```bash
git add tui/app.go tui/reader.go tui/list.go tui/statusbar.go tui/app_test.go tui/reader_test.go tui/list_test.go tui/statusbar_test.go
git commit -m "feat(tui): apply adaptive palette to chrome"
```

If Task 3 already committed, run:

```bash
git add tui/list.go tui/statusbar.go tui/app.go tui/list_test.go tui/statusbar_test.go
git commit -m "feat(tui): apply palette to lists and status bar"
```

---

## Task 5: Regression Tests for Plain Output, Help Legibility, and Existing Navigation

**Files:**
- Modify: `tui/app_test.go`
- Modify: `render/render_test.go`

- [ ] **Step 1: Add help panel style regression test**

Append to `tui/app_test.go`:

```go
func TestHelpPanelUsesSharedContrastStyles(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = step.(appModel)

	if !m.helpModel.ShowAll {
		t.Fatal("precondition: help panel should be expanded")
	}
	if !sameColor(m.helpModel.Styles.FullKey.GetForeground(), m.common.styles.palette.AccentViolet) {
		t.Fatal("help key colour should use accent violet")
	}
	if !sameColor(m.helpModel.Styles.FullDesc.GetForeground(), m.common.styles.palette.BarText) {
		t.Fatal("help description colour should use bar text")
	}
	view := m.View().Content
	if !strings.Contains(view, "back") || !strings.Contains(view, "raw") {
		t.Fatalf("help panel should still render enabled keys:\n%s", view)
	}
}
```

- [ ] **Step 2: Add render plain-output regression**

Append to `render/render_test.go`:

```go
func TestRenderWithBackgroundNoTTYHasNoANSI(t *testing.T) {
	body := []byte("Login: alice\nPlan: hello\n")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 123 * time.Millisecond, Bytes: len(body)}

	got := RenderWithBackground(target, body, meta, nil, colorprofile.NoTTY, false)
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("NoTTY output contains ANSI escape sequence: %q", got)
	}
	if !strings.Contains(got, "Login: alice") {
		t.Fatalf("NoTTY output should preserve body text: %q", got)
	}
}
```

- [ ] **Step 3: Run regression tests**

Run:

```bash
go test ./tui -run 'TestHelpPanelUsesSharedContrastStyles|TestHelpPanelHidesInertKeys|TestUpdateKeymapGatesByState|TestViewSetsNoMouseMode' -count=1 -v
go test ./render -run 'TestRenderWithBackgroundNoTTYHasNoANSI|TestRender_NoTTY_HasNoANSI|TestRender_BasicNoTTY' -count=1 -v
```

Expected: PASS.

- [ ] **Step 4: Commit regression coverage**

Run:

```bash
git add tui/app_test.go render/render_test.go
git commit -m "test: cover adaptive theme regressions"
```

---

## Task 6: Documentation Note, Full Gate, and Manual Smoke

**Files:**
- Modify: `docs/superpowers/plans/2026-05-31-fixed-palette-adaptive-theming.md`

- [ ] **Step 1: Add the required deferred-work note at the end of this plan**

Append this exact section to this plan if it is not already present:

```markdown
## Previously Deferred Work Resolved By This Plan

This plan resolves the current-app theming debt captured in earlier planning docs:

- adaptive light/dark colours for `tui/` and `render/`,
- selected list-row legibility,
- spinner foreground colour,
- help-panel contrast,
- the shared `render/` field-highlight palette,
- live `tea.BackgroundColorMsg` handling,
- automated contrast tests for the fixed palette.
```

- [ ] **Step 2: Run formatting and package tests**

Run:

```bash
make fmt
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Run the full CI gate**

Run:

```bash
make check
```

Expected: PASS. If `gofmt -l .` reports files, run `make fmt`, inspect the diff, then rerun `make check`.

- [ ] **Step 4: Manual TUI smoke in a real terminal**

Run:

```bash
make build
./lookit
```

Manual checks:

- dark terminal: landing input prompt is violet/pink and readable;
- `?` help panel keys/descriptions/separators are readable;
- `@tilde.team` or another host list opens with the B3 violet selection shelf;
- selected login and description remain readable;
- spinner appears mint while a fetch is in flight;
- status bar flags are legible;
- native mouse selection still works because mouse capture remains off;
- light terminal: repeat the help panel and list selection checks.

The TUI needs a real terminal; do not invent a headless smoke test for this step.

- [ ] **Step 5: Commit final docs/gate cleanup**

Run:

```bash
git status --short
git add docs/superpowers/plans/2026-05-31-fixed-palette-adaptive-theming.md
git commit -m "docs: plan fixed adaptive palette"
```

If `make fmt` changed Go files during Step 2, include those Go files in the commit that introduced the formatting change, not in this docs commit.

## Previously Deferred Work Resolved By This Plan

This plan resolves the current-app theming debt captured in earlier planning docs:

- adaptive light/dark colours for `tui/` and `render/`,
- selected list-row legibility,
- spinner foreground colour,
- help-panel contrast,
- the shared `render/` field-highlight palette,
- live `tea.BackgroundColorMsg` handling,
- automated contrast tests for the fixed palette.

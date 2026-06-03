# About Screen Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full-screen, `a`-opened "about" screen that re-homes lookit's wordmark and version, adds early-release personality, and lets the user finger the author (`↵`) or copy the issues URL (`y`) — leaving the landing a bare `target:` prompt.

**Architecture:** A new `stateAbout` joins the `stateReader`/`stateList` enum, driven by a pure-render `aboutModel` (new `tui/about.go`) that `appModel` sizes and themes like the reader. The `☞ lookit` `headerMark` moves off the landing chrome and becomes the about hero; the version line moves out of the `?` help panel; version/build data is plumbed structurally with a `debug.ReadBuildInfo()` fallback.

**Tech Stack:** Go, Bubble Tea v2 (`charm.land/bubbletea/v2`), Bubbles v2, Lip Gloss v2 (`charm.land/lipgloss/v2`), `colorprofile`. Spec: `docs/superpowers/specs/2026-06-03-about-screen-design.md`.

---

## File Structure

- **Create `tui/about.go`** — `aboutModel` + the pure `aboutView(...)` render function and the about-screen string constants. One responsibility: rendering the about screen.
- **Create `tui/about_test.go`** — golden/behaviour tests for `aboutView`.
- **Modify `tui/keys.go`** — add the `About` (`a`) binding and list it in `FullHelp`.
- **Modify `tui/run.go`** — `Options` gains `BuiltAt`; `Version` becomes the bare version.
- **Modify `main.go`** — `debug.ReadBuildInfo()` fallback + `vcsDate`; pass `Version`/`BuiltAt`.
- **Modify `tui/app.go`** — `stateAbout`, the `about` field, `openAbout`/`closeAbout`, key routing, `View`, status bar, sizing, theme propagation; drop the `headerMark` chrome row and the help-panel version band.
- **Modify tests** — `tui/keys_test.go`, `tui/app_test.go`, `main_test.go`.

Each task ends green (`go build ./...` compiles, touched tests pass). `headerMark`, `heroManicule`, `heroWordmark` (landing.go) are **kept** — the about hero reuses them.

---

## Task 1: The about view (pure render)

**Files:**
- Create: `tui/about.go`
- Test: `tui/about_test.go`

- [ ] **Step 1: Write the failing tests**

Create `tui/about_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestAboutViewRendersIdentityAndActions(t *testing.T) {
	out := aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 80, 24)
	plain := stripANSIForLandingTest(out)
	for _, want := range []string{
		heroWordmark,
		aboutTagline,
		"lookit v0.0.1 · MIT license",
		"built 2026-06-03",
		aboutRepo,
		"Built with Charm · charm.sh",
		"young software — bug reports & ideas welcome",
		"finger jonathan@tilde.team",
		"↵ go",
		"report a bug or idea",
		"y copy",
		"thanks for supporting the small internet",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("about view missing %q:\n%s", want, plain)
		}
	}
	for _, bad := range []string{"RFC 1288", "telemetry", "read-only"} {
		if strings.Contains(plain, bad) {
			t.Fatalf("about view should not contain %q:\n%s", bad, plain)
		}
	}
}

func TestAboutViewHidesBuildRowWhenUnknown(t *testing.T) {
	plain := stripANSIForLandingTest(aboutView(newStyles(true), colorprofile.TrueColor, "dev", "unknown", 80, 24))
	if strings.Contains(plain, "built ") {
		t.Fatalf("about view should hide the build row when builtAt is unknown:\n%s", plain)
	}
	if !strings.Contains(plain, "lookit dev · MIT license") {
		t.Fatalf("about view should still show the dev version line:\n%s", plain)
	}
}

func TestAboutViewHeroGradientByProfile(t *testing.T) {
	tc := aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 80, 24)
	if got := len(foregroundSequences(tc)); got < 3 {
		t.Fatalf("truecolor about hero should sweep >=3 colours, got %d", got)
	}
	an := aboutView(newStyles(true), colorprofile.ANSI, "v0.0.1", "2026-06-03", 80, 24)
	if got := len(foregroundSequences(an)); got > 2 {
		t.Fatalf("ANSI about hero should fall back to a solid wordmark, got %d colours", got)
	}
}

func TestAboutViewNarrowTruncatesLongLines(t *testing.T) {
	wide := stripANSIForLandingTest(aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 80, 24))
	narrow := stripANSIForLandingTest(aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 28, 24))
	if !strings.Contains(narrow, heroWordmark) {
		t.Fatalf("narrow about view should still show the wordmark:\n%s", narrow)
	}
	if strings.Contains(narrow, aboutTagline) {
		t.Fatalf("narrow about view should truncate the long tagline:\n%s", narrow)
	}
	if !strings.Contains(wide, aboutTagline) {
		t.Fatalf("wide about view should show the full tagline:\n%s", wide)
	}
}
```

`foregroundSequences` and `stripANSIForLandingTest` already exist in `tui/landing_test.go` (same package).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tui/ -run TestAboutView -count=1`
Expected: FAIL — `undefined: aboutView` (and `aboutTagline`, `aboutRepo`).

- [ ] **Step 3: Create `tui/about.go`**

```go
package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

const (
	aboutTagline      = "A modern TUI browser for the Finger protocol"
	aboutRepo         = "github.com/jonathandeamer/lookit"
	aboutFingerAuthor = "jonathan@tilde.team"
	aboutIssuesURL    = "https://github.com/jonathandeamer/lookit/issues"
)

// aboutModel renders the full-screen about view. Like readerModel it owns no
// lifecycle or quit; appModel drives it via setSize/setProfile/setBackground.
type aboutModel struct {
	profile colorprofile.Profile
	styles  styles
	version string // bare version, e.g. "v0.0.1" or "dev"
	builtAt string // build date, e.g. "2026-06-03"; "" / "unknown" hides the row
	width   int
	height  int
}

func newAbout(profile colorprofile.Profile, version, builtAt string) aboutModel {
	return aboutModel{
		profile: profile,
		styles:  newStyles(true),
		version: version,
		builtAt: builtAt,
	}
}

func (m *aboutModel) setSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *aboutModel) setProfile(p colorprofile.Profile) { m.profile = p }

func (m *aboutModel) setBackground(dark bool) { m.styles = newStyles(dark) }

func (m aboutModel) View() string {
	return aboutView(m.styles, m.profile, m.version, m.builtAt, m.width, m.height)
}

// aboutView composes the centered about block. Pure: string in, string out, so
// it is golden-testable. The identity lines are centered relative to each other;
// the personality and action groups are left-aligned internally and centered as
// blocks, matching the approved layout.
func aboutView(st styles, profile colorprofile.Profile, version, builtAt string, width, height int) string {
	dim := lipgloss.NewStyle().Foreground(st.palette.Dim)
	text := lipgloss.NewStyle().Foreground(st.palette.Text)
	spark := lipgloss.NewStyle().Foreground(st.palette.AccentMint)
	arrow := lipgloss.NewStyle().Foreground(st.palette.AccentViolet)

	identity := []string{
		headerMark(st, profile),
		dim.Render(aboutTagline),
		"",
		dim.Render("lookit " + version + " · MIT license"),
	}
	if builtAt != "" && builtAt != "unknown" {
		identity = append(identity, dim.Render("built "+builtAt))
	}
	identity = append(identity, dim.Render(aboutRepo))
	identityBlock := lipgloss.JoinVertical(lipgloss.Center, identity...)

	bullets := lipgloss.JoinVertical(
		lipgloss.Left,
		spark.Render("✦ ")+text.Render("Built with Charm · charm.sh"),
		spark.Render("✦ ")+text.Render("young software — bug reports & ideas welcome"),
	)

	// Right-pad the shorter action so both key hints align in a column.
	left1 := arrow.Render("➜ ") + text.Render("finger "+aboutFingerAuthor)
	left2 := arrow.Render("➜ ") + text.Render("report a bug or idea")
	leftW := lipgloss.Width(left1)
	if w := lipgloss.Width(left2); w > leftW {
		leftW = w
	}
	const hintGap = 6
	pad := func(s string) string {
		return s + strings.Repeat(" ", leftW-lipgloss.Width(s)+hintGap)
	}
	actions := lipgloss.JoinVertical(
		lipgloss.Left,
		pad(left1)+dim.Render("↵ go"),
		pad(left2)+dim.Render("y copy"),
	)

	block := lipgloss.JoinVertical(
		lipgloss.Center,
		identityBlock,
		"",
		bullets,
		"",
		actions,
		"",
		dim.Render("thanks for supporting the small internet"),
	)

	// Per-line truncation so long lines (tagline, repo URL) degrade on narrow
	// terminals instead of overflowing the placed width.
	if width > 0 {
		lines := strings.Split(block, "\n")
		for i, ln := range lines {
			lines[i] = ansi.Truncate(ln, width, "…")
		}
		block = strings.Join(lines, "\n")
	}

	if width <= 0 || height <= 0 {
		return block
	}
	vPos := lipgloss.Center
	if lipgloss.Height(block) >= height {
		vPos = lipgloss.Top // very short terminal: top-align rather than clip
	}
	return lipgloss.Place(width, height, lipgloss.Center, vPos, block)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./tui/ -run TestAboutView -count=1 -v`
Expected: PASS (all four `TestAboutView*`).

- [ ] **Step 5: Commit**

```bash
git add tui/about.go tui/about_test.go
git commit -m "feat(tui): add the about screen view"
```

---

## Task 2: The `a` about keybinding

**Files:**
- Modify: `tui/keys.go`
- Test: `tui/keys_test.go`

- [ ] **Step 1: Write the failing test**

Append to `tui/keys_test.go`:

```go
func TestKeyMapAboutBinding(t *testing.T) {
	k := newKeyMap()
	if got := k.About.Keys(); len(got) == 0 || !contains(got, "a") {
		t.Fatalf("About keys = %v, want to contain 'a'", got)
	}
	if h := k.About.Help(); h.Key != "a" || h.Desc != "about lookit" {
		t.Fatalf("About help = %+v, want {a, about lookit}", h)
	}
	var all []string
	for _, group := range k.FullHelp() {
		for _, b := range group {
			all = append(all, strings.Join(b.Keys(), ","))
		}
	}
	if !strings.Contains(strings.Join(all, " "), "a") {
		t.Fatalf("FullHelp should advertise the about key 'a': %v", all)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./tui/ -run TestKeyMapAboutBinding -count=1`
Expected: FAIL — `k.About undefined (type keyMap has no field or method About)`.

- [ ] **Step 3: Add the About binding**

In `tui/keys.go`, add the field to the `keyMap` struct (after `Help key.Binding`):

```go
	Help       key.Binding
	About      key.Binding
	Quit       key.Binding
```

In `newKeyMap()`, add (after the `Help:` line):

```go
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		About:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "about lookit")),
		Quit:       key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
```

In `FullHelp()`, add `About` to the last group:

```go
	return [][]key.Binding{
		{k.Open, k.FocusInput, k.Copy, k.Raw},
		{k.Move, k.Page, k.Jump, k.Filter},
		{k.Back, k.About, k.Quit},
	}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./tui/ -run 'TestKeyMap' -count=1 -v`
Expected: PASS (`TestKeyMapAboutBinding`, `TestKeyMapBindings`, `TestKeyMapFullHelpIncludesPageAndMoveKeys`).

- [ ] **Step 5: Commit**

```bash
git add tui/keys.go tui/keys_test.go
git commit -m "feat(tui): add the 'a' about keybinding"
```

---

## Task 3: Plumb structured version; drop it from the help panel

**Files:**
- Modify: `tui/run.go`, `main.go`, `tui/app.go`
- Test: `main_test.go`, `tui/app_test.go`

- [ ] **Step 1: Write the failing test for `vcsDate`**

Append to `main_test.go`:

```go
func TestVcsDate(t *testing.T) {
	if got := vcsDate("2026-06-03T10:20:30Z"); got != "2026-06-03" {
		t.Fatalf("vcsDate = %q, want 2026-06-03", got)
	}
	if got := vcsDate("short"); got != "short" {
		t.Fatalf("vcsDate passthrough = %q, want short", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test . -run TestVcsDate -count=1`
Expected: FAIL — `undefined: vcsDate`.

- [ ] **Step 3: Add `ReadBuildInfo` + `vcsDate` and pass structured version**

In `main.go`, add `"runtime/debug"` to the import block. Add this `init` + helper after the `var (...)` block:

```go
// init fills version/builtAt from the embedded build info when they were not set
// via -ldflags, so `go install …@latest` shows a real version + date instead of
// "dev"/"unknown". Release builds (ldflags set) keep their injected values.
func init() {
	if version != "dev" && builtAt != "unknown" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
	if builtAt == "unknown" {
		for _, s := range info.Settings {
			if s.Key == "vcs.time" {
				builtAt = vcsDate(s.Value)
			}
		}
	}
}

// vcsDate trims an RFC 3339 VCS timestamp to its date portion.
func vcsDate(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}
```

Change the `startTUI` call (was `Version: versionString()`):

```go
	if err := startTUI(tui.Options{InitialQuery: query, Seed: seed, Version: version, BuiltAt: builtAt}); err != nil {
```

Keep `versionString()` — it is still used by the `-v/--version` path.

- [ ] **Step 4: Update `Options` in `tui/run.go`**

Replace the `Version` field comment and add `BuiltAt`:

```go
	// Version is the bare build version (e.g. "v0.0.1"), shown on the about screen.
	Version string
	// BuiltAt is the build date (e.g. "2026-06-03"), shown on the about screen.
	// "" or "unknown" hides the build row.
	BuiltAt string
```

- [ ] **Step 5: Drop the version band from the help panel and `commonModel`**

In `tui/app.go`, remove the `version` field from `commonModel` (delete the `version string` line). Remove `version: opts.Version,` from the `commonModel` literal in `newAppWithOptions`.

Replace `helpView` (the whole function) with:

```go
func (m appModel) helpView() string {
	st := m.common.styles
	w := m.common.width
	return fullWidthHelpView(m.keys.FullHelp(), st, w, m.helpModel.FullSeparator)
}
```

- [ ] **Step 6: Remove the obsolete help-version tests**

In `tui/app_test.go`, delete `TestHelpPanelVersionSharesFirstKeyRow` and `TestHelpPanelOmitsVersionRowWhenUnset` in their entirety (they assert the version inside the `?` panel, which has moved to About). These are the only callers of `m.helpView()` and the only use of the `ansi` import in that file — **also remove `"github.com/charmbracelet/x/ansi"` from `tui/app_test.go`'s import block.**

- [ ] **Step 7: Run build + tests to verify green**

Run: `go build ./... && go test . -run TestVcsDate -count=1 && go test ./tui/ -run 'TestHelp' -count=1`
Expected: build OK; `TestVcsDate` PASS; remaining `TestHelp*` PASS (no version assertions left).

- [ ] **Step 8: Commit**

```bash
git add main.go main_test.go tui/run.go tui/app.go tui/app_test.go
git commit -m "refactor(tui): plumb structured version and drop it from the help panel"
```

---

## Task 4: Wire the about screen (open/close/render, bare landing)

**Files:**
- Modify: `tui/app.go`
- Test: `tui/app_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `tui/app_test.go`:

```go
func TestAboutOpensFromBlurredResult(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	step, _ := m.Update(fetchResultMsg{entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n"),
	}})
	m = step.(appModel)
	if m.inputFocused {
		t.Fatal("a landed result should be blurred")
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'a'})
	got := next.(appModel)
	if got.state != stateAbout {
		t.Fatalf("state = %d, want stateAbout", got.state)
	}
	if !strings.Contains(stripANSIForLandingTest(got.View().Content), "finger jonathan@tilde.team") {
		t.Fatalf("about view missing the author finger line:\n%s", got.View().Content)
	}
}

func TestAboutOpensFromHelpPanelOnLanding(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	step, _ := m.Update(tea.KeyPressMsg{Code: '?'}) // help opens even while focused
	m = step.(appModel)
	if !m.help {
		t.Fatal("'?' should open the help panel on the landing")
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'a'}) // 'a' from the open panel opens about
	got := next.(appModel)
	if got.help {
		t.Fatal("opening about should close the help panel")
	}
	if got.state != stateAbout {
		t.Fatalf("state = %d, want stateAbout", got.state)
	}
}

func TestLandingTypesAInsteadOfOpeningAbout(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	next, _ := m.Update(tea.KeyPressMsg{Code: 'a'}) // focused landing, help closed
	got := next.(appModel)
	if got.state == stateAbout {
		t.Fatal("'a' on the focused landing must type into the target, not open about")
	}
	if !strings.Contains(got.input.Value(), "a") {
		t.Fatalf("'a' should be typed into the target input, value = %q", got.input.Value())
	}
}

func TestAboutEscReturnsToOrigin(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	step, _ := m.Update(fetchResultMsg{entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n"),
	}})
	m = step.(appModel)
	opened, _ := m.Update(tea.KeyPressMsg{Code: 'a'})
	m = opened.(appModel)
	if m.state != stateAbout {
		t.Fatalf("precondition: state = %d, want stateAbout", m.state)
	}
	closed, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := closed.(appModel)
	if got.state != stateReader {
		t.Fatalf("esc from about: state = %d, want stateReader (origin)", got.state)
	}
	if got.pos != 0 || len(got.history) != 1 {
		t.Fatalf("esc from about must not change history: pos=%d len=%d", got.pos, len(got.history))
	}
}
```

Also **rewrite** the three wordmark-coupled tests for the bare landing. Replace `TestLaunchShowsHeaderMarkImmediatelyAboveTargetRow`, `TestFocusedInputChromeShowsHeaderMarkAboveTargetRow`, and `TestBackToLandingShowsNormalChromeNotSplash` with:

```go
func TestLaunchShowsBareTargetRowWithoutWordmark(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	view := stripANSIForLandingTest(m.View().Content)
	if strings.Contains(view, heroManicule+" "+heroWordmark) {
		t.Fatalf("landing should no longer show the wordmark (it moved to about):\n%s", view)
	}
	if !strings.Contains(view, "target:") {
		t.Fatalf("landing missing target row:\n%s", view)
	}
}

func TestFocusedInputChromeHasNoWordmark(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	m.input.SetValue("alice@plan.cat")
	(&m).submit()
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Login: alice\n"),
	}})
	m = step.(appModel)
	(&m).focusInput()
	view := stripANSIForLandingTest(m.View().Content)
	if strings.Contains(view, heroManicule+" "+heroWordmark) {
		t.Fatalf("re-focused input chrome should not show the wordmark:\n%s", view)
	}
	if !strings.Contains(view, "target:") {
		t.Fatalf("focused input chrome missing target row:\n%s", view)
	}
}

func TestBackToLandingShowsBareTargetRow(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	m.input.SetValue("alice@plan.cat")
	(&m).submit()
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Login: alice\n"),
	}})
	m = step.(appModel)
	(&m).back()
	if m.pos != -1 {
		t.Fatalf("want pos -1 after back-to-landing, got %d", m.pos)
	}
	view := stripANSIForLandingTest(m.View().Content)
	if strings.Contains(view, heroManicule+" "+heroWordmark) {
		t.Fatalf("back-to-landing should not show the wordmark:\n%s", view)
	}
	if !strings.Contains(view, "target:") {
		t.Fatalf("back-to-landing missing target row:\n%s", view)
	}
}
```

(Keep `TestFocusedInputHeaderKeepsTotalViewHeightStable` and `TestBlurredResultChromeDoesNotSpendHeaderRow` — they remain valid.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tui/ -run 'TestAbout|TestLandingTypesA|TestLaunchShowsBare|TestFocusedInputChromeHasNo|TestBackToLandingShowsBare' -count=1`
Expected: FAIL — `undefined: stateAbout` (compile error).

- [ ] **Step 3: Add `stateAbout`, the `about` field, and `aboutFromState`**

In `tui/app.go`, extend the state enum:

```go
const (
	stateReader appState = iota
	stateList
	stateAbout
)
```

Add to the `appModel` struct (near `reader`/`list`):

```go
	reader readerModel
	list   listModel
	about  aboutModel

	aboutFromState appState // state to restore when the about screen closes
```

- [ ] **Step 4: Construct and theme the about model in `newAppWithOptions`**

In the `appModel{...}` literal add `about: newAbout(profile, opts.Version, opts.BuiltAt),`. After `app.reader.setBackground(common.darkBackground)` / `app.reader.styles = st`, add:

```go
	app.about.setBackground(common.darkBackground)
```

- [ ] **Step 5: Add `openAbout`/`closeAbout`**

In `tui/app.go` (next to `focusInput`/`blurInput`):

```go
// openAbout switches to the full-screen about view, remembering the current
// state so closeAbout can restore it without a re-fetch. About is transient: it
// is not pushed onto history.
func (m *appModel) openAbout() {
	m.flash = ""
	m.aboutFromState = m.state
	m.state = stateAbout
	m.resizeForHelp()
}

// closeAbout returns from the about view to the screen it was opened from.
func (m *appModel) closeAbout() {
	m.state = m.aboutFromState
	m.resizeForHelp()
}
```

- [ ] **Step 6: Route keys for the about screen**

In `handleKey`, change the help-panel block so `a` opens about instead of closing:

```go
	// Help panel: any key closes it — except 'a', which opens the about screen.
	if m.help {
		if key.Matches(msg, m.keys.About) {
			m.help = false
			m.helpModel.ShowAll = false
			m.openAbout()
			return true, m, nil
		}
		m.help = false
		m.helpModel.ShowAll = false
		m.resizeForHelp()
		return true, m, nil
	}
```

Immediately **after** that block and **before** `if m.inputFocused {`, add the about-screen handler (so it runs regardless of input focus; full action set arrives in Task 5):

```go
	// About screen: its own keys, ahead of the input-focus branch.
	if m.state == stateAbout {
		switch {
		case key.Matches(msg, m.keys.About), key.Matches(msg, m.keys.Back):
			m.closeAbout()
			return true, m, nil
		}
		return true, m, nil // swallow other keys (actions land in Task 5)
	}
```

In the content-focused `switch` (the one starting `case key.Matches(msg, m.keys.Help):`), add an About case:

```go
	case key.Matches(msg, m.keys.About):
		m.openAbout()
		return true, m, nil
```

- [ ] **Step 7: Keep the About binding live in `updateKeymap`**

In `updateKeymap`, alongside `m.keys.Help.SetEnabled(true)`, add:

```go
	m.keys.About.SetEnabled(true)
```

At the end of `updateKeymap`, add the about override (its actions are live regardless of input focus; used by Task 5):

```go
	if m.state == stateAbout {
		// The about screen's own actions are live regardless of input focus.
		m.keys.Open.SetEnabled(true)
		m.keys.Copy.SetEnabled(true)
		m.keys.Back.SetEnabled(true)
		m.keys.Quit.SetEnabled(true)
	}
```

- [ ] **Step 8: Drop the wordmark chrome row and render the about state**

Replace `inputChromeView` and `topChromeHeight`:

```go
func (m appModel) topChromeHeight() int {
	return 1 // one target row; the wordmark now lives only on the about screen
}

func (m appModel) inputChromeView() string {
	return m.input.View()
}
```

In `resizeForHelp`, after sizing the list, also size the about model:

```go
	m.reader.setSize(m.common.width, h)
	if m.listReady {
		m.list.setSize(m.common.width, h)
	}
	ah := m.common.height - 1
	if ah < 1 {
		ah = 1
	}
	m.about.setSize(m.common.width, ah)
```

In `View()`, add an about branch at the top (after `(&m).updateKeymap()`):

```go
	if m.state == stateAbout {
		bottom := m.statusBarModel().render()
		v := tea.NewView(m.about.View() + "\n" + bottom)
		v.AltScreen = true
		return v
	}
```

- [ ] **Step 9: Add the about status bar and propagate theme/profile**

In `statusBarModel`, immediately after the `if m.loading { ... }` block and before `if m.pos < 0 {`, add:

```go
	if m.state == stateAbout {
		bar := statusBar{width: w, styles: st}
		if m.pos >= 0 {
			bar.escTarget = m.history[m.pos].entry.Target.Raw
		} else {
			bar.host = "about lookit"
		}
		parts := []string{"↵ go", "y copy"}
		if bar.escTarget == "" {
			parts = append(parts, "esc back")
		}
		parts = append(parts, "q quit")
		bar.hints = strings.Join(parts, " · ")
		if m.flash != "" {
			bar.hints = m.flash
		}
		return bar
	}
```

In `applyStyles`, add (so a background change restyles about):

```go
	m.about.setBackground(m.common.darkBackground)
```

In `Update`'s `case tea.ColorProfileMsg:`, add `m.about.setProfile(msg.Profile)` next to `m.reader.setProfile(msg.Profile)`.

- [ ] **Step 10: Run the tests to verify they pass**

Run: `go test ./tui/ -run 'TestAbout|TestLandingTypesA|TestLaunchShowsBare|TestFocusedInputChrome|TestBackToLanding|TestBlurredResultChrome|TestFocusedInputHeader' -count=1 -v`
Expected: PASS for all listed tests.

- [ ] **Step 11: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): wire the about screen and bare the landing"
```

---

## Task 5: About actions — finger the author, copy the issues URL

**Files:**
- Modify: `tui/app.go`
- Test: `tui/app_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `tui/app_test.go`:

```go
func TestAboutEnterFingersAuthor(t *testing.T) {
	fetch, seen := fetchRecorder("Plan: hi\n")
	m := newApp(fetch, colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	(&m).openAbout()
	if m.state != stateAbout {
		t.Fatalf("precondition: state = %d, want stateAbout", m.state)
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if !got.loading {
		t.Fatal("Enter on about should start a fetch (loading=true)")
	}
	if cmd == nil {
		t.Fatal("Enter on about should return a fetch command")
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != "jonathan@tilde.team" {
		t.Fatalf("fetched targets = %v, want [jonathan@tilde.team]", *seen)
	}
}

func TestAboutCopiesIssuesURL(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	(&m).openAbout()
	next, _ := m.Update(tea.KeyPressMsg{Code: 'y'})
	got := next.(appModel)
	if copied != aboutIssuesURL {
		t.Fatalf("copied = %q, want %q", copied, aboutIssuesURL)
	}
	if !strings.Contains(got.flash, "copied") {
		t.Fatalf("flash = %q, want it to mention the copied URL", got.flash)
	}
	if got.state != stateAbout {
		t.Fatalf("copy should keep the about screen open, state = %d", got.state)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tui/ -run 'TestAboutEnterFingersAuthor|TestAboutCopiesIssuesURL' -count=1`
Expected: FAIL — `TestAboutEnterFingersAuthor` (no fetch: `loading` false / `seen` empty) and `TestAboutCopiesIssuesURL` (`copied` empty), because the about branch currently swallows `↵`/`y`.

- [ ] **Step 3: Add the action cases to the about branch**

In `handleKey`, replace the `if m.state == stateAbout { ... }` block from Task 4 with the full action set:

```go
	// About screen: its own keys, ahead of the input-focus branch.
	if m.state == stateAbout {
		switch {
		case key.Matches(msg, m.keys.Open): // ↵ finger the author
			m.closeAbout()
			target, err := finger.ParseTarget(aboutFingerAuthor)
			if err != nil {
				return true, m, nil
			}
			return true, m, m.startFetch(target)
		case key.Matches(msg, m.keys.Copy): // y copy the issues URL
			m.flash = "copied " + aboutIssuesURL
			return true, m, tea.Batch(setClipboard(aboutIssuesURL), m.clearFlashCmd())
		case key.Matches(msg, m.keys.About), key.Matches(msg, m.keys.Back): // a / esc close
			m.closeAbout()
			return true, m, nil
		case key.Matches(msg, m.keys.Quit): // q quit
			return true, m, tea.Quit
		}
		return true, m, nil // swallow any other key on the about screen
	}
```

(`finger` is already imported in `tui/app.go`.)

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./tui/ -run 'TestAboutEnterFingersAuthor|TestAboutCopiesIssuesURL|TestAboutEscReturnsToOrigin' -count=1 -v`
Expected: PASS (and the esc-closes path still works).

- [ ] **Step 5: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): finger the author and copy issues from the about screen"
```

---

## Task 6: Full gate

- [ ] **Step 1: Run the full CI gate set**

Run: `make check`
Expected: PASS — `go vet`, `gofmt -l` empty, `golangci-lint run`, and `go test ./... -race` all green. If `gofmt` flags anything, run `make fmt` and re-run; re-commit any formatting fixup with `style(tui): gofmt`.

- [ ] **Step 2: Manual smoke (optional, needs a real TTY)**

`make build && ./lookit` — on the landing press `?` then `a` to open About; press `↵` to finger `jonathan@tilde.team`; from a result press `a`, then `y` (copy), then `esc` (back). The TUI cannot be smoke-tested headlessly.

---

## Notes for the implementer

- **Honesty invariant:** `a` must never be matched in the input-focused branch of `handleKey` — only in the help-open and content branches and the `stateAbout` handler. That is what keeps `a` typeable in a target like `alice@host` on the landing. `TestLandingTypesAInsteadOfOpeningAbout` guards this.
- **Why `About.SetEnabled(true)` always:** the binding must be matchable from the help panel even while the input is focused (the landing path `?`→`a`), exactly like `Help`. Focus safety comes from *where* the `case` lives, not from the enabled flag.
- **Pin/port safety:** the author finger goes through `finger.ParseTarget` + `startFetch` as an ordinary user-initiated lookup (port defaults to `:79`); it is not server-supplied, so `pinFingerPort` is not involved.
- **Kept assets:** `headerMark`, `heroManicule`, `heroWordmark` stay in `landing.go`; only their use in `inputChromeView` is removed. They are now consumed by `aboutView` and the landing tests.

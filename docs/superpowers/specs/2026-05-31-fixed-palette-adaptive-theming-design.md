# Fixed palette adaptive theming design

## Goal

Make lookit's colour a product strength without sacrificing terminal
legibility. This pass replaces the current hardcoded dark-only colours with one
fixed, Lip Gloss-inspired palette that adapts to light and dark backgrounds
across both user-facing paths:

- the interactive Bubble Tea TUI,
- the shared one-shot/reader renderer in `render/`.

The chosen visual direction is **Functional Bright**: hot pink, violet,
mint/teal, warm gold, red, and graphite neutrals, inspired by the Lip Gloss
demo palette. Colour should feel fun and distinctive, but each saturated colour
has a job. The selected list row uses the approved **B3 violet shelf**: a violet
rail/tint with pink login text, more visible than today and less saturated than
the Lip Gloss demo's full block treatment.

## Non-goals

- No user-selectable themes, config files, theme names, or runtime theme picker.
  This pass implements one fixed built-in palette only.
- No new product states for future subscriptions, discovery, or changed-content
  workflows. Those features do not exist yet.
- No `render/` migration to Lip Gloss v2. `render/` deliberately stays on
  `github.com/charmbracelet/lipgloss` v1; `tui/` stays on `charm.land/lipgloss/v2`.
- No mouse UI, layout redesign, or navigation changes.

## Palette model

The palette is semantic rather than component-local. The source values below
are the implementation target; if a contrast test proves one value is
insufficient, adjust the smallest number of values needed to satisfy the gate
without changing the approved Functional Bright direction.

| Role | Dark direction | Light direction | Use |
|---|---|---|---|
| `AccentPink` | hot pink | deeper berry-pink | field labels, selected login, identity accents |
| `AccentViolet` | bright violet | deeper violet | chrome badge, focused structure, selection rail |
| `AccentMint` | vivid mint/teal | deep teal | links, loading/spinner, positive/action affordances |
| `AccentGold` | soft warm yellow | deep ochre | warnings, truncation, partial flags |
| `AccentRed` | warm coral-red | deeper red | errors |
| `Text` | near-white lavender | graphite | main text where lookit owns foreground |
| `Dim` | muted lavender-gray | muted graphite | metadata, descriptions, help descriptions |
| `SubtleBg` | graphite-purple | pale lavender-gray | status/help/list selection backgrounds |
| `Rule` | dark separator | light separator | rules, help separators, status separators |

Implementation values derived from the approved mockups:

| Role | Dark | Light |
|---|---|---|
| `Text` | `#f0edf5` | `#25222a` |
| `Dim` | `#8c8792` | `#766f7d` |
| `AccentPink` | `#ff5fa2` | `#c92870` |
| `AccentViolet` | `#9878ff` | `#6d43d6` |
| `AccentMint` | `#38e7ad` | `#007f62` |
| `AccentGold` | `#eed76d` | `#765f00` |
| `AccentRed` | `#ff6f87` | `#c82f4d` |
| `BaseBg` | `#171719` | `#fbfafc` |
| `SubtleBg` | `#292631` | `#e9e4f0` |
| `SelectionBg` | `#342747` | `#f3e9f4` |
| `Rule` | `#35313d` | `#ded8e8` |

Light-mode colours are not simple inversions. They are separate, darker accent
values chosen for contrast on macOS light terminal backgrounds.

## TUI architecture

`tui/styles.go` becomes the central TUI style builder:

```go
type palette struct {
    Text, Dim, AccentPink, AccentViolet lipgloss.Color
    AccentMint, AccentGold, AccentRed   lipgloss.Color
    BaseBg, SubtleBg, SelectionBg, Rule lipgloss.Color
}

func paletteFor(isDark bool) palette
func newStyles(isDark bool) styles
```

`appModel` stores:

```go
darkBackground bool
styles         styles
```

`newApp` initializes with a conservative dark background assumption, matching
Lip Gloss and Bubble Tea defaults when the terminal does not answer background
queries. `Init` requests the background colour alongside the existing colour
capability requests:

```go
tea.RequestBackgroundColor()
tea.RequestCapability("RGB")
tea.RequestCapability("Tc")
```

`Update` handles `tea.BackgroundColorMsg`:

1. Set `darkBackground = msg.IsDark()`.
2. Rebuild `styles = newStyles(darkBackground)`.
3. Re-apply styles to owned components.
4. Re-render the current reader entry so embedded `render/` output follows the
   same background mode.

Components that must be styled from the central builder:

- bottom status bar (`statusBar.render` gets `m.styles` rather than calling
  `newStyles()` every frame),
- top text input prompt/cursor/focused styles,
- `bubbles/help` key/description/separator styles,
- loading spinner foreground,
- list default delegate normal/selected title/description/filter-match styles,
- list filter input styles if reachable through the list API,
- reader render path.

The list selection uses the approved B3 treatment:

- dark: violet rail and muted violet shelf background, pink selected login,
  light selected description;
- light: deeper violet rail and pale violet shelf, deeper pink selected login,
  graphite selected description.

`newList` must take styles from `commonModel` or `appModel`, not invent local
colour constants. Restoring history or reacting to background changes rebuilds
the list delegate from the same style builder.

## Render architecture

`render/` remains pure and on Lip Gloss v1. It gains a background-aware theme
constructor while preserving the simple public rendering path.

The API shape is:

```go
func Render(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile) string
func RenderWithBackground(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile, darkBackground bool) string
func NewTheme(profile colorprofile.Profile, darkBackground bool) Theme
```

`Render` keeps existing call sites working by using Lip Gloss v1's standalone
background detection once per call. The TUI uses `RenderWithBackground`, passing
`appModel.darkBackground`, so a live background change can re-render the current
entry deterministically.

`NewTheme` keeps the current `NoColor` behaviour for `Ascii` and `NoTTY`; those
profiles return unstyled text. For colour profiles, it converts the selected
semantic source colour through `colorprofile.Profile.Convert` just as today, so
ANSI/ANSI256 terminals still downsample through the existing mechanism.

## Accessibility and contrast

Accessibility is an acceptance criterion, not manual polish.

Add pure palette contrast tests using WCAG-style relative luminance:

- normal text foreground/background pairs must meet at least **4.5:1**;
- large/bold or non-text UI accents may use **3:1** only where the state is
  also communicated structurally, such as a selection rail, inverse status
  badge, or warning flag text that includes explicit copy;
- no state may rely on colour alone.

Test the source truecolour palette pairs directly. ANSI and ANSI256 downsampling
remain terminal-dependent, but the source palette must be defensible before
downsampling.

Pairs to gate include at minimum:

- text/dim/field/link/warning/error on each base background,
- status bar text and badges,
- help key, description, and separators on the help/status background,
- selected list login and description on selection background,
- spinner/loading text on the status background,
- render field labels, target, footer, warnings, and errors on both light and
  dark backgrounds.

## Behaviour

- Dark terminals get the Lip Gloss-inspired Functional Bright palette.
- Light terminals get paired colours tuned for contrast.
- The help popup is explicitly themed; it must be readable on macOS Terminal,
  iTerm2, and Ghostty in light and dark modes.
- The list selected row is more prominent via the violet shelf/rail treatment,
  without becoming a fully saturated block.
- The spinner is mint and updates with the active background mode.
- `NO_COLOR`, `Ascii`, and `NoTTY` output remain plain where they are plain
  today. Piped output stays grep-friendly.
- A terminal background change during a TUI session updates TUI chrome and
  re-renders the current reader body.

## Testing

Tests stay offline and no real TTY is required.

- Add `tui` palette contrast tests for all declared semantic pairs.
- Add `render` palette contrast tests for both light and dark theme builders.
- Add TUI model tests for `tea.BackgroundColorMsg`:
  - `darkBackground` changes,
  - `styles` are rebuilt,
  - `helpModel.Styles` changes,
  - spinner style changes,
  - current reader content is re-rendered.
- Add list style tests for selected title/description and normal description
  using the style builder rather than hardcoded local values.
- Add render tests for light and dark truecolour output. Existing `NoTTY`
  no-ANSI tests remain unchanged.
- `make check` remains the final gate.

## Accepted residual risk

Exact perceived colours after ANSI/ANSI256 downsampling can vary by terminal
theme. The implementation controls and tests truecolour source values, preserves
plain output for `NoTTY`/`Ascii`, and continues to let `colorprofile` handle
downsampling.

The first frame of the TUI may render with the conservative dark assumption
before the terminal responds to `RequestBackgroundColor`. Once the message
arrives, the app restyles and re-renders. This matches Bubble Tea's documented
background-colour workflow.

## Previously deferred work pulled into this pass

This spec intentionally resolves the current-app theming debt captured in prior
planning docs:

- adaptive light/dark colours for `tui/` and `render/`,
- selected list-row legibility,
- spinner foreground colour,
- help-panel contrast,
- the shared `render/` field-highlight palette,
- live `tea.BackgroundColorMsg` handling,
- automated contrast tests for the fixed palette.

The implementation plan must repeat this note so the deferred items stay
visible during execution.

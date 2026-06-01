# Landing hero design

## Goal

Replace the weakest frame in the app — the empty reader (`No response yet.`)
shown when `lookit` launches with no arguments — with a branded, centered
"hero" splash. This is the first thing a new user sees, and "looking cool,
well-designed, and Charm-native" is a deliberate USP for a new finger client.
The hero must make that first impression land without adding scope, network, or
config.

The visual direction is a **centered hero**: the `☞` manicule (a literal
pointing *finger* — the protocol's namesake) beside a `lookit` gradient
wordmark, a tagline, and the focused target input, all centered as one
composition. It reuses the fixed Functional Bright palette and adaptive
light/dark theming already shipped (see
`2026-05-31-fixed-palette-adaptive-theming-design.md`).

## Non-goals

- **No sample/catalog list.** The hero is a pure splash. The curated
  "try these hosts" discovery surface and the Phase 3 **Discover** tab remain
  deferred.
- **No `@localhost` probe or any network on launch.** The original deferred
  "probe localhost, fall back to catalog" idea is explicitly dropped. Launch
  stays instant and offline.
- **No persistent home/empty state.** The hero is launch-only (see Behaviour).
  It is not a recurring screen reachable by back-navigation.
- **No new theme tokens, theme picker, or config.** Colour comes entirely from
  the existing palette.
- **No layout/navigation redesign.** After the first lookup the app uses the
  existing chrome unchanged.

## Behaviour

- The hero renders only while `lookit` has launched with no arguments **and**
  no fetch has yet been dispatched.
- The composition, centered in the body region between the top of the screen
  and the status bar:
  - `☞ lookit` — manicule in `AccentPink`, wordmark in a per-rune gradient
    (see Visual treatment);
  - a tagline in `Dim`: *"a finger client for the modern terminal"*;
  - the focused target input, reusing the existing rotating placeholder
    (`pickSample()`);
  - the existing landing status bar at the bottom (`landingBar`), unchanged.
- **Hint honesty.** While the hero shows, the target input is **focused**, so
  every printable key — including `q` — is typed into the input (the keymap
  deliberately keeps `q` literal so users can type targets containing it). The
  status bar must therefore *not* advertise `q` as a quit key. The honest,
  already-shipped landing hint is reused verbatim: *"type a target and press ↵ ·
  ? help"*. The actual landing controls are: **Enter** looks up, **?** opens
  help, **Esc** quits (app.go's input-focused `Back` branch quits when
  `pos < 0`). No new keybindings and no hint copy change.
- **Transition (one-time):** the moment the first fetch is dispatched (the
  first `submit` that produces a fetch command), the hero is dismissed. The app
  collapses to the normal chrome (input pinned at the top, response in the
  body) with no layout shift thereafter. The hero never returns for the rest of
  the session — not on a completed fetch, not on error, not on full
  back-navigation.
- A terminal background change (`tea.BackgroundColorMsg`) while the hero is
  showing restyles it like the rest of the chrome, since it draws from the same
  `styles`.

## Visual treatment

- **Wordmark.** `lookit` rendered bold with a per-rune gradient sweeping the
  palette accents **pink → violet → mint**
  (`AccentPink #ff5fa2 → AccentViolet #9878ff → AccentMint #38e7ad` on dark;
  the paired light accents on light). `☞` is drawn in `AccentPink` to the left
  of the wordmark with a single space of separation.
- **Tagline.** `Dim` foreground, directly under the wordmark.
- **Background-aware.** The hero keys off the same `darkBackground` +
  `colorprofile.Profile` as the rest of the TUI, so light terminals get the
  light accents and the gradient stays legible.

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

## Architecture

A new file `tui/landing.go` holds the hero as **pure render functions** with no
fetch/network dependency:

- `heroView(st styles, profile colorprofile.Profile, width, height int, input string) string`
  — composes the centered block (wordmark + tagline + input) within the given
  body dimensions. **The hero is the sole owner of the input on the landing
  screen:** the caller passes the already-rendered input view as `input` and the
  hero centers it beneath the tagline. String in, string out, so it is
  golden-testable.
- `headerMark(st styles, profile colorprofile.Profile) string` — renders
  the `☞ lookit` wordmark, applying the gradient appropriate to the profile
  (see Degradation).

`appModel` (app.go) changes:

- Add `landing bool`, initialised `true` in `newApp`.
- In `submit`, set `landing = false` at the point a fetch command is produced
  (the one-time transition trigger). `landing` is a pure user-intent flag —
  no other site clears or sets it.
- `View()` branches **before** building the normal stack. The two branches are
  mutually exclusive about who renders the input:
  - **Landing branch (`m.landing && m.pos < 0`):** the compound guard is
    deliberate. `landing` alone would let the hero reappear for a fetch result
    delivered without a prior `submit` (the path several existing model tests
    take); pairing it with `pos < 0` ("nothing fetched yet") keeps the hero gone
    the instant anything lands, so `landing` can stay a pure user-intent flag
    cleared only in `submit`. This branch does **not** prepend `m.input.View()` as a
    top row. Instead it builds `bottom` (status bar, plus the help panel if
    open) and renders `heroView(styles, profile, width, height-Height(bottom),
    inputView) + "\n" + bottom`, where `inputView` is a width-bounded copy of
    the input (see Degradation → narrow terminals). The input appears **once**,
    centered inside the hero — never at the top.
  - **Normal branch:** the existing `input \n content \n bottom` stack,
    unchanged, where the input is the pinned top row.
- The status bar while landing is the existing `landingBar` path (`pos < 0`);
  `helpHeight`/`resizeForHelp` continue to work because the hero occupies
  the same body region a sub-model would.

The gradient helper lives alongside the palette (in `landing.go`, using the
`styles`/`palette` already built in `styles.go`). No change to `render/`,
`finger/`, `routeFetch`, `ParseUsers`, or the data model. The reader/list
sub-models are untouched.

## Degradation and edge cases

- **Gradient by profile.** TrueColor → full per-rune gradient. ANSI256 → the
  same gradient stops quantised through `colorprofile.Profile.Convert`, as the
  rest of the theme already downsamples. ANSI (16-colour) or any profile
  without usable gradient range → a solid `AccentViolet` bold wordmark (no
  gradient). The gradient is decorative and never the only signal; the wordmark
  is always legible.
- **`☞` glyph.** Kept as-is — a widely supported dingbat (U+261E). Its on-screen
  width is measured with `lipgloss.Width` so centering stays correct regardless
  of how a terminal widths it. No ASCII fallback (deliberate; revisit only if a
  target terminal proves to render it as tofu).
- **Narrow terminals.** `☞ lookit` is short and always fits. The tagline
  truncates with `…` (or is hidden) below roughly 40 columns. The block stays
  centered within the available width. Choosing the manicule over an ASCII
  banner avoids the wrapping/breakage a wide figlet banner would hit on narrow
  terminals.
- **Very short terminals.** When the body height is too small to center
  comfortably, the hero degrades gracefully (top-aligned within the body) rather
  than clipping the input.

## Testing

Offline, no real TTY, following the existing injected-fetch patterns.

- **Pure `heroView` goldens** across profiles (TrueColor / ANSI256 / ANSI) and
  both background modes, plus a narrow-width (`< 40` col) case asserting the
  tagline degrades and the wordmark/input remain.
- **Gradient helper** unit tests: TrueColor produces multiple distinct rune
  colours; ANSI falls back to a single solid colour; output contains the
  wordmark text and the `☞`.
- **`appModel` transitions:** `landing` is `true` after `newApp` and `View()`
  renders the hero; the first `submit` sets `landing = false`; a completed
  fetch, an errored fetch, and full back-navigation all leave `landing` false
  (the hero does not return).
- `make check` remains the final gate.

## Accepted residual risk

- Perceived gradient colours after ANSI256/ANSI downsampling vary by terminal
  theme; controlled by testing truecolour source stops and preserving a legible
  solid fallback, consistent with the existing theming pass.
- `☞` rendering width can vary by font; mitigated by measuring with
  `lipgloss.Width` rather than assuming a fixed column count.

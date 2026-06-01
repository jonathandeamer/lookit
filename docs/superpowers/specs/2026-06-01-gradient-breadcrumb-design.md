# Gradient breadcrumb design

## Goal

Make navigation feel designed by accenting the **leaf** of the status-bar
breadcrumb with the palette's pink→violet→mint gradient (the same sweep the now-
deferred splash used). This is feature #4 of the pre-MVP polish set. The
breadcrumb is the app's navigation signal — `@host` for a directory, `@host /
user` for a profile — and today the leaf is a flat bold token. Accenting the
leaf draws the eye to "where am I" without recolouring the quiet host metadata.

The chosen treatment is **leaf accent** (option B from brainstorming), not a
full-path sweep (too loud for status-bar metadata) and not identity-derived
colour (rejected: the colour would convey information the user already has from
the text, and it leans into asserting structure the data lacks).

## Non-goals

- **No change to the host or separator** — they stay dim (`barHost`/`barSep`).
- **No identity/hash-derived colour**, no full-path sweep, no transient
  change-highlight animation.
- **No change to the landing hero / splash.** It is being removed separately;
  this pass leaves `heroView`, `heroInputWidth`, and the `landing` wiring alone.
- No new dependencies.

## Helper relocation (so #4 doesn't depend on the doomed splash)

The gradient maths currently lives in `tui/landing.go` (`lerpColor`,
`wordmarkColors`), which will be deleted when the splash is removed. To keep the
breadcrumb independent of that, move the **generic** helpers into a new
`tui/gradient.go`:

- `lerpColor(a, b color.Color, t float64) color.Color` — moved verbatim.
- `gradientColors(p palette, n int) []color.Color` — **moved and renamed** from
  `wordmarkColors` (its behaviour is generic — n colours swept across
  AccentPink→AccentViolet→AccentMint — so the wordmark-specific name no longer
  fits now that the breadcrumb also uses it).
- `gradientString(base lipgloss.Style, p palette, s string) string` — **new**:
  renders `s` rune-by-rune, each rune in `base` with its foreground replaced by
  the gradient colour for that position (`gradientColors(p, len([]rune(s)))`).
  This is the reusable primitive the breadcrumb leaf needs (it preserves
  `base`'s background and bold, swapping only the foreground per rune).

`gradientWordmark` stays in `landing.go` (it is splash-specific and will be
removed with the hero); its one call to `wordmarkColors` is updated to
`gradientColors`. Its tests stay with it. The `lerpColor`/`gradientColors` tests
move to `tui/gradient_test.go`; a `gradientString` test is added there.

This is a pure move + rename (no behaviour change) plus one new helper.

## Breadcrumb change

In `statusBar.styleCrumb` (tui/statusbar.go), only the profile-that-fits branch
changes — the leaf is rendered via `gradientString` instead of `barUser`:

```go
// before:
return st.barHost.Render(b.host) + st.barSep.Render(" / ") + st.barUser.Render(b.user)
// after:
return st.barHost.Render(b.host) + st.barSep.Render(" / ") + gradientString(st.barUser, st.palette, b.user)
```

`st.barUser` already carries the status-bar background and bold; `gradientString`
keeps those and only varies the per-rune foreground. Behaviour in the other
branches is unchanged:

- **Directory (`b.user == ""`):** fully dim host, no leaf, no accent. (A nice
  honest side-signal: a profile gets the accented leaf, a directory stays
  quiet.)
- **Over budget:** unchanged — collapses to the single dim truncated string
  (the gradient is decorative and degrades to today's behaviour). Width maths is
  unaffected because it is computed on the plain `full` string and the gradient
  is only applied in the fits branch.

## Honest accessibility

The leaf sweeps the three accents on the status-bar `SubtleBg`. Measured
contrast (WCAG) of the stops on `SubtleBg`:

| stop | dark (`#292631`) | light (`#e9e4f0`) |
|---|---|---|
| AccentPink | 5.23 | 4.18 |
| AccentViolet | 4.57 | 4.91 |
| AccentMint | 9.32 | 3.99 |

Dark mode (lookit's default assumption) clears 4.5 on all three. On the **light**
palette, pink (4.18) and mint (3.99) sit between **3:1 and 4.5:1**.

The correct gate here is **3:1**, not 4.5, and this is a deliberate,
policy-consistent choice — not lowering the bar to pass:

- The leaf is **bold**, and the gradient is **decorative**: the username is
  fully conveyed by the leaf's own glyphs (we explicitly rejected
  identity-by-colour), so the colour carries no information that legibility
  depends on. This is exactly the theming spec's 3:1 tier — "large/bold or
  non-text UI accents where the state is also communicated structurally" (that
  spec already gates the selection rail at 3:1).
- Reaching 4.5 on light would require darkening the shipped `AccentPink`/
  `AccentMint`, which are reused across the theme and hardcoded in existing
  tests (e.g. the `38;2;201;40;112` light-field assertion). That is a
  theme-wide change well outside a breadcrumb polish.

Extend the palette contrast test (`TestTUIPaletteContrast` in
`tui/styles_test.go`) to gate **`AccentPink` and `AccentMint` on `SubtleBg` at
≥3.0**, in both light and dark (the test already runs both). `AccentViolet` on
`SubtleBg` keeps its existing ≥4.5 help-key gate. With all three stops gated on
`SubtleBg`, the gradient's legibility on the bar is verified, not eyeballed.

## Testing

Offline tests, no real TTY:

- `tui/gradient_test.go`: the moved `lerpColor` and `gradientColors` tests
  (renamed), plus `gradientString` — for a multi-rune input on a colour profile
  it produces ≥2 distinct foreground colours and, ANSI-stripped, equals the
  input; on a no-colour style it returns the text plainly.
- `tui/statusbar_test.go` (or `app_test.go`, wherever crumb rendering is
  exercised): `styleCrumb` with a multi-rune user that fits → the leaf region
  carries ≥2 distinct gradient foreground colours and the host stays dim
  (`BarText`); over-budget → collapses to the dim truncated string with no
  accent; directory (no user) → host dim, no accent.
- Extended `TestTUIPaletteContrast`: pink and mint on `SubtleBg` at ≥3.0, light
  + dark (violet keeps its existing ≥4.5 gate). The 3.0 threshold is the chosen,
  palette-satisfiable criterion (see Honest accessibility); using 4.5 here would
  fail on the current light palette.
- `gradientWordmark` tests still pass after the rename; `make check` is the
  final gate.

## Accepted residual risk

On the **light** palette the pink and mint leaf stops sit at ~4:1 on `SubtleBg`
— above the 3:1 decorative/bold tier but below 4.5:1 AA-normal. This is accepted
because the leaf is bold and the colour is decorative (the username is legible
from its glyphs regardless of hue), and dark mode (the default) is ≥4.5. The
alternative — darkening the shipped light accents — was rejected as
disproportionate (theme-wide ripple, hardcoded-colour test churn).

The contrast test gates the three gradient **stops** on `SubtleBg`; the
interpolated mid-points between them are not individually gated. This matches the
existing theming approach (test the source palette, accept terminal-dependent
downsampling): the interpolations lie between bright stops on the dark bar and
dark stops on the light bar, so they stay within a legible band. Short leaves
(e.g. a 3-character login) sample only a slice of the gradient and read as a
single accent that shifts subtly — intended, not a defect.

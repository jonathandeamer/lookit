# Flash consistency design

## Goal

Tighten lookit's existing transient-feedback ("flash") mechanism so the copy
confirmation behaves consistently. This is feature #3 of the pre-MVP polish
set, reframed: a copy-confirmation flash **already exists** (`copyAddress` sets
`m.flash = "copied " + addr` with a 2-second auto-clear, rendered in the status
bar), so the work is not to add a toast but to fix two consistency gaps around
it.

## Background (current behaviour)

- `appModel.flash string` is rendered in the status bar's hints region; copy
  sets it and schedules `clearFlashCmd()` (a 2s `tea.Tick` → `clearFlashMsg` →
  `m.flash = ""`).
- `submit` reuses `flash` for parse errors (`"error: " + err`) and clears it at
  its own start; error flashes otherwise persist until the next submit.
- `copyAddress` returns `nil` (no feedback) when no address is available.

## Non-goals

- **No restyling** of the flash / copy confirmation. It stays in the status bar
  in its current style. No new toast UI element, no overlay, no colour/glyph
  change.
- **No change to error-flash persistence.** Parse errors still persist until the
  next submit (they should: the user needs to read and fix their input). This
  pass does not auto-clear errors.
- No new flashes for actions that already have their own feedback (drill shows
  the loading bar, back changes the screen, `r` swaps the view, `/` shows filter
  state, scrolling moves the viewport). Copy is the only invisible-side-effect
  action, so it is the only one that warrants a flash.

## Changes

Two small behavioural changes in `tui/app.go`.

### 1. "nothing to copy" feedback

`copyAddress` currently returns `nil` when `addr == ""` (landing screen, or a
list with nothing selected/extractable) — pressing `y` does nothing, silently.
Change that branch to give feedback:

- set `m.flash = "nothing to copy"`,
- return `m.clearFlashCmd()` (so it auto-clears after 2s, like the success
  case).

The success path (`m.flash = "copied " + addr`; `tea.Batch(setClipboard(addr),
m.clearFlashCmd())`) is unchanged.

### 2. Clear stale flash on navigation

A flash currently survives a screen change until its 2s timer fires, so a
"copied …" message can linger on a screen that didn't produce it. Clear
`m.flash` at the two **screen-changing** navigation entry points, so transient
feedback stays tied to the screen that produced it:

- `back()` — Esc / back navigation,
- `drill()` — Enter into a list user (starts a new fetch).

(`drill` has a value receiver returning the model, so the clear is applied to
the returned `appModel`; `back` has a pointer receiver.)

**`focusInput()` is deliberately excluded** — and this is the important
constraint. `focusInput()` lies directly on the parse-error recovery path: a
failed submit leaves the input focused with the error flash set; the user may
Esc to blur (`blurInput()`, which does not clear the flash) and then press `i`
(`focusInput()`) to correct. Clearing the flash in `focusInput()` would wipe the
error before the user resubmits, contradicting the "errors persist until the
next submit" non-goal. `focusInput()` is only a same-screen focus change, not a
screen change, and any lingering copy flash there self-clears via its existing
2s timer — so excluding it costs nothing and protects error persistence.

Note that `back()` is safe with respect to that recovery path: when the input
is focused, Esc is handled by `blurInput()`, **not** `back()`, so `back()` is
never invoked between a failed submit and its correction.

Raw-view toggling (`r`) is also excluded: it is a view change on the same
history node, not navigation, and is not a source of cross-screen flash bleed
worth special-casing.

## Testing

Offline model tests in `tui/app_test.go`, following existing patterns (stubbed
fetch, no real TTY):

- **"nothing to copy":** with no address available (landing: `pos == -1`, not in
  a list), `copyAddress` sets `m.flash == "nothing to copy"` and returns a
  non-nil command. Add the analogous list-with-no-selection case if reachable.
- **success unchanged:** with a fetched profile at `pos >= 0`, `copyAddress`
  still sets `m.flash == "copied " + target.Raw`.
- **stale-flash clearing:** after setting `m.flash = "copied x"`, each of
  `back()` and `drill()` leaves `m.flash == ""`.
- **error survives refocus (regression guard):** after setting
  `m.flash = "error: bad target"`, calling `focusInput()` leaves the flash
  **unchanged** (`m.flash == "error: bad target"`) — locking in that the parse
  error is not wiped on the recovery path.
- Existing copy/flash/error and navigation tests stay green.
- `make check` is the final gate.

## Accepted residual risk

The flash still shares one status-bar field with parse errors, so a success
flash and an error are visually identical (both plain hint text). Distinguishing
them is a restyle, explicitly out of scope here; the lifecycle fixes above do
not depend on it.

# Charm-idiomatic blur-by-default navigation design

## Goal

Make lookit's TUI keyboard model match the Charm ecosystem (and, where Charm is
silent, the vim flavour Charm's own components use; only then smallnet
browsers). The just-merged navigation feature kept the target input
**permanently focused**, which forced a non-idiomatic `Alt+←/→` history chord
that (a) needs the macOS "Option as Meta" setting and (b) collides with
`bubbles/textinput`'s own word-motion keys. This spec replaces that with a
**blur-by-default** model: content owns the keyboard, the input is focused on
demand, and every binding is inherited from `bubbles` or its vim idiom.

It also folds in four idiomatic upgrades surfaced by a UI audit against glow:
a `key.Binding` keymap, a `bubbles/help` bottom panel (replacing the current
full-screen help takeover), a `bubbles/spinner` loading indicator, scroll
percentage in the status bar, and dropping the unused mouse capture that
currently breaks native macOS text-selection.

## Decision priority (the rule that produced this design)

For every key, prefer in order: **(1) what `bubbles`/Charm binds by default →
(2) the vim flavour Charm's components already use → (3) smallnet-browser
convention.** Worked example outcomes: scrolling/paging keeps the bubbles
defaults; `i` focuses the input (vim insert; Charm has no address-field idiom);
back is `Esc` (bubbles `list` backs out on esc); **forward is dropped** (bubbles
has no history/forward concept, so tier 1 yields nothing and we stop — we do not
reach smallnet's `b`/`f`, which are anyway taken by paging).

## Non-goals (deferred to their own specs)

- **Adaptive light/dark colours.** Both `tui/styles.go` and `render/theme.go`
  hardcode dark-mode hex; glow uses `lipgloss.AdaptiveColor{Light,Dark}`
  throughout. This is orthogonal theming work that also touches `render/` (the
  shared CLI path, deliberately v1 lipgloss) — its own follow-up spec.
- **A `--mouse` opt-in flag / runtime mouse toggle.** This spec only *removes*
  the current always-on capture. Re-introducing mouse as glow does (behind a
  flag) is a separate, optional follow-up.
- **Navigable links inside profile bodies.** Still the previously-planned
  follow-up; unaffected here.
- No change to `finger/`, `render/`, `ParseUsers`, the history-stack mechanics,
  the drill + port-79 pinning, or the honesty flags.

## Focus model

Two focus targets, tracked by a new `appModel.inputFocused bool`, orthogonal to
the existing `state` (`stateReader`/`stateList`, which still selects the active
*content* sub-model):

- **Content focused** (default): the active sub-model (reader viewport or list)
  owns the keyboard; single-key commands work.
- **Input focused**: the top target input owns the keyboard for typing.

The **landing** screen (nothing fetched, `pos == -1`) starts with the input
focused, so a fresh launch lets you type immediately. After a fetch lands,
focus moves to content.

## Input becomes top chrome

The target input **moves out of `readerModel` into `appModel`** as a persistent
top bar — shared chrome, mirroring the bottom status bar, reachable from both
the list and the reader. Consequences:

- `readerModel` shrinks to a viewport plus its rendered entry (it no longer owns
  the input, the `status`/`loading` fields, or the Enter→fetch logic).
- **All fetch initiation centralizes in `appModel`** (both typing a target and
  drilling a list entry), which is natural since `appModel` already routes
  results via `routeFetch`.
- `appModel` gains `input textinput.Model`. It is created blurred; `Focus()`
  on the landing and whenever the user presses the focus key, `Blur()` on
  submit or cancel.

`View` composes, top to bottom: **top input row** → optional expanded help block
→ active content → **bottom status bar**. Sizing therefore reserves the input
row (always) and the help block (only while expanded), in addition to the
existing one bottom-bar row.

## Keymap

Keys are defined as a `key.Binding` `keyMap` (idiomatic Charm; also drives
`bubbles/help`) and matched with `key.Matches`, replacing the current raw
`key.Code ==` comparisons. The content scroll/paging keys are *not* re-declared
— they remain owned by the `bubbles` viewport/list and are reached by delegating
unhandled keys to the active sub-model; the keyMap carries synthetic bindings
for them only so `help` can display them.

**Content focused** (bubbles defaults inherited verbatim, plus four app keys):

| Action | Key(s) | Source |
|---|---|---|
| Scroll / page / jump | `↑↓`/`j k`, `←→`/`h l`, `f`/`b`/`space`/PgDn/PgUp, `u`/`d`, `g`/`G` | bubbles viewport/list defaults — unchanged |
| Filter a list | `/` | bubbles list default (list owns it) |
| Open / drill | `Enter` | existing |
| Raw view (auto-detected list) | `r` | existing lookit affordance |
| **Focus input** | **`i`** | vim insert |
| **Back** (history) | **`Esc`** | bubbles list backs out on esc |
| Help | `?` | bubbles default |
| Quit | `q`, `Ctrl+C` | bubbles default |

No forward. No `Alt` bindings anywhere.

**Input focused** (textinput defaults, untouched):

| Action | Key(s) |
|---|---|
| Type / edit (incl. `Option+←/→`, `Ctrl+a/e/w/…` word & line motion) | textinput defaults — *restored* now that we don't intercept them |
| Submit → fetch | `Enter` |
| Cancel → content (or **quit** at the bare landing) | `Esc` |
| Quit | `Ctrl+C` |

While the input is focused, `?`/`q`/`i` are literal characters (typed), exactly
as `bubbles/list` treats keys while its filter is being typed. The existing
"list is filtering" guard is preserved: while the list filter is being typed,
app-level keys defer to the list.

## Transitions

- **Landing** → input focused; `Esc` (or `Ctrl+C`) quits; typing + `Enter`
  fetches.
- **`i`** (content focused) → focus input, pre-filled with the current target's
  raw string so you can edit the current address browser-style (overtype to
  replace). `Focus()` returns the textinput blink cmd.
- **`Enter`** (input focused) → `finger.ParseTarget(value)`; on error, show the
  error in the bottom bar and keep the input focused; on success, start the
  loading spinner, `Blur()` to content, and issue the fetch. The result routes
  through `routeFetch` as today (pushing a history node).
- **`Esc`** (input focused) → if anything has been fetched (`pos >= 0`), blur to
  content without navigating; at the bare landing, quit.
- **`Esc`** (content focused) → history back (`stepBack`), unchanged underneath.
- **History:** the `forward()` method and the two `Alt` key cases are
  **removed**; `pos`/`stepBack`/`push`/`restore`/`snapshot` are otherwise
  unchanged (the slice may briefly retain an unreachable forward tail, harmless
  and truncated on the next navigation).

## Loading indicator

`appModel` owns a `spinner.Model` (`bubbles/spinner`) and a `loading bool` plus
the in-flight target. On submit/drill, `loading = true` and the spinner ticks
(its `TickMsg` is handled in `Update`); on `fetchResultMsg`, `loading = false`.
While loading, the bottom bar shows the spinner and `loading <target>` in place
of the normal meta/hints. This replaces the plain `"loading …"` status string.

## Help: glow-style bottom panel

The current **full-screen** help view is removed. To avoid two components
owning the same line, ownership is split cleanly:

- **Short help (collapsed, default):** the bottom **status bar's hint slot**
  carries the compact, focus-aware hints (e.g. `i target · esc back · ? help`).
  This is the existing `statusBar` hint string — `bubbles/help` is *not* used
  for the one-line short form.
- **Full help (expanded):** `appModel` owns a `help.Model` (`bubbles/help`)
  whose `FullHelp()` is fed by the `keyMap`. The keyMap's `FullHelp()` **must
  include the list's page/move keys** (`←/→` page, `j/k` move, `g/G`
  top/bottom) — re-surfacing the discoverability that disabling the list's
  built-in help would otherwise remove. `?` sets `help.ShowAll = true`;
  `help.View(keyMap)` then renders the full multi-column keymap as a block
  **above the bottom bar**, and the content viewport/list **shrinks by that
  block's height** (never a full-screen takeover, matching glow). When
  collapsed, the help model renders nothing. Any key / `?` / `Esc` collapses it.

While the list filter is being typed, `?` is literal and the panel does not
open (same guard as before).

## Status bar updates

The existing pure `statusBar` renderer gains a **scroll-percentage** segment
(glow-style): when content is the reader viewport and it is scrollable,
`statusBarModel` fills it from `viewport.ScrollPercent()` (e.g. `42%`). It also
gains a **page-indicator** segment: when content is a list spanning more than
one page, `statusBarModel` fills it from the list's paginator
(`list.Paginator.Page`/`.TotalPages`) as `page 2/4`. Hints become focus-aware
(content vs input vs loading). The breadcrumb, `◂ esc:` target, byte/`N users`
meta, and honesty flags are otherwise unchanged.

## Mouse

`appModel.View` **stops setting `tea.MouseModeCellMotion`** (no mouse mode at
all). This restores native macOS drag-to-select/copy — important for a tool
whose content is text people quote (emails, `finger://` links) — and matches
glow, which leaves mouse off unless explicitly enabled. Keyboard scrolling
(`j/k`, arrows, `PgUp/Dn`, `u/d`, `g/G`) is the scroll path.

## Architecture summary

```
appModel (app.go)
├── input textinput.Model        ← NEW: moved from readerModel (top chrome)
├── inputFocused bool             ← NEW
├── spinner spinner.Model         ← NEW (bubbles/spinner)
├── help help.Model               ← NEW (bubbles/help)
├── keys keyMap                   ← NEW (key.Binding; key.Matches; help source)
├── history/pos/state/showingRaw/list  (unchanged; forward() removed)
├── Update: focus routing; submit→fetch; spinner tick; help toggle; Esc/i/q
├── View: input row + [help block] + content + status bar; NO mouse mode
│
├── readerModel (reader.go)       ← shrinks to viewport + entry (input removed)
└── statusBar (statusbar.go)      ← gains scroll-% + focus-aware/loading hints
```

`finger/` → `render/` → `tui/` dependency direction unchanged. New imports:
`charm.land/bubbles/v2/help`, `charm.land/bubbles/v2/spinner`,
`charm.land/bubbles/v2/key` (all within the existing bubbles module).

## Testing

Consistent with the project's injected-fakes, offline, no-TTY discipline:

- **Focus routing (unit, `appModel`):** `i` focuses the input (and content keys
  then type into it); `Esc`/`q`/`Ctrl+C` from content; `Enter` while
  input-focused submits and blurs to content (assert a fetch `Cmd` for the
  parsed target via stub fetch, and `inputFocused == false` after); `Esc` while
  input-focused blurs to content when `pos >= 0` and quits at the landing; `?`
  and `q` are literal characters while the input is focused; the list-filtering
  guard still defers keys to the list.
- **Keymap via `key.Matches`:** back/open/raw/help/quit resolve through the
  `keyMap`; scroll keys reach the active sub-model only when content-focused.
- **Removal:** `forward` and any `Alt` binding are gone (`grep` guard in the
  plan); `Alt+←/→` no longer navigate.
- **Help panel:** `?` toggles `help.ShowAll`; the rendered `View().Content`
  contains the full keymap when open and is **not** a full-screen replacement
  (the content is still present, just shorter); height shrinks by the panel.
  The full help includes the page/move keys (`←/→`, `j/k`, `g/G`).
- **Spinner:** `loading` is set on submit/drill and cleared on
  `fetchResultMsg`; the bar shows the loading target while loading.
- **Status bar:** scroll-% appears for a scrollable reader; `page 2/4` appears
  for a multi-page list and is absent for a single-page list; render stays pure
  and width-clamped (existing `statusBar` tests extended).
- **Mouse:** `View()` sets no mouse mode (assert the `tea.View` has the default
  `MouseMode`).
- **Regression:** history-stack, drill + port-79 pinning, honesty-flag, and
  `ParseUsers` golden tests stay green.

## Accepted residual scope

- Re-focusing the input pre-fills the current target and leaves the cursor at
  the end (overtype/clear to replace); no select-all. Acceptable for v1.
- The history slice may hold an unreachable forward tail after a back (no
  forward to reach it); it is truncated on the next navigation. Harmless.
- Dropping mouse capture means no wheel-scroll until the deferred `--mouse`
  flag lands; keyboard scrolling covers it.

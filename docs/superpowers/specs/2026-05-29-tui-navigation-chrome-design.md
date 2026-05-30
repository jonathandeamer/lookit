# TUI navigation & chrome design

## Goal

Give a single lookit TUI session a **sense of place and a way back**. Today the
TUI shows no persistent indication of where you are, and "back" is an ad-hoc
two-level affair (`state` + `fromList`) that only knows about the
list→user drill. This spec adds:

1. A **session history stack** with browser-style back/forward across every hop.
2. A **persistent bottom status bar** showing the current location as a
   breadcrumb, where "back" lands, response metadata, and contextual key hints.
3. **Honest copy** for the heuristic flags (`auto-detected`, `partial`).

The chosen navigation model is the **hybrid** of the two we considered: the
chrome displays only the current, structurally-shallow location (`@host` or
`@host / user`), while navigation runs over a full back/forward history
underneath. The breadcrumb tells you *where you are*; the history (surfaced via
the `◂ esc:` hint) tells you *where back goes*.

## Non-goals

- **No navigable links inside profile bodies.** Detecting and following
  `user@host` / `finger://` / hostnames *within a rendered body* is the explicit
  follow-up spec. This spec only displays and navigates hops that already exist
  (typing a target, drilling a list entry, server-supplied `finger://` links).
- **No mouse-clickable chrome or links.** Zone-marked clickable breadcrumbs are
  a possible later addition; navigation here is keyboard-only.
- **No tabs / multiple sessions.** Single session only. The design leaves the
  top row free so tabs could be added later, but they are out of scope.
- **No change to `finger/` or `render/`.** Networking, the 1 MiB cap, CRLF
  normalization, and the single `Render` path are untouched. All work is in
  `tui/`.
- **No new parser behavior.** `ParseUsers` and its golden corpus are unchanged.

## Navigation: a session history stack

### The history model

`appModel` gains a back/forward history:

```go
type histNode struct {
    entry    Entry      // target/body/meta/err — the fetched result
    state    appState   // stateReader | stateList (how it was shown)
    // light restorable view state so back/forward feel instant & faithful:
    scrollY  int        // reader viewport offset (0 for list nodes)
    listIdx  int        // list selected index (0 for reader nodes)
    listFltr string     // applied list filter, if any
}

type appModel struct {
    // ...existing fields minus fromList/hostList (see "Subsumes")...
    history []histNode
    pos     int          // cursor; -1 when empty, else index into history
}
```

A `histNode` is a snapshot of a **landed screen**. Because the body is cached in
`entry`, back/forward **never re-finger the network** — they restore.

### Building & moving the stack

- **Push.** Every fetch that lands a *new* screen pushes a node (a manual fetch
  from the input, a list drill). Before pushing, the **forward tail is
  truncated** (`history = history[:pos+1]`), matching browser behavior:
  navigating somewhere new discards any forward history. Then append and set
  `pos = len(history)-1`.
- **Back.** `Esc` or `Alt+←`: if `pos > 0`, snapshot the *current* live view
  state into `history[pos]` (scroll/selection may have changed since landing),
  decrement `pos`, and restore `history[pos]`.
- **Forward.** `Alt+→`: if `pos < len(history)-1`, increment `pos` and restore.
- **Restore** rebuilds the active sub-model from the node's `entry` (reader via
  `setEntry`, or list via `newListWithPreamble`) and re-applies `scrollY` /
  `listIdx` / `listFltr`, then sets `m.state = node.state`.
- `Ctrl+C` always quits. `Backspace` is **never** a navigation key — it always
  edits the target input (this is why back is `Esc`/`Alt+←`, not Backspace; see
  "Keymap" for the rationale).

### Subsumes `fromList` / `hostList`

The history stack **replaces** today's `fromList bool` and `hostList *Entry`.
The cached host listing is simply "the previous node," restored instantly on
back. `routeFetch` stops setting `fromList`/`hostList` and instead pushes a
node; `handleKey`'s reader/list Esc branches delegate to the back operation.
The `r` "view raw" affordance on a generic list is retained (it swaps the
reader content in place for the cached body) but does **not** push a node — it
is an in-place toggle, and Esc/back returns to the list as before.

### Keymap

| Action | Keys | Notes |
|---|---|---|
| Back | `Esc`, `Alt+←` | `Esc` steps back one history entry everywhere; from `pos == 0` it returns to the landing (`pos == -1`), and `Esc` at the landing quits. |
| Forward | `Alt+→` | No-op at the head of the stack. |
| Quit | `Ctrl+C` | Always. |
| Edit target | `Backspace` et al. | Reader input is always focused; Backspace must stay an editor key, so it is **not** overloaded for navigation. |
| Drill / open | `Enter` | Unchanged. |
| Filter list | `/` | Unchanged. |
| View raw (generic list) | `r` | Unchanged; in-place, no history push. |
| Toggle help | `?` | New; see status bar. |

While the list filter is actively being typed, the list owns `Esc`/`Enter` as
today; navigation keys defer to it.

## Status bar & layout

### A shared bottom bar

A new `tui/statusbar.go` renders **one inverse-video line** from a plain struct
of parts — it is a pure function of its inputs (no Bubble Tea model), mirroring
the `render.Render` philosophy and keeping it trivially testable:

```go
type statusBar struct {
    host     string   // e.g. "@tilde.team"  (always present once something is loaded)
    user     string   // e.g. "jonathan"     ("" for a directory/list)
    escTarget string  // previous node's target.Raw ("" when pos == 0)
    flags    []string // e.g. {"auto-detected"} or {"partial (truncated)"}
    meta     string   // e.g. "1.2 KB", "3 users"
    hints    string   // contextual: "↵ open · / filter · ? help" or "? help"
    width    int
    styles   styles
}
func (b statusBar) render() string
```

Layout, left→right: the **breadcrumb** (`host` dim + ` / ` + `user` bold; the
`user` half omitted for directories), then any **flags**, then right-aligned
context: `◂ esc: <escTarget>`, `meta`, `hints`. The breadcrumb is the first
thing truncated (with `…`) when width is tight; `◂ esc:` truncates its target
next. There is **no KIND tag** — the breadcrumb's *shape* (`@host` vs
`@host / user`) conveys directory-vs-profile honestly, derived from the real
target rather than asserting a type the finger protocol lacks.

The **landing state** (nothing fetched) shows only ` type a target and press ↵ ·
? help`.

### Composition & consolidated chrome

`appModel.View` composes the screen: `activeSubModelContent + "\n" + bar`. This
moves ownership of the bar to `appModel` (which already owns cross-screen
state), so the sub-models shed their scattered chrome:

- **`readerModel`** drops its own `status` line and `hint` line. It keeps the
  top `lookit` title and the `target:` input. `chromeRows` is recomputed
  accordingly; the bar's single line is reserved by `appModel` when sizing.
- **`listModel`** drops its `list.Title` and the bubbles built-in help
  (`SetShowHelp(false)`). The "host — N users" text and `(best guess)` /
  `(incomplete)` flags it carried move into the bar (as `host`, `meta`,
  `flags`). The preamble is unchanged.

Sizing math (`setSize` in both sub-models and the `WindowSizeMsg` path in
`appModel`) is adjusted so the viewport/list reserve exactly one row for the
bar. A thin separator rule above the bar (matching the existing reader divider
style) is optional polish, counted in the row math if kept.

### Help overlay

`?` toggles a help overlay (glow-style) listing the keymap, so the bar itself
never needs a second line. The overlay is a simple full-content replacement (or
centered box) rendered by `appModel`; any key dismisses it. While the list
filter is being typed, `?` is a literal character and the overlay does not open.

## Copy: honesty flags

Two heuristic flags get clearer, consequence-first copy, **tiered by space** —
a terse form in the one-line bar and a full sentence in the roomy preamble note:

| Cause | Bar flag | Preamble note |
|---|---|---|
| unrecognized format (generic fallback) | `auto-detected` | `Auto-detected user list from an unrecognized response — press r to view raw` |
| body cut off mid-line | `partial (truncated)` | `Partial user list — truncated response` |
| fetch errored with a parseable body | `partial (error)` | `Partial user list — fetch error` |

This **splits today's single `(incomplete)` flag by cause** (`Entry.Err != nil`
→ `error`; `Entry.Meta.Truncated` → `truncated`), since labeling an errored
response "truncated" would be inaccurate. `routeFetch` already distinguishes the
two conditions when it computes `incomplete`; that branch now yields the
specific flag instead of a single boolean. The existing generic-list preamble
note (currently "Auto-detected from an unrecognized response — press r to view
the raw text.") is reworded to the table's phrasing.

## Architecture summary

```
appModel (app.go)
├── history []histNode, pos int     ← NEW: replaces fromList/hostList
├── Update: Alt+←/→, Esc → back/forward; routeFetch pushes nodes
├── View: subModelContent + statusBar.render()  ← NEW composition
├── help overlay toggle              ← NEW
│
├── readerModel (reader.go)          ← sheds status+hint lines
├── listModel (list.go)             ← sheds Title + built-in help; flags→bar
└── statusBar (statusbar.go)        ← NEW pure renderer
```

Dependency direction is unchanged (`finger/` → `render/` → `tui/`). All new
code is in `tui/`.

## Testing approach

Consistent with the project's injected-fakes, offline, no-TTY discipline:

- **History stack (unit, on `appModel`).** Using the existing stub-`FetchFunc`
  pattern, drive: push on fetch; back restores the prior node without a new
  fetch (assert the stub's call count does not increase); forward re-applies;
  navigating after a back truncates the forward tail; back at `pos == 0`
  returns to the landing and a further `Esc` at the landing quits; restore
  re-applies `scrollY`/`listIdx`/`listFltr`.
- **Status bar (unit, pure).** `statusBar.render()` across states: profile,
  cross-host (where `escTarget` ≠ structural parent), directory/list (no `user`
  half), each flag, landing, and narrow-width truncation of the breadcrumb and
  `◂ esc:` target. Plain-text assertions; no color dependence (the renderer
  takes `styles`, but tests assert on stripped/structural content).
- **Keymap routing (unit).** `Alt+←`/`Alt+→` map to back/forward; `Backspace`
  reaches the input as an edit (never navigates); `Ctrl+C` quits; `?` toggles
  the overlay; filter-mode defers `Esc`/`Enter`/`?` to the list.
- **Regression.** `ParseUsers` golden corpus (`userlist_test.go`) and `finger/`
  local-server tests are untouched and must stay green. `main` router tests are
  unaffected.

## Accepted residual scope

- Back/forward restore re-applies *light* view state (scroll/selection/filter)
  but rebuilds the sub-model from the cached body; it does not preserve deeper
  ephemeral UI state. Acceptable — finger bodies are small and sessions short.
- History is unbounded for the session. Finger bodies are capped at 1 MiB and
  sessions are short-lived, so growth is not a practical concern; no eviction.
- `Alt+←/→` and the `?` overlay rely on Bubble Tea v2 key decoding; terminals
  that cannot deliver `Alt+arrow` still have `Esc` for back. Forward has no
  fallback key by design (it is the rarer action); this is acceptable for v1.

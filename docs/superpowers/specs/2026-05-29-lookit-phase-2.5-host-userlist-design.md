# lookit Phase 2.5 design — host user-list drill-down

## Goal

When `lookit` fingers a **host** (`@host`, no user) and the response contains a recognizable user list, present that list as a selectable screen. Pressing Enter on a user fingers `login@host` and shows the result in the reader. This makes the common pubnix workflow — "see who's here, read someone's plan" — a click instead of retyping a target.

Everything else stays exactly as Phase 2. This is a contained increment on top of the merged query-first TUI, not a rewrite.

## Non-goals

- **No no-arg landing screen (idea 1).** `lookit` with no arguments still opens the empty reader. Deferred — see "Deferred work".
- **No in-viewport token-selection fallback.** Hosts whose output we can't parse render in the plain reader with no selection. Deferred.
- No subscriptions, refresh, diffing, persistence, or catalog.
- No new CLI surface. `lookit @host` still works as a one-shot from the command line, unchanged; the list screen is a TUI-only behavior reached by fingering a host *inside* the TUI.
- No real-name column parsing for unicode-width names. List rows show login plus a best-effort name only when trivially available; login-only is acceptable.
- No general navigation stack. At most two levels: reader ↔ list ↔ drilled reader.

## Background: why hybrid, and why these rules

RFC 1288 leaves host (`/W`-less list) output entirely implementation-defined. Live captures on 2026-05-29 confirmed at least three incompatible shapes plus several "not a list at all" responses. A single universal parser is impossible; cue/header/marker gating with a confidence floor is what keeps us from hallucinating lists out of banners.

The selection model is **hybrid**: when we confidently recognize a format we show a clean list screen; otherwise we fall back to the plain reader. The original hybrid idea also included token-selection over raw text for unrecognized hosts; that is **deferred** (see "Deferred work") because the recognized formats already cover the real multi-user hosts, and in-viewport token highlighting is the costliest, riskiest component.

## CLI behavior

Unchanged from Phase 2:

```text
lookit                  open the TUI (empty reader)
lookit user@host[:port] one-shot query
lookit @host[:port]     one-shot server query
lookit version          print version
lookit -h / --help      print usage
```

The host user-list drill-down is reached only inside the TUI, by typing a `@host` target into the reader input and pressing Enter.

## Architecture

Refactor today's single `tui.Model` into a glow-style state machine of bounded units. `tui.Run(ctx, profile)` remains the only exported entry point, so `main.go` is untouched.

```
tui/
├── app.go            # appModel: state machine, routing, transitions, back/quit
├── app_test.go
├── reader.go         # readerModel: input + viewport + status (extracted from Phase 2 model.go)
├── reader_test.go
├── list.go           # listModel: wraps bubbles/v2/list + one-line item delegate
├── list_test.go
├── userlist.go       # ParseUsers(body) ([]User, bool) — pure parser, no TUI deps
├── userlist_test.go  # golden corpus
├── fetch.go          # Entry, FetchFunc, fetchCmd (existing, unchanged)
├── styles.go         # styles (existing, extended for list/delegate)
└── run.go            # Run (existing; constructs appModel instead of Model)
```

The Phase 2 `model.go`/`model_test.go` become `reader.go`/`reader_test.go`. The reader's behavior is preserved; it simply no longer owns process-level concerns like "what happens after a fetch" — `appModel` decides that.

### appModel

```go
type appState int

const (
	stateReader appState = iota
	stateList
)

type commonModel struct {
	width   int
	height  int
	profile colorprofile.Profile
	fetch   FetchFunc
}

type appModel struct {
	common   *commonModel
	state    appState
	reader   readerModel
	list     listModel

	// Cached host list so Back from a drilled user is instant (no re-fetch).
	hostList *Entry
	fromList bool // true when the reader is showing a user drilled from the list
}
```

`appModel.Init` delegates to the reader (blink + capability requests, as Phase 2). `Update` intercepts the cross-screen keys and the fetch result, then delegates the rest to the active sub-model. `View` renders the active sub-model.

### Sub-model boundaries

- **readerModel** — owns the input, viewport, status line, and rendering of a single `Entry` via `render.Render`. It does *not* start the TUI, route between screens, or decide what a fetch result means. It exposes enough for `appModel` to drive it: set the current entry, set status, focus the input.
- **listModel** — owns a `charm.land/bubbles/v2/list.Model` plus a one-line delegate. Constructed from `(host finger.Target, users []User)`. Exposes the currently highlighted user and its filter state.
- **userlist** — pure functions. No imports beyond stdlib.

## Data flow

1. User types a target in the reader and presses Enter. Reader validates with `finger.ParseTarget` (unchanged), starts the async fetch (unchanged `fetchCmd`).
2. `fetchResultMsg` arrives at `appModel`. **One routing decision** (`routeFetch`):

   ```
   if entry.Err == nil && entry.Target.User == "" {
       if users, ok := ParseUsers(entry.Body); ok {
           cache entry as hostList; build listModel(entry.Target, users); state = stateList
           return
       }
   }
   // otherwise: hand the entry to the reader (plain render), state = stateReader, fromList = false
   ```

   Keeping this in one function is the structural guard that makes the deferred features (idea 1, token-select) one-line additions later rather than refactors.
3. In `stateList`, `Enter` on a highlighted user constructs the target by re-parsing through `finger.ParseTarget(login + "@" + host)` — reusing Phase 2 validation and port normalization rather than hand-building the struct — sets `fromList = true`, starts a fetch, and switches to `stateReader` with a loading status. The `host` is taken from the cached `hostList.Target` (preserving any explicit `:port`).
4. That fetch result is a user target (`User != ""`), so `routeFetch` sends it to the reader. `fromList` stays true.

## Navigation and keys

Two levels of back, tracked by `state` + `fromList`. No general stack.

| Context | Key | Behavior |
|---|---|---|
| any | `Ctrl+C` | quit |
| reader home (`fromList == false`) | `Esc` | quit (Phase 2 behavior) |
| reader showing drilled user (`fromList == true`) | `Esc` or `⌫` | restore cached `hostList` → `stateList` (no re-fetch) |
| list, not filtering | `Esc` | back to reader home (`stateReader`, `fromList = false`) |
| list, filtering | `Esc` | clear filter (handled by the list component) |
| list | `Enter` | drill into highlighted user |
| list | `↑ ↓`, `PgUp/PgDn`, `/`, `?` | handled by the list component (move, paginate, filter, help) |
| reader | `↑ ↓`, `PgUp/PgDn`, `Home/End`, `Enter` | unchanged from Phase 2 |

`appModel.Update` intercepts `Ctrl+C` always; intercepts `Esc`/`⌫` and `Enter` based on `state`/`fromList`/filter-state; delegates everything else to the active sub-model. Starting a new host fetch from any reader resets `fromList = false`.

`WindowSizeMsg` and `ColorProfileMsg` are stored on `commonModel` and applied to both sub-models so a resize or profile change is correct regardless of the active screen.

## List screen layout

Dense, one line per user (chosen over the two-line Bubbles default — finger lists can be long):

```
lookit  @plan.cat — 38 users

> jss
  geurimja        Geurimja
  notroot         Not Root
  nonvernal
  artemvang       Artem Vang
  ...
•••○○   ↑↓ move · enter finger · / filter · esc back
```

- Title row: `lookit` + dim `@host — N users`.
- One-line delegate: highlighted-row marker, `login`, then dim real name if present. Grid/marker hosts (tilde.team, happynetbox) show login only.
- Footer: dot paginator + key hints, mirroring the reader's hint style.

## Parser specification (`ParseUsers`)

```go
type User struct {
	Login string
	Name  string // optional; "" when unknown
}

func ParseUsers(body []byte) ([]User, bool)
```

Returns `(users, true)` only when a format is confidently recognized; otherwise `(nil, false)`.

**Login shape:** `^[A-Za-z0-9_][A-Za-z0-9_.-]{0,31}$`. Leading alphanumeric (covers plan.cat's `26d0`); rejects FQDNs, words with punctuation, and over-long tokens.

**Format 2 — columnar (gated by header):**
- Trigger: a line whose first whitespace token is `Login` (case-insensitive), typically followed by `Name`/`Tty`.
- Each subsequent non-blank line: `Login` = first whitespace token (must pass login shape); `Name` best-effort. Stop at a blank line.
- Fixtures: plan.cat, tilde.institute, tilde.pink.

**Format 1 — grid (gated by cue, block-scoped):**
- Trigger: a cue line matching `/logged[\s-]?in|online/i`.
- Skip up to one blank line, then collect the **contiguous** block: each line is split on whitespace; **every** token must pass login shape or the line ends the block. Each token is a login. Stop at a blank line or the first non-conforming line.
- Block scoping is essential: it prevents cosmic.voyage's separate `Who control these ships:` block (multi-word personas) from being parsed, and captures only `klu tomasino` from its `Users currently online:` block.
- Never parse tokens inline on the cue line (typed-hole's `Users currently logged in: probably julien` must not yield `probably`).
- Fixtures: tilde.team, envs.net, zaibatsu, cosmic.voyage (online block).

**Format 3 — marker list:**
- Collect contiguous lines matching `^\s*>\s+<login>$` (a single login token after the marker). Excludes lines like `> finger random@happynetbox.com` (space after marker → not a single token).
- Fixture: happynetbox.

**Confidence and post-processing:** require ≥1 login once a format has triggered. Deduplicate, preserving first-seen order (tilde.pink lists `irek` on ~10 TTYs → one entry). Only attempt parsing for host targets; the caller already guarantees `Target.User == ""`.

## Testing

**`userlist_test.go` — golden corpus** from live captures (stored as fixtures):
- *Parses:* plan.cat & tilde.institute & tilde.pink (columnar; tilde.pink asserts dedup), tilde.team & envs.net & zaibatsu & cosmic.voyage-online-block (grid), happynetbox (marker). Assert logins and order; assert cosmic.voyage yields exactly `klu`, `tomasino`.
- *Correctly declines (`ok == false`):* tilde.town (banner), tilde.club (empty), typed-hole (services menu + inline cue), db.debian.org (daemon help).

**`app_test.go` — transitions with an injected `FetchFunc`:**
- Host fetch that parses → `stateList`, host entry cached.
- Host fetch that declines → `stateReader`, plain render.
- `Enter` in list → fetches `login@host`, `fromList == true`, `stateReader`.
- `Esc` in drilled reader → `stateList` restored from cache, no new fetch.
- `Esc` in list (not filtering) → reader home.
- `Ctrl+C` quits from either state.
- `WindowSizeMsg`/`ColorProfileMsg` propagate to both sub-models.

**`reader_test.go`** — the Phase 2 reader tests, carried over.

**`list_test.go`** — delegate renders login (+ name when present); highlighted-user accessor returns the right login after movement and filtering.

## Deferred work

Recorded here so the rationale and the (low) cost of deferral are on the record.

- **No-arg landing screen (idea 1).** `lookit` with no args probing `@localhost` and falling back to a curated catalog. Deferred to keep this increment small. **Cost of deferring: negligible.** It is additive — a new branch in the no-arg path plus reuse of `listModel`; it needs no change to `routeFetch`, `ParseUsers`, or the data model.
- **In-viewport token-selection fallback.** Selecting login-ish tokens in unparsed host output. **Cost of deferring: negligible.** Additive — a third case in `routeFetch` (`host && unrecognized && has tokens`), a token-extraction function alongside `ParseUsers`, and a selection sub-mode on `readerModel`. All data it needs (raw `Entry.Body`, host `Target`) is already retained.
- **Unicode-aware real-name columns.** plan.cat has names like `わだ`; column-width math is fiddly. Login-only rows are acceptable; names are shown only when trivially extracted.

## Future navigation direction

Recorded so the architecture grows in a known direction; **none of this is built in Phase 2.5.**

Navigation is layered in three nested levels, and the choices at each level are independent:

1. **Top-level tab router** (Phase 3) — switches between feature areas, e.g. `[ Look up ] [ Discover ] [ Subscriptions ]`. This is the soft-serve pattern: a tab bar over a page-router where each tab is a page implementing a common interface (`Init`/`Update`/`View`/`SetSize`).
2. **Per-tab navigation** — each tab picks the layout that suits its content: a **stacked** drill-down (full width, good for wide ASCII plans) or a **master-detail split** (`lipgloss.JoinHorizontal`, good for graze-and-preview of curated/short content).
3. **Within-screen components** — `bubbles/v2/list`, viewport, etc.

Mapping: the Phase 2.5 reader + host-list drill-down becomes the **Look up** page (stacked). Phase 3 **Discover** (curated catalog) and **Subscriptions** (watch/diff) are natural master-detail split pages.

Why this is forward-compatible with no rework: the tab router sits *above* today's `appModel`, which becomes the Look up page unchanged; the shared `commonModel` (size/profile/fetch) lifts to become the router's shared context. Adding a page is additive. The formal page-router interface should be introduced only when Phase 3 brings a third screen — abstracting on two screens is premature. Watch for tab-switch key collisions (`Tab` is consumed by text inputs) when the tab bar lands.

## Known limitations

- A logged-in/online grid block that contains a space-separated display name could mis-split into multiple logins. Cue-block scoping makes this unlikely in practice (logged-in blocks rarely carry display names); accepted for this phase.
- Hosts with real user lists but no recognizable cue/header/marker fall back to the plain reader. Users can still drill in by typing `login@host` manually.

# tui/

The interactive layer: a glow-style state machine on Bubble Tea **v2**
(`charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2` —
note the `charm.land` v2 import paths, not `github.com/charmbracelet`). `Run()`
in `run.go` is the only exported entry point, so `main.go` never touches the TUI
internals.

The repo-wide rules live in the root `CLAUDE.md`; this file covers the parts of
the TUI that span files. Networking is injected, never real: model tests drive
transitions with a stub `FetchFunc`/`fetchCmd` (fetch.go).

## The models

- **`appModel` (app.go)** is the top-level model. It owns lifecycle (Init/Update/View), the target input, the bottom status bar, quit, and routing between four sub-models via a `state` enum (`stateReader` | `stateList` | `stateAbout` | `stateStart`). The target input is also the active-address display: landing a response, restoring history, drilling, refreshing, and returning to the startpage synchronize it to the visible location; starting an edit seeds from that location, and canceling an edit restores it. `commonModel` holds shared width/height/profile/fetch. The sub-models — **`readerModel` (reader.go)** (a headerless viewport whose first physical line is the first rendered response line), **`listModel` (list.go)** (a `bubbles/v2/list` of a host's users), **`aboutModel` (about.go)** (the full-screen about view), and **`startModel` (start.go)** (the startpage list, assembled by `tui/sections.go`) — do *not* own quit/lifecycle; `appModel` drives them through small methods (`setSize`, `setEntry`, `setProfile`, `setBackground`, `selected`, `filtering`). Landed response latency belongs to the status bar for reader, list, raw, focused-input, and links-panel views; it is omitted when it would displace existing status information and is absent on Start/About.

**Message delegation follows the overlay, not just `state`.** `Update` handles
app-level messages in its own cases and delegates the rest to one sub-model. The
`state` enum does not name the links panel, so while `showingLinks` is set the
panel takes the delegated messages — otherwise they go to the view underneath
(the reader, for a panel over a profile) and the panel silently never sees its
own `list.FilterMatchesMsg`, which is exactly how its filter came to accept a
query, draw the prompt, and narrow nothing. **A test that drops the `tea.Cmd`
returned by a keystroke cannot catch this**: the message that exposes the
misroute is the one the runtime would have delivered back. Feed it through
`appModel.Update`, as `pumpKeys` does.

## The startpage (`stateStart`)

**`stateStart`** is the startpage — the launch screen shown at `pos == -1`, rendering the embedded catalog (`tui/catalog.txt`) plus the user's bookmarks; like About it is never stored in a `histNode`, so history semantics are untouched. A `catalog off` line in the bookmarks file hides the built-in catalog, and a `sort manual` line turns off the shelf's own ordering (below).

Startpage order is **computed, not file order**: `buildSections` sorts communities alphabetically by host and groups services under their host root (parent first, children drawn under a `├`/`└` connector showing only their query token), so a new `catalog.txt` line can be added anywhere. A host with non-root service entries must ship a root entry — `TestCatalogHasRootForEveryGroupedHost` fails the build otherwise. A root that heads a group but is listed elsewhere (`@happynetbox.com`, a community) or is pinned is marked `structural`: it renders as the parent and drops out while filtering, so grouping never adds a duplicate to a flattened view. It is still a row on screen, so it **counts** in the section that drew it: the overview and the status bar describe what each section is showing, not unique listings, which is why pinning a service host that keeps its children does not move the services count and why a host on screen twice is counted twice (#123). Because a structural parent heads a group rather than offering the place it names, a `group <host> <note>` catalog line can give that header its own note (`@happynetbox.com` describes itself differently in COMMUNITIES and as the SERVICES header); `group` is metadata, not a section — it never renders as a row, never reaches a bookmark, and a host without one keeps its root's note. `TestCatalogGroupLinesDescribeARealGroup` fails the build on a group line with no root listing or no children. A service child renders as a bare token under a `├`/`└` connector with an empty note column; its note appears when its row is selected, and whenever a non-empty filter flattens it into a listing rather than a group member.

**BOOKMARKS is ordered by that date too, longest ago first** (`sortByVisit`, sections.go), so the pins you have been neglecting surface without any manual arrangement. A pin with no date sorts *below* every dated one rather than above: an undated pin is usually one just added, and reading it as infinitely stale would bury the ordering under new arrivals. The sort is display-only — the file is never rewritten, so a hand-arranged shelf survives behind the view and a `sort manual` line brings it back. `SliceStable` makes file order the tie-break, which is what keeps the undated block and same-second visits deterministic.

**A pinned row's note column shows its relative last-visited date** (`visited today` / `visited yesterday` / `visited N days|months ago` / `visited over 1 year ago`, local-zone day buckets), blank when unknown. The `visited ` prefix is applied by `startRowNote`, not by `relativeDay`, which stays a pure bucket function: the column holds prose for catalog rows, so a bare `today` changes register with nothing to announce it, and a flattening filter interleaves the two kinds of row in that one column. The value stands down to the catalog note when a filter flattens the view — assembly still carries the catalog note onto a pin (so unpinning restores the description with no state to unwind, and `/` still matches on it), but `startRowNote` keys the displayed note for any row whose `source` is `sourceBookmark`. The cursor does not lift an unvisited row's blank, unlike a child's silence; a flattening filter lifts both, which is exactly where a note-scored match is visible. Suppression keys on `source`, not on the `bookmarked` flag, for the same reason the `★` marker does: a structural parent retained to head a SERVICES group is stamped bookmarked but was built from the catalog, and must keep its note.

Catalog notes are capped at 48 terminal cells so a note does not truncate at 100 columns. `FilterValue` returns target and note for selectable, non-structural rows; structural rows and headers return empty so filtering flattens to matches and cannot show a duplicate of the listing a structural parent stands in for. A repeated user-authored bookmark line is one target: `parseBookmarks` keeps the first occurrence's position and drops the rest, so the shelf never draws the same place twice and the counts follow. The dedupe is display-side only — the file keeps every line, the visited map still takes the last date, and `deleteBookmarkLine` still clears every matching record, so unpinning the one row unpins all of them.

## Expanded Help

**Expanded Help** is a transient, bottom-docked overlay, not a history node and
not a resizing layout element. `tui/help.go` derives a priority-ordered candidate
set from live `key.Binding` values, retains the longest prefix that fits a
one-to-three-column layout, and uses the same retained set as the execute gate.
`?` and Esc return to the underlying view; `a` deliberately opens About even
over the focused target input; every other displayed action replays its original
key message through normal app/component routing. Unrecognised, disabled, and
height-clipped commands do nothing while Help stays open. The renderer uses
Bubbles bindings and `help.Styles` but not `help.Model`, whose fixed columns
cannot provide this responsive layout. The permanent bar still says `? help`
while open, where `?` acts as the toggle back.

## Filtering: the accept key is lookit's, not the list's

All three filterable lists — startpage, user list, links panel — hand their
filter keys to `bubbles`, with **one interception: the accept key**. `bubbles`
computes matches in a `tea.Cmd` and its own accept path flips to `FilterApplied`
using whatever match set is installed at that instant, so an accept arriving
before the `FilterMatchesMsg` for the query on screen applies the *pre-filter*
selection — and the next key acts on it (issue #129: Enter fingered the wrong
account, `b` wrote a target the user never highlighted into the bookmarks file).
`handleKey` therefore matches `KeyMap.AcceptWhileFiltering` first and calls
`acceptFilter` (list.go), which re-applies the query through `SetFilterText` —
the synchronous path — reproducing the two cases `bubbles` special-cases (an
empty query and a zero-match query both clear the filter). The startpage wraps
it as `startModel.acceptFilter` so the header-skipping cursor rule runs too.
The filter commands already queued by earlier keystrokes still run, and their
`list.FilterMatchesMsg` carries no query or generation identity, so `Update`
delegates those messages only while the active list remains in `Filtering`;
after accept or reset every remaining result is stale and is dropped.
**Don't route the accept key back to the list**: the selection it settles on is
what the next keystroke acts on, and a fetch or a bookmark write is not
something to get from a stale set.

## Fetching, routing, navigation

- **Request lifecycle (request.go):** every fetch is a `pendingRequest` with its own cancellable context derived from the session ctx, so `esc` while loading aborts the connection itself (`finger.Query` honours ctx), not just the spinner. A result whose id no longer matches the pending request is dropped, so a canceled or superseded fetch can never repaint. `requestNavigate` pushes a new history node; `requestRefresh` (`r`) replaces the current node in place, preserving reader scroll/link focus and list filter/selection, and leaves a `requestFailure` warning with the previous response still on screen when a refresh comes back empty.
- **`routeEntry` (app.go) is the single decision point** for a completed fetch: a host response (`Target.HostQuery()` — an empty or `@`-prefixed query, via `shouldOpenList` — plus the special-cased `ring@thebackupbox.net`) that `ParseUsers` recognizes opens the list; everything else renders in the reader. Errored/truncated responses that still carry a parseable body open the list too, flagged `(incomplete)` in the title.
- **`ParseUsers` (userlist.go)** is a pure, dependency-free parser that recognizes whether a host response contains a selectable list. It tries cue/header-gated matchers in order — generic columnar / grid / marker, then service-specific menus (typed-hole, sava.rocks, redterminal, the Finger Ring, telehack), the `parseGenericList` structured-login fallback, and finally `parseAddressList` for bare `user@host` runs. It dedups and **declines** (returns false) rather than guessing. It is validated by a golden corpus of real server captures in `userlist_test.go`, with both parse and decline cases.
- **Drilling & navigation:** Enter on a list user fingers `login@host`; entries that carry an explicit `User.Target` (from `finger://` links or `finger user@host` commands in the response) drill cross-host. **Safety:** server-supplied targets are parsed with `finger.ParseTargetPinned`, which forces port 79 — discarding any explicit port rather than rejecting it, so a malformed/out-of-range port doesn't silently kill the drill — and refuses forwarded `user@host@relay` targets lifted from a response (`finger.ErrServerForwarding`); user-typed ports and forwarding are preserved. Back navigation is history-based: each landed screen is a `histNode` (`push`/`snapshot`/`restore`/`stepBack`), so Esc walks back **without re-fetching** and falls through to the empty target screen at the root; Ctrl+C always quits, Esc is context-dependent. `handleKey`/`drill` return a concrete `appModel`, and `Update` adopts the returned model even when a key isn't fully handled (so model mutations survive a non-handled key).

Reader word wrapping is an opt-in per-`histNode` display preference: new responses start unwrapped, refresh and Back preserve the choice, and source view temporarily ignores it. The renderer's logical-line map keeps the same source line visible across toggles and width changes (horizontal offset resets); a focused link instead keeps the existing centred-link policy, including after refresh. Every normal render positions exactly once after render, overlay, and content assignment. Height-only resizing does not re-render, and the reader's raw-view flag prevents a width change from replacing source bytes with a normal render. The reader applies link focus/OSC-8 overlays only after wrapping, while detection and byte metadata continue to use the original `Entry.Body`.

- **About screen (about.go):** a full-screen `stateAbout`, opened with `a`. It holds the `☞ lookit` gradient hero (the wordmark lives here, not on the landing), the version/repo/licence, and two actions — finger the author (`↵`) and copy the issues URL (`y`). **Honest keybinding:** `a` is matched only when content is focused (input blurred) or the `?` help panel is open (the landing reaches About via `?`→`a`) — *never* in the input-focused branch, so `a` types into a target like `alice@host`. About is transient: not pushed to history; `aboutFromState` restores the origin screen on close.

## Second ingress: the bookmarks file

The bookmarks file (`tui/bookmarks.go`) is the one ingress outside `finger.Query`.
It validates rather than sanitizes, and displays nothing unvalidated — see
`finger/CLAUDE.md` for the full contract, and change it only with that contract
in hand.

**The file is the whole of lookit's configuration UI**, so it documents itself: `loadBookmarks` seeds a new one with `bookmarkFileHeader` (comments only, written once and never re-applied) naming the record shape, the `#` comment rule, and both directives, followed by the `starterBookmarks` records (the author, plus the fingerverse noticeboard and one dated `.plan`) in declaration order, which is what undated pins tie-break on. A record's date is a plain `YYYY-MM-DD` day, not an instant — `relativeDay` buckets by local calendar day, so a day is everything the display consumes, and `updateBookmarkLine` leaves a record untouched when the rewrite would be byte-identical, which makes a second visit on the same day write nothing.

It is *not* untrusted input in the sense `Query` is: it lives at
`~/.config/lookit/bookmarks`, `0o600` under a `0o700` dir, so anyone who can write
it already owns the home directory. Treat it as an ingress because it parses
external bytes that must not panic and must not reach the display unvalidated —
the file is hand-editable by design, and `toggleBookmark` can persist a
drilled target derived from a remote response body, so a hostile host has narrow,
validator-filtered influence over its contents.

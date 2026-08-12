# Consistent link actions design

## Goal

Make link interaction predictable across lookit's reader, links panel, help
popover, and About screen. The keyboard vocabulary becomes a small invariant:

- **Enter goes somewhere inside lookit.** It submits a target, opens a selected
  user, or follows a definite finger target.
- **`y` copies.** Enter never copies as a fallback action.
- **`f` resolves an ambiguous `user@host` address** by explicitly treating it
  as a finger target.
- **External opening belongs to the terminal.** lookit continues to emit OSC-8
  hyperlink metadata, but it neither opens external applications nor teaches a
  terminal's mouse gestures.

This replaces two inconsistencies in the current behavior:

- In the **links panel**, Enter follows a definite finger target but *copies*
  every other row — the one place Enter means “copy” instead of “go”.
- In the **reader**, Enter is dead: the `Open` binding is disabled while reader
  content is focused (`updateKeymap` enables it only for the input, the user
  list, the panel, and About), so `activateFocusedLink` is unreachable from the
  keyboard. This spec makes Enter live in the reader for definite finger
  targets — an additive fix, not just a re-mapping.

## Decision and alternatives

Three interaction models were considered:

1. **Semantic keys (chosen):** Enter navigates inside lookit, `y` copies, and
   `f` handles the one ambiguous address shape. This preserves existing app
   conventions and never launches an external application.
2. **Universal activation:** Enter follows finger targets and launches URLs or
   mail applications. This gives every row an Enter action, but makes an
   untrusted response able to trigger an external application and takes over a
   responsibility already handled by terminal hyperlinks.
3. **Contextual primary action:** Enter performs whichever action is available,
   including copy. This is the current *links panel* model. It is compact, but
   the same key means navigation in lists and clipboard mutation on many panel
   rows, and the reader proves the model can't hold everywhere — its Enter had
   to be disabled rather than choose a meaning.

The semantic-key model is chosen because the action is knowable from the key
before it is pressed. The UI does not need to compensate for inconsistent
behavior with increasingly detailed status hints.

## Action model

The selected link's classification determines which actions are available:

| Link | Enter | `y` | `f` |
|---|---|---|---|
| Definite finger target | Go to the target | Copy the displayed link | No action |
| Ambiguous bare `user@host` | No action | Copy the displayed address | Go to it as a finger target |
| URL, email, or social address | No action | Copy the displayed link | No action |
| Blocked cross-relay finger target | Show the existing refusal | Copy the displayed link | No action |

“Definite finger target” includes `finger://` URLs, cue-qualified finger
addresses, and `@host` host queries. Existing target parsing, port-79 pinning,
same-relay forwarding, and cross-relay refusal rules are unchanged.

Pressing Enter on a copy-only URL, email, social address, or ambiguous address
does nothing: it does not copy, close the links panel, move focus, or show a
success flash. A blocked cross-relay target remains the exception because the
refusal explains why the apparently finger-shaped target cannot be followed.

One existing behavior must be narrowed to match this matrix: the reader's `f`
handler currently fires a fetch for **any** finger link with a parsed target,
including definite ones. Under this spec `f` acts only on ambiguous addresses,
matching what the links panel already does. (Blocked links carry no parsed
target, so `f` cannot bypass the cross-relay refusal today or after the
change.)

The internal `ActionDrill` name does not need to change. “Drill” is an
implementation term; all user-visible copy uses **go**.

## Reader behavior and status copy

Reader link navigation remains:

- `tab` or `n`: focus the next link, wrapping at the end.
- `shift+tab` or `N`: focus the previous link, wrapping at the beginning.
- `L`: open the links panel.
- `y`: copy the focused link; with no focused link, retain the existing behavior
  of copying the current result's address.

Enter's enablement while reader content is focused follows the selected link's
action: enabled for a definite target (`go`) and a blocked target (`refuse`),
disabled when there is no focused link or the link is copy-only. Dispatch then
fetches a definite target or flashes the refusal. Reader help is contextual: it
advertises `↵ go` only for `go`, never for `none` or `refuse`, even though the
blocked case keeps Enter enabled so pressing it can explain the refusal.

The focused-link status bar shows only actions available for that link. It uses
the word `tab`, not the less recognizable `⇥` glyph, and never uses “drill”:

```text
link 2/4 · url · y copy · tab next
link 3/4 · finger · ↵ go · y copy · tab next
link 4/4 · address (ambiguous) · f go · y copy · tab next
```

The existing cross-relay explanation remains visible for a blocked target,
alongside `y copy` and `tab next`. There is no `↵ go` hint because Enter cannot
navigate to it.

Copy renames this implies (current → new):

- focused-link hint `↵ drill` → `↵ go`, `f finger` → `f go`, `⇥ next` → `tab next`
- kind label `address (auto)` → `address (ambiguous)`

Remove `⌘-click opens` from every lookit-owned surface. Keep emitting OSC-8
hyperlinks for the schemes already supported (`isOSC8Openable` and the
`applyLinkOverlay` wrapping are untouched). Whether and how those hyperlinks
open is a terminal capability communicated by the terminal and its
documentation, not by lookit.

## Links panel

The links panel obeys the same action matrix as the reader. Moving the selection
changes the contextual actions; it does not alter their meanings.

- Enter closes the panel and starts a lookup only for a definite, allowed
  finger target.
- `f` closes the panel and starts a lookup only for an ambiguous `user@host`
  (this is the panel's current behavior; it stays).
- `y` copies the selected row and leaves the panel open, with the ordinary
  `copied …` flash. This is a new panel binding, handled at app level.
- Enter on a URL, email, social address, or ambiguous address leaves the panel
  open and does nothing.
- Enter on a blocked cross-relay target leaves the panel open and shows the
  existing refusal. (`Open` therefore stays enabled on blocked rows even though
  the hints advertise only `y copy` — the refusal is feedback, not a “go”.)
- `esc` or `L` closes the panel and returns to the reader with the selected link
  focused, as today.

**Filter lifecycle (new).** The panel currently intercepts `esc`/`L`/Enter/`f`
at app level even while its `/` filter input is active, so those keys trigger
actions instead of filtering — and adding `y` would extend the conflict into
plain filter text. Mirror both of the user list's existing filter guards:

- While `FilterState() == list.Filtering`, every key except the app-level
  `ctrl+c` force-quit delegates to the Bubbles list. Printable `y`, `f`, and `L`
  edit the filter, Enter/Tab apply it, and Esc cancels it; no panel action fires.
- While `FilterState() == list.FilterApplied`, Esc delegates to the list and
  clears the filter. A subsequent Esc, once unfiltered, closes the panel.
- While unfiltered, the ordinary panel actions apply.

**Panel status bar (new).** `buildStatusBar` has no panel branch today — the
bar under the open panel renders the reader's hints. Add a selection-aware
branch whose baseline is:

```text
↑/↓ move · / filter · esc back
```

It appends only the selected row's actions: `↵ go` for a definite finger
target, `f go` for an ambiguous address, and `y copy` for every row. A blocked
target shows `y copy` but not `↵ go`.

The baseline changes with the filter state so the bar remains honest:

- actively filtering with an empty value: `type to filter · esc cancel`;
- actively filtering with a non-empty value: `enter apply · esc cancel`;
- applied filter: `↑/↓ move · esc clear filter`, followed by live row actions;
- unfiltered: the baseline above, followed by live row actions.

The existing `statusBarModel` flash override remains responsible for rendering
the transient `copied …` or refusal message over every branch, including the
new panel branch; the panel does not add a second flash path.

## Help popover

The general action column retains the existing bindings, filtered to actions
that are live in the current state:

```text
↵ go
i target
y copy
v view source
```

On a reader result containing at least one detected link, add a contextual
column:

```text
tab/n next link
shift+tab/N previous link
L browse links
```

Mechanics: replace the unconditional `m.keys.FullHelp()` selection in
`helpView` with a small contextual `helpGroups()` helper. It reuses the existing
bindings and returns the visual columns appropriate to the active state;
`fullWidthHelpView` continues to render each returned group as one column and
each binding within it as a row. For a reader with links, the helper appends a
column containing `LinkNext`, `LinkPrev`, and `LinkPanel`.

Hide that column when the current reader result has no detected links by
conditioning all three bindings' reader-mode enablement on `len(links) > 0` —
today they are enabled for any reader result regardless. `LinkPanel` becomes
enabled again while the panel is open so `L` can close it. Copy renames in
`keys.go` WithHelp strings: `L` “links panel” → “browse links”,
`shift+tab/N` “prev link” → “previous link”.

Do not put `f go` in this general column: it exists only when an ambiguous
address is focused, and the status bar advertises it at that moment.

When the links panel is open, `?` opens the help popover with panel-relevant
bindings rather than reader scrolling bindings:

```text
↑/↓ move
/ filter
esc back
```

plus the selected row's live actions, following the same matrix as the panel
status bar. It does not advertise Enter for copy-only or blocked rows.
`helpGroups()` builds this view from the existing synthetic `Move` and `Filter`
bindings plus `Back`, then uses the shared action helper to decide whether to
include `Open`, `LinkFinger`, and `Copy`. The Bubbles list still owns the actual
move and filter behavior; the app-level bindings provide their help copy.

## About screen

About remains a static, self-describing screen rather than gaining a focus
model. Tab does nothing there and is not advertised. Its two direct actions
already fit the global semantics:

- Enter goes to `jonathan@tilde.team` inside lookit.
- `y` copies the issues URL.

Tighten the action copy to:

```text
➜ finger jonathan@tilde.team        ↵ go
➜ Report a bug or idea              y copy issues URL
```

The About status bar names the destinations because no selection supplies that
context:

```text
↵ go to author · y copy issues URL · esc back · q quit
```

As today, the breadcrumb carries the back destination when About was opened
from a result; in that case the redundant `esc back` text is omitted.
The global help popover continues to advertise `a about lookit`; About does not
need its own help popover because its actions are always visible.

URLs on About receive no special lookit hint. Terminal-native link interaction
remains available without being documented by the app.

## State and implementation boundaries

This is a TUI interaction and copy change. The concrete edits:

- **Shared action helper (the single source of truth).** Add a small pure
  function — e.g. `linkActions(Link) → {enter: none|go|refuse, f: bool, copy:
  true}` — that encodes the action matrix. Reader dispatch, panel dispatch, the
  focused-link status bar, the panel status bar, and the panel-mode help
  popover all derive from it, so copy and routing cannot drift. `key.Binding`
  enablement stays the source of truth for whether a key *fires*; the helper is
  the source for what it *does* and how it's advertised.
- **Reader:** set `Open` from the focused link's helper result (`go` or
  `refuse`) and dispatch through the helper, making `activateFocusedLink`
  reachable without enabling Enter for copy-only links. Narrow the reader `f`
  handler to ambiguous links only, and make `LinkFinger`'s enablement
  focused-link-dependent. Condition `LinkNext`, `LinkPrev`, and `LinkPanel`
  reader-mode enablement on `len(links) > 0`.
- **Panel:** add guards for both active and applied filter states; handle `y` at
  app level (copy, panel stays open); keep `Open` enabled for definite and
  blocked rows, disabled for copy-only rows; enable the synthetic `Filter`
  binding in panel mode whenever the filter input is not active so `/ filter`
  can appear in contextual help; add the selection- and filter-aware
  `showingLinks` branch to `buildStatusBar`. Continue to rely on
  `statusBarModel` for the global flash override.
- **Copy:** the renames listed in the reader and help sections, plus removing
  `⌘-click opens` and tightening the About strings.
- **Help:** add contextual `helpGroups()` selection around the existing help
  renderer, including the reader link column and panel-mode columns.

Preserve reader link focus, scrolling, history restoration, and the links
panel's current selection behavior.

No changes are required in `finger/`, `render/`, `DetectLinks`, link
classification, user-list parsing, network behavior, clipboard transport,
history structure, or terminal mouse handling. No configurable keymap or new
external-launch command is introduced.

## Edge cases and feedback

- A response with no links has no link-navigation column in help;
  link-navigation keys remain inert (disabled, not merely unhandled).
- A copy-only link never acquires an Enter action merely because it is selected
  in the links panel.
- While the links panel filter is active, printable `y`, `f`, and `L` edit the
  filter, Enter/Tab apply it, and Esc cancels it; none acts on the selected row.
  With a filter applied, Esc clears it before Esc can close the panel.
- `y` always copies the exact displayed `Link.Raw`, preserving current behavior.
- Failed or blocked go actions never fall back to copying.
- Copy success continues to use the existing transient `copied …` flash, in the
  panel status bar as well as the reader's.
- Cross-relay refusal text remains consequence-first and does not claim a go
  action is available.
- Raw view retains no link focus or link-navigation actions.
- About remains non-focusable even though its body contains URLs and two
  commands.

## Testing

Follow the existing injected-model patterns; tests remain offline and do not
open external applications. Note that dispatch-level tests for reader link
keys barely exist today (`focusedLink` has no coverage in `app_test.go`), so
most of this is new coverage, not edits.

- **Reader dispatch:** Enter fetches definite finger targets; Enter does not
  copy or fetch copy-only and ambiguous links; Enter on a blocked target flashes
  the refusal; `f` fetches only an ambiguous address (including the regression:
  `f` on a definite finger target does nothing); `y` copies every focused link.
- **Links panel dispatch:** the same matrix holds, including whether the panel
  closes, stays open, flashes a refusal, or copies. While the filter is active,
  printable `y`/`f`/`L`, Enter/Tab, and Esc are delegated to the filter;
  `ctrl+c` still quits. With a filter applied, the first Esc clears it and the
  next closes the panel.
- **Status copy:** no user-visible output contains `drill`, `⇥ next`, or
  `⌘-click opens`; expected rows contain `↵ go`, `f go`, `y copy`, and
  `tab next` only when applicable. The panel branch shows its
  `↑/↓ move · / filter · esc back` baseline plus the selected row's actions,
  and renders the flash.
- **Help enablement:** the reader link column appears only when links exist; the
  links panel shows panel bindings and only live selected-row actions; About
  does not advertise Tab. Copy-only and blocked links never advertise `↵ go`,
  even though blocked Enter remains routable to its refusal.
- **About:** body and status assertions cover `↵ go`, `y copy issues URL`, and
  the absence of link-focus behavior.
- **Regression:** target submission, user-list Enter, reader/list `y`, history,
  raw view, filtering, and existing link detection/classification continue to
  work.
- `make check` is the final implementation gate.

## Accepted residual behavior

OSC-8 support and the gesture used to open a hyperlink vary by terminal and
configuration. lookit deliberately neither detects nor describes that behavior;
supporting terminals remain useful, while unsupported terminals still receive
ordinary visible text and lookit's `y copy` action.

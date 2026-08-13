# Help Popover UX Design

**Date:** 2026-08-13

## Goal

Make the expanded `?` help popover a compact, trustworthy command map for both
new and returning users. It should teach lookit's core browsing loop first,
retain every command that is usable in the current context when terminal
dimensions permit, and let a user act directly from Help without turning the
popover into another mode to learn.

## Scope

This change builds on the help-label simplification that omits `g/G top/bottom`,
advertises `tab` and `shift+tab` as the primary link gestures, and calls the
startpage `home`. It changes only the expanded help popover's ordering,
responsive layout, open-state key handling, and the availability metadata needed
to keep that behavior honest.

The following remain unchanged:

- all runtime key bindings, including unadvertised aliases;
- the permanent status-bar copy, including `? help`;
- the bottom-docked, opaque overlay and its existing colours;
- the height given to the underlying reader, list, and startpage, so opening
  Help cannot repaginate or reflow their content;
- filtering and loading behavior, where Help is unavailable; and
- the About screen, which remains self-describing and has no Help popover.

There are no category headings, popover title, close hint, scrolling controls,
or compact-versus-full modes. Historical design documents remain point-in-time
records and are not rewritten.

The status bar therefore continues to say `? help` while Help is open, even
though `?` closes it in that state. This is the existing toggle behavior and an
accepted consequence of leaving the permanent bar unchanged, not an assertion
that `?` only ever opens the popover.

The existing Help-only route to About remains deliberate. While the target
input is focused, pressing `a` directly still types into an address such as
`alice@host`; pressing `?` and then `a` opens About. Help is a distinct
transient layer where About is both displayed and available, so this does not
claim that `a` is a direct command while the input itself owns the keyboard.

## Information Architecture

Help uses one priority-ordered command sequence instead of labelled semantic
groups. Reading order supplies the hierarchy without asking users to interpret
whether an operation belongs to navigation, reading, or application control.

For ordinary screens, the sequence is:

1. core browsing: move, go, back, target, home;
2. contextual tools: page, next link, previous link, filter, browse, view
   source, refresh; and
3. occasional and application actions: copy, bookmark, browse links, About,
   quit.

Bindings that are disabled in the current state are removed before layout. The
context builder may narrow that enabled set further for a specialized surface,
such as the links panel. Its output is the ordered candidate set passed to the
responsive layout. The remaining commands keep their relative order. A reader
with detected links, for example, presents this wide layout:

```text
↑/↓       move       ←/→        page            y  copy
↵         go         tab        next link       b  bookmark
esc       back       shift+tab  previous link   L  browse links
i         target     v          view source     a  about lookit
h         home       r          refresh         q  quit
```

The links panel keeps its deliberately smaller, selection-aware command set.
It uses the same priority principle: movement and Back first, available actions
for the selected link next, then filtering and About. It does not inherit
irrelevant reader commands merely to make its layout resemble other screens.
About appears in every Help context because Help itself owns that route.

Help continues to derive labels and accepted keys from `key.Binding`. A label
shows the primary gesture, while every key in that displayed binding remains an
accepted way to invoke its action. No second display-only command registry is
introduced.

`Enabled()` must be truthful for the active interaction layer. `About` is
disabled when the target input is focused and Help is closed, so `a` continues
to type into targets such as `alice@host`. Opening Help enables About because
Help owns a dedicated `a` route; the context builder includes it in every Help
layout, including the focused input and links panel. Closing Help restores the
underlying layer's availability before another key is handled. `Open` needs no
equivalent layer-dependent rule: while the input is focused, Enter intentionally
means submit the current target. The other bindings retain their existing
context-aware enablement.

The Help binding itself is never returned by the context builder and therefore
never appears in or enters generic dispatch from the expanded popover. Its
enabled state is tightened to the contexts where the dedicated `?` toggle is
reachable; `?` is always handled by that dedicated rule while Help is open.

## Responsive Layout

The layout function receives the ordered bindings, terminal width, and number
of body rows available above the status bar. It evaluates candidate column
counts from three down to one, never using more columns than there are bindings.
For a candidate count `c`, it computes `rows = ceil(n/c)`, caps that at the
available body rows, and retains the first `rows*c` bindings (or all `n`, when
fewer). It then fills columns top-to-bottom using
`rows = ceil(retained/c)`; every column except possibly the last has `rows`
entries, and the last column may be short.

The layout function uses the first candidate whose total measured grid width,
including keys, descriptions, and separators, fits the terminal. If no
candidate fits, it uses one column and applies the existing ANSI-aware ellipsis.
Key labels remain aligned within each column, and each rendered row remains a
full-width opaque help-band row.

Layout returns one value containing both the final columns and the retained
bindings in priority order. Rendering consumes its columns; execute-and-close
admission consumes its retained bindings. A command removed by short-height
clipping is therefore neither visible nor executable through Help. Both callers
obtain the value through the same context-builder-plus-layout path, so there is
no parallel command construction or availability registry.

At two columns, the first `ceil(n/2)` commands read down the left column and the
remaining commands read down the right. This preserves the same command order
as the wide layout rather than inventing a narrow-only hierarchy. Narrow width
alone never removes a lower-priority command when enough body rows are
available; if the one-column form is still wider than the terminal, the existing
ANSI-aware ellipsis truncates the row.

Height is a separate, degenerate constraint. The overlay may use the entire
body above the status bar but still does not resize the underlying component.
If only `R` rows and `C` columns fit, the layout retains the first `R*C`
commands and drops the rest before partitioning. This preserves a prefix of the
priority sequence rather than the tail row of every column. `overlayHelp` must
also clip excess help lines from the bottom defensively; its current top-clipping
behavior is reversed. Together these rules keep core browsing commands visible
and avoid adding help pagination for an unusually small terminal. When both
dimensions constrain the grid, the popover shows the longest priority prefix
that fits the selected column count; this is the only case where narrowing the
terminal can reduce the number of visible commands.

## Interaction Model

Help is a transient navigation layer over the current view:

- `?` returns to the underlying view by toggling Help closed;
- Esc performs its consistent conceptual action, Back, returning from Help to
  the underlying view without also navigating that view's history;
- `a` closes Help and opens About through Help's dedicated route;
- any other key belonging to a displayed binding closes Help and performs that
  binding's normal action on the underlying view;
- an unrecognised key is swallowed and leaves Help open; and
- `Ctrl+C` remains an always-available force quit.

This makes `esc back` honest in the popover: when Help is the current transient
layer, the previous place is the view beneath it. It is not a special exception
to Back semantics.

"Displayed binding" applies to the action, not only the glyph shown in its help
label. Thus `n` and `N` execute next-link and previous-link when those commands
are displayed, even though their labels show `tab` and `shift+tab`. The same
rule applies to other retained aliases such as `j`/`k` for movement and `tab`
for Browse from the focused startpage input.

Action dispatch preserves the underlying screen's normal routing by re-feeding
the original `tea.KeyPressMsg`, not by implementing a second action switch. The
open-Help handler computes the current layout and tests the original message
against its retained bindings with `key.Matches`. On a match, it closes Help and
returns a `tea.Cmd` that emits that unchanged message once. Bubble Tea then
feeds it through the normal `Update` / `handleKey` path. Because Help is already
closed, this cannot recurse into the Help handler.

About is the intentional exception to re-feeding. When `a` matches the displayed
About binding, the Help handler closes Help and calls the existing `openAbout`
transition directly. Re-feeding `a` would be incorrect over a focused input,
where normal routing must type it. The dedicated transition preserves the
established `?` then `a` route without duplicating About's screen behavior.

Re-feeding structurally preserves aliases and component ownership for every
other displayed action. App-level actions such as Home reach their existing
handlers, while movement, paging, and filtering fall through to the active
Bubble Tea component exactly as they do when Help is closed. This is
particularly important for the display-only `Move` and `Page` bindings, which
advertise keys owned by the viewport or list and have no app-level action
handler to call.

The command acts on the state that was visible beneath Help, and Help is closed
in the resulting state. Enter may therefore start a request. `q quit` is an
explicit member of the re-feed rule: when Quit is displayed, pressing `q` from
Help immediately quits without confirmation. That preserves the command-map
model and the behavior of the same key on the underlying content.

## Edge Cases

- Esc closes Help but does not also step history, exit raw view, close the links
  panel, blur the input, or quit from the root startpage.
- An unrecognised printable key never types into the target input or an active
  component while Help is open.
- Disabled actions are neither displayed nor executable through Help. This is
  especially important for context-dependent link actions.
- A binding removed by short-height clipping is neither displayed nor
  executable through Help.
- The links panel continues to show only actions supported by its selected
  link. A displayed synthetic `f go` action behaves like its underlying Finger
  link action. The execution gate consumes that same inline binding returned by
  the links-panel builder; it must not construct a second synthetic binding.
- `?` while Help is open on a focused target input closes Help through the
  dedicated toggle and cannot be re-fed into the text input.
- `a` while Help is open on a focused target input opens About through the
  dedicated transition and cannot be re-fed into the text input.
- A command that starts loading closes Help before the loading state appears.
- Opening and closing Help does not change list pagination, reader scroll,
  selection, filter state, or link focus unless the user invokes a displayed
  command whose normal purpose changes that state.

## Implementation Boundaries

Keep four concerns separate:

1. a context builder returns the ordered candidate bindings for the current
   state;
2. a pure layout function chooses the column count, retains the priority prefix
   that fits, and returns both those bindings and their columns;
3. a pure renderer turns the layout columns into the responsive opaque band;
   and
4. open-state key routing decides whether to go Back, execute a displayed
   binding, force quit, or ignore the key.

The builder and layout path remains the single source for both what the popover
shows and which actions it accepts while open. The router consumes that
layout's retained binding set, including the links panel's inline synthetic
`f go` binding produced by the same builder, instead of reconstructing a list.
This prevents visual and behavioral availability from drifting apart. The
renderer does not know application semantics, and the key router does not
duplicate either the priority order or normal action handling. Its only
action-specific transition is the established Help-to-About route, required
because normal input routing intentionally gives `a` a different meaning.

The custom renderer supersedes the remaining `help.Model` state. Remove
`m.helpModel` rather than retaining a model used only as a separator holder.
Lookit continues to use Bubbles' `key.Binding`, `help.Styles`, and
`help.DefaultStyles`; it owns only the full-panel separator and responsive
layout that Bubbles' fixed-column renderer cannot provide. This records the
narrow existing divergence honestly while preserving Charm's key and style
conventions.

## Testing

Tests must cover presentation and behavior independently:

- ordinary contexts use the approved priority order after disabled bindings are
  removed;
- the links panel retains its smaller selection-aware command set;
- wide, medium, and narrow widths select three, two, and one columns without
  dropping commands solely because of width, including odd command counts that
  prove `ceil(n/c)` rows and a short last column;
- a terminal too short for all rows keeps the leading priority rows rather than
  the tail;
- every help row remains full-width and uses the existing adaptive key,
  description, and background styles;
- `?` and Esc close Help without changing the underlying view;
- `?` from Help over a focused input does not type into the target;
- direct `a` over a focused input types normally, while `?` then `a` opens About
  without changing the target;
- About is displayed and opens from every Help context, including the links
  panel;
- representative displayed app actions and delegated component actions execute
  and close Help;
- hidden aliases of displayed bindings execute and close Help;
- unrecognised keys leave Help open and do not leak into the input or content;
- disabled or context-inapplicable actions cannot execute through Help;
- bindings clipped by short height cannot execute through Help;
- `Ctrl+C` still quits; and
- opening Help continues not to repaginate lists or disturb saved view state.

Add a table-driven context matrix whose central invariant is that the binding
set retained and rendered by Help is exactly the set admitted to execute from
Help: every displayed command works, and no undisplayed ordinary command does.
It must cover the focused input, startpage, user list, reader with and without
links, raw view, and links-panel selections with different supported actions.
Most displayed commands use execute-and-close re-dispatch. Back and About are
displayed commands with dedicated transitions: Esc returns from Help without
also navigating the underlying view, and `a` opens About even when the input
beneath Help is focused. Esc continues to close Help even if a degenerate short
layout clips its row. The non-displayed `?` toggle and `Ctrl+C` force quit are
also asserted as dedicated controls outside ordinary re-dispatch.

The matrix also asserts the layer-aware availability metadata itself: About is
disabled while the input directly owns keys, becomes enabled and displayed
when Help opens, and is displayed in the links panel's Help layout; Help is
disabled where its dedicated toggle is unreachable; and Open remains enabled
for target submission while the input is focused.

Existing tests continue to guard the earlier label simplification: `g`/`G`,
`n`/`N`, and the other runtime aliases keep working even though the popover
advertises a smaller set of primary gestures.

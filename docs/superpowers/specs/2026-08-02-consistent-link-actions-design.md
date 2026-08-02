# Consistent link actions design

## Goal

Make link interaction predictable across lookit's reader, links panel, help
popover, and About screen. The keyboard vocabulary becomes a small invariant:

- **Enter goes somewhere inside lookit.** It submits a target, opens a selected
  user, or follows a definite finger target.
- **`y` copies.** Enter never copies as a fallback action.
- **`f` resolves an ambiguous `user@host` address** by explicitly treating it
  as a finger target.
- **External opening belongs to the terminal.** lookit continues to emit OSC-8 hyperlink
  metadata, but it neither opens external applications nor teaches a terminal's
  mouse gestures.

This replaces the current link-specific exception where Enter copies URL,
email, social, ambiguous, and blocked links even though Enter means “go” and
`y` means “copy” everywhere else.

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
   including copy. This is the current model. It is compact, but the same key
   means navigation in lists and clipboard mutation on many reader links.

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

The internal `ActionDrill` name does not need to change. “Drill” is an
implementation term; all user-visible copy uses **go**.

## Reader behavior and status copy

Reader link navigation remains:

- `tab` or `n`: focus the next link, wrapping at the end.
- `shift+tab` or `N`: focus the previous link, wrapping at the beginning.
- `L`: open the links panel.
- `y`: copy the focused link; with no focused link, retain the existing behavior
  of copying the current result's address.

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

Remove `⌘-click opens` from every lookit-owned surface. Keep emitting OSC-8
hyperlinks for the schemes already supported. Whether and how those hyperlinks
open is a terminal capability communicated by the terminal and its
documentation, not by lookit.

## Links panel

The links panel obeys the same action matrix as the reader. Moving the selection
changes the contextual actions; it does not alter their meanings.

- Enter closes the panel and starts a lookup only for a definite, allowed
  finger target.
- `f` closes the panel and starts a lookup only for an ambiguous `user@host`.
- `y` copies the selected row and leaves the panel open, with the ordinary
  `copied …` flash.
- Enter on a URL, email, social address, or ambiguous address leaves the panel
  open and does nothing.
- Enter on a blocked cross-relay target leaves the panel open and shows the
  existing refusal.
- `esc` or `L` closes the panel and returns to the reader with the selected link
  focused, as today.

The panel status bar is selection-aware. Its baseline navigation is:

```text
↑/↓ move · / filter · esc back
```

It appends only the selected row's actions: `↵ go` for a definite finger target,
`f go` for an ambiguous address, and `y copy` for every row. A blocked target
shows `y copy` but not `↵ go`.

## Help popover

The general action row remains:

```text
↵ go    i target    y copy    v view source
```

On a reader result containing at least one detected link, add a contextual row:

```text
tab/n next link    shift+tab/N previous link    L browse links
```

Hide the entire row when the current reader result has no detected links. Do not
put `f go` in this general row: it exists only when an ambiguous address is
focused, and the status bar advertises it at that moment.

When the links panel is open, `?` opens the help popover with panel-relevant
bindings rather than reader scrolling bindings:

```text
↑/↓ move    / filter    esc back
```

The popover also shows the selected row's live action bindings, following the
same matrix as the panel status bar. It does not advertise Enter for copy-only
or blocked rows. Disabled bindings remain the single source of truth so the
help popover and key router cannot disagree.

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

Plain or OSC-8 URLs on About receive no special lookit hint. Terminal-native
link interaction remains available without being documented by the app.

## State and implementation boundaries

This is a TUI interaction and copy change. It reuses the existing
`key.Binding` enablement model:

- derive whether Enter, `f`, and the link-navigation bindings are enabled from
  the active screen and selected link;
- use the same enabled bindings to route keys, populate the help popover, and
  assemble contextual status hints;
- keep link detection and classification separate from action dispatch;
- preserve reader link focus, scrolling, history restoration, and the links
  panel's current selection behavior.

No changes are required in `finger/`, `render/`, `DetectLinks`, link
classification, user-list parsing, network behavior, clipboard transport,
history structure, or terminal mouse handling. No configurable keymap or new
external-launch command is introduced.

## Edge cases and feedback

- A response with no links has no link-navigation row in help; link-navigation
  keys remain inert.
- A copy-only link never acquires an Enter action merely because it is selected
  in the links panel.
- `y` always copies the exact displayed `Link.Raw`, preserving current behavior.
- Failed or blocked go actions never fall back to copying.
- Copy success continues to use the existing transient `copied …` flash.
- Cross-relay refusal text remains consequence-first and does not claim a go
  action is available.
- Raw view retains no link focus or link-navigation actions.
- About remains non-focusable even though its body contains URLs and two
  commands.

## Testing

Follow the existing injected-model patterns; tests remain offline and do not
open external applications.

- **Reader dispatch:** Enter fetches definite finger targets; Enter does not
  copy or fetch copy-only and ambiguous links; `f` fetches only an ambiguous
  address; `y` copies every focused link.
- **Links panel dispatch:** the same matrix holds, including whether the panel
  closes, stays open, flashes a refusal, or copies.
- **Status copy:** no user-visible output contains `drill`, `⇥ next`, or
  `⌘-click opens`; expected rows contain `↵ go`, `f go`, `y copy`, and
  `tab next` only when applicable.
- **Help enablement:** the reader link row appears only when links exist; the
  links panel shows panel bindings and only live selected-row actions; About
  does not advertise Tab.
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

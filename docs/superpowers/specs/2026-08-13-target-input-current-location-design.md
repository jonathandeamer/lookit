# Target row owns the active address

Date: 2026-08-13

## Goal

Give each piece of navigation chrome one clear job:

- The top `target:` row is the active, editable address. It shows the current
  `Target.Raw` when settled, a draft while editing, and the destination while a
  navigation is pending. Its only settled empty state is the startpage; a user
  may also clear it while editing a draft.
- The reader begins with the response body. The duplicated
  `➜ <Raw> <elapsed> ✦` header is removed.
- The status bar keeps the structured `@host` / `@host / user` breadcrumb and
  gains the landed response's latency as expendable metadata.

There is no relocated success sparkle. A response landing without a warning is
the success signal; partial and failed responses continue to identify
themselves explicitly.

## Current behaviour and problem

Three surfaces can name a place:

| Surface | What it shows | Role |
|---|---|---|
| Top `target:` row | Last *typed* string, or empty | Persistent chrome. `i` rewrites it to the current `Target.Raw` for browser-style editing. Typed submit leaves the value in place and blurs. |
| Reader header (`renderHeader`) | `➜ <Raw> <elapsed> ✦` | First line of a rendered response. It scrolls with the body and is absent from lists. |
| Status breadcrumb | `@host` or `@host / user` | Persistent structured location. The `/` shape is the directory-vs-profile signal. |

Only the typed-submit path leaves the field agreeing with a landed screen.
Drill, startpage Enter, about → author, and history back never write it. After
`@tilde.team` then Enter on `jonathan`, the field still says `@tilde.team` while
the breadcrumb says `@tilde.team / jonathan`. That is stale state, not a taste
call.

Keeping the reader header while fixing the field would introduce a second
problem on profiles: the exact same `Raw` value would occupy two adjacent rows,
then the status bar would express it a third time as a breadcrumb. A browser's
document title earns its separate line because it differs from the URL; the
finger target is not a document title. The reader header's only unique facts are
elapsed time and the success sparkle, neither of which justifies the duplicate
row.

The old wording "address of the visible place" is also too narrow during a
request. A typed submit already displays the destination while the prior screen
remains visible, whereas a drill leaves the prior address until land. One field
should not change meaning according to how navigation began.

## Decisions

### Chosen: the field is the active navigation address

The field has four explicit states:

| State | Field value |
|---|---|
| Startpage, settled | `""`; show the syntax placeholder |
| History node, settled | Current node's `entry.Target.Raw` |
| Editing | The user's draft |
| Navigation pending | Pending request's `Target.Raw`, regardless of whether navigation began by typing, drilling, selecting a startpage row, following a link, or choosing the author from About |

`Target.Raw` is the same canonical string used by `y` copy and the `◂ esc:`
hint. Scheme and path forms have already been rewritten into it by
`ParseTarget`; the field does not preserve the originally typed `finger://` or
`host/user` spelling after a result lands.

Starting any `requestNavigate` writes the pending target immediately. This is
the browser-style behaviour the persistent address row implies and makes every
navigation origin consistent. A refresh keeps the current address because its
pending and visible targets are the same.

Cancellation depends on where the request should return:

- If `returnToInput` is true, keep the submitted value and refocus it. It is now
  a draft the user can edit or retry.
- Otherwise restore the visible history node's `Raw`, or `""` on the
  startpage, and remain content-focused.

Landing, history restore, and home always write the address of the screen they
show. Esc from a focused input on a history node restores the current
`entry.Target.Raw` and blurs, discarding the draft. Esc from the focused input
on the startpage still quits.

### Chosen: remove the reader header

`render.RenderWithBackground` begins with the response body, `(no response
body)`, or the rendered error. It no longer prepends `➜ <Raw> <elapsed> ✦`.
The header is removed rather than reduced to `123ms ✦`: a successful response
does not need a permanent badge, and latency alone does not deserve a viewport
row.

This removes `renderHeader`, its header-only theme fields, and `render.Split`.
The link overlay applies to the complete rendered response, and focused-link
scrolling no longer adds a header-line offset. Links are still detected only
from the sanitized response body, so the appended error line does not become a
new link ingress.

The render goldens become headerless. There is no current one-shot response
consumer that needs a standalone fetch receipt: `main.go` uses `render/` only
for usage, version, and startup-error text. A future one-shot query mode can add
its own outer chrome rather than forcing duplicate chrome into the TUI.

### Chosen: latency moves to the status bar; the sparkle goes away

The landed response's `Meta.Elapsed` appears as a compact segment such as
`123ms`, using the existing formatting thresholds. It applies to readers and
lists because both are fetched responses. It is absent on the startpage and
About screen. The pending bar keeps its existing live elapsed counter; that is
request progress, not landed-response metadata.

Latency is the first expendable status segment. It is shown only when it fits
without causing the breadcrumb, flags, previous-target preview, page/scroll
position, response size/count, or action hints to lose information they would
have retained in the same bar without latency. On a roomy profile bar the shape
is approximately:

```text
@plan.cat / alice        123ms · 1.2 KB · ↑↓ scroll · r refresh · ? help
```

No `✦`, `ok`, or `success` badge is added elsewhere. Success is the default:
loading ends and the response lands. Exceptional outcomes already have the
right, consequence-first signals:

- `partial (truncated)` and `partial (error)` flags for usable partial bodies;
- a rendered error for a failed navigation;
- the persistent refresh/retry warning when the prior response remains visible.

The old sparkle was based on `queryErr == nil`, which could coexist with a
truncated response. Removing it avoids presenting celebration and caution at
the same time.

### Chosen: keep the structured breadcrumb

The status breadcrumb remains `@host` / `@host / user`. It is not another
editable address: it interprets the current target structurally and anchors the
response metadata, navigation preview, honesty flags, and controls in the
bottom bar. The top field keeps the exact parseable `Raw`; it does not grow a
`/`, and the breadcrumb does not become an input.

Host lists will show `@host` in both places. That local repetition is accepted
because the roles remain stable across every screen: exact/editable address at
the top, structured status context at the bottom. It is preferable to making
the bottom bar change shape only for host responses.

### Rejected: clear the field on land (compose box)

Clearing on land and pre-filling only when `i` is pressed would stop stale
values but invent a model the code does not have. Submit already keeps the
value, `i` is browser-style editing of the current address, and `gotoStart`
already treats a leftover target as stale because the user left that place.

An empty blurred field over content would also show `user@host or @host` on
every landed screen: startpage teaching copy in the one situation where the
user has already used the syntax. It would look ready for typing while still
requiring `i` to accept input.

### Rejected: hide the row when blurred

The input is persistent top chrome
(`docs/superpowers/specs/2026-05-30-tui-idiomatic-navigation-design.md`). Hiding
it on land would replace that model with a vim-style transient command line.

### Rejected: relocate the success sparkle

A transient sparkle is easily missed; a persistent one consumes scarce status
width; a textual success badge is heavier still. None adds information once a
response has visibly landed. Warning-only status is quieter and more honest.

## The rule, in transitions

Write the pending address from the request target and the settled address from
the screen being shown. Do not derive either from `m.pos` at an arbitrary
moment: `landNavigation` calls `showRouted` before `push`, so `pos` still names
the previous node while the new screen is being installed.

| Transition | Field becomes |
|---|---|
| Start any navigation request (typed submit, list drill, startpage Enter, About → author, link finger) | Pending `Target.Raw` immediately |
| Start a refresh/retry | Current `Raw` (the pending target is the same) |
| Land a navigation, including an error response | Landed `entry.Target.Raw` |
| Land a refresh | Refreshed node's `Raw` |
| Cancel a request with `returnToInput` | Keep submitted target, refocus; it becomes a draft |
| Cancel a content-originated request | Restore visible node's `Raw`, or `""` on the startpage |
| `restore` / history back onto a node | That node's `Raw` |
| `gotoStart` / Home / back off `pos == 0` | `""` |
| `i` (`focusInput`) | Current `Raw` if `pos >= 0`; empty on the startpage |
| Esc, input focused, `pos >= 0` | Current `Raw`, then blur |
| Esc, input focused, `pos == -1` | Unchanged; quit |
| Parse error on submit | Unchanged; stay focused |
| Open or close About, help, view-source, links panel | Unchanged; none is a navigation by itself |

`showRouted` already blurs before a landed screen appears. Landing therefore
ends with a settled address even when the request began while the input was
focused. Stale or canceled result messages remain unable to rewrite the field
because request IDs are checked before routing.

Suggested shape, not a required API: helpers for setting the address from a
target/current screen and for restoring the visible address on cancellation,
called from request start, `showRouted`, `restore`, `gotoStart`, and
Esc-from-input.

## What does not change

- Prompt, placeholder copy, `CharLimit`, and which keys focus or blur.
- History nodes, routing, startpage assembly, fetch, sanitize, and port-79
  pinning.
- The breadcrumb and the status bar's existing flags, page/scroll information,
  byte/user metadata, previous-target preview, priority warnings, and hints.
- About, help, view-source, and the links panel as navigation states.
- The honesty convention: the field shows a real `Raw`, never an invented kind.

## Testing

All tests are model or pure-renderer tests with injected fetches. No network or
TTY.

Pin at least:

- Starting navigation from a startpage selection, list drill, reader link, and
  About → author immediately writes the pending `Target.Raw`.
- Canceling a content-originated navigation restores the visible node's `Raw`;
  canceling one from the startpage restores `""`.
- Canceling an input-originated navigation keeps the submitted value, focuses
  the input, and a following Esc restores the visible node's `Raw` and blurs.
- Landing navigation writes the landed `Target.Raw`; refresh keeps the current
  raw; history back restores the previous raw; Home/back-to-start clears it.
- Parse failure preserves the focused draft and starts no request.
- Opening or closing About/help/view-source/links without navigating leaves the
  field unchanged.
- Reader and list status bars show formatted response latency at roomy widths.
- A narrow status bar drops latency before existing location, warning,
  navigation, metadata, or action information.
- Render goldens begin with the response body (or empty/error treatment), with
  no synthetic leading `➜ <Raw> <elapsed> ✦` line.
- Link overlay can highlight a link on the first response line, and focusing it
  scrolls without a phantom header offset.
- Partial/truncated/error status remains visible and no success marker appears.

`TestTargetPlaceholderSuggestsNoDestination` stays: the placeholder still
names no destination. It simply appears only on the startpage or in a
user-cleared draft.

## Files

- Modify: `tui/app.go` and `tui/request.go` — keep the field synchronized with
  pending, settled, restored, and canceled navigation.
- Modify: `tui/statusbar.go` — add low-priority landed latency formatting and
  placement.
- Modify: `tui/reader.go` — apply link overlay and focused-link scrolling to a
  headerless rendered response.
- Modify: `render/render.go` and `render/theme.go`; remove `render/chrome.go` —
  remove the response header, `Split`, and header-only styles.
- Update: renderer goldens and tests in `render/`; model/status/link tests in
  `tui/`; architecture text in `CLAUDE.md` that still describes the reader
  header or `render.Split`.
- No changes in `finger/`, list parsing, bookmarks, or catalog assembly.

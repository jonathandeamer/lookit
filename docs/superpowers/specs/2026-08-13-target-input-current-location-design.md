# Target input shows the current location

Date: 2026-08-13

## Goal

The top `target:` row displays the address of the screen you are looking at.
On a history node that is `Target.Raw`. On the startpage it is empty. The
reader header and the status-bar breadcrumb keep the jobs they already have.

This finishes a rule the chrome already half-implements. It is not a new
address-bar concept, and it does not destutter the rest of the chrome.

## Current behaviour and problem

Three surfaces can name a place:

| Surface | What it shows | Role |
|---|---|---|
| Top `target:` row | Last *typed* string, or empty | Persistent chrome. `i` rewrites it to the current `Target.Raw` “for browser-style editing.” Typed submit leaves the value in place and blurs. |
| Reader header (`renderHeader`) | `➜ <Raw> <elapsed> ✦` | First line of the rendered response. Scrolls with the body. Lists have none (`SetShowTitle(false)`). |
| Status breadcrumb | `@host` or `@host / user` | Persistent “where you are.” The `/` shape is the directory-vs-profile signal. |

`gotoStart` already treats a leftover value as stale and clears it, so `i` on
the startpage opens an empty row. The placeholder decision
(`docs/decisions/2026-08-12-target-input-teaches-syntax.md`) names only two
empty states: the startpage, and a field the user cleared mid-session. Empty
is not specified as the resting state of a landed page.

The gap is that only the typed-submit path leaves the field agreeing with the
visible place. Drill, startpage Enter, about → author, and history back never
write it. After `@tilde.team` then Enter on `jonathan`, the field still says
`@tilde.team` while the header and breadcrumb say you are on
`jonathan@tilde.team`. That is a lie, not a taste call.

`focusInput` papering over it on `i` proves the intended value is the current
`Raw`. The field just is not kept there while it is blurred.

## Decisions

### Chosen: the field is the address of the visible place

Whenever the visible screen is a history node (`pos >= 0`) and the input is
not holding a draft, `input.Value()` equals that node's `entry.Target.Raw`.
Whenever the visible screen is the startpage (`pos == -1`) and the input is
not holding a draft, the value is empty and the syntax placeholder shows.

`Target.Raw` is the same string the header, `y` copy, and `◂ esc:` hint
already use. Scheme and path forms have already been rewritten into it by
`ParseTarget`; the field does not try to preserve the originally typed
`finger://` or `host/user` spelling.

A draft is the only in-flight exception: keys must not rewrite the field
while the user is editing it. Land, restore, and home are not in-flight —
they change the visible place, so they write the new address (and
`showRouted` blurs first). Esc from a focused input on a history node
restores `history[pos].entry.Target.Raw` and then blurs — it does not leave
a half-edit sitting in the bar. Esc on the startpage still quits when the
input is focused and `pos == -1`.

This is the rule `gotoStart` and `focusInput` already speak. Drill and back
learn it; typed submit already satisfies it.

### Rejected: clear the field on land (compose box)

Clearing on land, and pre-filling only when `i` is pressed, would stop the
lie and keep the header unique. It also invents a model the code does not
have. Submit already keeps the value. `i` is documented as browser-style
editing of the current address. The placeholder decision treats empty as
startpage (or a user-cleared draft), not as “you are reading a page.”
`gotoStart` calls a leftover target stale *because you left that place*,
which only makes sense if being on a place means the field names it.

An empty blurred field over content would also show `user@host or @host` on
every landed screen — startpage teaching copy in the one situation the user
has already used the syntax. That is a stronger “type here” signal than a
filled, blurred value, and it still cannot accept keys until `i`.

### Rejected: destutter the reader header

If the field always shows `Raw`, the header's `t.Raw` is the same string one
row below. That is real duplication. It is not a reason to change the
header.

The header is response chrome, not app chrome. It lives in `render/`, carries
elapsed time and the success sparkle, scrolls with the body, is skipped by
link overlay on purpose, and is absent from lists. Moving elapsed onto the
status bar would crowd a line that already truncates hints, flags, and
`◂ esc:`. Dropping `Raw` from the header to justify the field is solving a
problem this spec does not need to create.

A browser has a URL, a document title, and a status bit. lookit can have a
target row, a fetch receipt, and a breadcrumb. They may name the same place.

### Rejected: hide the row when blurred

The input is persistent top chrome
(`docs/superpowers/specs/2026-05-30-tui-idiomatic-navigation-design.md`).
Hiding it on land would be a different product: vim's `:` line, not the bar
that spec put at the top. Out of scope.

## The rule, in transitions

Write the field from the *visible* place, not from `m.pos` at an arbitrary
moment. `landNavigation` calls `showRouted` before `push`, so `pos` is still
the previous node when the new screen appears. The writer takes the raw
string of the screen being shown.

| Transition | Field becomes |
|---|---|
| Land a navigation (typed submit, list drill, startpage Enter, about → author, link finger) | Landed `entry.Target.Raw` |
| Land a refresh | Current node's `Raw` (usually unchanged; still write it) |
| `restore` / history back onto a node | That node's `Raw` |
| `gotoStart` / Home / back off `pos == 0` | `""` (already) |
| `i` (`focusInput`) | Current `Raw` if `pos >= 0`; already empty on the startpage |
| Esc, input focused, `pos >= 0` | Current `Raw`, then blur |
| Esc, input focused, `pos == -1` | Unchanged; quit (already) |
| Parse error on submit | Unchanged; stay focused (already) |
| Open or close About, help, view-source, links panel | Unchanged — none of these are locations |
| Request in flight | Unchanged. The prior screen stays visible (`2026-08-02-request-controls-design.md`); the spinner already names the pending target. Typed submit has already put the new address in the field; drill leaves the visible list's address until land. Cancel restores focus only when the request was started from the input (`returnToInput`); the value is whatever was submitted. |

A draft is only live while the input is focused *and* no land/restore/home
is running. Those three paths own the visible place, so they write
unconditionally. `showRouted` already blurs before the new screen appears;
the write happens after that blur, so a request started from the input
(typed submit, or about → author while the field still had focus) still
ends on the landed `Raw`, not on a leftover draft. `restore` and `gotoStart`
cannot run while the input is focused: Esc in that state cancels the edit
instead of stepping back, and `h` is content-only.

Esc restore is the one write that happens *while* focused: it puts `Raw`
back and then blurs.

Suggested shape, not a required name: one helper that sets the value from a
raw string and is called from `showRouted`, `restore`, and `gotoStart`.

## Interaction with the header and the breadcrumb

No change.

- The field is the address you will edit. It is a target that `ParseTarget`
  will accept on Enter.
- The header is this response: `➜`, `Raw`, elapsed, sparkle. It stays in
  `render.RenderWithBackground`. Goldens stay.
- The breadcrumb is structured place. It stays `@host` / `@host / user`. The
  field does not grow a `/` and the breadcrumb does not become a second input.

Lists gain the most: they have no header, so a startpage drill to `@tilde.team`
currently leaves the top row empty. After this, the row says `@tilde.team`
and the breadcrumb says the same host without a user. That is agreement, not
a new signal.

## What does not change

- Prompt, placeholder copy, `CharLimit`, and which keys focus or blur. Esc
  from a focused input still cancels the edit rather than stepping back; it
  now also restores `Raw` instead of leaving the draft.
- History, `histNode`, startpage assembly, fetch, sanitize, port pinning.
- `render/` (header, goldens, `Split`, link-overlay skip of the header line).
- Status-bar layout and `breadcrumbParts`.
- About, help, view-source, links panel.
- The honesty convention: the field shows `Raw`, never an invented kind.

## Testing

Model tests in `tui/`, same injected-fetch style as the rest of `app_test.go`
and `request_test.go`. No network, no TTY.

Pin at least:

- Startpage Enter onto a catalog host → after the result lands, `input.Value()`
  equals the landed `Target.Raw`.
- List drill onto `login@host` → field equals that target, not the list's
  `@host`.
- History back from the drilled profile → field returns to the list's `Raw`.
- Back (or `h`) onto the startpage → field is empty.
- Typed submit of `alice@plan.cat` → field still `alice@plan.cat` after land
  (regression of today's typed path).
- `i`, edit to something else, Esc → field is the current `Raw` and the input
  is blurred; no fetch.
- About open and close over a landed node → field unchanged.
- Refresh of a landed node → field still that node's `Raw`.
- Reader view of a landed profile still contains the header's `Raw` (the
  header is not a casualty of the field).

`TestTargetPlaceholderSuggestsNoDestination` stays: the placeholder still
names no destination. It simply appears less often, because a landed screen
no longer leaves the field empty.

## Files

- Modify: `tui/app.go` — write the field in `showRouted` / `restore` /
  Esc-from-input; `gotoStart` already clears.
- Test: `tui/app_test.go` (and `tui/request_test.go` if a refresh case fits
  more naturally there).
- No changes in `finger/`, `render/`, `tui/statusbar.go`, or `tui/reader.go`.

# Request controls design

Date: 2026-08-02

## Goal

Give every finger request an explicit, cancellable lifecycle and add an honest
refresh/retry action without weakening lookit's history model.

The user can:

- cancel a slow request and return to exactly where it started;
- refresh a landed reader response or user list without adding a duplicate
  history entry;
- retry an empty-body failure in place; and
- tell when a refresh failed and the visible response is therefore the older
  one.

The prior screen remains visible while work is in flight. Loading is a modal
interaction rather than a new screen or history node.

## Current behaviour and problem

`appModel` currently represents an active request with separate `loading`,
`loadingTarget`, and `reqSeq` fields. `startFetch` passes
`context.Background()` to `fetchCmd`, so the TUI cannot cancel a dial or
blocking read even though `finger.Query` already observes context cancellation
and closes its connection.

The request ID prevents a superseded result from landing, but there is no user
action that cancels the underlying work. There is also no refresh or retry
action. Re-submitting the same address is treated as new navigation and pushes
another history node.

These gaps are most visible on the unreliable small internet: an unreachable
host can leave the user waiting for the network deadlines, and a failed attempt
to refresh old content cannot be distinguished from a current response without
leaving the screen.

## Decisions and alternatives

### Chosen: typed pending-request state

Represent the one active request with a single value containing its identity,
target, intent, start time, and cancel function. The intent is either
navigation or refresh:

- navigation results push a history node;
- refresh results replace the current node; and
- cancellation applies to either intent without landing a result.

This makes request semantics explicit and gives update routing, status copy,
history mutation, and cancellation one source of truth.

### Rejected: extend the existing booleans

Adding a cancel function and a `refreshing` boolean would reduce the immediate
diff, but request meaning would remain distributed across `startFetch`,
`Update`, `routeFetch`, the status bar, and history mutation. The resulting
combinations of booleans would be harder to keep valid than one bounded request
value.

### Rejected: loading and failures as screens

Dedicated loading/error screens provide maximum visibility, but they replace
useful content, complicate back navigation, and conflict with the chosen rule
that the prior screen remains visible. Neither loading nor a failed refresh is
a visited finger location, so neither belongs in history.

## User interaction

### Refresh and retry

Add `r` as an app-level `Refresh` binding.

- On an idle reader response or user-list screen, `r` refreshes the current
  target.
- On a landed reader node whose request failed with an empty body, `r` retries
  the target.
- After an in-place refresh fails with an empty body, `r` retries that refresh.

The binding is available only on the main reader and list screens. It is
disabled while the target input is focused and in About, help, source view,
and the links panel. Consequently, `r` remains ordinary input text in target
and filter fields. A transient view must be closed before refreshing the
underlying response.

The help description is contextual: `r refresh` for ordinary landed content
and `r retry` for an empty-body error or a persistent failed-refresh warning.

There are no automatic retries and refresh always sends the same ordinary
query. It never adds the RFC 1288 `/W` token or changes the target.

### Modal loading

Every active request is modal. The screen that initiated it remains visible,
but only these keys act:

- `Esc`: cancel the request;
- `q`: cancel and quit; and
- `Ctrl+C`: cancel and quit.

All other key presses are swallowed while loading. In particular, the user
cannot move or filter a list, scroll the reader, open help/About/source/links,
edit the target, navigate history, or initiate a second request. Window-size,
colour-profile, background-colour, spinner-tick, fetch-result, and session-
cancellation messages still update normally.

The loading status bar suppresses ordinary breadcrumb metadata and hints:

```text
⠋ loading alice@host · 3s · esc cancel · q quit
```

Elapsed time appears after the first whole second and advances on the existing
spinner ticks. `Ctrl+C` remains an unadvertised universal escape hatch.

### Cancellation outcome

Cancellation clears the pending request before invoking its cancel function.
An eventual result from that request therefore has no matching pending ID and
is discarded, including a partial body returned alongside `context.Canceled`.
User cancellation never renders a cancellation error or creates/replaces a
history node.

- Cancelling the first lookup returns to the focused target input with the
  typed or command-line-seeded address preserved.
- Cancelling a target submitted from the input over existing content restores
  that focused input and preserves the edited address.
- Cancelling navigation from a landed screen restores that exact screen.
- Cancelling a refresh leaves the current node and its view state unchanged.
- Cancelling a retry restores any persistent failure warning that the loading
  bar temporarily masked.
- Quitting cancels the request before returning `tea.Quit`.

No cancellation flash is necessary: removing the loading status and restoring
the origin screen is the feedback.

## Atomic refresh semantics

A refresh operates on `history[pos]` and never appends a node, truncates the
forward tail, or changes `pos`.

Classify the result as follows:

| Result | Outcome |
|---|---|
| `Err == nil`, including an empty body | Build and replace the current node |
| Non-empty sanitized body with an error and/or truncation | Build and replace the current node; retain the existing partial/error flags |
| `Err != nil` and empty body | Preserve the current node and record a persistent failed-refresh warning |

The non-empty-body rule deliberately retains the current useful-partial-body
behaviour: a timed-out or reset response can still open a reader or parseable
list. A clean empty response is also a legitimate new response and therefore
replaces the node.

Retrying an already landed empty-body error uses refresh intent. If it fails
again with an empty body, the existing error node remains in place and the
persistent warning records the new retry error. If it succeeds or returns a
non-empty partial body, it replaces that error node.

### Preserving view context

Before refresh starts, capture the current node's live view state. When the
new response routes to the same screen type, restore useful context by
identity:

- reader: retain the numeric scroll offset, clamped by the viewport, and retain
  the focused link only when a new detected link has the same `Raw` value;
- list: retain the applied filter text and select the same item by explicit
  target when present, otherwise by login; if it no longer exists or is not
  visible under the restored filter, use the first item accepted by that
  filter.

Do not preserve a numeric list index or link index across changed content.

If the refreshed body changes routing from reader to list or list to reader,
start the new screen at its natural default: top of reader, no focused link, or
the first list item with no filter. The content has changed shape, so old view
state has no honest equivalent.

## Persistent refresh-failure warning

An empty-body refresh failure must not use the ordinary two-second flash. Once
that flash disappeared, an old response would look current.

Store a separate optional failure value containing the operation (`refresh`
or `retry`), target, and error. The existing landed `Entry` remains untouched.
The status bar replaces its ordinary hints with consequence-first copy and
includes the concise error when width permits:

```text
refresh failed: read timed out · showing previous response · r retry
```

For an error node being retried, use `retry failed` rather than claiming that
a previous successful response is visible.

The warning survives reader scrolling and opening/closing help. A new request's
loading bar masks it without deleting it, so cancelling that request restores
the exact prior warning. Clear the warning when:

- a request lands successfully or with a usable body;
- a navigation request lands a new error node;
- the user navigates back;
- the target input is focused; or
- the user leaves main content for About, source view, or the links panel.

If a retry fails again with an empty body, replace the stored warning with the
new error. The user never sees a stale failure from an earlier attempt.

## State and architecture

### Request state

Add explicit request types in `tui/app.go` (or a focused `request.go` if the
implementation makes `app.go` materially clearer):

```go
type requestIntent uint8

const (
    requestNavigate requestIntent = iota
    requestRefresh
)

type pendingRequest struct {
    id            uint64
    target        finger.Target
    intent        requestIntent
    retry         bool
    returnToInput bool
    started       time.Time
    cancel        context.CancelFunc
    view          *refreshViewState
}

type requestFailure struct {
    retry  bool
    target finger.Target
    err    error
}
```

`appModel` replaces `loading` and `loadingTarget` with
`pending *pendingRequest` and adds `requestFailure *requestFailure`. `reqSeq`
remains the monotonically increasing request ID. A nil `pending` value is the
only idle state; there is no separate loading boolean that can disagree with
it.

`retry` preserves the operation label while the warning is masked by loading.
`returnToInput` records that navigation was submitted from the target input;
`cancelRequest` uses it to restore focus without reconstructing input state.
`view` is populated only for refresh intent and captures the reader/list state
when the request starts. It must not be reconstructed when the result lands,
because terminal resize messages remain live during modal loading and can
otherwise alter the state that should be restored into refreshed content.

The state invariant is fixed: at most one pending request, and its fields are
stored together.

### Session context

Pass the `ctx` already accepted by `tui.Run` into `appModel` through
`commonModel` or an equivalent constructor argument. Tests that call `newApp`
use `context.Background()`.

`Init` starts a command waiting for session-context cancellation. When it
fires, `Update` cancels any pending request and quits. Each request uses
`context.WithCancel(sessionCtx)`, so both the UI cancel action and session
shutdown interrupt `finger.Query`. The networking package already propagates
context cancellation to a blocking connection; no `finger/` change is needed.

### Starting and cancelling

Replace `startFetch` with a single helper such as
`startRequest(target, intent)`. It:

1. defensively cancels and invalidates any existing pending request;
2. leaves any persistent request failure stored but masked by loading;
3. increments `reqSeq`;
4. derives a cancellable child context from the session context;
5. stores the complete pending request; and
6. returns the existing batch of `fetchCmd` and spinner tick.

The modal keymap means a second request cannot normally start through the UI;
the defensive cancellation keeps the helper correct for programmatic and
future callers.

`cancelRequest` first clears `pending`, then calls its saved cancel function.
Clearing first closes the race where a prompt cancellation result could be
accepted as current.

### Landing a result

`fetchResultMsg` is accepted only when `pending != nil` and its request ID
matches `pending.id`. On a match, save the intent, cancel the child context to
release resources, and clear `pending` before routing the entry.

Split the current `routeFetch` responsibility into two bounded operations:

1. Build a `histNode` from an `Entry`: detect links, decide whether a host
   response opens a list, parse it, and cache its reader/list metadata.
2. Apply the node according to request intent: push for navigation or replace
   `history[pos]` for refresh, then construct the active reader/list model.

An empty-body refresh error stops between these operations: it records the
persistent warning and leaves the existing node and sub-model untouched.

This keeps `routeFetch` as the single parsing/routing policy even though the
history disposition differs. `readerModel`, `listModel`, `finger.ParseTarget`,
`ParseUsers`, and `DetectLinks` do not acquire request-lifecycle
responsibilities.

## Keymap, status bar, and help

Add `Refresh key.Binding` to `keyMap`, keyed by `r`. `updateKeymap` sets its
help dynamically with `SetHelp("r", "refresh")` or `SetHelp("r", "retry")`.

Idle resting hints become approximately:

```text
reader:  ↑↓ scroll · r refresh · esc back · ? help
list:    ↵ go · / filter · r refresh · esc back · ? help
error:   r retry · esc back · ? help
```

Existing breadcrumb-based omission of redundant `esc back` copy still applies.
Width truncation continues to be owned by `statusBar.render`.

When `pending != nil`, key routing handles loading before help, About, input,
filter, links-panel, raw-view, or ordinary content branches. Back, Quit, and
ForceQuit are enabled; every other app binding is disabled. Non-key messages
continue through `Update` normally.

The full help panel includes Refresh only where it is enabled. Help is not
available during loading because the loading bar already advertises every
ordinary live action.

## Error handling

- Explicit UI cancellation is control flow, not a landed error.
- Session cancellation cancels work and exits rather than landing an error.
- A navigation request that genuinely fails keeps current behaviour: it lands
  an error reader node, and `r retry` is then available.
- A body-bearing error remains useful content and continues to carry honest
  partial/error status.
- An empty-body refresh/retry failure never mutates the landed `Entry`.
- Error strings remain the wrapped networking errors already produced by
  `finger.Query`; humanising the whole error taxonomy is outside this change.

## Testing

Tests use injected `FetchFunc` values and never access the network. Blocking
fakes wait on `ctx.Done()` so cancellation is observable and deterministic.

### Request lifecycle

- Starting navigation and refresh creates a pending request with the correct
  intent, target, ID, and cancellable context.
- `Esc` cancels dial/read work and an eventual result for that ID is ignored.
- Cancelling the first lookup restores the focused input and preserves its
  typed or seeded value.
- Cancelling a target submitted over existing content restores the focused
  input and its edited value.
- Cancelling a drill or refresh preserves the exact history node, state, and
  sub-model view state.
- Cancelling a retry restores the persistent warning that preceded it.
- `q`, `Ctrl+C`, and session cancellation cancel before quitting.
- Starting a request defensively cancels a prior request.
- A matching result clears pending state; stale, superseded, and cancelled IDs
  cannot land.

### Modal routing and chrome

- While pending, movement, paging, filtering, help, About, input, links,
  source, back-navigation, refresh, and open/drill keys do nothing.
- Window-size, colour-profile, background-colour, spinner, and matching result
  messages continue to work.
- Loading status contains spinner, target, elapsed time after one second,
  `esc cancel`, and `q quit`.
- Help and resting status show `r refresh` or `r retry` only where live.
- In input and filter modes, `r` reaches the text component as a literal.

### Navigation and refresh disposition

- Navigation success and navigation failure each push exactly one node.
- Refresh success replaces exactly one node with history length, cursor, and
  forward tail unchanged.
- A clean empty response replaces the node.
- A non-empty body with error/truncation replaces the node and retains honest
  flags/routing.
- An empty-body refresh error preserves the exact entry and displays a
  persistent failed-refresh warning.
- Repeated failed retries update the warning without mutating the old entry.
- Successful retry replaces an existing empty-body error node in place.
- Reader-to-list and list-to-reader refreshes reset incompatible view state.

### View-state preservation

- Reader scroll offset is restored and clamped.
- Reader link focus follows an equal `Raw` link and clears when absent.
- List filter text is restored.
- List selection follows explicit target or login identity and falls back to
  the first filtered item when absent.
- Numeric link and list indexes are not reused across changed content.

### Warning lifecycle

- The warning survives scrolling and a help open/close cycle.
- It is masked during a request and reappears if that request is cancelled.
- It clears on a usable result, a landed navigation result, back navigation,
  target editing, About, source view, and links panel.
- A subsequent failure shows the latest error, not stale copy.

The final verification gate is `make check`.

## Non-goals

- Reader search.
- Automatic retries or fallback queries.
- RFC 1288 `/W` verbose querying.
- User-configurable connect/read timeouts.
- Connection-phase reporting beyond target, spinner, and elapsed time.
- Concurrent requests.
- Continued content interaction while loading.
- Background work, persistence, subscriptions, or refresh-all.
- New loading, error, or confirmation screens.
- Changes to sanitization, response limits, timeout values, target parsing,
  forwarding, or server-supplied port-79 pinning.

## Accepted trade-offs

- Modal loading temporarily prevents reading or moving through the visible
  prior response. This is deliberate: it makes `Esc` cancellation unambiguous
  and prevents view state from changing underneath an in-flight refresh.
- Preserving reader offset rather than locating an exact text anchor may shift
  the visible passage when earlier lines change. It is predictable, bounded,
  and cheaper than inventing fuzzy document anchors.
- A refresh may change a list into a reader or vice versa because routing is
  derived from the new body. Resetting view state in that case is more honest
  than forcing the old presentation.
- Network errors can be long and will truncate in the one-line status bar.
  The consequence and retry action take precedence over showing every detail.

# Failed-request status bar

Issue: #76

## Context

When a request fails and no response body arrives, the reader's status bar keeps
reporting facts drawn from a response that does not exist. At 45 columns:

```
◂ esc: trunc@127.0.0.1:2479 · 0 B · ↑↓ scrol…
```

`buildStatusBar`'s `stateReader` branch sets `bar.meta = formatBytes(len(node.
entry.Body))` and appends a scroll percentage whenever the viewport overflows,
then builds hints as `↑↓ scroll · r retry · …`. On a failed request all three
are wrong or useless:

- `0 B` presents a failed dial as an empty-but-successful response. lookit is
  otherwise careful to distinguish outcomes — `partial (truncated)`,
  `auto-detected` — and this line quietly erases one.
- The scroll hint advertises movement on a body of two error lines.
- `r retry`, the only useful action on the screen, is the last item in the right
  group and so the first thing truncation removes.

The bar's own priority machinery is not the problem. The right group is
truncated as one string, so a byte count and a scroll hint that should not be
there are consuming the width `r retry` needs.

## Decision

Make the reader branch's contents depend on whether a response actually landed.
When the current node carries an error and an empty body:

- omit `bar.meta` entirely — no byte count;
- omit `bar.scroll` — no scroll percentage;
- set hints to `r retry` plus the usual trailing `esc back` / `? help` from
  `joinHints`.

Dropping the two false fields is what makes `r retry` fit; no new priority rule
is added. At 45 columns the line becomes:

```
◂ esc: trunc@127.0.0.1:2479 · r retry · ? help
```

The condition is the one lookit already uses to decide that `r` means *retry*
rather than *refresh*: `entry.Err != nil && len(entry.Body) == 0`, spelled out
today inside `appModel.shouldRetry`. It is extracted to a named predicate on
`Entry` — `func (e Entry) failed() bool` — and both call sites use it, so the
bar and the retry keybinding cannot disagree about whether a response landed.

A response that arrived *and* errored (a truncated read, the common
reset-after-body case) is not a failure by this rule: bytes exist, the body is
scrollable, and the byte count and scroll percentage stay, as does the existing
`partial (truncated)` flag.

### A truncated read that delivered nothing

`Meta.Truncated` does **not** imply bytes: `finger.queryWith` sets it on a read
deadline even when the server accepted the connection and then said nothing at
all, leaving an empty body (`TestQuery_ReadDeadline` covers exactly this). Such
an entry is `failed()` by this rule, so it loses the `partial (truncated)` flag
along with the byte count and the scroll hint.

That is the intended outcome, not an oversight. "partial" claims part of a
response arrived; with zero bytes, none did, and the flag would assert the same
false thing `0 B` does. The error line already says what happened — `<addr>
stopped responding after 30s` — and `r retry` is the same next action. A test
pins this so the behaviour is a decision rather than a side effect of the early
return.

## Latency stays

`bar.latency` is not removed. The elapsed time is a true measurement of the
failed attempt — how long lookit waited before giving up — which is exactly the
fact a user wants after a timeout. It is also already the lowest-priority field:
the bar includes it only when the whole line fits, so it can never displace
`r retry`.

## The list branch is unaffected

`stateList` is only reached with a parseable body, so a node there always has
bytes; its `partial (error)` flag already covers the errored-but-parsed case.

## What does not change

- The refresh-failure priority status (`refresh failed: … · showing previous
  response · r retry`), which is the *stale-response* case and keeps its own
  treatment.
- The loading priority status, the breadcrumb, flags, pagination, the raw and
  links views, and every non-reader branch.
- The error text itself, which is issue #75.

## Testing

- A reader node with an error and no body produces a bar with no byte count, no
  scroll percentage, and `r retry` present.
- The same bar rendered at 45 columns still contains `r retry` in full.
- A reader node with a body and an error keeps its byte count, scroll
  percentage, and `partial (truncated)` flag.
- A successful reader node's bar is byte-for-byte unchanged.
- `shouldRetry` keeps its current behaviour through the extracted predicate,
  including the `requestFailure` case.

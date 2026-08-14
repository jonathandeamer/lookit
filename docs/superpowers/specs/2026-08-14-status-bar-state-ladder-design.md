# Status bar state ladder

**Status:** blocked. Stacked on `statusbar-hint-priority`; do not implement
until rules 1 and 2 from
[`2026-08-14-status-bar-hint-priority-design.md`](2026-08-14-status-bar-hint-priority-design.md)
have landed on `main`.

## Context

Visual review items 19 and 20 (`The Startpage Keyhole`, 13 August 2026) both
describe the status bar clipping. Item 19's `0 B` and phantom scroll hint were
fixed by #83; item 20 — the breadcrumb clipping at 100 columns while forty rows
sit empty — is claimed by the hint-priority spec, which expects the breadcrumb
to reclaim width as a consequence of dropping hints.

It does, at 60 columns and above. It does not below that, and the mechanism it
uses to get there spends the wrong currency. Measured by simulating rule 2 over
the real bar states at 45/60/80/100:

| Width | State | Rendered after rules 1+2 | Dropped |
|---|---|---|---|
| 45 | failed reader | `@127.0.… ◂ esc: trunc@127.0.0.1:2479 · ? help` | `r retry` |
| 60 | failed reader | `@127.0.0.1 / nobody   ◂ esc: trunc@127.0.0.1:2479 · 1ms · ? help` | `r retry` |
| 60 | reader | `@127.0.0.1 / alice   ◂ esc: @127.0.0.1 · 2ms · 1.2 KB · ? help` | `↑↓ scroll`, `r refresh` |
| 100 | reader, link focused | `… ◂ esc: @127.0.0.1 · 2ms · 1.2 KB · link 1/2 · URL · ↵ go · y copy · tab next` | `r refresh` |

Two problems, one cause. The right group's **state** segments — `latency`,
`meta`, and the `◂ esc: <target>` destination — are never candidates for
removal, so every width shortfall is paid for out of actions, and once the
actions are gone it is paid out of the address.

At 45 columns the address is still clipped in all three reader states, because
`◂ esc: trunc@127.0.0.1:2479` alone is 27 cells and hints are already down to
`? help`.

## Decision

Extend the drop order past the hints and into the state segments. Lowest value
first:

1. `latency` (`2ms`) — already has an all-or-nothing fit test in `render`;
   this generalises it rather than inventing a mechanism.
2. `meta` (`1.2 KB`, `3 users`)
3. `page` / `scroll` (`page 2/4`, `42%`)
4. `◂ esc: @127.0.0.1` degrades to bare `◂ esc` — keeps the affordance, drops
   the destination, buys 12–22 cells
5. `◂ esc` drops entirely
6. breadcrumb truncates (last resort)
7. hints truncate (only if 6 was not enough)

Hints are consumed by rule 2 before this ladder starts, so the two compose:
rule 2 decides which hints survive, the ladder decides what the survivors cost.
Honesty flags keep their existing reservation ahead of the hints; that
invariant is untouched.

## Conflict to resolve first: rule 2 versus issue #76

Rule 2 pins `? help` by value with no state exception. On a failed request that
drops `r retry` at 60 columns and below — exactly the outcome #76 was filed
about and #83 fixed by giving retry the width. `? help` on a screen whose only
useful action is retry is the same category of error as the `0 B` that #83
removed: the bar spending scarce width on its least useful fact.

Suggested resolution, for the hint-priority branch to own rather than this one:
pin the refresh/retry hint alongside `? help` when `node.entry.failed()`, or
rank `? help` below it in that state.

## Scope

`tui/statusbar.go` (`render` and `rightParts`), `tui/statusbar_test.go`. No
change to `tui/app.go` — the ladder reads struct fields `buildStatusBar`
already sets. No change to the help overlay, the keymap, `finger/`, `render/`,
or any user-facing string.

Not in scope: review finding 21 (the bar is right-aligned on an otherwise
left-aligned page). Separate taste call, separate decision.

## Testing

Assert rendered output at 45/60/80/100 across the failed reader, plain reader,
focused-link reader, and list states:

- The address renders whole at every width down to 45 in every state.
- Segments disappear in ladder order: a bar showing `1.2 KB` also shows the
  address; a bar that has dropped the esc destination has already dropped
  `2ms` and `1.2 KB`.
- `◂ esc` without a destination still renders when the target does not fit.
- At 100 columns the focused-link reader shows every action and the full
  address.
- No width produces a line wider than the terminal, and none panics.

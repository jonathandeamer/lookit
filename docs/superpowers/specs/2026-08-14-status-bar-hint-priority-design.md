# Status bar hint priority

## Context

The permanent status bar is doing two jobs. It is a **state readout** — address,
byte count, page, scroll percentage, latency, flags — and it is a **hint strip**
teaching the keymap. Since #71 the hint job has a better home: a responsive,
context-derived help overlay that lays itself out in one to three columns and
retains the longest prefix that fits. The bar kept its hints anyway.

Three findings from the visual review are the same problem seen from different
angles. Measured against the rendered bar, not read off the source:

**The bar does not react to the help overlay at all.** With help open and
closed, the rendered line is byte-identical in every context at every width. On
a reader that means the overlay lists fifteen commands and the bar two lines
below repeats four of them verbatim.

**The most important hint is the first one dropped.** `joinHints` appends
`? help` last, `rightParts` puts hints last in the right group, and the whole
right group gets a single positional `ansi.Truncate`. So the pointer to the
overlay — the one hint that stands in for all the others — is what a narrow
terminal loses first:

| Width | Context | Rendered tail |
|---|---|---|
| 60 | startpage | `… / filter · i target · ? he…` |
| 45 | startpage | `… b bookmark · / filter · …` |
| 45 | user list | `… ↵ go · / filter · r ref…` |
| 45 | reader | `… r refresh · esc back · ? h…` |

At 45 columns the startpage keeps `/ filter` and `i target` and loses `? help`,
which would have shown both of them and ten more.

**The breadcrumb is collateral damage.** The right group is measured first and
the left gets what remains, so hints crowd out the address: `@plan.cat / alice`
becomes `@plan.cat /…` at 60 columns and disappears at 45.

## Decision

Two rules. Neither introduces a general priority framework — that would be a
bigger mechanism than the problem needs.

**1. While the help overlay is open, the bar shows no hints.** State stays:
address, bytes, page, scroll, latency and flags are not in the overlay, so the
bar is the only place they exist. Hints are removed because the overlay is
showing the same commands, better laid out, two lines away.

**2. When hints do not fit, whole hints are dropped from the end, and `? help`
is never dropped.** Today the joined string is cut mid-word wherever the budget
runs out. Instead, drop trailing hints one at a time until the remainder fits,
always retaining `? help`. A bar that cannot fit `? help` alone falls back to
the existing ellipsis truncation, so there is no width at which this renders
worse than today.

The third finding needs no rule of its own. Both changes hand width back to the
left group, so the breadcrumb reclaims it as a consequence.

### What this deliberately does not do

It does not strip hints when help is closed. There is a real argument that the
bar's hints are how a user discovers keys without pressing `?` at all, and that
removing them punishes exactly the people who most need them. Rule 2 is the
lighter answer: keep the hints, and when they do not fit, lose the least useful
ones rather than the most useful one. If the bar later proves too noisy, that is
a separate decision with its own evidence.

## Mechanism

Rule 1 lives in `statusBarModel`, beside the existing `requestFailure` case that
already clears hints for a different reason.

Rule 2 lives in `statusBar.render`. `statusBar.hints` stays a single
` · `-joined string, and the renderer recovers its units by splitting on that
separator. While the right group is too wide it drops the last unit, stopping
if the only remaining unit is `? help`. The existing `ansi.Truncate` over the
joined right group stays as the final fallback, so nothing renders worse than
today at any width.

`? help` is identified by value, not position: `joinHints` is not the only
place hints are built, and the pinning rule must hold for any caller.

### Why the hints stay a string

The obvious alternative is to make `hints` a `[]string` and join at render
time, which is tidier and was this spec's first mechanism. It is rejected on
collision grounds rather than taste: it rewrites roughly twenty `statusBar{…}`
literals in `tui/statusbar_test.go`, and PR #86 is open against
`tui/statusbar_test.go` and `tui/app.go` at the time of writing.

The separator is not a guess. Every producer of `hints` joins with the same
` · `, so splitting recovers the units losslessly. The one string that is not a
hint list is a transient flash, which `statusBarModel` assigns directly; a flash
carries no separator, so it splits to a single unit, nothing is dropped, and it
falls through to the ellipsis path exactly as it does today.

If the field ever does become `[]string`, rule 2 moves unchanged — it operates
on units either way.

## Interaction with the flash and priority paths

`statusBarModel` already replaces hints with a transient flash message, and
already clears them when a `requestFailure` priority status is showing. Rule 1
is a third case in the same place and must compose with both: a flash while help
is open keeps the flash, because the flash reports something that just happened
and the overlay never shows it.

`renderPriority` already blanks hints when it recurses to render the ordinary
bar beside a priority status. That behaviour is unchanged and independent.

## Scope

`tui/app.go` (rule 1, a few lines in `statusBarModel`), `tui/statusbar.go`
(rule 2, in `render`), and new tests. No existing `statusBar{…}` literal
changes shape, so the diff stays clear of PR #86. No change to the help
overlay, the keymap, `finger/`, `render/`, or any user-facing string beyond
which hints are shown.

## Testing

Assert rendered output at real widths, because the decision is about what
survives truncation and an assertion on the hint slice would not see it.

- With help open, the rendered bar contains no hint text and still contains the
  byte count, scroll percentage and address.
- With help open, a flash still displaces the hints and is shown.
- At 45 columns, the startpage bar contains `? help` and has dropped whole
  hints, with no partial hint text.
- At 45 columns, no rendered bar ends in a truncated hint word.
- At 100 columns nothing is dropped: the full hint list is present.
- A width too small for `? help` alone still renders, ellipsis-truncated, and
  does not panic or produce a wider line than the terminal.
- The reader's breadcrumb survives at 60 columns, where it is currently cut.

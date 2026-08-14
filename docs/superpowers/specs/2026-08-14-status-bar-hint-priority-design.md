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

**2. When hints do not fit, whole hints are dropped from the end.** Today the
joined string is cut mid-word wherever the budget runs out. Instead, drop
trailing hints one at a time until the remainder fits, in three descending
stages:

1. Drop from the end, keeping the first hint and `? help`.
2. If those two still do not fit together, keep the first hint alone.
3. Give up, and let the existing ellipsis truncation take over — so there is no
   width at which this renders worse than today.

### Amendment: `? help` is not pinned above the first hint

An earlier draft of this spec pinned `? help` unconditionally, on the reasoning
that it is the pointer to the overlay carrying everything else. Implemented that
way it regressed issue #76, which #83 had just closed.

A failed request's hints are `r retry · ? help`. At 45 columns the unconditional
pin dropped `r retry` and kept `? help`, leaving the bar advertising a help
overlay on a screen whose only useful action it had just discarded — the same
category of mistake as the `0 B` that #83 removed.

Stage 2 is the correction. `? help` is a pointer; the first hint is an action,
and hint lists are built most-useful-first, so "the first hint" is a sound proxy
for "the one that matters" without the renderer needing to know which state the
app is in.

Caught by a parallel session reviewing this spec before implementation. It
reported the regression at 60 columns and below; only 45 reproduced against the
implementation, which is the difference between simulating a rule and running
it. The substance was right and the fix is the same either way.

The third finding needs no rule of its own **at 60 columns and above**. Both
changes hand width back to the left group, so the breadcrumb reclaims it as a
consequence.

At 45 columns it does not. Hints are already down to their last entry there and
`◂ esc: trunc@127.0.0.1:2479` alone is 27 cells, so the address stays clipped.
Closing that gap means extending the drop order past the hints and into the
state segments this spec deliberately leaves alone — `latency`, `meta`,
`page`/`scroll`, and degrading `◂ esc: <target>` to a bare `◂ esc`. That is
specced separately in
`2026-08-14-status-bar-state-ladder-design.md` and is out of scope here.

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
time, which is tidier and was this spec's first mechanism. It was first
deferred because PR #86 was open against `tui/statusbar_test.go` and
`tui/app.go`; #86 has since merged, so that reason is gone and the choice needs
a better one.

It stands on scope. The field type is orthogonal to the decision: rule 2
operates on units whichever way they are stored. Converting the field means
rewriting roughly twenty `statusBar{…}` literals in `tui/statusbar_test.go` and
fourteen assignment sites in `tui/app.go` — mechanical, but unrelated churn
inside a change about which hints survive a narrow terminal, and churn is where
review attention goes to die. It is worth doing, and worth doing on its own.

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

## Outcome

Implemented 2026-08-14. Measured in recorded stills at 45 columns:

| State | Before | After |
|---|---|---|
| startpage, help closed | `28 entries · ↵ go · b bookmark · / filter · …` | `29 entries · ↵ go · b remove · ? help` |
| startpage, help open | identical to help closed | `29 entries` |
| failed request | `… 0 B · ↑↓ scrol…` (pre-#83) | `@127.0… ◂ esc: trunc@127.0.0.1:2479 · r retry` |

No hint is cut mid-word at any width, `? help` survives wherever an action does
not need its space, and `r retry` outranks it on a failed request. The
breadcrumb is still clipped at 45 columns, which the state ladder covers.

Recording the stills also surfaced an unrelated breakage: the error scene's
`Wait` guard still matched `/connect:/`, a fragment of the raw dialer error that
#82 replaced. Every `responses-*.tape` had failed there since that merge. Fixed
in the same branch.

## Scope

`tui/app.go` (rule 1, a few lines in `statusBarModel`), `tui/statusbar.go`
(rule 2, in `render`), and new tests. No existing `statusBar{…}` literal
changes shape. No change to the help
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

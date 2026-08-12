# The target input teaches syntax; the startpage suggests destinations

Date: 2026-08-12
Status: accepted

## Context

Before the startpage existed, the empty target input carried a rotating,
randomly chosen placeholder drawn from five real hosts (`sampleTargets` /
`pickSample` in `tui/app.go`): `ring@thebackupbox.net`, `@happynetbox.com`,
`@plan.cat`, `@tilde.team`, `jonathan@tilde.team`. It did two jobs at once. It
suggested somewhere worth going, and — through the deliberate mix of `@host`
and `user@host` shapes — it taught the two input forms.

The startpage (the embedded catalog in `tui/catalog.txt` plus the user's
bookmarks) took over the first job completely, and does it better: real notes,
a stable list, keyboard selection. Four of the five samples became catalog rows
in their own right, so the placeholder was suggesting a destination that sat a
few lines below it on the same screen. The second job was left uncovered by
anything.

## Decision

The target input's placeholder is a syntax hint, not a suggestion. It is now a
single constant:

```go
const targetPlaceholder = "user@host or @host"
```

It shows in both places the empty input appears: the startpage, and an input
cleared with `i` mid-session.

Two properties are deliberate and are pinned by
`TestTargetPlaceholderSuggestsNoDestination`:

- It names no destination, and specifically is never equal to a catalog target.
  Discovery belongs to the startpage.
- It claims nothing about what a bare `@host` returns. Copy such as "`@host`
  for a directory" was rejected under the honesty convention: the protocol does
  not guarantee a directory, and `ParseUsers` declines often enough that the
  promise would visibly break.

## Rationale

The placeholder and the catalog were answering the same question, and the
catalog answers it with context the placeholder cannot fit. Meanwhile the
question the input is uniquely placed to answer — *what do I type here?* — had
no dedicated answer. Splitting the two jobs gives each surface one purpose.

Randomness earned nothing here. A hint that changes between runs is harder to
recognize, and it cost a `math/rand` dependency in `tui` for a decorative
effect.

Note that the hint is not "safely invalid": `finger.ParseTarget` accepts
`"user@host or @host"`, reading it as a forwarded target (query
`"user@host or "` at `host:79`). This is harmless because a `textinput`
placeholder is never a value — Enter on an empty input fails at
`ParseTarget("")` and issues no fetch — but it means the copy must not be
defended on the grounds that it could not be submitted.

## Consequences

- `sampleTargets` and `pickSample` are gone, as is `math/rand` in `tui`.
- Adding a destination to the placeholder is now a test failure, not a taste
  call.
- The `@host` form stays discoverable through the catalog rows themselves,
  which are mostly `@host` shaped, and through the `?` help panel.

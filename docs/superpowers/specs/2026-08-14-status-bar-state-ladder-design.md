# Status bar state ladder

**Status:** accepted and in progress. Rules 1 and 2 from
[`2026-08-14-status-bar-hint-priority-design.md`](2026-08-14-status-bar-hint-priority-design.md)
are implemented in the same branch rather than on `main`, which is what the
"do not implement until they land" condition was protecting against — there is
no divergence to resync. The #76 conflict this document flagged is resolved:
rule 2 now keeps the first hint over `? help`, so `r retry` survives a failed
request. See [Handoff](#handoff) for the evidence and the settled decisions.

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

## Conflict to resolve first: rule 2 versus issue #76 — RESOLVED

Resolved 14 August 2026 in commit `b12ab97`, before this ladder was started.
The report below stands as written; only the resolution differs from the
suggestion it offers.

Rather than special-casing `node.entry.failed()`, `hintsWithin` now degrades in
three stages and keeps **the first hint** as well as `? help`, then keeps the
first hint alone when the two cannot both fit. Hint lists are built
most-useful-first, so on a failed request the first hint is `r retry` and it
survives without the renderer knowing which state it is in. Verified in a
recorded still at 45 columns: `@127.0… ◂ esc: trunc@127.0.0.1:2479 · r retry`.

The reported 60-column case did not reproduce against the implementation; only
45 did. The substance was right either way.

### The report, as filed


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

## Handoff

Written by the session that scoped this (worktree `statusbar-width`, branch
`worktree-statusbar-width`, commit 60663e4) for the session that will build it.
That branch is frozen at one docs commit — this file — and will not move again,
so there is nothing to sync. Cherry-pick it or read it with
`git show worktree-statusbar-width:<path>`; both worktrees share the object
store. No `tui/` file was touched.

### Where this came from

Items 19 and 20 of the TUI visual review artefact *The Startpage Keyhole*
(13 August 2026), reviewed against `main` at 53790bd..d8c71f3.

- **Item 19** is done and needs nothing. #83 (ef62492) added the
  `node.entry.failed()` branch at `tui/app.go:1463`, which drops `meta` and
  `scroll` on a failure. The artefact was recorded before that merge, so its
  `0 B` and phantom scroll hint are already gone. What its 45-column frame
  still showed is the truncation order — this document.
- **Item 20** is the breadcrumb clipping. Rules 1 and 2 cover it at 60–100
  columns. This ladder covers what is left at 45.

### Decisions already taken, so they are not relitigated

Jonathan chose these; they are settled unless he reopens them.

1. **The bar stays one line.** A two-line bar when rows are spare was
   considered and rejected: it moves `bodyHeight`, `resize`, every sub-model's
   sizing and all twelve review tapes, and it stops the bar being a fixed
   landmark. Item 20's "forty rows sit empty above it" framing is *not* an
   invitation to spend height.
2. **Fix the drop order, don't just shorten the strings.** Abbreviating
   segments buys ~20 cells once and leaves the cut order still wrong.
3. **Stack, don't merge or supersede.** Rules 1 and 2 stand as specced for the
   hint half; this ladder only governs the state segments that spec
   deliberately leaves alone. Folding this into rule 2 was explicitly
   considered and set aside.

### Open conflict, unresolved at handoff

Rule 2 regresses #76 — see [the section above](#conflict-to-resolve-first-rule-2-versus-issue-76).
Messaged to the `statusbar-hint-priority` session on 14 August 2026; no reply
had arrived when this was written. It is a decision for the rule-2 owner, and
it should be settled before or alongside this ladder, because both change what
survives a narrow bar.

### Reproducing the measurements

The evidence tables come from a throwaway probe, deliberately not committed.
Recreate it as `tui/zz_probe_test.go`, run `go test ./tui/ -run TestProbe
-count=1 -v`, and delete it — it prints rendered bars, it asserts nothing.

```go
package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// applyRule2 simulates the hint-priority spec's rule 2: drop whole trailing
// hints until the bar fits, never dropping "? help". Replace with the real
// implementation once it exists.
func applyRule2(b statusBar) statusBar {
	units := strings.Split(b.hints, " · ")
	for {
		b.hints = strings.Join(units, " · ")
		if b.fullWidth(b.rightParts(true), strings.Join(b.flags, "  ")) <= b.width {
			return b
		}
		if len(units) <= 1 {
			return b
		}
		if units[len(units)-1] == "? help" {
			units = append(append([]string{}, units[:len(units)-2]...), "? help")
			continue
		}
		units = units[:len(units)-1]
	}
}

func TestProbeBars(t *testing.T) {
	st := newStyles(true)
	cases := []struct {
		name string
		b    statusBar
	}{
		{"error", statusBar{host: "@127.0.0.1", user: "nobody", escTarget: "trunc@127.0.0.1:2479",
			latency: "1ms", hints: "r retry · ? help", styles: st}},
		{"readerplain", statusBar{host: "@127.0.0.1", user: "alice", escTarget: "@127.0.0.1",
			latency: "2ms", meta: "1.2 KB", hints: "↑↓ scroll · r refresh · ? help", styles: st}},
		{"readerlink", statusBar{host: "@127.0.0.1", user: "alice", escTarget: "@127.0.0.1",
			latency: "2ms", meta: "1.2 KB", hints: "link 1/2 · URL · ↵ go · y copy · tab next · r refresh", styles: st}},
		{"list", statusBar{host: "@tilde.team", escTarget: "@tilde.team", latency: "42ms",
			meta: "37 users", page: "page 2/4", hints: "↵ go · / filter · r refresh · ? help", styles: st}},
	}
	for _, c := range cases {
		for _, w := range []int{45, 60, 80, 100} {
			b := c.b
			b.width = w
			fmt.Printf("%-11s %3d |%s|\n", c.name, w, ansi.Strip(applyRule2(b).render()))
		}
		fmt.Println()
	}
}
```

The four `statusBar` literals mirror what `buildStatusBar` (`tui/app.go:1375`)
actually produces in those states; they were read off it, not invented. Two
traps found while using this probe:

- Do not test "the address survived" with `strings.Contains(out, crumb)`. The
  `list` case gives a false positive, because `@tilde.team` also appears inside
  `◂ esc: @tilde.team`. Anchor on the left of the line instead.
- `render` budgets the right group first and gives the left the remainder, so
  a left-side change only shows up through a right-side one. That asymmetry is
  the defect, and it is easy to mistake for a broken assertion.

### Current behaviour, for the before/after

Today's `main`, help closed, no rule 2:

| Width | State | Rendered |
|---|---|---|
| 100 | reader, link focused | `@127.0.0.1 / ali… ◂ esc: @127.0.0.1 · 1.2 KB · link 1/2 · URL · ↵ go · y copy · tab next · r refresh` |
| 45 | failed reader | `◂ esc: trunc@127.0.0.1:2479 · r retry · ? he…` |
| 45 | reader | `◂ esc: @127.0.0.1 · 1.2 KB · ↑↓ scroll · r r…` |

Note the 100-column row: both ends clipped, and `2ms` silently dropped by the
one all-or-nothing fit test `render` already has (`tui/statusbar.go:141`). That
test is the seed of this ladder — generalise it rather than writing a new
mechanism beside it.

### Repo constraints worth restating

From `CLAUDE.md`, the ones this task will actually hit:

- `make check` before committing. The `tui` race tests take ~75s; budget for it.
- Conventional Commits, and **no AI co-author or "Generated with" trailers**
  anywhere outward-facing — commits, PR bodies, issue bodies.
- Branch → PR → green `test` → squash merge. Never a direct push to `main`.
- Edit ≠ ship: pushing, opening a PR, and enabling auto-merge each need an
  explicit go-ahead from Jonathan.
- Pair every colour with a light/dark value. Not expected to arise here — the
  ladder changes which segments render, not how they are styled.

### Not in scope

- Review finding 21 (the bar is right-aligned on an otherwise left-aligned
  page). Separate taste call.
- Re-recording the review stills (`make review-tui`, ~7 min, needs the loopback
  fingerd). Worth doing once this and rules 1–2 have both landed, so the frames
  show the finished bar rather than an intermediate state.

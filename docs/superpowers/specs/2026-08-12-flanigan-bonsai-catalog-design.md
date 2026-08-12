# Adding bonsai@flanigan.us to the catalog

Adding `@flanigan.us` as a host root
(`2026-08-12-startpage-child-hints-design.md` required one for any host with
grouped children) surfaced three services the catalog does not carry: the root's
own note reads *"Four fingers: bonsai, ping, wisdom, calendar"* and only
`calendar` was catalogued. That spec deferred surveying them. This one closes it.

All three were probed live on **2026-08-12**.

## What they are

| Target | Response |
|---|---|
| `bonsai@flanigan.us` | 27 lines, 1,948 bytes of ASCII art — a bonsai tree beside a speech bubble carrying a one-line saying. A second probe returned a **different tree**, so it is generated per request. |
| `wisdom@flanigan.us` | One aphorism: *"When in doubt, wear your favorite outfit!"* — byte-identical across two probes seconds apart, and the **same line** the bonsai prints in its bubble. |
| `ping@flanigan.us` | `PONG!` |

## Decision

**Add `bonsai` only.**

```
service bonsai@flanigan.us A fresh ASCII bonsai, with an aphorism
```

Both halves of the note are traceable: two probes returned different trees, and
the speech bubble carries the saying. The note deliberately does **not** claim a
rotation period for the aphorism — two probes seconds apart show only that it is
stable, not that it changes daily, and asserting a schedule lookit has not
observed is the failure the catalog already corrected once.

**No hint.** A hint rescues a token that does not say what it is; `bonsai` is a
common word, and the note appears the moment the cursor lands on the row. A hint
here would start the drift toward a hint on every child, which the hints spec
exists to prevent.

### Excluded, and why

**`wisdom@flanigan.us`** — it is the line `bonsai` already prints. The catalog
has settled this case twice before: `david@netbros.com` was dropped as
byte-identical to `david@collantes.us`, and four `weather:ZIP` services
collapsed into one entry. A second row for text already on screen in the first
buys nothing.

**`ping@flanigan.us`** — `PONG!` proves the host answers, which every other row
proves by loading. It is a liveness check, not a place to start from.

Both remain reachable: the `@flanigan.us` root advertises all four fingers in
its own response, and a user can type either address.

### The 80-column caveat

The art is **exactly 80 columns** wide, so in a standard 80-column terminal it
wraps once the reader's chrome takes its space, and the tree breaks.

This is recorded here rather than in the note. Every catalog note describes the
server's content; none describes lookit's rendering, and starting now would
misplace the information. It is worth recording because it is the **counter-case
to `ansi@happynetbox.com`**, which the original survey excluded for promising
art it could not deliver: that response carried 1,365 literal `\e` sequences and
no escape bytes at all, so a client showed `\e[0;44;37m` as visible text.
`bonsai` is the opposite — zero escape bytes, zero literal `\e`, one non-ASCII
character — and renders faithfully whenever it fits. The failure mode here is
width, not content, and it degrades rather than lies.

## Consequences

The one-line change moves numbers that several tests pin. These are the work:

- The catalog holds **20 services** and **26 entries** (from 19 and 25).
- `TestServicesGroupUnderHostRoots` (`tui/sections_test.go`) pins an explicit
  20-target ordering; `bonsai@flanigan.us` sorts before `calendar@flanigan.us`,
  so the list becomes 21 entries with `bonsai` inserted directly after
  `@flanigan.us`.
- `TestOverviewAndStatusCountsExcludeStructuralCopies` (`tui/app_test.go`) pins
  `services: 19` and totals of 25 across its scenarios; each rises by one. The
  `filtered` scenarios keyed to `happynetbox.com` are unaffected.
- `@flanigan.us` **stops being a single-child group.** `calendar` keeps
  `lastChild` because `bonsai` sorts first, so no rendering changes — but the
  child-hints plan
  (`docs/superpowers/plans/2026-08-12-startpage-child-hints.md`, Task 4) uses
  `calendar@flanigan.us` as its "only child of its group" fixture, and that
  comment becomes false. Correct the plan before executing it, or pick another
  single-child group — there will not be one, so the case should be dropped or
  built from a synthetic catalog.

No code changes. No grammar changes. Nothing in `finger/`.

## Testing

- Update the two pinned fixtures above to the new counts and ordering.
- `TestCatalogIsWellFormed` and `TestCatalogHasRootForEveryGroupedHost` already
  cover the new line: it is a service child whose host ships a root.
- No new test is warranted. The entry is data, and the data guards already
  exist.

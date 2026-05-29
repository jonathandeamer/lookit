# Generic token-selection fallback design

## Goal

Add a last-resort, dependency-free parser to `tui/userlist.go` that opens a
selectable list from host responses **no named parser recognizes** — but only
when the body carries a confident, structurally list-like signal. This delivers
the "in-viewport token-selection fallback" deferred in Phase 2.5, without
weakening the project's core guarantee: `ParseUsers` declines rather than
guesses.

## Non-goals

- No change to any existing matcher (columnar, grid, marker, typed-hole, sava,
  redterminal, Finger Ring, telehack) or to their ordering. The fallback runs
  **only after all of them decline**.
- No bare `@handle` / social-handle harvesting. No harvesting of arbitrary
  `user@host`-shaped tokens (e.g. email addresses) from prose.
- No new navigation, CLI surface, or rendering path. The fallback produces the
  same `[]User` the list screen already consumes.
- No machinery to suppress every conceivable false positive. The fallback is
  honest-by-labeling, not perfect (see "Accepted residual risk").

## Where it lives

A new pure function `parseGenericList(lines []string) ([]User, string, bool)`
becomes the **final** branch of `parseUserList`, after `parseTelehackStatus`:

```go
if users, preamble, ok := parseGenericList(lines); ok {
    return parsedUserList{users: users, preamble: preamble, generic: true}, true
}
return parsedUserList{}, false
```

`parsedUserList` gains a `generic bool` field so `routeFetch` can flag the list
as heuristic (see "Flagging"). All earlier branches leave it false.

## Activation model

Automatic, with reader fallback. Because the fallback sits dead-last, it only
ever sees bodies that today render in the plain reader. Its single job is to
*occasionally upgrade a decline into a list*; its only failure mode is a
false-positive list. That asymmetry drives every rule below toward **precision
over recall.**

- If the fallback finds a qualifying list → open the list (flagged generic).
- If it does not → return `false` → the body renders in the reader, exactly as
  today. `render/` can still highlight any `finger://` / `finger user@host`
  references in that reader view (unchanged behavior).

## What qualifies: structured login lines (the gate)

The fallback scans for the **longest contiguous run** of lines where each line
is a "structured login entry". A line (after `strings.TrimSpace`) is a
structured login entry if it is exactly one of:

1. **Bare login** — the whole trimmed line is a single token matching
   `loginRe`. (e.g. `betsy`, `SNCF68B`)
2. **Columnar login** — the first whitespace token matches `loginRe`, and it is
   followed by a **tab or 2+ spaces**, then arbitrary text taken as the name.
   (e.g. `fisher····fisher medders`)

Everything else — including a `login` followed by a **single space** and more
text, and any `login : description` colon form — is treated as prose and is
**not** an entry. A run ends at the first non-entry line (or blank line).

The run must contain **≥ 2 distinct logins** or the fallback declines. Logins
dedup by login string; order preserved.

### Why colon and single-space are excluded

`login : value` and `login value` legends/glossaries/prose are everywhere in
finger help text. The decisive case is `db.debian.org`, whose daemon help
contains a 10-line attribute legend:

```
      cn : First name
      mn : Middle name
      sn : Last name
      email : Email
      ...
      key : Key block
```

A colon-delimited rule would open a "user list" of LDAP attribute names. The
typed-hole parser handles the *same shape* safely only because it is gated on
the `Available fingers:` header cue; the generic fallback has no cue, so it must
not accept the colon form at all. With colon and single-space excluded,
`cn : First name` tokenizes to `["cn", ":", "First", "name"]` — `cn` is followed
by a single space, so the prose-guard rejects it, and db.debian.org drops to 0
entries and declines.

`tab / 2+ spaces` is kept because that gap is a deliberate column layout (a
headerless `who`-style dump), rare in prose and legends.

## What it adds: strong-context drill targets (additive only)

When — and only when — the structured-login gate has already opened the list,
the fallback also harvests cross-host drill targets by scanning the whole body
with the **existing** regexes `fingerURLRe` (`finger://host/login`) and
`fingerCommandRe` (`finger login@host`). These are the same strong-signal
contexts the Finger Ring and sava parsers already trust.

- Each harvested target is pinned to port 79 via the existing `pinFingerPort`
  path (carried on `User.Target`), so a hostile response cannot redirect lookit
  at another service.
- Bare emails, bare `@handles`, and arbitrary `user@host` tokens are **never**
  harvested — only the two strong-context regexes.
- Harvested targets are appended after the structured entries, deduped by
  target. They are *additive*: they can never, on their own, open a list.

### Why additive-only is load-bearing

`graph.no`'s help text contains `finger oslo@graph.no`, which `fingerCommandRe`
matches → `oslo@graph.no`. If strong-context targets could open a list alone,
graph.no would produce a phantom one-entry `oslo` list — `oslo` is a *usage
placeholder*, not a user. Because targets are additive and graph.no has 0
structured logins, no list opens and the help renders in the reader. Correct.

## Flagging

A generic list is heuristic by construction, so the user is told:

1. **Title suffix.** `routeFetch` appends a marker (e.g. `(detected)`) to the
   list title, composing with the existing `(incomplete)` flag when both apply.
2. **Preamble note.** A short line above the list (in the preamble the list
   screen already renders) explains the entries were auto-detected from an
   unrecognized response.

The mechanism reuses the existing title/preamble plumbing in
`newListWithPreamble`; the `generic` flag from `parsedUserList` is threaded
through `routeFetch` to choose the suffix and note.

## Accepted residual risk

- **Bare-login false positives.** `loginRe` is permissive, so a contiguous block
  of ≥2 single-word lines that are not really logins (a short-line poem, a
  single-word menu, a tag list) can register as a generic list. Mitigations: the
  ≥2 gate, the prose-guard (multi-word single-space lines never qualify), dead-
  last ordering, and the `(detected)` flag + preamble note. We accept this as the
  inherent cost of a fallback rather than adding fuzzy dictionary heuristics.
- **Placeholder target appended to a real list.** A genuine structured list whose
  preamble also contains a `finger example@host` usage hint will get a phantom
  `example` entry appended. Minor; same mitigations apply. Not worth machinery to
  suppress.

## Test corpus (additions to `userlist_test.go`)

The fallback is validated by real captures (2026-05-29), in the existing
golden-corpus style, with both parse and decline cases.

**Decline cases — fallback must return `false` (render in reader):**

- `db.debian.org` — daemon help with the `cn : First name` attribute legend.
  The headline regression guard: proves colon/single-space exclusion holds.
- `graph.no` — weather usage help; `finger oslo@graph.no` present. Proves the
  additive-only rule (no phantom `oslo`).
- `tilde.town` — one-line community blurb + URL.
- `nutts.org` — `must provide username`.
- `dead.garden` — empty body.

**Parse cases — fallback must open a list:**

- A headerless bare-login-per-line block (≥2 logins, no `Login` header, no
  `online`/`logged in` cue, no `>` marker) — the canonical thing every named
  parser declines.
- A headerless columnar block (login + 2-space/tab gap + name).
- A bare-login block **plus** a `finger user@host` line in the same body —
  asserts the harvested target is appended and pinned to `:79`.

**Preemption guard (regression):** re-running the full existing corpus must show
that every currently list-bearing host is still claimed by its earlier parser
and never reaches `parseGenericList` — `plan.cat`/`tilde.institute`/`tilde.pink`
(columnar), `tilde.team`/`envs.net`/`zaibatsu.circumlunar.space`/`cosmic.voyage`
(grid), `happynetbox.com` (marker). No existing assertion changes.

## CI gates

Unchanged: `go vet ./...`, `gofmt -l .` empty, `go test ./... -race`. The
fallback adds no I/O; all tests stay offline.

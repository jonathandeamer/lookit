# Catalogue Finger command link detection

**Author:** (lookit)
**Date:** 2026-08-13
**Status:** Draft

## Overview

Service responses advertised by the built-in catalogue (`tui/catalog.txt`)
often tell the reader to finger a colon-bearing or multi-word query
(`weather:seattle@…`, `finger urban:old school@…`,
`finger "oslo, united states"@graph.no`). `DetectLinks` in `tui/links.go`
currently misclassifies many of those as generic URLs, or splits them at
whitespace and quotes, so the focused reader link is the wrong target.

This spec restores the 2026-06-06 URL grammar (`scheme://` plus `mailto:`
only), then adds two tightly gated span adjustments on the existing phase-2
`@` scanner: cued spaced-query expansion, and one shell-style quote
production. Classification, forwarding, port pinning, and list harvesting
stay on today's functions.

`DetectLinks` does **not** run on the startpage or on `catalog.txt`. The
interesting inputs are service **response bodies** that land in
`stateReader`.

## Background & Motivation

### What the catalogue actually advertises

`tui/catalog.txt` holds usage *stubs* and startpage targets, not the
commands this scanner will see:

| Catalog line | What it is |
|---|---|
| `weather:city@…`, `dict:word@…`, `urban:word@…` | note stubs; not scanned |
| `sudoku:easy@…`, `wordsearch:today@…` | complete startpage targets; not scanned |
| `finger oslo@graph.no` | note stub; not scanned |

The 2026-08-11 catalog spec records the live usage shapes that *do* appear
in reader bodies after you finger those startpage rows:

- `dict@bbs.airandwave.net` — *"Dictionary Lookup / Use: finger dict:word@…"*
  (cued, single token)
- `@graph.no` — `finger oslo@graph.no` (cued, single token)
- `@bbs.airandwave.net` — a menu of service names whose usage text uses the
  `service:arg@host` shape

A 2026-08-13 reader sweep of those bodies produced the command *shapes*
below. They are the golden `DetectLinks` fixtures for this work. They are
**not** claimed to be byte-identical server captures; re-probing those
hosts is out of scope. Each fixture is marked cued or uncued so the two
action policies cannot be mixed up.

**Single-token colon-bearing addresses** (phase-1 bug after URL-grammar
drift). Uncued they must still be one complete `Raw`; a `finger` cue makes
them strong/drill:

- `weather:seattle@bbs.airandwave.net`
- `quake:1@bbs.airandwave.net`
- `dict:word@bbs.airandwave.net`
- `urban:yeet@bbs.airandwave.net`
- `wiki:albert_einstein:1@bbs.airandwave.net`
- `sudoku:print:utf8:easy@bbs.airandwave.net`

**Already one `@`-token today** (not broken by `schemeURLRe`; keep-working
regression). Shipped `schemeURLRe` requires a 2–31 character scheme
(`[A-Za-z][A-Za-z0-9+.\-]{1,30}:`), so a one-letter prefix never matches
phase 1. Phase 2 already expands through `:` (`:` is not `isDelim`):

- `o:oslo@graph.no`

**Spaced query** — one link only when a word-bounded `finger` cue
introduces it. Uncued, Non-goals stand: only the final `isDelim` token is
a link.

- cued: `finger urban:old school@bbs.airandwave.net`
- uncued (not one link): `urban:old school@bbs.airandwave.net`

**Quoted query** — one accepted production, `finger` optional:

- `finger "oslo, united states"@graph.no`
- `"oslo, united states"@graph.no`

If a live urban/graph.no body advertises the spaced form **without**
`finger`, that line stays split (`school@bbs.airandwave.net` only). That is
an accepted residual of the Non-goal, not a new inference rule.

### Root cause

`DetectLinks` is a two-phase scanner (`tui/links.go`).

**Phase 1** (`schemeURLRe` → `classifySchemeURL`) has first refusal and
marks matched bytes `consumed`. The 2026-06-06 body-link design, and the
original regex in `docs/superpowers/plans/2026-06-06-body-link-detection.md`,
accepted only `scheme://authority…`. The shipped regex added a
`scheme:path` alternative:

```go
`(?i)[A-Za-z][A-Za-z0-9+.\-]{1,30}:(?://[^\s<>"` + "`" + `]+|[^\s<>"` + "`" + `/][^\s<>"` + "`" + `]*)`
```

`classifySchemeURL`'s default branch then promotes every such span to
`LinkURL`. That consumes `wiki:albert_einstein:1@host` (and every other
2+-letter `label:value@host`) before phase 2 sees the `@`. The only
colon-only scheme the 2026-06-06 design assigned was `mailto:`. The
`scheme:path` alternative is implementation drift, not an intended policy
change.

**Phase 2** expands from each unconsumed `@` through `!isDelim` bytes.
`isDelim` is whitespace plus `<>"'`()`{}[]`. So:

- `finger urban:old school@host` classifies as a *strong* Finger link whose
  `Raw` is only `school@host` (`findCueWord` still sees `finger`).
- `"oslo, united states"@graph.no` stops at the closing quote and yields
  `@graph.no` — a Rule 3 host query, already strong/drillable, but the
  wrong target.

## Goals & Non-Goals

### Goals

- Every complete single-token catalogue address above is one `LinkFinger`
  with the full `Raw`, not a `LinkURL`.
- A cued `finger <spaced query>@host` is one strong, drillable Finger link.
- The accepted quote production is one Finger link whose `Raw` keeps the
  quotes and whose `Target.Query` does not.
- Uncued colon-bearing `query@host` stays policy B (copy-first, `f` drills).
- `scheme://` URLs and `mailto:` keep today's classification, including
  `consumed` so phase 2 does not also emit the address.

### Non-goals

- Do not infer spaced queries unless a word-bounded `finger` cue
  introduces them on the same line. Uncued `urban:old school@host` is not
  one link.
- Do not execute or preserve arbitrary shell syntax. Only the one quote
  production below is grouping; backticks, Unicode quotes, nested quotes,
  and other placements are ignored.
- Do not change `domainSane`, forwarding rules, port-79 pinning
  (`finger.ParseTargetPinned`), `loginRe` / `harvestableLogin`, OSC-8
  policy (`isOSC8Openable`), or the action policy for ambiguous bare
  addresses.
- Do not promote shorthand such as `@bonsai` from the Flanigan menu: it is
  not a complete domain-qualified Finger address.
- Do not scan `catalog.txt` or the startpage. Do not add a service-prefix
  allowlist. Do not change `applyLinkOverlay` in this slice.

## Proposed Design

```mermaid
flowchart TD
  body["sanitized body"] --> p1["Phase 1: schemeURLRe"]
  p1 -->|"://" or mailto:| classifyURL["classifySchemeURL / classifyFingerURL"]
  classifyURL --> consumed["mark bytes consumed"]
  p1 -->|"label:value, including wiki:…@host"| p2
  consumed --> p2["Phase 2: @ tokens via isDelim"]
  p2 --> cue["findCueWord / cueKind from original isDelim start"]
  cue --> quote{"quote production q query q @ host?"}
  quote -->|yes| qspan["Raw = q query q @ host; classifyAtToken gets query@host"]
  quote -->|no| shellquote{"ASCII quote/backtick in expansion candidate?"}
  shellquote -->|yes| today
  shellquote -->|no| expand{"findCueWord is finger and one-@ predicate?"}
  expand -->|yes| span["Raw = TrimSpace(span after finger)"]
  expand -->|no| today["today's isDelim token"]
  qspan --> class["classifyAtToken; then restore quoted Raw"]
  span --> class
  today --> class
  class --> pin["ParseTargetPinned, or existing same-relay exception"]
```

**Phase-2 precedence (frozen).** From the original `isDelim` `@` token,
in this order:

1. Try the quote production. A match is the link: suffix-only wins;
   `Link.Raw` keeps the quotes; `classifyAtToken` receives the unquoted
   `query@host` parse argument; quotes are never part of `Target.Query`.
2. Else, if the would-be expanded span contains ASCII `"`, `'`, or a
   backtick, keep today's `isDelim` token. Malformed shell-like syntax is
   never widened into a query.
3. Else, if `findCueWord` returned `finger`, try the one-`@` expansion.
4. Else keep today's `isDelim` token.

Quote grouping is not “expansion declined.” Cued quoted lines match step
1, so expansion never sees them and cannot pull `oslo,` (or the quotes)
into `Query`.

### URL grammar

Keep `classifySchemeURL`, `classifyFingerURL`, and `isOSC8Openable`.
Change only which phase-1 spans are candidates.

`schemeURLRe` must match, case-insensitively (`(?i)` as today). Both
alternatives use the **same body class as the shipped `://` arm**,
`[^\s<>"\`]` — not `!isDelim`. `isDelim` also stops on `(){}[]`; the
regex does not. The existing `stripTrailingPunct` post-filter still
runs on the match (trailing `. , ; : ! ?` and unbalanced closers).

```go
schemeURLRe = regexp.MustCompile(
    `(?i)(?:[A-Za-z][A-Za-z0-9+.\-]{1,30}://[^\s<>"` + "`" + `]+|mailto:[^\s<>"` + "`" + `]+)`)
```

1. `scheme://` plus one or more body-class authority bytes. The existing
   post-filter still drops a match whose authority is empty after `://`.
2. `mailto:` plus one or more body-class address bytes. Empty `mailto:`
   is not a link. `mailto:alice(work)@example.com` is one `LinkEmail`;
   do not truncate at `(`.

No other colon-only form is a phase-1 candidate. `label:value` text,
including `tel:`, `data:`, `magnet:`, and `wiki:albert_einstein:1@host`,
is left for later phases. Those three URI schemes therefore stop being
`LinkURL`. That is an intended restoration of 2026-06-06, not a regression
to paper over.

Must not change:

| Input | Result |
|---|---|
| `MAILTO:alice@example.com` | `LinkEmail`, `Strong`, `ActionCopy`, `isOSC8Openable`; bytes marked `consumed` so phase 2 does **not** also emit `alice@example.com` |
| `mailto:alice@example.com` | same |
| `finger://host/wiki:foo` | `LinkFinger` via `://` → `classifyFingerURL` |
| `http://user:pass@host` | `LinkURL`; the `@` is inside the consumed span |
| `https://…`, `gemini://…`, `gopher://…` | today's kinds and OSC-8 matrix |

Word-boundary checks stay as written in `DetectLinks` (char before the
span must not be a word char or `@`).

### Classification vs span expansion

These are two passes. Do not replace `findCueWord`.

**Classification** still uses `cueKind` / `findCueWord` exactly as
shipped:

- Walk at most five `strings.Fields` words on the same line before the
  *original* `isDelim` token start.
- First word for which `cueKind` succeeds wins.
- `finger` is case-insensitive (`FINGER` counts). `fingerprint` does not
  (`cueKind` requires the whole field).
- `finger:` / `finger,` / `(finger)` still miss: `strings.Fields` keeps
  the punctuation.
- A nearer `email` / `mail` / `e-mail` cue still wins. `finger email
  alice@host` remains `LinkEmail` with `Raw=alice@host`. The new `finger`
  expansion does **not** override that.

**Quote grouping** is tried next, from the original `@` token, before any
expansion. See [Quote grouping](#quote-grouping).

**Span expansion** is a later pass that looks only for a word-bounded,
case-insensitive `finger` on the same line. It never consults `cueKind`.
It runs only when the quote production missed, the candidate contains no
ASCII `"`, `'`, or backtick, and `findCueWord` returned `finger` (so a
nearer email/web cue cannot acquire a multi-word `Raw`, and malformed or
quoted syntax cannot be swallowed by intervening prose).

Word-bounded `finger` means the four bytes `f i n g e r` (any case) with
no `isWordChar` immediately before or after. That is the same boundary
`fingerprint` already fails, without going through `strings.Fields`.

### Cued spaced-query expansion

Attempted only when the quote production did **not** match, the existing
left/right `isDelim` walk has produced an `@` token, and `findCueWord`
returned `finger`:

1. Find the last word-bounded `finger` on the same line whose last byte
   is before this `@`.
2. Candidate start = the first non-whitespace byte after that cue.
3. Candidate end = the current `isDelim` end (then `stripTrailingPunct`,
   as today).
4. Accept the expansion **only if** the slice `[start:end]` contains
   **exactly one** `@`, contains **no other** word-bounded `finger`, and
   contains none of ASCII `"`, `'`, or backtick.
5. `Link.Raw = strings.TrimSpace(span after the cue)` — no leading space
   after `finger`. Tests today expect `alice@tilde.team`, not
   ` alice@tilde.team`.
6. Feed that `Raw` to `classifyAtToken` as today. `user` is everything
   before the single `@`, `host` everything after. `ParseTargetPinned`
   still pins `:79`.

Decline the expansion (keep the `isDelim` token) when any check fails.
Do not walk left through a second `@` "to find a better span."

Yes/no against the predicate:

| Line | `@` under consideration | Expand? | Result |
|---|---|---|---|
| `finger alice@h1 finger bob@h2` | first `@` | yes | `alice@h1` (last `finger` is the first cue; one `@`) |
| `finger alice@h1 finger bob@h2` | second `@` | yes | `bob@h2` (last `finger` is the second cue) |
| `finger alice@example.com and bob@other.host` | first `@` | yes | `alice@example.com` |
| `finger alice@example.com and bob@other.host` | second `@` | **no** — two `@`s from the only `finger` | `bob@other.host` as its own link; **never** one forwarded token |
| `finger please try urban:old school@host` | the `@` | yes | `please try urban:old school@host` (intervening prose is included; that is the predicate, not a bug) |
| `finger email alice@host` | the `@` | **not attempted** — `findCueWord` is `email` | `LinkEmail`, `Raw=alice@host` |
| `FINGER urban:old school@host` | the `@` | yes | case-folds |
| ten words of prose, then `finger urban:old school@host` | the `@` | yes if that `finger` is in the 5-word `findCueWord` window | otherwise policy B on `school@host` |
| a `finger` ten words before `alice@host` with no nearer cue | the `@` | **not attempted** — `findCueWord` misses it | policy B, `Raw=alice@host`. Expansion does not widen the 5-word classification window. |
| `finger "oslo, united states"@graph.no` | the `@` | **not attempted** — quote production already matched | `Raw="oslo, united states"@graph.no`, `Query=oslo, united states` |
| `finger oslo, "united states"@graph.no` | the `@` | **not attempted** — quote production already matched (suffix) | `Raw="united states"@graph.no`, `Query=united states`. Do not expand through `oslo,`. |
| `finger "oslo, united states@graph.no"` | the `@` | **no** — malformed quote placement blocks expansion | today's `states@graph.no` token |
| `finger "oslo, united states" @graph.no` | the `@` | **no** — quote-like syntax blocks expansion | today's `@graph.no` token |

Two-`@` tokens stay on today's forwarded path **only** when they are a
single whitespace-free `isDelim` token (`user@host@relay` →
`classifyForwardedAtToken`). An expanded span is forbidden from
containing two `@`s, so
`finger alice@example.com then bob@thebackupbox.net` cannot become
`Query: "alice@example.com then bob"` via the same-relay exception. That
would be a new server-influenced drill shape; this spec does not add one.

### Quote grouping

`isDelim` continues to treat `"`, `'`, and `` ` `` as hard token stops.
That keeps `'https://example.com'` punctuation tests intact. Quote
grouping is a production-matched adjustment tried **before** spaced
expansion, not a global change to delimiters.

**One accepted production:**

```text
[ finger WS ] q query q @ host
```

- `finger` is optional. If present it must be word-bounded and
  case-insensitive, separated from `q` by whitespace only.
- `q` is `"` or `'`. The two `q`s must match.
- `query` contains no `q` and no `@`.
- No whitespace between the closing `q` and `@`.
- `host` is the usual phase-2 right-expansion through `!isDelim`, then
  `stripTrailingPunct` (`finger "oslo"@graph.no.` → host `graph.no`).

Backticks, typographic quotes, nested quotes, and any other placement are
not grouping productions. ASCII quote/backtick syntax that fails the
production blocks spaced expansion and keeps today's delimiter-bounded
token. Typographic quotes remain ordinary printable query characters; they
receive no shell-style interpretation.

When the production matches it **wins over expansion**, including on
cued lines. Suffix-only is the whole span: do not walk left through
unquoted words before the opening `q`.

- `Link.Raw` is the exact `q query q @ host` substring (quotes kept;
  `finger` and its trailing space are not part of `Raw`).
- `classifyAtToken` still receives an **unquoted** `user@host` parse
  argument (`query + "@" + host`). It has no quote-stripping path today
  and does not grow one. After it returns, restore `Link.Raw` to the
  quoted substring. Wire query is `oslo, united states`, not
  `"oslo, united states"`.
- Classification still uses `findCueWord` from the original `@` token
  start. A preceding `finger` inside the 5-word window makes it
  strong/drill; without a cue it is policy B (copy-first) but still
  **one** complete `Raw`.

Decline / pass fixtures:

| Input | Grouping? | Link |
|---|---|---|
| `finger "oslo, united states"@graph.no` | yes | `Raw="oslo, united states"@graph.no`, `Query=oslo, united states`, strong/drill |
| `"oslo, united states"@graph.no` | yes | same `Raw`/`Query`, policy B |
| `finger 'oslo, united states'@graph.no` | yes | same with `'` |
| `finger "oslo@graph.no` (unmatched) | no | today's `oslo@graph.no` (opening `"` is a delimiter) |
| `"oslo@graph.no"` (quotes around the whole address) | no — `@` is inside the quotes, not `q @ host` | today's `oslo@graph.no` (quotes delimit) |
| `finger "oslo, united states@graph.no"` (host wrapped) | no — `query` would contain `@` | `isDelim` token `states@graph.no` (space + quotes) |
| `finger oslo, "united states"@graph.no` (partial) | **suffix only** — do not pull `oslo,` into the query | `"united states"@graph.no`, `Query=united states` |
| `finger "oslo, united states" @graph.no` (space before `@`) | no | Rule 3 `@graph.no` |
| mixed quotes `"oslo'@graph.no` | no | production requires matching `q` |

### Bare addresses

A colon-bearing `query@host` **without** an explicit `finger` cue follows
policy B, encoded today in `classifyAtToken` when `cueWord == ""`:
`Kind=Finger`, `Strong=false`, `Ambiguous=true`, `Action=Copy`, `f`
drills. The colon alone does not make prose a strong command.

After the URL-grammar fix, that address is still **one** complete `Raw`
(`wiki:albert_einstein:1@host`, not a `LinkURL` and not a split).

Do not call the uncued Goal bullets "commands." They are addresses.

### Host pinning, forwarding, harvest

- Direct targets, including expanded and quote-stripped forms, go through
  `finger.ParseTargetPinned`. `finger urban:old school@host:70` still
  yields `HostPort=host:79`.
- Same-relay forwarding remains the existing manual-`Target` exception in
  `classifyForwardedAtToken` / `classifyFingerURL`. It applies only to a
  whitespace-free two-`@` token or a `finger://relay/user@host` URL.
- `appendHarvestedTargets` (`tui/userlist.go`) still requires
  `Kind==Finger && Strong && Blocked=="" && !Target.HostQuery() &&
  harvestableLogin(Target)`.
- `harvestableLogin` stays `loginRe.MatchString(t.Query)` with
  `loginRe = ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,31}$` (`tui/userlist.go`).
  Colon- and space-bearing queries fail that class and remain
  **reader-only**. Neutrality is a written rule, not an accident of the
  regex: do not "fix" harvest to include service queries. A list body
  that quotes `finger dict:word@other.host` must not grow a row.

### Overlay

`applyLinkOverlay` finds literal `Raw` left-to-right. A multi-word `Raw`
is more likely to wrap in the viewport. That is an existing overlay risk
made slightly worse. Accepted residual: no renderer change in this slice.

## API / Interface Changes

None. `DetectLinks(body []byte, originHostPort string) []Link` and the
`Link` fields stay as in `tui/links.go`. Behaviour changes are confined
to `schemeURLRe` and to how phase 2 chooses the span and parse argument
that `classifyAtToken` receives.

`classifyAtToken` already splits on the first `@` and calls
`ParseTargetPinned(user + "@" + host)`. Expanded `user` values that
contain spaces or extra colons need no new parser API:
`parseDirectTarget` already sets `Query` to everything before `@`.
Quoted matches pass that same function the stripped `query@host` and
overwrite `Link.Raw` afterwards; they do not teach `classifyAtToken`
about quotes.

## Data Model Changes

None. No bookmark, catalog, or on-disk format change.

## Key Decisions

1. **Restore the 2026-06-06 URL grammar, do not invent a new scheme
   classifier.** Only `scheme://` and `mailto:` are phase-1 candidates.
   `classifySchemeURL` / `isOSC8Openable` stay. `tel:` / `data:` /
   `magnet:` cease to be `LinkURL` as a consequence, not as a new product
   feature.

2. **Classification stays on `cueKind` / `findCueWord`.** A nearer
   `email`/`mail` cue still wins. The 5-word same-line window stays.
   Expansion does not become "finger is the authority for kind."

3. **Spaced-query expansion is a separate, rejected-by-default pass.**
   Last word-bounded `finger` before this `@`; start at the first
   non-whitespace after that cue; accept only if the slice has exactly
   one `@`, no other word-bounded `finger`, and no ASCII `"`, `'`, or
   backtick. Attempted only when the quote production missed and
   `findCueWord` already returned `finger`.

4. **Exactly one `@` in any expanded span.** Two-`@` forwarding remains
   today's single-token path. This preserves the 2026-06-06 same-relay
   rule and blocks a synthesized `alice@example.com and bob` query.

5. **Quote production before expansion.** From the original `@` token,
   try `q query q @ host` first. A match is the link (suffix-only;
   `classifyAtToken` gets unquoted `query@host`; `Link.Raw` keeps the
   quotes). If quote-like ASCII syntax is present but malformed, keep the
   original delimiter-bounded token. Only a quote-free miss with
   `findCueWord == "finger"` may try the one-`@` expansion. This is how
   `finger oslo, "united states"@graph.no` stays suffix-only instead of
   expanding through `oslo,`.

6. **One quote production, `finger` optional.** `q ∈ {", '}`, query
   contains no `q` and no `@`, closing `q` abuts `@`. Other ASCII quote
   and backtick placements suppress expansion and retain today's token.

7. **Bare colon-bearing addresses stay policy B.** After the URL-grammar
   fix they are one complete Finger `Raw`, copy-first. A colon is not a
   command cue.

8. **Harvest stays neutral via unchanged `loginRe`.** Service queries
   with `:` or spaces are reader-only. Do not extend `harvestableLogin`.

9. **`Raw = strings.TrimSpace(span after cue)` on the expansion path;
   keep `stripTrailingPunct`.** Overlay wrap of multi-word `Raw` is
   accepted residual risk.

10. **Ship as three PRs; quotes do not depend on expansion.** URL
    grammar is a bugfix against 2026-06-06. PR 2 (spaces) and PR 3
    (quotes) are independent of each other and both depend only on
    PR 1. They must not land as one mixed diff.

## Alternatives Considered

1. **URL-grammar-only fix (rejected as the whole solution; chosen as
   PR 1).** Tightening phase 1 to `scheme://` + `mailto:` already
   restores 2026-06-06 and fixes every *single-token* colon example
   (`weather:seattle@…`, `wiki:albert_einstein:1@…`,
   `sudoku:print:utf8:easy@…`). It does nothing for spaces or quotes.
   Those are a second, independent change and must not block the
   regression fix.

2. **Third phase that parses `finger` command lines** instead of
   stretching the `@` expander through `isDelim`. Cleaner parser
   boundary, but it would duplicate `findCueWord`, forwarding, and
   `ParseTargetPinned` and risk a second classification path. The
   predicate above is a local adjustment to the scanner that already
   owns `@` tokens.

3. **Service-prefix allowlist** (`weather:`, `dict:`, `urban:`, `wiki:`,
   `sudoku:`, `quake:`). Narrower false-positive surface, but it is
   host-specific inference the Non-goals reject, and it cannot express
   `o:oslo`, graph.no quotes, or the next catalogue service.

4. **Treat any colon-bearing `query@host` as a strong command.**
   Rejected. 2026-06-06 policy B exists because a well-formed address is
   indistinguishable from email; a colon does not resolve that. Strong
   is reserved for `finger://`, a `finger` cue, and Rule 3 `@host`.

5. **Stop treating quotes as `@`-token delimiters globally.** Would
   join quoted URLs and break
   `TestDetectLinks_Punctuation_DoubleQuotes`
   (`'https://example.com'` / `"https://example.com"`). Quote grouping
   is a production, not a delimiter change.

## Security & Privacy Considerations

Threat model is unchanged from
`docs/superpowers/specs/2026-06-06-body-link-detection-design.md`:

- Drill is user-initiated (`Enter` / `f`). lookit never auto-opens.
- Server-supplied targets go through `ParseTargetPinned` (port 79;
  `ErrServerForwarding` for two-`@` forms). Same-relay forwarding is
  still the only exception, and only for a single whitespace-free token
  or a `finger://` URL.
- `hasControl` still sits on the parse path. Bodies are sanitized at
  `finger.Query` ingress.
- Expansion must not invent a forwarded query from two addresses that
  share a `finger` cue. The one-`@` acceptance rule is the mitigation
  (severity: high if omitted).
- Harvest must not grow phantom list rows from usage text. Unchanged
  `loginRe` is the mitigation.

This slice touches the server-supplied-target path. Per repo ship rules,
the implementing PR is pushed and **merged by the human**.

## Observability

N/A. `DetectLinks` is a pure function over a sanitized body. No metrics,
logs, or alerts.

## Rollout Plan

No feature flag. Behaviour is local to `DetectLinks` and is covered by
table-driven tests. Roll forward by reverting the PR; there is no
persistent state.

Sequence is the three PRs in [PR Plan](#pr-plan). Each is independently
reviewable and mergeable.

## Testing

Add table-driven cases to `tui/links_test.go` in the existing style
(`DetectLinks(body, origin)`, `findLink`, field assertions). Origin is
`example.com:79` unless a forwarding case needs another host.

`Kind` values below are the `LinkKind` constants. `Action` is
`ActionDrill` or `ActionCopy`. `Blocked` is empty unless noted.

### Table 1 — cued single-token colon addresses (strong / drill)

After PR 1 these already pass; keep them as the grammar regression.

| name | input | Raw | Strong | Ambiguous | Action | Target.Query | Target.HostPort |
|---|---|---|---|---|---|---|---|
| weather | `finger weather:seattle@bbs.airandwave.net` | `weather:seattle@bbs.airandwave.net` | true | false | Drill | `weather:seattle` | `bbs.airandwave.net:79` |
| quake | `finger quake:1@bbs.airandwave.net` | `quake:1@bbs.airandwave.net` | true | false | Drill | `quake:1` | `bbs.airandwave.net:79` |
| dict | `finger dict:word@bbs.airandwave.net` | `dict:word@bbs.airandwave.net` | true | false | Drill | `dict:word` | `bbs.airandwave.net:79` |
| urban | `finger urban:yeet@bbs.airandwave.net` | `urban:yeet@bbs.airandwave.net` | true | false | Drill | `urban:yeet` | `bbs.airandwave.net:79` |
| wiki | `finger wiki:albert_einstein:1@bbs.airandwave.net` | `wiki:albert_einstein:1@bbs.airandwave.net` | true | false | Drill | `wiki:albert_einstein:1` | `bbs.airandwave.net:79` |
| sudoku | `finger sudoku:print:utf8:easy@bbs.airandwave.net` | `sudoku:print:utf8:easy@bbs.airandwave.net` | true | false | Drill | `sudoku:print:utf8:easy` | `bbs.airandwave.net:79` |

Assert `Kind=LinkFinger` and `wiki:…` is **not** `LinkURL`.

### Table 2 — uncued single-token colon addresses (policy B, one Raw)

Same addresses with no `finger` cue. After PR 1, one complete `Raw`;
still copy-first.

| name | input | Raw | Strong | Ambiguous | Action | Target.Query | Target.HostPort |
|---|---|---|---|---|---|---|---|
| weather | `weather:seattle@bbs.airandwave.net` | `weather:seattle@bbs.airandwave.net` | false | true | Copy | `weather:seattle` | `bbs.airandwave.net:79` |
| wiki | `wiki:albert_einstein:1@bbs.airandwave.net` | `wiki:albert_einstein:1@bbs.airandwave.net` | false | true | Copy | `wiki:albert_einstein:1` | `bbs.airandwave.net:79` |
| sudoku | `sudoku:print:utf8:easy@bbs.airandwave.net` | `sudoku:print:utf8:easy@bbs.airandwave.net` | false | true | Copy | `sudoku:print:utf8:easy` | `bbs.airandwave.net:79` |

### Table 3 — keep-working: `o:oslo@graph.no`

Not a scheme-scanner victim. Must remain one Finger link after the regex
tightening; do not special-case one-letter prefixes.

| input | Raw | Strong | Ambiguous | Action | Query | HostPort |
|---|---|---|---|---|---|---|
| `finger o:oslo@graph.no` | `o:oslo@graph.no` | true | false | Drill | `o:oslo` | `graph.no:79` |
| `o:oslo@graph.no` | `o:oslo@graph.no` | false | true | Copy | `o:oslo` | `graph.no:79` |

### Table 4 — cued spaced-query expansion (PR 2)

| name | input | expect |
|---|---|---|
| urban spaced | `finger urban:old school@bbs.airandwave.net` | one link, `Raw=urban:old school@bbs.airandwave.net`, `Query=urban:old school`, `HostPort=bbs.airandwave.net:79`, Strong, Drill |
| two commands, two spaces | `finger alice@one.example finger bob@two.example` | two links: `alice@one.example`, `bob@two.example`, both Strong/Drill |
| two commands, tab column | `"finger alice@one.example\tfinger bob@two.example"` | same two links (`\t` is `isDelim` and `strings.Fields` whitespace) |
| shared cue, two addresses | `finger alice@example.com then bob@other.host` | two links, never one forwarded `Raw`; second is `bob@other.host` |
| intervening prose | `finger please try urban:old school@bbs.airandwave.net` | one link, `Raw=please try urban:old school@bbs.airandwave.net`, `Query=please try urban:old school` |
| port pin | `finger urban:old school@bbs.airandwave.net:70` | `Raw` keeps `:70`; `HostPort=bbs.airandwave.net:79` |
| trailing punct | `finger urban:old school@bbs.airandwave.net.` | `Raw=urban:old school@bbs.airandwave.net` (`stripTrailingPunct`) |

### Table 5 — quote production (PR 3)

| name | input | Raw | Query | Strong | Action |
|---|---|---|---|---|---|
| cued double | `finger "oslo, united states"@graph.no` | `"oslo, united states"@graph.no` | `oslo, united states` | true | Drill |
| uncued double | `"oslo, united states"@graph.no` | `"oslo, united states"@graph.no` | `oslo, united states` | false | Copy |
| cued single | `finger 'oslo, united states'@graph.no` | `'oslo, united states'@graph.no` | `oslo, united states` | true | Drill |
| trailing punct | `finger "oslo"@graph.no.` | `"oslo"@graph.no` | `oslo` | true | Drill |

### Table 6 — URL grammar must-not-change / must-change

| input | expect |
|---|---|
| `visit https://example.com/foo` | `LinkURL`, Strong, Copy, OSC-8 |
| `read gemini://rawtext.club/~alice` | `LinkURL`, not OSC-8 |
| `send to mailto:alice@example.com now` | `LinkEmail`, Strong, Copy, OSC-8; **exactly one** link (phase 2 does not emit `alice@example.com`) |
| `MAILTO:alice@example.com` | same as mailto, case-insensitive |
| `mailto:alice(work)@example.com` | one `LinkEmail`; body class is `[^\s<>"\`]`, not `!isDelim` — do not stop at `(` |
| `mailto:` (empty) | no link |
| `finger://bbs.airandwave.net/wiki:foo` | `LinkFinger`, `Query=wiki:foo`, `HostPort=bbs.airandwave.net:79` |
| `http://user:pass@example.com/x` | one `LinkURL`; no extra Finger/Email from the `@` |
| `tel:+15550000` | no `LinkURL` |
| `data:text/plain,hello` | no `LinkURL` |
| `magnet:?xt=urn:btih:abc` | no `LinkURL` |
| `Timezone: UTC` (no `@`) | no link |

### Table 7 — decline / keep

| input | expect |
|---|---|
| `@bonsai` | no Finger link (not domain-sane) |
| `fingerprint alice@example.com` | not a `finger` cue; policy B on `alice@example.com` |
| `urban:old school@bbs.airandwave.net` (uncued spaced) | **not** one spaced link; only `school@bbs.airandwave.net`, policy B |
| `label:value` with no `@` | no link |
| `finger "oslo@graph.no` | unmatched quote; `oslo@graph.no` only |
| `"oslo@graph.no"` | quotes delimit; `oslo@graph.no` |
| `finger "oslo, united states@graph.no"` | no host-wrapped grouping; `states@graph.no` |
| `finger oslo, "united states"@graph.no` | suffix only: `"united states"@graph.no`, `Query=united states` |
| `finger email alice@example.com` | `LinkEmail`, `Raw=alice@example.com` (no expansion) |

### Table 8 — harvest non-regression

`parseUserList` / `appendHarvestedTargets` over a columnar list that also
contains catalogue usage text:

```text
Login   Name
alice   Alice
bob     Bob

Use: finger dict:word@other.host
Use: finger urban:old school@bbs.airandwave.net
```

- `parseUserList` returns ok (structured-login gate still opens).
- Harvested logins are `alice` and `bob` only.
- `dict:word` and `urban:old school` are **not** rows:
  `loginRe` rejects `:` and spaces.
- `DetectLinks` on the same body still returns the reader links
  (`dict:word@other.host` strong/drill; `urban:old school@…` strong/drill
  after PR 2).

Also assert `loginRe.MatchString("dict:word") == false` and
`loginRe.MatchString("urban:old school") == false` directly, matching
`TestStrongGate_TildeLoginNotHarvestable`.

### Process

Run the new `./tui/` `DetectLinks` / `parseUserList` cases first, then
`make check`. That last line is the gate, not the coverage.

## Risks

| Risk | Severity | Mitigation |
|---|---|---|
| Expansion without a one-`@` guard synthesizes a forwarded query | high | hard acceptance rule; table 4 shared-cue case |
| Extending harvest to service queries adds phantom list rows | high | written neutrality; table 8; do not edit `loginRe` |
| Multi-word `Raw` wraps and `applyLinkOverlay` misses a highlight | low | accepted residual; no renderer change |
| Intervening prose (`please try …`) becomes part of `Query` | low | documented predicate; user can copy/edit. Do not add a prose denylist. |
| Live urban/graph.no text may omit `finger` on the spaced/quoted examples | low | Non-goal + optional-finger quote production. Do not infer uncued spaces. |
| PR 2 merges before PR 3: valid quoted queries are not grouped yet | low | PR 2's ASCII quote/backtick guard keeps today's token; PR 3 later groups only the accepted production. |

## Open Questions

None. Cue window, quote/expansion precedence, quote alphabet,
bare-address policy, `mailto:` body class, and harvest neutrality are
frozen in Key Decisions.

## References

- [Body link detection](./2026-06-06-body-link-detection-design.md) —
  URL grammar, policy B, harvest adapter, `ParseTargetPinned`
- [Body link detection plan](../plans/2026-06-06-body-link-detection.md) —
  original `://`-only `schemeURLRe`
- [Consistent link actions](./2026-08-02-consistent-link-actions-design.md) —
  Enter / `y` / `f` matrix
- [Bookmarks & catalog startpage](./2026-08-11-bookmarks-catalog-startpage-design.md) —
  recorded `finger dict:word@…` and `finger oslo@graph.no` usage
- `tui/links.go` — `DetectLinks`, `schemeURLRe`, `findCueWord`,
  `classifyAtToken`, `classifyForwardedAtToken`, `stripTrailingPunct`,
  `isDelim`, `harvestableLogin`
- `tui/userlist.go` — `loginRe`, `appendHarvestedTargets`
- `finger/query.go` — `ParseTargetPinned`, `parseDirectTarget`

## PR Plan

### PR 1 — restore `scheme://` + `mailto:` URL grammar

- **Title:** `fix(tui): stop classifying colon-bearing finger addresses as URLs`
- **Files:** `tui/links.go` (`schemeURLRe` and its comment),
  `tui/links_test.go` (tables 1–3, 6, and the uncued colon rows of
  table 7 that do not need expansion)
- **Depends on:** none
- **Changes:** Replace the shipped `scheme:path` alternative with the
  explicit regex above (`://` and `mailto:` sharing `[^\s<>"\`]`). Keep
  `stripTrailingPunct`, `classifySchemeURL`, `consumed` marking, and
  `isOSC8Openable`. Prove `wiki:…@host` is one Finger `Raw`, `mailto:`
  still consumes the `@`, `mailto:alice(work)@example.com` is not cut at
  `(`, `finger://host/wiki:foo` still drills, and `tel:`/`data:`/`magnet:`
  are no longer `LinkURL`. `o:oslo@graph.no` stays one Finger token.

This is the 2026-06-06 bugfix. It unblocks every single-token catalogue
address without changing span rules.

### PR 2 — cued spaced-query expansion

- **Title:** `feat(tui): expand cued finger commands across spaces`
- **Files:** `tui/links.go` (`DetectLinks` phase 2, new expansion helper),
  `tui/links_test.go` (table 4, uncued spaced decline, email-cue
  non-expansion, port-pin), and `tui/userlist_test.go` (harvest table 8)
- **Depends on:** PR 1 only (colon tokens must survive phase 1 before a
  multi-word `urban:old school@host` can be classified). **Not** PR 3.
- **Changes:** Implement the expansion predicate. Reject candidates that
  contain ASCII `"`, `'`, or backtick so malformed shell-like syntax is
  never widened. Do not change `findCueWord` / `cueKind`. Do not accept a
  two-`@` expanded span. Leave `loginRe` untouched; add the harvest
  non-regression.
- **If this merges before PR 3:** cued quoted lines such as
  `finger "oslo, united states"@graph.no` retain today's delimiter-bounded
  link until PR 3 recognizes the accepted production. Malformed quote and
  backtick placements already retain that fallback permanently.

### PR 3 — quote grouping

- **Title:** `feat(tui): group shell-quoted finger queries as one link`
- **Files:** `tui/links.go` (quote-production matcher tried **before**
  expansion; `isDelim` unchanged), `tui/links_test.go` (tables 5 and the
  quote decline rows of table 7)
- **Depends on:** PR 1 only. Uncued quotes never use expansion. Cued
  quotes use `findCueWord` as shipped, not the PR 2 expander.
- **Changes:** Match only the stated production. `classifyAtToken`
  receives unquoted `query@host`; `Link.Raw` keeps the quotes.
  Precedence: valid quote production, malformed ASCII quote/backtick
  fallback, then quote-free expansion. Punctuation tests that rely on
  quotes as URL delimiters stay green.

# Bookmark last-visited dates — design

Implements [#108](https://github.com/jonathandeamer/lookit/issues/108): show a
last-visited date on bookmarked startpage rows, so the shelf reads as a record
of where you go rather than a list of addresses.

A bookmark today is a target and nothing else. This adds per-target state to
the bookmarks file — a file-format change, not a UI tweak — while keeping the
file's ingress guarantee intact: nothing unvalidated ever reaches the terminal,
and the file still needs no `sanitize` call.

## Decisions (settled with the user)

- **What counts as a visit:** a fetch that lands with `queryErr == nil`. Per
  `finger/` semantics this includes post-body connection resets flagged
  `Meta.Truncated`; it excludes dial errors and mid-body failures. Stamping
  happens for any route (reader or list) and on `r` refresh, but only when the
  target is already in the bookmarks file — lookit never writes the file for
  unpinned targets and never adds a record as a side effect of visiting.
- **Where it renders:** the note column of `sourceBookmark` rows. The
  ownership-marker work (#97 / PR #110) means bookmark rows never carry a
  catalog note, so the column is free. No new layout machinery; catalog rows,
  structural parents, and service children are untouched.
- **Unknown date renders blank**, per the answer #97 settled for notes: lookit
  does not narrate its own ignorance on the user's shelf. No `—`, no `never`.
- **Relative and fuzzy** format: `today`, `yesterday`, `N days ago` (N < 30),
  `N months ago` (N < 12), `N years ago`. A future timestamp (clock skew or
  hand-edit) clamps to `today`.
- **Recency ordering of the shelf is out of scope** — a separate change and its
  own issue, as #108 states. File order remains the user's own.
- **Storage: a second field on the bookmark line** (approach A below), matching
  the issue's framing that this is a bookmarks file-format change.

## Approaches considered

**A. Second field on the bookmark line (chosen).** Grammar becomes
`<target> [<RFC3339 timestamp>]`. One file remains the whole truth of the
shelf; the strict-parse rule keeps the no-`sanitize` guarantee true.

**B. Separate state file** (e.g. `~/.config/lookit/visited`). Bookmarks grammar
untouched, but: a second ingress needing the same validation discipline, drift
between the two files (unpinned targets leave stale state), and the
hand-edited file stops being the whole story of the shelf.

**C. Date as a trailing `#` comment.** Zero grammar change, but comments are
currently "preserved but never parsed or displayed" — this inverts that
contract and puts free text nearest to the terminal.

## File format & ingress (`tui/bookmarks.go`)

**Grammar.** A record is `<target>` or `<target> <timestamp>`, timestamp in
strict RFC 3339 UTC (`2026-08-14T15:04:05Z`, seconds precision). One field
means *date unknown*.

**Parsing.** `parseBookmarkTarget` accepts one or two fields; anything else is
still refused. A two-field record whose second field fails
`time.Parse(time.RFC3339, …)` keeps the bookmark and **silently drops the
date** — the issue's cheapest-guarantee rule. The raw text is never rendered;
only a validated `time.Time` derives display text, so the ingress note in
`CLAUDE.md`/`AGENTS.md` stays true. `bookmarkFile` gains a
`visited map[string]time.Time` keyed by target; duplicate records for one
target keep last-wins semantics, matching how the rest of the file treats
repeats.

**Round-trip identity.** `validateBookmarkRecordTarget` extends: a record
round-trips if parse → canonical re-emit (`target` alone, or
`target + " " + ts`) reproduces it. The add path validates target-only records
exactly as today.

**Write path.** New `updateBookmarkLine(data, target, ts)` — the first
operation that *rewrites* an existing record rather than adding or removing a
whole line. It rewrites only valid records matching the target (all
duplicates, consistent with `deleteBookmarkLine`), preserves each rewritten
line's trailing `#` comment text, and leaves comments, malformed lines, blank
lines, directives, and ordering byte-identical. Writes go through the existing
atomic `saveBookmarkData` (temp file + rename, 0600).

**Write failure degrades silently.** A read-only or full filesystem means the
date simply does not advance. Navigation is never blocked by bookkeeping, and
no error is surfaced — the screen shows the in-memory date for the session and
the file stays stale.

## The unknown-date scenario

All of these render blank, and none is an error:

- a one-field record (never visited, or a pre-feature bookmarks file)
- a malformed date field (silently dropped at parse time)
- a failed write (date never reached the file)
- the seeded `jonathan@tilde.team` first-run bookmark

## Recording a visit (`tui/request.go` / `tui/app.go`)

When a fetch lands with `queryErr == nil` and the landed target is in the
bookmarks file:

1. The in-memory date for that target updates immediately (screen is correct
   for the session even if the write fails).
2. `updateBookmarkLine` + `saveBookmarkData` attempt the file write; failure is
   swallowed per above.

The stamp is the wall clock at landing, in UTC. The clock is injectable
(`nowFn` package var, the `bookmarksPathFn` pattern) so tests control time.

## Rendering (`tui/start.go`, `tui/sections.go`)

- The date occupies the note column of `sourceBookmark` rows, dim, capped by
  the existing 48-cell note cap.
- Column measurement must include the date alongside the `★`-prefixed target
  so the marker work in PR #110 keeps its gap math correct (byte-vs-cell
  caution from that PR applies: measure display cells, not bytes).
- Relative text derives from the validated `time.Time` against the injected
  clock: `today` / `yesterday` / `N days ago` (N < 30) / `N months ago`
  (N < 12) / `N years ago`; future timestamps clamp to `today`.
- **Filtering:** the date is *excluded* from `FilterValue`. Filtering stays
  about identity (target, catalog notes); "3 days ago" matching a filter would
  be noise. Bookmark rows filter on target only, as today.

## Testing

All offline with injected fakes, per project convention:

- parse one- and two-field records; malformed date dropped silently with the
  bookmark kept; three fields still refused
- round-trip validation for both record forms
- `updateBookmarkLine`: rewrites only matching records; preserves trailing
  comments, blank lines, directives, and ordering; updates duplicate records
- stamping: on success, not on error, not for unbookmarked targets, on refresh;
  a failing write does not break the landing
- rendering table: each fuzzy bucket, future-clamp, unknown → blank,
  marker-aware column measurement
- `docs/tui-review` fixtures stay dateless: dynamic "N days ago" text would
  break tape `Wait`s, so rendering is verified by unit tests, not stills

## Docs

The ingress note in `AGENTS.md`/`CLAUDE.md` ("records carry a target and
nothing else") is updated to describe the two-field grammar and the
strict-parse guarantee, including the new rewrite operation on the write path.

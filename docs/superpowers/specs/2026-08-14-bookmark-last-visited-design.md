# Bookmark last-visited dates — design

Implements [#108](https://github.com/jonathandeamer/lookit/issues/108): show a
last-visited date on bookmarked startpage rows, so the shelf reads as a record
of where you go rather than a list of addresses.

A bookmark today is a target and nothing else. This adds per-target state to
the bookmarks file — a file-format change, not a UI tweak — while keeping the
file's ingress guarantee intact: nothing unvalidated ever reaches the terminal,
and the file still needs no `sanitize` call.

**Depends on [#112](https://github.com/jonathandeamer/lookit/pull/112)**, which
suppresses the inherited catalog note on `sourceBookmark` rows. That is what
frees the note column; without it there is nowhere to render a date. #112 also
settles two rules this design has to respect rather than re-decide: the cursor
does not lift a pinned row's silence, and a flattening filter does.

## Decisions (settled with the user)

- **What counts as a visit:** a fetch that lands with `queryErr == nil`. Per
  `finger/` semantics this includes post-body connection resets flagged
  `Meta.Truncated`; it excludes dial errors and mid-body failures. Stamping
  happens for any route (reader or list) and on `r` refresh, but only for a
  target already recorded in the bookmarks file — lookit never writes the file
  for unpinned targets and never adds a record as a side effect of visiting.
- **Where it renders:** the note column of `sourceBookmark` rows, in the
  unflattened startpage. #112 leaves that column empty, so this adds no layout
  machinery; catalog rows, structural parents, and service children are
  untouched. **Under a flattening filter the catalog note returns and the date
  stands down** — see "Rendering" for why that direction, and not the reverse.
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

**Parsing.** `parseBookmarkTarget` accepts one or two fields; three or more is
still refused. A two-field record whose second field fails
`time.Parse(time.RFC3339, …)` is **refused as a malformed record and reported
through the existing `problems` / `startNotice` path** — it does not become a
dateless bookmark.

> **This reverses the earlier decision** ("keep the bookmark, silently drop the
> date"). Silent acceptance would convert lines the parser refuses today into
> live bookmarks with no notice: `@example.com rm -rf` currently reaches the
> user as a reported problem, and under a two-field grammar with a lenient
> second field it would quietly become a bookmark instead. The file's existing
> contract is that a line lookit cannot understand is surfaced, never guessed
> at, and a hand-edited date is exactly the case where the user wants to be
> told. The cost — a typo'd timestamp costs the bookmark until it is fixed —
> is paid loudly and is recoverable from the notice.

The raw text is never rendered; only a validated `time.Time` derives display
text, so the ingress note in `CLAUDE.md`/`AGENTS.md` stays true.

`parseBookmarkTarget` returns the parsed timestamp alongside the target
(`(string, time.Time, error)`, zero time meaning "no date"). It has three
callers and all three are in scope: `parseBookmarks` stores the date,
`deleteBookmarkLine` and `validateBookmarkRecordTarget` ignore it and keep
comparing targets exactly as they do now.

`bookmarkFile` gains `visited map[string]time.Time` keyed by target.
**Duplicate records:** rows and counts stay per-line, exactly as today
(`buildSections` does not dedupe, and CLAUDE.md records that as an invariant).
The map is per-target, so N duplicate rows of one target show one date. In
practice they converge by construction, because the write path rewrites every
matching record; they can differ only in a hand-edited file, where last-wins
applies. This is a property of the date being per-target while rows are
per-line — not a claim that the file dedupes.

**Round-trip identity.** Two distinct checks, because they have two callers:

- `validateBookmarkRecordTarget(target)` is unchanged and keeps its only
  caller, `toggleBookmark` (`app.go`), which passes a bare `Target.Raw`. The
  add path still writes target-only records.
- The write path gets its own guard: before rewriting, the emitted record
  (`target + " " + ts`) must parse back to the same target and the same instant.
  A record that fails is left untouched rather than written. This is what keeps
  a rewrite from producing a line the parser would later refuse.

**Target matching is exact string equality on the record's target**, the same
rule `toggleBookmark` and `deleteBookmarkLine` already use. `@tilde.team` and
`tilde.team:79` are therefore different bookmarks, and visiting one does not
stamp the other. That is existing bookmark behaviour, not a new rule — but it
is now visible, so it is stated rather than left implicit.

**Write path.** New `updateBookmarkLine(data, target, ts)` — the first
operation that *rewrites* an existing record rather than adding or removing a
whole line. It:

- rewrites only valid records whose target matches, all duplicates included,
  consistent with `deleteBookmarkLine`;
- **splices on raw byte offsets**: it replaces only the span from the start of
  the target field to the end of the record, and copies everything from the
  first `#` onward byte-identical, along with the line's original leading
  whitespace. `stripComment` cuts at the first `#` and the parse path
  `TrimSpace`s, so reconstructing a line from parsed fields would silently
  normalise the user's spacing — this is the first operation for which that
  would be a visible edit;
- leaves comments, malformed lines, blank lines, directives, and ordering
  byte-identical;
- writes nothing at all when no record matches. **That is also the
  "is it bookmarked?" test** — there is no separate lookup, no in-memory
  bookmark set, and no read of the file beyond the one the write already needs.

Writes go through the existing atomic `saveBookmarkData` (temp file + rename,
0600).

**Write failure degrades silently.** A read-only or full filesystem means the
date simply does not advance. Navigation is never blocked by bookkeeping and no
error is surfaced.

**There is no in-memory date overlay.** The bookmarks file is the single source
of truth for dates, read by `loadBookmarks` inside `reloadStart`, which rebuilds
the startpage from disk on construction, after every bookmark write, and on
every return to Start via `gotoStart`. An overlay would be discarded by the next
`reloadStart` anyway, so the earlier claim that "the screen is correct for the
session even if the write fails" could not have held: if the write fails, the
date does not advance on screen either. That is the honest behaviour and it
needs no code.

## The unknown-date scenario

All of these render blank, and none is an error:

- a one-field record (never visited, or a pre-feature bookmarks file)
- a failed write (date never reached the file)
- the seeded `jonathan@tilde.team` first-run bookmark

A malformed date field is *not* in this list: it is a reported problem, per the
parsing rule above.

## Recording a visit (`tui/request.go` / `tui/app.go`)

When a fetch lands with `queryErr == nil`: load the bookmarks data, call
`updateBookmarkLine` with the landed target and the current time, and save if
anything changed. An unpinned target matches no record, so the file is not
written. Failure at any step is swallowed.

The stamp written to the file is UTC, per the grammar. **Day buckets are
computed in the user's local zone**: the stored instant is converted with
`time.Local` before `today` / `yesterday` / `N days ago` are derived. Bucketing
in UTC would tell a user in UTC-8 that an evening visit happened `today` all
through the following morning, and a user in UTC+ that an hour-old visit was
`yesterday`. The buckets are calendar-day differences in local time, not
divisions of an elapsed duration.

The clock is injectable (`nowFn` package var, following the `bookmarksPathFn`
pattern) so tests control both the stamp and the buckets.

## Rendering (`tui/start.go`)

All of it lands in `startRowNote`, which #112 already made the single place
that decides what the note column says. Nothing else in the render path moves.

- A `sourceBookmark` row shows its **date** where #112 leaves it blank —
  including under the cursor, since #112 settled that selection does not lift a
  pinned row's silence.
- **Flattened, the catalog note returns and the date stands down.** #112
  restores the note under a filter precisely so `/` stays honest: `FilterValue`
  remains `target + " " + note`, and `splitStartMatches` runs *only* when
  flattened, so a note-scored match is always visible in the row that matched.
  Rendering the date there instead would put unmatched text under offsets
  computed against the note and shift every highlight. Keeping the swap in this
  direction means no `FilterValue` change and no offset arithmetic.
- **`FilterValue` is unchanged, and the date is not in it.** Filtering stays
  about identity; "3 days ago" matching a filter would be noise. (The earlier
  claim that bookmark rows "filter on target only, as today" was wrong in both
  halves: they filter on target *and* the inherited catalog note, and after #112
  that is load-bearing rather than incidental.)
- **Column measurement is untouched.** `startTargetColumn` measures
  `startRowTarget` only and the note column is the remainder from
  `startColumnWidths`; the date lives in the note column, so it must *not* enter
  target measurement — doing so would push the gutter right down the whole page
  for text that is not in that column, which is the regression #92/#106 fixed.
- **No 48-cell cap applies.** `startNoteMaxCells` lives in `catalog_test.go` and
  gates the maintainer-authored `catalog.txt`; nothing enforces it at runtime.
  Date text is short, and runtime truncation is `ansi.Truncate` at the computed
  note width, the same as any note.
- Relative text derives from the validated `time.Time` against the injected
  clock, in local time: `today` / `yesterday` / `N days ago` (N < 30) /
  `N months ago` (N < 12) / `N years ago`; future timestamps clamp to `today`.
  Rendered dim, as notes are.

## Testing

All offline with injected fakes, per project convention:

- parse one- and two-field records; a malformed date field is refused *and
  reported*, not silently accepted; three fields still refused
- `validateBookmarkRecordTarget` unchanged for the add path; the write path's
  own round-trip guard rejects a record that would not parse back
- `updateBookmarkLine`: rewrites only matching records; updates every duplicate;
  preserves trailing comments **including their original spacing**, blank lines,
  directives, and ordering; writes nothing when no record matches
- stamping: on success, not on error, not for unbookmarked targets, on refresh;
  a failing write does not break the landing; an exact-match-only test covering
  `@tilde.team` vs `tilde.team:79`
- rendering table: each fuzzy bucket, future-clamp, unknown → blank, date under
  the cursor, and **catalog note (not date) when flattened**, with the filter
  highlight still landing on the matched note text
- day bucketing in local time: a fixed instant read from two zones lands in
  different buckets
- **the stacked-layout section gap with a mixed shelf.** #112 made a pinned
  row's second terminal row always blank, and `headerNeedsBlank` keys the
  section gap off exactly that. A shelf holding one visited and one unvisited
  pin now takes both branches within one section. `rowEndsBlank` delegates to
  `startRowNote`, so the gap stays at one row either way — the test exists
  because "unknown date renders blank" is what makes the mixed case reachable.
- `docs/tui-review` fixtures stay dateless: dynamic "N days ago" text would
  break tape `Wait`s, so rendering is verified by unit tests, not stills

## Docs

The ingress note in `AGENTS.md`/`CLAUDE.md` ("records carry a target and
nothing else") is updated to describe the two-field grammar, the strict-parse
guarantee (a bad date is a reported problem, not a silent drop), and the new
rewrite operation on the write path. The startpage paragraph gains the date's
place in the note column and its stand-down under a filter, alongside the
suppression rule #112 records there.

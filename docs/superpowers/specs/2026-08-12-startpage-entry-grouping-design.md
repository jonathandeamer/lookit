# Startpage entry grouping

The startpage renders its rows in `catalog.txt` file order — nothing sorts them
(`tui/sections.go:58-76`; the only `sort` call in `tui/` is `links.go:159`, for
link positions). That was invisible while the catalog was small and freshly
authored. Two problems have surfaced since:

- **The order reads as arbitrary.** Nothing on screen explains why `@plan.cat`
  sits above `@tilde.team`. The order is editorial, but the editorial principle
  lives nowhere the reader can see, so it looks random.
- **SERVICES doesn't scan.** Seventeen rows run without a break, and the target
  column repeats `bbs.airandwave.net` seven times and `happynetbox.com` five,
  crowding out the token that actually differs.

A third symptom follows from the same cause: adding a catalog line means
guessing where it goes, and nothing catches a bad guess.

This spec restructures **display order and grouping only**. It changes no
catalog grammar, no bookmarks file format, and no network behaviour.

## Decisions

Ordering and grouping are **computed at assembly time**, not declared in the
catalog and not inherited from file order.

### Communities: alphabetical by host

Six rows, sorted on the host with any `user@` prefix ignored, so
`ring@thebackupbox.net` files under `t`:

```
COMMUNITIES ──────────────────────────────────
@cosmic.voyage          Collaborative science…
@happynetbox.com        .plan files updated vi…
@plan.cat               Simple .plan hosting, …
ring@thebackupbox.net   The finger ring — join…
@tilde.team             Small public access un…
@zaibatsu.circumlunar…  A small pubnix; it cal…
```

Alphabetical answers "why this order" completely, costs no rows, and a test can
hold it. Two alternatives were considered and rejected:

- **Curated, newcomer-first** (busiest first, jumping-off points last) — a
  defensible order, but it needs a written principle to stop being arbitrary,
  and every future entry becomes a judgement call.
- **Sub-grouped by character** — the best-evidenced cut is four groups over six
  rows (SHARED MACHINES: tilde.team, zaibatsu; .PLAN HOSTING: plan.cat,
  happynetbox; COLLABORATIVE FICTION: cosmic.voyage; DIRECTORIES: the ring),
  costing four rows for two groups of two and two singletons. Note that
  `@cosmic.voyage` cannot honestly join "shared machines" on finger evidence:
  `luna`'s record shows `Ships registered to user`, `Project`, `Plan` and
  `Pronouns` but no `Directory:` or `Shell:`, unlike zaibatsu's (`/bin/mksh`,
  `/bin/dash`). Grouping pays off where clusters are real; in COMMUNITIES they
  are not.

### Services: host parent, indented children

```
SERVICES ─────────────────────────────────────
@bbs.airandwave.net   Over two dozen service…
  dict                Dictionary lookup — di…
  quake               Latest earthquakes, M2…
  sudoku:easy         An easy sudoku, fresh …
  urban               Slang, internet terms …
  weather             Current weather and a …
  wordsearch:today    Daily word search puzz…
@flanigan.us          Four fingers: bonsai, …
  calendar            Today's date, across t…
@graph.no             Weather worldwide by p…
@happynetbox.com      .plan files updated vi…
  1                   Interactive fiction, o…
  bot                 Tech news headlines wi…
  browserversion      Current version number…
  originsfinger       Les Earnest tells how …
  random              Jump to a random happy…
@typed-hole.org       A small menu of finger…
  cyoa                Choose your own advent…
  smog                Saturday Morning Gemzi…
  textfile            A lucky dip into textf…
```

**The grouping rule.** An entry's host is the text after its final `@`. Entries
sharing a host form a group; the parent is the entry whose query is empty
(`@host`), children are the rest. Hosts sort alphabetically, children sort
alphabetically by query token, the parent always leads. A host with a root and
no children (`@graph.no`) renders as a lone row — no indent, no group.

**Children show only what differs.** The target column carries `dict`,
`wordsearch:today`, `1` — the host is stated once, by the parent above. This
reclaims roughly twenty characters of target column per child row.

### The catalog gains two roots, and an invariant

**Invariant: any host with children must carry a root entry.** No group may be
headed by a phantom. `TestCatalogIsWellFormed` enforces it, so an orphaned child
fails the build gate rather than shipping compiled into a binary.

Two entries satisfy it today. Both were probed on 2026-08-12 and both serve a
real menu:

| New entry | Note | Basis |
|---|---|---|
| `@typed-hole.org` | A small menu of fingers, from lobste.rs to smog | server lists `username`, `feed`, `lobsters`, `weather`, `temp`, `cyoa`, `textfile`, `smog` under *"Available fingers:"* |
| `@flanigan.us` | Four fingers: bonsai, ping, wisdom, calendar | server: *"Try @bonsai @ping @wisdom @calendar"* |

`@flanigan.us` also reveals three services the catalog does not carry
(`bonsai`, `ping`, `wisdom`). Surveying them is **out of scope here** — this
spec adds roots because the invariant requires them, not to grow the catalog.

### `@happynetbox.com` appears in both sections

It is a community *and* the natural parent of five services. The same row is
rendered in both places: once under COMMUNITIES as a listing, once as the parent
of its services group. Same target, same note, two jobs.

Rejected: a non-selectable `happynetbox.com` label for that one group, which
would make Enter work on some group headings and not others; and moving it out
of COMMUNITIES, which would stop filing the catalog's best "go read strangers'
plans" host as a community.

### File order stops mattering

Sorting happens in `buildSections`, so `catalog.txt` can stay grouped by host
for a human editor while the rendered order is guaranteed. A new line may be
appended anywhere. This is what retires the "where does this line go" problem;
the alphabetical rule alone would not, since the file would still have to be
kept in order by hand.

## Rendering

Two rules that do not follow from the layout:

**Under an applied filter, children render their full target again.** Filtering
already drops non-selectable rows and flattens the view to matches, so a bare
`dict` would float with no parent to give it a host. `FilterValue()` is
unchanged — still `target + " " + note` — so typing `airandwave` still matches
every child, and match highlighting is unaffected.

**Every flattened view shows a target once.** While a filter is active the
structural parent copy is dropped, keeping only the canonical listing row. This
matters because filtering removes the section headers that distinguish the two
`@happynetbox.com` copies: without the rule, a `happynetbox` filter would show
two adjacent, identical, both-selectable rows with identical `FilterValue()`,
and a pinned parent would show a third. Unfiltered, the copies stay — their
headers supply the context that filtering removes.

Concretely: a parent copy is dropped from a flattened view when the same target
already appears as a listing row (its own catalog section, or BOOKMARKS).
Parents that *are* their section's listing — `@bbs.airandwave.net`,
`@graph.no`, `@typed-hole.org`, `@flanigan.us` — are canonical and always
survive.

**Counting is by distinct target, and the status bar agrees.** The layout-polish
spec already fixes the rule: counts "describe the assembled rows after
bookmark/catalog deduplication… no target is counted twice"
(`2026-08-12-startpage-layout-polish-design.md`). A structural duplicate must
not weaken that, so both totals count **distinct selectable targets**:

| Situation | Overview | Status bar |
|---|---|---|
| Unfiltered | `CATALOG 6 communities · 19 services` — the second `@happynetbox.com` row is not counted again | distinct targets, so the duplicate row is not counted |
| A child is pinned | BOOKMARKS rises, its classification falls, exactly as today | unchanged in total |
| A parent is pinned | BOOKMARKS rises, its classification falls; the retained parent copy adds nothing | unchanged in total |
| Filter applied | counts describe the visible rows, which by the rule above hold no duplicates | same |

This changes `startCounts` (`tui/start.go:92`), which counts displayed items by
section today, and the status-bar tally (`tui/app.go:1496-1503`), which counts
visible selectable rows. Without the second change the bar would read one higher
than the overview whenever a duplicate is on screen.

Indentation is two spaces inside the target column. In the narrow two-line
layout the same indent prefixes both the target line and the note line.

## Behaviour

**Parents are ordinary rows.** Enter on `@bbs.airandwave.net` fingers the host
root exactly as today. Only section headers remain non-selectable, so
`skipNonEntry` and the cursor logic are untouched.

**Bookmark dedup gains one exception: a parent row is structure, not a
listing.** Today a bookmarked target is suppressed from its catalog section so
it cannot appear twice. Applied to parents, that would decapitate a group —
pinning `@bbs.airandwave.net` would leave its six children orphaned. The rule
becomes: pinning suppresses the *listing* copies, never the *parent* copy.

This also resolves the dual-role case under the same rule rather than a special
case: `b` on `@happynetbox.com` pins it once, removes the COMMUNITIES copy, and
leaves the SERVICES parent heading its five services.

**Bookmarked-ness becomes its own field, separate from `source`.** A retained
parent copy is `sourceCatalog` even when its target is bookmarked, and
`startBookmarkAction` (`tui/app.go:1509`) derives the `b` hint from
`entry.source == sourceBookmark` while `toggleBookmark` (`tui/app.go:373`)
decides by scanning the bookmarks file for the target. Left alone, a pinned
parent would advertise `b bookmark` and then *remove* the bookmark — the hint
lying about what the key does.

So `startEntry` gains a `bookmarked bool`, set during assembly from the
bookmarks file, and the hint reads that. `source` keeps its existing meaning:
which section rendered this row. The two were the same fact until parents
started outliving dedup; they are now distinct and must be stored distinctly.

**Children pin as themselves.** `b` on `dict` writes the full
`dict@bbs.airandwave.net` to the bookmarks file — the format does not change —
and the row moves to BOOKMARKS showing its full target, having no parent there.
It is suppressed from its group as today.

**BOOKMARKS stays flat and in file order.** No grouping, no sorting: that
section is the user's own list in their own order, and `b` appends. Sorting it
would overrule them.

## Testing

- `buildSections`: parent-first assembly, alphabetical hosts and children, a
  root with no children rendering as a lone row, the dual-role duplicate
  appearing in both sections, and the dedup exemption in both directions.
- `TestCatalogIsWellFormed`: the orphan check — a child whose host has no root
  entry fails the build.
- Delegate: a child renders its bare token unfiltered and its full target under
  an applied filter, with match highlighting intact in both.
- Flattening: a `happynetbox` filter yields exactly one `@happynetbox.com` row,
  both when it is unpinned and when it is pinned; the canonical parents
  (`@bbs.airandwave.net` and the rest) always survive a filter that matches
  them.
- Bookmark hint: `b` on a parent reads "bookmark" when unpinned and "remove"
  when pinned, and the key does what the hint says in both states — the
  regression this design would otherwise introduce.
- Counts: the duplicated row does not inflate `CATALOG` **or** the status-bar
  tally, and the two agree in every row of the table above.
- Existing startpage tests encode today's flat order and will need updating —
  `TestBookmarkingCatalogRowsStayAtSectionOrdinal` and its neighbours assert
  section ordinals against the current file order.

## Deliberately out of scope

- **Collapsible groups.** SERVICES grows to 20 rows plus a header (19 service
  entries, plus the `@happynetbox.com` parent), which is more
  scrolling than today on a short terminal — but the parent rows give the eye
  landmarks to scroll *between*, which is the scannability win being sought.
  Collapsing adds a keybinding, a persistence question and state to restore
  across history, for a list this size.
- **Sub-grouping COMMUNITIES**, per the alternatives above.
- **Surveying the three uncatalogued `@flanigan.us` services.**
- **Any change to `catalog.txt` grammar, the bookmarks format, or `finger/`.**

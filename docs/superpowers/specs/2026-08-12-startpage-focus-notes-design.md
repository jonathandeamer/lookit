# Startpage focus notes

The child-hints work
([design](./2026-08-12-startpage-child-hints-design.md)) made a service child a
bare token and gave it an optional short hint from the catalog, so an opaque
token like `smog` could still say something. Seven children earned one.

Two problems showed up once it was built:

- **The hint has no visual identity.** It renders in `NormalDesc`, the same
  style as any other note — the final review proved the "dim the hint" block was
  a no-op, because `NormalDesc` is already the dim colour. So a hint is just a
  shorter note, and the row it sits on looks like every other row with a note.
- **Two of the seven hints are the note's first clause**, which the hints
  design's own authoring rule forbids. The rule and the copy could not both
  stand.

Rather than invent a visual treatment or relitigate the copy, this spec removes
the hint. **A child's note column is empty until the row has focus.** The
description a hint was approximating is the note itself, shown where it is
wanted and nowhere else.

It also fixes the reason a short form looked necessary: some notes are too long
for the column they land in.

## Decisions

### The hint grammar is removed, not disabled

A catalog line returns to `<kind> <target> <note>`. Removed: the ` | <hint>`
delimiter and `splitCatalogNote`; `catalogNotePipeLines` and its fixture
self-test; `hintIsMisplaced`, `TestCatalogHintsOnlyOnServiceChildren` and
`TestCatalogHintValidationRejectsNonChildren`; the `hint` field on `startEntry`;
the hint branch in `startItem.FilterValue`; and the seven hints in
`tui/catalog.txt`.

Leaving a dormant field in a data file that ships compiled in would be a trap:
the next person to edit the catalog would find a documented grammar that
nothing reads, and the guards protecting it would keep running for no reason.

`splitStartMatches` also loses the `note` parameter added in the hints work. It
existed to drop match offsets landing in the appended hint; with `FilterValue`
back to `target + " " + note`, there is no third field and no offset to clamp.

**What survives** is everything that carried the design: the `├`/`└`
connectors, `lastChild`, the column-5 indent, and a note column that changes
with focus.

### A child's note appears only on focus

`startRowNote` becomes one rule: a child shows its note when selected or
flattened, and nothing otherwise. Everyone else always shows their note.

| Row | Note column |
|---|---|
| Host parent, community row, structural parent | full note |
| Child, unselected | empty |
| Child, selected | full note |
| Child in a flattened view (filter active, query non-empty) | full note |
| Bookmarked child in BOOKMARKS | full note |

**"Selected" means the cursor's row, whether or not content has keyboard
focus.** The two are distinct states in this app: when the target input is
focused, the list keeps its selected row and renders it with the inactive shelf
(`tui/start.go`, the `showShelf`/`contentFocused` branch). The note appears in
that state too. Tying it to `contentFocused` instead would make a child's note
blink out when the user clicks into the input and back when they leave it —
motion that says nothing about the row. Selection is the cursor's position;
focus is which pane takes keys.

The word "focus" in this spec's title means the cursor resting on a row, not
`contentFocused`. The rule is expressed in terms of **selection** everywhere it
matters.

```
SERVICES ─────────────────────────────────────
@bbs.airandwave.net   Over two dozen services…
   ├ dict
   ├ quake             Latest earthquakes, M2.5+ past day   ← selected
   ├ sudoku:easy
   ├ urban
   └ wordsearch:today
@flanigan.us          Four fingers: bonsai, ping, wisdom, calendar
   ├ bonsai
   └ calendar
```

### Notes are capped at 48 characters

A build-gate test caps **every** catalog note at 48 **terminal cells**. The cap
covers community and root notes too, because those are on screen at all times,
and it holds future copy to the same rule rather than relying on someone
remembering it.

**The guarantee is scoped: no truncation at 100 columns or wider.** The note
column is half the width less the frame, so 100 columns yields about 48 cells —
comfortable on any laptop terminal, including a half-screen split. Below that it
is not a promise and cannot be: the wide single-line layout starts at
`startWideMinWidth` (72), where the note column is roughly 35 cells, and at the
classic 80 columns it is about 39. A 48-cell note still truncates below roughly
97 columns, exactly as every note does today. The cap makes a stated width safe;
it does not make truncation impossible.

**The unit is display width, not runes or bytes.** Rendering truncates by
terminal cells (`ansi.Truncate`), and a 48-rune note can occupy more than 48
cells — a CJK character or an emoji takes two. The test measures with
`ansi.StringWidth` (or `lipgloss.Width`), the same accounting the renderer uses.
Counting runes would pass copy that still truncates; counting bytes would reject
the em dashes and the typographic apostrophe in `Today’s date, across the
years`.

The 50/50 column split is unchanged. Narrowing the target column was considered
— with children as bare tokens it is mostly empty — but it would move every
section's layout to solve a copy problem, and the flattened view still needs
room for a full target (`wordsearch:today@bbs.airandwave.net` is 35 characters).

### Four copy changes

| Target | Was | Now |
|---|---|---|
| `ring@thebackupbox.net` | The finger ring — join by linking it from your response (55) | **A webring, for finger** (21) |
| `@graph.no` | Weather worldwide by place name — finger oslo@graph.no (54) | **Weather worldwide by place name** (31) |
| `@tilde.team` | Small public access unix, for teaching and learning (51) | **Small public access unix, for learning** (38) |
| `weather@bbs.airandwave.net` | Current weather and a 7-day forecast — weather:city@… (53) | **removed from the catalog** |

`@tilde.team` still quotes its own banner, which reads *"we're a small public
access unix system with a goal of teaching and learning"* — the rewrite drops
two words from that quotation and keeps it traceable.

`weather@bbs.airandwave.net` goes rather than shrinks: `@graph.no` already
serves weather worldwide, and the catalog has settled this case before —
`david@netbros.com` was dropped as a duplicate of `david@collantes.us`, and four
`weather:ZIP` services collapsed into one entry. Nothing is hidden by the
removal: `@bbs.airandwave.net`'s own response advertises `weather` in its menu,
and its note still reads "Over two dozen services, from news to sudoku".

Both rewrites drop an inline usage example (`finger oslo@graph.no`,
`weather:city@…`). That is a deliberate trade: the syntax is in each service's
own response, and the startpage's job is to say what a thing is.

After these, four notes sit at 47 cells — `bot@happynetbox.com`,
`@zaibatsu.circumlunar.space`, `@typed-hole.org` and `@cosmic.voyage` — one
below the cap. That margin is thin by accident rather than design: the audit
that produced this list measured **every** note by display width, which is how
`@tilde.team` surfaced after an earlier pass over service children alone missed
it. Re-run that audit, not a spot check, when adding copy.

## Consequences

- The catalog holds **19 services and 25 entries** (from 20 and 26), four
  service roots and **15 children**. The rendered SERVICES section is **20
  rows**, including the structural `@happynetbox.com` copy.
- `bbs.airandwave.net` drops to five children — `dict`, `quake`, `sudoku:easy`,
  `urban`, `wordsearch:today`. `wordsearch:today` is still the last child, so no
  connector changes.
- Fixtures that move: the ordering list in `TestServicesGroupUnderHostRoots`;
  the four unfiltered scenarios in
  `TestOverviewAndStatusCountsExcludeStructuralCopies`, which become 19/25,
  18/25, 18/25 and 19/26; and any test naming a removed hint.
- `TestWideChildRowShowsHintNoteOrFullNote` (added by the final review's fix
  wave) is rewritten: its hinted-child case becomes an unselected child with an
  **empty** note column, keeping the selected-child case that proves the wide
  layout wires `rowNote` at all. That mutation coverage must survive the
  simplification — reverting the wide layout's `rowNote` to `item.entry.note`
  must still fail a test.
- `TestHintWordFindsItsChildButNotItsBookmark` is removed with the feature it
  covered.

## Testing

- **The cap:** every catalog note measures 48 cells or fewer under
  `ansi.StringWidth`. Include a fixture case proving the measure is display
  width — a note of 48 runes that is wider than 48 cells must fail the gate, or
  the test is measuring the wrong thing.
- **Focus behaviour:** an unselected child's note column is empty; the same row
  selected shows its full note; a non-child row always shows its note. At least
  one case asserted on rendered output in the wide layout, not only on the pure
  function.
- **Selection without content focus:** a selected child with `contentFocused`
  false — the inactive-shelf state — still shows its note. This is the state the
  rule's wording could most easily be read the other way, so it gets its own
  case.
- **Removal:** no catalog line contains `|`; `startEntry` has no `hint` field;
  `FilterValue` on a child equals `target + " " + note`.
- **Counts and ordering:** the fixtures above, at their new values.

## Out of scope

- **Narrowing the target column**, per the reasoning above.
- **Any new visual treatment for the note column.** The distinction now comes
  from focus, not colour.
- **Surveying `@bbs.airandwave.net`'s other services.** Removing `weather`
  leaves five catalogued; the host advertises far more, and choosing among them
  is separate work.

## Branching

This work continues on `feat/startpage-child-hints`, which is unmerged and
unshared. The simplification lands as new commits rather than a history rewrite:
the repo squash-merges, so the merged diff is identical either way, and
reshaping fifteen commits to hide an intermediate state invites mistakes it
cannot pay for.

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

A focused child's note must not truncate. The note column is half the width, so
at **100 columns** the budget is about 48 characters — comfortable on any laptop
terminal, including a half-screen split.

A build-gate test caps **every** catalog note at 48 characters. The cap covers
community and root notes too, because those are on screen at all times, and it
holds future copy to the same rule rather than relying on someone remembering
it.

The 50/50 column split is unchanged. Narrowing the target column was considered
— with children as bare tokens it is mostly empty — but it would move every
section's layout to solve a copy problem, and the flattened view still needs
room for a full target (`wordsearch:today@bbs.airandwave.net` is 35 characters).

### Three copy changes

| Target | Was | Now |
|---|---|---|
| `ring@thebackupbox.net` | The finger ring — join by linking it from your response (55) | **A webring, for finger** (21) |
| `@graph.no` | Weather worldwide by place name — finger oslo@graph.no (54) | **Weather worldwide by place name** (31) |
| `weather@bbs.airandwave.net` | Current weather and a 7-day forecast — weather:city@… (53) | **removed from the catalog** |

`weather@bbs.airandwave.net` goes rather than shrinks: `@graph.no` already
serves weather worldwide, and the catalog has settled this case before —
`david@netbros.com` was dropped as a duplicate of `david@collantes.us`, and four
`weather:ZIP` services collapsed into one entry. Nothing is hidden by the
removal: `@bbs.airandwave.net`'s own response advertises `weather` in its menu,
and its note still reads "Over two dozen services, from news to sudoku".

Both rewrites drop an inline usage example (`finger oslo@graph.no`,
`weather:city@…`). That is a deliberate trade: the syntax is in each service's
own response, and the startpage's job is to say what a thing is.

After these, the longest notes are `bot@happynetbox.com` and `@typed-hole.org`
at 47 characters — one below the cap.

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

- **The cap:** every catalog note is 48 characters or fewer, measured in runes,
  not bytes — `Today’s date, across the years` carries a typographic apostrophe
  and the em-dash notes would otherwise over-count.
- **Focus behaviour:** an unselected child's note column is empty; the same row
  selected shows its full note; a non-child row always shows its note. At least
  one case asserted on rendered output in the wide layout, not only on the pure
  function.
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

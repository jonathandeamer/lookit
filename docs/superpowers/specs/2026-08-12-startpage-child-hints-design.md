# Startpage child hints and group glyphs

Grouping services under their host root (see
`2026-08-12-startpage-entry-grouping-design.md`) fixed the ordering problem and
left a density one. Every row still carries a description, so the right-hand
column runs as an unbroken block of prose down the whole SERVICES section —
twenty lines of it, in which the host's own sentence and its children's
sentences carry identical visual weight. The eye has nothing to skip.

This spec makes a child row **a token by default**, gives the group's shape to
glyphs rather than to prose, and surfaces a child's full description only where
it is wanted: on the row the cursor is on.

It changes the catalog grammar, the delegate, and the catalog copy. It changes
no ordering, no counting, no bookmark behaviour, and nothing in `finger/`.

## Decisions

### A child may carry an optional short hint

The catalog grammar gains one optional field, delimited by ` | `:

```
service smog@typed-hole.org Saturday Morning Gemzine — back issues | gemzine back issues
service quake@bbs.airandwave.net Latest earthquakes, M2.5+ past day
```

Present, the hint is what the child row shows, dimmed. Absent, the row is its
token alone. The **full note stays authoritative**: it feeds filtering, the
selected row, and every context where the child renders as a listing rather
than as a group member.

Two rules join `TestCatalogIsWellFormed`, because the catalog ships compiled in
and a typo cannot be fixed without a release:

- `|` may not appear inside a note. This is the treatment `#` already gets: the
  parser would eat the rest of the line, so the grammar forbids the character
  rather than growing an escape.
- A hint on a **root** entry fails the build. Only children render as tokens, so
  a hint anywhere else is dead text that would silently never appear.

**Authoring rule.** A hint exists to rescue a token that does not say what it
is. It is lowercase, two or three words, and written for its slot — not the
first clause of the note. If a token is self-evident, it gets no hint; a hint
on every child would rebuild the wall this spec removes.

Of the nineteen service entries, four are host roots and fifteen are children.
By that rule seven of those fifteen earn a hint today, each traceable to the
entry's existing note:

| Entry | Hint |
|---|---|
| `cyoa@typed-hole.org` | pick-a-path stories |
| `smog@typed-hole.org` | gemzine back issues |
| `textfile@typed-hole.org` | random textfiles.com |
| `1@happynetbox.com` | interactive fiction |
| `bot@happynetbox.com` | tech news headlines |
| `random@happynetbox.com` | a random profile |
| `originsfinger@happynetbox.com` | how finger began |

The other eight — `dict`, `quake`, `urban`, `weather`, `sudoku:easy`,
`wordsearch:today`, `calendar` and `browserversion` — say what they are.

### Children are drawn with connectors, not indentation alone

A child's prefix becomes three spaces, a connector, and a space: `   ├ ` for
every child but the last of its group, `   └ ` for the last. Text lands at
column 5 instead of column 2. The connector takes the dim palette colour, so it
reads as rule-work rather than content.

```
SERVICES ─────────────────────────────────────
@bbs.airandwave.net   Over two dozen services…
   ├ dict
   ├ quake
   ├ sudoku:easy
   ├ urban
   ├ weather
   └ wordsearch:today
@flanigan.us          Four fingers: bonsai, p…
   └ calendar
@graph.no             Weather worldwide by pl…
@typed-hole.org       A small menu of fingers…
   ├ cyoa             pick-a-path stories
   ├ smog             gemzine back issues
   └ textfile         random textfiles.com
```

**Last-child detection happens at assembly.** The delegate renders one item at a
time and cannot see whether the next row belongs to the same host, so
`groupByHost` sets a `lastChild bool` beside the existing `child` and
`structural` flags. Deciding it at render time would mean reaching back into the
list from inside the delegate.

Rejected alongside: a blank line between groups (the connectors already bound
the group, and it cost four rows), and putting the host row in the gold accent
(the accent means "the section your cursor is in" elsewhere in this screen, and
overloading it would blur that).

### The selected child shows its full note in place

The highlighted child's note column swaps from its hint to its full note, in
normal weight. Nothing expands and nothing shifts.

This replaces a per-row expansion, which is not available: `bubbles` computes
`Paginator.PerPage` as `availHeight / (delegate.Height() + delegate.Spacing())`
(`list.go:793`) and pads short pages using the same fixed height
(`list.go:1233-1235`). One row rendering two lines while `Height()` reports one
overflows the page and corrupts that arithmetic. The uniform alternative —
every row two lines, second line blank unless selected — would halve the
density this spec exists to buy back.

## Rendering

The note column by row state:

| Row | Note column |
|---|---|
| Host parent, community row, structural parent | full note (unchanged) |
| Child, unselected, with a hint | the hint, dimmed |
| Child, unselected, no hint | empty |
| Child, selected | its full note, normal weight |
| Child under an applied filter | full note |
| Bookmarked child in BOOKMARKS | full note |

**Under an applied filter the connectors go away.** A filtered child already
renders its full target because its parent may be off screen; it now also drops
the connector and shows its full note, because it is a listing again rather than
a member of a visible group. The three states stay consistent: inside a group a
child is a token, in a flattened view it is an address.

**In BOOKMARKS a child has no parent**, so it renders as a listing there too:
full target, full note, no connector.

**The narrow two-line layout mirrors the wide one.** The first line carries the
connector; the second carries the hint (dimmed), the full note when selected, or
nothing. The second line aligns under the token at column 5, not under the
connector.

**Truncation is unchanged.** The prefix is part of the target string, so
`ansi.Truncate` and the column arithmetic in `startColumnWidths` treat it as
they treat any other characters. The prefix costs children three columns of
token width.

## Filtering

`FilterValue` becomes `target + " " + note + " " + hint` when a hint exists, so
everything visible on screen is matchable: typing `gemzine` finds `smog` whether
the word reaches the eye through the hint or through the note.

Match highlighting keeps its existing split against target and note. A match
index landing past the note region — inside the appended hint — is **dropped**
rather than mis-highlighted, the same defensive shape `splitStartMatches`
already uses for indices that fall in the separator.

## Unaffected

The overview and status-bar counts; `structural` rows and the rules that hide
them from flattened views; `bookmarked` state and the `b` hint; `i`/copy
actions; ordering; community rows; the reader.

## Testing

- **Parser:** ` | ` splits note from hint; an entry with no hint parses with an
  empty hint; `|` inside a note fails `TestCatalogIsWellFormed`; a hint on a
  root entry fails it too.
- **Assembly:** `lastChild` is true on exactly the final child of each group and
  false elsewhere, including the single-child group (`calendar@flanigan.us`
  takes `└`).
- **Delegate, one case per row state** in the table above: token-only child;
  hinted child, dimmed; selected child showing its full note; filtered child
  showing full target and full note with no connector.
- **Narrow layout:** the second line carries hint / full note / nothing, aligned
  to the token column.
- **Filtering:** a word that appears only in a hint finds its row, and highlight
  offsets stay within the target and note.
- **Existing test to update:** the child-indent differential test hardened in
  the grouping branch asserts child = sibling + 2 leading spaces. It becomes an
  assertion about the connector prefix — and stays differential, so it cannot
  pass vacuously if the prefix disappears.

## Deliberately out of scope

- **A hint on every child.** The authoring rule is the design; a hint per row
  would restore the wall.
- **A detail line under the list, or the note in the status bar.** Considered
  and dropped in favour of the in-place swap, which costs no rows.
- **A multi-column grid of child tokens.** The densest option, and the closest
  to what a finger host's own user list looks like, but `bubbles/list` selects
  vertically — a grid would mean hand-rolling selection, paging and filtering.
- **Any change to ordering, counting, or bookmark semantics.**

## Branching

This work stacks on `feat/startpage-entry-grouping`, which is complete but
unmerged. Branch and PR against that branch, not `main`, so it is reviewed
against the grouping it builds on.

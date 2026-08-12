# Startpage Layout Polish Design

**Date:** 2026-08-12

**Status:** Approved design; reviewed 2026-08-12 and revised — filtered overview
counts, single `BOOKMARKS` name, an even 50/50 column split, no per-row
bookmark marker, the catalog credit relocated to About, named layout constants,
delegate-focus and delegate-divergence notes, and the `b`-again trade-off stated
in full

**Scope:** Presentation and interaction polish for the bookmarks/catalog
startpage, plus the one About line the startpage hands off (see *The Catalog
Credit Moves to About*). This deliberately adds no new destinations, commands,
persistence fields, navigation modes, or network behavior.

**Builds on:**
[`2026-08-11-bookmarks-catalog-startpage-design.md`](./2026-08-11-bookmarks-catalog-startpage-design.md)

## Problem

The startpage is functionally complete, but its current two-line cells make the
first catalog section tall enough to hide later sections below the fold. A new
user can mistake the visible `COMMUNITIES` group for the whole catalog. The
screen also has three smaller usability problems:

- the input and list can both appear selected even though only one has focus;
- bookmarks and catalog classifications have insufficient visual hierarchy;
- toggling a bookmark can move selection to another section, turning a state
  change into an unexpected navigation jump.

The refinement should feel like a quiet Charm launcher: compact at normal
terminal widths, readable when narrow, keyboard-native, and explicit about
focus and ownership. It must preserve the existing startpage architecture,
bookmark grammar, filtering semantics, history model, and key assignments.

## Chosen Direction: Quiet Utility

The startpage remains a single `bubbles/list` with inline section headers, but
uses a responsive delegate:

- at **`startWideMinWidth` (72) columns and wider**, every item occupies one row,
  with the target in a left column and its description in a right column;
- below that width, every item occupies two rows, with the description stacked
  under the target.

Both columns truncate independently and never wrap. The wide target column takes
half the available width after the selected-row frame; the description gets the
remainder. The split is even because targets are the row's identity and the
notes are supporting context: at 80 columns the frame costs 2, leaving 39 per
column, which fits the longest catalog target
(`wordsearch:today@bbs.airandwave.net`, 35) whole. Targets are what the eye
scans and what `↵` acts on, so the layout is sized to never truncate one at the
ordinary terminal width; descriptions are authored prose and read fine clipped.
The narrow layout preserves the existing target-first, description-second
hierarchy.

This deliberately prefers density at the ordinary 80-column terminal while
preserving authored catalog context on narrow screens. Hiding descriptions on
narrow terminals was rejected: those notes are what make unfamiliar finger
destinations understandable and worth exploring.

The breakpoint and the column split are named constants
(`startWideMinWidth = 72`, `startTargetColumnPct = 50`). The same
`startWideMinWidth` governs both the delegate height and the overview's
one-versus-two-line form, so the two can never disagree about which mode the
screen is in.

**This is a deliberate reversal of a recorded decision.** `startDelegate`
currently defers every entry row to `userDelegate` so that "startpage rows look
exactly like user-list rows" (`tui/start.go`). A one-row two-column entry cannot
come from `list.DefaultDelegate`, so `startDelegate` takes over entry rendering
and that equivalence ends. Consequences to carry through: the selected row draws
**one** shelf spanning both columns, not one per line, so
`renderSelectedShelfLine` is applied to the composed row rather than to the
target and description separately; and user-list rows keep their current
two-line appearance, so the two screens now look deliberately different rather
than accidentally so.

Section headers occupy the same delegate height as entry rows so Bubbles'
uniform-height pagination remains valid. At wide widths a header is a compact
ruled label on one row; at narrow widths it may use two, with unused space left
blank rather than changing the delegate's per-item height. Headers are the only
remaining non-selectable row type — see below, where the catalog credit leaves
the list.

## Fixed Overview

A non-interactive overview appears between the target input and the scrolling
list. It describes two different levels of the information hierarchy:

```text
BOOKMARKS  2 │ CATALOG  8 communities · 15 services
```

`BOOKMARKS` is ownership, and reuses the exact name of the section it
summarises rather than introducing a second word for the same thing; because
the label already says "bookmarks", its count is bare. `communities` and
`services` are classifications inside lookit's catalog. They are not presented
as three peer categories, because that would make the catalog classifications
appear user-configurable.

The counts describe the assembled rows after bookmark/catalog deduplication.
When a catalog service becomes a bookmark, the bookmark count rises and the
service count falls; no target is counted twice.

Only non-empty catalog classifications appear in the overview, in section
order. If a future catalog adds people, the catalog group extends naturally to
`8 communities · 15 services · 3 people`; if pinning empties a classification,
that classification drops out along with its section.

At wide widths the overview is one line. When that line does not fit—always
below the `startWideMinWidth` layout breakpoint—it becomes two labelled rows:

```text
BOOKMARKS  2
CATALOG    8 communities · 15 services
```

The overview stays above the paginated list, so all available sections remain
discoverable even when a long section's heading is on an earlier page.

### Overview states

- With no bookmarks: `BOOKMARKS  none yet`. No empty bookmark section is
  inserted; the list starts with `COMMUNITIES`.
- With `catalog off`: only the `BOOKMARKS` group appears.
- With `catalog off` and no bookmarks: the existing file-level empty message
  replaces the overview and list, names the active bookmarks path, and explains
  how to turn the catalog back on.
- While actively typing a `/` filter, Bubbles' own filter input replaces the
  overview.
- **When a filter is applied, the overview returns above the flat filtered
  results and its counts describe the matching rows only**, dropping any group
  or classification the filter emptied (except when nothing matches at all —
  see below):

  ```text
  BOOKMARKS  1 │ CATALOG  3 communities
  ```

  The overview always counts exactly the rows the list is showing. The status
  bar's `N entries` already counts visible items, so an unfiltered overview over
  a filtered list would put two disagreeing totals on screen at once. Clearing
  the filter restores the assembled counts. If a filter matches nothing, the
  overview shows every group at zero — `BOOKMARKS  0 │ CATALOG  0` — above
  Bubbles' own `No entries.`, rather than vanishing.

## Focus and Visual Hierarchy

The existing violet/pink shelf remains the item-level selection treatment. The
overview gains an orientation cue marking which group the selection sits in —
gold **and bold**, since gold already means "focused link" elsewhere in the app
and the pairing keeps the cue from being colour-only:

- bookmark selected: `BOOKMARKS` is gold and bold;
- community selected: the `communities` count is gold and bold;
- service selected: the `services` count is gold and bold;
- person selected, if that future section exists: the `people` count is;
- target input focused: no segment is highlighted.

Only the relevant text changes foreground colour and weight. It receives no
background, border, or underline, so the overview reads as status rather than as
clickable tabs.

The weight is not decoration. On a continuation page the inline section header
has scrolled off, and in flat filtered results it does not exist at all, so the
overview segment is then the *only* indication of which section the selected row
belongs to — it cannot rely on hue alone on a monochrome or low-colour terminal.

Highlighting the inline section heading was rejected. It has better proximity
to the selected row, but disappears on continuation pages and during filtering.
Making it sticky would introduce another custom chrome layer that duplicates the
fixed overview.

Bookmark rows get **no per-row ownership marker**. A leading glyph was
considered and rejected: startpage rows carry no such marker today, adding one
spends target-column width on every bookmark row to restate what the
`BOOKMARKS` section header already says, and a screen of neutral glyphs is
exactly the visual noise this polish pass is meant to remove.

The cost is confined to flat filtered results, where section headers are absent
and a bookmarked row is then visually identical to a catalog row. Two signals
already cover it: the overview's highlighted segment names the selected row's
group, and the status bar reads `b remove` on a bookmark instead of
`b bookmark`. Both are focus-following rather than at-a-glance, which is the
accepted trade — ownership is a property you check before acting, not one you
need to scan a filtered list for.

When the target input owns focus, the list keeps its logical selection but
renders it with a neutral inactive treatment rather than the violet/pink shelf.
When focus moves into the list, the shelf and overview highlight appear. This
ensures only one component looks active at a time.

All colours continue to come from the existing adaptive palette. No new
hardcoded dark-only colours are introduced.

## The Catalog Credit Moves to About

The `Catalog inspired by` row leaves the startpage list and becomes a permanent
line in the About screen's identity block, beside the version, licence and repo:

```text
lookit v0.2.0 · MIT license
https://github.com/jonathandeamer/lookit
Catalog inspired by https://640kb.neocities.org/fingerverse/
```

The URL is written in full, scheme included, matching the repo line directly
above it and the existing `aboutRepo`/`aboutIssuesURL` constants. It keeps its
OSC 8 hyperlink — `aboutView` is a pure string function, so `lipgloss.Hyperlink`
composes there exactly as it does in the delegate today. It gets no key action:
`y` stays bound to the issues URL, since the credit is attribution, not a
workflow.

Three reasons, in order of weight:

1. **The current placement is not honest in every state.** `newStart` appends
   the credit only when some row has `source == sourceCatalog`. A bookmarked
   target that matches the catalog becomes `sourceBookmark` while *keeping the
   catalog's authored note*. So with `catalog off` and note-borrowing bookmarks,
   lookit displays catalog-derived prose and shows no credit at all. An
   unconditional About line is simply true: the catalog ships in the binary
   whether or not it is on screen.
2. **A trailing list row is unreliably visible.** It renders only on the last
   page — which the one-row delegate makes rarer to reach, not commoner — and it
   vanishes entirely under any applied filter, because its `FilterValue()` is
   empty. This is the same discoverability argument that justifies the fixed
   overview, applied to the one row that sits furthest from the eye.
3. **It removes the last non-selectable row type that earns nothing.** Headers
   are positional: they label the rows beneath them, so their pagination and
   `skipNonEntry` cost is paid for. The credit has no position, no relationship
   to neighbouring rows, and no selection behaviour. Dropping it deletes the
   `credit` field on `startItem`, one delegate branch, both responsive credit
   variants, and the `hasCatalogRow` bookkeeping in `newStart`.

A pinned dim credit line above the status bar was rejected: it costs a body row
on every startpage view, on top of the one or two the overview already takes,
and a permanent URL on the launch screen is the kind of chrome this pass exists
to remove.

**This makes `tui/about.go` deliberately in scope**, alongside the startpage
files. The catalog data and its classification are still untouched.

## Bookmark Toggle Selection

On the startpage, toggling a bookmark preserves the user's section context
rather than following the moved target:

- bookmarking a catalog row keeps selection in its current catalog section;
- removing a bookmark keeps selection in `BOOKMARKS`;
- within that section, select the row now occupying the same ordinal position;
- if that ordinal is past the end, select the section's last row;
- if the section becomes empty, select the next surviving section in display
  order, then the previous section if no later section exists;
- if the whole startpage becomes empty, focus the target input.

For example, bookmarking the third service leaves focus on the service that
moves into the third slot. Removing the final bookmark selects the first
community, or the first service if no communities remain.

This is deliberately section-first rather than identity-first. Following a
target from the catalog to the top bookmark section—or from bookmarks deep into
the catalog—makes `b` behave like an unexpected navigation command. Stable
section context better supports browsing several catalog entries and pruning
several bookmarks. The existing flash and updated overview counts identify the
target that moved.

**The cost is real and is accepted knowingly:** today a second `b` undoes the
first, because selection follows the target. Under this rule the second `b`
lands on a *different* row and bookmarks a second target. This is the highest-
regret consequence in the design. It is mitigated, not eliminated, by the flash
naming the exact target acted on (`✓ bookmarked user@host`) — which it already
does — so the accidental entry is identifiable and removable. Undo is not being
added: that would be a new command, outside this spec's scope.

Bookmark toggles from reader, raw-reader, and user-list screens keep their
current behavior; no startpage selection exists to restore there.

## Charm-Native Boundaries

The design stays Charm-native by retaining component ownership rather than
imitating every stock visual:

- `bubbles/list` continues to own cursor movement, `g`/`G`, paging, paginator
  dots and arabic fallback, filtering, filter matching, and filter key
  semantics;
- resizing installs the appropriate one- or two-row delegate through
  `SetDelegate`, allowing Bubbles to recalculate `Paginator.PerPage` — verified:
  `list.SetDelegate` calls `updatePagination()` itself, so no manual
  recalculation is needed;
- `/` continues to use the list's real `FilterInput`, including Enter-to-apply
  and Esc-to-cancel/clear;
- contextual help remains expressed through `key.Binding`, with `b bookmark`
  on catalog rows and `b remove` on bookmark rows.

The overview should not be forced into Bubbles' stock title style. It has
responsive two-line content and mixed semantic groups, while lookit already owns
the target row and app-wide status bar. `list.NewStatusMessage` is also not used
for bookmark confirmation because the same bookmark action exists outside the
startpage and must keep one consistent app-level flash path.

Glow-style section tabs are rejected. They would imply direct section
navigation and add a new interaction model, exceeding the polish-only scope.

## Data and Component Changes

Section identity becomes explicit rather than inferred from display strings:

```go
type startSectionID uint8

const (
	sectionUnknown startSectionID = iota
	sectionBookmarks
	sectionCommunities
	sectionServices
	sectionPeople
)
```

`startSection` and each flattened `startItem` carry a `startSectionID`. A header
row carries the ID of the section it opens; `sectionUnknown` remains the zero
value for a row that belongs to no section, though with the credit gone the only
row types left are headers and entries. `startItem` loses its `credit` field.
`startModel` retains assembled section counts for overview rendering and tracks
whether list focus is active. The selected item's section ID drives the
highlighted overview segment.

The overview is a pure renderer owned by `startModel`. `setSize` subtracts its
current one- or two-row height plus existing notices before sizing the list.
When Bubbles shows its live filter input, the overview contributes zero rows.
Under an applied filter the counts are recomputed from `VisibleItems()` rather
than from the assembled sections.

`startDelegate` owns the responsive entry, header, and inactive-selection
rendering. Its `Height()` returns one or two according to the width mode, and
all item variants render within that uniform height. `newStart` no longer tracks
`hasCatalogRow` or appends a trailing row; `aboutView` gains one identity line.

**Focus is delegate state, and the delegate is replaced behind your back.**
`startDelegate` is a value, and `applyListStyles` ends with
`SetDelegate(defaultUserDelegate(st))`, which is why `newStart` and
`applyStyles` each re-install the startpage delegate immediately afterwards
(`tui/start.go`). The focus flag and the current width mode must therefore be
re-applied at *every* `SetDelegate` site, or the first theme change silently
reverts an inactive row to the active shelf. The alternative — promoting list
focus onto `commonModel` and having the delegate read it through that shared
pointer, as it already does for width and profile — makes the delegate
stateless in this respect and removes the class of bug; it is preferred.

Before a startpage bookmark toggle, `appModel` records the selected section ID
and its selectable ordinal inside that section. After `reloadStart`, a
`selectSectionPosition` helper applies the section-stable fallback rules. This
replaces the existing restore-by-target behavior only for startpage toggles.

No changes are made to:

- `tui/catalog.txt` or catalog classification;
- the bookmarks file grammar, validation, atomic saving, or symlink handling;
- target identity or catalog-note borrowing;
- request routing, result history, reader/list behavior, or focus ladder;
- bookmark and startpage key assignments.

## Status and Copy

The content-focused startpage status bar remains compact:

```text
↵ go · b bookmark · / filter · i target · ? help
```

On a bookmark row, `b bookmark` becomes `b remove`. The entry count and Bubbles
paginator retain their current behavior. Input-focused copy remains:

```text
type a target and press ↵ · ↓ browse · ? help
```

Bookmark flashes retain their current exact target and success/error behavior.
Notices continue to name the resolved bookmarks path.

## Testing

Tests remain offline and use the existing injected bookmarks path.

### Overview and layout

- wide overview is one line and never exceeds the model width;
- narrow overview is two lines and neither line exceeds the model width;
- counts reflect assembled sections after deduplication;
- zero bookmarks uses `BOOKMARKS  none yet` without an empty section;
- `catalog off` omits the catalog group;
- future and emptied catalog classifications appear or disappear from the
  overview with their assembled sections;
- wide delegates have height one and render target/description columns;
- at 80 columns the longest catalog target renders untruncated;
- narrow delegates have height two and render stacked content;
- resizing across `startWideMinWidth` updates the delegate and Bubbles
  pagination;
- resizing across `startWideMinWidth` **while a filter is applied** keeps the
  filter, the overview's filtered counts, and a selectable row selected — this
  combines the delegate swap, the overview's height contribution, and
  `skipNonEntry`, which is where the existing code is most fragile;
- header variants obey the active delegate height and width;
- the startpage list contains no credit row in any state — catalog on, catalog
  off, with and without bookmarks — and its last row is a selectable entry;
- About renders the credit line with the full `https://` URL and its hyperlink,
  in every state including `catalog off`, and the existing About golden test is
  updated rather than bypassed.

### Focus and filtering

- input focus produces no active overview segment and no active selection shelf;
- bookmark, community, and service selections highlight the correct overview
  segment;
- active highlighting survives continuation pages;
- live filtering shows Bubbles' filter input instead of the overview;
- applied filtering restores the overview, and bookmark rows carry no marker
  distinguishing them from catalog rows;
- a selected bookmark inside flat filtered results still highlights the
  `BOOKMARKS` overview segment and still shows `b remove`;
- applied-filter overview counts equal the visible rows and match the status
  bar's `N entries`, emptied classifications drop out, and clearing the filter
  restores the assembled counts;
- zero-match filtering still uses Bubbles' `No entries.` state rather than the
  file-level empty message, with every overview group at zero.

### Bookmark selection stability

- adding the first, middle, and final row of a catalog section stays in that
  section at the nearest ordinal;
- removing the first, middle, and final bookmark stays in bookmarks;
- removing the only bookmark falls forward into the first catalog section;
- emptying the last catalog section falls to the previous surviving catalog
  section rather than jumping to bookmarks;
- an entirely empty startpage focuses the target input;
- reader, raw-reader, and user-list bookmark behavior is unchanged.

Existing round-trip, malformed-file, symlink, pagination, filtering, focus
ladder, and app-level bookmark tests remain green. The final verification gate
is `make check`.

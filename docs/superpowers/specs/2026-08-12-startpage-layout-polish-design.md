# Startpage Layout Polish Design

**Date:** 2026-08-12

**Status:** Approved design; awaiting written-spec review

**Scope:** Presentation and interaction polish for the bookmarks/catalog
startpage. This deliberately adds no new destinations, commands, persistence
fields, navigation modes, or network behavior.

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

- at **72 columns and wider**, every item occupies one row, with the target in
  a left column and its description in a right column;
- below **72 columns**, every item occupies two rows, with the description
  stacked under the target.

Both columns truncate independently and never wrap. The wide target column uses
43 percent of the available width after the selected-row frame; the description
gets the remainder. The narrow layout preserves the existing target-first,
description-second hierarchy.

This deliberately prefers density at the ordinary 80-column terminal while
preserving authored catalog context on narrow screens. Hiding descriptions on
narrow terminals was rejected: those notes are what make unfamiliar finger
destinations understandable and worth exploring.

Section headers and the catalog credit occupy the same delegate height as entry
rows so Bubbles' uniform-height pagination remains valid. At wide widths a
header is a compact ruled label on one row and the credit is a single linked
endnote. At narrow widths each may use two rows; unused space stays blank rather
than changing the delegate's per-item height.

## Fixed Overview

A non-interactive overview appears between the target input and the scrolling
list. It describes two different levels of the information hierarchy:

```text
YOURS  2 bookmarks │ CATALOG  8 communities · 15 services
```

`YOURS` is ownership. `COMMUNITIES` and `SERVICES` are classifications inside
lookit's catalog. They are not presented as three peer categories, because that
would make the catalog classifications appear user-configurable.

The counts describe the assembled rows after bookmark/catalog deduplication.
When a catalog service becomes a bookmark, the bookmark count rises and the
service count falls; no target is counted twice.

Only non-empty catalog classifications appear in the overview, in section
order. If a future catalog adds people, the catalog group extends naturally to
`8 communities · 15 services · 3 people`; if pinning empties a classification,
that classification drops out along with its section.

At wide widths the overview is one line. When that line does not fit—always
below the 72-column layout breakpoint—it becomes two labelled rows:

```text
YOURS      2 bookmarks
CATALOG    8 communities · 15 services
```

The overview stays above the paginated list, so all available sections remain
discoverable even when a long section's heading is on an earlier page.

### Overview states

- With no bookmarks: `YOURS  no bookmarks yet`. No empty bookmark section is
  inserted; the list starts with `COMMUNITIES`.
- With `catalog off`: only the `YOURS` group appears.
- With `catalog off` and no bookmarks: the existing file-level empty message
  replaces the overview and list, names the active bookmarks path, and explains
  how to turn the catalog back on.
- While actively typing a `/` filter, Bubbles' own filter input replaces the
  overview. When the filter is applied, the overview returns above the flat
  filtered results.

## Focus and Visual Hierarchy

The existing violet/pink shelf remains the item-level selection treatment.
Gold becomes a persistent orientation cue in the overview rather than permanent
bookmark decoration:

- bookmark selected: `YOURS` is gold;
- community selected: the `communities` count is gold;
- service selected: the `services` count is gold;
- person selected, if that future section exists: the `people` count is gold;
- target input focused: nothing in the overview is gold.

Only the relevant text changes foreground colour. It receives no background,
border, or underline, so the overview reads as status rather than as clickable
tabs. This cue is supplemental: the selected shelf remains the primary focus
indicator, so section focus is not communicated by colour alone.

Highlighting the inline section heading was rejected. It has better proximity
to the selected row, but disappears on continuation pages and during filtering.
Making it sticky would introduce another custom chrome layer that duplicates the
fixed overview.

Bookmark rows use a neutral `◆` before the target. The shape remains present in
flat filtered results, where section headers are absent, but bookmarks receive
no permanent gold treatment.

When the target input owns focus, the list keeps its logical selection but
renders it with a neutral inactive treatment rather than the violet/pink shelf.
When focus moves into the list, the shelf and overview highlight appear. This
ensures only one component looks active at a time.

All colours continue to come from the existing adaptive palette. No new
hardcoded dark-only colours are introduced.

## Bookmark Toggle Selection

On the startpage, toggling a bookmark preserves the user's section context
rather than following the moved target:

- bookmarking a catalog row keeps selection in its current catalog section;
- removing a bookmark keeps selection in `YOUR BOOKMARKS`;
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
target that moved. Immediate `b`-again undo is the accepted trade-off.

Bookmark toggles from reader, raw-reader, and user-list screens keep their
current behavior; no startpage selection exists to restore there.

## Charm-Native Boundaries

The design stays Charm-native by retaining component ownership rather than
imitating every stock visual:

- `bubbles/list` continues to own cursor movement, `g`/`G`, paging, paginator
  dots and arabic fallback, filtering, filter matching, and filter key
  semantics;
- resizing installs the appropriate one- or two-row delegate through
  `SetDelegate`, allowing Bubbles to recalculate `Paginator.PerPage`;
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

`startSection` and each flattened `startItem` carry a `startSectionID`.
`startModel` retains assembled section counts for overview rendering and tracks
whether list focus is active. The selected item's section ID drives the gold
overview segment.

The overview is a pure renderer owned by `startModel`. `setSize` subtracts its
current one- or two-row height plus existing notices before sizing the list.
When Bubbles shows its live filter input, the overview contributes zero rows.

`startDelegate` owns the responsive entry, header, credit, and inactive-selection
rendering. Its `Height()` returns one or two according to the width mode, and
all item variants render within that uniform height.

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
- zero bookmarks uses `no bookmarks yet` without an empty section;
- `catalog off` omits the catalog group;
- future and emptied catalog classifications appear or disappear from the
  overview with their assembled sections;
- wide delegates have height one and render target/description columns;
- narrow delegates have height two and render stacked content;
- resizing across 72 columns updates the delegate and Bubbles pagination;
- header and credit variants obey the active delegate height and width.

### Focus and filtering

- input focus produces no active overview segment and no active selection shelf;
- bookmark, community, and service selections highlight the correct overview
  segment;
- active highlighting survives continuation pages;
- live filtering shows Bubbles' filter input instead of the overview;
- applied filtering restores the overview and retains neutral bookmark diamonds;
- zero-match filtering still uses Bubbles' `No entries.` state rather than the
  file-level empty message.

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

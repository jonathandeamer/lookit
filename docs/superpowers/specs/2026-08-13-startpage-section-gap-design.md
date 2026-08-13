# Startpage section gap

## Context

The startpage already distinguishes `COMMUNITIES` and `SERVICES` with ruled
headers, but the last community row runs directly into the services header in
the primary one-line layout. After service entries gained host grouping and
focus-revealed notes, the boundary between the two catalog sections is the one
remaining dense transition.

The startpage uses a `bubbles/list` delegate with one fixed height for every
item: one terminal row at widths of 72 columns or more, and two rows below 72.
The narrow header already uses its first delegate row as a blank line, so the
gap exists there today. The wide header occupies its only row and cannot add a
blank line without violating the list's pagination arithmetic.

## Decision

Add exactly one blank terminal row immediately before the `SERVICES` header
when all of these are true:

- both `COMMUNITIES` and `SERVICES` sections are present;
- the startpage is unfiltered; and
- the terminal is at least 72 columns wide.

Represent the wide-layout gap as a dedicated, non-selectable `startItem`.
Rendering it as empty output between the list's normal item separators creates
one blank terminal row while keeping the delegate's one-row height honest.
The spacer returns an empty filter value, carries no section identity, and is
excluded from overview and status-bar counts.

Below 72 columns, do not assemble the spacer. `renderHeader` already emits one
blank row before every header in the two-row layout, so `SERVICES` retains a
single-row separation rather than growing to an oversized gap.

## Responsive behaviour

`setSize` adds or removes the spacer when an unfiltered startpage crosses the
72-column breakpoint. It captures the selected row's section-relative ordinal
before changing the item slice and restores that position afterward, so
inserting an item before the selection cannot move the cursor to a header,
another destination, or an earlier occurrence of a repeated bookmark target.

While a filter is being typed or applied, headers and the spacer are absent
from the visible results. A resize therefore leaves the underlying item slice
alone, avoiding an asynchronous re-filter operation that `setSize` cannot
return to Bubble Tea. When filtering is cleared, the existing filter-state
transition calls `setSize`, which synchronizes the spacer for the current
width before the sectioned view returns.

## Navigation and filtering

The spacer uses the same general non-entry navigation rule as headers. Moving
down from the last community skips both the spacer and the `SERVICES` header;
moving up makes the symmetric jump. It can never become the selected item,
including when it lies at a page boundary.

Because its `FilterValue` is empty, any non-empty filter drops it alongside
headers and structural service-parent copies. Filtered results remain a flat,
gap-free list.

## Scope

This changes only `tui/start.go`, its tests, and the design/plan documents. It
does not change catalog data, section assembly in `buildSections`, counts,
grouping, bookmarks, keybindings, or startpage copy.

## Testing

- At 80 columns, the rendered line between the last community and `SERVICES`
  is blank.
- At 40 columns, that boundary still contains exactly one blank line, supplied
  by the existing two-row header rather than a spacer item.
- A startpage missing either section gains no spacer item.
- Cursor movement skips the spacer and header in both directions.
- A non-empty filter contains neither headers nor the gap.
- Resizing unfiltered across 72 columns adds/removes the spacer and preserves
  the selected section-relative occurrence, including repeated bookmark
  targets; clearing a filter after a resize synchronizes it.

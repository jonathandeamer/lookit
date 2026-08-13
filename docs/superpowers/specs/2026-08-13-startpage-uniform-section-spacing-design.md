# Startpage uniform section spacing

## Context

The startpage separates its sections inconsistently. Measured against the
rendered view rather than by reading the assembly code, the blank rows above
each header are:

| Layout | Above first header | Above COMMUNITIES | Above SERVICES |
|---|---|---|---|
| Wide (>= 72 cols) | 2 | 0 | 1 |
| Narrow (< 72 cols) | 3 | 1 | 1 |

Three different values in the primary layout. The last community row runs
straight into the SERVICES header's predecessor while SERVICES itself gets a
gap, and the top of the list gets twice what any section boundary does.

This is not an accident of two changes colliding. `tui/start.go` encodes two
deliberate and separate rules: an unconditional spacer before the list, and a
spacer before SERVICES gated on `width >= startWideMinWidth && hasCommunities
&& s.id == sectionServices`. Each is correct in isolation. Together they
produce a page whose vertical rhythm changes depending on where in it you are
looking.

Two facts constrain any fix.

**One blank row is not ours to spend.** `bubbles/list` renders `titleView()`
whenever filtering is enabled, even with `SetShowTitle(false)`:

```go
if m.showTitle || (m.showFilter && m.filteringEnabled) {
```

That row is the slot the filter prompt occupies when `/` is pressed. Removing
it would make the layout jump on every filter. It is always present, so the
top gap is structurally *that row plus whatever we add*, and the honest fix
spends it rather than stacking another row on top of it.

**The two layouts have different row quanta.** The delegate is one row wide
and two rows narrow, and `renderHeader` already spends the narrow header's
first row on a blank. A spacer item therefore costs one row wide and two rows
narrow. Exact cross-layout uniformity is unreachable; uniformity within each
layout is not.

## Decision

One invariant, stated once:

> Exactly one blank terminal row sits immediately above every section header.

The mechanism differs per layout because the quanta do.

**Wide.** The list's reserved filter row supplies the gap above the first
header, so no leading spacer is assembled. A spacer item precedes every header
after the first. The result is 1 / 1 / 1 — exactly uniform.

**Narrow.** `renderHeader` already emits a blank as the first of its two rows,
for every header. No spacer items are assembled at all, which removes today's
two-row leading spacer. The result is 2 / 1 / 1.

The narrow top boundary keeps one extra row, and stops there deliberately. A
spacer costs two rows in that layout, so adding one overshoots. Suppressing the
first header's own blank does not help either: the item slot is a fixed two
rows, so the blank moves *below* the header rather than disappearing. Two is
the minimum reachable value, down from three.

## Assembly

`startItems` replaces both existing rules with one: at wide widths, insert a
spacer before every section header except the first.

The `hasCommunities` pre-scan and the `s.id == sectionServices` test both
disappear. Spacing stops being a fact about which sections exist and becomes a
fact about section boundaries, so a future fourth section — PEOPLE, if the
catalog ever gains person entries — is spaced correctly without a new clause.

No delegate change. Spacers already render as empty output, and header
rendering is untouched.

## Responsive behaviour

`setSize` rebuilds the item slice when an unfiltered startpage crosses 72
columns, capturing the selected row's section-relative ordinal beforehand and
restoring it afterward. That machinery now adds and removes N spacers rather
than one.

It operates on section-relative ordinals rather than absolute indices, so it
should carry the change unmodified. This is nonetheless the one place a defect
would hide, and it is tested explicitly rather than assumed: crossing the
boundary in both directions with the selection on the first row of a section,
which is the position most exposed to an item being inserted directly above it.

While a filter is active the item slice is left alone, unchanged from today.

## Navigation, filtering and counts

All unchanged. Spacers remain non-selectable, so cursor movement skips a
spacer and the header behind it in one step, exactly as it does now — a spacer
is only ever adjacent to a header, so the shape of that jump does not change
even though there are more of them. `FilterValue` returns empty, so any
non-empty filter drops every spacer and leaves a flat, gap-free result.
Spacers are excluded from overview and status-bar counts by `countsAsListing`.

## Scope

`tui/start.go` and its tests. No change to catalog data, `buildSections`,
grouping, bookmarks, counts, keybindings, or any user-facing string.

## Testing

Assert the invariant against the rendered view, not the item slice. Counting
items encodes the mechanism; counting blank rows above a header encodes the
decision, and stays honest if the mechanism changes again.

- Wide, three sections: the line above each header is blank, and the line
  above that is not — except at the first header, where the line above the
  blank is the overview.
- Narrow, three sections: one blank above COMMUNITIES and SERVICES, two above
  the first header.
- A single section (`catalog off`, leaving only BOOKMARKS) assembles no
  spacers at either width.
- A non-empty filter contains no spacers and no headers.
- Resizing across 72 columns in both directions preserves the selected row,
  with the selection on a section's first row.

`TestStartItemsAlwaysBeginsWithSpacer` asserts a contract this decision
deliberately reverses, and is replaced rather than adjusted.

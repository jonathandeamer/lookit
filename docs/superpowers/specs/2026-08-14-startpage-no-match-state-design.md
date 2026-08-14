# Startpage no-match state

Issue: #74

## Context

Typing a startpage filter that matches nothing leaves the body blank. The
filter prompt stays on the first row and everything below it is empty, so the
screen states nothing about why the catalog disappeared.

The cause is in Bubbles, not in lookit. `list.Model.populatedView` returns an
empty string as soon as the visible-item slice is empty *and* the filter is
still being typed:

```go
if len(items) == 0 {
    if m.filterState == Filtering {
        return ""
    }
    return m.Styles.NoItems.Render("No " + m.itemNamePlural + ".")
}
```

The `"No entries."` string that `newStart` configures via
`SetStatusBarItemName` is therefore unreachable in the state a user is actually
in. `startModel.View` has its own file-level empty state, but it is gated on
`len(m.list.Items()) == 0` — the whole page being empty — which is deliberately
not this case and must stay that way: a zero-match filter is a fully populated
page, and claiming "no bookmarks yet" there would be false.

`Filtering` is the only state this can occur in. Bubbles' `AcceptWhileFiltering`
handler calls `resetFiltering()` when the accepted filter has no matches, so
`FilterApplied` with zero visible items is unreachable.

## Decision

While the startpage filter is being typed, has a non-empty value, and matches
no row, draw a single line in the list's content region:

```
no match for “zzzzzz”
```

The copy names the query, not the catalog. Nothing is wrong with the catalog;
the query is the reason the body is empty, and it is the thing the user can
change. It is lowercase and unpunctuated, matching the rest of the app's
in-body and status copy, and it uses the same typographic quotes Bubbles uses
for a displayed filter value.

The line renders in `list.Styles.NoItems`, which `applyListStyles` already
themes, so it is dim in both backgrounds without a new style. It is indented by
two columns to sit under the filter prompt and on the same left edge as the
entry rows it replaces (both come from a padding of 2).

## Placement

The message occupies the first row of the list's content region.
`startModel.View` renders `m.list.View()` and overwrites that row, which
Bubbles has painted blank.

With the startpage's list configuration (title, status bar and help all off),
the filter prompt is the only thing Bubbles draws above the content region.
That block is `Styles.TitleBar` wrapped around a single-line `textinput`, and
the default `TitleBar` carries one row of bottom padding, so it is two rows
tall and the content region begins at line index 2 — measured, not assumed.

Rather than hardcode 2, `startModel` derives the offset the same way Bubbles
does, from exported fields:

```go
lipgloss.Height(m.list.Styles.TitleBar.Render(m.list.FilterInput.View()))
```

so a future style change to `TitleBar` moves the message with the prompt. If
the computed offset falls outside the rendered view, the message is appended
instead of overwriting, so the line can never be silently dropped.

Overwriting a blank row of the list's own view is a smaller, more honest change
than reimplementing the list's view assembly. A test pins the layout: the
filter prompt survives, and the message lands on the first row below it that
Bubbles left blank.

## Width and height behaviour

The line is truncated with `ansi.Truncate` at the list's width, so it degrades
to `no match for “z…` rather than wrapping. It is exactly one row tall in both
the wide one-line and narrow two-line layouts, so it fits the narrow stacked
geometry where the body is only a few rows tall.

No layout arithmetic changes: the message replaces a row the list already
reserved and painted blank, so `setSize`, `overviewHeight`, `noticeHeight`, and
the delegate's fixed item height are untouched.

## What does not change

- The file-level `empty` state and its gate on `len(m.list.Items()) == 0`.
- The overview line, which is already suppressed while filtering.
- The status bar, counts, filtering, navigation, and every other startpage
  behaviour.
- The host user list and links panel, which have their own filters and are out
  of scope for this issue.

## Testing

- A filter matching nothing renders the message on the row below the prompt,
  and the prompt itself survives.
- The message names the typed query.
- A filter matching at least one row renders no message.
- An empty filter (`/` pressed, nothing typed) renders no message: every row is
  still visible, dimmed, and the state is not a no-match.
- An unfiltered startpage renders no message.
- At a narrow width the message is truncated to the list width with an ellipsis
  and stays one row tall.
- The startpage with no rows at all still shows the file-level empty state.

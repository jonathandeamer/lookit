# Reader word-wrap toggle design

**Date:** 2026-08-20

## Goal

Add an optional reading mode to the TUI reader. Pressing `w` reflows prose at
word boundaries while preserving lookit's existing unwrapped presentation as
the default. Pressing `w` again restores the original layout.

This addresses real Finger content rather than a hypothetical case. A polite,
sequential crawl of the 135 addresses listed by
`@crossed-fingers.andros.dev` yielded 129 analysable results; six other targets
failed without a body. Twenty contained physical lines wider than 80 terminal
cells, and nineteen of those appeared prose-like.
Examples included a 154-cell quotation, several timestamped posts around 111
cells, and serialised prose with lines between roughly 500 and 700 cells.
The point-in-time method, classification rule, limitations and full measured
distribution live in
`docs/decisions/2026-08-20-reader-wrap-crawl.md`.

The feature is a deliberate opt-in because Finger bodies are unstructured.
ASCII art, tables, menus, code and hand-aligned text can depend on their
original physical lines. Lookit cannot reliably classify those forms, so it
must not choose to reflow them on the user's behalf.

## Non-goals

- Do not wrap response bodies by default.
- Do not detect a `.plan` section. Finger supplies one arbitrary response body,
  not typed sections that lookit can identify reliably.
- Do not add a configurable width or bookmarks-file directive.
- Do not reflow source view.
- Do not join existing physical lines into paragraphs.
- Do not hard-break long URLs, Finger addresses or other indivisible tokens.
- Do not add a persistent status-bar badge or flag.
- Do not change networking, sanitisation, link detection, copying, bookmarking
  or the stored response body.

## Interaction

### Availability

`w` is an app-level `key.Binding` with contextual help. It is enabled only
when content is focused in the normal reader and the current response has a
non-empty body. It remains available for partial responses that contain body
text, including a body accompanied by an error.

It is disabled while:

- the target input is focused;
- the startpage or a user list is active;
- `v` source view is active;
- the links panel is active; or
- the reader shows an empty failure response.

When the target input is focused, `w` continues to type a literal `w`.

Expanded Help derives the action from the current entry: `w wrap` while the
entry is unwrapped and `w unwrap` while it is wrapped. Like the existing Help
actions, pressing `w` in the overlay closes Help and replays the binding
through ordinary app routing.

`helpCandidates` places Wrap immediately after Raw and before Refresh, grouping
the adjacent reader-display actions while preserving the existing candidate
priority around them.

Wrap must remain in the retained Help prefix at all six established TUI-review
geometries. Automated layout tests pin that requirement; a narrower or shorter
terminal outside those geometries may still shed it under the ordinary prefix
rule.

Responsive Help's retained prefix is also its execute gate. If a constrained
layout drops Wrap from the visible binding set, pressing `w` while Help is open
does nothing and leaves Help open. This is accepted existing Help behavior,
not a hidden shortcut; closing Help restores the normal reader binding.

### Feedback

Toggling on briefly shows `wrapping on` through the existing two-second flash
mechanism. Toggling off briefly shows `original layout`. The ordinary status
bar then returns unchanged.

There is no persistent `wrapped` flag. Status flags currently communicate
important uncertainty or incompleteness (`partial (truncated)`,
`auto-detected`) and outrank hints when space is scarce. A display preference
must not displace those flags, the breadcrumb, navigation, scroll position,
byte count or latency. The reflowed content and contextual Help provide the
lasting indication.

## Per-response state

Wrapping belongs to a history entry, not to the whole session.

- A newly navigated response always starts unwrapped.
- Toggling changes only the visible history entry.
- Back navigation and history restoration restore that entry's wrapping
  choice.
- Refresh preserves the current entry's wrapping choice while replacing its
  response, just as refresh preserves the identity of the history entry.
- Entering source view temporarily ignores the choice. Source view always
  shows the original sanitised body layout, and leaving it returns to the
  entry's wrapped or unwrapped reader presentation.

The wrapping boolean lives on `histNode`, the owner of per-response navigation
state. Reader state mirrors it only while rendering the visible node; it is
not a second source of truth.

## Wrap semantics

The effective body wrap width is:

```text
min(reader viewport width, 80 terminal cells)
```

The width is measured in terminal cells, not bytes, runes or grapheme count.
At 100 columns the body therefore uses an 80-cell reading measure; at 80 it
uses 80; at 60 or 45 it uses the available viewport width. Wrapped content is
left-aligned rather than centred.

Each existing physical line is processed independently:

- A line already within the limit is unchanged.
- A wider line breaks only at word boundaries.
- Existing newlines remain exactly where they were; adjacent lines are never
  joined.
- The wrapper does not invent hanging indentation or other whitespace.
- A token wider than the limit stays intact on one display line. The current
  horizontal viewport controls keep it reachable.
- A physical line containing a tab is not wrapped at all. Tabs are meaningful
  layout instructions with a terminal-dependent stop width; treating them as
  zero-width break whitespace would mismeasure the line and could delete them
  at an inserted break. Passing the whole line through intact preserves both
  its tabs and the server's intended indentation.
- A whitespace-only physical line is emitted intact, even when its whitespace
  occupies more than the requested measure.

An empty physical line bypasses wrapping and is emitted unchanged, preserving
every blank paragraph gap.

The body path uses a dedicated `wordWrapBodyLine` primitive. It scans the
ANSI-bearing line as alternating whitespace and non-whitespace tokens, measures
visible width with `ansi.StringWidth`, and breaks only before a token that would
overflow a non-empty output line. A token wider than the limit is emitted
whole. Hyphens have no special meaning.

Non-breaking space (`U+00A0`) belongs to a non-whitespace token for wrapping
purposes, so it never becomes a line-break opportunity.

It must not call `ansi.Wrap`, which hard-breaks long tokens, or directly call
`ansi.Wordwrap`. The latter ordinarily preserves words but always treats a
hyphen as a breakpoint even when its `breakpoints` argument is empty; that can
split a hyphenated URL and defeat raw-token link matching. The dedicated body
primitive retains ANSI sequences while enforcing the stronger whitespace-token
contract chosen for this feature.

Bodies and error text deliberately differ on overlong tokens. Only the
response body uses the 80-cell maximum and `wordWrapBodyLine`, leaving an
indivisible token intact. Lookit's generated error text continues to use
`ansi.Wrap` at the full viewport width, hard-breaking when necessary so the
reason for a failure can never remain clipped. Error wrapping remains active
whether body wrapping is on or off.

## Rendering architecture

The `render` package remains the single response-body rendering path. It gains
an explicit optional-layout entry point; all existing public entry points keep
their current unwrapped behavior and compatibility.

The rendering order is:

1. Apply existing host-specific body preparation, such as tilde.team pronoun
   reflow.
2. Assign a stable logical-line identity to each resulting physical line.
3. Apply field highlighting to the original physical line. Only the first
   segment of a logical line is eligible for prefix matching.
4. If requested, call `wordWrapBodyLine(line, bodyWidth)` for each ANSI-styled
   line. Every continuation segment retains its logical-line identity.
5. Apply the existing error treatment and return the ANSI-rendered text plus a
   display-line-to-logical-line map.

The first-segment rule is load-bearing. `highlightFields` matches every input
line with `strings.HasPrefix`, and the field vocabulary includes prose-like
prefixes such as `On since`, `No mail.` and `Never logged in.`. A continuation
segment that happens to begin with one of those strings must not gain field
colour that the same text lacks in unwrapped mode. Highlighting before wrapping
is the simplest implementation; carrying first-segment eligibility into a
post-wrap highlighting pass is also valid, but the visible contract is the
same.

The logical lines are the stable pre-wrap representation after existing
renderer-specific preparation. This makes the map identical in meaning in
both modes without attempting to reverse transformations that predate this
feature.

The optional result can be represented as a small render-layout value holding
the text and line map. The precise type name is an implementation detail, but
the boundary is not: wrapping and mapping belong in `render`, while viewport
positioning and per-history state belong in `tui`.

Every displayed row has a map entry. Body rows map to their prepared logical
body line; every row of the separately wrapped error text maps to a non-body
sentinel. When the viewport top maps to that sentinel and a body is present,
position restoration uses the final logical body line. An empty failure has no
body target and never enables `w`; its existing resize behavior is unchanged.

This design deliberately does not use `viewport.SoftWrap`. Bubbles soft-wraps
at cell boundaries rather than word boundaries, uses the viewport's full
width, and does not expose the source mapping needed for stable position and
link focus.

## Position, resize and links

Every normal-reader render follows one positioning contract exactly once:
capture the old logical top line and exact-offset fallback when preserving a
view; render; apply the link overlay; set viewport content; then centre a
focused link if one exists, otherwise restore the logical line, otherwise use
the exact fallback and let the viewport clamp it. A newly navigated response
supplies a zero fallback and therefore starts at the top. No caller performs a
second `SetYOffset` after this sequence.

Before toggling or re-rendering at a changed width, the reader uses the old
line map and `YOffset` to identify the logical line at the top of the viewport.
After rendering, it places the first display segment for that logical line at
the top. If that logical line no longer exists after a refresh, the position
clamps to the nearest available line. If the old top row belongs to appended
error text, its non-body sentinel resolves to the final logical body line.

This preserves the same source passage, not the exact continuation segment
within a newly reflowed paragraph. Both toggling directions and width-changing
resizes reset horizontal offset to zero so the start of the reflowed content
cannot remain hidden.

Focused-link state takes precedence over top-line preservation. If a link is
focused when wrapping changes, the same link index remains focused and the
reader reuses `scrollToFocusedLink`, placing the focused link roughly in the
vertical centre as it does after every other focused-link render. This avoids
inventing a second "only if hidden" scroll policy. Long tokens remaining intact
is load-bearing here: the existing raw-token matching used by
`applyLinkOverlay` and `scrollToFocusedLink` must continue to find URLs and
Finger targets after surrounding prose is reflowed. Opening and closing the
links panel does not alter the history entry's wrapping choice.

This precedence deliberately changes refresh behavior for a focused link.
Previously refresh rendered and centred the restored link, then overwrote that
position with the captured physical `YOffset`; after this feature, refresh
uses the same single contract as every other render and the focused link stays
centred. Refresh without a focused link still preserves the source passage.

Only a width change reflows content during `setSize`: height-only size updates
resize the viewport without re-rendering or resetting horizontal position.
The old width is compared before `readerModel.width` is overwritten. Source
view is marked explicitly in the reader; width changes resize its viewport and
reset horizontal offset but never replace its raw content with a normal or
wrapped render.

The link overlay always applies after wrapping, to the wrapped rendered text;
`wordWrapBodyLine` never receives OSC-8-wrapped tokens.

Detected links, byte counts and stored `Entry.Body` always come from the
original sanitised response. Wrapping is display-only.

## Offline visual review based on the crawl

The TUI review kit will reproduce the shapes found in the crawl without
copying another person's live `.plan` into the repository and without making
network requests during recording.

The loopback review fingerd gains a deterministic `wrapplan` response with:

- a timestamped prose line around 110 cells;
- an ordinary prose paragraph around 150 cells;
- an extreme prose line in the observed 500–700-cell range;
- a preformatted line no wider than 80 cells, which must remain unchanged; and
- a URL longer than the wrap width, which must remain an intact token.

The prose is representative and original. Only its structure and measured
line lengths are derived from the live crawl.

The shared `responses-tour.tape` gains two scenes, and therefore records them
at all six existing response geometries (twelve new frames):

| File | State |
|---|---|
| `wrap-original.png` | `wrapplan` in its default unwrapped layout |
| `wrap-enabled.png` | the same response after `w`, after the transient flash has cleared |

Every scene keeps the review kit's load-bearing
`Wait` → `Sleep 1500ms` → `Screenshot` order. `wrap-enabled.png` does not try
to capture the two-second flash: a 1500ms settling delay would leave less than
500ms of timing margin after VHS polling. The fixture instead places a unique
marker beyond the unwrapped viewport edge. After `w`, the tape first waits for
that marker to appear on a continuation row, then waits for the ordinary
`? help` status hint to return, and only then performs the final
`Wait` → `Sleep 1500ms` → `Screenshot` capture. Both flash strings are covered
deterministically in TUI tests.

There is no separate `wrap-help.png`. The existing `reader-help.png` already
covers Help-over-reader composition at all six geometries; another Help frame
would add six near-duplicates. Automated help-layout tests instead pin Wrap's
candidate position, its dynamic label, its retention at named geometries and
the retained-prefix execute gate.

`docs/tui-review/README.md` gains the new scene inventory. The full
`make review-tui` run is required because wrapping and clipping directly
concern the 60- and 45-column geometries as well as the three first-class
size/theme combinations. Generated PNGs remain gitignored.

## Tests

### Renderer

Pure renderer tests cover:

- unchanged output from all existing unwrapped entry points;
- word-boundary wrapping at terminal-cell width;
- preservation of existing newlines and lines within the limit;
- explicit preservation of empty physical lines and paragraph gaps;
- a tab-bearing line remaining wholly unwrapped with every tab intact;
- a whitespace-only over-width line remaining intact;
- non-breaking space remaining inside an indivisible token;
- an overlong token remaining intact;
- a hyphenated overlong URL remaining intact and discoverable by raw-token
  matching;
- no invented continuation indentation;
- ANSI field styling remaining valid across continuation lines;
- a continuation beginning with a field-like prefix remaining unstyled;
- stable logical-line maps in wrapped and unwrapped modes;
- non-body sentinel mappings for every wrapped error row;
- the generated `(no response body)` row mapping to the non-body sentinel; and
- the existing error-line wrapping remaining independent of body wrapping.

### Reader and app

TUI tests cover:

- new history entries defaulting to unwrapped;
- independent wrapping choices on multiple history entries;
- back-navigation restoration and refresh preservation;
- source view ignoring but not clearing the choice;
- key enablement in reader, input, list, start, raw, links-panel, empty-error
  and partial-body states;
- Help's dynamic label and replay path;
- Wrap's position immediately after Raw and its retention at all six review
  geometries;
- identical retained Help binding membership before and after `w wrap` becomes
  `w unwrap`, including at 45 and 60 columns;
- every binding retained by the pre-Wrap candidate list remaining retained
  after Wrap is inserted, at all six geometries and with both Wrap labels;
- both flash messages and their existing clear path;
- top logical-line preservation across both toggle directions and resize;
- horizontal offset reset;
- focused-link preservation, styling, action and recentering to the wrapped
  token's display row through `scrollToFocusedLink`;
- focused-link refresh adopting centring precedence while unfocused refresh
  preserves the logical source line; and
- unchanged link detection and byte metadata.

### Review fixture

The loopback fingerd tests pin the presence and intended line-width shapes of
the representative `wrapplan` body. The tape/frame manifest tests, if any,
are updated for the two new screenshots.

Verification runs focused `render`, `tui`, and review-fingerd tests first,
then `make check`, then `make review-tui`. Review starts with the three
first-class response contact sheets and opens all new wrap frames at full
size; the 60- and 45-column wrap frames are then checked specifically for
clipping, misplaced horizontal offsets, dishonest Help, or broken status
chrome.

## Documentation

The implementation updates the living documentation and current package
contracts in the same change:

- `render/CLAUDE.md` records that unwrapped remains the default and describes
  the opt-in layout result.
- `tui/CLAUDE.md` records per-history wrapping state, availability, position
  restoration and focused-link behavior.
- `docs/user-facing-messages.md` inventories `w wrap`, `w unwrap`,
  `wrapping on` and `original layout`.
- `docs/tui-review/README.md` documents the new offline fixture scenes.
- `README.md` briefly advertises the reader toggle.
- `docs/decisions/2026-08-20-reader-wrap-crawl.md` preserves the evidence and
  method behind the product decision without retaining live response bodies.

`man/lookit.1` remains unchanged. It deliberately contains no keybinding list;
Expanded Help is the live, context-aware reference.

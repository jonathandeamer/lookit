# Reader Word-Wrap Toggle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in, per-response `w` toggle that word-wraps normal-reader response bodies at `min(viewport width, 80)` terminal cells while preserving source lines, long tokens, focused links, and the current passage.

**Architecture:** `render` gains an optional layout result containing rendered text and a display-row-to-logical-body-line map; existing render entry points delegate to it with body wrapping disabled. `readerModel` uses that map to preserve a logical source line across re-rendering and applies link overlays only after wrapping. `histNode` remains the source of truth for each response's wrap choice, while the app owns key availability, contextual Help, refresh restoration, and transient feedback.

**Tech Stack:** Go, Bubble Tea v2, Bubbles viewport v2, Lip Gloss v1 in `render`, Lip Gloss v2 in `tui`, `github.com/charmbracelet/x/ansi`, VHS TUI-review tapes.

**Spec:** `docs/superpowers/specs/2026-08-20-reader-word-wrap-design.md`

## Global Constraints

- Work only in the dedicated worktree `/Users/jonathan/lookit/.worktrees/reader-word-wrap` on branch `feat/reader-word-wrap`; another agent is using the main checkout.
- Before editing `render/` or `tui/`, read the package's complete `CLAUDE.md` and preserve its contracts.
- Do not touch `finger/`, networking, sanitisation, stored `Entry.Body`, link detection, copying, or bookmarks.
- Existing `Render`, `RenderWithBackground`, and `RenderWithWidth` behavior remains byte-for-byte compatible; body wrapping is available only through the new explicit layout path.
- The body wrapper breaks only at whitespace, measures terminal cells, never splits an overlong token, never joins physical lines, and preserves blank lines.
- Any physical line containing a tab passes through wholly unwrapped and byte-intact; a whitespace-only line also passes through intact, and `U+00A0` is not a break opportunity.
- Generated error text continues to use `ansi.Wrap` at the full viewport width.
- Field-prefix highlighting applies to an original physical line, never independently to a continuation segment.
- Link detection uses the original body; the OSC-8/focus overlay applies to rendered text after wrapping.
- `histNode.wrapped` is the source of truth. New navigation starts false; refresh and history restoration preserve it; source view ignores it temporarily.
- Toggling and width-changing resize reset horizontal offset. A focused link takes precedence and uses the existing centring policy.
- Every normal render positions exactly once after render, overlay, and `SetContent`: focused-link centre, otherwise logical source line, otherwise exact-offset fallback. Height-only and raw-view resizes never re-render content.
- User-facing strings are exactly `wrapping on`, `original layout`, `w wrap`, and `w unwrap`; update `docs/user-facing-messages.md` in the same change.
- Conventional Commits; no `Co-Authored-By` or AI attribution. Commit steps below are conditional on explicit user authorisation; otherwise leave changes uncommitted. Do not push or open a PR without separate approval.

---

### Task 1: Add the renderer's optional body layout and logical-line map

**Files:**
- Read first: `render/CLAUDE.md`
- Create: `render/layout.go`
- Modify: `render/render.go`
- Test: `render/layout_test.go`
- Test: `render/wrap_test.go`

**Interfaces:**

```go
const NoBodyLine = -1

type LayoutOptions struct {
	ErrorWidth int
	BodyWidth  int
}

type Layout struct {
	Text          string
	LineMap       []int
	BodyLineCount int
}

func RenderLayout(
	t finger.Target,
	body []byte,
	queryErr error,
	profile colorprofile.Profile,
	darkBackground bool,
	opts LayoutOptions,
) Layout

func (l Layout) LogicalLineAt(displayLine int) int
func (l Layout) DisplayLineFor(logicalLine int) int
func wordWrapBodyLine(line string, width int) []string
```

`LineMap` has one entry for every row represented by `strings.Split(Layout.Text, "\n")`, including the final empty row created by the renderer's trailing newline. Prepared body rows map to zero-based logical body lines; generated no-body/footer rows, error rows, and the final empty row map to `NoBodyLine`.

- [ ] **Step 1: Read the renderer contract**

Run: `sed -n '1,240p' render/CLAUDE.md`

Record any contract that conflicts with this plan before proceeding. The approved spec is intended to supersede only the old “body is never reflowed” statement, and only through an explicit optional path.

- [ ] **Step 2: Write failing primitive tests**

Create table-driven tests in `render/layout_test.go` that call `wordWrapBodyLine` directly:

```go
func TestWordWrapBodyLine(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		width int
		want  []string
	}{
		{"short line", "one two", 10, []string{"one two"}},
		{"word boundaries", "one two three", 7, []string{"one two", "three"}},
		{"blank line", "", 7, []string{""}},
		{"tab-indented line passes through", "\talpha beta gamma", 7, []string{"\talpha beta gamma"}},
		{"embedded tab passes through", "alpha\tbeta gamma", 7, []string{"alpha\tbeta gamma"}},
		{"whitespace-only over width passes through", "            ", 5, []string{"            "}},
		{"non-breaking space is not a break", "one two\u00a0three four", 8, []string{"one", "two\u00a0three", "four"}},
		{"no invented indentation", "alpha      beta gamma", 10, []string{"alpha", "beta gamma"}},
		{"long token intact", "lead https://example.com/a-very-long-path tail", 12, []string{"lead", "https://example.com/a-very-long-path", "tail"}},
		{"hyphen is not a breakpoint", "alpha beta-gamma-delta omega", 12, []string{"alpha", "beta-gamma-delta", "omega"}},
		{"wide cells", "ab 界界 cd", 7, []string{"ab 界界", "cd"}},
	}
	// Compare slices exactly. Also assert that joining all stripped output
	// tokens reproduces the input token sequence.
}
```

Add an ANSI case using `lipgloss.NewStyle().Foreground(...).Render("Login:") + " alice example"`; assert the escape sequences survive and each returned segment has `ansi.StringWidth(segment) <= width`, except the deliberately overlong-token case.

- [ ] **Step 3: Run the primitive test and verify RED**

Run: `go test ./render -run TestWordWrapBodyLine -count=1 -v`

Expected: FAIL — `wordWrapBodyLine` is undefined.

- [ ] **Step 4: Implement whitespace-only, ANSI-aware wrapping**

In `render/layout.go`, pass through any line containing `\t` before measuring
it: terminal tab stops are context-dependent, `ansi.DecodeSequence` reports a
tab as a zero-cell C0 control, and treating it as break whitespace could delete
it. Pass through whitespace-only lines as well. For other lines, scan with
`ansi.DecodeSequence` and its parser state so escape/control sequences are
copied into the current token with zero width and printable sequences
contribute the decoder's cell width. Classify printable sequences with
`unicode.IsSpace`, except `U+00A0`; preserve leading whitespace on the original
segment and discard whitespace only at an inserted break. Do not call
`ansi.Wrap` or `ansi.Wordwrap` in this function.

Implement this complete shape:

```go
func wordWrapBodyLine(line string, width int) []string {
	if width <= 0 || strings.ContainsRune(line, '\t') ||
		strings.TrimSpace(line) == "" || ansi.StringWidth(line) <= width {
		return []string{line}
	}

	var lines []string
	var current, word, space strings.Builder
	currentWidth, wordWidth, spaceWidth := 0, 0, 0
	state := byte(ansi.NormalState)
	parser := ansi.NewParser()

	flushLine := func() {
		lines = append(lines, current.String())
		current.Reset()
		currentWidth = 0
	}
	placeWord := func() {
		if word.Len() == 0 {
			return
		}
		if currentWidth > 0 && currentWidth+spaceWidth+wordWidth > width {
			flushLine()
			space.Reset()
			spaceWidth = 0
		}
		current.WriteString(space.String())
		current.WriteString(word.String())
		currentWidth += spaceWidth + wordWidth
		space.Reset()
		word.Reset()
		spaceWidth, wordWidth = 0, 0
	}

	for rest := line; rest != ""; {
		seq, cellWidth, n, nextState := ansi.DecodeSequence(rest, state, parser)
		state, rest = nextState, rest[n:]
		plain := ansi.Strip(seq)
		r, _ := utf8.DecodeRuneInString(plain)
		if plain != "" && unicode.IsSpace(r) && r != '\u00a0' {
			placeWord()
			space.WriteString(seq)
			spaceWidth += cellWidth
			continue
		}
		word.WriteString(seq)
		wordWidth += cellWidth
	}
	placeWord()
	current.WriteString(space.String()) // retain original trailing whitespace
	if current.Len() > 0 || len(lines) == 0 {
		lines = append(lines, current.String())
	}
	return lines
}
```

Import `strings`, `unicode`, and `unicode/utf8`. Add an explicit test with an
SGR open/reset sequence around the field prefix followed by enough prose to
wrap; this pins decoder state and proves both escape sequences survive.

- [ ] **Step 5: Run the primitive tests and verify GREEN**

Run: `go test ./render -run TestWordWrapBodyLine -count=1 -v`

Expected: PASS.

- [ ] **Step 6: Write failing layout-pipeline tests**

Add tests for `RenderLayout` covering:

1. `BodyWidth: 0` matches `RenderWithWidth` exactly and maps each original prepared line.
2. A 20-cell body width creates continuation rows with the same logical ID.
3. Existing blank lines remain separate mapped body rows.
4. A continuation beginning `On since Tuesday...` does not acquire field styling; compare ANSI-stripped text and count only the original field style sequence.
5. A long hyphenated URL remains on one row, even wider than the limit.
6. A body plus a wrapped error maps every error row to `NoBodyLine`.
7. An empty failure contains no body mapping and reports `BodyLineCount == 0`.
8. A successful empty response maps its generated `(no response body)` row and final empty row to `NoBodyLine`, with `BodyLineCount == 0`.
9. `DisplayLineFor` clamps negative/too-large logical lines to the nearest existing body line, while `LogicalLineAt` clamps display indices and returns `NoBodyLine` for error/final rows.
10. A tilde.team response maps the post-`reflowPronouns` physical lines, proving preparation happens before IDs are assigned.

Use a no-colour profile for text/map assertions and one colour-profile test for the field-highlight regression.

- [ ] **Step 7: Run the layout tests and verify RED**

Run: `go test ./render -run 'TestRenderLayout|TestLayoutLineMap' -count=1 -v`

Expected: FAIL — `RenderLayout`, `Layout`, and `LayoutOptions` are undefined.

- [ ] **Step 8: Refactor the existing renderer through `RenderLayout`**

Move the current rendering pipeline into `RenderLayout`. Prepare the body first; split prepared bytes into physical lines without turning a terminal newline into an extra logical body line; highlight each original line once; wrap its ANSI-bearing result when `BodyWidth > 0`; append one line-map entry per emitted segment. Render errors with `ansi.Wrap(text, opts.ErrorWidth, "")` exactly as today and map all their rows to `NoBodyLine`.

Keep the existing API as thin compatibility delegates:

```go
func RenderWithWidth(t finger.Target, body []byte, queryErr error, profile colorprofile.Profile, darkBackground bool, width int) string {
	return RenderLayout(t, body, queryErr, profile, darkBackground, LayoutOptions{
		ErrorWidth: width,
	}).Text
}
```

Do not route the body through wrapping when `BodyWidth <= 0`. Preserve the renderer's final newline and all existing golden output.

- [ ] **Step 9: Run renderer tests and verify compatibility**

Run:

```bash
go test ./render -count=1
go test ./... -run 'Render|Wrap|Field' -count=1
```

Expected: PASS, including every pre-existing render golden and error-wrap test.

- [ ] **Step 10: Conditionally commit the renderer slice**

Only if the user has authorised commits:

```bash
git add render/layout.go render/layout_test.go render/render.go render/wrap_test.go
git commit -m "feat(render): add optional response body layout"
```

---

### Task 2: Make the reader preserve source position and wrap before link overlay

**Files:**
- Read first: `tui/CLAUDE.md`
- Modify: `tui/reader.go`
- Test: `tui/reader_test.go`

**Interfaces:**

Extend `readerModel` with the visible render state:

```go
layout  render.Layout
wrapped bool
raw     bool
```

Define one position value and make the entry path consume it:

```go
type readerPosition struct {
	logicalLine int
	hasLogical  bool
	fallbackY   int
}

func (m readerModel) position() readerPosition
func (m *readerModel) setEntryWithLinks(entry Entry, links []Link, wrapped bool, position readerPosition)
func (m *readerModel) setWrapped(wrapped bool)
func (m readerModel) topLogicalLine() int
func (m *readerModel) restoreLogicalLine(logicalLine int)
```

Keep `setEntry(entry Entry)` as an unwrapped test/backward-compatible helper by
delegating to `setEntryWithLinks(entry, nil, false, readerPosition{})`.

Keep a small `bodyWrapWidth` helper:

```go
func bodyWrapWidth(viewportWidth int, wrapped bool) int {
	if !wrapped || viewportWidth <= 0 {
		return 0
	}
	return min(viewportWidth, 80)
}
```

- [ ] **Step 1: Read the TUI package contract**

Run: `sed -n '1,320p' tui/CLAUDE.md`

- [ ] **Step 2: Write failing reader tests for layout and position**

Add focused tests to `tui/reader_test.go`:

- `TestReaderWrapUsesViewportWidthCappedAt80`: compare a 100-column reader and a 45-column reader, and assert display row widths/line counts.
- `TestReaderTogglePreservesTopLogicalLineAndResetsHorizontalOffset`: set a body with several long physical lines, scroll to a continuation/source row, set a non-zero X offset, toggle both directions, and assert the same logical line is at the top and `XOffset() == 0`.
- `TestReaderResizePreservesTopLogicalLineAndResetsHorizontalOffset`: resize a wrapped reader across 100, 60, and 45 columns.
- `TestReaderHeightOnlyResizeDoesNotRerenderOrResetHorizontalOffset`: change only height and assert the stored layout/content and X offset are unchanged.
- `TestReaderRawResizeKeepsRawContent`: call `setRaw` on an entry whose normal wrapped rendering differs, resize width and height, and assert the viewport still contains the exact raw body while X resets only on the width change.
- `TestReaderTopErrorRowFallsBackToLastBodyLine`: use a body plus multi-row error, place the old top inside the error, rerender, and assert restoration targets the final body line.
- `TestReaderEmptyFailureResizeUnchanged`: no body target exists, so resizing retains the existing safe/clamped behavior.

Use `m.layout.LogicalLineAt(m.viewport.YOffset())` for semantic assertions rather than hard-coding continuation-row numbers.

- [ ] **Step 3: Write failing focused-link tests**

Use a body in which prose wraps before a URL and another in which a hyphenated URL itself is wider than the body width. Assert all of the following after toggling on:

- the same `focusedLink` index is retained;
- the URL still receives the focus styling and OSC-8 overlay;
- its action/classification is unchanged;
- `scrollToFocusedLink` finds its occurrence in the wrapped layout;
- the viewport offset is the existing `max(0, linkDisplayLine-height/2)` centre calculation.

Name the regression test `TestReaderFocusedLinkRecentresInWrappedLayout`; “preserves focused link” alone is not sufficient.

- [ ] **Step 4: Run the reader tests and verify RED**

Run: `go test ./tui -run 'TestReader(Wrap|Toggle|Resize|HeightOnly|Raw|TopError|EmptyFailure|FocusedLink)' -count=1 -v`

Expected: FAIL — the reader lacks wrapped layout state and the new `setEntryWithLinks` argument.

- [ ] **Step 5: Centralise reader rendering**

Replace the string-returning `render` helper with a layout-returning helper:

```go
func (m readerModel) render(entry Entry) render.Layout {
	return render.RenderLayout(
		entry.Target, entry.Body, entry.Err,
		m.profile, m.darkBackground,
		render.LayoutOptions{
			ErrorWidth: m.width,
			BodyWidth:  bodyWrapWidth(m.width, m.wrapped),
		},
	)
}
```

Add one `setRenderedContent` helper which stores the pre-overlay `m.layout`,
then calls `applyLinkOverlay(m.layout.Text, ...)`, then calls
`viewport.SetContent`. This ordering is mandatory: the renderer and
`wordWrapBodyLine` must never see OSC-8-wrapped tokens.

Update `scrollToFocusedLink` to search `m.layout.Text`; do not call `m.render`
internally and do not store the post-overlay string in `Layout`. `Layout.Text`
is the pre-overlay wrapped text, which is the correct string for
`strings.Index(remaining, link.Raw)`. The overlay inserts no newlines, so its
display-row count is identical while OSC-8 bytes cannot interfere with raw
token matching.

The single content setter should have this shape so every caller uses the same
ordering:

```go
func (m *readerModel) setRenderedContent(layout render.Layout) {
	m.layout = layout
	text := applyLinkOverlay(layout.Text, m.links, m.focusedLink, m.styles)
	m.viewport.SetContent(text)
}
```

- [ ] **Step 6: Implement the single post-render positioning contract**

`readerModel.position()` captures the exact `YOffset` fallback and the logical
top. If the mapped top is `render.NoBodyLine` but the layout has body lines, it
uses `BodyLineCount-1`; with no body it leaves `hasLogical` false.

```go
func (m readerModel) topLogicalLine() int {
	logicalLine := m.layout.LogicalLineAt(m.viewport.YOffset())
	if logicalLine == render.NoBodyLine && m.layout.BodyLineCount > 0 {
		return m.layout.BodyLineCount - 1
	}
	return logicalLine
}

func (m readerModel) position() readerPosition {
	logicalLine := m.topLogicalLine()
	return readerPosition{
		logicalLine: logicalLine,
		hasLogical:  logicalLine != render.NoBodyLine,
		fallbackY:   m.viewport.YOffset(),
	}
}
```

Every normal-reader render, from every caller, performs this sequence exactly
once:

```go
layout := m.render(entry)
m.setRenderedContent(layout) // render -> overlay -> SetContent
switch {
case m.focusedLink >= 0:
	m.scrollToFocusedLink(m.links)
case position.hasLogical:
	m.restoreLogicalLine(position.logicalLine)
default:
	m.viewport.SetYOffset(position.fallbackY)
}
```

`setEntryWithLinks` owns this complete sequence; remove its old unconditional
trailing `scrollToFocusedLink` and do not let app callers perform another
`restoreLogicalLine` or `SetYOffset` afterwards. New navigation passes a zero
`readerPosition`, history/refresh pass their stored position, and link-focus
renders pass the current position; focused-link centring wins in all cases.

`setWrapped` captures `position()` before changing the flag, calls
`setEntryWithLinks` with that captured value, and resets X offset to zero.
`setProfile` and `setBackground` capture and reuse the same position but do not
reset X because neither changes geometry or wrapping.

Add `raw bool` to the reader. `setRaw` sets it true and clears `layout`;
`setEntryWithLinks` sets it false. This prevents a resize from mistaking source
content for a normal rendered layout.

In `setSize`, compare before overwriting the stored width:

```go
func (m *readerModel) setSize(width, height int) {
	oldWidth := m.width
	widthChanged := width != oldWidth
	position := m.position()
	m.width, m.height = width, height
	if width <= 0 || height <= 0 {
		return
	}
	m.viewport.SetWidth(width)
	viewportHeight := height - chromeRows
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	m.viewport.SetHeight(viewportHeight)
	if !widthChanged {
		return // height-only: no re-render and no horizontal reset
	}
	m.viewport.SetXOffset(0)
	if m.current != nil && !m.raw {
		m.setEntryWithLinks(*m.current, m.links, m.wrapped, position)
	}
}
```

The width/height validity guard and viewport dimension updates remain before
the early return. During raw view a width change resizes the viewport and
resets X, but does not replace the raw bytes with rendered or wrapped content.

- [ ] **Step 7: Update reader call sites mechanically enough to compile**

Pass `false, readerPosition{}` temporarily at app/test call sites that do not
yet have history state. This is scaffolding only; Task 3 replaces app call
sites with `node.wrapped` and stored positions. Do not change behavior outside
the reader yet.

- [ ] **Step 8: Run reader and link tests and verify GREEN**

Run:

```bash
go test ./tui -run 'TestReader|TestApplyLinkOverlay|TestScrollToFocusedLink' -count=1
go test ./tui -run 'Link' -count=1
```

Expected: PASS. Existing unwrapped reader behavior remains unchanged.

- [ ] **Step 9: Conditionally commit the reader slice**

Only if commits are authorised:

```bash
git add tui/reader.go tui/reader_test.go
git commit -m "feat(tui): render optional wrapped reader layouts"
```

---

### Task 3: Add per-response state, key routing, contextual Help, and refresh restoration

**Files:**
- Modify: `tui/keys.go`
- Modify: `tui/help.go`
- Modify: `tui/app.go`
- Test: `tui/keys_test.go`
- Test: `tui/help_test.go`
- Test: `tui/app_test.go`
- Test: `tui/request_test.go`

**Interfaces:**

Add `Wrap key.Binding` immediately after `Raw` in `keyMap`, and `wrapped bool` to `histNode`. Add logical position and wrapping to refresh state:

```go
type refreshViewState struct {
	state       appState
	scrollY     int
	logicalLine int
	hasLogical  bool
	wrapped     bool
	linkRaw     string
	listFilter  string
	selected    userItem
}
```

Add:

```go
func (m *appModel) toggleWrap() tea.Cmd
func (n histNode) readerPosition() readerPosition
func (v refreshViewState) readerPosition() readerPosition
```

- [ ] **Step 1: Write failing key and availability tests**

Extend the key-map test to require `w` / `wrap`. Add an availability matrix in `tui/app_test.go` that proves Wrap is enabled only for:

- normal `stateReader`, content focus, non-empty body;
- a partial response with both body and error.

And disabled for:

- focused input, where a typed `w` remains in the input;
- startpage and user list;
- raw/source view;
- links panel;
- empty success and empty failure;
- a pending request.

Also set `m.pos = len(m.history)` on an otherwise reader-shaped model, call
`updateKeymap`, and assert it does not panic and leaves Wrap disabled. This
pins the upper-bound guard independently of ordinary state invariants.

The raw binding remains broader than Wrap: this feature must not accidentally narrow existing `v` behavior over lists.

- [ ] **Step 2: Write failing interaction/state tests**

Add tests that press `w` through `Update` and assert:

1. Current `histNode.wrapped` toggles and `reader.wrapped` mirrors it.
2. The flash is exactly `wrapping on`, then `original layout`; `clearFlashMsg` still clears it.
3. A new navigated node starts unwrapped even when the previous node is wrapped.
4. Back navigation restores each node's independent wrap choice and logical source position.
5. Entering source view shows the original body layout; a width-changing app resize while source is visible keeps those exact raw bytes; leaving source view restores the node's wrapped presentation.
6. Opening/closing the links panel retains wrapping.
7. Refresh success replaces the entry but preserves wrapping and the old logical top line, clamping when the new body is shorter.
8. Refresh with the same focused raw link preserves focus and lets centring override top-line restoration in both wrapped and unwrapped modes.
9. A failed refresh leaves the visible wrapped entry unchanged.
10. The byte count and detected-link slice are identical before and after toggling, and after `clearFlashMsg` the ordinary status bar contains no persistent `wrapped`, `wrap`, or `unwrap` flag.

- [ ] **Step 3: Run the app tests and verify RED**

Run: `go test ./tui -run 'Test(Wrap|ReaderWrap|HistoryWrap|Raw.*Wrap|LinksPanel.*Wrap|ReaderRefresh.*Wrap)' -count=1 -v`

Expected: FAIL — no binding, state field, or toggle exists.

- [ ] **Step 4: Wire per-history wrapping through every reader restoration**

Add `wrapped bool` to `histNode`; rely on the zero value for every newly routed
response. Every normal reader call passes both the choice and exactly one
position value. Use `node.readerPosition()` for history restore, defensive
fallback, and exit from source view; `m.reader.position()` for link next/prev
and links-panel close; `readerPosition{}` for a newly navigated response; and
the captured refresh position for refresh. For example, history restore is:

```go
m.reader.setEntryWithLinks(node.entry, node.links, node.wrapped, node.readerPosition())
```

This includes `restore`, defensive list fallback, `exitRaw`, link focus navigation, links-panel close, `showRouted`, and refresh re-render. Extend history state without overloading the existing physical-row field:

```go
type histNode struct {
	entry       Entry
	state       appState
	scrollY     int    // exact fallback for a reader with no body mapping
	logicalLine int    // prepared source-body line at the top
	hasLogical  bool
	wrapped     bool
	listIdx     int
	listFltr    string
	listUsers   int
	listGeneric bool
	links       []Link
	linkIdx     int
}
```

Implement the conversion once:

```go
func (n histNode) readerPosition() readerPosition {
	return readerPosition{
		logicalLine: n.logicalLine,
		hasLogical:  n.hasLogical,
		fallbackY:   n.scrollY,
	}
}

func (v refreshViewState) readerPosition() readerPosition {
	return readerPosition{
		logicalLine: v.logicalLine,
		hasLogical:  v.hasLogical,
		fallbackY:   v.scrollY,
	}
}
```

In `snapshot`, always store `scrollY`; obtain
`logicalLine := m.reader.topLogicalLine()` and set
`hasLogical = logicalLine != render.NoBodyLine`. In `restore`, set the saved
link index before the single `setEntryWithLinks(..., n.readerPosition())` call;
do not restore an offset afterwards. This keeps error-only history restoration
intact while using stable source units whenever a body exists.

Call `snapshot()` at the start of `enterRaw`, before `setRaw`, so source-view
scrolling cannot overwrite or replace the normal reader position that
`exitRaw` passes back into `setEntryWithLinks`.

For refresh, capture `wrapped`, `scrollY`, and `reader.topLogicalLine()` before
the request. After `routeEntry`, assign `routed.node.wrapped = view.wrapped`.
Resolve `view.linkRaw` to the new link index before the refreshed reader's one
`setEntryWithLinks` call and pass `view.readerPosition()`; do not retain
the current `restoreRefreshView` pattern that calls `SetYOffset` after render.
List refresh continues to restore filter/selection through its existing path.

This deliberately changes the existing focused-link refresh contract even
when wrapping is off. Rename `TestReaderRefreshPreservesScrollAndLinkRaw` to
`TestReaderRefreshPreservesLinkAndRecentresIt`; with its current body and
six-row viewport, change the expected `YOffset` from `8` to `0` because the
restored link is on display row 1. Keep
`TestRefreshUsesViewCapturedAtRequestStart` as the unfocused counterexample:
it still expects the captured source position (row 8 in its unwrapped body).

- [ ] **Step 5: Implement `w` routing and feedback**

In `keys.go`:

```go
Wrap: key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "wrap")),
```

In `updateKeymap`, dynamically set Help before calculating candidates:

```go
wrapAction := "wrap"
if m.pos >= 0 && m.pos < len(m.history) && m.history[m.pos].wrapped {
	wrapAction = "unwrap"
}
m.keys.Wrap.SetHelp("w", wrapAction)
```

Include Wrap in the pending-disable set. Enable it only when:

```go
canWrap := inReader && !m.showingLinks &&
	m.pos >= 0 && m.pos < len(m.history) &&
	len(m.history[m.pos].entry.Body) > 0
```

Place the key-handling case beside Raw. `toggleWrap` flips only `history[pos].wrapped`, calls `reader.setWrapped`, and returns `m.setFlash("wrapping on")` or `m.setFlash("original layout")`.

Use this exact transition:

```go
func (m *appModel) toggleWrap() tea.Cmd {
	if m.pos < 0 || m.pos >= len(m.history) {
		return nil
	}
	node := &m.history[m.pos]
	node.wrapped = !node.wrapped
	m.reader.setWrapped(node.wrapped)
	if node.wrapped {
		return m.setFlash("wrapping on")
	}
	return m.setFlash("original layout")
}
```

- [ ] **Step 6: Write failing contextual-Help tests**

Extend `helpCandidates` expectations so Wrap is immediately after Raw and before Refresh. Test both labels. At each established geometry (`80x24` dark, `80x24` light, `100x30`, `60x20`, `100x50`, `45x24`), assert:

- `w` is retained in normal reader Help;
- toggling changes only its description, not which binding keys are retained;
- the retained key sequence preserves `...,v,w,r,...`;
- every legacy binding retained by the same layout before Wrap is inserted,
  including `q quit` wherever it was retained, remains retained after
  insertion with both `w wrap` and `w unwrap`.

Compute that legacy baseline in the test rather than guessing it: copy
`m.helpCandidates()`, remove the candidate whose Help key is `w`, and pass that
slice through `layoutHelp` at the same width/height. For both Wrap labels,
assert every key in the baseline layout is present in the actual layout:

```go
baselineCandidates := slices.DeleteFunc(
	append([]key.Binding(nil), m.helpCandidates()...),
	func(binding key.Binding) bool { return binding.Help().Key == "w" },
)
baseline := helpKeys(layoutHelp(
	baselineCandidates, m.common.styles,
	m.common.width, m.common.height-1,
).bindings)
got := helpKeys(m.helpLayout().bindings)
for _, legacyKey := range baseline {
	if !slices.Contains(got, legacyKey) {
		t.Errorf("adding Wrap evicted legacy binding %q: baseline=%v got=%v", legacyKey, baseline, got)
	}
}
```

Also build a deliberately smaller layout where Wrap is dropped, open Help, press `w`, and assert Help remains open and wrapping does not change. This pins the retained-prefix execute gate.

In the ordinary reader geometry, open Help and press `w` through `Update`; assert
Help closes, the returned replay command emits the same key message, and feeding
that message through `Update` toggles the current node. This separately pins the
successful Help replay path.

- [ ] **Step 7: Run the Help tests and verify RED, then add Wrap to candidates**

Run: `go test ./tui -run 'Test.*Help.*Wrap|TestHelp.*Geometry|TestHelpCandidates' -count=1 -v`

Expected before implementation: FAIL. Then add `m.keys.Wrap` immediately after `m.keys.Raw` in `helpCandidates` and update only the exact candidate/admission expectations affected by the new binding.

- [ ] **Step 8: Run the complete TUI test package**

Run: `go test ./tui -count=1 -race`

Expected: PASS. Only focused-link refresh changes precedence: update the named
test above to expect centring. Unfocused refresh continues to assert the
captured source position, and unrelated exact-scroll tests must not be relaxed.

- [ ] **Step 9: Conditionally commit the app slice**

Only if commits are authorised:

```bash
git add tui/keys.go tui/help.go tui/app.go tui/keys_test.go tui/help_test.go tui/app_test.go tui/request_test.go
git commit -m "feat(tui): add per-response word-wrap toggle"
```

---

### Task 4: Add representative offline TUI-review content and scenes

**Files:**
- Modify: `docs/tui-review/fixtures/fingerd/main.go`
- Test: `docs/tui-review/fixtures/fingerd/main_test.go`
- Modify: `docs/tui-review/responses-tour.tape`
- Modify: `docs/tui-review/README.md`

- [ ] **Step 1: Re-read the complete visual-review instructions**

Run: `sed -n '1,320p' docs/tui-review/README.md`

Preserve the documented Wait/Sleep/Screenshot ordering and all existing fixture isolation.

- [ ] **Step 2: Write a failing fixture-shape test**

Add `TestResponseForWrapPlan` asserting `responseFor("wrapplan")` is deterministic and contains:

- one timestamped prose line around 110 cells;
- one ordinary paragraph around 150 cells;
- one 500–700-cell serialised-prose line;
- one preformatted block whose lines are all at most 80 cells;
- one hyphenated URL/token wider than the narrow review geometry.

Use `ansi.StringWidth` or `lipgloss.Width`, not `len`. Assert the fixture contains no copied usernames, addresses, or sentences from the crawl decision record.

Pin the stable fixture markers and URL explicitly:

```go
const wrapContinuationMarker = "WRAP-CONTINUATION-MARKER"
const wrapLongURL = "https://example.com/this-is-one-deliberately-indivisible-hyphenated-address-that-must-stay-intact"
```

- [ ] **Step 3: Run the fixture test and verify RED**

Run: `go test ./docs/tui-review/fixtures/fingerd -run TestResponseForWrapPlan -count=1 -v`

Expected: FAIL — `wrapplan` falls through to the generic response.

- [ ] **Step 4: Add original representative fixture prose**

Add this original fixture text; four copies produce a 628-cell ASCII extreme
line, inside the observed 500–700-cell range:

```go
const wrapContinuationMarker = "WRAP-CONTINUATION-MARKER"
const wrapLongURL = "https://example.com/this-is-one-deliberately-indivisible-hyphenated-address-that-must-stay-intact"

func wrapPlanBody() []byte {
	stamp := "2026-08-20 14:32 — This timestamped note runs past eighty cells while remaining a single ordinary prose line."
	paragraph := "This representative paragraph stays on one physical line so review can show optional wrapping without inventing metadata; " + wrapContinuationMarker + " follows."
	dispatchUnit := "Archived dispatches sometimes arrive serialised into one physical line, with sentence after sentence preserving meaning but not a comfortable reading width. "
	extreme := strings.Repeat(dispatchUnit, 4)
	preformatted := []string{
		"    HOST             STATE       NOTE",
		"    relay.example    ready       spacing is intentional",
	}

	lines := []string{
		"Login: wrapplan",
		"Name: Wrap Review",
		"Plan:",
		stamp,
		"",
		paragraph,
		"",
		extreme,
		"",
	}
	lines = append(lines, preformatted...)
	lines = append(lines, "", "Read "+wrapLongURL+" when horizontal scrolling is useful.")
	return []byte(strings.Join(lines, "\n") + "\n")
}
```

Add `case "wrapplan": return wrapPlanBody()` in `responseFor`. Do not paste or
lightly edit personal live Finger bodies.

- [ ] **Step 5: Run fixture tests and verify GREEN**

Run: `go test ./docs/tui-review/fixtures/fingerd -count=1`

Expected: PASS.

- [ ] **Step 6: Add two scenes to the response tour**

Add `wrap-original` and `wrap-enabled` to `responses-tour.tape`; the existing recorder fans each scene out to all six geometries, producing 12 new frames total.

For `wrap-original`, request `wrapplan@127.0.0.1:2479`, wait for a unique fixture marker, retain the established `Wait -> Sleep 1500ms -> Screenshot` sequence, and capture the default unwrapped layout.

For `wrap-enabled`, press `w`, wait for a continuation-line marker that only becomes visible after wrapping, then wait until the ordinary `tab 1 link` status has replaced the two-second flash, and only then use `Sleep 1500ms -> Screenshot`. The link hint is retained at all six geometries, while lower-priority `? help` is deliberately shed at 45 columns. Do not require either screenshot to catch `wrapping on`; that string is deterministic in Task 3's tests.

Do not add a third wrap-Help scene. Existing `reader-help` plus the six-geometry Help tests cover it without another six frames.

Insert these commands after `reader-scroll.png` and before the next request;
retain the surrounding `Type "i"`, `Sleep 200ms`, and `Ctrl+U` input-reset idiom:

```text
Type "i"
Sleep 200ms
Ctrl+U
Type "wrapplan@127.0.0.1:2479"
Enter
Wait+Screen /2026-08-20 14:32/
Sleep 1500ms
Screenshot out/tui-review/wrap-original.png
Sleep 250ms

Type "w"
Wait+Screen /WRAP-CONTINUATION-MARKER/
Wait+Screen /tab 1 link/
Sleep 1500ms
Screenshot out/tui-review/wrap-enabled.png
Sleep 250ms
```

- [ ] **Step 7: Update the review README's scene inventory**

Document both scenes, the representative/paraphrased origin of the fixture shapes, and the review questions: original layout remains intact; wrapped prose has a readable measure; preformatted lines and the overlong URL remain intact; narrow layouts do not clip ordinary prose.

- [ ] **Step 8: Record and inspect the review output**

Run: `make review-tui`

Expected: all tapes complete. Inspect the response-tour contact sheet first, then open the first-class `80x24` dark/light and `100x30` frames individually; use `60x20` for breakpoint behavior and `45x24`/`100x50` diagnostically. If any frame is stale, clipped, or captures the flash, fix the tape/implementation and re-record.

- [ ] **Step 9: Conditionally commit the review slice**

Generated frames remain gitignored. Only if commits are authorised:

```bash
git add docs/tui-review/fixtures/fingerd/main.go docs/tui-review/fixtures/fingerd/main_test.go docs/tui-review/responses-tour.tape docs/tui-review/README.md
git commit -m "test(tui): add word-wrap visual review scenes"
```

---

### Task 5: Update living documentation and package contracts

**Files:**
- Modify: `README.md`
- Modify: `docs/user-facing-messages.md`
- Modify: `render/CLAUDE.md`
- Modify: `tui/CLAUDE.md`
- Already added: `docs/superpowers/specs/2026-08-20-reader-word-wrap-design.md`
- Already added: `docs/decisions/2026-08-20-reader-wrap-crawl.md`

- [ ] **Step 1: Update README feature prose**

After the first Usage paragraph, add:

```markdown
Responses keep the server's original line layout by default; in the reader, press `w` to word-wrap long prose for the current response, and press it again to restore the original layout.
```

Do not describe it as `.plan` detection or imply a typed Finger section.

- [ ] **Step 2: Inventory all four new user-facing strings**

In `docs/user-facing-messages.md`, add rows for:

```text
w wrap
w unwrap
wrapping on
original layout
```

Give their exact source locations/surfaces and preserve the table's existing grouping.

Use these exact entries:

```markdown
| `wrapping on` / `original layout` | `tui/app.go` (`toggleWrap`) | Transient flash after enabling or disabling word wrapping for the current response. |
```

under **TUI Input**, and:

```markdown
| `w wrap` / `w unwrap` | `tui/keys.go`, `tui/app.go` (`updateKeymap`) | Contextual reader-display binding; available only for a non-empty body in normal reader view. |
```

under **TUI Help**.

Because Task 1 moves the empty-body and error rendering implementation into
`render/layout.go`, update the two existing **Response Body Renderer** source
cells from `render/render.go` to `render/layout.go` while retaining
`RenderWithWidth` as the public surface named in the prose.

- [ ] **Step 3: Update package-local architecture notes**

In `render/CLAUDE.md`, replace the absolute “never reflowed” paragraph with:

```markdown
`RenderWithWidth(…, width)` remains the compatibility renderer: it wraps the error line only at `width` cells and never reflows the response body. `RenderLayout` is the explicit TUI layout path; when given a positive body width it wraps each existing physical body line independently at whitespace, never splits an overlong token, and returns a display-row-to-logical-body-line map. A tab-bearing or whitespace-only physical line passes through intact, and non-breaking space is not a break opportunity. Error text still wraps independently at the full viewport width.
```

In `tui/CLAUDE.md`, add this paragraph beside the history/reader contract:

```markdown
Reader word wrapping is an opt-in per-`histNode` display preference: new responses start unwrapped, refresh and Back preserve the choice, and source view temporarily ignores it. The renderer's logical-line map keeps the same source line visible across toggles and width changes (horizontal offset resets); a focused link instead keeps the existing centred-link policy, including after refresh. Every normal render positions exactly once after render, overlay, and content assignment. Height-only resizing does not re-render, and the reader's raw-view flag prevents a width change from replacing source bytes with a normal render. The reader applies link focus/OSC-8 overlays only after wrapping, while detection and byte metadata continue to use the original `Entry.Body`.
```

- [ ] **Step 4: Confirm unrelated living docs do not need edits**

No wire behavior changes, so `docs/rfc1288-conformance.md` stays unchanged. No CLI option, bookmark grammar, or shipped keybinding table exists in the man page, so `man/lookit.1` stays unchanged. Verify those assumptions with:

Run: `rg -n "keybinding|view source|response body|wrap" man/lookit.1 docs/rfc1288-conformance.md`

- [ ] **Step 5: Check documentation references and formatting**

Run:

```bash
rg -n 'w wrap|w unwrap|wrapping on|original layout' README.md docs render tui
git diff --check
```

Expected: every app-authored string has a living-doc entry; no whitespace errors.

- [ ] **Step 6: Conditionally commit documentation**

Only if commits are authorised:

```bash
git add README.md docs/user-facing-messages.md render/CLAUDE.md tui/CLAUDE.md docs/superpowers/specs/2026-08-20-reader-word-wrap-design.md docs/superpowers/plans/2026-08-20-reader-word-wrap.md docs/decisions/2026-08-20-reader-wrap-crawl.md
git commit -m "docs(tui): document reader word wrapping"
```

---

### Task 6: Full verification and review handoff

**Files:**
- Verify all modified files; do not introduce new behavior in this task.

- [ ] **Step 1: Format**

Run: `make fmt`

- [ ] **Step 2: Run focused tests once more**

Run:

```bash
go test ./render -count=1
go test ./tui -run 'Wrap|Reader|Help|Refresh|Raw|Link' -count=1
go test ./docs/tui-review/fixtures/fingerd -count=1
```

Expected: PASS.

- [ ] **Step 3: Run the exact CI gate**

Run: `make check`

Expected: all five gates pass: vet, gofmt check, golangci-lint, cross-compile, and `go test ./... -race`.

- [ ] **Step 4: Inspect the final diff**

Run:

```bash
git status --short
git diff --stat
git diff --check
git diff -- render tui README.md docs/user-facing-messages.md docs/tui-review docs/decisions docs/superpowers
```

Confirm: no changes under `finger/`; no stored-body mutation; no viewport soft-wrap; no default wrapping; no copied crawl bodies; no generated review frames staged.

- [ ] **Step 5: Verify the approved behavior matrix manually from tests and frames**

- Default response: unwrapped.
- `w`: wraps at `min(viewport, 80)`; second `w`: original layout.
- Long and hyphenated tokens: intact and horizontally reachable.
- Blank lines, whitespace-only lines, tab-bearing lines, non-breaking spaces,
  and preformatted short lines: intact under their explicit contracts.
- Same logical source line: retained through toggle and resize.
- Focused link: same index, overlay/action intact, recentered in wrapped layout.
- New response: unwrapped; Back and refresh: per-entry choice preserved.
- Source view: always original; exit restores wrap choice.
- Source-view resize: raw bytes remain visible; height-only reader resize does
  not re-render or reset horizontal offset.
- Help: `w wrap`/`w unwrap`, stable retained membership at six geometries, and
  no legacy binding is evicted by inserting Wrap.
- Status bar: only transient flash changes; ordinary information returns intact.

- [ ] **Step 6: Conditionally commit any verification-only fixes**

If authorised and formatting/test corrections remain:

```bash
git add -u
git commit -m "test(tui): complete word-wrap coverage"
```

Do not create an empty commit. Do not push, open a PR, or merge without explicit user approval.

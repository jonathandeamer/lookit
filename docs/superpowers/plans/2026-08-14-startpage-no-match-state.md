# Startpage No-Match State Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A startpage filter that matches nothing says so, naming the query, instead of leaving the body blank.

**Architecture:** `startModel.View` already renders `m.list.View()`. While the filter is being typed with a non-empty value and no visible items, Bubbles paints the content region blank; the startpage overwrites its first row with a one-line message styled from `list.Styles.NoItems`. No layout arithmetic changes.

**Tech Stack:** Go, Bubble Tea v2, Bubbles list v2, Lip Gloss v2, `github.com/charmbracelet/x/ansi`.

**Spec:** `docs/superpowers/specs/2026-08-14-startpage-no-match-state-design.md`

## Global Constraints

- Do not touch `finger/` or any security invariant (sanitize at ingress, `hasControl` at egress, port-79 pinning).
- Copy is lowercase, unpunctuated, and honest: it names the query, never the catalog. The exact string is `no match for “<query>”` with typographic quotes (U+201C, U+201D).
- The message is exactly one terminal row at every width, in both the one-line (≥72 columns) and two-line (<72 columns) delegate layouts.
- Do not change the file-level empty state or its gate on `len(m.list.Items()) == 0`.
- Commit messages follow Conventional Commits. No `Co-Authored-By` and no AI attribution trailers.
- Do not commit outside this branch, and do not push or open a PR.

---

### Task 1: Draw the no-match message

**Files:**
- Modify: `tui/start.go` (the `startModel.View` method and new helpers beside it)
- Test: `tui/start_test.go`

**Interfaces:**
- Consumes: `startModel`, `newStart(common *commonModel, sections []startSection, notice, empty string) startModel`, `startModel.update`, the test helpers `testCommon()`, `threeSections()`, and `typeStartFilter(t *testing.T, m startModel, query string) startModel` — all already in the package.
- Produces: `func (m startModel) noMatchMessage() string`, `func (m startModel) filterPromptHeight() int`, and `func replaceLine(view string, n int, line string) string`.

- [ ] **Step 1: Write the failing tests**

Add to `tui/start_test.go`:

```go
func TestStartFilterNoMatchNamesTheQuery(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = typeStartFilter(t, m, "zzzzzz")

	if got := len(m.list.VisibleItems()); got != 0 {
		t.Fatalf("filter matched %d rows, want a zero-match filter for this test", got)
	}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "no match for “zzzzzz”") {
		t.Fatalf("no-match message missing from view:\n%s", view)
	}
	if !strings.Contains(view, "zzzzzz") || !strings.Contains(view, "Filter:") {
		t.Fatalf("filter prompt must survive alongside the message:\n%s", view)
	}
}

func TestStartFilterNoMatchSitsBelowThePrompt(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = typeStartFilter(t, m, "zzzzzz")

	lines := strings.Split(ansi.Strip(m.View()), "\n")
	offset := m.filterPromptHeight()
	if offset < 1 || offset >= len(lines) {
		t.Fatalf("filter prompt height %d is outside the %d-line view", offset, len(lines))
	}
	if got := strings.TrimSpace(lines[offset]); got != "no match for “zzzzzz”" {
		t.Fatalf("line %d = %q, want the no-match message", offset, got)
	}
	if !strings.Contains(lines[0], "Filter:") {
		t.Fatalf("line 0 = %q, want the filter prompt", lines[0])
	}
}

func TestStartFilterWithMatchesHasNoMessage(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = typeStartFilter(t, m, "plan")

	if len(m.list.VisibleItems()) == 0 {
		t.Fatal("expected at least one match for this test")
	}
	if strings.Contains(ansi.Strip(m.View()), "no match for") {
		t.Fatal("a matching filter must not show the no-match message")
	}
}

func TestStartEmptyFilterHasNoMessage(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})

	if strings.Contains(ansi.Strip(m.View()), "no match for") {
		t.Fatal("pressing / with nothing typed must not show the no-match message")
	}
}

func TestStartUnfilteredHasNoMessage(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")

	if strings.Contains(ansi.Strip(m.View()), "no match for") {
		t.Fatal("an unfiltered startpage must not show the no-match message")
	}
}

func TestStartFilterNoMatchTruncatesAtNarrowWidth(t *testing.T) {
	common := testCommon()
	common.width = 20
	m := newStart(common, threeSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = typeStartFilter(t, m, "zzzzzzzzzzzzzzzzzzzz")

	message := ansi.Strip(m.noMatchMessage())
	if message == "" {
		t.Fatal("expected a no-match message at 20 columns")
	}
	if strings.Contains(message, "\n") {
		t.Fatalf("message must stay one row tall, got %q", message)
	}
	if w := lipgloss.Width(message); w > m.list.Width() {
		t.Fatalf("message width %d exceeds list width %d: %q", w, m.list.Width(), message)
	}
	if !strings.HasSuffix(message, "…") {
		t.Fatalf("message should truncate with an ellipsis at this width, got %q", message)
	}
}
```

- [ ] **Step 2: Run the tests and verify RED**

Run: `go test ./tui -run 'TestStartFilterNoMatch|TestStartFilterWithMatches|TestStartEmptyFilterHasNoMessage|TestStartUnfilteredHasNoMessage' -count=1 -v`

Expected: FAIL — `m.noMatchMessage` and `m.filterPromptHeight` are undefined, and the view assertions fail because the body is blank.

- [ ] **Step 3: Implement the message**

In `tui/start.go`, add beside `View`:

```go
// noMatchMessage is the body copy for a filter that matches nothing. It names
// the query rather than the catalog: nothing is wrong with the catalog, and the
// query is the thing the user can change. Bubbles cannot supply this — its
// populatedView returns an empty string while the filter is being typed, so the
// "No entries." configured in newStart is unreachable in exactly this state.
//
// FilterApplied with zero matches is unreachable: bubbles' AcceptWhileFiltering
// handler resets filtering when the accepted filter matched nothing.
func (m startModel) noMatchMessage() string {
	if m.list.FilterState() != list.Filtering || len(m.list.VisibleItems()) > 0 {
		return ""
	}
	query := strings.TrimSpace(m.list.FilterValue())
	if query == "" {
		return "" // "/" pressed with nothing typed: every row is still on screen
	}
	// PaddingLeft(2) puts the message on the same left edge as the entry rows it
	// replaces and the filter prompt above it.
	style := m.list.Styles.NoItems.PaddingLeft(2)
	return ansi.Truncate(style.Render("no match for “"+query+"”"), m.list.Width(), "…")
}

// filterPromptHeight is how many rows bubbles draws above the list's content
// region while filtering: Styles.TitleBar wrapped around the single-line filter
// input, two rows with the default TitleBar's bottom padding. Derived rather
// than hardcoded so a style change moves the message with the prompt.
func (m startModel) filterPromptHeight() int {
	return lipgloss.Height(m.list.Styles.TitleBar.Render(m.list.FilterInput.View()))
}

// replaceLine overwrites line n of view, appending when the view is shorter so
// the caller's line can never be silently dropped.
func replaceLine(view string, n int, line string) string {
	lines := strings.Split(view, "\n")
	if n < 0 || n >= len(lines) {
		return view + "\n" + line
	}
	lines[n] = line
	return strings.Join(lines, "\n")
}
```

Then, in `View`, immediately after `body = m.list.View()` and before the
overview is prepended:

```go
		body = m.list.View()
		// A zero-match filter leaves the list's content region blank (bubbles
		// returns early while filtering); overwrite its first row.
		if message := m.noMatchMessage(); message != "" {
			body = replaceLine(body, m.filterPromptHeight(), message)
		}
```

- [ ] **Step 4: Run the tests and verify GREEN**

Run: `go test ./tui -run 'TestStartFilterNoMatch|TestStartFilterWithMatches|TestStartEmptyFilterHasNoMessage|TestStartUnfilteredHasNoMessage' -count=1 -v`

Expected: PASS.

- [ ] **Step 5: Run the whole tui package**

Run: `go test ./tui -count=1`

Expected: PASS. If a height-sensitive test fails, do not change the message's
height — the message replaces a row the list already reserved, so a failure
here means the replacement is inserting rather than overwriting. Confirm the
existing file-level empty-state test (a startpage with no rows at all) still
passes: that state is unchanged and must not start showing the no-match line.

- [ ] **Step 6: Update the View doc comment**

The comment above `View` currently says a zero-match filter "keeps the list,
which renders the filter input plus its own `"No entries."` (Styles.NoItems)".
That was never reachable. Replace that sentence with:

```go
// A filter that matches nothing keeps the list and overwrites the blank first
// row of its content region with noMatchMessage: bubbles' own "No entries."
// (Styles.NoItems) is unreachable while a filter is being typed.
```

- [ ] **Step 7: Run the gate and commit**

```bash
make check
git add tui/start.go tui/start_test.go
git commit -m "fix(startpage): explain a filter that matches nothing"
```

---

## Verification

- [ ] `make check` passes.
- [ ] `git diff --stat main` touches only `tui/start.go`, `tui/start_test.go`, and this plan's docs.

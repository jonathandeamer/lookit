# Startpage Uniform Section Spacing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every startpage section header exactly one blank row above it, replacing the three different gap sizes the page renders today.

**Architecture:** `startItems` currently encodes two independent spacing rules — an unconditional leading spacer and a spacer before SERVICES gated on section identity. Both collapse into one rule: at wide widths, insert a spacer before every header except the first, because bubbles' reserved filter row already supplies the first gap. The narrow layout assembles no spacers at all, since `renderHeader` already spends its first row on a blank. No delegate changes.

**Tech Stack:** Go 1.21+, Bubble Tea v2 (`charm.land/bubbletea/v2`), `charm.land/bubbles/v2/list`, `charm.land/lipgloss/v2`.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-08-13-startpage-uniform-section-spacing-design.md`.
- Conventional Commits. **No `Co-Authored-By` or AI-attribution trailers** in commits or PR bodies.
- `make check` must pass: `go vet ./...`, `gofmt -l .` empty, `golangci-lint run ./...`, `go test ./... -race`.
- Scope is `tui/start.go` and `tui/start_test.go`. Do not touch catalog data, `buildSections`, grouping, bookmarks, counts, keybindings, or any user-facing string.
- `startWideMinWidth` is 72. Wide means `width >= 72`.
- Run single tests with `go test ./tui/ -run TestName -count=1 -v`.

---

### Task 1: Make wide-layout spacing uniform

**Files:**
- Modify: `tui/start.go:23-31` (the `spacer` field comment), `tui/start.go:137-159` (`startItems`)
- Test: `tui/start_test.go`

**Interfaces:**
- Consumes: `startItem{spacer bool}`, `startSection{id, title, entries}`, `startWideMinWidth` — all already exist.
- Produces: `startItems(sections []startSection, width int) []list.Item` keeps its exact signature. Item counts change, which Task 1 Step 5 updates. New test helper `startHeaderLineIndex(t *testing.T, lines []string, title string) int`, used again in Task 2.

- [ ] **Step 1: Write the failing test**

Add to `tui/start_test.go`. The helper matches the header's rule, not the bare word, because the overview line also contains "BOOKMARKS".

```go
// startHeaderLineIndex finds the rendered section header for title. The
// overview line also contains "BOOKMARKS", so match the header's trailing
// rule rather than the word alone.
func startHeaderLineIndex(t *testing.T, lines []string, title string) int {
	t.Helper()
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), title+" ─") {
			return i
		}
	}
	t.Fatalf("no %s header line in:\n%s", title, strings.Join(lines, "\n"))
	return -1
}

// TestStartSectionSpacingIsUniformWhenWide asserts the decision (one blank row
// above every header) rather than the mechanism (how many spacer items exist),
// so it stays honest if the assembly changes again.
func TestStartSectionSpacingIsUniformWhenWide(t *testing.T) {
	common := testCommon()
	common.width = 100
	common.height = 40
	m := newStart(common, threeSections(), "", "")

	plain := stripANSIForLandingTest(m.View())
	lines := strings.Split(plain, "\n")

	for _, title := range []string{"BOOKMARKS", "COMMUNITIES", "SERVICES"} {
		header := startHeaderLineIndex(t, lines, title)
		if header < 2 {
			t.Fatalf("%s header at line %d, want room for a gap above it:\n%s", title, header, plain)
		}
		if got := strings.TrimSpace(lines[header-1]); got != "" {
			t.Errorf("line above %s = %q, want one blank row:\n%s", title, got, plain)
		}
		if got := strings.TrimSpace(lines[header-2]); got == "" {
			t.Errorf("two blank rows above %s, want exactly one:\n%s", title, plain)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./tui/ -run TestStartSectionSpacingIsUniformWhenWide -count=1 -v`

Expected: FAIL. `COMMUNITIES` reports `line above COMMUNITIES = "  @tilde.team"` (no gap), and `BOOKMARKS` reports two blank rows above it.

- [ ] **Step 3: Rewrite `startItems`**

Replace the whole function at `tui/start.go:137-159`:

```go
func startItems(sections []startSection, width int) []list.Item {
	var items []list.Item
	for _, s := range sections {
		// Exactly one blank row above every section header. The wide one-row
		// layout needs a spacer item to produce it — except above the first
		// header, where bubbles' reserved filter row (rendered whenever
		// filtering is enabled, even with SetShowTitle(false)) already
		// supplies one. The narrow two-row layout needs no spacer at any
		// boundary, because renderHeader spends its first row on a blank.
		if width >= startWideMinWidth && len(items) > 0 {
			items = append(items, startItem{spacer: true})
		}
		items = append(items, startItem{header: s.title, section: s.id})
		for _, e := range s.entries {
			items = append(items, startItem{entry: e, section: s.id})
		}
	}
	return items
}
```

Update the field comment at `tui/start.go:30`:

```go
	spacer  bool // one blank row above a section header, in the one-line layout
```

- [ ] **Step 4: Run the new test to verify it passes**

Run: `go test ./tui/ -run TestStartSectionSpacingIsUniformWhenWide -count=1 -v`

Expected: PASS.

- [ ] **Step 5: Update the tests whose contracts this reverses**

Five existing tests assert the old item counts. With `sectionGapSections()` (COMMUNITIES with 1 entry, SERVICES with 1 entry) the new counts are **5 wide** (header, entry, spacer, header, entry) and **4 narrow** (header, entry, header, entry).

Replace `TestStartItemsAlwaysBeginsWithSpacer` at `tui/start_test.go:57-68` entirely:

```go
func TestStartItemsBeginsWithTheFirstHeader(t *testing.T) {
	items := startItems(sectionGapSections(), 80)
	if len(items) == 0 {
		t.Fatal("got 0 items, want at least a header")
	}
	first, ok := items[0].(startItem)
	if !ok || first.header == "" {
		t.Fatalf("first item = %+v, want a header: bubbles' reserved filter row supplies the gap above it", items[0])
	}
}
```

In `TestStartSectionGapItemOnlyInWideTwoSectionLayout` (`tui/start_test.go:96-119`), rename it to `TestStartSpacerItemCounts` and change the `want` values:

```go
		{name: "wide both", width: 80, sections: sectionGapSections(), want: 5},
		{name: "narrow both", width: 40, sections: sectionGapSections(), want: 4},
		{name: "wide communities only", width: 80, sections: sectionGapSections()[:1], want: 2},
		{name: "wide services only", width: 80, sections: sectionGapSections()[1:], want: 2},
```

In `TestStartSectionGapResponsiveResizePreservesSelection` (`tui/start_test.go:513`), change `want 5` to `4` at line 522 and `want 6` to `5` at line 530, updating both message strings to match.

In `TestStartSectionGapResponsiveResizeSynchronizesAfterFilterClears` (`tui/start_test.go:576`), change `!= 6` to `!= 5` at line 584 and `!= 5` to `!= 4` at line 589, updating both message strings.

In `TestStartCursorSkipsHeaderAtPageBoundary` (`tui/start_test.go:592`), the narrow indices shift down by one now that no leading spacer is assembled. Change `m.list.Select(3)` to:

```go
	m.list.Select(2) // the COMMUNITIES header, on a later page
```

- [ ] **Step 6: Run the full package suite**

Run: `go test ./tui/ -count=1`

Expected: PASS. If anything else fails on an item index or count, it is the same class of change — fix the expectation, do not reintroduce a spacer.

Two existing tests must pass **unchanged**, and are the ones to worry about if they do not: `TestStartFilterDropsSectionGap` (a non-empty filter still contains no spacers and no headers) and `TestStartSectionGapRendersExactlyOneBlankRow` (the SERVICES boundary is still exactly one blank row at both 80 and 40 columns). Neither should be edited — if either fails, the assembly change is wrong.

- [ ] **Step 7: Commit**

```bash
git add tui/start.go tui/start_test.go
git commit -m "feat(startpage): give every section header one blank row above it"
```

---

### Task 2: Pin the narrow layout's spacing

**Files:**
- Test: `tui/start_test.go`

**Interfaces:**
- Consumes: `startHeaderLineIndex` from Task 1.
- Produces: nothing consumed later.

Task 1 already changed narrow behaviour (the two-row leading spacer is gone, so the top drops from three blank rows to two). This task pins that as an intended result rather than an accident, and records why it stops at two.

- [ ] **Step 1: Write the test**

```go
// TestStartSectionSpacingWhenNarrow pins the two-row layout. Section
// boundaries get exactly one blank row, supplied by renderHeader's own first
// row. The first header gets two: bubbles' reserved filter row sits above it
// and cannot be removed, and neither remedy works — a spacer item costs two
// rows in this layout, and suppressing the header's own blank moves it below
// the header, because the item slot is a fixed two rows.
func TestStartSectionSpacingWhenNarrow(t *testing.T) {
	common := testCommon()
	common.width = 45
	common.height = 40
	m := newStart(common, threeSections(), "", "")

	plain := stripANSIForLandingTest(m.View())
	lines := strings.Split(plain, "\n")

	first := startHeaderLineIndex(t, lines, "BOOKMARKS")
	if first < 3 {
		t.Fatalf("first header at line %d, want room for two blank rows above it:\n%s", first, plain)
	}
	if got := strings.TrimSpace(lines[first-1]); got != "" {
		t.Errorf("line above the first header = %q, want blank:\n%s", got, plain)
	}
	if got := strings.TrimSpace(lines[first-2]); got != "" {
		t.Errorf("second line above the first header = %q, want blank:\n%s", got, plain)
	}
	if got := strings.TrimSpace(lines[first-3]); got == "" {
		t.Errorf("three blank rows above the first header, want exactly two:\n%s", plain)
	}

	for _, title := range []string{"COMMUNITIES", "SERVICES"} {
		header := startHeaderLineIndex(t, lines, title)
		if header < 2 {
			t.Fatalf("%s header at line %d, want room for a gap above it:\n%s", title, header, plain)
		}
		if got := strings.TrimSpace(lines[header-1]); got != "" {
			t.Errorf("line above %s = %q, want one blank row:\n%s", title, got, plain)
		}
		if got := strings.TrimSpace(lines[header-2]); got == "" {
			t.Errorf("two blank rows above %s, want exactly one:\n%s", title, plain)
		}
	}
}
```

- [ ] **Step 2: Write the single-section test**

`catalog off` leaves only BOOKMARKS. No boundary exists, so no spacer should be assembled at either width.

```go
func TestStartSingleSectionAssemblesNoSpacer(t *testing.T) {
	sections := []startSection{{
		id: sectionBookmarks, title: "BOOKMARKS",
		entries: []startEntry{{target: "@tilde.team", source: sourceBookmark}},
	}}
	for _, width := range []int{45, 80, 100} {
		items := startItems(sections, width)
		for i, item := range items {
			row, ok := item.(startItem)
			if ok && row.spacer {
				t.Errorf("width %d: item %d is a spacer, want none in a single-section page", width, i)
			}
		}
	}
}
```

- [ ] **Step 3: Run both tests**

Run: `go test ./tui/ -run 'TestStartSectionSpacingWhenNarrow|TestStartSingleSectionAssemblesNoSpacer' -count=1 -v`

Expected: PASS, both.

- [ ] **Step 4: Commit**

```bash
git add tui/start_test.go
git commit -m "test(startpage): pin narrow section spacing and the single-section case"
```

---

### Task 3: Prove the resize path survives N spacers

**Files:**
- Test: `tui/start_test.go`

**Interfaces:**
- Consumes: `startModel.setSize`, `selectTarget`, `selected` — all existing.
- Produces: nothing consumed later.

`setSize` rebuilds the item slice when an unfiltered page crosses 72 columns, restoring the selection by section-relative ordinal. It now adds and removes several spacers rather than one. The riskiest selection is the first row of a section, which has an item inserted directly above it.

- [ ] **Step 1: Write the test**

```go
// TestStartUniformSpacingResizeKeepsFirstRowOfSection crosses the 72-column
// boundary with the selection on a section's first row — the position most
// exposed to a spacer being inserted directly above it. Three sections mean
// two spacers appear and disappear together.
func TestStartUniformSpacingResizeKeepsFirstRowOfSection(t *testing.T) {
	const target = "quake@bbs.airandwave.net" // the first SERVICES row

	common := testCommon()
	common.width = 100
	common.height = 40
	m := newStart(common, threeSections(), "", "")
	if !m.selectTarget(target) {
		t.Fatalf("could not select %s", target)
	}

	m.setSize(71, common.bodyHeight())
	if got, ok := m.selected(); !ok || got.target != target {
		t.Fatalf("after narrowing, selected = %+v, %v; want %s", got, ok, target)
	}

	m.setSize(100, common.bodyHeight())
	if got, ok := m.selected(); !ok || got.target != target {
		t.Fatalf("after widening, selected = %+v, %v; want %s", got, ok, target)
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./tui/ -run TestStartUniformSpacingResizeKeepsFirstRowOfSection -count=1 -v`

Expected: PASS. If it fails, the ordinal-preservation logic in `captureTogglePosition`/`selectSectionPosition` does not tolerate multiple inserted rows — fix that rather than the test, and say so in the commit.

- [ ] **Step 3: Run the whole gate set**

Run: `make check`

Expected: all four gates pass.

- [ ] **Step 4: Commit**

```bash
git add tui/start_test.go
git commit -m "test(startpage): cover resize across the width breakpoint with multiple spacers"
```

---

### Task 4: Confirm it visually and update the spec record

**Files:**
- Modify: `docs/superpowers/specs/2026-08-13-startpage-uniform-section-spacing-design.md` (outcome note only)

**Interfaces:**
- Consumes: the review kit merged as #72.
- Produces: nothing.

- [ ] **Step 1: Record the chrome tapes**

Run: `make review-tui`

Expected: twelve tapes, 138 stills, every guard passing. Takes about 7 minutes.

- [ ] **Step 2: Check the frames**

Open these four and confirm one blank row above each of BOOKMARKS, COMMUNITIES and SERVICES:

- `out/tui-review/chrome-100-tall/start-input.png` — the whole page, all three boundaries visible at once
- `out/tui-review/chrome-80-dark/start-input.png`
- `out/tui-review/chrome-80-light/start-input.png`
- `out/tui-review/chrome-45-dark/start-input.png` — two rows above the first header, one above the rest

Also open `out/tui-review/chrome-100-tall/start-many-bookmarks.png`: with six pins, BOOKMARKS carries several rows, so it is the frame where an inconsistent boundary would be most obvious.

- [ ] **Step 3: Append the outcome to the spec**

Specs in this repo are point-in-time records. Add at the end of the spec file:

```markdown
## Outcome

Implemented on 2026-08-13. Measured after the change: wide is 1 / 1 / 1, narrow
is 2 / 1 / 1, matching the decision above. Net vertical cost is zero rows wide
and one row saved narrow, so the pagination pressure described in the visual
review is unchanged.
```

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/specs/2026-08-13-startpage-uniform-section-spacing-design.md
git commit -m "docs(superpowers): record the startpage spacing outcome"
```

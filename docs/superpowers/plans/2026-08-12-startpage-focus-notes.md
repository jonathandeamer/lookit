# Startpage Focus Notes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the catalog hint feature entirely, leave a service child's note column empty until its row is selected, and cap every catalog note at 48 terminal cells.

**Architecture:** This is mostly subtraction. The ` | <hint>` grammar, its parser, its two build-gate guards and the `hint` field all come out; `startRowNote` collapses to "a child shows its note only when selected or flattened"; `FilterValue` and `splitStartMatches` return to their pre-hint shapes. What stays is the connector work: `├`/`└`, `lastChild`, and the column-5 indent. A new width gate replaces the deleted guards.

**Tech Stack:** Go 1.26 toolchain, Bubble Tea v2 (`charm.land/bubbletea/v2`), `charm.land/bubbles/v2/list`, `charm.land/lipgloss/v2`, `github.com/charmbracelet/x/ansi`. Tests are standard `go test` — no network, no TTY.

**Spec:** `docs/superpowers/specs/2026-08-12-startpage-focus-notes-design.md`

## Global Constraints

- **Commit messages: Conventional Commits.** No `Co-Authored-By`, no "Generated with Claude Code", nothing about AI — in commits or anywhere outward-facing. A `commit-msg` hook enforces `type(optional-scope): description`.
- **Do not push or open a PR** unless the user asks.
- **`make check` is the gate**: `go vet ./...`, `gofmt -l .` empty, `golangci-lint run ./...`, `go test ./... -race`. It must pass before every commit.
- **`tui/` uses the v2 import paths** (`charm.land/...`), never `github.com/charmbracelet/...`, except `github.com/charmbracelet/x/ansi`, already imported there. Do not touch `render/` (deliberately lipgloss v1).
- **Do not modify `finger/`.** This work is display-only.
- **Catalog copy must be traceable** to the server's own words or a conclusion its response plainly supports, and must contain no `#`.
- **Every catalog note must measure 48 cells or fewer** under `ansi.StringWidth` — cells, not runes, not bytes.
- **The word "hint" has two meanings in this codebase.** The catalog hint is what this plan removes. The status-bar key hints (`bar.hints`, `startBookmarkAction`, `TestJoinHintsDropsEscBackWhenBreadcrumbPresent`, `TestParentBookmarkHintAndActionAgree`) are unrelated and must be left alone.
- **macOS first.** No Alt/Option chords, no mouse capture.

---

### Task 1: Cap catalog notes at 48 cells

Do this first: it is the only task that adds a guarantee rather than removing
code, and its rewrites are what let the gate pass.

**Files:**
- Modify: `tui/catalog.txt` (three notes rewritten, one entry removed)
- Test: `tui/catalog_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `func catalogNoteWidths(data []byte) map[string]int` — target to note width in cells, used by the gate test.

- [ ] **Step 1: Write the failing tests**

Add to `tui/catalog_test.go`:

```go
// startNoteMaxCells is the widest a catalog note may render. The note column is
// half the startpage width less the frame, so this is what fits at 100 columns
// — the width the spec guarantees. Below that, notes truncate as they always
// have.
const startNoteMaxCells = 48

// catalogNoteWidths measures every note in display cells, keyed by target.
// Cells, not runes: rendering truncates by terminal width (ansi.Truncate), and
// a CJK character or emoji occupies two cells while counting as one rune.
func catalogNoteWidths(data []byte) map[string]int {
	widths := make(map[string]int)
	for _, raw := range strings.Split(string(data), "\n") {
		record := strings.TrimSpace(raw)
		if record == "" || strings.HasPrefix(record, "#") {
			continue
		}
		fields := strings.SplitN(strings.Join(strings.Fields(record), " "), " ", 3)
		if len(fields) < 3 {
			continue
		}
		widths[fields[1]] = ansi.StringWidth(fields[2])
	}
	return widths
}

// A 48-rune note can be wider than 48 cells, and it is cells that truncate.
// This fixture fails the gate only if the measure is display width.
func TestCatalogNoteWidthMeasuresCellsNotRunes(t *testing.T) {
	wide := strings.Repeat("的", 30) // 30 runes, 60 cells
	data := []byte("service wide@example.com " + wide + "\n")
	got := catalogNoteWidths(data)["wide@example.com"]
	if got != 60 {
		t.Fatalf("width = %d, want 60 cells for %d runes", got, len([]rune(wide)))
	}
	if got <= startNoteMaxCells {
		t.Fatalf("fixture width %d does not exceed the cap; it cannot prove the gate bites", got)
	}
}

func TestCatalogNotesFitTheNoteColumn(t *testing.T) {
	for target, width := range catalogNoteWidths(catalogData) {
		if width > startNoteMaxCells {
			t.Errorf("%s note is %d cells, want at most %d", target, width, startNoteMaxCells)
		}
	}
}
```

Add `"github.com/charmbracelet/x/ansi"` to the file's imports.

- [ ] **Step 2: Run them and read the failures**

Run: `go test ./tui/ -run 'TestCatalogNote' -count=1`
Expected: `TestCatalogNoteWidthMeasuresCellsNotRunes` PASSES.
`TestCatalogNotesFitTheNoteColumn` FAILS naming **ten** entries, in two groups:

- The four genuinely over-long notes: `ring@thebackupbox.net` (55),
  `@graph.no` (54), `weather@bbs.airandwave.net` (53), `@tilde.team` (51).
- Six of the seven lines still carrying a ` | <hint>` suffix, because the
  measure takes everything after the target as the note and the hint text is
  still there: `smog@` (60), `bot@` (69), `1@` (68), `textfile@` (53),
  `originsfinger@` (53), `random@` (52). (`cyoa@` lands at 47 and slips under.)

The second group is not a copy problem — it is the hint grammar, which step 3
strips as part of this task. The hints must go now rather than in Task 4: leave
them and this gate stays red for a reason that has nothing to do with note
length.

- [ ] **Step 3: Strip the seven hints, rewrite three notes, remove one entry**

First delete the ` | <hint>` suffix from all seven lines that carry one —
`cyoa@typed-hole.org`, `smog@typed-hole.org`, `textfile@typed-hole.org`,
`1@happynetbox.com`, `bot@happynetbox.com`, `random@happynetbox.com`,
`originsfinger@happynetbox.com` — leaving each note exactly as it reads before
the ` | `. Nothing else on those lines changes.

Then make exactly these four changes. Do not touch any other line, and do not
reorder anything:

```
community ring@thebackupbox.net A webring, for finger
community @tilde.team Small public access unix, for learning
service @graph.no Weather worldwide by place name
```

and **delete** the line:

```
service weather@bbs.airandwave.net Current weather and a 7-day forecast — weather:city@…
```

`weather@bbs.airandwave.net` goes rather than shrinks because `@graph.no`
already serves weather worldwide; `@bbs.airandwave.net`'s own response still
advertises `weather` in its menu.

- [ ] **Step 4: Run the catalog tests**

Run: `go test ./tui/ -run TestCatalog -count=1 -v`
Expected: PASS. Note `TestCatalogIsWellFormed` requires at least 20 entries; the
catalog now holds 25, so it still passes.

- [ ] **Step 5: Fix the count and ordering fixtures**

Run: `go test ./tui/ -count=1`

Two fixtures pin numbers the removal moves:

- `TestServicesGroupUnderHostRoots` (`tui/sections_test.go`) lists every service
  row in order — remove `"weather@bbs.airandwave.net"` from that slice.
- `TestOverviewAndStatusCountsExcludeStructuralCopies` (`tui/app_test.go`) — its
  four unfiltered scenarios each drop one service. They become, in order:

  | Scenario | Counts | Total |
  |---|---|---|
  | unfiltered | `{communities: 6, services: 19}` | 25 |
  | child pinned | `{bookmarks: 1, communities: 6, services: 18}` | 25 |
  | parent pinned | `{bookmarks: 1, communities: 6, services: 18}` | 25 |
  | repeated bookmarks | `{bookmarks: 2, communities: 5, services: 19}` | 26 |

  The two `happynetbox.com` filtered scenarios are unaffected.

If any other test fails, read what it protects before editing it.

- [ ] **Step 6: Run the full gate**

Run: `make check`
Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add tui/catalog.txt tui/catalog_test.go tui/sections_test.go tui/app_test.go
git commit -m "feat(catalog): cap notes at the width of their column"
```

---

### Task 2: Empty the note column until a row is selected

**Files:**
- Modify: `tui/start.go` (`startRowNote`, around line 320)
- Test: `tui/start_test.go`

**Interfaces:**
- Consumes: `startRowNote(entry startEntry, selected, flattened bool) string`, unchanged signature.
- Produces: the same function, with the hint branch gone.

- [ ] **Step 1: Rewrite the state test**

In `tui/start_test.go`, replace the body of `TestStartRowNotePerState` (it
currently has hinted cases) with:

```go
func TestStartRowNotePerState(t *testing.T) {
	const note = "Saturday Morning Gemzine — back issues"
	child := startEntry{target: "smog@typed-hole.org", note: note, child: true}
	root := startEntry{target: "@typed-hole.org", note: "A small menu of fingers, from lobste.rs to smog"}

	tests := []struct {
		name      string
		entry     startEntry
		selected  bool
		flattened bool
		want      string
	}{
		{name: "unselected child shows nothing", entry: child, want: ""},
		{name: "selected child shows its note", entry: child, selected: true, want: note},
		{name: "flattened child shows its note", entry: child, flattened: true, want: note},
		{name: "root always shows its note", entry: root, want: root.note},
		{name: "selected root is unchanged", entry: root, selected: true, want: root.note},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := startRowNote(tt.entry, tt.selected, tt.flattened); got != tt.want {
				t.Fatalf("startRowNote = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `go test ./tui/ -run TestStartRowNotePerState -count=1`
Expected: FAIL on "unselected child shows nothing" — the current
implementation returns `entry.hint`, which is `""` for this fixture, so if it
passes, check you replaced the whole function body rather than adding cases.

- [ ] **Step 3: Simplify the implementation**

In `tui/start.go`, replace `startRowNote`:

```go
// startRowNote is the note column's text. A child is silent by default, so the
// section reads as a list of tokens rather than a wall of prose; its note
// returns on the row the cursor is on, and wherever the child renders as a
// listing rather than a member of a visible group.
func startRowNote(entry startEntry, selected, flattened bool) string {
	if !entry.child || selected || flattened {
		return entry.note
	}
	return ""
}
```

- [ ] **Step 4: Run the test**

Run: `go test ./tui/ -run TestStartRowNotePerState -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Run the full gate**

Run: `make check`
Expected: pass — `tui/start.go` still compiles because `entry.hint` is only
read here and in `FilterValue`, which Task 3 handles.

- [ ] **Step 6: Commit**

```bash
git add tui/start.go tui/start_test.go
git commit -m "feat(startpage): show a child's note only when its row is selected"
```

---

### Task 3: Remove the hint from search

**Files:**
- Modify: `tui/start.go` (`FilterValue` around line 50, `splitStartMatches` around line 335, its caller in `renderEntry`)
- Test: `tui/start_test.go`, `tui/app_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `func splitStartMatches(matches []int, target string) (targetMatches, noteMatches []int)` — back to two parameters.

- [ ] **Step 1: Rewrite the two hint tests**

In `tui/start_test.go`, replace `TestFilterValueIncludesHintOnlyOnChildRows`
entirely with:

```go
func TestFilterValueIsTargetAndNote(t *testing.T) {
	entry := startEntry{target: "cyoa@typed-hole.org", note: "Choose your own adventure", child: true}
	item := startItem{entry: entry}
	if got, want := item.FilterValue(), "cyoa@typed-hole.org Choose your own adventure"; got != want {
		t.Errorf("child FilterValue = %q, want %q", got, want)
	}
	structural := startItem{entry: startEntry{target: "@happynetbox.com", structural: true}}
	if got := structural.FilterValue(); got != "" {
		t.Errorf("structural FilterValue = %q, want empty", got)
	}
}
```

and replace `TestSplitStartMatchesDropsHintOffsets` entirely with:

```go
func TestSplitStartMatchesMapsTargetAndNote(t *testing.T) {
	target := "cyoa@typed-hole.org"
	targetMatches, noteMatches := splitStartMatches([]int{0, len(target) + 1}, target)
	if len(targetMatches) != 1 || targetMatches[0] != 0 {
		t.Errorf("targetMatches = %v, want [0]", targetMatches)
	}
	if len(noteMatches) != 1 || noteMatches[0] != 0 {
		t.Errorf("noteMatches = %v, want [0]", noteMatches)
	}
}
```

In `tui/app_test.go`, delete `TestHintWordFindsItsChildButNotItsBookmark`
(around line 3685) — it tests a feature that no longer exists.

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./tui/ -run 'TestFilterValueIsTargetAndNote|TestSplitStartMatchesMapsTargetAndNote' -count=1`
Expected: FAIL — `splitStartMatches` still wants three arguments.

- [ ] **Step 3: Drop the hint from `FilterValue`**

In `tui/start.go`, replace `FilterValue`:

```go
// FilterValue drives "/". Non-entry rows return "" so they drop out while
// filtering, which flattens the view to matches — the behaviour we want.
// Structural rows return "" for a second reason: filtering removes the section
// headers that distinguish a parent copy from the listing it duplicates, so
// without this a filter would show two identical selectable rows.
func (i startItem) FilterValue() string {
	if !i.selectable() || i.entry.structural {
		return ""
	}
	return i.entry.target + " " + i.entry.note
}
```

- [ ] **Step 4: Narrow `splitStartMatches` back**

In `tui/start.go`, replace it:

```go
// splitStartMatches maps filter-match offsets in FilterValue onto the target
// and note fields.
func splitStartMatches(matches []int, target string) (targetMatches, noteMatches []int) {
	noteOffset := len(target) + 1
	for _, match := range matches {
		if match < noteOffset-1 {
			targetMatches = append(targetMatches, match)
		} else if match >= noteOffset {
			noteMatches = append(noteMatches, match-noteOffset)
		}
	}
	return targetMatches, noteMatches
}
```

and update its one caller in `renderEntry`:

```go
		targetMatches, noteMatches = splitStartMatches(m.MatchesForItem(index), item.entry.target)
```

- [ ] **Step 5: Run the tests**

Run: `go test ./tui/ -run 'TestFilterValue|TestSplitStartMatches|TestFilteredChildRendersFullTargetWithMatchHighlight' -count=1 -v`
Expected: PASS, including the pre-existing highlight test — if that one fails,
the offsets are wrong, not the test.

- [ ] **Step 6: Run the full gate**

Run: `make check`
Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add tui/start.go tui/start_test.go tui/app_test.go
git commit -m "feat(startpage): drop hint text from filter matching"
```

---

### Task 4: Delete the hint grammar

Nothing reads `entry.hint` after Task 3, so the field, its parser and its two
guards come out together.

**Files:**
- Modify: `tui/bookmarks.go` (`startEntry` line 55, `parseCatalogLine` and `splitCatalogNote` lines 185-217)
- Modify: `tui/catalog_test.go` (delete `hintIsMisplaced`, `TestCatalogHintsOnlyOnServiceChildren`, `TestCatalogHintValidationRejectsNonChildren`, `TestCatalogRawNoteValidationRejectsStrayPipe`, `catalogNotePipeLines`, and the pipe check inside `TestCatalogIsWellFormed`)
- Modify: `tui/bookmarks_test.go` (delete `TestParseCatalogLineSplitsHintFromNote`, `TestParseCatalogLineRejectsEmptyHintHalves`)
- Modify: `tui/start_test.go` (two fixtures still set `hint:`)

**Interfaces:**
- Consumes: nothing.
- Produces: `startEntry` without a `hint` field; `parseCatalogLine` returning the whole third field as the note.

- [ ] **Step 1: Write the failing test**

Add to `tui/bookmarks_test.go`:

```go
// The catalog grammar is <kind> <target> <note>. Everything after the target is
// the note, including a "|", which is ordinary text now the hint field is gone.
func TestParseCatalogLineKeepsTheWholeNote(t *testing.T) {
	entry, err := parseCatalogLine("service smog@typed-hole.org Saturday Morning Gemzine | back issues")
	if err != nil {
		t.Fatalf("parseCatalogLine = %v", err)
	}
	if got, want := entry.note, "Saturday Morning Gemzine | back issues"; got != want {
		t.Fatalf("note = %q, want %q", got, want)
	}
}
```

Add to `tui/catalog_test.go` — a removal check, so a hint line surviving the
edit is caught rather than silently becoming part of a note:

```go
func TestCatalogCarriesNoHints(t *testing.T) {
	if strings.Contains(string(catalogData), "|") {
		t.Error("catalog contains \"|\"; the hint grammar was removed, so a pipe is now just note text and is probably a leftover")
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./tui/ -run 'TestParseCatalogLineKeepsTheWholeNote|TestCatalogCarriesNoHints' -count=1`
Expected: `TestParseCatalogLineKeepsTheWholeNote` FAILS — `splitCatalogNote`
still cuts the note at the `|`, so it returns only "Saturday Morning Gemzine".
`TestCatalogCarriesNoHints` PASSES, because Task 1 already rewrote the catalog
and Task 3 removed the last hint reader; it is a guard against a leftover, not
a red test.

- [ ] **Step 3: Remove the field and the splitter**

In `tui/bookmarks.go`, delete the `hint` line from `startEntry`, delete
`splitCatalogNote` entirely, and restore `parseCatalogLine`'s final return:

```go
	return startEntry{target: target, kind: kind, note: fields[2], source: sourceCatalog}, nil
}
```

- [ ] **Step 4: Delete the hint guards**

In `tui/catalog_test.go`, delete `hintIsMisplaced`,
`TestCatalogHintsOnlyOnServiceChildren`, `TestCatalogHintValidationRejectsNonChildren`,
`TestCatalogRawNoteValidationRejectsStrayPipe` and `catalogNotePipeLines`, and
remove this block from `TestCatalogIsWellFormed`:

```go
	if lines := catalogNotePipeLines(catalogData); len(lines) != 0 {
		...
	}
```

Leave `catalogNoteCommentLines` and its `#` guard alone — that rule stands.

In `tui/bookmarks_test.go`, delete `TestParseCatalogLineSplitsHintFromNote` and
`TestParseCatalogLineRejectsEmptyHintHalves`.

- [ ] **Step 5: Clean the two remaining fixtures**

In `tui/start_test.go`, remove the `hint:` field from the fixture entries at the
narrow-layout test (around line 970) and at the `hinted` variable (around line
1178). Where a test's meaning depended on a hint being displayed, the note is
what shows now — check each assertion still describes what the row renders.

- [ ] **Step 6: Run the full gate**

Run: `make check`
Expected: pass. Compilation is the real test here: `entry.hint` no longer
exists, so any surviving reference fails the build.

- [ ] **Step 7: Commit**

```bash
git add tui/bookmarks.go tui/bookmarks_test.go tui/catalog_test.go tui/start_test.go
git commit -m "refactor(catalog): remove the hint grammar"
```

---

### Task 5: Keep the wide layout's mutation coverage

`TestWideChildRowShowsHintNoteOrFullNote` exists because a reviewer proved the
wide layout — the macOS default — could have `rowNote` reverted with the whole
suite still green. Its hinted case dies with the feature; the coverage must not.

**Files:**
- Modify: `tui/start_test.go` (`TestWideChildRowShowsHintNoteOrFullNote`, around line 1010)

**Interfaces:**
- Consumes: `startRowNote` from Task 2.
- Produces: nothing.

- [ ] **Step 1: Rewrite the test**

In `tui/start_test.go`, replace `TestWideChildRowShowsHintNoteOrFullNote` with:

```go
// The wide single-line layout is the macOS default and was once revertible with
// the suite still green, so it is asserted on rendered output rather than on
// startRowNote alone.
func TestWideChildRowShowsItsNoteOnlyWhenSelected(t *testing.T) {
	const note = "Latest earthquakes, M2.5+ past day"
	common := testCommon()
	common.width = 100
	st := common.ensureStyles()
	d := newStartDelegate(common, st)
	if d.Height() != 1 {
		t.Fatalf("delegate height = %d at width 100, want the one-line layout", d.Height())
	}
	items := []list.Item{
		startItem{entry: startEntry{target: "quake@bbs.airandwave.net", note: note, child: true}, section: sectionServices},
		startItem{entry: startEntry{target: "urban@bbs.airandwave.net", note: note, child: true, lastChild: true}, section: sectionServices},
	}
	l := list.New(items, d, 100, 6)

	l.Select(1) // row 0 unselected
	var unselected strings.Builder
	d.Render(&unselected, l, 0, items[0])
	if got := ansi.Strip(unselected.String()); strings.Contains(got, note) {
		t.Errorf("unselected child row = %q, want no note", got)
	}
	if got := ansi.Strip(unselected.String()); !strings.Contains(got, "quake") {
		t.Errorf("unselected child row = %q, want its token", got)
	}

	l.Select(0) // row 0 selected
	var selected strings.Builder
	d.Render(&selected, l, 0, items[0])
	if got := ansi.Strip(selected.String()); !strings.Contains(got, note) {
		t.Errorf("selected child row = %q, want its full note", got)
	}
}

// Selection is the cursor's row, not which pane takes keys. A selected child
// keeps its note while the target input is focused and the inactive shelf is
// drawn — otherwise the note would blink out every time focus moved.
func TestSelectedChildKeepsItsNoteWithoutContentFocus(t *testing.T) {
	const note = "Latest earthquakes, M2.5+ past day"
	common := testCommon()
	common.width = 100
	common.contentFocused = false
	st := common.ensureStyles()
	d := newStartDelegate(common, st)
	items := []list.Item{
		startItem{entry: startEntry{target: "quake@bbs.airandwave.net", note: note, child: true, lastChild: true}, section: sectionServices},
	}
	l := list.New(items, d, 100, 4)
	l.Select(0)

	var buf strings.Builder
	d.Render(&buf, l, 0, items[0])
	if got := ansi.Strip(buf.String()); !strings.Contains(got, note) {
		t.Fatalf("selected child without content focus = %q, want its full note", got)
	}
}
```

- [ ] **Step 2: Run them**

Run: `go test ./tui/ -run 'TestWideChildRowShowsItsNoteOnlyWhenSelected|TestSelectedChildKeepsItsNoteWithoutContentFocus' -count=1 -v`
Expected: PASS.

- [ ] **Step 3: Prove the wide-layout test still bites**

Temporarily change the wide branch of `renderEntry` in `tui/start.go` to pass
`item.entry.note` where it passes `rowNote`, then run:

Run: `go test ./tui/ -run TestWideChildRowShowsItsNoteOnlyWhenSelected -count=1`
Expected: FAIL on the unselected case. Restore `rowNote`, re-run, confirm PASS.
Put both outputs in your report — this is the evidence the task is judged on.

- [ ] **Step 4: Run the full gate**

Run: `make check`
Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add tui/start_test.go
git commit -m "test(startpage): keep wide-layout note coverage without hints"
```

---

### Task 6: Update the documents

**Files:**
- Modify: `CLAUDE.md` (the `appModel`/`stateStart` bullet)
- Modify: `tui/catalog.txt` (header comment)
- Modify: `docs/superpowers/specs/2026-08-12-startpage-child-hints-design.md` (supersession note)

**Interfaces:**
- Consumes: everything above.
- Produces: nothing.

- [ ] **Step 1: Correct the catalog header**

In `tui/catalog.txt`, the header comment currently documents the hint field.
Replace that block with:

```
# Format: <kind> <target> <note>
#
# A note must fit the note column: 48 terminal cells or fewer, which is what
# the startpage shows at 100 columns. TestCatalogNotesFitTheNoteColumn enforces
# it. A service child's note is hidden until its row is selected; every other
# row shows its note always.
```

- [ ] **Step 2: Correct the architecture note**

In `CLAUDE.md`, the `appModel`/`stateStart` bullet describes the hint. Replace
the hint sentence with:

```markdown
A service child renders as a bare token under a `├`/`└` connector with an empty
note column; its note appears when its row is selected, and whenever it renders
as a listing rather than a group member — flattened by a non-empty filter, or
pinned into BOOKMARKS. Catalog notes are capped at 48 terminal cells so a
selected note does not truncate at 100 columns.
```

- [ ] **Step 3: Mark the hints spec superseded**

At the top of `docs/superpowers/specs/2026-08-12-startpage-child-hints-design.md`,
directly under the title, add:

```markdown
> **Superseded** by
> [startpage focus notes](./2026-08-12-startpage-focus-notes-design.md), which
> removed the hint grammar this spec introduces. The connectors, `lastChild` and
> the indent it describes all shipped and remain; the hint field, its guards and
> its search behaviour did not survive. Read this for why children became tokens,
> and the newer spec for what a child's note column does today.
```

Leave the rest of that spec as written — it is a point-in-time record.

- [ ] **Step 4: Run the full gate**

Run: `make check`
Expected: pass.

- [ ] **Step 5: Verify in a real terminal**

Run: `make build && ./lookit`

Confirm by eye: a child row shows only its token; moving the cursor onto it
reveals its note with no row movement; the note is still there after pressing
`i` to focus the input; `/` alone changes nothing; typing a query expands
children to addresses with their notes. If you cannot run an interactive
terminal, say so in your report and leave this for the human.

- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md tui/catalog.txt docs/superpowers/specs/2026-08-12-startpage-child-hints-design.md
git commit -m "docs: describe focus-revealed child notes"
```

---

## Notes for the implementer

- **Two different "hints" live in this codebase.** The catalog hint is what you
  are removing. The status-bar key hints — `bar.hints`, `startBookmarkAction`,
  `TestJoinHintsDropsEscBackWhenBreadcrumbPresent`,
  `TestParentBookmarkHintAndActionAgree` — are unrelated. Read before deleting
  anything matching `hint`.
- **Task order matters.** Task 4 deletes the `hint` field, and the compiler
  finds every reader — but only if Tasks 2 and 3 have already removed the two
  legitimate ones. Doing 4 first buries you in errors.
- **Do not reorder `tui/catalog.txt`.** Display order is computed in
  `buildSections`; the file's host grouping is for human editors.
- **Three tests in this project's history were caught passing vacuously.** When
  you assert on rendered output, make the assertion something the delegate's
  base styling cannot satisfy on its own.

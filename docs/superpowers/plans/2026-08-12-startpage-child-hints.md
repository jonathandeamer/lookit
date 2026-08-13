# Startpage Child Hints Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a startpage service child a token by default — drawn with a dim connector, carrying an optional short hint, and showing its full note only when the cursor is on it.

**Architecture:** The catalog grammar gains one optional field (` | ` + hint) parsed in `tui/bookmarks.go` into a new `startEntry.hint`. `groupByHost` in `tui/sections.go` marks the final child of each group with a new `lastChild` flag. Everything else is `tui/start.go`: the delegate builds the child's prefix from `child`/`lastChild`, and picks the note column's contents from hint / full note / nothing according to selection and flattening.

**Tech Stack:** Go 1.26 toolchain, Bubble Tea v2 (`charm.land/bubbletea/v2`), `charm.land/bubbles/v2/list`, `charm.land/lipgloss/v2`, `github.com/charmbracelet/x/ansi`. Tests are standard `go test` — no network, no TTY.

**Spec:** `docs/superpowers/specs/2026-08-12-startpage-child-hints-design.md`

## Global Constraints

- **Commit messages: Conventional Commits.** No `Co-Authored-By`, no "Generated with Claude Code", nothing about AI — in commits, PR bodies, or anywhere outward-facing. A `commit-msg` hook enforces `type(optional-scope): description`.
- **Do not push or open a PR** unless the user asks. Committing per task is expected; shipping is not.
- **`make check` is the gate**: `go vet ./...`, `gofmt -l .` empty, `golangci-lint run ./...`, `go test ./... -race`. It must pass before every commit.
- **`tui/` uses the v2 import paths** (`charm.land/...`), never `github.com/charmbracelet/...`, except `github.com/charmbracelet/x/ansi` which is already imported there. Do not touch `render/` (deliberately lipgloss v1).
- **Do not modify `finger/`.** This work is display-only.
- **Catalog copy must be traceable** to the server's own words or a conclusion its response plainly supports, and must contain no `#` (the comment stripper eats it) and now no `|` outside the hint delimiter.
- **Hint copy style:** lowercase, two or three words, written for the slot — never the first clause of the note.
- **Pair every colour with a light/dark value.** Use the existing `styles.palette` entries; add no hardcoded hex.
- **macOS first.** No Alt/Option chords, no mouse capture.

---

### Task 1: Parse the optional hint

**Files:**
- Modify: `tui/bookmarks.go:51-60` (`startEntry`), `tui/bookmarks.go:166-184` (`parseCatalogLine`)
- Test: `tui/bookmarks_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `startEntry.hint string` — the short label a child row shows instead of its note; empty when the entry has none.

- [x] **Step 1: Write the failing tests**

Add to `tui/bookmarks_test.go`:

```go
func TestParseCatalogLineSplitsHintFromNote(t *testing.T) {
	tests := []struct {
		name string
		line string
		note string
		hint string
	}{
		{
			name: "hint present",
			line: "service smog@typed-hole.org Saturday Morning Gemzine — back issues | gemzine back issues",
			note: "Saturday Morning Gemzine — back issues",
			hint: "gemzine back issues",
		},
		{
			name: "no hint",
			line: "service quake@bbs.airandwave.net Latest earthquakes, M2.5+ past day",
			note: "Latest earthquakes, M2.5+ past day",
			hint: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parseCatalogLine(tt.line)
			if err != nil {
				t.Fatalf("parseCatalogLine(%q) = %v", tt.line, err)
			}
			if entry.note != tt.note {
				t.Errorf("note = %q, want %q", entry.note, tt.note)
			}
			if entry.hint != tt.hint {
				t.Errorf("hint = %q, want %q", entry.hint, tt.hint)
			}
		})
	}
}

// An empty half either side of the delimiter is a typo in a file that ships
// compiled in, so it is refused rather than rendered as a blank column.
func TestParseCatalogLineRejectsEmptyHintHalves(t *testing.T) {
	for _, line := range []string{
		"service smog@typed-hole.org Saturday Morning Gemzine |",
		"service smog@typed-hole.org  | gemzine back issues",
	} {
		if _, err := parseCatalogLine(line); err == nil {
			t.Errorf("parseCatalogLine(%q) = nil error, want a refusal", line)
		}
	}
}
```

- [x] **Step 2: Run them to make sure they fail**

Run: `go test ./tui/ -run 'TestParseCatalogLineSplitsHint|TestParseCatalogLineRejectsEmptyHint' -count=1`
Expected: FAIL — `entry.hint undefined`.

- [x] **Step 3: Add the field**

In `tui/bookmarks.go`, add to `startEntry` after `source` (keep the existing comment block above the struct):

```go
	hint string // short label a child row shows in place of its note; "" when absent
```

- [x] **Step 4: Split the note in the parser**

In `tui/bookmarks.go`, replace the final `return` of `parseCatalogLine`:

```go
	note, hint, err := splitCatalogNote(fields[2])
	if err != nil {
		return startEntry{}, err
	}
	return startEntry{target: target, kind: kind, note: note, hint: hint, source: sourceCatalog}, nil
}

// splitCatalogNote separates a note from its optional short hint at " | ". The
// hint is what a child row displays; the note stays authoritative and feeds
// filtering, the selected row, and every context where the child renders as a
// listing. An empty half is a typo in a file that ships compiled in, so it is
// refused rather than rendered as a blank column.
func splitCatalogNote(field string) (note, hint string, err error) {
	before, after, found := strings.Cut(field, "|")
	if !found {
		return field, "", nil
	}
	note, hint = strings.TrimSpace(before), strings.TrimSpace(after)
	if note == "" || hint == "" {
		return "", "", fmt.Errorf("note and hint must both be non-empty, got %q", field)
	}
	return note, hint, nil
}
```

- [x] **Step 5: Run the tests**

Run: `go test ./tui/ -run 'TestParseCatalogLine' -count=1 -v`
Expected: PASS, including the pre-existing `parseCatalogLine` tests.

- [x] **Step 6: Run the full gate**

Run: `make check`
Expected: pass.

- [x] **Step 7: Commit**

```bash
git add tui/bookmarks.go tui/bookmarks_test.go
git commit -m "feat(catalog): parse an optional short hint after a note"
```

---

### Task 2: Guard the hint at the build gate

A hint only renders on a service child. Anywhere else it is dead text, and the
catalog ships compiled in, so a wasted hint must fail the build rather than
reach a user who cannot fix it.

**Files:**
- Modify: `tui/catalog_test.go`
- Test: same file

**Interfaces:**
- Consumes: `startEntry.hint` from Task 1; `entryHost`/`entryToken` (already in `tui/sections.go`).
- Produces: nothing later tasks call.

- [x] **Step 1: Write the failing tests**

Add to `tui/catalog_test.go`:

```go
// A hint renders only where a token renders: on a service child. "Not a root"
// is too loose — a queried community such as ring@thebackupbox.net is non-root
// but is never grouped, so a hint on it would never appear.
func TestCatalogHintsOnlyOnServiceChildren(t *testing.T) {
	entries, _ := parseCatalogData(catalogData)
	for _, e := range entries {
		if e.hint == "" {
			continue
		}
		if e.kind != kindService || entryToken(e.target) == "" {
			t.Errorf("%s carries hint %q but never renders as a token", e.target, e.hint)
		}
	}
}

func TestCatalogHintValidationRejectsNonChildren(t *testing.T) {
	data := []byte(strings.Join([]string{
		"community ring@thebackupbox.net The finger ring | the ring",
		"service @graph.no Weather worldwide | weather",
	}, "\n"))
	entries, problems := parseCatalogData(data)
	if len(problems) != 0 {
		t.Fatalf("parse problems = %+v, want none; the grammar is valid, the placement is not", problems)
	}
	for _, e := range entries {
		if e.hint != "" && (e.kind != kindService || entryToken(e.target) == "") {
			return // the condition the catalog test asserts against catalogData
		}
	}
	t.Fatal("fixture did not produce a misplaced hint; the guard cannot be trusted")
}

// The note is cut at the first "|", so a note containing one would lose its
// tail. Same treatment "#" already gets: forbid the character.
func TestCatalogNotesContainNoPipe(t *testing.T) {
	for i, raw := range strings.Split(string(catalogData), "\n") {
		record := strings.TrimSpace(raw)
		if record == "" || strings.HasPrefix(record, "#") {
			continue
		}
		if strings.Count(record, "|") > 1 {
			t.Errorf("line %d has more than one \"|\": %q", i+1, record)
		}
	}
}
```

- [x] **Step 2: Run them**

Run: `go test ./tui/ -run 'TestCatalogHint|TestCatalogNotesContainNoPipe' -count=1 -v`
Expected: PASS — the catalog has no hints yet, so these guards pass vacuously
today. That is expected and fine: Task 3 adds the hints they protect. Confirm
`TestCatalogHintValidationRejectsNonChildren` passes, which proves the condition
itself detects a misplaced hint.

- [x] **Step 3: Run the full gate**

Run: `make check`
Expected: pass.

- [x] **Step 4: Commit**

```bash
git add tui/catalog_test.go
git commit -m "test(catalog): guard hint placement and pipe use"
```

---

### Task 3: Write the seven hints

**Files:**
- Modify: `tui/catalog.txt`
- Test: `tui/catalog_test.go` (existing guards from Task 2 now bite)

**Interfaces:**
- Consumes: the parser from Task 1 and the guards from Task 2.
- Produces: catalog data the delegate tasks render.

- [x] **Step 1: Add the hints**

In `tui/catalog.txt`, append ` | <hint>` to exactly these seven service lines,
leaving every other line untouched. Do not reorder anything — display order is
computed:

| Target | Hint to append |
|---|---|
| `cyoa@typed-hole.org` | `pick-a-path stories` |
| `smog@typed-hole.org` | `gemzine back issues` |
| `textfile@typed-hole.org` | `random textfiles.com` |
| `1@happynetbox.com` | `interactive fiction` |
| `bot@happynetbox.com` | `tech news headlines` |
| `random@happynetbox.com` | `a random profile` |
| `originsfinger@happynetbox.com` | `how finger began` |

For example the `smog` line becomes:

```
service smog@typed-hole.org Saturday Morning Gemzine — back issues | gemzine back issues
```

Leave the other nine children — `bonsai`, `dict`, `quake`, `urban`, `weather`,
`sudoku:easy`, `wordsearch:today`, `calendar` and `browserversion` — without
hints: their tokens say what they are, and a hint on every child rebuilds the
wall this work removes.

- [x] **Step 2: Verify the guards now have something to protect**

Run: `go test ./tui/ -run TestCatalog -count=1 -v`
Expected: PASS, with all seven hints landing on service children.

- [x] **Step 3: Prove the placement guard bites**

Temporarily move one hint onto a root line — append ` | menu of fingers` to the
`service @typed-hole.org …` line — and run:

Run: `go test ./tui/ -run TestCatalogHintsOnlyOnServiceChildren -count=1`
Expected: FAIL naming `@typed-hole.org`. Remove the temporary hint and confirm
PASS. Record both outputs in your report.

- [x] **Step 4: Run the full gate**

Run: `make check`
Expected: pass.

- [x] **Step 5: Commit**

```bash
git add tui/catalog.txt
git commit -m "feat(catalog): add short hints for seven opaque service tokens"
```

---

### Task 4: Mark the last child of each group

**Files:**
- Modify: `tui/bookmarks.go` (`startEntry`), `tui/sections.go:151-163` (`groupByHost`)
- Test: `tui/sections_test.go`

**Interfaces:**
- Consumes: `groupByHost(listed []startEntry, roots map[string]startEntry, pinned map[string]bool) []startEntry`.
- Produces: `startEntry.lastChild bool` — true for the final child of every group, false on non-final children and on every non-child row.

- [x] **Step 1: Write the failing test**

Add to `tui/sections_test.go`:

```go
// The delegate renders one item at a time and cannot see whether the next row
// shares a host, so the connector's shape is decided here.
func TestLastChildMarksTheFinalChildOfEveryGroup(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{})
	want := map[string]bool{
		"wordsearch:today@bbs.airandwave.net": true,  // final child of a six-child group
		"dict@bbs.airandwave.net":             false, // first child of the same group
		"calendar@flanigan.us":                true,  // final child of a two-child group
		"bonsai@flanigan.us":                  false, // its non-final sibling
		"textfile@typed-hole.org":             true,
		"cyoa@typed-hole.org":                 false,
		"random@happynetbox.com":              true,
		"@bbs.airandwave.net":                 false, // a root is not a child
		"@graph.no":                           false, // no group at all
		"@happynetbox.com":                    false, // structural parent
	}
	seen := make(map[string]bool, len(want))
	for _, s := range sections {
		if s.id != sectionServices {
			continue
		}
		for _, e := range s.entries {
			if expected, ok := want[e.target]; ok {
				seen[e.target] = true
				if e.lastChild != expected {
					t.Errorf("%s lastChild = %v, want %v", e.target, e.lastChild, expected)
				}
			}
		}
	}
	for target := range want {
		if !seen[target] {
			t.Errorf("%s never appeared in SERVICES", target)
		}
	}
}

// No service host in the shipped catalog has exactly one child any more —
// @flanigan.us gained bonsai — but the rule must still hold for one, so this
// case is built rather than found.
func TestLastChildMarksTheOnlyChildOfASingleChildGroup(t *testing.T) {
	catalog := []startEntry{
		{target: "@example.com", kind: kindService, note: "Root", source: sourceCatalog},
		{target: "only@example.com", kind: kindService, note: "Only child", source: sourceCatalog},
	}
	sections := buildSections(catalog, bookmarkFile{})
	for _, s := range sections {
		if s.id != sectionServices {
			continue
		}
		if len(s.entries) != 2 {
			t.Fatalf("entries = %+v, want a root and one child", s.entries)
		}
		if s.entries[0].lastChild {
			t.Errorf("root = %+v, want lastChild false", s.entries[0])
		}
		if !s.entries[1].child || !s.entries[1].lastChild {
			t.Errorf("only child = %+v, want child and lastChild true", s.entries[1])
		}
		return
	}
	t.Fatal("SERVICES section not found")
}
```

- [x] **Step 2: Run them to make sure they fail**

Run: `go test ./tui/ -run TestLastChildMarks -count=1`
Expected: FAIL — `e.lastChild undefined`.

- [x] **Step 3: Add the field**

In `tui/bookmarks.go`, add to `startEntry` beneath `child`:

```go
	lastChild  bool // final child of its group; draws "└" instead of "├"
```

- [x] **Step 4: Set it while grouping**

In `tui/sections.go`, replace the child loop at the end of `groupByHost`:

```go
		for i, e := range rows {
			e.child = true
			e.lastChild = i == len(rows)-1
			out = append(out, e)
		}
```

- [x] **Step 5: Run the test**

Run: `go test ./tui/ -run TestLastChildMarks -count=1 -v`
Expected: PASS.

- [x] **Step 6: Run the full gate**

Run: `make check`
Expected: pass.

- [x] **Step 7: Commit**

```bash
git add tui/bookmarks.go tui/sections.go tui/sections_test.go
git commit -m "feat(startpage): mark the final child of each host group"
```

---

### Task 5: Draw connectors and choose the note column

This is the visible change: children get `├`/`└` at a deeper indent, and the
note column shows the hint, the full note, or nothing depending on state.

**Files:**
- Modify: `tui/start.go` (`startRowTarget` around line 296, `renderEntry` 215-290)
- Test: `tui/start_test.go`

**Interfaces:**
- Consumes: `startEntry.hint` (Task 1), `startEntry.lastChild` (Task 4), and the existing `flattened` predicate in `renderEntry`.
- Produces: `func startRowNote(entry startEntry, selected, flattened bool) string` — the note column's text for a row.

- [x] **Step 1: Write the failing tests**

Add to `tui/start_test.go`:

```go
func TestStartRowTargetDrawsConnectors(t *testing.T) {
	mid := startEntry{target: "dict@bbs.airandwave.net", child: true}
	last := startEntry{target: "wordsearch:today@bbs.airandwave.net", child: true, lastChild: true}
	root := startEntry{target: "@bbs.airandwave.net"}
	if got, want := startRowTarget(mid, false), "   ├ dict"; got != want {
		t.Errorf("mid child = %q, want %q", got, want)
	}
	if got, want := startRowTarget(last, false), "   └ wordsearch:today"; got != want {
		t.Errorf("last child = %q, want %q", got, want)
	}
	if got, want := startRowTarget(root, false), "@bbs.airandwave.net"; got != want {
		t.Errorf("root = %q, want %q", got, want)
	}
	// Flattened: no parent above it, so the address returns and the connector
	// goes with it.
	if got, want := startRowTarget(mid, true), "dict@bbs.airandwave.net"; got != want {
		t.Errorf("flattened child = %q, want %q", got, want)
	}
}

func TestStartRowNotePerState(t *testing.T) {
	hinted := startEntry{target: "smog@typed-hole.org", note: "Saturday Morning Gemzine — back issues", hint: "gemzine back issues", child: true}
	bare := startEntry{target: "quake@bbs.airandwave.net", note: "Latest earthquakes, M2.5+ past day", child: true}
	root := startEntry{target: "@typed-hole.org", note: "A small menu of fingers, from lobste.rs to smog"}

	tests := []struct {
		name      string
		entry     startEntry
		selected  bool
		flattened bool
		want      string
	}{
		{name: "hinted child shows its hint", entry: hinted, want: "gemzine back issues"},
		{name: "child without a hint shows nothing", entry: bare, want: ""},
		{name: "selected child shows its full note", entry: hinted, selected: true, want: "Saturday Morning Gemzine — back issues"},
		{name: "selected child without a hint also shows it", entry: bare, selected: true, want: "Latest earthquakes, M2.5+ past day"},
		{name: "flattened child shows its full note", entry: hinted, flattened: true, want: "Saturday Morning Gemzine — back issues"},
		{name: "root keeps its note", entry: root, want: "A small menu of fingers, from lobste.rs to smog"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := startRowNote(tt.entry, tt.selected, tt.flattened); got != tt.want {
				t.Fatalf("startRowNote = %q, want %q", got, tt.want)
			}
		})
	}
}

// A bookmarked child has no parent in BOOKMARKS, so it is a listing there:
// full target, full note, no connector.
func TestBookmarkedChildRendersAsAListing(t *testing.T) {
	entry := startEntry{
		target: "smog@typed-hole.org", note: "Saturday Morning Gemzine — back issues",
		hint: "gemzine back issues", source: sourceBookmark, bookmarked: true,
	}
	if got, want := startRowTarget(entry, false), "smog@typed-hole.org"; got != want {
		t.Errorf("target = %q, want %q", got, want)
	}
	if got, want := startRowNote(entry, false, false), "Saturday Morning Gemzine — back issues"; got != want {
		t.Errorf("note = %q, want %q", got, want)
	}
}
```

- [x] **Step 2: Run them to make sure they fail**

Run: `go test ./tui/ -run 'TestStartRowTargetDrawsConnectors|TestStartRowNotePerState|TestBookmarkedChildRendersAsAListing' -count=1`
Expected: FAIL — `undefined: startRowNote`, and the connector assertions fail
against the current two-space indent.

- [x] **Step 3: Draw the connector**

In `tui/start.go`, replace `startRowTarget`:

```go
// startRowTarget is the target column's text. A child shows only its query
// token, prefixed by a connector that gives the group its shape, because the
// parent row above it states the host — but once the view flattens, the parent
// may be off screen, so the full address returns and the connector goes with
// it. Flattened is not the same as "a filter is active": see renderEntry.
func startRowTarget(entry startEntry, flattened bool) string {
	if !entry.child || flattened {
		return entry.target
	}
	connector := "├"
	if entry.lastChild {
		connector = "└"
	}
	return "   " + connector + " " + entryToken(entry.target)
}
```

- [x] **Step 4: Choose the note**

Append to `tui/start.go`:

```go
// startRowNote is the note column's text. A child is quiet by default — its
// hint, or nothing — so the section reads as a list of tokens rather than a
// wall of prose. The full note returns exactly where it is wanted: on the row
// the cursor is on, and wherever the child renders as a listing rather than as
// a member of a visible group.
func startRowNote(entry startEntry, selected, flattened bool) string {
	if !entry.child || selected || flattened {
		return entry.note
	}
	return entry.hint
}
```

- [x] **Step 5: Use both in `renderEntry`**

In `tui/start.go`, `renderEntry` already computes `isSelected` and `flattened`.
Below the existing `rowTarget` line add:

```go
	rowNote := startRowNote(item.entry, isSelected, flattened)
```

In the **wide** single-line branch, pass `rowNote` where `item.entry.note` is
passed today:

```go
		note := renderStartField(rowNote, noteWidth, noteMatches, descStyle, d.st.listItem.FilterMatch)
```

The **narrow** two-line branch currently declares its own `rowNote` by indenting
`item.entry.note`. That declaration must go — `rowNote` is now computed once,
above, for both branches. Replace the narrow branch's block with an indent
applied to the existing variable:

```go
	if item.entry.child && !flattened && rowNote != "" {
		rowNote = "     " + rowNote
	}
```

Five spaces, not two: the second line aligns under the token at column 5, not
under the connector. Leaving the old `rowNote :=` in place is a redeclaration
and will not compile, which is the fastest way to notice you missed it.

- [x] **Step 6: Dim the connector and the hint**

Still in `renderEntry`, after the style selection block that sets
`titleStyle`/`descStyle`, the connector must read as rule-work and an unselected
hint must recede. Add:

```go
	// A hint is a quiet aid, not content: it recedes unless the row is selected,
	// where it has been replaced by the full note anyway.
	if item.entry.child && !isSelected && !emptyFilter && !showShelf {
		descStyle = descStyle.Foreground(d.st.palette.Dim)
	}
```

The connector inherits `titleStyle`; colouring it separately would mean
splitting the target field into two styled spans and re-deriving the match
offsets across them, which buys less than it costs. Leave it inheriting.

- [x] **Step 7: Run the tests**

Run: `go test ./tui/ -run 'TestStartRow|TestBookmarkedChild' -count=1 -v`
Expected: PASS.

- [x] **Step 8: Reconcile the existing rendering tests**

Run: `go test ./tui/ -count=1`

Two known fixtures move:

- `TestNarrowChildRowIndentsBothLines` asserts a child's lines carry exactly two
  more leading spaces than a non-child sibling's. The prefix is now `   ├ `, so
  the expected difference becomes 3 leading spaces on the target line (the
  connector is not a space) and 5 on the note line. **Keep it differential** —
  compare against the sibling, never a bare `HasPrefix`, or it passes vacuously
  against the delegate's own 2-space padding.
- `TestEmptyFilterKeepsChildTokensAndQueryExpandsThem` looks for `"  dict"`;
  update it to the connector form.

For any other failure, read what the test protects before editing it.

- [x] **Step 9: Run the full gate**

Run: `make check`
Expected: pass.

- [x] **Step 10: Commit**

```bash
git add tui/start.go tui/start_test.go
git commit -m "feat(startpage): draw child connectors and quiet child notes"
```

---

### Task 6: Make hints searchable without polluting other rows

**Files:**
- Modify: `tui/start.go:44-52` (`FilterValue`), `tui/start.go` (`splitStartMatches`)
- Test: `tui/start_test.go`, `tui/app_test.go`

**Interfaces:**
- Consumes: `startEntry.hint` (Task 1).
- Produces: nothing later tasks call.

- [x] **Step 1: Write the failing tests**

Add to `tui/start_test.go`:

```go
func TestFilterValueIncludesHintOnlyOnChildRows(t *testing.T) {
	entry := startEntry{target: "cyoa@typed-hole.org", note: "Choose your own adventure", hint: "pick-a-path stories"}

	child := startItem{entry: entry}
	child.entry.child = true
	if got, want := child.FilterValue(), "cyoa@typed-hole.org Choose your own adventure pick-a-path stories"; got != want {
		t.Errorf("child FilterValue = %q, want %q", got, want)
	}

	// The same catalog entry copied into BOOKMARKS is not a child and shows no
	// hint, so it must not be findable by one.
	pinned := startItem{entry: entry}
	pinned.entry.source = sourceBookmark
	pinned.entry.bookmarked = true
	if got, want := pinned.FilterValue(), "cyoa@typed-hole.org Choose your own adventure"; got != want {
		t.Errorf("bookmark FilterValue = %q, want %q", got, want)
	}
}

// Matches are offsets into FilterValue. A match inside the appended hint lies
// past the note, and must be dropped rather than highlighted at a wrong column.
func TestSplitStartMatchesDropsHintOffsets(t *testing.T) {
	target, note := "cyoa@typed-hole.org", "Choose your own adventure"
	hintOffset := len(target) + 1 + len(note) + 1
	targetMatches, noteMatches := splitStartMatches([]int{0, len(target) + 1, hintOffset, hintOffset + 3}, target, note)
	if len(targetMatches) != 1 || targetMatches[0] != 0 {
		t.Errorf("targetMatches = %v, want [0]", targetMatches)
	}
	if len(noteMatches) != 1 || noteMatches[0] != 0 {
		t.Errorf("noteMatches = %v, want [0]", noteMatches)
	}
}
```

Add to `tui/app_test.go`:

```go
func TestHintWordFindsItsChildButNotItsBookmark(t *testing.T) {
	t.Run("child is findable by its hint", func(t *testing.T) {
		useTempBookmarks(t)
		m := newApp(stubFetch(t), colorprofile.NoTTY)
		m.blurInput()
		m.start.list.SetFilterText("pick-a-path")
		var found bool
		for _, item := range m.start.list.VisibleItems() {
			if it, ok := item.(startItem); ok && it.entry.target == "cyoa@typed-hole.org" {
				found = true
			}
		}
		if !found {
			t.Fatal("cyoa@typed-hole.org not found by a word from its hint")
		}
	})

	t.Run("its bookmark copy is not", func(t *testing.T) {
		seedBookmarks(t, "cyoa@typed-hole.org\n")
		m := newApp(stubFetch(t), colorprofile.NoTTY)
		m.blurInput()
		m.start.list.SetFilterText("pick-a-path")
		for _, item := range m.start.list.VisibleItems() {
			it, ok := item.(startItem)
			if ok && it.entry.target == "cyoa@typed-hole.org" && it.section == sectionBookmarks {
				t.Fatal("BOOKMARKS row matched hint text it never displays")
			}
		}
	})
}
```

- [x] **Step 2: Run them to make sure they fail**

Run: `go test ./tui/ -run 'TestFilterValueIncludesHint|TestSplitStartMatchesDropsHint|TestHintWordFinds' -count=1`
Expected: FAIL — the hint is absent from `FilterValue`, and `splitStartMatches`
takes two arguments, not three.

- [x] **Step 3: Append the hint for child rows only**

In `tui/start.go`, replace `FilterValue`:

```go
// FilterValue drives "/". Non-entry rows return "" so they drop out while
// filtering, which flattens the view to matches — the behaviour we want.
// Structural rows return "" for a second reason: filtering removes the section
// headers that distinguish a parent copy from the listing it duplicates, so
// without this a filter would show two identical selectable rows.
//
// A child appends its hint, so a row is findable by the text it displays. The
// gate is deliberate: the same catalog entry copied into BOOKMARKS is not a
// child and shows its full note, so matching it on hint text would match on
// something the row never shows.
func (i startItem) FilterValue() string {
	if !i.selectable() || i.entry.structural {
		return ""
	}
	value := i.entry.target + " " + i.entry.note
	if i.entry.child && i.entry.hint != "" {
		value += " " + i.entry.hint
	}
	return value
}
```

- [x] **Step 4: Drop match offsets that land in the hint**

In `tui/start.go`, give `splitStartMatches` the note so it knows where the note
ends:

```go
// splitStartMatches maps filter-match offsets in FilterValue onto the target
// and note fields. Offsets past the note lie inside the appended hint, which
// the row may not be displaying; they are dropped rather than highlighted at a
// column that means something else.
func splitStartMatches(matches []int, target, note string) (targetMatches, noteMatches []int) {
	noteOffset := len(target) + 1
	noteEnd := noteOffset + len(note)
	for _, match := range matches {
		switch {
		case match < noteOffset-1:
			targetMatches = append(targetMatches, match)
		case match >= noteOffset && match < noteEnd:
			noteMatches = append(noteMatches, match-noteOffset)
		}
	}
	return targetMatches, noteMatches
}
```

Update its single caller in `renderEntry`:

```go
		targetMatches, noteMatches = splitStartMatches(m.MatchesForItem(index), item.entry.target, item.entry.note)
```

- [x] **Step 5: Run the tests**

Run: `go test ./tui/ -run 'TestFilterValue|TestSplitStartMatches|TestHintWordFinds' -count=1 -v`
Expected: PASS.

- [x] **Step 6: Run the full gate**

Run: `make check`
Expected: pass. `TestFilteredChildRendersFullTargetWithMatchHighlight` exercises
the highlight path and must stay green — if it fails, the offsets are wrong, not
the test.

- [x] **Step 7: Commit**

```bash
git add tui/start.go tui/start_test.go tui/app_test.go
git commit -m "feat(startpage): make child hints searchable"
```

---

### Task 7: Document the grammar

**Files:**
- Modify: `tui/catalog.txt` (header comment), `CLAUDE.md` (the `stateStart` bullet)

**Interfaces:**
- Consumes: everything above.
- Produces: nothing.

- [x] **Step 1: Document the grammar where it is authored**

In `tui/catalog.txt`, the header comment currently reads
`# Format: <kind> <target> <note>`. Replace that line with:

```
# Format: <kind> <target> <note>[ | <hint>]
#
# The optional hint is the short label a service child shows in its row; the
# note stays authoritative and appears when the row is selected, filtered, or
# bookmarked. Write a hint only where the token does not say what it is —
# lowercase, two or three words. A hint on anything but a service child fails
# TestCatalogHintsOnlyOnServiceChildren.
```

- [x] **Step 2: Update the architecture note**

In `CLAUDE.md`, in the `appModel`/`stateStart` bullet, after the sentence about
structural rows, add:

```markdown
A service child renders as a bare token under a dim `├`/`└` connector; its note
column carries an optional short hint from the catalog (`<note> | <hint>`), its
full note when the row is selected, and its full note again wherever it renders
as a listing rather than a group member — flattened by a non-empty filter, or
pinned into BOOKMARKS. `FilterValue` appends the hint on child rows only, so a
row is findable by what it displays and no other row is findable by a hint it
never shows.
```

- [x] **Step 3: Run the full gate**

Run: `make check`
Expected: pass.

- [x] **Step 4: Verify in a real terminal**

Run: `make build && ./lookit`

Confirm by eye: connectors line up and the last child of each group takes `└`;
the seven hinted children read quietly and the rest are bare tokens; moving the
cursor onto a child swaps its column to the full note with no row movement;
pressing `/` alone changes nothing; typing a query expands children to addresses
with full notes and no connectors. If you cannot run an interactive terminal,
say so in your report and leave this for the human.

- [x] **Step 5: Commit**

```bash
git add tui/catalog.txt CLAUDE.md
git commit -m "docs: describe the catalog hint field"
```

---

## Notes for the implementer

- **This branch stacks on `feat/startpage-entry-grouping`** (PR #54). Do not
  rebase or merge anything; work on `feat/startpage-child-hints` as it stands.
- **`renderEntry` already has a `flattened` predicate** — `(Filtering ||
  FilterApplied) && query != ""`. Use it; do not reintroduce `isFiltered` at the
  call sites this plan touches.
- **Do not reorder `tui/catalog.txt`.** Display order is computed in
  `buildSections`; the file's grouping is for human editors.
- **Two tests in this repo were previously caught passing vacuously.** When a
  test asserts on rendered output, make it differential or assert on something
  the delegate's base styling cannot produce on its own.

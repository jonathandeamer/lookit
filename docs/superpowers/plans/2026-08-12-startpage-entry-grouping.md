# Startpage Entry Grouping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Order the startpage catalog by computed rules — communities alphabetical by host, services grouped under their host root with indented children — instead of by `catalog.txt` file order.

**Architecture:** All ordering and grouping happens in `buildSections` (`tui/sections.go`) at assembly time. `startEntry` gains three assembly-set display fields (`child`, `structural`, `bookmarked`); nothing about the catalog grammar, the bookmarks file format, or `finger/` changes. The rendering layer (`tui/start.go`) reads `child` to indent and shorten a row, and `structural` to drop a duplicate row from filtered views and from both counts.

**Tech Stack:** Go 1.26 toolchain, Bubble Tea v2 (`charm.land/bubbletea/v2`), `charm.land/bubbles/v2/list`, `charm.land/lipgloss/v2`. Tests are standard `go test`, no network, no real TTY.

**Spec:** `docs/superpowers/specs/2026-08-12-startpage-entry-grouping-design.md`

## Global Constraints

- **Commit messages: Conventional Commits.** No `Co-Authored-By`, no "Generated with Claude Code", in commits or anywhere outward-facing. A `commit-msg` hook enforces `type(scope): description`.
- **Do not commit, push, or open a PR unless the user asks.** Making edits and running checks is the unit of work.
- **`make check` is the gate**: `go vet ./...`, `gofmt -l .` empty, `golangci-lint run ./...`, `go test ./... -race`. It must pass before every commit in this plan.
- **`tui/` uses the v2 import paths** (`charm.land/...`), never `github.com/charmbracelet/...`. Do not touch `render/`, which is deliberately lipgloss v1.
- **Do not modify `finger/`.** This work is display-only; no target parsing, sanitizing or network behaviour changes.
- **Catalog notes must be traceable** to the server's own words or a conclusion its response plainly supports, and must contain no `#` (the comment stripper would eat it).
- **Pair every colour with a light/dark value** (`lipgloss.AdaptiveColor`) — no new hardcoded hex. This plan adds no colours; keep it that way.
- **macOS first.** No Alt/Option chords, no mouse capture.

---

### Task 1: Host/token helpers and the root invariant

The grouping rule needs to split a target into host and query token, and the
catalog must guarantee that every host with non-root services also ships a root
entry — otherwise a service group would be headed by a phantom. Queried
communities are sorted by host but remain plain rows. Both new catalog roots
were probed live on 2026-08-12; their notes come from the servers' own words.

**Files:**
- Modify: `tui/sections.go` (add two helpers at the bottom)
- Modify: `tui/catalog.txt` (two new service entries)
- Test: `tui/sections_test.go`, `tui/catalog_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `func entryHost(target string) string` and `func entryToken(target string) string`, used by every later task.

- [ ] **Step 1: Write the failing helper tests**

Add to `tui/sections_test.go`:

```go
func TestEntryHostAndToken(t *testing.T) {
	tests := []struct {
		target string
		host   string
		token  string
	}{
		{target: "@graph.no", host: "graph.no", token: ""},
		{target: "dict@bbs.airandwave.net", host: "bbs.airandwave.net", token: "dict"},
		{target: "wordsearch:today@bbs.airandwave.net", host: "bbs.airandwave.net", token: "wordsearch:today"},
		{target: "ring@thebackupbox.net", host: "thebackupbox.net", token: "ring"},
		{target: "1@happynetbox.com", host: "happynetbox.com", token: "1"},
	}
	for _, tt := range tests {
		if got := entryHost(tt.target); got != tt.host {
			t.Errorf("entryHost(%q) = %q, want %q", tt.target, got, tt.host)
		}
		if got := entryToken(tt.target); got != tt.token {
			t.Errorf("entryToken(%q) = %q, want %q", tt.target, got, tt.token)
		}
	}
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `go test ./tui/ -run TestEntryHostAndToken -count=1`
Expected: FAIL — `undefined: entryHost`.

- [ ] **Step 3: Implement the helpers**

Append to `tui/sections.go`:

```go
// entryHost is the address after the final "@": the machine a row belongs to.
// Grouping keys off this, so "@graph.no" and "oslo@graph.no" group together.
func entryHost(target string) string {
	if i := strings.LastIndex(target, "@"); i >= 0 {
		return target[i+1:]
	}
	return target
}

// entryToken is the query before the final "@" — empty for a host root, which
// is what makes a row a parent rather than a child.
func entryToken(target string) string {
	if i := strings.LastIndex(target, "@"); i >= 0 {
		return target[:i]
	}
	return ""
}
```

Add `import "strings"` to `tui/sections.go` (the file currently has no imports).

- [ ] **Step 4: Run the helper test**

Run: `go test ./tui/ -run TestEntryHostAndToken -count=1`
Expected: PASS.

- [ ] **Step 5: Write the failing invariant test**

Add to `tui/catalog_test.go`:

```go
// TestCatalogHasRootForEveryGroupedHost guards the grouping invariant: the
// startpage heads each host's services with that host's root row, so a child
// whose host ships no root would render under a phantom parent. The catalog is
// compiled in, so this must fail the build rather than ship.
func TestCatalogHasRootForEveryGroupedHost(t *testing.T) {
	entries, _ := parseCatalogData(catalogData)
	roots := make(map[string]bool, len(entries))
	for _, e := range entries {
		if entryToken(e.target) == "" {
			roots[entryHost(e.target)] = true
		}
	}
	for _, e := range entries {
		// Only services are grouped under parents. A queried community such as
		// ring@thebackupbox.net sorts by host but remains a plain row.
		if e.kind != kindService || entryToken(e.target) == "" {
			continue
		}
		if !roots[entryHost(e.target)] {
			t.Errorf("%s has no root entry for %s; every grouped child needs a parent row", e.target, entryHost(e.target))
		}
	}
}
```

- [ ] **Step 6: Run it and confirm it fails for the right reason**

Run: `go test ./tui/ -run TestCatalogHasRootForEveryGroupedHost -count=1`
Expected: FAIL, naming exactly four orphans — `textfile@typed-hole.org`,
`cyoa@typed-hole.org`, `smog@typed-hole.org` (host `typed-hole.org`) and
`calendar@flanigan.us` (host `flanigan.us`).

- [ ] **Step 7: Add the two roots to the catalog**

In `tui/catalog.txt`, add these two lines to the service block (position does
not matter — Task 2 makes display order independent of file order — but keep
each host's lines together for the next human editor):

```
service @typed-hole.org A small menu of fingers, from lobste.rs to smog
service @flanigan.us Four fingers: bonsai, ping, wisdom, calendar
```

- [ ] **Step 8: Run the catalog tests**

Run: `go test ./tui/ -run TestCatalog -count=1 -v`
Expected: PASS, including `TestCatalogIsWellFormed` (23 → 25 entries, still
above its floor of 20) and the new invariant test.

- [ ] **Step 9: Run the full gate**

Run: `make check`
Expected: all four gates pass.

- [ ] **Step 10: Commit**

```bash
git add tui/sections.go tui/sections_test.go tui/catalog.txt tui/catalog_test.go
git commit -m "feat(startpage): add host/token helpers and root invariant"
```

---

### Task 2: Computed ordering and host grouping

**Files:**
- Modify: `tui/bookmarks.go:38-54` (correct `entrySource`'s comment and add three `startEntry` fields)
- Modify: `tui/sections.go:29-78` (`buildSections`, plus a new `groupByHost`)
- Modify: `tui/app_test.go` (existing tests encode today's file order)
- Test: `tui/sections_test.go`

**Interfaces:**
- Consumes: `entryHost`, `entryToken` from Task 1.
- Produces: `startEntry.child bool` (render indented, token only), `startEntry.structural bool` (a parent copy duplicating a listing elsewhere), `startEntry.bookmarked bool` (declared here, populated in Task 3); `func groupByHost(listed []startEntry, roots map[string]startEntry, pinned map[string]bool) []startEntry`. The `pinned` parameter is accepted here and first read in Task 3 — take it now so the signature never changes under a later task.

- [ ] **Step 1: Write the failing ordering tests**

Add to `tui/sections_test.go`:

```go
func sectionTargets(t *testing.T, sections []startSection, id startSectionID) []string {
	t.Helper()
	for _, s := range sections {
		if s.id == id {
			targets := make([]string, 0, len(s.entries))
			for _, e := range s.entries {
				targets = append(targets, e.target)
			}
			return targets
		}
	}
	t.Fatalf("section %v not found", id)
	return nil
}

func TestCommunitiesSortAlphabeticallyByHost(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{})
	want := []string{
		"@cosmic.voyage",
		"@happynetbox.com",
		"@plan.cat",
		"ring@thebackupbox.net",
		"@tilde.team",
		"@zaibatsu.circumlunar.space",
	}
	if got := sectionTargets(t, sections, sectionCommunities); !reflect.DeepEqual(got, want) {
		t.Fatalf("communities = %v, want %v", got, want)
	}
}

// A queried community sorts on its host, but only service rows participate in
// parent/child grouping — even if that host also has a root catalog entry.
func TestQueriedCommunitySortsByHostButStaysFlat(t *testing.T) {
	catalog := []startEntry{
		{target: "ring@thebackupbox.net", kind: kindCommunity, note: "Ring", source: sourceCatalog},
		{target: "@thebackupbox.net", kind: kindService, note: "Root", source: sourceCatalog},
	}
	sections := buildSections(catalog, bookmarkFile{})
	for _, section := range sections {
		if section.id != sectionCommunities {
			continue
		}
		if got := section.entries[0]; got.child || got.structural {
			t.Fatalf("queried community = %+v, want a plain sorted row", got)
		}
		return
	}
	t.Fatal("COMMUNITIES section not found")
}

func TestServicesGroupUnderHostRoots(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{})
	want := []string{
		"@bbs.airandwave.net",
		"dict@bbs.airandwave.net",
		"quake@bbs.airandwave.net",
		"sudoku:easy@bbs.airandwave.net",
		"urban@bbs.airandwave.net",
		"weather@bbs.airandwave.net",
		"wordsearch:today@bbs.airandwave.net",
		"@flanigan.us",
		"calendar@flanigan.us",
		"@graph.no",
		"@happynetbox.com",
		"1@happynetbox.com",
		"bot@happynetbox.com",
		"browserversion@happynetbox.com",
		"originsfinger@happynetbox.com",
		"random@happynetbox.com",
		"@typed-hole.org",
		"cyoa@typed-hole.org",
		"smog@typed-hole.org",
		"textfile@typed-hole.org",
	}
	if got := sectionTargets(t, sections, sectionServices); !reflect.DeepEqual(got, want) {
		t.Fatalf("services = %v, want %v", got, want)
	}
}

// @graph.no has a root and no children, so it is a plain row: no indent.
func TestRootWithoutChildrenIsNotAParent(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{})
	for _, s := range sections {
		for _, e := range s.entries {
			if e.target == "@graph.no" && (e.child || e.structural) {
				t.Fatalf("@graph.no = %+v; want a plain row", e)
			}
		}
	}
}

// @happynetbox.com is a community listing AND the parent of its services, so
// the services copy is structural: a duplicate that exists only as structure.
func TestDualRoleHostAppearsInBothSections(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{})
	communities := sectionTargets(t, sections, sectionCommunities)
	if !slices.Contains(communities, "@happynetbox.com") {
		t.Fatalf("communities = %v, want @happynetbox.com", communities)
	}
	for _, s := range sections {
		if s.id != sectionServices {
			continue
		}
		if s.entries[10].target != "@happynetbox.com" || !s.entries[10].structural {
			t.Fatalf("services[10] = %+v, want a structural @happynetbox.com", s.entries[10])
		}
		if s.entries[11].target != "1@happynetbox.com" || !s.entries[11].child {
			t.Fatalf("services[11] = %+v, want a child row", s.entries[11])
		}
	}
}
```

Add `"reflect"` and `"slices"` to the test file's imports if absent.

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./tui/ -run 'TestCommunitiesSort|TestQueriedCommunity|TestServicesGroup|TestRootWithout|TestDualRole' -count=1`
Expected: FAIL — `e.child undefined` and order mismatches.

- [ ] **Step 3: Add the three display fields**

In `tui/bookmarks.go`, first replace the stale `entrySource` comment — source
continues to select the rendering section, but no longer decides what `b` does:

```go
// entrySource records whether an entry is rendered from BOOKMARKS or from the
// catalog. Bookmark state is separate because a retained catalog parent can
// represent a target that is also bookmarked.
```

Then replace the `startEntry` struct (lines 49-54):

```go
// startEntry is one startpage row. target/kind/note/source come from the two
// sources; child/structural/bookmarked are set during assembly and describe how
// the row is displayed, not where it came from.
type startEntry struct {
	target string
	kind   entryKind
	note   string
	source entrySource

	child      bool // indented under its host's parent row; renders its token only
	structural bool // a parent copy of a target listed elsewhere; not counted, hidden while filtering
	bookmarked bool // the target is in the bookmarks file, whatever section rendered this row
}
```

- [ ] **Step 4: Rewrite the catalog half of `buildSections`**

In `tui/sections.go`, replace the catalog loop (the `for _, group := range ...`
block, lines 58-76) with:

```go
	roots := make(map[string]startEntry, len(catalog))
	for _, e := range catalog {
		if entryToken(e.target) == "" {
			roots[entryHost(e.target)] = e
		}
	}

	for _, group := range []struct {
		title string
		kind  entryKind
		id    startSectionID
	}{
		{title: "COMMUNITIES", kind: kindCommunity, id: sectionCommunities},
		{title: "SERVICES", kind: kindService, id: sectionServices},
		{title: "PEOPLE", kind: kindPerson, id: sectionUnknown},
	} {
		var listed []startEntry
		for _, e := range catalog {
			if e.kind == group.kind && !pinned[e.target] {
				listed = append(listed, e)
			}
		}
		if len(listed) == 0 {
			continue
		}
		if group.kind == kindService {
			listed = groupByHost(listed, roots, pinned)
		} else {
			sort.SliceStable(listed, func(i, j int) bool {
				leftHost, rightHost := entryHost(listed[i].target), entryHost(listed[j].target)
				if leftHost != rightHost {
					return leftHost < rightHost
				}
				return entryToken(listed[i].target) < entryToken(listed[j].target)
			})
		}
		sections = append(sections, startSection{id: group.id, title: group.title, entries: listed})
	}
	return sections
}

// groupByHost orders service rows by host, then by query token within each host,
// with the host's root row first. Display order is therefore computed, not
// inherited from catalog.txt: a new catalog line can be added anywhere.
//
// A host with service children whose root is not itself a listed service row —
// because the root is classified differently (@happynetbox.com is a community)
// or because it is pinned — gets a structural copy of that root as its parent.
// Structure is not a listing: structural rows are not counted and vanish while
// filtering.
func groupByHost(listed []startEntry, roots map[string]startEntry, pinned map[string]bool) []startEntry {
	byHost := make(map[string][]startEntry, len(listed))
	var hosts []string
	for _, e := range listed {
		host := entryHost(e.target)
		if _, seen := byHost[host]; !seen {
			hosts = append(hosts, host)
		}
		byHost[host] = append(byHost[host], e)
	}
	sort.Strings(hosts)

	out := make([]startEntry, 0, len(listed)+len(hosts))
	for _, host := range hosts {
		rows := byHost[host]
		// A root's token is "", which sorts before every child.
		sort.SliceStable(rows, func(i, j int) bool {
			return entryToken(rows[i].target) < entryToken(rows[j].target)
		})

		hasChild := false
		for _, e := range rows {
			if entryToken(e.target) != "" {
				hasChild = true
				break
			}
		}
		root, hasRoot := roots[host]
		// No children means no group. No root means the catalog invariant is
		// broken (TestCatalogHasRootForEveryGroupedHost); render flat rather
		// than inventing a parent, so the rows stay reachable.
		if !hasChild || !hasRoot {
			out = append(out, rows...)
			continue
		}

		if rows[0].target == root.target {
			out = append(out, rows[0])
			rows = rows[1:]
		} else {
			parent := root
			parent.structural = true
			out = append(out, parent)
		}
		for _, e := range rows {
			e.child = true
			out = append(out, e)
		}
	}
	return out
}
```

Move the `pinned` map construction above the catalog loop (it is currently
built at line 54, after the `catalogHidden` early return — keep that order:
`catalogHidden` still returns before any catalog work). Add `"sort"` to the
imports.

- [ ] **Step 5: Run the new tests**

Run: `go test ./tui/ -run 'TestCommunitiesSort|TestQueriedCommunity|TestServicesGroup|TestRootWithout|TestDualRole' -count=1 -v`
Expected: PASS.

- [ ] **Step 6: Reconcile the existing order-dependent tests**

Run: `go test ./tui/ -count=1`

Several `tui/app_test.go` tests encode the old file order. Update the fixtures
to the new alphabetical order — do not weaken the assertions:

- `TestBookmarkingCatalogRowStaysAtSectionOrdinal`: bookmarks `@plan.cat`,
  which sits at community ordinal 2. After pinning, ordinal 2 is
  `ring@thebackupbox.net`; change the expected target from `@happynetbox.com`.
- `TestBookmarkingCatalogRowsStayAtSectionOrdinal`: `middle` pins `@tilde.team`
  (ordinal 4) → expect `@zaibatsu.circumlunar.space`; `final` pins
  `@zaibatsu.circumlunar.space` (last) → expect `@tilde.team`.
- Any test asserting the first community row now expects `@cosmic.voyage`.

For each failure, read what the test is protecting before editing it. If a
failure is *not* an order fixture — for example a count changing because the
catalog grew by two — stop and check it against the spec instead of adjusting
the number.

- [ ] **Step 7: Run the full gate**

Run: `make check`
Expected: all four gates pass.

- [ ] **Step 8: Commit**

```bash
git add tui/sections.go tui/sections_test.go tui/bookmarks.go tui/app_test.go
git commit -m "feat(startpage): compute catalog order and group services by host"
```

---

### Task 3: Bookmark state independent of row source

A retained parent copy is `sourceCatalog` even when its target is pinned.
`startBookmarkAction` derives the `b` hint from `source`, while `toggleBookmark`
decides by scanning the bookmarks file — so without this task a pinned parent
advertises `b bookmark` and then *removes* the bookmark.

**Files:**
- Modify: `tui/sections.go` (`buildSections`, `groupByHost`)
- Modify: `tui/app.go:1509-1515` (`startBookmarkAction`)
- Test: `tui/sections_test.go`, `tui/app_test.go`

**Interfaces:**
- Consumes: `startEntry.bookmarked` and the three-argument `groupByHost` declared in Task 2.
- Produces: populated `startEntry.bookmarked` state on bookmark rows and retained
  parents; `startBookmarkAction` reads that state instead of `source`.

- [ ] **Step 1: Write the failing tests**

Add to `tui/sections_test.go`:

```go
// A pinned parent keeps heading its group — structure is not a listing — but it
// must know it is bookmarked, or the b hint lies about what the key does.
func TestPinnedParentKeepsHeadingItsGroupAndKnowsItIsPinned(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{targets: []string{"@bbs.airandwave.net"}})
	services := sectionTargets(t, sections, sectionServices)
	if services[0] != "@bbs.airandwave.net" {
		t.Fatalf("services[0] = %q, want the parent retained", services[0])
	}
	for _, s := range sections {
		if s.id != sectionServices {
			continue
		}
		if !s.entries[0].structural || !s.entries[0].bookmarked {
			t.Fatalf("parent = %+v, want structural and bookmarked", s.entries[0])
		}
		if !s.entries[1].child || s.entries[1].target != "dict@bbs.airandwave.net" {
			t.Fatalf("services[1] = %+v, want dict still a child", s.entries[1])
		}
	}
}

func TestBookmarkSectionEntriesAreMarkedBookmarked(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{targets: []string{"@tilde.team"}})
	for _, s := range sections {
		if s.id != sectionBookmarks {
			continue
		}
		if !s.entries[0].bookmarked {
			t.Fatalf("bookmark row = %+v, want bookmarked", s.entries[0])
		}
	}
}
```

Add to `tui/app_test.go`:

```go
// The b hint must describe what b will actually do on both forms of a parent:
// its canonical service listing when unpinned and its retained structural copy
// when pinned.
func TestParentBookmarkHintAndActionAgree(t *testing.T) {
	const target = "@bbs.airandwave.net"
	tests := []struct {
		name       string
		seed       string
		structural bool
		wantHint   string
		wantSaved  bool
	}{
		{name: "unpinned adds", wantHint: "bookmark", wantSaved: true},
		{name: "pinned structural removes", seed: target + "\n", structural: true, wantHint: "remove"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.seed == "" {
				path = useTempBookmarks(t)
			} else {
				path = seedBookmarks(t, tt.seed)
			}
			m := newApp(stubFetch(t), colorprofile.NoTTY)
			m.blurInput()

			found := false
			for i, item := range m.start.list.VisibleItems() {
				it, ok := item.(startItem)
				if ok && it.section == sectionServices && it.entry.target == target && it.entry.structural == tt.structural {
					m.start.list.Select(i)
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("service parent %q (structural=%v) not found", target, tt.structural)
			}
			if got := m.startBookmarkAction(); got != tt.wantHint {
				t.Fatalf("hint = %q, want %q", got, tt.wantHint)
			}

			next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
			m = next.(appModel)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(data), target+"\n"); got != tt.wantSaved {
				t.Fatalf("bookmark present = %v, want %v; file = %q", got, tt.wantSaved, data)
			}
		})
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./tui/ -run 'TestPinnedParent|TestBookmarkSectionEntries|TestParentBookmarkHintAndAction' -count=1`
Expected: FAIL — parent not marked bookmarked; hint returns `"bookmark"`.

- [ ] **Step 3: Populate `bookmarked`**

In `tui/sections.go`, in the bookmarks branch of `buildSections`, after
`e = catalogEntry; e.source = sourceBookmark`, and for the unmatched case too,
set the flag on every bookmark row:

```go
		for _, target := range bm.targets {
			e := startEntry{target: target, source: sourceBookmark}
			if catalogEntry, ok := byTarget[target]; ok {
				e = catalogEntry
				e.source = sourceBookmark
			}
			e.bookmarked = true
			bookmarked = append(bookmarked, e)
		}
```

`groupByHost` already takes `pinned` from Task 2; read it now, inside the
structural branch:

```go
		} else {
			parent := root
			parent.structural = true
			parent.bookmarked = pinned[root.target]
			out = append(out, parent)
		}
```

- [ ] **Step 4: Fix the hint**

In `tui/app.go`, replace `startBookmarkAction`:

```go
// startBookmarkAction names what b will do. It reads bookmarked state, not the
// section that rendered the row: a parent copy survives dedup and is still
// sourceCatalog while pinned, so source would advertise the wrong verb.
func (m appModel) startBookmarkAction() string {
	entry, ok := m.start.selected()
	if ok && entry.bookmarked {
		return "remove"
	}
	return "bookmark"
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./tui/ -run 'TestPinnedParent|TestBookmarkSectionEntries|TestParentBookmarkHintAndAction' -count=1 -v`
Expected: PASS.

- [ ] **Step 6: Run the full gate**

Run: `make check`
Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add tui/sections.go tui/sections_test.go tui/app.go tui/app_test.go
git commit -m "fix(startpage): derive the bookmark hint from bookmark state"
```

---

### Task 4: Render children indented and suppress structural filter duplicates

**Files:**
- Modify: `tui/start.go:37-47` (`FilterValue`), `tui/start.go:203-263` (`renderEntry` and `startRowTarget`)
- Test: `tui/start_test.go`, `tui/app_test.go`

**Interfaces:**
- Consumes: `startEntry.child`, `startEntry.structural` from Task 2.
- Produces: `func startRowTarget(entry startEntry, filtered bool) string` — the target column text for a row.

- [ ] **Step 1: Write the failing tests**

Add to `tui/start_test.go`:

```go
func TestStartRowTargetShortensChildrenUnlessFiltered(t *testing.T) {
	child := startEntry{target: "dict@bbs.airandwave.net", child: true}
	if got, want := startRowTarget(child, false), "  dict"; got != want {
		t.Errorf("unfiltered child = %q, want %q", got, want)
	}
	// Filtering removes the parent that supplies the host, so the row must
	// carry its full address again.
	if got, want := startRowTarget(child, true), "dict@bbs.airandwave.net"; got != want {
		t.Errorf("filtered child = %q, want %q", got, want)
	}
	parent := startEntry{target: "@bbs.airandwave.net"}
	if got, want := startRowTarget(parent, false), "@bbs.airandwave.net"; got != want {
		t.Errorf("parent = %q, want %q", got, want)
	}
}

// In the narrow two-line layout the note sits under the target, so an
// unindented note would hang left of its own row.
func TestNarrowChildRowIndentsBothLines(t *testing.T) {
	common := testCommon()
	common.width = 40
	st := common.ensureStyles()
	d := newStartDelegate(common, st)
	if d.Height() != 2 {
		t.Fatalf("delegate height = %d at width 40, want the two-line layout", d.Height())
	}
	l := list.New([]list.Item{
		startItem{entry: startEntry{target: "dict@bbs.airandwave.net", note: "Dictionary lookup", child: true}, section: sectionServices},
		startItem{entry: startEntry{target: "other@example.com", note: "Other row"}, section: sectionServices},
	}, d, 40, 4)
	// Render the child unselected so the selection shelf's border and padding do
	// not get mistaken for the child's own two-space indent.
	l.Select(1)
	var buf strings.Builder
	d.Render(&buf, l, 0, l.Items()[0])
	for _, line := range strings.Split(ansi.Strip(buf.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			t.Errorf("line %q is not indented; both lines of a child row must be", line)
		}
	}
}

func TestFilteredChildRendersFullTargetWithMatchHighlight(t *testing.T) {
	for _, query := range []string{"dict", "airandwave"} {
		t.Run(query, func(t *testing.T) {
			common := testCommon()
			common.width = 80
			sections := []startSection{{
				id: sectionServices, title: "SERVICES",
				entries: []startEntry{{
					target: "dict@bbs.airandwave.net", note: "Dictionary lookup",
					source: sourceCatalog, child: true,
				}},
			}}
			m := newStart(common, sections, "", "")
			m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
			m = typeStartFilter(t, m, query)
			m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEnter})

			view := m.View()
			plain := ansi.Strip(view)
			lineIndex := lineIndexContaining(t, plain, "dict@bbs.airandwave.net")
			line := strings.Split(view, "\n")[lineIndex]
			if got := underlinedText(line); got != query {
				t.Fatalf("underlined text = %q, want %q\n%q", got, query, line)
			}
		})
	}
}

// A structural row duplicates a target listed elsewhere. Filtering drops the
// headers that tell the two copies apart, so the duplicate must drop out too.
func TestStructuralRowsDoNotMatchFilters(t *testing.T) {
	structural := startItem{entry: startEntry{target: "@happynetbox.com", structural: true}}
	if got := structural.FilterValue(); got != "" {
		t.Fatalf("FilterValue() = %q, want empty so the copy is filtered out", got)
	}
	listing := startItem{entry: startEntry{target: "@happynetbox.com", note: "n"}}
	if got, want := listing.FilterValue(), "@happynetbox.com n"; got != want {
		t.Fatalf("FilterValue() = %q, want %q", got, want)
	}
}
```

Add to `tui/app_test.go`:

```go
func TestFilteringShowsOneHappynetboxParentPinnedOrUnpinned(t *testing.T) {
	tests := []struct {
		name string
		seed string
	}{
		{name: "unpinned"},
		{name: "pinned", seed: "@happynetbox.com\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.seed == "" {
				useTempBookmarks(t)
			} else {
				seedBookmarks(t, tt.seed)
			}
			m := newApp(stubFetch(t), colorprofile.NoTTY)
			m.blurInput()
			m.start.list.SetFilterText("happynetbox.com")
			var seen int
			for _, item := range m.start.list.VisibleItems() {
				if it, ok := item.(startItem); ok && it.entry.target == "@happynetbox.com" {
					seen++
				}
			}
			if seen != 1 {
				t.Fatalf("@happynetbox.com appears %d times under a filter, want 1", seen)
			}
		})
	}
}

func TestFilteringKeepsCanonicalServiceParents(t *testing.T) {
	for _, target := range []string{
		"@bbs.airandwave.net",
		"@flanigan.us",
		"@graph.no",
		"@typed-hole.org",
	} {
		t.Run(target, func(t *testing.T) {
			useTempBookmarks(t)
			m := newApp(stubFetch(t), colorprofile.NoTTY)
			m.blurInput()
			m.start.list.SetFilterText(target)
			var roots []startEntry
			for _, item := range m.start.list.VisibleItems() {
				if it, ok := item.(startItem); ok && it.entry.target == target {
					roots = append(roots, it.entry)
				}
			}
			if len(roots) != 1 || roots[0].structural {
				t.Fatalf("filtered roots = %+v, want one canonical parent", roots)
			}
		})
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./tui/ -run 'TestStartRowTarget|TestNarrowChild|TestFilteredChild|TestStructuralRows|TestFilteringShowsOne|TestFilteringKeepsCanonical' -count=1`
Expected: FAIL — `undefined: startRowTarget`; structural duplicates remain visible.

- [ ] **Step 3: Suppress structural rows while filtering**

In `tui/start.go`, extend `FilterValue`:

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

- [ ] **Step 4: Add the target-column helper and use it**

Append to `tui/start.go`:

```go
// startRowTarget is the target column's text. A child shows only its query
// token, indented, because the parent row above it states the host — but under
// a filter the parent may not be on screen, so the full address returns.
func startRowTarget(entry startEntry, filtered bool) string {
	if entry.child && !filtered {
		return "  " + entryToken(entry.target)
	}
	return entry.target
}
```

In `renderEntry`, compute the row's target once, just after the existing
`isFiltered` declaration:

```go
	rowTarget := startRowTarget(item.entry, isFiltered)
```

Then replace both uses of `item.entry.target` with `rowTarget`: the wide
branch's `renderStartField(item.entry.target, targetWidth, …)` and the narrow
branch's `renderStartField(item.entry.target, titleWidth, …)`.

Leave the `splitStartMatches(m.MatchesForItem(index), item.entry.target)` call
exactly as it is. Match offsets are computed against the full target, and
matches only exist when `isFiltered` — where `rowTarget` *is* the full target —
so the two agree without any adjustment.

In the narrow two-line branch, the note line needs the same indent as the
target line, or a child's description hangs left of its own target. Indent it
for unfiltered children only:

```go
	rowNote := item.entry.note
	if item.entry.child && !isFiltered {
		rowNote = "  " + rowNote
	}
```

and pass `rowNote` to the narrow branch's `renderStartField(…, descWidth, …)`.
The wide single-line branch keeps `item.entry.note` unchanged: there the note
sits in its own column, already offset from the target.

- [ ] **Step 5: Run the tests**

Run: `go test ./tui/ -run 'TestStartRowTarget|TestNarrowChild|TestFilteredChild|TestStructuralRows|TestFilteringShowsOne|TestFilteringKeepsCanonical' -count=1 -v`
Expected: PASS.

- [ ] **Step 6: Run the full gate**

Run: `make check`
Expected: pass. Existing delegate tests that assert a service row's rendered
text may need their expected string indented — update them to the new shape.

- [ ] **Step 7: Commit**

```bash
git add tui/start.go tui/start_test.go tui/app_test.go
git commit -m "feat(startpage): indent child rows and flatten duplicates when filtering"
```

---

### Task 5: Exclude structural copies from both totals

**Files:**
- Modify: `tui/start.go:92-109` (`startCounts`)
- Modify: `tui/app.go:1496-1503` (status-bar tally)
- Test: `tui/start_test.go`, `tui/app_test.go`

**Interfaces:**
- Consumes: `startEntry.structural` from Task 2.
- Produces: nothing new.

- [ ] **Step 1: Write the failing tests**

Add to `tui/start_test.go`:

```go
// Counts describe displayed listings after bookmark/catalog suppression. A
// structural parent is navigation structure, not another listing, so it must
// not raise either total.
func TestCountsIgnoreStructuralRows(t *testing.T) {
	items := []list.Item{
		startItem{header: "SERVICES", section: sectionServices},
		startItem{entry: startEntry{target: "@happynetbox.com", structural: true}, section: sectionServices},
		startItem{entry: startEntry{target: "bot@happynetbox.com", child: true}, section: sectionServices},
	}
	if got := startCounts(items); got.services != 1 {
		t.Fatalf("services = %d, want 1 — the structural copy is not a listing", got.services)
	}
}
```

Add to `tui/app_test.go`:

```go
func TestOverviewAndStatusCountsExcludeStructuralCopies(t *testing.T) {
	tests := []struct {
		name   string
		seed   string
		filter string
		want   startOverviewCounts
		total  int
	}{
		{name: "unfiltered", want: startOverviewCounts{communities: 6, services: 19}, total: 25},
		{name: "child pinned", seed: "dict@bbs.airandwave.net\n", want: startOverviewCounts{bookmarks: 1, communities: 6, services: 18}, total: 25},
		{name: "parent pinned", seed: "@bbs.airandwave.net\n", want: startOverviewCounts{bookmarks: 1, communities: 6, services: 18}, total: 25},
		{name: "repeated bookmarks stay repeated", seed: "@tilde.team\n@tilde.team\n", want: startOverviewCounts{bookmarks: 2, communities: 5, services: 19}, total: 26},
		{name: "filtered", filter: "happynetbox.com", want: startOverviewCounts{communities: 1, services: 5}, total: 6},
		{name: "filtered pinned parent", seed: "@happynetbox.com\n", filter: "happynetbox.com", want: startOverviewCounts{bookmarks: 1, services: 5}, total: 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.seed == "" {
				useTempBookmarks(t)
			} else {
				seedBookmarks(t, tt.seed)
			}
			m := newApp(stubFetch(t), colorprofile.NoTTY)
			m.blurInput()
			if tt.filter != "" {
				m.start.list.SetFilterText(tt.filter)
			}
			if got := m.start.overviewCounts(); got != tt.want {
				t.Fatalf("overview counts = %+v, want %+v", got, tt.want)
			}
			bar := m.startBar(80, newStyles(true))
			if got, want := bar.meta, fmt.Sprintf("%d entries", tt.total); got != want {
				t.Fatalf("status meta = %q, want %q", got, want)
			}
		})
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./tui/ -run 'TestCountsIgnoreStructural|TestOverviewAndStatusCounts' -count=1`
Expected: FAIL — services = 2 in the focused unit test; unfiltered overview and
status counts are both 26 rather than 25.

- [ ] **Step 3: Skip structural rows in `startCounts`**

In `tui/start.go`:

```go
func startCounts(items []list.Item) startOverviewCounts {
	var counts startOverviewCounts
	for _, item := range items {
		it, ok := item.(startItem)
		// A structural parent is navigation structure, not another listing.
		if !ok || !it.selectable() || it.entry.structural {
			continue
		}
		switch it.section {
		case sectionBookmarks:
			counts.bookmarks++
		case sectionCommunities:
			counts.communities++
		case sectionServices:
			counts.services++
		}
	}
	return counts
}
```

- [ ] **Step 4: Skip them in the status bar too**

In `tui/app.go`, in the startpage bar branch:

```go
	n := 0
	for _, it := range m.start.list.VisibleItems() {
		// Structural parent copies are excluded here for the same reason as in
		// startCounts: the bar and the overview must agree.
		if si, ok := it.(startItem); ok && si.selectable() && !si.entry.structural {
			n++
		}
	}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./tui/ -run 'TestCountsIgnoreStructural|TestOverviewAndStatusCounts' -count=1 -v`
Expected: PASS.

- [ ] **Step 6: Run the full gate**

Run: `make check`
Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add tui/start.go tui/start_test.go tui/app.go tui/app_test.go
git commit -m "fix(startpage): exclude structural rows from entry counts"
```

---

### Task 6: Document the computed order

**Files:**
- Modify: `CLAUDE.md` (the startpage bullet under "TUI internals")

**Interfaces:**
- Consumes: everything above.
- Produces: nothing.

- [ ] **Step 1: Update the architecture note**

In `CLAUDE.md`, in the `appModel` bullet describing `stateStart`, add after the
existing `catalog off` sentence:

```markdown
Startpage order is **computed, not file order**: `buildSections` sorts
communities alphabetically by host and groups services under their host root
(parent first, children indented and showing only their query token), so a new
`catalog.txt` line can be added anywhere. A host with non-root service entries
must ship a root entry — `TestCatalogHasRootForEveryGroupedHost` fails the build
otherwise. A root that heads a group but is listed elsewhere
(`@happynetbox.com`, a community) or is pinned is marked `structural`: it
renders as the parent, is excluded from both the overview and status-bar
counts, and drops out while filtering so grouping never adds a duplicate to a
flattened view. Repeated user-authored bookmark lines retain their existing
row and count semantics.
```

- [ ] **Step 2: Run the full gate one last time**

Run: `make check`
Expected: pass.

- [ ] **Step 3: Verify against a real terminal**

Run: `make build && ./lookit`

Confirm by eye: communities alphabetical; services grouped with indented
children; `@happynetbox.com` heading its services and also listed under
COMMUNITIES; the overview and the status-bar count agreeing; `/happynetbox`
suppressing the structural duplicate; `b` on a pinned parent reading `remove`.

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: describe the computed startpage order"
```

---

## Notes for the implementer

- **`tui/start_test.go` already exists.** Its current imports include the list,
  Bubble Tea, ANSI, string, and testing helpers used by Task 4; add no duplicate
  imports.
- **Do not reorder `catalog.txt` to match the display.** File order is now
  irrelevant to rendering, and keeping the file grouped by host is what makes it
  editable by hand.
- **If an existing test fails in a way this plan does not predict**, read what it
  protects and check the spec before changing its expectations. Several
  startpage tests encode ordinals deliberately, as regression guards for
  selection behaviour after a bookmark toggle.

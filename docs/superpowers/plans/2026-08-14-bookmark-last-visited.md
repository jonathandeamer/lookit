# Bookmark Last-Visited Dates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show a relative last-visited date in the note column of bookmarked startpage rows, stamped into the bookmarks file on every successful visit — implementing the approved spec `docs/superpowers/specs/2026-08-14-bookmark-last-visited-design.md` (issue #108).

**Architecture:** The bookmarks file grammar grows an optional second field (`<target> [<RFC3339 UTC timestamp>]`). Parsing is strict — a malformed date is a reported problem, never rendered raw — so the file's no-`sanitize` ingress guarantee holds. A new `updateBookmarkLine` rewrites the date in place via raw byte-offset splicing; a successful landing (`entry.Err == nil`) stamps the target if and only if it is already in the file. Rendering lives entirely in `startRowNote` (the single place #112 made responsible for the note column): bookmark rows show a fuzzy relative date, standing down when a filter flattens the view so the catalog note returns.

**Tech Stack:** Go 1.21+, Bubble Tea v2 (`charm.land/bubbletea/v2`), lipgloss v2. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-08-14-bookmark-last-visited-design.md` — read it first; it records the *why* behind every rule below.

## Global Constraints

- Work in the worktree `/Users/jonathan/lookit/.claude/worktrees/bookmark-last-visited` on branch `feat/bookmark-last-visited` (already contains #110/#111/#112 via merge).
- Conventional Commits; **no `Co-Authored-By` or AI trailers** in any commit.
- Tests are offline with injected fakes: stub `bookmarksPathFn` (via `useTempBookmarks(t)` / `useBookmarksPath(t, path)` from `tui/bookmarks_test.go`) and `nowFn`, always restoring with `t.Cleanup`.
- Target matching is **exact string equality** on the record's target vs `entry.Target.Raw`. `@tilde.team` and `tilde.team:79` are different bookmarks.
- A malformed date field is a **refused, reported** record (a `parseProblem`), never a silent drop, never a dateless bookmark.
- A stamp is written only when `entry.Err == nil` (post-body resets flagged `Meta.Truncated` have `Err == nil` and do stamp; dial errors and mid-body failures do not).
- All write-path failures degrade silently — navigation is never blocked by bookkeeping.
- The date never enters `FilterValue` and never enters target-column measurement (`startTargetColumn`).
- Run `go test ./tui/ -run <TestName> -count=1 -v` for per-task verification and `make check` before each task's commit.

---

### Task 1: Two-field record grammar and the visited map

**Files:**
- Modify: `tui/bookmarks.go` (`bookmarkFile` struct ~line 76, `parseBookmarks` ~line 82, `parseBookmarkTarget` ~line 146, `deleteBookmarkLine` ~line 285, `validateBookmarkRecordTarget` ~line 159)
- Test: `tui/bookmarks_test.go`

**Interfaces:**
- Produces: `parseBookmarkTarget(line string) (string, time.Time, error)` — target, visit instant (zero = no date), error. `bookmarkFile` gains `visited map[string]time.Time` keyed by target (last-wins on duplicate lines).
- Consumed by: Task 2 (`parseBookmarkTarget`, round-trip guard), Task 4 (`bookmarkFile.visited`).

- [ ] **Step 1: Write the failing tests**

Append to `tui/bookmarks_test.go`:

```go
func TestParseBookmarksReadsLastVisited(t *testing.T) {
	got := parseBookmarks([]byte("@plan.cat 2026-08-14T15:04:05Z\njonathan@tilde.team\n"))
	if len(got.problems) != 0 {
		t.Fatalf("problems = %+v, want none", got.problems)
	}
	if len(got.targets) != 2 {
		t.Fatalf("targets = %+v, want 2", got.targets)
	}
	want := time.Date(2026, 8, 14, 15, 4, 5, 0, time.UTC)
	if !got.visited["@plan.cat"].Equal(want) {
		t.Errorf("visited[@plan.cat] = %v, want %v", got.visited["@plan.cat"], want)
	}
	if _, ok := got.visited["jonathan@tilde.team"]; ok {
		t.Errorf("visited[jonathan@tilde.team] present, want absent for a dateless record")
	}
}

func TestParseBookmarksRejectsBadDate(t *testing.T) {
	for _, line := range []string{
		"@plan.cat friendly",                  // two fields, second not a date
		"@plan.cat 2026-08-14",                // date only, not RFC 3339
		"@plan.cat 2026-08-14T15:04:05",       // no zone
		"@plan.cat 2026-08-14T15:04:05+00:00", // RFC 3339, but not the UTC Z form
		"@plan.cat 2026-08-14T15:04:05-07:00", // offset zone
		"@plan.cat 2026-08-14T15:04:05Z extra", // three fields
	} {
		got := parseBookmarks([]byte(line + "\n"))
		if len(got.targets) != 0 {
			t.Errorf("%q: targets = %+v, want none (a bad date refuses the record)", line, got.targets)
		}
		if len(got.problems) != 1 {
			t.Errorf("%q: problems = %+v, want exactly 1 reported problem", line, got.problems)
		}
	}
}

func TestParseBookmarksDuplicateDatesLastWins(t *testing.T) {
	got := parseBookmarks([]byte("@plan.cat 2026-08-01T00:00:00Z\n@plan.cat 2026-08-14T00:00:00Z\n"))
	if len(got.targets) != 2 {
		t.Fatalf("targets = %+v, want both duplicate rows kept", got.targets)
	}
	want := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	if !got.visited["@plan.cat"].Equal(want) {
		t.Errorf("visited[@plan.cat] = %v, want last-wins %v", got.visited["@plan.cat"], want)
	}

	// A later dateless duplicate is last-wins unknown, not "keep the earlier date".
	got = parseBookmarks([]byte("@plan.cat 2026-08-14T00:00:00Z\n@plan.cat\n"))
	if _, ok := got.visited["@plan.cat"]; ok {
		t.Errorf("visited[@plan.cat] = %v, want absent after a trailing dateless duplicate", got.visited["@plan.cat"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tui/ -run 'TestParseBookmarks' -count=1 -v`
Expected: FAIL — `parseBookmarks` result has no `visited` field (compile error).

- [ ] **Step 3: Implement the grammar**

In `tui/bookmarks.go`, add `"time"` to the imports. Change `bookmarkFile`:

```go
// bookmarkFile is the parsed user file.
type bookmarkFile struct {
	targets       []string
	visited       map[string]time.Time // last-visited instant per target; absent = unknown
	catalogHidden bool
	problems      []parseProblem
}
```

Change `parseBookmarkTarget` to the two-field grammar:

```go
// parseBookmarkTarget accepts a target with an optional RFC 3339 last-visited
// date: "<target>" or "<target> <timestamp>". Anything else is refused. A bad
// date refuses the whole record — a line lookit cannot understand is surfaced
// as a problem, never guessed at (the file's existing contract), and only a
// validated time.Time ever reaches the display, so the file still needs no
// sanitize call.
func parseBookmarkTarget(line string) (string, time.Time, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 || len(fields) > 2 {
		return "", time.Time{}, fmt.Errorf("expected a target with an optional RFC 3339 date, got %q", line)
	}
	if err := validateTarget(fields[0]); err != nil {
		return "", time.Time{}, err
	}
	if len(fields) == 1 {
		return fields[0], time.Time{}, nil
	}
	visited, err := time.Parse(time.RFC3339, fields[1])
	// Spec: strict RFC 3339 UTC at seconds precision (the Z form the write
	// path emits). time.RFC3339 also accepts offsets and +00:00; those are
	// refused so the file's grammar stays one token, not "any RFC 3339".
	if err != nil || fields[1] != visited.UTC().Truncate(time.Second).Format(time.RFC3339) {
		return "", time.Time{}, fmt.Errorf("bad last-visited date %q (want RFC 3339, e.g. 2026-08-14T15:04:05Z)", fields[1])
	}
	return fields[0], visited, nil
}
```

Update the three callers. In `parseBookmarks`:

```go
		target, visited, err := parseBookmarkTarget(line)
		if err != nil {
			out.problems = append(out.problems, parseProblem{line: lineNo, reason: err.Error()})
			continue
		}
		out.targets = append(out.targets, target)
		if out.visited == nil {
			out.visited = make(map[string]time.Time)
		}
		if visited.IsZero() {
			delete(out.visited, target) // last-line wins, including a trailing dateless duplicate
		} else {
			out.visited[target] = visited
		}
```

In `validateBookmarkRecordTarget` and `deleteBookmarkLine`, discard the new middle value:

```go
	parsed, _, err := parseBookmarkTarget(strings.TrimSpace(stripComment(target)))
```
```go
		parsed, _, err := parseBookmarkTarget(strings.TrimSpace(stripComment(line)))
```

Update the doc comment above `validateBookmarkRecordTarget`: it now verifies the target survives the comment-and-two-field grammar unchanged (the add path still passes a bare target, which parses as a one-field record).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestParseBookmarks|TestDeleteBookmark|TestAppendBookmark|TestValidateBookmark|TestLoadBookmarks|TestInitializeBookmark' -count=1`
Expected: PASS, including all pre-existing bookmark tests (the three-field and two-field-non-date cases in `TestParseBookmarksRejectsBadDate` keep `@plan.cat Big friendly pubnix` refused).

- [ ] **Step 5: Commit**

```bash
make check && git add tui/bookmarks.go tui/bookmarks_test.go
git commit -m "feat(bookmarks): parse an optional RFC 3339 last-visited date"
```

---

### Task 2: `updateBookmarkLine` — the first rewrite operation

**Files:**
- Modify: `tui/bookmarks.go` (add after `deleteBookmarkLine` ~line 296)
- Test: `tui/bookmarks_test.go`

**Interfaces:**
- Consumes: `parseBookmarkTarget` (Task 1).
- Produces: `updateBookmarkLine(data []byte, target string, ts time.Time) ([]byte, bool)` — updated bytes and whether any record matched. Consumed by Task 3's `stampVisit`.

- [ ] **Step 1: Write the failing tests**

Append to `tui/bookmarks_test.go`:

```go
func TestUpdateBookmarkLineStampsTheDate(t *testing.T) {
	ts := time.Date(2026, 8, 14, 15, 4, 5, 0, time.UTC)
	in := "# my shelf\n@plan.cat\n\njonathan@tilde.team 2026-01-01T00:00:00Z # the author\ncatalog off\n"
	want := "# my shelf\n@plan.cat 2026-08-14T15:04:05Z\n\njonathan@tilde.team 2026-08-14T15:04:05Z # the author\ncatalog off\n"
	got, changed := updateBookmarkLine([]byte(in), "@plan.cat", ts)
	if !changed || string(got) != in[:len("# my shelf\n")]+"@plan.cat 2026-08-14T15:04:05Z\n\njonathan@tilde.team 2026-01-01T00:00:00Z # the author\ncatalog off\n" {
		t.Fatalf("first target: changed=%v got=\n%q", changed, got)
	}
	got, changed = updateBookmarkLine([]byte(in), "jonathan@tilde.team", ts)
	if !changed || string(got) != want {
		t.Fatalf("second target: changed=%v got=\n%q\nwant=\n%q", changed, got, want)
	}
}

func TestUpdateBookmarkLinePreservesSpacingAndComments(t *testing.T) {
	ts := time.Date(2026, 8, 14, 15, 4, 5, 0, time.UTC)
	in := "  @plan.cat   2026-01-01T00:00:00Z   #  spaced comment\n"
	got, changed := updateBookmarkLine([]byte(in), "@plan.cat", ts)
	if !changed {
		t.Fatal("changed = false, want the record rewritten")
	}
	s := string(got)
	if !strings.HasPrefix(s, "  @plan.cat 2026-08-14T15:04:05Z") {
		t.Errorf("lost leading whitespace or record: %q", s)
	}
	if !strings.HasSuffix(s, "#  spaced comment\n") {
		t.Errorf("comment text changed: %q", s)
	}
}

func TestUpdateBookmarkLineNoMatchWritesNothing(t *testing.T) {
	ts := time.Date(2026, 8, 14, 15, 4, 5, 0, time.UTC)
	in := "@plan.cat\n@tilde.team not-a-date\n" // second line: malformed, never rewritten
	got, changed := updateBookmarkLine([]byte(in), "tilde.team:79", ts)
	if changed || string(got) != in {
		t.Fatalf("changed=%v got=%q, want the file byte-identical (exact match only; malformed lines untouched)", changed, got)
	}
}

func TestUpdateBookmarkLineUpdatesEveryDuplicate(t *testing.T) {
	ts := time.Date(2026, 8, 14, 15, 4, 5, 0, time.UTC)
	in := "@plan.cat\n@plan.cat 2026-01-01T00:00:00Z\n"
	want := "@plan.cat 2026-08-14T15:04:05Z\n@plan.cat 2026-08-14T15:04:05Z\n"
	got, changed := updateBookmarkLine([]byte(in), "@plan.cat", ts)
	if !changed || string(got) != want {
		t.Fatalf("changed=%v got=%q, want both duplicates at %q", changed, got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tui/ -run 'TestUpdateBookmarkLine' -count=1 -v`
Expected: FAIL — `updateBookmarkLine` undefined.

- [ ] **Step 3: Implement**

Append to `tui/bookmarks.go`:

```go
// updateBookmarkLine rewrites the last-visited date on every valid record for
// target — the write path's first in-place rewrite, so it is careful about
// what it touches: only valid records whose target matches exactly are
// rewritten (all duplicates, consistent with deleteBookmarkLine), each keeps
// its leading whitespace and its trailing comment byte-identical, and
// everything else — comments, malformed lines, blanks, directives, ordering —
// is untouched. changed is false when no record matched; that is also the
// "is it bookmarked?" test for the caller.
func updateBookmarkLine(data []byte, target string, ts time.Time) ([]byte, bool) {
	stamp := ts.UTC().Truncate(time.Second).Format(time.RFC3339)
	lines := strings.Split(string(data), "\n")
	changed := false
	for i, line := range lines {
		parsed, _, err := parseBookmarkTarget(strings.TrimSpace(stripComment(line)))
		if err != nil || parsed != target {
			continue
		}
		rewritten := rewriteBookmarkRecord(line, target, stamp)
		// Round-trip guard: the emitted record must parse back to the same
		// target and instant, or the line is left untouched rather than
		// writing a record the parser would later refuse.
		check, checkTS, err := parseBookmarkTarget(strings.TrimSpace(stripComment(rewritten)))
		want, _ := time.Parse(time.RFC3339, stamp)
		if err != nil || check != target || !checkTS.Equal(want) {
			continue
		}
		lines[i] = rewritten
		changed = true
	}
	if !changed {
		return data, false
	}
	return []byte(strings.Join(lines, "\n")), true
}

// rewriteBookmarkRecord replaces the record on line with target and stamp,
// splicing on raw offsets so the user's own spacing survives: the leading
// whitespace is kept, and everything from the first "#" onward (the gap before
// it included) is copied byte-identical.
func rewriteBookmarkRecord(line, target, stamp string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	rest := line[len(indent):]
	gap, comment := "", ""
	if i := strings.Index(rest, "#"); i >= 0 {
		record := strings.TrimRight(rest[:i], " \t")
		gap = rest[len(record):i]
		comment = rest[i:]
	}
	return indent + target + " " + stamp + gap + comment
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestUpdateBookmarkLine' -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
make check && git add tui/bookmarks.go tui/bookmarks_test.go
git commit -m "feat(bookmarks): rewrite a record's last-visited date in place"
```

---

### Task 3: Stamp the visit on a successful landing

**Files:**
- Modify: `tui/bookmarks.go` (add `nowFn` and `stampVisit`), `tui/app.go` (`landNavigation` ~line 1050, `landRefresh` ~line 1059)
- Test: `tui/app_test.go`

**Interfaces:**
- Consumes: `updateBookmarkLine` (Task 2).
- Produces: `nowFn func() time.Time` package var (the `bookmarksPathFn` pattern) and `stampVisit(raw string)`. `nowFn` is consumed again by Task 4's rendering.

- [ ] **Step 1: Write the failing tests**

Add `useNow` next to the other bookmarks stubs in `tui/bookmarks_test.go` (same package; Task 4's startpage tests need it too), and add `"time"` to that file's imports:

```go
// useNow stubs nowFn for one test, restoring time.Now afterwards.
func useNow(t *testing.T, now time.Time) {
	t.Helper()
	saved := nowFn
	nowFn = func() time.Time { return now }
	t.Cleanup(func() { nowFn = saved })
}
```

Append to `tui/app_test.go` (patterns: `useTempBookmarks`, `hostTarget`, `deliverNavigation`, `deliverRefresh`, `newApp`, `stubFetch` all exist):

```go
func TestLandingStampsABookmarkedTarget(t *testing.T) {
	path := useTempBookmarks(t)
	now := time.Date(2026, 8, 14, 15, 4, 5, 0, time.UTC)
	useNow(t, now)
	if err := os.WriteFile(path, []byte("alice@plan.cat\n@tilde.team\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	deliverNavigation(m, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "alice@plan.cat 2026-08-14T15:04:05Z\n@tilde.team\n"
	if string(data) != want {
		t.Fatalf("bookmarks =\n%q\nwant\n%q", data, want)
	}
}

func TestLandingDoesNotStampAnUnbookmarkedTarget(t *testing.T) {
	path := useTempBookmarks(t)
	useNow(t, time.Date(2026, 8, 14, 15, 4, 5, 0, time.UTC))
	if err := os.WriteFile(path, []byte("@tilde.team\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	deliverNavigation(m, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "@tilde.team\n" {
		t.Fatalf("bookmarks = %q, want untouched for an unpinned target", data)
	}
}

func TestLandingDoesNotStampAnErroredFetch(t *testing.T) {
	path := useTempBookmarks(t)
	useNow(t, time.Date(2026, 8, 14, 15, 4, 5, 0, time.UTC))
	if err := os.WriteFile(path, []byte("alice@plan.cat\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	deliverNavigation(m, Entry{Target: hostTarget(t, "alice@plan.cat"), Err: errors.New("dial: connection refused")})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "alice@plan.cat\n" {
		t.Fatalf("bookmarks = %q, want untouched after a failed fetch", data)
	}
}

func TestLandingStampsOnlyAnExactMatch(t *testing.T) {
	path := useTempBookmarks(t)
	useNow(t, time.Date(2026, 8, 14, 15, 4, 5, 0, time.UTC))
	if err := os.WriteFile(path, []byte("@tilde.team\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	// "tilde.team:79" names the same machine but is a different record string.
	deliverNavigation(m, Entry{Target: hostTarget(t, "tilde.team:79"), Body: []byte("users\n")})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "@tilde.team\n" {
		t.Fatalf("bookmarks = %q, want untouched: matching is exact string equality", data)
	}
}

func TestRefreshStampsTheVisit(t *testing.T) {
	path := useTempBookmarks(t)
	now := time.Date(2026, 8, 14, 15, 4, 5, 0, time.UTC)
	useNow(t, now)
	if err := os.WriteFile(path, []byte("alice@plan.cat\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m = deliverNavigation(m, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")})
	fresh := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi again\n")}
	m = deliverRefresh(m, fresh, false)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "2026-08-14T15:04:05Z") != 1 {
		t.Fatalf("bookmarks = %q, want the refresh to have re-stamped the record", data)
	}
}

func TestStampFailureDoesNotBreakTheLanding(t *testing.T) {
	// A read-only config directory: the atomic write must fail, the failure is
	// swallowed, and the landing completes with the file left stale.
	dir := t.TempDir()
	path := filepath.Join(dir, "bookmarks")
	if err := os.WriteFile(path, []byte("alice@plan.cat\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) }) //nolint:errcheck // so TempDir can clean up
	useBookmarksPath(t, path)
	useNow(t, time.Date(2026, 8, 14, 15, 4, 5, 0, time.UTC))
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	landed := deliverNavigation(m, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")})
	if landed.state != stateReader || landed.pos != 0 {
		t.Fatalf("state=%v pos=%d, want a normal reader landing despite the unwritable file", landed.state, landed.pos)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "alice@plan.cat\n" {
		t.Fatalf("bookmarks = %q, want the stale file left byte-identical", data)
	}
}
```

(`errors`, `os`, `path/filepath`, `strings`, `time` imports as needed in `app_test.go`; `useNow` lives in `bookmarks_test.go` with the other stubs. `TestMain` already isolates the default bookmarks path.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tui/ -run 'TestLanding|TestRefreshStamps|TestStampFailure' -count=1 -v`
Expected: FAIL — `nowFn` undefined (compile error).

- [ ] **Step 3: Implement**

In `tui/bookmarks.go`:

```go
// nowFn is the clock for visit stamps and relative dates, a package var so
// tests control time — the same pattern bookmarksPathFn uses for the path.
var nowFn = time.Now

// stampVisit records a successful landing in the bookmarks file. Every
// failure — no config dir, unreadable file, unwritable filesystem — degrades
// silently: the date simply does not advance, and navigation is never blocked
// by bookkeeping. A target with no record matches nothing, so unpinned
// targets never write the file and a visit never adds a record.
func stampVisit(raw string) {
	path, err := bookmarksPathFn()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	updated, changed := updateBookmarkLine(data, raw, nowFn())
	if !changed {
		return
	}
	_ = saveBookmarkData(path, updated) //nolint:errcheck // a stale date is the designed degradation
}
```

In `tui/app.go`, `landNavigation`:

```go
func (m appModel) landNavigation(entry Entry) appModel {
	m.snapshot()
	if entry.Err == nil {
		stampVisit(entry.Target.Raw)
	}
	routed := routeEntry(entry)
	...
```

and `landRefresh`, immediately after the `entry.failed()` early return:

```go
	if entry.Err == nil {
		stampVisit(entry.Target.Raw)
	}
```

(An errored response that still carries a body routes and displays, but does not stamp — only `Err == nil` counts as a visit. Post-body resets treated as success by `finger.Query` have `Err == nil` and do stamp.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestLanding|TestRefreshStamps|TestStampFailure' -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
make check && git add tui/bookmarks.go tui/bookmarks_test.go tui/app.go tui/app_test.go
git commit -m "feat(tui): stamp a bookmark's last-visited date on a successful landing"
```

---

### Task 4: Render the relative date in the note column

**Files:**
- Modify: `tui/bookmarks.go` (`startEntry` struct ~line 57), `tui/sections.go` (`buildSections` bookmark loop ~line 53), `tui/start.go` (`startRowNote` ~line 488, plus a new `relativeDay`)
- Test: `tui/start_test.go`

**Interfaces:**
- Consumes: `bookmarkFile.visited` (Task 1), `nowFn` (Task 3).
- Produces: `startEntry.visited time.Time` (zero = unknown); `relativeDay(ts, now time.Time) string`.

- [ ] **Step 1: Write the failing tests**

Append to `tui/start_test.go`:

```go
// useLocalZone points time.Local at loc for one test, restoring it afterwards.
func useLocalZone(t *testing.T, loc *time.Location) {
	t.Helper()
	saved := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = saved })
}

func TestRelativeDayBuckets(t *testing.T) {
	useLocalZone(t, time.UTC)
	now := time.Date(2026, 8, 14, 16, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		stamp string
		want  string
	}{
		{"2026-08-14T02:00:00Z", "today"},
		{"2026-08-13T23:00:00Z", "yesterday"},
		{"2026-08-11T16:00:00Z", "3 days ago"},
		{"2026-07-16T16:00:00Z", "29 days ago"},
		{"2026-07-10T16:00:00Z", "1 months ago"},  // 35 days
		{"2026-01-14T16:00:00Z", "7 months ago"},  // 212 days
		{"2024-08-14T16:00:00Z", "2 years ago"},   // 731 days
		{"2026-08-15T16:00:00Z", "today"},         // future clamps to today
	} {
		ts, err := time.Parse(time.RFC3339, tt.stamp)
		if err != nil {
			t.Fatal(err)
		}
		if got := relativeDay(ts, now); got != tt.want {
			t.Errorf("relativeDay(%s) = %q, want %q", tt.stamp, got, tt.want)
		}
	}
}

func TestRelativeDayBucketsInLocalTime(t *testing.T) {
	stamp, _ := time.Parse(time.RFC3339, "2026-08-14T02:30:00Z")
	now, _ := time.Parse(time.RFC3339, "2026-08-14T16:00:00Z")

	useLocalZone(t, time.UTC)
	if got := relativeDay(stamp, now); got != "today" {
		t.Errorf("UTC: relativeDay = %q, want today", got)
	}

	// UTC-8: the stamp is Aug 13 18:30 local, now is Aug 14 08:00 local.
	useLocalZone(t, time.FixedZone("UTC-8", -8*3600))
	if got := relativeDay(stamp, now); got != "yesterday" {
		t.Errorf("UTC-8: relativeDay = %q, want yesterday (buckets are local calendar days)", got)
	}
}

func TestRelativeDayCountsCalendarDaysAcrossDST(t *testing.T) {
	// US spring-forward 2026-03-08: Saturday noon EST → Monday noon EDT is two
	// calendar days but only 47 elapsed hours. Hours/24 would say "yesterday".
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	useLocalZone(t, loc)
	stamp := time.Date(2026, 3, 7, 12, 0, 0, 0, loc)
	now := time.Date(2026, 3, 9, 12, 0, 0, 0, loc)
	if got := relativeDay(stamp, now); got != "2 days ago" {
		t.Errorf("relativeDay across spring-forward = %q, want 2 days ago", got)
	}
}

func TestStartRowNoteShowsTheVisitDate(t *testing.T) {
	useLocalZone(t, time.UTC)
	useNow(t, time.Date(2026, 8, 14, 16, 0, 0, 0, time.UTC))
	visited := time.Date(2026, 8, 11, 16, 0, 0, 0, time.UTC)
	pin := startEntry{target: "@plan.cat", source: sourceBookmark, bookmarked: true, visited: visited}

	if got := startRowNote(pin, false, false); got != "3 days ago" {
		t.Errorf("unselected pinned row = %q, want the date", got)
	}
	if got := startRowNote(pin, true, false); got != "3 days ago" {
		t.Errorf("selected pinned row = %q, want the date (the cursor does not lift the row's text)", got)
	}

	unknown := startEntry{target: "@new.example", source: sourceBookmark, bookmarked: true}
	if got := startRowNote(unknown, false, false); got != "" {
		t.Errorf("unvisited pin = %q, want blank", got)
	}

	// Flattened, the catalog note returns and the date stands down: match
	// offsets are computed against FilterValue (target + note) only when
	// flattened, so the note must be what renders there.
	described := startEntry{target: "@cosmic.voyage", note: "Collaborative science fiction", source: sourceBookmark, bookmarked: true, visited: visited}
	if got := startRowNote(described, false, true); got != "Collaborative science fiction" {
		t.Errorf("flattened pinned row = %q, want the catalog note", got)
	}
	if fv := (startItem{entry: described}).FilterValue(); fv != "@cosmic.voyage Collaborative science fiction" {
		t.Errorf("FilterValue = %q, want target + note, date excluded", fv)
	}
}

// A visited pin still highlights the catalog-note match when flattened —
// the date must not be what splitStartMatches is scoring against.
func TestStartVisitedPinHighlightsTheNoteWhenFlattened(t *testing.T) {
	useLocalZone(t, time.UTC)
	useNow(t, time.Date(2026, 8, 14, 16, 0, 0, 0, time.UTC))
	common := testCommon()
	common.width = 100
	const note = "Collaborative science fiction"
	m := newStart(common, []startSection{
		{id: sectionBookmarks, title: "BOOKMARKS", entries: []startEntry{
			{target: "@cosmic.voyage", kind: kindCommunity, note: note, source: sourceBookmark, bookmarked: true,
				visited: time.Date(2026, 8, 11, 16, 0, 0, 0, time.UTC)},
		}},
	}, "", "")
	m.list.SetFilterText("fiction")
	m.list.SetFilterState(list.Filtering)

	view := m.View()
	plain := stripANSIForLandingTest(view)
	if strings.Contains(plain, "3 days ago") {
		t.Fatalf("flattened visited pin still shows the date:\n%s", plain)
	}
	if !strings.Contains(plain, note) {
		t.Fatalf("flattened visited pin hides its note:\n%s", plain)
	}
	lineIndex := lineIndexContaining(t, plain, "@cosmic.voyage")
	if got := underlinedText(strings.Split(view, "\n")[lineIndex]); got != "fiction" {
		t.Fatalf("underlined text = %q, want %q", got, "fiction")
	}
}

// A shelf mixing a visited and an unvisited pin takes both blank/not-blank
// branches of rowEndsBlank within one section; the header gap must stay at one
// row either way. Companion to TestStartSectionGapAfterASilentPinnedRow.
func TestStartSectionGapAfterAMixedShelf(t *testing.T) {
	useLocalZone(t, time.UTC)
	useNow(t, time.Date(2026, 8, 14, 16, 0, 0, 0, time.UTC))
	visited := time.Date(2026, 8, 11, 16, 0, 0, 0, time.UTC)
	common := testCommon()
	common.width = 40
	m := newStart(common, []startSection{
		{id: sectionBookmarks, title: "BOOKMARKS", entries: []startEntry{
			{target: "@new.example", source: sourceBookmark, bookmarked: true},
			{target: "@plan.cat", source: sourceBookmark, bookmarked: true, visited: visited},
		}},
		{id: sectionCommunities, title: "COMMUNITIES", entries: []startEntry{
			{target: "@graph.no", kind: kindCommunity, note: "Weather worldwide by place name", source: sourceCatalog},
		}},
	}, "", "")

	plain := stripANSIForLandingTest(m.View())
	lines := strings.Split(plain, "\n")
	header := lineIndexContaining(t, plain, "COMMUNITIES")
	if header < 2 {
		t.Fatalf("COMMUNITIES line = %d, want room for content and a gap:\n%s", header, plain)
	}
	if got := strings.TrimSpace(lines[header-1]); got != "" {
		t.Fatalf("line before COMMUNITIES = %q, want blank:\n%s", got, plain)
	}
	if got := strings.TrimSpace(lines[header-2]); got == "" {
		t.Fatalf("two blank lines before COMMUNITIES, want exactly one:\n%s", plain)
	}
}
```

(Add `"time"` to `tui/start.go` and `tui/start_test.go` imports. `useNow` lives in `bookmarks_test.go` from Task 3. `underlinedText` already exists in this file.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tui/ -run 'TestRelativeDay|TestStartRowNoteShowsTheVisitDate|TestStartVisitedPinHighlights|TestStartSectionGapAfterAMixedShelf' -count=1 -v`
Expected: FAIL — `relativeDay` / `startEntry.visited` undefined.

- [ ] **Step 3: Implement**

In `tui/bookmarks.go`, extend `startEntry` (its doc comment gains a line):

```go
type startEntry struct {
	target string
	kind   entryKind
	note   string
	source entrySource

	visited time.Time // last-visited instant from the bookmarks file; zero = unknown

	child      bool // ...
```

In `tui/sections.go`, `buildSections`, in the bookmark loop:

```go
		for _, target := range bm.targets {
			e := startEntry{target: target, source: sourceBookmark}
			if catalogEntry, ok := byTarget[target]; ok {
				e = catalogEntry
				e.source = sourceBookmark
			}
			e.bookmarked = true
			e.visited = bm.visited[target] // zero when unknown — the row renders blank
			bookmarked = append(bookmarked, e)
		}
```

In `tui/start.go`, add `relativeDay`:

```go
// relativeDay renders a last-visited instant relative to now, fuzzier the
// further back it goes. Buckets are calendar-day differences in the user's
// local zone, not divisions of an elapsed duration: bucketing in UTC would
// tell a user in UTC-8 that an evening visit happened "today" all through the
// following morning. A future stamp (clock skew, hand-edit) clamps to today.
func relativeDay(ts, now time.Time) string {
	t, n := ts.In(time.Local), now.In(time.Local)
	if t.After(n) {
		return "today"
	}
	// Walk local midnights with AddDate. Dividing elapsed hours by 24 is
	// wrong across DST: a spring-forward Saturday→Monday is 47 hours and
	// would land in the "yesterday" bucket.
	ty, tm, td := t.Date()
	ny, nm, nd := n.Date()
	cursor := time.Date(ty, tm, td, 0, 0, 0, 0, t.Location())
	end := time.Date(ny, nm, nd, 0, 0, 0, 0, n.Location())
	days := 0
	for cursor.Before(end) {
		cursor = cursor.AddDate(0, 0, 1)
		days++
	}
	switch {
	case days == 0:
		return "today"
	case days == 1:
		return "yesterday"
	case days < 30:
		return fmt.Sprintf("%d days ago", days)
	}
	if months := days / 30; months < 12 {
		return fmt.Sprintf("%d months ago", months)
	}
	return fmt.Sprintf("%d years ago", days/365)
}
```

Then rewire the bookmark branch of `startRowNote`, and update its doc comment so the "silent" paragraph describes the new rule (a pinned row's note column holds its last-visited date; the row is silent only while the date is unknown — the cursor never lifts an unvisited row's blank, and a flattening filter always restores the catalog note so `/` stays honest):

```go
func startRowNote(entry startEntry, selected, flattened bool) string {
	if flattened {
		return entry.note
	}
	if entry.source == sourceBookmark {
		if entry.visited.IsZero() {
			return ""
		}
		return relativeDay(entry.visited, nowFn())
	}
	if entry.child && !selected {
		return ""
	}
	return entry.note
}
```

Nothing else moves: the date lives in the note column, so `startTargetColumn` measurement, `FilterValue`, and `splitStartMatches` are all untouched by design — `rowEndsBlank` delegates to `startRowNote`, so the section-gap behaviour follows automatically.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestRelativeDay|TestStartRowNote|TestStartVisitedPinHighlights|TestStartSectionGap|TestStartOwnership' -count=1 -v`
Expected: PASS, including the pre-existing #112 note-suppression tests.

- [ ] **Step 5: Commit**

```bash
make check && git add tui/bookmarks.go tui/sections.go tui/start.go tui/start_test.go
git commit -m "feat(startpage): show a bookmark's last-visited date in the note column"
```

---

### Task 5: Update the ingress documentation

**Files:**
- Modify: `CLAUDE.md` (`AGENTS.md` is a symlink to it — one edit covers both)

**Interfaces:** none (docs only).

- [ ] **Step 1: Rewrite the bookmarks-ingress paragraph**

In `CLAUDE.md`, find the paragraph beginning "**A second ingress exists as of the startpage:** the bookmarks file (`tui/bookmarks.go`). It admits target-only records:" and rewrite it to describe the new contract, keeping the paragraph's structure and voice:

- records are now `<target> [<RFC3339 UTC last-visited date>]`; one field means date unknown;
- a malformed date is refused and reported as a problem, never silently dropped — so only a validated `time.Time` is ever displayed and the file still needs no `sanitize` call;
- the write path gains `updateBookmarkLine`, its first in-place rewrite: byte-offset splicing preserves comments/spacing/ordering, a round-trip guard refuses to emit a record the parser would reject, and writes happen only for already-bookmarked targets on successful landings, degrading silently on failure;
- the seeded `jonathan@tilde.team` first-run bookmark is unchanged.

- [ ] **Step 2: Update the startpage paragraph**

In the same file's TUI-internals section, the startpage bullet describes the note column. Add one sentence: a pinned row's note column shows its relative last-visited date (`today` / `yesterday` / `N days|months|years ago`, local-zone day buckets), blank when unknown, standing down to the catalog note when a filter flattens the view.

- [ ] **Step 3: Verify and commit**

```bash
make check && git add CLAUDE.md
git commit -m "docs: record the bookmarks file's last-visited grammar and rewrite path"
```

---

## Self-review notes (completed by the plan author)

- **Spec coverage:** grammar/parse (Task 1), rewrite + guard + byte splicing + exact-match (Task 2), stamping rules incl. refresh and silent failure (Task 3), rendering incl. local-zone buckets, future clamp, flattened stand-down, FilterValue exclusion, mixed-shelf gap (Task 4), docs (Task 5). The unknown-date scenario is covered by parse (one-field), render (zero time → blank), and stamp-failure tests.
- **Out of scope per spec:** recency ordering of the shelf; `docs/tui-review` fixtures stay dateless.
- **Type consistency:** `parseBookmarkTarget` returns `(string, time.Time, error)` in every task; `updateBookmarkLine(data []byte, target string, ts time.Time) ([]byte, bool)`; `relativeDay(ts, now time.Time) string`; `nowFn` introduced in Task 3 and consumed in Task 4 (`useNow` lives in `bookmarks_test.go`).
- **Plan fixes (2026-08-14 review):** parse requires the UTC `Z` form (not any RFC 3339); last-line-wins includes a trailing dateless duplicate; `relativeDay` walks calendar midnights so DST cannot collapse a 2-day span to "yesterday"; Task 3 refresh test uses the existing `deliverRefresh` helper; `useNow` is shared from `bookmarks_test.go`.

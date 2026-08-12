# Default Author Bookmark Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Initialize a missing bookmarks file with jonathan@tilde.team exactly once, while treating every existing file as authoritative so removing the line is permanent.

**Architecture:** Keep first-file initialization inside loadBookmarks, the existing disk-ingress boundary. Build the seed with appendBookmarkLine(nil, aboutFingerAuthor), persist it through saveBookmarkData, and parse the same bytes only after a successful write; existing files continue down the unchanged read-and-parse path.

**Tech Stack:** Go 1.26 toolchain, standard-library filesystem APIs, the existing tui bookmark parser/writer, Go testing, and Markdown.

**Spec:** docs/superpowers/specs/2026-08-12-default-author-bookmark-design.md

## Global Constraints

- Seed only when os.ReadFile reports that the resolved bookmarks path does not exist.
- Any existing file is authoritative, including empty, comment-only, and catalog-off-only files.
- Reuse aboutFingerAuthor; do not duplicate the author target in production code.
- Generate exact seed bytes with appendBookmarkLine(nil, aboutFingerAuthor).
- Persist through saveBookmarkData so the new lookit directory is 0700 and the file is 0600.
- On initialization failure, return zero targets and one line-zero problem beginning with cannot create:; never synthesize an in-memory bookmark.
- Do not change bookmark grammar, catalog data, section assembly, routing, or toggle behavior.
- Tests stay offline and never access the developer's real bookmarks.
- Preserve pre-existing dirty-worktree changes.
- Do not commit, push, or open a PR without a separate explicit request.

## File Map

- tui/bookmarks_test.go — isolate normal tests behind existing empty files and cover initialization, authoritative files, deletion persistence, permissions, and errors.
- tui/bookmarks.go — initialize only the missing-file branch and update the stale loadBookmarks and saveBookmarkData contract comments.
- README.md — explain the starter bookmark and removal semantics.
- CLAUDE.md and AGENTS.md — keep mirrored architecture guidance current.
- docs/superpowers/specs/2026-08-12-default-author-bookmark-design.md — approved design; no further behavior changes.
- docs/superpowers/plans/2026-08-12-default-author-bookmark.md — this execution plan.

---

### Task 1: First-file initialization and regression coverage

**Files:**
- Modify: tui/bookmarks_test.go:9-38,234-244
- Modify: tui/bookmarks.go:233-251,276-277

**Interfaces:**
- Consumes: aboutFingerAuthor string; appendBookmarkLine([]byte, string) []byte; saveBookmarkData(string, []byte) error; parseBookmarks([]byte) bookmarkFile.
- Produces: unchanged loadBookmarks() (bookmarkFile, string) with first-file initialization; test helpers useBookmarksPath and useMissingTempBookmarks.

- [ ] **Step 1: Make the package-wide fixture an existing empty file**

Replace TestMain's shared missing path with a shared existing empty file:

~~~go
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lookit-tui-test-*")
	if err != nil {
		panic(err)
	}
	path := filepath.Join(dir, "bookmarks")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		panic(err)
	}
	bookmarksPathFn = func() (string, error) { return path, nil }
	code := m.Run()
	os.RemoveAll(dir) //nolint:errcheck // best-effort cleanup
	os.Exit(code)
}
~~~

Update the comment: the existing empty file prevents ordinary tests from triggering initialization while isolating them from real config.

- [ ] **Step 2: Make ordinary per-test paths existing and empty**

Split path injection from creation. useTempBookmarks creates an empty file; useMissingTempBookmarks deliberately does not:

~~~go
func useBookmarksPath(t *testing.T, path string) {
	t.Helper()
	saved := bookmarksPathFn
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = saved })
}

func useTempBookmarks(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lookit", "bookmarks")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create bookmark fixture directory: %v", err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("create empty bookmark fixture: %v", err)
	}
	useBookmarksPath(t, path)
	return path
}

func useMissingTempBookmarks(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lookit", "bookmarks")
	useBookmarksPath(t, path)
	return path
}
~~~

Document both helpers. Existing tests continue using useTempBookmarks; only initialization tests use useMissingTempBookmarks.

- [ ] **Step 3: Verify the fixture-only change**

Run:

~~~bash
go test ./tui/ -count=1
~~~

Expected: PASS. No production behavior has changed.

- [ ] **Step 4: Replace the obsolete missing-file test with a failing initialization test**

Replace TestLoadBookmarksMissingFileIsNotAnError:

~~~go
func TestLoadBookmarksMissingFileCreatesAuthorBookmark(t *testing.T) {
	path := useMissingTempBookmarks(t)

	file, gotPath := loadBookmarks()
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if len(file.problems) != 0 {
		t.Fatalf("problems = %+v, want none", file.problems)
	}
	if len(file.targets) != 1 || file.targets[0] != aboutFingerAuthor {
		t.Fatalf("targets = %+v, want [%s]", file.targets, aboutFingerAuthor)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read initialized bookmarks: %v", err)
	}
	if got, want := string(data), aboutFingerAuthor+"\n"; got != want {
		t.Fatalf("bookmarks = %q, want %q", got, want)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat initialized bookmarks: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("bookmarks mode = %o, want 600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat initialized directory: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("bookmark directory mode = %o, want 700", got)
	}
}
~~~

- [ ] **Step 5: Add failing deletion-persistence and creation-failure tests**

Add strings to the imports, then add:

~~~go
func TestLoadBookmarksDoesNotRestoreDeletedAuthorBookmark(t *testing.T) {
	path := useMissingTempBookmarks(t)
	file, _ := loadBookmarks()
	if len(file.targets) != 1 || file.targets[0] != aboutFingerAuthor {
		t.Fatalf("initial targets = %+v, want [%s]", file.targets, aboutFingerAuthor)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read initialized bookmarks: %v", err)
	}
	if err := saveBookmarkData(path, deleteBookmarkLine(data, aboutFingerAuthor)); err != nil {
		t.Fatalf("remove author bookmark: %v", err)
	}

	file, _ = loadBookmarks()
	if len(file.targets) != 0 || len(file.problems) != 0 {
		t.Fatalf("file after removal = %+v, want empty", file)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bookmarks after removal: %v", err)
	}
	// deleteBookmarkLine keeps the trailing empty split segment, so the file
	// becomes a lone newline rather than zero bytes. That existing file is
	// still authoritative.
	if got, want := string(data), "\n"; got != want {
		t.Fatalf("bookmarks after removal = %q, want %q", got, want)
	}
}

func TestLoadBookmarksReportsDefaultCreationFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatalf("lock fixture directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(root, 0o700); err != nil {
			t.Errorf("restore fixture directory: %v", err)
		}
	})
	path := filepath.Join(root, "bookmarks")
	useBookmarksPath(t, path)

	file, gotPath := loadBookmarks()
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if len(file.targets) != 0 {
		t.Fatalf("targets = %+v, want none", file.targets)
	}
	if len(file.problems) != 1 || file.problems[0].line != 0 ||
		!strings.HasPrefix(file.problems[0].reason, "cannot create: ") {
		t.Fatalf("problems = %+v, want one line-zero cannot-create problem", file.problems)
	}
}
~~~

A regular file used as a parent yields ENOTDIR from os.ReadFile, which is not os.IsNotExist, so loadBookmarks reports cannot read: and never reaches saveBookmarkData. A missing child in a read-only directory is the Unix/macOS fixture that actually exercises cannot create:. Restore writability in t.Cleanup so t.TempDir can delete the tree. Do not add a production injection hook only for this test.

- [ ] **Step 6: Add regression tests for authoritative files and resolver errors**

Add errors to the imports, then add:

~~~go
func TestLoadBookmarksNeverRewritesExistingFile(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "empty", data: ""},
		{name: "author absent", data: "@plan.cat\n"},
		{name: "author present", data: aboutFingerAuthor + "\n@plan.cat\n"},
		{name: "comments only", data: "# deliberately empty\n\n"},
		{name: "catalog off only", data: "catalog off\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := useTempBookmarks(t)
			if err := os.WriteFile(path, []byte(tt.data), 0o600); err != nil {
				t.Fatalf("seed bookmarks: %v", err)
			}
			loadBookmarks()
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read bookmarks: %v", err)
			}
			if string(got) != tt.data {
				t.Fatalf("bookmarks = %q, want unchanged %q", got, tt.data)
			}
		})
	}
}

func TestLoadBookmarksPathResolutionFailureIsUnchanged(t *testing.T) {
	saved := bookmarksPathFn
	bookmarksPathFn = func() (string, error) { return "", errors.New("no home") }
	t.Cleanup(func() { bookmarksPathFn = saved })

	file, path := loadBookmarks()
	if path != "" || len(file.targets) != 0 || len(file.problems) != 1 {
		t.Fatalf("file = %+v, path = %q", file, path)
	}
	if got, want := file.problems[0].reason, "cannot locate a config directory: no home"; got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
}
~~~

- [ ] **Step 7: Run focused tests and verify RED**

Run:

~~~bash
go test ./tui/ -run 'TestLoadBookmarks(MissingFileCreatesAuthorBookmark|DoesNotRestoreDeletedAuthorBookmark|ReportsDefaultCreationFailure|NeverRewritesExistingFile|PathResolutionFailureIsUnchanged)$' -count=1 -v
~~~

Expected: FAIL because missing-file loading returns no target and creates no file. The existing-file and resolver cases already pass.

- [ ] **Step 8: Implement the minimal missing-file branch**

Update loadBookmarks and its stale comment. Also drop the now-false "Reading never creates anything" sentence from the saveBookmarkData comment:

~~~go
// saveBookmarkData writes atomically (temp file + rename) at 0600, creating the
// directory 0700 if needed.
~~~

~~~go
// loadBookmarks reads and parses the user's file. On first use it initializes a
// missing file with the author bookmark; every existing file, including an empty
// one, is authoritative. Read and initialization failures become problems the
// startpage surfaces. The resolved path is returned so every message can name
// the file actually in use.
func loadBookmarks() (bookmarkFile, string) {
	path, err := bookmarksPathFn()
	if err != nil {
		return bookmarkFile{problems: []parseProblem{{reason: "cannot locate a config directory: " + err.Error()}}}, ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			data = appendBookmarkLine(nil, aboutFingerAuthor)
			if err := saveBookmarkData(path, data); err != nil {
				return bookmarkFile{problems: []parseProblem{{reason: "cannot create: " + err.Error()}}}, path
			}
			return parseBookmarks(data), path
		}
		return bookmarkFile{problems: []parseProblem{{reason: "cannot read: " + err.Error()}}}, path
	}
	return parseBookmarks(data), path
}
~~~

- [ ] **Step 9: Run focused tests and verify GREEN**

Repeat Step 7's command.

Expected: PASS with all named tests green and no warnings.

- [ ] **Step 10: Run the whole TUI package**

Run:

~~~bash
go test ./tui/ -count=1
~~~

Expected: PASS. If an exact bookmark assertion contains the author seed, that test bypasses useTempBookmarks; repair the fixture rather than changing expected application behavior.

---

### Task 2: User and maintainer documentation

**Files:**
- Modify: README.md:50
- Modify: CLAUDE.md:31
- Modify: AGENTS.md:31

**Interfaces:**
- Consumes: loadBookmarks behavior from Task 1.
- Produces: exact user-facing removal guidance and matching architecture guidance.

- [ ] **Step 1: Update README.md**

Add this behavior to the bookmark paragraph:

~~~markdown
On first use, if that file does not exist, lookit creates it with jonathan@tilde.team as a starter bookmark. Remove that line (or empty the file) to remove it permanently; deleting the file itself makes lookit initialize it again next time.
~~~

Render the target as inline code in the actual Markdown. Do not say deletion is permanent without qualifying that the file remains.

- [ ] **Step 2: Update mirrored architecture guidance**

In both CLAUDE.md and AGENTS.md, add the same text to the bookmarks-ingress paragraph after its metadata sentence:

~~~markdown
On first load when the bookmarks file is absent, loadBookmarks atomically creates it with jonathan@tilde.team; any existing file, including an empty one, is authoritative, so removing the line does not resurrect it. This is a narrow exception to the original no-seeded-defaults decision: it is an ordinary unclassified bookmark, not a catalog person entry.
~~~

Render identifiers and the target as inline code in the actual Markdown. Keep the two files byte-for-byte aligned for this guidance.

- [ ] **Step 3: Check the documentation diff**

Run:

~~~bash
git diff --check
git diff -- README.md CLAUDE.md AGENTS.md
~~~

Expected: no whitespace errors; README distinguishes line removal from file deletion; both architecture files state identical behavior. This task does not edit the design or plan files.

---

### Task 3: Final verification and scope audit

**Files:**
- Verify only; do not add unrelated changes.

**Interfaces:**
- Consumes: Tasks 1-2.
- Produces: fresh CI-equivalent evidence and an edit-only handoff.

- [ ] **Step 1: Run the full repository gate**

Run:

~~~bash
make check
~~~

Expected: exit 0 from go vet, the gofmt emptiness check, golangci-lint, and go test with the race detector.

- [ ] **Step 2: Audit the final diff**

Run:

~~~bash
git status --short
git diff --check
git diff -- tui/bookmarks.go tui/bookmarks_test.go README.md CLAUDE.md AGENTS.md docs/superpowers/specs/2026-08-12-default-author-bookmark-design.md docs/superpowers/plans/2026-08-12-default-author-bookmark.md
~~~

Expected: feature changes are limited to listed files. Pre-existing modifications to docs/superpowers/plans/2026-08-12-startpage-entry-grouping.md and docs/superpowers/specs/2026-08-12-startpage-entry-grouping-design.md remain untouched and are called out separately.

- [ ] **Step 3: Report the edit-only handoff**

Summarize the seed, authoritative-existing-file behavior, tests, and fresh make check result. State that no commit, push, or PR was created.


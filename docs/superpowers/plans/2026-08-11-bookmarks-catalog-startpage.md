# Bookmarks & Catalog Startpage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace lookit's empty landing screen with a browsable startpage that renders an embedded curated catalog plus the user's own bookmarks as one sectioned list.

**Architecture:** Two sources parse into one `startEntry` type — a catalog embedded with `go:embed` (read-only, carries notes) and a line-oriented bookmarks file the user owns (no notes, machine-appended). A new `startModel` wraps a real `bubbles/v2/list` whose section headers are uniform-height items the cursor steps over. A fourth `appState` (`stateStart`) renders it at `pos == -1`; history semantics are untouched.

**Tech Stack:** Go 1.26, `charm.land/bubbletea/v2`, `charm.land/bubbles/v2` (list, key, textinput), `charm.land/lipgloss/v2`, stdlib `embed`/`os`/`strings`. No new dependencies.

**Spec:** [`docs/superpowers/specs/2026-08-11-bookmarks-catalog-startpage-design.md`](../specs/2026-08-11-bookmarks-catalog-startpage-design.md)

## Global Constraints

- **No new dependencies.** The bookmarks format is hand-parsed precisely to avoid adding a TOML/YAML library to `THIRD_PARTY_NOTICES.md`.
- **Commit messages: Conventional Commits.** No `Co-Authored-By`, no "Generated with Claude Code", no AI trailers anywhere — commits, PR bodies, or issue bodies.
- **`make check` must pass** before any commit: `go vet ./...`, `gofmt -l .` empty, `golangci-lint run ./...`, `go test ./... -race`.
- **No real network in tests.** Use `stubFetch(t)` (`tui/app_test.go:20`). File I/O goes in `t.TempDir()`.
- **Pair every colour with a light/dark value** via the existing `styles`/`palette` structs. Never hardcode a hex in a view.
- **Avoid Alt/Option chords.** macOS is the primary target.
- **UI copy is honest.** Don't assert structure the finger protocol lacks. Label uncertainty consequence-first.
- **`kind` is display grouping only, never routing.** `routeEntry` decides list-vs-reader from the real response.
- **Package layout is one-way:** `finger/` → `render/` → `tui/`. All new code is in `tui/`.

---

### Task 1: Bookmarks grammar (pure parser)

The grammar shared by both files: `<kind> <target> [note]`. Bookmark lines refuse the note field — that refusal is what keeps free text out of the terminal, so it is a correctness requirement, not parser trivia.

**Files:**
- Create: `tui/bookmarks.go`
- Create: `tui/bookmarks_test.go`

**Interfaces:**
- Consumes: `finger.ParseTarget` (`finger/query.go:59`)
- Produces: `entryKind`, `entrySource`, `startEntry`, `parseProblem`, `bookmarkFile`, `parseBookmarks([]byte) bookmarkFile`, `parseCatalogData([]byte) ([]startEntry, []parseProblem)`

- [ ] **Step 1: Write the failing test**

```go
package tui

import "testing"

func TestParseBookmarksValidLines(t *testing.T) {
	in := []byte("# my list\n\ncommunity  @tilde.team\nperson     jonathan@tilde.team\nservice    weather@bbs.airandwave.net\n")
	got := parseBookmarks(in)
	if len(got.problems) != 0 {
		t.Fatalf("problems = %+v, want none", got.problems)
	}
	if got.catalogHidden {
		t.Fatal("catalogHidden = true, want false")
	}
	want := []startEntry{
		{target: "@tilde.team", kind: kindCommunity, source: sourceBookmark},
		{target: "jonathan@tilde.team", kind: kindPerson, source: sourceBookmark},
		{target: "weather@bbs.airandwave.net", kind: kindService, source: sourceBookmark},
	}
	if len(got.entries) != len(want) {
		t.Fatalf("entries = %+v, want %d", got.entries, len(want))
	}
	for i, w := range want {
		if got.entries[i] != w {
			t.Errorf("entry %d = %+v, want %+v", i, got.entries[i], w)
		}
	}
}

func TestParseBookmarksCatalogOff(t *testing.T) {
	got := parseBookmarks([]byte("catalog off\ncommunity @plan.cat\n"))
	if !got.catalogHidden {
		t.Fatal("catalogHidden = false, want true")
	}
	if len(got.entries) != 1 {
		t.Fatalf("entries = %+v, want 1", got.entries)
	}
	if len(got.problems) != 0 {
		t.Fatalf("problems = %+v, want none", got.problems)
	}
}

func TestParseBookmarksRejects(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{name: "unknown kind", line: "wombat @plan.cat"},
		{name: "missing target", line: "community"},
		{name: "unparseable target", line: "community notatarget"},
		{name: "trailing note", line: "community @plan.cat Big friendly pubnix"},
		{name: "bidi override in target", line: "community @plan\u202ecat.example"},
		{name: "unknown directive", line: "catalog maybe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBookmarks([]byte(tt.line + "\n"))
			if len(got.entries) != 0 {
				t.Fatalf("entries = %+v, want none", got.entries)
			}
			if len(got.problems) != 1 {
				t.Fatalf("problems = %+v, want exactly 1", got.problems)
			}
			if got.problems[0].line != 1 {
				t.Errorf("problem line = %d, want 1", got.problems[0].line)
			}
		})
	}
}

func TestParseCatalogAllowsNotes(t *testing.T) {
	entries, problems := parseCatalogData([]byte("community @tilde.team Big, friendly pubnix\n"))
	if len(problems) != 0 {
		t.Fatalf("problems = %+v, want none", problems)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %+v, want 1", entries)
	}
	want := startEntry{target: "@tilde.team", kind: kindCommunity, note: "Big, friendly pubnix", source: sourceCatalog}
	if entries[0] != want {
		t.Errorf("entry = %+v, want %+v", entries[0], want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run 'TestParseBookmarks|TestParseCatalog' -count=1`
Expected: FAIL — `undefined: parseBookmarks`

- [ ] **Step 3: Write minimal implementation**

```go
package tui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/jonathandeamer/lookit/finger"
)

// entryKind groups a startpage entry under a section heading. It is a DISPLAY
// grouping only: what a target actually returns is decided by routeEntry from
// the real response, never asserted here.
type entryKind uint8

const (
	kindCommunity entryKind = iota
	kindService
	kindPerson
)

func (k entryKind) String() string {
	switch k {
	case kindCommunity:
		return "community"
	case kindService:
		return "service"
	default:
		return "person"
	}
}

func parseKind(s string) (entryKind, bool) {
	switch s {
	case "community":
		return kindCommunity, true
	case "service":
		return kindService, true
	case "person":
		return kindPerson, true
	}
	return 0, false
}

// entrySource records which file an entry came from: it decides section
// placement and whether 'b' adds or removes.
type entrySource uint8

const (
	sourceCatalog entrySource = iota
	sourceBookmark
)

// startEntry is one row on the startpage, from either source.
type startEntry struct {
	target string
	kind   entryKind
	note   string // catalog only; always empty for a bookmark
	source entrySource
}

// parseProblem is a line we refused, surfaced to the user rather than swallowed.
type parseProblem struct {
	line   int
	reason string
}

// bookmarkFile is the parsed user file.
type bookmarkFile struct {
	entries       []startEntry
	catalogHidden bool
	problems      []parseProblem
}

func parseBookmarks(data []byte) bookmarkFile {
	var out bookmarkFile
	for i, raw := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if fields[0] == "catalog" {
			switch {
			case len(fields) == 2 && fields[1] == "off":
				out.catalogHidden = true
			case len(fields) == 2 && fields[1] == "on":
				out.catalogHidden = false
			default:
				out.problems = append(out.problems, parseProblem{
					line:   lineNo,
					reason: `expected "catalog off" or "catalog on"`,
				})
			}
			continue
		}
		entry, err := parseEntryLine(line, false, sourceBookmark)
		if err != nil {
			out.problems = append(out.problems, parseProblem{line: lineNo, reason: err.Error()})
			continue
		}
		out.entries = append(out.entries, entry)
	}
	return out
}

func parseCatalogData(data []byte) ([]startEntry, []parseProblem) {
	var entries []startEntry
	var problems []parseProblem
	for i, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		entry, err := parseEntryLine(line, true, sourceCatalog)
		if err != nil {
			problems = append(problems, parseProblem{line: i + 1, reason: err.Error()})
			continue
		}
		entries = append(entries, entry)
	}
	return entries, problems
}

// stripComment drops a trailing "#" comment. A "#" only starts a comment at the
// start of a field, so it cannot appear inside a target or note we accept.
func stripComment(line string) string {
	if i := strings.Index(line, "#"); i >= 0 {
		return line[:i]
	}
	return line
}

// parseEntryLine parses "<kind> <target> [note]". allowNote gates the third
// field: the catalog carries notes, the bookmarks file refuses them, because a
// note is free text and every displayed byte must be validated or ours.
func parseEntryLine(line string, allowNote bool, src entrySource) (startEntry, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return startEntry{}, fmt.Errorf("expected \"<kind> <target>\", got %q", line)
	}
	kind, ok := parseKind(fields[0])
	if !ok {
		return startEntry{}, fmt.Errorf("unknown kind %q (want community, service or person)", fields[0])
	}
	target := fields[1]
	if err := validateTarget(target); err != nil {
		return startEntry{}, err
	}

	var note string
	if rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line[strings.Index(line, target)+len(target):]), " ")); rest != "" {
		if !allowNote {
			return startEntry{}, fmt.Errorf("unexpected text after target: %q (notes are catalog-only)", rest)
		}
		note = rest
	}
	return startEntry{target: target, kind: kind, note: note, source: src}, nil
}

// validateTarget screens a target from a config file. finger.ParseTarget rejects
// C0/DEL via hasControl, but not the non-printing Unicode format controls that
// sanitize visualizes in response bodies — and a target is displayed in the
// breadcrumb, so a bidi override could misrepresent the host being fingered.
// Rejecting matches the treatment targets already get: bodies are visualized
// because they are content, a target is refused because it is something we send.
// See issue #49 for the same gap on targets from every other source.
func validateTarget(target string) error {
	if hasFormatControl(target) {
		return fmt.Errorf("target contains a non-printing Unicode control")
	}
	if _, err := finger.ParseTarget(target); err != nil {
		return fmt.Errorf("bad target %q: %w", target, err)
	}
	return nil
}

func hasFormatControl(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestParseBookmarks|TestParseCatalog' -count=1 -v`
Expected: PASS, all subtests

- [ ] **Step 5: Run the full gate and commit**

```bash
make check
git add tui/bookmarks.go tui/bookmarks_test.go
git commit -m "feat(tui): parse the bookmarks and catalog line grammar"
```

---

### Task 2: Bookmarks file I/O — path resolution and line surgery

Writes must preserve the user's comments, blank lines, ordering and the `catalog off` directive byte-for-byte. That is why the format is line-oriented, so the write path must be line surgery, never re-marshalling.

**Files:**
- Modify: `tui/bookmarks.go`
- Modify: `tui/bookmarks_test.go`

**Interfaces:**
- Consumes: `parseBookmarks`, `startEntry`, `entryKind.String()` from Task 1
- Produces: `bookmarksPathFn` (stubbable package var), `resolveBookmarksPath() (string, error)`, `loadBookmarks() (bookmarkFile, string)`, `appendBookmarkLine([]byte, startEntry) []byte`, `deleteBookmarkLine([]byte, string) []byte`, `saveBookmarkData(string, []byte) error`, `shortenHome(string) string`

- [ ] **Step 1: Write the failing test**

```go
func TestResolveBookmarksPathPrefersXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-example")
	got, err := resolveBookmarksPath()
	if err != nil {
		t.Fatalf("resolveBookmarksPath() error = %v", err)
	}
	if want := "/tmp/xdg-example/lookit/bookmarks"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestResolveBookmarksPathFallsBackToDotConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/tmp/home-example")
	got, err := resolveBookmarksPath()
	if err != nil {
		t.Fatalf("resolveBookmarksPath() error = %v", err)
	}
	if want := "/tmp/home-example/.config/lookit/bookmarks"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestAppendBookmarkLinePreservesFile(t *testing.T) {
	in := []byte("# my careful notes\n\ncatalog off\n\ncommunity  @plan.cat\n")
	got := string(appendBookmarkLine(in, startEntry{target: "@tilde.team", kind: kindCommunity}))
	want := "# my careful notes\n\ncatalog off\n\ncommunity  @plan.cat\ncommunity @tilde.team\n"
	if got != want {
		t.Fatalf("appendBookmarkLine =\n%q\nwant\n%q", got, want)
	}
}

func TestAppendBookmarkLineToEmptyFile(t *testing.T) {
	got := string(appendBookmarkLine(nil, startEntry{target: "@plan.cat", kind: kindCommunity}))
	if want := "community @plan.cat\n"; got != want {
		t.Fatalf("appendBookmarkLine = %q, want %q", got, want)
	}
}

func TestDeleteBookmarkLinePreservesEverythingElse(t *testing.T) {
	in := []byte("# keep me\ncatalog off\ncommunity  @plan.cat\nperson jonathan@tilde.team\n\n# and me\n")
	got := string(deleteBookmarkLine(in, "@plan.cat"))
	want := "# keep me\ncatalog off\nperson jonathan@tilde.team\n\n# and me\n"
	if got != want {
		t.Fatalf("deleteBookmarkLine =\n%q\nwant\n%q", got, want)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lookit", "bookmarks")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = resolveBookmarksPath })

	if err := saveBookmarkData(path, []byte("community @plan.cat\n")); err != nil {
		t.Fatalf("saveBookmarkData() error = %v", err)
	}
	file, gotPath := loadBookmarks()
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if len(file.entries) != 1 || file.entries[0].target != "@plan.cat" {
		t.Fatalf("entries = %+v", file.entries)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %o, want 600", perm)
	}
}

func TestLoadBookmarksMissingFileIsNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = resolveBookmarksPath })

	file, gotPath := loadBookmarks()
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if len(file.entries) != 0 || len(file.problems) != 0 {
		t.Fatalf("file = %+v, want empty", file)
	}
}
```

Add `"os"`, `"path/filepath"` to the test file's imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run 'TestResolveBookmarks|TestAppendBookmark|TestDeleteBookmark|TestSaveAndLoad|TestLoadBookmarks' -count=1`
Expected: FAIL — `undefined: resolveBookmarksPath`

- [ ] **Step 3: Write minimal implementation**

Append to `tui/bookmarks.go` (add `"os"`, `"path/filepath"` to imports):

```go
// bookmarksPathFn resolves the active bookmarks path. It is a package var so
// tests can stub it, the same pattern main.go uses for startTUI.
var bookmarksPathFn = resolveBookmarksPath

// resolveBookmarksPath honours $XDG_CONFIG_HOME, falling back to ~/.config.
// Deliberately NOT os.UserConfigDir(), which on macOS resolves to
// ~/Library/Application Support and would bury a file meant to be hand-edited.
func resolveBookmarksPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "lookit", "bookmarks"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lookit", "bookmarks"), nil
}

// loadBookmarks reads and parses the user's file. It never creates anything: a
// missing file is the normal first run and yields an empty result. An unreadable
// file yields a problem the startpage surfaces. The resolved path is returned so
// every message can name the file actually in use.
func loadBookmarks() (bookmarkFile, string) {
	path, err := bookmarksPathFn()
	if err != nil {
		return bookmarkFile{problems: []parseProblem{{reason: "cannot locate a config directory: " + err.Error()}}}, ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return bookmarkFile{}, path
		}
		return bookmarkFile{problems: []parseProblem{{reason: "cannot read: " + err.Error()}}}, path
	}
	return parseBookmarks(data), path
}

// appendBookmarkLine adds one record, leaving every existing byte untouched.
func appendBookmarkLine(data []byte, e startEntry) []byte {
	out := string(data)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return []byte(out + e.kind.String() + " " + e.target + "\n")
}

// deleteBookmarkLine drops every record for target, leaving comments, blank
// lines, directives and ordering exactly as they were.
func deleteBookmarkLine(data []byte, target string) []byte {
	lines := strings.Split(string(data), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(stripComment(line))
		if len(fields) >= 2 && fields[1] == target {
			if _, ok := parseKind(fields[0]); ok {
				continue
			}
		}
		kept = append(kept, line)
	}
	return []byte(strings.Join(kept, "\n"))
}

// saveBookmarkData writes atomically (temp file + rename) at 0600, creating the
// directory 0700 if needed. Reading never creates anything; only writing does.
func saveBookmarkData(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".bookmarks-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck // best-effort cleanup if rename succeeded
	if _, err := tmp.Write(data); err != nil {
		tmp.Close() //nolint:errcheck
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// shortenHome renders a path with ~ for display without making it wrong.
func shortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || !strings.HasPrefix(path, home) {
		return path
	}
	return "~" + strings.TrimPrefix(path, home)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestResolveBookmarks|TestAppendBookmark|TestDeleteBookmark|TestSaveAndLoad|TestLoadBookmarks' -count=1 -v`
Expected: PASS

- [ ] **Step 5: Run the full gate and commit**

```bash
make check
git add tui/bookmarks.go tui/bookmarks_test.go
git commit -m "feat(tui): load and write the bookmarks file with line surgery"
```

---

### Task 3: Embedded catalog

The catalog is hand-edited data compiled into the binary. The guard test exists because a typo here ships to everyone and cannot be fixed without a release.

**Files:**
- Create: `tui/catalog.go`
- Create: `tui/catalog.txt`
- Create: `tui/catalog_test.go`

**Interfaces:**
- Consumes: `parseCatalogData`, `startEntry`, `entryKind` from Task 1
- Produces: `loadCatalog() []startEntry`

- [ ] **Step 1: Write the failing test**

```go
package tui

import "testing"

// TestCatalogIsWellFormed guards hand-edited data that ships compiled in: a
// typo cannot be corrected without cutting a release, so it must not merge.
func TestCatalogIsWellFormed(t *testing.T) {
	entries, problems := parseCatalogData(catalogData)
	if len(problems) != 0 {
		t.Fatalf("catalog.txt has %d bad lines: %+v", len(problems), problems)
	}
	if len(entries) < 20 {
		t.Fatalf("catalog has %d entries, want at least 20", len(entries))
	}

	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.note == "" {
			t.Errorf("%s has no note; every catalog entry must describe itself", e.target)
		}
		if e.source != sourceCatalog {
			t.Errorf("%s source = %v, want sourceCatalog", e.target, e.source)
		}
		if seen[e.target] {
			t.Errorf("%s appears twice", e.target)
		}
		seen[e.target] = true
	}
}

func TestCatalogShipsNoPeople(t *testing.T) {
	// The catalog deliberately contains no personal addresses; people are what
	// bookmarks are for. See the spec's "People — none".
	for _, e := range loadCatalog() {
		if e.kind == kindPerson {
			t.Errorf("%s is a person; the catalog ships none", e.target)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run TestCatalog -count=1`
Expected: FAIL — `undefined: catalogData`

- [ ] **Step 3: Create the catalog data file**

Create `tui/catalog.txt` exactly as below. Every address was probed live on 2026-08-11; see the spec for exclusions and their reasons.

```
# lookit's built-in catalog: places worth starting from.
#
# Format: <kind> <target> <note>
# Compiled into the binary; users override nothing here. Their own list lives
# in ~/.config/lookit/bookmarks, and "catalog off" there hides all of this.
#
# Every entry was probed live on 2026-08-11. When refreshing, probe SERIALLY —
# bbs.airandwave.net rate-limits and a concurrent sweep reports false deaths.

community @plan.cat Finger-first microblogging
community @tilde.team Big, friendly pubnix
community @happynetbox.com Hosted .plan pages, no shell account needed
community @telehack.com Retro-computing sandbox; .plan pages autogenerated
community ring@thebackupbox.net The Finger Ring — a webring for finger servers
community @cosmic.voyage Collaborative science fiction
community @athena.dialup.mit.edu MIT Athena, still answering
community @zaibatsu.circumlunar.space Circumlunar Space pubnix
community @chunboan.zone Small community server

service @bbs.airandwave.net Menu of a dozen-plus finger services
service weather@bbs.airandwave.net Weather and 7-day forecast — weather:city@…
service @graph.no Weather worldwide by place name — finger oslo@graph.no
service quake@bbs.airandwave.net Latest earthquakes, M2.5+ past day
service dict@bbs.airandwave.net Dictionary — dict:word@…
service urban@bbs.airandwave.net Urban Dictionary — urban:word@…
service wordsearch:today@bbs.airandwave.net Daily word search puzzle
service sudoku:easy@bbs.airandwave.net Sudoku, easy mode
service textfile@typed-hole.org A random file from textfiles.com
service calendar@flanigan.us Historical calendar: on this day
service bot@happynetbox.com Auto news bot: article titles and URLs
service random@happynetbox.com Jump to a random happynetbox user
service ansi@happynetbox.com ANSI art over finger
service browserversion@happynetbox.com Current browser version numbers
service 1@happynetbox.com Interactive fiction, chained over finger
service cyoa@typed-hole.org Choose your own adventure
service smog@typed-hole.org Saturday Morning Gemzine — back issues
service originsfinger@happynetbox.com Les Earnest on the origins of finger
```

- [ ] **Step 4: Write the loader**

Create `tui/catalog.go`:

```go
package tui

import _ "embed"

// catalogData is lookit's curated starting list, compiled in so it can be
// improved for every user on each release. Seeding these into the user's file
// instead would freeze them forever at the version first run.
//
//go:embed catalog.txt
var catalogData []byte

// loadCatalog parses the embedded catalog. Bad lines are impossible in a
// released binary — TestCatalogIsWellFormed fails the build gate first — so
// any that appear at runtime are skipped silently rather than shown to a user
// who cannot fix them.
func loadCatalog() []startEntry {
	entries, _ := parseCatalogData(catalogData)
	return entries
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./tui/ -run TestCatalog -count=1 -v`
Expected: PASS — 28 entries parsed, all with notes, no people

- [ ] **Step 6: Run the full gate and commit**

```bash
make check
git add tui/catalog.go tui/catalog.txt tui/catalog_test.go
git commit -m "feat(tui): embed the curated startpage catalog"
```

---

### Task 4: Section assembly

Merging the two sources: bookmarks first, catalog grouped and deduped, notes borrowed by target match.

**Files:**
- Create: `tui/sections.go`
- Create: `tui/sections_test.go`

> The spec's Structure section lists three new files; this splits assembly into
> a fourth. Assembly is pure logic with the densest test matrix (dedup, note
> borrowing, `catalog off`), and keeping it out of `start.go` lets it be tested
> without constructing a Bubble Tea model. Deliberate divergence, recorded here.

**Interfaces:**
- Consumes: `startEntry`, `bookmarkFile`, `entryKind` from Task 1
- Produces: `startSection{title string; entries []startEntry}`, `buildSections(catalog []startEntry, bm bookmarkFile) []startSection`

- [ ] **Step 1: Write the failing test**

```go
package tui

import "testing"

func catalogFixture() []startEntry {
	return []startEntry{
		{target: "@tilde.team", kind: kindCommunity, note: "Big, friendly pubnix", source: sourceCatalog},
		{target: "@plan.cat", kind: kindCommunity, note: "Finger-first microblogging", source: sourceCatalog},
		{target: "quake@bbs.airandwave.net", kind: kindService, note: "Latest earthquakes", source: sourceCatalog},
	}
}

func TestBuildSectionsCatalogOnly(t *testing.T) {
	got := buildSections(catalogFixture(), bookmarkFile{})
	if len(got) != 2 {
		t.Fatalf("sections = %d (%+v), want 2", len(got), got)
	}
	if got[0].title != "COMMUNITIES" || len(got[0].entries) != 2 {
		t.Errorf("section 0 = %+v", got[0])
	}
	if got[1].title != "SERVICES" || len(got[1].entries) != 1 {
		t.Errorf("section 1 = %+v", got[1])
	}
}

func TestBuildSectionsBookmarksComeFirstAndDedup(t *testing.T) {
	bm := bookmarkFile{entries: []startEntry{
		{target: "@tilde.team", kind: kindCommunity, source: sourceBookmark},
	}}
	got := buildSections(catalogFixture(), bm)
	if got[0].title != "BOOKMARKS" {
		t.Fatalf("section 0 title = %q, want BOOKMARKS", got[0].title)
	}
	if len(got[0].entries) != 1 || got[0].entries[0].target != "@tilde.team" {
		t.Fatalf("bookmarks section = %+v", got[0].entries)
	}
	// The note travels with the target even though the file stores none.
	if got[0].entries[0].note != "Big, friendly pubnix" {
		t.Errorf("note = %q, want the catalog's note", got[0].entries[0].note)
	}
	// And it is suppressed from COMMUNITIES rather than appearing twice.
	for _, e := range got[1].entries {
		if e.target == "@tilde.team" {
			t.Error("@tilde.team appears in both BOOKMARKS and COMMUNITIES")
		}
	}
}

func TestBuildSectionsBookmarkWithoutCatalogMatchShowsKind(t *testing.T) {
	bm := bookmarkFile{entries: []startEntry{
		{target: "jonathan@tilde.team", kind: kindPerson, source: sourceBookmark},
	}}
	got := buildSections(catalogFixture(), bm)
	if got[0].entries[0].note != "person" {
		t.Fatalf("note = %q, want the kind as a fallback", got[0].entries[0].note)
	}
}

func TestBuildSectionsCatalogOff(t *testing.T) {
	bm := bookmarkFile{
		catalogHidden: true,
		entries:       []startEntry{{target: "@plan.cat", kind: kindCommunity, source: sourceBookmark}},
	}
	got := buildSections(catalogFixture(), bm)
	if len(got) != 1 || got[0].title != "BOOKMARKS" {
		t.Fatalf("sections = %+v, want BOOKMARKS only", got)
	}
	// A hidden catalog still supplies notes for matching bookmarks.
	if got[0].entries[0].note != "Finger-first microblogging" {
		t.Errorf("note = %q, want the catalog's note", got[0].entries[0].note)
	}
}

func TestBuildSectionsEmpty(t *testing.T) {
	if got := buildSections(nil, bookmarkFile{catalogHidden: true}); len(got) != 0 {
		t.Fatalf("sections = %+v, want none", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run TestBuildSections -count=1`
Expected: FAIL — `undefined: buildSections`

- [ ] **Step 3: Write minimal implementation**

```go
package tui

// startSection is one titled group of startpage rows.
type startSection struct {
	title   string
	entries []startEntry
}

// buildSections merges the two sources into the rendered order: the user's
// bookmarks first, then the catalog grouped by kind.
//
// Two behaviours make bookmarking read as "pin to the top":
//   - a bookmarked target is suppressed from its catalog section rather than
//     appearing twice, and
//   - it keeps the catalog's note, so pinning never costs the description.
//
// That is what lets the bookmarks file store no notes at all — which in turn is
// what keeps free text out of the terminal (see the spec's trust section).
func buildSections(catalog []startEntry, bm bookmarkFile) []startSection {
	notes := make(map[string]string, len(catalog))
	for _, e := range catalog {
		notes[e.target] = e.note
	}

	var sections []startSection

	if len(bm.entries) > 0 {
		bookmarked := make([]startEntry, 0, len(bm.entries))
		for _, e := range bm.entries {
			if note, ok := notes[e.target]; ok && note != "" {
				e.note = note
			} else {
				e.note = e.kind.String()
			}
			bookmarked = append(bookmarked, e)
		}
		sections = append(sections, startSection{title: "BOOKMARKS", entries: bookmarked})
	}

	if bm.catalogHidden {
		return sections
	}

	pinned := make(map[string]bool, len(bm.entries))
	for _, e := range bm.entries {
		pinned[e.target] = true
	}
	for _, group := range []struct {
		title string
		kind  entryKind
	}{
		{title: "COMMUNITIES", kind: kindCommunity},
		{title: "SERVICES", kind: kindService},
		{title: "PEOPLE", kind: kindPerson},
	} {
		var entries []startEntry
		for _, e := range catalog {
			if e.kind == group.kind && !pinned[e.target] {
				entries = append(entries, e)
			}
		}
		if len(entries) > 0 {
			sections = append(sections, startSection{title: group.title, entries: entries})
		}
	}
	return sections
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run TestBuildSections -count=1 -v`
Expected: PASS

- [ ] **Step 5: Run the full gate and commit**

```bash
make check
git add tui/sections.go tui/sections_test.go
git commit -m "feat(tui): assemble startpage sections from catalog and bookmarks"
```

---

### Task 5: `startModel` — the list with skipped headers

`bubbles/v2/list` has no section support and its pagination assumes a uniform `delegate.Height()`. Headers are therefore items rendered at the same two-row cell height as an entry, and the cursor steps over them. This is the fiddliest part of the feature; the edge tests are the point.

**Files:**
- Create: `tui/start.go`
- Create: `tui/start_test.go`

**Interfaces:**
- Consumes: `startSection`, `startEntry` from Task 4; `commonModel`, `styles`, `userDelegate`, `applyListStyles` (`tui/list.go:88-146`)
- Produces: `startItem`, `startModel`, `newStart(common *commonModel, sections []startSection, notice, empty string) startModel`, and methods `update`, `View`, `setSize`, `selected() (startEntry, bool)`, `filtering() bool`, `applyStyles(styles)`

- [ ] **Step 1: Write the failing test**

```go
package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

func testCommon() *commonModel {
	return &commonModel{width: 80, height: 24, profile: colorprofile.NoTTY, darkBackground: true, styles: newStyles(true)}
}

func twoSections() []startSection {
	return []startSection{
		{title: "BOOKMARKS", entries: []startEntry{
			{target: "@tilde.team", kind: kindCommunity, note: "Big, friendly pubnix", source: sourceBookmark},
		}},
		{title: "COMMUNITIES", entries: []startEntry{
			{target: "@plan.cat", kind: kindCommunity, note: "Finger-first microblogging", source: sourceCatalog},
			{target: "@happynetbox.com", kind: kindCommunity, note: "Hosted .plan pages", source: sourceCatalog},
		}},
	}
}

// The cursor must never rest on a header, including at construction.
func TestStartSelectionSkipsLeadingHeader(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	got, ok := m.selected()
	if !ok {
		t.Fatal("selected() ok = false, want an entry")
	}
	if got.target != "@tilde.team" {
		t.Fatalf("selected = %q, want @tilde.team", got.target)
	}
}

// Moving down out of one section must land on the next section's first entry,
// stepping over its header rather than selecting it.
func TestStartCursorStepsOverInteriorHeader(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	got, ok := m.selected()
	if !ok {
		t.Fatal("selected() ok = false")
	}
	if got.target != "@plan.cat" {
		t.Fatalf("selected = %q, want @plan.cat", got.target)
	}
}

// Moving up must step over the header in the other direction too.
func TestStartCursorStepsOverHeaderUpward(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m, _ = m.update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	got, _ := m.selected()
	if got.target != "@tilde.team" {
		t.Fatalf("selected = %q, want @tilde.team", got.target)
	}
}

// At the last entry, down must not strand the cursor on a trailing header or
// move past the end.
func TestStartCursorStopsAtLastEntry(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	for range 6 {
		m, _ = m.update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	}
	got, ok := m.selected()
	if !ok {
		t.Fatal("selected() ok = false at the end of the list")
	}
	if got.target != "@happynetbox.com" {
		t.Fatalf("selected = %q, want @happynetbox.com", got.target)
	}
}

func TestStartEmptyStateHasNoSelection(t *testing.T) {
	m := newStart(testCommon(), nil, "", "No bookmarks yet.")
	if _, ok := m.selected(); ok {
		t.Fatal("selected() ok = true on an empty startpage")
	}
	if got := m.View(); !strings.Contains(got, "No bookmarks yet.") {
		t.Fatalf("View() = %q, want the empty state to explain itself", got)
	}
}

func TestStartViewShowsNoticeAndHeaders(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "2 unreadable lines in ~/.config/lookit/bookmarks", "")
	got := m.View()
	for _, want := range []string{"unreadable lines", "BOOKMARKS", "COMMUNITIES", "@tilde.team"} {
		if !strings.Contains(got, want) {
			t.Errorf("View() missing %q:\n%s", want, got)
		}
	}
}
```

Add `"strings"` to the test imports. Note `contains` already exists in
`tui/keys_test.go:81` with the signature `contains(ss []string, s string) bool`
— it takes a slice, so use `strings.Contains` for substring checks, not that.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run TestStart -count=1`
Expected: FAIL — `undefined: newStart`

- [ ] **Step 3: Write minimal implementation**

```go
package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

// startChromeRows matches listChromeRows: space the bubbles list reserves once
// its own title and help are hidden.
const startChromeRows = 1

// startItem is one row. A row is either a section header or an entry; headers
// occupy a normal item slot so the list's uniform-height pagination still holds.
type startItem struct {
	entry  startEntry
	header string // non-empty => this row is a section heading
}

// FilterValue drives "/". Headers return "" so they drop out while filtering,
// which flattens the view to matches — the behaviour we want.
func (i startItem) FilterValue() string {
	if i.header != "" {
		return ""
	}
	return i.entry.target + " " + i.entry.note
}

func (i startItem) Title() string {
	if i.header != "" {
		return i.header
	}
	return i.entry.target
}

func (i startItem) Description() string {
	if i.header != "" {
		return ""
	}
	return i.entry.note
}

// startModel is the launch screen: an embedded catalog plus the user's
// bookmarks, rendered as one sectioned list.
type startModel struct {
	common *commonModel
	list   list.Model
	notice string // parse problems, surfaced rather than swallowed
	empty  string // shown instead of the list when there is nothing to show
}

func newStart(common *commonModel, sections []startSection, notice, empty string) startModel {
	var items []list.Item
	for _, s := range sections {
		items = append(items, startItem{header: s.title})
		for _, e := range s.entries {
			items = append(items, startItem{entry: e})
		}
	}

	st := common.ensureStyles()
	height := common.bodyHeight() - startChromeRows
	if height < 1 {
		height = 1
	}
	l := list.New(items, startDelegate{userDelegate: defaultUserDelegate(st), st: st}, common.width, height)
	applyListStyles(&l, st)
	l.SetDelegate(startDelegate{userDelegate: defaultUserDelegate(st), st: st})
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)

	m := startModel{common: common, list: l, notice: notice, empty: empty}
	m.skipHeader(1) // never rest on the leading header
	m.setSize(common.width, common.bodyHeight())
	return m
}

// startDelegate renders headers itself and defers entries to the existing user
// delegate, so startpage rows look exactly like user-list rows.
type startDelegate struct {
	userDelegate
	st styles
}

func (d startDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	if it, ok := item.(startItem); ok && it.header != "" {
		// Two rows, matching the entry cell height that pagination assumes.
		fmt.Fprintf(w, "\n%s", d.st.barFlag.Render(it.header)) //nolint:errcheck
		return
	}
	d.userDelegate.Render(w, m, index, item)
}

func (m startModel) update(msg tea.Msg) (startModel, tea.Cmd) {
	before := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if after := m.list.Index(); after != before {
		dir := 1
		if after < before {
			dir = -1
		}
		m.skipHeader(dir)
	}
	return m, cmd
}

// skipHeader advances past a header in the direction of travel, reversing at
// the ends so the cursor can never come to rest on a heading.
func (m *startModel) skipHeader(dir int) {
	items := m.list.VisibleItems()
	if len(items) == 0 {
		return
	}
	idx := m.list.Index()
	for range len(items) {
		it, ok := items[idx].(startItem)
		if !ok || it.header == "" {
			m.list.Select(idx)
			return
		}
		idx += dir
		if idx < 0 || idx >= len(items) {
			dir = -dir
			idx += 2 * dir // step back inside, then continue the other way
			if idx < 0 || idx >= len(items) {
				return
			}
		}
	}
}

func (m startModel) View() string {
	if len(m.list.VisibleItems()) == 0 && m.empty != "" {
		if m.notice != "" {
			return m.notice + "\n\n" + m.empty
		}
		return m.empty
	}
	if m.notice != "" {
		return m.notice + "\n\n" + m.list.View()
	}
	return m.list.View()
}

func (m *startModel) setSize(width, height int) {
	h := height - startChromeRows - m.noticeHeight()
	if h < 1 {
		h = 1
	}
	m.list.SetSize(width, h)
}

func (m startModel) noticeHeight() int {
	if m.notice == "" {
		return 0
	}
	return len(strings.Split(m.notice, "\n")) + 1
}

// selected returns the highlighted entry. A header or an empty list yields false.
func (m startModel) selected() (startEntry, bool) {
	it, ok := m.list.SelectedItem().(startItem)
	if !ok || it.header != "" {
		return startEntry{}, false
	}
	return it.entry, true
}

func (m startModel) filtering() bool {
	return m.list.FilterState() == list.Filtering
}

func (m *startModel) applyStyles(st styles) {
	applyListStyles(&m.list, st)
	m.list.SetDelegate(startDelegate{userDelegate: defaultUserDelegate(st), st: st})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run TestStart -count=1 -v`
Expected: PASS — all cursor-skip edges

- [ ] **Step 5: Run the full gate and commit**

```bash
make check
git add tui/start.go tui/start_test.go
git commit -m "feat(tui): add the startpage list model with section headers"
```

---

### Task 6: Wire `stateStart` into the app

`stateStart` is used only at `pos == -1` and never stored in a `histNode` — About already establishes a state that is not a history entry.

**Files:**
- Modify: `tui/app.go` (`appState` const block ~46-50; `appModel` struct ~103-132; `newAppWithContext` ~142-186; `applyStyles` ~194-209; `gotoLanding` ~308-317; `Update` delegation ~538-544; `buildStatusBar` ~1165-1167; `View` ~1429-1439; `resize` ~1268-1285)
- Modify: `tui/app_test.go`

**Interfaces:**
- Consumes: `newStart`, `startModel` from Task 5; `loadCatalog` from Task 3; `loadBookmarks`, `shortenHome` from Task 2; `buildSections` from Task 4
- Produces: `stateStart` appState, `appModel.start startModel`, `(*appModel).reloadStart()`, `(*appModel).gotoStart()`, `startNotice(bookmarkFile, string) string`, `startEmptyMessage(bookmarkFile, string) string`

- [ ] **Step 1: Write the failing test**

```go
func TestAppOpensOnStartpage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bookmarks")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = resolveBookmarksPath })

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	if m.state != stateStart {
		t.Fatalf("state = %v, want stateStart", m.state)
	}
	if m.pos != -1 {
		t.Fatalf("pos = %d, want -1", m.pos)
	}
	if _, ok := m.start.selected(); !ok {
		t.Fatal("startpage has no selection; the catalog should populate it")
	}
}

func TestStartNoticeNamesResolvedPath(t *testing.T) {
	file := bookmarkFile{problems: []parseProblem{{line: 3, reason: "unknown kind \"wombat\""}}}
	got := startNotice(file, "/tmp/xdg/lookit/bookmarks")
	if !strings.Contains(got, "/tmp/xdg/lookit/bookmarks") {
		t.Fatalf("notice = %q, want the resolved path", got)
	}
	if !strings.Contains(got, "line 3") {
		t.Fatalf("notice = %q, want the line number", got)
	}
}

func TestStartEmptyMessageNamesResolvedPath(t *testing.T) {
	got := startEmptyMessage(bookmarkFile{catalogHidden: true}, "/tmp/xdg/lookit/bookmarks")
	if !strings.Contains(got, "/tmp/xdg/lookit/bookmarks") {
		t.Fatalf("empty message = %q, want the resolved path, not the ~/.config fallback", got)
	}
	if !strings.Contains(got, "catalog off") {
		t.Fatalf("empty message = %q, want it to name the directive to remove", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run 'TestAppOpensOnStartpage|TestStartNotice|TestStartEmptyMessage' -count=1`
Expected: FAIL — `undefined: stateStart`

- [ ] **Step 3: Write minimal implementation**

In `tui/app.go`, extend the state enum:

```go
const (
	stateReader appState = iota
	stateList
	stateAbout
	stateStart // the launch screen; used only at pos == -1, never in a histNode
)
```

Add to `appModel`, after the `about aboutModel` field:

```go
	start startModel
```

Add these functions to `tui/app.go`:

```go
// reloadStart rebuilds the startpage from disk. Called at construction and
// after every bookmark write, so the screen always reflects the file.
func (m *appModel) reloadStart() {
	file, path := loadBookmarks()
	sections := buildSections(loadCatalog(), file)
	m.start = newStart(m.common, sections, startNotice(file, path), startEmptyMessage(file, path))
}

// startNotice surfaces parse problems rather than swallowing them, naming the
// file actually in use so the user edits the one that has an effect.
func startNotice(file bookmarkFile, path string) string {
	if len(file.problems) == 0 {
		return ""
	}
	shown := shortenHome(path)
	if len(file.problems) == 1 {
		p := file.problems[0]
		if p.line == 0 {
			return fmt.Sprintf("%s: %s", shown, p.reason)
		}
		return fmt.Sprintf("%s line %d: %s", shown, p.line, p.reason)
	}
	lines := make([]string, 0, len(file.problems))
	for _, p := range file.problems {
		lines = append(lines, fmt.Sprintf("line %d", p.line))
	}
	return fmt.Sprintf("%d unreadable lines in %s (%s)", len(file.problems), shown, strings.Join(lines, ", "))
}

// startEmptyMessage explains a blank startpage instead of letting it look
// broken. It quotes the resolved path: with $XDG_CONFIG_HOME set, the
// ~/.config fallback would send the user to edit a file with no effect.
func startEmptyMessage(file bookmarkFile, path string) string {
	if file.catalogHidden {
		return fmt.Sprintf("No bookmarks yet. The catalog is off — remove `catalog off` from %s to see it.", shortenHome(path))
	}
	return "No bookmarks yet."
}
```

Replace `gotoLanding` with `gotoStart` (and update its two callers in `stepBack` and anywhere else `gotoLanding` appears):

```go
// gotoStart returns to the launch screen, reloading it so a bookmark added
// while browsing is present when you walk back.
func (m *appModel) gotoStart() {
	m.state = stateStart
	m.reader.current = nil
	m.reloadStart()
	m.inputFocused = true
	m.input.SetValue("")
	m.input.Focus() // discard the blink cmd; the cursor still shows
	m.resize()
}
```

In `newAppWithContext`, after `app.about.setBackground(...)`, add:

```go
	app.state = stateStart
	app.reloadStart()
```

In `applyStyles`, after the `m.about.setBackground(...)` line:

```go
	m.start.applyStyles(st)
```

In `resize`, after the reader/list sizing:

```go
	m.start.setSize(m.common.width, h)
```

In `Update`'s delegation switch, add a case before `default`:

```go
	case stateStart:
		m.start, cmd = m.start.update(msg)
```

In `View`'s content switch, add:

```go
		case stateStart:
			content = m.start.View()
```

In `buildStatusBar`, replace the `if m.pos < 0 { return landingBar(w, st) }` branch with:

```go
	if m.pos < 0 {
		return m.startBar(w, st)
	}
```

and add:

```go
// startBar is the launch screen's bottom bar. It replaces landingBar, which
// had nothing to advertise but typing.
func (m appModel) startBar(width int, st styles) statusBar {
	bar := statusBar{width: width, styles: st}
	if m.inputFocused {
		bar.hints = "type a target and press ↵ · ↓ browse · ? help"
		return bar
	}
	n := 0
	for _, it := range m.start.list.VisibleItems() {
		if si, ok := it.(startItem); ok && si.header == "" {
			n++
		}
	}
	if n > 0 {
		bar.meta = fmt.Sprintf("%d entries", n)
	}
	bar.hints = "↵ go · b bookmark · / filter · i target · ? help"
	return bar
}
```

Delete `landingBar` from `tui/statusbar.go:64-67`. It has one other caller —
`tui/statusbar_test.go:39` — so replace that test with one that exercises
`startBar` instead of deleting the coverage.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestAppOpensOnStartpage|TestStartNotice|TestStartEmptyMessage' -count=1 -v`
Expected: PASS

- [ ] **Step 5: Run the full suite — existing tests will need updating**

Run: `go test ./tui/ -count=1`
Expected: some existing landing tests fail, because the launch screen is no longer `stateReader` with `"No response yet."`. Update each to expect `stateStart`. Do not weaken an assertion to make it pass — if a test checked that the landing was empty, it should now check the startpage renders.

- [ ] **Step 6: Run the full gate and commit**

```bash
make check
git add tui/app.go tui/app_test.go tui/statusbar.go tui/statusbar_test.go
git commit -m "feat(tui): render the startpage as the launch screen"
```

---

### Task 7: The `b` and `h` keys

**Files:**
- Modify: `tui/keys.go`
- Modify: `tui/keys_test.go`
- Modify: `tui/app.go` (`handleKey`, `updateKeymap`)
- Modify: `tui/app_test.go`

**Interfaces:**
- Consumes: everything from Tasks 1-6
- Produces: `keyMap.Bookmark`, `keyMap.Home`, `(*appModel).toggleBookmark() tea.Cmd`, `(appModel).bookmarkTarget() (startEntry, bool)`, `(*appModel).goHome()`

- [ ] **Step 1: Write the failing test**

```go
func TestBookmarkAndHomeKeysBound(t *testing.T) {
	k := newKeyMap()
	if got := k.Bookmark.Keys(); !contains(got, "b") {
		t.Fatalf("Bookmark keys = %v, want b", got)
	}
	if got := k.Home.Keys(); !contains(got, "h") {
		t.Fatalf("Home keys = %v, want h", got)
	}
	// h moved from Page to Home; paging keeps the arrows it advertises.
	if got := k.Page.Keys(); contains(got, "h") {
		t.Fatalf("Page keys = %v, must not still claim h", got)
	}
	if got := k.Page.Keys(); !contains(got, "left") || !contains(got, "right") {
		t.Fatalf("Page keys = %v, want the arrows", got)
	}
}

func TestBookmarkOnStartpageTogglesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookmarks")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = resolveBookmarksPath })

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	first, ok := m.start.selected()
	if !ok {
		t.Fatal("no selection to bookmark")
	}

	_, m, _ = m.handleKey(tea.KeyPressMsg{Code: 'b', Text: "b"})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after bookmark: %v", err)
	}
	if !strings.Contains(string(data), first.target) {
		t.Fatalf("file = %q, want it to contain %q", data, first.target)
	}

	// The pinned entry now heads BOOKMARKS, so pressing b again removes it.
	_, m, _ = m.handleKey(tea.KeyPressMsg{Code: 'b', Text: "b"})
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after unbookmark: %v", err)
	}
	if strings.Contains(string(data), first.target) {
		t.Fatalf("file = %q, want %q removed", data, first.target)
	}
}

func TestHomeTruncatesHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookmarks")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = resolveBookmarksPath })

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.history = []histNode{
		{entry: Entry{Target: mustTarget(t, "@plan.cat")}, state: stateReader, linkIdx: -1},
		{entry: Entry{Target: mustTarget(t, "@tilde.team")}, state: stateReader, linkIdx: -1},
	}
	m.pos = 1
	m.state = stateReader
	m.blurInput()

	handled, m, _ := m.handleKey(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if !handled {
		t.Fatal("h not handled")
	}
	if m.state != stateStart {
		t.Fatalf("state = %v, want stateStart", m.state)
	}
	if m.pos != -1 {
		t.Fatalf("pos = %d, want -1", m.pos)
	}
	if len(m.history) != 0 {
		t.Fatalf("history = %+v, want truncated", m.history)
	}
}

func mustTarget(t *testing.T, raw string) finger.Target {
	t.Helper()
	target, err := finger.ParseTarget(raw)
	if err != nil {
		t.Fatalf("ParseTarget(%q) = %v", raw, err)
	}
	return target
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run 'TestBookmarkAndHomeKeys|TestBookmarkOnStartpage|TestHomeTruncates' -count=1`
Expected: FAIL — `k.Bookmark undefined`

- [ ] **Step 3: Write minimal implementation**

In `tui/keys.go`, add to the struct after `About`:

```go
	Bookmark key.Binding
	Home     key.Binding
```

In `newKeyMap`, add and amend `Page`:

```go
		Bookmark: key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "bookmark")),
		Home:     key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "startpage")),
		// 'h'/'l' dropped: 'h' is now Home. The arrows are what the bar advertises.
		Page: key.NewBinding(key.WithKeys("left", "right", "pgup", "pgdown"), key.WithHelp("←/→", "page")),
```

In `FullHelp`, put both in the navigation row:

```go
	return [][]key.Binding{
		{k.Open, k.FocusInput, k.Copy, k.Raw, k.Refresh},
		{k.Move, k.Page, k.Jump, k.Filter},
		{k.Bookmark, k.Home, k.Back, k.About, k.Quit},
	}
```

In `tui/app.go`, add the handlers:

```go
// bookmarkTarget reports what 'b' acts on for the current screen. On a list it
// is the host, not the highlighted user: 'b' on @tilde.team means "come back to
// this directory". To bookmark a person, drill in and press b there.
func (m appModel) bookmarkTarget() (startEntry, bool) {
	if m.state == stateStart {
		return m.start.selected()
	}
	if m.pos < 0 || m.pos >= len(m.history) {
		return startEntry{}, false
	}
	target := m.history[m.pos].entry.Target
	kind := kindPerson
	if target.HostQuery() {
		kind = kindCommunity
	}
	return startEntry{target: target.Raw, kind: kind, source: sourceBookmark}, true
}

// toggleBookmark adds or removes the current target, then reloads the startpage
// so it reflects the file. Kind inference is a guess — 'service' is not
// inferable, since weather:99501@host is user@host-shaped — but kind only
// affects display, and one word in the file corrects it.
func (m *appModel) toggleBookmark() tea.Cmd {
	entry, ok := m.bookmarkTarget()
	if !ok {
		return nil
	}
	path, err := bookmarksPathFn()
	if err != nil {
		return m.setFlash("error: " + err.Error())
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return m.setFlash("error: " + err.Error())
	}

	file := parseBookmarks(data)
	already := false
	for _, e := range file.entries {
		if e.target == entry.target {
			already = true
			break
		}
	}

	var updated []byte
	var msg string
	if already {
		updated = deleteBookmarkLine(data, entry.target)
		msg = "✓ removed " + entry.target
	} else {
		updated = appendBookmarkLine(data, entry)
		msg = "✓ bookmarked " + entry.target
	}
	if err := saveBookmarkData(path, updated); err != nil {
		return m.setFlash("error: " + err.Error())
	}
	if m.state == stateStart {
		m.reloadStart()
		m.resize()
	}
	return m.setFlash(msg)
}

// goHome is exactly equivalent to holding Esc: return to the startpage and drop
// the trail. The startpage is not a history node, so there is nothing to push.
func (m *appModel) goHome() {
	m.clearRequestFailure()
	m.showingRaw = false
	m.showingLinks = false
	m.flash = ""
	m.history = nil
	m.pos = -1
	m.gotoStart()
}
```

Add `"os"` to `tui/app.go`'s imports.

In `handleKey`, in the content-focused branch (alongside the other content-only keys, **not** in the input-focused branch — `b` and `h` must still type literally into a target):

```go
	case key.Matches(msg, m.keys.Bookmark):
		return true, m, m.toggleBookmark()
	case key.Matches(msg, m.keys.Home):
		m.goHome()
		return true, m, nil
```

In `updateKeymap`, with the other content-only keys:

```go
	_, canBookmark := m.bookmarkTarget()
	m.keys.Bookmark.SetEnabled(content && canBookmark && !m.showingRaw && !m.showingLinks)
	m.keys.Home.SetEnabled(content && (m.pos >= 0 || m.state != stateStart))
```

And in the `m.pending != nil` early-return block, add `&m.keys.Bookmark, &m.keys.Home` to the list of bindings disabled while loading.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestBookmarkAndHomeKeys|TestBookmarkOnStartpage|TestHomeTruncates' -count=1 -v`
Expected: PASS

- [ ] **Step 5: Run the full gate and commit**

```bash
make check
git add tui/keys.go tui/keys_test.go tui/app.go tui/app_test.go
git commit -m "feat(tui): add bookmark and startpage keybindings"
```

---

### Task 8: Focus model

`↓`/`Tab` drops from the input into the list; Esc backs out one level and then quits. This closes the trap where Esc, pressed to leave the input, exits the app.

**Files:**
- Modify: `tui/app.go` (`handleKey` input-focused branch)
- Modify: `tui/app_test.go`

**Interfaces:**
- Consumes: Tasks 6-7
- Produces: no new exported surface; behaviour only

- [ ] **Step 1: Write the failing test**

```go
func TestStartpageArrowDownEntersList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookmarks")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = resolveBookmarksPath })

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	if !m.inputFocused {
		t.Fatal("launch should focus the input")
	}
	handled, m, _ := m.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if !handled {
		t.Fatal("down not handled while the input is focused")
	}
	if m.inputFocused {
		t.Fatal("down should move focus into the startpage list")
	}
}

func TestStartpageEscBacksOutThenQuits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookmarks")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = resolveBookmarksPath })

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()

	// From the list, esc returns to the input rather than quitting.
	handled, m, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !handled {
		t.Fatal("esc not handled from the list")
	}
	if cmd != nil {
		t.Fatal("esc from the startpage list must not quit")
	}
	if !m.inputFocused {
		t.Fatal("esc should return focus to the input")
	}

	// From the input, esc quits.
	_, _, cmd = m.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc from the input at the startpage should quit")
	}
}

func TestBookmarkKeyTypesIntoFocusedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookmarks")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = resolveBookmarksPath })

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	handled, _, _ := m.handleKey(tea.KeyPressMsg{Code: 'b', Text: "b"})
	if handled {
		t.Fatal("b must type into a focused input, not bookmark")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run 'TestStartpageArrowDown|TestStartpageEsc|TestBookmarkKeyTypes' -count=1`
Expected: FAIL — esc quits instead of blurring

- [ ] **Step 3: Write minimal implementation**

In `handleKey`'s input-focused branch, before the existing Enter/Esc handling:

```go
	if m.inputFocused && m.state == stateStart {
		switch msg.String() {
		case "down", "tab":
			if _, ok := m.start.selected(); ok {
				m.blurInput()
				return true, m, nil
			}
		case "esc":
			// Esc backs out one level before it quits: from the list it returns
			// here, so quitting from the input is the outermost step.
			return true, m, tea.Quit
		}
	}
```

And in the content-focused branch, before the generic `Back` handling:

```go
	if m.state == stateStart && key.Matches(msg, m.keys.Back) {
		return true, m, m.focusInput()
	}
```

In `updateKeymap`, keep `Back` live on the startpage in both focus modes:

```go
	m.keys.Back.SetEnabled(m.inputFocused || (content && hasResult) || m.state == stateStart)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestStartpageArrowDown|TestStartpageEsc|TestBookmarkKeyTypes' -count=1 -v`
Expected: PASS

- [ ] **Step 5: Run the full gate and commit**

```bash
make check
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): route focus between the target input and the startpage"
```

---

### Task 9: Documentation

The spec requires CLAUDE.md to record the second ingress; the README's "Coming soon" promised this feature and must stop promising it.

**Files:**
- Modify: `CLAUDE.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: the completed feature
- Produces: documentation only

- [ ] **Step 1: Update CLAUDE.md's ingress claim**

In the `finger/` bullet, after the sentence ending "...for every current and future display path.", add:

```
  **A second ingress exists as of the startpage:** the bookmarks file
  (`tui/bookmarks.go`). It does not weaken the claim above, because it admits no
  free text — the kind is a closed keyword set, the target must survive
  `ParseTarget` *and* a Unicode-format-control screen, and notes are accepted
  only from the embedded catalog we author. Nothing from that file needs
  `sanitize`, because nothing unvalidated is ever displayed. If a future ingress
  does admit free text, it must call `sanitize` itself and this note must change.
```

In the `tui/` bullet, add to the state enum description that `stateStart` is the launch screen at `pos == -1` and is never a history node.

- [ ] **Step 2: Update the README**

Replace the first "Coming soon" bullet about discovery:

```markdown
- Discovery and subscriptions: following a `.plan` to see what's changed since you last looked.
```

And in the Usage section, after the paragraph beginning "Type a target and press Enter", add:

```markdown
lookit opens on a startpage: a built-in catalog of finger communities and services, with your own bookmarks pinned above it. Press `↓` to browse it, `↵` to go, and `b` to bookmark whatever you're looking at. Bookmarks live in `~/.config/lookit/bookmarks` (or `$XDG_CONFIG_HOME/lookit/bookmarks`), one `<kind> <target>` per line, so you can edit them by hand; add `catalog off` there to hide the built-in list entirely. Press `h` to return to the startpage from anywhere.
```

- [ ] **Step 3: Verify the docs match the code**

Run: `grep -n "catalog off" README.md CLAUDE.md tui/bookmarks.go`
Expected: the directive is spelled identically in all three.

- [ ] **Step 4: Run the full gate and commit**

```bash
make check
git add CLAUDE.md README.md
git commit -m "docs: document the startpage, bookmarks file and second ingress"
```

---

## Verification before opening the PR

- [ ] `make check` passes clean
- [ ] `./lookit` opens on the startpage; arrow down, `/` filter, `↵` on a community opens its user list, `↵` on a service renders in the reader
- [ ] `b` on a catalog row moves it into BOOKMARKS keeping its note; `b` again removes it
- [ ] Hand-edit the bookmarks file with comments and a `catalog off` line, press `b`, confirm the comments and directive survive verbatim
- [ ] `XDG_CONFIG_HOME=/tmp/x ./lookit` writes to `/tmp/x/lookit/bookmarks` and any message quotes that path
- [ ] `h` from several levels deep returns to the startpage
- [ ] `b` typed into a focused target input produces the letter `b`

**Do not self-merge.** The spec touches the untrusted-input invariant, which CLAUDE.md says needs a human merge: push the branch, open the PR, and leave the merge to the maintainer.

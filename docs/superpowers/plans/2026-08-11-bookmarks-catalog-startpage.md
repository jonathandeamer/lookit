# Bookmarks & Catalog Startpage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace lookit's empty landing screen with a browsable startpage that renders an embedded curated catalog plus the user's own bookmarks as one sectioned list.

**Architecture:** A catalog embedded with `go:embed` parses `<kind> <target> <note>` records into `startEntry` values; the user's line-oriented bookmarks file parses target-only records and borrows catalog metadata by exact target match during section assembly. A new `startModel` wraps a real `bubbles/v2/list` whose section headers and linked catalog credit are uniform-height items the cursor steps over. A fourth `appState` (`stateStart`) renders it at `pos == -1`; history semantics are untouched.

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
- **Catalog `kind` is display grouping only, never routing.** Bookmark records carry no kind. `routeEntry` decides list-vs-reader from the real response.
- **Package layout is one-way:** `finger/` → `render/` → `tui/`. All new code is in `tui/`.

---

### Task 1: Bookmark and catalog grammars (pure parsers)

Bookmark records are exactly `<target>`; catalog records are `<kind> <target> <note>`. Keeping them separate removes kind inference from user data, and refusing extra bookmark fields keeps free text out of the terminal.

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
	in := []byte("# my list\n\n@tilde.team\njonathan@tilde.team\nweather@bbs.airandwave.net # local comment\n")
	got := parseBookmarks(in)
	if len(got.problems) != 0 {
		t.Fatalf("problems = %+v, want none", got.problems)
	}
	if got.catalogHidden {
		t.Fatal("catalogHidden = true, want false")
	}
	want := []string{"@tilde.team", "jonathan@tilde.team", "weather@bbs.airandwave.net"}
	if len(got.targets) != len(want) {
		t.Fatalf("targets = %+v, want %d", got.targets, len(want))
	}
	for i, w := range want {
		if got.targets[i] != w {
			t.Errorf("target %d = %q, want %q", i, got.targets[i], w)
		}
	}
}

func TestParseBookmarksCatalogOff(t *testing.T) {
	got := parseBookmarks([]byte("catalog off\n@plan.cat\n"))
	if !got.catalogHidden {
		t.Fatal("catalogHidden = false, want true")
	}
	if len(got.targets) != 1 {
		t.Fatalf("targets = %+v, want 1", got.targets)
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
		{name: "multiple fields", line: "@plan.cat Big friendly pubnix"},
		{name: "unparseable target", line: "notatarget"},
		{name: "bidi override in target", line: "@plan\u202ecat.example"},
		{name: "c1 control in target", line: "@plan\u009bcat.example"},
		{name: "unknown directive", line: "catalog maybe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBookmarks([]byte(tt.line + "\n"))
			if len(got.targets) != 0 {
				t.Fatalf("targets = %+v, want none", got.targets)
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

func TestParseBookmarksRejectsInvalidUTF8(t *testing.T) {
	got := parseBookmarks([]byte{'@', 'p', 'l', 'a', 'n', '.', 0xff, 'c', 'a', 't', '\n'})
	if len(got.targets) != 0 || len(got.problems) != 1 {
		t.Fatalf("targets = %v, problems = %+v; want no targets and one problem", got.targets, got.problems)
	}
}

func TestParseCatalogAllowsNotes(t *testing.T) {
	entries, problems := parseCatalogData([]byte("community @tilde.team Small public access unix\n"))
	if len(problems) != 0 {
		t.Fatalf("problems = %+v, want none", problems)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %+v, want 1", entries)
	}
	want := startEntry{target: "@tilde.team", kind: kindCommunity, note: "Small public access unix", source: sourceCatalog}
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
	"unicode/utf8"

	"github.com/jonathandeamer/lookit/finger"
)

// entryKind groups catalog entries under section headings. It is DISPLAY
// metadata only: bookmarks do not carry it, and routeEntry decides what a
// target actually returns from the response.
type entryKind uint8

const (
	kindUnknown entryKind = iota
	kindCommunity
	kindService
	kindPerson
)

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

// startEntry is one assembled row on the startpage. kind and note come only
// from the catalog; an unmatched bookmark leaves both at their zero values.
type startEntry struct {
	target string
	kind   entryKind
	note   string
	source entrySource
}

// parseProblem is a line we refused, surfaced to the user rather than swallowed.
type parseProblem struct {
	line   int
	reason string
}

// bookmarkFile is the parsed user file.
type bookmarkFile struct {
	targets       []string
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
		target, err := parseBookmarkTarget(line)
		if err != nil {
			out.problems = append(out.problems, parseProblem{line: lineNo, reason: err.Error()})
			continue
		}
		out.targets = append(out.targets, target)
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
		entry, err := parseCatalogLine(line)
		if err != nil {
			problems = append(problems, parseProblem{line: i + 1, reason: err.Error()})
			continue
		}
		entries = append(entries, entry)
	}
	return entries, problems
}

// stripComment drops a trailing "#" comment. Comments are preserved by the
// write path but never parsed or displayed.
func stripComment(line string) string {
	if i := strings.Index(line, "#"); i >= 0 {
		return line[:i]
	}
	return line
}

// parseBookmarkTarget accepts exactly one target token. Any other text is
// refused because bookmark records carry no display metadata.
func parseBookmarkTarget(line string) (string, error) {
	fields := strings.Fields(line)
	if len(fields) != 1 {
		return "", fmt.Errorf("expected one target, got %q", line)
	}
	if err := validateTarget(fields[0]); err != nil {
		return "", err
	}
	return fields[0], nil
}

// parseCatalogLine parses the maintainer-authored "<kind> <target> <note>"
// grammar. Catalog notes are compiled into the binary, never read from the
// user's file.
func parseCatalogLine(line string) (startEntry, error) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return startEntry{}, fmt.Errorf("expected \"<kind> <target> <note>\", got %q", line)
	}
	kind, ok := parseKind(fields[0])
	if !ok {
		return startEntry{}, fmt.Errorf("unknown kind %q (want community, service or person)", fields[0])
	}
	target := fields[1]
	if err := validateTarget(target); err != nil {
		return startEntry{}, err
	}

	note := strings.TrimSpace(line[strings.Index(line, target)+len(target):])
	return startEntry{target: target, kind: kind, note: note, source: sourceCatalog}, nil
}

// validateTarget screens a target from a config file. finger.ParseTarget rejects
// C0/DEL via hasControl, but not invalid UTF-8, UTF-8-encoded C1 controls, or
// the non-printing Unicode controls that sanitize visualizes in response bodies.
// A target is displayed in the list and breadcrumb, so all are refused here.
// Rejecting matches the treatment targets already get: bodies are visualized
// because they are content, a target is refused because it is something we send.
// See issue #49 for the same gap on targets from every other source.
func validateTarget(target string) error {
	if !utf8.ValidString(target) {
		return fmt.Errorf("target is not valid UTF-8")
	}
	if hasNonPrintingControl(target) {
		return fmt.Errorf("target contains a non-printing Unicode control")
	}
	if _, err := finger.ParseTarget(target); err != nil {
		return fmt.Errorf("bad target %q: %w", target, err)
	}
	return nil
}

func hasNonPrintingControl(s string) bool {
	for _, r := range s {
		if r >= 0x80 && r <= 0x9f || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
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
- Consumes: `parseBookmarks`, `parseBookmarkTarget` from Task 1
- Produces: `bookmarksPathFn` (stubbable package var), `resolveBookmarksPath() (string, error)`, `loadBookmarks() (bookmarkFile, string)`, `appendBookmarkLine([]byte, string) []byte`, `deleteBookmarkLine([]byte, string) []byte`, `saveBookmarkData(string, []byte) error`, `shortenHome(string) string`

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
	in := []byte("# my careful notes\n\ncatalog off\n\n@plan.cat\n")
	got := string(appendBookmarkLine(in, "@tilde.team"))
	want := "# my careful notes\n\ncatalog off\n\n@plan.cat\n@tilde.team\n"
	if got != want {
		t.Fatalf("appendBookmarkLine =\n%q\nwant\n%q", got, want)
	}
}

func TestAppendBookmarkLineToEmptyFile(t *testing.T) {
	got := string(appendBookmarkLine(nil, "@plan.cat"))
	if want := "@plan.cat\n"; got != want {
		t.Fatalf("appendBookmarkLine = %q, want %q", got, want)
	}
}

func TestDeleteBookmarkLinePreservesEverythingElse(t *testing.T) {
	in := []byte("# keep me\ncatalog off\n@plan.cat\njonathan@tilde.team\n\n# and me\n")
	got := string(deleteBookmarkLine(in, "@plan.cat"))
	want := "# keep me\ncatalog off\njonathan@tilde.team\n\n# and me\n"
	if got != want {
		t.Fatalf("deleteBookmarkLine =\n%q\nwant\n%q", got, want)
	}
}

func TestDeleteBookmarkLinePreservesMalformedMatch(t *testing.T) {
	in := []byte("@plan.cat hand-written description\n@plan.cat\n")
	got := string(deleteBookmarkLine(in, "@plan.cat"))
	want := "@plan.cat hand-written description\n"
	if got != want {
		t.Fatalf("deleteBookmarkLine = %q, want %q", got, want)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lookit", "bookmarks")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = resolveBookmarksPath })

	if err := saveBookmarkData(path, []byte("@plan.cat\n")); err != nil {
		t.Fatalf("saveBookmarkData() error = %v", err)
	}
	file, gotPath := loadBookmarks()
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if len(file.targets) != 1 || file.targets[0] != "@plan.cat" {
		t.Fatalf("targets = %+v", file.targets)
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
	if len(file.targets) != 0 || len(file.problems) != 0 {
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
func appendBookmarkLine(data []byte, target string) []byte {
	out := string(data)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return []byte(out + target + "\n")
}

// deleteBookmarkLine drops every valid bookmark record for target, leaving
// comments, malformed records, blank lines, directives and ordering untouched.
func deleteBookmarkLine(data []byte, target string) []byte {
	lines := strings.Split(string(data), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		parsed, err := parseBookmarkTarget(strings.TrimSpace(stripComment(line)))
		if err == nil && parsed == target {
			continue
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
# in ~/.config/lookit/bookmarks as one target per line, and "catalog off" there
# hides all of this.
#
# Every entry was probed live on 2026-08-11, and every note below is traceable
# to the server's own words, to 640kb.neocities.org/fingerverse, or to a
# conclusion the response plainly supports. The spec's catalog tables record the
# basis for each. Do not describe a host from memory: four notes in an earlier
# draft were wrong, including one that called tilde.team big when its own banner
# says small.
#
# When refreshing, probe SERIALLY — bbs.airandwave.net rate-limits and a
# concurrent sweep reports false deaths.

community @plan.cat Classic finger, polished for the present
community @tilde.team Small public access unix, for teaching and learning
community @happynetbox.com Finger server of user profiles, run by Ben Brown
community @telehack.com Live system status and users; .plan pages are autogenerated
community ring@thebackupbox.net The finger ring — join by linking it from your response
community @cosmic.voyage Collaborative science fiction; users crew ships
community @athena.dialup.mit.edu MIT Athena dialup, still answering
community @zaibatsu.circumlunar.space Circumlunar Space pubnix
community @chunboan.zone A tiny shared community on one cheap server

service @bbs.airandwave.net Menu of a dozen-plus finger services
service weather@bbs.airandwave.net Current weather and a 7-day forecast — weather:city@…
service @graph.no Weather worldwide by place name — finger oslo@graph.no
service quake@bbs.airandwave.net Latest earthquakes, M2.5+ past day
service dict@bbs.airandwave.net Dictionary lookup — dict:word@…
service urban@bbs.airandwave.net Slang, internet terms and memes — urban:word@…
service wordsearch:today@bbs.airandwave.net Daily word search puzzle
service sudoku:easy@bbs.airandwave.net An easy sudoku, fresh each day
service textfile@typed-hole.org A lucky dip into textfiles.com
service calendar@flanigan.us Today’s date, across the years
service bot@happynetbox.com News headlines, with links for the curious
service random@happynetbox.com Jump to a random happynetbox user
service browserversion@happynetbox.com The latest versions across the browser world
service 1@happynetbox.com Interactive fiction, chained over finger
service cyoa@typed-hole.org Choose your own adventure
service smog@typed-hole.org Saturday Morning Gemzine — back issues
service originsfinger@happynetbox.com Les Earnest tells how finger began
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
Expected: PASS — 26 entries parsed, all with notes, no people

- [ ] **Step 6: Run the full gate and commit**

```bash
make check
git add tui/catalog.go tui/catalog.txt tui/catalog_test.go
git commit -m "feat(tui): embed the curated startpage catalog"
```

---

### Task 4: Section assembly

Merging the two sources: target-only bookmarks first, catalog grouped and deduped, metadata borrowed only by exact target match.

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
		{target: "@tilde.team", kind: kindCommunity, note: "Small public access unix", source: sourceCatalog},
		{target: "@plan.cat", kind: kindCommunity, note: "Classic finger, polished for the present", source: sourceCatalog},
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
	bm := bookmarkFile{targets: []string{"@tilde.team"}}
	got := buildSections(catalogFixture(), bm)
	if got[0].title != "BOOKMARKS" {
		t.Fatalf("section 0 title = %q, want BOOKMARKS", got[0].title)
	}
	if len(got[0].entries) != 1 || got[0].entries[0].target != "@tilde.team" {
		t.Fatalf("bookmarks section = %+v", got[0].entries)
	}
	// The note travels with the target even though the file stores none.
	if got[0].entries[0].note != "Small public access unix" {
		t.Errorf("note = %q, want the catalog's note", got[0].entries[0].note)
	}
	// And it is suppressed from COMMUNITIES rather than appearing twice.
	for _, e := range got[1].entries {
		if e.target == "@tilde.team" {
			t.Error("@tilde.team appears in both BOOKMARKS and COMMUNITIES")
		}
	}
}

func TestBuildSectionsBookmarkWithoutCatalogMatchHasNoDescription(t *testing.T) {
	bm := bookmarkFile{targets: []string{"weather:99501@bbs.airandwave.net"}}
	got := buildSections(catalogFixture(), bm)
	entry := got[0].entries[0]
	if entry.note != "" {
		t.Fatalf("note = %q, want blank for an unclassified bookmark", entry.note)
	}
	if entry.kind != kindUnknown {
		t.Fatalf("kind = %v, want no inferred classification", entry.kind)
	}
}

func TestBuildSectionsCatalogOff(t *testing.T) {
	bm := bookmarkFile{
		catalogHidden: true,
		targets:       []string{"@plan.cat"},
	}
	got := buildSections(catalogFixture(), bm)
	if len(got) != 1 || got[0].title != "BOOKMARKS" {
		t.Fatalf("sections = %+v, want BOOKMARKS only", got)
	}
	// A hidden catalog still supplies notes for matching bookmarks.
	if got[0].entries[0].note != "Classic finger, polished for the present" {
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
// The bookmark file stores targets only. A catalog match supplies its authored
// metadata; an unmatched target stays unclassified with a blank description.
func buildSections(catalog []startEntry, bm bookmarkFile) []startSection {
	byTarget := make(map[string]startEntry, len(catalog))
	for _, e := range catalog {
		byTarget[e.target] = e
	}

	var sections []startSection

	if len(bm.targets) > 0 {
		bookmarked := make([]startEntry, 0, len(bm.targets))
		for _, target := range bm.targets {
			e := startEntry{target: target, source: sourceBookmark}
			if catalogEntry, ok := byTarget[target]; ok {
				e = catalogEntry
				e.source = sourceBookmark
			}
			bookmarked = append(bookmarked, e)
		}
		sections = append(sections, startSection{title: "BOOKMARKS", entries: bookmarked})
	}

	if bm.catalogHidden {
		return sections
	}

	pinned := make(map[string]bool, len(bm.targets))
	for _, target := range bm.targets {
		pinned[target] = true
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

### Task 5: `startModel` — the list with skipped non-entry rows

`bubbles/v2/list` has no section support and its pagination assumes a uniform `delegate.Height()`. Headers are therefore items rendered at the same two-row cell height as an entry, and the cursor steps over them. This is the fiddliest part of the feature; the edge tests are the point.

The catalog credit uses the same mechanism: a final two-row, non-selectable item
that carries an OSC-8-linked URL and drops out during filtering.

**Files:**
- Create: `tui/start.go`
- Create: `tui/start_test.go`

**Interfaces:**
- Consumes: `startSection`, `startEntry` from Task 4; `commonModel`, `styles`, `userDelegate`, `applyListStyles` (`tui/list.go:88-146`)
- Produces: `catalogCreditURL`, `startItem`, `startModel`, `newStart(common *commonModel, sections []startSection, notice, empty string) startModel`, and methods `update`, `View`, `setSize`, `selected() (startEntry, bool)`, `selectTarget(string) bool`, `filtering() bool`, `filterApplied() bool`, `applyStyles(styles)`

- [ ] **Step 1: Write the failing test**

```go
package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// testCommon is shared with list_test.go; do not redeclare it here.

func twoSections() []startSection {
	return []startSection{
		{title: "BOOKMARKS", entries: []startEntry{
			{target: "@tilde.team", kind: kindCommunity, note: "Small public access unix", source: sourceBookmark},
		}},
		{title: "COMMUNITIES", entries: []startEntry{
			{target: "@plan.cat", kind: kindCommunity, note: "Classic finger, polished for the present", source: sourceCatalog},
			{target: "@happynetbox.com", kind: kindCommunity, note: "Finger server of user profiles, run by Ben Brown", source: sourceCatalog},
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

func TestStartSelectTargetPreservesIdentity(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	if !m.selectTarget("@happynetbox.com") {
		t.Fatal("selectTarget returned false for an existing row")
	}
	got, ok := m.selected()
	if !ok || got.target != "@happynetbox.com" {
		t.Fatalf("selected = %+v, %v; want @happynetbox.com", got, ok)
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

func TestStartCursorSkipsHeaderAtPageBoundary(t *testing.T) {
	common := testCommon()
	common.height = 8 // force pagination with the two-row delegate
	m := newStart(common, twoSections(), "", "")
	m.list.Select(2) // the COMMUNITIES header, on a later page
	m.skipNonEntry(1)
	got, ok := m.selected()
	if !ok || got.target != "@plan.cat" {
		t.Fatalf("selected = %+v, %v; want @plan.cat after boundary header", got, ok)
	}
}

// At the last entry, down must not strand the cursor on the trailing credit.
func TestStartCursorStopsBeforeCredit(t *testing.T) {
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

func TestStartCatalogCreditIsLinkedAndNonSelectable(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	items := m.list.Items()
	credit, ok := items[len(items)-1].(startItem)
	if !ok || !credit.credit {
		t.Fatalf("last item = %#v, want catalog credit", items[len(items)-1])
	}
	m.list.Select(len(items) - 1)
	m.skipNonEntry(1)
	got, ok := m.selected()
	if !ok || got.target != "@happynetbox.com" {
		t.Fatalf("selected = %+v, %v; want last catalog entry", got, ok)
	}

	view := m.View()
	for _, want := range []string{
		"Catalog inspired by",
		lipgloss.NewStyle().Hyperlink(catalogCreditURL).Render(catalogCreditURL),
	} {
		if !strings.Contains(view, want) {
			t.Errorf("View() missing %q:\n%s", want, view)
		}
	}
}

func TestStartCatalogCreditRequiresCatalogRow(t *testing.T) {
	sections := []startSection{{title: "BOOKMARKS", entries: []startEntry{
		{target: "@tilde.team", source: sourceBookmark},
	}}}
	m := newStart(testCommon(), sections, "", "")
	if strings.Contains(m.View(), "Catalog inspired by") {
		t.Fatalf("View() contains catalog credit without a catalog row:\n%s", m.View())
	}
}

func TestStartFilterSelectsFirstMatchAfterHeadersDisappear(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	var cmd tea.Cmd
	for _, r := range "plan" {
		m, cmd = m.update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	msg, ok := findFilterMatches(cmd)
	if !ok {
		t.Fatal("filter command produced no list.FilterMatchesMsg")
	}
	m, _ = m.update(msg)
	for _, item := range m.list.VisibleItems() {
		if si, ok := item.(startItem); ok && si.credit {
			t.Fatal("catalog credit survived filtering")
		}
	}
	got, ok := m.selected()
	if !ok || got.target != "@plan.cat" {
		t.Fatalf("selected = %+v, %v; want first filtered row @plan.cat", got, ok)
	}
}

func findFilterMatches(cmd tea.Cmd) (tea.Msg, bool) {
	if cmd == nil {
		return nil, false
	}
	msg := cmd()
	if _, ok := msg.(list.FilterMatchesMsg); ok {
		return msg, true
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, child := range batch {
			if msg, ok := findFilterMatches(child); ok {
				return msg, true
			}
		}
	}
	return nil, false
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
	"charm.land/lipgloss/v2"
)

// startChromeRows matches listChromeRows: space the bubbles list reserves once
// its own title and help are hidden.
const (
	startChromeRows = 1
	catalogCreditURL = "https://640kb.neocities.org/fingerverse/"
)

// startItem is one row: an entry, section header, or catalog credit. Non-entry
// rows occupy normal item slots so the list's uniform-height pagination holds.
type startItem struct {
	entry  startEntry
	header string // non-empty => this row is a section heading
	credit bool
}

func (i startItem) selectable() bool {
	return i.header == "" && !i.credit && i.entry.target != ""
}

// FilterValue drives "/". Non-entry rows return "" so they drop out while
// filtering, which flattens the view to matches — the behaviour we want.
func (i startItem) FilterValue() string {
	if !i.selectable() {
		return ""
	}
	return i.entry.target + " " + i.entry.note
}

func (i startItem) Title() string {
	if i.header != "" {
		return i.header
	}
	if i.credit {
		return "Catalog inspired by"
	}
	return i.entry.target
}

func (i startItem) Description() string {
	if i.header != "" {
		return ""
	}
	if i.credit {
		return catalogCreditURL
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
	hasCatalogRow := false
	for _, s := range sections {
		items = append(items, startItem{header: s.title})
		for _, e := range s.entries {
			items = append(items, startItem{entry: e})
			if e.source == sourceCatalog {
				hasCatalogRow = true
			}
		}
	}
	if hasCatalogRow {
		items = append(items, startItem{credit: true})
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
	m.skipNonEntry(1) // never rest on the leading header
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
	if it, ok := item.(startItem); ok && it.credit {
		dim := lipgloss.NewStyle().Foreground(d.st.palette.Dim)
		url := lipgloss.NewStyle().Hyperlink(catalogCreditURL).Render(catalogCreditURL)
		fmt.Fprintf(w, "%s\n%s", dim.Render("Catalog inspired by"), dim.Render(url)) //nolint:errcheck
		return
	}
	d.userDelegate.Render(w, m, index, item)
}

func (m startModel) update(msg tea.Msg) (startModel, tea.Cmd) {
	before := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if _, ok := msg.(list.FilterMatchesMsg); ok {
		// Filtering removes headers and the credit. The unfiltered cursor starts
		// at 1 to skip the leading header, so reset it to the first filtered row.
		m.list.Select(0)
		m.skipNonEntry(1)
		return m, cmd
	}
	if after := m.list.Index(); after != before {
		dir := 1
		if after < before {
			dir = -1
		}
		m.skipNonEntry(dir)
	}
	return m, cmd
}

// skipNonEntry advances past a header or credit in the direction of travel,
// reversing at the ends so the cursor can never rest on a non-entry row.
func (m *startModel) skipNonEntry(dir int) {
	items := m.list.VisibleItems()
	if len(items) == 0 {
		return
	}
	idx := m.list.Index()
	for range len(items) {
		it, ok := items[idx].(startItem)
		if !ok || it.selectable() {
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

// selected returns the highlighted entry. A non-entry row or empty list yields false.
func (m startModel) selected() (startEntry, bool) {
	it, ok := m.list.SelectedItem().(startItem)
	if !ok || !it.selectable() {
		return startEntry{}, false
	}
	return it.entry, true
}

// selectTarget restores selection by stable identity after a startpage reload.
func (m *startModel) selectTarget(target string) bool {
	for i, item := range m.list.VisibleItems() {
		entry, ok := item.(startItem)
		if ok && entry.selectable() && entry.entry.target == target {
			m.list.Select(i)
			return true
		}
	}
	return false
}

func (m startModel) filtering() bool {
	return m.list.FilterState() == list.Filtering
}

func (m startModel) filterApplied() bool {
	return m.list.FilterState() == list.FilterApplied
}

func (m *startModel) applyStyles(st styles) {
	applyListStyles(&m.list, st)
	m.list.SetDelegate(startDelegate{userDelegate: defaultUserDelegate(st), st: st})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run TestStart -count=1 -v`
Expected: PASS — all cursor-skip edges and catalog-credit cases

- [ ] **Step 5: Run the full gate and commit**

```bash
make check
git add tui/start.go tui/start_test.go
git commit -m "feat(tui): add the catalog startpage list"
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

func TestStartEnterRequestsSelectedTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookmarks")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = resolveBookmarksPath })

	fetch, seen := fetchRecorder("Plan: hello\n")
	m := newApp(fetch, colorprofile.NoTTY)
	m.blurInput()
	selected, ok := m.start.selected()
	if !ok {
		t.Fatal("startpage has no selected target")
	}

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter produced no request command")
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != selected.target {
		t.Fatalf("requested = %v, want [%s]", *seen, selected.target)
	}
}

func TestStartNoticeNamesResolvedPath(t *testing.T) {
	file := bookmarkFile{problems: []parseProblem{{line: 3, reason: "expected one target"}}}
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

Add `"path/filepath"` to `tui/app_test.go`'s imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run 'TestAppOpensOnStartpage|TestStartEnter|TestStartNotice|TestStartEmptyMessage' -count=1`
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

In `handleKey`'s content switch, route Enter on a startpage row through the
normal request lifecycle:

```go
	case key.Matches(msg, m.keys.Open) && m.state == stateStart:
		entry, ok := m.start.selected()
		if !ok {
			return true, m, nil
		}
		target, err := finger.ParseTarget(entry.target)
		if err != nil {
			return true, m, m.setFlash("error: " + err.Error())
		}
		return true, m, m.startRequest(target, requestNavigate, false)
```

In `updateKeymap`, define startpage content alongside `inList`, then include it
in the Open and Filter bindings:

```go
	inStart := content && m.state == stateStart
	_, startHasSelection := m.start.selected()
	startHasSelection = inStart && startHasSelection

	m.keys.Open.SetEnabled(m.inputFocused || inList || startHasSelection)
	m.keys.Filter.SetEnabled(inList || startHasSelection)
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
		if si, ok := it.(startItem); ok && si.selectable() {
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

Delete `landingBar` from `tui/statusbar.go:64-67`. Replace
`TestStatusBarLandingShowsHint` in `tui/statusbar_test.go` with:

```go
func TestStatusBarStartShowsFocusedInputHint(t *testing.T) {
	m := appModel{inputFocused: true}
	out := m.startBar(80, newStyles(true)).render()
	if !strings.Contains(out, "type a target") {
		t.Fatalf("start bar %q missing focused-input hint", out)
	}
}

func TestStatusBarStartDoesNotCountCatalogCredit(t *testing.T) {
	m := appModel{start: newStart(testCommon(), twoSections(), "", "")}
	out := m.startBar(80, newStyles(true)).render()
	if !strings.Contains(out, "3 entries") {
		t.Fatalf("start bar %q should count only selectable entries", out)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tui/ -run 'TestAppOpensOnStartpage|TestStartEnter|TestStartNotice|TestStartEmptyMessage|TestStatusBarStart' -count=1 -v`
Expected: PASS

- [ ] **Step 5: Update the existing landing assertions**

Make these exact expectation changes in `tui/app_test.go`.

Rename `TestEscInListReturnsToReaderHome` to `TestEscInListReturnsToStart` and
replace its final state assertion with:

```go
	if got.state != stateStart || got.pos != -1 {
		t.Fatalf("state=%d pos=%d, want start/-1 after Esc", got.state, got.pos)
	}
```

Rename `TestLandingViewShowsLandingBar` to `TestLandingViewShowsStartpage` and
replace its assertions with:

```go
	view := m.View().Content
	for _, want := range []string{"type a target", "@plan.cat"} {
		if !strings.Contains(view, want) {
			t.Fatalf("startpage missing %q:\n%s", want, view)
		}
	}
```

In `TestBackToLandingShowsBareTargetRow`, replace the post-back assertions with:

```go
	if m.pos != -1 || m.state != stateStart {
		t.Fatalf("back-to-start state=%d pos=%d, want start/-1", m.state, m.pos)
	}
	view := stripANSIForLandingTest(m.View().Content)
	for _, want := range []string{"target:", "@plan.cat"} {
		if !strings.Contains(view, want) {
			t.Fatalf("back-to-start view missing %q:\n%s", want, view)
		}
	}
```

Update landing comments that say `stateReader` to say `stateStart`; do not
change tests whose setup explicitly installs a fetched `stateReader` node.

Then run: `go test ./tui/ -count=1`
Expected: PASS

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
- Produces: `keyMap.Bookmark`, `keyMap.Home`, `(*appModel).toggleBookmark() tea.Cmd`, `(appModel).bookmarkTarget() (string, bool)`, `(*appModel).goHome()`

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
	if err := os.WriteFile(path, []byte("@tilde.team\n"), 0o600); err != nil {
		t.Fatalf("seed bookmarks: %v", err)
	}

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	if !m.start.selectTarget("@plan.cat") {
		t.Fatal("catalog row @plan.cat not found")
	}
	first, ok := m.start.selected()
	if !ok {
		t.Fatal("no selection to bookmark")
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after bookmark: %v", err)
	}
	if want := "@tilde.team\n@plan.cat\n"; string(data) != want {
		t.Fatalf("file = %q, want %q", data, want)
	}
	selected, ok := m.start.selected()
	if !ok || selected.target != first.target {
		t.Fatalf("selection after reload = %+v, %v; want %q", selected, ok, first.target)
	}

	// Selection follows the moved row, so pressing b again removes the same target.
	next, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after unbookmark: %v", err)
	}
	if want := "@tilde.team\n"; string(data) != want {
		t.Fatalf("file = %q, want %q (existing bookmark preserved)", data, want)
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

	next, _ := m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = next.(appModel)
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

Add `"os"` and `"path/filepath"` to `tui/app_test.go`'s imports if Task 6 has
not already added the latter.

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
func (m appModel) bookmarkTarget() (string, bool) {
	if m.state == stateStart {
		entry, ok := m.start.selected()
		return entry.target, ok
	}
	if m.pos < 0 || m.pos >= len(m.history) {
		return "", false
	}
	return m.history[m.pos].entry.Target.Raw, true
}

// toggleBookmark adds or removes the current target, then reloads the startpage
// so it reflects the file. Bookmark records contain only the target: the
// protocol cannot establish a kind, and routing remains response-derived.
func (m *appModel) toggleBookmark() tea.Cmd {
	target, ok := m.bookmarkTarget()
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
	for _, saved := range file.targets {
		if saved == target {
			already = true
			break
		}
	}

	var updated []byte
	var msg string
	if already {
		updated = deleteBookmarkLine(data, target)
		msg = "✓ removed " + target
	} else {
		updated = appendBookmarkLine(data, target)
		msg = "✓ bookmarked " + target
	}
	if err := saveBookmarkData(path, updated); err != nil {
		return m.setFlash("error: " + err.Error())
	}
	if m.state == stateStart {
		m.reloadStart()
		m.start.selectTarget(target)
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

func TestStartpageFilterOwnsCommandLetters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookmarks")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = resolveBookmarksPath })

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	next, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = next.(appModel)
	if !m.start.filtering() {
		t.Fatal("/ did not enter startpage filtering")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	if got := m.start.list.FilterInput.Value(); got != "b" {
		t.Fatalf("filter = %q, want b", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("typing b in the filter changed bookmarks: stat error = %v", err)
	}
}

func TestStartpageEscClearsAppliedFilterBeforeChangingFocus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookmarks")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = resolveBookmarksPath })

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	m.start.list.SetFilterText("plan")
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(appModel)
	if m.start.list.FilterState() != list.Unfiltered {
		t.Fatalf("filter state = %v, want unfiltered", m.start.list.FilterState())
	}
	if m.inputFocused {
		t.Fatal("first Esc should clear the applied filter, not focus the input")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run 'TestStartpageArrowDown|TestStartpageEsc|TestBookmarkKeyTypes|TestStartpageFilter' -count=1`
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

At the top of the content-focused branch, extend the existing list-filter guard
so the startpage list owns every key while editing a filter, and so Esc clears an
applied filter before changing focus:

```go
	if m.state == stateStart && m.start.filtering() {
		return false, m, nil
	}
	if m.state == stateStart && m.start.filterApplied() && key.Matches(msg, m.keys.Back) {
		return false, m, nil
	}
```

Then, before the generic `Back` handling:

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

Run: `go test ./tui/ -run 'TestStartpageArrowDown|TestStartpageEsc|TestBookmarkKeyTypes|TestStartpageFilter' -count=1 -v`
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
  (`tui/bookmarks.go`). It admits target-only records: each target must be valid
  UTF-8, must survive `ParseTarget`, and must contain no C1, Cf, Zl or Zp control.
  Descriptions and classifications come only from matching entries in the
  embedded catalog we author; comments and malformed trailing text are never
  displayed. Nothing from that file needs `sanitize`, because nothing
  unvalidated is ever displayed. If a future ingress does admit free text, it
  must call `sanitize` itself and this note must change.
```

In the `tui/` bullet, add to the state enum description that `stateStart` is the launch screen at `pos == -1` and is never a history node.

- [ ] **Step 2: Update the README**

Replace the first "Coming soon" bullet about discovery:

```markdown
- Discovery and subscriptions: following a `.plan` to see what's changed since you last looked.
```

And in the Usage section, after the paragraph beginning "Type a target and press Enter", add:

```markdown
lookit opens on a startpage: a built-in catalog of finger communities and services, with your own bookmarks pinned above it. Press `↓` to browse it, `↵` to go, and `b` to bookmark whatever you're looking at. Bookmarks live in `~/.config/lookit/bookmarks` (or `$XDG_CONFIG_HOME/lookit/bookmarks`), one `<target>` per line, so you can edit them by hand; add `catalog off` there to hide the built-in list entirely. Press `h` to return to the startpage from anywhere.
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

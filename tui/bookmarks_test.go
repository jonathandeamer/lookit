package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain points every test at an existing empty bookmarks file by default, so
// ordinary tests do not trigger initialization while remaining isolated from the
// real user's config. Tests that need a live file call useTempBookmarks.
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

// useBookmarksPath points bookmarksPathFn at path for one test, restoring the
// package default afterwards.
func useBookmarksPath(t *testing.T, path string) {
	t.Helper()
	saved := bookmarksPathFn
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = saved })
}

// useTempBookmarks points bookmarksPathFn at a fresh existing empty file for
// one test, restoring the package default afterwards.
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

// useMissingTempBookmarks points bookmarksPathFn at a missing path for one test,
// restoring the package default afterwards.
func useMissingTempBookmarks(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lookit", "bookmarks")
	useBookmarksPath(t, path)
	return path
}

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
	path := useTempBookmarks(t)

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

func TestSaveBookmarkDataPreservesSymlink(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	dotfilesDir := filepath.Join(root, "dotfiles")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.MkdirAll(dotfilesDir, 0o700); err != nil {
		t.Fatalf("create dotfiles dir: %v", err)
	}
	target := filepath.Join(dotfilesDir, "bookmarks")
	if err := os.WriteFile(target, []byte("@old.example\n"), 0o600); err != nil {
		t.Fatalf("seed symlink target: %v", err)
	}
	path := filepath.Join(configDir, "bookmarks")
	if err := os.Symlink(filepath.Join("..", "dotfiles", "bookmarks"), path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := saveBookmarkData(path, []byte("@new.example\n")); err != nil {
		t.Fatalf("saveBookmarkData() error = %v", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat bookmarks path: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("bookmarks path mode = %v, want symlink", info.Mode())
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read symlink target: %v", err)
	}
	if got, want := string(data), "@new.example\n"; got != want {
		t.Fatalf("symlink target = %q, want %q", got, want)
	}
}

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

func TestInitializeBookmarkDataReadsConcurrentWinnerWithoutClobbering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lookit", "bookmarks")
	if _, err := os.ReadFile(path); !os.IsNotExist(err) {
		t.Fatalf("initial read error = %v, want not exist", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create bookmark directory: %v", err)
	}
	winner := []byte("@plan.cat\n")
	if err := os.WriteFile(path, winner, 0o600); err != nil {
		t.Fatalf("publish concurrent winner: %v", err)
	}

	file := initializeBookmarkData(path, appendBookmarkLine(nil, aboutFingerAuthor))
	if len(file.problems) != 0 {
		t.Fatalf("problems = %+v, want none", file.problems)
	}
	if len(file.targets) != 1 || file.targets[0] != "@plan.cat" {
		t.Fatalf("targets = %+v, want [@plan.cat]", file.targets)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read concurrent winner: %v", err)
	}
	if got := string(data); got != string(winner) {
		t.Fatalf("bookmarks = %q, want concurrent winner %q", got, winner)
	}
}

func TestInitializeBookmarkDataReportsConcurrentWinnerReadFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bookmarks")
	if _, err := os.ReadFile(path); !os.IsNotExist(err) {
		t.Fatalf("initial read error = %v, want not exist", err)
	}
	if err := os.Symlink("missing-target", path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	file := initializeBookmarkData(path, appendBookmarkLine(nil, aboutFingerAuthor))
	if len(file.targets) != 0 {
		t.Fatalf("targets = %+v, want none", file.targets)
	}
	if len(file.problems) != 1 || file.problems[0].line != 0 ||
		!strings.HasPrefix(file.problems[0].reason, "cannot read: ") {
		t.Fatalf("problems = %+v, want one line-zero cannot-read problem", file.problems)
	}
}

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
	// Deleting the only line leaves an existing zero-byte file, which remains
	// authoritative rather than restoring the author bookmark.
	if got, want := string(data), ""; got != want {
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

func TestShortenHomePath(t *testing.T) {
	root := string(os.PathSeparator)
	home := filepath.Join(root, "users", "alice")
	descendant := filepath.Join(home, ".config", "lookit", "bookmarks")
	sibling := filepath.Join(root, "users", "alice-backup", "lookit", "bookmarks")
	rootDescendant := filepath.Join(root, "var", "lib", "lookit", "bookmarks")

	tests := []struct {
		name string
		path string
		home string
		want string
	}{
		{name: "exact home", path: home, home: home, want: "~"},
		{name: "descendant", path: descendant, home: home, want: filepath.Join("~", ".config", "lookit", "bookmarks")},
		{name: "sibling prefix", path: sibling, home: home, want: sibling},
		{name: "root home", path: rootDescendant, home: root, want: filepath.Join("~", "var", "lib", "lookit", "bookmarks")},
		{name: "separator-suffixed home", path: descendant, home: home + root, want: filepath.Join("~", ".config", "lookit", "bookmarks")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortenHomePath(tt.path, tt.home); got != tt.want {
				t.Fatalf("shortenHomePath(%q, %q) = %q, want %q", tt.path, tt.home, got, tt.want)
			}
		})
	}
}

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

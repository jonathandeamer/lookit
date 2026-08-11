package tui

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain points every test at a nonexistent bookmarks path by default, so no
// test in this package can read (or write) the real user's config. Tests that
// need a live file call useTempBookmarks.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lookit-tui-test-*")
	if err != nil {
		panic(err)
	}
	bookmarksPathFn = func() (string, error) { return filepath.Join(dir, "absent"), nil }
	code := m.Run()
	os.RemoveAll(dir) //nolint:errcheck // best-effort cleanup
	os.Exit(code)
}

// useTempBookmarks points bookmarksPathFn at a fresh file in t.TempDir for the
// duration of one test, restoring the package default afterwards. The file is
// not created: callers that want seeded content write it themselves.
func useTempBookmarks(t *testing.T) string {
	t.Helper()
	saved := bookmarksPathFn
	path := filepath.Join(t.TempDir(), "lookit", "bookmarks")
	bookmarksPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { bookmarksPathFn = saved })
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

func TestLoadBookmarksMissingFileIsNotAnError(t *testing.T) {
	path := useTempBookmarks(t) // deliberately never created

	file, gotPath := loadBookmarks()
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if len(file.targets) != 0 || len(file.problems) != 0 {
		t.Fatalf("file = %+v, want empty", file)
	}
}

func TestShortenHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/home-example")
	if got, want := shortenHome("/tmp/home-example/.config/lookit/bookmarks"), "~/.config/lookit/bookmarks"; got != want {
		t.Errorf("shortenHome = %q, want %q", got, want)
	}
	if got, want := shortenHome("/tmp/xdg/lookit/bookmarks"), "/tmp/xdg/lookit/bookmarks"; got != want {
		t.Errorf("shortenHome = %q, want %q unchanged", got, want)
	}
}

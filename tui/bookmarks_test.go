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

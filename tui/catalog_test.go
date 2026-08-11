package tui

import (
	"strings"
	"testing"
)

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
		// stripComment cuts at the first '#', so one in a note would silently
		// truncate the description rather than fail to parse.
		if strings.Contains(e.note, "#") {
			t.Errorf("%s note contains '#', which the comment stripper would eat: %q", e.target, e.note)
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

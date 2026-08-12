package tui

import (
	"reflect"
	"strings"
	"testing"
)

func TestCatalogRawNoteValidationRejectsCommentMarker(t *testing.T) {
	data := []byte("# heading\ncommunity @plan.cat Fine note\nservice date@example.com Date # accidentally truncated\n")
	if got, want := catalogNoteCommentLines(data), []int{3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("comment-marker lines = %v, want %v", got, want)
	}
}

func catalogNoteCommentLines(data []byte) []int {
	var lines []int
	for i, raw := range strings.Split(string(data), "\n") {
		record := strings.TrimSpace(raw)
		if record == "" || strings.HasPrefix(record, "#") {
			continue
		}
		fields := strings.Fields(record)
		if len(fields) >= 3 && strings.Contains(strings.Join(fields[2:], " "), "#") {
			lines = append(lines, i+1)
		}
	}
	return lines
}

// TestCatalogIsWellFormed guards hand-edited data that ships compiled in: a
// typo cannot be corrected without cutting a release, so it must not merge.
func TestCatalogIsWellFormed(t *testing.T) {
	if lines := catalogNoteCommentLines(catalogData); len(lines) != 0 {
		t.Fatalf("catalog notes contain '#', which the comment stripper would eat, on lines %v", lines)
	}
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

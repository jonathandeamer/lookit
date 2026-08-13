package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
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

	// Keyed by kind and target: a dual-role host carries both its listing and a
	// "group" line naming the same target, and only a repeated *listing* is the
	// duplicate this guards against.
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.note == "" {
			t.Errorf("%s has no note; every catalog entry must describe itself", e.target)
		}
		if e.source != sourceCatalog {
			t.Errorf("%s source = %v, want sourceCatalog", e.target, e.source)
		}
		key := fmt.Sprintf("%d %s", e.kind, e.target)
		if seen[key] {
			t.Errorf("%s appears twice with the same kind", e.target)
		}
		seen[key] = true
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
	roots := catalogRootHosts(entries)
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

// catalogRootHosts is the set of hosts with a root LISTING. A group line shares
// a root's shape — an empty query token — but supplies a note rather than a
// row, so it must not satisfy the parent invariant on its own.
func catalogRootHosts(entries []startEntry) map[string]bool {
	roots := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.kind != kindGroup && entryToken(e.target) == "" {
			roots[entryHost(e.target)] = true
		}
	}
	return roots
}

// A group line describes a SERVICES group header. One naming a host that has no
// header to describe is dead data: silently ignored at runtime, so it has to
// fail here instead.
func TestCatalogGroupLinesDescribeARealGroup(t *testing.T) {
	entries, _ := parseCatalogData(catalogData)
	roots := catalogRootHosts(entries)
	children := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.kind == kindService && entryToken(e.target) != "" {
			children[entryHost(e.target)] = true
		}
	}
	for _, e := range entries {
		if e.kind != kindGroup {
			continue
		}
		host := entryHost(e.target)
		if entryToken(e.target) != "" {
			t.Errorf("group %s names a query, not a host; a group note belongs to a host root", e.target)
			continue
		}
		if !roots[host] {
			t.Errorf("group %s has no root listing to head", e.target)
		}
		if !children[host] {
			t.Errorf("group %s has no service children, so nothing renders its note", e.target)
		}
	}
}

// startNoteMaxCells is the widest a catalog note may render. The note column is
// half the startpage width less the frame, so this is what fits at 100 columns
// — the width the spec guarantees. Below that, notes truncate as they always
// have.
const startNoteMaxCells = 48

// catalogNoteWidths measures every note in display cells, keyed by "<kind>
// <target>" — a dual-role host has both a listing note and a group note, and
// both have to fit. Cells, not runes: rendering truncates by terminal width
// (ansi.Truncate), and a CJK character or emoji occupies two cells while
// counting as one rune.
func catalogNoteWidths(data []byte) map[string]int {
	widths := make(map[string]int)
	for _, raw := range strings.Split(string(data), "\n") {
		record := strings.TrimSpace(raw)
		if record == "" || strings.HasPrefix(record, "#") {
			continue
		}
		fields := strings.SplitN(strings.Join(strings.Fields(record), " "), " ", 3)
		if len(fields) < 3 {
			continue
		}
		widths[fields[0]+" "+fields[1]] = ansi.StringWidth(fields[2])
	}
	return widths
}

// A 48-rune note can be wider than 48 cells, and it is cells that truncate.
// This fixture fails the gate only if the measure is display width.
func TestCatalogNoteWidthMeasuresCellsNotRunes(t *testing.T) {
	wide := strings.Repeat("的", 30) // 30 runes, 60 cells
	data := []byte("service wide@example.com " + wide + "\n")
	got := catalogNoteWidths(data)["service wide@example.com"]
	if got != 60 {
		t.Fatalf("width = %d, want 60 cells for %d runes", got, len([]rune(wide)))
	}
	if got <= startNoteMaxCells {
		t.Fatalf("fixture width %d does not exceed the cap; it cannot prove the gate bites", got)
	}
}

func TestCatalogNotesFitTheNoteColumn(t *testing.T) {
	for record, width := range catalogNoteWidths(catalogData) {
		if width > startNoteMaxCells {
			t.Errorf("%q note is %d cells, want at most %d", record, width, startNoteMaxCells)
		}
	}
}

// The catalog grammar is <kind> <target> <note>. A "|" is no longer a
// delimiter; a hint line surviving the removal would silently become part of
// the note instead of failing loudly, so this guards against a leftover.
// Comment lines are skipped, matching catalogNoteCommentLines/catalogNoteWidths:
// comment lines are not data, and every guard in this file skips them.
func TestCatalogCarriesNoHints(t *testing.T) {
	for i, raw := range strings.Split(string(catalogData), "\n") {
		record := strings.TrimSpace(raw)
		if record == "" || strings.HasPrefix(record, "#") {
			continue
		}
		if strings.Contains(record, "|") {
			t.Errorf("line %d contains \"|\": %q; the hint grammar is gone, so a pipe is now just note text and is probably a leftover", i+1, record)
		}
	}
}

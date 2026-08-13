package tui

import (
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
	if lines := catalogNotePipeLines(catalogData); len(lines) != 0 {
		t.Fatalf("catalog notes contain a stray '|' outside the ' | ' delimiter, on lines %v", lines)
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

// hintIsMisplaced reports whether e carries a hint somewhere it can never
// render: a hint renders only where a token renders, on a service child.
// "Not a root" is too loose — a queried community such as
// ring@thebackupbox.net is non-root but is never grouped, so a hint on it
// would never appear either. Shared by TestCatalogHintsOnlyOnServiceChildren
// and its meta-guard, TestCatalogHintValidationRejectsNonChildren, so the two
// tests can't drift apart from each other.
func hintIsMisplaced(e startEntry) bool {
	return e.hint != "" && (e.kind != kindService || entryToken(e.target) == "")
}

func TestCatalogHintsOnlyOnServiceChildren(t *testing.T) {
	entries, _ := parseCatalogData(catalogData)
	for _, e := range entries {
		if hintIsMisplaced(e) {
			t.Errorf("%s carries hint %q but never renders as a token", e.target, e.hint)
		}
	}
}

func TestCatalogHintValidationRejectsNonChildren(t *testing.T) {
	data := []byte(strings.Join([]string{
		"community ring@thebackupbox.net The finger ring | the ring",
		"service @graph.no Weather worldwide | weather",
	}, "\n"))
	entries, problems := parseCatalogData(data)
	if len(problems) != 0 {
		t.Fatalf("parse problems = %+v, want none; the grammar is valid, the placement is not", problems)
	}
	for _, e := range entries {
		if hintIsMisplaced(e) {
			return
		}
	}
	t.Fatal("fixture did not produce a misplaced hint; the guard cannot be trusted")
}

// startNoteMaxCells is the widest a catalog note may render. The note column is
// half the startpage width less the frame, so this is what fits at 100 columns
// — the width the spec guarantees. Below that, notes truncate as they always
// have.
const startNoteMaxCells = 48

// catalogNoteWidths measures every note in display cells, keyed by target.
// Cells, not runes: rendering truncates by terminal width (ansi.Truncate), and
// a CJK character or emoji occupies two cells while counting as one rune.
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
		widths[fields[1]] = ansi.StringWidth(fields[2])
	}
	return widths
}

// A 48-rune note can be wider than 48 cells, and it is cells that truncate.
// This fixture fails the gate only if the measure is display width.
func TestCatalogNoteWidthMeasuresCellsNotRunes(t *testing.T) {
	wide := strings.Repeat("的", 30) // 30 runes, 60 cells
	data := []byte("service wide@example.com " + wide + "\n")
	got := catalogNoteWidths(data)["wide@example.com"]
	if got != 60 {
		t.Fatalf("width = %d, want 60 cells for %d runes", got, len([]rune(wide)))
	}
	if got <= startNoteMaxCells {
		t.Fatalf("fixture width %d does not exceed the cap; it cannot prove the gate bites", got)
	}
}

func TestCatalogNotesFitTheNoteColumn(t *testing.T) {
	for target, width := range catalogNoteWidths(catalogData) {
		if width > startNoteMaxCells {
			t.Errorf("%s note is %d cells, want at most %d", target, width, startNoteMaxCells)
		}
	}
}

func TestCatalogRawNoteValidationRejectsStrayPipe(t *testing.T) {
	data := []byte(strings.Join([]string{
		"# heading",
		"community @plan.cat Fine note",
		"service weird@example.com Contains a stray|pipe",
		"service twice@example.com Split note | short hint | extra",
		"service ok@example.com Split note | short hint",
	}, "\n"))
	if got, want := catalogNotePipeLines(data), []int{3, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pipe lines = %v, want %v", got, want)
	}
}

// catalogNotePipeLines flags any "|" that is not part of exactly one " | "
// delimiter: a bare pipe with no surrounding spaces (which splitCatalogNote
// would silently leave inside the note, since it cuts on " | " and not "|"),
// and more than one " | " delimiter (ambiguous — splitCatalogNote only ever
// honours the first).
func catalogNotePipeLines(data []byte) []int {
	var lines []int
	for i, raw := range strings.Split(string(data), "\n") {
		record := strings.TrimSpace(raw)
		if record == "" || strings.HasPrefix(record, "#") {
			continue
		}
		total := strings.Count(record, "|")
		delims := strings.Count(record, " | ")
		if total != 0 && (delims != 1 || total != delims) {
			lines = append(lines, i+1)
		}
	}
	return lines
}

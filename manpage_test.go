package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/jonathandeamer/lookit/render"
)

const manPagePath = "man/lookit.1"

// runOptionPattern finds the option strings run() matches on, by reading the
// case labels in main.go. Deriving them from the source rather than listing
// them here is the point: a new flag added to run() with no man-page entry
// fails this test, which is the only thing keeping the two in step.
var runOptionPattern = regexp.MustCompile(`"(-{1,2}[A-Za-z][A-Za-z0-9-]*)"`)

func runOptions(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	var opts []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(src), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "case ") {
			continue
		}
		for _, m := range runOptionPattern.FindAllStringSubmatch(line, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				opts = append(opts, m[1])
			}
		}
	}
	if len(opts) == 0 {
		t.Fatal("found no option literals in main.go's case labels; the pattern has drifted")
	}
	return opts
}

func TestManPageDocumentsEveryOption(t *testing.T) {
	page, err := os.ReadFile(manPagePath)
	if err != nil {
		t.Fatalf("read %s: %v", manPagePath, err)
	}
	// roff escapes a leading hyphen as "\-" so it renders as a minus rather
	// than a typographic dash; unescape before matching.
	text := strings.ReplaceAll(string(page), `\-`, "-")
	for _, opt := range runOptions(t) {
		if !strings.Contains(text, opt) {
			t.Errorf("%s does not document %q", manPagePath, opt)
		}
	}
}

// TestTheThreeSurfacesPointAtEachOther guards the division of labour between
// lookit's three documentation surfaces, each of which is deliberately
// incomplete on its own:
//
//   - --help is a reminder of what to type, so it hands off to "?" for the keys
//     and to the man page for everything else;
//   - the man page is the full reference, but cannot list context-aware keys, so
//     it hands off to "?" too;
//   - the README sells and installs, and defers the bookmarks-file grammar to
//     the man page rather than restating it.
//
// A handoff that goes missing turns its surface into a dead end, and nothing
// else would notice.
func TestTheThreeSurfacesPointAtEachOther(t *testing.T) {
	help := render.Usage(colorprofile.NoTTY)
	for _, want := range []string{"man lookit", "?"} {
		if !strings.Contains(help, want) {
			t.Errorf("--help does not point at %q", want)
		}
	}

	page, err := os.ReadFile(manPagePath)
	if err != nil {
		t.Fatalf("read %s: %v", manPagePath, err)
	}
	if !strings.Contains(string(page), `\fB?\fR`) {
		t.Errorf("%s does not point at \"?\" for the keys it leaves out", manPagePath)
	}

	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !strings.Contains(string(readme), "man lookit") {
		t.Error("README.md does not point at the man page")
	}
}

// TestManPageDocumentsTheBookmarksFile guards the man page's reason for
// existing: it is where the bookmarks file is specified, so the README does not
// have to be. Each token below is a piece of the grammar a reader cannot guess.
func TestManPageDocumentsTheBookmarksFile(t *testing.T) {
	page, err := os.ReadFile(manPagePath)
	if err != nil {
		t.Fatalf("read %s: %v", manPagePath, err)
	}
	text := string(page)
	for _, want := range []string{
		"~/.config/lookit/bookmarks",
		"XDG_CONFIG_HOME",
		"catalog off",
		"catalog on",
		"sort manual",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("%s does not mention %q", manPagePath, want)
		}
	}
}

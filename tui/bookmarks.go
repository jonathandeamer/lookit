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
// write path but never parsed or displayed. The cut is at the FIRST "#", so a
// catalog note containing one would be truncated; TestCatalogIsWellFormed
// forbids that rather than complicating the grammar for a case no entry needs.
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
	// SplitN, not Fields: the note is everything after the second token and must
	// keep its interior spacing. Locating it by searching for the target would
	// work on today's data but silently depends on no target being a substring
	// of its own kind word.
	fields := strings.SplitN(strings.Join(strings.Fields(line), " "), " ", 3)
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
	return startEntry{target: target, kind: kind, note: fields[2], source: sourceCatalog}, nil
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

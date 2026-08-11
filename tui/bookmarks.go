package tui

import (
	"fmt"
	"os"
	"path/filepath"
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

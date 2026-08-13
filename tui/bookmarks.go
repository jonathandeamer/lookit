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
	// kindGroup is not a section: a "group" line is metadata carrying the note
	// for a host's SERVICES group header, used when that header is a structural
	// copy of a row listed elsewhere. It never renders as a row of its own.
	kindGroup
)

func parseKind(s string) (entryKind, bool) {
	switch s {
	case "community":
		return kindCommunity, true
	case "service":
		return kindService, true
	case "person":
		return kindPerson, true
	case "group":
		return kindGroup, true
	}
	return 0, false
}

// entrySource records whether an entry is rendered from BOOKMARKS or from the
// catalog. Bookmark state is separate because a retained catalog parent can
// represent a target that is also bookmarked.
type entrySource uint8

const (
	sourceCatalog entrySource = iota
	sourceBookmark
)

// startEntry is one startpage row. target/kind/note/source come from the two
// sources; child/structural/bookmarked are set during assembly and describe how
// the row is displayed, not where it came from.
type startEntry struct {
	target string
	kind   entryKind
	note   string
	source entrySource

	child      bool // drawn under its host's parent row behind a connector; renders its token only
	lastChild  bool // final child of its group; draws "└" instead of "├"
	structural bool // a parent copy of a target listed elsewhere; not counted, hidden while filtering
	bookmarked bool // the target is in the bookmarks file, whatever section rendered this row
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

// validateBookmarkRecordTarget verifies that target survives the bookmark
// file's comment and single-field grammar without changing identity.
func validateBookmarkRecordTarget(target string) error {
	parsed, err := parseBookmarkTarget(strings.TrimSpace(stripComment(target)))
	if err != nil {
		return err
	}
	if parsed != target {
		return fmt.Errorf("target %q does not round-trip through the bookmarks file", target)
	}
	return nil
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
		return startEntry{}, fmt.Errorf("unknown kind %q (want community, service, person or group)", fields[0])
	}
	target := fields[1]
	if err := validateTarget(target); err != nil {
		return startEntry{}, err
	}
	return startEntry{target: target, kind: kind, note: fields[2], source: sourceCatalog}, nil
}

// validateTarget screens a target from a config file. finger.ParseTarget rejects
// C0/DEL and Cf/Zl/Zp via hasControl, but not invalid UTF-8 or UTF-8-encoded C1
// controls. A target is displayed in the list and breadcrumb, so all are refused
// here. Rejecting matches the treatment targets already get: bodies are visualized
// because they are content, a target is refused because it is something we send.
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

// loadBookmarks reads and parses the user's file. On first use it initializes a
// missing file with the author bookmark; every existing file, including an empty
// one, is authoritative. Read and initialization failures become problems the
// startpage surfaces. The resolved path is returned so every message can name
// the file actually in use.
func loadBookmarks() (bookmarkFile, string) {
	path, err := bookmarksPathFn()
	if err != nil {
		return bookmarkFile{problems: []parseProblem{{reason: "cannot locate a config directory: " + err.Error()}}}, ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			data = appendBookmarkLine(nil, aboutFingerAuthor)
			return initializeBookmarkData(path, data), path
		}
		return bookmarkFile{problems: []parseProblem{{reason: "cannot read: " + err.Error()}}}, path
	}
	return parseBookmarks(data), path
}

// initializeBookmarkData publishes a fully staged first file without replacing
// a concurrent winner. A winner is authoritative and must be read from disk.
func initializeBookmarkData(path string, data []byte) bookmarkFile {
	if err := createBookmarkData(path, data); err != nil {
		if !os.IsExist(err) {
			return bookmarkFile{problems: []parseProblem{{reason: "cannot create: " + err.Error()}}}
		}
		data, err = os.ReadFile(path)
		if err != nil {
			return bookmarkFile{problems: []parseProblem{{reason: "cannot read: " + err.Error()}}}
		}
	}
	return parseBookmarks(data)
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
// directory 0700 if needed.
func saveBookmarkData(path string, data []byte) error {
	writePath, err := bookmarkWritePath(path)
	if err != nil {
		return err
	}
	tmpName, err := stageBookmarkData(writePath, data)
	if err != nil {
		return err
	}
	defer os.Remove(tmpName) //nolint:errcheck // best-effort cleanup if rename succeeded
	return os.Rename(tmpName, writePath)
}

// createBookmarkData atomically publishes a staged file only when path is still
// absent. Hard-link creation is atomic and refuses to replace an existing path.
func createBookmarkData(path string, data []byte) error {
	tmpName, err := stageBookmarkData(path, data)
	if err != nil {
		return err
	}
	defer os.Remove(tmpName) //nolint:errcheck // best-effort cleanup of the staging name
	return os.Link(tmpName, path)
}

// stageBookmarkData writes and closes a mode-0600 temporary file in path's
// directory so either publication operation stays on the same filesystem.
func stageBookmarkData(path string, data []byte) (string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".bookmarks-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	staged := false
	defer func() {
		if !staged {
			os.Remove(tmpName) //nolint:errcheck // best-effort cleanup after staging failure
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close() //nolint:errcheck
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return "", err
	}
	staged = true
	return tmpName, nil
}

// bookmarkWritePath follows a final symlink so the atomic rename replaces its
// target rather than the user-managed link itself. Parent-directory symlinks do
// not need special handling: normal path traversal already follows them.
func bookmarkWritePath(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, nil
		}
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve bookmarks symlink: %w", err)
	}
	return resolved, nil
}

// shortenHome renders a path with ~ for display without making it wrong.
func shortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return shortenHomePath(path, home)
}

func shortenHomePath(path, home string) string {
	if home == "" {
		return path
	}
	rel, err := filepath.Rel(filepath.Clean(home), filepath.Clean(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return path
	}
	if rel == "." {
		return "~"
	}
	return filepath.Join("~", rel)
}

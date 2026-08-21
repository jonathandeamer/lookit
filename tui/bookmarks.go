package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

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

	visited time.Time // last-visited instant from the bookmarks file; zero = unknown

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
	visited       map[string]time.Time // last-visited instant per target; absent = unknown
	catalogHidden bool
	sortManual    bool // "sort manual": render the shelf in file order, not oldest-visit-first
	problems      []parseProblem
}

func parseBookmarks(data []byte) bookmarkFile {
	var out bookmarkFile
	seen := make(map[string]bool)
	for i, raw := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if fields[0] == "sort" {
			switch {
			case len(fields) == 2 && fields[1] == "manual":
				out.sortManual = true
			case len(fields) == 2 && fields[1] == "visited":
				out.sortManual = false
			default:
				out.problems = append(out.problems, parseProblem{
					line:   lineNo,
					reason: `expected "sort visited" or "sort manual"`,
				})
			}
			continue
		}
		if fields[0] == "catalog" {
			switch {
			case len(fields) == 2 && fields[1] == "off":
				out.catalogHidden = true
			case len(fields) == 2 && fields[1] == "on":
				out.catalogHidden = false
			default:
				out.problems = append(out.problems, parseProblem{
					line:   lineNo,
					reason: `expected "catalog on" or "catalog off"`,
				})
			}
			continue
		}
		target, visited, err := parseBookmarkTarget(line)
		if err != nil {
			out.problems = append(out.problems, parseProblem{line: lineNo, reason: err.Error()})
			continue
		}
		if !seen[target] {
			seen[target] = true
			out.targets = append(out.targets, target)
		}

		if visited.IsZero() {
			delete(out.visited, target) // last-line wins, including a trailing dateless duplicate
			continue
		}
		if out.visited == nil {
			out.visited = make(map[string]time.Time)
		}
		out.visited[target] = visited
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

// bookmarkFileHeader is written above the starter bookmarks when lookit creates
// the file, and never again: it is a greeting, not managed content, so a user
// who deletes or rewrites it keeps their edit forever.
//
// The file is the whole of lookit's configuration UI, and a lone target line
// documents none of it — that the second field is a date lookit maintains, that
// "#" starts a comment, that either directive exists. Comments are already
// skipped by the parser and preserved byte-for-byte by all three write paths,
// so this costs the grammar nothing. TestBookmarkFileHeaderIsAllComments keeps
// it parseable.
const bookmarkFileHeader = `# lookit bookmarks — one target per line: @tilde.team, you@example.org
#
# lookit adds the date it last saw you visit a target (2026-08-14). The
# startpage lists them longest ago first, so what you have been neglecting is
# at the top. Anything after a "#" is a comment: lookit keeps it, never shows it.
#
# Edit this file while lookit is running: it re-reads it the next time you
# return to the startpage ("h" from a page you have open).
#
#   catalog off    hide the built-in catalog
#   sort manual    keep this file's order instead

`

// starterBookmarks are the records written under bookmarkFileHeader when
// lookit creates the file, in this order. The default display sorts these
// initially undated pins by host and query; "sort manual" reveals the order
// below. They are a first run's somewhere-to-go, not managed content — each is
// as deletable as the last, and an existing file is authoritative, so none of
// them comes back.
//
// aboutFingerAuthor is reused rather than repeated; the other two are the
// fingerverse's own noticeboard and one person keeping a dated .plan, so a new
// file opens on what the small internet is doing now, not only on who made the
// client.
var starterBookmarks = []string{
	aboutFingerAuthor,
	"fingerverse@happynetbox.com",
	"me@andros.dev",
}

// bookmarkDateLayout is the file's last-visited date: a plain calendar day.
//
// The display buckets by local calendar day (relativeDay), so a day is
// everything it consumes — an instant would record a precision nothing reads
// back, on every line of a file meant to be read by a person. The cost is
// real but small: a visit recorded in one timezone and read in another can
// land one bucket out, which day-fuzzy prose absorbs.
const bookmarkDateLayout = "2006-01-02"

// beta1BookmarkDateLayout is the exact UTC-at-seconds format v0.2.0-beta.1
// wrote. It remains readable so upgrading does not hide an existing bookmark;
// the next successful visit rewrites the record with bookmarkDateLayout.
const beta1BookmarkDateLayout = "2006-01-02T15:04:05Z"

// parseBookmarkTarget accepts a target with an optional last-visited date:
// "<target>" or "<target> <YYYY-MM-DD>". The exact timestamp spelling emitted
// by v0.2.0-beta.1 is accepted as legacy input. Anything else is refused. A bad
// date refuses the whole record — a line lookit cannot understand is surfaced
// as a problem, never guessed at (the file's existing contract), and only a
// validated time.Time ever reaches the display, so the file still needs no
// sanitize call.
func parseBookmarkTarget(line string) (string, time.Time, error) {
	// The reasons below quote nothing from the file. A notice leads with the
	// path and the line number, which already point at the offending text, and
	// every cell an echo takes is one the reason cannot have: the startpage
	// prints a notice as a single unwrapped row, so a reason that outgrows the
	// terminal costs the page a line it has not budgeted.
	fields := strings.Fields(line)
	if len(fields) == 0 || len(fields) > 2 {
		return "", time.Time{}, fmt.Errorf("expected a target and an optional date")
	}
	if err := validateTarget(fields[0]); err != nil {
		return "", time.Time{}, err
	}
	if len(fields) == 1 {
		return fields[0], time.Time{}, nil
	}
	// Local, not UTC: the date names the day the user was there, and that is
	// the day relativeDay compares against. The round-trip check pins one
	// spelling — "2026-8-4" parses but is refused, so what the file holds is
	// always what the write path would emit.
	visited, err := time.ParseInLocation(bookmarkDateLayout, fields[1], time.Local)
	if err == nil && fields[1] == visited.Format(bookmarkDateLayout) {
		return fields[0], visited, nil
	}
	visited, err = time.Parse(beta1BookmarkDateLayout, fields[1])
	if err == nil && fields[1] == visited.Format(beta1BookmarkDateLayout) {
		return fields[0], visited, nil
	}
	return "", time.Time{}, fmt.Errorf("expected a date like 2026-08-14")
}

// validateBookmarkRecordTarget verifies that target survives the bookmark
// file's comment-and-two-field grammar without changing identity. The add path
// still passes a bare target, which parses as a one-field record.
func validateBookmarkRecordTarget(target string) error {
	parsed, _, err := parseBookmarkTarget(strings.TrimSpace(stripComment(target)))
	if err != nil {
		return err
	}
	if parsed != target {
		return fmt.Errorf("target %q cannot be saved unchanged", target)
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
		// "Invisible character", not the Unicode category: the user is looking
		// at a target that appears ordinary, and naming Cf/Zl/Zp explains
		// nothing about what they can see.
		return fmt.Errorf("target has an invisible character")
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
// missing file with starterBookmarks; every existing file, including an empty
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
			data = []byte(bookmarkFileHeader)
			for _, target := range starterBookmarks {
				data = appendBookmarkLine(data, target, time.Time{})
			}
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
// A non-zero visited writes the date with the record, so bookmarking the page
// you are reading does not produce a row that claims never to have been
// visited. A zero visited writes the target alone: the caller could not
// establish a visit, and a blank note is the honest reading of that.
func appendBookmarkLine(data []byte, target string, visited time.Time) []byte {
	out := string(data)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	record := target
	if !visited.IsZero() {
		record += " " + visited.Format(bookmarkDateLayout)
	}
	return []byte(out + record + "\n")
}

// deleteBookmarkLine drops every valid bookmark record for target, leaving
// comments, malformed records, blank lines, directives and ordering untouched.
func deleteBookmarkLine(data []byte, target string) []byte {
	lines := strings.Split(string(data), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		parsed, _, err := parseBookmarkTarget(strings.TrimSpace(stripComment(line)))
		if err == nil && parsed == target {
			continue
		}
		kept = append(kept, line)
	}
	return []byte(strings.Join(kept, "\n"))
}

// updateBookmarkLine rewrites the last-visited date on every valid record for
// target — the write path's first in-place rewrite, so it is careful about
// what it touches: only valid records whose target matches exactly are
// rewritten (all duplicates, consistent with deleteBookmarkLine), each keeps
// its leading whitespace and its trailing comment byte-identical, and
// everything else — comments, malformed lines, blanks, directives, ordering —
// is untouched. changed is false when no record matched *or when every match
// already reads the way it would be rewritten*, so a second visit on the
// same day writes nothing. The caller treats that as "nothing to save",
// which is right for both cases.
func updateBookmarkLine(data []byte, target string, ts time.Time) ([]byte, bool) {
	stamp := ts.Format(bookmarkDateLayout)
	lines := strings.Split(string(data), "\n")
	changed := false
	for i, line := range lines {
		parsed, _, err := parseBookmarkTarget(strings.TrimSpace(stripComment(line)))
		if err != nil || parsed != target {
			continue
		}
		rewritten := rewriteBookmarkRecord(line, target, stamp)
		if rewritten == line {
			continue
		}
		// Round-trip guard: the emitted record must parse back to the same
		// target and day, or the line is left untouched rather than writing a
		// record the parser would later refuse. The day is compared as the
		// text the file holds, not as an instant: the parser reads a date as
		// local midnight, while ts carries whatever zone the caller had.
		check, checkTS, err := parseBookmarkTarget(strings.TrimSpace(stripComment(rewritten)))
		if err != nil || check != target || checkTS.Format(bookmarkDateLayout) != stamp {
			continue
		}
		lines[i] = rewritten
		changed = true
	}
	if !changed {
		return data, false
	}
	return []byte(strings.Join(lines, "\n")), true
}

// nowFn is the clock for visit stamps and relative dates, a package var so
// tests control time — the same pattern bookmarksPathFn uses for the path.
var nowFn = time.Now

// stampVisitCmd runs stampVisit off the update loop. The write is a read plus
// an atomic replace, and a config dir on a network filesystem — routine on the
// tilde and pubnix boxes this client is pointed at — would otherwise stall
// every landing for the duration of that round trip. Nothing observes the
// result: the command reports no message, matching the silent degradation
// stampVisit already documents.
func stampVisitCmd(raw string) tea.Cmd {
	return func() tea.Msg {
		stampVisit(raw)
		return nil
	}
}

// stampVisit records a successful landing in the bookmarks file. Every
// failure — no config dir, unreadable file, unwritable filesystem — degrades
// silently: the date simply does not advance, and navigation is never blocked
// by bookkeeping. A target with no record matches nothing, so unpinned
// targets never write the file and a visit never adds a record.
func stampVisit(raw string) {
	path, err := bookmarksPathFn()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	updated, changed := updateBookmarkLine(data, raw, nowFn())
	if !changed {
		return
	}
	_ = saveBookmarkData(path, updated) //nolint:errcheck // a stale date is the designed degradation
}

// rewriteBookmarkRecord replaces the record on line with target and stamp,
// splicing on raw offsets so the user's own spacing survives: the leading
// whitespace is kept, and everything from the first "#" onward (the gap before
// it included) is copied byte-identical.
func rewriteBookmarkRecord(line, target, stamp string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	rest := line[len(indent):]
	gap, comment := "", ""
	if i := strings.Index(rest, "#"); i >= 0 {
		record := strings.TrimRight(rest[:i], " \t")
		gap = rest[len(record):i]
		comment = rest[i:]
	}
	return indent + target + " " + stamp + gap + comment
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

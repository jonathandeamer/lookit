package tui

import (
	"regexp"
	"strings"
)

// User is one entry in a host's finger user list.
type User struct {
	Login  string
	Name   string // "" when unknown
	Target string // optional explicit target, e.g. "user@other.host"
}

// loginRe matches a plausible Unix login: leading alphanumeric/underscore,
// then login characters, max 32 runes. Rejects FQDNs, punctuated words, and
// over-long tokens.
var loginRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,31}$`)

// dateNoiseRe matches login-time / idle column tokens (weekday/month
// abbreviations and durations like "207d") that can land in fields[1] when a
// columnar row has no real name.
var dateNoiseRe = regexp.MustCompile(`^(?:\d+[dhms]?|Mon|Tue|Wed|Thu|Fri|Sat|Sun|Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)$`)

// gridCueRe gates Format 1 (token grid). It is tried after the columnar
// matcher because a "Login" header is a stronger signal. Only a recognized
// "who is here" cue turns the following lines into a login block.
var gridCueRe = regexp.MustCompile(`(?i)logged[\s-]?in|online`)

// markerRe matches Format 3 marker rows: "> login" with a single login token.
var markerRe = regexp.MustCompile(`^\s*>\s+(\S+)\s*$`)
var fingerURLRe = regexp.MustCompile(`finger://([^/\s]+)/([A-Za-z0-9_][A-Za-z0-9_.-]{0,31})`)
var fingerCommandRe = regexp.MustCompile(`\bfinger\s+([A-Za-z0-9_][A-Za-z0-9_.-]{0,31}@[A-Za-z0-9_.:-]+)\b`)

type parsedUserList struct {
	users    []User
	preamble string
}

// ParseUsers extracts a host's logged-in / listed users from a finger response
// body. It returns (users, true) only when a format is confidently recognized;
// otherwise (nil, false). The caller guarantees this is a host query.
//
// Three gated matchers are tried in order: columnar (Login header), grid
// (cue line), marker ("> login"). Results are deduplicated, order preserved.
func ParseUsers(body []byte) ([]User, bool) {
	parsed, ok := parseUserList(body)
	return parsed.users, ok
}

func parseUserList(body []byte) (parsedUserList, bool) {
	lines := strings.Split(string(body), "\n")

	if users, ok := parseColumnar(lines); ok {
		return parsedUserList{users: users, preamble: preambleBeforeColumnar(lines)}, true
	}
	if users, ok := parseGrid(lines); ok {
		return parsedUserList{users: users, preamble: preambleBeforeGrid(lines)}, true
	}
	if users, ok := parseMarker(lines); ok {
		return parsedUserList{users: users, preamble: preambleBeforeMarker(lines)}, true
	}
	if users, preamble, ok := parseTypedHoleMenu(lines); ok {
		return parsedUserList{users: users, preamble: preamble}, true
	}
	if users, preamble, ok := parseSavaTable(lines); ok {
		return parsedUserList{users: users, preamble: preamble}, true
	}
	if users, preamble, ok := parseRedterminalMenu(lines); ok {
		return parsedUserList{users: users, preamble: preamble}, true
	}
	if users, preamble, ok := parseFingerRing(lines); ok {
		return parsedUserList{users: users, preamble: preamble}, true
	}
	if users, preamble, ok := parseTelehackStatus(lines); ok {
		return parsedUserList{users: users, preamble: preamble}, true
	}
	return parsedUserList{}, false
}

// parseColumnar handles classic fingerd output: a "Login ... Name ..." header
// followed by one row per session. Login is the first whitespace token; Name
// is best-effort (the second token when it looks like a name, else "").
func parseColumnar(lines []string) ([]User, bool) {
	header := -1
	hasName := false
	for i, ln := range lines {
		fields := strings.Fields(ln)
		if len(fields) > 0 && strings.EqualFold(fields[0], "Login") {
			header = i
			for _, f := range fields[1:] {
				if strings.EqualFold(f, "Name") {
					hasName = true
				}
			}
			break
		}
	}
	if header < 0 {
		return nil, false
	}

	var users []User
	seen := map[string]bool{}
	for _, ln := range lines[header+1:] {
		if strings.TrimSpace(ln) == "" {
			break
		}
		fields := strings.Fields(ln)
		if len(fields) == 0 || !loginRe.MatchString(fields[0]) {
			continue
		}
		login := fields[0]
		if seen[login] {
			continue
		}
		seen[login] = true
		name := ""
		// Best-effort name: take a single non-login-looking second token.
		if hasName && len(fields) >= 2 && !looksLikeColumnNoise(fields[1]) {
			name = fields[1]
		}
		users = append(users, User{Login: login, Name: name})
	}
	if len(users) == 0 {
		return nil, false
	}
	return users, true
}

// looksLikeColumnNoise reports whether a second-column token is clearly not a
// real name — a tty/idle column ("pts/15", "*p1") or a login-time/idle token
// ("Fri", "May", "207d") — so a nameless row does not get a junk name.
// Best-effort only.
func looksLikeColumnNoise(s string) bool {
	if s == "" {
		return true
	}
	if strings.ContainsAny(s, "/*:") {
		return true
	}
	return dateNoiseRe.MatchString(s)
}

// parseGrid handles a whitespace/tab grid of bare logins that appears after a
// recognized cue line. It collects only the contiguous block immediately
// following the cue (after up to one blank line); a line containing any
// non-login token ends the block. Cue-line tokens are never parsed.
func parseGrid(lines []string) ([]User, bool) {
	cue := -1
	for i, ln := range lines {
		if gridCueRe.MatchString(ln) {
			cue = i
			break
		}
	}
	if cue < 0 {
		return nil, false
	}

	i := cue + 1
	// Skip up to one blank line between the cue and the block.
	if i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}

	var users []User
	seen := map[string]bool{}
	for ; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			break
		}
		fields := strings.Fields(lines[i])
		allLogins := len(fields) > 0
		for _, f := range fields {
			if !loginRe.MatchString(f) {
				allLogins = false
				break
			}
		}
		if !allLogins {
			break
		}
		for _, f := range fields {
			if seen[f] {
				continue
			}
			seen[f] = true
			users = append(users, User{Login: f})
		}
	}
	if len(users) == 0 {
		return nil, false
	}
	return users, true
}

// parseMarker handles "> login" lists (e.g. happynetbox). Each matching line
// must have exactly one login token after the marker.
func parseMarker(lines []string) ([]User, bool) {
	var users []User
	seen := map[string]bool{}
	for _, ln := range lines {
		m := markerRe.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		login := m[1]
		if !loginRe.MatchString(login) || seen[login] {
			continue
		}
		seen[login] = true
		users = append(users, User{Login: login})
	}
	if len(users) == 0 {
		return nil, false
	}
	return users, true
}

func parseTypedHoleMenu(lines []string) ([]User, string, bool) {
	start := -1
	for i, ln := range lines {
		if strings.EqualFold(strings.TrimSpace(ln), "Available fingers:") {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, "", false
	}

	var users []User
	seen := map[string]bool{}
	for _, ln := range lines[start+1:] {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		login, desc, ok := splitColonEntry(ln)
		if !ok {
			if len(users) > 0 {
				break
			}
			continue
		}
		if seen[login] {
			continue
		}
		seen[login] = true
		users = append(users, User{Login: login, Name: desc})
	}
	if len(users) == 0 {
		return nil, "", false
	}
	return users, trimPreamble(lines[:start+1]), true
}

func splitColonEntry(ln string) (string, string, bool) {
	before, after, ok := strings.Cut(ln, ":")
	if !ok {
		return "", "", false
	}
	login := strings.TrimSpace(before)
	if !loginRe.MatchString(login) {
		return "", "", false
	}
	desc := strings.TrimSpace(after)
	return login, desc, true
}

func parseSavaTable(lines []string) ([]User, string, bool) {
	title := -1
	for i, ln := range lines {
		if strings.Contains(ln, "Users on this finger server") {
			title = i
			break
		}
	}
	if title < 0 {
		return nil, "", false
	}

	var users []User
	seen := map[string]bool{}
	for _, ln := range lines[title+1:] {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "+") {
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			if len(users) > 0 {
				break
			}
			continue
		}
		cells := tableCells(ln)
		if len(cells) < 3 {
			continue
		}
		m := fingerCommandRe.FindStringSubmatch(cells[2])
		if m == nil {
			continue
		}
		login := cells[0]
		if !loginRe.MatchString(login) || seen[login] {
			continue
		}
		seen[login] = true
		users = append(users, User{Login: login, Name: cells[1], Target: m[1]})
	}
	if len(users) == 0 {
		return nil, "", false
	}
	return users, trimPreamble(lines[:title+1]), true
}

func tableCells(ln string) []string {
	parts := strings.Split(strings.Trim(ln, "|"), "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

func parseRedterminalMenu(lines []string) ([]User, string, bool) {
	start := -1
	for i, ln := range lines {
		if strings.Contains(ln, "Available Fingers") {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, "", false
	}

	var users []User
	seen := map[string]bool{}
	for _, ln := range lines[start+1:] {
		fields := strings.Fields(ln)
		if len(fields) == 0 {
			continue
		}
		login := fields[0]
		if !loginRe.MatchString(login) {
			if len(users) > 0 {
				break
			}
			continue
		}
		if seen[login] {
			continue
		}
		seen[login] = true
		desc := strings.TrimSpace(strings.TrimPrefix(ln, login))
		users = append(users, User{Login: login, Name: desc})
	}
	if len(users) == 0 {
		return nil, "", false
	}
	return users, trimPreamble(lines[:start+1]), true
}

func parseFingerRing(lines []string) ([]User, string, bool) {
	start := -1
	for i, ln := range lines {
		if strings.Contains(strings.ToLower(ln), "and now for the list") {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, "", false
	}

	var users []User
	seen := map[string]bool{}
	for _, ln := range lines[start+1:] {
		m := fingerURLRe.FindStringSubmatch(ln)
		if m == nil {
			if len(users) > 0 {
				break
			}
			continue
		}
		host, login := m[1], m[2]
		target := login + "@" + host
		if seen[target] {
			continue
		}
		seen[target] = true
		users = append(users, User{Login: login, Name: host, Target: target})
	}
	if len(users) == 0 {
		return nil, "", false
	}
	return users, trimPreamble(lines[:start+1]), true
}

func parseTelehackStatus(lines []string) ([]User, string, bool) {
	header := -1
	for i, ln := range lines {
		fields := strings.Fields(ln)
		if len(fields) >= 2 && fields[0] == "port" && fields[1] == "username" {
			header = i
			break
		}
	}
	if header < 0 || header+1 >= len(lines) {
		return nil, "", false
	}

	var users []User
	seen := map[string]bool{}
	for _, ln := range lines[header+2:] {
		fields := strings.Fields(ln)
		if len(fields) < 2 {
			if len(users) > 0 {
				break
			}
			continue
		}
		login := fields[1]
		if login == "-" || !loginRe.MatchString(login) || seen[login] {
			continue
		}
		seen[login] = true
		desc := ""
		if len(fields) >= 3 {
			desc = fields[2]
		}
		users = append(users, User{Login: login, Name: desc})
	}
	if len(users) == 0 {
		return nil, "", false
	}
	return users, trimPreamble(lines[:header+2]), true
}

func preambleBeforeColumnar(lines []string) string {
	preamble, _ := columnarPreamble(lines)
	return preamble
}

func preambleBeforeGrid(lines []string) string {
	preamble, _ := gridPreamble(lines)
	return preamble
}

func preambleBeforeMarker(lines []string) string {
	preamble, _ := markerPreamble(lines)
	return preamble
}

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
	generic  bool
}

// ParseUsers extracts a host's listed users/entries from a finger response
// body. It returns (users, true) only when a format is confidently recognized;
// otherwise (nil, false). The caller guarantees this is a host query.
//
// Several gated matchers are tried in order: the generic columnar (Login
// header), grid (cue line), and marker ("> login") formats, then service-
// specific menu/table formats (typed-hole, sava, redterminal, the Finger Ring,
// telehack). Results are deduplicated, order preserved.
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
	if users, preamble, ok := parseGenericList(lines); ok {
		return parsedUserList{users: users, preamble: preamble, generic: true}, true
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

// structuredLogin reports whether a single line is a generic structured login
// entry, returning the login and a best-effort name. It accepts only two
// shapes: a bare login (the whole trimmed line is one loginRe token), or a
// columnar login (first token is a loginRe token followed by a tab or 2+
// spaces, then the trimmed remainder is taken as a best-effort name). A login
// followed by a single space, and any "login : value" colon form, are treated
// as prose and rejected — those shapes appear constantly in help text, legends,
// and glossaries (e.g. db.debian.org's "cn : First name"), where they are not
// user lists.
func structuredLogin(line string) (login, name string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", "", false
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 1 {
		if loginRe.MatchString(fields[0]) {
			return fields[0], "", true
		}
		return "", "", false
	}
	first := fields[0]
	if !loginRe.MatchString(first) {
		return "", "", false
	}
	// trimmed begins with first (leading space already removed); the gap that
	// follows must be a tab or 2+ spaces to count as a deliberate column layout.
	rest := trimmed[len(first):]
	if strings.HasPrefix(rest, "\t") || strings.HasPrefix(rest, "  ") {
		return first, strings.TrimSpace(rest), true
	}
	return "", "", false
}

// appendHarvestedTargets adds cross-host drill targets found anywhere in the
// body via the existing strong-signal regexes (finger:// URLs and
// "finger user@host" commands) — the same contexts parseFingerRing and
// parseSavaTable already trust. Targets are additive: this is called only after
// the structured-login gate has already opened the list, so a stray mention
// can never open a list on its own. Bare emails and @handles are not harvested.
// Server-supplied targets are pinned to port 79 later, at drill time, by the
// existing pinFingerPort path in app.go.
func appendHarvestedTargets(users []User, lines []string) []User {
	// Key on Target so a structured login (Target=="") never blocks a harvested cross-host entry.
	seen := map[string]bool{}
	for _, u := range users {
		if u.Target != "" {
			seen[u.Target] = true
		}
	}
	body := strings.Join(lines, "\n")
	for _, m := range fingerURLRe.FindAllStringSubmatch(body, -1) {
		host, login := m[1], m[2]
		target := login + "@" + host
		if seen[target] {
			continue
		}
		seen[target] = true
		users = append(users, User{Login: login, Name: host, Target: target})
	}
	for _, m := range fingerCommandRe.FindAllStringSubmatch(body, -1) {
		target := m[1] // already in login@host form
		if seen[target] {
			continue
		}
		seen[target] = true
		login := target
		// fingerCommandRe guarantees an '@' in the capture; the guard is defensive.
		if at := strings.IndexByte(target, '@'); at >= 0 {
			login = target[:at]
		}
		users = append(users, User{Login: login, Target: target})
	}
	return users
}

// parseGenericList is the last-resort matcher, tried only after every named
// parser declines. It finds the longest contiguous run of structuredLogin
// lines and opens a list when that run holds >= 2 distinct logins; otherwise it
// declines. A blank or non-entry line ends a run.
func parseGenericList(lines []string) ([]User, string, bool) {
	bestStart, bestCount := -1, 0
	var bestUsers []User

	for i := 0; i < len(lines); {
		if _, _, ok := structuredLogin(lines[i]); !ok {
			i++
			continue
		}
		start := i
		seen := map[string]bool{}
		var runUsers []User
		for i < len(lines) {
			login, name, ok := structuredLogin(lines[i])
			if !ok {
				break
			}
			if !seen[login] {
				seen[login] = true
				runUsers = append(runUsers, User{Login: login, Name: name})
			}
			i++
		}
		if len(seen) > bestCount {
			bestCount, bestStart, bestUsers = len(seen), start, runUsers
		}
	}
	if bestCount < 2 {
		return nil, "", false
	}
	bestUsers = appendHarvestedTargets(bestUsers, lines)
	return bestUsers, trimPreamble(lines[:bestStart]), true
}

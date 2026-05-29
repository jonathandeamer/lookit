package tui

import (
	"regexp"
	"strings"
)

// User is one entry in a host's finger user list.
type User struct {
	Login string
	Name  string // "" when unknown
}

// loginRe matches a plausible Unix login: leading alphanumeric/underscore,
// then login characters, max 32 runes. Rejects FQDNs, punctuated words, and
// over-long tokens.
var loginRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,31}$`)

// gridCueRe gates Format 1 (token grid). Only a recognized "who is here" cue
// turns following lines into a login block.
var gridCueRe = regexp.MustCompile(`(?i)logged[\s-]?in|online`)

// markerRe matches Format 3 marker rows: "> login" with a single login token.
var markerRe = regexp.MustCompile(`^\s*>\s+(\S+)\s*$`)

// ParseUsers extracts a host's logged-in / listed users from a finger response
// body. It returns (users, true) only when a format is confidently recognized;
// otherwise (nil, false). The caller guarantees this is a host query.
//
// Three gated matchers are tried in order: columnar (Login header), grid
// (cue line), marker ("> login"). Results are deduplicated, order preserved.
func ParseUsers(body []byte) ([]User, bool) {
	lines := strings.Split(string(body), "\n")

	if users, ok := parseColumnar(lines); ok {
		return users, true
	}
	if users, ok := parseGrid(lines); ok {
		return users, true
	}
	if users, ok := parseMarker(lines); ok {
		return users, true
	}
	return nil, false
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

// looksLikeColumnNoise rejects obvious non-name second tokens (tty/idle/date
// fragments) so a bare login row does not get a junk name. Best-effort only.
func looksLikeColumnNoise(s string) bool {
	if s == "" {
		return true
	}
	// Tty/idle columns like "pts/15", "*p1", "t6", or dates "Fri"/"May".
	if strings.ContainsAny(s, "/*:") {
		return true
	}
	return false
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

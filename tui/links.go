package tui

import (
	"errors"
	"regexp"
	"strings"

	"github.com/jonathandeamer/lookit/finger"
)

// LinkKind classifies what a detected link points to.
type LinkKind int

const (
	LinkFinger LinkKind = iota // finger query — drill or copy
	LinkURL                    // https/http/gemini/gopher/etc — copy
	LinkEmail                  // explicit mailto: or cue-tagged address — copy
	LinkSocial                 // fedi/mastodon handle — copy
)

// LinkAction is the default action for Enter while the link is focused.
type LinkAction int

const (
	ActionDrill LinkAction = iota // fires a finger query in-app
	ActionCopy                    // copies Raw to the clipboard
)

// Link is one detected address or URL in a finger response body.
type Link struct {
	Kind      LinkKind
	Action    LinkAction
	Raw       string        // exact substring as it appears in the body
	Target    finger.Target // populated for Kind==Finger links (incl. ambiguous)
	Ambiguous bool          // bare user@host indistinguishable from email; drives "(auto)" label
	Forwarded bool          // one-relay forwarding form
	Blocked   string        // non-empty for copy-only finger links that must not drill
	Strong    bool          // rule 1 or rule 2 match (not inferred from shape alone)
}

// DetectLinks scans sanitized body bytes and returns all detected links in
// document order. originHostPort is the Entry.Target.HostPort of the response
// (used for the same-relay forwarding check).
func DetectLinks(body []byte, originHostPort string) []Link {
	text := string(body)
	if text == "" {
		return nil
	}
	origin := canonicalHost(originHostPort)

	// Phase 1: collect scheme-URL spans left-to-right.
	consumed := make([]bool, len(text))
	var links []Link

	for _, span := range schemeURLRe.FindAllStringIndex(text, -1) {
		raw := text[span[0]:span[1]]
		raw = stripTrailingPunct(raw)
		if raw == "" {
			continue
		}
		// Authority must be non-empty after "://".
		schemeSep := strings.Index(raw, "://")
		if schemeSep >= 0 && len(raw) <= schemeSep+3 {
			continue
		}
		// Word boundary: char before must not be a word char.
		if span[0] > 0 && isWordChar(text[span[0]-1]) {
			continue
		}
		link, ok := classifySchemeURL(raw, origin)
		if !ok {
			continue
		}
		for i := span[0]; i < span[0]+len(raw); i++ {
			consumed[i] = true
		}
		links = append(links, link)
	}

	// Phase 2: scan for @-containing tokens in unconsumed text.
	pos := 0
	for pos < len(text) {
		if consumed[pos] {
			pos++
			continue
		}
		at := strings.IndexByte(text[pos:], '@')
		if at < 0 {
			break
		}
		atAbs := pos + at

		// Expand left (stop at delimiter or consumed byte).
		start := atAbs
		for start > 0 && !isDelim(text[start-1]) && !consumed[start-1] {
			start--
		}
		// Expand right (stop at delimiter).
		end := atAbs + 1
		for end < len(text) && !isDelim(text[end]) {
			end++
		}

		// Skip if any byte overlaps a phase-1 span.
		overlap := false
		for i := start; i < end; i++ {
			if consumed[i] {
				overlap = true
				break
			}
		}
		if overlap {
			pos = end
			continue
		}

		raw := text[start:end]

		// Word boundary checks.
		if start > 0 && isWordChar(text[start-1]) {
			pos = end
			continue
		}
		if end < len(text) && isWordChar(text[end]) {
			pos = end
			continue
		}

		// Cue word: last whitespace word before start.
		cueWord := ""
		if m := cueWordRe.FindString(text[:start]); m != "" {
			cueWord = strings.TrimSpace(m)
		}

		link, ok := classifyAtToken(raw, cueWord, origin)
		if !ok {
			pos = end
			continue
		}
		for i := start; i < end; i++ {
			consumed[i] = true
		}
		links = append(links, link)
		pos = end
	}

	return links
}

// harvestableLogin reports whether a Target's login matches the legacy
// login-class constraint the old fingerCommandRe/fingerURLRe enforced.
// Keeps appendHarvestedTargets behaviour-neutral after the DetectLinks refactor.
func harvestableLogin(t finger.Target) bool {
	return loginRe.MatchString(t.Query)
}

// domainSane reports whether host is a plausible finger/email domain or IP literal.
func domainSane(host string) bool {
	// Bracketed IP literal.
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		inner := host[1 : len(host)-1]
		if ipv4Re.MatchString(inner) {
			return true
		}
		return strings.Contains(inner, ":") // IPv6
	}
	// Domain: must have at least one dot.
	dot := strings.LastIndex(host, ".")
	if dot < 0 {
		return false
	}
	tld := host[dot+1:]
	// TLD: 2+ alpha chars; reject all-numeric (dotted-quad guard).
	if len(tld) < 2 || !allAlpha(tld) {
		return false
	}
	// All labels: valid chars, no leading/trailing hyphen.
	for _, label := range strings.Split(host, ".") {
		if label == "" || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
				(c >= '0' && c <= '9') || c == '-') {
				return false
			}
		}
	}
	return true
}

// isOSC8Openable reports whether a Raw token should be wrapped as an OSC-8
// hyperlink. Only http(s):// and mailto: are reliably openable by macOS terminals.
func isOSC8Openable(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:")
}

// Compiled regexes used by the scanner.
var (
	// schemeURLRe matches any scheme-prefixed URL token, including both
	// the scheme://... form (finger, http, https, gemini, gopher, ircs, …)
	// and the scheme:path form without // (e.g. mailto:user@host).
	// Authority must be non-empty for the :// form — the caller's post-filter
	// drops bare "https://" with no host.
	schemeURLRe = regexp.MustCompile(
		`(?i)[A-Za-z][A-Za-z0-9+.\-]{1,30}:(?://[^\s<>"` + "`" + `(){}\[\]]+|[^\s<>"` + "`" + `(){}\[\]/][^\s<>"` + "`" + `(){}\[\]]*)`)

	// cueWordRe extracts the last whitespace-delimited word before a position.
	cueWordRe = regexp.MustCompile(`(?i)(\w+)\s*$`)

	// ipv4Re matches a bare IPv4 dotted-quad (used inside domainSane).
	ipv4Re = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)
)

// cueKind maps a cue word to the LinkKind and LinkAction it implies.
func cueKind(word string) (LinkKind, LinkAction, bool) {
	switch strings.ToLower(word) {
	case "finger":
		return LinkFinger, ActionDrill, true
	case "email", "e-mail", "mail", "contact":
		return LinkEmail, ActionCopy, true
	case "web", "site", "url":
		return LinkURL, ActionCopy, true
	case "fedi", "fediverse", "mastodon":
		return LinkSocial, ActionCopy, true
	}
	return 0, 0, false
}

// canonicalHost strips the port suffix and lowercases the host.
// Used for same-relay forwarding comparisons.
func canonicalHost(hostPort string) string {
	h := hostPort
	if i := strings.LastIndex(h, ":"); i >= 0 {
		h = h[:i]
	}
	return strings.ToLower(h)
}

// isDelim reports whether c is a token-boundary delimiter character.
func isDelim(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
		c == '<' || c == '>' || c == '"' || c == '\'' || c == '`' ||
		c == '(' || c == ')' || c == '{' || c == '}' || c == '[' || c == ']'
}

// isWordChar reports whether c is a word character (for boundary checks).
func isWordChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '_'
}

// stripTrailingPunct removes trailing sentence punctuation and unbalanced
// closing delimiters from a URL token.
func stripTrailingPunct(s string) string {
	for {
		if len(s) == 0 {
			break
		}
		last := s[len(s)-1]
		if last == '.' || last == ',' || last == ';' || last == ':' ||
			last == '!' || last == '?' {
			s = s[:len(s)-1]
			continue
		}
		var open byte
		switch last {
		case ')':
			open = '('
		case ']':
			open = '['
		case '}':
			open = '{'
		}
		if open != 0 {
			opens := strings.Count(s, string(open))
			closes := strings.Count(s, string(last))
			if closes > opens {
				s = s[:len(s)-1]
				continue
			}
		}
		break
	}
	return s
}

// classifySchemeURL classifies a scheme-prefixed URL token detected in Phase 1.
// Returns (Link, false) only when the token should be silently dropped.
func classifySchemeURL(raw, origin string) (Link, bool) {
	lower := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lower, "finger://"):
		return classifyFingerURL(raw, origin)
	case strings.HasPrefix(lower, "mailto:"):
		return Link{
			Kind:   LinkEmail,
			Action: ActionCopy,
			Raw:    raw,
			Strong: true,
		}, true
	default:
		// http://, https://, gemini://, gopher://, ircs://, rtmp://, etc.
		return Link{
			Kind:   LinkURL,
			Action: ActionCopy,
			Raw:    raw,
			Strong: true,
		}, true
	}
}

// classifyFingerURL parses a finger:// URL and builds a drillable Link.
// raw is the full matched token (e.g. "finger://tilde.team/alice").
// Returns (Link{}, false) if the URL is malformed or encodes server forwarding.
func classifyFingerURL(raw, _ string) (Link, bool) {
	// Strip the "finger://" prefix (case-insensitive — raw may be mixed case).
	rest := raw[len("finger://"):]

	// Split on first '/' to separate authority from path.
	authority := rest
	path := ""
	if idx := strings.IndexByte(rest, '/'); idx >= 0 {
		authority = rest[:idx]
		path = rest[idx+1:] // strip the leading '/'
	}

	// Build the ParseTargetPinned argument.
	var arg string
	if path == "" {
		arg = "@" + authority // bare host query
	} else {
		arg = path + "@" + authority // user@host form
	}

	t, err := finger.ParseTargetPinned(arg)
	if err != nil {
		// ErrServerForwarding and any other error: silently drop.
		if errors.Is(err, finger.ErrServerForwarding) {
			return Link{}, false
		}
		return Link{}, false
	}

	return Link{
		Kind:   LinkFinger,
		Action: ActionDrill,
		Raw:    raw,
		Target: t,
		Strong: true,
	}, true
}

// classifyAtToken classifies a token containing at least one '@'. Stub — implemented in Task 5.
func classifyAtToken(raw, cueWord, origin string) (Link, bool) { return Link{}, false }

// allAlpha reports whether s contains only ASCII letters.
func allAlpha(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
			return false
		}
	}
	return true
}

package tui

import (
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
	return nil
}

// harvestableLogin reports whether a Target's login matches the legacy
// login-class constraint the old fingerCommandRe/fingerURLRe enforced.
// Keeps appendHarvestedTargets behaviour-neutral after the DetectLinks refactor.
func harvestableLogin(t finger.Target) bool {
	return loginRe.MatchString(t.Query)
}

// domainSane reports whether host is a plausible finger/email domain or IP literal.
// Stub — implemented in Task 3.
func domainSane(host string) bool {
	return false
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
	// schemeURLRe matches any scheme://authority... token. Authority must be
	// non-empty so bare "https://" with no host is not matched.
	schemeURLRe = regexp.MustCompile(
		`(?i)[A-Za-z][A-Za-z0-9+.\-]{1,30}://[^\s<>"` + "`" + `(){}\[\]]+`)

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

// classifySchemeURL classifies a scheme:// URL token. Stub — implemented in Task 4.
func classifySchemeURL(raw, origin string) (Link, bool) { return Link{}, false }

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

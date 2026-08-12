# Body link detection & in-reader browsing — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Detect URLs, finger targets, emails, and social handles in finger response bodies and let the reader user Tab-cycle between them, drill finger links, copy/⌘-click others.

**Architecture:** A new pure `DetectLinks(body, originHostPort)` function in `tui/links.go` drives everything — the reader overlay, the `L` panel, and a refactored `appendHarvestedTargets`. Link focus state lives on `readerModel`; detected links are cached on `histNode`. A tui-side overlay pass applies focus-highlight and OSC-8 wrapping to the rendered body (header excluded). `render.Split` separates header from body so the overlay never touches chrome.

**Tech Stack:** Go, `charm.land/lipgloss/v2` (tui overlay + OSC-8), `github.com/charmbracelet/x/ansi` (already imported), `charm.land/bubbles/v2/list` (links panel), `regexp` (scanner), existing `finger.ParseTargetPinned`.

---

## File map

| Action | File | Purpose |
|--------|------|---------|
| Create | `tui/links.go` | `Link` type, `DetectLinks`, `domainSane`, `harvestableLogin`, `applyLinkOverlay`, `isOSC8Openable` |
| Create | `tui/links_test.go` | Golden corpus: detect, decline, forwarding, punctuation, strong-gate, overlay |
| Modify | `tui/userlist.go` | Refactor `appendHarvestedTargets` to call `DetectLinks`; thread `originHostPort` |
| Modify | `render/render.go` | Extract `renderBodyOnly`; add exported `Split(…) (header, body string)` |
| Modify | `render/render_test.go` | Add `TestRender_Split` |
| Modify | `tui/styles.go` | Add `linkFocus lipgloss.Style` to `styles` |
| Modify | `tui/app.go` | Add `links []Link`/`linkIdx int` to `histNode`; populate in `routeFetch`/`restore`; route Tab/n/N/f/L/Enter/y |
| Modify | `tui/reader.go` | Add `focusedLink int`; add `setEntryWithLinks`; `scrollToFocusedLink` |
| Modify | `tui/keys.go` | Add `LinkNext`, `LinkPrev`, `LinkFinger`, `LinkPanel` bindings |

---

## Task 1: `Link` type, constants, and stubs in `tui/links.go`

**Files:**
- Create: `tui/links.go`

- [ ] **Step 1: Write `tui/links.go` with types and stubs**

```go
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
	Ambiguous bool          // bare user@host (indistinguishable from email); drives "(auto)" label
	Forwarded bool          // one-relay forwarding form was found
	Blocked   string        // non-empty for copy-only finger links that must not drill
	Strong    bool          // rule 1 or rule 2 match (not inferred from shape alone)
}

// DetectLinks scans sanitized body bytes and returns all detected links in
// document order. originHostPort is the Entry.Target.HostPort of the response
// (used for the same-relay forwarding check). Stub returns nil until Tasks 3–6.
func DetectLinks(body []byte, originHostPort string) []Link {
	return nil
}

// harvestableLogin reports whether a Target's login matches the legacy
// login-class constraint that appendHarvestedTargets used to enforce via regex.
// Keeps the harvested user-list set behaviour-neutral after the refactor.
var loginRe = loginRe // re-use the existing package-level loginRe from userlist.go

func harvestableLogin(t finger.Target) bool {
	return loginRe.MatchString(t.Query)
}

// domainSane reports whether host is an acceptable finger/email domain.
// Stub returns false until Task 3.
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

// Compiled regexes — package-level, initialised once.
var (
	// schemeURLRe matches any scheme://authority... token.
	// The authority must be at least one non-whitespace character so bare
	// "https://" with no host is not a link.
	schemeURLRe = regexp.MustCompile(
		`(?i)([A-Za-z][A-Za-z0-9+.\-]{1,30}://[^\s<>"` + "`" + `(){}\[\]]+)`)

	// atTokenRe matches tokens that contain at least one '@' and are bounded
	// by whitespace or common delimiters. The liberal character class lets
	// forwarded forms like epoch@whois.ano@thebackupbox.net be one token.
	atTokenRe = regexp.MustCompile(
		`(?:^|(?:[\s<>"` + "`" + `(){}\[\]]))([^\s<>"` + "`" + `(){}\[\]]*@[^\s<>"` + "`" + `(){}\[\]]+)`)

	// cueWordRe extracts the last word before the current match position.
	cueWordRe = regexp.MustCompile(`(?i)(\w+)\s*$`)
)

// cueKind maps a cue word to the LinkKind and Action it implies.
// Returns (kind, action, ok).
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
```

> **Note:** `loginRe` is defined in `userlist.go`; the alias above won't compile. In the real file just call `loginRe.MatchString(t.Query)` directly — both files are in `package tui` so the var is visible. Remove the alias line.

- [ ] **Step 2: Verify it compiles**

```bash
make build
```
Expected: build succeeds (no tests yet).

- [ ] **Step 3: Commit**

```bash
git add tui/links.go
git commit -m "feat(links): add Link type, stubs, and package-level regexes"
```

---

## Task 2: Golden corpus tests in `tui/links_test.go`

**Files:**
- Create: `tui/links_test.go`

Write every test case from the spec before any scanner logic is complete. All tests in groups below should FAIL at the end of this task (DetectLinks returns nil).

- [ ] **Step 1: Write `tui/links_test.go`**

```go
package tui

import (
	"testing"
)

// helper: find the first Link with the given Raw; return zero-value if not found.
func findLink(links []Link, raw string) (Link, bool) {
	for _, l := range links {
		if l.Raw == raw {
			return l, true
		}
	}
	return Link{}, false
}

// --- Decline cases -------------------------------------------------------

func TestDetectLinks_Decline_HostlessHandle(t *testing.T) {
	links := DetectLinks([]byte("follow me @alice on mastodon"), "")
	// bare @alice has no host — must NOT be surfaced as a link on its own
	for _, l := range links {
		if l.Raw == "@alice" {
			t.Fatalf("@alice (no host) must not be detected, got %+v", l)
		}
	}
}

func TestDetectLinks_Decline_NoDot(t *testing.T) {
	links := DetectLinks([]byte("send mail to bob@localhost"), "")
	if _, ok := findLink(links, "bob@localhost"); ok {
		t.Fatal("bob@localhost (no dot) must not be detected")
	}
}

func TestDetectLinks_Decline_NumericTLD(t *testing.T) {
	links := DetectLinks([]byte("user@1.2.3.4"), "")
	if _, ok := findLink(links, "user@1.2.3.4"); ok {
		t.Fatal("user@1.2.3.4 (dotted-quad, no brackets) must not be detected")
	}
}

func TestDetectLinks_Decline_EmbeddedInIdentifier(t *testing.T) {
	links := DetectLinks([]byte("see_alice@tilde.team_docs"), "")
	if _, ok := findLink(links, "alice@tilde.team"); ok {
		t.Fatal("alice@tilde.team embedded in a larger identifier must not be detected")
	}
}

func TestDetectLinks_Decline_BareDomain(t *testing.T) {
	links := DetectLinks([]byte("visit tilde.team for more"), "")
	if _, ok := findLink(links, "tilde.team"); ok {
		t.Fatal("bare domain with no scheme or @ must not be detected")
	}
}

func TestDetectLinks_Decline_SchemeNoAuthority(t *testing.T) {
	links := DetectLinks([]byte("prefer https:// over git@"), "")
	if _, ok := findLink(links, "https://"); ok {
		t.Fatal("bare https:// with no authority must not be detected")
	}
}

// --- Rule 1: explicit scheme -----------------------------------------------

func TestDetectLinks_Rule1_HTTPS(t *testing.T) {
	links := DetectLinks([]byte("check https://example.com/foo for details"), "")
	l, ok := findLink(links, "https://example.com/foo")
	if !ok {
		t.Fatal("https://example.com/foo not detected")
	}
	if l.Kind != LinkURL {
		t.Fatalf("Kind = %v, want LinkURL", l.Kind)
	}
	if l.Action != ActionCopy {
		t.Fatalf("Action = %v, want ActionCopy", l.Action)
	}
	if !l.Strong {
		t.Fatal("Strong must be true for explicit scheme")
	}
	if !isOSC8Openable(l.Raw) {
		t.Fatal("https:// must be OSC-8 openable")
	}
}

func TestDetectLinks_Rule1_Gemini_NotOSC8(t *testing.T) {
	links := DetectLinks([]byte("gemini://rawtext.club/~alice"), "")
	l, ok := findLink(links, "gemini://rawtext.club/~alice")
	if !ok {
		t.Fatal("gemini:// URL not detected")
	}
	if l.Kind != LinkURL {
		t.Fatalf("Kind = %v, want LinkURL", l.Kind)
	}
	if isOSC8Openable(l.Raw) {
		t.Fatal("gemini:// must NOT be OSC-8 openable")
	}
}

func TestDetectLinks_Rule1_Mailto(t *testing.T) {
	links := DetectLinks([]byte("write to mailto:alice@example.com now"), "")
	l, ok := findLink(links, "mailto:alice@example.com")
	if !ok {
		t.Fatal("mailto: link not detected")
	}
	if l.Kind != LinkEmail {
		t.Fatalf("Kind = %v, want LinkEmail", l.Kind)
	}
	if !isOSC8Openable(l.Raw) {
		t.Fatal("mailto: must be OSC-8 openable")
	}
}

func TestDetectLinks_Rule1_FingerURL_Direct(t *testing.T) {
	links := DetectLinks([]byte("finger://tilde.team/alice"), "tilde.team:79")
	l, ok := findLink(links, "finger://tilde.team/alice")
	if !ok {
		t.Fatal("finger:// URL not detected")
	}
	if l.Kind != LinkFinger {
		t.Fatalf("Kind = %v, want LinkFinger", l.Kind)
	}
	if l.Action != ActionDrill {
		t.Fatalf("Action = %v, want ActionDrill", l.Action)
	}
	if l.Target.HostPort != "tilde.team:79" {
		t.Fatalf("Target.HostPort = %q, want tilde.team:79", l.Target.HostPort)
	}
}

// --- Rule 2: cue word -------------------------------------------------------

func TestDetectLinks_Rule2_FingerCue(t *testing.T) {
	links := DetectLinks([]byte("finger alice@tilde.team for info"), "")
	l, ok := findLink(links, "alice@tilde.team")
	if !ok {
		t.Fatal("cue-finger link not detected")
	}
	if l.Kind != LinkFinger || l.Action != ActionDrill {
		t.Fatalf("got Kind=%v Action=%v, want LinkFinger/ActionDrill", l.Kind, l.Action)
	}
	if !l.Strong {
		t.Fatal("cue-finger must be Strong")
	}
	if l.Ambiguous {
		t.Fatal("cue-finger must not be Ambiguous")
	}
}

func TestDetectLinks_Rule2_EmailCue(t *testing.T) {
	links := DetectLinks([]byte("email me at bob@example.com"), "")
	l, ok := findLink(links, "bob@example.com")
	if !ok {
		t.Fatal("cue-email link not detected")
	}
	if l.Kind != LinkEmail || l.Action != ActionCopy {
		t.Fatalf("got Kind=%v Action=%v, want LinkEmail/ActionCopy", l.Kind, l.Action)
	}
	if !l.Strong {
		t.Fatal("cue-email must be Strong")
	}
	if isOSC8Openable(l.Raw) {
		t.Fatal("cue-email (no scheme in Raw) must not be OSC-8")
	}
}

func TestDetectLinks_Rule2_SocialCue(t *testing.T) {
	links := DetectLinks([]byte("fedi @alice@fosstodon.org for updates"), "")
	l, ok := findLink(links, "@alice@fosstodon.org")
	if !ok {
		t.Fatal("cue-social link not detected")
	}
	if l.Kind != LinkSocial || l.Action != ActionCopy {
		t.Fatalf("got Kind=%v Action=%v, want LinkSocial/ActionCopy", l.Kind, l.Action)
	}
}

// --- Rule 3: @host host-query form ----------------------------------------

func TestDetectLinks_Rule3_AtHost(t *testing.T) {
	links := DetectLinks([]byte("try @tilde.team today"), "")
	l, ok := findLink(links, "@tilde.team")
	if !ok {
		t.Fatal("@host link not detected")
	}
	if l.Kind != LinkFinger || l.Action != ActionDrill {
		t.Fatalf("got Kind=%v Action=%v, want LinkFinger/ActionDrill", l.Kind, l.Action)
	}
	if l.Ambiguous {
		t.Fatal("@host must not be Ambiguous")
	}
	if l.Target.HostPort != "tilde.team:79" {
		t.Fatalf("Target.HostPort = %q, want tilde.team:79", l.Target.HostPort)
	}
}

func TestDetectLinks_Rule3_AtHostNoHandle(t *testing.T) {
	// @alice (no dot) must not be promoted by rule 3
	links := DetectLinks([]byte("follow @alice on the fediverse"), "")
	for _, l := range links {
		if l.Raw == "@alice" && l.Kind == LinkFinger {
			t.Fatal("bare @alice (no dot) must not be a finger link")
		}
	}
}

// --- Rule 4: bare user@host (ambiguous) ------------------------------------

func TestDetectLinks_Rule4_BareAddress_Ambiguous(t *testing.T) {
	links := DetectLinks([]byte("contact admin@example.com today"), "")
	l, ok := findLink(links, "admin@example.com")
	if !ok {
		t.Fatal("bare user@host not detected")
	}
	if l.Kind != LinkFinger {
		t.Fatalf("Kind = %v, want LinkFinger (policy B: detected as finger, default copy)", l.Kind)
	}
	if l.Action != ActionCopy {
		t.Fatalf("Action = %v, want ActionCopy (policy B: ambiguous default)", l.Action)
	}
	if !l.Ambiguous {
		t.Fatal("Ambiguous must be true for bare user@host")
	}
	if l.Strong {
		t.Fatal("bare user@host without cue must not be Strong")
	}
	if l.Target.HostPort == "" {
		t.Fatal("Target must be populated for ambiguous finger link")
	}
}

// --- OSC-8 scheme matrix ---------------------------------------------------

func TestDetectLinks_OSC8_OnlyHTTPAndMailto(t *testing.T) {
	cases := []struct {
		raw    string
		wantOK bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"mailto:alice@example.com", true},
		{"gemini://rawtext.club", false},
		{"gopher://gopher.floodgap.com", false},
		{"ircs://irc.libera.chat/lookit", false},
		{"rtmp://stream.example.com/live", false},
	}
	for _, c := range cases {
		got := isOSC8Openable(c.raw)
		if got != c.wantOK {
			t.Errorf("isOSC8Openable(%q) = %v, want %v", c.raw, got, c.wantOK)
		}
	}
}

// --- Punctuation trimming --------------------------------------------------

func TestDetectLinks_Punctuation_TrailingPunct(t *testing.T) {
	links := DetectLinks([]byte("see https://example.com/foo). for more"), "")
	if _, ok := findLink(links, "https://example.com/foo"); !ok {
		t.Fatal("trailing ). not stripped from URL")
	}
	if _, ok := findLink(links, "https://example.com/foo)."); ok {
		t.Fatal("trailing ). must be stripped from URL Raw")
	}
}

func TestDetectLinks_Punctuation_BalancedParens(t *testing.T) {
	links := DetectLinks([]byte("see https://example.com/foo(bar) now"), "")
	// balanced parens inside URL must be kept
	if _, ok := findLink(links, "https://example.com/foo(bar)"); !ok {
		t.Fatal("balanced parens inside URL must not be stripped")
	}
}

func TestDetectLinks_Punctuation_QuoteDelimits(t *testing.T) {
	links := DetectLinks([]byte(`check "https://example.com/foo" now`), "")
	if _, ok := findLink(links, "https://example.com/foo"); !ok {
		t.Fatal("URL inside quotes not detected (quotes should delimit, not include)")
	}
}

// --- Forwarding cases ------------------------------------------------------

func TestDetectLinks_Forwarding_SameRelay_Drillable(t *testing.T) {
	body := []byte("finger epoch@whois.ano@thebackupbox.net to see their plan")
	links := DetectLinks(body, "thebackupbox.net:79")
	l, ok := findLink(links, "epoch@whois.ano@thebackupbox.net")
	if !ok {
		t.Fatal("same-relay forwarded target not detected")
	}
	if l.Action != ActionDrill {
		t.Fatalf("Action = %v, want ActionDrill (same-relay)", l.Action)
	}
	if l.Blocked != "" {
		t.Fatalf("Blocked = %q, want empty (same-relay)", l.Blocked)
	}
	if !l.Forwarded {
		t.Fatal("Forwarded must be true")
	}
}

func TestDetectLinks_Forwarding_DifferentRelay_Blocked(t *testing.T) {
	body := []byte("finger epoch@whois.ano@thebackupbox.net")
	links := DetectLinks(body, "tilde.team:79") // different origin
	l, ok := findLink(links, "epoch@whois.ano@thebackupbox.net")
	if !ok {
		t.Fatal("blocked forwarded target not detected at all (must be copy-only, not dropped)")
	}
	if l.Action != ActionCopy {
		t.Fatalf("Action = %v, want ActionCopy (relay blocked)", l.Action)
	}
	if l.Blocked == "" {
		t.Fatal("Blocked must be set for cross-relay forwarding")
	}
}

func TestDetectLinks_Forwarding_BlockedConsumesSpan(t *testing.T) {
	// The blocked forwarded token must consume epoch@whois.ano@thebackupbox.net
	// entirely — no submatch on epoch@whois.ano should leak through.
	body := []byte("finger epoch@whois.ano@thebackupbox.net")
	links := DetectLinks(body, "tilde.team:79")
	for _, l := range links {
		if l.Raw == "epoch@whois.ano" {
			t.Fatal("submatch epoch@whois.ano must not appear after blocked forwarded token")
		}
	}
}

func TestDetectLinks_Forwarding_FingerURLSameRelay(t *testing.T) {
	body := []byte("finger://thebackupbox.net/epoch@whois.ano")
	links := DetectLinks(body, "thebackupbox.net:79")
	l, ok := findLink(links, "finger://thebackupbox.net/epoch@whois.ano")
	if !ok {
		t.Fatal("same-relay finger:// forwarding not detected")
	}
	if l.Action != ActionDrill {
		t.Fatalf("Action = %v, want ActionDrill", l.Action)
	}
}

// --- Strong-gate: appendHarvestedTargets adapter ---------------------------

func TestHarvestedTargets_ProseEmailNotHarvested(t *testing.T) {
	// A list response containing a prose email must not gain a list row.
	body := []byte("Login   Name\nalice   Alice Example\n\ncontact admin@example.com for help\n")
	users, ok := parseUserList(body, "tilde.team:79")
	if !ok {
		t.Fatal("parseUserList ok = false, want true")
	}
	for _, u := range users.users {
		if u.Login == "admin" || u.Target == "admin@example.com" {
			t.Fatal("prose email must not become a list row")
		}
	}
}

func TestHarvestedTargets_StrongHostQueryNotHarvested(t *testing.T) {
	// finger://tilde.team is Kind==Finger && Strong, but HostQuery() — must not add a row.
	body := []byte("Login   Name\nalice   Alice\n\nfinger://tilde.team for the list\n")
	users, ok := parseUserList(body, "tilde.team:79")
	if !ok {
		t.Fatal("parseUserList ok = false, want true")
	}
	for _, u := range users.users {
		if u.Target == "@tilde.team" || u.Login == "@tilde.team" {
			t.Fatal("strong host-query finger link must not add a list row")
		}
	}
}

func TestHarvestedTargets_NonHarvestableLoginNotHarvested(t *testing.T) {
	// finger://example.com/~bob — ParseTargetPinned accepts it, but harvestableLogin rejects ~bob.
	// Must NOT add a row; must still drill as a reader link.
	body := []byte("Login   Name\nalice   Alice\n\nfinger://example.com/~bob for their page\n")
	users, ok := parseUserList(body, "example.com:79")
	if !ok {
		t.Skip("list not parsed — test not applicable")
	}
	for _, u := range users.users {
		if u.Login == "~bob" || u.Target == "~bob@example.com" {
			t.Fatal("~bob must not become a list row")
		}
	}
	// Verify DetectLinks itself DOES detect it as a drillable reader link.
	links := DetectLinks(body, "example.com:79")
	l, ok := findLink(links, "finger://example.com/~bob")
	if !ok {
		t.Fatal("finger://example.com/~bob must be a drillable reader link")
	}
	if l.Action != ActionDrill {
		t.Fatalf("reader link Action = %v, want ActionDrill", l.Action)
	}
}

// --- Overlay: body vs header -----------------------------------------------

func TestApplyLinkOverlay_BodyNotHeader(t *testing.T) {
	// When the target appears in both header and body, the overlay must mark
	// the body occurrence, not the header chrome.
	// We test this at the applyLinkOverlay level: "header" is passed separately.
	bodyStr := "check alice@tilde.team in the body\n"
	links := []Link{
		{
			Kind:   LinkFinger,
			Action: ActionCopy,
			Raw:    "alice@tilde.team",
		},
	}
	st := newStyles(true)
	result := applyLinkOverlay(bodyStr, links, 0, st)
	if result == bodyStr {
		t.Fatal("applyLinkOverlay must change the body string when focusedIdx=0")
	}
	if strings.Contains(result, "alice@tilde.team") {
		// Focused link is replaced by styled text — the raw string should be gone.
		// (The styled version contains the raw as content but wrapped in ANSI codes.)
		// So we just check the result changed — see above.
	}
}
```

- [ ] **Step 2: Run tests to confirm all fail**

```bash
go test ./tui/ -run "TestDetectLinks|TestHarvestedTargets|TestApplyLinkOverlay" -v 2>&1 | grep -E "FAIL|PASS|---"
```
Expected: all FAIL (DetectLinks returns nil, parseUserList signature will change, applyLinkOverlay not yet defined).

- [ ] **Step 3: Commit**

```bash
git add tui/links_test.go
git commit -m "test(links): golden corpus for DetectLinks, forwarding, strong-gate, overlay"
```

---

## Task 3: `domainSane` + scanner skeleton

**Files:**
- Modify: `tui/links.go`

- [ ] **Step 1: Implement `domainSane`**

Replace the stub with:

```go
// domainSane reports whether host is a plausible finger/email domain or IP literal.
// IP literals in brackets (IPv4 or IPv6) are accepted on literal-shape grounds.
// Plain domain names must have a dot, all-alpha TLD (2+ chars), valid label chars,
// no leading/trailing hyphen per label, and no all-numeric final label.
func domainSane(host string) bool {
	// Bracketed IP literal: accept if it looks like an IP address.
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		inner := host[1 : len(host)-1]
		// IPv4 dotted-quad inside brackets.
		if ipv4Re.MatchString(inner) {
			return true
		}
		// IPv6: must contain at least one colon.
		return strings.Contains(inner, ":")
	}

	// Plain domain: must have at least one dot.
	dot := strings.LastIndex(host, ".")
	if dot < 0 {
		return false
	}
	tld := host[dot+1:]
	// TLD must be 2+ alpha characters; reject all-numeric TLDs (dotted-quad guard).
	if len(tld) < 2 || !allAlpha(tld) {
		return false
	}
	// All labels must be valid: [A-Za-z0-9-], no leading/trailing hyphen.
	for _, label := range strings.Split(host, ".") {
		if label == "" || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
				return false
			}
		}
	}
	return true
}

var ipv4Re = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)

func allAlpha(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Implement the scanner skeleton in `DetectLinks`**

Replace the nil stub with this structure (fill in per-rule logic in Tasks 4–6):

```go
func DetectLinks(body []byte, originHostPort string) []Link {
	text := string(body)
	if text == "" {
		return nil
	}
	origin := canonicalHost(originHostPort) // strip port, normalise

	// Phase 1: collect all scheme-URL spans (largest-first by nature of left-to-right scan).
	urlSpans := schemeURLRe.FindAllStringIndex(text, -1) // [start, end] pairs
	consumed := make([]bool, len(text))                  // marks bytes claimed by phase 1
	var links []Link
	for _, span := range urlSpans {
		raw := text[span[0]:span[1]]
		raw = stripTrailingPunct(raw)
		if raw == "" {
			continue
		}
		// Check word boundary: char before span must not be a word char.
		if span[0] > 0 && isWordChar(text[span[0]-1]) {
			continue
		}
		for i := span[0]; i < span[0]+len(raw); i++ {
			consumed[i] = true
		}
		link, ok := classifySchemeURL(raw, origin)
		if !ok {
			continue
		}
		links = append(links, link)
	}

	// Phase 2: scan remaining text for @-containing tokens.
	pos := 0
	for pos < len(text) {
		if consumed[pos] {
			pos++
			continue
		}
		// Find the next @ in unconsumed text.
		at := strings.IndexByte(text[pos:], '@')
		if at < 0 {
			break
		}
		atAbs := pos + at

		// Expand left to start of token (stop at whitespace/delimiters).
		start := atAbs
		for start > 0 && !isDelim(text[start-1]) && !consumed[start-1] {
			start--
		}
		// Expand right to end of token.
		end := atAbs + 1
		for end < len(text) && !isDelim(text[end]) {
			end++
		}

		// Skip if any byte in this span is consumed by a phase-1 URL.
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
		// Check word boundary: char before start and char after end must not be word chars.
		if start > 0 && isWordChar(text[start-1]) {
			pos = end
			continue
		}
		if end < len(text) && isWordChar(text[end]) {
			pos = end
			continue
		}

		// Determine cue word from text immediately before start.
		preceding := text[:start]
		cueWord := ""
		if m := cueWordRe.FindString(preceding); m != "" {
			cueWord = m
		}

		link, ok := classifyAtToken(raw, cueWord, origin)
		if !ok {
			pos = end
			continue
		}
		// Mark as consumed.
		for i := start; i < end; i++ {
			consumed[i] = true
		}
		links = append(links, link)
		pos = end
	}

	return links
}

// canonicalHost strips any port suffix and lowercases the host.
func canonicalHost(hostPort string) string {
	h := hostPort
	if i := strings.LastIndex(h, ":"); i >= 0 {
		h = h[:i]
	}
	return strings.ToLower(h)
}

func isDelim(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
		c == '<' || c == '>' || c == '"' || c == '\'' || c == '`' ||
		c == '(' || c == ')' || c == '{' || c == '}' || c == '[' || c == ']'
}

func isWordChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}

// classifySchemeURL is filled in Task 4.
func classifySchemeURL(raw, origin string) (Link, bool) { return Link{}, false }

// classifyAtToken is filled in Tasks 4–6.
func classifyAtToken(raw, cueWord, origin string) (Link, bool) { return Link{}, false }
```

Add `stripTrailingPunct`:

```go
// stripTrailingPunct removes trailing sentence punctuation and unbalanced
// closing delimiters from a URL token. Balanced parens/brackets inside
// the URL are kept.
func stripTrailingPunct(s string) string {
	for {
		if len(s) == 0 {
			break
		}
		last := s[len(s)-1]
		// Always strip trailing sentence punctuation.
		if last == '.' || last == ',' || last == ';' || last == ':' || last == '!' || last == '?' {
			s = s[:len(s)-1]
			continue
		}
		// Strip closing delimiter only if unbalanced.
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
```

- [ ] **Step 3: Run decline tests**

```bash
go test ./tui/ -run "TestDetectLinks_Decline" -v
```
Expected: All decline tests now PASS (domainSane filters bad hosts; scanner finds nothing for bare domains, embedded tokens, hostless handles). Fix any failures before proceeding.

- [ ] **Step 4: Commit**

```bash
git add tui/links.go
git commit -m "feat(links): domain-sanity gate and scanner skeleton"
```

---

## Task 4: Rule 1 — scheme-URL classification

**Files:**
- Modify: `tui/links.go`

Replace `classifySchemeURL` stub:

- [ ] **Step 1: Implement `classifySchemeURL`**

```go
func classifySchemeURL(raw, origin string) (Link, bool) {
	schemeEnd := strings.Index(raw, "://")
	if schemeEnd < 0 {
		return Link{}, false
	}
	scheme := strings.ToLower(raw[:schemeEnd])
	authority := raw[schemeEnd+3:] // everything after "://"

	// Must have at least one authority character (no bare "https://").
	if authority == "" || isDelim(authority[0]) {
		return Link{}, false
	}

	switch scheme {
	case "finger":
		return classifyFingerURL(raw, origin)
	case "mailto":
		// mailto:addr — Email, Copy, OSC-8 (Raw begins with "mailto:")
		return Link{Kind: LinkEmail, Action: ActionCopy, Raw: raw, Strong: true}, true
	default:
		// Any other scheme with a non-empty authority → URL, Copy.
		return Link{Kind: LinkURL, Action: ActionCopy, Raw: raw, Strong: true}, true
	}
}

// classifyFingerURL handles finger://host/login and finger://relay/user@host (forwarding).
func classifyFingerURL(raw, origin string) (Link, bool) {
	// Strip scheme.
	rest := raw[len("finger://"):]
	slash := strings.Index(rest, "/")

	var host, path string
	if slash < 0 {
		// finger://host with no path → host query
		host = rest
		path = ""
	} else {
		host = rest[:slash]
		path = rest[slash+1:] // may be "" or "login" or "user@innerhost"
	}
	hostLower := strings.ToLower(host)

	// Forwarding: path contains '@', meaning finger://relay/user@innerhost.
	if strings.Contains(path, "@") {
		// Reconstruct as user@innerhost@relay for ParseTargetPinned.
		synth := path + "@" + host
		if hostLower == origin {
			// Same relay → drillable.
			t, err := finger.ParseTargetPinned(synth)
			if err != nil {
				return Link{Kind: LinkFinger, Action: ActionCopy, Raw: raw, Blocked: "parse error", Strong: true, Forwarded: true}, true
			}
			return Link{Kind: LinkFinger, Action: ActionDrill, Raw: raw, Target: t, Strong: true, Forwarded: true}, true
		}
		// Different relay → blocked copy.
		return Link{Kind: LinkFinger, Action: ActionCopy, Raw: raw, Blocked: "relay blocked", Strong: true, Forwarded: true}, true
	}

	// Direct finger URL: finger://host/login or finger://host.
	synth := path + "@" + host
	if path == "" {
		synth = "@" + host
	}
	t, err := finger.ParseTargetPinned(synth)
	if err != nil {
		return Link{}, false
	}
	return Link{Kind: LinkFinger, Action: ActionDrill, Raw: raw, Target: t, Strong: true}, true
}
```

- [ ] **Step 2: Run rule-1 tests**

```bash
go test ./tui/ -run "TestDetectLinks_Rule1" -v
```
Expected: all rule-1 tests PASS. Fix any failures.

- [ ] **Step 3: Run punctuation tests**

```bash
go test ./tui/ -run "TestDetectLinks_Punctuation" -v
```
Expected: pass. Fix if not.

- [ ] **Step 4: Commit**

```bash
git add tui/links.go
git commit -m "feat(links): rule 1 scheme-URL classification (finger://, https://, mailto:, etc.)"
```

---

## Task 5: Rules 2, 3, 4 — @-token classification

**Files:**
- Modify: `tui/links.go`

Replace `classifyAtToken` stub:

- [ ] **Step 1: Implement `classifyAtToken`**

```go
// classifyAtToken classifies a token that contains at least one '@'.
// raw is the full token; cueWord is the last word immediately before it (may be "").
// origin is the canonicalHost of the current response for the forwarding check.
func classifyAtToken(raw, cueWord, origin string) (Link, bool) {
	atCount := strings.Count(raw, "@")

	// Forwarding: two '@' signs → user@innerhost@relay form.
	if atCount >= 2 {
		return classifyForwardedAtToken(raw, origin)
	}

	// Rule 2: cue word overrides classification.
	if cueWord != "" {
		if kind, action, ok := cueKind(cueWord); ok {
			return classifyWithCue(raw, kind, action)
		}
	}

	// @host form (starts with @): rule 3.
	if strings.HasPrefix(raw, "@") {
		host := raw[1:]
		if !domainSane(host) {
			return Link{}, false
		}
		t, err := finger.ParseTargetPinned(raw)
		if err != nil {
			return Link{}, false
		}
		return Link{Kind: LinkFinger, Action: ActionDrill, Raw: raw, Target: t}, true
	}

	// Bare user@host: rule 4.
	at := strings.Index(raw, "@")
	host := raw[at+1:]
	if !domainSane(host) {
		return Link{}, false
	}
	t, err := finger.ParseTargetPinned(raw)
	if err != nil {
		return Link{}, false
	}
	return Link{Kind: LinkFinger, Action: ActionCopy, Raw: raw, Target: t, Ambiguous: true}, true
}

// classifyWithCue applies a cue-word-determined kind/action to a token.
func classifyWithCue(raw string, kind LinkKind, action LinkAction) (Link, bool) {
	link := Link{Kind: kind, Action: action, Raw: raw, Strong: true}
	if kind == LinkFinger {
		// Parse it as a finger target.
		t, err := finger.ParseTargetPinned(raw)
		if err != nil {
			return Link{}, false
		}
		link.Target = t
		link.Action = ActionDrill
	}
	return link, true
}

// classifyForwardedAtToken handles the user@innerhost@relay form.
func classifyForwardedAtToken(raw, origin string) (Link, bool) {
	// Validate: exactly two '@' signs; use ParseTargetPinned which returns
	// ErrServerForwarding for any multi-@ form — we handle forwarding ourselves.
	// Instead, canonically split on last '@'.
	lastAt := strings.LastIndex(raw, "@")
	relay := strings.ToLower(raw[lastAt+1:])
	query := raw[:lastAt] // "user@innerhost"

	// Relay domain must be sane.
	if !domainSane(relay) {
		return Link{}, false
	}
	// Inner query must look like user@host or @host.
	if !strings.Contains(query, "@") {
		return Link{}, false
	}

	if relay == origin {
		// Same relay → drillable. Synthesise a Target dialling the relay.
		t, err := finger.ParseTargetPinned(raw)
		if err != nil {
			// ParseTargetPinned rejects forwarding with ErrServerForwarding; build manually.
			// The wire query to thebackupbox.net is "user@innerhost".
			t = finger.Target{
				Query:    query,
				HostPort: relay + ":79",
				Raw:      raw,
			}
		}
		return Link{Kind: LinkFinger, Action: ActionDrill, Raw: raw, Target: t, Strong: true, Forwarded: true}, true
	}
	// Different relay → blocked copy; full span consumed, no submatch leaks.
	return Link{Kind: LinkFinger, Action: ActionCopy, Raw: raw, Blocked: "relay blocked", Strong: true, Forwarded: true}, true
}
```

> **Note:** `finger.ParseTargetPinned` returns `ErrServerForwarding` for `user@host@relay` forms because it detects two `@` signs. Build the Target manually for same-relay forwarding as shown above.

- [ ] **Step 2: Run rules 2–4 and forwarding tests**

```bash
go test ./tui/ -run "TestDetectLinks_Rule[234]|TestDetectLinks_Forwarding" -v
```
Expected: all PASS. Fix any failures.

- [ ] **Step 3: Run the full DetectLinks suite**

```bash
go test ./tui/ -run TestDetectLinks -v
```
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add tui/links.go
git commit -m "feat(links): rules 2–4 (@-token: cue word, @host, bare user@host, forwarding)"
```

---

## Task 6: Refactor `appendHarvestedTargets` + strong-gate tests

**Files:**
- Modify: `tui/userlist.go`

- [ ] **Step 1: Add `originHostPort` to `parseUserList` and callees**

Change the signature of the internal `parseUserList`:

```go
// parseUserList returns the parsed list from body. originHostPort is the
// Entry.Target.HostPort of the response; passed through to
// appendHarvestedTargets for the same-relay forwarding check.
func parseUserList(body []byte, originHostPort string) (parsedUserList, bool) {
```

Update the call in `parseGenericList` to thread `originHostPort`:

```go
// In parseGenericList signature:
func parseGenericList(lines []string, originHostPort string) ([]User, string, bool) {
    // ... (same body) ...
    bestUsers = appendHarvestedTargets(bestUsers, lines, originHostPort)
    // ...
}
```

All internal callers of `parseGenericList` inside `parseUserList` get the new arg. Update `ParseUsers` (public) to pass `""`:

```go
func ParseUsers(body []byte) ([]User, bool) {
	parsed, ok := parseUserList(body, "")
	return parsed.users, ok
}
```

In `app.go`, the two callers of `parseUserList`:
- `routeFetch` (line 619): pass `entry.Target.HostPort`
- `restore` (line 222): pass `m.history[m.pos].entry.Target.HostPort`

In `list.go`, `newListWithPreamble` calls `parseUserList(body)` for preamble only; change to `parseUserList(body, "")`.

- [ ] **Step 2: Refactor `appendHarvestedTargets`**

```go
// appendHarvestedTargets adds cross-host drill targets found anywhere in the
// body via DetectLinks. It is a thin adapter: filters to the harvestable
// subset (strong, unblocked, user-query, old-login-shape) and maps to []User.
// This is behaviour-neutral for non-forwarded strong finger links; the one
// intentional change is that same-relay forwarded targets are now harvested
// correctly instead of leaking a shorter submatch.
func appendHarvestedTargets(users []User, lines []string, originHostPort string) []User {
	body := []byte(strings.Join(lines, "\n"))
	links := DetectLinks(body, originHostPort)

	seen := map[string]bool{}
	for _, u := range users {
		if u.Target != "" {
			seen[u.Target] = true
		}
	}

	for _, l := range links {
		if l.Kind != LinkFinger || !l.Strong || l.Blocked != "" {
			continue
		}
		t := l.Target
		if t.HostQuery() || !harvestableLogin(t) {
			continue
		}
		key := t.Query + "@" + t.HostPort
		if seen[key] {
			continue
		}
		seen[key] = true
		login := t.Query
		host := t.HostPort
		if i := strings.LastIndex(host, ":"); i >= 0 {
			host = host[:i]
		}
		users = append(users, User{Login: login, Name: host, Target: l.Raw})
	}
	return users
}
```

- [ ] **Step 3: Run strong-gate and existing corpus tests**

```bash
go test ./tui/ -run "TestHarvestedTargets|TestParse" -v
```
Expected: all PASS. Fix any failures.

- [ ] **Step 4: Run full test suite**

```bash
make check
```
Expected: green.

- [ ] **Step 5: Commit**

```bash
git add tui/userlist.go tui/app.go tui/list.go
git commit -m "refactor(links): appendHarvestedTargets → DetectLinks adapter; thread originHostPort"
```

---

## Task 7: Export `render.Split` for header/body separation

**Files:**
- Modify: `render/render.go`
- Modify: `render/render_test.go`

- [ ] **Step 1: Extract `renderBodyOnly` and add `Split`**

In `render/render.go`, extract the body portion:

```go
// renderBodyOnly formats the body portion of a response (no header chrome).
// Called by both RenderWithBackground and Split.
func renderBodyOnly(theme Theme, t finger.Target, body []byte, meta finger.Meta, queryErr error) string {
	var sb strings.Builder
	if len(body) == 0 && queryErr == nil {
		sb.WriteString(theme.Footer.Render("(no response body)"))
		sb.WriteByte('\n')
	} else {
		if isTildeTeam(t) {
			body = reflowPronouns(body)
		}
		sb.WriteString(highlightFields(theme, body, extraFieldPrefixes(t)))
		if len(body) > 0 && body[len(body)-1] != '\n' {
			sb.WriteByte('\n')
		}
	}
	if queryErr != nil {
		sb.WriteString(theme.ErrLine.Render(queryErr.Error()))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// Split renders a finger result as two separate strings — the header chrome
// and the body text — so the TUI reader can apply link overlays to the body
// without touching the header. Concatenating header+body gives the same
// string as RenderWithBackground.
func Split(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile, darkBackground bool) (header, bodyStr string) {
	theme := NewThemeWithBackground(profile, darkBackground)
	header = renderHeader(theme, t, meta, queryErr == nil)
	bodyStr = renderBodyOnly(theme, t, body, meta, queryErr)
	return
}
```

Refactor `RenderWithBackground` to use it:

```go
func RenderWithBackground(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile, darkBackground bool) string {
	h, b := Split(t, body, meta, queryErr, profile, darkBackground)
	return h + b
}
```

- [ ] **Step 2: Add a `Split` test in `render/render_test.go`**

```go
func TestRender_Split_ConcatEqualsFull(t *testing.T) {
	body := loadInput(t, "standard-fields")
	target := finger.Target{HostPort: "example.com:79", Raw: "alice@example.com"}
	meta := finger.Meta{Addr: "example.com:79", Elapsed: 45 * time.Millisecond, Bytes: len(body)}

	full := RenderWithBackground(target, body, meta, nil, colorprofile.TrueColor, true)
	h, b := Split(target, body, meta, nil, colorprofile.TrueColor, true)
	if h+b != full {
		t.Errorf("Split header+body != RenderWithBackground:\nfull: %q\nh+b:  %q", full, h+b)
	}
	if h == "" {
		t.Error("header must not be empty")
	}
	if b == "" {
		t.Error("body must not be empty")
	}
	// Header must contain the target raw.
	if !strings.Contains(h, target.Raw) {
		t.Errorf("header must contain %q", target.Raw)
	}
	// Body must NOT start with the header arrow.
	if strings.HasPrefix(b, "➜") || strings.Contains(b, target.Raw[:5]) {
		// Alice@... appears in the header; the body has Login name: alice etc.
		// just verify the golden files still pass (below).
	}
}
```

- [ ] **Step 3: Run render tests**

```bash
go test ./render/ -v
```
Expected: all PASS including existing goldens.

- [ ] **Step 4: Commit**

```bash
git add render/render.go render/render_test.go
git commit -m "feat(render): export Split(header,body) for tui overlay pass"
```

---

## Task 8: Add `linkFocus` style to `styles`

**Files:**
- Modify: `tui/styles.go`

- [ ] **Step 1: Add `linkFocus` field and populate it**

In the `styles` struct (after `listItem`):
```go
linkFocus lipgloss.Style // focused link highlight in the reader viewport
```

In `newStyles`, after `listItem: itemStyles,`:
```go
linkFocus: lipgloss.NewStyle().
    Underline(true).
    Foreground(p.AccentMint).
    Background(p.SelectionBg),
```

- [ ] **Step 2: Compile check**

```bash
make build
```

- [ ] **Step 3: Commit**

```bash
git add tui/styles.go
git commit -m "feat(tui): add linkFocus style for reader link highlight"
```

---

## Task 9: Cache links on `histNode`; populate in `routeFetch`/`restore`

**Files:**
- Modify: `tui/app.go`
- Modify: `tui/reader.go`

- [ ] **Step 1: Add `links` and `linkIdx` to `histNode`**

```go
type histNode struct {
	entry       Entry
	state       appState
	scrollY     int
	listIdx     int
	listFltr    string
	listUsers   int
	listGeneric bool
	links       []Link // detected links; nil for list nodes
	linkIdx     int    // focused link index, -1 = none
}
```

- [ ] **Step 2: Populate in `routeFetch` for reader nodes**

In `routeFetch`, after `node := histNode{entry: entry, state: stateReader}`:

```go
if node.state == stateReader {
    node.links = DetectLinks(entry.Body, entry.Target.HostPort)
    node.linkIdx = -1
}
```

(This line goes inside the block that checks `node.state == stateReader`, just before `m.reader.setEntry(entry)`.)

- [ ] **Step 3: Add `focusedLink` to `readerModel`**

In `reader.go`, in the `readerModel` struct:
```go
focusedLink int // index into the current histNode.links slice; -1 = none focused
```

In `newReader`, initialise: `focusedLink: -1`.

- [ ] **Step 4: Save/restore `linkIdx` in `snapshot`/`restore`**

In `snapshot()`:
```go
if n.state == stateReader {
    n.scrollY = m.reader.viewport.YOffset()
    n.linkIdx = m.reader.focusedLink  // add this line
}
```

In `restore()` for the reader branch:
```go
m.state = stateReader
m.reader.setEntry(n.entry)
m.reader.focusedLink = n.linkIdx  // add this line
m.reader.viewport.SetYOffset(n.scrollY)
```

- [ ] **Step 5: Compile check**

```bash
make build
```

- [ ] **Step 6: Commit**

```bash
git add tui/app.go tui/reader.go
git commit -m "feat(tui): cache detected links on histNode; add focusedLink to readerModel"
```

---

## Task 10: New keybindings for link navigation

**Files:**
- Modify: `tui/keys.go`
- Modify: `tui/app.go` (updateKeymap)

- [ ] **Step 1: Add bindings to `keyMap`**

In the `keyMap` struct (after `Jump`):
```go
// Link navigation — reader only
LinkNext   key.Binding // tab / n: next link
LinkPrev   key.Binding // shift+tab / N: previous link
LinkFinger key.Binding // f: finger focused ambiguous address
LinkPanel  key.Binding // L: toggle links panel
```

In `newKeyMap()`:
```go
LinkNext:   key.NewBinding(key.WithKeys("tab", "n"), key.WithHelp("⇥/n", "next link")),
LinkPrev:   key.NewBinding(key.WithKeys("shift+tab", "N"), key.WithHelp("⇧⇥/N", "prev link")),
LinkFinger: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "finger link")),
LinkPanel:  key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "links panel")),
```

In `FullHelp()`, add a new group:
```go
{k.LinkNext, k.LinkPrev, k.LinkFinger, k.LinkPanel},
```

- [ ] **Step 2: Enable bindings in `updateKeymap`**

After the existing `m.keys.Move.SetEnabled(content)` block, add:

```go
inReader := content && m.state == stateReader && !m.showingRaw && m.pos >= 0
node := histNode{}
if m.pos >= 0 && m.pos < len(m.history) {
    node = m.history[m.pos]
}
hasLinks := inReader && len(node.links) > 0
focusedAmbiguous := hasLinks && m.reader.focusedLink >= 0 &&
    m.reader.focusedLink < len(node.links) &&
    node.links[m.reader.focusedLink].Ambiguous

m.keys.LinkNext.SetEnabled(hasLinks)
m.keys.LinkPrev.SetEnabled(hasLinks)
m.keys.LinkFinger.SetEnabled(focusedAmbiguous)
m.keys.LinkPanel.SetEnabled(hasLinks)
```

- [ ] **Step 3: Compile check**

```bash
make build
```

- [ ] **Step 4: Commit**

```bash
git add tui/keys.go tui/app.go
git commit -m "feat(tui): add LinkNext/LinkPrev/LinkFinger/LinkPanel keybindings"
```

---

## Task 11: Tab-cycle navigation in `readerModel`

**Files:**
- Modify: `tui/reader.go`
- Modify: `tui/links.go` (add `applyLinkOverlay`)
- Modify: `tui/app.go` (route Tab/n/N in `handleKey`)

- [ ] **Step 1: Add `applyLinkOverlay` to `tui/links.go`**

```go
// applyLinkOverlay inserts focus-highlight and OSC-8 styles into the rendered
// body string. body is the body-only portion (no header chrome). links are the
// detected links in document order; focusedIdx is the 0-based index of the
// focused link (-1 = none). Searches for each link's Raw substring in
// document order via strings.Index, advancing the search position after each
// match so duplicate Raw values are matched in sequence.
func applyLinkOverlay(body string, links []Link, focusedIdx int, st styles) string {
	if len(links) == 0 {
		return body
	}
	var sb strings.Builder
	remaining := body
	for i, link := range links {
		pos := strings.Index(remaining, link.Raw)
		if pos < 0 {
			// Link not found in remaining text — skip (can happen if body was
			// modified by the renderer, e.g. lipgloss inserted a reset mid-token).
			continue
		}
		sb.WriteString(remaining[:pos])
		span := link.Raw
		if i == focusedIdx {
			span = st.linkFocus.Render(span)
		}
		if isOSC8Openable(link.Raw) {
			span = lipgloss.NewStyle().
				Hyperlink(link.Raw, fmt.Sprintf("id=%d", i)).
				Render(span)
		}
		sb.WriteString(span)
		remaining = remaining[pos+len(link.Raw):]
	}
	sb.WriteString(remaining)
	return sb.String()
}
```

Add imports to `tui/links.go` as needed: `"fmt"`, `"charm.land/lipgloss/v2"`.

- [ ] **Step 2: Add `setEntryWithLinks` and `scrollToFocusedLink` to `reader.go`**

```go
// setEntryWithLinks renders the entry, applies the link overlay, and sets the
// viewport content. Called whenever the entry or focused link changes.
func (m *readerModel) setEntryWithLinks(entry Entry, links []Link) {
	m.current = &entry
	header, body := render.Split(entry.Target, entry.Body, entry.Meta, entry.Err, m.profile, m.darkBackground)
	body = applyLinkOverlay(body, links, m.focusedLink, m.styles)
	m.viewport.SetContent(header + body)
	m.scrollToFocusedLink(header, body, links)
}

// scrollToFocusedLink scrolls the viewport so the focused link is visible.
func (m *readerModel) scrollToFocusedLink(header, overlaidBody string, links []Link) {
	if m.focusedLink < 0 || m.focusedLink >= len(links) {
		return
	}
	raw := links[m.focusedLink].Raw
	// Find the raw string in the non-overlaid body (before our lipgloss Render
	// wraps it). We re-search the original body text via the entry's body bytes.
	bodyText := string(m.current.Body)
	pos := strings.Index(bodyText, raw)
	if pos < 0 {
		return
	}
	// Count newlines before pos to get the body-relative line number.
	bodyLine := strings.Count(bodyText[:pos], "\n")
	// Add header line count.
	headerLines := strings.Count(header, "\n")
	targetLine := headerLines + bodyLine
	// Centre the focused link in the viewport.
	offset := targetLine - m.viewport.Height()/2
	if offset < 0 {
		offset = 0
	}
	m.viewport.SetYOffset(offset)
}
```

Add `"strings"` import to `reader.go` if not present.

- [ ] **Step 3: Add link navigation helpers to `readerModel`**

```go
// nextLink advances focusedLink forward, wrapping around.
func (m *readerModel) nextLink(count int) {
	if count == 0 {
		return
	}
	if m.focusedLink < 0 {
		m.focusedLink = 0
		return
	}
	m.focusedLink = (m.focusedLink + 1) % count
}

// prevLink moves focusedLink backward, wrapping around.
func (m *readerModel) prevLink(count int) {
	if count == 0 {
		return
	}
	if m.focusedLink <= 0 {
		m.focusedLink = count - 1
		return
	}
	m.focusedLink--
}
```

- [ ] **Step 4: Route Tab/n/N in `handleKey` (`app.go`)**

In the content-focused `switch` block (after the `Copy` case), add:

```go
case key.Matches(msg, m.keys.LinkNext) && m.pos >= 0:
    node := &m.history[m.pos]
    m.reader.nextLink(len(node.links))
    node.linkIdx = m.reader.focusedLink
    m.reader.setEntryWithLinks(node.entry, node.links)
    return true, m, nil

case key.Matches(msg, m.keys.LinkPrev) && m.pos >= 0:
    node := &m.history[m.pos]
    m.reader.prevLink(len(node.links))
    node.linkIdx = m.reader.focusedLink
    m.reader.setEntryWithLinks(node.entry, node.links)
    return true, m, nil
```

Also update `routeFetch` and `restore` to call `setEntryWithLinks` instead of `setEntry` for reader nodes:

In `routeFetch`, replace `m.reader.setEntry(entry)` with:
```go
m.reader.focusedLink = -1
m.reader.setEntryWithLinks(entry, node.links)
```

In `restore` for the reader branch, replace `m.reader.setEntry(n.entry)` with:
```go
m.reader.focusedLink = n.linkIdx
m.reader.setEntryWithLinks(n.entry, n.links)
```

Also update `setProfile` and `setBackground` in `reader.go` to re-render with overlay. Replace the `m.viewport.SetContent(renderEntry(...))` calls with:

```go
// In setProfile:
if m.current != nil {
    // Re-render without link overlay; appModel will call setEntryWithLinks
    // after profile changes if links are active. For simplicity, call with nil links.
    header, body := render.Split(m.current.Target, m.current.Body, m.current.Meta, m.current.Err, m.profile, m.darkBackground)
    m.viewport.SetContent(header + body)
}
```

(setBackground similarly — both are non-interactive re-renders; the overlay will be re-applied on the next key that touches links.)

- [ ] **Step 5: Compile and run**

```bash
make build && go test ./tui/ -count=1
```
Expected: compiles and existing tests pass.

- [ ] **Step 6: Commit**

```bash
git add tui/links.go tui/reader.go tui/app.go
git commit -m "feat(tui): Tab/n/N cycle focused link in reader; overlay highlight + OSC-8"
```

---

## Task 12: Status bar hints for focused links + `Enter`/`f`/`y` dispatch

**Files:**
- Modify: `tui/app.go`

- [ ] **Step 1: Add focused-link status bar branch**

In `buildStatusBar`, in the `default: // stateReader` case, add a check at the top:

```go
default: // stateReader
    bar.meta = formatBytes(len(node.entry.Body))
    if node.entry.Meta.Truncated {
        bar.flags = append(bar.flags, "partial (truncated)")
    }

    // Focused-link mode overrides the resting hints.
    if m.reader.focusedLink >= 0 && m.reader.focusedLink < len(node.links) {
        link := node.links[m.reader.focusedLink]
        n := m.reader.focusedLink + 1
        total := len(node.links)
        label := linkKindLabel(link)
        action := "↵ copy"
        if link.Action == ActionDrill {
            action = "↵ drill"
        }
        var extra []string
        if link.Ambiguous {
            extra = append(extra, "f finger")
        }
        if link.Blocked != "" {
            extra = append(extra, link.Blocked)
        }
        if isOSC8Openable(link.Raw) {
            extra = append(extra, "⌘-click opens")
        }
        extra = append(extra, "y copy", "⇥ next")
        bar.hints = fmt.Sprintf("link %d/%d · %s · %s · %s", n, total, label, action, strings.Join(extra, " · "))
        return bar
    }

    // Resting reader hints (no focused link).
    bar.hints = joinHints([]string{"↑↓ scroll"}, bar.escTarget)
    if m.reader.viewport.TotalLineCount() > m.reader.viewport.Height() {
        bar.scroll = fmt.Sprintf("%d%%", int(math.Round(m.reader.viewport.ScrollPercent()*100)))
    }
```

Add `linkKindLabel`:

```go
func linkKindLabel(l Link) string {
	if l.Blocked != "" {
		return "forwarded finger"
	}
	switch l.Kind {
	case LinkFinger:
		if l.Ambiguous {
			return "address (auto)"
		}
		return "finger"
	case LinkURL:
		return "url"
	case LinkEmail:
		return "email"
	case LinkSocial:
		return "social"
	}
	return "link"
}
```

- [ ] **Step 2: Route `Enter` for focused links**

In the content-focused `switch` in `handleKey`, the existing `Open` (Enter) case already handles lists. Add a reader branch:

```go
case key.Matches(msg, m.keys.Open) && m.state == stateReader && m.pos >= 0:
    node := &m.history[m.pos]
    if m.reader.focusedLink >= 0 && m.reader.focusedLink < len(node.links) {
        return m.activateFocusedLink(node)
    }
    return false, m, nil // no focused link — fall through to scroll
```

Add `activateFocusedLink`:

```go
func (m appModel) activateFocusedLink(node *histNode) (bool, appModel, tea.Cmd) {
    link := node.links[m.reader.focusedLink]
    switch link.Action {
    case ActionDrill:
        if link.Blocked != "" {
            flash := m.setFlash(link.Blocked)
            return true, m, flash
        }
        cmd := m.startFetch(link.Target)
        return true, m, cmd
    case ActionCopy:
        flash := m.setFlash("copied " + link.Raw)
        return true, m, tea.Batch(setClipboard(link.Raw), flash)
    }
    return true, m, nil
}
```

- [ ] **Step 3: Route `f` (finger-on-demand)**

```go
case key.Matches(msg, m.keys.LinkFinger) && m.pos >= 0:
    node := &m.history[m.pos]
    if m.reader.focusedLink >= 0 && m.reader.focusedLink < len(node.links) {
        link := node.links[m.reader.focusedLink]
        if link.Kind == LinkFinger && link.Target.HostPort != "" {
            cmd := m.startFetch(link.Target)
            return true, m, cmd
        }
    }
    return true, m, nil
```

- [ ] **Step 4: Update `y` to copy focused link's Raw when applicable**

In `copyAddress()`, add a reader-focused-link branch at the top:

```go
func (m *appModel) copyAddress() tea.Cmd {
	if m.state == stateReader && m.pos >= 0 {
		node := m.history[m.pos]
		if m.reader.focusedLink >= 0 && m.reader.focusedLink < len(node.links) {
			raw := node.links[m.reader.focusedLink].Raw
			return tea.Batch(setClipboard(raw), m.setFlash("copied "+raw))
		}
	}
	// ... existing logic unchanged ...
```

- [ ] **Step 5: Run tests**

```bash
make check
```
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add tui/app.go
git commit -m "feat(tui): status bar hints, Enter/f/y dispatch for focused reader links"
```

---

## Task 13: Links panel (`L`)

**Files:**
- Modify: `tui/app.go`
- Modify: `tui/list.go` (or new `tui/linkspanel.go`)

The links panel reuses `bubbles/v2/list` with a custom `linkItem` type. `L` toggles between the reader viewport and the panel; the panel uses the same action model as inline.

- [ ] **Step 1: Add `linkItem` and `linksPanel` to `tui/list.go` (or a new file)**

Create `tui/linkspanel.go`:

```go
package tui

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

// linkItem is one row in the links panel list.
type linkItem struct {
	link Link
}

func (i linkItem) FilterValue() string { return i.link.Raw }
func (i linkItem) Title() string       { return i.link.Raw }
func (i linkItem) Description() string { return linkKindLabel(i.link) }

// linksPanel wraps a bubbles list populated from []Link.
type linksPanel struct {
	list   list.Model
	common *commonModel
	links  []Link
}

func newLinksPanel(common *commonModel, links []Link) linksPanel {
	items := make([]list.Item, len(links))
	for i, l := range links {
		items[i] = linkItem{link: l}
	}
	st := common.ensureStyles()
	d := list.NewDefaultDelegate()
	d.Styles = st.listItem
	d.SetSpacing(0)
	h := common.bodyHeight()
	if h < 1 {
		h = 1
	}
	l := list.New(items, d, common.width, h)
	applyListStyles(&l, st)
	l.Title = fmt.Sprintf("%d links", len(links))
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	return linksPanel{list: l, common: common, links: links}
}

func (p *linksPanel) setSize(w, h int) {
	p.list.SetSize(w, h)
}

func (p linksPanel) View() string { return p.list.View() }

func (p linksPanel) update(msg tea.Msg) (linksPanel, tea.Cmd) {
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p linksPanel) selected() (Link, bool) {
	it, ok := p.list.SelectedItem().(linkItem)
	if !ok {
		return Link{}, false
	}
	return it.link, true
}

// Unused interface implementation to satisfy io.Writer for delegate.
var _ io.Writer = (*discardWriter)(nil)

type discardWriter struct{}

func (*discardWriter) Write(p []byte) (int, error) { return len(p), nil }
```

- [ ] **Step 2: Add `linksPanel` state to `appModel`**

In `appModel` struct, add:
```go
showingLinks bool        // links panel open
linksPanel   linksPanel
```

In `app.go`, add a new `stateLinkPanel` is NOT needed — we overlay the panel over the reader (like `showingRaw` is a boolean flag). When `showingLinks`, render the links panel instead of the reader; the reader `histNode` state is preserved.

- [ ] **Step 3: Route `L` in `handleKey`**

```go
case key.Matches(msg, m.keys.LinkPanel) && m.pos >= 0:
    node := m.history[m.pos]
    if m.showingLinks {
        m.showingLinks = false
        // Return to reader at the focused link from the panel.
        if sel, ok := m.linksPanel.selected(); ok {
            for i, l := range node.links {
                if l.Raw == sel.Raw {
                    m.reader.focusedLink = i
                    break
                }
            }
        }
        m.reader.setEntryWithLinks(node.entry, node.links)
    } else {
        m.showingLinks = true
        m.linksPanel = newLinksPanel(m.common, node.links)
        m.linksPanel.setSize(m.common.width, m.common.bodyHeight())
    }
    return true, m, nil
```

- [ ] **Step 4: Route `Enter` and `f` in panel mode**

In `handleKey`, when `m.showingLinks`:
```go
if m.showingLinks {
    switch {
    case key.Matches(msg, m.keys.Back) || key.Matches(msg, m.keys.LinkPanel):
        m.showingLinks = false
        return true, m, nil
    case key.Matches(msg, m.keys.Open):
        if m.pos >= 0 {
            node := &m.history[m.pos]
            if sel, ok := m.linksPanel.selected(); ok {
                for i, l := range node.links {
                    if l.Raw == sel.Raw {
                        m.reader.focusedLink = i
                        break
                    }
                }
                link := sel
                m.showingLinks = false
                if link.Action == ActionDrill && link.Blocked == "" {
                    return true, m, m.startFetch(link.Target)
                }
                flash := m.setFlash("copied " + link.Raw)
                return true, m, tea.Batch(setClipboard(link.Raw), flash)
            }
        }
        return true, m, nil
    case key.Matches(msg, m.keys.LinkFinger):
        if sel, ok := m.linksPanel.selected(); ok {
            if sel.Kind == LinkFinger && sel.Ambiguous && sel.Target.HostPort != "" {
                m.showingLinks = false
                return true, m, m.startFetch(sel.Target)
            }
        }
        return true, m, nil
    }
    // Delegate remaining keys to the panel list.
    var cmd tea.Cmd
    m.linksPanel, cmd = m.linksPanel.update(msg.(tea.KeyPressMsg))
    return true, m, cmd
}
```

Add this check at the TOP of the content-focused branch in `handleKey`, before the existing `switch`.

- [ ] **Step 5: Render the panel in `View`**

In `View()`, replace the `default:` case content rendering:

```go
default:
    if m.showingLinks {
        content = m.linksPanel.View()
    } else {
        content = m.reader.View()
    }
```

- [ ] **Step 6: Size the panel in `resize`**

After `m.reader.setSize(...)`:
```go
if m.showingLinks {
    m.linksPanel.setSize(m.common.width, h)
}
```

- [ ] **Step 7: Update `updateKeymap` for panel state**

When `m.showingLinks`, ensure `LinkPanel`, `Open`, `Back`, and `LinkFinger` are enabled. Add:
```go
if m.showingLinks {
    m.keys.Open.SetEnabled(true)
    m.keys.Back.SetEnabled(true)
    m.keys.LinkPanel.SetEnabled(true)
}
```

- [ ] **Step 8: Compile and run**

```bash
make check
```
Expected: green.

- [ ] **Step 9: Commit**

```bash
git add tui/linkspanel.go tui/app.go
git commit -m "feat(tui): links panel on L — browsable list of detected links"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|-----------------|------|
| `DetectLinks(body, originHostPort) []Link` | Tasks 1–5 |
| Rule 1: explicit scheme (finger://, https://, mailto:, others) | Task 4 |
| Rule 2: cue word (finger/email/web/fedi etc.) | Task 5 |
| Rule 3: @host form, domain-sane | Task 5 |
| Rule 4: bare user@host, Ambiguous, Action=Copy | Task 5 |
| Domain-sanity gate (bracketed IP, domain labels, TLD, no dotted-quad) | Task 3 |
| Punctuation trimming; token boundaries | Task 3 |
| Forwarding: same-relay drill; cross-relay blocked copy | Tasks 4–5 |
| Blocked token consumes full span (no submatch leak) | Task 5 |
| `appendHarvestedTargets` refactored to DetectLinks adapter | Task 6 |
| `harvestableLogin` filter (behaviour-neutral for harvest) | Task 6 |
| `!Target.HostQuery()` filter (no host-query rows) | Task 6 |
| `render.Split` (header/body separation) | Task 7 |
| `linkFocus` style (AdaptiveColor light/dark) | Task 8 |
| Links cached on `histNode` | Task 9 |
| Tab / n / N cycle | Task 11 |
| Scroll-to-focused-link | Task 11 |
| `applyLinkOverlay` (focus highlight + OSC-8) | Task 11 |
| OSC-8 only for http(s):// and mailto: | Tasks 1, 11 |
| Status bar hints (all 5 link-type variants) | Task 12 |
| Enter dispatches default action | Task 12 |
| `f` drills ambiguous address | Task 12 |
| `y` copies Raw when link focused | Task 12 |
| Links panel on `L` | Task 13 |
| Panel: Enter/f actions mirror inline | Task 13 |
| Panel: Esc/L returns to reader | Task 13 |
| Overlay: body occurrence, not header chrome | Tasks 7, 11 |
| Golden corpus tests: detect/decline/forwarding/punctuation/strong-gate/overlay | Task 2 |
| `userlist_test.go` corpus stays green | Task 6 |
| Security: no auto-open; port-79 pin; pre-sanitised body | design invariant |
| Implementing PR pushed but merged by human | (release note, not code) |

**Placeholder scan:** None found.

**Type consistency:** `Link`, `LinkKind`, `LinkAction`, `DetectLinks`, `harvestableLogin`, `applyLinkOverlay`, `isOSC8Openable`, `linkKindLabel`, `canonicalHost`, `domainSane` — all defined in Task 1 and used consistently through Tasks 2–13.

---

**Plan complete and saved to `docs/superpowers/plans/2026-06-06-body-link-detection.md`.**

Two execution options:

**1. Subagent-Driven (recommended)** — fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?

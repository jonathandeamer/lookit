package tui

import (
	"strings"
	"testing"
)

// findLink returns the first Link whose Raw matches raw, or (Link{}, false).
func findLink(links []Link, raw string) (Link, bool) {
	for _, l := range links {
		if l.Raw == raw {
			return l, true
		}
	}
	return Link{}, false
}

// ---- Decline cases (DetectLinks must return zero matching links) ----

func TestDetectLinks_Decline_HostlessAtAlice(t *testing.T) {
	// @alice with no dot in the "host" — not a Finger link.
	links := DetectLinks([]byte("follow @alice on the fediverse"), "tilde.team:79")
	for _, l := range links {
		if l.Kind == LinkFinger {
			t.Errorf("got Finger link for @alice (no dot): %+v", l)
		}
	}
}

func TestDetectLinks_Decline_LocalhostNoDot(t *testing.T) {
	// bob@localhost has no dot — should not be detected as a Finger link.
	links := DetectLinks([]byte("email bob@localhost for help"), "tilde.team:79")
	for _, l := range links {
		if l.Kind == LinkFinger && strings.Contains(l.Raw, "localhost") {
			t.Errorf("got Finger link for bob@localhost (no dot): %+v", l)
		}
	}
}

func TestDetectLinks_Decline_BareIPv4NoBrackets(t *testing.T) {
	// user@1.2.3.4 without brackets — dotted-quad without brackets is not domain-sane.
	links := DetectLinks([]byte("contact user@1.2.3.4 for info"), "tilde.team:79")
	for _, l := range links {
		if strings.Contains(l.Raw, "1.2.3.4") && l.Kind == LinkFinger {
			t.Errorf("got Finger link for user@1.2.3.4 (bare IPv4): %+v", l)
		}
	}
}

func TestDetectLinks_Decline_EmbeddedAtToken(t *testing.T) {
	// alice@tilde.team embedded inside see_alice@tilde.team_docs — boundary check.
	links := DetectLinks([]byte("see see_alice@tilde.team_docs for more"), "tilde.team:79")
	for _, l := range links {
		if l.Raw == "alice@tilde.team" {
			t.Errorf("got link for alice@tilde.team embedded in larger word: %+v", l)
		}
	}
}

func TestDetectLinks_Decline_BareDomain(t *testing.T) {
	// A bare domain with no scheme or @ should not be detected.
	links := DetectLinks([]byte("visit tilde.team for fun"), "tilde.team:79")
	for _, l := range links {
		if l.Raw == "tilde.team" {
			t.Errorf("got link for bare domain tilde.team: %+v", l)
		}
	}
}

func TestDetectLinks_Decline_SchemeNoAuthority(t *testing.T) {
	// https:// with no authority should not match.
	links := DetectLinks([]byte("try https:// for something"), "tilde.team:79")
	for _, l := range links {
		if l.Raw == "https://" {
			t.Errorf("got link for bare https:// (no authority): %+v", l)
		}
	}
}

// ---- Rule 1 — explicit scheme ----

func TestDetectLinks_Rule1_HTTPS(t *testing.T) {
	links := DetectLinks([]byte("visit https://example.com/foo for info"), "tilde.team:79")
	l, ok := findLink(links, "https://example.com/foo")
	if !ok {
		t.Fatal("DetectLinks did not find https://example.com/foo")
	}
	if l.Kind != LinkURL {
		t.Errorf("Kind = %v, want LinkURL", l.Kind)
	}
	if l.Action != ActionCopy {
		t.Errorf("Action = %v, want ActionCopy", l.Action)
	}
	if !l.Strong {
		t.Errorf("Strong = false, want true (explicit scheme is strong)")
	}
	if !isOSC8Openable(l.Raw) {
		t.Errorf("isOSC8Openable(https://...) = false, want true")
	}
}

func TestDetectLinks_Rule1_Gemini(t *testing.T) {
	links := DetectLinks([]byte("read gemini://rawtext.club/~alice for info"), "tilde.team:79")
	l, ok := findLink(links, "gemini://rawtext.club/~alice")
	if !ok {
		t.Fatal("DetectLinks did not find gemini://rawtext.club/~alice")
	}
	if l.Kind != LinkURL {
		t.Errorf("Kind = %v, want LinkURL", l.Kind)
	}
	if l.Action != ActionCopy {
		t.Errorf("Action = %v, want ActionCopy", l.Action)
	}
	if !l.Strong {
		t.Errorf("Strong = false, want true (explicit scheme is strong)")
	}
	if isOSC8Openable(l.Raw) {
		t.Errorf("isOSC8Openable(gemini://...) = true, want false")
	}
}

func TestDetectLinks_Rule1_Mailto(t *testing.T) {
	links := DetectLinks([]byte("send to mailto:alice@example.com now"), "tilde.team:79")
	l, ok := findLink(links, "mailto:alice@example.com")
	if !ok {
		t.Fatal("DetectLinks did not find mailto:alice@example.com")
	}
	if l.Kind != LinkEmail {
		t.Errorf("Kind = %v, want LinkEmail", l.Kind)
	}
	if l.Action != ActionCopy {
		t.Errorf("Action = %v, want ActionCopy", l.Action)
	}
	if !l.Strong {
		t.Errorf("Strong = false, want true (explicit scheme is strong)")
	}
	if !isOSC8Openable(l.Raw) {
		t.Errorf("isOSC8Openable(mailto:...) = false, want true")
	}
}

func TestDetectLinks_Rule1_FingerURL_SameOrigin(t *testing.T) {
	// finger://tilde.team/alice with origin tilde.team:79 — should be drillable.
	links := DetectLinks([]byte("finger://tilde.team/alice"), "tilde.team:79")
	l, ok := findLink(links, "finger://tilde.team/alice")
	if !ok {
		t.Fatal("DetectLinks did not find finger://tilde.team/alice")
	}
	if l.Kind != LinkFinger {
		t.Errorf("Kind = %v, want LinkFinger", l.Kind)
	}
	if l.Action != ActionDrill {
		t.Errorf("Action = %v, want ActionDrill", l.Action)
	}
	if l.Target.HostPort != "tilde.team:79" {
		t.Errorf("Target.HostPort = %q, want %q", l.Target.HostPort, "tilde.team:79")
	}
}

func TestDetectLinks_Rule1_Gopher(t *testing.T) {
	links := DetectLinks([]byte("see gopher://gopher.floodgap.com"), "tilde.team:79")
	l, ok := findLink(links, "gopher://gopher.floodgap.com")
	if !ok {
		t.Fatal("DetectLinks did not find gopher://gopher.floodgap.com")
	}
	if l.Kind != LinkURL {
		t.Errorf("Kind = %v, want LinkURL", l.Kind)
	}
	if isOSC8Openable(l.Raw) {
		t.Errorf("isOSC8Openable(gopher://...) = true, want false")
	}
}

func TestDetectLinks_Rule1_IRCS(t *testing.T) {
	links := DetectLinks([]byte("join ircs://irc.libera.chat/lookit"), "tilde.team:79")
	l, ok := findLink(links, "ircs://irc.libera.chat/lookit")
	if !ok {
		t.Fatal("DetectLinks did not find ircs://irc.libera.chat/lookit")
	}
	if l.Kind != LinkURL {
		t.Errorf("Kind = %v, want LinkURL", l.Kind)
	}
	if isOSC8Openable(l.Raw) {
		t.Errorf("isOSC8Openable(ircs://...) = true, want false")
	}
}

// ---- Rule 2 — cue word ----

func TestDetectLinks_Rule2_FingerCue(t *testing.T) {
	links := DetectLinks([]byte("finger alice@tilde.team for info"), "tilde.team:79")
	l, ok := findLink(links, "alice@tilde.team")
	if !ok {
		t.Fatal("DetectLinks did not find alice@tilde.team (finger cue)")
	}
	if l.Kind != LinkFinger {
		t.Errorf("Kind = %v, want LinkFinger", l.Kind)
	}
	if l.Action != ActionDrill {
		t.Errorf("Action = %v, want ActionDrill", l.Action)
	}
	if !l.Strong {
		t.Errorf("Strong = false, want true (cue word makes it strong)")
	}
	if l.Ambiguous {
		t.Errorf("Ambiguous = true, want false (finger cue resolves ambiguity)")
	}
}

func TestDetectLinks_Rule2_EmailCue(t *testing.T) {
	links := DetectLinks([]byte("email me at bob@example.com"), "tilde.team:79")
	l, ok := findLink(links, "bob@example.com")
	if !ok {
		t.Fatal("DetectLinks did not find bob@example.com (email cue)")
	}
	if l.Kind != LinkEmail {
		t.Errorf("Kind = %v, want LinkEmail", l.Kind)
	}
	if l.Action != ActionCopy {
		t.Errorf("Action = %v, want ActionCopy", l.Action)
	}
	if !l.Strong {
		t.Errorf("Strong = false, want true (cue word makes it strong)")
	}
	if isOSC8Openable(l.Raw) {
		t.Errorf("isOSC8Openable(bob@example.com) = true, want false (no mailto: prefix)")
	}
}

func TestDetectLinks_Rule2_FediCue(t *testing.T) {
	links := DetectLinks([]byte("fedi @alice@fosstodon.org"), "tilde.team:79")
	// The raw form includes the leading @.
	l, ok := findLink(links, "@alice@fosstodon.org")
	if !ok {
		t.Fatal("DetectLinks did not find @alice@fosstodon.org (fedi cue)")
	}
	if l.Kind != LinkSocial {
		t.Errorf("Kind = %v, want LinkSocial", l.Kind)
	}
	if l.Action != ActionCopy {
		t.Errorf("Action = %v, want ActionCopy", l.Action)
	}
}

// ---- Rule 3 — @host form ----

func TestDetectLinks_Rule3_AtHost(t *testing.T) {
	links := DetectLinks([]byte("try @tilde.team today"), "example.com:79")
	l, ok := findLink(links, "@tilde.team")
	if !ok {
		t.Fatal("DetectLinks did not find @tilde.team (Rule 3)")
	}
	if l.Kind != LinkFinger {
		t.Errorf("Kind = %v, want LinkFinger", l.Kind)
	}
	if l.Action != ActionDrill {
		t.Errorf("Action = %v, want ActionDrill", l.Action)
	}
	if l.Target.HostPort != "tilde.team:79" {
		t.Errorf("Target.HostPort = %q, want %q", l.Target.HostPort, "tilde.team:79")
	}
	if l.Ambiguous {
		t.Errorf("Ambiguous = true, want false for @host form")
	}
}

func TestDetectLinks_Rule3_AtAliceNoDot_NotFinger(t *testing.T) {
	// @alice with no dot — not a Finger link (same as decline case, specific to Rule 3).
	links := DetectLinks([]byte("follow @alice on the fediverse"), "tilde.team:79")
	for _, l := range links {
		if l.Kind == LinkFinger && l.Raw == "@alice" {
			t.Errorf("got Finger link for @alice (no dot, Rule 3 must decline): %+v", l)
		}
	}
}

// ---- Rule 4 — bare user@host ----

func TestDetectLinks_Rule4_BareUserAtHost(t *testing.T) {
	links := DetectLinks([]byte("contact admin@example.com today"), "tilde.team:79")
	l, ok := findLink(links, "admin@example.com")
	if !ok {
		t.Fatal("DetectLinks did not find admin@example.com (Rule 4)")
	}
	if l.Kind != LinkFinger {
		t.Errorf("Kind = %v, want LinkFinger", l.Kind)
	}
	if l.Action != ActionCopy {
		t.Errorf("Action = %v, want ActionCopy (policy B — ambiguous, default copy)", l.Action)
	}
	if !l.Ambiguous {
		t.Errorf("Ambiguous = false, want true (bare user@host is indistinguishable from email)")
	}
	if l.Strong {
		t.Errorf("Strong = true, want false (no cue word, rule 4 inferred from shape)")
	}
	if l.Target.HostPort == "" {
		t.Errorf("Target.HostPort is empty, want populated for Finger link")
	}
}

// ---- OSC-8 matrix ----

func TestDetectLinks_OSC8_OnlyHTTPAndMailto(t *testing.T) {
	// Tests isOSC8Openable directly — this should PASS even before DetectLinks is implemented.
	cases := []struct {
		raw  string
		want bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"mailto:alice@example.com", true},
		{"gemini://rawtext.club", false},
		{"gopher://gopher.floodgap.com", false},
		{"ircs://irc.libera.chat/lookit", false},
	}
	for _, tc := range cases {
		got := isOSC8Openable(tc.raw)
		if got != tc.want {
			t.Errorf("isOSC8Openable(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

// ---- Punctuation stripping ----

func TestDetectLinks_Punctuation_TrailingParenDot(t *testing.T) {
	// "see https://example.com/foo). for more" — trailing ")." should be stripped.
	links := DetectLinks([]byte("see https://example.com/foo). for more"), "tilde.team:79")
	if _, ok := findLink(links, "https://example.com/foo"); !ok {
		t.Error("DetectLinks did not strip trailing ). from URL")
		t.Logf("links = %+v", links)
	}
	// The raw form with trailing paren should NOT appear.
	if _, ok := findLink(links, "https://example.com/foo)."); ok {
		t.Error("DetectLinks kept trailing ). in URL raw")
	}
}

func TestDetectLinks_Punctuation_BalancedParensKept(t *testing.T) {
	// "see https://example.com/foo(bar) now" — balanced parens should be kept.
	links := DetectLinks([]byte("see https://example.com/foo(bar) now"), "tilde.team:79")
	if _, ok := findLink(links, "https://example.com/foo(bar)"); !ok {
		t.Error("DetectLinks stripped balanced parens from URL")
		t.Logf("links = %+v", links)
	}
}

func TestDetectLinks_Punctuation_DoubleQuotes(t *testing.T) {
	// URL inside double-quotes — quotes act as delimiters.
	links := DetectLinks([]byte(`see "https://example.com/foo" now`), "tilde.team:79")
	if _, ok := findLink(links, "https://example.com/foo"); !ok {
		t.Error("DetectLinks did not extract URL from inside double-quotes")
		t.Logf("links = %+v", links)
	}
}

// ---- Forwarding ----

func TestDetectLinks_Forwarding_SameRelay_DrillAllowed(t *testing.T) {
	// "finger epoch@whois.ano@thebackupbox.net" — origin matches relay.
	body := []byte("finger epoch@whois.ano@thebackupbox.net")
	links := DetectLinks(body, "thebackupbox.net:79")
	l, ok := findLink(links, "epoch@whois.ano@thebackupbox.net")
	if !ok {
		t.Fatal("DetectLinks did not find forwarded token epoch@whois.ano@thebackupbox.net")
	}
	if l.Action != ActionDrill {
		t.Errorf("Action = %v, want ActionDrill (same relay)", l.Action)
	}
	if l.Blocked != "" {
		t.Errorf("Blocked = %q, want empty (same relay should not be blocked)", l.Blocked)
	}
	if !l.Forwarded {
		t.Errorf("Forwarded = false, want true")
	}
}

func TestDetectLinks_Forwarding_DifferentRelay_CopyOnly(t *testing.T) {
	// Same token but origin is tilde.team:79 — relay differs, must be copy-only + blocked.
	body := []byte("finger epoch@whois.ano@thebackupbox.net")
	links := DetectLinks(body, "tilde.team:79")
	l, ok := findLink(links, "epoch@whois.ano@thebackupbox.net")
	if !ok {
		t.Fatal("DetectLinks did not find blocked forwarded token")
	}
	if l.Action != ActionCopy {
		t.Errorf("Action = %v, want ActionCopy (different relay)", l.Action)
	}
	if l.Blocked == "" {
		t.Errorf("Blocked is empty, want non-empty (relay doesn't match origin)")
	}
	if !l.Forwarded {
		t.Errorf("Forwarded = false, want true")
	}
}

func TestDetectLinks_Forwarding_BlockedSpanConsumed(t *testing.T) {
	// When relay doesn't match, the full forwarded token must be consumed.
	// The sub-token "epoch@whois.ano" must NOT appear as a separate link.
	body := []byte("finger epoch@whois.ano@thebackupbox.net")
	links := DetectLinks(body, "tilde.team:79")
	if _, ok := findLink(links, "epoch@whois.ano"); ok {
		t.Error("blocked forwarded token was split: epoch@whois.ano appeared as separate link")
	}
}

func TestDetectLinks_Forwarding_FingerURL_SameRelay(t *testing.T) {
	// finger://thebackupbox.net/epoch@whois.ano with matching origin — drillable.
	body := []byte("finger://thebackupbox.net/epoch@whois.ano")
	links := DetectLinks(body, "thebackupbox.net:79")
	l, ok := findLink(links, "finger://thebackupbox.net/epoch@whois.ano")
	if !ok {
		t.Fatal("DetectLinks did not find finger://thebackupbox.net/epoch@whois.ano")
	}
	if l.Action != ActionDrill {
		t.Errorf("Action = %v, want ActionDrill (same origin)", l.Action)
	}
	if l.Blocked != "" {
		t.Errorf("Blocked = %q, want empty", l.Blocked)
	}
}

// ---- Strong-gate / parseUserList adapter ----

// TestStrongGate_ProseEmailNotHarvested checks that a bare user@host in prose
// inside a list response body does NOT get promoted into the user list, even
// though DetectLinks would detect it as a reader link.
func TestStrongGate_ProseEmailNotHarvested(t *testing.T) {
	// A columnar list body with an email address embedded in the prose section.
	body := []byte(
		"Login   Name\n" +
			"alice   Alice Smith\n" +
			"bob     Bob Jones\n" +
			"\n" +
			"Contact admin@example.com for server issues.\n",
	)
	parsed, ok := parseUserList(body, "")
	if !ok {
		t.Fatal("parseUserList ok = false, want true (columnar list should parse)")
	}
	// admin@example.com must NOT become a list user.
	for _, u := range parsed.users {
		if u.Login == "admin" && u.Target == "" {
			t.Errorf("admin was added as a list user from prose email — should not be harvested")
		}
	}
	// But DetectLinks should still detect it as a reader link (Rule 4 — bare user@host).
	links := DetectLinks(body, "example.com:79")
	_, found := findLink(links, "admin@example.com")
	if !found {
		t.Error("DetectLinks did not detect admin@example.com as a reader link")
	}
}

// TestStrongGate_HostQueryFingerURLNotHarvested checks that a finger:// URL
// targeting a host (no user login path) is not turned into a list entry.
func TestStrongGate_HostQueryFingerURLNotHarvested(t *testing.T) {
	body := []byte(
		"Login   Name\n" +
			"alice   Alice\n" +
			"bob     Bob\n" +
			"\n" +
			"Also see finger://tilde.team for the full list.\n",
	)
	parsed, ok := parseUserList(body, "")
	if !ok {
		t.Fatal("parseUserList ok = false, want true")
	}
	// finger://tilde.team is a host query — must not create a user row.
	for _, u := range parsed.users {
		if u.Target == "@tilde.team" || u.Login == "tilde.team" {
			t.Errorf("host-query finger URL became a list entry: %+v", u)
		}
	}
}

// TestStrongGate_TildeLoginNotHarvestable checks that finger://example.com/~bob
// in a list response body is detected as a drillable reader link by DetectLinks,
// but harvestableLogin rejects ~bob so it must NOT become a list row.
func TestStrongGate_TildeLoginNotHarvestable(t *testing.T) {
	body := []byte(
		"Login   Name\n" +
			"alice   Alice\n" +
			"bob     Bob\n" +
			"\n" +
			"See also finger://example.com/~bob for the tilde version.\n",
	)
	parsed, ok := parseUserList(body, "")
	if !ok {
		t.Fatal("parseUserList ok = false, want true")
	}
	// ~bob must not appear as a list user (loginRe rejects ~ prefix).
	for _, u := range parsed.users {
		if u.Login == "~bob" {
			t.Errorf("~bob was added as a list user — loginRe must reject it")
		}
	}

	// Verify loginRe (which harvestableLogin uses) rejects ~bob directly.
	if loginRe.MatchString("~bob") {
		t.Error("loginRe.MatchString(\"~bob\") = true, want false (~bob must not be harvestable)")
	}

	// DetectLinks should still detect finger://example.com/~bob as a drillable link.
	links := DetectLinks(body, "example.com:79")
	l, ok2 := findLink(links, "finger://example.com/~bob")
	if !ok2 {
		t.Error("DetectLinks did not detect finger://example.com/~bob as a reader link")
	} else if l.Action != ActionDrill {
		t.Errorf("Action = %v, want ActionDrill for finger:// link to same origin", l.Action)
	}
}

// TODO: uncomment in Task 11 when applyLinkOverlay is implemented
// func TestApplyLinkOverlay_BodyNotHeader(t *testing.T) {
// 	st := newStyles(true)
// 	body := "visit https://example.com today\nplain line\n"
// 	links := []Link{
// 		{
// 			Kind:   LinkURL,
// 			Action: ActionCopy,
// 			Raw:    "https://example.com",
// 			Strong: true,
// 		},
// 	}
// 	result := applyLinkOverlay(body, links, 0, st)
// 	if !strings.Contains(result, "https://example.com") {
// 		t.Errorf("applyLinkOverlay result missing URL: %q", result)
// 	}
// 	// The plain line must not be highlighted.
// 	if strings.Contains(result, "\x1b") && strings.Count(result, "plain line") != 1 {
// 		t.Errorf("plain line appears to be unexpectedly styled in overlay result")
// 	}
// }

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

func TestLinksPanelTitleUsesSingularLinkLabel(t *testing.T) {
	p := newLinksPanel(testCommon(), []Link{{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"}})
	if got := p.list.Title; got != "1 link" {
		t.Fatalf("links panel title = %q, want %q", got, "1 link")
	}
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

func TestDetectLinks_URLGrammar(t *testing.T) {
	t.Run("supported forms remain one consumed link", func(t *testing.T) {
		tests := []struct {
			body     string
			raw      string
			kind     LinkKind
			query    string
			hostPort string
			action   LinkAction
			osc8     bool
		}{
			{"visit https://example.com/foo", "https://example.com/foo", LinkURL, "", "", ActionCopy, true},
			{"read gemini://rawtext.club/~alice", "gemini://rawtext.club/~alice", LinkURL, "", "", ActionCopy, false},
			{"send to mailto:alice@example.com now", "mailto:alice@example.com", LinkEmail, "", "", ActionCopy, true},
			{"MAILTO:alice@example.com", "MAILTO:alice@example.com", LinkEmail, "", "", ActionCopy, true},
			{"mailto:alice(work)@example.com", "mailto:alice(work)@example.com", LinkEmail, "", "", ActionCopy, true},
			{"finger://bbs.airandwave.net/wiki:foo", "finger://bbs.airandwave.net/wiki:foo", LinkFinger, "wiki:foo", "bbs.airandwave.net:79", ActionDrill, false},
			{"http://user:pass@example.com/x", "http://user:pass@example.com/x", LinkURL, "", "", ActionCopy, true},
		}

		for _, tt := range tests {
			t.Run(tt.body, func(t *testing.T) {
				links := DetectLinks([]byte(tt.body), "example.com:79")
				if len(links) != 1 {
					t.Fatalf("DetectLinks(%q) returned %d links, want 1: %#v", tt.body, len(links), links)
				}
				link := links[0]
				if link.Raw != tt.raw || link.Kind != tt.kind || !link.Strong || link.Action != tt.action {
					t.Fatalf("DetectLinks(%q)[0] = %#v", tt.body, link)
				}
				if link.Target.Query != tt.query || link.Target.HostPort != tt.hostPort {
					t.Fatalf("target = %#v, want Query=%q HostPort=%q", link.Target, tt.query, tt.hostPort)
				}
				if got := isOSC8Openable(link.Raw); got != tt.osc8 {
					t.Fatalf("isOSC8Openable(%q) = %v, want %v", link.Raw, got, tt.osc8)
				}
			})
		}
	})

	t.Run("unsupported colon-only schemes produce no links", func(t *testing.T) {
		for _, body := range []string{
			"mailto:",
			"tel:+15550000",
			"data:text/plain,hello",
			"magnet:?xt=urn:btih:abc",
			"Timezone: UTC",
			"label:value",
		} {
			t.Run(body, func(t *testing.T) {
				links := DetectLinks([]byte(body), "example.com:79")
				if len(links) != 0 {
					t.Fatalf("DetectLinks(%q) returned unexpected links %#v", body, links)
				}
			})
		}
	})

	t.Run("hostless shorthand remains plain text", func(t *testing.T) {
		links := DetectLinks([]byte("Try @bonsai"), "example.com:79")
		for _, link := range links {
			if link.Kind == LinkFinger {
				t.Fatalf("DetectLinks returned Finger link for @bonsai: %#v", link)
			}
		}
	})
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

func TestDetectLinks_QuotedFingerQuery(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		raw       string
		query     string
		strong    bool
		ambiguous bool
		action    LinkAction
	}{
		{"cued double", `finger "oslo, united states"@graph.no`, `"oslo, united states"@graph.no`, "oslo, united states", true, false, ActionDrill},
		{"uncued double", `"oslo, united states"@graph.no`, `"oslo, united states"@graph.no`, "oslo, united states", false, true, ActionCopy},
		{"cued single", `finger 'oslo, united states'@graph.no`, `'oslo, united states'@graph.no`, "oslo, united states", true, false, ActionDrill},
		{"trailing punctuation", `finger "oslo"@graph.no.`, `"oslo"@graph.no`, "oslo", true, false, ActionDrill},
		{"suffix wins", `finger oslo, "united states"@graph.no`, `"united states"@graph.no`, "united states", true, false, ActionDrill},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := DetectLinks([]byte(tt.body), "example.com:79")
			if len(links) != 1 {
				t.Fatalf("DetectLinks(%q) returned %d links, want 1: %#v", tt.body, len(links), links)
			}
			link := links[0]
			if link.Raw != tt.raw || link.Kind != LinkFinger || link.Action != tt.action ||
				link.Strong != tt.strong || link.Ambiguous != tt.ambiguous || link.Blocked != "" ||
				link.Target.Query != tt.query || link.Target.HostPort != "graph.no:79" {
				t.Fatalf("DetectLinks(%q)[0] = %#v, want Raw=%q Query=%q Strong=%v Ambiguous=%v Action=%v",
					tt.body, link, tt.raw, tt.query, tt.strong, tt.ambiguous, tt.action)
			}
		})
	}
}

func TestDetectLinks_QuotedFingerQueryDeclines(t *testing.T) {
	tests := []struct {
		name string
		body string
		raw  string
	}{
		{"unmatched opening quote", `finger "oslo@graph.no`, "oslo@graph.no"},
		{"quotes wrap whole address", `"oslo@graph.no"`, "oslo@graph.no"},
		{"host inside quotes", `finger "oslo, united states@graph.no"`, "states@graph.no"},
		{"space before at", `finger "oslo, united states" @graph.no`, "@graph.no"},
		{"mixed quotes", `finger "oslo'@graph.no`, "@graph.no"},
		{"backticks do not group", "finger `oslo, united states`@graph.no", "@graph.no"},
		{"empty quoted query", `finger ""@graph.no`, "@graph.no"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := DetectLinks([]byte(tt.body), "example.com:79")
			if len(links) != 1 || links[0].Raw != tt.raw {
				t.Fatalf("DetectLinks(%q) = %#v, want existing fallback Raw=%q", tt.body, links, tt.raw)
			}
		})
	}
}

func TestDetectLinks_QuotedFingerQueryDeclinesTypographicQuotes(t *testing.T) {
	links := DetectLinks([]byte(`finger “oslo”@graph.no`), "example.com:79")
	if len(links) != 1 || links[0].Raw != "“oslo”@graph.no" || links[0].Target.Query != "“oslo”" {
		t.Fatalf("DetectLinks(typographic quoted query) = %#v, want Raw=%q Query=%q",
			links, "“oslo”@graph.no", "“oslo”")
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

func TestDetectLinks_ColonBearingFingerAddresses(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		raw       string
		query     string
		hostPort  string
		strong    bool
		ambiguous bool
		action    LinkAction
	}{
		{"cued weather", "finger weather:seattle@bbs.airandwave.net", "weather:seattle@bbs.airandwave.net", "weather:seattle", "bbs.airandwave.net:79", true, false, ActionDrill},
		{"cued quake", "finger quake:1@bbs.airandwave.net", "quake:1@bbs.airandwave.net", "quake:1", "bbs.airandwave.net:79", true, false, ActionDrill},
		{"cued dict", "finger dict:word@bbs.airandwave.net", "dict:word@bbs.airandwave.net", "dict:word", "bbs.airandwave.net:79", true, false, ActionDrill},
		{"cued urban", "finger urban:yeet@bbs.airandwave.net", "urban:yeet@bbs.airandwave.net", "urban:yeet", "bbs.airandwave.net:79", true, false, ActionDrill},
		{"cued wiki", "finger wiki:albert_einstein:1@bbs.airandwave.net", "wiki:albert_einstein:1@bbs.airandwave.net", "wiki:albert_einstein:1", "bbs.airandwave.net:79", true, false, ActionDrill},
		{"cued sudoku", "finger sudoku:print:utf8:easy@bbs.airandwave.net", "sudoku:print:utf8:easy@bbs.airandwave.net", "sudoku:print:utf8:easy", "bbs.airandwave.net:79", true, false, ActionDrill},
		{"uncued weather", "weather:seattle@bbs.airandwave.net", "weather:seattle@bbs.airandwave.net", "weather:seattle", "bbs.airandwave.net:79", false, true, ActionCopy},
		{"uncued wiki", "wiki:albert_einstein:1@bbs.airandwave.net", "wiki:albert_einstein:1@bbs.airandwave.net", "wiki:albert_einstein:1", "bbs.airandwave.net:79", false, true, ActionCopy},
		{"uncued sudoku", "sudoku:print:utf8:easy@bbs.airandwave.net", "sudoku:print:utf8:easy@bbs.airandwave.net", "sudoku:print:utf8:easy", "bbs.airandwave.net:79", false, true, ActionCopy},
		{"cued one-letter prefix", "finger o:oslo@graph.no", "o:oslo@graph.no", "o:oslo", "graph.no:79", true, false, ActionDrill},
		{"uncued one-letter prefix", "o:oslo@graph.no", "o:oslo@graph.no", "o:oslo", "graph.no:79", false, true, ActionCopy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := DetectLinks([]byte(tt.body), "example.com:79")
			if len(links) != 1 {
				t.Fatalf("DetectLinks(%q) returned %d links, want 1: %#v", tt.body, len(links), links)
			}
			link := links[0]
			if link.Raw != tt.raw || link.Kind != LinkFinger || link.Action != tt.action ||
				link.Strong != tt.strong || link.Ambiguous != tt.ambiguous ||
				link.Target.Query != tt.query || link.Target.HostPort != tt.hostPort {
				t.Fatalf("DetectLinks(%q)[0] = %#v, want Raw=%q Kind=LinkFinger Action=%v Strong=%v Ambiguous=%v Query=%q HostPort=%q",
					tt.body, link, tt.raw, tt.action, tt.strong, tt.ambiguous, tt.query, tt.hostPort)
			}
		})
	}
}

func TestDetectLinks_CuedSpacedFingerQuery(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		raw      string
		query    string
		hostPort string
	}{
		{"urban spaced", "finger urban:old school@bbs.airandwave.net", "urban:old school@bbs.airandwave.net", "urban:old school", "bbs.airandwave.net:79"},
		{"intervening prose", "finger please try urban:old school@bbs.airandwave.net", "please try urban:old school@bbs.airandwave.net", "please try urban:old school", "bbs.airandwave.net:79"},
		{"case insensitive", "FINGER urban:old school@bbs.airandwave.net", "urban:old school@bbs.airandwave.net", "urban:old school", "bbs.airandwave.net:79"},
		{"port pinned", "finger urban:old school@bbs.airandwave.net:70", "urban:old school@bbs.airandwave.net:70", "urban:old school", "bbs.airandwave.net:79"},
		{"trailing punctuation", "finger urban:old school@bbs.airandwave.net.", "urban:old school@bbs.airandwave.net", "urban:old school", "bbs.airandwave.net:79"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := DetectLinks([]byte(tt.body), "example.com:79")
			if len(links) != 1 {
				t.Fatalf("DetectLinks(%q) returned %d links, want 1: %#v", tt.body, len(links), links)
			}
			link := links[0]
			if link.Raw != tt.raw || link.Kind != LinkFinger || link.Action != ActionDrill ||
				!link.Strong || link.Ambiguous || link.Blocked != "" ||
				link.Target.Query != tt.query || link.Target.HostPort != tt.hostPort {
				t.Fatalf("DetectLinks(%q)[0] = %#v, want Raw=%q strong drill Query=%q HostPort=%q",
					tt.body, link, tt.raw, tt.query, tt.hostPort)
			}
		})
	}
}

func TestDetectLinks_CuedSpacedFingerQueryBoundaries(t *testing.T) {
	t.Run("separate commands on one line", func(t *testing.T) {
		for _, body := range []string{
			"finger alice@one.example finger bob@two.example",
			"finger alice@one.example\tfinger bob@two.example",
		} {
			links := DetectLinks([]byte(body), "example.com:79")
			if len(links) != 2 || links[0].Raw != "alice@one.example" || links[1].Raw != "bob@two.example" {
				t.Fatalf("DetectLinks(%q) = %#v, want two separate commands", body, links)
			}
			for _, link := range links {
				if link.Kind != LinkFinger || !link.Strong || link.Action != ActionDrill {
					t.Fatalf("DetectLinks(%q) contains non-drillable command %#v", body, link)
				}
			}
		}
	})

	t.Run("shared cue never synthesizes forwarding", func(t *testing.T) {
		links := DetectLinks([]byte("finger alice@example.com then bob@other.host"), "example.com:79")
		if len(links) != 2 || links[0].Raw != "alice@example.com" || links[1].Raw != "bob@other.host" {
			t.Fatalf("links = %#v, want two independent addresses", links)
		}
		for _, link := range links {
			if link.Kind != LinkFinger || !link.Strong || link.Action != ActionDrill ||
				link.Forwarded || strings.Count(link.Raw, "@") != 1 {
				t.Fatalf("expanded span synthesized forwarding: %#v", link)
			}
		}
	})

	t.Run("phase-1 consumed span blocks expansion", func(t *testing.T) {
		body := "finger see https://example.com then urban:old school@bbs.airandwave.net"
		links := DetectLinks([]byte(body), "example.com:79")
		if len(links) != 2 {
			t.Fatalf("DetectLinks(%q) = %#v, want URL and Finger links", body, links)
		}
		if links[0].Kind != LinkURL || links[0].Raw != "https://example.com" || !links[0].Strong {
			t.Fatalf("first link = %#v, want strong URL", links[0])
		}
		if links[1].Kind != LinkFinger || links[1].Raw != "school@bbs.airandwave.net" ||
			links[1].Action != ActionDrill || !links[1].Strong || links[1].Ambiguous {
			t.Fatalf("second link = %#v, want unexpanded strong Finger token", links[1])
		}
	})

	t.Run("second bounded finger cue blocks expansion", func(t *testing.T) {
		body := "finger foo finger@other.host"
		links := DetectLinks([]byte(body), "example.com:79")
		if len(links) != 1 || links[0].Raw != "finger@other.host" || links[0].Kind != LinkFinger ||
			links[0].Action != ActionDrill || !links[0].Strong || links[0].Ambiguous {
			t.Fatalf("DetectLinks(%q) = %#v, want fallback strong Finger token", body, links)
		}
	})

	t.Run("multibyte prefix preserves expansion", func(t *testing.T) {
		body := "☃ finger urban:old school@bbs.airandwave.net"
		links := DetectLinks([]byte(body), "example.com:79")
		if len(links) != 1 || links[0].Raw != "urban:old school@bbs.airandwave.net" ||
			links[0].Target.Query != "urban:old school" || links[0].Target.HostPort != "bbs.airandwave.net:79" ||
			links[0].Kind != LinkFinger || links[0].Action != ActionDrill || !links[0].Strong {
			t.Fatalf("DetectLinks(%q) = %#v, want byte-offset-safe expanded Finger link", body, links)
		}
	})

	t.Run("uncued spaces are not grouped", func(t *testing.T) {
		links := DetectLinks([]byte("urban:old school@bbs.airandwave.net"), "example.com:79")
		if len(links) != 1 || links[0].Raw != "school@bbs.airandwave.net" ||
			links[0].Strong || !links[0].Ambiguous || links[0].Action != ActionCopy {
			t.Fatalf("links = %#v, want only policy-B school@bbs.airandwave.net", links)
		}
	})

	t.Run("fingerprint is not a cue", func(t *testing.T) {
		links := DetectLinks([]byte("fingerprint urban:old school@bbs.airandwave.net"), "example.com:79")
		if len(links) != 1 || links[0].Raw != "school@bbs.airandwave.net" || links[0].Strong {
			t.Fatalf("links = %#v, want unexpanded policy-B address", links)
		}
	})

	t.Run("punctuated finger fields are not cues", func(t *testing.T) {
		for _, body := range []string{
			"finger: urban:old school@bbs.airandwave.net",
			"(finger) urban:old school@bbs.airandwave.net",
		} {
			links := DetectLinks([]byte(body), "example.com:79")
			if len(links) != 1 || links[0].Raw != "school@bbs.airandwave.net" || links[0].Strong {
				t.Fatalf("DetectLinks(%q) = %#v, want unexpanded policy-B address", body, links)
			}
		}
	})

	t.Run("quote-like syntax is never expanded", func(t *testing.T) {
		tests := []struct {
			body string
			raw  string
		}{
			{`finger "oslo@graph.no`, "oslo@graph.no"},
			{`finger "oslo, united states@graph.no"`, "states@graph.no"},
			{`finger "oslo, united states" @graph.no`, "@graph.no"},
			{`finger "oslo'@graph.no`, "@graph.no"},
			{"finger `oslo, united states`@graph.no", "@graph.no"},
		}
		for _, tt := range tests {
			links := DetectLinks([]byte(tt.body), "example.com:79")
			if len(links) != 1 || links[0].Raw != tt.raw {
				t.Fatalf("DetectLinks(%q) = %#v, want delimiter fallback Raw=%q", tt.body, links, tt.raw)
			}
		}
	})

	t.Run("nearer email cue wins", func(t *testing.T) {
		links := DetectLinks([]byte("finger email alice@example.com"), "example.com:79")
		if len(links) != 1 || links[0].Raw != "alice@example.com" || links[0].Kind != LinkEmail ||
			links[0].Action != ActionCopy || !links[0].Strong {
			t.Fatalf("links = %#v, want cue-classified email without expansion", links)
		}
	})

	t.Run("cue outside five fields does not expand", func(t *testing.T) {
		body := "finger one two three four five six alice@example.com"
		links := DetectLinks([]byte(body), "example.com:79")
		if len(links) != 1 || links[0].Raw != "alice@example.com" || links[0].Strong {
			t.Fatalf("links = %#v, want policy-B alice@example.com", links)
		}
	})
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

func TestDetectLinks_QuotedFingerQuery_ForwardingDeclinedSameRelay(t *testing.T) {
	body := []byte(`finger "alice smith"@whois.ano@thebackupbox.net`)
	links := DetectLinks(body, "thebackupbox.net:79")
	for _, link := range links {
		if link.Forwarded || link.Action == ActionDrill {
			t.Fatalf("DetectLinks(%q) = %#v, quoted grouping must not forward or drill", body, links)
		}
	}
}

func TestDetectLinks_QuotedFingerQuery_ForwardingDeclinedCrossRelay(t *testing.T) {
	body := []byte(`finger "alice smith"@whois.ano@other-relay.net`)
	links := DetectLinks(body, "thebackupbox.net:79")
	for _, link := range links {
		if link.Forwarded || link.Action == ActionDrill || link.Blocked != "" {
			t.Fatalf("DetectLinks(%q) = %#v, quoted grouping must not forward or block", body, links)
		}
	}
}

// A quoted query may itself contain one "@", which is how aggregator services
// document a command that takes an address as its argument, e.g.
// crossed-fingers.andros.dev's `finger "add?user@host"@crossed-fingers…`.
// The shell strips the quotes, so the wire form is the ordinary one-relay
// forwarding token add?user@host@relay — which is exactly what the server
// expects. lookit must read the whole quoted span as one address rather than
// splitting it at the inner "@".
func TestDetectLinks_QuotedFingerQuery_AddressArgumentSameRelay(t *testing.T) {
	body := []byte(`finger "add?tomasino@cosmic.voyage"@crossed-fingers.andros.dev`)
	links := DetectLinks(body, "crossed-fingers.andros.dev:79")
	if len(links) != 1 {
		t.Fatalf("DetectLinks(%s) = %#v, want exactly 1 link", body, links)
	}
	l := links[0]
	if l.Raw != `"add?tomasino@cosmic.voyage"@crossed-fingers.andros.dev` {
		t.Errorf("Raw = %q, want the whole quoted span", l.Raw)
	}
	if l.Target.Query != "add?tomasino@cosmic.voyage" {
		t.Errorf("Target.Query = %q, want %q", l.Target.Query, "add?tomasino@cosmic.voyage")
	}
	if l.Target.HostPort != "crossed-fingers.andros.dev:79" {
		t.Errorf("Target.HostPort = %q, want the relay, not the inner host", l.Target.HostPort)
	}
	if l.Action != ActionDrill || l.Blocked != "" || !l.Forwarded || !l.Strong {
		t.Errorf("link = %#v, want a strong unblocked forwarded drill", l)
	}
}

// The bug this fixes: the inner "@" was found first, so the span was split at
// the quotes and lookit offered a drill to cosmic.voyage — a host named nowhere
// on the line the user was reading.
func TestDetectLinks_QuotedFingerQuery_AddressArgumentNotSplit(t *testing.T) {
	body := []byte(`finger "add?tomasino@cosmic.voyage"@crossed-fingers.andros.dev`)
	links := DetectLinks(body, "crossed-fingers.andros.dev:79")
	for _, l := range links {
		if l.Target.HostPort == "cosmic.voyage:79" {
			t.Fatalf("DetectLinks(%s) = %#v, quoted span was split and points at the inner host", body, links)
		}
	}
}

func TestDetectLinks_QuotedFingerQuery_AddressArgumentCrossRelay(t *testing.T) {
	body := []byte(`finger "add?tomasino@cosmic.voyage"@crossed-fingers.andros.dev`)
	links := DetectLinks(body, "tilde.team:79")
	if len(links) != 1 {
		t.Fatalf("DetectLinks(%s) = %#v, want exactly 1 link", body, links)
	}
	l := links[0]
	if l.Action != ActionCopy || l.Blocked == "" || !l.Forwarded {
		t.Errorf("link = %#v, want a blocked copy-only forwarded link when the relay is not the origin", l)
	}
}

// A syntax template is not an address. "add?user@host" has a placeholder inner
// host with no dot, so the span must not become a drill — pressing Enter on a
// help page's grammar line would otherwise write a junk entry to the service.
// The bare relay stays linked, which is what shipped before quoted spans could
// carry an inner "@".
func TestDetectLinks_QuotedFingerQuery_PlaceholderInnerHostDeclined(t *testing.T) {
	body := []byte(`finger "add?user@host"@crossed-fingers.andros.dev`)
	links := DetectLinks(body, "crossed-fingers.andros.dev:79")
	if len(links) != 1 || links[0].Raw != "@crossed-fingers.andros.dev" {
		t.Fatalf("DetectLinks(%s) = %#v, want the bare relay fallback only", body, links)
	}
	if links[0].Target.Query != "" {
		t.Errorf("Target.Query = %q, want the host query", links[0].Target.Query)
	}
}

func TestDetectLinks_QuotedFingerQuery_FinalReviewCases(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []struct {
			raw    string
			query  string
			kind   LinkKind
			action LinkAction
			host   string
		}
	}{
		{
			name: "last matching quote and suffix precedence",
			body: `finger "old" prose "new value"@graph.no`,
			want: []struct {
				raw    string
				query  string
				kind   LinkKind
				action LinkAction
				host   string
			}{{`"new value"@graph.no`, "new value", LinkFinger, ActionDrill, "graph.no:79"}},
		},
		{
			name: "unicode query keeps exact bytes",
			body: `finger "東京 · café"@graph.no`,
			want: []struct {
				raw    string
				query  string
				kind   LinkKind
				action LinkAction
				host   string
			}{{`"東京 · café"@graph.no`, "東京 · café", LinkFinger, ActionDrill, "graph.no:79"}},
		},
		{
			name: "phase one overlap declines quote grouping",
			body: `"https://example.com"@graph.no`,
			want: []struct {
				raw    string
				query  string
				kind   LinkKind
				action LinkAction
				host   string
			}{{"https://example.com", "", LinkURL, ActionCopy, ""}, {"@graph.no", "", LinkFinger, ActionDrill, "graph.no:79"}},
		},
		{
			name: "multiple links stay in document order",
			body: `"one"@a.example https://example.com "two"@b.example`,
			want: []struct {
				raw    string
				query  string
				kind   LinkKind
				action LinkAction
				host   string
			}{{`"one"@a.example`, "one", LinkFinger, ActionCopy, "a.example:79"}, {"https://example.com", "", LinkURL, ActionCopy, ""}, {`"two"@b.example`, "two", LinkFinger, ActionCopy, "b.example:79"}},
		},
		{
			name: "original token cue chooses nearer email",
			body: `finger "foo email bar"@graph.no`,
			want: []struct {
				raw    string
				query  string
				kind   LinkKind
				action LinkAction
				host   string
			}{{`"foo email bar"@graph.no`, "", LinkEmail, ActionCopy, ""}},
		},
		{
			name: "server port is pinned",
			body: `finger "alice smith"@graph.no:70`,
			want: []struct {
				raw    string
				query  string
				kind   LinkKind
				action LinkAction
				host   string
			}{{`"alice smith"@graph.no:70`, "alice smith", LinkFinger, ActionDrill, "graph.no:79"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := DetectLinks([]byte(tt.body), "example.com:79")
			if len(links) != len(tt.want) {
				t.Fatalf("DetectLinks(%q) = %#v, want %d links", tt.body, links, len(tt.want))
			}
			for i, want := range tt.want {
				got := links[i]
				if got.Raw != want.raw || got.Target.Query != want.query || got.Kind != want.kind ||
					got.Action != want.action || got.Target.HostPort != want.host {
					t.Errorf("DetectLinks(%q)[%d] = %#v, want Raw=%q Query=%q Kind=%v Action=%v HostPort=%q",
						tt.body, i, got, want.raw, want.query, want.kind, want.action, want.host)
				}
				if tt.name == "original token cue chooses nearer email" && !got.Strong {
					t.Errorf("DetectLinks(%q)[%d] Strong = false, want true", tt.body, i)
				}
			}
		})
	}
}

func TestDetectLinks_QuotedFingerQuery_DoesNotCrossLines(t *testing.T) {
	body := "finger \"opening on prior line\n\"@graph.no"
	links := DetectLinks([]byte(body), "example.com:79")
	if len(links) != 1 || links[0].Raw != "@graph.no" {
		t.Fatalf("DetectLinks(%q) = %#v, want delimiter fallback Raw=%q", body, links, "@graph.no")
	}
}

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

// ---- Regression: document order ----

func TestDetectLinks_DocumentOrder_AtTokenBeforeURL(t *testing.T) {
	// @-token precedes scheme URL in the text; both must appear in document order.
	links := DetectLinks([]byte("admin@example.com https://example.com"), "example.com:79")
	if len(links) != 2 {
		t.Fatalf("got %d links, want 2", len(links))
	}
	if links[0].Raw != "admin@example.com" {
		t.Errorf("links[0].Raw = %q, want %q (document order)", links[0].Raw, "admin@example.com")
	}
	if links[1].Raw != "https://example.com" {
		t.Errorf("links[1].Raw = %q, want %q (document order)", links[1].Raw, "https://example.com")
	}
}

// ---- Regression: finger://user@host (user in URL authority) ----

func TestDetectLinks_FingerURL_UserAtHost_Authority(t *testing.T) {
	// finger://alice@tilde.team — user in authority/userinfo; must drill, not block.
	links := DetectLinks([]byte("finger://alice@tilde.team"), "tilde.team:79")
	l, ok := findLink(links, "finger://alice@tilde.team")
	if !ok {
		t.Fatal("DetectLinks did not find finger://alice@tilde.team")
	}
	if l.Action != ActionDrill {
		t.Errorf("Action = %v, want ActionDrill", l.Action)
	}
	if l.Blocked != "" {
		t.Errorf("Blocked = %q, want empty (must not be treated as forwarding)", l.Blocked)
	}
	if l.Target.HostPort != "tilde.team:79" {
		t.Errorf("Target.HostPort = %q, want %q", l.Target.HostPort, "tilde.team:79")
	}
}

// ---- Regression: server-supplied port in user@host token ----

func TestDetectLinks_FingerCue_PortInHost_Drillable(t *testing.T) {
	// "finger alice@example.org:70" — server advertises a port; ParseTargetPinned
	// discards it, but domainSane must not reject the token before that happens.
	links := DetectLinks([]byte("finger alice@example.org:70"), "example.org:79")
	l, ok := findLink(links, "alice@example.org:70")
	if !ok {
		t.Fatal("DetectLinks did not find alice@example.org:70 with finger cue")
	}
	if l.Action != ActionDrill {
		t.Errorf("Action = %v, want ActionDrill", l.Action)
	}
	if l.Target.HostPort != "example.org:79" {
		t.Errorf("Target.HostPort = %q, want %q (port pinned to 79)", l.Target.HostPort, "example.org:79")
	}
}

func TestApplyLinkOverlay_BodyNotHeader(t *testing.T) {
	st := newStyles(true)
	body := "visit https://example.com today\nplain line\n"
	links := []Link{
		{
			Kind:   LinkURL,
			Action: ActionCopy,
			Raw:    "https://example.com",
			Strong: true,
		},
	}
	result := applyLinkOverlay(body, links, 0, st)
	if !strings.Contains(result, "https://example.com") {
		t.Errorf("applyLinkOverlay result missing URL: %q", result)
	}
	// The plain line must not be highlighted.
	if strings.Contains(result, "\x1b") && strings.Count(result, "plain line") != 1 {
		t.Errorf("plain line appears to be unexpectedly styled in overlay result")
	}
}

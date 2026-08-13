# Colon-bearing Finger addresses implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore the intended `scheme://` plus `mailto:` URL grammar so colon-bearing `query@host` text reaches Finger-address classification intact.

**Architecture:** Keep the two-phase `DetectLinks` scanner and all classifiers unchanged. Narrow only `schemeURLRe`, then lock the boundary with table-driven reader-link tests covering catalogue addresses, supported URLs, deliberately unsupported colon-only URI schemes, byte consumption, and port-79 parsing.

**Tech Stack:** Go 1.26 toolchain from `go.mod`, standard-library `regexp` and `testing`, existing `tui.DetectLinks` tests.

## Global Constraints

- This is PR 1 of 3 and has no code dependency on either later PR.
- Work only on `fix/catalog-finger-links`, already isolated at `/Users/jonathan/lookit/.worktrees/fix-catalog-finger-links` and based on freshly fetched `origin/main`.
- Do not edit `classifySchemeURL`, `classifyFingerURL`, `classifyAtToken`, `findCueWord`, `isDelim`, `stripTrailingPunct`, `isOSC8Openable`, `loginRe`, or list harvesting.
- Preserve the exact phase-1 URL body class shown in the design spec; do not replace it with `!isDelim` semantics.
- Server-provided Finger targets must continue through `finger.ParseTargetPinned`, preserving the port-79 security invariant.
- Do not add dependencies or change exported APIs.
- Do not push, open a PR, merge, or commit until the user explicitly authorizes that action. When shipping is authorized, this security-adjacent target-classification PR must be left for human merge.
- Commit and PR text must use Conventional Commits and contain no AI attribution or co-author trailers.

---

### Task 1: Lock the restored URL grammar with failing regression tests

**Files:**
- Modify: `tui/links_test.go:70-300`

**Interfaces:**
- Consumes: `DetectLinks(body []byte, originHostPort string) []Link`, `findLink([]Link, string) (Link, bool)`, `isOSC8Openable(string) bool`.
- Produces: regression tests defining colon-address classification and the supported phase-1 URL grammar.

- [ ] **Step 1: Add a table test for cued and uncued catalogue addresses**

Add this after `TestDetectLinks_Rule4_BareUserAtHost`:

```go
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
```

- [ ] **Step 2: Add a table test for supported and rejected scheme forms**

Add this near the existing Rule 1 tests:

```go
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
					t.Fatalf("DetectLinks(%q) returned unexpected links: %#v", body, links)
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
```

- [ ] **Step 3: Run the focused tests and verify the expected RED failures**

Run:

```bash
go test ./tui/ -run 'TestDetectLinks_(ColonBearingFingerAddresses|URLGrammar)$' -count=1 -v
```

Expected: FAIL because two-or-more-letter `label:value@host` fixtures are currently `LinkURL` values with empty targets, and `tel:`, `data:`, and `magnet:` are currently generic `LinkURL` values. Confirm that the `o:oslo@graph.no` subtests already pass; they are keep-working coverage, not the source of RED.

---

### Task 2: Narrow phase-1 URL candidates

**Files:**
- Modify: `tui/links.go:219-230`
- Test: `tui/links_test.go`

**Interfaces:**
- Consumes: the Task 1 regression tests and existing `classifySchemeURL(raw, origin) (Link, bool)` behavior.
- Produces: `schemeURLRe` matching only non-empty `scheme://…` and non-empty `mailto:…` forms.

- [ ] **Step 1: Replace the regex and its comment**

Replace the current `schemeURLRe` declaration with:

```go
// schemeURLRe matches explicit scheme:// URLs and mailto: addresses. The
// colon-only alternative is deliberately limited to mailto:; other
// label:value text must remain available to the @-token scanner because Finger
// services commonly use query shapes such as wiki:article@host.
//
// Both alternatives retain the shipped URL body class. Parentheses and square
// brackets may therefore appear inside a URL; stripTrailingPunct removes only
// trailing sentence punctuation and unbalanced closing delimiters.
schemeURLRe = regexp.MustCompile(
	`(?i)(?:[A-Za-z][A-Za-z0-9+.\-]{1,30}://[^\s<>"` + "`" + `]+|mailto:[^\s<>"` + "`" + `]+)`)
```

Do not add a special case for `wiki:`, any catalogue service name, or one-letter prefixes.

- [ ] **Step 2: Run the focused tests and verify GREEN**

Run:

```bash
go test ./tui/ -run 'TestDetectLinks_(ColonBearingFingerAddresses|URLGrammar)$' -count=1 -v
```

Expected: PASS. In particular, `wiki:albert_einstein:1@bbs.airandwave.net` is `LinkFinger`, the two `mailto:` cases each produce exactly one consumed link, and the unsupported colon-only URI schemes are not `LinkURL`.

- [ ] **Step 3: Run the complete TUI package tests**

Run:

```bash
go test ./tui/ -count=1
```

Expected: PASS with no changed existing expectations.

- [ ] **Step 4: Run the repository gate**

Before claiming completion, invoke `superpowers:verification-before-completion`, then run:

```bash
make check
```

Expected: all four Makefile gates pass: `go vet ./...`, empty `gofmt -l .`, `golangci-lint run ./...`, and `go test ./... -race`.

- [ ] **Step 5: Review the diff for PR-1 scope**

Run:

```bash
git diff --check
git diff -- tui/links.go tui/links_test.go docs/superpowers/specs/2026-08-13-catalog-finger-command-links-design.md docs/superpowers/plans/2026-08-13-colon-finger-addresses.md
git status --short
```

Expected: production changes are limited to `schemeURLRe` and its comment; tests cover only PR-1 grammar behavior. The shared design and implementation-plan documents may accompany PR 1, but no spaced-query or quote-grouping production code may appear.

- [ ] **Step 6: Commit only after explicit approval**

```bash
git add tui/links.go tui/links_test.go docs/superpowers/specs/2026-08-13-catalog-finger-command-links-design.md docs/superpowers/plans/2026-08-13-*.md
git commit -m "fix(tui): stop classifying colon-bearing finger addresses as URLs"
```

Do not push or open the PR without a separate explicit go-ahead. When authorized, use the same Conventional Commit text as the PR title and leave merge to the human because the diff touches server-supplied target classification.

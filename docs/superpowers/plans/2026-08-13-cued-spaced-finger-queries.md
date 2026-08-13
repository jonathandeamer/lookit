# Cued spaced Finger queries implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a word-bounded `finger` cue group a following one-`@` query across spaces while preserving existing cue classification, forwarding restrictions, port pinning, and list-harvest behavior.

**Architecture:** Keep `classifyAtToken` and `findCueWord` authoritative for kind and action. Add small private helpers that locate the last word-bounded `finger` on the line and propose a wider span; `DetectLinks` accepts that span only when the existing five-word cue lookup returned `finger`, the candidate contains exactly one `@`, contains no second word-bounded `finger`, contains no ASCII quote/backtick syntax, and does not overlap bytes already consumed by another link.

**Tech Stack:** Go 1.26 toolchain from `go.mod`, standard-library `strings` and `testing`, existing `tui.DetectLinks` and `parseUserList` tests.

## Global Constraints

- This is PR 2 of 3. It depends on PR 1 (`schemeURLRe` restoration) and must be implemented on a fresh branch from updated `origin/main` after PR 1 is merged.
- Suggested branch/worktree after syncing `main`: `feat/cued-finger-queries` at `.worktrees/feat-cued-finger-queries`.
- It has no logical dependency on quote grouping. Its ASCII quote/backtick guard keeps valid and malformed quoted text on today's delimiter-bounded fallback until PR 3 groups the one accepted production.
- Do not change `cueKind` or `findCueWord`: the existing nearest recognized cue in the five-field same-line window continues to determine classification.
- Do not change `classifyAtToken`, `classifyForwardedAtToken`, `domainSane`, `isDelim`, `loginRe`, `harvestableLogin`, or `appendHarvestedTargets`.
- Never accept an expanded span containing two `@` bytes; ordinary whitespace-free two-`@` forwarding remains on the existing path.
- Never accept an expanded span containing ASCII `"`, `'`, or backtick; only PR 3 may interpret the accepted quote production.
- Server-provided direct targets must continue through `finger.ParseTargetPinned`, including expanded targets with an advertised non-79 port.
- Do not push, open a PR, merge, or commit until the user explicitly authorizes that action. When shipping is authorized, leave this target-classification PR for human merge.
- Commit and PR text must use Conventional Commits and contain no AI attribution or co-author trailers.

---

### Task 1: Specify cued expansion and its rejection boundaries

**Files:**
- Modify: `tui/links_test.go`
- Modify: `tui/userlist_test.go`

**Interfaces:**
- Consumes: `DetectLinks(body []byte, originHostPort string) []Link`, existing `Link` fields, `parseUserList(body []byte, originHostPort string) (parsedUserList, bool)`, and `loginRe`.
- Produces: failing tests for the exact expansion predicate and harvest neutrality.

- [ ] **Step 1: Create the isolated PR-2 branch after PR 1 merges**

From `/Users/jonathan/lookit`, run each command separately:

```bash
git fetch origin
git log -1 --oneline origin/main
git worktree add .worktrees/feat-cued-finger-queries -b feat/cued-finger-queries origin/main
```

Confirm the displayed `origin/main` commit contains PR 1, then switch all remaining commands in this plan to `/Users/jonathan/lookit/.worktrees/feat-cued-finger-queries`. Establish the baseline:

```bash
go mod download
go test ./...
```

Expected: the new worktree is clean and the full baseline passes before adding PR-2 tests.

- [ ] **Step 2: Add successful expansion cases**

Add:

```go
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
```

- [ ] **Step 3: Add multi-command and decline cases**

Add:

```go
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
			if link.Forwarded || strings.Count(link.Raw, "@") != 1 {
				t.Fatalf("expanded span synthesized forwarding: %#v", link)
			}
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
```

The test file already imports `strings`; do not add another import.

- [ ] **Step 4: Add list-harvest neutrality coverage**

Add to `tui/userlist_test.go` near the generic-harvest tests (that file already imports `reflect` and defines `logins`):

```go
func TestStrongGate_ServiceQueriesStayReaderOnly(t *testing.T) {
	body := []byte(
		"Login   Name\n" +
			"alice   Alice\n" +
			"bob     Bob\n" +
			"\n" +
			"Use: finger dict:word@other.host\n" +
			"Use: finger urban:old school@bbs.airandwave.net\n",
	)
	parsed, ok := parseUserList(body, "example.com:79")
	if !ok {
		t.Fatal("parseUserList ok = false, want true")
	}
	harvested := appendHarvestedTargets(parsed.users, body, "example.com:79")
	if got := logins(harvested); !reflect.DeepEqual(got, []string{"alice", "bob"}) {
		t.Fatalf("logins = %v, want only structured rows", got)
	}
	if loginRe.MatchString("dict:word") || loginRe.MatchString("urban:old school") {
		t.Fatal("service queries unexpectedly match the harvestable login grammar")
	}

	links := DetectLinks(body, "example.com:79")
	if link, ok := findLink(links, "dict:word@other.host"); !ok || !link.Strong || link.Action != ActionDrill {
		t.Fatalf("dict reader link = %#v, found=%v", link, ok)
	}
	if link, ok := findLink(links, "urban:old school@bbs.airandwave.net"); !ok || !link.Strong || link.Action != ActionDrill {
		t.Fatalf("urban reader link = %#v, found=%v", link, ok)
	}
}
```

- [ ] **Step 5: Run the new tests and verify RED**

Run:

```bash
go test ./tui/ -run 'TestDetectLinks_CuedSpacedFingerQuery|TestStrongGate_ServiceQueriesStayReaderOnly' -count=1 -v
```

Expected: FAIL because `DetectLinks` currently returns only `school@…` for the expanded fixtures. Confirm the uncued, `fingerprint`, five-field-window, and email-cue subtests already pass; they protect unchanged behavior.

---

### Task 2: Add a rejected-by-default expansion helper

**Files:**
- Modify: `tui/links.go:96-158, 276-293`
- Test: `tui/links_test.go`
- Test: `tui/userlist_test.go`

**Interfaces:**
- Consumes: original phase-2 `start`/`end` byte offsets, `findCueWord(text, start) string`, `isWordChar(byte) bool`, and the phase-1 `consumed []bool` map.
- Produces: `isWordBoundedFinger(text string, start int) bool`, `lastFingerCue(text string, lineStart, before int) int`, and `expandFingerSpan(text string, tokenStart, tokenEnd int) (start int, raw string, ok bool)`.

- [ ] **Step 1: Add exact word-boundary and span helpers**

Place these after `findCueWord`:

```go
func isWordBoundedFinger(text string, start int) bool {
	const cue = "finger"
	if start < 0 || start+len(cue) > len(text) || !strings.EqualFold(text[start:start+len(cue)], cue) {
		return false
	}
	return (start == 0 || !isWordChar(text[start-1])) &&
		(start+len(cue) == len(text) || !isWordChar(text[start+len(cue)]))
}

func lastFingerCue(text string, lineStart, before int) int {
	for start := before - len("finger"); start >= lineStart; start-- {
		if isWordBoundedFinger(text, start) {
			return start
		}
	}
	return -1
}

func expandFingerSpan(text string, tokenStart, tokenEnd int) (int, string, bool) {
	lineStart := strings.LastIndex(text[:tokenStart], "\n") + 1
	cueStart := lastFingerCue(text, lineStart, tokenStart)
	if cueStart < 0 {
		return tokenStart, "", false
	}

	afterCue := text[cueStart+len("finger") : tokenEnd]
	raw := strings.TrimSpace(afterCue)
	if raw == "" || strings.Count(raw, "@") != 1 ||
		lastFingerCue(raw, 0, len(raw)) >= 0 || strings.ContainsAny(raw, "\"'`") {
		return tokenStart, "", false
	}
	leading := strings.Index(afterCue, raw)
	return cueStart + len("finger") + leading, raw, true
}
```

`lastFingerCue` searches only the current line because the caller supplies `lineStart`. The `strings.TrimSpace` call establishes the frozen `Raw` rule and the `strings.Index` result converts it back to an absolute byte offset.

- [ ] **Step 2: Apply expansion only after existing cue classification says Finger**

In phase 2, retain the original token offsets for cue detection and boundary checks. Immediately after:

```go
cueWord := findCueWord(text, start)
```

add:

```go
if strings.EqualFold(cueWord, "finger") {
	if expandedStart, expandedRaw, ok := expandFingerSpan(text, start, end); ok {
		overlapsConsumed := false
		for i := expandedStart; i < end; i++ {
			if consumed[i] {
				overlapsConsumed = true
				break
			}
		}
		if !overlapsConsumed {
			start = expandedStart
			raw = expandedRaw
		}
	}
}
```

Then call the unchanged:

```go
link, ok := classifyAtToken(raw, cueWord, origin)
```

Do not move `findCueWord` to the expanded start: classification must retain the existing five-field window calculated from the original `isDelim` token. Do not add any two-`@` fallback to `expandFingerSpan`, and do not remove the ASCII quote/backtick rejection when PR 3 later adds valid quote grouping before this path.

- [ ] **Step 3: Run the focused suite and verify GREEN**

Run:

```bash
go test ./tui/ -run 'TestDetectLinks_CuedSpacedFingerQuery|TestStrongGate_ServiceQueriesStayReaderOnly' -count=1 -v
```

Expected: PASS. The shared-cue case produces two links and never a `Forwarded` link; the advertised port is pinned to 79; only `alice` and `bob` remain list rows.

- [ ] **Step 4: Run all link, forwarding, and list tests**

Run:

```bash
go test ./tui/ -run 'TestDetectLinks|TestStrongGate|TestGenericHarvest' -count=1 -v
```

Expected: PASS, including existing same-relay forwarding and document-order tests.

- [ ] **Step 5: Run the repository gate**

Before claiming completion, invoke `superpowers:verification-before-completion`, then run:

```bash
make check
```

Expected: all four Makefile gates pass.

- [ ] **Step 6: Review the diff for PR-2 scope**

Run:

```bash
git diff --check
git diff -- tui/links.go tui/links_test.go tui/userlist_test.go
git status --short
```

Expected: no quote-production matcher, no URL-regex changes, no edits to list-harvest production code, and no `loginRe` change. The only quote-related production behavior is rejecting quote/backtick-bearing expansion candidates.

- [ ] **Step 7: Commit only after explicit approval**

```bash
git add tui/links.go tui/links_test.go tui/userlist_test.go
git commit -m "feat(tui): expand cued finger commands across spaces"
```

Do not push or open the PR without a separate explicit go-ahead; leave merge to the human.

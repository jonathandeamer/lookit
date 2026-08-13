# Quoted Finger queries implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Recognize the exact shell-style `q query q @ host` production as one Finger link while preserving quote characters in `Link.Raw` and removing them from the wire query.

**Architecture:** Keep quote bytes as global token delimiters. Add a private suffix matcher that starts from the original `@`, accepts only matching ASCII single or double quotes abutting `@`, returns separate display and parse strings, and runs before cued space expansion; classification still uses `findCueWord` and `classifyAtToken` unchanged. If matching fails, PR 2's ASCII quote/backtick rejection preserves the original delimiter-bounded link instead of widening malformed shell-like syntax.

**Tech Stack:** Go 1.26 toolchain from `go.mod`, standard-library `strings` and `testing`, existing `tui.DetectLinks` tests.

## Global Constraints

- This is PR 3 of 3. Its behavior depends only on PR 1, but its combined scanner precedence must be tested with PR 2. While PR 1 awaits human merge, implement PR 3 locally on `feat/quoted-finger-queries`, stacked from local PR-2 commit `2e4b4de`; do not push the stacked branch. After PR 1 is squash-merged, rebase PR 2 onto updated `origin/main`; after PR 2 is human-merged, rebase only the PR-3 changes onto updated `origin/main` before the first push.
- Use branch/worktree `feat/quoted-finger-queries` at `.worktrees/feat-quoted-finger-queries`.
- Quote grouping must execute before PR-2 spaced expansion when both are present. If PR 2 is absent, grouping still works without an expansion helper.
- Keep PR 2's ASCII quote/backtick rejection in `expandFingerSpan`; malformed quote-like syntax must fall back to the original `isDelim` token.
- Do not remove `'`, `"`, or backtick from `isDelim`; existing quoted-URL punctuation behavior must remain unchanged.
- Accept only matching ASCII `'` or `"`, with a non-empty query containing neither that quote nor `@`, and no whitespace between the closing quote and `@`.
- Quoted grouping is direct one-`@` syntax only: the query contains no `@`, and the host suffix contains no additional `@` beyond its leading separator. Quoted forwarding is out of scope; existing whitespace-free `user@host@relay` and `finger://` forwarding behavior remains unchanged.
- The `finger` cue is optional for grouping. It affects classification only through the unchanged `findCueWord` five-field window.
- Keep exact quotes in `Link.Raw`; pass `query@host` without quotes to `classifyAtToken`; restore `Link.Raw` only after successful classification.
- Do not change forwarding rules, `domainSane`, `ParseTargetPinned`, `loginRe`, harvesting, OSC-8 policy, or `applyLinkOverlay`.
- Do not push, open a PR, merge, or commit until the user explicitly authorizes that action. When shipping is authorized, leave this target-classification PR for human merge.
- Commit and PR text must use Conventional Commits and contain no AI attribution or co-author trailers.

---

### Task 1: Specify the accepted quote production and declines

**Files:**
- Modify: `tui/links_test.go`

**Interfaces:**
- Consumes: `DetectLinks(body []byte, originHostPort string) []Link` and existing `Link` fields.
- Produces: failing tests freezing display `Raw`, parse `Target.Query`, cue action, suffix precedence, punctuation stripping, and rejected quote placements.

- [ ] **Step 1: Create the isolated local PR-3 branch from PR 2**

While PR 2 remains local, from `/Users/jonathan/lookit` run:

```bash
git fetch origin
git log -1 --oneline feat/cued-finger-queries
git worktree add .worktrees/feat-quoted-finger-queries -b feat/quoted-finger-queries feat/cued-finger-queries
```

Confirm the displayed branch tip is the reviewed PR-2 commit, then switch all remaining commands in this plan to `/Users/jonathan/lookit/.worktrees/feat-quoted-finger-queries`. Establish the baseline:

```bash
go mod download
go test ./...
```

Expected: the new worktree is clean and the full baseline passes before adding PR-3 tests.

After PR 2 is human-merged, fetch `origin`, rebase only the PR-3 changes onto updated `origin/main`, and verify again before the first push. This avoids publishing a stacked branch that would later require a force-push.

- [ ] **Step 2: Add accepted quote-production cases**

Add:

```go
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
```

- [ ] **Step 3: Add rejected-placement cases**

Add:

```go
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
```

- [ ] **Step 4: Run the new tests and verify RED**

Run:

```bash
go test ./tui/ -run 'TestDetectLinks_QuotedFingerQuery' -count=1 -v
```

Expected: accepted cases FAIL because current phase 2 focuses only `@graph.no` or a suffix token. Every decline case passes because PR 2 rejects quote/backtick-bearing expansion candidates.

---

### Task 2: Match quoted suffixes before spaced expansion

**Files:**
- Modify: `tui/links.go:96-158, 276-293`
- Test: `tui/links_test.go`

**Interfaces:**
- Consumes: original `atAbs` and `end` byte offsets from phase 2, `stripTrailingPunct(string) string`, `findCueWord(text string, pos int) string`, and `classifyAtToken(raw, cueWord, origin string) (Link, bool)`.
- Produces: `quotedAtToken(text string, at, tokenEnd int) (start int, raw, parseRaw string, ok bool)` and quote-first phase-2 precedence.

- [ ] **Step 1: Add the narrow quote-production helper**

Place after `findCueWord`:

```go
func quotedAtToken(text string, at, tokenEnd int) (int, string, string, bool) {
	if at == 0 {
		return 0, "", "", false
	}
	quote := text[at-1]
	if quote != '\'' && quote != '"' {
		return 0, "", "", false
	}

	lineStart := strings.LastIndex(text[:at-1], "\n") + 1
	relOpen := strings.LastIndexByte(text[lineStart:at-1], quote)
	if relOpen < 0 {
		return 0, "", "", false
	}
	open := lineStart + relOpen
	query := text[open+1 : at-1]
	if query == "" || strings.Contains(query, "@") || strings.ContainsRune(query, rune(quote)) {
		return 0, "", "", false
	}

	hostPart := stripTrailingPunct(text[at:tokenEnd])
	if len(hostPart) <= 1 || strings.Contains(hostPart[1:], "@") {
		return 0, "", "", false
	}
	raw := text[open:at] + hostPart
	parseRaw := query + hostPart
	return open, raw, parseRaw, true
}
```

Because `relOpen` selects the last matching quote before the closing quote, the substring between them cannot contain the same quote. The explicit `ContainsRune` check documents the production and guards future edits.

- [ ] **Step 2: Route a matched quote through the existing classifier**

In phase 2, calculate the cue from the original delimiter-bounded token exactly as today:

```go
cueWord := findCueWord(text, start)
parseRaw := raw
quoted := false
if quotedStart, quotedRaw, quotedParseRaw, ok := quotedAtToken(text, atAbs, end); ok {
	overlapsConsumed := false
	for i := quotedStart; i < end; i++ {
		if consumed[i] {
			overlapsConsumed = true
			break
		}
	}
	if !overlapsConsumed {
		start = quotedStart
		raw = quotedRaw
		parseRaw = quotedParseRaw
		quoted = true
	}
}
```

The PR-2 expansion block must follow only when grouping did not win; its existing ASCII quote/backtick rejection handles malformed syntax:

```go
if !quoted && strings.EqualFold(cueWord, "finger") {
	// Existing PR-2 expandFingerSpan block, unchanged.
}
```

Classify the parse form, then restore the display form:

```go
link, ok := classifyAtToken(parseRaw, cueWord, origin)
if !ok {
	pos = end
	continue
}
if quoted {
	link.Raw = raw
}
```

Keep the existing consumed marking, document-order position, and `pos = end` behavior. Do not move quote removal into `classifyAtToken`.

- [ ] **Step 3: Run the quote suite and verify GREEN**

Run:

```bash
go test ./tui/ -run 'TestDetectLinks_QuotedFingerQuery' -count=1 -v
```

Expected: PASS. Cued quotes drill, uncued quotes remain policy B, suffix-only precedence excludes `oslo,`, trailing punctuation is absent from `Raw`, and every `Target.Query` is unquoted.

- [ ] **Step 4: Run quote-sensitive and link-regression tests**

Run:

```bash
go test ./tui/ -run 'TestDetectLinks_(QuotedFingerQuery|Punctuation|CuedSpacedFingerQuery|Forwarding|DocumentOrder|URLGrammar)' -count=1 -v
```

Expected: PASS, including PR-2 spaced expansion and quote-first precedence.

- [ ] **Step 5: Run the repository gate**

Before claiming completion, invoke `superpowers:verification-before-completion`, then run:

```bash
make check
```

Expected: all four Makefile gates pass.

- [ ] **Step 6: Review the diff for PR-3 scope and precedence**

Run:

```bash
git diff --check
git diff -- tui/links.go tui/links_test.go
git status --short
```

Expected: `isDelim`, URL grammar, classifiers, forwarding, harvest, and overlay are untouched. In the combined code path, quote matching precedes cued spaced expansion.

- [ ] **Step 7: Commit only after explicit approval**

```bash
git add docs/superpowers/plans/2026-08-13-quoted-finger-queries.md docs/superpowers/specs/2026-08-13-catalog-finger-command-links-design.md tui/links.go tui/links_test.go
git commit -m "feat(tui): group shell-quoted finger queries as one link"
```

Do not push or open the PR without a separate explicit go-ahead; leave merge to the human.

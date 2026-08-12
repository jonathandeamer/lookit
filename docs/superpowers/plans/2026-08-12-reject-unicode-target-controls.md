# Reject Unicode Target Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject Cf, Zl, and Zp Unicode controls anywhere a parsed or manually constructed Finger query could reach the terminal or wire.

**Architecture:** Extend the existing unexported `finger.hasControl(string) bool` predicate so both `ParseTarget`/`ParseTargetPinned` and `queryWith` inherit the same reject-don't-strip rule. Keep response sanitization unchanged: response bodies remain content that is visualized, while targets and outbound query tokens are refused.

**Tech Stack:** Go 1.26 toolchain, standard-library `unicode` tables, table-driven unit tests, Go fuzz seeds.

## Global Constraints

- Keep the change limited to Cf/Zl/Zp; invalid UTF-8 and UTF-8-encoded C1 target handling remain outside this issue.
- Preserve the existing `target contains control characters` and `query contains control characters` error text.
- Apply the rule equally to user-typed targets and `ParseTargetPinned` server-derived targets.
- Do not change historical specs or the dated security-review record; update only living architecture documentation and now-stale source comments.
- Do not commit, push, open a PR, or merge without a separate explicit request.

---

### Task 1: Enforce the shared Unicode-control invariant

**Files:**
- Modify: `finger/query_test.go`
- Modify: `finger/client_test.go`
- Modify: `finger/fuzz_test.go`
- Modify: `finger/query.go`
- Modify: `tui/bookmarks.go`
- Modify: `CLAUDE.md`
- Modify: `AGENTS.md`

**Interfaces:**
- Consumes: `hasControl(string) bool`, `ParseTarget(string) (Target, error)`, `ParseTargetPinned(string) (Target, error)`, and `Query(context.Context, Target) ([]byte, Meta, error)`.
- Produces: the unchanged `hasControl(string) bool` interface with a wider C0/DEL/Cf/Zl/Zp rejection contract.

- [x] **Step 1: Add parser regressions**

Add literal table cases proving `ParseTarget` rejects representative Cf (`U+202E`), Zl (`U+2028`), and Zp (`U+2029`) runes with `target contains control characters`. Add a pinned-target case proving `ParseTargetPinned` rejects the same class instead of constructing a drill target.

- [x] **Step 2: Add the direct-query regression**

Add a real loopback-listener test that constructs `Target{Query: "a\u202eb", HostPort: ln.Addr().String()}`, calls `Query`, requires `query contains control characters`, and proves the server received zero bytes. This covers manually constructed targets that bypass the parser.

- [x] **Step 3: Strengthen the fuzz property and seed corpus**

Add `alice@\u202eexample.org` as a `FuzzParseTarget` seed. Independently scan every accepted target's `QueryLine` and `HostPort` with `unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp)` so the property fails against the old implementation rather than reusing `hasControl` as its own oracle.

- [x] **Step 4: Verify the tests fail for the missing behavior**

Run: `go test ./finger -run 'TestParseTarget|TestParseTargetPinned|TestQueryRejectsUnicodeFormatControlsInQuery|FuzzParseTarget' -count=1`

Expected: parser and direct-query cases fail because the old byte-only predicate accepts U+202E/U+2028/U+2029; the fuzz seed reports that an accepted target carries a Unicode control.

- [x] **Step 5: Implement the minimal predicate change**

Import `unicode` in `finger/query.go`. Iterate through runes in `hasControl` and return true for existing C0/DEL cases or `unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp)`. Update the helper comment to describe the expanded display and egress invariant.

- [x] **Step 6: Verify focused tests pass**

Run: `go test ./finger -run 'TestParseTarget|TestParseTargetPinned|TestQueryRejectsUnicodeFormatControlsInQuery|FuzzParseTarget' -count=1`

Expected: PASS.

- [x] **Step 7: Update living documentation**

Update `tui/bookmarks.go` so its validator comment says `ParseTarget` now rejects C0/DEL/Cf/Zl/Zp while bookmark-specific validation still covers invalid UTF-8 and C1. Update the matching `finger/` architecture paragraphs in `CLAUDE.md` and `AGENTS.md`; leave dated specs and decisions unchanged as point-in-time records.

- [x] **Step 8: Format and run the complete gate**

Run: `make fmt`

Run: `make check`

Expected: all vet, formatting, lint, and race-test gates pass.

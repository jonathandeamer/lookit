# Filter Accept Late Results Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep a synchronously accepted list filter authoritative when an older asynchronous Bubbles filter result arrives afterward.

**Architecture:** Continue using `acceptFilter` to synchronously compute the accepted query. At the `appModel.Update` routing boundary, discard `list.FilterMatchesMsg` whenever the active list has already left `list.Filtering`; Bubbles does not identify the query that produced a result, so any result delivered after acceptance or reset is stale by definition.

**Tech Stack:** Go, Bubble Tea v2, Bubbles list v2.

## Global Constraints

- Apply the rule to all three filterable lists: startpage, user list, and links panel.
- Preserve Bubbles' asynchronous results while the active list is still in `list.Filtering`.
- Preserve the synchronous empty-query and zero-match reset behavior already implemented by `acceptFilter`.
- Do not change user-facing copy.

---

### Task 1: Reject post-accept filter results

**Files:**
- Modify: `tui/app_test.go`
- Modify: `tui/app.go`
- Modify: `tui/CLAUDE.md`

**Interfaces:**
- Consumes: `appModel.Update(tea.Msg) (tea.Model, tea.Cmd)`, `appModel.helpFilterActive() bool`, and Bubbles' `list.FilterMatchesMsg`.
- Produces: an update-loop invariant that `list.FilterMatchesMsg` is delegated only while the active list is actively filtering.

- [ ] **Step 1: Write the failing regression test**

Add `TestLateFilterResultCannotReplaceAcceptedSelection`. Build a real user list containing `ax`, `ab`, and `slow`; capture the real filter command produced for prefix `a`; finish typing `ab`; accept synchronously; deliver the older `a` result; then press Enter. Assert the recorded fetch target is the literal `ab@tilde.team`, not `ax@tilde.team`.

- [ ] **Step 2: Run the regression test and verify RED**

Run:

```bash
go test ./tui -run TestLateFilterResultCannotReplaceAcceptedSelection -count=1 -v
```

Expected: FAIL because the late prefix result replaces `filteredItems` and Enter fetches `ax@tilde.team`.

- [ ] **Step 3: Implement the minimal routing guard**

In the `appModel.Update` type switch, handle `list.FilterMatchesMsg` before generic delegation:

```go
case list.FilterMatchesMsg:
	if !m.helpFilterActive() {
		return m, nil
	}
```

When a list remains in `list.Filtering`, fall through to the existing active-content delegation. When acceptance or reset has changed the filter state, return without handing the unversioned result back to Bubbles.

- [ ] **Step 4: Record the invariant in package guidance**

Extend the filtering section of `tui/CLAUDE.md` to state that asynchronous filter results are accepted only while the active list is still filtering, because queued results have no query or generation identity.

- [ ] **Step 5: Verify GREEN and repository gates**

Run:

```bash
go test ./tui -run 'TestLateFilterResultCannotReplaceAcceptedSelection|TestEnterAfterFilterDrillsTheVisibleSelection|TestBookmarkAfterFilterPinsTheVisibleSelection|TestLinksPanelAcceptFilterSelectsTheMatch' -count=1 -v
make check
```

Expected: both commands exit 0; the regression fetches `ab@tilde.team`; all vet, formatting, lint, and race-test gates pass.

- [ ] **Step 6: Commit and push the existing PR branch**

```bash
git add docs/superpowers/plans/2026-08-15-filter-accept-late-results.md tui/CLAUDE.md tui/app.go tui/app_test.go
git commit -m "fix(tui): ignore stale filter results after accept"
git push origin fix/filter-accept-race
```

Expected: PR #139 updates with the regression and fix; no new PR is created.

# Startpage Section Gap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render one blank row between the `COMMUNITIES` and `SERVICES` sections without adding excess height on narrow terminals.

**Architecture:** A presentation-only `startItem` marks the gap in the one-row layout. A pure item builder includes it only at widths of 72 columns or more when both sections exist; `setSize` synchronizes unfiltered items across the breakpoint while restoring selection by section-relative ordinal. The existing two-row header supplies the narrow gap.

**Tech Stack:** Go, Bubble Tea v2, Bubbles list v2, Lip Gloss v2.

## Global Constraints

- Keep `finger/` and its security invariants untouched.
- Preserve the fixed-height delegate contract: one row at widths of 72 or more, two rows below 72.
- Preserve filtering, counts, bookmarks, catalog assembly, and startpage copy.
- Do not commit, push, or open a PR without explicit user authorization.

---

### Task 1: Add the responsive section separator

**Files:**
- Modify: `tui/start.go`
- Test: `tui/start_test.go`

**Interfaces:**
- Consumes: `startWideMinWidth`, `startSection`, `startItem`, `startModel.setSize`, `startModel.captureTogglePosition`, and `startModel.selectSectionPosition`.
- Produces: `startItem.spacer bool` and `startItems(sections []startSection, width int) []list.Item`.

- [x] **Step 1: Write failing rendering and assembly tests**

Add tests which assert a literal blank line immediately before `SERVICES` at
80 columns, exactly one such line at 40 columns, no spacer without both catalog
sections, and no spacer in filtered visible items.

- [x] **Step 2: Run the focused tests and verify RED**

Run: `go test ./tui -run 'TestStartSectionGap|TestStartFilterDropsSectionGap' -count=1 -v`

Expected: FAIL because the wide rendering has no blank line and `startItem`
does not yet represent a spacer.

- [x] **Step 3: Implement the minimal spacer and item builder**

Add `spacer bool` to `startItem`; make `startItems` append a spacer immediately
before `SERVICES` only when width is at least `startWideMinWidth` and a
`COMMUNITIES` section is also present. Return early from delegate rendering for
the spacer. Use `startItems` in `newStart`.

- [x] **Step 4: Run the focused tests and verify GREEN**

Run: `go test ./tui -run 'TestStartSectionGap|TestStartFilterDropsSectionGap' -count=1 -v`

Expected: PASS.

- [x] **Step 5: Write failing navigation and resize tests**

Add tests proving downward/upward movement skips both spacer and header,
unfiltered resizing across widths 72/71 preserves a selected service and a
repeated bookmark occurrence, and clearing a filter after a narrow resize
restores the narrow item set.

- [x] **Step 6: Run the new tests and verify RED**

Run: `go test ./tui -run 'TestStartCursorSkipsSectionGap|TestStartSectionGapResponsiveResize' -count=1 -v`

Expected: the resize cases FAIL because `setSize` does not yet synchronize the
width-dependent item slice.

- [x] **Step 7: Implement responsive synchronization**

When `setSize` is unfiltered, capture the selected section-relative ordinal,
rebuild list items for the new width, and restore that occurrence. Do not replace items during
`Filtering` or `FilterApplied`; rely on the existing transition back to
`Unfiltered` to call `setSize` and synchronize then.

- [x] **Step 8: Run focused and package tests**

Run:

```bash
go test ./tui -run 'TestStartSectionGap|TestStartFilterDropsSectionGap|TestStartCursorSkipsSectionGap' -count=1 -v
go test ./tui -count=1
```

Expected: PASS.

- [x] **Step 9: Run the repository gate and review scope**

Run:

```bash
make check
git diff --check
git status --short
git diff --stat
git diff -- tui/start.go tui/start_test.go docs/superpowers/specs/2026-08-13-startpage-section-gap-design.md docs/superpowers/plans/2026-08-13-startpage-section-gap.md
```

Expected: `make check` and `git diff --check` exit 0; changes are limited to
the four listed files.

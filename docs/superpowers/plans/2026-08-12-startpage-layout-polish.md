# Startpage Layout Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the startpage denser, clearer, and more stable without adding destinations, commands, persistence fields, navigation modes, or network behavior.

**Architecture:** Keep the startpage as one `bubbles/list`, but give every flattened row an explicit section identity and replace the borrowed user-list delegate with a responsive startpage delegate. `startModel` owns the fixed overview and filter-aware sizing; `appModel` owns focus truth and captures/restores section positions around bookmark-file reloads. Move catalog attribution out of the selectable list and into About.

**Tech Stack:** Go, Bubble Tea v2 (`charm.land/bubbletea/v2`), Bubbles v2 list, Lip Gloss v2, `github.com/charmbracelet/x/ansi`, table-driven Go tests.

## Global Constraints

- Build against the versions resolved by `go.mod`; keep the `charm.land/*/v2` imports and add no dependencies.
- Preserve stock Bubbles cursor movement, `g`/`G`, paging, paginator, filter matching, filter input, Enter-to-apply, Esc-to-cancel/clear, and native zero-match behavior.
- Use `startWideMinWidth = 72` and `startTargetColumnPct = 50`; neither rendered column nor overview line may wrap or exceed the model width.
- Keep all colors adaptive through the existing palette; use gold plus bold only for the focused overview segment, never as permanent bookmark decoration.
- Do not change `tui/catalog.txt`, catalog classification, bookmark grammar or persistence, request routing, history, reader/list behavior, or key assignments.
- Keep the dormant `kindPerson` parser and section assembly, but add no user-facing `PEOPLE` identity, overview segment, focus rule, or test.
- Tests stay offline and use `FetchFunc`, temporary bookmark paths, and pure render helpers.
- Use Conventional Commits without co-author or generated-by trailers.
- Run `make check` before the final implementation commit.

## File Map

- `tui/start.go`: start row identity, responsive delegate, overview rendering, filter-aware sizing, selection-position helpers.
- `tui/start_test.go`: delegate, overview, focus, filtering, pagination, and position-helper tests.
- `tui/sections.go`: attach stable IDs to assembled bookmark/community/service sections.
- `tui/sections_test.go`: assert section IDs survive pinning, deduplication, and `catalog off`.
- `tui/about.go`: own the catalog credit URL and render its unconditional hyperlink.
- `tui/about_test.go`: verify credit copy, hyperlink, narrow truncation, and unknown-build behavior.
- `tui/app.go`: synchronize input/content focus, restore startpage selection after bookmark writes, and expose contextual bookmark copy.
- `tui/app_test.go`: cover section-stable and filter-stable bookmark toggles plus focus integration.
- `tui/request.go`: use the shared focus setter when cancellation returns to the input.
- `tui/request_test.go`: verify request cancellation keeps shared focus state synchronized.
- `tui/statusbar_test.go`: pin `b bookmark` versus `b remove` startpage copy.

---

### Task 1: Move Catalog Attribution to About

**Files:**
- Modify: `tui/start.go:14-138`
- Modify: `tui/start_test.go:73-150`
- Modify: `tui/about.go:10-76`
- Modify: `tui/about_test.go:9-72`

**Interfaces:**
- Consumes: the existing pure `aboutView` renderer and `lipgloss.Style.Hyperlink`.
- Produces: `aboutCatalogCreditURL` in `tui/about.go`; a startpage whose flattened items are only headers and entries.

- [ ] **Step 1: Write failing attribution and row-shape tests**

Replace the startpage credit tests with a structural assertion, and extend the About identity test:

```go
func TestStartHasNoCatalogCreditRow(t *testing.T) {
	m := newStart(testCommon(), twoSections(), "", "")
	items := m.list.Items()
	if len(items) == 0 {
		t.Fatal("startpage has no items")
	}
	last, ok := items[len(items)-1].(startItem)
	if !ok || !last.selectable() {
		t.Fatalf("last item = %#v, want a selectable entry", items[len(items)-1])
	}
	if strings.Contains(stripANSIForLandingTest(m.View()), "Catalog inspired by") {
		t.Fatalf("startpage still renders catalog attribution:\n%s", m.View())
	}
}

func TestAboutViewRendersCatalogCreditHyperlink(t *testing.T) {
	out := aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 80, 24)
	plain := ansi.Strip(out)
	wantLine := "Catalog inspired by " + aboutCatalogCreditURL
	if !strings.Contains(plain, wantLine) {
		t.Fatalf("about view missing %q:\n%s", wantLine, plain)
	}
	wantLink := lipgloss.NewStyle().Hyperlink(aboutCatalogCreditURL).Render(aboutCatalogCreditURL)
	if !strings.Contains(out, wantLink) {
		t.Fatalf("about view missing catalog OSC 8 hyperlink:\n%s", out)
	}
}
```

Make `TestStartHasNoCatalogCreditRow` table-driven over the states the spec names, so the assertion is not width- or fixture-specific: catalog on with a bookmark (`twoSections()`), catalog on with no bookmark section, and `catalog off` (a lone `BOOKMARKS` section whose entry carries a borrowed catalog note — the state the old `hasCatalogRow` bookkeeping got wrong). Assert in every case that no item is non-selectable except a header and that the view omits `Catalog inspired by`.

Delete `TestStartCursorStopsBeforeCredit`, `TestStartCatalogCreditIsLinkedAndNonSelectable`, `TestStartCatalogCreditFitsNarrowListWidth`, and `TestStartCatalogCreditRequiresCatalogRow`. Remove the `si.credit` assertion from `TestStartFilterSelectsFirstMatchAfterHeadersDisappear`; Task 1 makes that field cease to exist.

Update all About tests from `stripANSIForLandingTest` to `ansi.Strip`: the former strips only SGR sequences and would swallow content after the new OSC 8 hyperlink. Add the `ansi` and `lipgloss` test imports, and remove the now-unused `lipgloss`/`ansi` imports from `start.go` and `start_test.go`; Task 2 adds both back for responsive rendering.

- [ ] **Step 2: Run the focused tests and verify failure**

Run:

```bash
go test ./tui -run 'Test(StartHasNoCatalogCreditRow|AboutViewRendersCatalogCreditHyperlink)$' -count=1 -v
```

Expected: compile failure for `aboutCatalogCreditURL` and/or a startpage assertion showing the trailing credit still exists.

- [ ] **Step 3: Remove the credit row and render the hyperlink in About**

Reduce `startItem` to entry/header data and simplify its item methods:

```go
type startItem struct {
	entry  startEntry
	header string
}

func (i startItem) selectable() bool {
	return i.header == "" && i.entry.target != ""
}

func (i startItem) Title() string {
	if i.header != "" {
		return i.header
	}
	return i.entry.target
}

func (i startItem) Description() string {
	if i.header != "" {
		return ""
	}
	return i.entry.note
}
```

Delete `hasCatalogRow`, the appended `credit` item, the credit render branch, and the credit-specific checks in `skipNonEntry`. Keep headers non-selectable. Move the URL constant and append the linked line to the About identity block:

```go
const aboutCatalogCreditURL = "https://640kb.neocities.org/fingerverse/"

catalogCredit := "Catalog inspired by " +
	lipgloss.NewStyle().Hyperlink(aboutCatalogCreditURL).Render(aboutCatalogCreditURL)
identity = append(identity, dim.Render(aboutRepo), dim.Render(catalogCredit))
```

Do not add a key action; `y` continues to copy only `aboutIssuesURL`.

Verified, so do not re-litigate it during implementation: the OSC 8 sequence survives `dim.Render` → `aboutView`'s per-line `ansi.Truncate` → `JoinVertical(Center, …)` → `lipgloss.Place` intact, at 80 columns and when truncated at 40 and 24 (the visible URL is clipped, the link target is not). The credit is the widest identity line at 59 columns, so it widens the centered identity block; re-run the existing About layout tests rather than assuming the centering is unchanged.

- [ ] **Step 4: Run the complete start/About tests**

Run:

```bash
go test ./tui -run 'Test(Start|About)' -count=1 -v
```

Expected: PASS, including narrow About truncation and startpage filtering.

- [ ] **Step 5: Commit the attribution move**

`feat`, not `refactor`: a row leaves the startpage and a line appears in About, which is user-visible. Release notes are generated by grouping Conventional Commit types, so `refactor` would file a visible change under internals.

```bash
git add tui/start.go tui/start_test.go tui/about.go tui/about_test.go
git commit -m "feat(tui): move catalog credit to about"
```

---

### Task 2: Add Section Identity and the Responsive Start Delegate

**Files:**
- Modify: `tui/sections.go:1-67`
- Modify: `tui/sections_test.go:1-78`
- Modify: `tui/start.go:14-252`
- Modify: `tui/start_test.go:1-303`

**Interfaces:**
- Consumes: `startEntry`, `entryKind`, `entrySource`, `list.Model.MatchesForItem`, existing palette and list item styles.
- Produces: `startSectionID`, `startSection.id`, `startItem.section`, `newStartDelegate(*commonModel, styles) startDelegate`, and a delegate with responsive `Height`, `Spacing`, `Update`, and `Render` methods.

- [ ] **Step 1: Write failing section-identity and responsive-layout tests**

Update `twoSections()` to include IDs, then add tests with these assertions:

```go
func TestBuildSectionsAssignsStableIDs(t *testing.T) {
	got := buildSections(catalogFixture(), bookmarkFile{targets: []string{"@tilde.team"}})
	if got[0].id != sectionBookmarks || got[1].id != sectionCommunities || got[2].id != sectionServices {
		t.Fatalf("section IDs = [%v %v %v]", got[0].id, got[1].id, got[2].id)
	}
}

func TestStartDelegateResponsiveHeight(t *testing.T) {
	common := testCommon()
	common.width = startWideMinWidth
	wide := newStart(common, twoSections(), "", "")
	if got := wide.list.Paginator.PerPage; got < 2 {
		t.Fatalf("wide PerPage = %d, want multiple one-row items", got)
	}
	if got := newStartDelegate(common, common.ensureStyles()).Height(); got != 1 {
		t.Fatalf("wide delegate height = %d, want 1", got)
	}

	common.width = startWideMinWidth - 1
	narrow := newStart(common, twoSections(), "", "")
	if got := newStartDelegate(common, common.ensureStyles()).Height(); got != 2 {
		t.Fatalf("narrow delegate height = %d, want 2", got)
	}
	if narrow.list.Paginator.PerPage >= wide.list.Paginator.PerPage {
		t.Fatalf("narrow PerPage = %d, wide = %d", narrow.list.Paginator.PerPage, wide.list.Paginator.PerPage)
	}
}

func TestStartWideRowKeepsLongestCatalogTarget(t *testing.T) {
	common := testCommon()
	common.width = 80
	sections := []startSection{{
		id: sectionServices, title: "SERVICES",
		entries: []startEntry{{
			target: "wordsearch:today@bbs.airandwave.net",
			note: "Daily word search puzzle", source: sourceCatalog,
		}},
	}}
	m := newStart(common, sections, "", "")
	line := lineContaining(t, stripANSIForLandingTest(m.View()), "wordsearch:today@bbs.airandwave.net")
	if !strings.Contains(line, "wordsearch:today@bbs.airandwave.net") {
		t.Fatalf("target truncated at 80 columns: %q", line)
	}
}
```

Also add table-driven delegate tests at widths 80, 72, 71, and 24 that assert every rendered line is at most `m.list.Width()`, wide rows place target and description on one physical line, narrow rows stack them, selected wide shelves span the full width with `SelectionBg`, and header rows consume the active uniform height.

Repair one existing test whose premise this task invalidates. `TestStartCursorSkipsHeaderAtPageBoundary` sets `common.height = 8` and comments "force pagination with the two-row delegate"; at `testCommon`'s width 80 the delegate is now one row, so all five fixture items fit a single page and the test stays green while covering nothing. Add `common.width = 40` to keep it in narrow mode, and update the comment to name the width as part of the premise.

- [ ] **Step 2: Run the focused tests and verify failure**

Run:

```bash
go test ./tui -run 'Test(BuildSectionsAssignsStableIDs|StartDelegateResponsiveHeight|StartWideRowKeepsLongestCatalogTarget|StartDelegateWidthVariants)$' -count=1 -v
```

Expected: compile failure for the new IDs/constants/delegate factory, followed by layout failures until the custom renderer replaces `userDelegate`.

- [ ] **Step 3: Attach section IDs during assembly**

Add the explicit IDs and carry them through flattening:

```go
type startSectionID uint8

const (
	sectionUnknown startSectionID = iota
	sectionBookmarks
	sectionCommunities
	sectionServices
)

type startSection struct {
	id      startSectionID
	title   string
	entries []startEntry
}

type startItem struct {
	entry   startEntry
	header  string
	section startSectionID
}
```

Assign `sectionBookmarks` to the bookmark group. Expand the catalog group table with an ID: communities and services receive their matching IDs; the retained dormant `PEOPLE`/`kindPerson` group receives `sectionUnknown`. In `newStart`, assign `s.id` to both the header and every entry item.

- [ ] **Step 4: Implement the responsive delegate and width helpers**

Replace the embedded `userDelegate` with a self-contained delegate:

```go
const (
	startChromeRows      = 1
	startWideMinWidth    = 72
	startTargetColumnPct = 50
)

type startDelegate struct {
	common *commonModel
	st     styles
}

func newStartDelegate(common *commonModel, st styles) startDelegate {
	return startDelegate{common: common, st: st}
}

func (d startDelegate) Height() int {
	if d.common.width >= startWideMinWidth {
		return 1
	}
	return 2
}

func (d startDelegate) Spacing() int { return 0 }

func (d startDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
```

For a header, render a bold `barFlag` label followed by a `palette.Rule` horizontal rule. Use one row in wide mode; use a blank first row plus the ruled label in narrow mode so every item consumes the delegate height.

For an entry, compute the frame before the columns:

```go
func startColumnWidths(width, frame int) (int, int) {
	available := width - frame
	if available < 0 {
		available = 0
	}
	target := available * startTargetColumnPct / 100
	return target, available - target
}

func splitStartMatches(matches []int, target string) (targetMatches, noteMatches []int) {
	noteOffset := len([]rune(target)) + 1
	for _, match := range matches {
		if match < noteOffset-1 {
			targetMatches = append(targetMatches, match)
		} else if match >= noteOffset {
			noteMatches = append(noteMatches, match-noteOffset)
		}
	}
	return targetMatches, noteMatches
}
```

Render target and note with independently truncated widths and pad the target field to its exact column width. Preserve Bubbles' matched-rune styling by splitting `m.MatchesForItem(index)` at the single space inserted by `FilterValue()`, styling matches with `st.listItem.FilterMatch`, and dropping match indexes beyond a truncated field. State behavior must be exact:

- `Filtering` with an empty input: use `DimmedTitle`/`DimmedDesc`, no shelf.
- `Filtering` with text: use normal title/description colors plus match styling, no shelf.
- selected and not `Filtering`: use the existing selected title/description colors and one full-width shelf; in wide mode compose both padded columns first and apply one left rail/background frame to that composed row.
- unselected: use normal title/description colors with the existing two-cell left inset.
- narrow mode: target on row one and note on row two; both lines truncate, never wrap, and a selected active row uses the full-width shelf on both physical lines.

Use `ansi.Truncate` with the single-rune `"…"` ellipsis, `lipgloss.Width`, and `renderSelectedShelfLine`; do not call `DefaultDelegate.Render` for start entries.

- [ ] **Step 5: Install the delegate consistently and verify tests**

Use `newStartDelegate(common, st)` in `list.New`, immediately after `applyListStyles`, in `startModel.applyStyles`, and from `setSize` before `list.SetSize` so crossing width 72 calls `SetDelegate` and recalculates `Paginator.PerPage`. `setSize` receives a width argument but the delegate reads `d.common.width`; assign `m.common.width` from the argument (or assert they agree) before constructing the delegate, so a caller that passes a width other than the shared one cannot leave the delegate a mode behind. Verified against `charm.land/bubbles/v2@v2.1.1`: `SetDelegate` calls `updatePagination()` itself and `PerPage` is `max(1, …)`, so no manual recalculation is needed and a zero width cannot divide by zero.

Run:

```bash
gofmt -w tui/start.go tui/start_test.go tui/sections.go tui/sections_test.go
go test ./tui -run 'Test(BuildSections|Start)' -count=1 -v
```

Expected: PASS; existing cursor/header/filter tests remain green with the custom renderer.

- [ ] **Step 6: Commit section identity and responsive rows**

```bash
git add tui/start.go tui/start_test.go tui/sections.go tui/sections_test.go
git commit -m "feat(tui): make startpage rows responsive"
```

---

### Task 3: Add the Fixed Overview and Shared Focus Truth

**Files:**
- Modify: `tui/start.go:60-252`
- Modify: `tui/start_test.go:1-303`
- Modify: `tui/app.go:55-70, 320-340, 484-505, 995-1010`
- Modify: `tui/app_test.go:2851-3185`
- Modify: `tui/request.go:72-90`
- Modify: `tui/request_test.go:1-620`

**Interfaces:**
- Consumes: `startItem.section`, `startWideMinWidth`, `startModel.selected`, existing palette, `commonModel` shared pointer.
- Produces: `commonModel.contentFocused`, `appModel.setInputFocused(bool)`, `startOverviewCounts`, `startModel.overviewHeight() int`, and `startModel.overviewView() string`.

- [ ] **Step 1: Write failing overview state tests**

Add fixtures containing bookmarks, communities, and services, then test the exact stripped copy:

```go
func threeSections() []startSection {
	return []startSection{
		{id: sectionBookmarks, title: "BOOKMARKS", entries: []startEntry{
			{target: "@tilde.team", source: sourceBookmark},
		}},
		{id: sectionCommunities, title: "COMMUNITIES", entries: []startEntry{
			{target: "@plan.cat", kind: kindCommunity, source: sourceCatalog},
			{target: "@happynetbox.com", kind: kindCommunity, source: sourceCatalog},
		}},
		{id: sectionServices, title: "SERVICES", entries: []startEntry{
			{target: "quake@bbs.airandwave.net", kind: kindService, source: sourceCatalog},
			{target: "dict@bbs.airandwave.net", kind: kindService, source: sourceCatalog},
		}},
	}
}

func TestStartOverviewWideCountsAssembledRows(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, threeSections(), "", "")
	got := stripANSIForLandingTest(m.overviewView())
	want := "BOOKMARKS  1 │ CATALOG  2 communities · 2 services"
	if got != want {
		t.Fatalf("overview = %q, want %q", got, want)
	}
}

func TestStartOverviewNarrowStacksOwnershipAndCatalog(t *testing.T) {
	common := testCommon()
	common.width = 40
	m := newStart(common, threeSections(), "", "")
	got := strings.Split(stripANSIForLandingTest(m.overviewView()), "\n")
	want := []string{"BOOKMARKS  1", "CATALOG    2 communities · 2 services"}
	if !slices.Equal(got, want) {
		t.Fatalf("overview = %#v, want %#v", got, want)
	}
}
```

Add cases for: no bookmarks (`BOOKMARKS  none yet` with no empty bookmark section), `catalog off` (only `BOOKMARKS`), file-level empty state (no overview), catalog rows pinned into bookmarks (counts move and never duplicate), singular `1 community`/`1 service`, and an applied filter whose visible items produce only the non-empty matching groups. Assert each overview line has `ansi.StringWidth(line) <= m.list.Width()`. In the applied-filter case, compare the sum of overview counts with the selectable count used by `appModel.startBar` and assert clearing the filter restores the assembled counts. Assert filtered bookmark rows contain no added `◆` or other ownership prefix.

- [ ] **Step 2: Run overview tests and verify failure**

Run:

```bash
go test ./tui -run 'TestStartOverview' -count=1 -v
```

Expected: compile failure because the overview model and renderer do not exist.

- [ ] **Step 3: Implement pure counts and copy composition**

Add the count value and helpers:

```go
type startOverviewCounts struct {
	bookmarks   int
	communities int
	services    int
}

func startCounts(items []list.Item) startOverviewCounts {
	var counts startOverviewCounts
	for _, item := range items {
		it, ok := item.(startItem)
		if !ok || !it.selectable() {
			continue
		}
		switch it.section {
		case sectionBookmarks:
			counts.bookmarks++
		case sectionCommunities:
			counts.communities++
		case sectionServices:
			counts.services++
		}
	}
	return counts
}

// A row in a section with no overview identity (today only the dormant
// PEOPLE/kindPerson group, which carries sectionUnknown) is counted nowhere. The
// shipped catalog contains no person entries, so the overview total equals
// startBar's visible count; adding one would silently break that invariant.
func startCountLabel(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}
```

Store `assembledCounts` on `startModel`. `overviewCounts` returns `startCounts(m.list.VisibleItems())` for `FilterApplied`, otherwise the assembled counts. In an applied filter, omit `BOOKMARKS` when its matching count is zero and omit each empty catalog classification. Unfiltered mode always renders `BOOKMARKS`, using `none yet` at zero; render `CATALOG` only when a catalog classification remains.

Decide the one-versus-two-line form from `m.common.width`, not `m.list.Width()`: `setSize` computes `overviewHeight()` *before* calling `list.SetSize`, so the list's own width is a frame stale there while `common.width` is already current. At `m.common.width >= startWideMinWidth`, join ownership and catalog groups with ` │ `. Below that, place ownership and catalog on separate rows; a lone group remains one row. Render base labels with `palette.Dim`, values with `palette.Text`, truncate every complete line with `ansi.Truncate(line, m.list.Width(), "…")`, and never pad beyond the model width. Truncation width may lag by a frame; the line *count* may not, which is why the mode decision reads `common.width`.

`overviewView` itself returns `""` while `Filtering` or when the assembled item list is empty. This must match `overviewHeight`; hiding only the height contribution while still composing overview text would overflow the body.

- [ ] **Step 4: Centralize focus truth and add overview highlighting**

Add `contentFocused bool` to `commonModel` and centralize production assignments:

```go
func (m *appModel) setInputFocused(focused bool) {
	m.inputFocused = focused
	m.common.contentFocused = !focused
}
```

Use it from `focusInput`, `blurInput`, `gotoStart`'s empty fallback, `showRouted`, and `cancelRequest`. Initialize both `inputFocused: true` and `common.contentFocused: false`. Preserve existing input `Focus`, `Blur`, cursor, resize, and request-failure behavior around those assignments.

When `common.contentFocused` is true, read the selected `startItem.section` and render exactly one overview value segment with `Foreground(AccentGold).Bold(true)`: `BOOKMARKS`, the complete `N community/communities` value, or the complete `N service/services` value. When input focus is active, render no gold/bold segment. Do not add background, border, or underline.

Add an app test that drives `↓` and Esc through the public `Update` path and asserts `inputFocused` is always the inverse of `common.contentFocused`. Add a request test that cancels a request whose `returnToInput` is true and asserts both fields return to their input-focused values.

Add a section-focus table that selects bookmark, community, and service entries and asserts only the corresponding rendered overview segment carries `AccentGold` plus bold. Repeat one case after selecting an entry on page 2, proving the fixed overview highlight survives after the inline section header scrolls away. Repeat it once more with a filter applied and a bookmark row selected among flat filtered results, where the inline header does not exist at all and the segment is the only remaining ownership signal — this is the case the spec accepts a per-row marker's absence for, so it has to be pinned. With input focus active, assert the overview contains neither the gold foreground sequence nor bold on any segment.

- [ ] **Step 5: Compose the overview above the list and size for it**

Implement:

```go
func (m startModel) overviewHeight() int {
	view := m.overviewView()
	if view == "" {
		return 0
	}
	return len(strings.Split(view, "\n"))
}
```

Derive the height from the composed string, not from the two state predicates: `strings.Split("", "\n")` has length 1, so a state-only guard claims a row the view never draws. The overview also composes to `""` under an applied filter whose every group is empty — reachable transiently in Task 5 between `SetFilterText` and its `ResetFilter` fallback — and that path is not covered by the `Filtering`/empty-items checks.

In `startModel.View`, retain the file-level empty-message gate, and keep the notice above everything: the overview belongs inside `body`, so the composed order stays notice → blank line → overview → list. Prepend `overviewView()` plus one newline when the overview is non-empty — one newline, not the notice's `"\n\n"`, so the overview sits tight against the list it summarises while `noticeHeight()`'s existing `+1` keeps accounting for the notice's blank separator. In `setSize`, subtract `overviewHeight()` in addition to `startChromeRows` and `noticeHeight()` before `list.SetSize`.

Add `TestStartSizingIncludesNoticeAndOverview`: construct a narrow `threeSections()` model with a two-line notice and assert `m.list.Height()` equals the supplied body height minus `startChromeRows`, `noticeHeight()`, and `overviewHeight()` exactly.

- [ ] **Step 6: Verify overview, focus, and sizing tests**

Run:

```bash
gofmt -w tui/start.go tui/start_test.go tui/app.go tui/app_test.go tui/request.go tui/request_test.go
go test ./tui -run 'Test(StartOverview|StartpageArrowDown|StartpageEsc|CancelRequest)' -count=1 -v
```

Expected: PASS, with no `PEOPLE` overview case and no synthetic zero-count overview.

- [ ] **Step 7: Commit the fixed overview**

```bash
git add tui/start.go tui/start_test.go tui/app.go tui/app_test.go tui/request.go tui/request_test.go
git commit -m "feat(tui): add a fixed startpage overview"
```

---

### Task 4: Make Focus Styling and Filter Relayout State-Safe

**Files:**
- Modify: `tui/start.go:110-252`
- Modify: `tui/start_test.go:1-303`
- Modify: `tui/app_test.go:1505-1528, 3113-3148`

**Interfaces:**
- Consumes: `commonModel.contentFocused`, `startModel.overviewHeight`, responsive `startDelegate`, Bubbles `FilterState`.
- Produces: inactive start selection styling and automatic relayout on every filter-state transition.

- [ ] **Step 1: Write failing focus and filter-lifecycle tests**

Add tests that distinguish active and inactive selection without relying on text alone:

```go
func TestStartSelectionShelfFollowsContentFocus(t *testing.T) {
	common := testCommon()
	common.width = 80
	m := newStart(common, twoSections(), "", "")

	common.contentFocused = true
	active := lineContaining(t, m.View(), "@tilde.team")
	assertFullWidthStyledLine(t, "active start selection", active, m.list.Width(), common.styles.palette.SelectionBg)

	common.contentFocused = false
	inactive := lineContaining(t, m.View(), "@tilde.team")
	assertFullWidthStyledLine(t, "inactive start selection", inactive, m.list.Width(), common.styles.palette.SubtleBg)
}
```

Add a lifecycle table that starts at narrow width and records `m.list.Height()`:

```go
func TestStartFilterTransitionsReclaimOverviewHeight(t *testing.T) {
	common := testCommon()
	common.width = 40
	m := newStart(common, threeSections(), "", "")
	unfilteredHeight := m.list.Height()

	m, _ = m.update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if got, want := m.list.Height(), unfilteredHeight+2; got != want {
		t.Fatalf("filtering list height = %d, want %d", got, want)
	}

	m = typeStartFilter(t, m, "plan")
	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.list.FilterState() != list.FilterApplied || m.list.Height() != unfilteredHeight {
		t.Fatalf("applied state=%v height=%d, want applied/%d", m.list.FilterState(), m.list.Height(), unfilteredHeight)
	}
}
```

Define `typeStartFilter` in `start_test.go` to send each rune through `startModel.update`, execute the returned filter command with the existing `findFilterMatches`, and feed that message back into `update`. Cover `/` entry, Enter apply, Esc cancel, Esc clear from `FilterApplied`, and Enter on a zero-match live filter. The zero-match case must end `Unfiltered`, restore the full overview/list, and leave a selectable row. Add a resize test that applies a filter, crosses 72 in both directions, and asserts filter text, filtered counts, selection, delegate height, and paginator capacity survive.

- [ ] **Step 2: Run the focused tests and verify failure**

Run:

```bash
go test ./tui -run 'Test(StartSelectionShelfFollowsContentFocus|StartFilterTransitionsReclaimOverviewHeight|StartAppliedFilterSurvivesResponsiveResize)$' -count=1 -v
```

Expected: the active shelf remains visible with input focus and list height changes only after terminal resize.

- [ ] **Step 3: Render a neutral inactive selection**

Have `startDelegate.Render` choose these exact states:

- active selection (`contentFocused && state != Filtering`): current violet rail, `SelectionBg`, `SelectionLogin`, and `SelectionDesc`;
- inactive logical selection (`!contentFocused && state != Filtering`): `Rule` rail, `SubtleBg`, normal `Text` target, and `Dim` description;
- live filtering: no active or inactive shelf, preserving Bubbles' filter-editing hierarchy.

Construct the inactive frame from existing palette values:

```go
inactiveShelf := lipgloss.NewStyle().
	Border(lipgloss.NormalBorder(), false, false, false, true).
	BorderForeground(d.st.palette.Rule).
	Background(d.st.palette.SubtleBg).
	Padding(0, 0, 0, 1)
```

Apply it to the same composed wide row or both stacked narrow rows as the active shelf. Because the delegate reads the shared `commonModel` pointer, `applyListStyles` followed by `SetDelegate(newStartDelegate(m.common, st))` cannot reset focus state during a background/theme change.

- [ ] **Step 4: Relayout after Bubbles changes filter state**

Update `startModel.update` around the existing list delegation:

```go
beforeState := m.list.FilterState()
beforeIndex := m.list.Index()
m.list, cmd = m.list.Update(msg)
if m.list.FilterState() != beforeState {
	m.setSize(m.common.width, m.common.bodyHeight())
}
```

Keep the existing `FilterMatchesMsg` first-match reset and `skipNonEntry` direction logic after the relayout. Do not force a state on Enter: Bubbles must clear a zero-match filter itself. Calling `setSize` after `list.Update` ensures the final filter state controls `overviewHeight` and `Paginator.PerPage` for that frame.

- [ ] **Step 5: Verify focus, filtering, theme, and pagination**

Run:

```bash
gofmt -w tui/start.go tui/start_test.go tui/app_test.go
go test ./tui -run 'Test(StartSelection|StartFilter|StartAppliedFilter|StartApplyStyles|BackgroundColorMsgRestylesTUI)' -count=1 -v
```

Expected: PASS; zero-match behavior remains native and theme changes preserve inactive focus styling.

- [ ] **Step 6: Commit focus and filter relayout**

```bash
git add tui/start.go tui/start_test.go tui/app_test.go
git commit -m "fix(tui): keep startpage focus and filtering aligned"
```

---

### Task 5: Restore Bookmark Toggles by Section and Preserve Applied Filters

**Files:**
- Modify: `tui/start.go:215-245`
- Modify: `tui/start_test.go:1-303`
- Modify: `tui/app.go:340-405, 1190-1260, 1469-1490`
- Modify: `tui/app_test.go:2908-2996, 3113-3148`
- Modify: `tui/statusbar_test.go:32-56`

**Interfaces:**
- Consumes: `startItem.section`, `list.Model.Items`, `VisibleItems`, `FilterValue`, `SetFilterText`, `ResetFilter`, existing `reloadStart` and bookmark persistence.
- Produces: `startSectionPosition`, `startTogglePosition`, `startModel.captureTogglePosition()`, `startModel.selectSectionPosition(startSectionPosition) bool`, and `appModel.startBookmarkAction() string`.

- [ ] **Step 1: Replace identity-first tests with failing section-first tests**

Replace `TestBookmarkOnStartpageTogglesFile` with separate add/remove cases. Pin exact outcomes:

```go
func TestBookmarkingCatalogRowStaysAtSectionOrdinal(t *testing.T) {
	path := seedBookmarks(t, "@tilde.team\n")
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	if !m.start.selectTarget("@plan.cat") {
		t.Fatal("@plan.cat not found")
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	selected, ok := m.start.selected()
	if !ok || selected.target != "@happynetbox.com" {
		t.Fatalf("selected = %+v, %v; want next community at the same ordinal", selected, ok)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "@plan.cat\n") {
		t.Fatalf("bookmark file = %q, err=%v", data, err)
	}
}

func TestRemovingMiddleBookmarkStaysAtBookmarkOrdinal(t *testing.T) {
	seedBookmarks(t, "@plan.cat\n@tilde.team\n@happynetbox.com\n")
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	m.start.selectTarget("@tilde.team")
	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	selected, ok := m.start.selected()
	if !ok || selected.target != "@happynetbox.com" {
		t.Fatalf("selected = %+v, %v; want bookmark moved into the removed row's slot", selected, ok)
	}
}
```

Define `seedBookmarks(t, data) string` in `tui/app_test.go` near these tests: call `useTempBookmarks(t)`, create its parent with mode `0700`, write exactly `data` with mode `0600`, and return the path. Add app tests for removing first/final/only bookmarks, adding first/middle/final catalog rows, an emptied final catalog section falling backward, and a completely empty `catalog off` page focusing input.

- [ ] **Step 2: Add failing applied-filter toggle tests**

Cover all three post-toggle filter outcomes:

**Bubbles orders filtered items by fuzzy rank, not catalog order.** Against the shipped catalog, `SetFilterText("typed-hole")` yields `[cyoa@typed-hole.org, smog@typed-hole.org, textfile@typed-hole.org]` — verified by running it, not read off `tui/catalog.txt`. So the acted-on row is `cyoa` at filtered ordinal 0, and bookmarking it leaves `[smog, textfile]`, selecting `smog@typed-hole.org`. Do not hardcode those three targets: derive the expectation from the visible order so a catalog edit or a bubbles ranking change fails loudly at the precondition instead of silently testing the wrong ordinal.

Define `visibleTargets` once, in `tui/app_test.go` beside these tests (the package is shared, so do not redeclare it in `start_test.go`):

```go
// visibleTargets returns the selectable startpage targets in list order,
// which under an applied filter is bubbles' fuzzy rank order.
func visibleTargets(m startModel) []string {
	var out []string
	for _, it := range m.list.VisibleItems() {
		if si, ok := it.(startItem); ok && si.selectable() {
			out = append(out, si.entry.target)
		}
	}
	return out
}

func TestFilteredBookmarkTogglePreservesFilterAndOrdinal(t *testing.T) {
	useTempBookmarks(t)
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	m.start.list.SetFilterText("typed-hole")
	matches := visibleTargets(m.start)
	if len(matches) < 3 {
		t.Fatalf("precondition: filter matched %v, want at least 3 services", matches)
	}
	before, ok := m.start.selected()
	if !ok || before.target != matches[0] {
		t.Fatalf("precondition selected = %+v, %v; want first match %q", before, ok, matches[0])
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	selected, ok := m.start.selected()
	if m.start.list.FilterState() != list.FilterApplied || m.start.list.FilterValue() != "typed-hole" {
		t.Fatalf("filter state=%v value=%q", m.start.list.FilterState(), m.start.list.FilterValue())
	}
	// The pinned row leaves SERVICES for BOOKMARKS; ordinal 0 of the remaining
	// filtered services is what was matches[1].
	if !ok || selected.target != matches[1] {
		t.Fatalf("selected = %+v, %v; want next filtered service %q", selected, ok, matches[1])
	}
	if selected.source == sourceBookmark {
		t.Fatalf("selection followed the pinned target into BOOKMARKS: %+v", selected)
	}
}

func TestFilteredToggleClearsFilterWhenFinalMatchDisappears(t *testing.T) {
	seedBookmarks(t, "alice@plan.cat\n")
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	m.start.list.SetFilterText("alice@plan.cat")
	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	if m.start.list.FilterState() != list.Unfiltered || m.start.list.FilterValue() != "" {
		t.Fatalf("filter state=%v value=%q, want cleared", m.start.list.FilterState(), m.start.list.FilterValue())
	}
	selected, ok := m.start.selected()
	if !ok || selected.kind != kindCommunity {
		t.Fatalf("selected = %+v, %v; want first unfiltered catalog section", selected, ok)
	}
}
```

Add `TestFilteredRemovalFallsToNextMatchingSection`: seed the unmatched bookmark `alice@plan.cat`, apply `plan`, remove it, and assert the still-applied filter selects catalog community `@plan.cat`. Add `TestFilteredPinFallsToPreviousMatchingSection`: with no bookmarks, apply `quake@`, bookmark `quake@bbs.airandwave.net`, and assert the still-applied filter falls backward to the moved bookmark because no service match remains. Add one `catalog off` case where removing the final row clears the filter and focuses the input.

The three fixed filter strings above were checked against the shipped catalog rather than assumed: `quake@` matches exactly one row (so pinning it empties SERVICES and forces the backward fallback); `plan` ranks community `@plan.cat` first among eleven matches (so the forward fallback lands there); and `alice@plan.cat` matches the seeded bookmark only — it is *not* a fuzzy subsequence of `@plan.cat Classic finger, polished for the present`, which is what makes `TestFilteredToggleClearsFilterWhenFinalMatchDisappears` reach the zero-match clear path. If a later catalog edit changes any of these, the preconditions must be re-derived, not patched.

- [ ] **Step 3: Run bookmark tests and verify failure**

Run:

```bash
go test ./tui -run 'Test(BookmarkingCatalogRowStaysAtSectionOrdinal|RemovingMiddleBookmarkStaysAtBookmarkOrdinal|FilteredBookmarkTogglePreservesFilterAndOrdinal|FilteredToggleClearsFilterWhenFinalMatchDisappears)' -count=1 -v
```

Expected: current identity restoration follows the moved target and `reloadStart` drops applied filter state.

- [ ] **Step 4: Implement position capture and section fallback**

Add these values:

```go
type startSectionPosition struct {
	section startSectionID
	ordinal int
}

type startTogglePosition struct {
	full     startSectionPosition
	filtered *startSectionPosition
	filter   string
}

var startSectionOrder = [...]startSectionID{
	sectionBookmarks,
	sectionCommunities,
	sectionServices,
}
```

`captureTogglePosition` must:

1. Read the selected `startItem` and its section.
2. Find the selected target in `m.list.Items()` and count earlier selectable items with the same section for `full.ordinal`.
3. If `FilterApplied`, count earlier entries in `VisibleItems()` with the same section for `filtered.ordinal`, and copy `m.list.FilterValue()`.
4. Return false only when there is no selected entry.

`selectSectionPosition` must scan `VisibleItems()`, collect indexes for each ID in `startSectionOrder`, choose the requested section when non-empty, otherwise the first non-empty later section, otherwise the nearest non-empty earlier section. Select `min(position.ordinal, len(indexes)-1)`. Return false when no selectable row exists. Never select a header or infer identity from its title.

- [ ] **Step 5: Restore section/filter state around `reloadStart`**

Before target validation and the existing file mutation, capture only when the action comes from the startpage:

```go
var position startTogglePosition
hasPosition := false
if m.state == stateStart {
	position, hasPosition = m.start.captureTogglePosition()
}
```

Keep the existing validation, parse, add/delete, atomic save, and flash-message selection in their current order. After `saveBookmarkData` succeeds, replace the old startpage `selectTarget(target)` block with:

```go
if m.state == stateStart {
	m.reloadStart()
	restored := false
	if hasPosition && position.filtered != nil {
		m.start.list.SetFilterText(position.filter)
		if len(m.start.list.VisibleItems()) > 0 {
			restored = m.start.selectSectionPosition(*position.filtered)
		} else {
			m.start.list.ResetFilter()
			restored = m.start.selectSectionPosition(position.full)
		}
	} else if hasPosition {
		restored = m.start.selectSectionPosition(position.full)
	}
	m.resize()
	if !restored {
		return tea.Batch(m.focusInput(), m.setFlash(msg))
	}
}
```

Capturing before the persistence path ensures the old assembled and filtered ordinals still exist. `SetFilterText` is synchronous in Bubbles v2.1.1 — verified in `list.go`: it runs the filter command inline, assigns `filteredItems`, and lands on `FilterApplied`, so no `FilterMatchesMsg` round-trip is needed here.

This was `toggleBookmark`'s only production use of `startModel.selectTarget`. Keep the method: it stays exercised by `TestStartSelectTargetPreservesIdentity` and is the natural helper for identity-first restoration elsewhere, but add a doc comment saying startpage bookmark toggles deliberately no longer use it, so the next reader does not "restore" the behavior this task removes. Immediately call `ResetFilter` when it produces zero visible items so lookit never leaves the programmatic applied-zero state visible.

- [ ] **Step 6: Make bookmark status/help copy contextual**

Define one copy source and use it from both `updateKeymap` and `startBar`:

```go
func (m appModel) startBookmarkAction() string {
	entry, ok := m.start.selected()
	if ok && entry.source == sourceBookmark {
		return "remove"
	}
	return "bookmark"
}
```

In `updateKeymap`, set the help text on **both** branches — `SetHelp` mutates persistent binding state, so an unbranched call leaves `b remove` in the reader and user-list help panels after the user navigates off a bookmarked startpage row:

```go
if m.state == stateStart {
	m.keys.Bookmark.SetHelp("b", m.startBookmarkAction())
} else {
	m.keys.Bookmark.SetHelp("b", "bookmark")
}
```

`updateKeymap` runs from both `Update` and `View`, so the help panel and status bar always render the current row's verb. Build the start bar from the same helper:

```go
bar.hints = fmt.Sprintf("↵ go · b %s · / filter · i target · ? help", m.startBookmarkAction())
```

Add status tests for both catalog and bookmark rows, including a selected bookmark in flat filtered results.

- [ ] **Step 7: Run focused and full verification**

Run:

```bash
gofmt -w tui/start.go tui/start_test.go tui/app.go tui/app_test.go tui/statusbar_test.go
go test ./tui -run 'Test(Bookmark|Removing|Filtered|StatusBarStart|Startpage)' -count=1 -v
make check
```

Expected: all focused tests pass; `make check` exits 0 after vet, formatting, lint, and race tests.

- [ ] **Step 8: Review scope and commit the completed interaction polish**

Run:

```bash
git diff --check
git status --short
git diff --stat
```

Confirm the diff changes only the files listed in this plan, does not touch `tui/catalog.txt` or bookmark persistence code, and adds no dependency. Then commit:

```bash
git add tui/start.go tui/start_test.go tui/sections.go tui/sections_test.go tui/about.go tui/about_test.go tui/app.go tui/app_test.go tui/request.go tui/request_test.go tui/statusbar_test.go
git commit -m "feat(tui): stabilize startpage bookmark toggles"
```

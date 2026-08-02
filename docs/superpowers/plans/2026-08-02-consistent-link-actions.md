# Consistent Link Actions Implementation Plan

> Execute this plan task-by-task with test-driven development and a review checkpoint after every commit.

**Goal:** Make Enter mean “go”, `y` mean “copy”, and `f` explicitly follow an ambiguous finger address across the reader and links panel, with honest status and help copy.

**Architecture:** Add one pure link-action policy in `tui/linkactions.go` and make reader dispatch, panel dispatch, key enablement, status hints, and contextual help consume it. Keep detection, parsing, OSC-8 emission, history, and networking unchanged; `appModel` remains the owner of routing and screen-level chrome.

**Tech Stack:** Go 1.26 toolchain, Bubble Tea v2, Bubbles v2 `key`/`list`, Lip Gloss v2, standard `testing` package.

## Global Constraints

- Enter navigates inside lookit; it never copies as a fallback.
- `y` copies the exact displayed `Link.Raw` for a focused or selected link.
- `f` acts only on an ambiguous bare `user@host` link.
- All user-visible action copy says **go**, never **drill**.
- Use `tab next`, never `⇥ next`.
- Do not advertise terminal-native hyperlink gestures in lookit's UI.
- Preserve OSC-8 emission, detection/classification, port-79 pinning, forwarding safety, history, and raw view.
- About remains non-focusable; Tab does nothing there.
- Tests remain offline and use injected fetch/clipboard seams.
- Commit messages use Conventional Commits with no attribution trailers.

## File Structure

- Create `tui/linkactions.go`: pure action-policy types and `actionsForLink`.
- Create `tui/linkactions_test.go`: table tests for the policy matrix.
- Modify `tui/app.go`: routing, key enablement, panel filtering, status, and help groups.
- Modify `tui/app_test.go`: dispatch, filtering, status, help, and About integration tests.
- Modify `tui/linkspanel.go`: expose Bubbles filter state/value through query methods.
- Modify `tui/keys.go` and `tui/keys_test.go`: visible link-help labels.
- Modify `tui/about.go` and `tui/about_test.go`: concise About action copy.

---

### Task 1: Centralize the link action policy

**Files:**
- Create: `tui/linkactions.go`
- Create: `tui/linkactions_test.go`

**Interfaces:**
- Consumes: `Link`, `LinkFinger`, `ActionDrill`, `Blocked`, `Ambiguous`, and `Target.HostPort`.
- Produces: `linkEnterAction`, `linkEnterNone`, `linkEnterGo`, `linkEnterRefuse`, `linkActions`, and `actionsForLink(Link) linkActions`.

- [ ] **Step 1: Write the failing policy test**

Create `tui/linkactions_test.go` with this matrix:

```go
package tui

import (
	"testing"

	"github.com/jonathandeamer/lookit/finger"
)

func TestActionsForLink(t *testing.T) {
	definite := Link{Kind: LinkFinger, Action: ActionDrill, Target: finger.Target{HostPort: "tilde.team:79"}}
	ambiguous := Link{Kind: LinkFinger, Action: ActionCopy, Ambiguous: true, Target: finger.Target{HostPort: "tilde.team:79"}}
	blocked := Link{Kind: LinkFinger, Action: ActionCopy, Blocked: "cross-relay"}
	tests := []struct {
		name string
		link Link
		want linkActions
	}{
		{"definite", definite, linkActions{enter: linkEnterGo, copy: true}},
		{"ambiguous", ambiguous, linkActions{finger: true, copy: true}},
		{"url", Link{Kind: LinkURL, Action: ActionCopy}, linkActions{copy: true}},
		{"email", Link{Kind: LinkEmail, Action: ActionCopy}, linkActions{copy: true}},
		{"social", Link{Kind: LinkSocial, Action: ActionCopy}, linkActions{copy: true}},
		{"blocked", blocked, linkActions{enter: linkEnterRefuse, copy: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := actionsForLink(tt.link); got != tt.want {
				t.Fatalf("actionsForLink = %+v, want %+v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./tui/ -run TestActionsForLink -count=1 -v`

Expected: compilation fails because the policy types and function do not exist.

- [ ] **Step 3: Implement the minimal policy**

Create `tui/linkactions.go`:

```go
package tui

type linkEnterAction uint8

const (
	linkEnterNone linkEnterAction = iota
	linkEnterGo
	linkEnterRefuse
)

type linkActions struct {
	enter  linkEnterAction
	finger bool
	copy   bool
}

func actionsForLink(link Link) linkActions {
	actions := linkActions{copy: true}
	if link.Blocked != "" {
		actions.enter = linkEnterRefuse
		return actions
	}
	if link.Kind != LinkFinger || link.Target.HostPort == "" {
		return actions
	}
	if link.Ambiguous {
		actions.finger = true
		return actions
	}
	if link.Action == ActionDrill {
		actions.enter = linkEnterGo
	}
	return actions
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./tui/ -run TestActionsForLink -count=1 -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tui/linkactions.go tui/linkactions_test.go
git commit -m "refactor(tui): centralize link action policy"
```

### Task 2: Apply semantic actions in the reader

**Files:**
- Modify: `tui/app.go:621-666,756-799,842-883`
- Modify: `tui/app_test.go`

**Interfaces:**
- Consumes: `actionsForLink` from Task 1.
- Produces: `focusedReaderLink() (Link, bool)` and policy-driven reader Enter/`f` routing and enablement.

- [ ] **Step 1: Add a focused-reader test fixture**

Add near the existing `app_test.go` helpers:

```go
func readerWithFocusedLink(t *testing.T, fetch FetchFunc, link Link) appModel {
	t.Helper()
	m := newApp(fetch, colorprofile.NoTTY)
	target := hostTarget(t, "viewer@origin.example")
	entry := Entry{Target: target, Body: []byte(link.Raw + "\n")}
	m.history = []histNode{{entry: entry, state: stateReader, links: []Link{link}, linkIdx: 0}}
	m.pos, m.state, m.inputFocused = 0, stateReader, false
	m.reader.focusedLink = 0
	m.reader.setEntryWithLinks(entry, []Link{link})
	return m
}
```

- [ ] **Step 2: Write failing reader integration tests**

Add tests with these exact assertions:

- Definite finger + Enter: starts loading, returns a fetch command, and records `Target.Raw`.
- URL + Enter: returns no command, does not load, and does not call `setClipboard`.
- Blocked + Enter: sets `m.flash` to `Link.Blocked` and leaves loading false.
- Ambiguous + `f`: starts a fetch.
- Definite + `f`: returns no command and does not load.
- Focused URL + `y`: copies its `Raw`, not the current page target.

Use `fetchRecorder`, `stubFetch`, `runCmds`, and the existing `setClipboard` seam.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./tui/ -run 'TestReader(Enter|F|YCopiesFocused)' -count=1 -v`

Expected: definite Enter is disabled; blocked Enter does not flash; definite `f` still fetches.

- [ ] **Step 4: Add the focused-link query**

Add near `activateFocusedLink`:

```go
func (m appModel) focusedReaderLink() (Link, bool) {
	if m.state != stateReader || m.pos < 0 {
		return Link{}, false
	}
	node := m.history[m.pos]
	if m.reader.focusedLink < 0 || m.reader.focusedLink >= len(node.links) {
		return Link{}, false
	}
	return node.links[m.reader.focusedLink], true
}
```

- [ ] **Step 5: Dispatch Enter and `f` through the policy**

Replace `activateFocusedLink` with:

```go
func (m appModel) activateFocusedLink(node *histNode) (bool, appModel, tea.Cmd) {
	link := node.links[m.reader.focusedLink]
	switch actionsForLink(link).enter {
	case linkEnterGo:
		return true, m, m.startFetch(link.Target)
	case linkEnterRefuse:
		return true, m, m.setFlash(link.Blocked)
	default:
		return true, m, nil
	}
}
```

Require `actionsForLink(link).finger` in the reader `f` case.

- [ ] **Step 6: Make reader key enablement focus-aware**

In `updateKeymap`, compute whether the current reader node has links. Enable `LinkNext`, `LinkPrev`, and reader-mode `LinkPanel` only when it does. If `focusedReaderLink` succeeds, enable `Open` when `enter != linkEnterNone` and `LinkFinger` when `finger`; otherwise disable both reader link actions. Preserve input/list/About enablement.

- [ ] **Step 7: Run focused reader and keymap tests**

Run: `go test ./tui/ -run 'TestReader(Enter|F|YCopiesFocused)|TestUpdateKeymapGatesByState' -count=1 -v`

Expected: PASS. Extend `TestUpdateKeymapGatesByState` to cover definite, ambiguous, URL, and blocked focused links.

- [ ] **Step 8: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "fix(tui): make reader link actions semantic"
```

### Task 3: Align links-panel actions and filtering

**Files:**
- Modify: `tui/linkspanel.go`
- Modify: `tui/app.go:543-599,866-874`
- Modify: `tui/app_test.go`

**Interfaces:**
- Consumes: `actionsForLink` and Bubbles `list.FilterState`.
- Produces: `linksPanel.filtering`, `filterApplied`, `filterValue`, and policy-driven panel actions.

- [ ] **Step 1: Write failing panel action tests**

Add a `linksPanelModel(t, fetch, links)` fixture that creates a reader history node, sets `showingLinks`, initializes `newLinksPanel`, and selects the first link. Test:

1. Definite + Enter closes the panel and fetches.
2. URL + Enter stays open and does not copy.
3. Blocked + Enter stays open and flashes the refusal.
4. Ambiguous + `f` closes and fetches.
5. Selected row + `y` copies `Raw`, flashes, and stays open.

- [ ] **Step 2: Write failing filter lifecycle tests**

Drive the real Bubbles list with `tea.KeyPressMsg`:

- `/`, then printable `y`, `f`, and `L` update `FilterValue` and never trigger panel actions.
- Enter/Tab with a non-empty matching filter produces `list.FilterApplied`.
- Esc while `list.Filtering` cancels without closing.
- Esc while `list.FilterApplied` clears without closing; the next Esc closes.
- `Ctrl+C` still returns a quit command.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./tui/ -run TestLinksPanel -count=1 -v`

Expected: URL Enter copies/closes, panel `y` is unhandled, and app actions intercept filtering.

- [ ] **Step 4: Expose panel filter queries**

Add to `tui/linkspanel.go`:

```go
func (p linksPanel) filtering() bool { return p.list.FilterState() == list.Filtering }
func (p linksPanel) filterApplied() bool { return p.list.FilterState() == list.FilterApplied }
func (p linksPanel) filterValue() string { return p.list.FilterValue() }
```

- [ ] **Step 5: Guard filter states before panel actions**

In `handleKey`, after force-quit/help/About/input handling but before the normal panel switch:

```go
if m.showingLinks && m.linksPanel.filtering() {
	var cmd tea.Cmd
	m.linksPanel, cmd = m.linksPanel.update(msg)
	return true, m, cmd
}
if m.showingLinks && m.linksPanel.filterApplied() && key.Matches(msg, m.keys.Back) {
	var cmd tea.Cmd
	m.linksPanel, cmd = m.linksPanel.update(msg)
	return true, m, cmd
}
```

Return `handled=true`: ordinary `Update` delegation targets `readerModel`, not the overlaying panel.

- [ ] **Step 6: Route panel actions through the policy**

For the selected link:

- Enter `go`: close and fetch.
- Enter `refuse`: stay open and flash `Blocked`.
- Enter `none`: stay open, no command.
- `f`: require `.finger`, then close and fetch.
- `y`: require `.copy`, copy `Raw`, flash, and stay open.
- Esc/`L`: preserve close-and-focus behavior.

- [ ] **Step 7: Make panel bindings selection-aware**

When `showingLinks`, enable Back and `LinkPanel`; enable Filter when not actively filtering; initialize Open and `LinkFinger` disabled, then set them from the selected link's policy. Copy remains enabled through the existing content/result rule.

- [ ] **Step 8: Run panel and reader regression tests**

Run: `go test ./tui/ -run 'TestLinksPanel|TestReader(Enter|F)|TestEnterInListDrills' -count=1 -v`

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add tui/linkspanel.go tui/app.go tui/app_test.go
git commit -m "fix(tui): align links panel actions and filtering"
```

### Task 4: Make reader and panel status bars honest

**Files:**
- Modify: `tui/linkactions.go`
- Modify: `tui/linkactions_test.go`
- Modify: `tui/app.go:756-799,914-1017`
- Modify: `tui/app_test.go`

**Interfaces:**
- Consumes: action/filter interfaces from Tasks 1 and 3.
- Produces: `linkActionHints(Link) []string` and a panel branch in `buildStatusBar`.

- [ ] **Step 1: Write failing pure hint tests**

Test these exact results using `slices.Equal`:

```go
definite  -> []string{"↵ go", "y copy"}
ambiguous -> []string{"f go", "y copy"}
URL       -> []string{"y copy"}
blocked   -> []string{"y copy"}
```

- [ ] **Step 2: Write failing status integration tests**

Assert:

- Reader definite: `finger · ↵ go · y copy · tab next`.
- Reader ambiguous: `address (ambiguous) · f go · y copy · tab next`.
- Reader URL: `url · y copy · tab next`, without `↵`.
- Reader output omits `drill`, `⇥ next`, and `⌘-click opens`.
- Unfiltered panel: `↑/↓ move · / filter · esc back` plus selected actions.
- Empty/non-empty active filters: `type to filter · esc cancel` / `enter apply · esc cancel`.
- Applied filter: `↑/↓ move · esc clear filter` plus selected actions.
- Copy/refusal flash overrides panel resting hints.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./tui/ -run 'Test(LinkActionHints|ReaderFocusedLinkStatus|LinksPanelStatus)' -count=1 -v`

Expected: missing helper/panel branch and old visible copy.

- [ ] **Step 4: Implement shared action hints**

Add to `tui/linkactions.go`:

```go
func linkActionHints(link Link) []string {
	actions := actionsForLink(link)
	var hints []string
	if actions.enter == linkEnterGo { hints = append(hints, "↵ go") }
	if actions.finger { hints = append(hints, "f go") }
	if actions.copy { hints = append(hints, "y copy") }
	return hints
}
```

- [ ] **Step 5: Rewrite focused-reader hints**

Build the action portion from `linkActionHints`, append `Blocked` when non-empty, then append `tab next`. Rename `address (auto)` to `address (ambiguous)`. Delete only the status-hint use of `isOSC8Openable`; keep OSC-8 wrapping unchanged.

- [ ] **Step 6: Add the filter-aware panel status branch**

Before the reader/list state switch, return a panel bar with:

```go
parts := []string{}
switch {
case m.linksPanel.filtering() && m.linksPanel.filterValue() == "":
	bar.hints = "type to filter · esc cancel"
	return bar
case m.linksPanel.filtering():
	bar.hints = "enter apply · esc cancel"
	return bar
case m.linksPanel.filterApplied():
	parts = []string{"↑/↓ move", "esc clear filter"}
default:
	parts = []string{"↑/↓ move", "/ filter", "esc back"}
}
if selected, ok := m.linksPanel.selected(); ok {
	parts = append(parts, linkActionHints(selected)...)
}
bar.hints = strings.Join(parts, " · ")
return bar
```

Do not render `m.flash` here; `statusBarModel` already owns the global override.

- [ ] **Step 7: Run status and dispatch tests**

Run: `go test ./tui/ -run 'Test(LinkActionHints|ReaderFocusedLinkStatus|LinksPanelStatus|ReaderEnter|LinksPanel)' -count=1 -v`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add tui/linkactions.go tui/linkactions_test.go tui/app.go tui/app_test.go
git commit -m "fix(tui): make link status hints action-specific"
```

### Task 5: Make the help popover contextual

**Files:**
- Modify: `tui/keys.go`
- Modify: `tui/keys_test.go`
- Modify: `tui/app.go:480-620,1040-1044`
- Modify: `tui/app_test.go`

**Interfaces:**
- Consumes: action and focused/selected-link interfaces from earlier tasks.
- Produces: `helpGroups() [][]key.Binding`.

- [ ] **Step 1: Write failing key-label tests**

Add `TestLinkKeyHelp` expecting:

```go
LinkNext  -> key "tab/n", desc "next link"
LinkPrev  -> key "shift+tab/N", desc "previous link"
LinkPanel -> key "L", desc "browse links"
```

- [ ] **Step 2: Write failing contextual-help tests**

Test `helpView()` after `updateKeymap`:

1. Reader without links omits all three link-navigation descriptions.
2. Reader with links includes their column.
3. Definite reader includes `↵ go`; URL and blocked readers omit it.
4. Panel URL help includes move/filter/back/copy but no go.
5. Panel ambiguous help includes `f go` and copy.
6. Panel definite help includes `↵ go` and copy.
7. `?` opens help from an unfiltered panel but types into an active filter.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./tui/ -run 'Test(LinkKeyHelp|ReaderHelp|LinksPanelHelp)' -count=1 -v`

Expected: old labels, absent link column, generic groups, and no panel help routing.

- [ ] **Step 4: Rename visible binding help**

Set `LinkPrev` help to `shift+tab/N · previous link` and `LinkPanel` to `L · browse links`. Keep `LinkNext` unchanged.

- [ ] **Step 5: Implement contextual groups**

Add `helpGroups()` in `app.go`:

- Panel: first group `{Move, Filter, Back}`; second group contains Open only for `linkEnterGo`, a local `f · go` binding only for `.finger`, and Copy for the selected row.
- Other screens: begin with `keys.FullHelp()`; for a reader blocked/refuse action, disable Open only in the local keymap copy used for display; append `{LinkNext, LinkPrev, LinkPanel}` only when the current node has links.

Change `helpView()` to pass `m.helpGroups()` to `fullWidthHelpView`.

- [ ] **Step 6: Route `?` from a non-filtering panel**

Add before panel row actions:

```go
case key.Matches(msg, m.keys.Help):
	m.openHelp()
	return true, m, nil
```

The active-filter guard remains earlier, so `?` reaches the filter there.

- [ ] **Step 7: Run help and overlay tests**

Run: `go test ./tui/ -run 'Test(LinkKeyHelp|ReaderHelp|LinksPanelHelp|QuestionMark|HelpPanel)' -count=1 -v`

Expected: PASS, including existing overlay width and contrast coverage.

- [ ] **Step 8: Commit**

```bash
git add tui/keys.go tui/keys_test.go tui/app.go tui/app_test.go
git commit -m "feat(tui): add contextual link help"
```

### Task 6: Align About copy and run final verification

**Files:**
- Modify: `tui/about.go:74-94`
- Modify: `tui/about_test.go`
- Modify: `tui/app.go:914-934`
- Modify: `tui/app_test.go:1984-2065`

**Interfaces:**
- Consumes: existing About Enter/`y` routing.
- Produces: final About body/status copy; no new exported interface.

- [ ] **Step 1: Update About tests first**

Require the body to contain `finger jonathan@tilde.team`, `↵ go`, `Report a bug or idea`, and `y copy issues URL`. Require the status to contain `↵ go to author` and `y copy issues URL`, preserving the existing landing/result distinction for `esc back`.

Add `TestAboutTabDoesNothing`: open About, press `tea.KeyTab`, and assert no command, no state change, and identical About view.

- [ ] **Step 2: Run About tests to verify they fail**

Run: `go test ./tui/ -run TestAbout -count=1 -v`

Expected: old body/status copy fails the new assertions.

- [ ] **Step 3: Update About body and status**

In `aboutView`, render `y copy issues URL`. In the About status branch, start with `[]string{"↵ go to author", "y copy issues URL"}`. Preserve back/breadcrumb and quit behavior; add no Tab handling or OSC-8 wrapping.

- [ ] **Step 4: Run the TUI suite**

Run: `go test ./tui/ -count=1`

Expected: PASS.

- [ ] **Step 5: Scan production copy**

Run:

```bash
rg -n '↵ drill|f finger|⇥ next|⌘-click opens|address \(auto\)|y to copy the issues URL' tui --glob '*.go'
```

Expected: no matches. Rename stale test messages/comments that describe the new UI.

- [ ] **Step 6: Run the full gate**

Run: `make check`

Expected: vet, formatting, lint, and race-enabled tests all exit successfully.

- [ ] **Step 7: Review scope**

Run:

```bash
git diff --check
git status --short
git diff --stat
git diff
```

Confirm the diff is limited to the TUI files/tests in this plan and does not change `finger/`, `render/`, detection/classification, dependencies, or networking.

- [ ] **Step 8: Commit**

```bash
git add tui/about.go tui/about_test.go tui/app.go tui/app_test.go
git commit -m "fix(tui): align about action copy"
```

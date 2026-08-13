# Help Popover UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the expanded ? popover into a responsive, priority-ordered command map whose displayed commands can be executed directly without changing underlying key behavior.

**Architecture:** Add a focused tui/help.go boundary: app state supplies contextual candidate bindings, a pure layout function chooses a one-to-three-column priority prefix, and a pure renderer draws the opaque bottom band. The retained binding set also gates Help-open dispatch; Esc and About use deliberate Help-layer transitions, while every other displayed action re-emits the original tea.KeyPressMsg through existing app/component routing.

**Tech Stack:** Go, Bubble Tea v2, Bubbles v2 key.Binding and help.Styles, Lip Gloss v2, github.com/charmbracelet/x/ansi, standard testing, Markdown.

## Global Constraints

- Work only in /Users/jonathan/lookit/.worktrees/feat-help-popover-ux on branch feat/help-popover-ux. It is stacked on PR 66 commit 03cbb04, and another agent is working elsewhere.
- Preserve every runtime binding and hidden alias, including g/G, n/N, j/k, and the focused-input Tab Browse gesture.
- Keep g/G top/bottom omitted and retain the simplified tab, shift+tab, and h home labels from the parent branch.
- Keep Help bottom-docked and opaque. Opening it must not resize or repaginate the reader, list, links panel, or startpage.
- Keep the permanent status bar unchanged, including ? help while Help is open. Add no title, category heading, close hint, scrolling control, or compact/full mode.
- Direct a must type while the target input is focused. ? then a must open About from every Help context, including the focused input and links panel.
- ? and Esc close Help without acting on the underlying view. Ctrl+C always quits. A displayed q quit re-dispatches and quits without confirmation.
- A command excluded by context or short-height clipping must be neither displayed nor executable through Help.
- Continue using Bubbles key.Binding, help.Styles, and help.DefaultStyles. Remove the now-unneeded help.Model state and own only the responsive full-panel layout and separator.
- Add no dependencies and do not change networking, target parsing, bookmarks, or response rendering.
- Do not push, open a PR, or merge without separate user authorization.

---

### Task 1: Build the pure responsive Help layout

**Files:**
- Create: tui/help.go
- Create: tui/help_test.go
- Modify: tui/app.go (move two renderer helpers without changing callers)
- Reuse: tui/styles.go (styles and padStyledLine)

**Interfaces:**
- Consumes: `[]key.Binding`, `styles`, terminal width, body height, and `fullHelpSeparator`.
- Produces: helpLayout, layoutHelp, partitionHelpBindings, helpColumnsWidth, renderHelp, and helpLayout.matches.

- [ ] **Step 1: Write failing unit tests for filtering, partitioning, width, height, matching, and rendering**

Create tui/help_test.go:

~~~go
package tui

import (
    "strings"
    "testing"

    "charm.land/bubbles/v2/key"
    tea "charm.land/bubbletea/v2"
    "github.com/charmbracelet/x/ansi"
)

func helpTestBinding(k, desc string) key.Binding {
    return key.NewBinding(key.WithKeys(k), key.WithHelp(k, desc))
}

func helpKeys(bindings []key.Binding) []string {
    out := make([]string, 0, len(bindings))
    for _, binding := range bindings {
        out = append(out, binding.Help().Key)
    }
    return out
}

func TestLayoutHelpFiltersDisabledAndFillsColumnsTopToBottom(t *testing.T) {
    st := newStyles(true)
    bindings := []key.Binding{
        helpTestBinding("1", "one"),
        helpTestBinding("2", "two"),
        helpTestBinding("3", "three"),
        helpTestBinding("4", "four"),
        helpTestBinding("5", "five"),
        helpTestBinding("6", "six"),
    }
    bindings[2].SetEnabled(false)

    layout := layoutHelp(bindings, st, 200, 20)
    if got, want := len(layout.columns), 3; got != want {
        t.Fatalf("columns = %d, want %d", got, want)
    }
    if got, want := strings.Join(helpKeys(layout.bindings), ","), "1,2,4,5,6"; got != want {
        t.Fatalf("retained = %q, want %q", got, want)
    }
    wants := []string{"1,2", "4,5", "6"}
    for i, want := range wants {
        if got := strings.Join(helpKeys(layout.columns[i]), ","); got != want {
            t.Fatalf("column %d = %q, want %q", i, got, want)
        }
    }
}

func TestLayoutHelpChoosesMostColumnsThatFit(t *testing.T) {
    st := newStyles(true)
    bindings := []key.Binding{
        helpTestBinding("1", "alpha"),
        helpTestBinding("2", "bravo"),
        helpTestBinding("3", "charlie"),
        helpTestBinding("4", "delta"),
        helpTestBinding("5", "echo"),
    }
    tests := []struct {
        name string
        width int
        want int
    }{
        {"three", helpColumnsWidth(partitionHelpBindings(bindings, 3), st), 3},
        {"two", helpColumnsWidth(partitionHelpBindings(bindings, 2), st), 2},
        {"one", helpColumnsWidth(partitionHelpBindings(bindings, 1), st), 1},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := len(layoutHelp(bindings, st, tt.width, 20).columns); got != tt.want {
                t.Fatalf("columns = %d, want %d at width %d", got, tt.want, tt.width)
            }
        })
    }
}

func TestLayoutHelpShortHeightRetainsPriorityPrefix(t *testing.T) {
    st := newStyles(true)
    bindings := []key.Binding{
        helpTestBinding("1", "one"), helpTestBinding("2", "two"),
        helpTestBinding("3", "three"), helpTestBinding("4", "four"),
        helpTestBinding("5", "five"), helpTestBinding("6", "six"),
        helpTestBinding("7", "seven"),
    }
    layout := layoutHelp(bindings, st, 200, 1)
    if got, want := strings.Join(helpKeys(layout.bindings), ","), "1,2,3"; got != want {
        t.Fatalf("retained = %q, want priority prefix %q", got, want)
    }
    if layout.matches(tea.KeyPressMsg{Code: '7', Text: "7"}) {
        t.Fatal("height-clipped binding must not match Help dispatch")
    }
    if !layout.matches(tea.KeyPressMsg{Code: '2', Text: "2"}) {
        t.Fatal("retained binding should match Help dispatch")
    }
}

func TestRenderHelpUsesFullWidthStyledRows(t *testing.T) {
    st := newStyles(true)
    layout := layoutHelp([]key.Binding{
        helpTestBinding("x", "first"),
        helpTestBinding("y", "second"),
    }, st, 40, 20)
    view := renderHelp(layout, st, 40)
    for _, line := range strings.Split(view, "\n") {
        assertFullWidthStyledLine(t, "help row", line, 40, st.palette.SubtleBg)
    }
    plain := ansi.Strip(view)
    if !strings.Contains(plain, "x") || !strings.Contains(plain, "first") {
        t.Fatalf("rendered Help missing binding:\n%s", plain)
    }
}

func TestOverlayHelpClipsFromBottom(t *testing.T) {
    got := overlayHelp("body one\nbody two", "core\nsecondary\nlast")
    if want := "core\nsecondary"; got != want {
        t.Fatalf("overlay = %q, want priority prefix %q", got, want)
    }
}
~~~

- [ ] **Step 2: Run the new tests and verify RED**

~~~bash
go test ./tui/ -run 'Test(LayoutHelp|RenderHelp|OverlayHelpClipsFromBottom)' -count=1 -v
~~~

Expected: build failure because `layoutHelp`, `partitionHelpBindings`,
`helpColumnsWidth`, and `renderHelp` do not exist; the overlay regression also
fails because current `overlayHelp` retains the tail.

- [ ] **Step 3: Implement the pure layout core in tui/help.go**

~~~go
package tui

import (
    "strings"

    "charm.land/bubbles/v2/key"
    tea "charm.land/bubbletea/v2"
    "charm.land/lipgloss/v2"
    "github.com/charmbracelet/x/ansi"
)

const fullHelpSeparator = "    "

type helpLayout struct {
    bindings []key.Binding
    columns  [][]key.Binding
}

func layoutHelp(candidates []key.Binding, st styles, width, height int) helpLayout {
    bindings := enabledHelpBindings(candidates)
    if len(bindings) == 0 || height < 1 {
        return helpLayout{}
    }
    maxColumns := min(3, len(bindings))
    for columnCount := maxColumns; columnCount >= 1; columnCount-- {
        retainedCount := min(len(bindings), height*columnCount)
        retained := append([]key.Binding(nil), bindings[:retainedCount]...)
        columns := partitionHelpBindings(retained, columnCount)
        if width <= 0 || columnCount == 1 || helpColumnsWidth(columns, st) <= width {
            return helpLayout{bindings: retained, columns: columns}
        }
    }
    return helpLayout{}
}

func enabledHelpBindings(bindings []key.Binding) []key.Binding {
    out := make([]key.Binding, 0, len(bindings))
    for _, binding := range bindings {
        if binding.Enabled() {
            out = append(out, binding)
        }
    }
    return out
}

func partitionHelpBindings(bindings []key.Binding, columnCount int) [][]key.Binding {
    if len(bindings) == 0 || columnCount < 1 {
        return nil
    }
    rows := (len(bindings) + columnCount - 1) / columnCount
    columns := make([][]key.Binding, 0, columnCount)
    for start := 0; start < len(bindings); start += rows {
        end := min(start+rows, len(bindings))
        columns = append(columns, bindings[start:end])
    }
    return columns
}

func (layout helpLayout) matches(msg tea.KeyPressMsg) bool {
    for _, binding := range layout.bindings {
        if key.Matches(msg, binding) {
            return true
        }
    }
    return false
}
~~~

In `tui/app.go`, reverse the defensive clipping in `overlayHelp`:

~~~go
if n := len(helpLines); n > len(bodyLines) {
    helpLines = helpLines[:len(bodyLines)]
}
~~~

Update its comment to say it keeps the leading priority rows if a caller ever
provides more Help lines than the body can hold.

Move helpColumnRows and maxLineWidth unchanged from tui/app.go into tui/help.go, then add:

~~~go
func helpColumnsWidth(columns [][]key.Binding, st styles) int {
    width := 0
    separatorWidth := lipgloss.Width(
        st.help.FullSeparator.Render(fullHelpSeparator),
    )
    for i, column := range columns {
        if i > 0 {
            width += separatorWidth
        }
        width += maxLineWidth(helpColumnRows(column, st))
    }
    return width
}

func renderHelp(layout helpLayout, st styles, width int) string {
    if len(layout.columns) == 0 {
        return ""
    }

    columns := make([][]string, 0, len(layout.columns))
    widths := make([]int, 0, len(layout.columns))
    maxRows := 0
    for _, bindings := range layout.columns {
        rows := helpColumnRows(bindings, st)
        columnWidth := maxLineWidth(rows)
        for i, row := range rows {
            rows[i] = padStyledLine(row, columnWidth, st.helpBand)
        }
        columns = append(columns, rows)
        widths = append(widths, columnWidth)
        maxRows = max(maxRows, len(rows))
    }

    lines := make([]string, maxRows)
    separator := st.help.FullSeparator.Render(fullHelpSeparator)
    for row := range maxRows {
        var line strings.Builder
        for column, rows := range columns {
            if column > 0 {
                line.WriteString(separator)
            }
            if row < len(rows) {
                line.WriteString(rows[row])
            } else {
                line.WriteString(st.helpBand.Render(
                    strings.Repeat(" ", widths[column]),
                ))
            }
        }
        out := line.String()
        if width > 0 && lipgloss.Width(out) > width {
            out = ansi.Truncate(out, width, "...")
        }
        lines[row] = padStyledLine(out, width, st.helpBand)
    }
    return strings.Join(lines, "\n")
}
~~~

- [ ] **Step 4: Run layout tests and verify GREEN**

~~~bash
gofmt -w tui/help.go tui/help_test.go
go test ./tui/ -run 'Test(LayoutHelp|RenderHelp|OverlayHelpClipsFromBottom)' -count=1 -v
~~~

Expected: PASS. Odd counts have a short last column, height one retains 1,2,3, and rendered rows span the requested width.

- [ ] **Step 5: Commit the pure layout unit**

~~~bash
git add docs/superpowers/plans/2026-08-13-help-popover-ux.md tui/help.go tui/help_test.go tui/app.go
git diff --cached --check
git commit -m "feat(tui): add responsive help layout"
~~~

---

### Task 2: Build contextual candidates and integrate the renderer

**Files:**
- Modify: tui/help.go
- Modify: tui/help_test.go
- Modify: tui/app.go
- Modify: tui/app_test.go
- Modify: tui/keys.go
- Modify: tui/keys_test.go
- Modify: tui/request_test.go

**Interfaces:**
- Consumes: layoutHelp and renderHelp from Task 1, appModel keys/state, actionsForLink, and commonModel.bodyHeight.
- Produces: appModel.helpCandidates, appModel.linksPanelHelpCandidates, appModel.helpLayout, and appModel.helpView. Removes keyMap FullHelp/ShortHelp, appModel.helpModel, and the old renderer in app.go.

- [ ] **Step 1: Write failing tests for priority order and links-panel scope**

Add to tui/help_test.go:

~~~go
func TestHelpCandidatesUsePriorityOrder(t *testing.T) {
    m := readerWithFocusedLink(t, stubFetch(t), Link{
        Kind: LinkFinger, Action: ActionDrill, Raw: "alice@tilde.team",
        Target: hostTarget(t, "alice@tilde.team"),
    })
    m.help = true
    (&m).updateKeymap()
    got := strings.Join(helpKeys(m.helpLayout().bindings), ",")
    want := "↑/↓,↵,esc,i,h,←/→,tab,shift+tab,v,r,y,b,L,a,q"
    if got != want {
        t.Fatalf("Help order = %q, want %q", got, want)
    }
}

func TestLinksPanelHelpCandidatesStaySelectionAwareAndIncludeAbout(t *testing.T) {
    target := hostTarget(t, "alice@tilde.team")
    tests := []struct {
        name string
        link Link
        want string
    }{
        {"URL", Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"}, "↑/↓,esc,y,/,a"},
        {"ambiguous", Link{Kind: LinkFinger, Action: ActionCopy, Raw: target.Raw, Target: target, Ambiguous: true}, "↑/↓,esc,f,y,/,a"},
        {"definite", Link{Kind: LinkFinger, Action: ActionDrill, Raw: target.Raw, Target: target}, "↑/↓,esc,↵,y,/,a"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := linksPanelModel(t, stubFetch(t), []Link{tt.link})
            m.help = true
            (&m).updateKeymap()
            if got := strings.Join(helpKeys(m.helpLayout().bindings), ","); got != tt.want {
                t.Fatalf("Help order = %q, want %q", got, tt.want)
            }
        })
    }
}
~~~

In tui/app_test.go:

- add about lookit to the required strings in TestLinksPanelHelpContext and remove it from unwanted;
- remove the `helpModel.ShowAll` precondition from
  `TestHelpPanelUsesSharedContrastStyles`, then use:

~~~go
if !sameColor(m.common.styles.help.FullKey.GetForeground(),
    m.common.styles.palette.AccentViolet) {
    t.Fatal("help key colour should use accent violet")
}
if !sameColor(m.common.styles.help.FullDesc.GetForeground(),
    m.common.styles.palette.BarText) {
    t.Fatal("help description colour should use bar text")
}
~~~

- replace the background-restyle assertion on `got.helpModel` with:

~~~go
if !sameColor(got.common.styles.help.FullKey.GetForeground(),
    got.common.styles.palette.AccentViolet) {
    t.Fatal("help key style was not rebuilt for the new background")
}
if !sameColor(got.common.styles.help.FullDesc.GetForeground(),
    got.common.styles.palette.BarText) {
    t.Fatal("help description style was not rebuilt for the new background")
}
~~~

In tui/request_test.go, remove m.helpModel.ShowAll = true from the existing open-Help precondition. Setting m.help = true is sufficient.

- [ ] **Step 2: Run focused tests and verify RED**

~~~bash
go test ./tui/ -run 'Test(HelpCandidatesUsePriorityOrder|LinksPanelHelpCandidatesStaySelectionAwareAndIncludeAbout|LinksPanelHelpContext|HelpPanelUsesSharedContrastStyles|BackgroundColorMsgRestylesTUI|RWhileHelpOpenOnlyClosesHelp)$' -count=1 -v
~~~

Expected: build failures for helpCandidates/helpLayout and failures because links-panel Help omits About.

- [ ] **Step 3: Implement contextual candidate methods in tui/help.go**

~~~go
func (m appModel) helpCandidates() []key.Binding {
    if m.showingLinks {
        return m.linksPanelHelpCandidates()
    }
    open := m.keys.Open
    if link, ok := m.focusedReaderLink(); ok &&
        actionsForLink(link).enter == linkEnterRefuse {
        open.SetEnabled(false)
    }
    return []key.Binding{
        m.keys.Move, open, m.keys.Back, m.keys.FocusInput, m.keys.Home,
        m.keys.Page, m.keys.LinkNext, m.keys.LinkPrev, m.keys.Filter,
        m.keys.Browse, m.keys.Raw, m.keys.Refresh,
        m.keys.Copy, m.keys.Bookmark, m.keys.LinkPanel, m.keys.About, m.keys.Quit,
    }
}

func (m appModel) linksPanelHelpCandidates() []key.Binding {
    bindings := []key.Binding{m.keys.Move, m.keys.Back}
    if link, ok := m.linksPanel.selected(); ok {
        actions := actionsForLink(link)
        if actions.enter == linkEnterGo {
            bindings = append(bindings, m.keys.Open)
        }
        if actions.finger {
            bindings = append(bindings, key.NewBinding(
                key.WithKeys("f"), key.WithHelp("f", "go"),
            ))
        }
        if actions.copy {
            bindings = append(bindings, m.keys.Copy)
        }
    }
    return append(bindings, m.keys.Filter, m.keys.About)
}

func (m appModel) helpLayout() helpLayout {
    height := m.common.height - 1 // reserve only the permanent status bar
    if height < 1 {
        height = 1
    }
    return layoutHelp(
        m.helpCandidates(), m.common.styles,
        m.common.width, height,
    )
}

func (m appModel) helpView() string {
    return renderHelp(m.helpLayout(), m.common.styles, m.common.width)
}
~~~

The height argument is total terminal height minus the permanent status-bar
row, not `common.bodyHeight()`: Help overlays the full body assembled from the
target row plus sub-model content. Clamp it to at least one before calling
`layoutHelp` when terminal height is zero or one.

- [ ] **Step 4: Remove the superseded Bubbles model and old renderer**

In tui/app.go:

- remove the Bubbles help and x/ansi imports;
- remove helpModel from appModel construction and style/background updates;
- remove helpModel.SetWidth from WindowSizeMsg;
- make openHelp and closeHelp toggle only m.help;
- delete helpView, helpGroups, fullWidthHelpView, helpColumnRows, and maxLineWidth; and
- leave resize and overlayHelp behavior unchanged.

In `tui/keys.go`, remove `ShortHelp` and `FullHelp` and revise the `keyMap`
comment so it no longer claims to implement `help.KeyMap`. In
`tui/keys_test.go`, replace `TestKeyMapFullHelpOmitsJumpButKeepsBinding` with:

~~~go
func TestJumpBindingRemainsAvailable(t *testing.T) {
    k := newKeyMap()
    for _, want := range []string{"g", "G"} {
        if got := k.Jump.Keys(); !contains(got, want) {
            t.Fatalf("Jump keys = %v, want to retain %q", got, want)
        }
    }
}
~~~

Remove only the `FullHelp` traversal from `TestKeyMapAboutBinding`; retain its
checks that About's runtime key and help label are `a` / `about lookit`.
`TestHelpCandidatesUsePriorityOrder` now proves both that Jump is absent from
the popover and About is present.

- [ ] **Step 5: Run integration tests and verify GREEN**

~~~bash
gofmt -w tui/help.go tui/help_test.go tui/app.go tui/app_test.go tui/keys.go tui/keys_test.go tui/request_test.go
go test ./tui/ -run 'Test(Help|LinksPanelHelp|KeyMap|QuestionMark|BackgroundColorMsgRestylesTUI|RWhileHelpOpenOnlyClosesHelp)' -count=1 -v
git diff --check
~~~

Expected: PASS, and this command prints no remaining production/test reference:

~~~bash
rg -n 'helpModel|FullHelp\(|ShortHelp\(' tui
~~~

- [ ] **Step 6: Commit the renderer integration**

~~~bash
git add tui/help.go tui/help_test.go tui/app.go tui/app_test.go tui/keys.go tui/keys_test.go tui/request_test.go
git diff --cached --check
git commit -m "refactor(tui): centralize contextual help commands"
~~~

---

### Task 3: Make binding availability truthful for the active layer

**Files:**
- Modify: tui/app.go
- Modify: tui/help_test.go
- Modify: tui/app_test.go

**Interfaces:**
- Consumes: appModel.help, focus/state/filter flags, and the candidate/layout path from Task 2.
- Produces: appModel.helpFilterActive and layer-aware About/Help enablement: direct input a types, Help-layer a is displayed, and active filters own ?/a.

- [ ] **Step 1: Write failing layer-availability tests**

Add to tui/help_test.go:

~~~go
func TestHelpLayerAvailability(t *testing.T) {
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    (&m).updateKeymap()
    if m.keys.About.Enabled() {
        t.Fatal("About must be disabled while focused input directly owns a")
    }
    if !m.keys.Help.Enabled() || !m.keys.Open.Enabled() {
        t.Fatal("Help and target submission must remain available")
    }

    m.openHelp()
    (&m).updateKeymap()
    if !m.keys.About.Enabled() {
        t.Fatal("opening Help must enable its dedicated About route")
    }
    got := strings.Join(helpKeys(m.helpLayout().bindings), ",")
    if !strings.Contains(got, "a") {
        t.Fatalf("focused-input Help must display About: %q", got)
    }
}

func TestHelpAvailabilityFollowsFilterAndAboutOwnership(t *testing.T) {
    filtered := settledList(t)
    next, _ := filtered.Update(tea.KeyPressMsg{Code: '/'})
    filtered = next.(appModel)
    (&filtered).updateKeymap()
    if filtered.keys.Help.Enabled() || filtered.keys.About.Enabled() {
        t.Fatal("active list filter must own ? and a")
    }

    panel := linksPanelModel(t, stubFetch(t), []Link{
        {Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"},
    })
    (&panel).updateKeymap()
    if panel.keys.About.Enabled() {
        t.Fatal("About must be disabled while links panel directly owns keys")
    }
    panel.openHelp()
    (&panel).updateKeymap()
    if !panel.keys.About.Enabled() {
        t.Fatal("links-panel Help must enable About")
    }

    about := newApp(stubFetch(t), colorprofile.NoTTY)
    about.openAbout()
    (&about).updateKeymap()
    if about.keys.Help.Enabled() {
        t.Fatal("About screen must not advertise or accept Help")
    }
}
~~~

- [ ] **Step 2: Add the failing context matrix**

Add these helpers and `TestHelpLayoutsByContext` to `tui/help_test.go`:

~~~go
func helpContextModels(t *testing.T) map[string]appModel {
    focused := newApp(stubFetch(t), colorprofile.NoTTY)

    start := newApp(stubFetch(t), colorprofile.NoTTY)
    start.blurInput()

    noLinks := settledReader(t, Entry{
        Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n"),
    })

    linked := readerWithFocusedLink(t, stubFetch(t), Link{
        Kind: LinkFinger, Action: ActionDrill, Raw: "alice@tilde.team",
        Target: hostTarget(t, "alice@tilde.team"),
    })

    raw := settledReader(t, Entry{
        Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n"),
    })
    raw.enterRaw()

    panel := linksPanelModel(t, stubFetch(t), []Link{
        {Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"},
    })

    return map[string]appModel{
        "focused input": focused,
        "start content": start,
        "reader no links": noLinks,
        "reader with link": linked,
        "raw view": raw,
        "links URL": panel,
    }
}

func TestHelpLayoutsByContext(t *testing.T) {
    wants := map[string]string{
        "focused input": "↵,esc,↓,a",
        "start content": "↑/↓,↵,esc,i,←/→,/,b,a,q",
        "reader no links": "↑/↓,esc,i,h,←/→,v,r,y,b,a,q",
        "reader with link": "↑/↓,↵,esc,i,h,←/→,tab,shift+tab,v,r,y,b,L,a,q",
        "raw view": "↑/↓,esc,i,h,←/→,v,y,b,a,q",
        "links URL": "↑/↓,esc,y,/,a",
    }
    for name, m := range helpContextModels(t) {
        t.Run(name, func(t *testing.T) {
            m.common.width, m.common.height = 200, 40
            m.openHelp()
            (&m).updateKeymap()
            got := strings.Join(helpKeys(m.helpLayout().bindings), ",")
            if got != wants[name] {
                t.Fatalf("Help bindings = %q, want %q", got, wants[name])
            }
        })
    }
}
~~~

The asserted sequences are:

~~~text
focused input:    ↵,esc,↓,a
start content:    ↑/↓,↵,esc,i,←/→,/,b,a,q
reader no links:  ↑/↓,esc,i,h,←/→,v,r,y,b,a,q
reader with link: ↑/↓,↵,esc,i,h,←/→,tab,shift+tab,v,r,y,b,L,a,q
raw view:         ↑/↓,esc,i,h,←/→,v,y,b,a,q
links URL:        ↑/↓,esc,y,/,a
~~~

The wide/tall size ensures this matrix tests context rather than clipping. Add
`github.com/charmbracelet/colorprofile` to `tui/help_test.go` imports for these
fixtures.

- [ ] **Step 3: Run availability tests and verify RED**

~~~bash
go test ./tui/ -run 'Test(HelpLayerAvailability|HelpAvailabilityFollowsFilterAndAboutOwnership|HelpLayoutsByContext)$' -count=1 -v
~~~

Expected: FAIL because About and Help are currently enabled unconditionally and Help-open ownership is not represented.

- [ ] **Step 4: Implement layer-aware enablement**

Add beside updateKeymap:

~~~go
func (m appModel) helpFilterActive() bool {
    return (m.state == stateList && m.list.filtering()) ||
        (m.state == stateStart && m.start.filtering()) ||
        (m.showingLinks && m.linksPanel.filtering())
}
~~~

Keep the pending-request early return, which disables Help and About. In the normal branch, replace the unconditional assignments with:

~~~go
filtering := m.helpFilterActive()
content := !m.inputFocused

m.keys.Help.SetEnabled(m.state != stateAbout && !filtering)
m.keys.About.SetEnabled(
    m.state == stateAbout ||
        m.help ||
        (content && !filtering && !m.showingLinks),
)
~~~

Retain the stateAbout block that enables Enter/Copy/Back/Quit and explicitly enable About there so a still closes About. Do not enable About merely because the target input is focused: m.help is the focused-input route that makes it live.

Update the updateKeymap comment to say it models availability in the active interaction layer and that the Help layout retained set is the final execute-through-Help gate.

- [ ] **Step 5: Run the matrix and existing honest-About tests**

~~~bash
gofmt -w tui/app.go tui/help_test.go tui/app_test.go
go test ./tui/ -run 'Test(HelpLayerAvailability|HelpAvailabilityFollowsFilterAndAboutOwnership|HelpLayoutsByContext|LandingTypesAInsteadOfOpeningAbout|AboutOpensFromHelpPanelOnLanding|QuestionMarkWhileFilteringDoesNotOpenHelp|LinksPanelHelpQuestionMarkRouting)$' -count=1 -v
~~~

Expected: PASS. Direct focused-input a types, Help-layer About appears, and filters receive literal ?/a without advertising app actions.

- [ ] **Step 6: Commit layer-aware availability**

~~~bash
git add tui/app.go tui/help_test.go tui/app_test.go
git diff --cached --check
git commit -m "fix(tui): make help availability layer-aware"
~~~

---

### Task 4: Execute displayed commands from the Help layer

**Files:**
- Modify: tui/app.go
- Modify: tui/app_test.go
- Modify: tui/help_test.go
- Modify: tui/request_test.go

**Interfaces:**
- Consumes: appModel.helpLayout, helpLayout.matches, openAbout, closeHelp, and existing Update/handleKey routing.
- Produces: replayKey(tea.KeyPressMsg) tea.Cmd and exact Help-open dispatch.

- [ ] **Step 1: Replace the old refresh characterization with a failing replay test**

Replace TestRWhileHelpOpenOnlyClosesHelp in tui/request_test.go:

~~~go
func TestRWhileHelpOpenRefreshes(t *testing.T) {
    m := settledReader(t, Entry{
        Target: hostTarget(t, "alice@plan.cat"),
        Body: []byte("Plan\n"),
    })
    m.common.width, m.common.height = 120, 24
    m.help = true

    next, replay := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
    m = next.(appModel)
    if m.help || replay == nil {
        t.Fatalf("first r = help %v replay %T", m.help, replay)
    }
    next, _ = m.Update(replay().(tea.KeyPressMsg))
    m = next.(appModel)
    if m.pending == nil || m.pending.intent != requestRefresh {
        t.Fatalf("replayed r pending = %#v, want refresh", m.pending)
    }
}
~~~

- [ ] **Step 2: Add failing Help-open behavior tests**

Add to tui/app_test.go:

~~~go
func TestUnrecognisedKeyLeavesHelpOpenWithoutTyping(t *testing.T) {
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.input.SetValue("alice@")
    next, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
    m = next.(appModel)
    next, cmd := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
    got := next.(appModel)
    if !got.help || cmd != nil || got.input.Value() != "alice@" {
        t.Fatalf("unknown = help %v cmd %T input %q", got.help, cmd, got.input.Value())
    }
}

func TestQuestionMarkAndEscGoBackFromHelpOnly(t *testing.T) {
    m := settledReader(t, Entry{
        Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n"),
    })
    startPos := m.pos
    for _, msg := range []tea.KeyPressMsg{
        {Code: '?', Text: "?"},
        {Code: tea.KeyEsc},
    } {
        m.help = true
        next, cmd := m.Update(msg)
        got := next.(appModel)
        if got.help || got.pos != startPos || cmd != nil {
            t.Fatalf("key %v = help %v pos %d cmd %T", msg, got.help, got.pos, cmd)
        }
    }
}

func TestAboutFromFocusedInputHelpUsesDedicatedTransition(t *testing.T) {
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.input.SetValue("alice@")
    next, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
    m = next.(appModel)
    next, cmd := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
    got := next.(appModel)
    if got.help || got.state != stateAbout || cmd != nil ||
        got.input.Value() != "alice@" {
        t.Fatalf("Help a = help %v state %d cmd %T input %q",
            got.help, got.state, cmd, got.input.Value())
    }
}

func TestClippedAndUnadvertisedCommandsDoNotExecuteFromHelp(t *testing.T) {
    m := settledReader(t, Entry{
        Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n"),
    })
    m.common.width, m.common.height = 200, 3
    m.help = true
    for _, msg := range []tea.KeyPressMsg{
        {Code: 'q', Text: "q"},
        {Code: 'g', Text: "g"},
    } {
        next, cmd := m.Update(msg)
        got := next.(appModel)
        if !got.help || cmd != nil {
            t.Fatalf("key %q = help %v cmd %T", msg.Text, got.help, cmd)
        }
    }
}
~~~

Add TestDisplayedAliasAndDelegatedMoveReplayThroughNormalRouting:

~~~go
func TestDisplayedAliasAndDelegatedMoveReplayThroughNormalRouting(t *testing.T) {
    links := []Link{
        {Kind: LinkURL, Action: ActionCopy, Raw: "https://one.example"},
        {Kind: LinkURL, Action: ActionCopy, Raw: "https://two.example"},
    }
    entry := Entry{
        Target: hostTarget(t, "viewer@origin.example"),
        Body: []byte(links[0].Raw + "\n" + links[1].Raw + "\n"),
    }
    reader := settledReader(t, entry)
    reader.history[0].links, reader.history[0].linkIdx = links, 1
    reader.reader.links, reader.reader.focusedLink = links, 1
    reader.reader.setEntryWithLinks(entry, links)
    reader.help = true

    next, replay := reader.Update(tea.KeyPressMsg{Code: 'N', Text: "N"})
    reader = next.(appModel)
    if reader.help || replay == nil {
        t.Fatal("displayed N alias should close Help and return replay")
    }
    next, _ = reader.Update(replay().(tea.KeyPressMsg))
    if got := next.(appModel).reader.focusedLink; got != 0 {
        t.Fatalf("replayed N focused link = %d, want 0", got)
    }

    listed := settledList(t)
    listed.list.list.Select(0)
    listed.help = true
    next, replay = listed.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
    listed = next.(appModel)
    if listed.help || replay == nil {
        t.Fatal("displayed j alias should close Help and return replay")
    }
    next, _ = listed.Update(replay().(tea.KeyPressMsg))
    if got := next.(appModel).list.list.Index(); got != 1 {
        t.Fatalf("replayed j selected index = %d, want 1", got)
    }
}
~~~

- [ ] **Step 3: Add the failing displayed-set admission matrix**

Add TestHelpAdmissionMatchesRetainedBindingsAcrossContexts to tui/help_test.go. Reuse `helpContextModels(t)` at 200x40 and this message universe:

~~~go
messages := []tea.KeyPressMsg{
    {Code: tea.KeyEnter},
    {Code: 'i', Text: "i"},
    {Code: 'j', Text: "j"},
    {Code: tea.KeyRight},
    {Code: '/', Text: "/"},
    {Code: tea.KeyTab},
    {Code: 'N', Text: "N"},
    {Code: 'v', Text: "v"},
    {Code: 'r', Text: "r"},
    {Code: 'y', Text: "y"},
    {Code: 'b', Text: "b"},
    {Code: 'h', Text: "h"},
    {Code: 'L', Text: "L"},
    {Code: 'a', Text: "a"},
    {Code: 'q', Text: "q"},
    {Code: 'f', Text: "f"},
    {Code: 'g', Text: "g"},
    {Code: 'x', Text: "x"},
}
~~~

Use this loop so each key starts from a fresh context:

~~~go
for name := range helpContextModels(t) {
    t.Run(name, func(t *testing.T) {
        for _, msg := range messages {
            clone := helpContextModels(t)[name]
            clone.common.width, clone.common.height = 200, 40
            clone.openHelp()
            (&clone).updateKeymap()
            want := clone.helpLayout().matches(msg)
            next, _ := clone.Update(msg)
            if got := next.(appModel).help; got == want {
                t.Errorf("key %v leaves Help=%v, want %v", msg, got, !want)
            }
        }
    })
}
~~~

About is retained and dedicated, so it satisfies the same displayed/executable
invariant. Test ?, Esc, and Ctrl+C separately because they are dedicated
controls outside the retained ordinary set.

- [ ] **Step 4: Run dispatch tests and verify RED**

~~~bash
go test ./tui/ -run 'Test(RWhileHelpOpenRefreshes|UnrecognisedKeyLeavesHelpOpenWithoutTyping|QuestionMarkAndEscGoBackFromHelpOnly|AboutFromFocusedInputHelpUsesDedicatedTransition|DisplayedAliasAndDelegatedMoveReplayThroughNormalRouting|ClippedAndUnadvertisedCommandsDoNotExecuteFromHelp|HelpAdmissionMatchesRetainedBindingsAcrossContexts)$' -count=1 -v
~~~

Expected: FAIL because current Help closes on every key and does not replay displayed actions.

- [ ] **Step 5: Implement retained-set dispatch**

Add beside Help lifecycle:

~~~go
func replayKey(msg tea.KeyPressMsg) tea.Cmd {
    return func() tea.Msg { return msg }
}
~~~

Keep force quit before Help. Replace the current m.help block:

~~~go
if m.help {
    switch {
    case key.Matches(msg, m.keys.Help), key.Matches(msg, m.keys.Back):
        m.closeHelp()
        m.resize()
        return true, m, nil
    }

    layout := m.helpLayout()
    if !layout.matches(msg) {
        return true, m, nil
    }
    m.closeHelp()
    if key.Matches(msg, m.keys.About) {
        m.openAbout()
        return true, m, nil
    }
    m.resize()
    return true, m, replayKey(msg)
}
~~~

Do not add an action switch for Open, Home, Refresh, movement, paging, aliases, links, filters, Copy, Bookmark, or Quit. The replayed Bubble Tea update is the only normal-dispatch path for them.

- [ ] **Step 6: Add explicit quit, question-mark, Finger-link, and force-quit regressions**

Add to `tui/app_test.go`:

~~~go
func TestQFromHelpReplaysAndQuits(t *testing.T) {
    m := settledReader(t, Entry{
        Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n"),
    })
    m.common.width, m.common.height = 120, 24
    m.help = true
    next, replay := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
    m = next.(appModel)
    if m.help || replay == nil {
        t.Fatal("q should close Help and return replay")
    }
    _, quit := m.Update(replay().(tea.KeyPressMsg))
    if !isQuit(quit) {
        t.Fatal("replayed q should quit")
    }
}

func TestQuestionMarkFromFocusedInputHelpDoesNotType(t *testing.T) {
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.input.SetValue("alice@")
    next, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
    m = next.(appModel)
    next, cmd := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
    got := next.(appModel)
    if got.help || cmd != nil || got.input.Value() != "alice@" {
        t.Fatalf("? = help %v cmd %T input %q", got.help, cmd, got.input.Value())
    }
}

func TestFingerLinkActionReplaysFromLinksPanelHelp(t *testing.T) {
    target := hostTarget(t, "alice@tilde.team")
    m := linksPanelModel(t, stubFetch(t), []Link{{
        Kind: LinkFinger, Action: ActionCopy, Raw: target.Raw,
        Target: target, Ambiguous: true,
    }})
    m.help = true
    next, replay := m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
    m = next.(appModel)
    if m.help || replay == nil {
        t.Fatal("f should close Help and return replay")
    }
    next, _ = m.Update(replay().(tea.KeyPressMsg))
    got := next.(appModel)
    if got.pending == nil || got.pending.target.Raw != target.Raw {
        t.Fatalf("replayed f pending = %#v, want %q", got.pending, target.Raw)
    }
}

func TestForceQuitBypassesHelpReplay(t *testing.T) {
    m := newApp(stubFetch(t), colorprofile.NoTTY)
    m.help = true
    next, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
    if !next.(appModel).help || !isQuit(cmd) {
        t.Fatal("Ctrl+C should quit immediately without mutating Help")
    }
}
~~~

Also update `TestQuestionMarkTogglesHelpOverlay`: replace its stale failure text
`any key should close the help overlay` with `Esc should go back from Help`.

- [ ] **Step 7: Run the complete TUI package and verify GREEN**

~~~bash
gofmt -w tui/app.go tui/app_test.go tui/help_test.go tui/request_test.go
go test ./tui/ -count=1
git diff --check
~~~

Expected: PASS. Refresh runs, unknown/clipped keys leave Help open, aliases and delegated keys replay, ? then a still opens About, and no-repagination remains green.

- [ ] **Step 8: Commit Help interaction**

~~~bash
git add tui/app.go tui/app_test.go tui/help_test.go tui/request_test.go
git diff --cached --check
git commit -m "feat(tui): execute commands from help"
~~~

---

### Task 5: Record the architecture and run the full gate

**Files:**
- Modify: CLAUDE.md (AGENTS.md is its symlink)
- Modify: docs/user-facing-messages.md
- Verify: all Go packages and repository gates

**Interfaces:**
- Consumes: completed Help layout and routing from Tasks 1-4.
- Produces: current documentation for the custom renderer, transient layer, About exception, aliases, and status-bar wording.

- [ ] **Step 1: Update the repository architecture note**

Add this paragraph to the `appModel` section of `CLAUDE.md`:

~~~markdown
**Expanded Help** is a transient, bottom-docked overlay, not a history node and
not a resizing layout element. `tui/help.go` derives a priority-ordered candidate
set from live `key.Binding` values, retains the longest prefix that fits a
one-to-three-column layout, and uses the same retained set as the execute gate.
`?` and Esc return to the underlying view; `a` deliberately opens About even
over the focused target input; every other displayed action replays its original
key message through normal app/component routing. Unrecognised, disabled, and
height-clipped commands do nothing while Help stays open. The renderer uses
Bubbles bindings and `help.Styles` but not `help.Model`, whose fixed columns
cannot provide this responsive layout. The permanent bar still says `? help`
while open, where `?` acts as the toggle back.
~~~

Keep the existing About paragraph's honest-keybinding statement and ? then a route unchanged.

- [ ] **Step 2: Update the live Help catalogue explanation**

Replace the introduction under `## TUI Help` in
`docs/user-facing-messages.md`:

~~~markdown
These are app-level key binding labels. The expanded Help popover shows enabled
bindings in priority order, so not every label is visible in every state or at
every terminal height. Its displayed primary gesture does not remove hidden
runtime aliases, and `? help` remains in the status bar rather than the popover.
~~~

Add these rows to the TUI Help table:

~~~markdown
| `↓ browse` | `tui/keys.go` | Move from the focused target input into the startpage list; Tab remains an accepted alias. |
| `b bookmark` / `b remove` | `tui/keys.go`, `tui/app.go` | Add or remove the current target in bookmarks. |
| `L browse links` | `tui/keys.go` | Open the detected-links panel. |
| `a about lookit` | `tui/keys.go`, `tui/app.go` | Open About from content or from Help, including Help over the focused input. |
| `f go` | `tui/help.go` | Finger the selected ambiguous Finger link from the links panel. |
~~~

- [ ] **Step 3: Run documentation and source consistency checks**

~~~bash
if rg -n 'helpModel|any key closes' tui CLAUDE.md docs/user-facing-messages.md; then
    exit 1
fi
git diff --check
go test ./tui/ -count=1
~~~

Expected: no stale helpModel or any-key-closes claim, a clean whitespace check, and passing TUI tests.

- [ ] **Step 4: Run the complete CI-equivalent gate**

~~~bash
make check
~~~

Expected: go vet, gofmt check, golangci-lint, and go test ./... -race all pass.

- [ ] **Step 5: Review scope and commit documentation**

~~~bash
git status --short
git diff --check
git diff --stat 03cbb04...HEAD
git add CLAUDE.md docs/user-facing-messages.md
git diff --cached --check
git commit -m "docs(tui): document responsive help behavior"
~~~

Expected: branch changes are restricted to the committed design/plan, Help-related Go code/tests, CLAUDE.md, and the live message catalogue. Browser-companion files under .superpowers remain ignored.

- [ ] **Step 6: Verify the exact committed branch**

~~~bash
test -z "$(git status --porcelain)"
make check
git log --oneline --decorate 03cbb04..HEAD
~~~

Expected: clean worktree, full gate passing against the committed tree, and
design, responsive layout (including this plan), candidate, availability,
interaction, and documentation changes in the log. Stop and request separate
authorization before push or PR creation.

# Help Menu Simplification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Simplify the expanded help panel's advertised keys while preserving every existing keyboard shortcut.

**Architecture:** Keep runtime key lists and display labels on the existing `key.Binding` values: `Keys()` retains every accepted alias while `Help()` names only the primary gesture. Remove `Jump` only from `keyMap.FullHelp()`; do not change Bubble Tea component keymaps, `updateKeymap`, or `handleKey`.

**Tech Stack:** Go, Bubble Tea v2, Bubbles v2 `key.Binding`, standard `testing`, Markdown.

## Global Constraints

- Work only in `/Users/jonathan/lookit/.worktrees/refactor-help-menu-simplification` on branch `refactor/help-menu-simplification`; another agent is working in the main checkout.
- Change only the expanded help presentation: omit `g/G top/bottom`, show `tab next link`, show `shift+tab previous link`, and show `h home`.
- Preserve runtime bindings for `g`, `G`, `tab`, `n`, `shift+tab`, `N`, and `h`.
- Leave status-bar hints and every other help label unchanged.
- Do not rewrite historical specs or plans; update only the current user-facing message catalogue.
- Do not add or change dependencies.

---

### Task 1: Simplify help copy without removing aliases

**Files:**
- Modify: `tui/keys_test.go`
- Modify: `tui/app_test.go`
- Modify: `tui/list_test.go`
- Modify: `tui/keys.go`
- Modify: `docs/user-facing-messages.md`

**Interfaces:**
- Consumes: `newKeyMap() keyMap`, `keyMap.FullHelp() [][]key.Binding`, `key.Binding.Keys() []string`, and `key.Binding.Help() key.Help`.
- Produces: unchanged runtime bindings with simplified `Help()` metadata and a `FullHelp()` result that omits `Jump`.

- [ ] **Step 1: Write failing tests for simplified labels and preserved aliases**

In `tui/keys_test.go`, replace `TestLinkKeyHelp` with a test that checks both the displayed primary gesture and every accepted key:

```go
func TestLinkKeyHelpSimplifiesDisplayWithoutRemovingAliases(t *testing.T) {
	k := newKeyMap()
	tests := []struct {
		name     string
		binding  key.Binding
		wantHelp key.Help
		wantKeys []string
	}{
		{
			name:     "next",
			binding:  k.LinkNext,
			wantHelp: key.Help{Key: "tab", Desc: "next link"},
			wantKeys: []string{"tab", "n"},
		},
		{
			name:     "previous",
			binding:  k.LinkPrev,
			wantHelp: key.Help{Key: "shift+tab", Desc: "previous link"},
			wantKeys: []string{"shift+tab", "N"},
		},
		{
			name:     "panel",
			binding:  k.LinkPanel,
			wantHelp: key.Help{Key: "L", Desc: "browse links"},
			wantKeys: []string{"L"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.binding.Help(); got != tt.wantHelp {
				t.Fatalf("link help = %+v, want %+v", got, tt.wantHelp)
			}
			for _, want := range tt.wantKeys {
				if got := tt.binding.Keys(); !contains(got, want) {
					t.Errorf("keys = %v, want to retain %q", got, want)
				}
			}
		})
	}
}
```

Replace `TestKeyMapFullHelpIncludesPageAndMoveKeys` with the following. It
compares complete binding key sets, rather than substrings, so the hidden `g`
cannot be confused with the advertised `pgup`/`pgdown` keys:

```go
func TestKeyMapFullHelpOmitsJumpButKeepsBinding(t *testing.T) {
	k := newKeyMap()
	var all []string
	for _, group := range k.FullHelp() {
		for _, b := range group {
			all = append(all, strings.Join(b.Keys(), ","))
		}
	}
	joined := strings.Join(all, " ")
	for _, want := range []string{"i", "y", "r", "esc", "q", "left,right,l,pgup,pgdown"} {
		if !contains(all, want) {
			t.Fatalf("FullHelp missing %q; got %s", want, joined)
		}
	}
	if contains(all, "?") {
		t.Fatalf("FullHelp should omit '?' (it lives in the bottom bar); got %s", joined)
	}
	if contains(all, "g,G") {
		t.Fatalf("FullHelp should omit Jump; got %s", joined)
	}
	for _, want := range []string{"g", "G"} {
		if got := k.Jump.Keys(); !contains(got, want) {
			t.Fatalf("Jump keys = %v, want to retain %q", got, want)
		}
	}
}
```

Extend `TestBookmarkAndHomeKeysBound` immediately after its existing `h` runtime assertion:

```go
	if got, want := k.Home.Help(), (key.Help{Key: "h", Desc: "home"}); got != want {
		t.Fatalf("Home help = %+v, want %+v", got, want)
	}
```

In `tui/app_test.go`, add a behavioral regression test beside the other reader
link-navigation tests. This protects the hidden `n` and `N` aliases through the
real `appModel.Update` routing, not only through binding metadata:

```go
func TestReaderLinkAliasesRemainAvailable(t *testing.T) {
	links := []Link{
		{Kind: LinkURL, Action: ActionCopy, Raw: "https://one.example"},
		{Kind: LinkURL, Action: ActionCopy, Raw: "https://two.example"},
	}
	entry := Entry{
		Target: hostTarget(t, "viewer@origin.example"),
		Body:   []byte(links[0].Raw + "\n" + links[1].Raw + "\n"),
	}
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.history = []histNode{{entry: entry, state: stateReader, links: links, linkIdx: 0}}
	m.pos, m.state, m.inputFocused = 0, stateReader, false
	m.reader.focusedLink = 0
	m.reader.setEntryWithLinks(entry, links)

	next, _ := m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	m = next.(appModel)
	if got := m.reader.focusedLink; got != 1 {
		t.Fatalf("n focused link = %d, want 1", got)
	}

	next, _ = m.Update(tea.KeyPressMsg{Code: 'N', Text: "N"})
	m = next.(appModel)
	if got := m.reader.focusedLink; got != 0 {
		t.Fatalf("N focused link = %d, want 0", got)
	}
}
```

In `tui/list_test.go`, add a regression beside
`TestListMoveDownChangesSelection`. It exercises the real Bubbles list through
lookit's `listModel.update`, protecting the delegated `g`/`G` feature after its
help row disappears:

```go
func TestListJumpAliasesRemainAvailable(t *testing.T) {
	users := []User{{Login: "alice"}, {Login: "bob"}, {Login: "carol"}}
	m := newList(testCommon(), hostTarget(t, "@tilde.team"), users)
	m.list.Select(1)

	m, _ = m.update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if got := m.list.Index(); got != 0 {
		t.Fatalf("g selected index = %d, want 0", got)
	}

	m, _ = m.update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if got := m.list.Index(); got != len(users)-1 {
		t.Fatalf("G selected index = %d, want %d", got, len(users)-1)
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
go test ./tui/ -run 'Test(LinkKeyHelpSimplifiesDisplayWithoutRemovingAliases|KeyMapFullHelpOmitsJumpButKeepsBinding|BookmarkAndHomeKeysBound|ReaderLinkAliasesRemainAvailable|ListJumpAliasesRemainAvailable)$' -count=1 -v
```

Expected: the three presentation assertions FAIL because link help still
renders `tab/n` and `shift+tab/N`, `Jump` is still present in `FullHelp()`, and
Home still describes itself as `startpage`. The alias metadata and behavioral
characterization tests should pass; they cover deliberately preserved existing
behavior and do not drive the production change.

- [ ] **Step 3: Make the minimal help metadata and membership changes**

In `tui/keys.go`, keep each `WithKeys` argument unchanged and update only the display metadata:

```go
		Home:     key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "home")),
		LinkNext:   key.NewBinding(key.WithKeys("tab", "n"), key.WithHelp("tab", "next link")),
		LinkPrev:   key.NewBinding(key.WithKeys("shift+tab", "N"), key.WithHelp("shift+tab", "previous link")),
```

Remove `k.Jump` from the middle `FullHelp()` group:

```go
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Open, k.FocusInput, k.Copy, k.Raw, k.Refresh},
		{k.Move, k.Page, k.Filter, k.Browse},
		{k.Bookmark, k.Home, k.Back, k.About, k.Quit},
	}
}
```

Adjust the file-level comment so it no longer claims every display-only binding is advertised:

```go
// Scroll, page, and jump bindings are owned by the bubbles viewport/list at
// runtime. Move and Page appear here so the help panel can advertise them; Jump
// mirrors the working advanced shortcut but is intentionally omitted from help.
```

Do not modify `Jump`'s `WithKeys`, Bubble Tea/Bubbles keymaps, `updateKeymap`, or `handleKey`.

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run:

```bash
go test ./tui/ -run 'Test(LinkKeyHelpSimplifiesDisplayWithoutRemovingAliases|KeyMapFullHelpOmitsJumpButKeepsBinding|BookmarkAndHomeKeysBound|ReaderLinkAliasesRemainAvailable|ListJumpAliasesRemainAvailable)$' -count=1 -v
```

Expected: PASS, including the assertions that all hidden aliases remain in `Keys()`.

- [ ] **Step 5: Update the live user-facing message catalogue**

In the `## TUI Help` table in `docs/user-facing-messages.md`:

- delete the `g/G top/bottom` row because that message is no longer displayed;
- add `h home` with source `tui/keys.go` and surface text `Open the startpage.`;
- add `tab next link` with source `tui/keys.go` and surface text `Focus the next detected reader link; n remains an accepted alias.`; and
- add `shift+tab previous link` with source `tui/keys.go` and surface text `Focus the previous detected reader link; N remains an accepted alias.`.

Do not change the historical files under `docs/superpowers/specs/` or `docs/superpowers/plans/` other than this implementation plan.

- [ ] **Step 6: Run formatting and regression tests**

Run:

```bash
gofmt -w tui/keys.go tui/keys_test.go tui/app_test.go tui/list_test.go
go test ./tui/ -count=1
git diff --check
```

Expected: all commands exit 0 and `git diff --check` prints nothing.

- [ ] **Step 7: Run the complete CI-equivalent gate**

Run:

```bash
make check
```

Expected: `go vet ./...`, the formatting check, `golangci-lint run ./...`, and `go test ./... -race` all pass.

- [ ] **Step 8: Commit the implementation**

Review the staged scope before committing:

```bash
git add tui/keys.go tui/keys_test.go tui/app_test.go tui/list_test.go docs/user-facing-messages.md docs/superpowers/plans/2026-08-13-help-menu-simplification.md
git diff --cached --check
git diff --cached --stat
```

Expected: only the four TUI files, live message catalogue, and this plan are
staged; the committed design spec is already in branch history.

Commit:

```bash
git commit -m "refactor(tui): simplify help menu labels"
```

Do not push or create a PR without separate user authorization.

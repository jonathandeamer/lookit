# lookit Phase 2 TUI design

## Goal

Build the first interactive TUI for `lookit`: a query-first reader that lets a user type a finger target, fetch it, and browse the rendered response in a scrollable terminal view.

Phase 2 should make `lookit` useful as an interactive reader without pulling in Phase 3 subscriptions, persistence, catalog browsing, or configurable workflows.

## Non-goals

- No subscriptions, refresh, diffing, persistence, or catalog.
- No `lookit get` command in this phase.
- No `--tui target` flag in this phase.
- No `help` pseudo-command; use `-h` and `--help`.
- No markdown rendering or `.plan` reflowing.
- No configurable themes or keymaps.
- No sidebars, split panes, tabs, fuzzy finders, tables, or command palette.
- No fang/cobra-style command framework yet.

## CLI behavior

Phase 2 keeps the CLI small:

```text
lookit                 open the TUI
lookit user@host[:port] one-shot query
lookit @host[:port]    one-shot server query
lookit version         print version
lookit -h              print usage
lookit --help          print usage
```

Bare one-shot target arguments remain backward compatible with Phase 1. `lookit` with no arguments opens the TUI.

`lookit version` prints:

```text
lookit dev (built unknown)
```

`main.go` should define:

```go
var (
	version = "dev"
	builtAt = "unknown"
)
```

Release builds can set these later:

```bash
go build -ldflags "-X main.version=0.2.0 -X main.builtAt=2026-05-29"
```

The binary does not need to parse or enforce SemVer. When releases start, tags can use normal `v0.x.y` versioning.

## TUI behavior

The TUI is a single-screen reader:

- Top row: compact target input.
- Status row: neutral/loading/error/success text.
- Main area: scrollable viewport containing the same rendered finger output produced by `render.Render`.

Initial state:

- Input is focused.
- No query has been fetched yet.
- Viewport shows a short empty-state hint, not a marketing or tutorial page.

Fetch flow:

1. User types `user@host`, `@host`, `user@host:port`, or `@host:port`.
2. User presses Enter.
3. TUI validates the input with `finger.ParseTarget`.
4. Invalid input stays in the TUI and shows an error in the status row.
5. Valid input starts an async fetch.
6. While loading, duplicate Enter submits are ignored.
7. Fetch completion updates the current entry and viewport content.
8. A query error is rendered through `render.Render` so partial bodies, truncation notices, and error lines stay consistent with one-shot output.

Keys:

```text
Enter            fetch the typed target
Esc, Ctrl+C      quit
Up, Down         scroll response
PageUp, PageDown scroll response by page
Home, End        jump response to top/bottom
?                reserved for compact help
```

The first implementation may show the key hints inline rather than building a full help modal. The keymap should still be centralized so adding Bubbles `help` later is straightforward.

## State model

The UI only displays one response now, but the state should not assume that one response is the only future shape. Model the fetched response as an entry:

```go
type Entry struct {
	Target finger.Target
	Body   []byte
	Meta   finger.Meta
	Err    error
}
```

The Bubble Tea model can then hold:

```go
type Model struct {
	input    textinput.Model
	viewport viewport.Model
	current  *Entry
	loading  bool
	status   string
	profile  colorprofile.Profile
	fetch    FetchFunc
}
```

Later phases can add `entries []Entry` and `selected int` for history or split-pane browsing without replacing the fetch/render core.

## Architecture

Add a new `tui/` package that consumes the existing packages:

```text
tui/
  model.go       Bubble Tea model, update, view, key handling
  fetch.go       async fetch command and fetch result message
  styles.go      compact styles for input/status/layout chrome
  model_test.go  model/update tests with injected fetch function
```

Boundaries:

- `finger/` remains networking only.
- `render/` remains output rendering only.
- `tui/` owns interactive state and terminal layout.
- `main.go` owns process-level routing and exits.

The TUI should not duplicate finger networking logic or field-highlighting logic.

## Charm tools

Use the current Charm v2 stack for new TUI work:

- Bubble Tea: `charm.land/bubbletea/v2`
- Bubbles: `charm.land/bubbles/v2`
- Lip Gloss: `charm.land/lipgloss/v2`

Phase 2 should add only the Charm tools that carry their weight:

- Bubble Tea for the event loop.
- Bubbles `textinput` for target entry.
- Bubbles `viewport` for response scrolling.
- Bubbles `key` for centralized key bindings.
- Bubble Tea `tea.View` fields for terminal mode declarations.
- Optional Bubbles `spinner` only if loading text feels too static during implementation.

Skip for now:

- fang or cobra-style command routing.
- huh forms.
- glamour rendering.
- Charm log.
- VHS/freeze docs tooling.
- Bubbles list/table/tabs.

Bubble Tea examples worth following during implementation:

- `textinput` for input setup and focus behavior.
- pager/viewport examples for scrolling behavior.
- `http` and `send-msg` examples for async command/message flow.
- `spinner` only if a spinner is added.

Other Bubble Tea apps reinforce these choices:

- Keep TUI code separate from reusable core logic.
- Make controls discoverable.
- Do not copy large app structures intended for dashboards, file managers, or games.
- Avoid configurable themes/keymaps until they solve a real user problem.

## Dependency migration

The current Phase 1 renderer uses `github.com/charmbracelet/lipgloss`. Bubbles v2 uses `charm.land/lipgloss/v2`.

Phase 2 should not migrate `render/` to Lip Gloss v2. Local review of `~/lipgloss/UPGRADE_GUIDE_V2.md` shows v2 removed `Renderer`; the current renderer relies on `lipgloss.NewRenderer(io.Discard)` and explicit color-profile conversion. Migrating `render/` would be a real renderer rewrite, not a small import-path update.

Use Lip Gloss v2 only inside `tui/` for TUI chrome, and keep `render/` on its existing Lip Gloss version for this phase. Revisit a full `render/` migration in a separate plan if dependency cleanup becomes important.

The local `~/bubbletea`, `~/bubbles`, and `~/lipgloss` checkouts are useful reference material, but the versions resolved into this repository's `go.mod` are authoritative during implementation. After adding Bubble Tea/Bubbles/Lip Gloss v2, verify the exact installed APIs with `go doc` and `go test` before relying on examples or local clone contents.

## Styling

Use plain compact chrome:

- No full-screen border.
- No heavy panels.
- No card-like boxes.
- Fixed-height input and status rows.
- Viewport uses the remaining terminal height.

The TUI should use Bubble Tea's alternate screen buffer. That makes the reader feel like an app, gives viewport sizing predictable semantics, and leaves the shell clean when the user quits. This is terminal mode, not visual chrome; it does not conflict with the no-border/no-panel design.

Enable Bubble Tea cell-motion mouse mode so the Bubbles viewport can receive mouse-wheel events. Do not build mouse-specific UI controls in Phase 2.

Palette:

- Reuse the existing pink/cyan/gold/red/dim family from `render`.
- Active input/caret: pink or cyan.
- Neutral status: dim.
- Loading: dim or cyan text.
- Success: gold sparkle or concise gold status.
- Error: red, with explicit `error:` wording.
- Warning/truncation: gold, with explicit text.

Do not use rainbow gradients or decorative effects in Phase 2. Keep the rainbow inspiration reserved for future changed-content subscription states.

Do not rely on color alone. The TUI should still communicate state through text in limited-color terminals.

## Color profile handling

Bubble Tea and Lip Gloss v2 handle terminal color downsampling for TUI chrome. The rendered finger response still needs a `colorprofile.Profile` because `render.Render` takes one explicitly.

The TUI should:

- initialize with a conservative profile,
- handle Bubble Tea color profile messages when available,
- pass the current profile into `render.Render`.

Bubble Tea v2 emits `tea.ColorProfileMsg`. Update the model's profile from that message.

The TUI `Init` command should request color capabilities with `tea.RequestCapability("RGB")` and `tea.RequestCapability("Tc")`, alongside the text input blink command. This lets Bubble Tea upgrade the terminal color profile when supported.

## Error handling

Input parse errors stay inside the TUI:

```text
error: target must be of the form user@host or @host
```

Fetch errors are represented by `Entry.Err` and rendered into the viewport with `render.Render`, preserving Phase 1 behavior for DNS failures, connection failures, read timeouts, partial bodies, and truncation notices.

The TUI itself exits with code 0 when the user quits normally. Startup failures creating or running the Bubble Tea program should print to stderr and exit 2 from `main.go`.

## Testing

TUI tests should avoid real networking. Inject a fetch function:

```go
type FetchFunc func(context.Context, finger.Target) ([]byte, finger.Meta, error)
```

Tests should focus on model behavior rather than exact full-screen ANSI output:

- initial model has focused input, no current entry, and not loading
- invalid Enter sets status error and does not call fetch
- valid Enter starts loading and clears old status
- duplicate Enter while loading does not start another fetch
- fetch success stores `Entry`, stops loading, and puts rendered response in the viewport
- fetch error stores `Entry.Err`, stops loading, and puts rendered error output in the viewport
- viewport resize updates dimensions
- quit keys return a quit command
- `versionString()` formats `lookit <version> (built <builtAt>)`

Keep existing `finger/` and `render/` tests unchanged except for any import-path migration needed by Lip Gloss v2.

## README updates

After implementation, update README usage:

```bash
lookit                  # open the TUI reader
lookit alice@plan.cat   # one-shot query
lookit @tilde.team      # one-shot server query
lookit version          # print version
```

Add a short TUI controls block:

```text
Enter fetches, arrows/PageUp/PageDown scroll, Esc/Ctrl+C quits.
```

Do not document `get`, `--tui`, or Phase 3 commands as current behavior.

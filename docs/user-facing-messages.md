# User-facing Messages

This is an inventory of app-authored text that can reach users through the CLI,
the one-shot renderer, or the interactive TUI. It excludes arbitrary response
body text returned by finger servers. The goal is to make future copy changes or
message configurability easier by recording the source locations and the runtime
surface where each message appears.

Line numbers are current as of 2026-06-01 and should be treated as a starting
point rather than a permanent API.

## CLI

| Message | Source | Surface |
| --- | --- | --- |
| `A finger client built for exploring, not just querying.` plus the structured `Usage:`, `Targets:`, `Options:`, and `Examples:` sections | `render/cli.go` (`Usage`) | `-h`/`--help` output on stdout. |
| `Press ? inside lookit for keyboard shortcuts.` | `render/cli.go` (`Usage`) | Final pointer in `-h`/`--help` output. |
| `lookit version <version> (built <builtAt>)`, or `lookit version <version>` when the build date is unknown | `main.go` (`versionString`) | `-v`/`--version` output on stdout. |
| `lookit: unknown option "<option>"` followed by `Try 'lookit --help' for usage.` | `main.go` (`run`), `render/cli.go` (`InvocationError`) | Unknown option diagnostic on stderr. |
| `lookit: expected at most one target` followed by `Try 'lookit --help' for usage.` | `main.go` (`run`), `render/cli.go` (`InvocationError`) | Excess positional-target diagnostic on stderr. |
| `lookit: <error>` | `main.go` (`run`), `render/cli.go` (`ErrorLine`) | TUI startup failure on stderr. |

## Target Parsing

These errors originate in `finger.ParseTarget` and are surfaced by the CLI as
`lookit: <error>`, and by the TUI input as `error: <error>`.

| Message | Source | Surface |
| --- | --- | --- |
| `empty target` | `finger/query.go:29` | Empty TUI submit or empty one-shot target. |
| `target must be of the form user@host or @host` | `finger/query.go:34` | Invalid target shape after normalization. |
| `missing host after @` | `finger/query.go:39` | Target has `@` but no host. |
| `target contains control characters` | `finger/query.go:45` | Inbound target guard. |

## Network And Query Errors

These errors originate in `finger.Query`. In one-shot mode they are rendered
inside the response chrome by `render.RenderWithBackground`; in the TUI they
appear in the reader viewport for profile/raw result states, or contribute to
the `partial (error)` list flag when a parseable list body was returned.

| Message | Source | Surface |
| --- | --- | --- |
| `dial <host:port>: <error>` | `finger/client.go:51` | Connection/DNS failure. |
| `set deadline: <error>` | `finger/client.go:71` | Connection deadline setup failure. |
| `query user contains control characters` | `finger/client.go:76` | Outbound user guard. |
| `write query: <error>` | `finger/client.go:80` | Failure writing the RFC 1288 query line. |
| `read response timed out after <duration>: <error>` | `finger/client.go:115` | Read timeout after connecting and writing. |
| `read response: <error>` | `finger/client.go:129` | Non-timeout read failure with no body. |

## One-shot Renderer

These messages are produced by `render.RenderWithBackground`, which is used by
both one-shot CLI output and the TUI reader viewport.

| Message | Source | Surface |
| --- | --- | --- |
| `➜ <target> <elapsed> [✦]` | `render/chrome.go:11-20` | Header line. The sparkle appears only on success. |
| `(no response body)` | `render/render.go:27-29` | Successful query with an empty body. |
| `<bytes> · <elapsed>` | `render/chrome.go:23-29` | Footer stats. |
| `truncated` | `render/render.go:40-43` | Footer notice when `finger.Meta.Truncated` is true. |
| `<queryErr.Error()>` | `render/render.go`, `tui/request.go`, `tui/app.go` | Renderer error line following header/body processing for ordinary query errors. Explicit TUI cancellation is suppressed, and any returned partial body is discarded, before rendering. |

## TUI Landing And Input

| Message | Source | Surface |
| --- | --- | --- |
| `ring@thebackupbox.net` | `tui/app.go:30-35` | Rotating placeholder sample in the target input. |
| `@happynetbox.com` | `tui/app.go:30-35` | Rotating placeholder sample in the target input. |
| `@plan.cat` | `tui/app.go:30-35` | Rotating placeholder sample in the target input. |
| `@tilde.team` | `tui/app.go:30-35` | Rotating placeholder sample in the target input. |
| `jonathan@tilde.team` | `tui/app.go:30-35` | Rotating placeholder sample in the target input. |
| `target: ` | `tui/app.go:129` | Target input prompt. |
| `No response yet.` | `tui/reader.go:28`, `tui/app.go:226` | Empty reader viewport on first launch or after returning to landing. |
| `type a target and press ↵ · ? help` | `tui/statusbar.go:31-32` | Landing status bar hint. |
| `error: <parse error>` | `tui/app.go:310-314` | TUI flash message after invalid input submit. |

## TUI Status Bar

| Message | Source | Surface |
| --- | --- | --- |
| `◂ esc: <target>` | `tui/statusbar.go:41-44` | Back breadcrumb when history has a previous node. |
| `esc back` | `tui/app.go:644-649`, `tui/app.go:691` | Back hint. Omitted from joined hints when the breadcrumb already shows the target. |
| `? help` | `tui/app.go:644-649`, `tui/app.go:691` | Help hint. |
| `<spinner> loading <target> · <elapsed> · esc cancel · q quit` | `tui/request.go` | Loading status bar. Elapsed time starts after one second. |
| `r refresh` | `tui/app.go`, `tui/keys.go` | Refresh hint/status and enabled help binding for an ordinary landed reader response or user list. |
| `r retry` | `tui/app.go`, `tui/keys.go` | Retry hint/status and enabled help binding for an empty-body failure or persistent failed-refresh warning. |
| `refresh failed: <error> · showing previous response · r retry` | `tui/request.go`, `tui/app.go` | Persistent status after an empty-body refresh failure; the prior response remains visible. |
| `retry failed: <error> · r retry` | `tui/request.go`, `tui/app.go` | Persistent status after an empty-body retry failure. |
| `↵ go · esc cancel` | `tui/app.go:674-683` | Status bar while editing the target input over existing content. |
| `<n> users` | `tui/app.go:698-700` | List metadata. |
| `↵ go` | `tui/app.go:700-711` | List action hint. |
| `/ filter` | `tui/app.go:700-711` | List action hint. |
| `auto-detected` | `tui/app.go:702-704` | List flag for generic list detection. |
| `v view source` | `tui/app.go` | List hint shown for generic lists. |
| `partial (error)` | `tui/app.go:706-708` | List flag for parseable list bodies returned with an error. |
| `partial (truncated)` | `tui/app.go:708-710` | List flag for parseable list bodies returned truncated. |
| `page <n>/<total>` | `tui/app.go:712-714` | List pagination metadata. |
| `↑↓ scroll` | `tui/app.go:715-719` | Reader scroll hint. |
| `<scroll>%` | `tui/app.go:718-720` | Reader scroll position. |
| `copied <address>` | `tui/app.go:597-604` | TUI flash after copying an address. |

## TUI Help

These are app-level key binding labels. The expanded help panel is generated
from enabled bindings, so not every label is visible in every state.

| Message | Source | Surface |
| --- | --- | --- |
| `i target` | `tui/keys.go:29` | Focus target input. |
| `esc back` | `tui/keys.go:30` | Back/cancel binding. |
| `↵ go` | `tui/keys.go:31` | Submit/open binding. |
| `/ filter` | `tui/keys.go:32` | List filter binding. |
| `v view source` | `tui/keys.go` | Raw/source view binding. |
| `r refresh` / `r retry` | `tui/keys.go`, `tui/app.go` | Contextual refresh/retry binding; disabled outside an idle reader or list screen. |
| `y copy` | `tui/keys.go:34` | Copy address binding. |
| `? help` | `tui/keys.go:35` | Help binding. This is shown in the status bar, not inside the open help panel. |
| `q quit` | `tui/keys.go:36` | Quit binding, disabled while the input is focused. |
| `↑/↓ move` | `tui/keys.go:38` | Movement help. |
| `←/→ page` | `tui/keys.go:39` | Page help. |
| `g/G top/bottom` | `tui/keys.go:40` | Jump help. |

## TUI List

| Message | Source | Surface |
| --- | --- | --- |
| `<host> — <n> users` | `tui/list.go:80` | Bubble list title. Currently hidden by `SetShowTitle(false)`, but still stored on the model. |
| `<name> · <target>` | `tui/list.go:45-49` | User list row description when both name and explicit target are present. |
| `Auto-detected user list from an unrecognized response — press v to view source.` | `tui/list.go` | Preamble note for generic list detection. |
| `List truncated — showing first <max> of <total>` | `tui/list.go:165-170` | Preamble note when parsed list entries exceed `maxListEntries`. |

## Inherited Component Text

The TUI uses `charm.land/bubbles/v2/list`. Most built-in list chrome is hidden
by `tui/list.go:80-83`, but the filter prompt is visible while filtering.
These strings are not authored in this repo, so making them configurable would
either require setting component fields after construction or wrapping the
component.

| Message | Source | Surface |
| --- | --- | --- |
| `Filter: ` | `$GOMODCACHE/charm.land/bubbles/v2@v2.1.0/list/list.go:216-218` | Filter input prompt shown after pressing `/` in a list. |
| `Nothing matched` | `$GOMODCACHE/charm.land/bubbles/v2@v2.1.0/list/list.go:1149-1153` | Built-in filter status text. Status bar is hidden in lookit's list today, so this is not normally visible unless that setting changes. |
| `No items` / `No items.` | `$GOMODCACHE/charm.land/bubbles/v2@v2.1.0/list/list.go:1156-1158`, `$GOMODCACHE/charm.land/bubbles/v2@v2.1.0/list/list.go:1209-1214` | Built-in empty states. Not expected in normal lookit flow because lists are only opened after parsing at least one user. |
| `0 items`, `1 item`, `<n> items`, `<n> filtered` | `$GOMODCACHE/charm.land/bubbles/v2@v2.1.0/list/list.go:1134-1178` | Built-in list status text. Status bar is hidden in lookit's list today. |

## Notes For Future Configurability

- Parse and network errors are currently plain Go errors. Human-friendly copy
  would likely fit best at the presentation layer so callers can still inspect
  wrapped errors with `errors.Is` / `errors.As`.
- `render.RenderWithBackground` is shared by one-shot mode and the TUI reader,
  so copy changes there affect both surfaces.
- TUI status-bar and help copy is state-dependent. Any configurable message
  layer should preserve the keymap enablement rules in `tui/app.go:updateKeymap`.
- Some list text comes from `bubbles/list`; check the exact module version in
  `go.mod` before relying on upstream line numbers.

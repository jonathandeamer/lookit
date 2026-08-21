# User-facing Messages

This is an inventory of app-authored text that can reach users through the CLI,
the body renderer, or the interactive TUI. It excludes arbitrary response body
text returned by finger servers, and it does not duplicate the catalog notes in
`tui/catalog.txt` (those are authored there and compiled in). The goal is to
make future copy changes easier by recording the source locations and the
runtime surface where each message appears.

Source references are starting points, not a permanent API. Search for the
named function or constant when changing copy; line numbers drift.

## CLI

| Message | Source | Surface |
| --- | --- | --- |
| `A finger client built for exploring, not just querying.` plus the structured `Usage:`, `Targets:`, `Options:`, and `Examples:` sections | `render/cli.go` (`Usage`) | `-h`/`--help` output on stdout. |
| `Press ? inside lookit for keyboard shortcuts.` | `render/cli.go` (`Usage`) | First of two closing pointers in `-h`/`--help` output: the keys live in the app, not here. |
| `See man lookit for the full reference.` | `render/cli.go` (`Usage`) | Second closing pointer in `-h`/`--help` output: everything help deliberately omits is in `man/lookit.1`. |
| `lookit version <version> (built <builtAt>)`, or `lookit version <version>` when the build date is unknown | `main.go` (`versionString`) | `-v`/`--version` output on stdout. |
| `lookit: unknown option "<option>"` followed by `Try 'lookit --help' for usage.` | `main.go` (`run`), `render/cli.go` (`InvocationError`) | Unknown option diagnostic on stderr. |
| `lookit: expected at most one target` followed by `Try 'lookit --help' for usage.` | `main.go` (`run`), `render/cli.go` (`InvocationError`) | Excess positional-target diagnostic on stderr. |
| `lookit: <error>` | `main.go` (`run`), `render/cli.go` (`ErrorLine`) | TUI startup failure on stderr. |

## Target Parsing

These errors originate in `finger.ParseTarget` and are surfaced by the CLI as
`lookit: <error>`, and by the TUI input as `error: <error>`.

| Message | Source | Surface |
| --- | --- | --- |
| `empty target` | `finger/query.go` (`parseTarget`) | Empty TUI submit, or a blank CLI seed. |
| `target must be of the form user@host or @host` | `finger/query.go` (`parseTarget`) | Invalid target shape after normalization (no `@`, no scheme, no path). |
| `missing host after @` | `finger/query.go` (`errMissingHost`) | Target has `@` but no host. |
| `target contains control characters` | `finger/query.go` (`parseTarget`) | Inbound target guard (C0/DEL and Cf/Zl/Zp). |
| `IPv6 literals must be bracketed, e.g. [::1]` | `finger/query.go` (`errBracketIPv6`) | Unbracketed IPv6, or a broken `[…]` form. |
| `invalid port` | `finger/query.go` (`parsePort`) | Empty, zero, or out-of-range port. |
| `invalid host/port "…"` | `finger/query.go` (`splitHostPort`) | Bracketed IPv6 with a suffix that is not `:port`. |
| `forwarding through multiple relays is not supported yet` | `finger/query.go` (`errMultipleRelays`) | More than one relay (`user@host@relay@other`). |
| `forwarded host ports are not supported; put a port only on the relay` | `finger/query.go` (`errForwardedHostPort`) | `user@host:port@relay`. |
| `forwarding in finger:// URLs is not supported yet; use user@host@relay` | `finger/query.go` (`errURLForwarding`) | A `finger://` form that embeds forwarding. |
| `forwarded targets must be user@host@relay or @host@relay` | `finger/query.go` (`errMalformedForwarding`) | Two `@` but not a well-formed one-relay query. |
| `forwarded targets from server responses are not opened` | `finger/query.go` (`ErrServerForwarding`) | `ParseTargetPinned` on a forwarded or multi-relay token lifted from a response. Also flashed when copying such a list row. |

## Network And Query Errors

These errors originate in `finger.Query`. Recognised dial and read failures
are classified by `finger.QueryError`; unrecognised ones keep the underlying
text. Ordinary query errors are appended to the body-only reader output by
`render.RenderLayout`; a parseable list body returned with an error
instead contributes the `partial (error)` list flag.

| Message | Source | Surface |
| --- | --- | --- |
| `connection refused by <host:port>` | `finger/errors.go` (`QueryError.Error`) | TCP refused. |
| `no such host: <host>` | `finger/errors.go` (`QueryError.Error`) | DNS name not found. |
| `couldn't look up <host>: <reason>` | `finger/errors.go` (`QueryError.Error`) | Other DNS failure. |
| `network unreachable: <host:port>` | `finger/errors.go` (`QueryError.Error`) | ENETUNREACH. |
| `host unreachable: <host:port>` | `finger/errors.go` (`QueryError.Error`) | EHOSTUNREACH. |
| `no answer from <host:port> after <duration>` | `finger/errors.go` (`QueryError.Error`) | Dial timeout. |
| `<host:port> stopped responding after <duration>` | `finger/errors.go` (`QueryError.Error`) | Read timeout. |
| `couldn't reach <host:port>: <error>` | `finger/errors.go` (`QueryError.Error`) | Unclassified dial failure. |
| `couldn't read from <host:port>: <error>` | `finger/errors.go` (`QueryError.Error`) | Unclassified read failure. |
| `set deadline: <error>` | `finger/client.go` (`queryWith`) | Connection deadline setup failure. |
| `query contains control characters` | `finger/client.go` (`queryWith`) | Outbound query guard. |
| `write query: <error>` | `finger/client.go` (`queryWith`) | Failure writing the RFC 1288 query line. |

## Response Body Renderer

These messages are produced by `render.RenderLayout` for the TUI reader
viewport (`render.RenderWithWidth`, in `render/render.go`, is a thin
compatibility wrapper over it and is no longer called by the reader). The
renderer receives no response metadata and adds no synthetic receipt, target
header, latency, byte-count footer, or truncation footer. Output starts with
the first response-body line, the empty-response treatment, or an error.
`main.go` never renders a response body.

| Message | Source | Surface |
| --- | --- | --- |
| `(no response body)` | `render/layout.go` (`RenderLayout`) | Successful query with an empty body. |
| `<queryErr.Error()>` | `render/layout.go` (`RenderLayout`) | Error line after any returned partial body. Canceled or superseded request results are dropped in `tui/request.go` before they can repaint. |

## TUI Input

| Message | Source | Surface |
| --- | --- | --- |
| `user@host or @host` | `tui/app.go` (`targetPlaceholder`) | Greyed-out hint in the empty target input. Teaches the two shapes and names no destination. |
| `target: ` | `tui/app.go` (`newAppWithContext`) | Target input prompt. |
| `filter: ` | `tui/list.go` (`filterPrompt`, `applyListStyles`) | Filter prompt after `/` on the startpage, user list, and links panel. |
| `No response yet.` | `tui/reader.go` (`newReader`) | The reader's empty viewport. Not shown at launch (launch is the startpage). Reappears if the reader is shown with no current entry. |
| `error: <parse error>` | `tui/app.go` (`submit`) | Persistent flash after an invalid input submit. Not cleared by the flash timer. |
| `error: cannot bookmark: <reason>` | `tui/app.go` (`toggleBookmark`) | Flash when the current target cannot be written as a bookmark record. |
| `wrapping on` / `original layout` | `tui/app.go` (`toggleWrap`) | Transient flash after enabling or disabling word wrapping for the current response. |
| `✓ bookmarked <target>` / `✓ removed <target>` | `tui/app.go` (`toggleBookmark`) | Transient flash after a successful pin or unpin. |
| `copied <address>` | `tui/app.go` (`copyAddress`, About `y`) | Transient flash after copying an address or the issues URL. |
| `nothing to copy` | `tui/app.go` (`copyAddress`) | Transient flash when `y` has no address. |

## TUI Startpage

The launch screen at `pos == -1`. Assembled by `buildSections` from the
embedded catalog and the bookmarks file.

| Message | Source | Surface |
| --- | --- | --- |
| `jonathan@tilde.team` / `fingerverse@happynetbox.com` / `me@andros.dev` | `tui/bookmarks.go` (`starterBookmarks`) | Ordered starter records written to a new bookmarks file and rendered under `BOOKMARKS` on first run. Each remains ordinary, independently deletable user configuration. |
| `YOURS` / `CATALOG` | `tui/start.go` (`overviewOwnershipLabel`, `overviewCatalogLabel`) | Overview line above the list. `YOURS` is deliberately not `BOOKMARKS` — the section header already says that. |
| `none yet` | `tui/start.go` (`overviewView`) | `YOURS` value when the user has no bookmarks. |
| `<n>` | `tui/start.go` (`overviewView`) | `YOURS` value when there is at least one bookmark (a bare count, not `N bookmarks`). |
| `1 community` / `<n> communities` | `tui/start.go` (`overviewView`, `countLabel`) | Catalog community count. |
| `1 service` / `<n> services` | `tui/start.go` (`overviewView`, `countLabel`) | Catalog service count. |
| `BOOKMARKS` / `COMMUNITIES` / `SERVICES` | `tui/sections.go` (`buildSections`) | Section headers. `PEOPLE` is assembled only if the catalog contains a `person` line; the shipped catalog has none. |
| `★` | `tui/start.go` (`startBookmarkMarker`) | Prefix on a row the user pinned themselves. |
| `visited today` / `visited yesterday` / `visited <n> days ago` / `visited 1 month ago` / `visited <n> months ago` / `visited over 1 year ago` | `tui/start.go` (`startRowNote`, `relativeDay`) | Note column on a pinned row with a last-visited date. Buckets are local-zone calendar days; a future stamp clamps to `today`. Blank when the pin has no date, and stands back down to the catalog note while a filter flattens the sections. |
| `no match for “<query>”` | `tui/start.go` (`noMatchMessage`) | First content row while a typed filter matches nothing. Names the query, not the catalog. |
| `No entries.` | `tui/start.go` (`newStart`, bubbles `NoItems`) | List empty chrome. Unreachable while typing a zero-match filter (that state uses `noMatchMessage`); reachable only if the list itself has no items. |
| `No bookmarks yet.` | `tui/app.go` (`startEmptyMessage`) | File-level empty body when the catalog is visible and there is nothing to show. |
| `No bookmarks yet. The catalog is off — remove \`catalog off\` from <path> to see it.` | `tui/app.go` (`startEmptyMessage`) | File-level empty body when `catalog off` is set and there are no bookmarks. |
| `<path>: <reason>` / `<path> line <n>: <reason>` / `… (+<n>)` | `tui/app.go` (`startNotice`) | Bookmark-file problems, named so the user edits the file actually in use. Several problems report the first in full and count the rest; the count is terse because the notice is printed as one unwrapped row. |
| `expected "catalog on" or "catalog off"` | `tui/bookmarks.go` (`parseBookmarks`) | Reason on a bad `catalog` directive. |
| `expected "sort visited" or "sort manual"` | `tui/bookmarks.go` (`parseBookmarks`) | Reason on a bad `sort` directive. |
| `expected a target and an optional date` | `tui/bookmarks.go` (`parseBookmarkTarget`) | Reason on a bookmark line carrying more than a target and a date — most often a description, which belongs after a `#`. |
| `expected a date like 2026-08-14` | `tui/bookmarks.go` (`parseBookmarkTarget`) | Reason on a bookmark line whose date is neither a zero-padded `YYYY-MM-DD` calendar day nor the exact UTC timestamp format written by v0.2.0-beta.1. |
| `target "…" cannot be saved unchanged` | `tui/bookmarks.go` (`validateBookmarkRecordTarget`) | Reason a target cannot be persisted as a bookmark record; surfaced via the `error: cannot bookmark: <reason>` flash. |
| `target is not valid UTF-8` | `tui/bookmarks.go` (`validateTarget`) | Bookmark target is not UTF-8. |
| `target has an invisible character` | `tui/bookmarks.go` (`validateTarget`) | Bookmark target carries C1/Cf/Zl/Zp. |
| `bad target "…": <parse error>` | `tui/bookmarks.go` (`validateTarget`) | Bookmark target failed `ParseTarget`. |
| `cannot locate a config directory: …` / `cannot read: …` / `cannot create: …` | `tui/bookmarks.go` (`loadBookmarks`, `initializeBookmarkData`) | File-level problems with no line number. |

## TUI Status Bar

| Message | Source | Surface |
| --- | --- | --- |
| `◂ esc: <target>` / `◂ esc` | `tui/statusbar.go` (`statusBar.render`) | Back breadcrumb when history has a previous node. The short form is the state-ladder shed of the destination. |
| `esc back` | `tui/app.go` (`joinHints`) | Back hint. Omitted from joined hints when the breadcrumb already shows the target. |
| `? help` | `tui/app.go` (`joinHints`, `startBar`) | Help hint on resting bars that use `joinHints` or `startBar`. The expanded overlay omits `?`. While Help is open, `statusBarModel` clears hints so the bar keeps address, flags, page, scroll, latency, and meta only. |
| `<spinner> loading <target> · <elapsed> · esc cancel · q quit` | `tui/request.go` (`pendingPriorityStatus`) | Loading status. Elapsed time starts after one second. |
| `<landed elapsed>` (`500µs`, `42ms`, `1.50s`) | `tui/statusbar.go` (`formatElapsed`), `tui/app.go` (`buildStatusBar`) | Optional landed-response segment. Shed first on a tight bar. Absent on Start, About, and loading. |
| `r refresh` / `r retry` | `tui/app.go` (`refreshHint`) | Contextual refresh or retry. |
| `refresh failed: <error> · showing previous response · r retry` | `tui/request.go` (`requestFailure.priorityStatus`) | Persistent status after an empty-body refresh failure. |
| `retry failed: <error> · r retry` | `tui/request.go` (`requestFailure.priorityStatus`) | Persistent status after an empty-body retry failure. |
| `type a target and press ↵ · ↓ browse · ? help` | `tui/app.go` (`startBar`) | Startpage, target input focused. |
| `↵ go · b bookmark` / `↵ go · b remove` plus `/ filter · i target · ? help` | `tui/app.go` (`startBar`, `startBookmarkAction`) | Startpage, content focused. `b` names what it will do on the selected row. `↵ go` and `b …` act on the selection, so both are dropped when a filter leaves no row selected. |
| `esc clear filter` | `tui/app.go` (`startBar`) | Replaces `/ filter` on the startpage once a filter is applied — that mode is already open. |
| `1 entry` / `<n> entries` | `tui/app.go` (`startBar`, `countLabel`) | Startpage listing count (visible selectable rows). |
| `page <n>/<total>` | `tui/app.go` (`startBar`, list branch of `buildStatusBar`) | Pagination when the startpage or user list has more than one page. |
| `↵ go · esc cancel` | `tui/app.go` (`buildStatusBar`) | Editing the target over a landed response. |
| `esc back · ? help` | `tui/app.go` (`buildStatusBar`) | View-source. No history breadcrumb: Esc returns to the same node. |
| `type to filter · esc cancel` | `tui/app.go` (`filterModeHints`, `buildStatusBar`) | Any list with `/` just opened and the query still empty: startpage, user list, links panel. |
| `enter apply · esc cancel` | `tui/app.go` (`filterModeHints`, `buildStatusBar`) | Filter being typed with at least one match. |
| `esc cancel` | `tui/app.go` (`filterModeHints`) | Filter being typed that matches nothing. Enter is deliberately unnamed: bubbles refuses to apply a zero-match filter and drops back to the unfiltered list, which is what Esc does, so there is no second action to offer. |
| `↑/↓ move · esc clear filter` plus link actions | `tui/app.go` (`buildStatusBar`) | Links panel, filter applied. |
| `↑/↓ move · / filter · esc back` plus link actions | `tui/app.go` (`buildStatusBar`) | Links panel, resting. Link actions come from `linkActionHints` (`↵ go`, `f go`, `y copy`). No links-panel state shows a history breadcrumb: Esc closes the panel or clears its filter and returns to the same node. |
| `1 user` / `<n> users` | `tui/app.go` (`buildStatusBar`, `countLabel`) | User-list metadata. |
| `512 B` / `1.2 KB` / `3.4 MB` | `tui/statusbar.go` (`formatBytes`), `tui/app.go` (`buildStatusBar`) | Response-body size metadata on the reader and on view-source. Absent on a failed request. |
| `↵ go · / filter` plus `r refresh`/`r retry` | `tui/app.go` (`buildStatusBar`) | User-list resting hints, then `joinHints`. While `/` is open the list uses `filterModeHints` instead, with no history breadcrumb: Esc cancels the filter rather than walking back. |
| `v view source` | `tui/app.go` (`buildStatusBar`) | Extra list hint on a generic (auto-detected) list. |
| `auto-detected` | `tui/app.go` (`buildStatusBar`) | List flag for generic list detection. |
| `partial (error)` | `tui/app.go` (`buildStatusBar`) | List flag for a parseable list body returned with an error. |
| `partial (truncated)` | `tui/app.go` (`buildStatusBar`) | List or reader flag when the body was cut. |
| `↑↓ scroll` | `tui/app.go` (`buildStatusBar`) | Reader resting hint (then `joinHints`). |
| `tab 1 link` / `tab <n> links` | `tui/app.go` (`buildStatusBar`, `countLabel`) | Reader resting hint, ahead of `↑↓ scroll`, only when the body carries links and none is focused. |
| `<scroll>%` | `tui/app.go` (`buildStatusBar`) | Reader scroll position when the body is taller than the viewport. |
| `link <n>/<total> · <kind> · <actions>` | `tui/app.go` (`buildStatusBar`, `linkKindLabel`) | Reader with a focused link. Kinds: `finger`, `address (ambiguous)`, `url`, `email`, `social`, `forwarded finger`. |
| `tab next` | `tui/app.go` (`buildStatusBar`) | Extra hint while a reader link is focused. |
| `cross-relay: finger URL relay does not match current host` / `cross-relay: relay <host> does not match current host` | `tui/links.go` (`DetectLinks`) | `Link.Blocked` text, flashed on Enter and shown in the focused-link bar. |
| `about lookit` | `tui/app.go` (`buildStatusBar`) | About, opened from the startpage: left-side host slot. |
| `↵ go to author · y copy issues URL · esc back · q quit` | `tui/app.go` (`buildStatusBar`) | About, from the startpage (`esc back` only when there is no history breadcrumb). From a landed screen, `esc back` is omitted because `◂ esc: <origin>` is shown instead. |

## TUI Help

These are app-level key binding labels. The expanded Help popover shows enabled
bindings in priority order, so not every label is visible in every state or at
every terminal height. Its displayed primary gesture does not remove hidden
runtime aliases.

| Message | Source | Surface |
| --- | --- | --- |
| `i target` | `tui/keys.go` | Focus target input. |
| `esc back` | `tui/keys.go` | Back/cancel binding. |
| `↵ go` | `tui/keys.go` | Submit/open binding. |
| `/ filter` | `tui/keys.go` | Filter binding. |
| `v view source` | `tui/keys.go` | Raw/source view binding. |
| `w wrap` / `w unwrap` | `tui/keys.go`, `tui/app.go` (`updateKeymap`) | Contextual reader-display binding; available only for a non-empty body in normal reader view. |
| `r refresh` / `r retry` | `tui/keys.go`, `tui/app.go` (`refreshHelp`) | Contextual refresh/retry; disabled outside an idle reader or list. |
| `y copy` | `tui/keys.go` | Copy address binding. |
| `? help` | `tui/keys.go` | Help binding. Shown in the status bar on resting screens, not inside the open overlay. |
| `q quit` | `tui/keys.go` | Quit binding, disabled while the input is focused. |
| `↑/↓ move` | `tui/keys.go` | Movement help in the lists (startpage, user list, links panel). |
| `↑/↓ scroll` | `tui/help.go` (`moveHelpBinding`) | The same binding relabelled in the reader. Matches the reader's status-bar hint. |
| `←/→ page` | `tui/keys.go` | Page help. |
| `h home` | `tui/keys.go` | Open the startpage. |
| `↓ browse` | `tui/keys.go` | Move from the focused target input into the startpage list; Tab remains an accepted alias. |
| `b bookmark` / `b remove` | `tui/keys.go`, `tui/app.go` (`updateKeymap`) | Add or remove the current target. On the startpage the label follows `startBookmarkAction`. |
| `L browse links` | `tui/keys.go` | Open or close the detected-links panel. |
| `a about lookit` | `tui/keys.go` | Open About from content or from Help, including Help over the focused input. |
| `f go` | `tui/help.go` (`linksPanelHelpCandidates`) | Finger the selected ambiguous Finger link from the links panel. |
| `tab next link` | `tui/keys.go` | Focus the next detected reader link; `n` remains an accepted alias. |
| `shift+tab previous link` | `tui/keys.go` | Focus the previous detected reader link; `N` remains an accepted alias. |

`g/G top/bottom` exists on `keyMap.Jump` but is intentionally omitted from Help.

## TUI About

| Message | Source | Surface |
| --- | --- | --- |
| `A modern TUI browser for the finger protocol` | `tui/about.go` (`aboutTagline`) | Tagline under the wordmark. |
| `lookit <version> · MIT license` | `tui/about.go` (`aboutView`) | Identity line. |
| `built <date>` | `tui/about.go` (`aboutView`) | Build-date row; hidden when the date is empty or `unknown`. |
| `https://github.com/jonathandeamer/lookit` | `tui/about.go` (`aboutRepo`) | Repository URL. |
| `Built with Charm` + `https://charm.sh` | `tui/about.go` (`aboutView`) | Credit. |
| `Young software; bug reports & ideas welcome` | `tui/about.go` (`aboutView`) | Personality line. |
| `Catalog inspired by` + `https://640kb.neocities.org/fingerverse/` | `tui/about.go` (`aboutView`) | Catalog credit. |
| `finger jonathan@tilde.team` / `↵ go` | `tui/about.go` (`aboutView`) | First action. |
| `Report a bug or idea` / `y copy issues URL` | `tui/about.go` (`aboutView`) | Second action. Copies `aboutIssuesURL`. |
| `Thanks for supporting the small internet` | `tui/about.go` (`aboutView`) | Closing line. |

## TUI List

| Message | Source | Surface |
| --- | --- | --- |
| `<host> — 1 user` / `<host> — <n> users` | `tui/list.go` (`newList`) | Bubble list title. Hidden by `SetShowTitle(false)`, but still stored on the model. |
| `<name> · <target>` | `tui/list.go` (`userItem.Description`) | User-list row description when both name and explicit target are present. |
| `Auto-detected user list from an unrecognized response — press v to view source.` | `tui/list.go` (`newListWithPreamble`) | Preamble note for generic list detection. |
| `List truncated — showing first <max> of <total>` | `tui/list.go` (`newListWithPreamble`) | Preamble note when parsed list entries exceed `maxListEntries`. |
| `1 link` / `<n> links` | `tui/linkspanel.go` (`newLinksPanel`) | Links-panel title. Hidden by `SetShowTitle(false)`. |

## Inherited Component Text

The TUI uses `charm.land/bubbles/v2/list`. Most built-in list chrome is hidden
(title, status bar). These leftover strings are not authored in this repo.

| Message | Source | Surface |
| --- | --- | --- |
| `Nothing matched` | `charm.land/bubbles/v2/list` | Built-in filter status text. Status bar is hidden in lookit's lists today, so this is not normally visible unless that setting changes. |
| `No items` / `No items.` | `charm.land/bubbles/v2/list` | Built-in empty states. The startpage overrides the noun to `entry`/`entries` (`No entries.`). User lists are only opened after parsing at least one user. |
| `0 items`, `1 item`, `<n> items`, `<n> filtered` | `charm.land/bubbles/v2/list` | Built-in list status text. Status bar is hidden in lookit's lists today. |

## Notes For Future Configurability

- Network errors are classified in `finger.QueryError`. Recognised failures
  get lookit's own sentence; unrecognised ones keep the underlying text.
  `errors.Is` / `errors.As` still unwrap to the original net error.
- `render.RenderLayout` owns only the TUI reader's body/empty/error
  presentation; response metadata and transient request state belong to the
  status bar.
- TUI status-bar and help copy is state-dependent. Any configurable message
  layer should preserve the keymap enablement rules in `tui/app.go`
  (`updateKeymap`).
- Some list text comes from `bubbles/list`; check the exact module version in
  `go.mod` before relying on upstream identifiers.
- Catalog notes are authored in `tui/catalog.txt` and enforced by
  `TestCatalogNotesFitTheNoteColumn`. Do not copy them here.

# lookit — design

A beautiful, modern, usable TUI finger client in Go. Showcases the charmbracelet stack.

## Goals

- Make finger feel alive in 2026: a polished CLI for one-shot queries, a TUI reader for browsing, watch-and-diff subscriptions, and a curated catalog so new users can find people to follow.
- Be a credible showcase for the charmbracelet stack — code, demo, and aesthetics worth sharing.
- Stay small and composable: each layer (finger, render, store) usable independently and testable without a network or a terminal.

## Non-goals

- Hosting finger servers, posting `.plan` files, or otherwise writing to the fingerverse. Read-only.
- Supporting the deprecated multi-hop `user@host1@host2` form (RFC 1288 §2.5.2 deprecates it for privacy).
- Running on Pi Zero W / armv6. TUI on a single-core armv6 is sad. macOS arm64 and Linux amd64 only.
- Windows. No test surface for users who don't exist.
- A background daemon. Polling is on-demand only.

## Stack

- **fang** — CLI dispatcher. Introduced in Phase 2 when we add a second subcommand. Not used in MVP.
- **bubbletea** + **bubbles** + **lipgloss** — TUI framework, components (list, viewport, textinput), styling/layout.
- **colorprofile** — terminal color-capability detection (TrueColor / ANSI256 / ANSI / Ascii / NoTTY) plus automatic color downsampling. Used from the MVP. Replaces hand-rolled `term.IsTerminal` + `NO_COLOR` handling and ensures rainbow accents degrade gracefully on 8/16-color terminals.
- **huh** — interactive forms for "add subscription," "pick a server," etc. Phase 3.
- **glamour** — markdown rendering. Used selectively in Phase 3+ (the `.plan` body is never auto-markdownified in the MVP).
- **log** — Charm's colorful structured logger. Phase 2+ when `lookit refresh` and the TUI grow `--verbose` output and we want logs that match the app's aesthetic. MVP uses stdlib `log`.
- **VHS** — recording the README demo gif. Build-time only. Phase 4.
- **freeze** — static PNG screenshots of terminal output for the README. Build-time only. Phase 4.
- Standard library only for `finger/` networking and `store/` JSON persistence.

## Phasing

### Phase 1 — CLI MVP

One binary, two invocations, no fang yet (added in Phase 2):

- `lookit user@host` — finger a user on a server.
- `lookit @host` — finger the server itself (banner / user list).
- `lookit user@host:port` — custom port (useful for testing and the rare non-79 server).

Output: chrome + structured field highlighting (see Rendering below). Body bytes pass through verbatim — ASCII art in `.plan` files is never reflowed or restyled.

Out of scope for MVP: fang, TUI, persistence, catalog, subscriptions, glamour rendering, `--json` output, configuration file.

### Phase 2 — TUI

`lookit` with no arguments opens the TUI reader. `lookit user@host` still works and prints to stdout (does not open the TUI). Fang is introduced here as the top-level dispatcher because we now have:

- `lookit` → TUI
- `lookit get user@host` → explicit one-shot (same as bare `lookit user@host`)
- `lookit version`

Reuses `finger/` and `render/` from Phase 1 unchanged.

### Phase 3 — Subscriptions & catalog

- `lookit subscribe add user@host` / `lookit subscribe list` / `lookit subscribe rm user@host`
- `lookit refresh` — fetches all subscriptions, diffs against stored last-seen, marks new entries as "changed since last seen." On-demand only; no daemon. README documents cron/launchd recipes.
- `lookit discover` and an "Explore" tab in the TUI — browses the bundled catalog.
- Catalog: `catalog.json` embedded via `go:embed`, optionally refreshed from a versioned URL.

### Phase 4 — Polish

VHS-recorded demo gif for README, possible Homebrew tap, refinements that fall out of dogfooding.

## Aesthetic

Charm-default base (pink / cyan / charcoal) with nyancat-inspired rainbow accents used sparingly:

- "New content" / "changed since last seen" highlights in subscriptions.
- Success sparkle on the finished header.
- Optional rainbow gradient on section dividers in the TUI.

No literal nyancat features — no mascot, no easter egg command, no rainbow trail animation. The palette is the inspiration; the application is restrained.

Adaptive light/dark is out of scope for MVP. Phase 2+ can add it.

## Targets

- `darwin/arm64` (primary)
- `linux/amd64`

Cross-compiled from macOS; release builds via goreleaser in Phase 4.

## Component layout (Phase 1)

```
lookit/
├── main.go                  # arg parsing (stdlib flag), wiring
├── finger/
│   ├── client.go            # dial, send query, read response, timeouts
│   ├── query.go             # build RFC 1288 query line
│   └── client_test.go       # against an in-process fake server
├── render/
│   ├── theme.go             # lipgloss styles
│   ├── chrome.go            # header/footer
│   ├── fields.go            # structured field highlighter
│   └── render_test.go       # golden tests
├── go.mod
└── README.md
```

### Why these boundaries

- `finger/` knows nothing about styling. Returns `(body []byte, meta Meta)` where `Meta` is `{Addr, Elapsed, Bytes, Truncated}`. Testable without a terminal.
- `render/` knows nothing about networking. Takes body + meta and returns a styled string. Testable without a network.
- `main.go` is wiring only: parse args → call finger → call render → print. Stays under ~50 lines.

### Phase 2-3 additions (forward-looking)

- `tui/` — Bubble Tea program; reuses `finger/` and `render/` unchanged.
- `store/` — JSON files under `$XDG_STATE_HOME/lookit/` (default `~/.local/state/lookit/` on Linux, `~/Library/Application Support/lookit/` on macOS). Subscriptions list and per-subscription last-seen body files.
- `catalog/` — embedded `catalog.json` via `go:embed`, plus optional remote refresh.
- Fang introduced at top-level.

The MVP isn't a throwaway. The split between `finger/` and `render/` is chosen specifically so the TUI in Phase 2 can reuse them with zero refactoring.

## Data flow (MVP)

### The finger request (RFC 1288)

1. Parse the argument. Three shapes:
   - `user@host` → query line = `user\r\n`, dial `host:79`
   - `@host` → query line = `\r\n`, dial `host:79`
   - `user@host:port` → custom port
2. Dial with a 10s connect timeout using `net.Dialer.DialContext` with a `context.WithTimeout`. The connect timeout is purely about reaching the server.
3. Set a 30s overall read deadline on the connection using `conn.SetDeadline(time.Now().Add(30 * time.Second))`. `context.WithTimeout` alone does **not** interrupt a blocking `net.Conn.Read` — a server that accepts and then hangs would otherwise block forever. Additionally, launch a small goroutine that closes the conn if the caller's context is cancelled, so cancellation propagates promptly.
4. Send the query line. **Never send `/W`** (the verbose flag) by default — RFC 1288 §2.5.5 calls it out as privacy-sensitive and most modern servers ignore it. Add `--verbose` later if anyone asks.
5. Read until EOF, the read deadline fires, or 1 MiB is reached (defensive — finger has no length marker and a hostile server could stream forever). Set `Truncated = true` on the meta **only** if the size cap or read deadline triggers. Plain TCP EOF (including EOF mid-line) is the normal end of a finger response and is **not** truncation — finger has no in-protocol "response complete" sentinel; the server signals end-of-response by closing the connection.
6. Normalize line endings: convert `\r\n` → `\n`. Preserve all other bytes byte-for-byte. **Do not decode bytes as UTF-8** — some servers emit Latin-1 or other encodings. Pass bytes through; the terminal handles display.

### Rendering pipeline

```
bytes from finger/  →  split into lines  →  for each line:
                                              ├── matches a known field prefix? color the label
                                              └── else: print verbatim
                                            →  wrap with header + footer chrome
                                            →  write to stdout
```

- Header: `➜ user@host` in pink, dim latency on the right, a sparkle on success.
- Footer: bytes received, elapsed, truncation notice if applicable.
- Field prefixes (small allow-list): `Login:`, `Name:`, `Plan:`, `Project:`, `Office:`, `Office Phone:`, `Home Phone:`, `Directory:`, `Shell:`, `Last login`, `No Plan.`, `On since`.
- Critically: matched lines still print their content verbatim. We only style the label, never the data after the colon. This prevents ever mangling `.plan` content.

### TTY detection & color profile

Use `colorprofile.Detect(os.Stdout, os.Environ())` to determine what the terminal supports. The returned profile is one of `TrueColor`, `ANSI256`, `ANSI`, `Ascii`, or `NoTTY`. This single call handles:

- Stdout-not-a-terminal (piped, redirected) → `NoTTY` → render plain text. Required for `lookit user@host | grep` to work.
- `NO_COLOR=1` → `Ascii` → no styling.
- Limited-color terminals → downsample our pink/cyan/charcoal + rainbow accents to whatever the terminal can render via `profile.Convert(color)`.

Render code stores colors as `lipgloss.Color` values and runs them through the detected profile before emitting.

## Error handling

| Failure | Exit code | Display |
|---|---|---|
| DNS resolve fails | 2 | `✗ user@host` + dim red `couldn't resolve host` |
| Connect refused | 2 | `✗ user@host` + dim red `connection refused` |
| Connect timeout | 2 | `✗ user@host` + dim red `timed out after 10s` |
| Read timeout mid-response | 2 | Header + partial body + dim red footer `response cut off after 30s` |
| Empty response | 0 | Header + dim `(no response body)` + footer |
| Response > 1 MiB | 0 | Header + truncated body + dim yellow footer `truncated at 1 MiB` |
| Invalid arg | 64 (`EX_USAGE`) | `usage: lookit user@host` to stderr |

Exit codes follow `sysexits.h` so the tool composes well in pipelines.

## Testing

### `finger/` package

Integration tests against an in-process fake finger server (`net.Listen` on `127.0.0.1:0`, random port). No real network in CI. Tests run in milliseconds.

Cases:

- Happy path: user query, server query (`@host`), custom port.
- Server hangs forever after accepting → 30s read deadline fires; returns timeout error with `Truncated = true` on whatever partial body arrived.
- Caller context cancelled mid-read → conn-close goroutine fires; `Query` returns promptly with context error.
- Server sends > 1 MiB → response truncated cleanly at the cap, `Truncated = true`, no panic.
- Server sends a partial response then closes (including EOF mid-line, no trailing newline) → body returned as-is, `Truncated = false`. TCP close is the normal end of a finger response.
- Server sends Latin-1 / non-UTF-8 bytes → bytes pass through unchanged.

### `render/` package

Golden tests under `render/testdata/`:

```
basic.input            basic.golden
plan-ascii-art.input   plan-ascii-art.golden   # ASCII art survives untouched
empty-response.input   empty-response.golden
no-color.golden                                 # NO_COLOR=1 case
```

`-update` flag pattern to regenerate goldens for intentional changes. Right approach for styling code — reading ANSI escapes by eye in test assertions is awful.

### Out of scope for tests

- Real-world finger servers (flaky, slow, ethically dubious to hammer in CI).
- Lipgloss internals.
- `main.go` wiring (covered transitively by manual smoke tests / Phase 4 integration scripts if we add them).

### CI

GitHub Actions, free tier, runs on push:

- `go test ./...`
- `go vet ./...`
- Formatting check: `test -z "$(gofmt -l .)"` — fails (non-zero exit) if any file would be reformatted. (Plain `gofmt -d` prints the diff but exits 0, so it can't gate CI on its own.)

## Open questions

- Catalog distribution URL — own-domain vs github raw vs gist. Resolve in Phase 3.
- Adaptive light/dark theming — defer until Phase 2 TUI work surfaces real need.
- Homebrew tap vs `go install` — Phase 4 decision.

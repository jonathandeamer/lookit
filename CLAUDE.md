# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`lookit` is a finger client (RFC 1288, TCP/79) for the modern terminal: a one-shot CLI and an interactive Bubble Tea TUI, built with Charm tools.

## Commands

```bash
go build -o lookit .                         # build the binary
go test ./...                                # all tests
go test ./... -race                          # race build (matches CI)
go test ./tui/ -run TestEnterInListDrills -count=1 -v   # single test
go vet ./...                                 # CI gate
gofmt -l .                                   # CI gate: must print nothing
```

CI (`.github/workflows/ci.yml`) runs exactly `go vet ./...`, a `gofmt -l .` emptiness check, and `go test ./... -race`. Run all three locally before committing — `gofmt -w .` to fix formatting. Tests use injected fakes and never hit the network (see below), so they're fast and offline.

Run the app: `./lookit` (TUI), `./lookit user@host[:port]` or `./lookit @host[:port]` (one-shot), `./lookit version`. The TUI requires a real TTY and can't be smoke-tested headlessly. Release version info is injected via `-ldflags "-X main.version=… -X main.builtAt=…"`.

## Architecture

Three packages with a strict one-way dependency direction — `finger/` → `render/` → `tui/` — wired together by `main.go`.

- **`finger/`** — networking only, no UI deps. `ParseTarget` (defaults the port to `:79`), `Query` → `(body []byte, Meta, error)`. The client caps the body at 1 MiB, applies a read timeout, normalizes CRLF→LF, and treats a connection reset *after* the body as success (marking `Meta.Truncated` only when the body was cut mid-line, i.e. no trailing newline). `Meta` carries `Addr/Bytes/Truncated/Elapsed`.
- **`render/`** — a pure function `Render(target, body, meta, queryErr, profile) string`. It keys off `colorprofile.Profile`, so piped / `NO_COLOR` output is plain text and TTY output gets themed chrome + field highlighting. This is the single rendering path used by **both** the one-shot CLI and the TUI viewport. Note: `render/` deliberately uses the **v1** `github.com/charmbracelet/lipgloss`; do not migrate it to v2.
- **`tui/`** — a glow-style state machine on Bubble Tea **v2** (`charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2` — note the `charm.land` v2 import paths, not `github.com/charmbracelet`). `Run()` in `run.go` is the only exported entry point, so `main.go` never touches the TUI internals.

`main.go`'s `run(args, stdout, stderr) int` is the testable router: no args → TUI; one `user@host`/`@host` arg → one-shot (`finger.Query` → `render.Render` → stdout); `version`; `-h/--help`. `startTUI` and `runOneShotFunc` are package vars so tests stub them. Exit codes: 0 ok, 2 network, 64 usage.

### TUI internals (the parts that span files)

- **`appModel` (app.go)** is the top-level model. It owns lifecycle (Init/Update/View), quit, and routing between two sub-models via a `state` enum (`stateReader` | `stateList`). `commonModel` holds shared width/height/profile/fetch. The sub-models — **`readerModel` (reader.go)** (target input + viewport + status) and **`listModel` (list.go)** (a `bubbles/v2/list` of a host's users) — do *not* own quit/lifecycle; `appModel` drives them through small methods (`setSize`, `setEntry`, `setLoading`, `selected`, `filtering`).
- **`routeFetch` (app.go) is the single decision point** for a completed fetch: a host response (`Target.User == ""`, plus the special-cased `ring@thebackupbox.net`) that `ParseUsers` recognizes opens the list; everything else renders in the reader. Errored/truncated responses that still carry a parseable body open the list too, flagged `(incomplete)` in the title.
- **`ParseUsers` (userlist.go)** is a pure, dependency-free parser that recognizes whether a host response contains a selectable list. It tries cue/header-gated matchers in order — generic columnar / grid / marker, then service-specific menus (typed-hole, sava.rocks, redterminal, the Finger Ring, telehack) — dedups, and **declines** (returns false) rather than guessing. It is validated by a golden corpus of real server captures in `userlist_test.go`, with both parse and decline cases.
- **Drilling & navigation:** Enter on a list user fingers `login@host`; entries that carry an explicit `User.Target` (from `finger://` links or `finger user@host` commands in the response) drill cross-host. **Safety:** server-supplied targets are pinned to port 79 (`pinFingerPort`) so a hostile response can't steer lookit at another service; user-typed ports are preserved. Back navigation is two-level via `state`+`fromList`; Ctrl+C always quits, Esc is context-dependent. `handleKey`/`drill` return a concrete `appModel`, and `Update` adopts the returned model even when a key isn't fully handled (so e.g. the `fromList` reset survives).

### Testing approach

Networking and the TUI program are injected, never real: `tui` uses `FetchFunc`/`fetchCmd` (fetch.go) so model tests drive transitions with a stub fetch; `finger` tests spin up a local `net.Listen` server; `main` tests stub `startTUI`/`runOneShotFunc`. Follow these patterns rather than adding real I/O to tests.

## Conventions

- **Commit messages: Conventional Commits, and do NOT add `Co-Authored-By` or Codex trailers** — this project deliberately omits them (overrides the usual default).
- The local `~/bubbletea`, `~/bubbles`, `~/lipgloss` clones are reference material only; the versions resolved in `go.mod` are authoritative. After adding a Charm dependency, run `go mod tidy`.
- Design rationale lives in `docs/superpowers/specs/` (specs) and `docs/superpowers/plans/` (implementation plans), phase by phase — consult them for the "why" behind a decision before changing behavior.

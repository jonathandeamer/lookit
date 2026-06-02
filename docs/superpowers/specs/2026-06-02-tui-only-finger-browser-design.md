# TUI-only finger browser design

## Goal

Collapse lookit from a dual-mode tool (a one-shot Unix filter **and** an
interactive TUI) into a single interactive **finger browser** — Amfora /
Bombadillo / lynx class. After this change there is no headless render path: the
binary always opens the TUI. A target given on the command line is an *address
to open*, not a query to dump.

This is a deliberate **category change**, not a cleanup. lookit stops being a
composable shell utility and becomes an interactive application. The motivation
is identity: a coherent single experience regardless of how it is invoked, in
line with the smallnet browsers CLAUDE.md already cites as convention references
(Amfora, and Bombadillo — which also speaks `finger://`).

### Accepted tradeoffs

These are the price of the pivot, accepted on purpose:

- **No pipe / redirection composition.** `lookit user@host | grep` and
  `> file.txt` stop working; the TUI owns the terminal and needs a real TTY.
- **No scripting / non-TTY use.** cron, CI, and non-interactive `ssh host
  lookit ...` can no longer drive a finger query.
- **No meaningful per-result exit code.** A failed fetch is shown *in the app*,
  not signalled to the shell. The only non-zero exit becomes a genuine
  startup/usage failure.

The divergence is deliberate: lynx-style `-dump`/`-source`/`-listonly` escape
hatches are exactly the "browser that is also a pipe" model we are rejecting. No
`--print`/`--raw` flag is added.

## Non-goals

- **No cobra / fang / new dependencies.** lookit remains a hand-rolled
  single-argument app (the `2026-06-01-styled-cli-chrome-design` decision still
  stands; this change does not justify reversing it).
- **No new flags** beyond `-h`/`--help` and `-v`/`--version`.
- **No terminal window-title manipulation.** (`tea.View.WindowTitle` is
  available and was considered; declined — the bottom status bar already carries
  the current target, and changing the user's terminal title is unwanted.)
- **No change to the security model.** `finger.ParseTarget` remains the single
  untrusted-input chokepoint; server-supplied target port-pinning
  (`pinFingerPort`) is untouched. The chokepoint simply moves wholly inside the
  TUI's submit handler.
- **No protocol / parsing / rendering-of-finger-body changes.** Only the entry
  surface and the chrome around it change.

## Why not cobra / viper / fang

The pivot makes the command surface *smaller* — one optional positional and two
flags (`-h`/`--help`, `-v`/`--version`), zero subcommands — which weakens, not
strengthens, the case for a CLI framework. Each tool earns its weight on a
feature we just deleted or never had:

- **cobra** exists for command *trees*; we have one command. Its best
  discoverability win (shell completions) is near-worthless here — no
  subcommands to complete, and a `user@host` target isn't completable. It would
  also replace the ~30-line table-testable `run(...)` router and wants to own
  the exit path right after we collapsed to `{0, 1}`.
- **viper** manages layered config; lookit has *zero* configuration today and
  this spec adds none. It would solve a problem we don't have (YAGNI).
- **fang** is the Charm-native CLI skin, and is the genuinely tempting one — but
  it *requires* cobra (it wraps it), and it styles to fang's theme, whereas
  `render/cli.go` (shipped in `2026-06-01-styled-cli-chrome`) already styles the
  CLI to lookit's own `Theme`. Adopting fang means taking the cobra weight *and*
  deleting recent working code in exchange for *less* palette coherence with the
  TUI — a regression on the one thing fang is meant to win.

**Decision: keep the hand-rolled launcher.** A TUI-only lookit's "CLI" is
honestly just a launcher; a small router that calls `tui.Run` represents that
more truthfully than a one-command cobra tree.

**Revisit trigger:** the day lookit grows genuine subcommands (e.g. `lookit
bookmarks`, `lookit config`) or a persistent config/bookmarks store, cobra+viper
begin to pay for themselves and fang becomes the right Charm-native skin
(superseding `render/cli.go`). Not before. This is the same line
`2026-06-01-styled-cli-chrome` drew; the pivot does not move it.

## Architecture after the change

The three-package direction (`finger/` → `render/` → `tui/`, wired by
`main.go`) is unchanged. What changes is the *shape of the entry surface*:

- `main.go` becomes a thin launcher: it recognises `-h`/`--help` and
  `-v`/`--version`, and otherwise launches the TUI — optionally seeded with the
  one positional argument. It performs **no** `finger.ParseTarget` itself.
- `tui.Run` gains options carrying the initial query string and the version
  line. The TUI is the only execution mode.
- `render/` keeps its styled CLI helpers for the two surviving CLI surfaces
  (`--help`, `--version`) and the launch-error line; its one-shot finger-body
  footer path retires.

## CLI router (`main.go`)

New `run(args, stdout, stderr) int` behaviour, hand-rolled flag scan:

| invocation | behaviour | exit |
|---|---|---|
| `-h` / `--help` (any position) | print `render.Usage(...)` to **stdout** | `0` |
| `-v` / `--version` (any position) | print `render.Version(versionString(), ...)` to **stdout** | `0` |
| no positional arg | launch TUI, empty initial query | `0` on clean quit |
| exactly one positional arg | launch TUI seeded with that **raw, unparsed** string | `0` on clean quit |
| 2+ positional args / unknown flag | print `render.Usage(...)` to **stderr** | `1` |
| TUI returns an error | print `render.ErrorLine(...)` to **stderr** | `1` |

Explicit `--help`/`--version` exit `0` (a small correction from today's `64`;
explicitly-requested help is success). The exit-code set collapses to **`{0,
1}`**: `exitOK` and a single `exitError = 1`. `exitNetwork` (2) and `exitUsage`
(64) are deleted — they encoded a Unix-utility contract this binary no longer
honours. Ctrl-C quits cleanly and exits `0` (unchanged, charm-native).

Deleted from `main.go`: `runOneShot`, `runOneShotFunc`, `exitCodeFor`, the
`version` subcommand branch, and the `exitNetwork`/`exitUsage` constants.
Retained: `version`/`builtAt` ldflags vars and `versionString()` (now feed both
the `--version` flag and the TUI), `detectProfile` (per-writer styling for the
`--help`/`--version`/error surfaces), and the `startTUI` seam — which gains
parameters but stays stub-able for tests.

## Seeding the TUI (approach A — replay as a typed submission)

`main.go` does no parsing. It passes the raw `args[0]` into the TUI, which treats
it **exactly** as if the user typed it into the landing input and pressed Enter.
The landing input already has both branches for interactive use — see
`submit()` (`tui/app.go:314`):

- parses cleanly → `blurInput()` + `startFetch(...)`; `routeFetch` then decides
  reader vs. list. The user never sees the landing.
- fails to parse → sets `m.flash` to the error and **keeps the input focused**
  with the bad text still in it, ready to edit. This is exactly the desired
  "open pre-filled, show the error" behaviour — no special pre-made-error path.

"Typed" and "seeded" are the same code path by construction, which also keeps a
single `ParseTarget` chokepoint.

**Wiring (Bubble Tea v2 constraint).** `Init()` cannot mutate the persistent
model — Bubble Tea keeps the model passed to `NewProgram` and only takes
`Init`'s returned `Cmd`. So:

- `tui.Run(ctx, profile, Options{InitialQuery, Seed, Version})`; `newApp` takes
  the same options.
- **`Seed bool` is tracked separately from `InitialQuery`.** "A positional arg
  was supplied" is not the same as "the arg is non-empty": `lookit ""` and
  `lookit "   "` supply an arg whose value is blank, and the empty string is
  *also* the no-arg `InitialQuery`. So `main` sets `Seed = true` whenever exactly
  one positional arg was given (blank or not), and leaves it `false` for the
  no-arg launch. When `Seed` is true, `newApp` sets the landing input's value to
  `InitialQuery` (visible/editable immediately) and keeps focus.
- `Init()` emits a one-shot `seedSubmitMsg` **iff `Seed` is true** (not based on
  whether the input is non-blank); `Update` handles it with
  `return m, m.submit()`, running the seed through the normal mutation flow.
  Because `submit()` on a blank value yields the same parse-error flash that
  pressing Enter on the empty landing produces interactively, a blank seeded arg
  consistently shows that error rather than silently landing — one rule, no
  special case for blank.

`tui.Options` is a small struct (room to grow without churning the `Run`
signature again).

## Version surfaces

The `version` subcommand is replaced by **two** surfaces, both light:

1. **`-v`/`--version` flag** — prints `render.Version(versionString(), profile)`
   and exits `0`. This is the single strongest convention for this app class
   (both Amfora and Bombadillo keep a version flag) and remains useful for bug
   reports. `render.Version` therefore **survives** (its consumer moves from the
   subcommand to the flag).

2. **A band on the `?` help panel** — `main` passes `versionString()` into
   `tui.Options.Version`; the help panel gains:
   - a **title band** at the top: `lookit <version>` + the one-line tagline
     "A modern TUI browser for the Finger protocol",
   - the existing keybinding groups (unchanged),
   - a **footer pointer**: `finger · RFC 1288 · github.com/jonathandeamer/lookit`.

   No new screen, no new navigation, no new state — the panel is already the
   conventional home for meta-info and is universally discovered via `?`. Its
   height is measured dynamically (`helpHeight`/`resizeForHelp` already call
   `lipgloss.Height(m.helpView())`), so the added rows are accommodated
   automatically.

(The tagline text is fixed as above; exact footer-pointer punctuation is a plan
detail.)

## `render/` cleanup entailed by the pivot

- **`render/cli.go`:** `Usage` is edited — replace the `version` subcommand
  example line with a `--version` line, and append a dim "press ? in lookit for
  keys" pointer. The resulting block is:

  ```
  usage:
    lookit
    lookit user@host[:port]
    lookit @host[:port]
    lookit --version

  press ? in lookit for keys
  ```

  `Version` and `ErrorLine` are **kept** (now consumed by the `--version` flag
  and the launch-error path respectively).
- **`render/render.go`:** the footer-on path existed *for* the one-shot CLI
  (`render/render.go:11-12`); the TUI already renders footerless and surfaces
  truncation in its status bar (`tui/app.go:726`). With one-shot gone there is
  no footer-on consumer, so the `footer` option, `WithoutFooter`, `renderFooter`,
  and `fmtBytes` are **deleted**, and `RenderWithBackground` becomes footerless.
  The `Render` convenience wrapper is **kept** (a grep confirms it is still called
  by render-package tests — `render/tildeteam_test.go` and `render/render_test.go`);
  it simply delegates to the now-footerless `RenderWithBackground`. (`fmtElapsed`
  stays — the header still uses it.)

## Things audited and deliberately left unchanged

The pivot does not disturb these, and they were checked to confirm they remain
correct/native: alt-screen on (`tui/app.go:880`); mouse not captured (per
CLAUDE.md); OSC-52 clipboard via the `setClipboard` seam; `NO_COLOR` /
`colorprofile` honoured throughout; Ctrl-C → quit → exit `0`.

## Testing

- **tui:**
  - seeded-valid: `newApp` with a valid `InitialQuery` + stub `FetchFunc`; drive
    `Init`→`seedSubmitMsg`→`submit`→`fetchResultMsg`; assert the result lands in
    the reader (or list, for a host listing) and the landing was skipped.
  - seeded-invalid: `InitialQuery` that fails `ParseTarget`; assert the input
    retains the text, the flash shows the error, focus stays on the input, and
    no fetch is issued.
  - help-panel band: assert the rendered `?` view contains the version line and
    the footer pointer.
- **main:** no-arg → `startTUI` called with empty query; one arg → called with
  that query + version; `-h`/`--help` → usage to stdout, exit `0`; `-v`/
  `--version` → version line to stdout, exit `0`; 2+ args / unknown flag →
  usage to stderr, exit `1`; `startTUI` error → `ErrorLine` to stderr, exit
  `1`. The one-shot and `version`-subcommand tests are deleted; the
  `detectProfile`-pinning seam from the styled-CLI spec keeps the exact-string
  assertions hermetic.
- **render:** drop any footer-on test; update the `Usage` test for the new
  block; keep the `Version`/`ErrorLine` tests.
- `make check` is the final gate.

## Deferred: licensing & packaging (out of scope)

Recorded here so it is not lost, but **explicitly deferred** — it is a
packaging/release-layer concern, not part of this entry-surface change, and per
CLAUDE.md ("keep specs focused") it does not belong in this spec's
implementation. None of it touches the version-surfacing UI.

- **lookit's own license: already done.** `LICENSE` is MIT (Copyright (c) 2026
  Jonathan Deamer). No action needed. (For the record, an earlier draft of this
  spec wrongly claimed there was no license; there is.)
- **`README.md` license stub:** the `## License` section currently reads
  `TBD.` — should point at the MIT `LICENSE`. Trivial, standalone, not blocking.
- **Third-party notices:** shipping release binaries that statically link
  MIT/BSD/Apache dependencies carries those licenses' redistribution clause (the
  same clause lookit's own MIT license contains). Best practice is a generated
  `THIRD_PARTY_NOTICES` file shipped with releases — e.g. via
  `google/go-licenses`, ideally wired into goreleaser if/when release tooling is
  added. This is a *distribution* obligation, satisfied by a file, **not** an
  in-app credits/licenses screen and **not** the `?` help band or `--version`
  output. At most an optional one-line pointer from `--version`; not required.

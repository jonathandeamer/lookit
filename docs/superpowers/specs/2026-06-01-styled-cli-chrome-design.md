# Styled CLI chrome design

## Goal

Give lookit's non-TUI command surface — the help text, the version line, and
CLI-level error messages — the same themed, Charm-native polish the TUI and the
one-shot finger output already have. This is the first thing a CLI user sees on
`lookit --help`, `lookit version`, or a mistyped target, so it should look
designed rather than like default `fmt.Fprintln` output.

The chosen approach is **deliberately not fang/cobra.** lookit is a
single-command app (one optional positional target, plus `version`); adopting
cobra + fang adds two heavyweight dependencies (and a third lipgloss module),
replaces the small hand-rolled router, and forces an exit-code workaround to
preserve sysexits codes — all to style three low-traffic surfaces we can style
ourselves in a few dozen lines using the palette we already ship. So this pass
hand-rolls the styling in `render/`, reusing the existing theme.

## Non-goals

- **No cobra, no fang, no new dependencies.**
- **No behaviour or surface changes.** Same `run(args, stdout, stderr) int`
  router, same `startTUI`/`runOneShotFunc` seams, same `version` subcommand,
  same `-h`/`--help` handling, same exit codes (0 ok, 2 network, 64 usage).
- **No change to the one-shot finger rendering** (`render.Render`) or the TUI.
- No new flags (`--verbose`, `--json`, completions, manpages). If the CLI later
  grows real flags/subcommands, revisit cobra+fang then — not now (YAGNI).

## Approach

Add small **pure styling functions to the `render` package**, where the
one-shot output already lives. They build from the existing v1
`render.NewTheme(profile)` (so colour, light/dark adaptation, and
`colorprofile` downsampling all match the finger-response rendering) and are
called from `main.go` in place of the current plain prints.

**Critical invariant — plain output is byte-identical to today.** On
`Ascii`/`NoTTY` profiles, each function returns exactly the current plain text.
This keeps the common piped/redirected case grep-friendly and lets the existing
tests assert exact strings.

**One documented exception: forced colour.** `colorprofile.Detect` honours
`CLICOLOR_FORCE` (and friends), so it returns a colour profile *even for a
non-TTY writer or a pipe* when the user has explicitly forced colour. That is
correct, idiomatic behaviour and lookit keeps it — the invariant therefore
holds "absent a forced-colour signal," not unconditionally. The consequence is
that the *tests* must not depend on ambient environment (a developer or CI with
`CLICOLOR_FORCE=1` would otherwise get ANSI and fail exact-string assertions);
this is handled by a detection seam, below — not by reading the real
environment in tests.

## Components

New file `render/cli.go`, three pure functions keyed off `colorprofile.Profile`:

- `Usage(profile colorprofile.Profile) string` — the usage block. Plain form is
  byte-identical to the current `printUsage` output:

  ```
  usage:
    lookit
    lookit user@host[:port]
    lookit @host[:port]
    lookit version
  ```

  (trailing newline included). Styled form: `usage:` in the dim `Footer` style,
  the `lookit` command token in the `Target` accent, and the
  `user@host`/`@host` example tokens in the `Field` accent.

- `Version(line string, profile colorprofile.Profile) string` — styles a
  pre-formatted version line. Plain form returns `line` unchanged. Styled form
  accents the leading `lookit` token (`Target`) and dims the remainder
  (`Footer`). The line is always produced by the existing `versionString()`
  (`"lookit %s (built %s)"`), which remains the single source of the format.

- `ErrorLine(msg string, profile colorprofile.Profile) string` — returns
  `lookit: <msg>`. Plain form is exactly `"lookit: " + msg`. Styled form renders
  it in the red `ErrLine` style. (Callers add the trailing newline via
  `Fprintln`, matching today's `fmt.Fprintf(..., "lookit: %v\n", err)`.)

These reuse `Theme`'s existing fields (`Target`, `Field`, `Footer`, `ErrLine`);
no new palette plumbing. `NewTheme` already returns a no-colour theme for
`Ascii`/`NoTTY`, so the plain-output invariant falls out of the theme rather
than special-casing each function — but each function is still tested for exact
plain output.

## main.go wiring

- `printUsage(w)` becomes `fmt.Fprint(w, render.Usage(profile))`.
- The version path becomes
  `fmt.Fprintln(stdout, render.Version(versionString(), profile))`.
- Both `fmt.Fprintf(stderr, "lookit: %v\n", err)` sites become
  `fmt.Fprintln(stderr, render.ErrorLine(err.Error(), profile))`.

**Profile detection is per-writer, through a stub-able seam.** Detection goes
through a package var `detectProfile = colorprofile.Detect` (mirroring the
existing `startTUI`/`runOneShotFunc` seams). `run` detects from the writer it is
about to use — `detectProfile(stdout, os.Environ())` for the version line,
`detectProfile(stderr, os.Environ())` for help and errors — rather than a
hardcoded `os.Stdout`. In production `main` passes the real
`os.Stdout`/`os.Stderr` (a TTY, or `CLICOLOR_FORCE`, resolves to a colour
profile → styled output). The one-shot path keeps detecting against `os.Stdout`
as it does today.

The seam exists so tests are **hermetic with respect to the environment**: the
`main_test.go` cases that assert exact plain strings set
`detectProfile = func(io.Writer, []string) colorprofile.Profile { return colorprofile.NoTTY }`
(restored via `t.Cleanup`), so they produce plain output regardless of whether
the developer's or CI's environment has `CLICOLOR_FORCE`/`NO_COLOR` set. This is
the controlled-environment fix the per-writer approach requires; without it the
exact-string assertions would be ambient-env-dependent.

## Testing

- `render/cli_test.go`:
  - `Usage(colorprofile.NoTTY)` equals the exact current usage block; `Version`
    plain equals its input; `ErrorLine(NoTTY, "x")` equals `"lookit: x"`.
  - Truecolor variants contain the literal text after ANSI-stripping, and
    contain the expected accent SGR (e.g. the `Target` foreground for `lookit`).
- `main_test.go`: the exit-code/seam tests (no-arg → TUI, TUI failure → 2,
  invalid-target → 64) are unchanged. The tests that assert exact CLI text
  (usage block, `version` line, error line) pin `detectProfile` to `NoTTY` via
  the seam so they are deterministic regardless of ambient
  `CLICOLOR_FORCE`/`NO_COLOR`. Add one test that, with `detectProfile` pinned to
  `TrueColor`, the usage/version/error output is styled (contains ANSI), proving
  the wiring passes the profile through.
- `make check` is the final gate.

## Accepted residual risk

`Version`/`ErrorLine` styling keys off known string structure (the version line
always starts with `lookit `; the error is `lookit: <msg>`). This is acceptable
because both strings are produced in-process by lookit itself, never from
untrusted input. Perceived colour after ANSI/ANSI256 downsampling varies by
terminal, mitigated as elsewhere by testing the truecolour source and
preserving byte-identical plain output.

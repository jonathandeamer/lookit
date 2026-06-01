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
`Ascii`/`NoTTY` profiles (which is what a non-TTY writer, `NO_COLOR`, or a pipe
resolves to), each function returns exactly the current plain text. This keeps
piped output grep-friendly and lets every existing test pass unchanged.

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

**Profile detection is per-writer.** `run` detects the profile from the writer
it is about to use — `colorprofile.Detect(stdout, os.Environ())` for the
version line, `colorprofile.Detect(stderr, os.Environ())` for help and errors —
rather than a hardcoded `os.Stdout`. In production `main` passes the real
`os.Stdout`/`os.Stderr` (a TTY resolves to a colour profile → styled output);
in tests `run` receives `bytes.Buffer`s (not a TTY → `NoTTY` → plain output),
so the existing exact-string assertions in `main_test.go` keep passing with no
changes. The one-shot path keeps detecting against `os.Stdout` as it does today.

## Testing

- `render/cli_test.go`:
  - `Usage(colorprofile.NoTTY)` equals the exact current usage block; `Version`
    plain equals its input; `ErrorLine(NoTTY, "x")` equals `"lookit: x"`.
  - Truecolor variants contain the literal text after ANSI-stripping, and
    contain the expected accent SGR (e.g. the `Target` foreground for `lookit`).
- `main_test.go` is unchanged and must still pass (buffers → plain): usage text,
  `version` output, invalid-target → 64, no-arg → TUI, TUI failure → 2.
- `make check` is the final gate.

## Accepted residual risk

`Version`/`ErrorLine` styling keys off known string structure (the version line
always starts with `lookit `; the error is `lookit: <msg>`). This is acceptable
because both strings are produced in-process by lookit itself, never from
untrusted input. Perceived colour after ANSI/ANSI256 downsampling varies by
terminal, mitigated as elsewhere by testing the truecolour source and
preserving byte-identical plain output.

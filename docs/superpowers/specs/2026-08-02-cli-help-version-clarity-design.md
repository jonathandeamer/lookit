# CLI help and version clarity design

## Goal

Make `lookit --help` self-explanatory to someone who does not already know
finger, while keeping the command surface compact and friendly to terminals,
pipes, and scripts. Improve `--version` and invalid-invocation output at the
same time so every non-TUI CLI path says plainly what it is showing or what
went wrong.

The result remains a small, hand-built interface. lookit still has one optional
positional target, two flags, no subcommands, and no need for a CLI framework.

## Decision and alternatives

Three approaches were considered:

1. **Compact, structured help (chosen):** add a description plus conventional
   `Usage`, `Targets`, `Options`, and `Examples` sections; keep uncommon syntax
   to a short note; make invocation errors specific; and label the version
   value explicitly.
2. **Minimal refinement:** retain the current list of usage forms and add only
   a tagline, annotations, and the short flag aliases. This is smaller, but it
   leaves target syntax scattered and supported URL/forwarding forms hard to
   discover.
3. **Comprehensive reference:** enumerate every accepted direct, URL,
   path-style, forwarded, port, and IPv6 form. This is complete but too dense
   for a single-command TUI; it obscures the common paths.

The structured approach follows the hierarchy familiar from Charm's Cobra- or
Fang-backed tools without adopting either dependency. It gives a newcomer
enough context to start and leaves parser edge cases to validation errors and
the longer-form project documentation.

## Help output

The exact plain `--help` output is:

```text
A finger client built for exploring, not just querying.

Usage:
  lookit [TARGET]

Targets:
  user@host[:port]    look up a person
  @host[:port]        browse a host
  finger://host/user  open a finger URL

  Ports default to 79. One-relay forwarding is also supported.

Options:
  -h, --help       show help
  -v, --version    show version

Examples:
  lookit jonathan@tilde.team
  lookit @plan.cat

Press ? inside lookit for keyboard shortcuts.
```

The opening line deliberately reuses the README's positioning: lookit is for
exploration rather than a sequence of one-off queries. `look up a person` is
plain language for newcomers and does not claim that finger defines a formal
profile or page type.

The three target rows describe the most useful shapes. The forwarding note
makes that capability discoverable without making relay syntax compete with
the ordinary path. Path-style targets and IPv6 remain accepted but are not
listed: they are parser capabilities rather than necessary getting-started
material.

`lookit [TARGET]` replaces four repeated usage lines. The optional brackets
communicate that no target opens the browser, while the `Targets` section
explains what supplying one does. Both flag aliases are shown rather than
listing only `--version` and omitting help from the usage block.

The final `?` line preserves the boundary between CLI help and the TUI's
context-aware key help. `--help` explains how to launch lookit; it does not
duplicate changing in-app keybindings.

## Version output

`-v` and `--version` continue to print one line to stdout and exit successfully.
The line gains an explicit `version` label:

```text
lookit version 0.2.0 (built 2026-08-02)
```

When the build date is absent or `unknown`, the suffix remains omitted:

```text
lookit version v0.2.0
```

Development builds render as `lookit version dev`. The underlying version
value is not normalized, so the existing difference between Go module versions
(`v0.2.0`) and GoReleaser-injected versions (`0.2.0`) is preserved. The output
stays single-line and stable enough for ordinary shell inspection; no verbose,
JSON, commit, platform, or Go-toolchain variant is added.

## Invalid invocation diagnostics

Invalid invocations no longer print the entire help block without identifying
the problem. They print one concrete error and a short route to help on stderr,
then exit with status 1.

Unknown option:

```text
lookit: unknown option "--bogus"
Try 'lookit --help' for usage.
```

More than one positional target:

```text
lookit: expected at most one target
Try 'lookit --help' for usage.
```

This matches Glow's preference for silencing automatic usage dumps on command
errors. The user learns the consequence first and can request the full help
when needed. TUI startup errors keep their current single `lookit: <error>`
line because a usage hint cannot help with a terminal initialization failure.

Help and version retain their current precedence wherever their flags appear:
encountering `-h`/`--help` or `-v`/`--version` prints that requested output and
returns success rather than validating other arguments. Unknown dash-prefixed
tokens are options, not targets, matching current behavior.

## Rendering and routing

`render/cli.go` remains the owner of non-TUI CLI presentation:

- `Usage(profile)` builds the new structured help block.
- `Version(line, profile)` continues to style the preformatted version line.
- `ErrorLine(message, profile)` remains the one-line startup-error renderer.
- A new pure invocation-error helper builds the two-line diagnostic and help
  hint without changing startup errors.

The precise helper name is an implementation detail. Its important boundary is
that `main.run` supplies the semantic error message while `render` owns the
wording, spacing, trailing newline, and styles for the complete block.

The existing adaptive theme is reused. On a colour-capable terminal:

- section headings and supporting notes use the dim footer style;
- `lookit` command tokens use the target accent;
- target and option syntax use the field accent;
- invocation error lines use the existing error style; and
- the `Try 'lookit --help'` hint is dimmed.

Descriptions remain ordinary foreground text for readability. On `Ascii` and
`NoTTY` profiles, output is exactly the plain text shown in this spec. Forced
colour continues to follow `colorprofile.Detect`, as it does today.

`main.run` keeps its hand-written router and per-writer profile detection. It
switches unknown-option and excess-target branches from `Usage` to the new
invocation diagnostic. No argument parser, dependency, target parser,
networking path, or TUI state changes.

## Testing

Tests cover text, styling, routing, streams, and status:

- `render.Usage(NoTTY)` equals the exact approved help block, including blank
  lines and its trailing newline.
- ANSI-stripping the styled help yields that same block, and styled output
  contains ANSI sequences.
- plain and styled version tests cover the added `version` word; an unknown
  build date still omits the build suffix.
- the new invocation-error helper has exact plain-output and ANSI-stripped
  styled-output tests.
- `-h` and `--help` write only to stdout and return 0.
- `-v` and `--version` write only to stdout and return 0.
- an unknown-option routing test passes `--bogus` and asserts the exact stderr
  diagnostic includes `unknown option "--bogus"`, proving the reported token
  is the option the user actually supplied; it returns 1.
- multiple positional targets write their exact diagnostic only to stderr and
  return 1.
- invalid invocations do not contain the full `Usage:` block.
- TUI startup failures retain their existing one-line error behavior.

Tests continue to pin the colour profile so ambient `NO_COLOR` or
`CLICOLOR_FORCE` values cannot change exact assertions.

## Documentation and non-goals

Update `docs/user-facing-messages.md` so its CLI inventory reflects the new
help, version, and invocation-error copy. The README already contains the
long-form explanation and representative commands; changing its usage section
is not required for this focused pass.

This change does not add flags, shell completions, man pages, environment
documentation, machine-readable version output, target aliases, or new target
forms. It does not duplicate TUI key help in the CLI and does not adopt Cobra or
Fang.

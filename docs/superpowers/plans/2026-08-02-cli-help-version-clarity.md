# CLI Help and Version Clarity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make lookit's help self-explanatory to finger newcomers, label version output explicitly, and replace undiagnosed usage dumps with precise invocation errors.

**Architecture:** Keep the existing hand-written router in `main.go` and the pure, profile-aware CLI presentation functions in `render/cli.go`. Extend those boundaries with structured help and a dedicated two-line invocation-error renderer; do not introduce a CLI framework or change target parsing, networking, or TUI behavior.

**Tech Stack:** Go, `github.com/charmbracelet/colorprofile`, Lip Gloss v1 in `render`, the standard `testing` package, and the repository's Makefile gates.

## Global Constraints

- Spell the protocol name lowercase as `finger` in all new user-facing copy.
- The plain help, version, and diagnostic strings in the approved spec are exact, including capitalization, spacing, blank lines, quoting, and trailing newlines.
- `-h`/`--help` and `-v`/`--version` write to stdout and return 0; invocation errors write to stderr and return 1.
- Plain `Ascii`/`NoTTY` output contains no ANSI; styled output must reduce to the exact plain text after ANSI stripping.
- Preserve `colorprofile.Detect` forced-colour behavior and per-writer detection.
- Keep the router hand-written. Add no dependency, flag, subcommand, parser behavior, network behavior, or TUI behavior.
- Keep the release build date when known and omit it when empty or `unknown`; do not normalize the underlying version value.
- Do not commit, push, or open a PR unless the user separately authorizes that action during execution.

## File map

- `render/cli.go` — owns the exact help, version styling, startup-error styling, and new invocation-error presentation.
- `render/cli_test.go` — owns byte-exact plain-output and ANSI-preservation tests for the CLI renderers.
- `main.go` — owns argument routing, stream/exit behavior, semantic diagnostic messages, and version-string assembly.
- `main_test.go` — owns routing tests, including the actual unknown option token, aliases, streams, and exit statuses.
- `docs/user-facing-messages.md` — inventories the revised CLI-authored copy and its runtime surfaces.

---

### Task 1: Render the structured help block

**Files:**
- Modify: `render/cli_test.go`
- Modify: `render/cli.go`

**Interfaces:**
- Consumes: `NewTheme(profile colorprofile.Profile) Theme` and its existing `Target`, `Field`, and `Footer` styles.
- Produces: `Usage(profile colorprofile.Profile) string`, with byte-exact plain output and equivalent styled output.

- [ ] **Step 1: Replace the expected plain usage fixture**

In `render/cli_test.go`, replace `plainUsage` with the approved block:

```go
const plainUsage = "A finger client built for exploring, not just querying.\n" +
	"\n" +
	"Usage:\n" +
	"  lookit [TARGET]\n" +
	"\n" +
	"Targets:\n" +
	"  user@host[:port]    look up a person\n" +
	"  @host[:port]        browse a host\n" +
	"  finger://host/user  open a finger URL\n" +
	"\n" +
	"  Ports default to 79. One-relay forwarding is also supported.\n" +
	"\n" +
	"Options:\n" +
	"  -h, --help       show help\n" +
	"  -v, --version    show version\n" +
	"\n" +
	"Examples:\n" +
	"  lookit jonathan@tilde.team\n" +
	"  lookit @plan.cat\n" +
	"\n" +
	"Press ? inside lookit for keyboard shortcuts.\n"
```

Keep `TestUsagePlainIsByteIdentical` and `TestUsageStyledKeepsTextAddsAnsi` unchanged: together they assert exact plain text, a trailing newline, ANSI presence in TrueColor, and exact text after stripping.

- [ ] **Step 2: Run the focused tests and verify the new fixture fails**

Run:

```bash
go test ./render -run 'TestUsage' -count=1 -v
```

Expected: both usage tests fail because `Usage` still returns the old lowercase `usage:` block.

- [ ] **Step 3: Replace `Usage` with the structured renderer**

In `render/cli.go`, replace `Usage` with:

```go
// Usage returns lookit's CLI help block. On no-colour profiles
// (Ascii/NoTTY) the Theme styles are no-ops, so the output is byte-identical
// to the plain help text; on colour profiles headings, commands, syntax, and
// supporting notes use the existing adaptive theme.
func Usage(profile colorprofile.Profile) string {
	t := NewTheme(profile)
	cmd := t.Target.Render("lookit")
	var b strings.Builder
	fmt.Fprintln(&b, "A finger client built for exploring, not just querying.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, t.Footer.Render("Usage:"))
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Field.Render("[TARGET]"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, t.Footer.Render("Targets:"))
	fmt.Fprintf(&b, "  %s    look up a person\n", t.Field.Render("user@host[:port]"))
	fmt.Fprintf(&b, "  %s        browse a host\n", t.Field.Render("@host[:port]"))
	fmt.Fprintf(&b, "  %s  open a finger URL\n", t.Field.Render("finger://host/user"))
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "  %s\n", t.Footer.Render("Ports default to 79. One-relay forwarding is also supported."))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, t.Footer.Render("Options:"))
	fmt.Fprintf(&b, "  %s       show help\n", t.Field.Render("-h, --help"))
	fmt.Fprintf(&b, "  %s    show version\n", t.Field.Render("-v, --version"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, t.Footer.Render("Examples:"))
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Field.Render("jonathan@tilde.team"))
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Field.Render("@plan.cat"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, t.Footer.Render("Press ? inside lookit for keyboard shortcuts."))
	return b.String()
}
```

Do not use width formatting such as `%-18s` on already-styled strings: ANSI bytes would be counted as visible width. The literal padding above keeps the three target descriptions and two option descriptions aligned in both plain and styled output.

- [ ] **Step 4: Run the focused tests and verify they pass**

Run:

```bash
gofmt -w render/cli.go render/cli_test.go
go test ./render -run 'TestUsage' -count=1 -v
```

Expected: both usage tests pass.

---

### Task 2: Label the version line explicitly

**Files:**
- Modify: `main_test.go`
- Modify: `main.go`
- Modify: `render/cli_test.go`

**Interfaces:**
- Consumes: package variables `version` and `builtAt`, plus `render.Version(line string, profile colorprofile.Profile) string`.
- Produces: `versionString() string` returning `lookit version <value>` with the existing optional build suffix.

- [ ] **Step 1: Change the main-package version expectations**

Update the three affected expectations in `main_test.go`:

```go
func TestVersionString(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() { version, builtAt = oldVersion, oldBuiltAt })
	version = "0.2.0"
	builtAt = "2026-05-29"
	if got, want := versionString(), "lookit version 0.2.0 (built 2026-05-29)"; got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}

func TestVersionStringOmitsUnknownBuildDate(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() { version, builtAt = oldVersion, oldBuiltAt })
	version = "v0.1.0"
	builtAt = "unknown"
	if got, want := versionString(), "lookit version v0.1.0"; got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}
```

In `TestRunVersionFlag`, change the exact stdout expectation to:

```go
if got, want := stdout.String(), "lookit version dev\n"; got != want {
	t.Fatalf("stdout = %q, want %q", got, want)
}
```

In `TestRunVersionFlagStyled`, change the stripped-output assertion to:

```go
if got := ansi.Strip(stdout.String()); got != "lookit version dev\n" {
	t.Fatalf("stripped version = %q, want %q", got, "lookit version dev\n")
}
```

- [ ] **Step 2: Run the focused tests and verify they fail**

Run:

```bash
go test . -run 'TestVersionString|TestRunVersionFlag' -count=1 -v
```

Expected: the tests fail because `versionString` still omits the word `version`.

- [ ] **Step 3: Update `versionString` and align renderer fixtures with real input**

Replace `versionString` in `main.go` with:

```go
func versionString() string {
	if builtAt == "" || builtAt == "unknown" {
		return "lookit version " + version
	}
	return fmt.Sprintf("lookit version %s (built %s)", version, builtAt)
}
```

In both `TestVersionPlainIsInputVerbatim` and `TestVersionStyledKeepsTextAddsAnsi` in `render/cli_test.go`, use the actual caller shape:

```go
const line = "lookit version 1.2.3 (built 2026-05-29)"
```

`render.Version` itself requires no change: it already accents the leading `lookit` token and dims the complete remainder.

- [ ] **Step 4: Run version tests and verify they pass**

Run:

```bash
gofmt -w main.go main_test.go render/cli_test.go
go test . -run 'TestVersionString|TestRunVersionFlag' -count=1 -v
go test ./render -run 'TestVersion' -count=1 -v
```

Expected: all listed tests pass, including the preserved `v0.1.0` value and unknown-date omission.

---

### Task 3: Add a dedicated invocation-error renderer

**Files:**
- Modify: `render/cli_test.go`
- Modify: `render/cli.go`

**Interfaces:**
- Consumes: `ErrorLine(message string, profile colorprofile.Profile) string` and the existing `Footer` style.
- Produces: `InvocationError(message string, profile colorprofile.Profile) string`, including both trailing newlines.

- [ ] **Step 1: Add exact plain and styled renderer tests**

Append to `render/cli_test.go`:

```go
const plainInvocationError = "lookit: unknown option \"--bogus\"\n" +
	"Try 'lookit --help' for usage.\n"

func TestInvocationErrorPlain(t *testing.T) {
	if got := InvocationError(`unknown option "--bogus"`, colorprofile.NoTTY); got != plainInvocationError {
		t.Fatalf("InvocationError(NoTTY) =\n%q\nwant\n%q", got, plainInvocationError)
	}
}

func TestInvocationErrorStyledKeepsTextAddsAnsi(t *testing.T) {
	out := InvocationError(`unknown option "--bogus"`, colorprofile.TrueColor)
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("styled invocation error has no ANSI:\n%q", out)
	}
	if got := ansi.Strip(out); got != plainInvocationError {
		t.Fatalf("stripped invocation error =\n%q\nwant\n%q", got, plainInvocationError)
	}
}
```

- [ ] **Step 2: Run the focused tests and verify the missing interface fails**

Run:

```bash
go test ./render -run 'TestInvocationError' -count=1 -v
```

Expected: compilation fails with `undefined: InvocationError`.

- [ ] **Step 3: Implement the pure two-line renderer**

Append to `render/cli.go`:

```go
// InvocationError returns a specific CLI argument error followed by a short
// route to the full help. Unlike ErrorLine, it includes trailing newlines.
func InvocationError(message string, profile colorprofile.Profile) string {
	t := NewTheme(profile)
	var b strings.Builder
	fmt.Fprintln(&b, ErrorLine(message, profile))
	fmt.Fprintln(&b, t.Footer.Render("Try 'lookit --help' for usage."))
	return b.String()
}
```

Keep `ErrorLine` unchanged so TUI startup failures remain one line and its callers continue to own their newline.

- [ ] **Step 4: Run the renderer tests and verify they pass**

Run:

```bash
gofmt -w render/cli.go render/cli_test.go
go test ./render -run 'TestInvocationError|TestErrorLine' -count=1 -v
```

Expected: plain, styled, and existing startup-error renderer tests all pass.

---

### Task 4: Route invalid invocations to precise diagnostics

**Files:**
- Modify: `main_test.go`
- Modify: `main.go`

**Interfaces:**
- Consumes: `render.InvocationError(message string, profile colorprofile.Profile) string` from Task 3.
- Produces: exact unknown-option and excess-target behavior from `run(args []string, stdout, stderr io.Writer) int`.

- [ ] **Step 1: Cover both help aliases and their stream/status contract**

Replace `TestRunHelp` in `main_test.go` with:

```go
func TestRunHelpFlags(t *testing.T) {
	pinProfile(t, colorprofile.NoTTY)
	for _, flag := range []string{"-h", "--help"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{flag}, &stdout, &stderr)
			if code != exitOK {
				t.Fatalf("exit code = %d, want %d", code, exitOK)
			}
			if !strings.Contains(stdout.String(), "Usage:") {
				t.Fatalf("stdout = %q, want help block", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}
```

- [ ] **Step 2: Add the missing unknown-option regression test**

Add this test immediately before `TestRunTooManyArgs`:

```go
func TestRunUnknownOptionIdentifiesFlag(t *testing.T) {
	pinProfile(t, colorprofile.NoTTY)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--bogus"}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if got, want := stderr.String(), "lookit: unknown option \"--bogus\"\nTry 'lookit --help' for usage.\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
```

The exact assertion is intentional: it proves the diagnostic interpolates the actual bad token `--bogus`, rather than returning a generic unknown-option message.

- [ ] **Step 3: Tighten the excess-target and startup-error tests**

Replace `TestRunTooManyArgs` with:

```go
func TestRunTooManyArgs(t *testing.T) {
	pinProfile(t, colorprofile.NoTTY)
	var stdout, stderr bytes.Buffer
	code := run([]string{"a@b", "c@d"}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if got, want := stderr.String(), "lookit: expected at most one target\nTry 'lookit --help' for usage.\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
```

Replace `TestRunTUIFailure` with:

```go
func TestRunTUIFailure(t *testing.T) {
	pinProfile(t, colorprofile.NoTTY)
	stubStartTUI(t, errors.New("terminal unavailable"))
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if got, want := stderr.String(), "lookit: terminal unavailable\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
```

- [ ] **Step 4: Run the routing tests and verify only the diagnostic cases fail**

Run:

```bash
go test . -run 'TestRun(HelpFlags|UnknownOptionIdentifiesFlag|TooManyArgs|TUIFailure)' -count=1 -v
```

Expected: help aliases and the tightened startup error pass; unknown-option and excess-target tests fail because `run` still emits the full help block.

- [ ] **Step 5: Replace usage dumps with semantic invocation errors**

In the unknown-option branch in `main.go`, replace the `render.Usage` call with:

```go
if strings.HasPrefix(a, "-") {
	fmt.Fprint(stderr, render.InvocationError(fmt.Sprintf("unknown option %q", a), errProfile))
	return exitError
}
```

Replace the excess-target branch with:

```go
if len(positional) > 1 {
	fmt.Fprint(stderr, render.InvocationError("expected at most one target", errProfile))
	return exitError
}
```

Do not change the `-h`/`--help`, `-v`/`--version`, malformed positional target, blank target, or TUI startup-error branches.

- [ ] **Step 6: Run the main-package tests and verify they pass**

Run:

```bash
gofmt -w main.go main_test.go
go test . -count=1
```

Expected: all main-package tests pass. The exact unknown-option assertion includes `--bogus`, invalid invocation output contains no `Usage:` block, and TUI startup failure remains one line.

---

### Task 5: Update the message inventory and run all gates

**Files:**
- Modify: `docs/user-facing-messages.md`

**Interfaces:**
- Consumes: the final strings and function ownership from Tasks 1–4.
- Produces: documentation matching the implemented non-TUI CLI surface.

- [ ] **Step 1: Replace the CLI message table with the current surface**

Replace the table under `## CLI` in `docs/user-facing-messages.md` with:

```markdown
| Message | Source | Surface |
| --- | --- | --- |
| `A finger client built for exploring, not just querying.` plus the structured `Usage:`, `Targets:`, `Options:`, and `Examples:` sections | `render/cli.go` (`Usage`) | `-h`/`--help` output on stdout. |
| `Press ? inside lookit for keyboard shortcuts.` | `render/cli.go` (`Usage`) | Final pointer in `-h`/`--help` output. |
| `lookit version <version> (built <builtAt>)`, or `lookit version <version>` when the build date is unknown | `main.go` (`versionString`) | `-v`/`--version` output on stdout. |
| `lookit: unknown option "<option>"` followed by `Try 'lookit --help' for usage.` | `main.go` (`run`), `render/cli.go` (`InvocationError`) | Unknown option diagnostic on stderr. |
| `lookit: expected at most one target` followed by `Try 'lookit --help' for usage.` | `main.go` (`run`), `render/cli.go` (`InvocationError`) | Excess positional-target diagnostic on stderr. |
| `lookit: <error>` | `main.go` (`run`), `render/cli.go` (`ErrorLine`) | TUI startup failure on stderr. |
```

Do not broaden this task into a rewrite of the older TUI inventory or historical one-shot sections; that staleness predates and is independent of this CLI copy change.

- [ ] **Step 2: Format and inspect the complete diff**

Run:

```bash
make fmt
git diff --check
git diff -- main.go main_test.go render/cli.go render/cli_test.go docs/user-facing-messages.md
```

Expected: formatting and whitespace checks pass; the diff contains only the approved help/version/diagnostic changes and their tests/documentation. Preserve the user's unrelated untracked `docs/superpowers/specs/2026-08-02-request-controls-design.md`.

- [ ] **Step 3: Run the complete CI-equivalent gate**

Run:

```bash
make check
```

Expected: `go vet ./...`, the `gofmt -l .` emptiness check, `golangci-lint run ./...`, and `go test ./... -race` all pass.

- [ ] **Step 4: Manually inspect the three CLI surfaces**

Run:

```bash
go run . --help
go run . --version
go run . --bogus
```

Expected:

- help matches the approved structured block;
- version is one line beginning `lookit version`;
- the bad option is quoted as `--bogus`, the help hint follows it, and `go run` reports the program's nonzero exit after the two application-authored lines.

- [ ] **Step 5: Stop at the verified working tree unless commit authorization is explicit**

Report the changed files and verification results. Do not commit, push, open a PR, or enable auto-merge without a separate explicit request. If the user authorizes a commit, use:

```bash
git add main.go main_test.go render/cli.go render/cli_test.go docs/user-facing-messages.md docs/superpowers/specs/2026-08-02-cli-help-version-clarity-design.md docs/superpowers/plans/2026-08-02-cli-help-version-clarity.md
git commit -m "feat(cli): clarify help and version output"
```

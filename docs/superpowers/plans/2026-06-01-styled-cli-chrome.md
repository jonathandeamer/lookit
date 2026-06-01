# Styled CLI Chrome Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Style lookit's help, version, and CLI error output with the existing palette — no cobra/fang, no behaviour or exit-code changes, and byte-identical plain output when not styled.

**Architecture:** Add three pure styling functions to the `render` package (built from the existing v1 `render.NewTheme(profile)`), and call them from `main.go` in place of the current plain prints. Profile is detected per-writer through a stub-able `detectProfile` package var so tests are hermetic regardless of ambient `CLICOLOR_FORCE`/`NO_COLOR`. The router, `startTUI`/`runOneShotFunc` seams, `version` subcommand, and exit codes 0/2/64 are unchanged.

**Tech Stack:** Go, `github.com/charmbracelet/lipgloss` v1 (via `render`'s `Theme`), `github.com/charmbracelet/colorprofile`, `github.com/charmbracelet/x/ansi` (tests only).

**Spec:** `docs/superpowers/specs/2026-06-01-styled-cli-chrome-design.md`

---

## File Structure

- **Create `render/cli.go`** — `Usage`, `Version`, `ErrorLine`: pure functions turning a `colorprofile.Profile` into styled (or, on no-colour profiles, byte-identical plain) CLI chrome strings. Reuses `Theme`'s existing fields; no new palette.
- **Create `render/cli_test.go`** — plain-output exactness + styled-output (ANSI present, text preserved) tests.
- **Modify `main.go`** — add `detectProfile` seam; route help/version/error through the new functions; delete `printUsage`.
- **Modify `main_test.go`** — pin `detectProfile` in the text-asserting tests; add one styled-wiring test.

The existing `render` package already owns the one-shot output, so the CLI chrome belongs there too (same `Theme`, same `colorprofile` handling). `main.go` stays a thin router.

---

### Task 1: Styled chrome functions in `render`

**Files:**
- Create: `render/cli.go`
- Test: `render/cli_test.go`

- [ ] **Step 1: Write the failing tests**

Create `render/cli_test.go`:

```go
package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

const plainUsage = "usage:\n" +
	"  lookit\n" +
	"  lookit user@host[:port]\n" +
	"  lookit @host[:port]\n" +
	"  lookit version\n"

func TestUsagePlainIsByteIdentical(t *testing.T) {
	if got := Usage(colorprofile.NoTTY); got != plainUsage {
		t.Fatalf("Usage(NoTTY) =\n%q\nwant\n%q", got, plainUsage)
	}
}

func TestUsageStyledKeepsTextAddsAnsi(t *testing.T) {
	out := Usage(colorprofile.TrueColor)
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("styled usage has no ANSI:\n%q", out)
	}
	if got := ansi.Strip(out); got != plainUsage {
		t.Fatalf("stripped styled usage =\n%q\nwant\n%q", got, plainUsage)
	}
}

func TestVersionPlainIsInputVerbatim(t *testing.T) {
	const line = "lookit 1.2.3 (built 2026-05-29)"
	if got := Version(line, colorprofile.NoTTY); got != line {
		t.Fatalf("Version plain = %q, want %q", got, line)
	}
}

func TestVersionStyledKeepsTextAddsAnsi(t *testing.T) {
	const line = "lookit 1.2.3 (built 2026-05-29)"
	out := Version(line, colorprofile.TrueColor)
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("styled version has no ANSI:\n%q", out)
	}
	if got := ansi.Strip(out); got != line {
		t.Fatalf("stripped styled version = %q, want %q", got, line)
	}
}

func TestErrorLinePlain(t *testing.T) {
	if got := ErrorLine("bad target", colorprofile.NoTTY); got != "lookit: bad target" {
		t.Fatalf("ErrorLine plain = %q, want %q", got, "lookit: bad target")
	}
}

func TestErrorLineStyledKeepsTextAddsAnsi(t *testing.T) {
	out := ErrorLine("bad target", colorprofile.TrueColor)
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("styled error has no ANSI:\n%q", out)
	}
	if got := ansi.Strip(out); got != "lookit: bad target" {
		t.Fatalf("stripped styled error = %q, want %q", got, "lookit: bad target")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./render/ -run 'TestUsage|TestVersion|TestErrorLine' -count=1 -v`
Expected: FAIL to compile — `undefined: Usage`, `undefined: Version`, `undefined: ErrorLine`.

- [ ] **Step 3: Write minimal implementation**

Create `render/cli.go`:

```go
package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/colorprofile"
)

// Usage returns lookit's CLI usage block. On no-colour profiles
// (Ascii/NoTTY) the Theme styles are no-ops, so the output is byte-identical
// to the plain usage text; on colour profiles the command and example tokens
// are accented.
func Usage(profile colorprofile.Profile) string {
	t := NewTheme(profile)
	cmd := t.Target.Render("lookit")
	var b strings.Builder
	fmt.Fprintln(&b, t.Footer.Render("usage:"))
	fmt.Fprintf(&b, "  %s\n", cmd)
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Field.Render("user@host[:port]"))
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Field.Render("@host[:port]"))
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Footer.Render("version"))
	return b.String()
}

// Version styles a pre-formatted version line ("lookit <rest>"). On no-colour
// profiles it returns the line unchanged; otherwise it accents the leading
// "lookit" token and dims the remainder.
func Version(line string, profile colorprofile.Profile) string {
	t := NewTheme(profile)
	if t.NoColor {
		return line
	}
	name, rest, found := strings.Cut(line, " ")
	if !found {
		return t.Target.Render(line)
	}
	return t.Target.Render(name) + " " + t.Footer.Render(rest)
}

// ErrorLine returns "lookit: <msg>", in the error style on colour profiles and
// plain otherwise. Callers add the trailing newline.
func ErrorLine(msg string, profile colorprofile.Profile) string {
	t := NewTheme(profile)
	return t.ErrLine.Render("lookit: " + msg)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./render/ -run 'TestUsage|TestVersion|TestErrorLine' -count=1 -v`
Expected: PASS (all six).

- [ ] **Step 5: Commit**

```bash
gofmt -w render/cli.go render/cli_test.go
git add render/cli.go render/cli_test.go
git commit -m "feat(render): styled CLI usage, version, and error helpers"
```

(Conventional Commits; **no `Co-Authored-By` or other trailers** — this repo forbids them.)

---

### Task 2: Wire styled chrome into `main.go`

**Files:**
- Modify: `main.go` (`run` ~lines 39-65; `printUsage` ~lines 67-73; add `detectProfile` var near the existing package vars ~lines 25-33)
- Test: `main_test.go`

- [ ] **Step 1: Update tests first**

In `main_test.go`, add imports `"github.com/charmbracelet/colorprofile"` and `"github.com/charmbracelet/x/ansi"` (the file already imports `bytes`, `context`, `errors`, `io`, `net`, `strings`, `testing`, and `finger`). Then add this helper and styled test, and pin `detectProfile` in the three text-asserting tests.

Add near the top of the file (after imports):

```go
// pinProfile forces detectProfile to a fixed profile for the duration of a
// test, so CLI-text assertions don't depend on the ambient environment
// (CLICOLOR_FORCE/NO_COLOR).
func pinProfile(t *testing.T, p colorprofile.Profile) {
	t.Helper()
	old := detectProfile
	t.Cleanup(func() { detectProfile = old })
	detectProfile = func(io.Writer, []string) colorprofile.Profile { return p }
}
```

Replace `TestRunVersion`, `TestRunUsage`, and `TestRunInvalidTarget` with these versions (each gains a `pinProfile(t, colorprofile.NoTTY)` line):

```go
func TestRunVersion(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() {
		version, builtAt = oldVersion, oldBuiltAt
	})
	version = "dev"
	builtAt = "unknown"
	pinProfile(t, colorprofile.NoTTY)

	var stdout, stderr bytes.Buffer
	code := run([]string{"version"}, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if got, want := stdout.String(), "lookit dev (built unknown)\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunUsage(t *testing.T) {
	pinProfile(t, colorprofile.NoTTY)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "lookit version") {
		t.Fatalf("stderr usage missing version command: %q", stderr.String())
	}
}

func TestRunInvalidTarget(t *testing.T) {
	pinProfile(t, colorprofile.NoTTY)
	var stdout, stderr bytes.Buffer
	code := run([]string{"just-a-name"}, &stdout, &stderr)

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "lookit:") {
		t.Fatalf("stderr = %q, want lookit error", stderr.String())
	}
}
```

Append this new test (proves the profile flows through to styled output):

```go
func TestRunVersionStyled(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() {
		version, builtAt = oldVersion, oldBuiltAt
	})
	version = "dev"
	builtAt = "unknown"
	pinProfile(t, colorprofile.TrueColor)

	var stdout, stderr bytes.Buffer
	code := run([]string{"version"}, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("styled version has no ANSI: %q", stdout.String())
	}
	if got := ansi.Strip(stdout.String()); got != "lookit dev (built unknown)\n" {
		t.Fatalf("stripped styled version = %q, want %q", got, "lookit dev (built unknown)\n")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test . -run 'TestRunVersion|TestRunUsage|TestRunInvalidTarget' -count=1 -v`
Expected: FAIL to compile — `undefined: detectProfile`.

- [ ] **Step 3: Add the `detectProfile` seam**

In `main.go`, in the `var ( … )` block (the one with `version`, `builtAt`, `runOneShotFunc`, `startTUI`), add:

```go
	detectProfile = colorprofile.Detect
```

(`colorprofile` is already imported.)

- [ ] **Step 4: Route `run` through the styled helpers**

In `main.go`, replace the body of `run` (currently lines ~39-65) with:

```go
func run(args []string, stdout, stderr io.Writer) int {
	outProfile := detectProfile(stdout, os.Environ())
	errProfile := detectProfile(stderr, os.Environ())

	if len(args) == 0 {
		if err := startTUI(); err != nil {
			fmt.Fprintln(stderr, render.ErrorLine(err.Error(), errProfile))
			return exitNetwork
		}
		return exitOK
	}

	if len(args) != 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stderr, render.Usage(errProfile))
		return exitUsage
	}

	if args[0] == "version" {
		fmt.Fprintln(stdout, render.Version(versionString(), outProfile))
		return exitOK
	}

	target, err := finger.ParseTarget(args[0])
	if err != nil {
		fmt.Fprintln(stderr, render.ErrorLine(err.Error(), errProfile))
		return exitUsage
	}

	return runOneShotFunc(context.Background(), target, stdout)
}
```

- [ ] **Step 5: Delete the now-unused `printUsage`**

In `main.go`, delete the entire `printUsage` function (currently lines ~67-73):

```go
func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  lookit")
	fmt.Fprintln(w, "  lookit user@host[:port]")
	fmt.Fprintln(w, "  lookit @host[:port]")
	fmt.Fprintln(w, "  lookit version")
}
```

(`versionString`, `runOneShot`, `exitCodeFor` stay. `io` is still used by `run`'s signature and `runOneShot`; `os` by `os.Environ()`/`runOneShot`.)

- [ ] **Step 6: Run the main tests to verify they pass**

Run: `go test . -run 'TestRunVersion|TestRunUsage|TestRunInvalidTarget|TestRunOneShotTarget|TestRunNoArgs' -count=1 -v`
Expected: PASS (including `TestRunVersionStyled` and the unchanged seam tests).

- [ ] **Step 7: Full gate**

Run: `make check`
Expected: `go vet` clean, `gofmt -l` empty, golangci-lint `0 issues`, all tests pass with `-race`.

- [ ] **Step 8: Commit**

```bash
gofmt -w main.go main_test.go
git add main.go main_test.go
git commit -m "feat: route CLI help, version, and errors through styled render helpers"
```

(Conventional Commits; **no trailers**.)

---

## Self-Review

**1. Spec coverage:**
- Styled `Usage`/`Version`/`ErrorLine` in `render`, built from `NewTheme` → Task 1.
- Byte-identical plain output on no-colour profiles → Task 1 (`TestUsagePlainIsByteIdentical`, `TestVersionPlainIsInputVerbatim`, `TestErrorLinePlain`); the `Theme` no-op styles make this hold.
- `main.go` wiring (help/version/errors), `printUsage` removed → Task 2 Steps 4-5.
- Per-writer detection through stub-able `detectProfile` seam → Task 2 Steps 3-4.
- Hermetic tests w.r.t. `CLICOLOR_FORCE`/`NO_COLOR` → Task 2 `pinProfile`.
- Forced-colour exception honoured at runtime → `run` uses `detectProfile(writer, os.Environ())`, which respects `CLICOLOR_FORCE` in production (seam un-stubbed).
- No new deps; router/seams/`version` subcommand/exit codes unchanged → no other `main.go` edits.
- Styled-wiring proof → Task 2 `TestRunVersionStyled`.

**2. Placeholder scan:** None — every step shows complete code and exact commands.

**3. Type consistency:** `detectProfile` has type `func(io.Writer, []string) colorprofile.Profile` (matches `colorprofile.Detect` and the `pinProfile` stub). `render.Usage(profile)`, `render.Version(line, profile)`, `render.ErrorLine(msg, profile)` signatures match between Task 1 definitions and Task 2 call sites. `Theme` fields used (`Target`, `Field`, `Footer`, `ErrLine`, `NoColor`) exist on the current `render.Theme`. `NewTheme(profile)` is the single-arg constructor (auto-detects background) present in `render/theme.go`.

**4. Ambiguity check:** Plain Usage is fixed by `plainUsage` const equal to the deleted `printUsage` output line-for-line (incl. the `version` line and trailing newline), so "byte-identical" is unambiguous and test-enforced. `Version` styling splits on the first space (the line always begins `lookit `); the `!found` branch is covered defensively though unreachable for real version strings.

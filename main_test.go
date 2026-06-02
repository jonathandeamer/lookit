package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/jonathandeamer/lookit/tui"
)

// pinProfile forces detectProfile to a fixed profile for the duration of a
// test, so CLI-text assertions don't depend on the ambient environment.
func pinProfile(t *testing.T, p colorprofile.Profile) {
	t.Helper()
	old := detectProfile
	t.Cleanup(func() { detectProfile = old })
	detectProfile = func(io.Writer, []string) colorprofile.Profile { return p }
}

// stubStartTUI replaces the startTUI seam, recording the options it was called
// with and returning err.
func stubStartTUI(t *testing.T, err error) *tui.Options {
	t.Helper()
	old := startTUI
	t.Cleanup(func() { startTUI = old })
	var got tui.Options
	startTUI = func(opts tui.Options) error {
		got = opts
		return err
	}
	return &got
}

func TestVersionString(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() { version, builtAt = oldVersion, oldBuiltAt })
	version = "0.2.0"
	builtAt = "2026-05-29"
	if got, want := versionString(), "lookit 0.2.0 (built 2026-05-29)"; got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}

func TestRunHelp(t *testing.T) {
	pinProfile(t, colorprofile.NoTTY)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout.String(), "usage:") {
		t.Fatalf("stdout = %q, want usage block", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunVersionFlag(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() { version, builtAt = oldVersion, oldBuiltAt })
	version = "dev"
	builtAt = "unknown"
	pinProfile(t, colorprofile.NoTTY)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--version"}, &stdout, &stderr)
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

func TestRunVersionFlagStyled(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() { version, builtAt = oldVersion, oldBuiltAt })
	version = "dev"
	builtAt = "unknown"
	pinProfile(t, colorprofile.TrueColor)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-v"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("styled version has no ANSI: %q", stdout.String())
	}
	if got := ansi.Strip(stdout.String()); got != "lookit dev (built unknown)\n" {
		t.Fatalf("stripped version = %q, want %q", got, "lookit dev (built unknown)\n")
	}
}

func TestRunNoArgsStartsTUI(t *testing.T) {
	got := stubStartTUI(t, nil)
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if got.InitialQuery != "" {
		t.Fatalf("InitialQuery = %q, want empty", got.InitialQuery)
	}
	if got.Seed {
		t.Fatalf("Seed = true, want false for the no-arg launch")
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q, want both empty", stdout.String(), stderr.String())
	}
}

func TestRunSeedsTUIWithTarget(t *testing.T) {
	got := stubStartTUI(t, nil)
	var stdout, stderr bytes.Buffer
	code := run([]string{"alice@plan.cat"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !got.Seed {
		t.Fatalf("Seed = false, want true when an arg is supplied")
	}
	if got.InitialQuery != "alice@plan.cat" {
		t.Fatalf("InitialQuery = %q, want %q", got.InitialQuery, "alice@plan.cat")
	}
}

func TestRunSeedsTUIWithMalformedTarget(t *testing.T) {
	// A malformed target is NOT rejected at the CLI; it seeds the TUI, which
	// shows the parse error in-app.
	got := stubStartTUI(t, nil)
	var stdout, stderr bytes.Buffer
	code := run([]string{"just-a-name"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !got.Seed || got.InitialQuery != "just-a-name" {
		t.Fatalf("Seed=%v InitialQuery=%q, want true / %q", got.Seed, got.InitialQuery, "just-a-name")
	}
}

func TestRunSeedsTUIWithBlankArg(t *testing.T) {
	// lookit "": an arg was supplied (Seed=true) even though its value is blank,
	// so the TUI replays it and surfaces the parse error in-place.
	got := stubStartTUI(t, nil)
	var stdout, stderr bytes.Buffer
	code := run([]string{""}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !got.Seed {
		t.Fatalf("Seed = false, want true for a supplied-but-blank arg")
	}
	if got.InitialQuery != "" {
		t.Fatalf("InitialQuery = %q, want empty", got.InitialQuery)
	}
}

func TestRunTooManyArgs(t *testing.T) {
	pinProfile(t, colorprofile.NoTTY)
	var stdout, stderr bytes.Buffer
	code := run([]string{"a@b", "c@d"}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr = %q, want usage block", stderr.String())
	}
}

func TestRunTUIFailure(t *testing.T) {
	pinProfile(t, colorprofile.NoTTY)
	stubStartTUI(t, errors.New("terminal unavailable"))
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "terminal unavailable") {
		t.Fatalf("stderr = %q, want TUI error", stderr.String())
	}
}

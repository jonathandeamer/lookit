package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/jonathandeamer/lookit/finger"
)

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "dns error",
			err:  &net.DNSError{Err: "no such host", Name: "example.invalid"},
			want: exitNetwork,
		},
		{
			name: "generic error",
			err:  errors.New("read failed"),
			want: exitNetwork,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCodeFor(tc.err); got != tc.want {
				t.Fatalf("exitCodeFor(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() {
		version, builtAt = oldVersion, oldBuiltAt
	})

	version = "0.2.0"
	builtAt = "2026-05-29"

	if got, want := versionString(), "lookit 0.2.0 (built 2026-05-29)"; got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}

func TestRunVersion(t *testing.T) {
	oldVersion, oldBuiltAt := version, builtAt
	t.Cleanup(func() {
		version, builtAt = oldVersion, oldBuiltAt
	})
	version = "dev"
	builtAt = "unknown"

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

func TestRunOneShotTarget(t *testing.T) {
	oldRunOneShotFunc := runOneShotFunc
	t.Cleanup(func() {
		runOneShotFunc = oldRunOneShotFunc
	})

	const (
		stubCode      = 23
		stubbedStdout = "stubbed one-shot output\n"
	)
	var called bool
	var gotTarget finger.Target
	runOneShotFunc = func(ctx context.Context, target finger.Target, stdout io.Writer) int {
		called = true
		gotTarget = target
		_, _ = io.WriteString(stdout, stubbedStdout)
		return stubCode
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"alice@plan.cat"}, &stdout, &stderr)

	if code != stubCode {
		t.Fatalf("exit code = %d, want %d", code, stubCode)
	}
	if !called {
		t.Fatal("runOneShotFunc was not called")
	}
	if gotTarget.Raw != "alice@plan.cat" {
		t.Fatalf("target.Raw = %q, want %q", gotTarget.Raw, "alice@plan.cat")
	}
	if gotTarget.User != "alice" {
		t.Fatalf("target.User = %q, want %q", gotTarget.User, "alice")
	}
	if gotTarget.HostPort != "plan.cat:79" {
		t.Fatalf("target.HostPort = %q, want %q", gotTarget.HostPort, "plan.cat:79")
	}
	if got := stdout.String(); got != stubbedStdout {
		t.Fatalf("stdout = %q, want %q", got, stubbedStdout)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunNoArgsStartsTUI(t *testing.T) {
	oldStartTUI := startTUI
	t.Cleanup(func() {
		startTUI = oldStartTUI
	})

	called := false
	startTUI = func() error {
		called = true
		return nil
	}

	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !called {
		t.Fatalf("startTUI was not called")
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q, want both empty", stdout.String(), stderr.String())
	}
}

func TestRunNoArgsTUIFailure(t *testing.T) {
	oldStartTUI := startTUI
	t.Cleanup(func() {
		startTUI = oldStartTUI
	})

	startTUI = func() error {
		return errors.New("terminal unavailable")
	}

	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)

	if code != exitNetwork {
		t.Fatalf("exit code = %d, want %d", code, exitNetwork)
	}
	if !strings.Contains(stderr.String(), "terminal unavailable") {
		t.Fatalf("stderr = %q, want TUI error", stderr.String())
	}
}

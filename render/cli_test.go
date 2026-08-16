package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

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
	"Press ? inside lookit for keyboard shortcuts.\n" +
	"See man lookit for the full reference.\n"

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
	const line = "lookit version 1.2.3 (built 2026-05-29)"
	if got := Version(line, colorprofile.NoTTY); got != line {
		t.Fatalf("Version plain = %q, want %q", got, line)
	}
}

func TestVersionStyledKeepsTextAddsAnsi(t *testing.T) {
	const line = "lookit version 1.2.3 (built 2026-05-29)"
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

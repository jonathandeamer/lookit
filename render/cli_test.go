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
	"  lookit --version\n" +
	"\n" +
	"press ? in lookit for keys\n"

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

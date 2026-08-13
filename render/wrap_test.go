package render

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/jonathandeamer/lookit/finger"
)

// errDial is a long connection-failure message: at 60 columns the word
// "reason" — the part that says what went wrong — used to fall off screen.
// finger no longer produces the raw Go dial error this once was, but the
// renderer must still wrap whatever unclassified text it's handed.
var errDial = errors.New("couldn't reach a-very-long-hostname.example.org:79: some unclassified reason")

func TestRenderWithWidthWrapsErrorLine(t *testing.T) {
	target := finger.Target{HostPort: "127.0.0.1:1", Raw: "nobody@127.0.0.1:1"}
	got := RenderWithWidth(target, nil, errDial, colorprofile.NoTTY, true, 60)

	if !strings.Contains(got, "reason") {
		t.Fatalf("wrapped error dropped the reason:\n%q", got)
	}
	for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if w := ansi.StringWidth(line); w > 60 {
			t.Errorf("line exceeds width 60 (%d cells): %q", w, line)
		}
	}
}

func TestRenderWithWidthHardWrapsLongToken(t *testing.T) {
	target := finger.Target{HostPort: "example.com:79", Raw: "alice@example.com"}
	long := strings.Repeat("x", 100)
	got := RenderWithWidth(target, nil, errors.New(long), colorprofile.NoTTY, true, 20)

	if strings.Count(got, "x") != 100 {
		t.Fatalf("hard wrap lost or added characters:\n%q", got)
	}
	for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if w := ansi.StringWidth(line); w > 20 {
			t.Errorf("line exceeds width 20 (%d cells): %q", w, line)
		}
	}
}

func TestRenderWithWidthLeavesBodyUnwrapped(t *testing.T) {
	target := finger.Target{HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	long := strings.Repeat("ab", 60) // 120 cells, no spaces to break on
	body := []byte("Login: alice\n" + long + "\n")
	got := RenderWithWidth(target, body, nil, colorprofile.NoTTY, true, 40)

	if !strings.Contains(got, long) {
		t.Fatalf("body line was reflowed at width 40:\n%q", got)
	}
}

func TestRenderWithWidthZeroDoesNotWrap(t *testing.T) {
	target := finger.Target{HostPort: "127.0.0.1:1", Raw: "nobody@127.0.0.1:1"}
	got := RenderWithWidth(target, nil, errDial, colorprofile.NoTTY, true, 0)
	unwrapped := RenderWithBackground(target, nil, errDial, colorprofile.NoTTY, true)

	if got != unwrapped {
		t.Fatalf("width 0 changed output:\n got %q\nwant %q", got, unwrapped)
	}
	if !strings.Contains(got, errDial.Error()) {
		t.Fatalf("width 0 wrapped the error line anyway:\n%q", got)
	}
}

func TestRenderWithWidthStylesEveryWrappedLine(t *testing.T) {
	target := finger.Target{HostPort: "127.0.0.1:1", Raw: "nobody@127.0.0.1:1"}
	got := RenderWithWidth(target, nil, errDial, colorprofile.TrueColor, true, 40)

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected the error to wrap onto multiple lines: %q", got)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "\x1b[") {
			t.Errorf("wrapped error line is unstyled: %q", line)
		}
		if strings.HasSuffix(line, " ") {
			t.Errorf("wrapped error line is padded with trailing space: %q", line)
		}
	}
}

package render

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

var update = flag.Bool("update", false, "update golden files")

// loadInput reads testdata/<name>.input.
func loadInput(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name+".input"))
	if err != nil {
		t.Fatalf("read input %s: %v", name, err)
	}
	return b
}

// compareGolden compares got against testdata/<name>.<profile>.golden.
// With -update, writes got to the golden file.
func compareGolden(t *testing.T, name, profile, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+"."+profile+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		t.Logf("updated golden %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if got != string(want) {
		t.Errorf("output differs from golden %s:\n--- got ---\n%s\n--- want ---\n%s", path, got, string(want))
	}
}

func TestRender_BasicTrueColor(t *testing.T) {
	body := loadInput(t, "basic")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{
		Addr:    "plan.cat:79",
		Elapsed: 123 * time.Millisecond,
		Bytes:   len(body),
	}
	got := RenderWithBackground(target, body, meta, nil, colorprofile.TrueColor, true)
	compareGolden(t, "basic", "truecolor", got)
}

func TestRenderWithoutFooterOmitsStats(t *testing.T) {
	body := []byte("Login: alice\nPlan: hello\n")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 123 * time.Millisecond, Bytes: len(body)}

	bytes := fmtBytes(meta.Bytes)
	with := RenderWithBackground(target, body, meta, nil, colorprofile.NoTTY, true)
	if !strings.Contains(with, bytes) {
		t.Fatalf("default render missing footer stats %q:\n%q", bytes, with)
	}
	without := RenderWithBackground(target, body, meta, nil, colorprofile.NoTTY, true, WithoutFooter())
	if strings.Contains(without, bytes) {
		t.Fatalf("WithoutFooter render still contains footer stats %q:\n%q", bytes, without)
	}
}

func TestRender_BasicNoTTY(t *testing.T) {
	body := loadInput(t, "basic")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{
		Addr:    "plan.cat:79",
		Elapsed: 123 * time.Millisecond,
		Bytes:   len(body),
	}
	got := Render(target, body, meta, nil, colorprofile.NoTTY)
	compareGolden(t, "basic", "notty", got)
}

func TestRender_NoTTY_HasNoANSI(t *testing.T) {
	body := loadInput(t, "basic")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{
		Addr:    "plan.cat:79",
		Elapsed: 123 * time.Millisecond,
		Bytes:   len(body),
	}
	got := Render(target, body, meta, nil, colorprofile.NoTTY)
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("NoTTY output contains ANSI escape sequence: %q", got)
	}
}

func TestRenderWithBackgroundNoTTYHasNoANSI(t *testing.T) {
	body := []byte("Login: alice\nPlan: hello\n")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 123 * time.Millisecond, Bytes: len(body)}

	got := RenderWithBackground(target, body, meta, nil, colorprofile.NoTTY, false)
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("NoTTY output contains ANSI escape sequence: %q", got)
	}
	if !strings.Contains(got, "Login: alice") {
		t.Fatalf("NoTTY output should preserve body text: %q", got)
	}
}

func TestRender_AsciiArtPreserved(t *testing.T) {
	body := loadInput(t, "ascii-art")
	target := finger.Target{User: "bob", HostPort: "example.com:79", Raw: "bob@example.com"}
	meta := finger.Meta{Addr: "example.com:79", Elapsed: 90 * time.Millisecond, Bytes: len(body)}
	got := RenderWithBackground(target, body, meta, nil, colorprofile.TrueColor, true)
	compareGolden(t, "ascii-art", "truecolor", got)

	// Programmatic check: every input line that is not a field-prefixed line
	// must appear verbatim somewhere in the output.
	inputLines := strings.Split(string(body), "\n")
	for _, line := range inputLines {
		if line == "" || strings.HasPrefix(line, "Login:") || strings.HasPrefix(line, "Plan:") {
			continue
		}
		if !strings.Contains(got, line) {
			t.Errorf("ASCII art line not preserved verbatim:\n  line:   %q\n  output: %s", line, got)
		}
	}
}

func TestRender_EmptyResponse(t *testing.T) {
	body := loadInput(t, "empty")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 42 * time.Millisecond, Bytes: 0}
	got := RenderWithBackground(target, body, meta, nil, colorprofile.TrueColor, true)
	compareGolden(t, "empty", "truecolor", got)
}

func TestRender_Truncated(t *testing.T) {
	body := loadInput(t, "truncated")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{
		Addr:      "plan.cat:79",
		Elapsed:   800 * time.Millisecond,
		Bytes:     len(body),
		Truncated: true,
	}
	got := RenderWithBackground(target, body, meta, nil, colorprofile.TrueColor, true)
	compareGolden(t, "truncated", "truecolor", got)
}

func TestRender_Timeout(t *testing.T) {
	body := loadInput(t, "timeout")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{
		Addr:      "plan.cat:79",
		Elapsed:   30 * time.Second,
		Bytes:     len(body),
		Truncated: true,
	}
	queryErr := errors.New("read response timed out after 30s")
	got := RenderWithBackground(target, body, meta, queryErr, colorprofile.TrueColor, true)
	compareGolden(t, "timeout", "truecolor", got)
}

func TestRenderWithBackgroundUsesLightPalette(t *testing.T) {
	body := []byte("Login: alice\nPlan: hello\n")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 123 * time.Millisecond, Bytes: len(body)}

	got := RenderWithBackground(target, body, meta, nil, colorprofile.TrueColor, false)
	if !strings.Contains(got, "\x1b[38;2;201;40;112mLogin:\x1b[0m") {
		t.Fatalf("light render missing light field colour escape:\n%q", got)
	}
}

func TestRenderWithBackgroundUsesDarkPalette(t *testing.T) {
	body := []byte("Login: alice\nPlan: hello\n")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 123 * time.Millisecond, Bytes: len(body)}

	got := RenderWithBackground(target, body, meta, nil, colorprofile.TrueColor, true)
	if !strings.Contains(got, "\x1b[38;2;255;95;162mLogin:\x1b[0m") {
		t.Fatalf("dark render missing dark field colour escape:\n%q", got)
	}
}

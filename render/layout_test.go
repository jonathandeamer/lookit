package render

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/jonathandeamer/lookit/finger"
)

func TestWordWrapBodyLine(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		width int
		want  []string
	}{
		{"short line", "one two", 10, []string{"one two"}},
		{"word boundaries", "one two three", 7, []string{"one two", "three"}},
		{"blank line", "", 7, []string{""}},
		{"tab-indented line passes through", "\talpha beta gamma", 7, []string{"\talpha beta gamma"}},
		{"embedded tab passes through", "alpha\tbeta gamma", 7, []string{"alpha\tbeta gamma"}},
		{"whitespace-only over width passes through", "            ", 5, []string{"            "}},
		{"non-breaking space is not a break", "one two\u00a0three four", 8, []string{"one", "two\u00a0three", "four"}},
		{"no invented indentation", "alpha      beta gamma", 10, []string{"alpha", "beta gamma"}},
		{"long token intact", "lead https://example.com/a-very-long-path tail", 12, []string{"lead", "https://example.com/a-very-long-path", "tail"}},
		{"hyphen is not a breakpoint", "alpha beta-gamma-delta omega", 12, []string{"alpha", "beta-gamma-delta", "omega"}},
		{"wide cells", "ab 界界 cd", 7, []string{"ab 界界", "cd"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wordWrapBodyLine(tt.line, tt.width)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("wordWrapBodyLine(%q, %d) = %#v, want %#v", tt.line, tt.width, got, tt.want)
			}
		})
	}
}

func TestRenderLayoutUnwrappedMatchesCompatibilityRenderer(t *testing.T) {
	target := finger.Target{HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	body := []byte("Login: alice\nPlan:\nalpha beta gamma\n")
	err := errors.New("read response timed out after 30s")
	want := RenderWithWidth(target, body, err, colorprofile.NoTTY, true, 18)

	got := RenderLayout(target, body, err, colorprofile.NoTTY, true, LayoutOptions{ErrorWidth: 18})
	if got.Text != want {
		t.Fatalf("unwrapped layout differs from compatibility renderer:\n got %q\nwant %q", got.Text, want)
	}
	if got.BodyLineCount != 3 {
		t.Fatalf("BodyLineCount = %d, want 3", got.BodyLineCount)
	}
}

func TestRenderLayoutMapsWrappedRowsToPhysicalBodyLines(t *testing.T) {
	target := finger.Target{HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	layout := RenderLayout(target, []byte("alpha beta gamma\n\nlast"), nil, colorprofile.NoTTY, true, LayoutOptions{BodyWidth: 10})

	if layout.Text != "alpha beta\ngamma\n\nlast\n" {
		t.Fatalf("Text = %q", layout.Text)
	}
	wantMap := []int{0, 0, 1, 2, NoBodyLine}
	if !slices.Equal(layout.LineMap, wantMap) {
		t.Fatalf("LineMap = %v, want %v", layout.LineMap, wantMap)
	}
	if layout.BodyLineCount != 3 {
		t.Fatalf("BodyLineCount = %d, want 3", layout.BodyLineCount)
	}
}

func TestRenderLayoutDoesNotHighlightFieldLikeContinuation(t *testing.T) {
	target := finger.Target{HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	body := []byte("Plan: alpha beta On since Tuesday ordinary words\n")
	layout := RenderLayout(target, body, nil, colorprofile.TrueColor, true, LayoutOptions{BodyWidth: 16})

	if !strings.Contains(ansi.Strip(layout.Text), "\nOn since Tuesday") {
		t.Fatalf("test fixture did not put field-like prose on a continuation: %q", ansi.Strip(layout.Text))
	}
	const fieldColour = "\x1b[38;2;255;95;162m"
	if got := strings.Count(layout.Text, fieldColour); got != 1 {
		t.Fatalf("field colour appears %d times, want only the original Plan label styled: %q", got, layout.Text)
	}
}

func TestRenderLayoutKeepsHyphenatedURLIntact(t *testing.T) {
	target := finger.Target{HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	const url = "https://example.com/a-very-long-hyphenated-path"
	layout := RenderLayout(target, []byte("read "+url+" later\n"), nil, colorprofile.NoTTY, true, LayoutOptions{BodyWidth: 12})

	lines := strings.Split(strings.TrimSuffix(layout.Text, "\n"), "\n")
	if !slices.Contains(lines, url) {
		t.Fatalf("URL was split across display rows: %#v", lines)
	}
}

func TestRenderLayoutMapsErrorAndGeneratedRowsToNoBodyLine(t *testing.T) {
	target := finger.Target{HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	withBody := RenderLayout(target, []byte("body\n"), errors.New(strings.Repeat("x", 25)), colorprofile.NoTTY, true, LayoutOptions{ErrorWidth: 10, BodyWidth: 10})
	if withBody.LineMap[0] != 0 {
		t.Fatalf("body row maps to %d, want 0", withBody.LineMap[0])
	}
	for row, logical := range withBody.LineMap[1:] {
		if logical != NoBodyLine {
			t.Errorf("non-body row %d maps to %d", row+1, logical)
		}
	}

	emptyFailure := RenderLayout(target, nil, errors.New("no answer"), colorprofile.NoTTY, true, LayoutOptions{ErrorWidth: 5, BodyWidth: 5})
	if emptyFailure.BodyLineCount != 0 {
		t.Fatalf("empty failure BodyLineCount = %d", emptyFailure.BodyLineCount)
	}
	for row, logical := range emptyFailure.LineMap {
		if logical != NoBodyLine {
			t.Errorf("empty failure row %d maps to %d", row, logical)
		}
	}

	emptySuccess := RenderLayout(target, nil, nil, colorprofile.NoTTY, true, LayoutOptions{BodyWidth: 5})
	if emptySuccess.Text != "(no response body)\n" || emptySuccess.BodyLineCount != 0 {
		t.Fatalf("empty success = %#v", emptySuccess)
	}
	if !slices.Equal(emptySuccess.LineMap, []int{NoBodyLine, NoBodyLine}) {
		t.Fatalf("empty success LineMap = %v", emptySuccess.LineMap)
	}
}

func TestLayoutLineLookupClampsAndFindsFirstSegment(t *testing.T) {
	layout := Layout{LineMap: []int{0, 1, 1, 2, NoBodyLine}, BodyLineCount: 3}
	if got := layout.LogicalLineAt(-4); got != 0 {
		t.Errorf("LogicalLineAt(-4) = %d, want 0", got)
	}
	if got := layout.LogicalLineAt(99); got != NoBodyLine {
		t.Errorf("LogicalLineAt(99) = %d, want NoBodyLine", got)
	}
	for logical, want := range []int{0, 1, 3} {
		if got := layout.DisplayLineFor(logical); got != want {
			t.Errorf("DisplayLineFor(%d) = %d, want %d", logical, got, want)
		}
	}
	if got := layout.DisplayLineFor(-3); got != 0 {
		t.Errorf("DisplayLineFor(-3) = %d, want 0", got)
	}
	if got := layout.DisplayLineFor(99); got != 3 {
		t.Errorf("DisplayLineFor(99) = %d, want 3", got)
	}
}

func TestRenderLayoutMapsPreparedTildeTeamLines(t *testing.T) {
	target := finger.Target{HostPort: "tilde.team:79", Raw: "alice@tilde.team"}
	layout := RenderLayout(target, []byte("Pronouns: they/them\nPlan:\nhello\n"), nil, colorprofile.NoTTY, true, LayoutOptions{})

	if layout.Text != "Pronouns:\n  they/them\nPlan:\nhello\n" {
		t.Fatalf("Text = %q", layout.Text)
	}
	if layout.BodyLineCount != 4 || !slices.Equal(layout.LineMap, []int{0, 1, 2, 3, NoBodyLine}) {
		t.Fatalf("prepared map = count %d map %v", layout.BodyLineCount, layout.LineMap)
	}
}

func TestWordWrapBodyLinePreservesANSISequences(t *testing.T) {
	const open = "\x1b[38;2;255;95;162m"
	const reset = "\x1b[0m"
	line := open + "Login:" + reset + " alice has a plan with several words"

	got := wordWrapBodyLine(line, 16)
	joined := strings.Join(got, "\n")
	if strings.Count(joined, open) != 1 || strings.Count(joined, reset) != 1 {
		t.Fatalf("ANSI field style was lost or duplicated: %q", joined)
	}
	for _, segment := range got {
		if width := ansi.StringWidth(segment); width > 16 {
			t.Errorf("wrapped segment is %d cells, want at most 16: %q", width, segment)
		}
	}
}

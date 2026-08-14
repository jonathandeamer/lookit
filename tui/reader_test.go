package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/jonathandeamer/lookit/finger"
)

func TestReaderSetEntryUpdatesViewport(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	m.setEntry(Entry{
		Target: target,
		Body:   []byte("Login: alice\n"),
		Meta:   finger.Meta{Addr: target.HostPort, Bytes: len("Login: alice\n")},
	})
	if m.current == nil || m.current.Target.Raw != "alice@plan.cat" {
		t.Fatalf("current = %#v, want alice entry", m.current)
	}
	if !strings.Contains(m.viewport.View(), "Login: alice") {
		t.Fatalf("viewport content missing body: %q", m.viewport.View())
	}
}

func TestReaderSetEntryError(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	m.setEntry(Entry{
		Target: target,
		Meta:   finger.Meta{Addr: target.HostPort},
		Err:    errors.New("dial failed"),
	})
	if m.current == nil || m.current.Err == nil {
		t.Fatalf("current = %#v, want error entry", m.current)
	}
	if !strings.Contains(m.viewport.View(), "dial failed") {
		t.Fatalf("viewport content missing error: %q", m.viewport.View())
	}
}

func TestReaderSetEntryWithLinks_StoresLinks(t *testing.T) {
	// setEntryWithLinks must set m.links so snapshot() can save them.
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	links := []Link{
		{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com", Strong: true},
	}
	m.setEntryWithLinks(Entry{Target: target, Body: []byte("see https://example.com\n")}, links)
	if len(m.links) != 1 || m.links[0].Raw != "https://example.com" {
		t.Errorf("m.links = %v, want the passed link slice stored", m.links)
	}
}

func TestReaderSetSize(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	m.setSize(100, 30)
	if m.viewport.Width() != 100 {
		t.Fatalf("viewport width = %d, want 100", m.viewport.Width())
	}
	// chromeRows == 0: the reader is viewport-only, so viewport height == height passed.
	if m.viewport.Height() != 30 {
		t.Fatalf("viewport height = %d, want 30 (chromeRows==0, full height to viewport)", m.viewport.Height())
	}
}

// dialErrText is a long connection-failure message: at 60 columns the reason
// ("reason") used to be clipped off the right edge unreachably. The reader
// must wrap unclassified text, not assume a classified shape.
const dialErrText = "couldn't reach a-very-long-hostname.example.org:79: some unclassified reason"

func TestReaderWrapsErrorAtViewportWidth(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("nobody@127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	m.setSize(60, 10)
	m.setEntry(Entry{Target: target, Err: errors.New(dialErrText)})

	got := ansi.Strip(m.viewport.View())
	if !strings.Contains(got, "reason") {
		t.Fatalf("reader clipped the error reason at width 60:\n%q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if w := ansi.StringWidth(strings.TrimRight(line, " ")); w > 60 {
			t.Errorf("reader line exceeds width 60 (%d cells): %q", w, line)
		}
	}
}

func TestReaderRewrapsErrorOnResize(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("nobody@127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	m.setSize(100, 10)
	m.setEntry(Entry{Target: target, Err: errors.New(dialErrText)})
	m.setSize(40, 10)

	got := ansi.Strip(m.viewport.View())
	if !strings.Contains(got, "reason") {
		t.Fatalf("error reason lost after resize:\n%q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if w := ansi.StringWidth(strings.TrimRight(line, " ")); w > 40 {
			t.Errorf("line not re-wrapped after resize to 40 (%d cells): %q", w, line)
		}
	}
}

func TestReaderDoesNotWrapBody(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("ab", 60) // 120 cells with no break opportunity
	m.setSize(40, 10)
	m.setEntry(Entry{Target: target, Body: []byte(long + "\n")})

	// The viewport clips horizontally, so the visible first line is 40 cells
	// either way. What distinguishes wrap from clip is line 2: a soft-wrapped
	// body would continue there.
	lines := strings.Split(ansi.Strip(m.viewport.View()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected a padded viewport, got %q", m.viewport.View())
	}
	if strings.TrimRight(lines[1], " ") != "" {
		t.Fatalf("body line was reflowed at width 40, continuation on line 2: %q", lines[1])
	}
}

// The issue's repro: `lookit nobody@127.0.0.1:1` in a 60-column terminal.
// Drives the whole app so the wrap survives the app -> reader wiring.
func TestDialFailureStaysReadableAt60Columns(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	target, err := finger.ParseTarget("nobody@127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	landed := deliverNavigation(sized.(appModel), Entry{
		Target: target,
		Meta:   finger.Meta{Addr: target.HostPort},
		Err:    errors.New(dialErrText),
	})

	got := ansi.Strip(landed.View().Content)
	if !strings.Contains(got, "reason") {
		t.Fatalf("dial reason clipped at 60 columns:\n%s", got)
	}
}

func TestReaderLinkCanOccupyFirstResponseLine(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	const raw = "alice@example.com"
	m.focusedLink = 0
	m.setEntryWithLinks(Entry{Target: target, Body: []byte(raw + "\nrest\n")}, []Link{{Kind: LinkFinger, Action: ActionCopy, Raw: raw}})
	got := ansi.Strip(m.viewport.View())
	first, _, _ := strings.Cut(got, "\n")
	if first = strings.TrimRight(first, " "); first != raw {
		t.Fatalf("reader starts with %q, want body link first", got)
	}
}

func TestReaderFocusedLinkScrollHasNoHeaderOffset(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	const raw = "alice@example.com"
	m.setSize(40, 2)
	m.focusedLink = 0
	m.setEntryWithLinks(Entry{Target: target, Body: []byte("zero\none\ntwo\n" + raw + "\ntail\ntail\n")}, []Link{{Kind: LinkFinger, Action: ActionCopy, Raw: raw}})
	if got := m.viewport.YOffset(); got != 2 {
		t.Fatalf("YOffset = %d, want 2 without a header offset", got)
	}
}

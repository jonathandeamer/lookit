package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/jonathandeamer/lookit/finger"
	"github.com/jonathandeamer/lookit/render"
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
	m.setEntryWithLinks(Entry{Target: target, Body: []byte("see https://example.com\n")}, links, false, readerPosition{})
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
	m.setEntryWithLinks(Entry{Target: target, Body: []byte(raw + "\nrest\n")}, []Link{{Kind: LinkFinger, Action: ActionCopy, Raw: raw}}, false, readerPosition{})
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
	m.setEntryWithLinks(Entry{Target: target, Body: []byte("zero\none\ntwo\n" + raw + "\ntail\ntail\n")}, []Link{{Kind: LinkFinger, Action: ActionCopy, Raw: raw}}, false, readerPosition{})
	if got := m.viewport.YOffset(); got != 2 {
		t.Fatalf("YOffset = %d, want 2 without a header offset", got)
	}
}

func TestReaderFocusedLinkScrollUsesRepeatedOccurrence(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	const raw = "alice@example.com"
	m.setSize(40, 2)
	entry := Entry{Target: target, Body: []byte(raw + "\none\ntwo\nthree\nfour\n" + raw + "\ntail\ntail\n")}
	links := []Link{
		{Kind: LinkFinger, Action: ActionCopy, Raw: raw},
		{Kind: LinkFinger, Action: ActionCopy, Raw: raw},
	}

	m.focusedLink = 0
	m.setEntryWithLinks(entry, links, false, readerPosition{})
	if got := m.viewport.YOffset(); got != 0 {
		t.Fatalf("YOffset = %d, want 0 for the first occurrence", got)
	}

	m.focusedLink = 1
	m.setEntryWithLinks(entry, links, false, readerPosition{})
	if got := m.viewport.YOffset(); got != 4 {
		t.Fatalf("YOffset = %d, want 4 for the second occurrence", got)
	}
}

func TestReaderFocusedLinkScrollUsesRenderedLines(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@tilde.team")
	if err != nil {
		t.Fatal(err)
	}
	const raw = "alice@example.com"
	m.setSize(40, 2)
	m.focusedLink = 0
	m.setEntryWithLinks(
		Entry{Target: target, Body: []byte("Pronouns: they/them\nzero\n" + raw + "\ntail\ntail\n")},
		[]Link{{Kind: LinkFinger, Action: ActionCopy, Raw: raw}},
		false,
		readerPosition{},
	)
	if got := m.viewport.YOffset(); got != 2 {
		t.Fatalf("YOffset = %d, want 2 after rendered pronouns reflow", got)
	}
}

func TestReaderWrapUsesViewportWidthCappedAt80(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	body := []byte(strings.Repeat("word ", 30) + "end\n")

	wide := newReader(colorprofile.NoTTY)
	wide.setSize(100, 20)
	wide.setEntryWithLinks(Entry{Target: target, Body: body}, nil, true, readerPosition{})
	if got := len(strings.Split(strings.TrimSuffix(wide.layout.Text, "\n"), "\n")); got != 2 {
		t.Fatalf("100-column reader produced %d rows, want 2 at the 80-cell cap", got)
	}
	for _, line := range strings.Split(strings.TrimSuffix(wide.layout.Text, "\n"), "\n") {
		if width := ansi.StringWidth(line); width > 80 {
			t.Errorf("100-column reader row is %d cells, want at most 80", width)
		}
	}

	narrow := newReader(colorprofile.NoTTY)
	narrow.setSize(45, 20)
	narrow.setEntryWithLinks(Entry{Target: target, Body: body}, nil, true, readerPosition{})
	if got := len(strings.Split(strings.TrimSuffix(narrow.layout.Text, "\n"), "\n")); got != 4 {
		t.Fatalf("45-column reader produced %d rows, want 4", got)
	}
}

func TestReaderTogglePreservesTopLogicalLineAndResetsHorizontalOffset(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	body := []byte("first line has enough words to wrap across rows\nsecond source line has enough words to wrap too\nthird source line remains available\n")
	m := newReader(colorprofile.NoTTY)
	m.setSize(20, 3)
	m.setEntryWithLinks(Entry{Target: target, Body: body}, nil, false, readerPosition{})
	m.viewport.SetYOffset(1)
	m.viewport.SetXOffset(6)

	m.setWrapped(true)
	if got := m.topLogicalLine(); got != 1 {
		t.Fatalf("wrapped top logical line = %d, want 1", got)
	}
	if got := m.viewport.XOffset(); got != 0 {
		t.Fatalf("wrapped XOffset = %d, want 0", got)
	}

	m.viewport.SetXOffset(4)
	m.setWrapped(false)
	if got := m.topLogicalLine(); got != 1 {
		t.Fatalf("unwrapped top logical line = %d, want 1", got)
	}
	if got := m.viewport.XOffset(); got != 0 {
		t.Fatalf("unwrapped XOffset = %d, want 0", got)
	}
}

func TestReaderResizePreservesTopLogicalLineAndResetsHorizontalOffset(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	body := []byte("first source line has enough words to wrap across rows\nsecond source line has enough words to wrap across several rows too\nthird source line\n")
	m := newReader(colorprofile.NoTTY)
	m.setSize(100, 3)
	m.setEntryWithLinks(Entry{Target: target, Body: body}, nil, true, readerPosition{})
	m.viewport.SetYOffset(m.layout.DisplayLineFor(1))
	m.viewport.SetXOffset(5)

	m.setSize(45, 3)
	if got := m.topLogicalLine(); got != 1 {
		t.Fatalf("top logical line after resize = %d, want 1", got)
	}
	if got := m.viewport.XOffset(); got != 0 {
		t.Fatalf("XOffset after resize = %d, want 0", got)
	}
}

func TestReaderHeightOnlyResizeDoesNotRerenderOrResetHorizontalOffset(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	body := []byte(strings.Repeat("wide ", 20) + "\n")
	m := newReader(colorprofile.NoTTY)
	m.setSize(30, 4)
	m.setEntryWithLinks(Entry{Target: target, Body: body}, nil, false, readerPosition{})
	m.viewport.SetXOffset(7)
	wantContent, wantLayout := m.viewport.GetContent(), m.layout.Text

	m.setSize(30, 8)
	if got := m.viewport.GetContent(); got != wantContent {
		t.Fatalf("height-only resize changed content:\n got %q\nwant %q", got, wantContent)
	}
	if m.layout.Text != wantLayout {
		t.Fatal("height-only resize replaced the stored layout")
	}
	if got := m.viewport.XOffset(); got != 7 {
		t.Fatalf("height-only resize XOffset = %d, want 7", got)
	}
}

func TestReaderRawResizeKeepsRawContent(t *testing.T) {
	target := hostTarget(t, "alice@tilde.team")
	body := []byte("Pronouns: they/them\n" + strings.Repeat("raw ", 20) + "\n")
	m := newReader(colorprofile.NoTTY)
	m.setSize(30, 4)
	m.setEntryWithLinks(Entry{Target: target, Body: body}, nil, true, readerPosition{})
	m.setRaw(body)
	m.viewport.SetXOffset(6)

	m.setSize(20, 4)
	if got := m.viewport.GetContent(); got != string(body) {
		t.Fatalf("raw width resize replaced source content:\n got %q\nwant %q", got, body)
	}
	if got := m.viewport.XOffset(); got != 0 {
		t.Fatalf("raw width resize XOffset = %d, want 0", got)
	}
	m.viewport.SetXOffset(5)
	m.setSize(20, 8)
	if got := m.viewport.GetContent(); got != string(body) {
		t.Fatalf("raw height resize replaced source content: %q", got)
	}
	if got := m.viewport.XOffset(); got != 5 {
		t.Fatalf("raw height resize XOffset = %d, want 5", got)
	}
}

func TestReaderTopErrorRowFallsBackToLastBodyLine(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	entry := Entry{Target: target, Body: []byte("one\ntwo\nthree\n"), Err: errors.New(strings.Repeat("failure ", 8))}
	m := newReader(colorprofile.NoTTY)
	m.setSize(12, 2)
	m.setEntryWithLinks(entry, nil, false, readerPosition{})
	m.viewport.SetYOffset(3)

	m.setWrapped(true)
	if got := m.topLogicalLine(); got != 2 {
		t.Fatalf("top logical line = %d, want final body line 2", got)
	}
	if got := m.viewport.YOffset(); got != m.layout.DisplayLineFor(2) {
		t.Fatalf("YOffset = %d, want final body display row %d", got, m.layout.DisplayLineFor(2))
	}
}

func TestReaderFocusedLinkRecentresInWrappedLayout(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	const raw = "https://example.com/a-very-long-hyphenated-path"
	entry := Entry{Target: target, Body: []byte("one two three four five six seven eight\nmiddle\n" + raw + "\n")}
	link := Link{Kind: LinkURL, Action: ActionCopy, Raw: raw, Strong: true}
	m := newReader(colorprofile.NoTTY)
	m.setSize(10, 4)
	m.focusedLink = 0
	m.setEntryWithLinks(entry, []Link{link}, false, readerPosition{})
	m.setWrapped(true)

	if m.focusedLink != 0 || m.links[0].Action != ActionCopy {
		t.Fatalf("focused link changed: index=%d links=%#v", m.focusedLink, m.links)
	}
	if !strings.Contains(m.layout.Text, "\n"+raw+"\n") {
		t.Fatalf("wrapped layout split raw URL: %q", m.layout.Text)
	}
	overlaid := m.viewport.GetContent()
	if !strings.Contains(overlaid, m.styles.linkFocus.Render(raw)) {
		t.Fatalf("wrapped toggle lost focused-link styling: %q", overlaid)
	}
	if !strings.Contains(overlaid, "\x1b]8;;"+raw) {
		t.Fatalf("wrapped toggle lost OSC-8 hyperlink: %q", overlaid)
	}
	if got := m.viewport.YOffset(); got != 4 {
		t.Fatalf("focused wrapped link YOffset = %d, want centred offset 4", got)
	}
}

func TestReaderEmptyFailureResizeRetainsFallbackPosition(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	entry := Entry{Target: target, Err: errors.New(strings.Repeat("failure ", 12))}
	m := newReader(colorprofile.NoTTY)
	m.setSize(12, 3)
	m.setEntryWithLinks(entry, nil, false, readerPosition{})
	m.viewport.SetYOffset(2)

	m.setSize(10, 3)
	if got := m.topLogicalLine(); got != render.NoBodyLine {
		t.Fatalf("empty failure gained logical body line %d", got)
	}
	if got := m.viewport.YOffset(); got != 2 {
		t.Fatalf("empty failure YOffset = %d, want fallback 2", got)
	}
}

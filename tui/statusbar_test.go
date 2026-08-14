package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/jonathandeamer/lookit/finger"
)

func TestStatusBarProfileShowsBreadcrumb(t *testing.T) {
	b := statusBar{host: "@tilde.team", user: "jonathan", escTarget: "@tilde.team",
		meta: "1.2 KB", hints: "esc back · ? help", width: 80, styles: newStyles(true)}
	out := b.render()
	stripped := ansi.Strip(out)
	for _, want := range []string{"@tilde.team", "jonathan", "◂ esc: @tilde.team", "1.2 KB", "? help"} {
		if !strings.Contains(stripped, want) {
			t.Fatalf("bar %q missing %q", stripped, want)
		}
	}
	if w := lipgloss.Width(out); w != 80 {
		t.Fatalf("bar width = %d, want 80", w)
	}
}

func TestStatusBarDirectoryHasNoUserHalf(t *testing.T) {
	b := statusBar{host: "@tilde.team", meta: "3 users",
		hints: "↵ open · ? help", width: 80, styles: newStyles(true)}
	out := b.render()
	if strings.Contains(out, " / ") {
		t.Fatalf("directory bar should have no ' / ' separator: %q", out)
	}
	if !strings.Contains(out, "3 users") {
		t.Fatalf("bar %q missing meta", out)
	}
}

func TestStatusBarStartShowsFocusedInputHint(t *testing.T) {
	m := appModel{inputFocused: true}
	out := m.startBar(80, newStyles(true)).render()
	if !strings.Contains(out, "type a target") {
		t.Fatalf("start bar %q missing focused-input hint", out)
	}
}

func TestStatusBarStartDoesNotCountCatalogCredit(t *testing.T) {
	m := appModel{start: newStart(testCommon(), twoSections(), "", "")}
	out := m.startBar(80, newStyles(true)).render()
	if !strings.Contains(out, "3 entries") {
		t.Fatalf("start bar %q should count only selectable entries", out)
	}
}

func TestStatusBarStartUsesSingularEntryLabel(t *testing.T) {
	sections := []startSection{{
		id: sectionCommunities, title: "COMMUNITIES",
		entries: []startEntry{{target: "@plan.cat", source: sourceCatalog}},
	}}
	m := appModel{start: newStart(testCommon(), sections, "", "")}
	if got := m.startBar(80, newStyles(true)).meta; got != "1 entry" {
		t.Fatalf("start bar meta = %q, want %q", got, "1 entry")
	}
}

func TestStatusBarListUsesSingularUserLabel(t *testing.T) {
	m := appModel{
		common:  testCommon(),
		pos:     0,
		state:   stateList,
		history: []histNode{{entry: Entry{Target: hostTarget(t, "@tilde.team")}, state: stateList, listUsers: 1}},
	}
	if got := m.buildStatusBar().meta; got != "1 user" {
		t.Fatalf("list bar meta = %q, want %q", got, "1 user")
	}
}

func TestStatusBarStartBookmarkActionIsContextual(t *testing.T) {
	tests := []struct {
		name   string
		seed   string
		filter string
		pick   string
		want   string
	}{
		{name: "catalog row", pick: "@plan.cat", want: "b bookmark"},
		{name: "bookmark row", seed: "@tilde.team\n", pick: "@tilde.team", want: "b remove"},
		{name: "filtered bookmark row", seed: "@tilde.team\n", filter: "tilde", pick: "@tilde.team", want: "b remove"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.seed == "" {
				useTempBookmarks(t)
			} else {
				seedBookmarks(t, tt.seed)
			}
			m := newApp(stubFetch(t), colorprofile.NoTTY)
			m.blurInput()
			if tt.filter != "" {
				m.start.list.SetFilterText(tt.filter)
			}
			if !m.start.selectTarget(tt.pick) {
				t.Fatalf("%s not found", tt.pick)
			}
			out := m.startBar(80, newStyles(true)).render()
			if !strings.Contains(out, tt.want) {
				t.Fatalf("start bar %q missing %q", out, tt.want)
			}
		})
	}
}

func TestStatusBarTruncatesBreadcrumbFirst(t *testing.T) {
	b := statusBar{host: "@an-extremely-long-hostname.example.org", user: "verylonguser",
		meta: "1.2 KB", hints: "esc back · ? help", width: 40, styles: newStyles(true)}
	out := b.render()
	if w := lipgloss.Width(out); w != 40 {
		t.Fatalf("bar width = %d, want 40 (must clamp)", w)
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis when breadcrumb overflows: %q", out)
	}
	if !strings.Contains(out, "? help") {
		t.Fatalf("right-side hints must survive truncation: %q", out)
	}
}

func TestStatusBarShowsScrollAndPage(t *testing.T) {
	b := statusBar{host: "@tilde.team", user: "bob", scroll: "42%",
		hints: "? help", width: 80, styles: newStyles(true)}
	if !strings.Contains(b.render(), "42%") {
		t.Fatalf("bar missing scroll %%: %q", b.render())
	}
	b2 := statusBar{host: "@sdf.org", page: "page 2/4", meta: "42 users",
		hints: "? help", width: 80, styles: newStyles(true)}
	if !strings.Contains(b2.render(), "page 2/4") {
		t.Fatalf("bar missing page indicator: %q", b2.render())
	}
}

func TestStatusBarZeroWidthIsEmpty(t *testing.T) {
	if out := (statusBar{width: 0, styles: newStyles(true)}).render(); out != "" {
		t.Fatalf("zero-width bar = %q, want empty", out)
	}
}

func TestStatusBarFormatsLandedLatency(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{500 * time.Microsecond, "500µs"},
		{42 * time.Millisecond, "42ms"},
		{1500 * time.Millisecond, "1.50s"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			m := settledReader(t, Entry{
				Target: hostTarget(t, "alice@plan.cat"),
				Body:   []byte("Plan: hi\n"),
				Meta:   finger.Meta{Elapsed: tt.in},
			})
			m.common.width = 100
			if got := ansi.Strip(m.buildStatusBar().render()); !strings.Contains(got, tt.want+" · 9 B") {
				t.Errorf("status bar = %q, want formatted latency %q before byte count", got, tt.want)
			}
		})
	}
}

func TestStatusBarShowsLandedLatencyWhenItFits(t *testing.T) {
	m := settledReader(t, Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("Plan: hi\n"),
		Meta:   finger.Meta{Elapsed: 123 * time.Millisecond},
	})
	m.common.width = 100
	if got := ansi.Strip(m.buildStatusBar().render()); !strings.Contains(got, "123ms · 9 B") {
		t.Fatalf("status bar missing latency: %q", got)
	}
}

func TestStatusBarDropsLatencyBeforeExistingInformation(t *testing.T) {
	m := settledReader(t, Entry{
		Target: hostTarget(t, "alice@x.example"),
		Body:   []byte("Plan: hi\n"),
		Meta:   finger.Meta{Elapsed: 123 * time.Millisecond},
	})
	m.common.width = lipgloss.Width("@x.example / alice") + 1 + lipgloss.Width("9 B · ↑↓ scroll · r refresh · ? help")
	got := ansi.Strip(m.buildStatusBar().render())
	if strings.Contains(got, "123ms") || !strings.Contains(got, "9 B") || !strings.Contains(got, "? help") {
		t.Fatalf("latency displaced existing information: %q", got)
	}
}

func TestStatusBarIncludesLatencyAtExactCandidateWidth(t *testing.T) {
	// Exact candidate width: "@h" (2) + gap (1) +
	// "123ms · 9 B · ? help" (20) = 23 cells.
	b := statusBar{
		host: "@h", latency: "123ms", meta: "9 B", hints: "? help",
		width: 23, styles: newStyles(true),
	}
	got := ansi.Strip(b.render())
	if !strings.Contains(got, "123ms · 9 B · ? help") {
		t.Fatalf("exact-fit status omitted latency: %q", got)
	}
}

func TestStatusBarOmitsLatencyOneCellBeforeFit(t *testing.T) {
	b := statusBar{
		host: "@h", latency: "123ms", meta: "9 B", hints: "? help",
		width: 22, styles: newStyles(true),
	}
	got := ansi.Strip(b.render())
	if strings.Contains(got, "123ms") || !strings.Contains(got, "9 B · ? help") {
		t.Fatalf("one-cell-short status displaced existing information: %q", got)
	}
}

// TestFormatBytes pins the byte-count formatting across all three magnitude
// branches (B / KB / MB) and the boundaries between them, so a simplification
// of formatBytes can't silently shift a unit or rounding.
func TestFormatBytes(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},    // last value below the KB threshold
		{1024, "1.0 KB"},    // first KB
		{1536, "1.5 KB"},    // mid-KB rounding
		{1048576, "1.0 MB"}, // first MB (1024*1024)
		{1572864, "1.5 MB"}, // mid-MB rounding
	}
	for _, tc := range cases {
		if got := formatBytes(tc.n); got != tc.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestStatusBarWarnFlagRendered(t *testing.T) {
	b := statusBar{host: "@tilde.team", flags: []string{"partial (truncated)"},
		meta: "3 users", hints: "? help", width: 80, styles: newStyles(true)}
	if !strings.Contains(b.render(), "partial (truncated)") {
		t.Fatalf("bar missing warn flag")
	}
}

func TestStatusBarShowsFlagAtExactWidth(t *testing.T) {
	b := statusBar{flags: []string{"auto-detected"}, width: 15, styles: newStyles(true)}
	if got := b.render(); !strings.Contains(got, "auto-detected") {
		t.Fatalf("exact-width flag missing: %q", got)
	}
}

func TestStatusBarDoesNotReserveSpaceForHiddenBreadcrumb(t *testing.T) {
	b := statusBar{host: "@tilde.team", hints: "abcdefghij", width: 10, styles: newStyles(true)}
	if got := ansi.Strip(b.render()); !strings.Contains(got, "abcdefghij") {
		t.Fatalf("full-width hint truncated for hidden breadcrumb: %q", got)
	}
}

func TestStatusBarFlagNeverOverflowsNarrowWidth(t *testing.T) {
	for w := 1; w <= 30; w++ {
		b := statusBar{host: "@tilde.team", flags: []string{"partial (truncated)"},
			meta: "3 users", hints: "? help", width: w, styles: newStyles(true)}
		out := b.render()
		if strings.Contains(out, "\n") {
			t.Fatalf("width %d: flagged bar wrapped to multiple lines:\n%q", w, out)
		}
		if lipgloss.Width(out) > w {
			t.Fatalf("width %d: flagged bar width = %d, exceeds limit", w, lipgloss.Width(out))
		}
	}
}

func TestStatusBarNeverWrapsAtNarrowWidth(t *testing.T) {
	// At a width too small for any gap, the bar must clip to one line, not wrap.
	for w := 1; w <= 30; w++ {
		b := statusBar{host: "@tilde.team", user: "jonathan", escTarget: "@tilde.team",
			meta: "1.2 KB", hints: "esc back · ? help", width: w, styles: newStyles(true)}
		out := b.render()
		if strings.Contains(out, "\n") {
			t.Fatalf("width %d: bar wrapped to multiple lines:\n%q", w, out)
		}
		if lipgloss.Width(out) > w {
			t.Fatalf("width %d: bar width = %d, exceeds limit", w, lipgloss.Width(out))
		}
	}
}

func TestStatusBarUsesBadgeStyle(t *testing.T) {
	st := newStyles(true)
	out := (statusBar{host: "@tilde.team", hints: "? help", width: 40, styles: st}).render()
	if strings.Contains(out, "STATUS") {
		t.Fatal("status bar should not invent a STATUS badge")
	}
	if !sameColor(st.barBadge.GetBackground(), st.palette.AccentViolet) {
		t.Fatal("bar badge should use accent violet")
	}
}

func TestStatusBarLeafIsSolidAccent(t *testing.T) {
	st := newStyles(true)
	b := statusBar{host: "@tilde.team", user: "jonathan", width: 80, styles: st}
	crumb := b.styleCrumb(60) // 60 > width of "@tilde.team / jonathan", so it fits
	if got := ansi.Strip(crumb); got != "@tilde.team / jonathan" {
		t.Fatalf("stripped crumb = %q, want %q", got, "@tilde.team / jonathan")
	}
	// The leaf renders as a single solid barUser accent run, not a per-rune gradient.
	if !strings.HasSuffix(crumb, st.barUser.Render("jonathan")) {
		t.Fatalf("leaf should be a single barUser accent, got %q", crumb)
	}
}

func TestStatusBarLeafCollapsesToDimWhenOverBudget(t *testing.T) {
	b := statusBar{host: "@a-very-long-hostname.example.org", user: "verylonguser", width: 80, styles: newStyles(true)}
	crumb := b.styleCrumb(10) // narrow budget forces the dim collapse
	if !strings.Contains(crumb, "…") {
		t.Fatalf("expected ellipsis collapse: %q", crumb)
	}
	if got := len(foregroundSequences(crumb)); got != 1 {
		t.Fatalf("collapsed crumb should be a single dim colour, got %d: %q", got, crumb)
	}
}

func TestStatusBarDirectoryLeafHasNoGradient(t *testing.T) {
	b := statusBar{host: "@tilde.team", width: 80, styles: newStyles(true)}
	crumb := b.styleCrumb(60)
	if got := ansi.Strip(crumb); got != "@tilde.team" {
		t.Fatalf("stripped crumb = %q, want %q", got, "@tilde.team")
	}
	if got := len(foregroundSequences(crumb)); got != 1 {
		t.Fatalf("directory crumb should be a single dim colour, got %d: %q", got, crumb)
	}
}

func TestStatusBarFailedRequestDropsBytesAndScroll(t *testing.T) {
	m := settledReader(t, Entry{
		Target: hostTarget(t, "nobody@127.0.0.1:1"),
		Err:    errors.New("connection refused by 127.0.0.1:1"),
	})
	bar := m.buildStatusBar()

	if bar.meta != "" {
		t.Errorf("meta = %q, want empty: no response landed, so there are no bytes to report", bar.meta)
	}
	if bar.scroll != "" {
		t.Errorf("scroll = %q, want empty: there is nothing to scroll", bar.scroll)
	}
	if !strings.Contains(bar.hints, "r retry") {
		t.Errorf("hints = %q, want them to include \"r retry\"", bar.hints)
	}
	if strings.Contains(bar.hints, "scroll") {
		t.Errorf("hints = %q, want no scroll hint", bar.hints)
	}
}

func TestStatusBarFailedRequestKeepsRetryAt45Columns(t *testing.T) {
	m := settledReader(t, Entry{
		Target: hostTarget(t, "nobody@127.0.0.1:1"),
		Err:    errors.New("connection refused by 127.0.0.1:1"),
	})
	m.common.width = 45
	bar := m.buildStatusBar()

	line := ansi.Strip(bar.render())
	if !strings.Contains(line, "r retry") {
		t.Errorf("45-column bar dropped the only useful action:\n%s", line)
	}
	if strings.Contains(line, "0 B") {
		t.Errorf("45-column bar still reports a byte count:\n%s", line)
	}
}

func TestStatusBarErroredResponseWithBodyKeepsBytes(t *testing.T) {
	m := settledReader(t, Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("half a plan\n"),
		Meta:   finger.Meta{Bytes: 12, Truncated: true},
		Err:    errors.New("plan.cat:79 stopped responding after 30s"),
	})
	bar := m.buildStatusBar()

	if bar.meta == "" {
		t.Error("a response with bytes must still report its byte count")
	}
	if !slices.Contains(bar.flags, "partial (truncated)") {
		t.Errorf("flags = %v, want the partial (truncated) flag", bar.flags)
	}
}

// A read deadline can set Meta.Truncated even when nothing was ever read
// (see finger.queryWith / TestQuery_ReadDeadline): the server accepted the
// connection, then sent nothing before the timeout fired. That still counts
// as a failed entry (no bytes), so it gets the plain-failure treatment, not
// the "partial (truncated)" flag — "partial" claims part of a response
// arrived, and here none did.
func TestStatusBarTruncatedWithNoBodyIsAPlainFailure(t *testing.T) {
	m := settledReader(t, Entry{
		Target: hostTarget(t, "nobody@127.0.0.1:1"),
		Meta:   finger.Meta{Truncated: true},
		Err:    errors.New("127.0.0.1:1 stopped responding after 30s"),
	})
	bar := m.buildStatusBar()

	if bar.meta != "" {
		t.Errorf("meta = %q, want empty: no bytes landed", bar.meta)
	}
	if bar.scroll != "" {
		t.Errorf("scroll = %q, want empty: there is nothing to scroll", bar.scroll)
	}
	if slices.Contains(bar.flags, "partial (truncated)") {
		t.Errorf("flags = %v, want no partial (truncated) flag: nothing partial arrived", bar.flags)
	}
	if !strings.Contains(bar.hints, "r retry") {
		t.Errorf("hints = %q, want them to include \"r retry\"", bar.hints)
	}
}

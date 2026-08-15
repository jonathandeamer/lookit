package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
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

// startFilterModel drives a real startpage filter through appModel, pumping the
// list.FilterMatchesMsg bubbles computes asynchronously so the model has settled
// on the query before the bar is read.
func startFilterModel(t *testing.T, filter string, apply bool) appModel {
	t.Helper()
	useTempBookmarks(t)
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	m.blurInput()
	step, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = step.(appModel)
	for _, r := range filter {
		var cmd tea.Cmd
		step, cmd = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = step.(appModel)
		if msg, ok := findFilterMatches(cmd); ok {
			step, _ = m.Update(msg)
			m = step.(appModel)
		}
	}
	if apply {
		step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m = step.(appModel)
	}
	return m
}

// TestStartBarNoMatchOffersOnlyTheKeyThatWorks is issue #116. On a zero-match
// filter there is no row to go to and none to bookmark, so "↵ go" and
// "b bookmark" are no-ops. Enter is a no-op too in the useful sense: bubbles
// refuses to apply a filter that matched nothing and drops back to the
// unfiltered list, which is what Esc does, so "enter apply" would be a second
// wrong promise. Esc is the only key on that screen worth naming.
//
// The applied-filter variant of this state is unreachable: pressing Enter on a
// zero-match filter leaves FilterState unfiltered, never FilterApplied.
func TestStartBarNoMatchOffersOnlyTheKeyThatWorks(t *testing.T) {
	m := startFilterModel(t, "zzzzzz", false)
	if _, ok := m.start.selected(); ok {
		t.Fatal("this test needs a filter that selects nothing")
	}
	if got := len(m.start.list.VisibleItems()); got != 0 {
		t.Fatalf("filter matched %d rows, want zero for this test", got)
	}
	if got, want := m.startBar(80, newStyles(true)).hints, "esc cancel"; got != want {
		t.Errorf("no-match start bar hints = %q, want %q", got, want)
	}
}

// TestStartBarNamesTheFilterStates: while a filter is open, "/ filter" is the
// mode you are already in. Use the words the links panel already established.
func TestStartBarNamesTheFilterStates(t *testing.T) {
	t.Run("empty filter", func(t *testing.T) {
		m := startFilterModel(t, "", false)
		if got, want := m.startBar(80, newStyles(true)).hints, "type to filter · esc cancel"; got != want {
			t.Errorf("start bar hints = %q, want %q", got, want)
		}
	})
	t.Run("filter being typed", func(t *testing.T) {
		m := startFilterModel(t, "plan", false)
		if got, want := m.startBar(80, newStyles(true)).hints, "enter apply · esc cancel"; got != want {
			t.Errorf("start bar hints = %q, want %q", got, want)
		}
	})
	t.Run("filter applied offers to clear it", func(t *testing.T) {
		m := startFilterModel(t, "plan", true)
		if _, ok := m.start.selected(); !ok {
			t.Fatal("this test needs a filter that selects a row")
		}
		hints := m.startBar(80, newStyles(true)).hints
		if !strings.Contains(hints, "esc clear filter") {
			t.Errorf("applied-filter start bar hints = %q, want an esc clear filter hint", hints)
		}
		if strings.Contains(hints, "/ filter") {
			t.Errorf("applied-filter start bar hints = %q, must not offer / filter again", hints)
		}
	})
}

// listFilterModel lands a user list and opens its filter, pumping the
// asynchronous list.FilterMatchesMsg so the model has settled on the query.
func listFilterModel(t *testing.T, filter string) appModel {
	t.Helper()
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	host := hostTarget(t, "@tilde.team")
	users, _ := ParseUsers([]byte(hostListBody()), "")
	m.history = []histNode{{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList}}
	m.pos, m.state, m.listReady, m.inputFocused = 0, stateList, true, false
	m.list = newList(m.common, host, users)
	m.list.setSize(m.common.width, m.common.bodyHeight())

	step, _ := m.Update(tea.KeyPressMsg{Code: '/'})
	m = step.(appModel)
	for _, r := range filter {
		var cmd tea.Cmd
		step, cmd = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = step.(appModel)
		if msg, ok := findFilterMatches(cmd); ok {
			step, _ = m.Update(msg)
			m = step.(appModel)
		}
	}
	if !m.list.filtering() {
		t.Fatal("expected the user list to be filtering")
	}
	return m
}

// TestListBarNamesTheFilterStates: the user list showed its resting hints while
// `/` was open, offering "/ filter" as a way into the mode it was already in.
func TestListBarNamesTheFilterStates(t *testing.T) {
	t.Run("empty filter", func(t *testing.T) {
		m := listFilterModel(t, "")
		if got, want := m.buildStatusBar().hints, "type to filter · esc cancel"; got != want {
			t.Errorf("list bar hints = %q, want %q", got, want)
		}
	})
	t.Run("filter being typed", func(t *testing.T) {
		m := listFilterModel(t, "a")
		if got := len(m.list.list.VisibleItems()); got == 0 {
			t.Fatal("this test needs a filter that matches at least one user")
		}
		if got, want := m.buildStatusBar().hints, "enter apply · esc cancel"; got != want {
			t.Errorf("list bar hints = %q, want %q", got, want)
		}
	})
	t.Run("no match", func(t *testing.T) {
		m := listFilterModel(t, "zzzzzz")
		if got := len(m.list.list.VisibleItems()); got != 0 {
			t.Fatalf("filter matched %d users, want zero for this test", got)
		}
		if got, want := m.buildStatusBar().hints, "esc cancel"; got != want {
			t.Errorf("list bar hints = %q, want %q", got, want)
		}
	})
	t.Run("drilled in: no back breadcrumb", func(t *testing.T) {
		m := listFilterModel(t, "a")
		prev := histNode{entry: Entry{Target: hostTarget(t, "@origin.example")}, state: stateReader}
		m.history = append([]histNode{prev}, m.history...)
		m.pos = 1
		if got := m.buildStatusBar().escTarget; got != "" {
			t.Errorf("filtering list escTarget = %q, want empty: Esc cancels the filter, it does not walk history", got)
		}
	})
}

// TestStartBarRestingHintsAreUnchanged pins the unfiltered bar: this fix must
// not disturb the screen users see at launch.
func TestStartBarRestingHintsAreUnchanged(t *testing.T) {
	useTempBookmarks(t)
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	m.blurInput()
	if _, ok := m.start.selected(); !ok {
		t.Fatal("the unfiltered startpage must select a row")
	}
	want := "↵ go · b bookmark · / filter · i target · ? help"
	if got := m.startBar(80, newStyles(true)).hints; got != want {
		t.Errorf("resting start bar hints = %q, want %q", got, want)
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
	// The hint list must match what joinHints actually builds: with no esc
	// target it appends "esc back" as well as "? help". Omitting it made this
	// width 11 cells short, which only ever passed because the bar used to
	// clip the address instead of shedding state. It now keeps the address and
	// drops "9 B", so an honest width is what keeps this test about latency.
	m.common.width = lipgloss.Width("@x.example / alice") + 1 +
		lipgloss.Width("9 B · ↑↓ scroll · r refresh · esc back · ? help")
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

const startHints = "↵ go · b bookmark · / filter · i target · ? help"

// TestStatusBarShedsWholeHints: the joined right group used to be truncated
// positionally, so a narrow bar cut a hint mid-word and lost "? help" first —
// the one hint that stands in for all the others.
func TestStatusBarShedsWholeHints(t *testing.T) {
	for _, width := range []int{45, 50, 60} {
		b := statusBar{meta: "28 entries", hints: startHints, width: width, styles: newStyles(true)}
		out := stripANSIForLandingTest(b.render())

		if !strings.Contains(out, "? help") {
			t.Errorf("width %d: %q dropped %q", width, out, "? help")
		}
		if strings.Contains(out, "…") {
			t.Errorf("width %d: %q cut a hint mid-word, want whole hints dropped", width, out)
		}
	}
}

// TestStatusBarKeepsRetryOnAFailedRequest: a failed request's hints are
// "r retry · ? help". Retry is the only useful action on that screen, and #83
// (issue #76) exists because the bar kept spending its width on less useful
// facts. Shedding must not undo that by pinning "? help" over it.
func TestStatusBarKeepsRetryOnAFailedRequest(t *testing.T) {
	for _, width := range []int{45, 60, 80} {
		b := statusBar{
			host: "@127.0.0.1", user: "nobody",
			escTarget: "trunc@127.0.0.1:2479", latency: "1ms",
			hints: "r retry · ? help", width: width, styles: newStyles(true),
		}
		out := stripANSIForLandingTest(b.render())
		if !strings.Contains(out, "r retry") {
			t.Errorf("width %d: %q dropped %q", width, out, "r retry")
		}
	}
}

// TestStatusBarKeepsEveryHintWhenItFits guards against over-eager shedding.
func TestStatusBarKeepsEveryHintWhenItFits(t *testing.T) {
	b := statusBar{meta: "28 entries", hints: startHints, width: 100, styles: newStyles(true)}
	out := stripANSIForLandingTest(b.render())
	for _, hint := range strings.Split(startHints, " · ") {
		if !strings.Contains(out, hint) {
			t.Errorf("width 100: %q dropped %q, want the full list", out, hint)
		}
	}
}

// TestStatusBarNarrowerThanHelpStillRenders: below the width of "? help"
// itself there is nothing to keep, and the existing ellipsis path takes over.
// The bar must not exceed its width or panic.
func TestStatusBarNarrowerThanHelpStillRenders(t *testing.T) {
	for _, width := range []int{1, 4, 8} {
		b := statusBar{host: "@tilde.team", hints: startHints, width: width, styles: newStyles(true)}
		out := b.render()
		if got := lipgloss.Width(out); got > width {
			t.Errorf("width %d: rendered %d cells, want at most %d", width, got, width)
		}
	}
}

// readerBar mirrors what buildStatusBar produces for a landed reader.
func readerBar(width int) statusBar {
	return statusBar{
		host: "@127.0.0.1", user: "alice", escTarget: "@127.0.0.1",
		latency: "2ms", meta: "1.2 KB", scroll: "42%",
		hints: "↑↓ scroll · r refresh · ? help",
		width: width, styles: newStyles(true),
	}
}

// crumbSurvives anchors on the left of the line. Do not use strings.Contains:
// the address also appears inside "◂ esc: <target>", which false-positives.
func crumbSurvives(out, crumb string) bool {
	return strings.HasPrefix(strings.TrimLeft(out, " "), crumb)
}

func TestStatusBarKeepsTheAddressDownTo45(t *testing.T) {
	for _, width := range []int{45, 60, 80, 100} {
		out := stripANSIForLandingTest(readerBar(width).render())
		if !crumbSurvives(out, "@127.0.0.1 / alice") {
			t.Errorf("width %d: %q clipped the address", width, out)
		}
		if got := lipgloss.Width(out); got > width {
			t.Errorf("width %d: rendered %d cells", width, got)
		}
	}
}

// TestStatusBarShedsStateInLadderOrder: a bar still showing a cheaper segment
// must still show every dearer one.
func TestStatusBarShedsStateInLadderOrder(t *testing.T) {
	for width := 30; width <= 110; width++ {
		out := stripANSIForLandingTest(readerBar(width).render())
		if strings.Contains(out, "2ms") && !strings.Contains(out, "1.2 KB") {
			t.Errorf("width %d: %q kept latency but dropped meta", width, out)
		}
		if strings.Contains(out, "1.2 KB") && !strings.Contains(out, "42%") {
			t.Errorf("width %d: %q kept meta but dropped scroll", width, out)
		}
		// scroll is rung 3 and the esc destination is rung 4, so the
		// destination is the dearer of the two: scroll surviving implies it
		// survives, not the other way round.
		if strings.Contains(out, "42%") && !strings.Contains(out, "◂ esc: @127.0.0.1") {
			t.Errorf("width %d: %q kept scroll but dropped the esc destination", width, out)
		}
	}
}

// listBar mirrors what buildStatusBar produces for a paged user list. It is
// the only state carrying `page`, which rung 3 sheds alongside `scroll`.
func listBar(width int) statusBar {
	return statusBar{
		host: "@tilde.team", escTarget: "@tilde.team",
		page: "page 2/4", latency: "2ms", meta: "12 users",
		hints: "↵ go · / filter · r refresh · ? help",
		width: width, styles: newStyles(true),
	}
}

// linkedReaderBar mirrors a reader with a focused link: the longest hint list
// the app produces, against a full complement of state.
func linkedReaderBar(width int) statusBar {
	return statusBar{
		host: "@127.0.0.1", user: "alice", escTarget: "@127.0.0.1",
		latency: "2ms", meta: "1.2 KB", scroll: "42%",
		hints: "link 1/2 · URL · ↵ go · y copy · tab next · r refresh",
		width: width, styles: newStyles(true),
	}
}

// TestStatusBarLadderAcrossStates covers the two states the spec names that
// readerBar does not reach. The list state matters most: it is the only one
// with `page`, so without it half of rung 3 goes unexercised.
func TestStatusBarLadderAcrossStates(t *testing.T) {
	for _, tt := range []struct {
		name  string
		bar   func(int) statusBar
		crumb string
	}{
		{"list", listBar, "@tilde.team"},
		{"reader with focused link", linkedReaderBar, "@127.0.0.1 / alice"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, width := range []int{45, 60, 80, 100} {
				out := stripANSIForLandingTest(tt.bar(width).render())
				if !crumbSurvives(out, tt.crumb) {
					t.Errorf("width %d: %q clipped the address", width, out)
				}
				if got := lipgloss.Width(out); got > width {
					t.Errorf("width %d: rendered %d cells", width, got)
				}
			}
			// Ladder order, swept: a bar showing a cheaper segment must still
			// show every dearer one.
			for width := 30; width <= 110; width++ {
				out := stripANSIForLandingTest(tt.bar(width).render())
				if strings.Contains(out, "2ms") && !strings.Contains(out, "users") &&
					!strings.Contains(out, "1.2 KB") {
					t.Errorf("width %d: %q kept latency but dropped meta", width, out)
				}
			}
		})
	}
}

// TestStatusBarListShedsPageWithScroll pins the half of rung 3 that readerBar
// cannot reach. `meta` is rung 2 and `page` is rung 3, so meta is the cheaper
// of the two and goes first: meta surviving implies page survives, never the
// reverse.
func TestStatusBarListShedsPageWithScroll(t *testing.T) {
	for width := 30; width <= 110; width++ {
		out := stripANSIForLandingTest(listBar(width).render())
		if strings.Contains(out, "12 users") && !strings.Contains(out, "page 2/4") {
			t.Errorf("width %d: %q kept meta but dropped page, which is dearer", width, out)
		}
	}
}

func TestStatusBarEscDegradesToBareAffordance(t *testing.T) {
	full := statusBar{escTarget: "trunc@127.0.0.1:2479", width: 80, styles: newStyles(true)}
	if got := stripANSIForLandingTest(full.render()); !strings.Contains(got, "◂ esc: trunc@127.0.0.1:2479") {
		t.Fatalf("precondition: %q, want the full destination", got)
	}

	short := full
	short.escShort = true
	got := stripANSIForLandingTest(short.render())
	if !strings.Contains(got, "◂ esc") {
		t.Errorf("%q dropped the esc affordance entirely", got)
	}
	if strings.Contains(got, "trunc@127.0.0.1:2479") {
		t.Errorf("%q kept the destination, want it dropped", got)
	}
}

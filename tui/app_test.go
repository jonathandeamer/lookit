package tui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/jonathandeamer/lookit/finger"
)

// stubFetch returns a FetchFunc that fails the test if called.
func stubFetch(t *testing.T) FetchFunc {
	t.Helper()
	return func(_ context.Context, _ finger.Target) ([]byte, finger.Meta, error) {
		t.Fatalf("fetch should not be called")
		return nil, finger.Meta{}, nil
	}
}

// fetchOnce returns a fetch func yielding fixed bytes and records the targets.
func fetchRecorder(body string) (FetchFunc, *[]string) {
	var seen []string
	f := func(_ context.Context, t finger.Target) ([]byte, finger.Meta, error) {
		seen = append(seen, t.Raw)
		return []byte(body), finger.Meta{Addr: t.HostPort, Bytes: len(body)}, nil
	}
	return f, &seen
}

func readerWithFocusedLink(t *testing.T, fetch FetchFunc, link Link) appModel {
	t.Helper()
	m := newApp(fetch, colorprofile.NoTTY)
	target := hostTarget(t, "viewer@origin.example")
	entry := Entry{Target: target, Body: []byte(link.Raw + "\n")}
	m.history = []histNode{{entry: entry, state: stateReader, links: []Link{link}, linkIdx: 0}}
	m.pos, m.state, m.inputFocused = 0, stateReader, false
	m.reader.focusedLink = 0
	m.reader.setEntryWithLinks(entry, []Link{link})
	return m
}

func TestReaderFocusedLinkStatus(t *testing.T) {
	definite := Link{Kind: LinkFinger, Action: ActionDrill, Raw: "alice@tilde.team", Target: finger.Target{HostPort: "tilde.team:79"}}
	ambiguous := Link{Kind: LinkFinger, Action: ActionCopy, Raw: "alice@tilde.team", Ambiguous: true, Target: finger.Target{HostPort: "tilde.team:79"}}
	blocked := Link{Kind: LinkFinger, Action: ActionCopy, Raw: "alice@tilde.team@relay.example", Blocked: "cross-relay"}
	tests := []struct {
		name string
		link Link
		want string
	}{
		{"definite", definite, "link 1/1 · finger · ↵ go · y copy · tab next · r refresh"},
		{"ambiguous", ambiguous, "link 1/1 · address (ambiguous) · f go · y copy · tab next · r refresh"},
		{"url", Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"}, "link 1/1 · url · y copy · tab next · r refresh"},
		{"blocked", blocked, "link 1/1 · forwarded finger · y copy · cross-relay · tab next · r refresh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := readerWithFocusedLink(t, stubFetch(t), tt.link)
			got := m.buildStatusBar().hints
			if got != tt.want {
				t.Fatalf("status hints = %q, want %q", got, tt.want)
			}
		})
	}
}

func linksPanelModel(t *testing.T, fetch FetchFunc, links []Link) appModel {
	t.Helper()
	if len(links) == 0 {
		t.Fatal("linksPanelModel requires at least one link")
	}
	m := newApp(fetch, colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	target := hostTarget(t, "viewer@origin.example")
	entry := Entry{Target: target, Body: []byte(links[0].Raw + "\n")}
	m.history = []histNode{{entry: entry, state: stateReader, links: links, linkIdx: 0}}
	m.pos, m.state, m.inputFocused = 0, stateReader, false
	m.reader.focusedLink = 0
	m.reader.setEntryWithLinks(entry, links)
	m.showingLinks = true
	m.linksPanel = newLinksPanel(m.common, links)
	m.linksPanel.setSize(m.common.width, m.common.bodyHeight())
	if _, ok := m.linksPanel.selected(); !ok {
		t.Fatal("links panel should select its first row")
	}
	return m
}

func TestLinksPanelStatus(t *testing.T) {
	link := Link{Kind: LinkFinger, Action: ActionDrill, Raw: "alice@tilde.team", Target: finger.Target{HostPort: "tilde.team:79"}}
	t.Run("unfiltered", func(t *testing.T) {
		m := linksPanelModel(t, stubFetch(t), []Link{link})
		if got, want := m.buildStatusBar().hints, "↑/↓ move · / filter · esc back · ↵ go · y copy"; got != want {
			t.Fatalf("status hints = %q, want %q", got, want)
		}
	})
	t.Run("empty filter", func(t *testing.T) {
		m := linksPanelModel(t, stubFetch(t), []Link{link})
		next, _ := m.Update(tea.KeyPressMsg{Code: '/'})
		m = next.(appModel)
		if got, want := m.buildStatusBar().hints, "type to filter · esc cancel"; got != want {
			t.Fatalf("status hints = %q, want %q", got, want)
		}
	})
	t.Run("non-empty filter", func(t *testing.T) {
		m := linksPanelModel(t, stubFetch(t), []Link{link})
		for _, msg := range []tea.KeyPressMsg{{Code: '/'}, {Code: 'a', Text: "a"}} {
			next, _ := m.Update(msg)
			m = next.(appModel)
		}
		if got, want := m.buildStatusBar().hints, "enter apply · esc cancel"; got != want {
			t.Fatalf("status hints = %q, want %q", got, want)
		}
	})
	t.Run("applied filter", func(t *testing.T) {
		m := linksPanelModel(t, stubFetch(t), []Link{link})
		for _, msg := range []tea.KeyPressMsg{{Code: '/'}, {Code: 'a', Text: "a"}, {Code: tea.KeyEnter}} {
			next, _ := m.Update(msg)
			m = next.(appModel)
		}
		if got, want := m.buildStatusBar().hints, "↑/↓ move · esc clear filter · ↵ go · y copy"; got != want {
			t.Fatalf("status hints = %q, want %q", got, want)
		}
	})
	for _, flash := range []string{"copied alice@tilde.team", "cross-relay"} {
		t.Run("flash overrides resting hints: "+flash, func(t *testing.T) {
			m := linksPanelModel(t, stubFetch(t), []Link{link})
			m.flash = flash
			if got, want := m.statusBarModel().hints, m.flash; got != want {
				t.Fatalf("status hints = %q, want flash %q", got, want)
			}
		})
	}
}

func fetchTargetRecorder(body string) (FetchFunc, *[]finger.Target) {
	var seen []finger.Target
	f := func(_ context.Context, t finger.Target) ([]byte, finger.Meta, error) {
		seen = append(seen, t)
		return []byte(body), finger.Meta{Addr: t.HostPort, Bytes: len(body)}, nil
	}
	return f, &seen
}

func hostListBody() string {
	return "users currently logged in are:\n\nalrs\tdtracker\tkapad\n"
}

func hostListBodyWithPreamble() string {
	return "welcome to tilde.team\n\n" +
		"hello example.net,\n" +
		"users currently logged in are:\n\n" +
		"alrs\tdtracker\tkapad\n"
}

// manyUserGridBody builds a parseable host listing with n users laid out three
// per line, enough to span several paginated list pages in tests.
func manyUserGridBody(n int) string {
	var b strings.Builder
	b.WriteString("users currently logged in are:\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "user%02d", i)
		if (i+1)%3 == 0 {
			b.WriteByte('\n')
		} else {
			b.WriteByte('\t')
		}
	}
	if n%3 != 0 {
		b.WriteByte('\n')
	}
	return b.String()
}

func TestHostFetchThatParsesOpensList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@tilde.team")
	entry := Entry{Target: target, Body: []byte(hostListBody()), Meta: finger.Meta{Addr: target.HostPort}}

	next, _ := deliverNavigationResult(m, fetchResultMsg{entry: entry})
	got := next.(appModel)

	if got.state != stateList {
		t.Fatalf("state = %d, want stateList", got.state)
	}
	if len(got.history) != 1 || got.pos != 0 || got.history[0].state != stateList {
		t.Fatalf("history=%d pos=%d, want one list node", len(got.history), got.pos)
	}
	sel, ok := got.list.selected()
	if !ok || sel.login != "alrs" {
		t.Fatalf("list selection = %+v ok=%v, want alrs", sel, ok)
	}
}

func TestHostFetchWithBodyAndReadErrorCanOpenList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@telehack.com")
	body := []byte("TELEHACK SYSTEM STATUS  2026-May-29  06:47:34\n" +
		"112 users  load 0.93  up 87d\n\n" +
		" port username   status                last what       where\n" +
		" ---- --------   ------                ---- ----       -----\n" +
		" 0    operator   System Operator       1m              console\n" +
		" 69   underwood  AN/FPS-118 OTH-B      0s              Vauxhall Cross, UK\n")
	entry := Entry{
		Target: target,
		Body:   body,
		Meta:   finger.Meta{Addr: target.HostPort, Bytes: len(body)},
		Err:    errors.New("read response: connection reset by peer"),
	}

	next, _ := deliverNavigationResult(m, fetchResultMsg{entry: entry})
	got := next.(appModel)

	if got.state != stateList {
		t.Fatalf("state = %d, want stateList", got.state)
	}
	sel, ok := got.list.selected()
	if !ok || sel.login != "operator" {
		t.Fatalf("list selection = %+v ok=%v, want operator", sel, ok)
	}
}

func TestHostListViewKeepsPreambleWithoutRawUserGrid(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	target := hostTarget(t, "@tilde.team")
	entry := Entry{Target: target, Body: []byte(hostListBodyWithPreamble()), Meta: finger.Meta{Addr: target.HostPort}}

	next, _ := deliverNavigationResult(m, fetchResultMsg{entry: entry})
	got := next.(appModel)
	view := got.View().Content

	if !strings.Contains(view, "welcome to tilde.team") {
		t.Fatalf("list view missing preamble: %q", view)
	}
	if !strings.Contains(view, "hello example.net") {
		t.Fatalf("list view missing host greeting: %q", view)
	}
	if strings.Contains(view, "alrs\tdtracker\tkapad") {
		t.Fatalf("list view duplicated raw user grid: %q", view)
	}
	if !strings.Contains(view, "alrs") {
		t.Fatalf("list view missing selectable user: %q", view)
	}
}

func TestHostFetchThatDeclinesStaysInReader(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@tilde.town")
	entry := Entry{Target: target, Body: []byte("just a banner\n"), Meta: finger.Meta{Addr: target.HostPort}}

	next, _ := deliverNavigationResult(m, fetchResultMsg{entry: entry})
	got := next.(appModel)

	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader", got.state)
	}
	if !strings.Contains(got.reader.viewport.View(), "just a banner") {
		t.Fatalf("reader viewport missing body: %q", got.reader.viewport.View())
	}
}

func TestUserFetchStaysInReader(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "alice@plan.cat")
	entry := Entry{Target: target, Body: []byte("Plan: hi\n"), Meta: finger.Meta{Addr: target.HostPort}}

	next, _ := deliverNavigationResult(m, fetchResultMsg{entry: entry})
	got := next.(appModel)

	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader", got.state)
	}
}

func TestEnterInListDrillsIntoUser(t *testing.T) {
	fetch, seen := fetchRecorder("Plan: hi\n")
	m := newApp(fetch, colorprofile.NoTTY)
	// Put the app in list state for @tilde.team.
	host := hostTarget(t, "@tilde.team")
	m.history = []histNode{{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList}}
	m.pos = 0
	users, _ := ParseUsers([]byte(hostListBody()), "")
	m.list = newList(m.common, host, users)
	m.state = stateList
	m.inputFocused = false // Enter must reach the list, not the input

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)

	// Drilling keeps the list on screen while loading (no eager switch to the
	// reader, which used to flash the previous profile for a frame).
	if got.pending == nil || got.state != stateList {
		t.Fatalf("after drill: pending=%#v state=%d, want pending state=stateList", got.pending, got.state)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want fetch command")
	}
	runCmds(cmd) // run the fetch command (may be batched with spinner tick)
	if len(*seen) != 1 || (*seen)[0] != "alrs@tilde.team" {
		t.Fatalf("fetched targets = %v, want [alrs@tilde.team]", *seen)
	}
	// When the result lands it routes to the reader.
	landed, _ := got.Update(fetchResultMsg{reqID: got.reqSeq, entry: Entry{Target: hostTarget(t, "alrs@tilde.team"), Body: []byte("Plan: hi\n")}})
	if landed.(appModel).state != stateReader {
		t.Fatalf("after the drilled result lands, state = %d, want stateReader", landed.(appModel).state)
	}
}

func TestMenuListKeepsPreambleAndDrillsIntoExplicitTarget(t *testing.T) {
	fetch, seen := fetchRecorder("Plan: ring entry\n")
	m := newApp(fetch, colorprofile.NoTTY)
	host := hostTarget(t, "ring@thebackupbox.net")
	body := []byte("This is the finger ring!\n" +
		"and now for the list:\n" +
		"=> 2026-05-25 finger://tilde.team/yalla\n")
	m.history = []histNode{{entry: Entry{Target: host, Body: body}, state: stateList}}
	m.pos = 0
	users, ok := ParseUsers(body, "")
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	m.list = newListWithPreamble(m.common, host, users, body, false)
	m.state = stateList
	m.inputFocused = false // Enter must reach the list, not the input

	view := m.View().Content
	if !strings.Contains(view, "This is the finger ring!") {
		t.Fatalf("list view missing preamble: %q", view)
	}
	if strings.Contains(view, "=> 2026-05-25") {
		t.Fatalf("list view duplicated raw ring row: %q", view)
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if got.pending == nil || got.state != stateList {
		t.Fatalf("after drill: pending=%#v state=%d, want pending state=stateList", got.pending, got.state)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want fetch command")
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != "yalla@tilde.team" {
		t.Fatalf("fetched targets = %v, want [yalla@tilde.team]", *seen)
	}
}

func TestEscInDrilledReaderRestoresList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	user := hostTarget(t, "bob@tilde.team")
	m.history = []histNode{
		{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList},
		{entry: Entry{Target: user, Body: []byte("Login: bob\n")}, state: stateReader},
	}
	m.pos = 1
	m.state = stateReader
	m.inputFocused = false // Esc must reach content, not the input

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)

	if got.state != stateList || got.pos != 0 {
		t.Fatalf("state=%d pos=%d, want list/0 after Esc", got.state, got.pos)
	}
}

func TestEscInListReturnsToStart(t *testing.T) {
	useTempBookmarks(t)
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	m.history = []histNode{{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList}}
	m.pos = 0
	m.state = stateList
	m.inputFocused = false // Esc must reach the list, not the input
	users, _ := ParseUsers([]byte(hostListBody()), "")
	m.list = newList(m.common, host, users)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)

	if got.state != stateStart || got.pos != -1 {
		t.Fatalf("state=%d pos=%d, want start/-1 after Esc", got.state, got.pos)
	}
	// Focus follows how you arrived: Esc from content lands on the startpage
	// with a row selected, not in the input.
	if got.inputFocused {
		t.Fatal("Esc from the list should land content-focused on the startpage")
	}
	if _, ok := got.start.selected(); !ok {
		t.Fatal("startpage should have a selected row after Esc")
	}
	if cmd != nil && isQuit(cmd) {
		t.Fatal("Esc in list must not quit while history is non-empty")
	}
}

func TestEscInReaderHomeQuits(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil || !isQuit(cmd) {
		t.Fatal("Esc at reader home should quit")
	}
}

func TestCtrlCQuitsFromList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.state = stateList
	m.list = newList(m.common, hostTarget(t, "@tilde.team"), []User{{Login: "alrs"}})

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil || !isQuit(cmd) {
		t.Fatal("Ctrl+C should quit from list state")
	}
}

func TestWindowSizePropagatesToBothSubModels(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	// listReady set so the guarded list-resize branch runs (must not panic).
	m.listReady = true
	m.state = stateList
	m.list = newList(m.common, host, []User{{Login: "alrs"}})

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	got := next.(appModel)

	if got.common.width != 100 || got.common.height != 30 {
		t.Fatalf("common size = %dx%d, want 100x30", got.common.width, got.common.height)
	}
	if got.reader.viewport.Width() != 100 {
		t.Fatalf("reader viewport width = %d, want 100", got.reader.viewport.Width())
	}
}

func TestWindowSizeReservesBarRow(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = step.(appModel)
	// The top chrome is a single target row (the wordmark moved to the about
	// screen). reader viewport = 24 - 1 (chrome) - 1 (bar) = 22.
	if m.reader.viewport.Height() != 22 {
		t.Fatalf("viewport height = %d, want 22 (target + bar reserved)", m.reader.viewport.Height())
	}
}

// isQuit runs a command and reports whether it produced tea.QuitMsg.
func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// runCmds executes all leaf commands in a potentially batched tea.Cmd,
// unwrapping tea.BatchMsg recursively. This is needed in tests that call
// cmd() directly to trigger side effects (e.g. populating a fetchRecorder),
// because submit/drill now batch the fetch with the spinner tick.
func runCmds(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			runCmds(c)
		}
		return
	}
	// non-batch: side effects already executed
}

func TestCtrlCQuitsFromReader(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil || !isQuit(cmd) {
		t.Fatal("Ctrl+C should quit from reader state")
	}
}

func TestColorProfileMsgPropagates(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	next, _ := m.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	got := next.(appModel)
	if got.common.profile != colorprofile.TrueColor {
		t.Fatalf("common.profile = %v, want TrueColor", got.common.profile)
	}
	if got.reader.profile != colorprofile.TrueColor {
		t.Fatalf("reader.profile = %v, want TrueColor", got.reader.profile)
	}
}

func TestEscWhileFilteringDelegatesToList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	users, _ := ParseUsers([]byte(hostListBody()), "")
	m.history = []histNode{{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList}}
	m.pos = 0
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.state = stateList
	m.inputFocused = false // keys must reach the list, not the input

	// Enter filtering mode (the list's default filter key is "/").
	next, _ := m.Update(tea.KeyPressMsg{Code: '/'})
	m = next.(appModel)
	if !m.list.filtering() {
		t.Fatal("expected list to be filtering after '/'")
	}

	// Esc while filtering must be delegated to the list (cancels the filter),
	// NOT intercepted as a back-out to the reader.
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)
	if got.state != stateList {
		t.Fatalf("state = %d, want stateList (Esc while filtering must not back out)", got.state)
	}
}

func barFor(t *testing.T, entry Entry) string {
	t.Helper()
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 100, 24
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: entry})
	return step.(appModel).statusBarModel().render()
}

func TestTruncatedHostFetchMarksListIncomplete(t *testing.T) {
	host := hostTarget(t, "@tilde.team")
	bar := barFor(t, Entry{Target: host, Body: []byte(hostListBody()),
		Meta: finger.Meta{Addr: host.HostPort, Truncated: true}})
	if !strings.Contains(bar, "partial (truncated)") {
		t.Fatalf("bar = %q, want partial (truncated)", bar)
	}
}

func TestTruncatedReaderFetchMarksReaderTruncated(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	bar := barFor(t, Entry{Target: target, Body: []byte("Plan: hi\n"),
		Meta: finger.Meta{Addr: target.HostPort, Truncated: true}})
	if !strings.Contains(bar, "partial (truncated)") {
		t.Fatalf("bar = %q, want partial (truncated)", bar)
	}
}

func TestErroredHostFetchWithBodyMarksListIncomplete(t *testing.T) {
	host := hostTarget(t, "@tilde.team")
	bar := barFor(t, Entry{Target: host, Body: []byte(hostListBody()),
		Meta: finger.Meta{Addr: host.HostPort}, Err: errors.New("connection reset")})
	if !strings.Contains(bar, "partial (error)") {
		t.Fatalf("bar = %q, want partial (error)", bar)
	}
}

func TestCompleteHostFetchListNotMarkedIncomplete(t *testing.T) {
	host := hostTarget(t, "@tilde.team")
	bar := barFor(t, Entry{Target: host, Body: []byte(hostListBody()), Meta: finger.Meta{Addr: host.HostPort}})
	if strings.Contains(bar, "partial") {
		t.Fatalf("bar = %q, should not flag partial", bar)
	}
}

// captureFetch returns a fetch func that records the target it was called with.
func captureFetch(got *finger.Target) FetchFunc {
	return func(_ context.Context, tg finger.Target) ([]byte, finger.Meta, error) {
		*got = tg
		return []byte("x\n"), finger.Meta{}, nil
	}
}

func drillFirstUser(t *testing.T, host finger.Target, users []User, fetch FetchFunc) tea.Cmd {
	t.Helper()
	m := newApp(fetch, colorprofile.NoTTY)
	m.history = []histNode{{entry: Entry{Target: host}, state: stateList}}
	m.pos = 0
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.state = stateList
	m.inputFocused = false // Enter must reach the list, not the input
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if got.pending == nil {
		t.Fatal("expected drilling to start a request")
	}
	if got.state != stateList {
		t.Fatalf("drill should keep the list on screen while loading (state=%d)", got.state)
	}
	if cmd == nil {
		t.Fatal("expected a fetch command from drilling")
	}
	return cmd
}

func TestDrillServerSuppliedTargetPinnedToPort79(t *testing.T) {
	var got finger.Target
	host := hostTarget(t, "@thebackupbox.net")
	// A ring-style entry whose server-supplied target carries a hostile port.
	users := []User{{Login: "evil", Target: "evil@example.com:22"}}

	cmd := drillFirstUser(t, host, users, captureFetch(&got))
	runCmds(cmd)

	if got.HostPort != "example.com:79" {
		t.Fatalf("HostPort = %q, want example.com:79 (server-supplied port must be pinned to 79)", got.HostPort)
	}
}

func TestDrillPinnedServerTargetRefillsInputWithPinnedRaw(t *testing.T) {
	var fetched finger.Target
	host := hostTarget(t, "@thebackupbox.net")
	users := []User{{Login: "evil", Target: "evil@example.com:22"}}

	cmd := drillFirstUser(t, host, users, captureFetch(&fetched))
	runCmds(cmd)

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.history = []histNode{{entry: Entry{Target: fetched, Body: []byte("Plan: hi\n")}, state: stateReader}}
	m.pos = 0
	m.reader.setEntry(m.history[0].entry)
	m.inputFocused = false

	step, _ := m.Update(tea.KeyPressMsg{Code: 'i'})
	got := step.(appModel)

	if got.input.Value() != "evil@example.com:79" {
		t.Fatalf("input value = %q, want pinned target evil@example.com:79", got.input.Value())
	}
}

func TestDrillServerSuppliedTargetKeepsCrossHost(t *testing.T) {
	var got finger.Target
	host := hostTarget(t, "@thebackupbox.net")
	// A legitimate Finger Ring entry points at another host on port 79.
	users := []User{{Login: "yalla", Target: "yalla@tilde.team"}}

	cmd := drillFirstUser(t, host, users, captureFetch(&got))
	runCmds(cmd)

	if got.HostPort != "tilde.team:79" {
		t.Fatalf("HostPort = %q, want tilde.team:79 (cross-host drilling must be preserved)", got.HostPort)
	}
}

func TestDrillServerSuppliedForwardedTargetFlashesRefusal(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@thebackupbox.net")
	users := []User{{Login: "alice", Target: "alice@tilde.team@thebackupbox.net"}}
	m.history = []histNode{{entry: Entry{Target: host}, state: stateList}}
	m.pos = 0
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.list.list.Select(0)
	m.state = stateList
	m.inputFocused = false

	step, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := step.(appModel)

	if cmd == nil {
		t.Fatal("drill refusal should return a clear-flash command")
	}
	if got.pending != nil {
		t.Fatal("server-supplied forwarded target must not start a request")
	}
	if got.flash != finger.ErrServerForwarding.Error() {
		t.Fatalf("flash = %q, want %q", got.flash, finger.ErrServerForwarding.Error())
	}
}

func TestDrillSameHostKeepsUserTypedPort(t *testing.T) {
	var got finger.Target
	host := hostTarget(t, "@plan.cat:7979") // user typed an explicit port
	users := []User{{Login: "alice"}}       // no server-supplied target

	cmd := drillFirstUser(t, host, users, captureFetch(&got))
	runCmds(cmd)

	if got.HostPort != "plan.cat:7979" {
		t.Fatalf("HostPort = %q, want plan.cat:7979 (user-typed port must be preserved)", got.HostPort)
	}
}

func genericListBody() string {
	// No Login header / online cue / "> " marker: forces the generic fallback.
	return "the crew:\nbetsy\nMelchizedek\nOleander\nStarbloom\n"
}

func TestGenericTruncatedListShowsBothFlags(t *testing.T) {
	host := hostTarget(t, "@unknown.host")
	bar := barFor(t, Entry{Target: host, Body: []byte(genericListBody()),
		Meta: finger.Meta{Addr: host.HostPort, Truncated: true}})
	if !strings.Contains(bar, "auto-detected") {
		t.Fatalf("bar = %q, want auto-detected flag", bar)
	}
	if !strings.Contains(bar, "partial (truncated)") {
		t.Fatalf("bar = %q, want partial (truncated) flag", bar)
	}
}

func TestGenericTruncatedListPrioritizesPartialFlagAtNarrowWidth(t *testing.T) {
	host := hostTarget(t, "@unknown.host")
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 21, 24
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{
		Target: host,
		Body:   []byte(genericListBody()),
		Meta:   finger.Meta{Addr: host.HostPort, Truncated: true},
	}})
	bar := ansi.Strip(step.(appModel).statusBarModel().render())
	if !strings.Contains(bar, "partial (truncated)") {
		t.Fatalf("narrow generic truncated bar = %q, want partial flag", bar)
	}
	if strings.Contains(bar, "auto-detected") {
		t.Fatalf("narrow generic truncated bar = %q, should prioritize partial flag", bar)
	}
}

func TestGenericHostFetchOpensFlaggedList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 100, 24
	target := hostTarget(t, "@unknown.host")
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: target, Body: []byte(genericListBody()), Meta: finger.Meta{Addr: target.HostPort}}})
	got := step.(appModel)
	if got.state != stateList || !got.list.generic {
		t.Fatalf("state=%d generic=%v, want list/true", got.state, got.list.generic)
	}
	if !strings.Contains(got.statusBarModel().render(), "auto-detected") {
		t.Fatalf("bar missing auto-detected flag")
	}
}

func TestVViewsSourceOnGenericList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@unknown.host")
	entry := Entry{Target: target, Body: []byte(genericListBody()), Meta: finger.Meta{Addr: target.HostPort}}
	opened, _ := deliverNavigationResult(m, fetchResultMsg{entry: entry})
	m = opened.(appModel)

	next, _ := m.Update(tea.KeyPressMsg{Code: 'v'})
	got := next.(appModel)

	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader after v", got.state)
	}
	if !got.showingRaw {
		t.Fatal("showingRaw = false, want true after viewing raw")
	}
	if !strings.Contains(got.reader.viewport.View(), "Melchizedek") {
		t.Fatalf("reader viewport missing raw body: %q", got.reader.viewport.View())
	}

	back, _ := got.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if back.(appModel).state != stateList {
		t.Fatalf("state = %d, want stateList after Esc", back.(appModel).state)
	}
}

func TestVViewsSourceOnRecognizedList(t *testing.T) {
	// 'v' views the source on any list, recognized ones included.
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@tilde.team")
	entry := Entry{Target: target, Body: []byte(hostListBody()), Meta: finger.Meta{Addr: target.HostPort}}
	opened, _ := deliverNavigationResult(m, fetchResultMsg{entry: entry})
	m = opened.(appModel)

	next, _ := m.Update(tea.KeyPressMsg{Code: 'v'})
	got := next.(appModel)
	if !got.showingRaw || got.state != stateReader {
		t.Fatalf("v should view source on a recognized list: showingRaw=%v state=%d", got.showingRaw, got.state)
	}
	// The raw body carries the header line the parsed list view omits.
	if !strings.Contains(got.reader.viewport.View(), "users currently logged in are:") {
		t.Fatalf("raw view missing the unprocessed body: %q", got.reader.viewport.View())
	}

	back, _ := got.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if back.(appModel).state != stateList {
		t.Fatalf("state = %d, want stateList after Esc from raw view", back.(appModel).state)
	}
}

func TestVTogglesRawBodyOnProfile(t *testing.T) {
	// 'v' toggles "view source" on a profile too; a second 'v' restores it.
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "alice@plan.cat")
	body := "Login: alice\nPlan:\nhello from the raw body\n"
	opened, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: target, Body: []byte(body)}})
	m = opened.(appModel)
	if m.state != stateReader {
		t.Fatalf("precondition: a profile opens in the reader (state=%d)", m.state)
	}
	rendered := m.reader.viewport.View()

	raw, _ := m.Update(tea.KeyPressMsg{Code: 'v'})
	gotRaw := raw.(appModel)
	if !gotRaw.showingRaw {
		t.Fatal("v should enter source view on a profile")
	}
	rawView := gotRaw.reader.viewport.View()
	if !strings.Contains(rawView, "hello from the raw body") {
		t.Fatalf("raw view missing body text: %q", rawView)
	}
	if rawView == rendered {
		t.Fatal("raw view should differ from the rendered profile (view source)")
	}

	off, _ := gotRaw.Update(tea.KeyPressMsg{Code: 'v'})
	gotOff := off.(appModel)
	if gotOff.showingRaw {
		t.Fatal("a second v should exit source view")
	}
	if gotOff.state != stateReader {
		t.Fatalf("exiting raw on a profile returns to the reader (state=%d)", gotOff.state)
	}
}

func TestBookmarkKeyTogglesCurrentTargetInRawReader(t *testing.T) {
	path := useTempBookmarks(t)
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "alice@plan.cat")
	opened, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{
		Target: target,
		Body:   []byte("Login: alice\nPlan:\nhello\n"),
	}})
	m = opened.(appModel)
	raw, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = raw.(appModel)
	if !m.showingRaw {
		t.Fatal("precondition: v did not enter raw reader mode")
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bookmark written from raw reader: %v", err)
	}
	if got, want := string(data), "alice@plan.cat\n"; got != want {
		t.Fatalf("bookmarks = %q, want %q", got, want)
	}
	if !strings.Contains(m.flash, "bookmarked alice@plan.cat") {
		t.Fatalf("flash = %q, want bookmark confirmation", m.flash)
	}
}

func TestEscBackDoesNotRefetch(t *testing.T) {
	// Esc navigates back through history without re-fetching.
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	userT := hostTarget(t, "bob@tilde.team")

	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	step, _ = deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: userT, Body: []byte("Login: bob\n")}})
	m = step.(appModel)

	if len(m.history) != 2 || m.pos != 1 || m.state != stateReader {
		t.Fatalf("history=%d pos=%d state=%d, want 2/1/reader", len(m.history), m.pos, m.state)
	}

	// Esc backs to the list (no re-fetch; stubFetch would panic if called).
	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = step.(appModel)
	if m.pos != 0 || m.state != stateList {
		t.Fatalf("after Esc back: pos=%d state=%d, want 0/list", m.pos, m.state)
	}
}

func TestRouteFetchSnapshotsListBeforeReplacingIt(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	nextHost := hostTarget(t, "@sdf.org")

	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	m.list.list.SetFilterText("kap")

	step, _ = deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: nextHost, Body: []byte(hostListBody())}})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = step.(appModel)

	if got := m.list.list.FilterValue(); got != "kap" {
		t.Fatalf("restored filter = %q, want kap", got)
	}
	sel, ok := m.list.selected()
	if !ok || sel.login != "kapad" {
		t.Fatalf("restored selection = %+v ok=%v, want kapad", sel, ok)
	}
}

func TestNewNavigationTruncatesForwardTail(t *testing.T) {
	// After fetching a, b; Esc-backing to a; then fetching c, the forward tail
	// (b) must be truncated: history = [a, c], pos = 1.
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	a := hostTarget(t, "@a.example")
	b := hostTarget(t, "@b.example")
	c := hostTarget(t, "@c.example")

	for _, tg := range []finger.Target{a, b} {
		step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: tg, Body: []byte(hostListBody())}})
		m = step.(appModel)
	}
	// Esc back to a (pos=0).
	step, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = step.(appModel)
	// Now fetch c — this must truncate the forward tail (b).
	fetch, _ := fetchRecorder(hostListBody())
	m.common.fetch = fetch
	step, _ = deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: c, Body: []byte(hostListBody())}})
	m = step.(appModel)

	if len(m.history) != 2 || m.pos != 1 {
		t.Fatalf("history=%d pos=%d, want 2/1 (forward tail truncated)", len(m.history), m.pos)
	}
	if got := m.history[1].entry.Target.Raw; got != c.Raw {
		t.Fatalf("head = %q, want %q", got, c.Raw)
	}
}

func TestAltLeftAtRootIsNoOp(t *testing.T) {
	// Alt+← is now inert (navigation moved to Esc); must not quit or change pos.
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt})
	if cmd != nil && isQuit(cmd) {
		t.Fatal("Alt+← on landing must not quit")
	}
	if got := step.(appModel); got.pos != -1 {
		t.Fatalf("pos = %d, want -1 (unchanged)", got.pos)
	}
}

func TestViewIncludesBreadcrumbBar(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	host := hostTarget(t, "@tilde.team")
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)

	view := m.View().Content
	if !strings.Contains(view, "@tilde.team") {
		t.Fatalf("view missing breadcrumb host:\n%s", view)
	}
	if !strings.Contains(view, "? help") {
		t.Fatalf("view missing help hint:\n%s", view)
	}
}

func TestLandingViewShowsStartpage(t *testing.T) {
	useTempBookmarks(t)
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)

	view := stripANSIForLandingTest(m.View().Content)
	for _, want := range []string{"type a target", "COMMUNITIES", "@plan.cat"} {
		if !strings.Contains(view, want) {
			t.Fatalf("startpage missing %q:\n%s", want, view)
		}
	}
}

func TestQuestionMarkTogglesHelpOverlay(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	host := hostTarget(t, "@tilde.team")
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)

	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = step.(appModel)
	if !m.help {
		t.Fatal("help should be open after '?'")
	}
	if !strings.Contains(m.View().Content, "move") {
		t.Fatalf("help overlay missing keymap:\n%s", m.View().Content)
	}

	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if step.(appModel).help {
		t.Fatal("any key should close the help overlay")
	}
}

// TestHelpToggleDoesNotRepaginateList guards the fix for the help panel
// reflowing the list: because the panel is an overlay, opening it must not
// change the list's pagination (which is derived from the list's height).
func TestHelpToggleDoesNotRepaginateList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	host := hostTarget(t, "@tilde.team")
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: host, Body: []byte(manyUserGridBody(30))}})
	m = step.(appModel)
	if m.state != stateList {
		t.Fatalf("state=%d, want stateList (body did not parse as a user list)", m.state)
	}
	before := m.list.list.Paginator.TotalPages
	if before < 2 {
		t.Fatalf("test needs a multi-page list to exercise repagination; TotalPages=%d", before)
	}

	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = step.(appModel)
	if !m.help {
		t.Fatal("help should be open after '?'")
	}
	if got := m.list.list.Paginator.TotalPages; got != before {
		t.Fatalf("opening the help panel repaginated the list: TotalPages %d -> %d", before, got)
	}
}

func TestQuestionMarkWhileFilteringDoesNotOpenHelp(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	users, _ := ParseUsers([]byte(hostListBody()), "")
	m.history = []histNode{{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList}}
	m.pos = 0
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.state = stateList
	m.inputFocused = false // keys must reach the list, not the input

	step, _ := m.Update(tea.KeyPressMsg{Code: '/'})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	if step.(appModel).help {
		t.Fatal("'?' must be a literal filter character while filtering, not open help")
	}
}

func TestQuestionMarkFromReaderOpensHelp(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	// Drive a fetch so we reach a content-focused reader state.
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n")}})
	m = step.(appModel)
	// Now inputFocused==false; '?' should open help.
	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	if !step.(appModel).help {
		t.Fatal("'?' should open help from content-focused reader state")
	}
}

func TestReaderHelpContext(t *testing.T) {
	t.Run("without links omits navigation", func(t *testing.T) {
		m := newApp(stubFetch(t), colorprofile.NoTTY)
		step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{
			Target: hostTarget(t, "alice@plan.cat"),
			Body:   []byte("Plan: hi\n"),
		}})
		m = step.(appModel)
		(&m).updateKeymap()

		view := ansi.Strip(m.helpView())
		for _, unwanted := range []string{"next link", "previous link", "browse links"} {
			if strings.Contains(view, unwanted) {
				t.Fatalf("reader help without links contains %q:\n%s", unwanted, view)
			}
		}
	})

	t.Run("with links includes navigation", func(t *testing.T) {
		m := readerWithFocusedLink(t, stubFetch(t), Link{
			Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com",
		})
		(&m).updateKeymap()

		view := ansi.Strip(m.helpView())
		for _, want := range []string{"next link", "previous link", "browse links"} {
			if !strings.Contains(view, want) {
				t.Fatalf("reader help with links missing %q:\n%s", want, view)
			}
		}
	})

	t.Run("enter follows focused action", func(t *testing.T) {
		target := hostTarget(t, "alice@tilde.team")
		tests := []struct {
			name   string
			link   Link
			wantGo bool
		}{
			{
				name:   "definite",
				link:   Link{Kind: LinkFinger, Action: ActionDrill, Raw: target.Raw, Target: target},
				wantGo: true,
			},
			{
				name: "URL",
				link: Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"},
			},
			{
				name: "blocked",
				link: Link{Kind: LinkFinger, Action: ActionCopy, Raw: "alice@tilde.team@relay.example", Blocked: "cross-relay"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				m := readerWithFocusedLink(t, stubFetch(t), tt.link)
				(&m).updateKeymap()

				view := ansi.Strip(m.helpView())
				got := strings.Contains(view, "↵ go")
				if got != tt.wantGo {
					t.Fatalf("reader help contains ↵ go = %v, want %v:\n%s", got, tt.wantGo, view)
				}
			})
		}
	})
}

func TestLinksPanelHelpContext(t *testing.T) {
	target := hostTarget(t, "alice@tilde.team")
	tests := []struct {
		name   string
		link   Link
		wantGo string
		noGo   bool
	}{
		{
			name: "URL",
			link: Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"},
			noGo: true,
		},
		{
			name:   "ambiguous",
			link:   Link{Kind: LinkFinger, Action: ActionCopy, Raw: target.Raw, Target: target, Ambiguous: true},
			wantGo: "f go",
		},
		{
			name:   "definite",
			link:   Link{Kind: LinkFinger, Action: ActionDrill, Raw: target.Raw, Target: target},
			wantGo: "↵ go",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := linksPanelModel(t, stubFetch(t), []Link{tt.link})
			(&m).updateKeymap()

			view := ansi.Strip(m.helpView())
			for _, want := range []string{"move", "filter", "back", "copy"} {
				if !strings.Contains(view, want) {
					t.Fatalf("panel help missing %q:\n%s", want, view)
				}
			}
			for _, unwanted := range []string{"target", "view source", "page", "top/bottom", "about lookit", "quit"} {
				if strings.Contains(view, unwanted) {
					t.Fatalf("panel help contains non-panel action %q:\n%s", unwanted, view)
				}
			}
			if tt.wantGo != "" && !strings.Contains(view, tt.wantGo) {
				t.Fatalf("panel help missing %q:\n%s", tt.wantGo, view)
			}
			if tt.noGo && strings.Contains(view, "go") {
				t.Fatalf("copy-only panel help advertises go:\n%s", view)
			}
		})
	}
}

func TestLinksPanelHelpQuestionMarkRouting(t *testing.T) {
	link := Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"}

	t.Run("opens help when unfiltered", func(t *testing.T) {
		m := linksPanelModel(t, stubFetch(t), []Link{link})
		step, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
		got := step.(appModel)
		if !got.help {
			t.Fatal("? should open help from an unfiltered links panel")
		}
	})

	t.Run("types while filtering", func(t *testing.T) {
		m := linksPanelModel(t, stubFetch(t), []Link{link})
		step, _ := m.Update(tea.KeyPressMsg{Code: '/'})
		m = step.(appModel)
		step, _ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
		got := step.(appModel)
		if got.help {
			t.Fatal("? must not open help while the links panel filter is active")
		}
		if value := got.linksPanel.filterValue(); value != "?" {
			t.Fatalf("filter value = %q, want ?", value)
		}
	})
}

func TestHelpPanelUsesSharedContrastStyles(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = step.(appModel)

	if !m.helpModel.ShowAll {
		t.Fatal("precondition: help panel should be expanded")
	}
	if !sameColor(m.helpModel.Styles.FullKey.GetForeground(), m.common.styles.palette.AccentViolet) {
		t.Fatal("help key colour should use accent violet")
	}
	if !sameColor(m.helpModel.Styles.FullDesc.GetForeground(), m.common.styles.palette.BarText) {
		t.Fatal("help description colour should use bar text")
	}
	view := m.View().Content
	if !strings.Contains(view, "back") || !strings.Contains(view, "view source") {
		t.Fatalf("help panel should still render enabled keys:\n%s", view)
	}
}

func TestHelpPanelRowsSpanFullWidth(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	m = step.(appModel)
	step, _ = deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = step.(appModel)

	line := lineContaining(t, m.View().Content, "view source")
	assertFullWidthStyledLine(t, "help row", line, m.common.width, m.common.styles.palette.SubtleBg)
}

func TestQuestionMarkOpensHelpWhileInputFocused(t *testing.T) {
	// On the landing the input is focused; '?' (never valid in a finger address)
	// should still open help rather than typing a literal '?'.
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	if !m.inputFocused {
		t.Fatal("precondition: landing should focus the input")
	}
	step, _ := m.Update(tea.KeyPressMsg{Code: '?'})
	got := step.(appModel)
	if !got.help {
		t.Fatal("'?' should open help while the input is focused")
	}
	if got.input.Value() != "" {
		t.Fatalf("'?' must not be typed into the input; value = %q", got.input.Value())
	}
}

func TestEscFromRawViewClearsRawState(t *testing.T) {
	// Esc from raw view returns to the list (clears showingRaw, does not pop history).
	// A second Esc backs to the startpage (pops the history node), a third moves
	// focus to the target input, and only then does Esc quit.
	useTempBookmarks(t)
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@unknown.host")
	opened, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: target, Body: []byte(genericListBody()), Meta: finger.Meta{Addr: target.HostPort}}})
	m = opened.(appModel)

	raw, _ := m.Update(tea.KeyPressMsg{Code: 'v'})
	m = raw.(appModel)
	if !m.showingRaw {
		t.Fatal("precondition: v should enter source view on a generic list")
	}

	// Esc must exit raw view, returning to the list at the same history position.
	back, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = back.(appModel)
	if m.showingRaw {
		t.Fatal("Esc must clear showingRaw")
	}
	if m.state != stateList {
		t.Fatalf("state = %d, want stateList after Esc from raw view", m.state)
	}
	if m.pos != 0 {
		t.Fatalf("pos = %d, want 0 (still at the list node, Esc from raw view does not pop)", m.pos)
	}

	// Second Esc backs to the startpage, content-focused.
	back2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = back2.(appModel)
	if m.pos != -1 || m.state != stateStart {
		t.Fatalf("state=%d pos=%d, want start/-1 after second Esc", m.state, m.pos)
	}

	// Third Esc backs out of the startpage list into the target input.
	back3, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = back3.(appModel)
	if cmd != nil && isQuit(cmd) {
		t.Fatal("Esc from the startpage list must not quit")
	}
	if !m.inputFocused {
		t.Fatal("third Esc should return focus to the input")
	}

	// At the startpage with the input focused, Esc quits.
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil || !isQuit(cmd) {
		t.Fatal("Esc at the startpage input should quit")
	}
}

func TestRestorePreservesListSelection(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = step.(appModel)
	wantIdx := m.list.list.Index()
	step, _ = deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: hostTarget(t, "x@tilde.team"), Body: []byte("Login: x\n")}})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = step.(appModel)
	if m.list.list.Index() != wantIdx {
		t.Fatalf("restored list index = %d, want %d", m.list.list.Index(), wantIdx)
	}
}

func TestRestorePreservesFilteredListSelection(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	body := []byte("Login\nalpha\nbeta\ngamma\n")
	node := histNode{
		entry:    Entry{Target: host, Body: body},
		state:    stateList,
		listIdx:  2,
		listFltr: "a",
	}

	m.restore(node)

	if got := m.list.list.FilterValue(); got != "a" {
		t.Fatalf("restored filter = %q, want a", got)
	}
	if got := m.list.list.Index(); got != 2 {
		t.Fatalf("restored list index = %d, want 2", got)
	}
}

func TestLandingFocusesInput(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	if !m.inputFocused {
		t.Fatal("landing should focus the input")
	}
}

func TestIFocusesInputFromContent(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	if m.inputFocused {
		t.Fatal("after a fetch, content should have focus")
	}
	step, _ = m.Update(tea.KeyPressMsg{Code: 'i'})
	m = step.(appModel)
	if !m.inputFocused {
		t.Fatal("'i' should focus the input")
	}
}

func TestTypingReachesInputOnlyWhenFocused(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY) // landing: input focused
	// textinput inserts from msg.Text, not msg.Code; both must be set for printable keys.
	step, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	step, _ = step.(appModel).Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	m = step.(appModel)
	if m.input.Value() != "bo" {
		t.Fatalf("input value = %q, want \"bo\"", m.input.Value())
	}
}

func TestSubmitFetchesParsedTargetAndBlurs(t *testing.T) {
	fetch, seen := fetchRecorder("Plan: hi\n")
	m := newApp(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@plan.cat")
	step, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = step.(appModel)
	if m.inputFocused {
		t.Fatal("submit should blur the input to content")
	}
	if cmd == nil {
		t.Fatal("submit should return a fetch command")
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != "alice@plan.cat" {
		t.Fatalf("fetched %v, want [alice@plan.cat]", *seen)
	}
}

func TestSubmitFetchesForwardedTarget(t *testing.T) {
	fetch, seen := fetchTargetRecorder("Plan: forwarded\n")
	m := newApp(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@tilde.team@thebackupbox.net")

	step, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = step.(appModel)
	if m.inputFocused {
		t.Fatal("submit should blur the input to content")
	}
	if cmd == nil {
		t.Fatal("submit should return a fetch command")
	}
	runCmds(cmd)

	if len(*seen) != 1 {
		t.Fatalf("fetched %d targets, want 1", len(*seen))
	}
	got := (*seen)[0]
	if got.HostPort != "thebackupbox.net:79" {
		t.Fatalf("HostPort = %q, want thebackupbox.net:79", got.HostPort)
	}
	if got.QueryLine() != "alice@tilde.team" {
		t.Fatalf("QueryLine = %q, want alice@tilde.team", got.QueryLine())
	}
	if got.Raw != "alice@tilde.team@thebackupbox.net" {
		t.Fatalf("Raw = %q, want alice@tilde.team@thebackupbox.net", got.Raw)
	}
}

func TestQQuitsFromContent(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	if cmd == nil || !isQuit(cmd) {
		t.Fatal("'q' should quit from content")
	}
}

func TestQIsLiteralWhenInputFocused(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY) // input focused
	// textinput inserts from msg.Text, not msg.Code; both must be set for printable keys.
	step, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m = step.(appModel)
	if cmd != nil && isQuit(cmd) {
		t.Fatal("'q' must be literal while the input is focused")
	}
	if m.input.Value() != "q" {
		t.Fatalf("input value = %q, want \"q\"", m.input.Value())
	}
}

func TestEscFromInputBlursToContentThenQuitsAtLanding(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY) // landing, input focused, pos -1
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil || !isQuit(cmd) {
		t.Fatal("Esc from the bare landing input should quit")
	}

	// With content present, Esc from the input blurs (does not quit).
	m2 := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	step, _ := deliverNavigationResult(m2, fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m2 = step.(appModel)
	step, _ = m2.Update(tea.KeyPressMsg{Code: 'i'}) // focus input
	m2 = step.(appModel)
	step, cmd2 := m2.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m2 = step.(appModel)
	if cmd2 != nil && isQuit(cmd2) {
		t.Fatal("Esc from input with content present must not quit")
	}
	if m2.inputFocused {
		t.Fatal("Esc from input should blur to content")
	}
}

func TestAltArrowsNoLongerNavigate(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	a := hostTarget(t, "@a.example")
	b := hostTarget(t, "@b.example")
	for _, tg := range []finger.Target{a, b} {
		step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: tg, Body: []byte(hostListBody())}})
		m = step.(appModel)
	}
	// Alt+Left used to go back; now it's inert (content key, delegated, no-op for the list).
	step, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt})
	if step.(appModel).pos != 1 {
		t.Fatalf("pos = %d, want 1 (Alt+Left must not navigate)", step.(appModel).pos)
	}
}

func TestLoadingShowsSpinnerTarget(t *testing.T) {
	// A fetch that we drive manually: set loading via submit, render the bar.
	m := newApp(func(_ context.Context, tg finger.Target) ([]byte, finger.Meta, error) {
		return []byte("Plan\n"), finger.Meta{}, nil
	}, colorprofile.NoTTY)
	m.common.width = 80
	m.input.SetValue("bob@sdf.org")
	step, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = step.(appModel)
	if m.pending == nil {
		t.Fatal("submit should start a request")
	}
	if !strings.Contains(m.statusBarModel().render(), "bob@sdf.org") {
		t.Fatalf("loading bar should name the target:\n%s", m.statusBarModel().render())
	}
}

func TestBackgroundColorMsgRestylesTUI(t *testing.T) {
	useTempBookmarks(t)
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	oldBg := m.common.styles.palette.BaseBg
	assertFullWidthStyledLine(t, "inactive start selection before restyle", lineContaining(t, m.start.View(), "@cosmic.voyage"), m.start.list.Width(), m.common.styles.palette.SubtleBg)

	next, _ := m.Update(tea.BackgroundColorMsg{Color: color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}})
	got := next.(appModel)

	if got.common.darkBackground {
		t.Fatal("darkBackground = true after light background message, want false")
	}
	if sameColor(got.common.styles.palette.BaseBg, oldBg) {
		t.Fatal("palette base background did not change")
	}
	if !sameColor(got.helpModel.Styles.FullKey.GetForeground(), got.common.styles.help.FullKey.GetForeground()) {
		t.Fatal("help styles were not reapplied")
	}
	if !sameColor(got.spin.Style.GetForeground(), got.common.styles.spinner.GetForeground()) {
		t.Fatal("spinner style was not reapplied")
	}
	if !sameColor(got.input.Styles().Focused.Prompt.GetForeground(), got.common.styles.input.Focused.Prompt.GetForeground()) {
		t.Fatal("input styles were not reapplied")
	}
	assertFullWidthStyledLine(t, "inactive start selection after restyle", lineContaining(t, got.start.View(), "@cosmic.voyage"), got.start.list.Width(), got.common.styles.palette.SubtleBg)
}

func TestBackgroundColorMsgRerendersCurrentReader(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	target := hostTarget(t, "alice@plan.cat")
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: target, Body: []byte("Login: alice\n")}})
	m = step.(appModel)
	if !strings.Contains(m.reader.viewport.View(), "\x1b[38;2;255;95;162mLogin:\x1b[0m") {
		t.Fatalf("precondition: reader did not render dark field colour:\n%q", m.reader.viewport.View())
	}

	step, _ = m.Update(tea.BackgroundColorMsg{Color: color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}})
	got := step.(appModel)
	if !strings.Contains(got.reader.viewport.View(), "\x1b[38;2;201;40;112mLogin:\x1b[0m") {
		t.Fatalf("reader did not re-render with light field colour:\n%q", got.reader.viewport.View())
	}
}

func TestBackgroundColorMsgPreservesRawView(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	target := hostTarget(t, "alice@plan.cat")
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: target, Body: []byte("Login: alice\nPlan: raw\n")}})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: 'v'})
	m = step.(appModel)

	step, _ = m.Update(tea.BackgroundColorMsg{Color: color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}})
	got := step.(appModel)
	if !got.showingRaw {
		t.Fatal("background update should not exit raw mode")
	}
	view := got.reader.viewport.View()
	if strings.Contains(view, "\x1b[") || !strings.Contains(view, "Plan: raw") {
		t.Fatalf("background update should preserve raw body, got %q", view)
	}
}

func TestResultClearsLoading(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.reqSeq = 1
	host := hostTarget(t, "@tilde.team")
	m.pending = &pendingRequest{id: 1, target: host, intent: requestNavigate, cancel: func() {}}
	step, _ := m.Update(fetchResultMsg{reqID: 1, entry: Entry{Target: host, Body: []byte(hostListBody())}})
	if step.(appModel).pending != nil {
		t.Fatal("a fetch result should settle the request")
	}
}

func TestHelpExpandsAtBottomNotFullScreen(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = step.(appModel)
	host := hostTarget(t, "@tilde.team")
	step, _ = deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)

	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = step.(appModel)
	view := m.View().Content
	if !strings.Contains(view, "move") || !strings.Contains(view, "page") {
		t.Fatalf("expanded help missing move/page keys:\n%s", view)
	}
	// Not a full-screen takeover: a list user is still visible alongside help.
	if !strings.Contains(view, "alrs") {
		t.Fatalf("help should not blank the content:\n%s", view)
	}
}

func TestListBarShowsPageIndicatorWhenPaged(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 40, 8 // small height forces multiple pages

	// Build a columnar body large enough to require multiple pages.
	// parseColumnar recognises a "Login" header followed by one login per line.
	body := "Login\n"
	for i := range 40 {
		body += fmt.Sprintf("u%02d\n", i)
	}

	step, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 8})
	m = step.(appModel)
	step, _ = deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: hostTarget(t, "@big.host"), Body: []byte(body)}})
	m = step.(appModel)

	if m.state != stateList {
		t.Fatalf("state = %d, want stateList", m.state)
	}
	tp := m.list.list.Paginator.TotalPages
	if tp <= 1 {
		t.Fatalf("TotalPages = %d, want > 1 (test requires multiple pages to be meaningful)", tp)
	}
	if !strings.Contains(m.statusBarModel().render(), "page 1/") {
		t.Fatalf("expected page indicator in bar:\n%s", m.statusBarModel().render())
	}
}

func TestViewSetsNoMouseMode(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	if m.View().MouseMode != tea.MouseModeNone {
		t.Fatalf("MouseMode = %v, want none (native copy preserved)", m.View().MouseMode)
	}
}

func TestYCopiesAddressWithFlash(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel) // list of @tilde.team, content focused

	step, _ = m.Update(tea.KeyPressMsg{Code: 'y'})
	m = step.(appModel)
	if copied != "alrs@tilde.team" {
		t.Fatalf("copied = %q, want alrs@tilde.team", copied)
	}
	if !strings.Contains(m.flash, "alrs@tilde.team") {
		t.Fatalf("flash = %q, want it to mention the copied address", m.flash)
	}
}

// TestLandingParseErrorFlashesInBar verifies that a parse error on Enter at the
// landing (pos == -1) is visible in the status bar. This is Fix 2 from the
// Task 6 review: before the fix, the landing early-return in statusBarModel
// bypassed the flash override, so the error was silently dropped.
func TestLandingParseErrorFlashesInBar(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width = 80
	// An empty input (after TrimSpace) is rejected by finger.ParseTarget with
	// "empty target" — the simplest guaranteed-invalid input.
	m.input.SetValue("")
	step, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = step.(appModel)
	if cmd != nil {
		t.Fatal("submit on invalid input should return nil cmd (no fetch)")
	}
	if m.flash == "" {
		t.Fatal("flash should be set after a parse error at the landing")
	}
	if m.pos != -1 {
		t.Fatalf("pos = %d, want -1 (input stays focused at landing on error)", m.pos)
	}
	bar := m.statusBarModel().render()
	if !strings.Contains(bar, "error") {
		t.Fatalf("status bar = %q, want it to contain the flash error text", bar)
	}
}

// TestSuccessfulSubmitClearsStaleErrorFlash is a regression test for the bug
// where a parse-error flash set by a failed submit would persist and bleed over
// the status bar after a subsequent successful submit.
func TestSuccessfulSubmitClearsStaleErrorFlash(t *testing.T) {
	fetch, _ := fetchRecorder("Plan: hi\n")
	m := newApp(fetch, colorprofile.NoTTY)
	m.common.width = 80

	// Step 1: submit an invalid input so a parse-error flash is set.
	m.input.SetValue("")
	step, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = step.(appModel)
	if cmd != nil {
		t.Fatal("invalid submit should return nil cmd")
	}
	wantErr := m.flash
	if wantErr == "" {
		t.Fatal("precondition: flash should be set after invalid submit")
	}
	bar := m.statusBarModel().render()
	if !strings.Contains(bar, "error") {
		t.Fatalf("precondition: status bar %q should contain error text", bar)
	}

	// Step 2: submit a valid input — flash must be cleared before the fetch lands.
	m.input.SetValue("alice@plan.cat")
	step, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = step.(appModel)
	if cmd == nil {
		t.Fatal("valid submit should return a fetch command")
	}
	if m.flash != "" {
		t.Fatalf("flash = %q after valid submit, want empty (stale error must be cleared)", m.flash)
	}

	// Step 3: deliver the fetch result and confirm the bar shows no error text.
	target := hostTarget(t, "alice@plan.cat")
	result := fetchResultMsg{reqID: m.reqSeq, entry: Entry{Target: target, Body: []byte("Plan: hi\n"), Meta: finger.Meta{Addr: target.HostPort}}}
	step, _ = m.Update(result)
	m = step.(appModel)
	bar = m.statusBarModel().render()
	if strings.Contains(bar, wantErr) {
		t.Fatalf("status bar %q still contains stale error %q after successful fetch", bar, wantErr)
	}
	if strings.Contains(bar, "error:") {
		t.Fatalf("status bar %q must not show error text after a successful fetch", bar)
	}
}

// TestReaderYCopiesAddressWithFlash verifies y-copy from the reader (content)
// path: after a profile fetch the state is reader with pos>=0; pressing y
// copies the target's Raw address and sets a flash message.
func TestReaderYCopiesAddressWithFlash(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "alice@plan.cat")
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: target, Body: []byte("Plan\n")}})
	m = step.(appModel) // reader, content focused, pos==0

	if m.state != stateReader {
		t.Fatalf("state = %d, want stateReader", m.state)
	}
	if m.inputFocused {
		t.Fatal("expected content focus after fetch")
	}

	step, _ = m.Update(tea.KeyPressMsg{Code: 'y'})
	m = step.(appModel)
	if copied != target.Raw {
		t.Fatalf("copied = %q, want %q", copied, target.Raw)
	}
	if !strings.Contains(m.flash, target.Raw) {
		t.Fatalf("flash = %q, want it to mention %q", m.flash, target.Raw)
	}
}

func TestReaderEnterDefiniteFingersFocusedLink(t *testing.T) {
	target := hostTarget(t, "alice@tilde.team")
	fetch, seen := fetchRecorder("Plan: hi\n")
	m := readerWithFocusedLink(t, fetch, Link{
		Kind: LinkFinger, Action: ActionDrill, Raw: target.Raw, Target: target,
	})

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if got.pending == nil {
		t.Fatal("Enter on a definite finger link should start a request")
	}
	if cmd == nil {
		t.Fatal("Enter on a definite finger link should return a fetch command")
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != target.Raw {
		t.Fatalf("fetched targets = %v, want [%s]", *seen, target.Raw)
	}
}

func TestReaderEnterURLDoesNothing(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	link := Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"}
	m := readerWithFocusedLink(t, stubFetch(t), link)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if cmd != nil {
		t.Fatal("Enter on a URL should not return a command")
	}
	if got.pending != nil {
		t.Fatal("Enter on a URL should not start a request")
	}
	if copied != "" {
		t.Fatalf("Enter on a URL copied %q, want no clipboard write", copied)
	}
}

func TestReaderEnterBlockedFlashesRefusal(t *testing.T) {
	link := Link{
		Kind: LinkFinger, Action: ActionCopy, Raw: "alice@tilde.team@relay.example",
		Blocked: "cross-relay: relay relay.example does not match current host",
	}
	m := readerWithFocusedLink(t, stubFetch(t), link)

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if got.flash != link.Blocked {
		t.Fatalf("flash = %q, want %q", got.flash, link.Blocked)
	}
	if got.pending != nil {
		t.Fatal("Enter on a blocked link should not start a request")
	}
}

func TestReaderFAmbiguousFingersFocusedLink(t *testing.T) {
	target := hostTarget(t, "alice@tilde.team")
	fetch, seen := fetchRecorder("Plan: hi\n")
	m := readerWithFocusedLink(t, fetch, Link{
		Kind: LinkFinger, Action: ActionCopy, Raw: target.Raw, Target: target, Ambiguous: true,
	})

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'f'})
	got := next.(appModel)
	if got.pending == nil {
		t.Fatal("f on an ambiguous finger link should start a request")
	}
	if cmd == nil {
		t.Fatal("f on an ambiguous finger link should return a fetch command")
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != target.Raw {
		t.Fatalf("fetched targets = %v, want [%s]", *seen, target.Raw)
	}
}

func TestReaderFDefiniteDoesNothing(t *testing.T) {
	target := hostTarget(t, "alice@tilde.team")
	m := readerWithFocusedLink(t, stubFetch(t), Link{
		Kind: LinkFinger, Action: ActionDrill, Raw: target.Raw, Target: target,
	})

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'f'})
	got := next.(appModel)
	if cmd != nil {
		t.Fatal("f on a definite finger link should not return a command")
	}
	if got.pending != nil {
		t.Fatal("f on a definite finger link should not start a request")
	}
}

func TestReaderYCopiesFocusedLink(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	link := Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"}
	m := readerWithFocusedLink(t, stubFetch(t), link)

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	got := next.(appModel)
	runCmds(cmd)
	if copied != link.Raw {
		t.Fatalf("copied = %q, want focused link %q", copied, link.Raw)
	}
	if !strings.Contains(got.flash, link.Raw) {
		t.Fatalf("flash = %q, want it to mention %q", got.flash, link.Raw)
	}
}

func TestLinksPanelEnterDefiniteFingersSelectedLink(t *testing.T) {
	target := hostTarget(t, "alice@tilde.team")
	fetch, seen := fetchRecorder("Plan: hi\n")
	m := linksPanelModel(t, fetch, []Link{{
		Kind: LinkFinger, Action: ActionDrill, Raw: target.Raw, Target: target,
	}})

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if got.showingLinks {
		t.Fatal("Enter on a definite finger link should close the links panel")
	}
	if got.pending == nil || cmd == nil {
		t.Fatalf("after Enter: pending=%#v cmd=nil=%v, want a fetch", got.pending, cmd == nil)
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != target.Raw {
		t.Fatalf("fetched targets = %v, want [%s]", *seen, target.Raw)
	}
}

func TestLinksPanelEnterURLStaysOpen(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	link := Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"}
	m := linksPanelModel(t, stubFetch(t), []Link{link})

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if !got.showingLinks {
		t.Fatal("Enter on a URL should leave the links panel open")
	}
	if cmd != nil || got.pending != nil {
		t.Fatalf("after Enter: cmd=nil=%v pending=%#v, want no action", cmd == nil, got.pending)
	}
	if copied != "" {
		t.Fatalf("Enter on a URL copied %q, want no clipboard write", copied)
	}
}

func TestLinksPanelEnterBlockedStaysOpenAndFlashesRefusal(t *testing.T) {
	link := Link{
		Kind: LinkFinger, Action: ActionCopy, Raw: "alice@tilde.team@relay.example",
		Blocked: "cross-relay: relay relay.example does not match current host",
	}
	m := linksPanelModel(t, stubFetch(t), []Link{link})

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if !got.showingLinks {
		t.Fatal("Enter on a blocked link should leave the links panel open")
	}
	if got.flash != link.Blocked {
		t.Fatalf("flash = %q, want %q", got.flash, link.Blocked)
	}
	if got.pending != nil {
		t.Fatal("Enter on a blocked link should not start a request")
	}
}

func TestLinksPanelFAmbiguousFingersSelectedLink(t *testing.T) {
	target := hostTarget(t, "alice@tilde.team")
	fetch, seen := fetchRecorder("Plan: hi\n")
	m := linksPanelModel(t, fetch, []Link{{
		Kind: LinkFinger, Action: ActionCopy, Raw: target.Raw, Target: target, Ambiguous: true,
	}})

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'f'})
	got := next.(appModel)
	if got.showingLinks {
		t.Fatal("f on an ambiguous finger link should close the links panel")
	}
	if got.pending == nil || cmd == nil {
		t.Fatalf("after f: pending=%#v cmd=nil=%v, want a fetch", got.pending, cmd == nil)
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != target.Raw {
		t.Fatalf("fetched targets = %v, want [%s]", *seen, target.Raw)
	}
}

func TestLinksPanelYCopiesSelectedRawAndStaysOpen(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	link := Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"}
	m := linksPanelModel(t, stubFetch(t), []Link{link})

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	got := next.(appModel)
	if !got.showingLinks {
		t.Fatal("y should leave the links panel open")
	}
	if cmd == nil {
		t.Fatal("y should return the clipboard/flash command")
	}
	if copied != link.Raw {
		t.Fatalf("copied = %q, want selected Raw %q", copied, link.Raw)
	}
	if !strings.Contains(got.flash, link.Raw) {
		t.Fatalf("flash = %q, want it to mention %q", got.flash, link.Raw)
	}
}

func TestLinksPanelFilteringConsumesActionKeys(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	target := hostTarget(t, "yfL@tilde.team")
	for _, code := range []rune{'y', 'f', 'L'} {
		t.Run(string(code), func(t *testing.T) {
			fetch, seen := fetchRecorder("Plan: hi\n")
			m := linksPanelModel(t, fetch, []Link{{
				Kind: LinkFinger, Action: ActionCopy, Raw: target.Raw, Target: target, Ambiguous: true,
			}})

			next, _ := m.Update(tea.KeyPressMsg{Code: '/'})
			m = next.(appModel)
			if m.linksPanel.list.FilterState() != list.Filtering {
				t.Fatal("/ should start filtering the links panel")
			}

			next, _ = m.Update(tea.KeyPressMsg{Code: code, Text: string(code)})
			got := next.(appModel)
			if !got.showingLinks {
				t.Fatalf("%q while filtering should not close the links panel", code)
			}
			if value := got.linksPanel.list.FilterValue(); value != string(code) {
				t.Fatalf("FilterValue = %q, want %q", value, string(code))
			}
			if got.pending != nil || len(*seen) != 0 {
				t.Fatalf("%q while filtering triggered a fetch: pending=%#v targets=%v", code, got.pending, *seen)
			}
			if copied != "" {
				t.Fatalf("%q while filtering copied %q, want no clipboard write", code, copied)
			}
		})
	}
}

func TestLinksPanelAcceptsNonEmptyMatchingFilter(t *testing.T) {
	for _, keyMsg := range []tea.KeyPressMsg{
		{Code: tea.KeyEnter},
		{Code: tea.KeyTab},
	} {
		t.Run(keyMsg.String(), func(t *testing.T) {
			link := Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://yellow.example"}
			m := linksPanelModel(t, stubFetch(t), []Link{link})

			for _, msg := range []tea.KeyPressMsg{{Code: '/'}, {Code: 'y', Text: "y"}, keyMsg} {
				next, _ := m.Update(msg)
				m = next.(appModel)
			}
			if state := m.linksPanel.list.FilterState(); state != list.FilterApplied {
				t.Fatalf("FilterState = %v, want FilterApplied", state)
			}
			if value := m.linksPanel.list.FilterValue(); value != "y" {
				t.Fatalf("FilterValue = %q, want y", value)
			}
			if !m.showingLinks {
				t.Fatal("accepting a filter should leave the links panel open")
			}
		})
	}
}

func TestLinksPanelEscWhileFilteringCancelsWithoutClosing(t *testing.T) {
	link := Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://yellow.example"}
	m := linksPanelModel(t, stubFetch(t), []Link{link})
	for _, msg := range []tea.KeyPressMsg{{Code: '/'}, {Code: 'y', Text: "y"}} {
		next, _ := m.Update(msg)
		m = next.(appModel)
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)
	if !got.showingLinks {
		t.Fatal("Esc while filtering should not close the links panel")
	}
	if state := got.linksPanel.list.FilterState(); state != list.Unfiltered {
		t.Fatalf("FilterState = %v, want Unfiltered", state)
	}
}

func TestLinksPanelEscClearsAppliedFilterBeforeClosing(t *testing.T) {
	link := Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://yellow.example"}
	m := linksPanelModel(t, stubFetch(t), []Link{link})
	for _, msg := range []tea.KeyPressMsg{{Code: '/'}, {Code: 'y', Text: "y"}, {Code: tea.KeyTab}} {
		next, _ := m.Update(msg)
		m = next.(appModel)
	}
	if state := m.linksPanel.list.FilterState(); state != list.FilterApplied {
		t.Fatalf("precondition: FilterState = %v, want FilterApplied", state)
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(appModel)
	if !m.showingLinks {
		t.Fatal("first Esc should clear the applied filter without closing the links panel")
	}
	if state := m.linksPanel.list.FilterState(); state != list.Unfiltered {
		t.Fatalf("after first Esc: FilterState = %v, want Unfiltered", state)
	}

	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if next.(appModel).showingLinks {
		t.Fatal("second Esc should close the links panel")
	}
}

func TestLinksPanelCtrlCQuits(t *testing.T) {
	link := Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"}
	m := linksPanelModel(t, stubFetch(t), []Link{link})

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !isQuit(cmd) {
		t.Fatal("Ctrl+C should quit while the links panel is open")
	}
}

func TestLinksPanelBindingsFollowSelectedAction(t *testing.T) {
	target := hostTarget(t, "alice@tilde.team")
	tests := []struct {
		name           string
		link           Link
		wantOpen       bool
		wantLinkFinger bool
	}{
		{
			name: "definite finger", link: Link{Kind: LinkFinger, Action: ActionDrill, Raw: target.Raw, Target: target},
			wantOpen: true,
		},
		{
			name: "ambiguous finger", link: Link{Kind: LinkFinger, Action: ActionCopy, Raw: target.Raw, Target: target, Ambiguous: true},
			wantLinkFinger: true,
		},
		{
			name: "URL", link: Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"},
		},
		{
			name: "blocked finger", link: Link{Kind: LinkFinger, Action: ActionCopy, Raw: "alice@tilde.team@relay.example", Blocked: "cross-relay"},
			wantOpen: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := linksPanelModel(t, stubFetch(t), []Link{tt.link})
			(&m).updateKeymap()
			if m.keys.Open.Enabled() != tt.wantOpen {
				t.Errorf("Open enabled = %v, want %v", m.keys.Open.Enabled(), tt.wantOpen)
			}
			if m.keys.LinkFinger.Enabled() != tt.wantLinkFinger {
				t.Errorf("LinkFinger enabled = %v, want %v", m.keys.LinkFinger.Enabled(), tt.wantLinkFinger)
			}
			if !m.keys.Back.Enabled() || !m.keys.LinkPanel.Enabled() || !m.keys.Filter.Enabled() || !m.keys.Copy.Enabled() {
				t.Error("links panel should enable back, panel, filter, and copy")
			}

			next, _ := m.Update(tea.KeyPressMsg{Code: '/'})
			m = next.(appModel)
			(&m).updateKeymap()
			if m.keys.Filter.Enabled() {
				t.Error("Filter should be disabled while the links panel filter is active")
			}
		})
	}
}

// TestClearFlashMsgClearsFlash verifies that receiving a clearFlashMsg zeroes
// m.flash.
func TestClearFlashMsgClearsFlash(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.flash = "copied alice@plan.cat"
	step, _ := m.Update(clearFlashMsg{})
	if got := step.(appModel).flash; got != "" {
		t.Fatalf("flash = %q after clearFlashMsg, want empty", got)
	}
}

// --- Task 8: state-driven binding enablement (updateKeymap) ---

// TestUpdateKeymapGatesByState: enablement mirrors what handleKey actually does
// in each state, so the '?' help panel advertises only live keys.
func TestUpdateKeymapGatesByState(t *testing.T) {
	// Landing: input focused, no result. Content-only keys (i/y/r/q) disable so
	// they type literally; the dual-mode commands (Enter/Esc/?) stay live.
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	(&m).updateKeymap()
	if m.keys.FocusInput.Enabled() || m.keys.Copy.Enabled() || m.keys.Raw.Enabled() || m.keys.Quit.Enabled() {
		t.Fatal("content-only keys (focus/copy/raw/quit) should be disabled while the input is focused")
	}
	if !m.keys.Open.Enabled() || !m.keys.Back.Enabled() || !m.keys.Help.Enabled() {
		t.Fatal("dual-mode commands (Enter/Esc/?) must stay enabled while the input is focused")
	}

	// Host list lands → content focused, list state: open/filter/back/copy/focus live.
	host := hostTarget(t, "@tilde.team")
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	(&m).updateKeymap()
	if !m.keys.Open.Enabled() || !m.keys.Filter.Enabled() || !m.keys.Back.Enabled() ||
		!m.keys.Copy.Enabled() || !m.keys.FocusInput.Enabled() {
		t.Fatal("list content keys (open/filter/back/copy/focus) should be enabled")
	}

	// Profile reader → no link actions or panel (nothing to drill, finger, or
	// browse); raw/copy/back live.
	step, _ = deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
	m = step.(appModel)
	(&m).updateKeymap()
	if m.keys.Open.Enabled() || m.keys.Filter.Enabled() || m.keys.LinkNext.Enabled() ||
		m.keys.LinkPrev.Enabled() || m.keys.LinkFinger.Enabled() || m.keys.LinkPanel.Enabled() {
		t.Fatal("link and filter actions should be disabled in a profile reader without links")
	}
	if !m.keys.Raw.Enabled() || !m.keys.Copy.Enabled() || !m.keys.Back.Enabled() {
		t.Fatal("raw/copy/back should be enabled in a content reader with a result")
	}

	definiteTarget := hostTarget(t, "alice@tilde.team")
	tests := []struct {
		name           string
		link           Link
		wantOpen       bool
		wantLinkFinger bool
	}{
		{
			name: "definite finger", link: Link{Kind: LinkFinger, Action: ActionDrill, Raw: definiteTarget.Raw, Target: definiteTarget},
			wantOpen: true,
		},
		{
			name: "ambiguous finger", link: Link{Kind: LinkFinger, Action: ActionCopy, Raw: definiteTarget.Raw, Target: definiteTarget, Ambiguous: true},
			wantLinkFinger: true,
		},
		{
			name: "URL", link: Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"},
		},
		{
			name: "blocked finger", link: Link{Kind: LinkFinger, Action: ActionCopy, Raw: "alice@tilde.team@relay.example", Blocked: "cross-relay"},
			wantOpen: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := readerWithFocusedLink(t, stubFetch(t), tt.link)
			(&reader).updateKeymap()
			if reader.keys.Open.Enabled() != tt.wantOpen {
				t.Errorf("Open enabled = %v, want %v", reader.keys.Open.Enabled(), tt.wantOpen)
			}
			if reader.keys.LinkFinger.Enabled() != tt.wantLinkFinger {
				t.Errorf("LinkFinger enabled = %v, want %v", reader.keys.LinkFinger.Enabled(), tt.wantLinkFinger)
			}
			if !reader.keys.LinkNext.Enabled() || !reader.keys.LinkPrev.Enabled() || !reader.keys.LinkPanel.Enabled() {
				t.Error("reader navigation and panel should be enabled when the current node has links")
			}
		})
	}
}

// TestHelpPanelHidesInertKeys: the expanded '?' panel omits keys that do nothing
// in the current state (bubbles/help skips disabled bindings).
func TestHelpPanelHidesInertKeys(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
	m = step.(appModel)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = step.(appModel)
	view := m.View().Content
	if strings.Contains(view, "open") || strings.Contains(view, "filter") {
		t.Fatalf("profile-reader help must not advertise open/filter:\n%s", view)
	}
	if !strings.Contains(view, "back") || !strings.Contains(view, "view source") {
		t.Fatalf("help should still show the live keys (back/view source):\n%s", view)
	}
}

// TestInputFocusedBarShowsGoCancel: focusing the input over existing content
// shows a target-entry hint (↵ go · esc cancel), not the stale content hints.
func TestInputFocusedBarShowsGoCancel(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width = 80
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: 'i'})
	m = step.(appModel)
	if !m.inputFocused {
		t.Fatal("'i' should focus the input")
	}
	bar := m.statusBarModel().render()
	if !strings.Contains(bar, "go") || !strings.Contains(bar, "cancel") {
		t.Fatalf("input-focused bar should show go/cancel:\n%s", bar)
	}
}

// TestJoinHintsDropsEscBackWhenBreadcrumbPresent: when the "◂ esc: <target>"
// breadcrumb is shown, the redundant "esc back" hint is omitted; "? help" stays.
func TestJoinHintsDropsEscBackWhenBreadcrumbPresent(t *testing.T) {
	withCrumb := joinHints([]string{"↑↓ scroll"}, "@tilde.team")
	if strings.Contains(withCrumb, "esc back") {
		t.Fatalf("esc back should be omitted when the ◂ esc: breadcrumb is present: %q", withCrumb)
	}
	if !strings.Contains(withCrumb, "? help") {
		t.Fatalf("? help should always be present: %q", withCrumb)
	}
	if noCrumb := joinHints([]string{"↑↓ scroll"}, ""); !strings.Contains(noCrumb, "esc back") {
		t.Fatalf("esc back should be present when there is no breadcrumb: %q", noCrumb)
	}
}

func TestStaleFetchResultDropped(t *testing.T) {
	common := testCommon()
	m := appModel{common: common}
	m.reqSeq = 2
	m.pending = &pendingRequest{id: 2, target: finger.Target{Raw: "b@x"}, intent: requestNavigate, cancel: func() {}}

	stale := fetchResultMsg{reqID: 1, entry: Entry{Target: finger.Target{Raw: "a@x"}, Body: []byte("old\n")}}
	updated, _ := m.Update(stale)
	got := updated.(appModel)
	if got.pending == nil || got.pending.id != 2 {
		t.Fatal("stale result cleared or replaced the in-flight request")
	}

	current := fetchResultMsg{reqID: 2, entry: Entry{Target: finger.Target{Raw: "b@x"}, Body: []byte("new\n")}}
	updated2, _ := got.Update(current)
	got2 := updated2.(appModel)
	if got2.pending != nil {
		t.Fatal("current result did not settle the request")
	}
	if got2.state != stateReader {
		t.Fatalf("current result did not route to reader: state = %d", got2.state)
	}
	if got2.pos < 0 {
		t.Fatal("current result did not push history: pos < 0")
	}
}

// TestTargetPlaceholderSuggestsNoDestination pins the division of labour the
// placeholder was rewritten for: the input teaches the two target shapes, and
// the startpage (catalog + bookmarks, rendered directly below it) is the only
// thing that names somewhere to go. A placeholder that drifted back into
// naming a host would duplicate a row sitting inches beneath it.
func TestTargetPlaceholderSuggestsNoDestination(t *testing.T) {
	for _, entry := range loadCatalog() {
		if targetPlaceholder == entry.target {
			t.Fatalf("targetPlaceholder = %q, which is a catalog destination; the input hints syntax, the startpage suggests places", targetPlaceholder)
		}
	}
	// The hint has to survive the input it lives in: newApp sets the width to
	// 40 columns, less the "target: " prompt.
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	if avail := m.input.Width() - lipgloss.Width(m.input.Prompt); lipgloss.Width(targetPlaceholder) > avail {
		t.Fatalf("targetPlaceholder is %d cols, wider than the %d available in the input", lipgloss.Width(targetPlaceholder), avail)
	}
	if m.input.Placeholder != targetPlaceholder {
		t.Fatalf("input placeholder = %q, want %q", m.input.Placeholder, targetPlaceholder)
	}
}

// TestCopyAddressPinsServerTarget verifies that copying (y) a list item whose
// target was supplied by the server is pinned to port 79 before being placed on
// the clipboard, mirroring the protection applied in the drill path.
func TestCopyAddressPinsServerTarget(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@thebackupbox.net")
	// A server-supplied entry pointing at a non-finger port.
	users := []User{{Login: "evil", Target: "finger://example.com:22/evil"}}
	m.history = []histNode{{entry: Entry{Target: host}, state: stateList}}
	m.pos = 0
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.list.list.Select(0)
	m.state = stateList
	m.inputFocused = false

	step, _ := m.Update(tea.KeyPressMsg{Code: 'y'})
	m = step.(appModel)

	if strings.Contains(m.flash, ":22") {
		t.Fatalf("flash = %q, must not contain the hostile port :22", m.flash)
	}
	if !strings.Contains(m.flash, ":79") {
		t.Fatalf("flash = %q, want it to contain the pinned port :79", m.flash)
	}
	if strings.Contains(copied, ":22") {
		t.Fatalf("copied = %q, must not contain the hostile port :22", copied)
	}
	if !strings.Contains(copied, ":79") {
		t.Fatalf("copied = %q, want it to contain the pinned port :79", copied)
	}
}

func TestCopyServerSuppliedForwardedTargetFlashesRefusal(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@thebackupbox.net")
	users := []User{{Login: "alice", Target: "alice@tilde.team@thebackupbox.net"}}
	m.history = []histNode{{entry: Entry{Target: host}, state: stateList}}
	m.pos = 0
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.list.list.Select(0)
	m.state = stateList
	m.inputFocused = false

	step, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	got := step.(appModel)

	if cmd == nil {
		t.Fatal("copy refusal should return a clear-flash command")
	}
	if copied != "" {
		t.Fatalf("copied = %q, want empty", copied)
	}
	if got.flash != finger.ErrServerForwarding.Error() {
		t.Fatalf("flash = %q, want %q", got.flash, finger.ErrServerForwarding.Error())
	}
}

func TestLaunchShowsBareTargetRowWithoutWordmark(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	view := stripANSIForLandingTest(m.View().Content)
	if strings.Contains(view, heroManicule+" "+heroWordmark) {
		t.Fatalf("landing should no longer show the wordmark (it moved to about):\n%s", view)
	}
	if !strings.Contains(view, "target:") {
		t.Fatalf("landing missing target row:\n%s", view)
	}
}

func TestFocusedInputChromeHasNoWordmark(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	m.input.SetValue("alice@plan.cat")
	(&m).submit()
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Login: alice\n"),
	}})
	m = step.(appModel)
	(&m).focusInput()
	view := stripANSIForLandingTest(m.View().Content)
	if strings.Contains(view, heroManicule+" "+heroWordmark) {
		t.Fatalf("re-focused input chrome should not show the wordmark:\n%s", view)
	}
	if !strings.Contains(view, "target:") {
		t.Fatalf("focused input chrome missing target row:\n%s", view)
	}
}

func TestFocusedInputHeaderKeepsTotalViewHeightStable(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m = sized.(appModel)

	m.input.SetValue("alice@plan.cat")
	(&m).submit()
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte(strings.Repeat("line\n", 20)),
	}})
	m = step.(appModel)

	step, _ = m.Update(tea.KeyPressMsg{Code: 'i'})
	m = step.(appModel)
	if got := lipgloss.Height(m.View().Content); got != m.common.height {
		t.Fatalf("view height = %d, want terminal height %d:\n%s", got, m.common.height, m.View().Content)
	}

	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = step.(appModel)
	if got := lipgloss.Height(m.View().Content); got != m.common.height {
		t.Fatalf("view height after Esc = %d, want terminal height %d:\n%s", got, m.common.height, m.View().Content)
	}
}

func TestBlurredResultChromeDoesNotSpendHeaderRow(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)

	m.input.SetValue("alice@plan.cat")
	(&m).submit()
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("Login: alice\n"),
	}})
	m = step.(appModel)

	view := stripANSIForLandingTest(m.View().Content)
	if strings.Contains(view, heroManicule+" "+heroWordmark) {
		t.Fatalf("blurred result view should not spend a row on the header mark:\n%s", view)
	}
}

func TestBackToLandingShowsBareTargetRow(t *testing.T) {
	useTempBookmarks(t)
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	m.input.SetValue("alice@plan.cat")
	(&m).submit()
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Login: alice\n"),
	}})
	m = step.(appModel)
	(&m).back()
	if m.pos != -1 || m.state != stateStart {
		t.Fatalf("back-to-start state=%d pos=%d, want start/-1", m.state, m.pos)
	}
	if m.inputFocused {
		t.Fatal("back-to-start should land content-focused, not in the input")
	}
	view := stripANSIForLandingTest(m.View().Content)
	if strings.Contains(view, heroManicule+" "+heroWordmark) {
		t.Fatalf("back-to-start should not show the wordmark:\n%s", view)
	}
	for _, want := range []string{"target:", "@plan.cat", "b bookmark"} {
		if !strings.Contains(view, want) {
			t.Fatalf("back-to-start view missing %q:\n%s", want, view)
		}
	}
}

func TestAboutOpensFromBlurredResult(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n"),
	}})
	m = step.(appModel)
	if m.inputFocused {
		t.Fatal("a landed result should be blurred")
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'a'})
	got := next.(appModel)
	if got.state != stateAbout {
		t.Fatalf("state = %d, want stateAbout", got.state)
	}
	if !strings.Contains(stripANSIForLandingTest(got.View().Content), "finger jonathan@tilde.team") {
		t.Fatalf("about view missing the author finger line:\n%s", got.View().Content)
	}
}

func TestAboutOpensFromHelpPanelOnLanding(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	step, _ := m.Update(tea.KeyPressMsg{Code: '?'}) // help opens even while focused
	m = step.(appModel)
	if !m.help {
		t.Fatal("'?' should open the help panel on the landing")
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'a'}) // 'a' from the open panel opens about
	got := next.(appModel)
	if got.help {
		t.Fatal("opening about should close the help panel")
	}
	if got.state != stateAbout {
		t.Fatalf("state = %d, want stateAbout", got.state)
	}
}

func TestLandingTypesAInsteadOfOpeningAbout(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	// textinput inserts from msg.Text, not msg.Code; both must be set for
	// printable keys (see TestQIsLiteralWhenInputFocused).
	next, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"}) // focused landing, help closed
	got := next.(appModel)
	if got.state == stateAbout {
		t.Fatal("'a' on the focused landing must type into the target, not open about")
	}
	if !strings.Contains(got.input.Value(), "a") {
		t.Fatalf("'a' should be typed into the target input, value = %q", got.input.Value())
	}
}

func TestAboutEscReturnsToOrigin(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n"),
	}})
	m = step.(appModel)
	opened, _ := m.Update(tea.KeyPressMsg{Code: 'a'})
	m = opened.(appModel)
	if m.state != stateAbout {
		t.Fatalf("precondition: state = %d, want stateAbout", m.state)
	}
	closed, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := closed.(appModel)
	if got.state != stateReader {
		t.Fatalf("esc from about: state = %d, want stateReader (origin)", got.state)
	}
	if got.pos != 0 || len(got.history) != 1 {
		t.Fatalf("esc from about must not change history: pos=%d len=%d", got.pos, len(got.history))
	}
}

func TestCopyAddressNothingToCopy(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY) // startpage: pos == -1, stateStart, no address
	cmd := (&m).copyAddress()
	if m.flash != "nothing to copy" {
		t.Fatalf("flash = %q, want %q", m.flash, "nothing to copy")
	}
	if cmd == nil {
		t.Fatal("copyAddress returned nil cmd; want a clear-flash command")
	}
}

func TestCopyAddressSuccessSetsCopiedFlash(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("Plan: hi\n"),
	}})
	m = step.(appModel)

	_ = (&m).copyAddress()
	if want := "copied alice@plan.cat"; m.flash != want {
		t.Fatalf("flash = %q, want %q", m.flash, want)
	}
}

func TestBackClearsFlash(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("Plan: hi\n"),
	}})
	m = step.(appModel) // pos == 0 now, so back() steps back rather than quitting

	m.flash = "copied alice@plan.cat"
	(&m).back()
	if m.flash != "" {
		t.Fatalf("flash = %q after back, want empty", m.flash)
	}
}

func TestDrillClearsFlash(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.state = stateList
	m.list = newList(m.common, hostTarget(t, "@tilde.team"), []User{{Login: "alrs"}})
	m.listReady = true

	m.flash = "copied alrs@tilde.team"
	_, got, _ := m.drill() // value receiver: the clear lands on the returned model
	if got.flash != "" {
		t.Fatalf("flash = %q after drill, want empty", got.flash)
	}
}

func TestFocusInputPreservesErrorFlash(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"),
		Body:   []byte("Plan: hi\n"),
	}})
	m = step.(appModel)

	m.flash = "error: bad target"
	(&m).focusInput() // on the parse-error recovery path: must NOT clear the flash
	if m.flash != "error: bad target" {
		t.Fatalf("flash = %q after focusInput, want it preserved", m.flash)
	}
}

// collectMsgs runs a command (recursing into batches) and returns every
// non-batch message produced. Safe for Init's commands: textinput.Blink and the
// capability requests all return their message immediately (no timers).
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	var out []tea.Msg
	if cmd == nil {
		return out
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			out = append(out, collectMsgs(c)...)
		}
		return out
	}
	if msg != nil {
		out = append(out, msg)
	}
	return out
}

func hasSeedSubmit(msgs []tea.Msg) bool {
	for _, msg := range msgs {
		if _, ok := msg.(seedSubmitMsg); ok {
			return true
		}
	}
	return false
}

func TestSeededInitEmitsSeedSubmit(t *testing.T) {
	m := newAppWithOptions(stubFetch(t), colorprofile.NoTTY, Options{InitialQuery: "alice@plan.cat", Seed: true})
	if !hasSeedSubmit(collectMsgs(m.Init())) {
		t.Fatal("Init() did not emit seedSubmitMsg when a query was seeded")
	}
}

func TestBlankSeedStillEmitsSeedSubmit(t *testing.T) {
	// lookit "" / lookit "   ": an arg was supplied, so it must still be replayed.
	m := newAppWithOptions(stubFetch(t), colorprofile.NoTTY, Options{InitialQuery: "   ", Seed: true})
	if !hasSeedSubmit(collectMsgs(m.Init())) {
		t.Fatal("Init() did not emit seedSubmitMsg for a supplied-but-blank arg")
	}
}

func TestUnseededInitOmitsSeedSubmit(t *testing.T) {
	m := newAppWithOptions(stubFetch(t), colorprofile.NoTTY, Options{})
	if hasSeedSubmit(collectMsgs(m.Init())) {
		t.Fatal("Init() emitted seedSubmitMsg without a seed")
	}
}

func TestSeededValidQueryFetchesAndRoutesToReader(t *testing.T) {
	fetch, seen := fetchRecorder("Plan: hi\n")
	m := newAppWithOptions(fetch, colorprofile.NoTTY, Options{InitialQuery: "alice@plan.cat", Seed: true})

	next, cmd := m.Update(seedSubmitMsg{})
	got := next.(appModel)
	if got.pending == nil {
		t.Fatalf("after seed submit: pending=nil, want request")
	}
	if cmd == nil {
		t.Fatal("seed submit cmd = nil, want a fetch command")
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != "alice@plan.cat" {
		t.Fatalf("fetched targets = %v, want [alice@plan.cat]", *seen)
	}

	landed, _ := got.Update(fetchResultMsg{reqID: got.reqSeq, entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
	if landed.(appModel).state != stateReader {
		t.Fatalf("state = %d, want stateReader", landed.(appModel).state)
	}
}

func TestSeededInvalidQueryShowsErrorOnLanding(t *testing.T) {
	m := newAppWithOptions(stubFetch(t), colorprofile.NoTTY, Options{InitialQuery: "just-a-name", Seed: true})

	next, cmd := m.Update(seedSubmitMsg{})
	got := next.(appModel)

	if got.pending != nil {
		t.Fatalf("invalid seed: pending=%#v, want nil", got.pending)
	}
	if cmd != nil {
		t.Fatalf("invalid seed: cmd != nil, want nil (no fetch)")
	}
	if !got.inputFocused {
		t.Fatalf("invalid seed: inputFocused=false, want true")
	}
	if !strings.Contains(got.flash, "error") {
		t.Fatalf("invalid seed: flash=%q, want it to contain \"error\"", got.flash)
	}
	if got.input.Value() != "just-a-name" {
		t.Fatalf("invalid seed: input=%q, want it to retain \"just-a-name\"", got.input.Value())
	}
}

func TestAboutEnterFingersAuthor(t *testing.T) {
	fetch, seen := fetchRecorder("Plan: hi\n")
	m := newApp(fetch, colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	(&m).openAbout()
	if m.state != stateAbout {
		t.Fatalf("precondition: state = %d, want stateAbout", m.state)
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if got.pending == nil {
		t.Fatal("Enter on about should start a request")
	}
	if cmd == nil {
		t.Fatal("Enter on about should return a fetch command")
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != "jonathan@tilde.team" {
		t.Fatalf("fetched targets = %v, want [jonathan@tilde.team]", *seen)
	}
}

func TestAboutCopiesIssuesURL(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	(&m).openAbout()
	next, _ := m.Update(tea.KeyPressMsg{Code: 'y'})
	got := next.(appModel)
	if copied != aboutIssuesURL {
		t.Fatalf("copied = %q, want %q", copied, aboutIssuesURL)
	}
	if !strings.Contains(got.flash, "copied") {
		t.Fatalf("flash = %q, want it to mention the copied URL", got.flash)
	}
	if got.state != stateAbout {
		t.Fatalf("copy should keep the about screen open, state = %d", got.state)
	}
}

func TestAboutTabDoesNothing(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	(&m).openAbout()
	wantView := m.View().Content

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	got := next.(appModel)
	if cmd != nil {
		t.Fatal("Tab on about should not return a command")
	}
	if got.state != stateAbout {
		t.Fatalf("Tab on about: state = %d, want stateAbout", got.state)
	}
	if got.View().Content != wantView {
		t.Fatalf("Tab on about should leave the view unchanged:\nwant:\n%s\n\ngot:\n%s", wantView, got.View().Content)
	}
}

func TestAboutStatusBarFromLandingAndResult(t *testing.T) {
	// Opened from the bare landing (pos<0): left label "about lookit", and the
	// hints advertise all four about keys including "esc back".
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	(&m).openAbout()
	bar := m.statusBarModel()
	if bar.host != "about lookit" {
		t.Fatalf("landing-origin about bar host = %q, want \"about lookit\"", bar.host)
	}
	for _, want := range []string{"↵ go to author", "y copy issues URL", "esc back", "q quit"} {
		if !strings.Contains(bar.hints, want) {
			t.Fatalf("landing-origin about hints = %q, missing %q", bar.hints, want)
		}
	}

	// Opened from a result (pos>=0): the breadcrumb shows where esc returns, so
	// "esc back" is omitted from the hints (the escTarget convention).
	m2 := newApp(stubFetch(t), colorprofile.NoTTY)
	sized2, _ := m2.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 = sized2.(appModel)
	step, _ := deliverNavigationResult(m2, fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
	m2 = step.(appModel)
	(&m2).openAbout()
	bar2 := m2.statusBarModel()
	if bar2.escTarget != "alice@plan.cat" {
		t.Fatalf("result-origin about bar escTarget = %q, want \"alice@plan.cat\"", bar2.escTarget)
	}
	if strings.Contains(bar2.hints, "esc back") {
		t.Fatalf("result-origin about hints should omit \"esc back\" (breadcrumb shows it): %q", bar2.hints)
	}
	for _, want := range []string{"↵ go to author", "y copy issues URL", "q quit"} {
		if !strings.Contains(bar2.hints, want) {
			t.Fatalf("result-origin about hints = %q, missing %q", bar2.hints, want)
		}
	}
}

func TestAppOpensOnStartpage(t *testing.T) {
	useTempBookmarks(t)

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	if m.state != stateStart {
		t.Fatalf("state = %v, want stateStart", m.state)
	}
	if m.pos != -1 {
		t.Fatalf("pos = %d, want -1", m.pos)
	}
	if _, ok := m.start.selected(); !ok {
		t.Fatal("startpage has no selection; the catalog should populate it")
	}
}

func TestStartEnterRequestsSelectedTarget(t *testing.T) {
	useTempBookmarks(t)

	fetch, seen := fetchRecorder("Plan: hello\n")
	m := newApp(fetch, colorprofile.NoTTY)
	m.blurInput()
	selected, ok := m.start.selected()
	if !ok {
		t.Fatal("startpage has no selected target")
	}

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter produced no request command")
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != selected.target {
		t.Fatalf("requested = %v, want [%s]", *seen, selected.target)
	}
}

func TestStartNoticeNamesResolvedPath(t *testing.T) {
	file := bookmarkFile{problems: []parseProblem{{line: 3, reason: "expected one target"}}}
	got := startNotice(file, "/tmp/xdg/lookit/bookmarks")
	if !strings.Contains(got, "/tmp/xdg/lookit/bookmarks") {
		t.Fatalf("notice = %q, want the resolved path", got)
	}
	if !strings.Contains(got, "line 3") {
		t.Fatalf("notice = %q, want the line number", got)
	}
}

func TestStartEmptyMessageNamesResolvedPath(t *testing.T) {
	got := startEmptyMessage(bookmarkFile{catalogHidden: true}, "/tmp/xdg/lookit/bookmarks")
	if !strings.Contains(got, "/tmp/xdg/lookit/bookmarks") {
		t.Fatalf("empty message = %q, want the resolved path, not the ~/.config fallback", got)
	}
	if !strings.Contains(got, "catalog off") {
		t.Fatalf("empty message = %q, want it to name the directive to remove", got)
	}
}

func TestEmptyStartpageKeepsStatusOnLastTerminalRow(t *testing.T) {
	seedBookmarks(t, "catalog off\n")
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = step.(appModel)

	view := m.View().Content
	if got := lipgloss.Height(view); got != 24 {
		t.Fatalf("view height = %d, want terminal height 24:\n%s", got, view)
	}
	lines := strings.Split(view, "\n")
	if got, want := lines[len(lines)-1], m.statusBarModel().render(); got != want {
		t.Fatalf("last row = %q, want status bar %q", got, want)
	}
}

func TestEmptyStartpageExpandedHelpIsNotTruncated(t *testing.T) {
	seedBookmarks(t, "catalog off\n")
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = step.(appModel)
	m.blurInput()

	step, _ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m = step.(appModel)
	if !m.help {
		t.Fatal("precondition: help should be expanded")
	}
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	body := strings.Join(lines[:len(lines)-1], "\n")
	for _, want := range []string{"↑/↓", "move", "esc", "back"} {
		if !strings.Contains(body, want) {
			t.Errorf("expanded help missing %q:\n%s", want, body)
		}
	}
}

func seedBookmarks(t *testing.T, data string) string {
	t.Helper()
	path := useTempBookmarks(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("seed bookmarks: %v", err)
	}
	return path
}

// visibleTargets returns the selectable startpage targets in list order,
// which under an applied filter is bubbles' fuzzy rank order.
func visibleTargets(m startModel) []string {
	var out []string
	for _, it := range m.list.VisibleItems() {
		if si, ok := it.(startItem); ok && si.selectable() {
			out = append(out, si.entry.target)
		}
	}
	return out
}

func TestBookmarkingCatalogRowStaysAtSectionOrdinal(t *testing.T) {
	path := seedBookmarks(t, "@tilde.team\n")

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	if !m.start.selectTarget("@plan.cat") {
		t.Fatal("@plan.cat not found")
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	selected, ok := m.start.selected()
	if !ok || selected.target != "ring@thebackupbox.net" {
		t.Fatalf("selected = %+v, %v; want next community at the same ordinal", selected, ok)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "@plan.cat\n") {
		t.Fatalf("bookmark file = %q, err=%v", data, err)
	}
}

func TestBookmarkingCatalogRowsStayAtSectionOrdinal(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{name: "middle", target: "@tilde.team", want: "@zaibatsu.circumlunar.space"},
		{name: "final", target: "@zaibatsu.circumlunar.space", want: "@tilde.team"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useTempBookmarks(t)
			m := newApp(stubFetch(t), colorprofile.NoTTY)
			m.blurInput()
			if !m.start.selectTarget(tt.target) {
				t.Fatalf("%s not found", tt.target)
			}
			next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
			m = next.(appModel)
			selected, ok := m.start.selected()
			if !ok || selected.target != tt.want {
				t.Fatalf("selected = %+v, %v; want %q at the catalog ordinal", selected, ok, tt.want)
			}
		})
	}
}

func TestRemovingMiddleBookmarkStaysAtBookmarkOrdinal(t *testing.T) {
	seedBookmarks(t, "@plan.cat\n@tilde.team\n@happynetbox.com\n")
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	if !m.start.selectTarget("@tilde.team") {
		t.Fatal("@tilde.team not found")
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	selected, ok := m.start.selected()
	if !ok || selected.target != "@happynetbox.com" {
		t.Fatalf("selected = %+v, %v; want bookmark moved into the removed row's slot", selected, ok)
	}
}

func TestRemovingBookmarksStaysAtSectionOrdinal(t *testing.T) {
	tests := []struct {
		name string
		seed string
		pick string
		want string
	}{
		{name: "first", seed: "@plan.cat\n@tilde.team\n@happynetbox.com\n", pick: "@plan.cat", want: "@tilde.team"},
		{name: "final", seed: "@plan.cat\n@tilde.team\n@happynetbox.com\n", pick: "@happynetbox.com", want: "@tilde.team"},
		{name: "only", seed: "@tilde.team\n", pick: "@tilde.team", want: "@cosmic.voyage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seedBookmarks(t, tt.seed)
			m := newApp(stubFetch(t), colorprofile.NoTTY)
			m.blurInput()
			if !m.start.selectTarget(tt.pick) {
				t.Fatalf("%s not found", tt.pick)
			}
			next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
			m = next.(appModel)
			selected, ok := m.start.selected()
			if !ok || selected.target != tt.want {
				t.Fatalf("selected = %+v, %v; want %q at the section ordinal", selected, ok, tt.want)
			}
		})
	}
}

func TestRemovingLaterDuplicateBookmarkUsesActualOrdinal(t *testing.T) {
	seedBookmarks(t, "catalog off\n@tilde.team\n@plan.cat\n@tilde.team\n@happynetbox.com\n@telehack.com\n")
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	seen := 0
	selected := false
	for i, item := range m.start.list.VisibleItems() {
		entry, ok := item.(startItem)
		if !ok || !entry.selectable() || entry.entry.target != "@tilde.team" {
			continue
		}
		seen++
		if seen == 2 {
			m.start.list.Select(i)
			selected = true
			break
		}
	}
	if !selected {
		t.Fatal("second @tilde.team bookmark not found")
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	got, ok := m.start.selected()
	if !ok || got.target != "@telehack.com" {
		t.Fatalf("selected = %+v, %v; want ordinal of the acted-on duplicate", got, ok)
	}
}

func TestBookmarkingOnlyFinalCatalogSectionRowFallsBackward(t *testing.T) {
	var seed strings.Builder
	for _, entry := range loadCatalog() {
		if entry.kind == kindService && entry.target != "quake@bbs.airandwave.net" {
			seed.WriteString(entry.target)
			seed.WriteByte('\n')
		}
	}
	seedBookmarks(t, seed.String())
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	if !m.start.selectTarget("quake@bbs.airandwave.net") {
		t.Fatal("sole remaining service not found")
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	selected, ok := m.start.selected()
	// quake is the sole remaining service, but it sits at ordinal 1 within
	// SERVICES: a structural @bbs.airandwave.net parent row occupies ordinal 0
	// and is selectable() like any other row, so it counts toward ordinal
	// arithmetic. The fallback lands on COMMUNITIES ordinal 1.
	if !ok || selected.target != "@happynetbox.com" {
		t.Fatalf("selected = %+v, %v; want nearest earlier catalog section", selected, ok)
	}
}

func TestFilteredBookmarkTogglePreservesFilterAndOrdinal(t *testing.T) {
	useTempBookmarks(t)
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	m.start.list.SetFilterText("typed-hole")
	matches := visibleTargets(m.start)
	if len(matches) < 3 {
		t.Fatalf("precondition: filter matched %v, want at least 3 services", matches)
	}
	before, ok := m.start.selected()
	if !ok || before.target != matches[0] {
		t.Fatalf("precondition selected = %+v, %v; want first match %q", before, ok, matches[0])
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	selected, ok := m.start.selected()
	if m.start.list.FilterState() != list.FilterApplied || m.start.list.FilterValue() != "typed-hole" {
		t.Fatalf("filter state=%v value=%q", m.start.list.FilterState(), m.start.list.FilterValue())
	}
	// The pinned row leaves SERVICES for BOOKMARKS; ordinal 0 of the remaining
	// filtered services is what was matches[1].
	if !ok || selected.target != matches[1] {
		t.Fatalf("selected = %+v, %v; want next filtered service %q", selected, ok, matches[1])
	}
	if selected.source == sourceBookmark {
		t.Fatalf("selection followed the pinned target into BOOKMARKS: %+v", selected)
	}
}

func TestFilteredToggleClearsFilterWhenFinalMatchDisappears(t *testing.T) {
	seedBookmarks(t, "alice@plan.cat\n")
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	m.start.list.SetFilterText("alice@plan.cat")
	if got := visibleTargets(m.start); len(got) != 1 || got[0] != "alice@plan.cat" {
		t.Fatalf("precondition: matches = %v, want only alice@plan.cat", got)
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	if m.start.list.FilterState() != list.Unfiltered || m.start.list.FilterValue() != "" {
		t.Fatalf("filter state=%v value=%q, want cleared", m.start.list.FilterState(), m.start.list.FilterValue())
	}
	selected, ok := m.start.selected()
	if !ok || selected.kind != kindCommunity {
		t.Fatalf("selected = %+v, %v; want first unfiltered catalog section", selected, ok)
	}
}

func TestFilteredRemovalFallsToNextMatchingSection(t *testing.T) {
	seedBookmarks(t, "alice@plan.cat\n")
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	// "plan.cat", not "plan": the filter also matches notes, and
	// @happynetbox.com's note mentions .plan files.
	m.start.list.SetFilterText("plan.cat")
	if !m.start.selectTarget("alice@plan.cat") {
		t.Fatalf("precondition: filtered matches = %v, missing bookmark", visibleTargets(m.start))
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	selected, ok := m.start.selected()
	if m.start.list.FilterState() != list.FilterApplied || m.start.list.FilterValue() != "plan.cat" {
		t.Fatalf("filter state=%v value=%q, want applied plan.cat", m.start.list.FilterState(), m.start.list.FilterValue())
	}
	if !ok || selected.target != "@plan.cat" || selected.source != sourceCatalog {
		t.Fatalf("selected = %+v, %v; want next matching catalog section", selected, ok)
	}
}

func TestFilteredPinFallsToPreviousMatchingSection(t *testing.T) {
	useTempBookmarks(t)
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	m.start.list.SetFilterText("quake@")
	if got := visibleTargets(m.start); len(got) != 1 || got[0] != "quake@bbs.airandwave.net" {
		t.Fatalf("precondition: matches = %v, want only quake service", got)
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	selected, ok := m.start.selected()
	if m.start.list.FilterState() != list.FilterApplied || m.start.list.FilterValue() != "quake@" {
		t.Fatalf("filter state=%v value=%q, want applied quake@", m.start.list.FilterState(), m.start.list.FilterValue())
	}
	if !ok || selected.target != "quake@bbs.airandwave.net" || selected.source != sourceBookmark {
		t.Fatalf("selected = %+v, %v; want previous matching bookmark section", selected, ok)
	}
}

func TestBookmarkRejectsTargetThatCannotRoundTripThroughFile(t *testing.T) {
	path := useTempBookmarks(t)
	for _, raw := range []string{"weather:#oslo@bbs.airandwave.net", "alice smith@host"} {
		t.Run(raw, func(t *testing.T) {
			m := newApp(stubFetch(t), colorprofile.NoTTY)
			m.history = []histNode{{entry: Entry{Target: finger.Target{Raw: raw}}, state: stateReader, linkIdx: -1}}
			m.pos = 0
			m.state = stateReader
			m.blurInput()

			next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
			m = next.(appModel)
			if !strings.Contains(m.flash, "cannot bookmark") {
				t.Fatalf("flash = %q, want a bookmark validation error", m.flash)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("invalid bookmark changed the file: stat error = %v", err)
			}
		})
	}
}

func TestFilteredRemovingOnlyCatalogOffBookmarkFocusesInput(t *testing.T) {
	seedBookmarks(t, "catalog off\n@plan.cat\n")
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	m.start.list.SetFilterText("plan")
	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)

	if !m.inputFocused {
		t.Fatal("removing the only filtered startpage row should focus the input")
	}
	if m.start.list.FilterState() != list.Unfiltered || m.start.list.FilterValue() != "" {
		t.Fatalf("empty startpage filter state=%v value=%q, want cleared", m.start.list.FilterState(), m.start.list.FilterValue())
	}
	if _, ok := m.start.selected(); ok {
		t.Fatal("empty startpage unexpectedly has a selection")
	}
}

func TestStartpageBookmarkHelpIsContextualAndResetsOutsideStart(t *testing.T) {
	seedBookmarks(t, "@tilde.team\n")
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	m.updateKeymap()
	if got := m.keys.Bookmark.Help().Desc; got != "remove" {
		t.Fatalf("bookmark help = %q, want remove for a selected bookmark", got)
	}

	m.state = stateReader
	m.history = []histNode{{entry: Entry{Target: mustTarget(t, "alice@plan.cat")}, state: stateReader, linkIdx: -1}}
	m.pos = 0
	m.updateKeymap()
	if got := m.keys.Bookmark.Help().Desc; got != "bookmark" {
		t.Fatalf("reader bookmark help = %q, want bookmark after leaving startpage", got)
	}
}

func TestHomeTruncatesHistory(t *testing.T) {
	useTempBookmarks(t)

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.history = []histNode{
		{entry: Entry{Target: mustTarget(t, "@plan.cat")}, state: stateReader, linkIdx: -1},
		{entry: Entry{Target: mustTarget(t, "@tilde.team")}, state: stateReader, linkIdx: -1},
	}
	m.pos = 1
	m.state = stateReader
	m.blurInput()

	next, _ := m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = next.(appModel)
	if m.state != stateStart {
		t.Fatalf("state = %v, want stateStart", m.state)
	}
	if m.pos != -1 {
		t.Fatalf("pos = %d, want -1", m.pos)
	}
	if len(m.history) != 0 {
		t.Fatalf("history = %+v, want truncated", m.history)
	}
	// Focus follows how you arrived: h is pressed from content, so it lands on
	// content with a row selected rather than costing an extra ↓.
	if m.inputFocused {
		t.Fatal("h should land content-focused on the startpage")
	}
	if _, ok := m.start.selected(); !ok {
		t.Fatal("h should land on a selected row")
	}
}

// The exception: nothing to select is a dead end, so fall back to the input.
func TestHomeOnEmptyStartpageFocusesInput(t *testing.T) {
	path := useTempBookmarks(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("catalog off\n"), 0o600); err != nil {
		t.Fatalf("seed bookmarks: %v", err)
	}

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.history = []histNode{{entry: Entry{Target: mustTarget(t, "@plan.cat")}, state: stateReader, linkIdx: -1}}
	m.pos = 0
	m.state = stateReader
	m.blurInput()

	next, _ := m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = next.(appModel)
	if !m.inputFocused {
		t.Fatal("h onto an empty startpage should focus the input, not strand the cursor")
	}
}

func mustTarget(t *testing.T, raw string) finger.Target {
	t.Helper()
	target, err := finger.ParseTarget(raw)
	if err != nil {
		t.Fatalf("ParseTarget(%q) = %v", raw, err)
	}
	return target
}

func TestStartpageArrowDownEntersList(t *testing.T) {
	useTempBookmarks(t)

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	if !m.inputFocused {
		t.Fatal("launch should focus the input")
	}
	handled, m, _ := m.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if !handled {
		t.Fatal("down not handled while the input is focused")
	}
	if m.inputFocused {
		t.Fatal("down should move focus into the startpage list")
	}
}

func TestStartpageArrowDownAndEscSynchronizeSharedFocus(t *testing.T) {
	useTempBookmarks(t)

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	assertSharedFocusInverse(t, m)

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = next.(appModel)
	if m.inputFocused || !m.common.contentFocused {
		t.Fatalf("after down: inputFocused=%v contentFocused=%v", m.inputFocused, m.common.contentFocused)
	}
	assertSharedFocusInverse(t, m)

	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(appModel)
	if !m.inputFocused || m.common.contentFocused {
		t.Fatalf("after Esc: inputFocused=%v contentFocused=%v", m.inputFocused, m.common.contentFocused)
	}
	assertSharedFocusInverse(t, m)
}

func assertSharedFocusInverse(t *testing.T, m appModel) {
	t.Helper()
	if m.inputFocused == m.common.contentFocused {
		t.Fatalf("focus truths are not inverse: inputFocused=%v contentFocused=%v", m.inputFocused, m.common.contentFocused)
	}
}

func TestStartpageEscBacksOutThenQuits(t *testing.T) {
	useTempBookmarks(t)

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()

	// From the list, esc returns to the input rather than quitting.
	handled, m, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !handled {
		t.Fatal("esc not handled from the list")
	}
	if cmd != nil && isQuit(cmd) {
		t.Fatal("esc from the startpage list must not quit")
	}
	if !m.inputFocused {
		t.Fatal("esc should return focus to the input")
	}

	// From the input, esc quits.
	_, _, cmd = m.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc from the input at the startpage should quit")
	}
}

func TestBookmarkKeyTypesIntoFocusedInput(t *testing.T) {
	useTempBookmarks(t)

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	handled, _, _ := m.handleKey(tea.KeyPressMsg{Code: 'b', Text: "b"})
	if handled {
		t.Fatal("b must type into a focused input, not bookmark")
	}
}

func TestStartpageFilterOwnsCommandLetters(t *testing.T) {
	path := useTempBookmarks(t)

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	next, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = next.(appModel)
	if !m.start.filtering() {
		t.Fatal("/ did not enter startpage filtering")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = next.(appModel)
	if got := m.start.list.FilterInput.Value(); got != "b" {
		t.Fatalf("filter = %q, want b", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("typing b in the filter changed bookmarks: stat error = %v", err)
	}
}

func TestStartpageEscClearsAppliedFilterBeforeChangingFocus(t *testing.T) {
	useTempBookmarks(t)

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	m.start.list.SetFilterText("plan")
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(appModel)
	if m.start.list.FilterState() != list.Unfiltered {
		t.Fatalf("filter state = %v, want unfiltered", m.start.list.FilterState())
	}
	if m.inputFocused {
		t.Fatal("first Esc should clear the applied filter, not focus the input")
	}
}

// The full ladder from depth: content -> startpage list -> input -> quit. Three
// Esc presses, one per layer, with focus preserved until the last content layer.
func TestEscLadderFromResultToQuit(t *testing.T) {
	useTempBookmarks(t)

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	step, _ := deliverNavigationResult(m, fetchResultMsg{entry: Entry{
		Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n"),
	}})
	m = step.(appModel)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc}) // reader -> startpage
	m = next.(appModel)
	if cmd != nil && isQuit(cmd) {
		t.Fatal("first Esc must not quit")
	}
	if m.state != stateStart || m.inputFocused {
		t.Fatalf("state=%d inputFocused=%v, want the startpage, content-focused", m.state, m.inputFocused)
	}

	next, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc}) // list -> input
	m = next.(appModel)
	if cmd != nil && isQuit(cmd) {
		t.Fatal("second Esc must not quit")
	}
	if !m.inputFocused {
		t.Fatal("second Esc should return focus to the input")
	}

	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc}) // input -> quit
	if cmd == nil || !isQuit(cmd) {
		t.Fatal("third Esc should quit")
	}
}

// The bookmarks file is hand-editable, so a write from inside the app must be
// line surgery: comments, blank lines, ordering and the catalog directive all
// survive verbatim.
func TestBookmarkWriteThroughAppPreservesHandEdits(t *testing.T) {
	path := useTempBookmarks(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	seed := "# my careful notes\n\ncatalog off\n\n@plan.cat\n\n# trailing thought\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed bookmarks: %v", err)
	}

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.blurInput()
	if _, ok := m.start.selected(); !ok {
		t.Fatal("seeded bookmark is not selectable")
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"}) // removes @plan.cat
	m = next.(appModel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after unbookmark: %v", err)
	}
	want := "# my careful notes\n\ncatalog off\n\n\n# trailing thought\n"
	if string(data) != want {
		t.Fatalf("file =\n%q\nwant\n%q", data, want)
	}

	// And the startpage now explains itself rather than looking broken.
	if got := m.start.View(); !strings.Contains(got, "catalog off") {
		t.Errorf("empty startpage does not name the directive to remove:\n%s", got)
	}
}

// The b hint must describe what b will actually do on both forms of a parent:
// its canonical service listing when unpinned and its retained structural copy
// when pinned.
func TestParentBookmarkHintAndActionAgree(t *testing.T) {
	const target = "@bbs.airandwave.net"
	tests := []struct {
		name       string
		seed       string
		structural bool
		wantHint   string
		wantSaved  bool
	}{
		{name: "unpinned adds", wantHint: "bookmark", wantSaved: true},
		{name: "pinned structural removes", seed: target + "\n", structural: true, wantHint: "remove"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.seed == "" {
				path = useTempBookmarks(t)
			} else {
				path = seedBookmarks(t, tt.seed)
			}
			m := newApp(stubFetch(t), colorprofile.NoTTY)
			m.blurInput()

			found := false
			for i, item := range m.start.list.VisibleItems() {
				it, ok := item.(startItem)
				if ok && it.section == sectionServices && it.entry.target == target && it.entry.structural == tt.structural {
					m.start.list.Select(i)
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("service parent %q (structural=%v) not found", target, tt.structural)
			}
			if got := m.startBookmarkAction(); got != tt.wantHint {
				t.Fatalf("hint = %q, want %q", got, tt.wantHint)
			}

			next, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
			m = next.(appModel)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(data), target+"\n"); got != tt.wantSaved {
				t.Fatalf("bookmark present = %v, want %v; file = %q", got, tt.wantSaved, data)
			}
		})
	}
}

func TestFilteringShowsOneHappynetboxParentPinnedOrUnpinned(t *testing.T) {
	tests := []struct {
		name string
		seed string
	}{
		{name: "unpinned"},
		{name: "pinned", seed: "@happynetbox.com\n"},
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
			m.start.list.SetFilterText("happynetbox.com")
			var seen int
			for _, item := range m.start.list.VisibleItems() {
				if it, ok := item.(startItem); ok && it.entry.target == "@happynetbox.com" {
					seen++
				}
			}
			if seen != 1 {
				t.Fatalf("@happynetbox.com appears %d times under a filter, want 1", seen)
			}
		})
	}
}

func TestFilteringKeepsCanonicalServiceParents(t *testing.T) {
	for _, target := range []string{
		"@bbs.airandwave.net",
		"@flanigan.us",
		"@graph.no",
		"@typed-hole.org",
	} {
		t.Run(target, func(t *testing.T) {
			useTempBookmarks(t)
			m := newApp(stubFetch(t), colorprofile.NoTTY)
			m.blurInput()
			m.start.list.SetFilterText(target)
			var roots []startEntry
			for _, item := range m.start.list.VisibleItems() {
				if it, ok := item.(startItem); ok && it.entry.target == target {
					roots = append(roots, it.entry)
				}
			}
			if len(roots) != 1 || roots[0].structural {
				t.Fatalf("filtered roots = %+v, want one canonical parent", roots)
			}
		})
	}
}

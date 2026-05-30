package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

// fetchOnce returns a fetch func yielding fixed bytes and records the targets.
func fetchRecorder(body string) (FetchFunc, *[]string) {
	var seen []string
	f := func(_ context.Context, t finger.Target) ([]byte, finger.Meta, error) {
		seen = append(seen, t.Raw)
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

func TestHostFetchThatParsesOpensList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@tilde.team")
	entry := Entry{Target: target, Body: []byte(hostListBody()), Meta: finger.Meta{Addr: target.HostPort}}

	next, _ := m.Update(fetchResultMsg{entry: entry})
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

	next, _ := m.Update(fetchResultMsg{entry: entry})
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
	target := hostTarget(t, "@tilde.team")
	entry := Entry{Target: target, Body: []byte(hostListBodyWithPreamble()), Meta: finger.Meta{Addr: target.HostPort}}

	next, _ := m.Update(fetchResultMsg{entry: entry})
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

	next, _ := m.Update(fetchResultMsg{entry: entry})
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

	next, _ := m.Update(fetchResultMsg{entry: entry})
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
	users, _ := ParseUsers([]byte(hostListBody()))
	m.list = newList(m.common, host, users)
	m.state = stateList

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)

	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader after drill", got.state)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want fetch command")
	}
	cmd() // run the fetch command
	if len(*seen) != 1 || (*seen)[0] != "alrs@tilde.team" {
		t.Fatalf("fetched targets = %v, want [alrs@tilde.team]", *seen)
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
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	m.list = newListWithPreamble(m.common, host, users, body, false, false)
	m.state = stateList

	view := m.View().Content
	if !strings.Contains(view, "This is the finger ring!") {
		t.Fatalf("list view missing preamble: %q", view)
	}
	if strings.Contains(view, "=> 2026-05-25") {
		t.Fatalf("list view duplicated raw ring row: %q", view)
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader after drill", got.state)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want fetch command")
	}
	cmd()
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

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)

	if got.state != stateList || got.pos != 0 {
		t.Fatalf("state=%d pos=%d, want list/0 after Esc", got.state, got.pos)
	}
}

func TestEscInListReturnsToReaderHome(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	m.history = []histNode{{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList}}
	m.pos = 0
	m.state = stateList
	users, _ := ParseUsers([]byte(hostListBody()))
	m.list = newList(m.common, host, users)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)

	if got.state != stateReader || got.pos != -1 {
		t.Fatalf("state=%d pos=%d, want reader/-1 (landing)", got.state, got.pos)
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
	// bodyHeight = 24 - 1 (bar) = 23; reader viewport = 23 - chromeRows(2) = 21.
	if m.reader.viewport.Height() != 21 {
		t.Fatalf("viewport height = %d, want 21 (one row reserved for the bar)", m.reader.viewport.Height())
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
	users, _ := ParseUsers([]byte(hostListBody()))
	m.history = []histNode{{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList}}
	m.pos = 0
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.state = stateList

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
	step, _ := m.Update(fetchResultMsg{entry: entry})
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
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if got.state != stateReader {
		t.Fatalf("expected a drilled reader (state=%d)", got.state)
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
	cmd()

	if got.HostPort != "example.com:79" {
		t.Fatalf("HostPort = %q, want example.com:79 (server-supplied port must be pinned to 79)", got.HostPort)
	}
}

func TestDrillServerSuppliedTargetKeepsCrossHost(t *testing.T) {
	var got finger.Target
	host := hostTarget(t, "@thebackupbox.net")
	// A legitimate Finger Ring entry points at another host on port 79.
	users := []User{{Login: "yalla", Target: "yalla@tilde.team"}}

	cmd := drillFirstUser(t, host, users, captureFetch(&got))
	cmd()

	if got.HostPort != "tilde.team:79" {
		t.Fatalf("HostPort = %q, want tilde.team:79 (cross-host drilling must be preserved)", got.HostPort)
	}
}

func TestDrillSameHostKeepsUserTypedPort(t *testing.T) {
	var got finger.Target
	host := hostTarget(t, "@plan.cat:7979") // user typed an explicit port
	users := []User{{Login: "alice"}}       // no server-supplied target

	cmd := drillFirstUser(t, host, users, captureFetch(&got))
	cmd()

	if got.HostPort != "plan.cat:7979" {
		t.Fatalf("HostPort = %q, want plan.cat:7979 (user-typed port must be preserved)", got.HostPort)
	}
}

func genericListBody() string {
	// No Login header / online cue / "> " marker: forces the generic fallback.
	return "the crew:\nbetsy\nMelchizedek\nOleander\nStarbloom\n"
}

func TestGenericHostFetchOpensFlaggedList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 100, 24
	target := hostTarget(t, "@unknown.host")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: target, Body: []byte(genericListBody()), Meta: finger.Meta{Addr: target.HostPort}}})
	got := step.(appModel)
	if got.state != stateList || !got.list.generic {
		t.Fatalf("state=%d generic=%v, want list/true", got.state, got.list.generic)
	}
	if !strings.Contains(got.statusBarModel().render(), "auto-detected") {
		t.Fatalf("bar missing auto-detected flag")
	}
}

func TestRViewsRawBodyOnGenericList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@unknown.host")
	entry := Entry{Target: target, Body: []byte(genericListBody()), Meta: finger.Meta{Addr: target.HostPort}}
	opened, _ := m.Update(fetchResultMsg{entry: entry})
	m = opened.(appModel)

	next, _ := m.Update(tea.KeyPressMsg{Code: 'r'})
	got := next.(appModel)

	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader after r", got.state)
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

func TestRInertOnRecognizedList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@tilde.team")
	entry := Entry{Target: target, Body: []byte(hostListBody()), Meta: finger.Meta{Addr: target.HostPort}}
	opened, _ := m.Update(fetchResultMsg{entry: entry})
	m = opened.(appModel)

	next, _ := m.Update(tea.KeyPressMsg{Code: 'r'})
	got := next.(appModel)

	if got.state != stateList {
		t.Fatalf("state = %d, want stateList (r must be inert on a recognized list)", got.state)
	}
}

func TestForwardBackDoNotRefetch(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	userT := hostTarget(t, "bob@tilde.team")

	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	step, _ = m.Update(fetchResultMsg{entry: Entry{Target: userT, Body: []byte("Login: bob\n")}})
	m = step.(appModel)

	if len(m.history) != 2 || m.pos != 1 || m.state != stateReader {
		t.Fatalf("history=%d pos=%d state=%d, want 2/1/reader", len(m.history), m.pos, m.state)
	}

	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt})
	m = step.(appModel)
	if m.pos != 0 || m.state != stateList {
		t.Fatalf("after back: pos=%d state=%d, want 0/list", m.pos, m.state)
	}

	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModAlt})
	m = step.(appModel)
	if m.pos != 1 || m.state != stateReader {
		t.Fatalf("after forward: pos=%d state=%d, want 1/reader", m.pos, m.state)
	}
}

func TestNewNavigationTruncatesForwardTail(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	a := hostTarget(t, "@a.example")
	b := hostTarget(t, "@b.example")
	c := hostTarget(t, "@c.example")

	for _, tg := range []finger.Target{a, b} {
		step, _ := m.Update(fetchResultMsg{entry: Entry{Target: tg, Body: []byte(hostListBody())}})
		m = step.(appModel)
	}
	step, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt})
	m = step.(appModel)
	step, _ = m.Update(fetchResultMsg{entry: Entry{Target: c, Body: []byte(hostListBody())}})
	m = step.(appModel)

	if len(m.history) != 2 || m.pos != 1 {
		t.Fatalf("history=%d pos=%d, want 2/1 (forward tail truncated)", len(m.history), m.pos)
	}
	if got := m.history[1].entry.Target.Raw; got != c.Raw {
		t.Fatalf("head = %q, want %q", got, c.Raw)
	}
}

func TestAltLeftAtRootIsNoOp(t *testing.T) {
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
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)

	view := m.View().Content
	if !strings.Contains(view, "@tilde.team") {
		t.Fatalf("view missing breadcrumb host:\n%s", view)
	}
	if !strings.Contains(view, "? help") {
		t.Fatalf("view missing help hint:\n%s", view)
	}
}

func TestLandingViewShowsLandingBar(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	if !strings.Contains(m.View().Content, "type a target") {
		t.Fatalf("landing view missing landing hint:\n%s", m.View().Content)
	}
}

func TestRestorePreservesListSelection(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = step.(appModel)
	wantIdx := m.list.list.Index()
	step, _ = m.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t, "x@tilde.team"), Body: []byte("Login: x\n")}})
	m = step.(appModel)
	step, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = step.(appModel)
	if m.list.list.Index() != wantIdx {
		t.Fatalf("restored list index = %d, want %d", m.list.list.Index(), wantIdx)
	}
}

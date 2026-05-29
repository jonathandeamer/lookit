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
	if got.hostList == nil {
		t.Fatal("hostList not cached")
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
	m.hostList = &Entry{Target: host, Body: []byte(hostListBody())}
	users, _ := ParseUsers([]byte(hostListBody()))
	m.list = newList(m.common, host, users)
	m.state = stateList

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)

	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader after drill", got.state)
	}
	if !got.fromList {
		t.Fatal("fromList = false, want true after drill")
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
	m.hostList = &Entry{Target: host, Body: body}
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
	users, _ := ParseUsers([]byte(hostListBody()))
	m.hostList = &Entry{Target: host, Body: []byte(hostListBody())}
	m.list = newList(m.common, host, users)
	m.state = stateReader
	m.fromList = true

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)

	if got.state != stateList {
		t.Fatalf("state = %d, want stateList after Esc", got.state)
	}
	if got.fromList {
		t.Fatal("fromList = true, want false after returning to list")
	}
}

func TestEscInListReturnsToReaderHome(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	users, _ := ParseUsers([]byte(hostListBody()))
	m.list = newList(m.common, host, users)
	m.state = stateList

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)

	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader", got.state)
	}
	if cmd != nil && isQuit(cmd) {
		t.Fatal("Esc in list must not quit")
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
	// hostList set so the guarded list-resize branch runs (must not panic).
	m.hostList = &Entry{Target: host}
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
	m.hostList = &Entry{Target: host, Body: []byte(hostListBody())}
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

func TestTruncatedHostFetchMarksListIncomplete(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	entry := Entry{
		Target: host,
		Body:   []byte(hostListBody()),
		Meta:   finger.Meta{Addr: host.HostPort, Truncated: true},
	}

	next, _ := m.Update(fetchResultMsg{entry: entry})
	got := next.(appModel)

	if got.state != stateList {
		t.Fatalf("state = %d, want stateList", got.state)
	}
	if !strings.Contains(got.list.list.Title, "(incomplete)") {
		t.Fatalf("list title = %q, want it to contain (incomplete)", got.list.list.Title)
	}
}

func TestErroredHostFetchWithBodyMarksListIncomplete(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	entry := Entry{
		Target: host,
		Body:   []byte(hostListBody()),
		Meta:   finger.Meta{Addr: host.HostPort},
		Err:    errors.New("connection reset"),
	}

	next, _ := m.Update(fetchResultMsg{entry: entry})
	got := next.(appModel)

	if got.state != stateList {
		t.Fatalf("state = %d, want stateList (errored body that parses opens the list)", got.state)
	}
	if !strings.Contains(got.list.list.Title, "(incomplete)") {
		t.Fatalf("list title = %q, want it to contain (incomplete)", got.list.list.Title)
	}
}

func TestCompleteHostFetchListNotMarkedIncomplete(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	entry := Entry{
		Target: host,
		Body:   []byte(hostListBody()),
		Meta:   finger.Meta{Addr: host.HostPort},
	}

	next, _ := m.Update(fetchResultMsg{entry: entry})
	got := next.(appModel)

	if strings.Contains(got.list.list.Title, "(incomplete)") {
		t.Fatalf("list title = %q, should not contain (incomplete)", got.list.list.Title)
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
	m.hostList = &Entry{Target: host}
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.state = stateList
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if !got.fromList || got.state != stateReader {
		t.Fatalf("expected a drilled reader (fromList=%v state=%d)", got.fromList, got.state)
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

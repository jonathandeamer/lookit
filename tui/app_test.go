package tui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
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
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
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
	m.inputFocused = false // Enter must reach the list, not the input

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)

	// Drilling keeps the list on screen while loading (no eager switch to the
	// reader, which used to flash the previous profile for a frame).
	if !got.loading || got.state != stateList {
		t.Fatalf("after drill: loading=%v state=%d, want loading=true state=stateList", got.loading, got.state)
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
	users, ok := ParseUsers(body)
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
	if !got.loading || got.state != stateList {
		t.Fatalf("after drill: loading=%v state=%d, want loading=true state=stateList", got.loading, got.state)
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

func TestEscInListReturnsToReaderHome(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@tilde.team")
	m.history = []histNode{{entry: Entry{Target: host, Body: []byte(hostListBody())}, state: stateList}}
	m.pos = 0
	m.state = stateList
	m.inputFocused = false // Esc must reach the list, not the input
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
	users, _ := ParseUsers([]byte(hostListBody()))
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
	if !got.loading {
		t.Fatal("expected drilling to start loading")
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
	if got.loading {
		t.Fatal("server-supplied forwarded target must not start loading")
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

func TestVViewsSourceOnGenericList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@unknown.host")
	entry := Entry{Target: target, Body: []byte(genericListBody()), Meta: finger.Meta{Addr: target.HostPort}}
	opened, _ := m.Update(fetchResultMsg{entry: entry})
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
	opened, _ := m.Update(fetchResultMsg{entry: entry})
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

func TestRTogglesRawBodyOnProfile(t *testing.T) {
	// 'r' toggles "view source" on a profile too; a second 'r' restores it.
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "alice@plan.cat")
	body := "Login: alice\nPlan:\nhello from the raw body\n"
	opened, _ := m.Update(fetchResultMsg{entry: Entry{Target: target, Body: []byte(body)}})
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
		t.Fatal("a second r should exit raw view")
	}
	if gotOff.state != stateReader {
		t.Fatalf("exiting raw on a profile returns to the reader (state=%d)", gotOff.state)
	}
}

func TestEscBackDoesNotRefetch(t *testing.T) {
	// Esc navigates back through history without re-fetching.
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

	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	m.list.list.SetFilterText("kap")

	step, _ = m.Update(fetchResultMsg{entry: Entry{Target: nextHost, Body: []byte(hostListBody())}})
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
		step, _ := m.Update(fetchResultMsg{entry: Entry{Target: tg, Body: []byte(hostListBody())}})
		m = step.(appModel)
	}
	// Esc back to a (pos=0).
	step, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = step.(appModel)
	// Now fetch c — this must truncate the forward tail (b).
	fetch, _ := fetchRecorder(hostListBody())
	m.common.fetch = fetch
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

func TestQuestionMarkTogglesHelpOverlay(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 80, 24
	host := hostTarget(t, "@tilde.team")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
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
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(manyUserGridBody(30))}})
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
	users, _ := ParseUsers([]byte(hostListBody()))
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
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n")}})
	m = step.(appModel)
	// Now inputFocused==false; '?' should open help.
	step, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	if !step.(appModel).help {
		t.Fatal("'?' should open help from content-focused reader state")
	}
}

func TestHelpPanelUsesSharedContrastStyles(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
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
	step, _ = m.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
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
	// A second Esc backs to landing (pops the history node).
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@unknown.host")
	opened, _ := m.Update(fetchResultMsg{entry: Entry{Target: target, Body: []byte(genericListBody()), Meta: finger.Meta{Addr: target.HostPort}}})
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

	// Second Esc backs to landing.
	back2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = back2.(appModel)
	if m.pos != -1 {
		t.Fatalf("pos = %d, want -1 (landing) after second Esc", m.pos)
	}

	// At the landing (input focused), Esc quits.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil || !isQuit(cmd) {
		t.Fatal("Esc at landing should quit")
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
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
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
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
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
	step, _ := m2.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
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
		step, _ := m.Update(fetchResultMsg{entry: Entry{Target: tg, Body: []byte(hostListBody())}})
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
	if !m.loading {
		t.Fatal("submit should set loading")
	}
	if !strings.Contains(m.statusBarModel().render(), "bob@sdf.org") {
		t.Fatalf("loading bar should name the target:\n%s", m.statusBarModel().render())
	}
}

func TestBackgroundColorMsgRestylesTUI(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	oldBg := m.common.styles.palette.BaseBg

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
}

func TestBackgroundColorMsgRerendersCurrentReader(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.TrueColor)
	target := hostTarget(t, "alice@plan.cat")
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: target, Body: []byte("Login: alice\n")}})
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
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: target, Body: []byte("Login: alice\nPlan: raw\n")}})
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
	m.loading = true
	m.reqSeq = 1
	host := hostTarget(t, "@tilde.team")
	step, _ := m.Update(fetchResultMsg{reqID: 1, entry: Entry{Target: host, Body: []byte(hostListBody())}})
	if step.(appModel).loading {
		t.Fatal("a fetch result should clear loading")
	}
}

func TestHelpExpandsAtBottomNotFullScreen(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = step.(appModel)
	host := hostTarget(t, "@tilde.team")
	step, _ = m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
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
	step, _ = m.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t, "@big.host"), Body: []byte(body)}})
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
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
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
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: target, Body: []byte("Plan\n")}})
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
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: host, Body: []byte(hostListBody())}})
	m = step.(appModel)
	(&m).updateKeymap()
	if !m.keys.Open.Enabled() || !m.keys.Filter.Enabled() || !m.keys.Back.Enabled() ||
		!m.keys.Copy.Enabled() || !m.keys.FocusInput.Enabled() {
		t.Fatal("list content keys (open/filter/back/copy/focus) should be enabled")
	}

	// Profile reader → no Open/Filter (nothing to drill or filter); raw/copy/back live.
	step, _ = m.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
	m = step.(appModel)
	(&m).updateKeymap()
	if m.keys.Open.Enabled() || m.keys.Filter.Enabled() {
		t.Fatal("open/filter should be disabled in a profile reader")
	}
	if !m.keys.Raw.Enabled() || !m.keys.Copy.Enabled() || !m.keys.Back.Enabled() {
		t.Fatal("raw/copy/back should be enabled in a content reader with a result")
	}
}

// TestHelpPanelHidesInertKeys: the expanded '?' panel omits keys that do nothing
// in the current state (bubbles/help skips disabled bindings).
func TestHelpPanelHidesInertKeys(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
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
	step, _ := m.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
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
	m.loading = true

	stale := fetchResultMsg{reqID: 1, entry: Entry{Target: finger.Target{Raw: "a@x"}, Body: []byte("old\n")}}
	updated, _ := m.Update(stale)
	got := updated.(appModel)
	if !got.loading {
		t.Fatal("stale result cleared loading; in-flight request should still be loading")
	}

	current := fetchResultMsg{reqID: 2, entry: Entry{Target: finger.Target{Raw: "b@x"}, Body: []byte("new\n")}}
	updated2, _ := got.Update(current)
	got2 := updated2.(appModel)
	if got2.loading {
		t.Fatal("current result did not clear loading")
	}
	if got2.state != stateReader {
		t.Fatalf("current result did not route to reader: state = %d", got2.state)
	}
	if got2.pos < 0 {
		t.Fatal("current result did not push history: pos < 0")
	}
}

func TestPickSampleIsMember(t *testing.T) {
	for i := 0; i < 50; i++ {
		got := pickSample()
		found := false
		for _, s := range sampleTargets {
			if got == s {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("pickSample() = %q, not in sampleTargets", got)
		}
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
	if m.pos != -1 {
		t.Fatalf("want pos -1 after back-to-landing, got %d", m.pos)
	}
	view := stripANSIForLandingTest(m.View().Content)
	if strings.Contains(view, heroManicule+" "+heroWordmark) {
		t.Fatalf("back-to-landing should not show the wordmark:\n%s", view)
	}
	if !strings.Contains(view, "target:") {
		t.Fatalf("back-to-landing missing target row:\n%s", view)
	}
}

func TestAboutOpensFromBlurredResult(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(appModel)
	step, _ := m.Update(fetchResultMsg{entry: Entry{
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
	step, _ := m.Update(fetchResultMsg{entry: Entry{
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
	m := newApp(stubFetch(t), colorprofile.NoTTY) // landing: pos == -1, stateReader, no address
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
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
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
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
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
	step, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: Entry{
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
	if !got.loading {
		t.Fatalf("after seed submit: loading=false, want true")
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

	if got.loading {
		t.Fatalf("invalid seed: loading=true, want false")
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
	if !got.loading {
		t.Fatal("Enter on about should start a fetch (loading=true)")
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
	for _, want := range []string{"↵ go", "y copy", "esc back", "q quit"} {
		if !strings.Contains(bar.hints, want) {
			t.Fatalf("landing-origin about hints = %q, missing %q", bar.hints, want)
		}
	}

	// Opened from a result (pos>=0): the breadcrumb shows where esc returns, so
	// "esc back" is omitted from the hints (the escTarget convention).
	m2 := newApp(stubFetch(t), colorprofile.NoTTY)
	sized2, _ := m2.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 = sized2.(appModel)
	step, _ := m2.Update(fetchResultMsg{entry: Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: hi\n")}})
	m2 = step.(appModel)
	(&m2).openAbout()
	bar2 := m2.statusBarModel()
	if bar2.escTarget != "alice@plan.cat" {
		t.Fatalf("result-origin about bar escTarget = %q, want \"alice@plan.cat\"", bar2.escTarget)
	}
	if strings.Contains(bar2.hints, "esc back") {
		t.Fatalf("result-origin about hints should omit \"esc back\" (breadcrumb shows it): %q", bar2.hints)
	}
	for _, want := range []string{"↵ go", "y copy", "q quit"} {
		if !strings.Contains(bar2.hints, want) {
			t.Fatalf("result-origin about hints = %q, missing %q", bar2.hints, want)
		}
	}
}

package tui

import (
	"context"
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

package tui

import (
	"context"
	"errors"
	"image/color"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/jonathandeamer/lookit/finger"
)

func asyncMessages(cmd tea.Cmd) <-chan tea.Msg {
	out := make(chan tea.Msg, 8)
	var run func(tea.Cmd)
	run = func(next tea.Cmd) {
		go func() {
			msg := next()
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, child := range batch {
					run(child)
				}
				return
			}
			out <- msg
		}()
	}
	run(cmd)
	return out
}

func nextFetchResult(t *testing.T, msgs <-chan tea.Msg) fetchResultMsg {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case msg := <-msgs:
			if result, ok := msg.(fetchResultMsg); ok {
				return result
			}
		case <-deadline:
			t.Fatal("timed out waiting for fetchResultMsg")
		}
	}
}

func deliverNavigation(m appModel, entry Entry) appModel {
	m.reqSeq++
	m.pending = &pendingRequest{
		id: m.reqSeq, target: entry.Target, intent: requestNavigate,
		started: time.Now(), cancel: func() {},
	}
	next, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: entry})
	return next.(appModel)
}

func deliverNavigationResult(m appModel, msg fetchResultMsg) (tea.Model, tea.Cmd) {
	return deliverNavigation(m, msg.entry), nil
}

func settledReader(t *testing.T, entry Entry) appModel {
	t.Helper()
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.inputFocused = false
	m.input.Blur()
	node := histNode{entry: entry, state: stateReader, links: DetectLinks(entry.Body, entry.Target.HostPort), linkIdx: -1}
	m.history, m.pos, m.state = []histNode{node}, 0, stateReader
	m.reader.focusedLink = -1
	m.reader.setEntryWithLinks(entry, node.links)
	return m
}

func settledList(t *testing.T) appModel {
	t.Helper()
	target := hostTarget(t, "@tilde.team")
	routed := routeEntry(Entry{Target: target, Body: []byte("Users currently online:\n\nalice bob\n")})
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.history, m.pos = []histNode{routed.node}, 0
	m.showRouted(routed)
	return m
}

func deliverRefresh(m appModel, entry Entry, retry bool) appModel {
	view := m.captureRefreshView()
	m.reqSeq++
	m.pending = &pendingRequest{id: m.reqSeq, target: entry.Target, intent: requestRefresh, retry: retry, cancel: func() {}, view: &view}
	next, _ := m.Update(fetchResultMsg{reqID: m.reqSeq, entry: entry})
	return next.(appModel)
}

func TestReaderRefreshPreservesScrollAndLinkRaw(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	old := Entry{Target: target, Body: []byte("top\nhttps://example.com\n" + strings.Repeat("line\n", 40))}
	m := settledReader(t, old)
	m.reader.setSize(40, 6)
	m.reader.focusedLink = 0
	m.reader.setEntryWithLinks(old, m.history[0].links)
	m.reader.viewport.SetYOffset(8)
	fresh := Entry{Target: target, Body: []byte("changed\nhttps://example.com\n" + strings.Repeat("new\n", 40))}
	got := deliverRefresh(m, fresh, false)
	if got.reader.viewport.YOffset() != 8 {
		t.Fatalf("YOffset = %d", got.reader.viewport.YOffset())
	}
	if got.reader.focusedLink < 0 || got.reader.links[got.reader.focusedLink].Raw != "https://example.com" {
		t.Fatalf("focused link = %d, links=%#v", got.reader.focusedLink, got.reader.links)
	}
}

func TestRefreshUsesViewCapturedAtRequestStart(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	old := Entry{Target: target, Body: []byte(strings.Repeat("old\n", 40))}
	m := settledReader(t, old)
	m.reader.setSize(40, 6)
	m.reader.viewport.SetYOffset(8)
	_ = m.refreshCurrent()
	m.reader.viewport.SetYOffset(1) // simulate a live resize/clamp while modal
	fresh := Entry{Target: target, Body: []byte(strings.Repeat("fresh\n", 40))}
	next, _ := m.Update(fetchResultMsg{reqID: m.pending.id, entry: fresh})
	got := next.(appModel)
	if got.reader.viewport.YOffset() != 8 {
		t.Fatalf("YOffset = %d, want start-time snapshot 8", got.reader.viewport.YOffset())
	}
}

func TestReaderRefreshClearsMissingLinkInsteadOfReusingIndex(t *testing.T) {
	target := hostTarget(t, "alice@plan.cat")
	old := Entry{Target: target, Body: []byte("https://old.example\nhttps://keep.example\n")}
	m := settledReader(t, old)
	m.reader.focusedLink = 1
	m.history[0].linkIdx = 1
	fresh := Entry{Target: target, Body: []byte("https://replacement.example\n")}
	got := deliverRefresh(m, fresh, false)
	if got.reader.focusedLink != -1 || got.history[0].linkIdx != -1 {
		t.Fatalf("missing link reused numeric index: reader=%d node=%d", got.reader.focusedLink, got.history[0].linkIdx)
	}
}

func TestListRefreshPreservesFilterAndIdentity(t *testing.T) {
	target := hostTarget(t, "@tilde.team")
	routed := routeEntry(Entry{Target: target, Body: []byte("Users currently online:\n\nalice bob bobby\n")})
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.history, m.pos = []histNode{routed.node}, 0
	m.showRouted(routed)
	m.list.list.SetFilterText("bob")
	m.list.selectIdentity(userItem{login: "bobby"})
	got := deliverRefresh(m, Entry{Target: target, Body: []byte("Users currently online:\n\nbob bobby carol\n")}, false)
	selected, ok := got.list.selected()
	if got.list.list.FilterValue() != "bob" || !ok || selected.login != "bobby" {
		t.Fatalf("filter=%q selected=%#v ok=%v", got.list.list.FilterValue(), selected, ok)
	}
}

func TestRefreshTypeChangeResetsViewState(t *testing.T) {
	target := hostTarget(t, "@tilde.team")
	old := Entry{Target: target, Body: []byte(strings.Repeat("old\n", 30))}
	m := settledReader(t, old)
	m.reader.viewport.SetYOffset(9)
	got := deliverRefresh(m, Entry{Target: target, Body: []byte("Users currently online:\n\nalice bob\n")}, false)
	if got.state != stateList || got.list.list.Index() != 0 || got.list.list.FilterValue() != "" {
		t.Fatal("type-changing refresh retained incompatible state")
	}
}

func TestListToReaderRefreshResetsViewState(t *testing.T) {
	target := hostTarget(t, "@tilde.team")
	routed := routeEntry(Entry{Target: target, Body: []byte("Users currently online:\n\nalice bob\n")})
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.history, m.pos = []histNode{routed.node}, 0
	m.showRouted(routed)
	m.list.list.SetFilterText("bob")
	got := deliverRefresh(m, Entry{Target: target, Body: []byte("plain response\n")}, false)
	if got.state != stateReader || got.reader.viewport.YOffset() != 0 || got.reader.focusedLink != -1 {
		t.Fatalf("list-to-reader state = state %d offset %d link %d", got.state, got.reader.viewport.YOffset(), got.reader.focusedLink)
	}
}

func TestCancelRefreshPreservesReaderView(t *testing.T) {
	entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("https://example.com\n" + strings.Repeat("line\n", 30))}
	m := settledReader(t, entry)
	m.reader.setSize(40, 6)
	m.reader.focusedLink = 0
	m.reader.viewport.SetYOffset(7)
	before := m.history[0].entry
	_ = m.startRequest(entry.Target, requestRefresh, false)
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)
	if got.pending != nil || got.reader.viewport.YOffset() != 7 || got.reader.focusedLink != 0 || string(got.history[0].entry.Body) != string(before.Body) {
		t.Fatalf("cancelled refresh = pending %#v offset %d link %d entry %#v", got.pending, got.reader.viewport.YOffset(), got.reader.focusedLink, got.history[0].entry)
	}
}

func TestCancelListDrillPreservesFilterAndSelection(t *testing.T) {
	m := settledList(t)
	m.list.list.SetFilterText("bob")
	m.list.selectIdentity(userItem{login: "bob"})
	target := hostTarget(t, "bob@tilde.team")
	_ = m.startRequest(target, requestNavigate, false)
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)
	selected, ok := got.list.selected()
	if got.pending != nil || got.list.list.FilterValue() != "bob" || !ok || selected.login != "bob" || got.pos != 0 {
		t.Fatalf("cancelled drill = pending %#v filter %q selected %#v ok=%v pos=%d", got.pending, got.list.list.FilterValue(), selected, ok, got.pos)
	}
}

func TestRefreshReplacesNodeWithoutChangingHistoryShape(t *testing.T) {
	old := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
	m := settledReader(t, old)
	tail := histNode{entry: Entry{Target: hostTarget(t, "bob@plan.cat"), Body: []byte("tail\n")}, state: stateReader}
	m.history = append(m.history, tail)
	got := deliverRefresh(m, Entry{Target: old.Target, Body: []byte("fresh\n")}, false)
	if got.pos != 0 || len(got.history) != 2 || string(got.history[0].entry.Body) != "fresh\n" || string(got.history[1].entry.Body) != "tail\n" {
		t.Fatalf("refresh changed history shape: pos=%d len=%d", got.pos, len(got.history))
	}
}

func TestNavigationFailureStillPushesErrorNode(t *testing.T) {
	old := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
	m := settledReader(t, old)
	failed := Entry{Target: hostTarget(t, "bob@plan.cat"), Err: errors.New("dial failed")}
	got := deliverNavigation(m, failed)
	if got.pos != 1 || len(got.history) != 2 || got.history[1].entry.Err == nil || got.requestFailure != nil {
		t.Fatalf("navigation failure = pos %d history %#v warning %#v", got.pos, got.history, got.requestFailure)
	}
}

func TestEmptyBodyRefreshFailurePreservesEntry(t *testing.T) {
	old := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
	got := deliverRefresh(settledReader(t, old), Entry{Target: old.Target, Err: errors.New("dial failed")}, false)
	if string(got.history[0].entry.Body) != "old\n" || got.requestFailure == nil || got.requestFailure.err.Error() != "dial failed" {
		t.Fatalf("empty failure result = entry %#v failure %#v", got.history[0].entry, got.requestFailure)
	}
}

func TestCleanEmptyRefreshReplacesNode(t *testing.T) {
	old := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
	got := deliverRefresh(settledReader(t, old), Entry{Target: old.Target}, false)
	if len(got.history[0].entry.Body) != 0 || got.history[0].entry.Err != nil {
		t.Fatalf("clean empty refresh did not replace: %#v", got.history[0].entry)
	}
}

func TestBodyBearingRefreshErrorReplacesNode(t *testing.T) {
	old := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
	partial := Entry{Target: old.Target, Body: []byte("partial\n"), Meta: finger.Meta{Truncated: true}, Err: errors.New("read timed out")}
	got := deliverRefresh(settledReader(t, old), partial, false)
	if string(got.history[0].entry.Body) != "partial\n" || got.history[0].entry.Err == nil || got.requestFailure != nil {
		t.Fatalf("partial refresh = entry %#v failure %#v", got.history[0].entry, got.requestFailure)
	}
}

func TestSuccessfulRetryReplacesEmptyErrorNode(t *testing.T) {
	failed := Entry{Target: hostTarget(t, "alice@plan.cat"), Err: errors.New("dial failed")}
	got := deliverRefresh(settledReader(t, failed), Entry{Target: failed.Target, Body: []byte("recovered\n")}, true)
	if got.pos != 0 || len(got.history) != 1 || string(got.history[0].entry.Body) != "recovered\n" || got.history[0].entry.Err != nil || got.requestFailure != nil {
		t.Fatalf("successful retry = history %#v warning %#v", got.history, got.requestFailure)
	}
}

func TestEscapeCancelsPendingRequestAndDropsResult(t *testing.T) {
	started := make(chan struct{})
	fetch := func(ctx context.Context, _ finger.Target) ([]byte, finger.Meta, error) {
		close(started)
		<-ctx.Done()
		return []byte("partial"), finger.Meta{}, ctx.Err()
	}
	m := newApp(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@plan.cat")
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(appModel)
	msgs := asyncMessages(cmd)
	<-started

	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(appModel)
	if m.pending != nil || !m.inputFocused || m.input.Value() != "alice@plan.cat" {
		t.Fatalf("cancelled request = pending %#v, focused %v, value %q", m.pending, m.inputFocused, m.input.Value())
	}
	result := nextFetchResult(t, msgs)
	next, _ = m.Update(result)
	m = next.(appModel)
	if m.pos != -1 || len(m.history) != 0 {
		t.Fatalf("cancelled result landed: pos=%d history=%d", m.pos, len(m.history))
	}
}

func TestCancelRequestRestoresSharedInputFocus(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.inputFocused = false
	m.common.contentFocused = true
	m.pending = &pendingRequest{returnToInput: true, cancel: func() {}}

	cmd := m.cancelRequest()
	if cmd == nil {
		t.Fatal("cancelRequest returned no input focus command")
	}
	if !m.inputFocused || m.common.contentFocused {
		t.Fatalf("cancelled request focus: inputFocused=%v contentFocused=%v", m.inputFocused, m.common.contentFocused)
	}
}

func TestUnmatchedZeroIDResultIsDropped(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan: stale\n")}
	next, _ := m.Update(fetchResultMsg{entry: entry})
	got := next.(appModel)
	if got.pos != -1 || len(got.history) != 0 {
		t.Fatalf("unmatched zero-ID result landed: pos=%d history=%d", got.pos, len(got.history))
	}
}

func TestPendingRequestIsModal(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.pending = &pendingRequest{id: 1, target: hostTarget(t, "alice@plan.cat"), intent: requestNavigate, started: time.Now(), cancel: func() {}}
	before := m.input.Value()
	for _, msg := range []tea.KeyPressMsg{{Code: '?'}, {Code: 'i'}, {Code: 'j'}, {Code: tea.KeyEnter}} {
		next, cmd := m.Update(msg)
		m = next.(appModel)
		if cmd != nil || m.help || m.input.Value() != before || m.pending == nil {
			t.Fatalf("key %q escaped modal loading", msg.String())
		}
	}
}

func TestPendingStatusIncludesElapsedAndControls(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.pending = &pendingRequest{target: hostTarget(t, "alice@plan.cat"), started: time.Unix(100, 0), cancel: func() {}}
	got := m.pendingPriorityStatus(time.Unix(103, 900_000_000)).text()
	for _, want := range []string{"loading alice@plan.cat", "3s", "esc cancel", "q quit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("pending status %q missing %q", got, want)
		}
	}
}

func TestPendingStatusPrioritizesCancellationControls(t *testing.T) {
	target := hostTarget(t, "alice-with-an-extraordinarily-long-login-name@a-very-long-finger-hostname.example.org")
	for _, width := range []int{80, 40} {
		m := newApp(stubFetch(t), colorprofile.NoTTY)
		m.common.width = width
		m.pending = &pendingRequest{target: target, started: time.Now(), cancel: func() {}}

		bar := ansi.Strip(m.statusBarModel().render())
		if got := ansi.StringWidth(bar); got != width {
			t.Fatalf("width %d: rendered width = %d, want %d: %q", width, got, width, bar)
		}
		if strings.Contains(bar, "\n") {
			t.Fatalf("width %d: loading status wrapped: %q", width, bar)
		}
		for _, want := range []string{"loading", "…", "esc cancel", "q quit"} {
			if !strings.Contains(bar, want) {
				t.Fatalf("width %d: loading status %q missing %q", width, bar, want)
			}
		}
	}
}

func TestSessionCancellationCancelsAndQuits(t *testing.T) {
	ctx, cancelSession := context.WithCancel(context.Background())
	m := newAppWithContext(ctx, stubFetch(t), colorprofile.NoTTY, Options{})
	requestCancelled := false
	m.pending = &pendingRequest{cancel: func() { requestCancelled = true }}
	cancelSession()
	next, cmd := m.Update(sessionCanceledMsg{})
	if !requestCancelled || next.(appModel).pending != nil || cmd == nil {
		t.Fatal("session cancellation did not cancel and quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("session cancellation command = %T", cmd())
	}
}

func TestSubmitRecordsInputOriginBeforeBlur(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.input.SetValue("alice@plan.cat")
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(appModel)
	if cmd == nil || got.pending == nil || !got.pending.returnToInput || got.pending.intent != requestNavigate {
		t.Fatalf("submitted pending = %#v, cmd nil=%v", got.pending, cmd == nil)
	}
}

func TestCancelSeededLookupRestoresSeededInput(t *testing.T) {
	m := newAppWithOptions(stubFetch(t), colorprofile.NoTTY, Options{InitialQuery: "alice@plan.cat", Seed: true})
	next, _ := m.Update(seedSubmitMsg{})
	m = next.(appModel)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)
	if !got.inputFocused || got.input.Value() != "alice@plan.cat" || got.pending != nil || got.pos != -1 {
		t.Fatalf("cancelled seed = focused %v value %q pending %#v pos %d", got.inputFocused, got.input.Value(), got.pending, got.pos)
	}
}

func TestCancelSubmittedTargetOverContentRestoresEditor(t *testing.T) {
	old := Entry{Target: hostTarget(t, "old@plan.cat"), Body: []byte("old\n")}
	m := deliverNavigation(newApp(stubFetch(t), colorprofile.NoTTY), old)
	m.common.fetch = func(ctx context.Context, _ finger.Target) ([]byte, finger.Meta, error) {
		<-ctx.Done()
		return nil, finger.Meta{}, ctx.Err()
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'i'})
	m = next.(appModel)
	m.input.SetValue("new@plan.cat")
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(appModel)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)
	if !got.inputFocused || got.input.Value() != "new@plan.cat" || got.pos != 0 || string(got.history[0].entry.Body) != "old\n" {
		t.Fatalf("cancelled edit = focused %v value %q pos %d entry %#v", got.inputFocused, got.input.Value(), got.pos, got.history[0].entry)
	}
}

func TestStartRequestDefensivelyCancelsPriorRequest(t *testing.T) {
	cancelled := false
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.reqSeq = 4
	m.pending = &pendingRequest{id: 4, cancel: func() { cancelled = true }}
	target := hostTarget(t, "alice@plan.cat")
	cmd := m.startRequest(target, requestRefresh, true)
	if !cancelled || cmd == nil || m.pending == nil || m.pending.id != 5 || m.pending.target != target || m.pending.intent != requestRefresh || !m.pending.retry {
		t.Fatalf("replacement pending = %#v, prior cancelled=%v", m.pending, cancelled)
	}
}

func TestPendingQuitKeysCancelBeforeQuit(t *testing.T) {
	for _, keyMsg := range []tea.KeyPressMsg{{Code: 'q'}, {Code: 'c', Mod: tea.ModCtrl}} {
		cancelled := false
		m := newApp(stubFetch(t), colorprofile.NoTTY)
		m.pending = &pendingRequest{id: 1, cancel: func() { cancelled = true }}
		next, cmd := m.Update(keyMsg)
		if !cancelled || next.(appModel).pending != nil || !isQuit(cmd) {
			t.Fatalf("key %q: cancelled=%v pending=%#v quit=%v", keyMsg.String(), cancelled, next.(appModel).pending, isQuit(cmd))
		}
	}
}

func TestPendingStillProcessesTerminalMessages(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.pending = &pendingRequest{id: 1, cancel: func() {}}
	beforeSpinner := m.spin.View()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 91, Height: 27})
	m = next.(appModel)
	next, _ = m.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	m = next.(appModel)
	next, _ = m.Update(tea.BackgroundColorMsg{Color: color.White})
	m = next.(appModel)
	next, tickCmd := m.Update(spinner.TickMsg{Time: time.Now()})
	got := next.(appModel)
	if got.pending == nil || got.common.width != 91 || got.common.height != 27 || got.common.profile != colorprofile.TrueColor || got.common.darkBackground || got.spin.View() == beforeSpinner || tickCmd == nil {
		t.Fatalf("terminal updates lost while pending: pending=%#v size=%dx%d profile=%v dark=%v spinner=%q", got.pending, got.common.width, got.common.height, got.common.profile, got.common.darkBackground, got.spin.View())
	}
}

func TestRefreshKeyScopeAndDynamicHelp(t *testing.T) {
	entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n")}
	m := settledReader(t, entry)
	m.updateKeymap()
	if !m.keys.Refresh.Enabled() || m.keys.Refresh.Help().Desc != "refresh" {
		t.Fatal("reader refresh disabled")
	}
	m.inputFocused = true
	m.updateKeymap()
	if m.keys.Refresh.Enabled() {
		t.Fatal("refresh enabled in input")
	}
	m.inputFocused, m.showingRaw = false, true
	m.updateKeymap()
	if m.keys.Refresh.Enabled() {
		t.Fatal("refresh enabled in source")
	}
	m.showingRaw = false
	m.requestFailure = &requestFailure{err: errors.New("timeout")}
	m.updateKeymap()
	if !m.keys.Refresh.Enabled() || m.keys.Refresh.Help().Desc != "retry" {
		t.Fatal("retry help not active")
	}
}

func TestRStartsInPlaceRefresh(t *testing.T) {
	fetch, seen := fetchRecorder("fresh\n")
	m := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")})
	m.common.fetch = fetch
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'r'})
	got := next.(appModel)
	if got.pending == nil || got.pending.intent != requestRefresh || cmd == nil {
		t.Fatal("r did not start refresh")
	}
	runCmds(cmd)
	if len(*seen) != 1 || (*seen)[0] != "alice@plan.cat" {
		t.Fatalf("targets = %v", *seen)
	}
}

func TestFailedRefreshWarningCopy(t *testing.T) {
	entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
	m := settledReader(t, entry)
	m.common.width = 120
	m.requestFailure = &requestFailure{err: errors.New("read timed out")}
	bar := m.statusBarModel().render()
	for _, want := range []string{"refresh failed: read timed out", "showing previous response", "r retry"} {
		if !strings.Contains(bar, want) {
			t.Fatalf("bar %q missing %q", bar, want)
		}
	}
	m.requestFailure.retry = true
	bar = m.statusBarModel().render()
	if !strings.Contains(bar, "retry failed") || strings.Contains(bar, "showing previous response") {
		t.Fatalf("retry bar = %q", bar)
	}
}

func TestFailedRefreshWarningPrioritizesConsequenceAndRetry(t *testing.T) {
	target := hostTarget(t, "alice-with-a-long-login@a-very-long-finger-hostname.example.org")
	entry := Entry{Target: target, Body: []byte(strings.Repeat("old response line\n", 40))}
	for _, width := range []int{80, 40} {
		m := settledReader(t, entry)
		m.common.width = width
		m.reader.setSize(width, 5)
		m.requestFailure = &requestFailure{err: errors.New(strings.Repeat("upstream read timed out; ", 8))}

		bar := ansi.Strip(m.statusBarModel().render())
		if got := ansi.StringWidth(bar); got != width {
			t.Fatalf("width %d: rendered width = %d, want %d: %q", width, got, width, bar)
		}
		if strings.Contains(bar, "\n") {
			t.Fatalf("width %d: refresh warning wrapped: %q", width, bar)
		}
		for _, want := range []string{"showing previous response", "r retry"} {
			if !strings.Contains(bar, want) {
				t.Fatalf("width %d: refresh warning %q missing %q", width, bar, want)
			}
		}
		if strings.Contains(bar, "%") || strings.Contains(bar, formatBytes(len(entry.Body))) {
			t.Fatalf("width %d: lower-priority scroll/meta survived ahead of warning: %q", width, bar)
		}
	}
}

func TestCancelledRetryRestoresWarning(t *testing.T) {
	entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("old\n")}
	m := settledReader(t, entry)
	failure := &requestFailure{err: errors.New("timeout")}
	m.requestFailure = failure
	m.pending = &pendingRequest{target: entry.Target, retry: true, cancel: func() {}}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := next.(appModel)
	if got.pending != nil || got.requestFailure != failure {
		t.Fatal("cancelled retry lost warning")
	}
}

func TestMainContentRefreshStatus(t *testing.T) {
	reader := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n")})
	if got, want := reader.buildStatusBar().hints, "↑↓ scroll · r refresh · esc back · ? help"; got != want {
		t.Fatalf("reader hints = %q, want %q", got, want)
	}
	failed := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Err: errors.New("dial failed")})
	if got, want := failed.buildStatusBar().hints, "↑↓ scroll · r retry · esc back · ? help"; got != want {
		t.Fatalf("error hints = %q, want %q", got, want)
	}
	listed := settledList(t)
	if got, want := listed.buildStatusBar().hints, "↵ go · / filter · r refresh · esc back · ? help"; got != want {
		t.Fatalf("list hints = %q, want %q", got, want)
	}
}

func helpText(m appModel) string {
	m.updateKeymap()
	return ansi.Strip(m.helpView())
}

func TestRefreshHelpContexts(t *testing.T) {
	reader := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n")})
	if got := helpText(reader); !strings.Contains(got, "r refresh") {
		t.Fatalf("reader help:\n%s", got)
	}

	failed := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Err: errors.New("dial failed")})
	if got := helpText(failed); !strings.Contains(got, "r retry") {
		t.Fatalf("error help:\n%s", got)
	}

	warning := reader
	warning.requestFailure = &requestFailure{err: errors.New("timeout")}
	if got := helpText(warning); !strings.Contains(got, "r retry") {
		t.Fatalf("warning help:\n%s", got)
	}

	raw := reader
	raw.enterRaw()
	about := reader
	about.openAbout()
	linked := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("https://example.com\n")})
	next, _ := linked.Update(tea.KeyPressMsg{Code: 'L'})
	links := next.(appModel)
	filtering := settledList(t)
	next, _ = filtering.Update(tea.KeyPressMsg{Code: '/'})
	filtering = next.(appModel)
	for name, model := range map[string]appModel{"source": raw, "about": about, "links": links, "filter": filtering} {
		got := helpText(model)
		if strings.Contains(got, "r refresh") || strings.Contains(got, "r retry") {
			t.Fatalf("%s help advertises refresh:\n%s", name, got)
		}
	}
}

func TestRIsLiteralInTargetAndFilterInputs(t *testing.T) {
	target := newApp(stubFetch(t), colorprofile.NoTTY)
	next, _ := target.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	gotTarget := next.(appModel)
	if gotTarget.input.Value() != "r" || gotTarget.pending != nil {
		t.Fatalf("target r = %q pending %#v", gotTarget.input.Value(), gotTarget.pending)
	}

	filtered := settledList(t)
	next, _ = filtered.Update(tea.KeyPressMsg{Code: '/'})
	filtered = next.(appModel)
	next, _ = filtered.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	gotFilter := next.(appModel)
	if gotFilter.list.list.FilterValue() != "r" || gotFilter.pending != nil {
		t.Fatalf("filter r = %q pending %#v", gotFilter.list.list.FilterValue(), gotFilter.pending)
	}
}

func TestListFilteringDoesNotAdvertiseRefreshOrRetry(t *testing.T) {
	m := settledList(t)
	next, _ := m.Update(tea.KeyPressMsg{Code: '/'})
	m = next.(appModel)
	if got := m.buildStatusBar().hints; strings.Contains(got, "r refresh") {
		t.Fatalf("filter hints advertise refresh: %q", got)
	}
	m.requestFailure = &requestFailure{err: errors.New("timeout")}
	if got := m.statusBarModel().hints; strings.Contains(got, "r retry") {
		t.Fatalf("filter warning advertises retry: %q", got)
	}
}

func TestRWhileHelpOpenOnlyClosesHelp(t *testing.T) {
	m := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n")})
	m.help = true
	m.helpModel.ShowAll = true
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'r'})
	got := next.(appModel)
	if got.help || got.pending != nil || cmd != nil {
		t.Fatalf("r in help = help %v pending %#v cmd %T", got.help, got.pending, cmd)
	}
}

func modelWithWarning(t *testing.T) appModel {
	t.Helper()
	m := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte(strings.Repeat("line\n", 30))})
	m.requestFailure = &requestFailure{err: errors.New("old error")}
	return m
}

func TestWarningSurvivesScrollAndHelpCycle(t *testing.T) {
	m := modelWithWarning(t)
	failure := m.requestFailure
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = next.(appModel)
	next, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = next.(appModel)
	next, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	got := next.(appModel)
	if got.help || got.requestFailure != failure {
		t.Fatalf("warning after scroll/help = help %v warning %#v", got.help, got.requestFailure)
	}
}

func TestWarningClearsOnExplicitTransitions(t *testing.T) {
	tests := map[string]func(appModel) appModel{
		"back":   func(m appModel) appModel { _ = m.stepBack(); return m },
		"input":  func(m appModel) appModel { _ = m.focusInput(); return m },
		"about":  func(m appModel) appModel { m.openAbout(); return m },
		"source": func(m appModel) appModel { m.enterRaw(); return m },
	}
	for name, transition := range tests {
		got := transition(modelWithWarning(t))
		if got.requestFailure != nil {
			t.Fatalf("%s retained warning %#v", name, got.requestFailure)
		}
	}

	links := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("https://example.com\n")})
	links.requestFailure = &requestFailure{err: errors.New("old error")}
	next, _ := links.Update(tea.KeyPressMsg{Code: 'L'})
	if got := next.(appModel); got.requestFailure != nil {
		t.Fatalf("links retained warning %#v", got.requestFailure)
	}

	navigation := deliverNavigation(modelWithWarning(t), Entry{Target: hostTarget(t, "bob@plan.cat"), Err: errors.New("new navigation error")})
	if navigation.requestFailure != nil {
		t.Fatalf("navigation retained warning %#v", navigation.requestFailure)
	}

	refreshed := deliverRefresh(modelWithWarning(t), Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("fresh\n")}, true)
	if refreshed.requestFailure != nil {
		t.Fatalf("usable refresh retained warning %#v", refreshed.requestFailure)
	}
}

func TestRepeatedRetryFailureUsesLatestError(t *testing.T) {
	m := modelWithWarning(t)
	got := deliverRefresh(m, Entry{Target: m.history[0].entry.Target, Err: errors.New("new error")}, true)
	if got.requestFailure == nil || !got.requestFailure.retry || got.requestFailure.err.Error() != "new error" || string(got.history[0].entry.Body) == "" {
		t.Fatalf("repeated failure = warning %#v entry %#v", got.requestFailure, got.history[0].entry)
	}
}

func TestPendingKeymapEnablesOnlyCancellationAndQuit(t *testing.T) {
	m := settledReader(t, Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n")})
	m.pending = &pendingRequest{id: 1, cancel: func() {}}
	m.updateKeymap()
	bindings := map[string]key.Binding{
		"focus": m.keys.FocusInput, "back": m.keys.Back, "open": m.keys.Open,
		"filter": m.keys.Filter, "raw": m.keys.Raw, "refresh": m.keys.Refresh,
		"copy": m.keys.Copy, "help": m.keys.Help, "about": m.keys.About,
		"quit": m.keys.Quit, "force-quit": m.keys.ForceQuit, "move": m.keys.Move,
		"page": m.keys.Page, "jump": m.keys.Jump, "link-next": m.keys.LinkNext,
		"link-prev": m.keys.LinkPrev, "link-finger": m.keys.LinkFinger,
		"link-panel": m.keys.LinkPanel,
	}
	for name, binding := range bindings {
		want := name == "back" || name == "quit" || name == "force-quit"
		if binding.Enabled() != want {
			t.Fatalf("%s enabled=%v, want %v", name, binding.Enabled(), want)
		}
	}
}

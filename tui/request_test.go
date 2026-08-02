package tui

import (
	"context"
	"image/color"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
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
	got := m.pendingStatus(time.Unix(103, 900_000_000))
	for _, want := range []string{"loading alice@plan.cat", "3s", "esc cancel", "q quit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("pending status %q missing %q", got, want)
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

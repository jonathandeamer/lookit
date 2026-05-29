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

func stubFetch(t *testing.T) FetchFunc {
	t.Helper()
	return func(context.Context, finger.Target) ([]byte, finger.Meta, error) {
		t.Fatalf("fetch should not be called")
		return nil, finger.Meta{}, nil
	}
}

func TestNewModelInitialState(t *testing.T) {
	m := New(stubFetch(t), colorprofile.NoTTY)

	if !m.input.Focused() {
		t.Fatalf("input should be focused")
	}
	if m.loading {
		t.Fatalf("loading = true, want false")
	}
	if m.current != nil {
		t.Fatalf("current = %#v, want nil", m.current)
	}
	if m.status == "" {
		t.Fatalf("status should contain an initial hint")
	}
}

func TestInvalidEnterSetsStatusError(t *testing.T) {
	m := New(stubFetch(t), colorprofile.NoTTY)
	m.input.SetValue("not-a-target")

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(Model)

	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil for invalid input", cmd)
	}
	if got.loading {
		t.Fatalf("loading = true, want false")
	}
	if !strings.Contains(got.status, "error:") {
		t.Fatalf("status = %q, want error", got.status)
	}
}

func TestQuitKeysReturnCommand(t *testing.T) {
	for _, msg := range []tea.Msg{
		tea.KeyPressMsg{Code: tea.KeyEsc},
		tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl},
	} {
		m := New(stubFetch(t), colorprofile.NoTTY)
		_, cmd := m.Update(msg)
		if cmd == nil {
			t.Fatalf("Update(%#v) returned nil cmd, want quit cmd", msg)
		}
	}
}

func TestValidEnterStartsFetch(t *testing.T) {
	calls := 0
	fetch := func(context.Context, finger.Target) ([]byte, finger.Meta, error) {
		calls++
		return []byte("Login: alice\n"), finger.Meta{Addr: "plan.cat:79", Bytes: 13}, nil
	}

	m := New(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@plan.cat")

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := next.(Model)

	if !got.loading {
		t.Fatalf("loading = false, want true")
	}
	if cmd == nil {
		t.Fatalf("cmd = nil, want fetch command")
	}
	msg := cmd()
	if _, ok := msg.(fetchResultMsg); !ok {
		t.Fatalf("cmd returned %T, want fetchResultMsg", msg)
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls)
	}
}

func TestDuplicateEnterWhileLoadingDoesNotFetch(t *testing.T) {
	calls := 0
	fetch := func(context.Context, finger.Target) ([]byte, finger.Meta, error) {
		calls++
		return nil, finger.Meta{}, nil
	}

	m := New(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@plan.cat")
	m.loading = true

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil while loading", cmd)
	}
	if calls != 0 {
		t.Fatalf("fetch calls = %d, want 0", calls)
	}
}

func TestFetchSuccessUpdatesCurrentAndViewport(t *testing.T) {
	m := New(stubFetch(t), colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	msg := fetchResultMsg{entry: Entry{
		Target: target,
		Body:   []byte("Login: alice\n"),
		Meta:   finger.Meta{Addr: target.HostPort, Bytes: len("Login: alice\n")},
	}}

	next, cmd := m.Update(msg)
	got := next.(Model)

	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil", cmd)
	}
	if got.loading {
		t.Fatalf("loading = true, want false")
	}
	if got.current == nil || got.current.Target.Raw != "alice@plan.cat" {
		t.Fatalf("current = %#v, want alice entry", got.current)
	}
	if !strings.Contains(got.viewport.View(), "Login: alice") {
		t.Fatalf("viewport content missing body: %q", got.viewport.View())
	}
}

func TestFetchErrorUpdatesCurrentAndViewport(t *testing.T) {
	m := New(stubFetch(t), colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	msg := fetchResultMsg{entry: Entry{
		Target: target,
		Meta:   finger.Meta{Addr: target.HostPort},
		Err:    errors.New("dial failed"),
	}}

	next, _ := m.Update(msg)
	got := next.(Model)

	if got.loading {
		t.Fatalf("loading = true, want false")
	}
	if got.current == nil || got.current.Err == nil {
		t.Fatalf("current = %#v, want error entry", got.current)
	}
	if !strings.Contains(got.viewport.View(), "dial failed") {
		t.Fatalf("viewport content missing error: %q", got.viewport.View())
	}
}

func TestWindowSizeUpdatesViewport(t *testing.T) {
	m := New(stubFetch(t), colorprofile.NoTTY)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	got := next.(Model)

	if !got.ready {
		t.Fatalf("ready = false, want true")
	}
	if got.width != 100 || got.height != 30 {
		t.Fatalf("size = %dx%d, want 100x30", got.width, got.height)
	}
	if got.viewport.Width() != 100 {
		t.Fatalf("viewport width = %d, want 100", got.viewport.Width())
	}
	if got.viewport.Height() != 26 {
		t.Fatalf("viewport height = %d, want 26", got.viewport.Height())
	}
}

func TestColorProfileMessageUpdatesProfile(t *testing.T) {
	m := New(stubFetch(t), colorprofile.NoTTY)

	next, _ := m.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	got := next.(Model)

	if got.profile != colorprofile.TrueColor {
		t.Fatalf("profile = %v, want TrueColor", got.profile)
	}
}

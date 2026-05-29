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

func TestNewReaderInitialState(t *testing.T) {
	m := newReader(stubFetch(t), colorprofile.NoTTY)
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

func TestReaderInvalidEnterSetsStatusError(t *testing.T) {
	m := newReader(stubFetch(t), colorprofile.NoTTY)
	m.input.SetValue("not-a-target")

	next, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil for invalid input", cmd)
	}
	if next.loading {
		t.Fatalf("loading = true, want false")
	}
	if !strings.Contains(next.status, "error:") {
		t.Fatalf("status = %q, want error", next.status)
	}
}

func TestReaderValidEnterStartsFetch(t *testing.T) {
	calls := 0
	fetch := func(context.Context, finger.Target) ([]byte, finger.Meta, error) {
		calls++
		return []byte("Login: alice\n"), finger.Meta{Addr: "plan.cat:79", Bytes: 13}, nil
	}
	m := newReader(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@plan.cat")

	next, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !next.loading {
		t.Fatalf("loading = false, want true")
	}
	if cmd == nil {
		t.Fatalf("cmd = nil, want fetch command")
	}
	if _, ok := cmd().(fetchResultMsg); !ok {
		t.Fatalf("cmd did not return fetchResultMsg")
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls)
	}
}

func TestReaderDuplicateEnterWhileLoadingDoesNotFetch(t *testing.T) {
	calls := 0
	fetch := func(context.Context, finger.Target) ([]byte, finger.Meta, error) {
		calls++
		return nil, finger.Meta{}, nil
	}
	m := newReader(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@plan.cat")
	m.loading = true

	_, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil while loading", cmd)
	}
	if calls != 0 {
		t.Fatalf("fetch calls = %d, want 0", calls)
	}
}

func TestReaderSetEntryUpdatesViewport(t *testing.T) {
	m := newReader(stubFetch(t), colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	m.setEntry(Entry{
		Target: target,
		Body:   []byte("Login: alice\n"),
		Meta:   finger.Meta{Addr: target.HostPort, Bytes: len("Login: alice\n")},
	})
	if m.loading {
		t.Fatalf("loading = true, want false")
	}
	if m.current == nil || m.current.Target.Raw != "alice@plan.cat" {
		t.Fatalf("current = %#v, want alice entry", m.current)
	}
	if !strings.Contains(m.viewport.View(), "Login: alice") {
		t.Fatalf("viewport content missing body: %q", m.viewport.View())
	}
}

func TestReaderSetEntryError(t *testing.T) {
	m := newReader(stubFetch(t), colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	m.setEntry(Entry{
		Target: target,
		Meta:   finger.Meta{Addr: target.HostPort},
		Err:    errors.New("dial failed"),
	})
	if m.current == nil || m.current.Err == nil {
		t.Fatalf("current = %#v, want error entry", m.current)
	}
	if !strings.Contains(m.viewport.View(), "dial failed") {
		t.Fatalf("viewport content missing error: %q", m.viewport.View())
	}
}

func TestReaderSetSize(t *testing.T) {
	m := newReader(stubFetch(t), colorprofile.NoTTY)
	m.setSize(100, 30)
	if m.viewport.Width() != 100 {
		t.Fatalf("viewport width = %d, want 100", m.viewport.Width())
	}
	if m.viewport.Height() != 26 {
		t.Fatalf("viewport height = %d, want 26", m.viewport.Height())
	}
}

package tui

import (
	"context"
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

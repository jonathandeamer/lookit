package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

func TestReaderSetEntryUpdatesViewport(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
	target, err := finger.ParseTarget("alice@plan.cat")
	if err != nil {
		t.Fatal(err)
	}
	m.setEntry(Entry{
		Target: target,
		Body:   []byte("Login: alice\n"),
		Meta:   finger.Meta{Addr: target.HostPort, Bytes: len("Login: alice\n")},
	})
	if m.current == nil || m.current.Target.Raw != "alice@plan.cat" {
		t.Fatalf("current = %#v, want alice entry", m.current)
	}
	if !strings.Contains(m.viewport.View(), "Login: alice") {
		t.Fatalf("viewport content missing body: %q", m.viewport.View())
	}
}

func TestReaderSetEntryError(t *testing.T) {
	m := newReader(colorprofile.NoTTY)
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
	m := newReader(colorprofile.NoTTY)
	m.setSize(100, 30)
	if m.viewport.Width() != 100 {
		t.Fatalf("viewport width = %d, want 100", m.viewport.Width())
	}
	// chromeRows == 0: the reader is viewport-only, so viewport height == height passed.
	if m.viewport.Height() != 30 {
		t.Fatalf("viewport height = %d, want 30 (chromeRows==0, full height to viewport)", m.viewport.Height())
	}
}

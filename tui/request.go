package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jonathandeamer/lookit/finger"
)

type requestIntent uint8

const (
	requestNavigate requestIntent = iota
	requestRefresh
)

type pendingRequest struct {
	id            uint64
	target        finger.Target
	intent        requestIntent
	retry         bool
	returnToInput bool
	started       time.Time
	cancel        context.CancelFunc
	view          *refreshViewState
}

type requestFailure struct {
	retry  bool
	target finger.Target
	err    error
}

type sessionCanceledMsg struct{}

func waitForSessionCancel(ctx context.Context) tea.Cmd {
	return func() tea.Msg { <-ctx.Done(); return sessionCanceledMsg{} }
}

func (m *appModel) takePending() *pendingRequest {
	pending := m.pending
	m.pending = nil
	return pending
}

func (m *appModel) startRequest(target finger.Target, intent requestIntent, retry bool) tea.Cmd {
	if previous := m.takePending(); previous != nil {
		previous.cancel()
	}
	base := m.common.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)
	m.reqSeq++
	m.pending = &pendingRequest{
		id: m.reqSeq, target: target, intent: intent, retry: retry,
		returnToInput: m.inputFocused, started: time.Now(), cancel: cancel,
	}
	return tea.Batch(fetchCmd(ctx, m.common.fetch, target, m.reqSeq), m.spin.Tick)
}

func (m *appModel) finishRequest(id uint64) (pendingRequest, bool) {
	if m.pending == nil || m.pending.id != id {
		return pendingRequest{}, false
	}
	pending := *m.takePending()
	pending.cancel()
	return pending, true
}

func (m *appModel) cancelRequest() tea.Cmd {
	pending := m.takePending()
	if pending == nil {
		return nil
	}
	pending.cancel()
	if !pending.returnToInput {
		return nil
	}
	m.inputFocused = true
	m.input.CursorEnd()
	m.resize()
	return m.input.Focus()
}

func (m appModel) pendingStatus(now time.Time) string {
	if m.pending == nil {
		return ""
	}
	parts := []string{m.spin.View() + " loading " + m.pending.target.Raw}
	if elapsed := now.Sub(m.pending.started).Truncate(time.Second); elapsed >= time.Second {
		parts = append(parts, elapsed.String())
	}
	return strings.Join(append(parts, "esc cancel", "q quit"), " · ")
}

func (f requestFailure) statusText() string {
	operation, consequence := "refresh", " · showing previous response"
	if f.retry {
		operation, consequence = "retry", ""
	}
	return fmt.Sprintf("%s failed: %s%s · r retry", operation, f.err, consequence)
}

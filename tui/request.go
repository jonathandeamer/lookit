package tui

import (
	"context"
	"fmt"
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

// requestFailure is the persistent warning left behind by an empty-body
// refresh or retry. It carries no target: the failure always belongs to the
// current history node, which the status bar's breadcrumb already names.
type requestFailure struct {
	retry bool
	err   error
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
	returnToInput := m.inputFocused
	if intent == requestNavigate {
		m.setAddress(target.Raw)
	}
	m.pending = &pendingRequest{
		id: m.reqSeq, target: target, intent: intent, retry: retry,
		returnToInput: returnToInput, started: time.Now(), cancel: cancel,
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
		m.restoreVisibleAddress()
		return nil
	}
	m.setInputFocused(true)
	m.input.CursorEnd()
	m.resize()
	return m.input.Focus()
}

func (m appModel) pendingPriorityStatus(now time.Time) priorityStatus {
	if m.pending == nil {
		return priorityStatus{}
	}
	suffix := ""
	if elapsed := now.Sub(m.pending.started).Truncate(time.Second); elapsed >= time.Second {
		suffix = " · " + elapsed.String()
	}
	return priorityStatus{
		prefix: m.spin.View() + " loading ",
		detail: m.pending.target.Raw,
		suffix: suffix + " · esc cancel · q quit",
	}
}

func (f requestFailure) priorityStatus() priorityStatus {
	operation, consequence := "refresh", " · showing previous response"
	if f.retry {
		operation, consequence = "retry", ""
	}
	return priorityStatus{
		prefix: fmt.Sprintf("%s failed: ", operation),
		detail: f.err.Error(),
		suffix: consequence + " · r retry",
	}
}

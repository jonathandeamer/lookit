package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/jonathandeamer/lookit/finger"
)

type Entry struct {
	Target finger.Target
	Body   []byte
	Meta   finger.Meta
	Err    error
}

// failed reports a request that produced no response at all. It is the
// difference between "the server said nothing" and "the server said nothing
// useful": an errored response that still carries bytes is a partial success,
// with a body to scroll and a byte count worth reporting.
//
// The status bar and the retry keybinding both key off this, so they cannot
// disagree about whether a response landed.
func (e Entry) failed() bool {
	return e.Err != nil && len(e.Body) == 0
}

type FetchFunc func(context.Context, finger.Target) ([]byte, finger.Meta, error)

type fetchResultMsg struct {
	reqID uint64
	entry Entry
}

func defaultFetch(ctx context.Context, target finger.Target) ([]byte, finger.Meta, error) {
	return finger.Query(ctx, target)
}

func fetchCmd(ctx context.Context, fetch FetchFunc, target finger.Target, reqID uint64) tea.Cmd {
	return func() tea.Msg {
		body, meta, err := fetch(ctx, target)
		return fetchResultMsg{
			reqID: reqID,
			entry: Entry{
				Target: target,
				Body:   body,
				Meta:   meta,
				Err:    err,
			},
		}
	}
}

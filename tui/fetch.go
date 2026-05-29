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

type FetchFunc func(context.Context, finger.Target) ([]byte, finger.Meta, error)

type fetchResultMsg struct {
	entry Entry
}

func defaultFetch(ctx context.Context, target finger.Target) ([]byte, finger.Meta, error) {
	return finger.Query(ctx, target)
}

func fetchCmd(ctx context.Context, fetch FetchFunc, target finger.Target) tea.Cmd {
	return func() tea.Msg {
		body, meta, err := fetch(ctx, target)
		return fetchResultMsg{
			entry: Entry{
				Target: target,
				Body:   body,
				Meta:   meta,
				Err:    err,
			},
		}
	}
}

package tui

import (
	"testing"

	"github.com/jonathandeamer/lookit/finger"
)

func TestActionsForLink(t *testing.T) {
	definite := Link{Kind: LinkFinger, Action: ActionDrill, Target: finger.Target{HostPort: "tilde.team:79"}}
	ambiguous := Link{Kind: LinkFinger, Action: ActionCopy, Ambiguous: true, Target: finger.Target{HostPort: "tilde.team:79"}}
	blocked := Link{Kind: LinkFinger, Action: ActionCopy, Blocked: "cross-relay"}
	tests := []struct {
		name string
		link Link
		want linkActions
	}{
		{"definite", definite, linkActions{enter: linkEnterGo, copy: true}},
		{"ambiguous", ambiguous, linkActions{finger: true, copy: true}},
		{"url", Link{Kind: LinkURL, Action: ActionCopy}, linkActions{copy: true}},
		{"email", Link{Kind: LinkEmail, Action: ActionCopy}, linkActions{copy: true}},
		{"social", Link{Kind: LinkSocial, Action: ActionCopy}, linkActions{copy: true}},
		{"blocked", blocked, linkActions{enter: linkEnterRefuse, copy: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := actionsForLink(tt.link); got != tt.want {
				t.Fatalf("actionsForLink = %+v, want %+v", got, tt.want)
			}
		})
	}
}

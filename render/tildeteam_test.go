package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

func TestIsTildeTeam(t *testing.T) {
	cases := []struct {
		hostport string
		want     bool
	}{
		{"tilde.team:79", true},
		{"TILDE.TEAM:79", true},
		{"tilde.team", true},
		{"plan.cat:79", false},
		{"nottilde.team:79", false},
	}
	for _, c := range cases {
		got := isTildeTeam(finger.Target{HostPort: c.hostport})
		if got != c.want {
			t.Errorf("isTildeTeam(%q) = %v, want %v", c.hostport, got, c.want)
		}
	}
}

func TestRender_PronounsHighlightedOnTildeOnly(t *testing.T) {
	body := []byte("Pronouns: he/him\n")
	theme := NewTheme(colorprofile.TrueColor)
	styledLabel := theme.Field.Render("Pronouns:")

	tilde := finger.Target{HostPort: "tilde.team:79", Raw: "@tilde.team"}
	gotTilde := Render(tilde, body, finger.Meta{Addr: "tilde.team:79"}, nil, colorprofile.TrueColor)
	if !strings.Contains(gotTilde, styledLabel) {
		t.Errorf("tilde.team render should style the Pronouns label.\n--- got ---\n%s", gotTilde)
	}

	other := finger.Target{HostPort: "plan.cat:79", Raw: "@plan.cat"}
	gotOther := Render(other, body, finger.Meta{Addr: "plan.cat:79"}, nil, colorprofile.TrueColor)
	if strings.Contains(gotOther, styledLabel) {
		t.Errorf("non-tilde render must NOT style the Pronouns label.\n--- got ---\n%s", gotOther)
	}
}

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

func TestReflowPronouns(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"inline value", "Pronouns: he/him\n", "Pronouns:\n  he/him\n"},
		{"value with spaces", "Pronouns: she/her, ask\n", "Pronouns:\n  she/her, ask\n"},
		{"bare label untouched", "Pronouns:\n", "Pronouns:\n"},
		{"no pronouns line", "Plan:\n  hi\n", "Plan:\n  hi\n"},
		{"surrounded by blocks", "Plan:\n  hi\n\nPronouns: they/them\n", "Plan:\n  hi\n\nPronouns:\n  they/them\n"},
	}
	for _, c := range cases {
		got := string(reflowPronouns([]byte(c.in)))
		if got != c.want {
			t.Errorf("%s: reflowPronouns(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestRender_PronounsReflowedOnTildeOnly(t *testing.T) {
	body := []byte("Pronouns: he/him\n")

	tilde := finger.Target{HostPort: "tilde.team:79", Raw: "@tilde.team"}
	gotTilde := Render(tilde, body, finger.Meta{Addr: "tilde.team:79"}, nil, colorprofile.NoTTY)
	if !strings.Contains(gotTilde, "Pronouns:\n  he/him") {
		t.Errorf("tilde.team render should reflow Pronouns into a block.\n--- got ---\n%s", gotTilde)
	}

	other := finger.Target{HostPort: "plan.cat:79", Raw: "@plan.cat"}
	gotOther := Render(other, body, finger.Meta{Addr: "plan.cat:79"}, nil, colorprofile.NoTTY)
	if !strings.Contains(gotOther, "Pronouns: he/him") || strings.Contains(gotOther, "Pronouns:\n  he/him") {
		t.Errorf("non-tilde render must leave the Pronouns line inline.\n--- got ---\n%s", gotOther)
	}
}

func TestRender_PronounsHighlightedOnTildeOnly(t *testing.T) {
	body := []byte("Pronouns: he/him\n")
	theme := NewThemeWithBackground(colorprofile.TrueColor, true)
	styledLabel := theme.Field.Render("Pronouns:")

	tilde := finger.Target{HostPort: "tilde.team:79", Raw: "@tilde.team"}
	gotTilde := RenderWithBackground(tilde, body, finger.Meta{Addr: "tilde.team:79"}, nil, colorprofile.TrueColor, true)
	if !strings.Contains(gotTilde, styledLabel) {
		t.Errorf("tilde.team render should style the Pronouns label.\n--- got ---\n%s", gotTilde)
	}

	other := finger.Target{HostPort: "plan.cat:79", Raw: "@plan.cat"}
	gotOther := RenderWithBackground(other, body, finger.Meta{Addr: "plan.cat:79"}, nil, colorprofile.TrueColor, true)
	if strings.Contains(gotOther, styledLabel) {
		t.Errorf("non-tilde render must NOT style the Pronouns label.\n--- got ---\n%s", gotOther)
	}
}

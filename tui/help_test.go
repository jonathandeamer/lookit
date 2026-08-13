package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func helpTestBinding(k, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(k), key.WithHelp(k, desc))
}

func helpKeys(bindings []key.Binding) []string {
	out := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, binding.Help().Key)
	}
	return out
}

func TestHelpCandidatesUsePriorityOrder(t *testing.T) {
	m := readerWithFocusedLink(t, stubFetch(t), Link{
		Kind: LinkFinger, Action: ActionDrill, Raw: "alice@tilde.team",
		Target: hostTarget(t, "alice@tilde.team"),
	})
	m.common.width, m.common.height = 200, 40
	m.help = true
	(&m).updateKeymap()
	got := strings.Join(helpKeys(m.helpLayout().bindings), ",")
	want := "↑/↓,↵,esc,i,h,←/→,tab,shift+tab,v,r,y,b,L,a,q"
	if got != want {
		t.Fatalf("Help order = %q, want %q", got, want)
	}
}

func TestLinksPanelHelpCandidatesStaySelectionAwareAndIncludeAbout(t *testing.T) {
	target := hostTarget(t, "alice@tilde.team")
	tests := []struct {
		name string
		link Link
		want string
	}{
		{"URL", Link{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"}, "↑/↓,esc,y,/,a"},
		{"ambiguous", Link{Kind: LinkFinger, Action: ActionCopy, Raw: target.Raw, Target: target, Ambiguous: true}, "↑/↓,esc,f,y,/,a"},
		{"definite", Link{Kind: LinkFinger, Action: ActionDrill, Raw: target.Raw, Target: target}, "↑/↓,esc,↵,y,/,a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := linksPanelModel(t, stubFetch(t), []Link{tt.link})
			m.help = true
			(&m).updateKeymap()
			if got := strings.Join(helpKeys(m.helpLayout().bindings), ","); got != tt.want {
				t.Fatalf("Help order = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLayoutHelpFiltersDisabledAndFillsColumnsTopToBottom(t *testing.T) {
	st := newStyles(true)
	bindings := []key.Binding{
		helpTestBinding("1", "one"), helpTestBinding("2", "two"),
		helpTestBinding("3", "three"), helpTestBinding("4", "four"),
		helpTestBinding("5", "five"), helpTestBinding("6", "six"),
	}
	bindings[2].SetEnabled(false)

	layout := layoutHelp(bindings, st, 200, 20)
	if got, want := len(layout.columns), 3; got != want {
		t.Fatalf("columns = %d, want %d", got, want)
	}
	if got, want := strings.Join(helpKeys(layout.bindings), ","), "1,2,4,5,6"; got != want {
		t.Fatalf("retained = %q, want %q", got, want)
	}
	wants := []string{"1,2", "4,5", "6"}
	for i, want := range wants {
		if got := strings.Join(helpKeys(layout.columns[i]), ","); got != want {
			t.Fatalf("column %d = %q, want %q", i, got, want)
		}
	}
}

func TestLayoutHelpChoosesMostColumnsThatFit(t *testing.T) {
	st := newStyles(true)
	bindings := []key.Binding{
		helpTestBinding("1", "alpha"), helpTestBinding("2", "bravo"),
		helpTestBinding("3", "charlie"), helpTestBinding("4", "delta"),
		helpTestBinding("5", "echo"),
	}
	tests := []struct {
		name  string
		width int
		want  int
	}{
		{"three", helpColumnsWidth(partitionHelpBindings(bindings, 3), st), 3},
		{"two", helpColumnsWidth(partitionHelpBindings(bindings, 2), st), 2},
		{"one", helpColumnsWidth(partitionHelpBindings(bindings, 1), st), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(layoutHelp(bindings, st, tt.width, 20).columns); got != tt.want {
				t.Fatalf("columns = %d, want %d at width %d", got, tt.want, tt.width)
			}
		})
	}
}

func TestLayoutHelpShortHeightRetainsPriorityPrefix(t *testing.T) {
	st := newStyles(true)
	bindings := []key.Binding{
		helpTestBinding("1", "one"), helpTestBinding("2", "two"),
		helpTestBinding("3", "three"), helpTestBinding("4", "four"),
		helpTestBinding("5", "five"), helpTestBinding("6", "six"),
		helpTestBinding("7", "seven"),
	}
	layout := layoutHelp(bindings, st, 200, 1)
	if got, want := strings.Join(helpKeys(layout.bindings), ","), "1,2,3"; got != want {
		t.Fatalf("retained = %q, want priority prefix %q", got, want)
	}
	if layout.matches(tea.KeyPressMsg{Code: '7', Text: "7"}) {
		t.Fatal("height-clipped binding must not match Help dispatch")
	}
	if !layout.matches(tea.KeyPressMsg{Code: '2', Text: "2"}) {
		t.Fatal("retained binding should match Help dispatch")
	}
}

func TestRenderHelpUsesFullWidthStyledRows(t *testing.T) {
	st := newStyles(true)
	layout := layoutHelp([]key.Binding{
		helpTestBinding("x", "first"), helpTestBinding("y", "second"),
	}, st, 40, 20)
	view := renderHelp(layout, st, 40)
	for _, line := range strings.Split(view, "\n") {
		assertFullWidthStyledLine(t, "help row", line, 40, st.palette.SubtleBg)
	}
	plain := ansi.Strip(view)
	if !strings.Contains(plain, "x") || !strings.Contains(plain, "first") {
		t.Fatalf("rendered Help missing binding:\n%s", plain)
	}
}

func TestOverlayHelpClipsFromBottom(t *testing.T) {
	got := overlayHelp("body one\nbody two", "core\nsecondary\nlast")
	if want := "core\nsecondary"; got != want {
		t.Fatalf("overlay = %q, want priority prefix %q", got, want)
	}
}

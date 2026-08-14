package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
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

func TestHelpLayerAvailability(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	m.common.width, m.common.height = 200, 40
	(&m).updateKeymap()
	if m.keys.About.Enabled() {
		t.Fatal("About must be disabled while focused input directly owns a")
	}
	if !m.keys.Help.Enabled() || !m.keys.Open.Enabled() {
		t.Fatal("Help and target submission must remain available")
	}

	m.openHelp()
	(&m).updateKeymap()
	if !m.keys.About.Enabled() {
		t.Fatal("opening Help must enable its dedicated About route")
	}
	got := strings.Join(helpKeys(m.helpLayout().bindings), ",")
	if !strings.Contains(got, "a") {
		t.Fatalf("focused-input Help must display About: %q", got)
	}
}

func TestHelpAvailabilityFollowsFilterAndAboutOwnership(t *testing.T) {
	filtered := settledList(t)
	next, _ := filtered.Update(tea.KeyPressMsg{Code: '/'})
	filtered = next.(appModel)
	(&filtered).updateKeymap()
	if filtered.keys.Help.Enabled() || filtered.keys.About.Enabled() {
		t.Fatal("active list filter must own ? and a")
	}

	panel := linksPanelModel(t, stubFetch(t), []Link{
		{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"},
	})
	(&panel).updateKeymap()
	if panel.keys.About.Enabled() {
		t.Fatal("About must be disabled while links panel directly owns keys")
	}
	panel.openHelp()
	(&panel).updateKeymap()
	if !panel.keys.About.Enabled() {
		t.Fatal("links-panel Help must enable About")
	}

	about := newApp(stubFetch(t), colorprofile.NoTTY)
	about.openAbout()
	(&about).updateKeymap()
	if about.keys.Help.Enabled() {
		t.Fatal("About screen must not advertise or accept Help")
	}
}

func helpContextModels(t *testing.T) map[string]appModel {
	focused := newApp(stubFetch(t), colorprofile.NoTTY)

	start := newApp(stubFetch(t), colorprofile.NoTTY)
	start.blurInput()

	noLinks := settledReader(t, Entry{
		Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n"),
	})

	linked := readerWithFocusedLink(t, stubFetch(t), Link{
		Kind: LinkFinger, Action: ActionDrill, Raw: "alice@tilde.team",
		Target: hostTarget(t, "alice@tilde.team"),
	})

	raw := settledReader(t, Entry{
		Target: hostTarget(t, "alice@plan.cat"), Body: []byte("Plan\n"),
	})
	raw.enterRaw()

	panel := linksPanelModel(t, stubFetch(t), []Link{
		{Kind: LinkURL, Action: ActionCopy, Raw: "https://example.com"},
	})
	linkTarget := hostTarget(t, "alice@tilde.team")
	ambiguousPanel := linksPanelModel(t, stubFetch(t), []Link{{
		Kind: LinkFinger, Action: ActionCopy, Raw: linkTarget.Raw,
		Target: linkTarget, Ambiguous: true,
	}})
	definitePanel := linksPanelModel(t, stubFetch(t), []Link{{
		Kind: LinkFinger, Action: ActionDrill, Raw: linkTarget.Raw,
		Target: linkTarget,
	}})

	return map[string]appModel{
		"focused input":    focused,
		"start content":    start,
		"user list":        settledList(t),
		"reader no links":  noLinks,
		"reader with link": linked,
		"raw view":         raw,
		"links URL":        panel,
		"links ambiguous":  ambiguousPanel,
		"links definite":   definitePanel,
	}
}

func TestHelpLayoutsByContext(t *testing.T) {
	wants := map[string]string{
		"focused input":    "↵,esc,↓,a",
		"start content":    "↑/↓,↵,esc,i,←/→,/,b,a,q",
		"user list":        "↑/↓,↵,esc,i,h,←/→,/,v,r,y,b,a,q",
		"reader no links":  "↑/↓,esc,i,h,←/→,v,r,y,b,a,q",
		"reader with link": "↑/↓,↵,esc,i,h,←/→,tab,shift+tab,v,r,y,b,L,a,q",
		"raw view":         "↑/↓,esc,i,h,←/→,v,y,b,a,q",
		"links URL":        "↑/↓,esc,y,/,a",
		"links ambiguous":  "↑/↓,esc,f,y,/,a",
		"links definite":   "↑/↓,esc,↵,y,/,a",
	}
	for name, m := range helpContextModels(t) {
		t.Run(name, func(t *testing.T) {
			m.common.width, m.common.height = 200, 40
			m.openHelp()
			(&m).updateKeymap()
			got := strings.Join(helpKeys(m.helpLayout().bindings), ",")
			if got != wants[name] {
				t.Fatalf("Help bindings = %q, want %q", got, wants[name])
			}
		})
	}
}

// TestHelpMoveLabelMatchesTheStatusBar pins ↑/↓'s wording to what the keys
// actually do in each view. In a list they move a selection and the viewport
// follows; in the reader there is no cursor and nothing moves but the text.
//
// app.go's status bar already draws this line — its list hints say "move"
// (app.go:1432, :1434) and its reader hints say "scroll" (app.go:1485) — and
// over a landed reader the bar and the help overlay are on screen at the same
// time, so the two must agree rather than label one gesture two ways.
func TestHelpMoveLabelMatchesTheStatusBar(t *testing.T) {
	wants := map[string]string{
		"start content":    "move",
		"user list":        "move",
		"reader no links":  "scroll",
		"reader with link": "scroll",
		"raw view":         "scroll",
		"links URL":        "move",
		"links ambiguous":  "move",
		"links definite":   "move",
	}
	for name, m := range helpContextModels(t) {
		want, ok := wants[name]
		if !ok {
			continue // focused input never offers ↑/↓
		}
		t.Run(name, func(t *testing.T) {
			m.common.width, m.common.height = 200, 40
			m.openHelp()
			(&m).updateKeymap()
			for _, binding := range m.helpLayout().bindings {
				if binding.Help().Key != "↑/↓" {
					continue
				}
				if got := binding.Help().Desc; got != want {
					t.Fatalf("↑/↓ label = %q, want %q", got, want)
				}
				return
			}
			t.Fatal("no ↑/↓ binding in the help layout")
		})
	}
}

func TestHelpAdmissionMatchesRetainedBindingsAcrossContexts(t *testing.T) {
	messages := []tea.KeyPressMsg{
		{Code: tea.KeyEnter},
		{Code: 'i', Text: "i"},
		{Code: 'j', Text: "j"},
		{Code: tea.KeyRight},
		{Code: '/', Text: "/"},
		{Code: tea.KeyTab},
		{Code: 'N', Text: "N"},
		{Code: 'v', Text: "v"},
		{Code: 'r', Text: "r"},
		{Code: 'y', Text: "y"},
		{Code: 'b', Text: "b"},
		{Code: 'h', Text: "h"},
		{Code: 'L', Text: "L"},
		{Code: 'a', Text: "a"},
		{Code: 'q', Text: "q"},
		{Code: 'f', Text: "f"},
		{Code: 'g', Text: "g"},
		{Code: 'x', Text: "x"},
	}

	for name := range helpContextModels(t) {
		t.Run(name, func(t *testing.T) {
			for _, msg := range messages {
				clone := helpContextModels(t)[name]
				clone.common.width, clone.common.height = 200, 40
				clone.openHelp()
				(&clone).updateKeymap()
				retained := clone.helpLayout().matches(msg)
				next, cmd := clone.Update(msg)
				got := next.(appModel)
				if !retained {
					if !got.help || cmd != nil {
						t.Errorf("unmatched key %v = help %v cmd %T", msg, got.help, cmd)
					}
					continue
				}
				if key.Matches(msg, clone.keys.About) {
					if got.help || got.state != stateAbout || cmd != nil {
						t.Errorf("About key %v = help %v state %d cmd %T", msg, got.help, got.state, cmd)
					}
					continue
				}
				if got.help || cmd == nil {
					t.Errorf("retained key %v = help %v cmd %T", msg, got.help, cmd)
					continue
				}
				replay, ok := cmd().(tea.KeyPressMsg)
				if !ok || replay != msg {
					t.Errorf("retained key %v replay = %#v", msg, replay)
				}
			}
		})
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

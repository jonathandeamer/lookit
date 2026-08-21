package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

func wrapKey() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'w', Text: "w"} }

func TestWrapKeyBinding(t *testing.T) {
	binding := newKeyMap().Wrap
	if !contains(binding.Keys(), "w") {
		t.Fatalf("Wrap keys = %v, want w", binding.Keys())
	}
	if got, want := binding.Help(), (key.Help{Key: "w", Desc: "wrap"}); got != want {
		t.Fatalf("Wrap help = %+v, want %+v", got, want)
	}
}

func TestWrapKeyAvailability(t *testing.T) {
	body := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("a body\n")}
	partial := body
	partial.Err = errors.New("read stopped")

	tests := []struct {
		name string
		make func() appModel
		want bool
	}{
		{"reader", func() appModel { return settledReader(t, body) }, true},
		{"partial body", func() appModel { return settledReader(t, partial) }, true},
		{"empty success", func() appModel { return settledReader(t, Entry{Target: body.Target}) }, false},
		{"empty failure", func() appModel {
			return settledReader(t, Entry{Target: body.Target, Err: errors.New("failed")})
		}, false},
		{"focused input", func() appModel {
			m := settledReader(t, body)
			m.inputFocused = true
			m.input.Focus()
			return m
		}, false},
		{"raw", func() appModel {
			m := settledReader(t, body)
			m.enterRaw()
			return m
		}, false},
		{"pending", func() appModel {
			m := settledReader(t, body)
			m.pending = &pendingRequest{cancel: func() {}}
			return m
		}, false},
		{"links panel", func() appModel {
			return linksPanelModel(t, stubFetch(t), []Link{{Raw: "bob@plan.cat"}})
		}, false},
		{"list", func() appModel { return settledList(t) }, false},
		{"start", func() appModel {
			m := newApp(stubFetch(t), colorprofile.NoTTY)
			m.blurInput()
			return m
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.make()
			m.updateKeymap()
			if got := m.keys.Wrap.Enabled(); got != tt.want {
				t.Fatalf("Wrap enabled = %v, want %v", got, tt.want)
			}
		})
	}

	outOfRange := settledReader(t, body)
	outOfRange.pos = len(outOfRange.history)
	outOfRange.updateKeymap()
	if outOfRange.keys.Wrap.Enabled() {
		t.Fatal("Wrap enabled with pos == len(history)")
	}
}

func TestWrapTypesLiterallyInFocusedInput(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	next, _ := m.Update(wrapKey())
	got := next.(appModel)
	if got.input.Value() != "w" {
		t.Fatalf("input value = %q, want literal w", got.input.Value())
	}
}

func TestWrapToggleUpdatesCurrentNodeAndFlash(t *testing.T) {
	entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("one two three four five six seven eight\n")}
	m := settledReader(t, entry)
	m.reader.setSize(12, 4)

	next, cmd := m.Update(wrapKey())
	wrapped := next.(appModel)
	if !wrapped.history[0].wrapped || !wrapped.reader.wrapped {
		t.Fatalf("wrapped state = node %v reader %v", wrapped.history[0].wrapped, wrapped.reader.wrapped)
	}
	if wrapped.flash != "wrapping on" || cmd == nil {
		t.Fatalf("wrap flash/cmd = %q %#v", wrapped.flash, cmd)
	}
	if !strings.Contains(wrapped.reader.layout.Text, "\n") || wrapped.reader.layout.BodyLineCount != 1 {
		t.Fatalf("reader was not reflowed: %#v", wrapped.reader.layout)
	}

	next, _ = wrapped.Update(wrapKey())
	unwrapped := next.(appModel)
	if unwrapped.history[0].wrapped || unwrapped.reader.wrapped || unwrapped.flash != "original layout" {
		t.Fatalf("unwrap state = node %v reader %v flash %q", unwrapped.history[0].wrapped, unwrapped.reader.wrapped, unwrapped.flash)
	}
	next, _ = unwrapped.Update(clearFlashMsg{})
	unwrapped = next.(appModel)
	if got := unwrapped.flash; got != "" {
		t.Fatalf("flash after clear = %q", got)
	}
	if got := ansi.Strip(unwrapped.statusBarModel().render()); strings.Contains(got, "wrapp") {
		t.Fatalf("status bar persistently exposes wrapping state: %q", got)
	}
}

func TestWrapDoesNotMutateResponseOrLinkMetadata(t *testing.T) {
	body := []byte("Profile: https://example.com/an/extremely/long/path\n")
	entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: body}
	m := settledReader(t, entry)
	wantBody := append([]byte(nil), m.history[0].entry.Body...)
	wantLinks := append([]Link(nil), m.history[0].links...)

	next, _ := m.Update(wrapKey())
	m = next.(appModel)
	if !slices.Equal(m.history[0].entry.Body, wantBody) {
		t.Fatalf("response body changed: got %q want %q", m.history[0].entry.Body, wantBody)
	}
	if !slices.Equal(m.history[0].links, wantLinks) {
		t.Fatalf("link metadata changed: got %#v want %#v", m.history[0].links, wantLinks)
	}
}

func TestLinksPanelRoundTripPreservesWrapChoice(t *testing.T) {
	entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte("bob@plan.cat\n")}
	m := settledReader(t, entry)
	next, _ := m.Update(wrapKey())
	m = next.(appModel)

	next, _ = m.Update(tea.KeyPressMsg{Code: 'L', Text: "L"})
	m = next.(appModel)
	if !m.showingLinks || !m.history[0].wrapped {
		t.Fatalf("links panel entry lost wrapping: panel=%v wrapped=%v", m.showingLinks, m.history[0].wrapped)
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(appModel)
	if m.showingLinks || !m.reader.wrapped || !m.history[0].wrapped {
		t.Fatalf("links panel exit lost wrapping: panel=%v reader=%v node=%v", m.showingLinks, m.reader.wrapped, m.history[0].wrapped)
	}
}

func TestWrapChoiceIsPerHistoryNodeAndNewResponsesDefaultOff(t *testing.T) {
	first := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte(
		strings.Repeat("first ", 20) + "\n" +
			strings.Repeat("second ", 20) + "\n" +
			"third source line\n",
	)}
	m := settledReader(t, first)
	m.reader.setSize(18, 3)
	next, _ := m.Update(wrapKey())
	m = next.(appModel)
	m.reader.viewport.SetYOffset(m.reader.layout.DisplayLineFor(1))

	second := Entry{Target: hostTarget(t, "bob@plan.cat"), Body: []byte(strings.Repeat("second ", 20) + "\n")}
	m = deliverNavigation(m, second)
	if m.history[1].wrapped || m.reader.wrapped {
		t.Fatal("new response inherited wrapping")
	}

	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(appModel)
	if m.pos != 0 || !m.history[0].wrapped || !m.reader.wrapped {
		t.Fatalf("Back did not restore wrapped node: pos=%d node=%v reader=%v", m.pos, m.history[0].wrapped, m.reader.wrapped)
	}
	if got := m.reader.topLogicalLine(); got != 1 {
		t.Fatalf("Back restored logical source line %d, want 1", got)
	}
}

func TestRawViewResizePreservesSourceAndWrapChoice(t *testing.T) {
	body := []byte("Pronouns: they/them\n" + strings.Repeat("source ", 20) + "\n")
	entry := Entry{Target: hostTarget(t, "alice@tilde.team"), Body: body}
	m := settledReader(t, entry)
	next, _ := m.Update(wrapKey())
	m = next.(appModel)
	next, _ = m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = next.(appModel)
	if !m.showingRaw || m.reader.viewport.GetContent() != string(body) {
		t.Fatalf("source entry = raw %v content %q", m.showingRaw, m.reader.viewport.GetContent())
	}

	next, _ = m.Update(tea.WindowSizeMsg{Width: 45, Height: 20})
	m = next.(appModel)
	if m.reader.viewport.GetContent() != string(body) {
		t.Fatalf("resize replaced source: %q", m.reader.viewport.GetContent())
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(appModel)
	if m.showingRaw || !m.reader.wrapped || !m.history[0].wrapped {
		t.Fatalf("source exit lost wrapping: raw=%v reader=%v node=%v", m.showingRaw, m.reader.wrapped, m.history[0].wrapped)
	}
}

func TestWrapHelpOrderLabelsReplayAndLegacyRetention(t *testing.T) {
	entry := Entry{Target: hostTarget(t, "alice@plan.cat"), Body: []byte(strings.Repeat("word ", 30) + "\n")}
	geometries := []struct {
		name   string
		width  int
		height int
		dark   bool
	}{
		{"80 dark", 80, 24, true},
		{"80 light", 80, 24, false},
		{"100", 100, 30, true},
		{"60", 60, 20, true},
		{"100 tall", 100, 50, true},
		{"45", 45, 24, true},
	}
	for _, geometry := range geometries {
		t.Run(geometry.name, func(t *testing.T) {
			var memberships [][]string
			for _, wrapped := range []bool{false, true} {
				m := settledReader(t, entry)
				m.common.width, m.common.height = geometry.width, geometry.height
				m.common.darkBackground = geometry.dark
				m.common.styles = newStyles(geometry.dark)
				m.history[0].wrapped = wrapped
				m.reader.wrapped = wrapped
				m.openHelp()
				m.updateKeymap()

				wantDesc := "wrap"
				if wrapped {
					wantDesc = "unwrap"
				}
				if got := m.keys.Wrap.Help().Desc; got != wantDesc {
					t.Fatalf("Wrap description = %q, want %q", got, wantDesc)
				}
				candidates := m.helpCandidates()
				baselineCandidates := slices.DeleteFunc(
					append([]key.Binding(nil), candidates...),
					func(binding key.Binding) bool { return binding.Help().Key == "w" },
				)
				baseline := helpKeys(layoutHelp(
					baselineCandidates, m.common.styles,
					m.common.width, m.common.height-1,
				).bindings)
				got := helpKeys(m.helpLayout().bindings)
				if !slices.Contains(got, "w") {
					t.Fatalf("Wrap not retained: %v", got)
				}
				for _, legacyKey := range baseline {
					if !slices.Contains(got, legacyKey) {
						t.Errorf("adding Wrap evicted %q: baseline=%v got=%v", legacyKey, baseline, got)
					}
				}
				memberships = append(memberships, got)
			}
			if !slices.Equal(memberships[0], memberships[1]) {
				t.Fatalf("wrap label changed retained membership: %v vs %v", memberships[0], memberships[1])
			}
			joined := strings.Join(memberships[0], ",")
			if !strings.Contains(joined, "v,w,r") {
				t.Fatalf("reader display actions not adjacent: %s", joined)
			}
		})
	}

	m := settledReader(t, entry)
	m.common.width, m.common.height = 80, 24
	m.openHelp()
	next, cmd := m.Update(wrapKey())
	m = next.(appModel)
	if m.help || cmd == nil {
		t.Fatalf("retained Wrap did not close/replay Help: help=%v cmd=%#v", m.help, cmd)
	}
	replayed, ok := cmd().(tea.KeyPressMsg)
	if !ok {
		t.Fatalf("replay command returned %T", cmd())
	}
	next, _ = m.Update(replayed)
	if !next.(appModel).history[0].wrapped {
		t.Fatal("replayed Wrap did not toggle node")
	}

	clipped := settledReader(t, entry)
	clipped.common.width, clipped.common.height = 45, 2
	clipped.openHelp()
	next, _ = clipped.Update(wrapKey())
	clipped = next.(appModel)
	if !clipped.help || clipped.history[0].wrapped {
		t.Fatalf("clipped Wrap escaped Help gate: help=%v wrapped=%v", clipped.help, clipped.history[0].wrapped)
	}
}

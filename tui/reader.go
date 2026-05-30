package tui

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
	"github.com/jonathandeamer/lookit/render"
)

const initialStatus = "enter a finger target, then press Enter"

// chromeRows counts the non-viewport lines in the reader view: title + input.
// (The status and hint lines moved to appModel's bottom bar.)
const chromeRows = 2

// readerModel is the query reader: a target input, a status line, and a
// scrollable viewport showing one rendered finger response. It owns typing,
// scrolling, and starting a fetch on Enter. It does NOT route fetch results,
// quit, or declare terminal modes — appModel owns those.
type readerModel struct {
	input    textinput.Model
	viewport viewport.Model
	current  *Entry
	loading  bool
	status   string
	profile  colorprofile.Profile
	fetch    FetchFunc
	styles   styles
	width    int
	height   int
}

func newReader(fetch FetchFunc, profile colorprofile.Profile) readerModel {
	if fetch == nil {
		fetch = defaultFetch
	}

	input := textinput.New()
	input.Placeholder = "alice@plan.cat"
	input.Prompt = "target: "
	input.Focus()
	input.CharLimit = 256
	input.SetWidth(40)

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(10))
	vp.SetContent("No response yet.")

	return readerModel{
		input:    input,
		viewport: vp,
		status:   initialStatus,
		profile:  profile,
		fetch:    fetch,
		styles:   newStyles(),
	}
}

// Init returns the input blink command.
func (m readerModel) Init() tea.Cmd {
	return textinput.Blink
}

// update handles reader-local messages: typing, scrolling, and Enter to start
// a fetch. Enter returns a fetch command (the result is routed by appModel).
// It never quits and never handles fetchResultMsg.
func (m readerModel) update(msg tea.Msg) (readerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.Key()
		if key.Code == tea.KeyEnter {
			if m.loading {
				return m, nil
			}
			target, err := finger.ParseTarget(strings.TrimSpace(m.input.Value()))
			if err != nil {
				m.status = "error: " + err.Error()
				return m, nil
			}
			m.setLoading(target)
			return m, fetchCmd(context.Background(), m.fetch, target)
		}
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// View renders the reader as a plain string. appModel wraps it in a tea.View.
func (m readerModel) View() string {
	var b strings.Builder
	b.WriteString(m.styles.title.Render("lookit"))
	b.WriteByte('\n')
	b.WriteString(m.input.View())
	b.WriteByte('\n')
	b.WriteString(m.viewport.View())
	return b.String()
}

// setSize lays out the input and viewport for the given terminal size.
func (m *readerModel) setSize(width, height int) {
	m.width = width
	m.height = height
	if width <= 0 || height <= 0 {
		return
	}
	inputWidth := width - len(m.input.Prompt)
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.input.SetWidth(inputWidth)
	m.viewport.SetWidth(width)
	vh := height - chromeRows
	if vh < 1 {
		vh = 1
	}
	m.viewport.SetHeight(vh)
}

// setProfile updates the color profile and re-renders the current entry.
func (m *readerModel) setProfile(p colorprofile.Profile) {
	m.profile = p
	if m.current != nil {
		m.viewport.SetContent(renderEntry(m.profile, *m.current))
	}
}

// setLoading marks a fetch in progress for the given target.
func (m *readerModel) setLoading(target finger.Target) {
	m.loading = true
	m.status = "loading " + target.Raw + "..."
}

// setEntry displays a fetched result, clearing the loading state.
func (m *readerModel) setEntry(entry Entry) {
	m.loading = false
	m.current = &entry
	m.status = statusForEntry(entry)
	m.viewport.SetContent(renderEntry(m.profile, entry))
}

func statusForEntry(entry Entry) string {
	if entry.Err != nil {
		return "error: " + entry.Err.Error()
	}
	if entry.Meta.Truncated {
		return "loaded " + entry.Target.Raw + " (truncated)"
	}
	return "loaded " + entry.Target.Raw
}

func renderEntry(profile colorprofile.Profile, entry Entry) string {
	return render.Render(entry.Target, entry.Body, entry.Meta, entry.Err, profile)
}

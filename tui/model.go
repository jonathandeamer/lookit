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

type Model struct {
	input    textinput.Model
	viewport viewport.Model
	current  *Entry
	loading  bool
	status   string
	profile  colorprofile.Profile
	fetch    FetchFunc
	ready    bool
	width    int
	height   int
	styles   styles
}

func New(fetch FetchFunc, profile colorprofile.Profile) Model {
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

	return Model{
		input:    input,
		viewport: vp,
		status:   initialStatus,
		profile:  profile,
		fetch:    fetch,
		styles:   newStyles(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		tea.RequestCapability("RGB"),
		tea.RequestCapability("Tc"),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.Key()
		switch {
		case key.Code == tea.KeyEsc || (key.Code == 'c' && key.Mod == tea.ModCtrl):
			return m, tea.Quit
		case key.Code == tea.KeyEnter:
			if m.loading {
				return m, nil
			}
			target, err := finger.ParseTarget(strings.TrimSpace(m.input.Value()))
			if err != nil {
				m.status = "error: " + err.Error()
				return m, nil
			}
			m.loading = true
			m.status = "loading " + target.Raw + "..."
			return m, fetchCmd(context.Background(), m.fetch, target)
		}
	case tea.WindowSizeMsg:
		m.ready = true
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
	case tea.ColorProfileMsg:
		m.profile = msg.Profile
		if m.current != nil {
			m.viewport.SetContent(renderEntry(m.profile, *m.current))
		}
	case fetchResultMsg:
		m.loading = false
		m.current = &msg.entry
		m.status = statusForEntry(msg.entry)
		m.viewport.SetContent(renderEntry(m.profile, msg.entry))
		return m, nil
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m Model) View() tea.View {
	var b strings.Builder
	b.WriteString(m.styles.title.Render("lookit"))
	b.WriteByte('\n')
	b.WriteString(m.input.View())
	b.WriteByte('\n')
	if strings.HasPrefix(m.status, "error:") {
		b.WriteString(m.styles.error.Render(m.status))
	} else {
		b.WriteString(m.styles.status.Render(m.status))
	}
	b.WriteByte('\n')
	b.WriteString(m.viewport.View())
	b.WriteByte('\n')
	b.WriteString(m.styles.hint.Render("Enter fetches - arrows/PageUp/PageDown scroll - Esc quits"))

	v := tea.NewView(b.String())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *Model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	inputWidth := m.width - len(m.input.Prompt)
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.input.SetWidth(inputWidth)
	m.viewport.SetWidth(m.width)

	viewportHeight := m.height - 5
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	m.viewport.SetHeight(viewportHeight)
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

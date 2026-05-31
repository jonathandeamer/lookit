package tui

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/render"
)

// chromeRows counts the reader's own non-viewport lines. The reader is now
// viewport-only; the input and status moved to appModel (top bar / status bar).
const chromeRows = 0

// readerModel shows one rendered finger response in a scrollable viewport. It
// owns scrolling only; appModel owns the input, fetch, quit, and chrome.
type readerModel struct {
	viewport viewport.Model
	current  *Entry
	profile  colorprofile.Profile
	styles   styles
	width    int
	height   int
}

func newReader(profile colorprofile.Profile) readerModel {
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(10))
	vp.SetContent("No response yet.")
	return readerModel{viewport: vp, profile: profile, styles: newStyles()}
}

// Init is a no-op (the input's blink command now lives in appModel.Init).
func (m readerModel) Init() tea.Cmd { return nil }

// update forwards scroll messages to the viewport.
func (m readerModel) update(msg tea.Msg) (readerModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders just the viewport.
func (m readerModel) View() string { return m.viewport.View() }

func (m *readerModel) setSize(width, height int) {
	m.width = width
	m.height = height
	if width <= 0 || height <= 0 {
		return
	}
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

// setEntry displays a fetched result.
func (m *readerModel) setEntry(entry Entry) {
	m.current = &entry
	m.viewport.SetContent(renderEntry(m.profile, entry))
}

// setRaw shows the unprocessed response body as plain text ("view source"),
// bypassing render's chrome and field highlighting.
func (m *readerModel) setRaw(body []byte) {
	m.viewport.SetContent(string(body))
}

func renderEntry(profile colorprofile.Profile, entry Entry) string {
	return render.Render(entry.Target, entry.Body, entry.Meta, entry.Err, profile)
}

package tui

import (
	"strings"

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
	viewport       viewport.Model
	current        *Entry
	profile        colorprofile.Profile
	darkBackground bool
	styles         styles
	width          int
	height         int
	links          []Link // detected links in document order
	focusedLink    int    // index into links; -1 = none focused
}

func newReader(profile colorprofile.Profile) readerModel {
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(10))
	vp.SetContent("No response yet.")
	return readerModel{viewport: vp, profile: profile, darkBackground: true, styles: newStyles(true), focusedLink: -1}
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
		m.viewport.SetContent(render.RenderWithBackground(m.current.Target, m.current.Body, m.current.Meta, m.current.Err, m.profile, m.darkBackground))
	}
}

func (m *readerModel) setBackground(dark bool) {
	m.darkBackground = dark
	if m.current != nil {
		m.viewport.SetContent(render.RenderWithBackground(m.current.Target, m.current.Body, m.current.Meta, m.current.Err, m.profile, m.darkBackground))
	}
}

// setEntry displays a fetched result.
func (m *readerModel) setEntry(entry Entry) {
	m.current = &entry
	m.viewport.SetContent(renderEntry(m.profile, m.darkBackground, entry))
}

// setEntryWithLinks displays a fetched result and applies the link overlay
// (focus highlight + OSC-8 hyperlinks) to the body portion of the rendered
// output. links is the DetectLinks result for this entry; focusedLink is the
// current focused index (-1 = none).
func (m *readerModel) setEntryWithLinks(entry Entry, links []Link) {
	m.current = &entry
	m.links = links
	rendered := render.RenderWithBackground(entry.Target, entry.Body, entry.Meta, entry.Err, m.profile, m.darkBackground)
	header, body := render.Split(rendered)
	body = applyLinkOverlay(body, links, m.focusedLink, m.styles)
	m.viewport.SetContent(header + body)
	m.scrollToFocusedLink(header, links)
}

// scrollToFocusedLink scrolls the viewport so the focused link is roughly
// centred vertically. It works by counting newlines in the body bytes before
// the link's raw token and adding the header line count.
func (m *readerModel) scrollToFocusedLink(header string, links []Link) {
	if m.focusedLink < 0 || m.focusedLink >= len(links) || m.current == nil {
		return
	}
	raw := links[m.focusedLink].Raw
	bodyText := string(m.current.Body)
	pos := strings.Index(bodyText, raw)
	if pos < 0 {
		return
	}
	bodyLine := strings.Count(bodyText[:pos], "\n")
	headerLines := strings.Count(header, "\n")
	targetLine := headerLines + bodyLine
	offset := targetLine - m.viewport.Height()/2
	if offset < 0 {
		offset = 0
	}
	m.viewport.SetYOffset(offset)
}

// nextLink advances the focused link index by one (wrapping).
func (m *readerModel) nextLink(count int) {
	if count == 0 {
		return
	}
	if m.focusedLink < 0 {
		m.focusedLink = 0
		return
	}
	m.focusedLink = (m.focusedLink + 1) % count
}

// prevLink moves the focused link index back by one (wrapping).
func (m *readerModel) prevLink(count int) {
	if count == 0 {
		return
	}
	if m.focusedLink <= 0 {
		m.focusedLink = count - 1
		return
	}
	m.focusedLink--
}

// setRaw shows the unprocessed response body as plain text ("view source"),
// bypassing render's chrome and field highlighting.
func (m *readerModel) setRaw(body []byte) {
	m.viewport.SetContent(string(body))
}

func renderEntry(profile colorprofile.Profile, darkBackground bool, entry Entry) string {
	// The status bar pins the byte count and truncation flag and the header
	// carries the elapsed time, so render itself is footerless.
	return render.RenderWithBackground(entry.Target, entry.Body, entry.Meta, entry.Err, profile, darkBackground)
}

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
	// Error lines wrap at the viewport width, so a resize must re-render.
	if m.current != nil {
		m.viewport.SetContent(m.render(*m.current))
	}
}

// render renders one entry at the reader's current width, so a long error line
// wraps instead of being clipped off the right edge.
func (m readerModel) render(entry Entry) string {
	return render.RenderWithWidth(entry.Target, entry.Body, entry.Err, m.profile, m.darkBackground, m.width)
}

// setProfile updates the color profile and re-renders the current entry.
func (m *readerModel) setProfile(p colorprofile.Profile) {
	m.profile = p
	if m.current != nil {
		m.viewport.SetContent(m.render(*m.current))
	}
}

func (m *readerModel) setBackground(dark bool) {
	m.darkBackground = dark
	if m.current != nil {
		m.viewport.SetContent(m.render(*m.current))
	}
}

// setEntry displays a fetched result.
func (m *readerModel) setEntry(entry Entry) {
	m.current = &entry
	m.viewport.SetContent(m.render(entry))
}

// setEntryWithLinks displays a fetched result and applies the link overlay
// (focus highlight + OSC-8 hyperlinks) to the complete rendered response.
// links is the DetectLinks result for this entry; focusedLink is the current
// focused index (-1 = none).
func (m *readerModel) setEntryWithLinks(entry Entry, links []Link) {
	m.current = &entry
	m.links = links
	rendered := m.render(entry)
	rendered = applyLinkOverlay(rendered, links, m.focusedLink, m.styles)
	m.viewport.SetContent(rendered)
	m.scrollToFocusedLink(links)
}

// scrollToFocusedLink scrolls the viewport so the focused link is roughly
// centred vertically. Resolve links against the rendered response in document
// order, matching the overlay pass, so repeated tokens and renderer-inserted
// lines map to the occurrence that is actually focused.
func (m *readerModel) scrollToFocusedLink(links []Link) {
	if m.focusedLink < 0 || m.focusedLink >= len(links) || m.current == nil {
		return
	}
	rendered := m.render(*m.current)
	remaining := rendered
	consumed := 0
	pos := -1
	for i, link := range links[:m.focusedLink+1] {
		rel := strings.Index(remaining, link.Raw)
		if rel < 0 {
			if i == m.focusedLink {
				return
			}
			continue
		}
		pos = consumed + rel
		consumed = pos + len(link.Raw)
		remaining = rendered[consumed:]
	}
	bodyLine := strings.Count(rendered[:pos], "\n")
	offset := bodyLine - m.viewport.Height()/2
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
// bypassing render's field highlighting and error treatment.
func (m *readerModel) setRaw(body []byte) {
	m.viewport.SetContent(string(body))
}

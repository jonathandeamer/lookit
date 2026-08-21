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
	layout         render.Layout
	wrapped        bool
	raw            bool
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
	oldWidth := m.width
	widthChanged := width != oldWidth
	position := m.position()
	m.width, m.height = width, height
	if width <= 0 || height <= 0 {
		return
	}
	m.viewport.SetWidth(width)
	vh := height - chromeRows
	if vh < 1 {
		vh = 1
	}
	m.viewport.SetHeight(vh)
	if !widthChanged {
		return
	}
	m.viewport.SetXOffset(0)
	if m.current != nil && !m.raw {
		m.setEntryWithLinks(*m.current, m.links, m.wrapped, position)
	}
}

// render renders one entry at the reader's current width, so a long error line
// wraps instead of being clipped off the right edge.
func (m readerModel) render(entry Entry) render.Layout {
	return render.RenderLayout(
		entry.Target, entry.Body, entry.Err,
		m.profile, m.darkBackground,
		render.LayoutOptions{
			ErrorWidth: m.width,
			BodyWidth:  bodyWrapWidth(m.width, m.wrapped),
		},
	)
}

// setProfile updates the color profile and re-renders the current entry.
func (m *readerModel) setProfile(p colorprofile.Profile) {
	m.profile = p
	if m.current != nil && !m.raw {
		m.setEntryWithLinks(*m.current, m.links, m.wrapped, m.position())
	}
}

func (m *readerModel) setBackground(dark bool) {
	m.darkBackground = dark
	if m.current != nil && !m.raw {
		m.setEntryWithLinks(*m.current, m.links, m.wrapped, m.position())
	}
}

// setEntry displays a fetched result.
func (m *readerModel) setEntry(entry Entry) {
	m.setEntryWithLinks(entry, nil, false, readerPosition{})
}

type readerPosition struct {
	logicalLine int
	hasLogical  bool
	fallbackY   int
}

func bodyWrapWidth(viewportWidth int, wrapped bool) int {
	if !wrapped || viewportWidth <= 0 {
		return 0
	}
	return min(viewportWidth, 80)
}

func (m readerModel) topLogicalLine() int {
	logicalLine := m.layout.LogicalLineAt(m.viewport.YOffset())
	if logicalLine == render.NoBodyLine && m.layout.BodyLineCount > 0 {
		return m.layout.BodyLineCount - 1
	}
	return logicalLine
}

func (m readerModel) position() readerPosition {
	logicalLine := m.topLogicalLine()
	return readerPosition{
		logicalLine: logicalLine,
		hasLogical:  logicalLine != render.NoBodyLine,
		fallbackY:   m.viewport.YOffset(),
	}
}

func (m *readerModel) restoreLogicalLine(logicalLine int) {
	m.viewport.SetYOffset(m.layout.DisplayLineFor(logicalLine))
}

func (m *readerModel) setRenderedContent(layout render.Layout) {
	m.layout = layout
	text := applyLinkOverlay(layout.Text, m.links, m.focusedLink, m.styles)
	m.viewport.SetContent(text)
}

func (m *readerModel) positionAfterRender(position readerPosition) {
	switch {
	case m.focusedLink >= 0:
		m.scrollToFocusedLink(m.links)
	case position.hasLogical:
		m.restoreLogicalLine(position.logicalLine)
	default:
		m.viewport.SetYOffset(position.fallbackY)
	}
}

// setEntryWithLinks displays a fetched result and applies the link overlay
// (focus highlight + OSC-8 hyperlinks) to the complete rendered response.
// links is the DetectLinks result for this entry; focusedLink is the current
// focused index (-1 = none).
func (m *readerModel) setEntryWithLinks(entry Entry, links []Link, wrapped bool, position readerPosition) {
	m.current = &entry
	m.links = links
	m.wrapped = wrapped
	m.raw = false
	m.setRenderedContent(m.render(entry))
	m.positionAfterRender(position)
}

func (m *readerModel) setWrapped(wrapped bool) {
	position := m.position()
	m.wrapped = wrapped
	if m.current != nil && !m.raw {
		m.setEntryWithLinks(*m.current, m.links, wrapped, position)
	}
	m.viewport.SetXOffset(0)
}

// scrollToFocusedLink scrolls the viewport so the focused link is roughly
// centred vertically. Resolve links against the rendered response in document
// order, matching the overlay pass, so repeated tokens and renderer-inserted
// lines map to the occurrence that is actually focused.
func (m *readerModel) scrollToFocusedLink(links []Link) {
	if m.focusedLink < 0 || m.focusedLink >= len(links) || m.current == nil {
		return
	}
	rendered := m.layout.Text
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
	m.raw = true
	m.layout = render.Layout{}
	m.viewport.SetContent(string(body))
}

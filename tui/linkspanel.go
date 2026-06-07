package tui

import (
	"fmt"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

// linkItem is one row in the links panel list.
type linkItem struct {
	link Link
}

func (i linkItem) FilterValue() string { return i.link.Raw }
func (i linkItem) Title() string       { return i.link.Raw }
func (i linkItem) Description() string { return linkKindLabel(i.link) }

// linksPanel wraps a bubbles list populated from []Link.
type linksPanel struct {
	list   list.Model
	common *commonModel
	links  []Link
}

func newLinksPanel(common *commonModel, links []Link) linksPanel {
	items := make([]list.Item, len(links))
	for i, l := range links {
		items[i] = linkItem{link: l}
	}
	st := common.ensureStyles()
	h := common.bodyHeight()
	if h < 1 {
		h = 1
	}
	// Build with a temporary delegate; applyListStyles will overwrite it with
	// defaultUserDelegate, so we re-set after.
	d := list.NewDefaultDelegate()
	d.Styles = st.listItem
	d.SetSpacing(0)
	l := list.New(items, d, common.width, h)
	applyListStyles(&l, st)
	// Re-apply the link delegate (applyListStyles overwrites with defaultUserDelegate).
	ld := list.NewDefaultDelegate()
	ld.Styles = st.listItem
	ld.SetSpacing(0)
	l.SetDelegate(ld)
	l.Title = fmt.Sprintf("%d links", len(links))
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	return linksPanel{list: l, common: common, links: links}
}

func (p *linksPanel) setSize(w, h int) {
	p.list.SetSize(w, h)
}

func (p linksPanel) View() string { return p.list.View() }

func (p linksPanel) update(msg tea.Msg) (linksPanel, tea.Cmd) {
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p linksPanel) selected() (Link, bool) {
	it, ok := p.list.SelectedItem().(linkItem)
	if !ok {
		return Link{}, false
	}
	return it.link, true
}

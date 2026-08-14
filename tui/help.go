package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const fullHelpSeparator = "    "

type helpLayout struct {
	bindings []key.Binding
	columns  [][]key.Binding
}

func layoutHelp(candidates []key.Binding, st styles, width, height int) helpLayout {
	bindings := enabledHelpBindings(candidates)
	if len(bindings) == 0 || height < 1 {
		return helpLayout{}
	}
	maxColumns := min(3, len(bindings))
	for columnCount := maxColumns; columnCount >= 1; columnCount-- {
		retainedCount := min(len(bindings), height*columnCount)
		retained := append([]key.Binding(nil), bindings[:retainedCount]...)
		columns := partitionHelpBindings(retained, columnCount)
		if width <= 0 || columnCount == 1 || helpColumnsWidth(columns, st) <= width {
			return helpLayout{bindings: retained, columns: columns}
		}
	}
	return helpLayout{}
}

func enabledHelpBindings(bindings []key.Binding) []key.Binding {
	out := make([]key.Binding, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Enabled() {
			out = append(out, binding)
		}
	}
	return out
}

func partitionHelpBindings(bindings []key.Binding, columnCount int) [][]key.Binding {
	if len(bindings) == 0 || columnCount < 1 {
		return nil
	}
	rows := (len(bindings) + columnCount - 1) / columnCount
	columns := make([][]key.Binding, 0, columnCount)
	for start := 0; start < len(bindings); start += rows {
		end := min(start+rows, len(bindings))
		columns = append(columns, bindings[start:end])
	}
	return columns
}

func (layout helpLayout) matches(msg tea.KeyPressMsg) bool {
	for _, binding := range layout.bindings {
		if key.Matches(msg, binding) {
			return true
		}
	}
	return false
}

func helpColumnsWidth(columns [][]key.Binding, st styles) int {
	width := 0
	separatorWidth := lipgloss.Width(
		st.help.FullSeparator.Render(fullHelpSeparator),
	)
	for i, column := range columns {
		if i > 0 {
			width += separatorWidth
		}
		width += maxLineWidth(helpColumnRows(column, st))
	}
	return width
}

func renderHelp(layout helpLayout, st styles, width int) string {
	if len(layout.columns) == 0 {
		return ""
	}

	columns := make([][]string, 0, len(layout.columns))
	widths := make([]int, 0, len(layout.columns))
	maxRows := 0
	for _, bindings := range layout.columns {
		rows := helpColumnRows(bindings, st)
		columnWidth := maxLineWidth(rows)
		for i, row := range rows {
			rows[i] = padStyledLine(row, columnWidth, st.helpBand)
		}
		columns = append(columns, rows)
		widths = append(widths, columnWidth)
		maxRows = max(maxRows, len(rows))
	}

	lines := make([]string, maxRows)
	separator := st.help.FullSeparator.Render(fullHelpSeparator)
	for row := range maxRows {
		var line strings.Builder
		for column, rows := range columns {
			if column > 0 {
				line.WriteString(separator)
			}
			if row < len(rows) {
				line.WriteString(rows[row])
			} else {
				line.WriteString(st.helpBand.Render(
					strings.Repeat(" ", widths[column]),
				))
			}
		}
		out := line.String()
		if width > 0 && lipgloss.Width(out) > width {
			out = ansi.Truncate(out, width, "...")
		}
		lines[row] = padStyledLine(out, width, st.helpBand)
	}
	return strings.Join(lines, "\n")
}

func helpColumnRows(group []key.Binding, st styles) []string {
	keyWidth := 0
	for _, binding := range group {
		if !binding.Enabled() {
			continue
		}
		if w := lipgloss.Width(binding.Help().Key); w > keyWidth {
			keyWidth = w
		}
	}
	if keyWidth == 0 {
		return nil
	}

	var rows []string
	for _, binding := range group {
		if !binding.Enabled() {
			continue
		}
		help := binding.Help()
		key := st.help.FullKey.Render(help.Key + strings.Repeat(" ", keyWidth-lipgloss.Width(help.Key)))
		rows = append(rows, key+st.helpBand.Render(" ")+st.help.FullDesc.Render(help.Desc))
	}
	return rows
}

func maxLineWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > width {
			width = w
		}
	}
	return width
}

func (m appModel) helpCandidates() []key.Binding {
	if m.showingLinks {
		return m.linksPanelHelpCandidates()
	}
	open := m.keys.Open
	if link, ok := m.focusedReaderLink(); ok &&
		actionsForLink(link).enter == linkEnterRefuse {
		open.SetEnabled(false)
	}
	return []key.Binding{
		m.moveHelpBinding(), open, m.keys.Back, m.keys.FocusInput, m.keys.Home,
		m.keys.Page, m.keys.LinkNext, m.keys.LinkPrev, m.keys.Filter,
		m.keys.Browse, m.keys.Raw, m.keys.Refresh,
		m.keys.Copy, m.keys.Bookmark, m.keys.LinkPanel, m.keys.About, m.keys.Quit,
	}
}

// moveHelpBinding labels ↑/↓ for the view the overlay is describing. In a list
// the keys move a selection and the viewport follows, so "move" names the
// thing that moves. The reader has no cursor — nothing moves but the text —
// and "scroll" is what the status bar already calls it there (app.go:1485,
// against "move" in the list hints at :1432 and :1434). Over a landed reader
// the bar and the overlay are on screen together, so they cannot disagree.
//
// The keys and the enabled flag are inherited: the retained set doubles as the
// execute gate, so a relabelled binding must still match and still be
// suppressed when Move is disabled.
//
// The links panel is a list and keeps "move"; it returns from
// helpCandidates before reaching here.
func (m appModel) moveHelpBinding() key.Binding {
	binding := m.keys.Move
	if m.state == stateReader {
		binding.SetHelp("↑/↓", "scroll")
	}
	return binding
}

func (m appModel) linksPanelHelpCandidates() []key.Binding {
	bindings := []key.Binding{m.keys.Move, m.keys.Back}
	if link, ok := m.linksPanel.selected(); ok {
		actions := actionsForLink(link)
		if actions.enter == linkEnterGo {
			bindings = append(bindings, m.keys.Open)
		}
		if actions.finger {
			bindings = append(bindings, key.NewBinding(
				key.WithKeys("f"), key.WithHelp("f", "go"),
			))
		}
		if actions.copy {
			bindings = append(bindings, m.keys.Copy)
		}
	}
	return append(bindings, m.keys.Filter, m.keys.About)
}

func (m appModel) helpLayout() helpLayout {
	height := m.common.height - 1 // reserve only the permanent status bar
	if height < 1 {
		height = 1
	}
	return layoutHelp(
		m.helpCandidates(), m.common.styles,
		m.common.width, height,
	)
}

func (m appModel) helpView() string {
	return renderHelp(m.helpLayout(), m.common.styles, m.common.width)
}

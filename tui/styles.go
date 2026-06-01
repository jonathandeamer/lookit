package tui

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type palette struct {
	Text           color.Color
	Dim            color.Color
	AccentPink     color.Color
	AccentViolet   color.Color
	AccentMint     color.Color
	AccentGold     color.Color
	AccentRed      color.Color
	BaseBg         color.Color
	SubtleBg       color.Color
	SelectionBg    color.Color
	Rule           color.Color
	BarText        color.Color
	SelectionLogin color.Color
	SelectionDesc  color.Color
}

type styles struct {
	palette palette

	// bottom status bar
	barFill  lipgloss.Style // full-width background
	barHost  lipgloss.Style // "@host" (dim)
	barSep   lipgloss.Style // " / " separator
	barUser  lipgloss.Style // "user" (bold/bright)
	barFlag  lipgloss.Style // neutral flag, e.g. "auto-detected"
	barWarn  lipgloss.Style // caution flag, e.g. "partial (truncated)"
	barDim   lipgloss.Style // right-aligned context (esc/meta/hints)
	barBadge lipgloss.Style

	input    textinput.Styles
	help     help.Styles
	helpBand lipgloss.Style
	spinner  lipgloss.Style
	list     list.Styles
	listItem list.DefaultItemStyles
}

func paletteFor(dark bool) palette {
	if dark {
		return palette{
			Text:           hexColor("#f0edf5"),
			Dim:            hexColor("#8c8792"),
			AccentPink:     hexColor("#ff5fa2"),
			AccentViolet:   hexColor("#9878ff"),
			AccentMint:     hexColor("#38e7ad"),
			AccentGold:     hexColor("#eed76d"),
			AccentRed:      hexColor("#ff6f87"),
			BaseBg:         hexColor("#171719"),
			SubtleBg:       hexColor("#292631"),
			SelectionBg:    hexColor("#342747"),
			Rule:           hexColor("#35313d"),
			BarText:        hexColor("#bbb3c8"),
			SelectionLogin: hexColor("#ff86ba"),
			SelectionDesc:  hexColor("#cbc1dc"),
		}
	}
	return palette{
		Text:           hexColor("#25222a"),
		Dim:            hexColor("#766f7d"),
		AccentPink:     hexColor("#c92870"),
		AccentViolet:   hexColor("#6d43d6"),
		AccentMint:     hexColor("#007f62"),
		AccentGold:     hexColor("#765f00"),
		AccentRed:      hexColor("#c82f4d"),
		BaseBg:         hexColor("#fbfafc"),
		SubtleBg:       hexColor("#e9e4f0"),
		SelectionBg:    hexColor("#f3e9f4"),
		Rule:           hexColor("#ded8e8"),
		BarText:        hexColor("#4c4554"),
		SelectionLogin: hexColor("#a81f62"),
		SelectionDesc:  hexColor("#6f6677"),
	}
}

func hexColor(s string) color.Color {
	return lipgloss.Color(s)
}

func newStyles(dark bool) styles {
	p := paletteFor(dark)
	bar := lipgloss.NewStyle().Background(p.SubtleBg)

	inputStyles := textinput.DefaultStyles(dark)
	inputStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(p.AccentViolet)
	inputStyles.Focused.Text = lipgloss.NewStyle().Foreground(p.Text)
	inputStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(p.Dim)
	inputStyles.Focused.Suggestion = lipgloss.NewStyle().Foreground(p.Dim)
	inputStyles.Blurred.Prompt = lipgloss.NewStyle().Foreground(p.Dim)
	inputStyles.Blurred.Text = lipgloss.NewStyle().Foreground(p.Text)
	inputStyles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(p.Dim)
	inputStyles.Blurred.Suggestion = lipgloss.NewStyle().Foreground(p.Dim)
	inputStyles.Cursor.Color = p.AccentPink
	inputStyles.Cursor.Shape = tea.CursorBar

	helpStyles := help.DefaultStyles(dark)
	helpStyles.ShortKey = helpStyles.ShortKey.Foreground(p.AccentViolet).Background(p.SubtleBg)
	helpStyles.ShortDesc = helpStyles.ShortDesc.Foreground(p.BarText).Background(p.SubtleBg)
	helpStyles.ShortSeparator = helpStyles.ShortSeparator.Foreground(p.BarText).Background(p.SubtleBg)
	helpStyles.Ellipsis = helpStyles.Ellipsis.Foreground(p.BarText).Background(p.SubtleBg)
	helpStyles.FullKey = helpStyles.FullKey.Foreground(p.AccentViolet).Background(p.SubtleBg)
	helpStyles.FullDesc = helpStyles.FullDesc.Foreground(p.BarText).Background(p.SubtleBg)
	helpStyles.FullSeparator = helpStyles.FullSeparator.Foreground(p.BarText).Background(p.SubtleBg)

	listStyles := list.DefaultStyles(dark)
	listStyles.Title = listStyles.Title.
		Background(p.AccentViolet).
		Foreground(hexColor("#ffffff"))
	listStyles.Spinner = listStyles.Spinner.Foreground(p.AccentMint)
	listStyles.Filter = inputStyles
	listStyles.DefaultFilterCharacterMatch = listStyles.DefaultFilterCharacterMatch.
		Foreground(p.AccentPink).
		Underline(true)
	listStyles.StatusBar = listStyles.StatusBar.Foreground(p.BarText).Background(p.SubtleBg)
	listStyles.StatusEmpty = listStyles.StatusEmpty.Foreground(p.Dim)
	listStyles.StatusBarActiveFilter = listStyles.StatusBarActiveFilter.Foreground(p.Text)
	listStyles.StatusBarFilterCount = listStyles.StatusBarFilterCount.Foreground(p.BarText)
	listStyles.NoItems = listStyles.NoItems.Foreground(p.Dim)
	listStyles.PaginationStyle = listStyles.PaginationStyle.Foreground(p.BarText)
	listStyles.HelpStyle = listStyles.HelpStyle.Foreground(p.BarText)
	listStyles.ActivePaginationDot = listStyles.ActivePaginationDot.
		Foreground(p.AccentViolet).
		SetString("•")
	listStyles.InactivePaginationDot = listStyles.InactivePaginationDot.
		Foreground(p.Rule).
		SetString("•")
	listStyles.ArabicPagination = listStyles.ArabicPagination.Foreground(p.BarText)
	listStyles.DividerDot = listStyles.DividerDot.
		Foreground(p.Rule).
		SetString(" • ")

	itemStyles := list.NewDefaultItemStyles(dark)
	itemStyles.NormalTitle = lipgloss.NewStyle().
		Foreground(p.Text).
		Padding(0, 0, 0, 2)
	itemStyles.NormalDesc = lipgloss.NewStyle().
		Foreground(p.Dim).
		Padding(0, 0, 0, 2)
	itemStyles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(p.AccentViolet).
		Background(p.SelectionBg).
		Foreground(p.SelectionLogin).
		Padding(0, 0, 0, 1)
	itemStyles.SelectedDesc = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(p.AccentViolet).
		Background(p.SelectionBg).
		Foreground(p.SelectionDesc).
		Padding(0, 0, 0, 1)
	itemStyles.DimmedTitle = lipgloss.NewStyle().
		Foreground(p.Dim).
		Padding(0, 0, 0, 2)
	itemStyles.DimmedDesc = lipgloss.NewStyle().
		Foreground(p.BarText).
		Padding(0, 0, 0, 2)
	itemStyles.FilterMatch = lipgloss.NewStyle().
		Foreground(p.AccentPink).
		Underline(true)

	return styles{
		palette: p,
		barFill: bar,
		barHost: bar.Foreground(p.BarText),
		barSep:  bar.Foreground(p.BarText),
		barUser: bar.Foreground(p.Text).Bold(true),
		barFlag: bar.Foreground(p.BarText),
		barWarn: bar.Foreground(p.AccentGold),
		barDim:  bar.Foreground(p.BarText),
		barBadge: lipgloss.NewStyle().
			Background(p.AccentViolet).
			Foreground(hexColor("#ffffff")).
			Bold(true),
		input: inputStyles,
		help:  helpStyles,
		helpBand: lipgloss.NewStyle().
			Background(p.SubtleBg).
			Foreground(p.BarText),
		spinner:  lipgloss.NewStyle().Foreground(p.AccentMint),
		list:     listStyles,
		listItem: itemStyles,
	}
}

func padStyledLine(line string, width int, fill lipgloss.Style) string {
	if width <= 0 {
		return line
	}
	if pad := width - lipgloss.Width(line); pad > 0 {
		return line + fill.Render(strings.Repeat(" ", pad))
	}
	return line
}

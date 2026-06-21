package style

import (
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const accentColor = lipgloss.BrightGreen

var (
	Header           = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	ButtonSelected   = lipgloss.NewStyle().Foreground(lipgloss.Black).Background(accentColor)
	InputBox         = makeInputBoxStyle()
	ErrString        = lipgloss.NewStyle().Foreground(lipgloss.Red)
	Table            = makeTableStyles()
	TableNoSelection = makeTableNoSelectionStyles()
)

func makeInputBoxStyle() textinput.Styles {
	style := textinput.DefaultDarkStyles()
	style.Cursor = textinput.CursorStyle{
		Color: accentColor,
		Shape: tea.CursorBlock,
	}
	style.Focused.Prompt = Header

	return style
}

func makeTableStyles() table.Styles {
	style := table.DefaultStyles()
	style.Selected = style.Selected.Foreground(lipgloss.Color("#000000")).Background(accentColor)
	style.Header = style.Header.
		Foreground(lipgloss.White).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(accentColor).
		BorderBottom(true).
		Bold(true)
	return style
}

func makeTableNoSelectionStyles() table.Styles {
	style := table.DefaultStyles()
	style.Selected = style.Cell.Padding(0)
	style.Header = style.Header.
		Foreground(lipgloss.White).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(accentColor).
		BorderBottom(true).
		Bold(true)
	return style
}

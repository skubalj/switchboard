package style

import (
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	Header         = lipgloss.NewStyle().Foreground(lipgloss.BrightBlue).Bold(true)
	ButtonSelected = lipgloss.NewStyle().Foreground(lipgloss.Black).Background(lipgloss.BrightBlue)
	InputBox       = makeInputBoxStyle()
	ErrString      = lipgloss.NewStyle().Foreground(lipgloss.Red)
	Table          = makeTableStyles()
)

func makeInputBoxStyle() textinput.Styles {
	style := textinput.DefaultDarkStyles()
	style.Cursor = textinput.CursorStyle{
		Color: lipgloss.BrightBlue,
		Shape: tea.CursorBlock,
	}
	style.Focused.Prompt = Header

	return style
}

func makeTableStyles() table.Styles {
	style := table.DefaultStyles()
	style.Selected = style.Selected.Foreground(lipgloss.Color("#000000")).Background(lipgloss.BrightBlue)
	style.Header = style.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.BrightBlue).
		BorderBottom(true).
		Bold(true)
	return style
}

package style

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	Header         = lipgloss.NewStyle().Foreground(lipgloss.BrightBlue).Bold(true)
	ButtonSelected = lipgloss.NewStyle().Background(lipgloss.BrightBlue)
	InputBox       = makeInputBoxStyle()
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

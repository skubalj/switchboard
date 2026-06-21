package errormodal

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/skubalj/switchboard/tui/components"
	"github.com/skubalj/switchboard/tui/modal"
)

type ErrorModal struct {
	Title   string
	Content string
}

type ErrorMsg struct {
	Title string
	Err   error
}

func NewErrorModal(title, content string) modal.Window {
	return &ErrorModal{Title: title, Content: content}
}

func (m *ErrorModal) Update(msg tea.Msg) (modal.Window, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "enter":
			return nil, nil
		case "ctrl+c":
			return nil, tea.Quit
		}
	}

	return m, nil
}

func (m ErrorModal) Render() modal.ContentBlock {
	lines := slices.Collect(strings.Lines(m.Content))
	length := 0
	for _, line := range lines {
		length = max(length, len(line))
	}

	frame := components.Frame{
		Title:    m.Title,
		Width:    length + 4,
		Height:   len(lines) + 2,
		PaddingX: 1,
	}

	return modal.ContentBlock{
		Content: frame.Render(strings.Join(lines, "")),
		Width:   frame.Width,
		Height:  frame.Height,
	}
}

package passwordmodal

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/skubalj/switchboard/tui/components"
	"github.com/skubalj/switchboard/tui/modal"
	"github.com/skubalj/switchboard/tui/style"
)

type PasswordModal struct {
	title    string
	res      chan<- string
	selected int
	input    textinput.Model
}

func NewPasswordModal(title string, response chan<- string) modal.Window {
	input := textinput.New()
	input.Prompt = "> "
	input.SetVirtualCursor(true)
	input.SetWidth(48)
	input.SetStyles(style.InputBox)

	return &PasswordModal{
		title:    title,
		res:      response,
		selected: 0,
		input:    input,
	}
}

func (m *PasswordModal) Update(msg tea.Msg) (modal.Window, tea.Cmd) {
	m.input, _ = m.input.Update(msg)
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return nil, nil
		case "ctrl+c":
			return nil, tea.Quit
		case "enter":
			switch m.selected {
			case 1:
				m.res <- m.input.Value()
			case 2:
				close(m.res)
			}
			return nil, nil
		case "up":
			if m.selected > 0 {
				m.selected = 0
			}
		case "down":
			if m.selected < 1 {
				m.selected = 1
			}
		case "right":
			if m.selected == 1 {
				m.selected = 2
			}
		case "left":
			if m.selected == 2 {
				m.selected = 1
			}
		case "tab":
			if m.selected < 2 {
				m.selected++
			}
		case "shift+tab":
			if m.selected > 0 {
				m.selected--
			}
		}
	}

	return m, nil
}

func (m *PasswordModal) Render() modal.ContentBlock {
	frame := components.Frame{
		Title:    m.title,
		Width:    50,
		Height:   3,
		PaddingX: 1,
	}

	return modal.ContentBlock{
		Content: frame.Render(m.input.View()),
		Width:   frame.Width,
		Height:  frame.Height,
	}
}

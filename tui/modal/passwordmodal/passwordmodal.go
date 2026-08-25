package passwordmodal

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/skubalj/switchboard/tui/components"
	"github.com/skubalj/switchboard/tui/modal"
	"github.com/skubalj/switchboard/tui/style"
)

type PasswordModal struct {
	title string
	res   chan<- string
	input textinput.Model
}

func NewPasswordModal(title string, response chan<- string) modal.Window {
	input := textinput.New()
	input.Prompt = "> "
	input.EchoMode = textinput.EchoPassword
	input.SetVirtualCursor(true)
	input.SetWidth(48)
	input.SetStyles(style.InputBox)
	input.Focus()

	return &PasswordModal{
		title: title,
		res:   response,
		input: input,
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
			m.res <- m.input.Value()
			close(m.res)
			return nil, nil
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

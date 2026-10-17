package passwordmodal

import (
	"errors"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/skubalj/switchboard/tui/components"
	"github.com/skubalj/switchboard/tui/modal"
	"github.com/skubalj/switchboard/tui/style"
)

type PasswordModal struct {
	title string
	res   Responder
	input textinput.Model
}

type Responder interface {
	Respond(string)
	RespondErr(error)
}

func NewPasswordModal(title string, response Responder) modal.Window {
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
			m.res.RespondErr(errors.New("operation cancelled"))
			return nil, nil
		case "ctrl+c":
			m.res.RespondErr(errors.New("operation cancelled"))
			return nil, tea.Quit
		case "enter":
			m.res.Respond(m.input.Value())
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

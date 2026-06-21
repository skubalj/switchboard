package passwordmodal

import (
	tea "charm.land/bubbletea/v2"
	"github.com/skubalj/switchboard/tui/modal"
	"github.com/skubalj/switchboard/tui/modal/textinputmodal"
)

type PasswordModal struct {
	connectionID uint32
	inner        textinputmodal.TextInputModal
}

func NewPasswordModal(connID uint32, targetStrings []string) modal.Window {
	inputs := make([]textinputmodal.TextInput, 0, len(targetStrings))
	for _, str := range targetStrings {
		inputs = append(inputs, textinputmodal.TextInput{
			Prompt:      str,
			IsPassword:  true,
		})
	}

	inner := textinputmodal.NewTextInputModal("Enter Password", inputs, []string{"OK", "Cancel"})
	return &PasswordModal{connectionID: connID, inner: inner}
}

type PasswordMessage struct {
	ConnectionID uint32
	Passwords    []string
}

func (m *PasswordModal) Update(msg tea.Msg) (modal.Window, tea.Cmd) {
	button, state := m.inner.Update(msg)
	switch state {
	case textinputmodal.Ok:
		switch button {
		case "OK":
			passwords := m.inner.GetValues()
			return nil, func() tea.Msg {
				return PasswordMessage{
					ConnectionID: m.connectionID,
					Passwords:    passwords,
				}
			}

		case "Cancel":
			return nil, nil
		}
		return m, nil
	case textinputmodal.Cancel:
		return nil, nil
	case textinputmodal.Quit:
		return nil, tea.Quit
	}

	return m, nil
}

func (m PasswordModal) Render() modal.ContentBlock {
	return m.inner.Render()
}

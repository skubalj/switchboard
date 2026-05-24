package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/skubalj/switchboard/tui/style"
)

type modal interface {
	Update(msg tea.Msg) (modal, tea.Cmd)
	Render() contentBlock
}

type contentBlock struct {
	Content string
	Width   int
	Height  int
}

type PasswordModal struct {
	ConnectionID   uint32
	PasswordTarget string
	Password       textinput.Model
}

func NewPasswordModal(target string, connID uint32) modal {
	input := textinput.New()
	input.SetVirtualCursor(true)
	input.SetWidth(48)
	input.SetStyles(style.InputBox)
	input.Focus()
	input.EchoMode = textinput.EchoPassword
	input.EchoCharacter = '*'

	return &PasswordModal{ConnectionID: connID, PasswordTarget: target, Password: input}
}

type PasswordMessage struct {
	ConnectionID uint32
	Password     string
}

func (m *PasswordModal) Update(msg tea.Msg) (modal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return nil, nil
		case "ctrl+c":
			return nil, tea.Quit
		case "enter":
			return nil, func() tea.Msg {
				return PasswordMessage{
					ConnectionID: m.ConnectionID,
					Password:     m.Password.Value(),
				}
			}
		default:
			var cmd tea.Cmd
			m.Password, cmd = m.Password.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m PasswordModal) Render() contentBlock {
	frame := Frame{
		Title:    "Password for " + m.PasswordTarget,
		Width:    50,
		Height:   3,
		PaddingX: 1,
	}

	return contentBlock{
		Content: frame.Render(m.Password.View()),
		Width:   frame.Width,
		Height:  frame.Height,
	}
}

///////////////////////////////////////////////////////////////////////////////

type ErrorModal struct {
	Title   string
	Content string
}

func NewErrorModal(title, content string) modal {
	return &ErrorModal{Title: title, Content: content}
}

func (m *ErrorModal) Update(msg tea.Msg) (modal, tea.Cmd) {
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

func (m ErrorModal) Render() contentBlock {
	lines := slices.Collect(strings.Lines(m.Content))
	length := 0
	for _, line := range lines {
		length = max(length, len(line))
	}

	frame := Frame{
		Title:    m.Title,
		Width:    length + 4,
		Height:   len(lines) + 2,
		PaddingX: 1,
	}

	return contentBlock{
		Content: frame.Render(strings.Join(lines, "")),
		Width:   frame.Width,
		Height:  frame.Height,
	}
}

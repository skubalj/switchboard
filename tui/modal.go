package tui

import (
	"cmp"
	"os/user"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/skubalj/switchboard/tui/style"
)

type ConnectionModal struct {
	modalTitle string
	user       textinput.Model
	host       textinput.Model
	sshKey     textinput.Model
	selected   ConnectionModalField
}

type ConnectionModalField int

const (
	userField ConnectionModalField = iota
	hostField
	sshKeyField
	connectButtonField
	cancelButtonField
	numFields
)

func NewCollectionModal() *ConnectionModal {
	var input textinput.Model
	input.SetVirtualCursor(true)
	input.SetWidth(48)
	input.SetStyles(style.InputBox)

	modal := ConnectionModal{
		modalTitle: "New Connection",
		user:       input,
		host:       input,
		sshKey:     input,
	}

	osUser, err := user.Current()
	if err == nil {
		modal.user.Placeholder = osUser.Username
	}
	modal.user.Prompt = "User: "

	modal.host.Placeholder = "hostname:22"
	modal.host.Prompt = "Host: "

	modal.sshKey.Prompt = "SSH Key: "

	modal.SetFocus()
	return &modal
}

func EditCollectionModal(row connectionRow) modal {
	modal := NewCollectionModal()
	modal.modalTitle = "Edit Connection"
	modal.user.SetValue(row.User)
	modal.host.SetValue(row.Host)
	modal.sshKey.SetValue(row.SSHKey)

	return modal
}

func (m *ConnectionModal) Update(msg tea.Msg) (modal, tea.Cmd) {
	var cmdArray []tea.Cmd

	// Only the "focused" input fields will respond, so we can publish to all of them
	var cmd tea.Cmd
	m.user, cmd = m.user.Update(msg)
	cmdArray = append(cmdArray, cmd)
	m.host, cmd = m.host.Update(msg)
	cmdArray = append(cmdArray, cmd)
	m.sshKey, cmd = m.sshKey.Update(msg)
	cmdArray = append(cmdArray, cmd)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return nil, nil
		case "ctrl+c":
			return nil, tea.Quit
		case "up":
			m.selected = max(0, m.selected-1)
			cmdArray = append(cmdArray, m.SetFocus())
		case "down":
			m.selected = min(numFields-2, m.selected+1)
			cmdArray = append(cmdArray, m.SetFocus())
		case "shift+tab":
			m.selected = max(0, m.selected-1)
			cmdArray = append(cmdArray, m.SetFocus())
		case "tab":
			m.selected = min(numFields-1, m.selected+1)
			cmdArray = append(cmdArray, m.SetFocus())

		case "enter":
			switch m.selected {
			case connectButtonField:
				return nil, func() tea.Msg {
					return NewConnectionRow(
						cmp.Or(m.user.Value(), m.user.Placeholder),
						m.host.Value(),
						m.sshKey.Value(),
					)
				}
			case cancelButtonField:
				return nil, nil
			}
		case "right":
			if m.selected == connectButtonField {
				m.selected = cancelButtonField
			}
		case "left":
			if m.selected == cancelButtonField {
				m.selected = connectButtonField
			}
		}
	}

	return m, tea.Batch(cmdArray...)
}

func (m *ConnectionModal) SetFocus() tea.Cmd {
	m.user.Blur()
	m.host.Blur()
	m.sshKey.Blur()

	switch m.selected {
	case userField:
		return m.user.Focus()
	case hostField:
		return m.host.Focus()
	case sshKeyField:
		return m.sshKey.Focus()
	default:
		return nil
	}
}

func (m ConnectionModal) Render() contentBlock {
	var buf strings.Builder
	buf.WriteString(m.user.View())
	buf.WriteRune('\n')
	buf.WriteString(m.host.View())
	buf.WriteRune('\n')
	buf.WriteString(m.sshKey.View())
	buf.WriteRune('\n')
	buf.WriteRune('\n')

	connectBtn := "<Connect>"
	if m.selected == connectButtonField {
		connectBtn = style.ButtonSelected.Render(connectBtn)
	}
	buf.WriteString(connectBtn)
	buf.WriteString(" ")

	cancelBtn := "<Cancel>"
	if m.selected == cancelButtonField {
		cancelBtn = style.ButtonSelected.Render(cancelBtn)
	}
	buf.WriteString(cancelBtn)

	frame := Frame{
		Title:    m.modalTitle,
		Width:    54,
		Height:   7,
		PaddingX: 1,
	}

	return contentBlock{
		Content: frame.Render(buf.String()),
		Width:   frame.Width,
		Height:  frame.Height,
	}
}

type PasswordModal struct {
	PasswordTarget string
	Password       textinput.Model
}

func NewPasswordModal(target string) modal {
	var input textinput.Model
	input.SetVirtualCursor(true)
	input.SetWidth(48)
	input.SetStyles(style.InputBox)
	input.Focus()
	input.EchoMode = textinput.EchoPassword
	input.EchoCharacter = '*'

	return &PasswordModal{PasswordTarget: target, Password: input}
}

type PasswordMessage string

func (m *PasswordModal) Update(msg tea.Msg) (modal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return nil, nil
		case "ctrl+c":
			return nil, tea.Quit
		case "enter":
			return nil, func() tea.Msg { return PasswordMessage(m.Password.Value()) }
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
	frame := Frame{
		Title:    m.Title,
		Width:    len(m.Content) + 4,
		Height:   3,
		PaddingX: 1,
	}

	return contentBlock{
		Content: frame.Render(m.Content),
		Width:   frame.Width,
		Height:  frame.Height,
	}
}

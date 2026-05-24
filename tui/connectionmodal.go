package tui

import (
	"cmp"
	"os/user"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/skubalj/switchboard/config"
	"github.com/skubalj/switchboard/tui/style"
)

type ConnectionModal struct {
	configLookup func(string) (config.Host, error)
	modalTitle   string
	user         textinput.Model
	host         textinput.Model
	port         textinput.Model
	sshKey       textinput.Model
	errString    string
	selected     ConnectionModalField
}

var connectionModalKeymap = struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Next     key.Binding
	Previous key.Binding
	Select   key.Binding
	Cancel   key.Binding
	Exit     key.Binding
}{
	Up:       key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "Up")),
	Down:     key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "Down")),
	Left:     key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "Left")),
	Right:    key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "Right")),
	Next:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "Next")),
	Previous: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("Shift+Tab", "Previous")),
	Select:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Connect/Accept")),
	Cancel:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "Cancel")),
	Exit:     key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "Exit")),
}

type ConnectionModalField int

const (
	userField ConnectionModalField = iota
	hostField
	portField
	sshKeyField
	loadButtonField
	saveButtonField
	cancelButtonField
	numFields
)

func NewCollectionModal(configLookup func(string) (config.Host, error)) *ConnectionModal {
	input := textinput.New()
	input.SetVirtualCursor(true)
	input.SetWidth(48)
	input.SetStyles(style.InputBox)

	modal := ConnectionModal{
		configLookup: configLookup,
		modalTitle:   "New Connection",
		user:         input,
		host:         input,
		port:         input,
		sshKey:       input,
	}

	osUser, err := user.Current()
	if err == nil {
		modal.user.Placeholder = osUser.Username
	}
	modal.user.Prompt = "User: "

	modal.host.Prompt = "Host: "

	modal.port.Prompt = "Port: "
	modal.port.Placeholder = "22"

	modal.sshKey.Prompt = "SSH Key: "

	modal.SetFocus()
	return &modal
}

func EditCollectionModal(row connectionRow, configLookup func(string) (config.Host, error)) modal {
	modal := NewCollectionModal(configLookup)
	modal.modalTitle = "Edit Connection"
	modal.user.SetValue(row.User)
	modal.host.SetValue(row.Host)
	modal.port.SetValue(strconv.FormatUint(uint64(row.Port), 10))
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
	m.port, cmd = m.port.Update(msg)
	cmdArray = append(cmdArray, cmd)
	m.sshKey, cmd = m.sshKey.Update(msg)
	cmdArray = append(cmdArray, cmd)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, connectionModalKeymap.Cancel):
			return nil, nil
		case key.Matches(msg, connectionModalKeymap.Exit):
			return nil, tea.Quit
		case key.Matches(msg, connectionModalKeymap.Up):
			if m.selected >= loadButtonField {
				m.selected = sshKeyField
			} else if m.selected > 0 {
				m.selected--
			}
			cmdArray = append(cmdArray, m.SetFocus())

		case key.Matches(msg, connectionModalKeymap.Down):
			if m.selected < loadButtonField {
				m.selected++
			}
			cmdArray = append(cmdArray, m.SetFocus())

		case key.Matches(msg, connectionModalKeymap.Previous):
			m.selected = max(0, m.selected-1)
			cmdArray = append(cmdArray, m.SetFocus())

		case key.Matches(msg, connectionModalKeymap.Next):
			m.selected = min(numFields-1, m.selected+1)
			cmdArray = append(cmdArray, m.SetFocus())

		case key.Matches(msg, connectionModalKeymap.Select):
			switch m.selected {
			case saveButtonField:
				port, err := strconv.ParseUint(m.port.Value(), 10, 16)
				if err == nil {
					return nil, func() tea.Msg {
						return NewConnectionRow(
							cmp.Or(m.user.Value(), m.user.Placeholder),
							m.host.Value(),
							uint16(port),
							m.sshKey.Value(),
						)
					}
				} else {
					return NewErrorModal("Error", "port must be a number"), nil
				}
			case cancelButtonField:
				return nil, nil
			case loadButtonField:
				m.setFromConfig()
				return m, nil
			}

		case key.Matches(msg, connectionModalKeymap.Right):
			switch m.selected {
			case loadButtonField:
				m.selected = saveButtonField
			case saveButtonField:
				m.selected = cancelButtonField
			}

		case key.Matches(msg, connectionModalKeymap.Left):
			switch m.selected {
			case saveButtonField:
				m.selected = loadButtonField
			case cancelButtonField:
				m.selected = saveButtonField
			}
		}
	}

	return m, tea.Batch(cmdArray...)
}

func (m *ConnectionModal) setFromConfig() {
	host, err := m.configLookup(m.host.Value())
	if err != nil {
		m.errString = err.Error()
		return
	}

	m.user.SetValue(host.User)
	m.host.SetValue(host.Host)
	m.port.SetValue(strconv.FormatUint(uint64(host.Port), 10))
	m.sshKey.SetValue(host.IdentityFile)
}

func (m *ConnectionModal) SetFocus() tea.Cmd {
	m.user.Blur()
	m.host.Blur()
	m.port.Blur()
	m.sshKey.Blur()

	switch m.selected {
	case userField:
		return m.user.Focus()
	case hostField:
		return m.host.Focus()
	case portField:
		return m.port.Focus()
	case sshKeyField:
		return m.sshKey.Focus()
	default:
		return nil
	}
}

func (m *ConnectionModal) Render() contentBlock {
	var buf strings.Builder
	buf.WriteString(m.user.View())
	buf.WriteRune('\n')
	buf.WriteString(m.host.View())
	buf.WriteRune('\n')
	buf.WriteString(m.port.View())
	buf.WriteRune('\n')
	buf.WriteString(m.sshKey.View())
	buf.WriteRune('\n')
	buf.WriteString(style.ErrString.Render(m.errString))
	buf.WriteRune('\n')

	loadBtn := "<Load From Config>"
	if m.selected == loadButtonField {
		loadBtn = style.ButtonSelected.Render(loadBtn)
	}
	buf.WriteString(loadBtn)
	buf.WriteString(" ")

	connectBtn := "<Save>"
	if m.selected == saveButtonField {
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
		Height:   8,
		PaddingX: 1,
	}

	return contentBlock{
		Content: frame.Render(buf.String()),
		Width:   frame.Width,
		Height:  frame.Height,
	}
}

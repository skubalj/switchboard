package tui

import (
	"cmp"
	"os/user"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"
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
		switch msg.String() {
		case "esc":
			return nil, nil
		case "ctrl+c":
			return nil, tea.Quit
		case "up":
			if m.selected >= loadButtonField {
				m.selected = sshKeyField
			} else if m.selected > 0 {
				m.selected--
			}
			cmdArray = append(cmdArray, m.SetFocus())
		case "down":
			if m.selected < loadButtonField {
				m.selected++
			}
			cmdArray = append(cmdArray, m.SetFocus())
		case "shift+tab":
			m.selected = max(0, m.selected-1)
			cmdArray = append(cmdArray, m.SetFocus())
		case "tab":
			m.selected = min(numFields-1, m.selected+1)
			cmdArray = append(cmdArray, m.SetFocus())

		case "enter":
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
		case "right":
			switch m.selected {
			case loadButtonField:
				m.selected = saveButtonField
			case saveButtonField:
				m.selected = cancelButtonField
			}
		case "left":
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

///////////////////////////////////////////////////////////////////////////////

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

///////////////////////////////////////////////////////////////////////////////

type PortForwardingModal struct {
	portForwards []PortForward
	tableState   table.Model
	isLocal      bool
	editMode     bool
}

func NewLocalForwardingModal(forwards []PortForward) modal {
	return &PortForwardingModal{
		portForwards: forwards,
		tableState:   newLocalForwardingTable(forwards),
		isLocal:      true,
		editMode:     true,
	}
}

func NewRemoteForwardingModal(forwards []PortForward) modal {
	return &PortForwardingModal{
		portForwards: forwards,
		tableState:   newRemoteForwardingTable(forwards),
		isLocal:      false,
	}
}

func (m *PortForwardingModal) Update(msg tea.Msg) (modal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return nil, nil
		case "ctrl+c":
			return nil, tea.Quit
		case "enter":
			if m.tableState.Cursor() >= len(m.portForwards) {
				if m.isLocal {
					return NewEditLocalForwardModal(), nil
				} else {
					return NewEditRemoteForwardModal(), nil
				}
			}
		case "delete":
			idx := m.tableState.Cursor()
			if idx < len(m.portForwards) {
				if m.isLocal {
					return nil, func() tea.Msg { return DeleteLocalForward(idx) }
				} else {
					return nil, func() tea.Msg { return DeleteRemoteForward(idx) }
				}
			}
		case "up":
			m.tableState.MoveUp(1)
		case "down":
			m.tableState.MoveDown(1)
		default:
		}
	}

	return m, nil
}

func (m PortForwardingModal) Render() contentBlock {
	title := "Remote Port Forwards"
	if m.isLocal {
		title = "Local Port Forwards"
	}

	frame := Frame{
		Title:  title,
		Width:  80,
		Height: 10,
	}

	if m.isLocal {
		m.tableState.SetColumns(makeLocalForwardingColumns(frame.InnerWidth()))
	} else {
		m.tableState.SetColumns(makeRemoteForwardingColumns(frame.InnerWidth()))
	}

	m.tableState.SetWidth(frame.InnerWidth())
	m.tableState.SetHeight(frame.InnerHeight())

	return contentBlock{
		Content: frame.Render(m.tableState.View()),
		Width:   frame.Width,
		Height:  frame.Height,
	}
}

///////////////////////////////////////////////////////////////////////////////

type EditLocalForwardModal struct {
	selectedInput int
	localHost     textinput.Model
	localPort     textinput.Model
	remoteHost    textinput.Model
	remotePort    textinput.Model
}

func NewEditLocalForwardModal() modal {
	textInput := textinput.New()
	textInput.SetStyles(style.InputBox)

	modal := &EditLocalForwardModal{
		selectedInput: 0,
		localHost:     textInput,
		localPort:     textInput,
		remoteHost:    textInput,
		remotePort:    textInput,
	}

	modal.localHost.Focus()
	modal.localHost.Placeholder = "localhost"
	modal.remoteHost.Placeholder = "localhost"

	return modal
}

func (m *EditLocalForwardModal) Update(msg tea.Msg) (modal, tea.Cmd) {
	cmds := make([]tea.Cmd, 4)

	m.localHost, cmds[0] = m.localHost.Update(msg)
	m.localPort, cmds[1] = m.localPort.Update(msg)
	m.remoteHost, cmds[2] = m.remoteHost.Update(msg)
	m.remotePort, cmds[3] = m.remotePort.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return nil, nil
		case "ctrl+c":
			return nil, tea.Quit
		case "enter":
			switch m.selectedInput {
			case 4: // accept
				forward, err := newPortForward(m.localHost.Value(), m.localPort.Value(), m.remoteHost.Value(), m.remotePort.Value())
				if err != nil {
					return NewErrorModal("Error", "unable to create port forward:\n"+err.Error()), nil
				}
				return nil, func() tea.Msg { return NewLocalForward(forward) }

			case 5: // cancel
				return nil, nil
			}

		case "tab":
			if m.selectedInput < 5 {
				m.selectedInput += 1
				m.updateFocus()
			}
		case "shift+tab":
			if m.selectedInput > 0 {
				m.selectedInput -= 1
				m.updateFocus()
			}

		case "right":
			if m.selectedInput == 4 {
				m.selectedInput = 5
			}

		case "left":
			if m.selectedInput == 5 {
				m.selectedInput = 4
			}

		case "up":
			if m.selectedInput > 3 {
				m.selectedInput = 0
			}
			m.updateFocus()

		case "down":
			if m.selectedInput < 4 {
				m.selectedInput = 4
			}
			m.updateFocus()

		default:
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *EditLocalForwardModal) updateFocus() {
	m.localHost.Blur()
	m.localPort.Blur()
	m.remoteHost.Blur()
	m.remotePort.Blur()

	switch m.selectedInput {
	case 0:
		m.localHost.Focus()
	case 1:
		m.localPort.Focus()
	case 2:
		m.remoteHost.Focus()
	case 3:
		m.remotePort.Focus()
	}
}

func (m EditLocalForwardModal) Render() contentBlock {
	frame := Frame{
		Title:    "New Local Port Forward",
		Width:    100,
		Height:   6,
		PaddingX: 1,
	}

	dividedWidth := (frame.InnerWidth()) / 4
	m.localHost.SetWidth(dividedWidth - 3)
	m.localPort.SetWidth(dividedWidth - 3)
	m.remoteHost.SetWidth(dividedWidth - 3)
	m.remotePort.SetWidth(dividedWidth - 3)

	labelsBuf := asciiSpaceArr(frame.InnerWidth())
	labels := []string{"Local Bind Address", "Local Bind Port", "Remote Address", "Remote Port"}
	for idx, element := range labels {
		copy(labelsBuf[idx*dividedWidth:], element)
	}
	var lines strings.Builder
	lines.WriteString(string(labelsBuf))
	lines.WriteRune('\n')

	lines.WriteString(m.localHost.View())
	lines.WriteString(m.localPort.View())
	lines.WriteString(m.remoteHost.View())
	lines.WriteString(m.remotePort.View())
	lines.WriteString("\n\n")

	okButton := "<Apply>"
	if m.selectedInput == 4 {
		okButton = style.ButtonSelected.Render(okButton)
	}
	lines.WriteString(okButton)
	lines.WriteRune(' ')

	cancelButton := "<Cancel>"
	if m.selectedInput == 5 {
		cancelButton = style.ButtonSelected.Render(cancelButton)
	}
	lines.WriteString(cancelButton)

	return contentBlock{
		Content: frame.Render(lines.String()),
		Width:   frame.Width,
		Height:  frame.Height,
	}
}

///////////////////////////////////////////////////////////////////////////////

type EditRemoteForwardModal struct {
	selectedInput int
	localHost     textinput.Model
	localPort     textinput.Model
	remoteHost    textinput.Model
	remotePort    textinput.Model
}

func NewEditRemoteForwardModal() modal {
	textInput := textinput.New()
	textInput.SetStyles(style.InputBox)

	modal := &EditRemoteForwardModal{
		selectedInput: 0,
		localHost:     textInput,
		localPort:     textInput,
		remoteHost:    textInput,
		remotePort:    textInput,
	}

	modal.remoteHost.Focus()
	modal.remoteHost.Placeholder = "localhost"
	modal.localHost.Placeholder = "localhost"

	return modal
}

func (m *EditRemoteForwardModal) Update(msg tea.Msg) (modal, tea.Cmd) {
	cmds := make([]tea.Cmd, 4)

	m.localHost, cmds[0] = m.localHost.Update(msg)
	m.localPort, cmds[1] = m.localPort.Update(msg)
	m.remoteHost, cmds[2] = m.remoteHost.Update(msg)
	m.remotePort, cmds[3] = m.remotePort.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return nil, nil
		case "ctrl+c":
			return nil, tea.Quit
		case "enter":
			switch m.selectedInput {
			case 4: // accept
				forward, err := newPortForward(m.localHost.Value(), m.localPort.Value(), m.remoteHost.Value(), m.remotePort.Value())
				if err != nil {
					return NewErrorModal("Error", "unable to create port forward:\n"+err.Error()), nil
				}
				return nil, func() tea.Msg { return NewRemoteForward(forward) }

			case 5: // cancel
				return nil, nil
			}

		case "tab":
			if m.selectedInput < 5 {
				m.selectedInput += 1
				m.updateFocus()
			}
		case "shift+tab":
			if m.selectedInput > 0 {
				m.selectedInput -= 1
				m.updateFocus()
			}

		case "right":
			if m.selectedInput == 4 {
				m.selectedInput = 5
			}

		case "left":
			if m.selectedInput == 5 {
				m.selectedInput = 4
			}

		case "up":
			if m.selectedInput > 3 {
				m.selectedInput = 0
			}
			m.updateFocus()

		case "down":
			if m.selectedInput < 4 {
				m.selectedInput = 4
			}
			m.updateFocus()

		default:
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *EditRemoteForwardModal) updateFocus() {
	m.localHost.Blur()
	m.localPort.Blur()
	m.remoteHost.Blur()
	m.remotePort.Blur()

	switch m.selectedInput {
	case 0:
		m.remoteHost.Focus()
	case 1:
		m.remotePort.Focus()
	case 2:
		m.localHost.Focus()
	case 3:
		m.localPort.Focus()
	}
}

func (m EditRemoteForwardModal) Render() contentBlock {
	frame := Frame{
		Title:    "New Remote Port Forward",
		Width:    100,
		Height:   6,
		PaddingX: 1,
	}

	dividedWidth := (frame.InnerWidth()) / 4
	m.localHost.SetWidth(dividedWidth - 3)
	m.localPort.SetWidth(dividedWidth - 3)
	m.remoteHost.SetWidth(dividedWidth - 3)
	m.remotePort.SetWidth(dividedWidth - 3)

	labelsBuf := asciiSpaceArr(frame.InnerWidth())
	labels := []string{"Remote Bind Address", "Remote Bind Port", "Local Address", "Local Port"}
	for idx, element := range labels {
		copy(labelsBuf[idx*dividedWidth:], element)
	}
	var lines strings.Builder
	lines.WriteString(string(labelsBuf))
	lines.WriteRune('\n')

	lines.WriteString(m.remoteHost.View())
	lines.WriteString(m.remotePort.View())
	lines.WriteString(m.localHost.View())
	lines.WriteString(m.localPort.View())
	lines.WriteString("\n\n")

	okButton := "<Apply>"
	if m.selectedInput == 4 {
		okButton = style.ButtonSelected.Render(okButton)
	}
	lines.WriteString(okButton)
	lines.WriteRune(' ')

	cancelButton := "<Cancel>"
	if m.selectedInput == 5 {
		cancelButton = style.ButtonSelected.Render(cancelButton)
	}
	lines.WriteString(cancelButton)

	return contentBlock{
		Content: frame.Render(lines.String()),
		Width:   frame.Width,
		Height:  frame.Height,
	}
}

func asciiSpaceArr(len int) []byte {
	buf := make([]byte, len)
	for i := range buf {
		buf[i] = ' '
	}
	return buf
}

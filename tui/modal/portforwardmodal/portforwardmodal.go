package portforwardmodal

import (
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/skubalj/switchboard/config"
	"github.com/skubalj/switchboard/tui/components"
	"github.com/skubalj/switchboard/tui/modal"
	"github.com/skubalj/switchboard/tui/modal/errormodal"
	"github.com/skubalj/switchboard/tui/style"
	"github.com/skubalj/switchboard/tui/utils"
)

type NewLocalForward config.PortForward
type DeleteLocalForward int
type NewRemoteForward config.PortForward
type DeleteRemoteForward int

type PortForwardingModal struct {
	portForwards []PortForward
	tableState   table.Model
	isLocal      bool
}

func NewLocalForwardingModal(forwards []PortForward) modal.Window {
	return &PortForwardingModal{
		portForwards: forwards,
		tableState:   newLocalForwardingTable(forwards),
		isLocal:      true,
	}
}

func NewRemoteForwardingModal(forwards []PortForward) modal.Window {
	return &PortForwardingModal{
		portForwards: forwards,
		tableState:   newRemoteForwardingTable(forwards),
		isLocal:      false,
	}
}

func (m *PortForwardingModal) Update(msg tea.Msg) (modal.Window, tea.Cmd) {
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
		case "pgup":
			if utils.ReorderUp(m.portForwards, m.tableState.Cursor()) {
				m.tableState.MoveUp(1)
				m.updateTable()
			}
		case "pgdown":
			if utils.ReorderDown(m.portForwards, m.tableState.Cursor()) {
				m.tableState.MoveDown(1)
				m.updateTable()
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

func (m *PortForwardingModal) updateTable() {
	if m.isLocal {
		m.tableState.SetRows(makeLocalForwardingRows(m.portForwards))
	} else {
		m.tableState.SetRows(makeRemoteForwardingRows(m.portForwards))
	}
}

func (m *PortForwardingModal) Render() modal.ContentBlock {
	title := "Remote Port Forwards"
	if m.isLocal {
		title = "Local Port Forwards"
	}

	frame := components.Frame{
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

	return modal.ContentBlock{
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

func NewEditLocalForwardModal() modal.Window {
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

func (m *EditLocalForwardModal) Update(msg tea.Msg) (modal.Window, tea.Cmd) {
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
					return errormodal.NewErrorModal("Error", "unable to create port forward:\n"+err.Error()), nil
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

func (m EditLocalForwardModal) Render() modal.ContentBlock {
	frame := components.Frame{
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

	return modal.ContentBlock{
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

func NewEditRemoteForwardModal() modal.Window {
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

func (m *EditRemoteForwardModal) Update(msg tea.Msg) (modal.Window, tea.Cmd) {
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
					return errormodal.NewErrorModal("Error", "unable to create port forward:\n"+err.Error()), nil
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

func (m EditRemoteForwardModal) Render() modal.ContentBlock {
	frame := components.Frame{
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

	return modal.ContentBlock{
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

package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/skubalj/switchboard/config"
	"github.com/skubalj/switchboard/messaging"
	"github.com/skubalj/switchboard/portforwarding"
	"github.com/skubalj/switchboard/ringbuffer"
	"github.com/skubalj/switchboard/tui/style"
)

type Model struct {
	// Application State
	ctx         context.Context
	ctxCancel   func()
	msgTx       messaging.Tx
	msgRx       messaging.Rx
	connections []connectionRow
	config      config.Config

	// UI State
	viewportWidth   int
	viewportHeight  int
	keyMap          keyMap
	help            help.Model
	logs            ringbuffer.RingBuffer[string]
	logsExpanded    bool
	logsViewport    viewport.Model
	connectionTable table.Model
	modalLayer      modal
}

type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	Connect       key.Binding
	Disconnect    key.Binding
	ExpandLogs    key.Binding
	ForwardLocal  key.Binding
	ForwardRemote key.Binding
	Cancel        key.Binding
	Exit          key.Binding
}

var DefaultKeyMap = keyMap{
	Up:            key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "Up")),
	Down:          key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "Down")),
	Connect:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Connect/Accept")),
	Disconnect:    key.NewBinding(key.WithKeys("delete"), key.WithHelp("Delete", "Disconnect")),
	ExpandLogs:    key.NewBinding(key.WithKeys("e"), key.WithHelp("E", "Expand Logs")),
	ForwardLocal:  key.NewBinding(key.WithKeys("l"), key.WithHelp("L", "Forward Local")),
	ForwardRemote: key.NewBinding(key.WithKeys("r"), key.WithHelp("R", "Forward Remote")),
	Cancel:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "Cancel")),
	Exit:          key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "Exit")),
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.Connect,
		k.Disconnect,
		k.ExpandLogs,
		k.ForwardLocal,
		k.ForwardRemote,
		k.Cancel,
		k.Exit,
	}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMap) FullHelp() [][]key.Binding {
	panic("unimplemented")
}

func InitialModel(verbose bool, cfg config.Config) tea.Model {
	ctx, cancel := context.WithCancel(context.TODO())
	tx, rx := messaging.NewChannels()

	connections := make([]connectionRow, 0, len(cfg.Connections))
	for _, conn := range cfg.Connections {
		connections = append(connections, connectionRowFromConfig(ctx, conn))
	}

	modal := &Model{
		ctx:             ctx,
		ctxCancel:       cancel,
		msgTx:           tx,
		msgRx:           rx,
		connections:     connections,
		config:          cfg,
		keyMap:          DefaultKeyMap,
		help:            help.New(),
		logs:            ringbuffer.New[string](100),
		logsViewport:    viewport.New(),
		connectionTable: newTable(),
	}
	modal.updateTableRows()

	return modal
}

type LogMessage string
type ConnectionEstablished uint32
type ConnectionDropped uint32
type NewLocalForward config.PortForward
type DeleteLocalForward int
type NewRemoteForward config.PortForward
type DeleteRemoteForward int
type Error struct {
	Title string
	Err   error
}

func (m Model) Init() tea.Cmd {
	return m.getLogMessage
}

func (m Model) getLogMessage() tea.Msg {
	return LogMessage(m.msgRx.NextMessage(m.ctx))
}

func (m Model) GetConfigConnections() []config.Connection {
	conns := make([]config.Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		lfs := make([]config.PortForward, 0, len(conn.LocalForwards))
		for _, fw := range conn.LocalForwards {
			lfs = append(lfs, config.PortForward{LocalAddr: fw.LocalAddr, RemoteAddr: fw.RemoteAddr})
		}
		rfs := make([]config.PortForward, 0, len(conn.RemoteForwards))
		for _, fw := range conn.RemoteForwards {
			lfs = append(lfs, config.PortForward{LocalAddr: fw.LocalAddr, RemoteAddr: fw.RemoteAddr})
		}
		conns = append(conns, config.Connection{
			Host: config.Host{
				Name:         conn.Name,
				User:         conn.User,
				Host:         conn.Host,
				Port:         conn.Port,
				IdentityFile: conn.SSHKey,
			},
			LocalForwards:  lfs,
			RemoteForwards: rfs,
		})
	}

	return conns
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// If we set a width on the help menu it can gracefully truncate
		// its view as needed.
		m.help.SetWidth(msg.Width)
		m.viewportWidth = msg.Width
		m.viewportHeight = msg.Height

	case tea.KeyPressMsg:
		if m.modalLayer != nil {
			var cmd tea.Cmd
			m.modalLayer, cmd = m.modalLayer.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, m.keyMap.Exit):
			m.ctxCancel()
			return m, tea.Quit

		case key.Matches(msg, m.keyMap.Up):
			if m.logsExpanded {
				m.logsViewport.ScrollUp(1)
			} else {
				m.connectionTable.MoveUp(1)
			}

		case key.Matches(msg, m.keyMap.Down):
			if m.logsExpanded {
				m.logsViewport.ScrollDown(1)
			} else {
				m.connectionTable.MoveDown(1)
			}

		case key.Matches(msg, m.keyMap.Connect):
			selectedIdx := m.connectionTable.Cursor()
			if selectedIdx >= len(m.connections) {
				m.modalLayer = NewCollectionModal(m.config.FetchSSHConfig)
			} else if m.connections[selectedIdx].Online {
				m.connections[selectedIdx].DropConnection()
			} else {
				selectedConnection := m.connections[selectedIdx]
				target := m.connectionTable.SelectedRow()[1]
				if selectedConnection.SSHKey != "" {
					target = "key " + filepath.Base(selectedConnection.SSHKey)
				}
				m.modalLayer = NewPasswordModal(target, selectedConnection.UID)
			}

		case key.Matches(msg, m.keyMap.Disconnect):
			selectedIdx := m.connectionTable.Cursor()
			if selectedIdx < len(m.connections) {
				if m.connections[selectedIdx].Online {
					m.modalLayer = NewErrorModal("Delete Error", "You cannot delete a config while it is connected.\nIf you are sure you want to delete this connection, disconnect it first.")
				} else {
					m.connections = slices.Delete(m.connections, selectedIdx, selectedIdx+1)
					m.updateTableRows()
				}
			}

		case key.Matches(msg, m.keyMap.ForwardLocal):
			selectedIdx := m.connectionTable.Cursor()
			if selectedIdx < len(m.connections) {
				m.modalLayer = NewLocalForwardingModal(m.connections[selectedIdx].LocalForwards)
			}

		case key.Matches(msg, m.keyMap.ForwardRemote):
			selectedIdx := m.connectionTable.Cursor()
			if selectedIdx < len(m.connections) {
				m.modalLayer = NewRemoteForwardingModal(m.connections[selectedIdx].RemoteForwards)
			}

		case key.Matches(msg, m.keyMap.ExpandLogs):
			m.logsExpanded = !m.logsExpanded

		case key.Matches(msg, m.keyMap.Cancel):
			m.logsExpanded = false
		}

	case LogMessage:
		if msg != "" {
			m.logs = m.logs.Append(string(msg))
			lines := m.logs.AsSlice()
			m.logsViewport.SetContentLines(lines)
			if !m.logsExpanded {
				m.logsViewport.SetYOffset(max(0, len(lines)-m.viewportHeight+2))
			}
			return m, m.getLogMessage
		}

	case connectionRow:
		m.connections = append(m.connections, msg)
		m.updateTableRows()

	case PasswordMessage:
		idx := slices.IndexFunc(m.connections, func(c connectionRow) bool { return c.UID == msg.ConnectionID })
		if idx < 0 {
			m.modalLayer = NewErrorModal("Error", "got password for unknown connection")
			return m, nil
		}
		row := m.connections[idx]
		connection := row.MakeConnection(msg.Password)
		ctx, dropCallback := context.WithCancel(m.ctx)
		m.connections[idx].DropConnection = dropCallback
		errCh := make(chan error)
		m.updateTableRows()

		return m, tea.Batch(
			func() tea.Msg {
				<-errCh
				return ConnectionDropped(row.UID)
			},
			func() tea.Msg {
				err := portforwarding.ConnectToClient(ctx, m.config, errCh, m.msgTx, connection)
				if err != nil {
					return Error{"Connection Error", err}
				}
				return ConnectionEstablished(row.UID)
			},
		)

	case ConnectionEstablished:
		for i, row := range m.connections {
			if row.UID == uint32(msg) {
				m.connections[i].Online = true
				m.updateTableRows()
				return m, nil
			}
		}

	case ConnectionDropped:
		for i, row := range m.connections {
			if row.UID == uint32(msg) {
				m.connections[i].Online = false
				m.updateTableRows()
				return m, nil
			}
		}

	case NewLocalForward:
		idx := m.connectionTable.Cursor()
		if idx >= len(m.connections) {
			return m, nil
		}
		arr := m.connections[idx].LocalForwards
		display, backend := NewPortForwardFromConfig(m.ctx, config.PortForward(msg))
		arr = append(arr, display)
		go func() { m.connections[idx].NewLocalForwards <- backend }()
		m.modalLayer = NewLocalForwardingModal(arr)
		m.connections[idx].LocalForwards = arr
		m.updateTableRows()

	case DeleteLocalForward:
		idx := m.connectionTable.Cursor()
		if idx >= len(m.connections) {
			return m, nil
		}
		portIdx := int(msg)
		arr := m.connections[idx].LocalForwards
		arr[portIdx].stopCallback()
		arr = slices.Delete(arr, portIdx, portIdx+1)
		m.modalLayer = NewLocalForwardingModal(arr)
		m.connections[idx].LocalForwards = arr
		m.updateTableRows()

	case NewRemoteForward:
		idx := m.connectionTable.Cursor()
		if idx >= len(m.connections) {
			return m, nil
		}
		arr := m.connections[idx].RemoteForwards
		display, backend := NewPortForwardFromConfig(m.ctx, config.PortForward(msg))
		arr = append(arr, display)
		go func() { m.connections[idx].NewRemoteForwards <- backend }()
		m.modalLayer = NewLocalForwardingModal(arr)
		m.connections[idx].RemoteForwards = arr
		m.updateTableRows()

	case DeleteRemoteForward:
		idx := m.connectionTable.Cursor()
		if idx >= len(m.connections) {
			return m, nil
		}
		portIdx := int(msg)
		arr := m.connections[idx].RemoteForwards
		arr[portIdx].stopCallback()
		arr = slices.Delete(arr, portIdx, portIdx+1)
		m.modalLayer = NewLocalForwardingModal(arr)
		m.connections[idx].RemoteForwards = arr
		m.updateTableRows()

	case Error:
		m.modalLayer = NewErrorModal(msg.Title, msg.Err.Error())
		return m, nil
	}

	return m, nil
}

func (m *Model) updateTableRows() {
	m.connectionTable.SetRows(tableRows(m.connections))
}

func (m *Model) View() tea.View {
	if m.ctx.Err() != nil {
		return tea.NewView("Goodbye from switchboard!\n")
	}

	// View Setup
	var v tea.View
	v.AltScreen = true
	v.WindowTitle = "switchboard"

	if m.logsExpanded {
		v.SetContent(m.showFullLogs())
		return v
	}

	// Content
	content := m.mainContent()
	if m.modalLayer != nil {
		modal := m.modalLayer.Render()

		content = lipgloss.NewCompositor(
			lipgloss.NewLayer(content),
			lipgloss.NewLayer(modal.Content).
				X(m.viewportWidth/2-modal.Width/2).
				Y(m.viewportHeight/2-modal.Height/2).
				Z(1),
		).Render()
	}

	v.SetContent(content)
	return v
}

func (m Model) showFullLogs() string {
	frame := Frame{
		Title:  "Logs",
		Width:  m.viewportWidth,
		Height: m.viewportHeight,
	}

	m.logsViewport.SetWidth(frame.InnerWidth())
	m.logsViewport.SetHeight(frame.InnerHeight())

	return frame.Render(m.logsViewport.View())
}

func (m Model) mainContent() string {
	var content strings.Builder

	logsFrameDefaultHeight := 8
	connectionsFrameHeight := max(6, min(20, m.viewportHeight-1-logsFrameDefaultHeight))
	logsFrameHeight := max(3, m.viewportHeight-connectionsFrameHeight-1)

	connectionsFrame := Frame{
		Title:  "Active Connections",
		Width:  m.viewportWidth,
		Height: connectionsFrameHeight,
	}

	m.connectionTable.SetColumns(makeColumns(connectionsFrame.InnerWidth()))
	m.connectionTable.SetWidth(connectionsFrame.InnerWidth())
	m.connectionTable.SetHeight(connectionsFrame.InnerHeight())

	fmt.Fprintln(&content, connectionsFrame.Render(m.connectionTable.View()))

	logsFrame := Frame{
		Title:  "Logs",
		Width:  m.viewportWidth,
		Height: logsFrameHeight,
	}

	var logs strings.Builder
	logsSlice := m.logs.AsSlice()
	for _, logEntry := range logsSlice[max(0, len(logsSlice)-logsFrame.InnerHeight()):] {
		logs.WriteString(logEntry)
		logs.WriteRune('\n')
	}

	fmt.Fprintln(&content, logsFrame.Render(strings.Trim(logs.String(), "\n")))
	fmt.Fprintln(&content, m.help.View(m.keyMap))

	return content.String()
}

type Frame struct {
	Title    string
	Width    int
	Height   int
	PaddingX int
	PaddingY int
}

func (f Frame) InnerWidth() int {
	return f.Width - 2 - (2 * f.PaddingX)
}

func (f Frame) InnerHeight() int {
	return f.Height - 2 - (2 * f.PaddingY)
}

func (f Frame) Render(content string) string {
	titleContent := style.Header.Padding(0, 1).Render(f.Title)

	sizedContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		Padding(f.PaddingY, f.PaddingX).
		Render(lipgloss.Place(f.InnerWidth(), f.InnerHeight(), lipgloss.Top, lipgloss.Left, content))

	return lipgloss.NewCompositor(
		lipgloss.NewLayer(titleContent).X(2).Z(1),
		lipgloss.NewLayer(sizedContent),
	).Render()
}

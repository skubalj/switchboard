package tui

import (
	"context"
	"fmt"
	"os"
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
	"github.com/skubalj/switchboard/tui/components"
	"github.com/skubalj/switchboard/tui/connectiontable"
	"github.com/skubalj/switchboard/tui/modal"
	"github.com/skubalj/switchboard/tui/modal/errormodal"
	"github.com/skubalj/switchboard/tui/modal/hostsmodal"
	"github.com/skubalj/switchboard/tui/modal/passwordmodal"
	"github.com/skubalj/switchboard/tui/modal/portforwardmodal"
	"github.com/skubalj/switchboard/tui/utils"
)

type Model struct {
	// Application State
	ctx               context.Context
	ctxCancel         context.CancelFunc
	msgTx             messaging.Tx
	msgRx             messaging.Rx
	logFileCh         chan string
	getPassword       chan portforwarding.GetPasswordRequest
	connectionFactory portforwarding.ConnectionFactory
	connections       []*connectiontable.ConnectionRow
	config            config.Config

	// UI State
	viewportWidth   int
	viewportHeight  int
	help            help.Model
	logs            ringbuffer.RingBuffer[string]
	logsExpanded    bool
	logsViewport    viewport.Model
	connectionTable table.Model
	modalLayer      modal.Window
}

type keyMap struct {
	Up               key.Binding
	Down             key.Binding
	Connect          key.Binding
	DeleteConnection key.Binding
	ReorderUp        key.Binding
	ReorderDown      key.Binding
	EditConnection   key.Binding
	ExpandLogs       key.Binding
	ForwardLocal     key.Binding
	ForwardRemote    key.Binding
	Cancel           key.Binding
	Exit             key.Binding
}

var MainKeyMap = keyMap{
	Up:               key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "Up")),
	Down:             key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "Down")),
	Connect:          key.NewBinding(key.WithKeys("space"), key.WithHelp("Space", "Connect/Disconnect")),
	DeleteConnection: key.NewBinding(key.WithKeys("delete"), key.WithHelp("Delete", "Remove Connection")),
	ReorderUp:        key.NewBinding(key.WithKeys("pgup"), key.WithHelp("PgUp", "Reorder Up")),
	ReorderDown:      key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("PgDown", "Reorder Down")),
	EditConnection:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Edit Connection")),
	ExpandLogs:       key.NewBinding(key.WithKeys("e"), key.WithHelp("E", "Expand Logs")),
	ForwardLocal:     key.NewBinding(key.WithKeys("l"), key.WithHelp("L", "Local Forwards")),
	ForwardRemote:    key.NewBinding(key.WithKeys("r"), key.WithHelp("R", "Remote Forwards")),
	Cancel:           key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "Cancel")),
	Exit:             key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "Exit")),
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.ReorderUp,
		k.ReorderDown,
		k.Connect,
		k.DeleteConnection,
		k.EditConnection,
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

func InitialModel(logConfig config.LogConfig, cfg config.Config) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	tx, rx := messaging.NewChannels(logConfig.Level())

	var logFileCh chan string
	if logConfig.LogFile != "" {
		f, err := os.Create(logConfig.LogFile)
		if err != nil {
			tx.SendError(fmt.Errorf("unable to open log file '%s': %w", logConfig.LogFile, err))
		} else {
			logFileCh = make(chan string)
			go writeLogFile(logFileCh, f)
		}
	}

	getPassword := make(chan portforwarding.GetPasswordRequest)
	connectionFactory, err := portforwarding.New(cfg, tx, getPassword)
	if err != nil {
		tx.SendError(err)
	}

	connections := make([]*connectiontable.ConnectionRow, 0, len(cfg.Connections))
	for _, conn := range cfg.Connections {
		connections = append(connections, connectiontable.ConnectionRowFromConfig(ctx, conn))
	}

	modal := &Model{
		ctx:               ctx,
		ctxCancel:         cancel,
		msgTx:             tx,
		msgRx:             rx,
		logFileCh:         logFileCh,
		getPassword:       getPassword,
		connectionFactory: connectionFactory,
		connections:       connections,
		config:            cfg,
		help:              help.New(),
		logs:              ringbuffer.New[string](100),
		logsViewport:      viewport.New(),
		connectionTable:   connectiontable.NewTable(),
	}
	modal.updateTableRows()

	return modal
}

type ConnectionEstablished struct {
	UID          uint32
	OnDisconnect <-chan struct{}
}

type ConnectionDropped uint32

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.getLogMessage,
		m.getPasswordRequest,
	)
}

func (m *Model) Close() {
	m.ctxCancel()
	m.msgTx.Close()
	if m.logFileCh != nil {
		close(m.logFileCh)
	}
}

func (m *Model) getLogMessage() tea.Msg      { return m.msgRx.NextMessage(m.ctx) }
func (m *Model) getPasswordRequest() tea.Msg { return <-m.getPassword }

func (m *Model) GetConfigConnections() []config.Connection {
	conns := make([]config.Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		lfs := make([]config.PortForward, 0, len(conn.LocalForwards))
		for _, fw := range conn.LocalForwards {
			lfs = append(lfs, fw.AsConfig())
		}

		rfs := make([]config.PortForward, 0, len(conn.RemoteForwards))
		for _, fw := range conn.RemoteForwards {
			rfs = append(rfs, fw.AsConfig())
		}

		hosts := make([]config.Host, 0, len(conn.Hosts))
		for _, host := range conn.Hosts {
			hosts = append(hosts, config.Host{
				User:         host.User,
				Host:         host.Host,
				Port:         host.Port,
				IdentityFile: host.SSHKey,
			})
		}

		conns = append(conns, config.Connection{
			Name:           conn.Name,
			Hosts:          hosts,
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
		case key.Matches(msg, MainKeyMap.Exit):
			m.ctxCancel()
			return m, tea.Quit

		case key.Matches(msg, MainKeyMap.Up):
			if m.logsExpanded {
				m.logsViewport.ScrollUp(1)
			} else {
				m.connectionTable.MoveUp(1)
			}

		case key.Matches(msg, MainKeyMap.Down):
			if m.logsExpanded {
				m.logsViewport.ScrollDown(1)
			} else {
				m.connectionTable.MoveDown(1)
			}

		case key.Matches(msg, MainKeyMap.ReorderUp):
			if !m.logsExpanded && utils.ReorderUp(m.connections, m.connectionTable.Cursor()) {
				m.connectionTable.MoveUp(1)
				m.updateTableRows()
			}

		case key.Matches(msg, MainKeyMap.ReorderDown):
			if !m.logsExpanded && utils.ReorderDown(m.connections, m.connectionTable.Cursor()) {
				m.connectionTable.MoveDown(1)
				m.updateTableRows()
			}

		case key.Matches(msg, MainKeyMap.EditConnection):
			selectedIdx := m.connectionTable.Cursor()
			if !m.logsExpanded {
				if selectedIdx < len(m.connections) {
					connection := m.connections[selectedIdx]
					if connection.Online {
						m.modalLayer = errormodal.NewErrorModal("Error", "You cannot edit a connection while it is active.\nDisconnect, then try again.")
					} else {
						m.modalLayer = hostsmodal.EditConnectionHostsModal(connection.Name, connection.Hosts, m.config.FetchSSHConfig)
					}
				} else {
					m.modalLayer = hostsmodal.NewConnectionHostsModal(m.config.FetchSSHConfig)
				}
			}

		case key.Matches(msg, MainKeyMap.Connect):
			selectedIdx := m.connectionTable.Cursor()
			if selectedIdx >= len(m.connections) {
				m.modalLayer = hostsmodal.NewConnectionHostsModal(m.config.FetchSSHConfig)
			} else if m.connections[selectedIdx].Online {
				m.connections[selectedIdx].DropConnection()
			} else {
				selectedConnection := m.connections[selectedIdx]
				selectedConnection.SetContext(m.ctx)
				onClose := make(chan struct{})
				conn := selectedConnection.MakeConnection(onClose)

				return m, func() tea.Msg {
					err := m.connectionFactory.ConnectToClient(selectedConnection.Ctx, conn)
					if err != nil {
						return errormodal.ErrorMsg{
							Title: fmt.Sprintf("Error Connecting To %s", selectedConnection.Name),
							Err:   err,
						}
					}

					return ConnectionEstablished{
						UID:          selectedConnection.UID,
						OnDisconnect: onClose,
					}
				}
			}

		case key.Matches(msg, MainKeyMap.DeleteConnection):
			selectedIdx := m.connectionTable.Cursor()
			if selectedIdx < len(m.connections) {
				if m.connections[selectedIdx].Online {
					m.modalLayer = errormodal.NewErrorModal("Delete Error", "You cannot delete a config while it is connected.\nIf you are sure you want to delete this connection, disconnect it first.")
				} else {
					m.connections = slices.Delete(m.connections, selectedIdx, selectedIdx+1)
					m.updateTableRows()
				}
			}

		case key.Matches(msg, MainKeyMap.ForwardLocal):
			selectedIdx := m.connectionTable.Cursor()
			if selectedIdx < len(m.connections) {
				m.modalLayer = portforwardmodal.NewLocalForwardingModal(m.connections[selectedIdx].LocalForwards)
			}

		case key.Matches(msg, MainKeyMap.ForwardRemote):
			selectedIdx := m.connectionTable.Cursor()
			if selectedIdx < len(m.connections) {
				m.modalLayer = portforwardmodal.NewRemoteForwardingModal(m.connections[selectedIdx].RemoteForwards)
			}

		case key.Matches(msg, MainKeyMap.ExpandLogs):
			m.logsExpanded = !m.logsExpanded

		case key.Matches(msg, MainKeyMap.Cancel):
			m.logsExpanded = false
		}

	case messaging.Message:
		if !msg.IsZero() {
			m.logs = m.logs.Append(msg.FormatMessage())
			lines := m.logs.AsSlice()
			m.logsViewport.SetContentLines(lines)
			if !m.logsExpanded {
				m.logsViewport.SetYOffset(max(0, len(lines)-m.viewportHeight+2))
			}
			if m.logFileCh != nil {
				m.logFileCh <- msg.FormatMessageNoColor()
			}
			return m, m.getLogMessage
		}

	case *connectiontable.ConnectionRow:
		selectedIdx := m.connectionTable.Cursor()
		if selectedIdx < len(m.connections) {
			m.connections[selectedIdx] = msg
		} else {
			m.connections = append(m.connections, msg)
		}

		m.updateTableRows()

	case portforwarding.GetPasswordRequest:
		m.modalLayer = passwordmodal.NewPasswordModal(msg.Comment, msg.Response)

	case ConnectionEstablished:
		row := m.getConnectionByUID(msg.UID)
		if row != nil {
			row.Online = true
			row.StartPortForwards()
			m.updateTableRows()
			// Rig the disconnect handler
			return m, func() tea.Msg {
				<-msg.OnDisconnect
				return ConnectionDropped(msg.UID)
			}
		}

	case ConnectionDropped:
		row := m.getConnectionByUID(uint32(msg))
		if row != nil {
			row.Online = false
			m.updateTableRows()
			return m, nil
		}

	case portforwardmodal.NewLocalForward:
		idx := m.connectionTable.Cursor()
		if idx >= len(m.connections) {
			return m, nil
		}
		arr := m.connections[idx].LocalForwards
		pf := portforwardmodal.NewPortForwardFromConfig(m.ctx, config.PortForward(msg))
		arr = append(arr, pf)
		go func() { m.connections[idx].NewLocalForwards <- pf }()
		m.modalLayer = portforwardmodal.NewLocalForwardingModal(arr)
		m.connections[idx].LocalForwards = arr
		m.updateTableRows()

	case portforwardmodal.DeleteLocalForward:
		idx := m.connectionTable.Cursor()
		if idx >= len(m.connections) {
			return m, nil
		}
		portIdx := int(msg)
		arr := m.connections[idx].LocalForwards
		arr[portIdx].Close()
		arr = slices.Delete(arr, portIdx, portIdx+1)
		m.modalLayer = portforwardmodal.NewLocalForwardingModal(arr)
		m.connections[idx].LocalForwards = arr
		m.updateTableRows()

	case portforwardmodal.NewRemoteForward:
		idx := m.connectionTable.Cursor()
		if idx >= len(m.connections) {
			return m, nil
		}
		arr := m.connections[idx].RemoteForwards
		pf := portforwardmodal.NewPortForwardFromConfig(m.ctx, config.PortForward(msg))
		arr = append(arr, pf)
		go func() { m.connections[idx].NewRemoteForwards <- pf }()
		m.modalLayer = portforwardmodal.NewRemoteForwardingModal(arr)
		m.connections[idx].RemoteForwards = arr
		m.updateTableRows()

	case portforwardmodal.DeleteRemoteForward:
		idx := m.connectionTable.Cursor()
		if idx >= len(m.connections) {
			return m, nil
		}
		portIdx := int(msg)
		arr := m.connections[idx].RemoteForwards
		arr[portIdx].Close()
		arr = slices.Delete(arr, portIdx, portIdx+1)
		m.modalLayer = portforwardmodal.NewRemoteForwardingModal(arr)
		m.connections[idx].RemoteForwards = arr
		m.updateTableRows()

	case errormodal.ErrorMsg:
		m.modalLayer = errormodal.NewErrorModal(msg.Title, msg.Err.Error())
		return m, nil
	}

	return m, nil
}

func (m *Model) getConnectionByUID(uid uint32) *connectiontable.ConnectionRow {
	for idx, conn := range m.connections {
		if conn.UID == uid {
			return m.connections[idx]
		}
	}
	return nil
}

func (m *Model) updateTableRows() {
	m.connectionTable.SetRows(connectiontable.TableRows(m.connections))
}

func (m *Model) View() tea.View {
	if m.ctx.Err() != nil {
		return tea.NewView("")
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
	frame := components.Frame{
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

	connectionsFrame := components.Frame{
		Title:  "Active Connections",
		Width:  m.viewportWidth,
		Height: connectionsFrameHeight,
	}

	m.connectionTable.SetColumns(connectiontable.MakeColumns(connectionsFrame.InnerWidth()))
	m.connectionTable.SetWidth(connectionsFrame.InnerWidth())
	m.connectionTable.SetHeight(connectionsFrame.InnerHeight())

	fmt.Fprintln(&content, connectionsFrame.Render(m.connectionTable.View()))

	logsFrame := components.Frame{
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
	fmt.Fprintln(&content, lipgloss.NewStyle().Padding(0, 1).Render(m.help.View(MainKeyMap)))

	return content.String()
}

func writeLogFile(rx <-chan string, file *os.File) {
	defer file.Close()
	for line := range rx {
		fmt.Fprintln(file, line)
	}
}

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

type modal interface {
	Update(msg tea.Msg) (modal, tea.Cmd)
	Render() contentBlock
}

type contentBlock struct {
	Content string
	Width   int
	Height  int
}

type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	Select        key.Binding
	Apply         key.Binding
	ExpandLogs    key.Binding
	ForwardLocal  key.Binding
	ForwardRemote key.Binding
	Cancel        key.Binding
	Exit          key.Binding
}

var DefaultKeyMap = keyMap{
	Up:            key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "Up")),
	Down:          key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "Down")),
	Select:        key.NewBinding(key.WithKeys(" "), key.WithHelp("Space", "Select")),
	Apply:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Apply")),
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
		k.Select,
		k.ExpandLogs,
		k.ForwardLocal,
		k.ForwardRemote,
		k.Apply,
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
		connections = append(connections, connectionRowFromConfig(conn))
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

		case key.Matches(msg, m.keyMap.Apply):
			selectedRow := m.connectionTable.SelectedRow()
			selectedIdx := m.connectionTable.Cursor()
			switch selectedRow[0] {
			case "+":
				m.modalLayer = NewCollectionModal(m.config.FetchSSHConfig)
			case "✅":
				m.connections[selectedIdx].DropConnection()
			default:
				selectedConnection := m.connections[selectedIdx]
				target := selectedRow[1]
				if selectedConnection.SSHKey != "" {
					target = filepath.Base(selectedConnection.SSHKey)
				}
				m.modalLayer = NewPasswordModal(target, selectedConnection.UID)
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
		errCh := make(chan error)
		err := portforwarding.ConnectToClient(ctx, errCh, m.msgTx, connection)
		if err != nil {
			dropCallback()
			m.modalLayer = NewErrorModal("Connection Error", err.Error())
			return m, nil
		}

		m.connections[idx].DropConnection = dropCallback
		m.connections[idx].Online = true
		m.updateTableRows()

		return m, func() tea.Msg {
			<-errCh
			return ConnectionDropped(row.UID)
		}

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
		Title:    "Logs",
		Width:    m.viewportWidth,
		Height:   m.viewportHeight,
		PaddingX: 1,
	}

	m.logsViewport.SetWidth(frame.InnerWidth())
	m.logsViewport.SetHeight(frame.InnerHeight())

	return frame.Render(m.logsViewport.View())
}

func (m Model) mainContent() string {
	var content strings.Builder

	logsFrameHeight := 8

	connectionsFrame := Frame{
		Title:  "Active Connections",
		Width:  m.viewportWidth,
		Height: m.viewportHeight - 1 - logsFrameHeight,
	}

	m.connectionTable.SetColumns(makeColumns(connectionsFrame.InnerWidth()))
	m.connectionTable.SetWidth(connectionsFrame.InnerWidth())
	m.connectionTable.SetHeight(connectionsFrame.InnerHeight())

	fmt.Fprintln(&content, connectionsFrame.Render(m.connectionTable.View()))

	logsFrame := Frame{
		Title:    "Logs",
		Width:    m.viewportWidth,
		Height:   logsFrameHeight,
		PaddingX: 1,
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

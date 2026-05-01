package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/skubalj/switchboard/messaging"
	"github.com/skubalj/switchboard/portforwarding"
	"github.com/skubalj/switchboard/ringbuffer"
)

type model struct {
	// Application State
	ctx         context.Context
	ctxCancel   func()
	msgTx       messaging.Tx
	msgRx       messaging.Rx
	connections []portforwarding.ConnectionConfig

	// UI State
	viewportWidth   int
	viewportHeight  int
	keyMap          keyMap
	help            help.Model
	logs            ringbuffer.RingBuffer[string]
	logsViewport    viewport.Model
	connectionTable table.Model
}

type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Apply  key.Binding
	Cancel key.Binding
	Exit   key.Binding
}

var DefaultKeyMap = keyMap{
	Up:     key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "Up")),
	Down:   key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "Down")),
	Select: key.NewBinding(key.WithKeys(" "), key.WithHelp("Space", "Select")),
	Apply:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Apply")),
	Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "Cancel")),
	Exit:   key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "Exit")),
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.Select,
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

func InitialModel() tea.Model {
	ctx, cancel := context.WithCancel(context.TODO())
	tx, rx := messaging.NewChannels()

	return model{
		ctx:             ctx,
		ctxCancel:       cancel,
		msgTx:           tx,
		msgRx:           rx,
		connections:     nil,
		keyMap:          DefaultKeyMap,
		help:            help.New(),
		logs:            ringbuffer.New[string](100),
		logsViewport:    viewport.New(),
		connectionTable: newTable(),
	}
}

type LogMessage string

func (m model) Init() tea.Cmd {
	return m.getLogMessage
}

func (m model) getLogMessage() tea.Msg {
	return LogMessage(m.msgRx.NextMessage(m.ctx))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// If we set a width on the help menu it can gracefully truncate
		// its view as needed.
		m.help.SetWidth(msg.Width)
		m.viewportWidth = msg.Width
		m.viewportHeight = msg.Height

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Exit):
			m.ctxCancel()
			return m, tea.Quit
		case key.Matches(msg, m.keyMap.Up):
			m.connectionTable.MoveUp(1)
		case key.Matches(msg, m.keyMap.Down):
			m.connectionTable.MoveDown(1)
		}

	case LogMessage:
		if msg != "" {
			m.logs = m.logs.Append(string(msg))
			m.logsViewport.SetContentLines(m.logs.AsSlice())
			return m, m.getLogMessage
		}
	}

	return m, nil
}

func (m model) View() tea.View {
	if m.ctx.Err() != nil {
		return tea.NewView("Goodbye from switchboard!\n")
	}

	// View Setup
	var v tea.View
	v.AltScreen = true
	v.WindowTitle = "switchboard"

	// Content
	var content strings.Builder

	connectionsFrame := Frame{
		Title:  "Active Connections",
		Width:  m.viewportWidth,
		Height: max(8, len(m.connections)),
	}

	m.connectionTable.SetColumns(makeColumns(connectionsFrame.InnerWidth()))
	m.connectionTable.SetWidth(connectionsFrame.InnerWidth())
	m.connectionTable.SetHeight(connectionsFrame.InnerHeight())

	fmt.Fprintln(&content, connectionsFrame.Render(m.connectionTable.View()))

	logsFrame := Frame{
		Title:  "Logs",
		Width:  m.viewportWidth,
		Height: m.viewportHeight - connectionsFrame.Height - 1, // -1 for the footer we haven't written yet
	}

	m.logsViewport.SetWidth(logsFrame.InnerWidth())
	m.logsViewport.SetHeight(logsFrame.InnerHeight())

	fmt.Fprintln(&content, logsFrame.Render(m.logsViewport.View()))
	fmt.Fprintln(&content, m.help.View(m.keyMap))

	v.SetContent(content.String())
	return v
}

func makeColumns(width int) []table.Column {
	width -= 6
	dividedWidth := width / 4
	remainder := width % dividedWidth

	return []table.Column{
		{Title: "", Width: 3},
		{Title: "Connection", Width: dividedWidth + remainder - 1},
		{Title: "SSH Key", Width: dividedWidth - 2},
		{Title: "Local Ports", Width: dividedWidth - 2},
		{Title: "Remote Ports", Width: dividedWidth - 2},
	}
}

func tableRows(cons []portforwarding.ConnectionConfig) []table.Row {
	rows := make([]table.Row, 0, len(cons))
	rows = append(rows, table.Row{
		fmt.Sprintf("%3d", 1),
		"user@hostname",
		"",
		"localPorts",
		"remotePorts",
	})
	rows = append(rows, table.Row{
		fmt.Sprintf("%3d", 2),
		"user@192.168.0.1:2222",
		"",
		"localPorts",
		"remotePorts",
	})
	for idx, connection := range cons {
		rows = append(rows, table.Row{
			fmt.Sprintf("%3d", idx+1),
			connection.User + "@" + connection.Host,
			"",
			"localPorts",
			"remotePorts",
		})
	}

	rows = append(rows, table.Row{
		"  +",
		"New Connection",
		"",
		"",
		"",
	})

	return rows
}

func newTable() table.Model {
	connectionTable := table.New(
		table.WithColumns(makeColumns(100)),
		table.WithRows(tableRows(nil)),
		table.WithFocused(false),
		table.WithWidth(100),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.BrightBlue).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Black).
		Background(lipgloss.BrightBlue).
		Bold(false)
	connectionTable.SetStyles(s)

	return connectionTable
}

type Frame struct {
	Title  string
	Width  int
	Height int
}

func (f Frame) InnerWidth() int {
	return f.Width - 2
}

func (f Frame) InnerHeight() int {
	return f.Height - 2
}

func (f Frame) Render(content string) string {
	titleContent := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.BrightBlue).
		Padding(0, 2).
		Render(f.Title)

	sizedContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		Render(lipgloss.Place(f.InnerWidth(), f.InnerHeight(), lipgloss.Top, lipgloss.Left, content))

	return lipgloss.NewCompositor(
		lipgloss.NewLayer(titleContent).X(2).Z(1),
		lipgloss.NewLayer(sizedContent),
	).Render()
}

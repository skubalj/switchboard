package hostsmodal

import (
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/skubalj/switchboard/config"
	"github.com/skubalj/switchboard/tui/components"
	"github.com/skubalj/switchboard/tui/connectiontable"
	"github.com/skubalj/switchboard/tui/modal"
	"github.com/skubalj/switchboard/tui/modal/connectionmodal"
	"github.com/skubalj/switchboard/tui/style"
)

type HostsModal struct {
	configLookup   func(string) (config.Host, error)
	hosts          []connectiontable.ConnectionHost
	tableState     table.Model
	selectedIdx    int
	connectionName textinput.Model
	buttons        []string
	subModal       *connectionmodal.ConnectionModal
}

func NewHostsModal(hosts []connectiontable.ConnectionHost, configLookup func(string) (config.Host, error)) modal.Window {
	connectionName := textinput.New()
	connectionName.Prompt = "> "
	connectionName.Focus()
	connectionName.SetVirtualCursor(true)
	connectionName.SetStyles(style.InputBox)

	tableState := table.New(
		table.WithFocused(false),
		table.WithStyles(style.TableNoSelection),
		table.WithColumns(makeColumns(64)),
		table.WithRows(makeRows(hosts)),
	)

	return &HostsModal{
		configLookup:   configLookup,
		hosts:          hosts,
		tableState:     tableState,
		connectionName: connectionName,
		buttons:        []string{"Load From Config", "Save", "Cancel"},
	}
}

func (m *HostsModal) Update(msg tea.Msg) (modal.Window, tea.Cmd) {
	if m.subModal != nil {
		modal, host, cmd := m.subModal.Update(msg)
		if cmd != nil {
			return nil, cmd
		} else if modal == nil {
			m.hosts = append(m.hosts, host)
		}
		m.subModal = modal
		m.tableState.SetRows(makeRows(m.hosts))
		return m, nil
	}

	m.connectionName, _ = m.connectionName.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, hostModalKeyMap.Cancel):
			return nil, nil
		case key.Matches(msg, hostModalKeyMap.Quit):
			return nil, tea.Quit
		case key.Matches(msg, hostModalKeyMap.Apply):
			switch m.selectedIdx {
			case 1:
				if m.tableState.Cursor() >= len(m.hosts) {
					m.subModal = connectionmodal.NewConnectionModal(m.configLookup)
				}
			case 2:
				// load
			case 3:
				row := m.createConnectionRow()
				return nil, func() tea.Msg { return row }
			case 4:
				return nil, nil
			}

		case key.Matches(msg, hostModalKeyMap.Delete):
			idx := m.tableState.Cursor()
			if idx < len(m.hosts) {
				m.hosts = slices.Delete(m.hosts, idx, idx+1)
				m.tableState.SetRows(makeRows(m.hosts))
			}

		case key.Matches(msg, hostModalKeyMap.ReorderUp):
			if len(m.hosts) > 1 {
				idx := m.tableState.Cursor()
				if idx < len(m.hosts) && idx > 0 && len(m.hosts) > 1 {
					m.hosts[idx], m.hosts[idx-1] = m.hosts[idx-1], m.hosts[idx]
					m.tableState.MoveUp(1)
				}
				m.tableState.SetRows(makeRows(m.hosts))
			}

		case key.Matches(msg, hostModalKeyMap.ReorderDown):
			if len(m.hosts) > 1 {
				idx := m.tableState.Cursor()
				if idx < len(m.hosts)-1 {
					m.hosts[idx], m.hosts[idx+1] = m.hosts[idx+1], m.hosts[idx]
					m.tableState.MoveDown(1)
				}
				m.tableState.SetRows(makeRows(m.hosts))
			}

		case key.Matches(msg, hostModalKeyMap.Up):
			if m.selectedIdx == 1 {
				if m.tableState.Cursor() == 0 {
					m.selectedIdx = 0
				} else {
					m.tableState.MoveUp(1)
				}
			} else if m.selectedIdx > 1 {
				m.selectedIdx = 1
				m.tableState.GotoBottom()
			}
			m.updateFocus()

		case key.Matches(msg, hostModalKeyMap.Down):
			switch m.selectedIdx {
			case 0:
				m.selectedIdx = 1
				m.tableState.GotoTop()
			case 1:
				if m.tableState.Cursor() < len(m.hosts)-1 {
					m.tableState.MoveDown(1)
				} else {
					m.selectedIdx = 2
				}
			}
			m.updateFocus()

		case key.Matches(msg, hostModalKeyMap.Right):
			if m.selectedIdx >= 2 && m.selectedIdx < len(m.buttons)+1 {
				m.selectedIdx++
			}

		case key.Matches(msg, hostModalKeyMap.Left):
			if m.selectedIdx > 2 {
				m.selectedIdx--
			}

		case key.Matches(msg, hostModalKeyMap.Next):
			if m.selectedIdx < 1+len(m.buttons) {
				m.selectedIdx++
			}
			m.updateFocus()

		case key.Matches(msg, hostModalKeyMap.Previous):
			if m.selectedIdx > 0 {
				m.selectedIdx--
			}
			m.updateFocus()
		}
	}

	return m, nil
}

func (m *HostsModal) createConnectionRow() connectiontable.ConnectionRow {
	return connectiontable.NewConnectionRow(m.connectionName.Value(), m.hosts)
}

func (m *HostsModal) updateFocus() {
	switch m.selectedIdx {
	case 0:
		m.connectionName.Focus()
		m.tableState.SetStyles(style.TableNoSelection)

	case 1:
		m.connectionName.Blur()
		m.tableState.SetStyles(style.Table)

	default:
		m.connectionName.Blur()
		m.tableState.SetStyles(style.TableNoSelection)
	}
}

func (m *HostsModal) Render() modal.ContentBlock {
	if m.subModal != nil {
		return m.subModal.Render()
	}

	frame := components.Frame{
		Width:    72,
		Height:   16,
		PaddingX: 1,
	}

	var content strings.Builder
	passwordFrame := components.Frame{
		Title:    "Connection Name",
		Width:    frame.InnerWidth(),
		Height:   3,
		PaddingX: 1,
	}
	m.connectionName.SetWidth(passwordFrame.InnerWidth() - 4)
	content.WriteString(passwordFrame.Render(m.connectionName.View()))
	content.WriteRune('\n')

	hostsFrame := components.Frame{
		Title:  "Hosts Path",
		Width:  frame.InnerWidth(),
		Height: frame.InnerHeight() - passwordFrame.Height - 1,
	}
	m.tableState.SetWidth(hostsFrame.InnerWidth())
	m.tableState.SetHeight(hostsFrame.InnerHeight())
	m.tableState.SetColumns(makeColumns(hostsFrame.InnerWidth()))
	content.WriteString(hostsFrame.Render(m.tableState.View()))
	content.WriteRune('\n')

	content.WriteString(m.buttonRow())

	return modal.ContentBlock{
		Content: frame.Render(content.String()),
		Width:   frame.Width,
		Height:  frame.Height,
	}
}

func (m *HostsModal) buttonRow() string {
	var buf strings.Builder
	btnIdx := m.selectedIdx - 2
	for idx, btn := range m.buttons {
		btn = "<" + btn + ">"
		if idx == btnIdx {
			btn = style.ButtonSelected.Render(btn)
		}
		buf.WriteString(btn)
		buf.WriteRune(' ')
	}
	return buf.String()
}

func makeColumns(width int) []table.Column {
	width -= 14
	numCols := 3
	divided := width / numCols
	remainder := width % numCols

	return []table.Column{
		{Title: "User", Width: divided},
		{Title: "Host", Width: divided + remainder},
		{Title: "Port", Width: 6},
		{Title: "SSH Key", Width: divided},
	}
}

func makeRows(hosts []connectiontable.ConnectionHost) []table.Row {
	rows := make([]table.Row, 0, len(hosts)+1)
	for _, host := range hosts {
		rows = append(rows, table.Row{
			host.User,
			host.Host,
			strconv.Itoa(int(host.Port)),
			host.SSHKey,
		})
	}

	rows = append(rows, table.Row{"+ Add Host", "", "", ""})
	return rows
}

type keyMap struct {
	Apply       key.Binding
	Cancel      key.Binding
	Quit        key.Binding
	Delete      key.Binding
	ReorderUp   key.Binding
	ReorderDown key.Binding
	Next        key.Binding
	Previous    key.Binding
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
}

var hostModalKeyMap = keyMap{
	Apply:       key.NewBinding(key.WithKeys("enter")),
	Cancel:      key.NewBinding(key.WithKeys("esc")),
	Quit:        key.NewBinding(key.WithKeys("ctrl+c")),
	Delete:      key.NewBinding(key.WithKeys("del")),
	ReorderUp:   key.NewBinding(key.WithKeys("pgup")),
	ReorderDown: key.NewBinding(key.WithKeys("pgdown")),
	Next:        key.NewBinding(key.WithKeys("tab")),
	Previous:    key.NewBinding(key.WithKeys("shift+tab")),
	Up:          key.NewBinding(key.WithKeys("up")),
	Down:        key.NewBinding(key.WithKeys("down")),
	Left:        key.NewBinding(key.WithKeys("left")),
	Right:       key.NewBinding(key.WithKeys("right")),
}

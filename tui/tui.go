package tui

import (
	tea "charm.land/bubbletea/v2"
	// "charm.land/lipgloss/v2"
	"github.com/skubalj/spfa/messaging"
	"github.com/skubalj/spfa/portforwarding"
)

type model struct {
	msgTx   messaging.Tx
	msgRx   messaging.Rx
	configs []portforwarding.SSHConfig
}

func initialModel(rows []portforwarding.SSHConfig) tea.Model {
	tx, rx := messaging.NewChannels()
	return model{
		msgTx:   tx,
		msgRx:   rx,
		configs: rows,
	}
}

func (model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return nil, nil
}

func (m model) View() tea.View {
	var v tea.View
	v.AltScreen = true

	return v
}

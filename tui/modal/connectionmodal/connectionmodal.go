package connectionmodal

import (
	"os/user"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/skubalj/switchboard/config"
	"github.com/skubalj/switchboard/tui/connectiontable"
	"github.com/skubalj/switchboard/tui/modal"
	"github.com/skubalj/switchboard/tui/modal/errormodal"
	"github.com/skubalj/switchboard/tui/modal/textinputmodal"
)

type ConnectionModal struct {
	configLookup configLookup
	inner        *textinputmodal.TextInputModal
}

type configLookup = func(string) (config.Host, error)

const (
	LoadButton = "Load From Config"
	Save       = "Save"
	Cancel     = "Cancel"
)

func NewConnectionModal(configLookup configLookup) *ConnectionModal {
	var username string
	{
		osUser, err := user.Current()
		if err == nil {
			username = osUser.Username
		}
	}

	inputs := []textinputmodal.TextInput{
		{Prompt: "Name: ", Placeholder: ""},
		{Prompt: "User: ", Placeholder: username},
		{Prompt: "Host: ", Placeholder: ""},
		{Prompt: "Port: ", Placeholder: "22"},
		{Prompt: "SSH Key: ", Placeholder: ""},
	}

	innerModal := textinputmodal.NewTextInputModal("New Connection", inputs, []string{LoadButton, Save, Cancel})
	return &ConnectionModal{configLookup: configLookup, inner: &innerModal}
}

func (m *ConnectionModal) Update(msg tea.Msg) (modal.Window, tea.Cmd) {
	button, state := m.inner.Update(msg)
	switch state {
	case textinputmodal.Ok:
		switch button {
		case LoadButton:
			m.setFromConfig()
		case Save:
			values := m.inner.GetValues()
			name := values[0]
			user := values[1]
			host := values[2]
			portString := values[3]
			sshKey := values[4]

			port, err := strconv.ParseUint(portString, 10, 16)
			if err != nil {
				return errormodal.NewErrorModal("Error", "port must be a number"), nil
			}

			return nil, func() tea.Msg {
				return connectiontable.NewConnectionRow(name, []connectiontable.ConnectionHost{
					{
						User:   user,
						Host:   host,
						Port:   uint16(port),
						SSHKey: sshKey,
					},
				})
			}
		case Cancel:
			return nil, nil
		}
	case textinputmodal.Cancel:
		return nil, nil
	case textinputmodal.Quit:
		return nil, tea.Quit
	}

	return m, nil
}

func (m *ConnectionModal) setFromConfig() {
	values := m.inner.GetValues()

	host, err := m.configLookup(values[0])
	if err != nil {
		m.inner.SetError(err.Error())
		return
	}

	values[1] = host.User
	values[2] = host.Host
	values[3] = strconv.Itoa(int(host.Port))
	values[4] = host.IdentityFile
	m.inner.SetInputs(values)
}

func (m *ConnectionModal) Render() modal.ContentBlock {
	return m.inner.Render()
}

package connectionmodal

import (
	"fmt"
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
	configLookup  configLookup
	defaultInputs []textinputmodal.TextInput
	selectedModal int
	inner         []textinputmodal.TextInputModal
}

type configLookup = func(string) (config.Host, error)

const (
	LoadButton = "Load From Config"
	Save       = "Save"
	Cancel     = "Cancel"
)

const (
	nameIdx = iota
	userIdx
	hostIdx
	portIdx
	sshKeyIdx
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
	return &ConnectionModal{
		configLookup:  configLookup,
		defaultInputs: inputs,
		inner:         []textinputmodal.TextInputModal{innerModal},
	}
}

func (m *ConnectionModal) Update(msg tea.Msg) (*ConnectionModal, connectiontable.ConnectionHost, tea.Cmd) {
	button, state := m.inner[m.selectedModal].Update(msg)
	switch state {
	case textinputmodal.Ok:
		switch button {
		case LoadButton:
			m.setFromConfig()
		case Save:
			values := m.inner[m.selectedModal].GetValues()
			port, err := strconv.ParseUint(values[portIdx], 10, 16)
			if err != nil {
				return nil, connectiontable.ConnectionHost{}, func() tea.Msg {
					return errormodal.ErrorMsg{
						Title: "Error",
						Err:   fmt.Errorf("port must be a number"),
					}
				}
			}

			return nil, connectiontable.ConnectionHost{
				User:   values[userIdx],
				Host:   values[hostIdx],
				Port:   uint16(port),
				SSHKey: values[sshKeyIdx],
			}, nil

		case Cancel:
			return nil, connectiontable.ConnectionHost{}, nil
		}
	case textinputmodal.Cancel:
		return nil, connectiontable.ConnectionHost{}, nil
	case textinputmodal.Quit:
		return nil, connectiontable.ConnectionHost{}, tea.Quit
	}

	return m, connectiontable.ConnectionHost{}, nil
}

func (m *ConnectionModal) setFromConfig() {
	values := m.inner[m.selectedModal].GetValues()

	host, err := m.configLookup(values[nameIdx])
	if err != nil {
		m.inner[m.selectedModal].SetError(err.Error())
		return
	}

	values[userIdx] = host.User
	values[hostIdx] = host.Host
	values[portIdx] = strconv.Itoa(int(host.Port))
	values[sshKeyIdx] = host.IdentityFile
	m.inner[m.selectedModal].SetInputs(values)
}

func (m *ConnectionModal) Render() modal.ContentBlock {
	return m.inner[m.selectedModal].Render()
}

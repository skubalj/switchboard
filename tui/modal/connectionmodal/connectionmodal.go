package connectionmodal

import (
	"fmt"
	"os/user"
	"strconv"
	"sync"

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

type configLookup = func(string) ([]config.Host, error)

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

var username string
var setUsernameOnce sync.Once

func getUsername() string {
	setUsernameOnce.Do(func() {
		osUser, err := user.Current()
		if err == nil {
			username = osUser.Username
		}
	})
	return username
}

func NewConnectionModal(configLookup configLookup) *ConnectionModal {
	inputs := []textinputmodal.TextInput{
		{Prompt: "Name: ", Placeholder: ""},
		{Prompt: "User: ", Placeholder: getUsername()},
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

func EditConnectionModal(connection connectiontable.ConnectionHost, configLookup configLookup) *ConnectionModal {
	inputs := []textinputmodal.TextInput{
		{Prompt: "Name: ", Placeholder: ""},
		{Prompt: "User: ", Placeholder: getUsername(), Value: connection.User},
		{Prompt: "Host: ", Placeholder: "", Value: connection.Host},
		{Prompt: "Port: ", Placeholder: "22", Value: strconv.Itoa(int(connection.Port))},
		{Prompt: "SSH Key: ", Placeholder: "", Value: connection.SSHKey},
	}

	innerModal := textinputmodal.NewTextInputModal("Edit Connection", inputs, []string{LoadButton, Save, Cancel})
	return &ConnectionModal{
		configLookup:  configLookup,
		defaultInputs: inputs,
		inner:         []textinputmodal.TextInputModal{innerModal},
	}
}

func (m *ConnectionModal) Update(msg tea.Msg) (*ConnectionModal, *connectiontable.ConnectionHost, tea.Cmd) {
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
				return nil, nil, func() tea.Msg {
					return errormodal.ErrorMsg{
						Title: "Error",
						Err:   fmt.Errorf("port must be a number"),
					}
				}
			}

			return nil, &connectiontable.ConnectionHost{
				User:   values[userIdx],
				Host:   values[hostIdx],
				Port:   uint16(port),
				SSHKey: values[sshKeyIdx],
			}, nil

		case Cancel:
			return nil, nil, nil
		}
	case textinputmodal.Cancel:
		return nil, nil, nil
	case textinputmodal.Quit:
		return nil, nil, tea.Quit
	}

	return m, nil, nil
}

func (m *ConnectionModal) setFromConfig() {
	values := m.inner[m.selectedModal].GetValues()
	hosts, err := m.configLookup(values[nameIdx])
	if err != nil {
		m.inner[m.selectedModal].SetError(err.Error())
		return
	}
	host := hosts[len(hosts)-1]

	values[userIdx] = host.User
	values[hostIdx] = host.Host
	values[portIdx] = strconv.Itoa(int(host.Port))
	values[sshKeyIdx] = host.IdentityFile
	m.inner[m.selectedModal].SetInputs(values)
}

func (m *ConnectionModal) Render() modal.ContentBlock {
	return m.inner[m.selectedModal].Render()
}

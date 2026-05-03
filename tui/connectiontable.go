package tui

import (
	"context"
	"strings"
	"sync/atomic"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"github.com/skubalj/switchboard/portforwarding"
	"github.com/skubalj/switchboard/tui/style"
)

// A row in the connection table
type connectionRow struct {
	UID               uint32
	Online            bool
	User              string
	Host              string
	SSHKey            string
	LocalForwards     []portforwarding.PortForward
	NewLocalForwards  chan portforwarding.PortForward
	RemoteForwards    []portforwarding.PortForward
	NewRemoteForwards chan portforwarding.PortForward
	DropConnection    context.CancelFunc // Signal that the connection should be dropped
}

var rowIdx = new(atomic.Uint32)

// Initialize a new connection row
func NewConnectionRow(user, host, sshkey string) connectionRow {
	return connectionRow{
		UID:               rowIdx.Add(1),
		User:              user,
		Host:              host,
		SSHKey:            sshkey,
		LocalForwards:     nil,
		NewLocalForwards:  make(chan portforwarding.PortForward),
		RemoteForwards:    nil,
		NewRemoteForwards: make(chan portforwarding.PortForward),
	}
}

func (row connectionRow) AsTableRow() table.Row {
	localForwards := make([]string, 0, len(row.LocalForwards))
	for _, fw := range row.LocalForwards {
		localForwards = append(localForwards, fw.LocalString())
	}

	remoteForwards := make([]string, 0, len(row.RemoteForwards))
	for _, fw := range row.RemoteForwards {
		remoteForwards = append(remoteForwards, fw.RemoteString())
	}

	var status string
	if row.Online {
		status = "✅"
	}

	return table.Row{
		status,
		row.User + "@" + row.Host,
		row.SSHKey,
		strings.Join(localForwards, "  "),
		strings.Join(remoteForwards, "  "),
	}
}

// Create the port forwarding connection side
func (row connectionRow) MakeConnection(password string) portforwarding.Connection {
	var auth portforwarding.AuthMethod
	if row.SSHKey == "" {
		auth = portforwarding.PasswordAuth{Password: password}
	} else {
		auth = portforwarding.PrivateKeyAuth{Path: row.SSHKey, Password: password}
	}

	return portforwarding.Connection{
		User:           row.User,
		Auth:           auth,
		Host:           row.Host,
		LocalForwards:  row.NewLocalForwards,
		RemoteForwards: row.NewRemoteForwards,
	}
}

func makeColumns(width int) []table.Column {
	width -= 4
	dividedWidth := width / 6
	remainder := width % dividedWidth

	return []table.Column{
		{Title: "S", Width: 1},
		{Title: "Connection", Width: dividedWidth + remainder - 1},
		{Title: "SSH Key", Width: dividedWidth - 2},
		{Title: "Local Ports", Width: dividedWidth*2 - 2},
		{Title: "Remote Ports", Width: dividedWidth*2 - 2},
	}
}

func tableRows(cons []connectionRow) []table.Row {
	rows := make([]table.Row, 0, len(cons))
	for _, connection := range cons {
		rows = append(rows, connection.AsTableRow())
	}

	rows = append(rows, table.Row{
		"+",
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
	s.Selected = s.Selected.Inherit(style.ButtonSelected)
	connectionTable.SetStyles(s)

	return connectionTable
}

type AddConnectionRow struct {
	ConnectionRow connectionRow
	Password      string
}

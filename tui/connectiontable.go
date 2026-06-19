package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"charm.land/bubbles/v2/table"
	"github.com/skubalj/switchboard/config"
	"github.com/skubalj/switchboard/portforwarding"
	"github.com/skubalj/switchboard/tui/style"
)

// A row in the connection table
type connectionRow struct {
	UID               uint32
	Online            bool
	Name              string
	User              string
	Host              string
	Port              uint16
	SSHKey            string
	LocalForwards     []PortForward
	NewLocalForwards  chan portforwarding.PortForward
	RemoteForwards    []PortForward
	NewRemoteForwards chan portforwarding.PortForward
	DropConnection    context.CancelFunc // Signal that the connection should be dropped
}

var rowIdx = new(atomic.Uint32)

// Initialize a new connection row
func NewConnectionRow(name string, user string, host string, port uint16, sshkey string) connectionRow {
	return connectionRow{
		UID:               rowIdx.Add(1),
		Name:              name,
		User:              user,
		Host:              host,
		Port:              port,
		SSHKey:            sshkey,
		LocalForwards:     nil,
		NewLocalForwards:  make(chan portforwarding.PortForward),
		RemoteForwards:    nil,
		NewRemoteForwards: make(chan portforwarding.PortForward),
	}
}

func connectionRowFromConfig(ctx context.Context, conn config.Connection) connectionRow {
	connectionRow := NewConnectionRow(conn.Host.Name, conn.Host.User, conn.Host.Host, conn.Host.Port, conn.Host.IdentityFile)

	connectionRow.LocalForwards = make([]PortForward, 0, len(conn.LocalForwards))
	localForwards := make([]portforwarding.PortForward, 0, len(conn.LocalForwards))
	for _, f := range conn.LocalForwards {
		pfRow, pf := NewPortForwardFromConfig(ctx, f)
		connectionRow.LocalForwards = append(connectionRow.LocalForwards, pfRow)
		localForwards = append(localForwards, pf)
	}

	go func() {
		for _, forward := range localForwards {
			connectionRow.NewLocalForwards <- forward
		}
	}()

	connectionRow.RemoteForwards = make([]PortForward, 0, len(conn.RemoteForwards))
	remoteForwards := make([]portforwarding.PortForward, 0, len(conn.RemoteForwards))
	for _, f := range conn.RemoteForwards {
		pfRow, pf := NewPortForwardFromConfig(ctx, f)
		connectionRow.RemoteForwards = append(connectionRow.RemoteForwards, pfRow)
		remoteForwards = append(remoteForwards, pf)
	}

	go func() {
		for _, forward := range remoteForwards {
			connectionRow.NewRemoteForwards <- forward
		}
	}()

	return connectionRow
}

func (row connectionRow) AsTableRow() table.Row {
	var status string
	if row.Online {
		status = "✓"
	}

	return table.Row{
		status,
		row.Name,
		fmt.Sprintf("%s@%s:%d", row.User, row.Host, row.Port),
		row.SSHKey,
		strconv.Itoa(len(row.LocalForwards)),
		strconv.Itoa(len(row.RemoteForwards)),
	}
}

// Create the port forwarding connection side
func (row connectionRow) MakeConnection(password string) portforwarding.Connection {
	var auth portforwarding.AuthMethod
	if row.SSHKey == "" {
		auth = portforwarding.PasswordAuth{Password: password}
	} else {
		auth = portforwarding.PrivateKeyAuth{Path: resolveUserDir(row.SSHKey), Password: password}
	}

	return portforwarding.Connection{
		User:           row.User,
		Auth:           auth,
		Host:           row.Host + ":" + strconv.FormatUint(uint64(row.Port), 10),
		LocalForwards:  row.NewLocalForwards,
		RemoteForwards: row.NewRemoteForwards,
	}
}

func resolveUserDir(path string) string {
	suffix, found := strings.CutPrefix(path, "~")
	if !found {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return filepath.Join(home, suffix)
}

func makeColumns(width int) []table.Column {
	width -= 4  // border and padding
	width -= 36 // local and remote ports, plus padding on columns
	dividedWidth := width / 4

	var remainder int
	if dividedWidth != 0 {
		remainder = width % dividedWidth
	}

	return []table.Column{
		{Title: " ", Width: 1},
		{Title: "Name", Width: dividedWidth - 2},
		{Title: "Connection", Width: 2*dividedWidth + remainder - 2},
		{Title: "SSH Key", Width: dividedWidth - 2},
		{Title: "Local Forwards", Width: 16},
		{Title: "Remote Forwards", Width: 16},
	}
}

func tableRows(cons []connectionRow) []table.Row {
	rows := make([]table.Row, 0, len(cons)+1)
	for _, connection := range cons {
		rows = append(rows, connection.AsTableRow())
	}

	rows = append(rows, table.Row{
		"+",
		"New Connection",
		"",
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
	connectionTable.SetStyles(style.Table)

	return connectionTable
}

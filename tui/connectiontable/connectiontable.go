package connectiontable

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"charm.land/bubbles/v2/table"
	"github.com/skubalj/switchboard/config"
	"github.com/skubalj/switchboard/portforwarding"
	"github.com/skubalj/switchboard/tui/modal/portforwardmodal"
	"github.com/skubalj/switchboard/tui/style"
)

var rowIdx = new(atomic.Uint32)

// A row in the connection table
type ConnectionRow struct {
	UID               uint32
	Online            bool
	Name              string
	Hosts             []ConnectionHost
	LocalForwards     []portforwardmodal.PortForward
	NewLocalForwards  chan portforwarding.PortForward
	RemoteForwards    []portforwardmodal.PortForward
	NewRemoteForwards chan portforwarding.PortForward
	DropConnection    context.CancelFunc // Signal that the connection should be dropped
}

type ConnectionHost struct {
	User   string
	Host   string
	Port   uint16
	SSHKey string
}

func (h ConnectionHost) Address() string {
	return fmt.Sprintf("%s@%s:%d", h.User, h.Host, h.Port)
}

// Initialize a new connection row
func NewConnectionRow(name string, hosts []ConnectionHost) ConnectionRow {
	return ConnectionRow{
		UID:               rowIdx.Add(1),
		Name:              name,
		Hosts:             hosts,
		LocalForwards:     nil,
		NewLocalForwards:  make(chan portforwarding.PortForward),
		RemoteForwards:    nil,
		NewRemoteForwards: make(chan portforwarding.PortForward),
	}
}

func ConnectionRowFromConfig(ctx context.Context, conn config.Connection) ConnectionRow {
	hosts := make([]ConnectionHost, 0, len(conn.Hosts))
	for _, host := range conn.Hosts {
		hosts = append(hosts, ConnectionHost{
			User:   host.User,
			Host:   host.Host,
			Port:   host.Port,
			SSHKey: host.IdentityFile,
		})
	}

	connectionRow := NewConnectionRow(conn.Name, hosts)

	connectionRow.LocalForwards = make([]portforwardmodal.PortForward, 0, len(conn.LocalForwards))
	localForwards := make([]portforwarding.PortForward, 0, len(conn.LocalForwards))
	for _, f := range conn.LocalForwards {
		pfRow, pf := portforwardmodal.NewPortForwardFromConfig(ctx, f)
		connectionRow.LocalForwards = append(connectionRow.LocalForwards, pfRow)
		localForwards = append(localForwards, pf)
	}

	go func() {
		for _, forward := range localForwards {
			connectionRow.NewLocalForwards <- forward
		}
	}()

	connectionRow.RemoteForwards = make([]portforwardmodal.PortForward, 0, len(conn.RemoteForwards))
	remoteForwards := make([]portforwarding.PortForward, 0, len(conn.RemoteForwards))
	for _, f := range conn.RemoteForwards {
		pfRow, pf := portforwardmodal.NewPortForwardFromConfig(ctx, f)
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

func (row ConnectionRow) AsTableRow() table.Row {
	var status string
	if row.Online {
		status = "✓"
	}

	return table.Row{
		status,
		row.Name,
		row.ConnectionString(),
		row.sshKeyString(),
		strconv.Itoa(len(row.LocalForwards)),
		strconv.Itoa(len(row.RemoteForwards)),
	}
}

func (row ConnectionRow) sshKeyString() string {
	sshKeys := make([]string, 0, len(row.Hosts))
	for _, host := range row.Hosts {
		if host.SSHKey != "" {
			sshKeys = append(sshKeys, host.SSHKey)
		}
	}
	slices.Sort(sshKeys)
	return strings.Join(slices.Compact(sshKeys), ",")
}

func (row ConnectionRow) ConnectionString() string {
	if len(row.Hosts) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(row.Hosts[0].Address())
	for _, hosts := range row.Hosts[1:] {
		builder.WriteString(" → ")
		builder.WriteString(hosts.Address())
	}
	return builder.String()
}

// Create the port forwarding connection side
func (row ConnectionRow) MakeConnection(passwords []string) portforwarding.Connection {
	hosts := make([]portforwarding.ConnectionHost, 0, len(row.Hosts))
	for idx, host := range row.Hosts {
		hosts = append(hosts, makeConnectionHost(host, passwords[idx]))
	}

	return portforwarding.Connection{
		Hosts:          hosts,
		LocalForwards:  row.NewLocalForwards,
		RemoteForwards: row.NewRemoteForwards,
	}
}

func makeConnectionHost(host ConnectionHost, password string) portforwarding.ConnectionHost {
	var auth portforwarding.AuthMethod
	if host.SSHKey == "" {
		auth = portforwarding.PasswordAuth{Password: password}
	} else {
		auth = portforwarding.PrivateKeyAuth{Path: resolveUserDir(host.SSHKey), Password: password}
	}

	return portforwarding.ConnectionHost{
		User: host.User,
		Auth: auth,
		Host: host.Host + ":" + strconv.Itoa(int(host.Port)),
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

func MakeColumns(width int) []table.Column {
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

func TableRows(cons []ConnectionRow) []table.Row {
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

func NewTable() table.Model {
	connectionTable := table.New(
		table.WithColumns(MakeColumns(100)),
		table.WithRows(TableRows(nil)),
		table.WithFocused(false),
		table.WithWidth(100),
	)
	connectionTable.SetStyles(style.Table)

	return connectionTable
}

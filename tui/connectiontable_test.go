package tui

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"charm.land/bubbles/v2/table"
	"github.com/skubalj/switchboard/config"
	"github.com/stretchr/testify/require"
)

func Test_connectionRowFromConfig(t *testing.T) {
	row := connectionRowFromConfig(context.Background(), config.Connection{
		Host: config.Host{
			User:         "user",
			Host:         "hostname",
			Port:         22,
			IdentityFile: "",
		},
		LocalForwards: []config.PortForward{{
			LocalAddr:  netip.MustParseAddrPort("127.0.0.1:8080"),
			RemoteAddr: netip.MustParseAddrPort("127.0.0.1:3000"),
		}},
		RemoteForwards: []config.PortForward{{
			LocalAddr:  netip.MustParseAddrPort("192.168.0.1:4500"),
			RemoteAddr: netip.MustParseAddrPort("0.0.0.0:5672"),
		}, {
			LocalAddr:  netip.MustParseAddrPort("192.168.39.2:3200"),
			RemoteAddr: netip.MustParseAddrPort("0.0.0.0:3030"),
		}},
	})

	require.False(t, row.Online)
	require.Equal(t, "user", row.User)
	require.Equal(t, "hostname", row.Host)
	require.Equal(t, uint16(22), row.Port)
	require.Empty(t, row.SSHKey)
	require.Len(t, row.LocalForwards, 1)
	require.Equal(t, "127.0.0.1:8080", row.LocalForwards[0].LocalAddr.String())
	require.Equal(t, "127.0.0.1:3000", row.LocalForwards[0].RemoteAddr.String())

	require.Len(t, row.RemoteForwards, 2)
	require.Equal(t, "192.168.0.1:4500", row.RemoteForwards[0].LocalAddr.String())
	require.Equal(t, "0.0.0.0:5672", row.RemoteForwards[0].RemoteAddr.String())
	require.Equal(t, "192.168.39.2:3200", row.RemoteForwards[1].LocalAddr.String())
	require.Equal(t, "0.0.0.0:3030", row.RemoteForwards[1].RemoteAddr.String())

	go func() { time.Sleep(500 * time.Millisecond); close(row.NewLocalForwards) }()
	require.Equal(t, 1, countInChannel(row.NewLocalForwards))
	go func() { time.Sleep(500 * time.Millisecond); close(row.NewRemoteForwards) }()
	require.Equal(t, 2, countInChannel(row.NewRemoteForwards))
}

func countInChannel[T any](ch chan T) int {
	count := 0
	for range ch {
		count++
	}
	return count
}

func Test_connectionRow_AsTableRow(t *testing.T) {
	row := NewConnectionRow("Connection", "user", "hostname", 22, "")
	require.Equal(t, table.Row{"", "Connection", "user@hostname:22", "", "0", "0"}, row.AsTableRow())

	row.SSHKey = "~/.ssh/id_ed25519"
	row.Port = 2222
	require.Equal(t, table.Row{"", "Connection", "user@hostname:2222", "~/.ssh/id_ed25519", "0", "0"}, row.AsTableRow())

	row.LocalForwards = append(row.LocalForwards, PortForward{
		LocalAddr:  netip.MustParseAddrPort("127.0.0.1:8080"),
		RemoteAddr: netip.MustParseAddrPort("127.0.0.1:3030"),
	})
	require.Equal(t, table.Row{"", "Connection", "user@hostname:2222", "~/.ssh/id_ed25519", "1", "0"}, row.AsTableRow())

	row.RemoteForwards = append(row.RemoteForwards, PortForward{
		LocalAddr:  netip.MustParseAddrPort("127.0.0.1:1234"),
		RemoteAddr: netip.MustParseAddrPort("127.0.0.1:2234"),
	})
	require.Equal(t, table.Row{"", "Connection", "user@hostname:2222", "~/.ssh/id_ed25519", "1", "1"}, row.AsTableRow())
}

package portforwardmodal

import (
	"cmp"
	"context"
	"net/netip"
	"strconv"

	"charm.land/bubbles/v2/table"
	"github.com/skubalj/switchboard/config"
	"github.com/skubalj/switchboard/portforwarding"
	"github.com/skubalj/switchboard/tui/style"
)

type PortForward struct {
	StopCallback context.CancelFunc
	LocalAddr    netip.AddrPort
	RemoteAddr   netip.AddrPort
}

type ForwardPair struct {
	IsLocal bool
	Tx      PortForward
	Rx      portforwarding.PortForward
}

func NewPortForwardFromConfig(ctx context.Context, f config.PortForward) (PortForward, portforwarding.PortForward) {
	ctx, cancel := context.WithCancel(ctx)

	return PortForward{
			StopCallback: cancel,
			LocalAddr:    f.LocalAddr,
			RemoteAddr:   f.RemoteAddr,
		}, portforwarding.PortForward{
			Ctx:        ctx,
			LocalAddr:  f.LocalAddr,
			RemoteAddr: f.RemoteAddr,
		}
}

func newPortForward(localHost, localPort, remoteHost, remotePort string) (config.PortForward, error) {
	local, e1 := parseAddrPort(localHost, localPort)
	remote, e2 := parseAddrPort(remoteHost, remotePort)
	return config.PortForward{LocalAddr: local, RemoteAddr: remote}, cmp.Or(e1, e2)
}

func parseAddrPort(host, port string) (netip.AddrPort, error) {
	var addr netip.Addr
	var e1 error

	switch host {
	case "", "localhost":
		addr, e1 = netip.ParseAddr("127.0.0.1")
	default:
		addr, e1 = netip.ParseAddr(host)
	}

	p, e2 := strconv.ParseUint(port, 10, 16)
	return netip.AddrPortFrom(addr, uint16(p)), cmp.Or(e1, e2)
}

func newLocalForwardingTable(forwards []PortForward) table.Model {
	tbl := table.New(
		table.WithColumns(makeLocalForwardingColumns(80)),
		table.WithRows(makeLocalForwardingRows(forwards)),
		table.WithFocused(false),
	)
	tbl.SetStyles(style.Table)
	return tbl
}

func makeLocalForwardingColumns(width int) []table.Column {
	width -= 8
	dividedWidth := width / 4
	remainder := width % 4

	return []table.Column{
		{Title: "Local Bind Address", Width: dividedWidth + remainder},
		{Title: "Local Bind Port", Width: dividedWidth},
		{Title: "Remote Address", Width: dividedWidth},
		{Title: "Remote Port", Width: dividedWidth},
	}
}

func makeLocalForwardingRows(forwards []PortForward) []table.Row {
	rows := make([]table.Row, 0, len(forwards)+1)
	for _, fw := range forwards {
		rows = append(rows, table.Row{
			fw.LocalAddr.Addr().String(),
			strconv.FormatUint(uint64(fw.LocalAddr.Port()), 10),
			fw.RemoteAddr.Addr().String(),
			strconv.FormatUint(uint64(fw.RemoteAddr.Port()), 10),
		})
	}

	rows = append(rows, table.Row{
		"+  New Forward",
		"",
		"",
		"",
	})

	return rows
}

func newRemoteForwardingTable(forwards []PortForward) table.Model {
	tbl := table.New(
		table.WithColumns(makeLocalForwardingColumns(100)),
		table.WithRows(makeRemoteForwardingRows(forwards)),
		table.WithFocused(false),
	)
	tbl.SetStyles(style.Table)

	return tbl
}

func makeRemoteForwardingColumns(width int) []table.Column {
	width -= 8
	dividedWidth := width / 4
	remainder := width % dividedWidth

	return []table.Column{
		{Title: "Remote Bind Address", Width: dividedWidth + remainder},
		{Title: "Remote Bind Port", Width: dividedWidth},
		{Title: "Local Address", Width: dividedWidth},
		{Title: "Local Port", Width: dividedWidth},
	}
}

func makeRemoteForwardingRows(forwards []PortForward) []table.Row {
	rows := make([]table.Row, 0, len(forwards)+1)
	for _, fw := range forwards {
		rows = append(rows, table.Row{
			fw.RemoteAddr.Addr().String(),
			strconv.FormatUint(uint64(fw.RemoteAddr.Port()), 10),
			fw.LocalAddr.Addr().String(),
			strconv.FormatUint(uint64(fw.LocalAddr.Port()), 10),
		})
	}

	rows = append(rows, table.Row{
		"+  New Forward",
		"",
		"",
		"",
	})

	return rows
}

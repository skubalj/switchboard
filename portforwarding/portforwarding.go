package portforwarding

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/skubalj/chanutils"
	"github.com/skubalj/switchboard/config"
	"github.com/skubalj/switchboard/messaging"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/sync/errgroup"
)

type ConnectionHost struct {
	User         string // user to authenticate as
	Host         string // in the form HOST:PORT
	IdentityFile string
}

func (h ConnectionHost) AsConfig(
	hostKeyAlgorithms []string,
	sshAgent agent.Agent,
	pwCallback func(comment string) (string, error),
	hostKeyCB ssh.HostKeyCallback,
) ssh.ClientConfig {
	return ssh.ClientConfig{
		User: h.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(sshAgent.Signers),
			ssh.PublicKeysCallback(func() (signers []ssh.Signer, err error) {
				return addKey(sshAgent, pwCallback, h.IdentityFile)
			}),
			ssh.PasswordCallback(func() (string, error) {
				return pwCallback(fmt.Sprintf("Password for %s@%s", h.User, h.Host))
			}),
		},
		HostKeyCallback:   hostKeyCB,
		Timeout:           60 * time.Second,
		HostKeyAlgorithms: hostKeyAlgorithms,
	}
}

type Connection struct {
	// A channel to report errors. Will be closed when the connection is dropped
	OnClose chan<- struct{}
	// A list of hosts. The final element is the remote host, the rest are "jump hosts"
	Hosts          []ConnectionHost
	LocalForwards  <-chan PortForward
	RemoteForwards <-chan PortForward
}

// Typed definition of a port forward operation
type PortForward interface {
	Ctx() context.Context
	LocalAddr() netip.AddrPort
	RemoteAddr() netip.AddrPort
}

type ConnectionFactory struct {
	cfg         config.Config
	sshAgent    agent.Agent
	msgs        messaging.Tx
	getPassword chan<- GetPasswordRequest
}

func New(
	cfg config.Config,
	msgs messaging.Tx,
	getPassword chan<- GetPasswordRequest,
) (ConnectionFactory, error) {
	a, err := getAgent()
	if err != nil {
		return ConnectionFactory{}, err
	}

	return ConnectionFactory{
		cfg:         cfg,
		sshAgent:    a,
		msgs:        msgs,
		getPassword: getPassword,
	}, nil
}

// Connect to the client and listen for commands on a background thread
func (a ConnectionFactory) ConnectToClient(ctx context.Context, conn Connection) error {
	if len(conn.Hosts) == 0 {
		close(conn.OnClose)
		return errors.New("list of hosts cannot be empty")
	}

	hostKeyCB, err := hostKeyCB(a.cfg.KnownHostsFile, a.msgs)
	if err != nil {
		close(conn.OnClose)
		return fmt.Errorf("unable to create host key callback: %w", err)
	}

	var client *ssh.Client
	for _, host := range conn.Hosts {
		config := host.AsConfig(a.cfg.HostKeyAlgorithms, a.sshAgent, a.getPasswordCB(ctx), hostKeyCB)
		client, err = jumpToHost(client, &config, host)
		if err != nil {
			close(conn.OnClose)
			return fmt.Errorf("unable to connect to SSH server %s@%s: %w", host.User, host.Host, err)
		}
	}

	go mainLoop(ctx, a.msgs, client, conn)
	return nil
}

func (a ConnectionFactory) getPasswordCB(ctx context.Context) func(comment string) (string, error) {
	return func(comment string) (string, error) {
		ch := make(chan string, 1)
		return chanutils.SendAndRecv(ctx, a.getPassword, ch, GetPasswordRequest{
			Comment:  comment,
			Response: ch,
		})
	}
}

type GetPasswordRequest struct {
	Comment  string
	Response chan<- string
}

func hostKeyCB(knownHostsFile string, msgs messaging.Tx) (ssh.HostKeyCallback, error) {
	innerKnownHostsCallback, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return nil, err
	}
	hostKeyCB := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := innerKnownHostsCallback(hostname, remote, key)
		if err != nil {
			typed, ok := err.(*knownhosts.KeyError)
			if ok {
				msgs.Errorf("key for host '%s' with type '%s' not found in known_hosts file -- connect with ssh to save the key", hostname, key.Type())
				msgs.Tracef("found %d keys for host '%s' with types: %s", len(typed.Want), hostname, strings.Join(keyTypes(typed.Want), ", "))
			}
		} else {
			msgs.Tracef("verified host '%s' with %s key", hostname, key.Type())
		}

		return err
	}

	return hostKeyCB, nil
}

func jumpToHost(client *ssh.Client, config *ssh.ClientConfig, host ConnectionHost) (*ssh.Client, error) {
	if client == nil {
		return ssh.Dial("tcp", host.Host, config)
	}

	rawConn, err := client.Dial("tcp", host.Host)
	if err != nil {
		return nil, fmt.Errorf("unable to dial target host from jump host: %w", err)
	}
	sshConn, newChannelCh, sshRequestCh, err := ssh.NewClientConn(rawConn, host.Host, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create client connection for target: %w", err)
	}
	return ssh.NewClient(sshConn, newChannelCh, sshRequestCh), nil
}

func keyTypes(keys []knownhosts.KnownKey) []string {
	types := make([]string, 0, len(keys))
	for _, key := range keys {
		types = append(types, key.Key.Type())
	}
	return types
}

func mainLoop(ctx context.Context, msgs messaging.Tx, client *ssh.Client, conn Connection) {
	defer client.Close()
	defer close(conn.OnClose) // Close the channel to inform the UI that we disconnected

	host := conn.Hosts[len(conn.Hosts)-1]
	msgs.Infof("established connection to %s@%s", host.User, host.Host)
	defer msgs.Infof("closed connection to %s@%s", host.User, host.Host)

	eg, ctx := errgroup.WithContext(ctx)

	for {
		select {
		case <-ctx.Done():
			err := eg.Wait()
			if err != nil {
				msgs.Errorf("connection %s@%s disconnected: %v", host.User, host.Host, err)
			}
			return
		case forward := <-conn.LocalForwards:
			eg.Go(func() error { return forwardLocal(ctx, client, forward, msgs) })
		case forward := <-conn.RemoteForwards:
			eg.Go(func() error { return forwardRemote(ctx, client, forward, msgs) })
		}
	}
}

// Connections to the given TCP port on the local (client) host are to be forwarded to the given host and port
func forwardLocal(ctx context.Context, client *ssh.Client, addresses PortForward, msgs messaging.Tx) error {
	if cmp.Or(ctx.Err(), addresses.Ctx().Err()) != nil {
		msgs.Tracef("skipping closed forward from local address '%s' to remote address '%s'", addresses.LocalAddr().String(), addresses.RemoteAddr().String())
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	localListener, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(addresses.LocalAddr()))
	if err != nil {
		return fmt.Errorf("unable to open listener for local address %s: %w", addresses.LocalAddr(), err)
	}

	msgs.Infof("now listening on local port %s", addresses.LocalAddr())
	defer msgs.Infof("listener on local port %s closed", addresses.LocalAddr())

	go func() {
		select {
		case <-ctx.Done():
		case <-addresses.Ctx().Done():
		}
		localListener.Close()
	}()

	for {
		localConn, err := localListener.Accept()
		if errors.Is(err, net.ErrClosed) {
			return nil
		} else if err != nil {
			return fmt.Errorf("local listener closed: %w", err)
		}
		msgs.Tracef("new connection to local port %s", addresses.LocalAddr())

		go func() {
			defer localConn.Close()
			defer msgs.Tracef("connection to local port %s closed", addresses.LocalAddr())

			remoteConn, err := client.Dial("tcp", addresses.RemoteAddr().String())
			if err != nil {
				msgs.Errorf("unable to dial remote address %s: %w", addresses.RemoteAddr(), err)
				return
			}
			defer remoteConn.Close()

			err = copyData(ctx, localConn, remoteConn)
			if err != nil {
				msgs.SendError(err)
				return
			}
		}()
	}
}

// Connections to the given TCP port on the remote (server) host are to be forwarded to the local side
func forwardRemote(ctx context.Context, client *ssh.Client, addresses PortForward, msgs messaging.Tx) error {
	if cmp.Or(ctx.Err(), addresses.Ctx().Err()) != nil {
		msgs.Tracef("skipping closed forward from remote address '%s' to local address '%s'", addresses.RemoteAddr().String(), addresses.LocalAddr().String())
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	remoteListener, err := client.ListenTCP(net.TCPAddrFromAddrPort(addresses.RemoteAddr()))
	if err != nil {
		return fmt.Errorf("unable to open listener for remote address %s: %w", addresses.RemoteAddr(), err)
	}

	msgs.Infof("now listening on host %s@%s to port %s", client.User(), client.RemoteAddr(), addresses.RemoteAddr())
	defer msgs.Infof("listener on host %s@%s to port %s closed", client.User(), client.RemoteAddr(), addresses.RemoteAddr())

	go func() {
		select {
		case <-ctx.Done():
		case <-addresses.Ctx().Done():
		}
		remoteListener.Close()
	}()

	for {
		remoteConn, err := remoteListener.Accept()
		if errors.Is(err, io.EOF) {
			// Note: For some reason, the remote listener returns `io.EOF`
			// instead of `net.ErrClosed` when it is closed.
			return nil
		} else if err != nil {
			return fmt.Errorf("remote listener closed: %w", err)
		}
		msgs.Tracef("new connection to remote port %s", addresses.RemoteAddr())

		go func() {
			defer remoteConn.Close()
			defer msgs.Tracef("connection to remote port %s closed", addresses.RemoteAddr())

			localConn, err := net.DialTCP("tcp", nil, net.TCPAddrFromAddrPort(addresses.LocalAddr()))
			if err != nil {
				msgs.Errorf("unable to dial local address %s: %w", addresses.LocalAddr(), err)
				return
			}
			defer localConn.Close()

			err = copyData(ctx, localConn, remoteConn)
			if err != nil {
				msgs.SendError(err)
				return
			}
		}()
	}
}

// Connect the local conn's writer to the remote conn's reader and vice versa
func copyData(ctx context.Context, localConn net.Conn, remoteConn net.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg, _ := errgroup.WithContext(ctx)

	go func() {
		<-ctx.Done()
		localConn.Close()
		remoteConn.Close()
	}()

	eg.Go(func() error {
		_, err := io.Copy(localConn, remoteConn)
		if err != nil {
			return fmt.Errorf("error copying to local device: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		_, err := io.Copy(remoteConn, localConn)
		if err != nil {
			return fmt.Errorf("error copying to remote device: %w", err)
		}
		return nil
	})

	return eg.Wait()
}

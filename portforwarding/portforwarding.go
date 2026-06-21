package portforwarding

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/skubalj/switchboard/config"
	"github.com/skubalj/switchboard/messaging"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/sync/errgroup"
)

type ConnectionHost struct {
	User string // user to authenticate as
	Auth AuthMethod
	Host string // in the form HOST:PORT
}

func (h ConnectionHost) AsConfig(hostKeyAlgorithms []string, hostKeyCB ssh.HostKeyCallback) (ssh.ClientConfig, error) {
	auth, err := h.Auth.Create()
	if err != nil {
		return ssh.ClientConfig{}, err
	}

	return ssh.ClientConfig{
		User:              h.User,
		Auth:              []ssh.AuthMethod{auth},
		HostKeyCallback:   hostKeyCB,
		Timeout:           60 * time.Second,
		HostKeyAlgorithms: hostKeyAlgorithms,
	}, nil
}

type Connection struct {
	Hosts          []ConnectionHost // a list of jump hosts leading to the endpoint: the last element
	LocalForwards  <-chan PortForward
	RemoteForwards <-chan PortForward
}

// Typed defintiion of a port forward operation
type PortForward struct {
	Ctx        context.Context
	LocalAddr  netip.AddrPort
	RemoteAddr netip.AddrPort
}

type AuthMethod interface {
	Create() (ssh.AuthMethod, error)
}

type PasswordAuth struct {
	Password string
}

func (a PasswordAuth) Create() (ssh.AuthMethod, error) {
	return ssh.Password(a.Password), nil
}

type PrivateKeyAuth struct {
	Path     string
	Password string
}

func (a PrivateKeyAuth) Create() (ssh.AuthMethod, error) {
	key, err := os.ReadFile(a.Path)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %w", err)
	}

	var signer ssh.Signer
	if a.Password != "" {
		decryptedKey, err := ssh.ParseRawPrivateKeyWithPassphrase(key, []byte(a.Password))
		if err != nil {
			return nil, fmt.Errorf("unable to decyrpt private key: %w", err)
		}

		signer, err = ssh.NewSignerFromKey(decryptedKey)
		if err != nil {
			return nil, fmt.Errorf("unable to create signer from key: %w", err)
		}

	} else {
		signer, err = ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("unable to parse private key: %v", err)
		}
	}

	return ssh.PublicKeys(signer), nil
}

// Connect to the client and listen for commands on a background thread
func ConnectToClient(
	ctx context.Context,
	cfg config.Config,
	errCh chan<- error,
	msgs messaging.Tx,
	conn Connection,
) error {
	if len(conn.Hosts) == 0 {
		return fmt.Errorf("list of hosts cannot be empty")
	}

	hostKeyCB, err := hostKeyCB(cfg.KnownHostsFile, msgs)
	if err != nil {
		return fmt.Errorf("unable to create host key callback: %w", err)
	}

	var client *ssh.Client
	for _, host := range conn.Hosts {
		config, err := host.AsConfig(cfg.HostKeyAlgorithms, hostKeyCB)
		if err != nil {
			return fmt.Errorf("unable to connect to client %s@%s: %w", host.User, host.Host, err)
		}
		client, err = jumpToHost(client, &config, host)
		if err != nil {
			return fmt.Errorf("unable to connect to SSH server %s@%s: %w", host.User, host.Host, err)
		}
	}

	go mainLoop(ctx, errCh, msgs, client, conn)
	return nil
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
				msgs.Debugf("found %d keys for host '%s' with types: %s", len(typed.Want), hostname, strings.Join(keyTypes(typed.Want), ", "))
			}
		} else {
			msgs.Debugf("verified host '%s' with %s key", hostname, key.Type())
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

func mainLoop(
	ctx context.Context,
	errCh chan<- error,
	msgs messaging.Tx,
	client *ssh.Client,
	conn Connection,
) {
	defer client.Close()

	host := conn.Hosts[len(conn.Hosts)-1]
	msgs.Infof("Established connection to %s@%s", host.User, host.Host)
	defer msgs.Infof("Closed connection to %s@%s", host.User, host.Host)

	eg, ctx := errgroup.WithContext(ctx)

	for {
		select {
		case <-ctx.Done():
			err := eg.Wait()
			if err != nil {
				msgs.Errorf("connection %s@%s broken: %v", host.User, host.Host, err)
			}
			// Push to the err channel to inform the UI that we disconnected
			errCh <- err
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
	if cmp.Or(ctx.Err(), addresses.Ctx.Err()) != nil {
		msgs.Debugf("skipping closed forward from local address '%s' to remote address '%s'", addresses.LocalAddr.String(), addresses.RemoteAddr.String())
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	localListener, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(addresses.LocalAddr))
	if err != nil {
		return fmt.Errorf("unable to open listener for local address %s: %w", addresses.LocalAddr, err)
	}

	msgs.Infof("Now listening on local port %s", addresses.LocalAddr)
	defer msgs.Infof("Listener on local port %s closed", addresses.LocalAddr)

	go func() {
		select {
		case <-ctx.Done():
		case <-addresses.Ctx.Done():
		}
		localListener.Close()
	}()

	for {
		localConn, err := localListener.Accept()
		if errors.Is(err, net.ErrClosed) {
			return nil
		} else if err != nil {
			return fmt.Errorf("listener closed: %w", err)
		}
		msgs.Debugf("New connection to local port %s", addresses.LocalAddr)

		go func() {
			defer localConn.Close()
			defer msgs.Debugf("Connection to local port %s closed", addresses.LocalAddr)

			remoteConn, err := client.Dial("tcp", addresses.RemoteAddr.String())
			if err != nil {
				msgs.Errorf("unable to dial remote address %s: %w", addresses.RemoteAddr, err)
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
	if cmp.Or(ctx.Err(), addresses.Ctx.Err()) != nil {
		msgs.Debugf("skipping closed forward from remote address '%s' to local address '%s'", addresses.RemoteAddr.String(), addresses.LocalAddr.String())
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	remoteListener, err := client.ListenTCP(net.TCPAddrFromAddrPort(addresses.RemoteAddr))
	if err != nil {
		return fmt.Errorf("unable to open listener for remote address %s: %w", addresses.RemoteAddr, err)
	}

	msgs.Infof("Now listening on host %s@%s to port %s", client.User(), client.LocalAddr(), addresses.RemoteAddr)
	defer msgs.Infof("Listener on host %s@%s to port %s closed", client.User(), client.LocalAddr(), addresses.RemoteAddr)

	go func() {
		<-ctx.Done()
		remoteListener.Close()
	}()

	for {
		remoteConn, err := remoteListener.Accept()
		if errors.Is(err, net.ErrClosed) {
			return nil
		} else if err != nil {
			return fmt.Errorf("listener closed: %w", err)
		}
		msgs.Debugf("New connection to remote port %s", addresses.RemoteAddr)

		go func() {
			defer remoteConn.Close()
			defer msgs.Debugf("Connection to remote port %s closed", addresses.RemoteAddr)

			localConn, err := net.DialTCP("tcp", nil, net.TCPAddrFromAddrPort(addresses.LocalAddr))
			if err != nil {
				msgs.Errorf("unable to dial local address %s: %w", addresses.LocalAddr, err)
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

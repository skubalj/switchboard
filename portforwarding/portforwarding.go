package portforwarding

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/skubalj/spfa/messaging"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
)

type ConnectionConfig struct {
	User           string // user to authenticate as
	Auth           AuthMethod
	Host           string // in the form HOST:PORT
	LocalForwards  chan PortForward
	RemoteForwards chan PortForward
}

// Typed defintiion of a port forward operation
type PortForward struct {
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

func ConnectToClient(ctx context.Context, msgs messaging.Tx, savedConfig ConnectionConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	authMethod, err := savedConfig.Auth.Create()
	if err != nil {
		return fmt.Errorf("unable to generate auth: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            savedConfig.User,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         60 * time.Second,
	}

	// Connect to the remote server and perform the SSH handshake.
	client, err := ssh.Dial("tcp", savedConfig.Host, config)
	if err != nil {
		return fmt.Errorf("unable to connect to client %s@%s: %v", savedConfig.User, savedConfig.Host, err)
	}
	defer client.Close()

	msgs.Infof("Established connection to %s@%s", savedConfig.User, savedConfig.Host)
	defer msgs.Infof("Closed connection to %s@%s", savedConfig.User, savedConfig.Host)

	eg, ctx := errgroup.WithContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return eg.Wait()
		case forwarding := <-savedConfig.LocalForwards:
			eg.Go(func() error { return forwardLocal(ctx, client, forwarding, msgs) })
		case forwarding := <-savedConfig.RemoteForwards:
			eg.Go(func() error { return forwardRemote(ctx, client, forwarding, msgs) })
		}
	}
}

// Connections to the given TCP port on the local (client) host are to be forwarded to the given host and port
func forwardLocal(ctx context.Context, client *ssh.Client, addresses PortForward, msgs messaging.Tx) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	localListener, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(addresses.LocalAddr))
	if err != nil {
		return fmt.Errorf("unable to open listener for local address %s: %w", addresses.LocalAddr, err)
	}

	msgs.Infof("Now listening on local port %s", addresses.LocalAddr)

	go func() {
		<-ctx.Done()
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	remoteListener, err := client.ListenTCP(net.TCPAddrFromAddrPort(addresses.RemoteAddr))
	if err != nil {
		return fmt.Errorf("unable to open listener for remote address %s: %w", addresses.RemoteAddr, err)
	}

	msgs.Infof("Now listening on host %s@%s to port %s", client.User(), client.LocalAddr(), addresses.LocalAddr)

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

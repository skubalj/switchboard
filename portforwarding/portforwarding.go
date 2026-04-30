package portforwarding

import (
	"context"
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

type SSHConfig struct {
	User           string // user to authenticate as
	Auth           AuthMethod
	Host           string // in the form HOST:PORT
	LocalForwards  []PortForward
	RemoteForwards []PortForward
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
	key, err := os.ReadFile("/home/user/.ssh/id_rsa")
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

func ConnectToClient(ctx context.Context, msgs messaging.Tx, savedConfig SSHConfig) error {
	var hostKey ssh.PublicKey
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	authMethod, err := savedConfig.Auth.Create()
	if err != nil {
		return fmt.Errorf("unable to generate auth: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            savedConfig.User,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.FixedHostKey(hostKey),
		Timeout:         60 * time.Second,
	}

	// Connect to the remote server and perform the SSH handshake.
	client, err := ssh.Dial("tcp", savedConfig.Host, config)
	if err != nil {
		return fmt.Errorf("unable to connect to client %s@%s: %v", savedConfig.User, savedConfig.Host, err)
	}
	defer client.Close()

	eg, ctx := errgroup.WithContext(ctx)

	for _, forwarding := range savedConfig.LocalForwards {
		eg.Go(func() error { return forwardLocal(ctx, client, forwarding, msgs) })
	}
	for _, forwarding := range savedConfig.RemoteForwards {
		eg.Go(func() error { return forwardRemote(ctx, client, forwarding, msgs) })
	}

	return eg.Wait()
}

// Connections to the given TCP port on the local (client) host are to be forwarded to the given host and port
func forwardLocal(ctx context.Context, client *ssh.Client, addresses PortForward, msgs messaging.Tx) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	localListener, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(addresses.LocalAddr))
	if err != nil {
		return fmt.Errorf("unable to open listener for local address %s: %w", addresses.LocalAddr, err)
	}

	go func() {
		<-ctx.Done()
		localListener.Close()
	}()

	for {
		localConn, err := localListener.Accept()
		if err != nil {
			return fmt.Errorf("listener closed: %w", err)
		}

		go func() {
			defer localConn.Close()

			remoteConn, err := client.Dial("tcp", addresses.RemoteAddr.String())
			if err != nil {
				msgs.SendError(fmt.Errorf("unable to dial remote address %s: %w", addresses.RemoteAddr, err))
			}
			defer remoteConn.Close()

			err = copyData(ctx, localConn, remoteConn)
			if err != nil {
				msgs.SendError(fmt.Errorf("error copying data: %w", err))
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

	go func() {
		<-ctx.Done()
		remoteListener.Close()
	}()

	for {
		remoteConn, err := remoteListener.Accept()
		if err != nil {
			return fmt.Errorf("listener closed: %w", err)
		}

		go func() {
			defer remoteConn.Close()

			localConn, err := net.DialTCP("tcp", nil, net.TCPAddrFromAddrPort(addresses.LocalAddr))
			if err != nil {
				msgs.SendError(fmt.Errorf("unable to dial local address %s: %w", addresses.LocalAddr, err))
			}
			defer localConn.Close()

			err = copyData(ctx, localConn, remoteConn)
			if err != nil {
				msgs.SendError(fmt.Errorf("error copying data: %w", err))
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
		defer cancel()
		_, err := io.Copy(localConn, remoteConn)
		if err != nil {
			return fmt.Errorf("error copying data to local device: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		defer cancel()
		_, err := io.Copy(remoteConn, localConn)
		if err != nil {
			return fmt.Errorf("error copying data to remote device: %w", err)
		}
		return nil
	})

	return eg.Wait()
}

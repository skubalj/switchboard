package portforwarding

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/skubalj/chanutils"
	"github.com/skubalj/switchboard/messaging"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const sshAgentSocketVar = "SSH_AUTH_SOCK"

func getAgent() (agent.Agent, error) {
	socketPath := os.Getenv(sshAgentSocketVar)
	if socketPath == "" {
		return agent.NewKeyring(), nil
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open SSH_AUTH_SOCKET: %w", err)
	}

	return agent.NewClient(conn), nil
}

func addKey(
	store agent.Agent,
	getPwd func(comment string) (string, error),
	path string,
	msgs messaging.Tx,
) ([]ssh.Signer, error) {
	if path == "" {
		msgs.Tracef("no key set")
		return nil, nil
	}

	keys, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("unable to get keys from agent")
	}
	for _, key := range keys {
		if key.Comment == path {
			// Key is already in the keystore
			msgs.Tracef("found key '%s' in agent", path)
			return store.Signers()
		}
	}

	msgs.Infof("key '%s' not in keystore -- attempting to add", path)

	abspath := path
	if after, ok := strings.CutPrefix(path, "~/"); ok {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("unable to resolve home dir for path '%s': %w", path, err)
		}
		abspath = filepath.Join(homedir, after)
	}

	key, err := os.ReadFile(abspath)
	if err != nil {
		return nil, fmt.Errorf("unable to open key file: %w", err)
	}

	rawKey, err := ssh.ParseRawPrivateKey(key)
	// For some reason, `errors.Is(err, &ssh.PassphraseMissingError{})` doesn't work here,
	// so we're using a more heavy-handed type switch
	switch err.(type) {
	case *ssh.PassphraseMissingError:
		pw, err := getPwd(fmt.Sprintf("Password For Key '%s'", path))
		if err != nil {
			return nil, err
		}

		rawKey, err = ssh.ParseRawPrivateKeyWithPassphrase(key, []byte(pw))
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt '%s': %w", path, err)
		}
	case nil:
	default:
		return nil, fmt.Errorf("unable to parse private key %T: %w", err, err)
	}

	err = store.Add(agent.AddedKey{
		PrivateKey: rawKey,
		Comment:    path,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to add key to agent: %w", err)
	}

	return store.Signers()
}

type GetPasswordRequest struct {
	Comment  string
	Response chan<- string
}

func getPasswordCB(ctx context.Context, req chan<- GetPasswordRequest) func(comment string) (string, error) {
	return func(comment string) (string, error) {
		ch := make(chan string, 1)
		return chanutils.SendAndRecv(ctx, req, ch, GetPasswordRequest{
			Comment:  comment,
			Response: ch,
		})
	}
}

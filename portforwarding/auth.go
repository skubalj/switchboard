package portforwarding

import (
	"errors"
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const sshAgentSocketVar = "SSH_AUTH_SOCK"

func getAgent() (agent.Agent, error) {
	socketPath, ok := os.LookupEnv(sshAgentSocketVar)
	if !ok {
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
) ([]ssh.Signer, error) {
	if path == "" {
		return nil, nil
	}

	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to open key file: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if errors.Is(err, &ssh.PassphraseMissingError{}) {
		pw, err := getPwd(fmt.Sprintf("Password For Key '%s'", path))
		if err != nil {
			return nil, err
		}

		signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(pw))
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt '%s': %w", path, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %w", err)
	}

	err = store.Add(agent.AddedKey{
		PrivateKey: signer,
		Comment:    path,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to add key to agent: %w", err)
	}

	return []ssh.Signer{signer}, nil
}

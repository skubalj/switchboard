package config

import (
	"cmp"
	"fmt"
	"iter"
	"log/slog"
	"net/netip"
	"os"
	"os/exec"
	osuser "os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinburke/ssh_config"
	"sigs.k8s.io/yaml"
)

func GetConfig(path string) (Config, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("unable to read switchboard config file: %w", err)
	}

	var cfg Config
	err = yaml.Unmarshal(payload, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("unable to unmarshal payload: %w", err)
	}

	return cfg, nil
}

func DefaultConfig() (Config, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}

	hostKeyTypes, err := FetchHostKeyTypes()
	if err != nil {
		slog.Error("unable to fetch key types from ssh command", "error", err)
	}

	return Config{
		SSHConfigFile:     filepath.Join(homedir, ".ssh", "config"),
		KnownHostsFile:    filepath.Join(homedir, ".ssh", "known_hosts"),
		HostKeyAlgorithms: hostKeyTypes,
	}, nil
}

func FetchHostKeyTypes() ([]string, error) {
	keyTypes, err := exec.Command("ssh", "-Q", "key").Output()
	if err != nil {
		return nil, fmt.Errorf("unable to fetch host key types from ssh command")
	}

	return slices.Collect(trimIter(strings.Lines(string(keyTypes)))), nil
}

func trimIter(x iter.Seq[string]) iter.Seq[string] {
	return func(yield func(string) bool) {
		for val := range x {
			trimmed := strings.Trim(val, "\n\r ")
			if trimmed == "" {
				continue // remove blank lines
			} else if !yield(trimmed) {
				return
			}
		}
	}
}

func SaveConfig(path string, cfg Config) error {
	payload, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, payload, 0o664)
	if err != nil {
		return fmt.Errorf("unable to write config to switchboard file: %w", err)
	}

	return nil
}

type Config struct {
	SSHConfigFile     string       `json:"sshConfigFile"`
	KnownHostsFile    string       `json:"knownHostsFile"`
	HostKeyAlgorithms []string     `json:"hostKeyAlgorithms"`
	Connections       []Connection `json:"connections"`
}

type Connection struct {
	Host           Host          `json:"host"`
	LocalForwards  []PortForward `json:"localForwards"`
	RemoteForwards []PortForward `json:"remoteForwards"`
}

type Host struct {
	User         string `json:"user"`
	Host         string `json:"host"`
	Port         uint16 `json:"port"`
	IdentityFile string `json:"identityFile"`
}

type PortForward struct {
	LocalAddr  netip.AddrPort `json:"localAddr"`
	RemoteAddr netip.AddrPort `json:"remoteAddr"`
}

func (cfg Config) FetchSSHConfig(host string) (Host, error) {
	configFile, err := os.ReadFile(cfg.SSHConfigFile)
	if err != nil {
		return Host{}, fmt.Errorf("unable to open ssh config file: %w", err)
	}

	sshCfg, err := ssh_config.DecodeBytes(configFile)
	if err != nil {
		return Host{}, fmt.Errorf("unable to decode ssh config file: %w", err)
	}

	user, err := sshCfg.Get(host, "user")
	if err != nil {
		return Host{}, fmt.Errorf("unable to get user: %w", err)
	}
	if user == "" {
		usr, err := osuser.Current()
		if err != nil {
			return Host{}, fmt.Errorf("unable to fetch current user: %w", err)
		}
		user = usr.Name
	}

	hostName, err := sshCfg.Get(host, "hostname")
	if err != nil {
		return Host{}, fmt.Errorf("unable to get hostname: %w", err)
	}

	portStr, err := sshCfg.Get(host, "port")
	if err != nil {
		return Host{}, fmt.Errorf("unable to get port: %w", err)
	}
	var port uint64 = 22
	if portStr != "" {
		port, err = strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return Host{}, fmt.Errorf("unable to parse port: %w", err)
		}
	}

	identityFile, err := sshCfg.Get(host, "identityfile")
	if err != nil {
		return Host{}, fmt.Errorf("unable to get identityFile: %w", err)
	}

	return Host{
		User:         user,
		Host:         cmp.Or(hostName, host),
		Port:         uint16(port),
		IdentityFile: identityFile,
	}, nil
}

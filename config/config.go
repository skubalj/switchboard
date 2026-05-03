package config

import (
	"cmp"
	"fmt"
	"net/netip"
	"os"
	osuser "os/user"
	"path/filepath"
	"strconv"

	"github.com/kevinburke/ssh_config"
	"gopkg.in/yaml.v3"
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

	return Config{SSHConfigFile: filepath.Join(homedir, ".ssh", "config")}, nil
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
	SSHConfigFile string       `yaml:"sshConfigFile"`
	Connections   []Connection `yaml:"connections"`
}

type Connection struct {
	Host           Host          `yaml:"host"`
	LocalForwards  []PortForward `yaml:"localForwards"`
	RemoteForwards []PortForward `yaml:"remoteForwards"`
}

type Host struct {
	User         string `yaml:"user"`
	Host         string `yaml:"host"`
	Port         uint16 `yaml:"port"`
	IdentityFile string `yaml:"identityFile"`
}

type PortForward struct {
	LocalAddr  netip.AddrPort `yaml:"localAddr"`
	RemoteAddr netip.AddrPort `yaml:"remoteAddr"`
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
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return Host{}, fmt.Errorf("unable to parse port: %w", err)
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

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/alexflint/go-arg"
	"github.com/skubalj/switchboard/config"
	"github.com/skubalj/switchboard/tui"
)

const versionString = "v0.1.0"

type Args struct {
	GetConfig *getConfigSubcommand `arg:"subcommand:get-config" help:"print the config file"`
	SetConfig *setConfigSubcommand `arg:"subcommand:set-config" help:"set values in the config file"`

	ConfigFile string `arg:"--config-file" placeholder:"CONFIG_FILE" help:"override the switchboard config file [default: ~/.ssh/switchboard.json]"`
	Copyright  bool   `arg:"--copyright" help:"display GPL copyright notice"`
	LogFile    string `arg:"--log-file" placeholder:"LOG_FILE" help:"write logs to the given path"`
	Quiet      bool   `arg:"-q,--quiet" help:"do not show trace logging"`
}

func (Args) Version() string {
	return fmt.Sprintf(`switchboard %s
Copyright (C) 2026 Joseph Skubal
This program is free software released under the GNU GPLv3`, versionString)
}

func (a Args) LogConfig() config.LogConfig {
	return config.LogConfig{Quiet: a.Quiet, LogFile: a.LogFile}
}

const gplCopyrightNotice = `switchboard: SSH Port Forwarding Interface
Copyright (C) 2026 Joseph Skubal

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.`

type getConfigSubcommand struct{}

func (s getConfigSubcommand) Run(configFilePath string) error {
	cfg, err := config.GetConfig(configFilePath)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("config file '%s' not found", configFilePath)
	} else if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to serialize config data: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

type setConfigSubcommand struct {
	SSHConfigFile      string `arg:"--ssh-config-file" help:"set the SSH config file"`
	KnownHostsFile     string `arg:"--known-hosts-file" help:"set the SSH known_hosts file"`
	ImportHostKeyTypes bool   `arg:"--import-host-key-types" help:"import the set the set of host key types supported by the ssh command on this system"`
}

func (s setConfigSubcommand) Run(configFilePath string) error {
	cfg, err := config.GetConfig(configFilePath)
	if err != nil {
		cfg, _ = config.DefaultConfig()
	}

	if s.SSHConfigFile != "" {
		cfg.SSHConfigFile = s.SSHConfigFile
	}

	if s.KnownHostsFile != "" {
		cfg.KnownHostsFile = s.KnownHostsFile
	}

	if s.ImportHostKeyTypes {
		cfg.HostKeyAlgorithms, err = config.FetchHostKeyTypes()
		if err != nil {
			return err
		}
	}

	return config.SaveConfig(configFilePath, cfg)
}

func main() {
	var args Args
	arg.MustParse(&args)

	// Get config file path
	configFilePath := args.ConfigFile
	if configFilePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Printf("unable to find user home dir: %v\n", err)
			fmt.Println("You can manually configure the switchboard config file with '--config-file'")
			os.Exit(1)
		}
		configFilePath = filepath.Join(home, ".ssh", "switchboard.json")
	}

	var err error
	if args.GetConfig != nil {
		err = args.GetConfig.Run(configFilePath)
	} else if args.SetConfig != nil {
		err = args.SetConfig.Run(configFilePath)
	} else if args.Copyright {
		fmt.Println(gplCopyrightNotice)
	} else {
		err = mainCommand(args.LogConfig(), configFilePath)
		defer fmt.Println("Goodbye from switchboard!")
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func mainCommand(logConfig config.LogConfig, configFilePath string) error {
	// Read config file, or generate default
	cfg, err := config.GetConfig(configFilePath)
	if errors.Is(err, os.ErrNotExist) {
		cfg, err = config.DefaultConfig()
		if err != nil {
			return fmt.Errorf("unable to create default config file: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("unable to open config file: %w", err)
	}

	model := tui.InitialModel(logConfig, cfg)
	defer model.Close()
	p := tea.NewProgram(model)
	_, err = p.Run()
	if err != nil {
		return err
	}

	cfg.Connections = model.GetConfigConnections()
	return config.SaveConfig(configFilePath, cfg)
}

package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/alexflint/go-arg"
)

type Args struct {
	Verbose bool `arg:"-v,--verbose" help:"Show additional logging"`
}

func (Args) Epilogue() string {
	return `This program is free software released under the GNU GPLv3
Copyright (C) 2026 Joseph Skubal`
}

const gplCopyrightNotice = `tag-monorepo: Per-module Monorepo Tagging
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

func main() {
	var args Args
	arg.MustParse(&args)

	var initialModel tea.Model
	p := tea.NewProgram(initialModel)
	_, err := p.Run()
	if err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}

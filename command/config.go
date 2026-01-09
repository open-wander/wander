// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type ConfigCommand struct {
	Meta
}

func (f *ConfigCommand) Help() string {
	helpText := `
Usage: wander config <subcommand> [options] [args]

  This command groups subcommands for interacting with configurations.
  Users can validate configurations for the Wander agent.

  Validate configuration:

      $ wander config validate <config_path> [<config_path>...]

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *ConfigCommand) Synopsis() string {
	return "Interact with configurations"
}

func (f *ConfigCommand) Name() string { return "config" }

func (f *ConfigCommand) Run(args []string) int {
	return cli.RunResultHelp
}

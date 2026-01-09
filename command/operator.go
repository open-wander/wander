// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type OperatorCommand struct {
	Meta
}

func (f *OperatorCommand) Help() string {
	helpText := `
Usage: wander operator <subcommand> [options]

  Provides cluster-level tools for Wander operators, such as interacting with
  the Raft subsystem. NOTE: Use this command with extreme caution, as improper
  use could lead to a Wander outage and even loss of data.

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (f *OperatorCommand) Synopsis() string {
	return "Provides cluster-level tools for Wander operators"
}

func (f *OperatorCommand) Name() string { return "operator" }

func (f *OperatorCommand) Run(args []string) int {
	return cli.RunResultHelp
}

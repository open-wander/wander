// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type OperatorRaftCommand struct {
	Meta
}

func (c *OperatorRaftCommand) Help() string {
	helpText := `
Usage: wander operator raft <subcommand> [options]

  This command groups subcommands for interacting with Wander's Raft subsystem.
  The command can be used to verify Raft peers or in rare cases to recover
  quorum by removing invalid peers.

  List Raft peers:

      $ wander operator raft list-peers

  Remove a Raft peer:

      $ wander operator raft remove-peer -peer-address "IP:Port"

  Display info about the raft logs in the data directory:

      $ wander operator raft info /var/wander/data

  Display the log entries persisted in data dir in JSON format.

      $ wander operator raft logs /var/wander/data

  Display the server state obtained by replaying raft log entries
  persisted in data dir in JSON format.

      $ wander operator raft state /var/wander/data

  Please see the individual subcommand help for detailed usage information.


`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftCommand) Synopsis() string {
	return "Provides access to the Raft subsystem"
}

func (c *OperatorRaftCommand) Name() string { return "operator raft" }

func (c *OperatorRaftCommand) Run(args []string) int {
	return cli.RunResultHelp
}

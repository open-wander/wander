// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"github.com/open-wander/wander/version"
	"github.com/mitchellh/cli"
)

// VersionCommand is a Command implementation prints the version.
type VersionCommand struct {
	Version *version.VersionInfo
	Ui      cli.Ui
}

func (c *VersionCommand) Help() string {
	return ""
}

func (c *VersionCommand) Name() string { return "version" }

func (c *VersionCommand) Run(_ []string) int {
	c.Ui.Output(c.Version.FullVersionNumber(true))
	return 0
}

func (c *VersionCommand) Synopsis() string {
	return "Prints the Nomad version"
}

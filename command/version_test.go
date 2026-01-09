// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/open-wander/wander/ci"
)

func TestVersionCommand_implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VersionCommand{}
}

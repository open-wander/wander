// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/mitchellh/cli"
)

func TestSentinelListCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &SentinelListCommand{}
}

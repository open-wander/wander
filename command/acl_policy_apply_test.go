// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"os"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/command/agent"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestACLPolicyApplyCommand(t *testing.T) {
	ci.Parallel(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	must.NotNil(t, token)

	ui := cli.NewMockUi()
	cmd := &ACLPolicyApplyCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a test policy
	policy := mock.ACLPolicy()

	// Get a file
	file, rm := getTempFile(t, "nomad-test")
	t.Cleanup(rm)

	// Write the policy to the file
	err := os.WriteFile(file, []byte(policy.Rules), 0700)
	must.NoError(t, err)

	// Attempt to apply a policy without a valid management token
	code := cmd.Run([]string{"-address=" + url, "-token=foo", "test-policy", file})
	must.One(t, code)

	// Apply a policy with a valid management token
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "test-policy", file})
	must.Zero(t, code)

	// Check the output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, "Successfully wrote")
}

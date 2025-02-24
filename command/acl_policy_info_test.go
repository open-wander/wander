// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/command/agent"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestACLPolicyInfoCommand(t *testing.T) {
	ci.Parallel(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	state := srv.Agent.Server().State()
	defer srv.Shutdown()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	must.NotNil(t, token)

	// Create a test ACLPolicy
	policy := &structs.ACLPolicy{
		Name:  "testPolicy",
		Rules: "node { policy = \"read\" }",
	}
	policy.SetHash()
	must.NoError(t, state.UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy}))

	ui := cli.NewMockUi()
	cmd := &ACLPolicyInfoCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Attempt to apply a policy without a valid management token
	invalidToken := mock.ACLToken()
	code := cmd.Run([]string{"-address=" + url, "-token=" + invalidToken.SecretID, policy.Name})
	must.One(t, code)

	// Apply a policy with a valid management token
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, policy.Name})
	must.Zero(t, code)

	// Check the output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, policy.Name)
}

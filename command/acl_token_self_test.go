// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"testing"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/command/agent"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestACLTokenSelfCommand_ViaEnvVar(t *testing.T) {
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	state := srv.Agent.Server().State()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	must.NotNil(t, token)

	ui := cli.NewMockUi()
	cmd := &ACLTokenSelfCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a valid token
	mockToken := mock.ACLToken()
	mockToken.Policies = []string{acl.PolicyWrite}
	mockToken.SetHash()
	must.NoError(t, state.UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{mockToken}))

	// Attempt to fetch info on a token without providing a valid management
	// token
	invalidToken := mock.ACLToken()
	t.Setenv("NOMAD_TOKEN", invalidToken.SecretID)
	code := cmd.Run([]string{"-address=" + url})
	must.One(t, code)

	// Fetch info on a token with a valid token
	t.Setenv("NOMAD_TOKEN", mockToken.SecretID)
	code = cmd.Run([]string{"-address=" + url})
	must.Zero(t, code)

	// Check the output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, mockToken.AccessorID)
}

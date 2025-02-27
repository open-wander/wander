// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"strings"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
)

func TestDeploymentResumeCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &DeploymentResumeCommand{}
}

func TestDeploymentResumeCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &DeploymentResumeCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	if code := cmd.Run([]string{"-address=nope", "12"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error retrieving deployment") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestDeploymentResumeCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &DeploymentResumeCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake deployment
	state := srv.Agent.Server().State()
	d := mock.Deployment()
	assert.Nil(state.UpsertDeployment(1000, d))

	prefix := d.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(d.ID, res[0])
}

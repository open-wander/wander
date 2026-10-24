// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/testutil"
	"github.com/stretchr/testify/assert"
)

func TestIntegration_Command_WanderInit(t *testing.T) {
	ci.Parallel(t)
	tmpDir := t.TempDir()

	{
		cmd := exec.Command("wander", "job", "init")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("error running init: %v", err)
		}
	}

	{
		cmd := exec.Command("wander", "job", "validate", "example.nomad.hcl")
		cmd.Dir = tmpDir
		cmd.Env = []string{`WANDER_ADDR=http://127.0.0.1:0`}
		if err := cmd.Run(); err != nil {
			t.Fatalf("error validating example.nomad.hcl: %v", err)
		}
	}
}

func TestIntegration_Command_RoundTripJob(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	assert := assert.New(t)
	tmpDir := t.TempDir()

	// Start in dev mode so we get a node registration
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	{
		cmd := exec.Command("wander", "job", "init", "-short")
		cmd.Dir = tmpDir
		assert.Nil(cmd.Run())
	}

	{
		cmd := exec.Command("wander", "job", "run", "example.nomad.hcl")
		cmd.Dir = tmpDir
		cmd.Env = []string{fmt.Sprintf("WANDER_ADDR=%s", url)}
		err := cmd.Run()
		if err != nil && !strings.Contains(err.Error(), "exit status 2") {
			t.Fatalf("error running example.nomad.hcl: %v", err)
		}
	}

	{
		cmd := exec.Command("wander", "job", "inspect", "example")
		cmd.Dir = tmpDir
		cmd.Env = []string{fmt.Sprintf("WANDER_ADDR=%s", url)}
		out, err := cmd.Output()
		assert.Nil(err)

		var req api.JobRegisterRequest
		dec := json.NewDecoder(bytes.NewReader(out))
		assert.Nil(dec.Decode(&req))

		var resp api.JobRegisterResponse
		_, err = client.Raw().Write("/v1/jobs", req, &resp, nil)
		assert.Nil(err)
		assert.NotZero(resp.EvalID)
	}

	{
		cmd := exec.Command("wander", "job", "stop", "example")
		cmd.Dir = tmpDir
		cmd.Env = []string{fmt.Sprintf("WANDER_ADDR=%s", url)}
		_, err := cmd.Output()
		assert.Nil(err)
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package getter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/client/testutil"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

func artifactConfig(timeout time.Duration) *config.ArtifactConfig {
	return &config.ArtifactConfig{
		HTTPReadTimeout: timeout,
		HTTPMaxBytes:    1e6,
		GCSTimeout:      timeout,
		GitTimeout:      timeout,
		HgTimeout:       timeout,
		S3Timeout:       timeout,
	}
}

// comprehensive scenarios tested in e2e/artifact

func TestSandbox_Get_http(t *testing.T) {
	testutil.RequireRoot(t)
	logger := testlog.HCLogger(t)

	ac := artifactConfig(10 * time.Second)
	sbox := New(ac, logger)

	_, taskDir := SetupDir(t)
	env := noopTaskEnv(taskDir)

	artifact := &structs.TaskArtifact{
		GetterSource: "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod",
		RelativeDest: "local/downloads",
	}

	err := sbox.Get(env, artifact)
	must.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(taskDir, "local", "downloads", "go.mod"))
	must.NoError(t, err)
	must.StrContains(t, string(b), "module github.com/hashicorp/go-set")
}

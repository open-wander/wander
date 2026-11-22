// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

// -------------------------------------------------------------------------------
// Integration Test Helpers
//
// Project: Wander
//
// Polling for tests that read state the cluster converges on, and helpers that
// run a test job and read its allocs. Only reads are polled: a write is sent
// once, so a slow response is never resent to another server.
// -------------------------------------------------------------------------------

//go:build integration

package integration

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"

	"github.com/open-wander/wander/api"
)

// -------------------------------------------------------------------------
// POLLING
// -------------------------------------------------------------------------

// pollGap is the pause between two attempts of an eventually check.
const pollGap = time.Second

// eventually retries check until it returns nil, and fails the test with the
// last error check returned if timeout passes first.
func eventually(t *testing.T, timeout time.Duration, check func() error) {
	t.Helper()
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(check),
		wait.Timeout(timeout),
		wait.Gap(pollGap),
	))
}

// -------------------------------------------------------------------------
// JOBS
// -------------------------------------------------------------------------

// jobTimeout bounds how long a test job may take to place and start its
// allocs.
const jobTimeout = 2 * time.Minute

// runJob registers jobs/<name>.wander.hcl through client and purges the job in
// t.Cleanup. It returns the job's ID.
func runJob(t *testing.T, client *api.Client, name string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("jobs", name+".wander.hcl"))
	must.NoError(t, err)
	job, err := client.Jobs().ParseHCL(string(src), true)
	must.NoError(t, err)
	_, _, err = client.Jobs().Register(job, nil)
	must.NoError(t, err)

	t.Cleanup(func() {
		if _, _, err := client.Jobs().Deregister(*job.ID, true, nil); err != nil {
			t.Logf("purge job %s: %v", *job.ID, err)
		}
	})
	return *job.ID
}

// runningAllocs waits until jobID has want allocs running and returns them.
// Allocs in any other state are not counted.
func runningAllocs(t *testing.T, client *api.Client, jobID string, want int) []*api.AllocationListStub {
	t.Helper()
	var running []*api.AllocationListStub
	eventually(t, jobTimeout, func() error {
		allocs, _, err := client.Jobs().Allocations(jobID, false, nil)
		if err != nil {
			return err
		}
		running = nil
		for _, a := range allocs {
			if a.ClientStatus == api.AllocClientStatusRunning {
				running = append(running, a)
			}
		}
		if len(running) != want {
			return fmt.Errorf("%d of %d allocs of %s running, want %d", len(running), len(allocs), jobID, want)
		}
		return nil
	})
	return running
}

// allocFile returns the file at path in an alloc's directory, read through the
// fs API. The API refuses paths under a task's secrets/ directory.
func allocFile(client *api.Client, allocID, path string) (string, error) {
	alloc, _, err := client.Allocations().Info(allocID, nil)
	if err != nil {
		return "", err
	}
	r, err := client.AllocFS().Cat(alloc, path, nil)
	if err != nil {
		return "", err
	}
	defer r.Close()

	b, err := io.ReadAll(r)
	return string(b), err
}

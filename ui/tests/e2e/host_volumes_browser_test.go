// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-wander/wander/api"
)

// Synthetic identifiers shared with specs/host-volumes.spec.ts.
const (
	browserNodeA   = "aaaaaaaa-1111-2222-3333-444444444444"
	browserNodeB   = "bbbbbbbb-1111-2222-3333-444444444444"
	browserVolumeA = "11111111-aaaa-bbbb-cccc-000000000001"
	browserVolumeB = "22222222-aaaa-bbbb-cccc-000000000002"
	browserAlloc   = "dddddddd-1111-2222-3333-444444444444"
)

// TestHostVolumeBrowserStates drives Chromium against the real UI server, with
// a fake Nomad API behind it. The embedded release cluster cannot produce
// unavailable or forbidden states, and a browser route mock cannot intercept
// the UI server's own calls to the API, so the states are chosen by the
// synthetic namespace or volume ID that the fake API receives.
func TestHostVolumeBrowserStates(t *testing.T) {
	requireReleaseE2E(t, "host volume browser")
	if testing.Short() {
		t.Skip("Skipping e2e tests in short mode")
	}
	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("npx not found - skipping e2e tests")
	}
	if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
		t.Skip("node_modules not found - run 'npm install' first")
	}

	backend := httptest.NewServer(http.HandlerFunc(serveSyntheticHostVolumeAPI()))
	defer backend.Close()
	t.Setenv("WANDER_ADDR", backend.URL)
	t.Setenv("NOMAD_ADDR", backend.URL)

	server, cleanup := StartTestServer(TestServerConfig{})
	defer cleanup()
	t.Setenv("BASE_URL", server.URL())

	cmd := exec.Command("npx", "playwright", "test", "--project=chromium", "--config=playwright.host-volumes.config.ts")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("host volume Playwright tests failed: %v", err)
	}
}

// serveSyntheticHostVolumeAPI returns a fake Nomad API. The namespace picks
// the state: "team" and "*" succeed, "empty-ns" is empty, "broken" fails with
// 500, "denied" with 403, and "slow" answers after the first request late.
func serveSyntheticHostVolumeAPI() http.HandlerFunc {
	var slowCalls atomic.Int32
	volumes := []*api.HostVolumeStub{
		{ID: browserVolumeA, Name: "data-a", Namespace: "team", PluginID: "mkdir", NodeID: browserNodeA, NodePool: "pool-a", CapacityBytes: 1 << 30, State: api.HostVolumeStateReady},
		{ID: browserVolumeB, Name: "data-a", Namespace: "team", NodeID: browserNodeB, NodePool: "pool-b", CapacityBytes: 2 << 30, State: api.HostVolumeStatePending},
	}
	claims := []*api.HostVolumeClaim{
		{ID: "claim-1", Namespace: "team", VolumeID: browserVolumeA, VolumeName: "data-a", JobID: "web", TaskGroupName: "app", AllocID: browserAlloc},
		{ID: "claim-2", Namespace: "team", VolumeID: browserVolumeA, VolumeName: "data-a", JobID: "web", TaskGroupName: "app", AllocID: "eeeeeeee-1111"},
		{ID: "claim-3", Namespace: "team", VolumeID: browserVolumeB, VolumeName: "data-a", JobID: "batch", TaskGroupName: "other", AllocID: "ffffffff-1111"},
	}
	created := time.Date(2026, time.March, 4, 5, 6, 7, 0, time.UTC).UnixNano()
	detail := &api.HostVolume{
		ID: browserVolumeA, Name: "data-a", Namespace: "team", Type: "host", NodeID: browserNodeA, NodePool: "pool-a",
		HostPath: `/srv/<script>window.__pwned = true</script>`, PluginID: "mkdir", CapacityBytes: 1 << 30,
		RequestedCapacityMinBytes: 1 << 29, RequestedCapacityMaxBytes: 2 << 30,
		RequestedCapabilities: []*api.HostVolumeCapability{{AttachmentMode: api.HostVolumeAttachmentModeFilesystem, AccessMode: api.HostVolumeAccessModeSingleNodeWriter}},
		Constraints:           []*api.Constraint{{LTarget: "${attr.kernel.name}", Operand: "=", RTarget: "linux"}},
		Parameters:            map[string]string{"mode": `"><img src=x onerror="window.__pwned = true">`},
		State:                 api.HostVolumeStateReady, StatusDescription: "Plugin reported a healthy mount",
		Origin:      api.HostVolumeOriginPluginCreated,
		Attachments: []*api.HostVolumeAttachment{{AllocationID: browserAlloc, NodeID: browserNodeA, AttachmentMode: api.HostVolumeAttachmentModeFilesystem, AccessMode: api.HostVolumeAccessModeSingleNodeWriter}},
		CreateTime:  created, ModifyTime: created + int64(time.Hour), CreateIndex: 42, ModifyIndex: 57,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		namespace := r.URL.Query().Get("namespace")
		switch {
		case r.URL.Path == "/v1/acl/token/self":
			http.Error(w, "ACL support disabled", http.StatusBadRequest)
		case r.URL.Path == "/v1/volumes" && r.URL.Query().Get("type") == "host":
			switch namespace {
			case "broken":
				http.Error(w, "backend failure", http.StatusInternalServerError)
			case "denied":
				http.Error(w, "Permission denied", http.StatusForbidden)
			case "slow":
				if slowCalls.Add(1) > 1 {
					time.Sleep(1500 * time.Millisecond)
				}
				writeBrowserJSON(w, volumes)
			case "team", "*":
				writeBrowserJSON(w, volumes)
			default:
				writeBrowserJSON(w, []*api.HostVolumeStub{})
			}
		case r.URL.Path == "/v1/volumes/claims":
			if namespace == "team" || namespace == "*" || namespace == "slow" {
				writeBrowserJSON(w, claims)
			} else {
				writeBrowserJSON(w, []*api.HostVolumeClaim{})
			}
		case strings.HasPrefix(r.URL.Path, "/v1/volume/host/"):
			switch strings.TrimPrefix(r.URL.Path, "/v1/volume/host/") {
			case browserVolumeA:
				writeBrowserJSON(w, detail)
			case "denied-volume":
				http.Error(w, "Permission denied", http.StatusForbidden)
			case "broken-volume":
				http.Error(w, "backend failure", http.StatusInternalServerError)
			default:
				http.NotFound(w, r)
			}
		case r.URL.Path == "/v1/jobs", r.URL.Path == "/v1/namespaces", r.URL.Path == "/v1/regions",
			r.URL.Path == "/v1/allocations", r.URL.Path == "/v1/nodes", r.URL.Path == "/v1/evaluations":
			writeBrowserJSON(w, []string{})
		case r.URL.Path == "/v1/agent/members":
			_, _ = w.Write([]byte(`{"ServerName":"","ServerRegion":"global","ServerDC":"dc1","Members":[]}`))
		case r.URL.Path == "/v1/status/leader":
			_, _ = w.Write([]byte(`""`))
		default:
			http.NotFound(w, r)
		}
	}
}

func writeBrowserJSON(w http.ResponseWriter, value any) {
	_ = json.NewEncoder(w).Encode(value)
}

// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ci"
	"github.com/shoenig/test/must"
)

// expectedHostVolume is the volume every equivalent spec below decodes to.
func expectedHostVolume() *api.HostVolume {
	return &api.HostVolume{
		ID:                        "vol-a",
		Name:                      "data-a",
		Namespace:                 "ns-a",
		NodeID:                    "node-a",
		NodePool:                  "pool-a",
		HostPath:                  "/srv/data-a",
		PluginID:                  "mkdir",
		CapacityBytes:             3 << 30,
		RequestedCapacityMinBytes: 1 << 30,
		RequestedCapacityMaxBytes: 2 << 30,
		Parameters:                map[string]string{"k1": "v1", "k2": "v2"},
		RequestedCapabilities: []*api.HostVolumeCapability{
			{AccessMode: api.HostVolumeAccessModeSingleNodeWriter, AttachmentMode: api.HostVolumeAttachmentModeFilesystem},
			{AccessMode: api.HostVolumeAccessModeSingleNodeReader, AttachmentMode: api.HostVolumeAttachmentModeFilesystem},
		},
		Constraints: []*api.Constraint{
			{LTarget: "${meta.rack}", Operand: "=", RTarget: "r1"},
			{LTarget: "${attr.kernel.name}", Operand: "=", RTarget: "linux"},
		},
	}
}

const hostSpecHCL = `
id           = "vol-a"
name         = "data-a"
namespace    = "ns-a"
type         = "host"
plugin_id    = "mkdir"
node_id      = "node-a"
node_pool    = "pool-a"
host_path    = "/srv/data-a"
capacity     = "3GiB"
capacity_min = "1GiB"
capacity_max = "2GiB"
parameters {
  k1 = "v1"
  k2 = "v2"
}
capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}
capability {
  access_mode     = "single-node-reader-only"
  attachment_mode = "file-system"
}
constraint {
  attribute = "${meta.rack}"
  operator  = "="
  value     = "r1"
}
constraint {
  attribute = "${attr.kernel.name}"
  operator  = "="
  value     = "linux"
}
`

const hostSpecJSON = `{
  "id": "vol-a", "name": "data-a", "namespace": "ns-a", "type": "host",
  "plugin_id": "mkdir", "node_id": "node-a", "node_pool": "pool-a",
  "host_path": "/srv/data-a",
  "capacity": "3GiB", "capacity_min": "1GiB", "capacity_max": "2GiB",
  "parameters": [{"k1": "v1", "k2": "v2"}],
  "capability": [
    {"access_mode": "single-node-writer", "attachment_mode": "file-system"},
    {"access_mode": "single-node-reader-only", "attachment_mode": "file-system"}
  ],
  "constraint": [
    {"attribute": "${meta.rack}", "operator": "=", "value": "r1"},
    {"attribute": "${attr.kernel.name}", "operator": "=", "value": "linux"}
  ]
}`

// hostSpecJSONObjects uses the object form for the parameters and the
// repeated keys form that the HCL JSON syntax also allows.
const hostSpecJSONObjects = `{
  "id": "vol-a", "name": "data-a", "namespace": "ns-a", "type": "host",
  "plugin_id": "mkdir", "node_id": "node-a", "node_pool": "pool-a",
  "host_path": "/srv/data-a",
  "capacity": "3GiB", "capacity_min": "1GiB", "capacity_max": "2GiB",
  "parameters": {"k1": "v1", "k2": "v2"},
  "capability": [
    {"access_mode": "single-node-writer", "attachment_mode": "file-system"},
    {"access_mode": "single-node-reader-only", "attachment_mode": "file-system"}
  ],
  "constraint": [
    {"attribute": "${meta.rack}", "operator": "=", "value": "r1"},
    {"attribute": "${attr.kernel.name}", "operator": "=", "value": "linux"}
  ]
}`

func decodeHostSpec(t *testing.T, spec string) (*api.HostVolume, error) {
	t.Helper()
	ast, typ, err := parseVolumeType(spec)
	must.NoError(t, err)
	must.Eq(t, "host", typ)
	return hostDecodeVolume(ast)
}

func TestHostVolumeDecode_FormatsAreEquivalent(t *testing.T) {
	ci.Parallel(t)

	for name, spec := range map[string]string{
		"hcl":          hostSpecHCL,
		"json":         hostSpecJSON,
		"json objects": hostSpecJSONObjects,
	} {
		t.Run(name, func(t *testing.T) {
			vol, err := decodeHostSpec(t, spec)
			must.NoError(t, err)
			must.Eq(t, expectedHostVolume(), vol)
		})
	}
}

func TestHostVolumeDecode_OneBlockAndMinimal(t *testing.T) {
	ci.Parallel(t)

	want := &api.HostVolume{
		Name:     "data-a",
		PluginID: "mkdir",
		RequestedCapabilities: []*api.HostVolumeCapability{
			{AccessMode: api.HostVolumeAccessModeSingleNodeWriter, AttachmentMode: api.HostVolumeAttachmentModeFilesystem},
		},
	}
	for name, spec := range map[string]string{
		"hcl": `
type = "host"
name = "data-a"
plugin_id = "mkdir"
capability {
  access_mode = "single-node-writer"
  attachment_mode = "file-system"
}`,
		"json array": `{"type":"host","name":"data-a","plugin_id":"mkdir",
 "capability":[{"access_mode":"single-node-writer","attachment_mode":"file-system"}]}`,
		"json object": `{"type":"host","name":"data-a","plugin_id":"mkdir",
 "capability":{"access_mode":"single-node-writer","attachment_mode":"file-system"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			vol, err := decodeHostSpec(t, spec)
			must.NoError(t, err)
			must.Eq(t, want, vol)
		})
	}
}

func TestHostVolumeDecode_Refused(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name string
		spec string
		want string
	}{
		{"unknown key hcl", "type = \"host\"\nsecret_key = \"x\"", "volume specification has an unknown key"},
		{"unknown key json", `{"type":"host","secret_key":"x"}`, "volume specification has an unknown key"},
		{"csi key", "type = \"host\"\nexternal_id = \"x\"", "volume specification has an unknown key"},
		{"unknown key in capability", "type = \"host\"\ncapability {\n access_mode = \"single-node-writer\"\n mode = \"x\"\n}", "capability has an unknown key"},
		{"unknown key in capability json", `{"type":"host","capability":[{"mode":"x"}]}`, "capability has an unknown key"},
		{"unknown key in constraint", "type = \"host\"\nconstraint {\n attribute = \"a\"\n bogus = \"x\"\n}", "constraint has an unknown key"},
		{"unknown key in constraint json", `{"type":"host","constraint":[{"bogus":"x"}]}`, "constraint has an unknown key"},
		{"scalar twice", "type = \"host\"\nname = \"a\"\nname = \"b\"", "volume specification gives a key twice"},
		{"scalar twice json", `{"type":"host","name":"a","name":"b"}`, "volume specification gives a key twice"},
		{"parameters twice", "type = \"host\"\nparameters {\n a = \"b\"\n}\nparameters {\n c = \"d\"\n}", "volume specification gives a key twice"},
		{"key twice in capability", "type = \"host\"\ncapability {\n access_mode = \"a\"\n access_mode = \"b\"\n}", "capability gives a key twice"},
		{"parameter twice", "type = \"host\"\nparameters {\n a = \"b\"\n a = \"c\"\n}", "parameters gives a key twice"},
		{"bad capacity_min", "type = \"host\"\ncapacity_min = \"LEAKED-SIZE\"", "capacity_min is not a valid size"},
		{"bad capacity_max", "type = \"host\"\ncapacity_max = \"LEAKED-SIZE\"", "capacity_max is not a valid size"},
		{"bad capacity", "type = \"host\"\ncapacity = \"LEAKED-SIZE\"", "capacity is not a valid size"},
		{"capacity number", "type = \"host\"\ncapacity_min = 5", "capacity_min is not a valid size"},
		{"capacity too large", "type = \"host\"\ncapacity_min = \"100EiB\"", "capacity_min is not a valid size"},
		{"scalar is a block", "type = \"host\"\nname {\n a = \"b\"\n}", "name must be a string"},
		{"scalar is a number", "type = \"host\"\nnode_id = 5", "node_id must be a string"},
		{"block is a string", "type = \"host\"\ncapability = \"x\"", "capability must be a block"},
		{"parameters is a string", "type = \"host\"\nparameters = \"x\"", "parameters must be a block"},
		{"parameter value not a string", "type = \"host\"\nparameters {\n a = 5\n}", "parameters must be strings"},
		{"capability value not a string", "type = \"host\"\ncapability {\n access_mode = 5\n}", "capability values must be strings"},
		{"labelled block", "type = \"host\"\ncapability \"x\" {\n access_mode = \"a\"\n}", "volume specification has an unknown key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeHostSpec(t, tc.spec)
			must.EqError(t, err, tc.want)
		})
	}
}

// A refusal never echoes a host path, a parameter value or an unknown key.
func TestHostVolumeDecode_ErrorsDoNotEchoInput(t *testing.T) {
	ci.Parallel(t)

	for _, spec := range []string{
		"type = \"host\"\nhost_path = \"/LEAK-PATH\"\nLEAK-KEY = \"LEAK-VALUE\"",
		"type = \"host\"\nhost_path = \"/LEAK-PATH\"\nparameters {\n p = 5\n}",
		"type = \"host\"\nhost_path = \"/LEAK-PATH\"\ncapacity_min = \"LEAK-SIZE\"",
		"type = \"host\"\nparameters {\n LEAK-KEY = \"v\"\n LEAK-KEY = \"v\"\n}",
		"type = \"host\"\ncapability {\n LEAK-KEY = \"v\"\n}",
	} {
		_, err := decodeHostSpec(t, spec)
		must.Error(t, err)
		must.StrNotContains(t, err.Error(), "LEAK")
	}
}

func TestVolumeInit_Host(t *testing.T) {
	// Not parallel: t.Chdir is refused in a parallel test.
	cases := []struct {
		name string
		args []string
		file string
		want string
	}{
		{"hcl default name", []string{"-type", "host"}, "volume.hcl", "hcl"},
		{"json default name", []string{"-type", "host", "-json"}, "volume.json", "json"},
		{"hcl named", []string{"-type=host", "custom.hcl"}, "custom.hcl", "hcl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ui := cli.NewMockUi()
			cmd := &VolumeInitCommand{Meta: Meta{Ui: ui}}
			t.Chdir(dir)

			must.Eq(t, 0, cmd.Run(tc.args))
			must.StrContains(t, ui.OutputWriter.String(), "Example volume specification written to "+tc.file)

			raw, err := os.ReadFile(filepath.Join(dir, tc.file))
			must.NoError(t, err)
			spec := string(raw)
			if tc.want == "json" {
				must.Eq(t, strings.TrimSpace(spec), strings.TrimSpace(defaultJsonHostVolumeSpec))
				must.True(t, json.Valid(raw))
			} else {
				must.Eq(t, strings.TrimSpace(spec), strings.TrimSpace(defaultHclHostVolumeSpec))
			}

			// The generated spec is accepted by the decoder.
			vol, err := decodeHostSpec(t, spec)
			must.NoError(t, err)
			must.Eq(t, "data-a", vol.Name)
			must.SliceNotEmpty(t, vol.RequestedCapabilities)
		})
	}
}

func TestVolumeInit_CSIIsDefault(t *testing.T) {
	// Not parallel: t.Chdir is refused in a parallel test.
	for _, tc := range []struct {
		name string
		args []string
		file string
		want string
	}{
		{"no flag", nil, "volume.hcl", defaultHclVolumeSpec},
		{"explicit csi", []string{"-type", "csi"}, "volume.hcl", defaultHclVolumeSpec},
		{"json", []string{"-json"}, "volume.json", defaultJsonVolumeSpec},
		{"csi json", []string{"-type=csi", "-json"}, "volume.json", defaultJsonVolumeSpec},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			ui := cli.NewMockUi()
			cmd := &VolumeInitCommand{Meta: Meta{Ui: ui}}
			must.Eq(t, 0, cmd.Run(tc.args))
			raw, err := os.ReadFile(filepath.Join(dir, tc.file))
			must.NoError(t, err)
			must.Eq(t, tc.want, string(raw))
		})
	}
}

func TestVolumeInit_UnknownType(t *testing.T) {
	// Not parallel: t.Chdir is refused in a parallel test.
	dir := t.TempDir()
	t.Chdir(dir)
	ui := cli.NewMockUi()
	cmd := &VolumeInitCommand{Meta: Meta{Ui: ui}}
	must.Eq(t, 1, cmd.Run([]string{"-type", "LEAK-TYPE"}))
	must.StrContains(t, ui.ErrorWriter.String(), `Volume type must be "csi" or "host"`)
	must.StrNotContains(t, ui.ErrorWriter.String(), "LEAK-TYPE")
	entries, err := os.ReadDir(dir)
	must.NoError(t, err)
	must.SliceEmpty(t, entries)
}

// fakeHostVolumeServer stands in for the host volume endpoints.
type fakeHostVolumeServer struct {
	*httptest.Server

	mu sync.Mutex

	// createVolume is the volume of the create and register replies.
	createVolume api.HostVolume
	warnings     string
	writeStatus  int
	writeBody    string

	// states are the replies of the info endpoint in turn; the last repeats.
	// A nil state answers 404.
	states []*api.HostVolume

	createReq   []api.HostVolumeCreateRequest
	registerReq []api.HostVolumeRegisterRequest
	infoQueries []map[string]string
	otherPaths  []string
}

func newFakeHostVolumeServer(t *testing.T) *fakeHostVolumeServer {
	t.Helper()
	f := &fakeHostVolumeServer{
		createVolume: api.HostVolume{ID: "vol-1", Name: "data-a", Namespace: "ns-a", ModifyIndex: 7},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/volume/host/create", func(w http.ResponseWriter, r *http.Request) {
		var req api.HostVolumeCreateRequest
		must.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		f.mu.Lock()
		f.createReq = append(f.createReq, req)
		f.mu.Unlock()
		f.reply(w)
	})
	mux.HandleFunc("/v1/volume/host/register", func(w http.ResponseWriter, r *http.Request) {
		var req api.HostVolumeRegisterRequest
		must.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		f.mu.Lock()
		f.registerReq = append(f.registerReq, req)
		f.mu.Unlock()
		f.reply(w)
	})
	mux.HandleFunc("/v1/volume/host/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		q := map[string]string{
			"id":        strings.TrimPrefix(r.URL.Path, "/v1/volume/host/"),
			"namespace": r.URL.Query().Get("namespace"),
			"index":     r.URL.Query().Get("index"),
		}
		f.infoQueries = append(f.infoQueries, q)
		n := len(f.infoQueries) - 1
		if n >= len(f.states) {
			n = len(f.states) - 1
		}
		if n < 0 || f.states[n] == nil {
			http.Error(w, "volume not found", http.StatusNotFound)
			return
		}
		w.Header().Set("X-Nomad-Index", strconv.Itoa(20+n))
		_ = json.NewEncoder(w).Encode(f.states[n])
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.otherPaths = append(f.otherPaths, r.URL.Path)
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"Volumes":[]}`))
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeHostVolumeServer) reply(w http.ResponseWriter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writeStatus != 0 {
		http.Error(w, f.writeBody, f.writeStatus)
		return
	}
	vol := f.createVolume
	_ = json.NewEncoder(w).Encode(&api.HostVolumeWriteResponse{Volume: &vol, Warnings: f.warnings})
}

func hostVol(state api.HostVolumeState, desc string) *api.HostVolume {
	return &api.HostVolume{
		ID: "vol-1", Name: "data-a", Namespace: "ns-a", NodeID: "node-a", NodePool: "pool-a",
		PluginID: "mkdir", CapacityBytes: 1 << 30, State: state, StatusDescription: desc,
	}
}

func writeSpec(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "spec")
	must.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func runCreate(t *testing.T, f *fakeHostVolumeServer, args ...string) (int, *cli.MockUi) {
	t.Helper()
	ui := cli.NewMockUi()
	cmd := &VolumeCreateCommand{Meta: Meta{Ui: ui}}
	return cmd.Run(append([]string{"-address=" + f.URL}, args...)), ui
}

func runRegister(t *testing.T, f *fakeHostVolumeServer, args ...string) (int, *cli.MockUi) {
	t.Helper()
	ui := cli.NewMockUi()
	cmd := &VolumeRegisterCommand{Meta: Meta{Ui: ui}}
	return cmd.Run(append([]string{"-address=" + f.URL}, args...)), ui
}

func TestVolumeCreate_Host_DetachReturnsAtOnce(t *testing.T) {
	ci.Parallel(t)

	f := newFakeHostVolumeServer(t)
	f.createVolume.State = api.HostVolumeStatePending
	code, ui := runCreate(t, f, "-detach", writeSpec(t, hostSpecHCL))
	must.Eq(t, 0, code)
	must.Eq(t, "Created host volume vol-1\n", ui.OutputWriter.String())
	must.SliceEmpty(t, f.infoQueries)
	must.Len(t, 1, f.createReq)
	must.Eq(t, expectedHostVolume(), f.createReq[0].Volume)
	must.False(t, f.createReq[0].PolicyOverride)
}

func TestVolumeCreate_Host_MonitorsUntilReady(t *testing.T) {
	ci.Parallel(t)

	f := newFakeHostVolumeServer(t)
	f.createVolume.State = api.HostVolumeStatePending
	f.states = []*api.HostVolume{
		hostVol(api.HostVolumeStatePending, ""),
		hostVol(api.HostVolumeStateReady, ""),
	}
	code, ui := runCreate(t, f, writeSpec(t, hostSpecHCL))
	must.Eq(t, 0, code)

	out := ui.OutputWriter.String()
	must.Eq(t, "Created host volume vol-1\nVolume vol-1 is ready\n", out)

	// Each query is for the namespace of the volume, and blocks on the index
	// of the last reply.
	must.Len(t, 2, f.infoQueries)
	must.Eq(t, map[string]string{"id": "vol-1", "namespace": "ns-a", "index": "7"}, f.infoQueries[0])
	must.Eq(t, map[string]string{"id": "vol-1", "namespace": "ns-a", "index": "20"}, f.infoQueries[1])
}

func TestVolumeCreate_Host_PrintsEachStateChange(t *testing.T) {
	ci.Parallel(t)

	f := newFakeHostVolumeServer(t)
	f.createVolume.State = api.HostVolumeStateRequested
	f.states = []*api.HostVolume{
		hostVol(api.HostVolumeStatePending, ""),
		hostVol(api.HostVolumeStatePending, ""),
		hostVol(api.HostVolumeStateReady, ""),
	}
	code, ui := runCreate(t, f, writeSpec(t, hostSpecHCL))
	must.Eq(t, 0, code)
	must.Eq(t, "Created host volume vol-1\nVolume vol-1 is pending\nVolume vol-1 is ready\n", ui.OutputWriter.String())
}

func TestVolumeCreate_Host_ReadyReplyNeedsNoQuery(t *testing.T) {
	ci.Parallel(t)

	f := newFakeHostVolumeServer(t)
	f.createVolume.State = api.HostVolumeStateReady
	code, ui := runCreate(t, f, writeSpec(t, hostSpecHCL))
	must.Eq(t, 0, code)
	must.Eq(t, "Created host volume vol-1\nVolume vol-1 is ready\n", ui.OutputWriter.String())
	must.SliceEmpty(t, f.infoQueries)
}

func TestVolumeCreate_Host_FailedExitsOne(t *testing.T) {
	ci.Parallel(t)

	for name, replyState := range map[string]api.HostVolumeState{
		"monitored": api.HostVolumeStatePending,
		"in reply":  api.HostVolumeStateFailed,
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeHostVolumeServer(t)
			f.createVolume.State = replyState
			f.createVolume.StatusDescription = "plugin failed"
			f.states = []*api.HostVolume{
				hostVol(api.HostVolumeStatePending, ""),
				hostVol(api.HostVolumeStateFailed, "plugin failed"),
			}
			code, ui := runCreate(t, f, writeSpec(t, hostSpecHCL))
			must.Eq(t, 1, code)
			must.StrContains(t, ui.OutputWriter.String(), "Created host volume vol-1")
			must.StrContains(t, ui.ErrorWriter.String(), "Volume vol-1 failed: plugin failed")
		})
	}
}

func TestVolumeCreate_Host_VolumeGoneWhileMonitoring(t *testing.T) {
	ci.Parallel(t)

	f := newFakeHostVolumeServer(t)
	f.createVolume.State = api.HostVolumeStatePending
	f.states = []*api.HostVolume{hostVol(api.HostVolumeStatePending, ""), nil}
	code, ui := runCreate(t, f, writeSpec(t, hostSpecHCL))
	must.Eq(t, 1, code)
	must.StrContains(t, ui.ErrorWriter.String(), "Error monitoring volume:")
	must.StrContains(t, ui.ErrorWriter.String(), "volume not found")
}

func TestVolumeCreate_Host_Verbose(t *testing.T) {
	ci.Parallel(t)

	run := func(t *testing.T, args ...string) string {
		f := newFakeHostVolumeServer(t)
		f.createVolume.State = api.HostVolumeStatePending
		f.states = []*api.HostVolume{hostVol(api.HostVolumeStateReady, "")}
		code, ui := runCreate(t, f, append(args, writeSpec(t, hostSpecHCL))...)
		must.Eq(t, 0, code)
		return ui.OutputWriter.String()
	}

	plain := run(t)
	must.StrNotContains(t, plain, "Node Pool")

	verbose := run(t, "-verbose")
	for _, want := range []string{"ID", "Name", "Namespace", "Plugin ID", "Node ID", "Node Pool", "Capacity", "State", "ready", "node-a", "pool-a", "1.0 GiB"} {
		must.StrContains(t, verbose, want)
	}
}

func TestVolumeCreate_Host_PolicyOverrideAndWarnings(t *testing.T) {
	ci.Parallel(t)

	f := newFakeHostVolumeServer(t)
	f.createVolume.State = api.HostVolumeStateReady
	f.warnings = "policy override has no effect"
	code, ui := runCreate(t, f, "-policy-override", "-detach", writeSpec(t, hostSpecHCL))
	must.Eq(t, 0, code)
	must.Len(t, 1, f.createReq)
	must.True(t, f.createReq[0].PolicyOverride)
	must.StrContains(t, ui.ErrorWriter.String(), "policy override has no effect")
	// Warnings go to stderr and the ID to stdout, so a script reading stdout
	// gets the ID alone.
	must.StrContains(t, ui.OutputWriter.String(), "Created host volume vol-1")
	must.StrNotContains(t, ui.OutputWriter.String(), "policy override has no effect")
}

func TestVolumeCreate_Host_ServerErrorsPassThrough(t *testing.T) {
	ci.Parallel(t)

	for name, tc := range map[string]struct {
		status int
		body   string
	}{
		"namespace mismatch": {http.StatusBadRequest, "request namespace does not match volume namespace"},
		"forbidden":          {http.StatusForbidden, "Permission denied"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeHostVolumeServer(t)
			f.writeStatus, f.writeBody = tc.status, tc.body
			code, ui := runCreate(t, f, writeSpec(t, hostSpecHCL))
			must.Eq(t, 1, code)
			must.StrContains(t, ui.ErrorWriter.String(), "Error creating volume:")
			must.StrContains(t, ui.ErrorWriter.String(), tc.body)
			must.SliceEmpty(t, f.infoQueries)
		})
	}
}

func TestVolumeCreate_Host_RefusedSpecSendsNothing(t *testing.T) {
	ci.Parallel(t)

	f := newFakeHostVolumeServer(t)
	code, ui := runCreate(t, f, writeSpec(t, "type = \"host\"\nbogus = \"x\""))
	must.Eq(t, 1, code)
	must.StrContains(t, ui.ErrorWriter.String(), "Error decoding the volume definition: volume specification has an unknown key")
	must.SliceEmpty(t, f.createReq)
}

func TestVolumeCreate_Host_UpdateSendsID(t *testing.T) {
	ci.Parallel(t)

	f := newFakeHostVolumeServer(t)
	f.createVolume.State = api.HostVolumeStateReady
	code, ui := runCreate(t, f, writeSpec(t, hostSpecHCL))
	must.Eq(t, 0, code)
	must.Eq(t, "vol-a", f.createReq[0].Volume.ID)
	must.Eq(t, "Created host volume vol-1\nVolume vol-1 is ready\n", ui.OutputWriter.String())

	// The output of an update equals that of a create without an ID.
	f2 := newFakeHostVolumeServer(t)
	f2.createVolume.State = api.HostVolumeStateReady
	noID := strings.Replace(hostSpecHCL, "id           = \"vol-a\"\n", "", 1)
	must.NotEq(t, noID, hostSpecHCL)
	code, ui2 := runCreate(t, f2, writeSpec(t, noID))
	must.Eq(t, 0, code)
	must.Eq(t, "", f2.createReq[0].Volume.ID)
	must.Eq(t, ui2.OutputWriter.String(), ui.OutputWriter.String())
}

func TestVolumeCreate_Host_JSONAndStdinMatchFile(t *testing.T) {
	// Not parallel: it swaps os.Stdin.
	for name, spec := range map[string]string{"hcl": hostSpecHCL, "json": hostSpecJSON} {
		t.Run(name, func(t *testing.T) {
			f := newFakeHostVolumeServer(t)
			f.createVolume.State = api.HostVolumeStateReady

			r, w, err := os.Pipe()
			must.NoError(t, err)
			_, err = w.WriteString(spec)
			must.NoError(t, err)
			must.NoError(t, w.Close())
			old := os.Stdin
			os.Stdin = r
			t.Cleanup(func() { os.Stdin = old; _ = r.Close() })

			code, _ := runCreate(t, f, "-detach", "-")
			must.Eq(t, 0, code)
			must.Eq(t, expectedHostVolume(), f.createReq[0].Volume)
		})
	}
}

func TestVolumeRegister_Host(t *testing.T) {
	ci.Parallel(t)

	f := newFakeHostVolumeServer(t)
	f.warnings = "policy override has no effect"
	code, ui := runRegister(t, f, "-policy-override", writeSpec(t, hostSpecHCL))
	must.Eq(t, 0, code)
	must.Eq(t, "Registered host volume vol-1; no storage was created or modified\n", ui.OutputWriter.String())
	must.StrContains(t, ui.ErrorWriter.String(), "policy override has no effect")
	must.SliceEmpty(t, f.createReq)
	must.Len(t, 1, f.registerReq)
	must.True(t, f.registerReq[0].PolicyOverride)
	must.Eq(t, expectedHostVolume(), f.registerReq[0].Volume)
	must.SliceEmpty(t, f.infoQueries)
}

func TestVolumeRegister_Host_Update(t *testing.T) {
	ci.Parallel(t)

	f := newFakeHostVolumeServer(t)
	code, ui := runRegister(t, f, writeSpec(t, hostSpecJSON))
	must.Eq(t, 0, code)
	must.Eq(t, "vol-a", f.registerReq[0].Volume.ID)
	must.False(t, f.registerReq[0].PolicyOverride)
	must.Eq(t, "Registered host volume vol-1; no storage was created or modified\n", ui.OutputWriter.String())

	// A spec without an ID prints the same line.
	f2 := newFakeHostVolumeServer(t)
	code, ui2 := runRegister(t, f2, writeSpec(t, strings.Replace(hostSpecHCL, "id           = \"vol-a\"\n", "", 1)))
	must.Eq(t, 0, code)
	must.Eq(t, "", f2.registerReq[0].Volume.ID)
	must.Eq(t, ui2.OutputWriter.String(), ui.OutputWriter.String())
}

func TestVolumeRegister_Host_Errors(t *testing.T) {
	ci.Parallel(t)

	f := newFakeHostVolumeServer(t)
	f.writeStatus, f.writeBody = http.StatusBadRequest, "request namespace does not match volume namespace"
	code, ui := runRegister(t, f, writeSpec(t, hostSpecHCL))
	must.Eq(t, 1, code)
	must.StrContains(t, ui.ErrorWriter.String(), "Error registering volume:")
	must.StrContains(t, ui.ErrorWriter.String(), "request namespace does not match volume namespace")

	f2 := newFakeHostVolumeServer(t)
	code, ui = runRegister(t, f2, writeSpec(t, "type = \"host\"\nhost_path = \"/LEAK\"\nbogus = 1"))
	must.Eq(t, 1, code)
	must.StrNotContains(t, ui.ErrorWriter.String(), "LEAK")
	must.SliceEmpty(t, f2.registerReq)
}

// The CSI branch ignores the host-only flags and still uses the CSI endpoint.
func TestVolumeCreate_CSIIgnoresHostFlags(t *testing.T) {
	ci.Parallel(t)

	f := newFakeHostVolumeServer(t)
	spec := writeSpec(t, "id = \"v1\"\nname = \"v1\"\ntype = \"csi\"\nplugin_id = \"p\"\n")
	code, _ := runCreate(t, f, "-detach", "-verbose", "-policy-override", spec)
	must.Eq(t, 0, code)
	must.SliceEmpty(t, f.createReq)
	must.SliceEmpty(t, f.infoQueries)
	must.Len(t, 1, f.otherPaths)
	must.StrContains(t, f.otherPaths[0], "/v1/volume/csi/")
}

// TestVolumeCreateRegister_Host_RealAgent runs the commands against a real
// agent, so the paths, index header and reply shapes are those of the real
// HTTP handlers and the api client.
func TestVolumeCreateRegister_Host_RealAgent(t *testing.T) {
	ci.Parallel(t)

	_, client, url := testServer(t, true, nil)
	waitForNodes(t, client)
	nodes, _, err := client.Nodes().List(nil)
	must.NoError(t, err)
	must.SliceNotEmpty(t, nodes)

	spec0 := `
type      = "host"
name      = "data-a"
plugin_id = "mkdir"
node_id   = "` + nodes[0].ID + `"
host_path = "` + t.TempDir() + `"
capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}
`
	spec := writeSpec(t, spec0)

	ui := cli.NewMockUi()
	cmd := &VolumeRegisterCommand{Meta: Meta{Ui: ui}}
	code := cmd.Run([]string{"-address=" + url, spec})
	must.Eq(t, 0, code, must.Sprintf("stderr: %s", ui.ErrorWriter.String()))
	out := ui.OutputWriter.String()
	must.StrHasPrefix(t, "Registered host volume ", out)
	must.StrHasSuffix(t, "; no storage was created or modified\n", out)
	id := strings.TrimSuffix(strings.TrimPrefix(out, "Registered host volume "), "; no storage was created or modified\n")

	vol, _, err := client.HostVolumes().Info(id, nil)
	must.NoError(t, err)
	must.Eq(t, "data-a", vol.Name)

	// Create with -detach reaches the real create handler and prints its ID.
	ui = cli.NewMockUi()
	create := &VolumeCreateCommand{Meta: Meta{Ui: ui}}
	code = create.Run([]string{"-address=" + url, "-detach", spec})

	// The real server refuses the second volume of the same name; the
	// fixed error text of the server reaches the user as returned.
	must.Eq(t, 1, code)
	must.StrContains(t, ui.ErrorWriter.String(), "Error creating volume: ")
	must.StrContains(t, ui.ErrorWriter.String(), "name_conflict")
	must.Eq(t, "", ui.OutputWriter.String())

	// A fresh name reaches the create handler and prints the ID.
	other := writeSpec(t, strings.Replace(spec0, `"data-a"`, `"data-b"`, 1))
	ui = cli.NewMockUi()
	create = &VolumeCreateCommand{Meta: Meta{Ui: ui}}
	code = create.Run([]string{"-address=" + url, "-detach", other})
	must.Eq(t, 0, code, must.Sprintf("stderr: %s", ui.ErrorWriter.String()))
	must.StrHasPrefix(t, "Created host volume ", ui.OutputWriter.String())
}

// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-wander/wander/api"
)

const (
	hvNodeA   = "aaaaaaaa-1111-2222-3333-444444444444"
	hvNodeB   = "bbbbbbbb-1111-2222-3333-444444444444"
	hvVolA    = "11111111-aaaa-bbbb-cccc-000000000001"
	hvVolB    = "22222222-aaaa-bbbb-cccc-000000000002"
	hvVolC    = "33333333-aaaa-bbbb-cccc-000000000003"
	hvAlloc   = "dddddddd-1111-2222-3333-444444444444"
	hvGiB     = int64(1024 * 1024 * 1024)
	hvScript  = `/srv/<script>alert("path")</script>`
	hvAttrVal = `" onmouseover="alert(1)`

	// hvTeamToken is the secret of a client token whose only policy grants
	// host-volume-read in the "team" namespace.
	hvTeamToken  = "synthetic-team-reader"
	hvTeamPolicy = "team-hv"
)

// hvBackend is a fake Nomad API that records host volume calls.
type hvBackend struct {
	mu          sync.Mutex
	volumes     []*api.HostVolumeStub
	claims      []*api.HostVolumeClaim
	info        map[string]*api.HostVolume
	listStatus  int
	claimStatus int
	infoStatus  int
	listQueries []string
	claimCalls  []string
	infoCalls   []string
}

func (b *hvBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	query := r.URL.RawQuery
	switch {
	case r.URL.Path == "/v1/acl/token/self":
		if r.Header.Get("X-Nomad-Token") == hvTeamToken {
			_ = writeJSONNoT(w, &api.ACLToken{AccessorID: "hv-team", SecretID: hvTeamToken, Type: "client", Policies: []string{hvTeamPolicy}})
			return
		}
		http.Error(w, "ACL support disabled", http.StatusBadRequest)
	case r.URL.Path == "/v1/acl/policy/"+hvTeamPolicy:
		_ = writeJSONNoT(w, &api.ACLPolicy{Name: hvTeamPolicy, Rules: `namespace "team" { capabilities = ["host-volume-read"] }`})
	case r.URL.Path == "/v1/namespaces" || r.URL.Path == "/v1/regions":
		_ = writeJSONNoT(w, []string{})
	case r.URL.Path == "/v1/volumes" && r.URL.Query().Get("type") == "host":
		b.listQueries = append(b.listQueries, query)
		if b.listStatus != 0 {
			http.Error(w, "list failure", b.listStatus)
			return
		}
		_ = writeJSONNoT(w, b.volumes)
	case r.URL.Path == "/v1/volumes/claims":
		b.claimCalls = append(b.claimCalls, query)
		if b.claimStatus != 0 {
			http.Error(w, "claims failure", b.claimStatus)
			return
		}
		_ = writeJSONNoT(w, b.claims)
	case strings.HasPrefix(r.URL.Path, "/v1/volume/host/"):
		id := strings.TrimPrefix(r.URL.Path, "/v1/volume/host/")
		b.infoCalls = append(b.infoCalls, id+"?"+query)
		if b.infoStatus != 0 {
			http.Error(w, "info failure", b.infoStatus)
			return
		}
		vol, ok := b.info[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_ = writeJSONNoT(w, vol)
	default:
		http.NotFound(w, r)
	}
}

func hvStubs() []*api.HostVolumeStub {
	return []*api.HostVolumeStub{
		{ID: hvVolA, Name: "data-a", Namespace: "team", PluginID: "mkdir", NodeID: hvNodeA, NodePool: "pool-a", CapacityBytes: hvGiB, State: api.HostVolumeStateReady},
		{ID: hvVolB, Name: "data-a", Namespace: "team", NodeID: hvNodeB, NodePool: "pool-b", CapacityBytes: 2 * hvGiB, State: api.HostVolumeStatePending},
		{ID: hvVolC, Name: "data-c", Namespace: "data", NodeID: hvNodeA, NodePool: "pool-a", State: api.HostVolumeStateFailed},
	}
}

func hvClaims() []*api.HostVolumeClaim {
	return []*api.HostVolumeClaim{
		{ID: "claim-1", Namespace: "team", VolumeID: hvVolA, VolumeName: "data-a", JobID: "web", TaskGroupName: "app", AllocID: hvAlloc},
		{ID: "claim-2", Namespace: "team", VolumeID: hvVolA, VolumeName: "data-a", JobID: "web", TaskGroupName: "app", AllocID: "eeeeeeee-1111"},
		{ID: "claim-3", Namespace: "team", VolumeID: hvVolB, VolumeName: "data-a", JobID: "batch", TaskGroupName: "other", AllocID: "ffffffff-1111"},
		// Same volume ID in another namespace must not be counted for team.
		{ID: "claim-4", Namespace: "elsewhere", VolumeID: hvVolA, VolumeName: "data-a", JobID: "x", TaskGroupName: "y", AllocID: "gggggggg"},
	}
}

func hvInfo() *api.HostVolume {
	created := time.Date(2026, time.March, 4, 5, 6, 7, 0, time.UTC).UnixNano()
	return &api.HostVolume{
		ID: hvVolA, Name: "data-a", Namespace: "team", Type: "host", NodeID: hvNodeA, NodePool: "pool-a",
		HostPath: hvScript, PluginID: "mkdir", CapacityBytes: hvGiB,
		RequestedCapacityMinBytes: hvGiB / 2, RequestedCapacityMaxBytes: 2 * hvGiB,
		RequestedCapabilities: []*api.HostVolumeCapability{
			{AttachmentMode: api.HostVolumeAttachmentModeFilesystem, AccessMode: api.HostVolumeAccessModeSingleNodeWriter},
		},
		Constraints: []*api.Constraint{{LTarget: "${attr.kernel.name}", Operand: "=", RTarget: "linux"}},
		Parameters: map[string]string{"zeta": `<script>alert("param")</script>`, "alpha": hvAttrVal,
			"beta": "b", "gamma": "g", "delta": "d", "epsilon": "e", "eta": "h"},
		State: api.HostVolumeStateReady, StatusDescription: "Plugin degraded <b>diag</b>",
		Origin: api.HostVolumeOriginPluginCreated,
		Attachments: []*api.HostVolumeAttachment{{
			AllocationID: hvAlloc, NodeID: hvNodeA, AttachmentMode: api.HostVolumeAttachmentModeFilesystem,
			AccessMode: api.HostVolumeAccessModeSingleNodeWriter, ReadOnly: true,
		}},
		CreateTime: created, ModifyTime: created + int64(time.Hour), CreateIndex: 42, ModifyIndex: 57,
	}
}

// hvTestHandler returns the real router built by NewRouter, wired to the fake
// backend, so every request passes through the route middleware.
func hvTestHandler(t *testing.T, backend *hvBackend) http.Handler {
	t.Helper()
	server := httptest.NewServer(backend)
	t.Cleanup(server.Close)
	t.Setenv("WANDER_ADDR", server.URL)
	getTokenCache().Clear()
	t.Cleanup(getTokenCache().Clear)
	return NewRouter(newJobsTestHandler(t, server.URL))
}

func hvServe(t *testing.T, handler http.Handler, path string, htmx bool, token string, wantStatus int) string {
	t.Helper()
	return hvServeMethod(t, handler, http.MethodGet, path, htmx, token, wantStatus)
}

func hvServeMethod(t *testing.T, handler http.Handler, method, path string, htmx bool, token string, wantStatus int) string {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	if htmx {
		request.Header.Set(HXRequest, "true")
		request.Header.Set("HX-Target", "host-volumes-table-container")
	}
	if token != "" {
		request.Header.Set(XNomadToken, token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d; body: %s", method, path, response.Code, wantStatus, response.Body.String())
	}
	return response.Body.String()
}

func hvRefuteAll(t *testing.T, body string, values ...string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(body, value) {
			t.Errorf("response unexpectedly contains %q", value)
		}
	}
}

// hvTeamReader builds permissions for a token that may read host volumes only
// in the "team" namespace.
func hvTeamReader(t *testing.T) *UserPermissions {
	t.Helper()
	perms := NewUserPermissions(&ACLTokenInfo{
		Token:      &api.ACLToken{Type: "client", SecretID: "synthetic-secret"},
		IsValid:    true,
		ACLEnabled: true,
	})
	if err := perms.parsePolicy(&api.ACLPolicy{
		Name:  "team-hv",
		Rules: `namespace "team" { capabilities = ["host-volume-read"] }`,
	}); err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	return perms
}

func TestHostVolumeListRendersColumnsAndCountsClaimsByVolumeID(t *testing.T) {
	backend := &hvBackend{volumes: hvStubs()[:2], claims: hvClaims()}
	handler := hvTestHandler(t, backend)

	body := hvServe(t, handler, "/ui/host-volumes?namespace=team", false, "", http.StatusOK)

	assertContainsAll(t, body,
		"<title>Dynamic host volumes", "Dynamic host volumes", "CSI volumes are listed under CSI",
		"data-a", "team", "pool-a", "pool-b", "mkdir", "None (registered)", "1.0 GB", "2.0 GB",
		"ready", "pending",
		`href="/ui/host-volumes/`+hvVolA+`?namespace=team"`,
		`href="/ui/host-volumes/`+hvVolB+`?namespace=team"`,
		`href="/ui/clients/`+hvNodeA+`"`, ">aaaaaaaa<", ">bbbbbbbb<",
		`id="host-volumes-table-container"`,
		`data-claims="2"`, `data-claims="1"`,
	)
	// Names repeat across nodes: a count by name would give 3 to both rows.
	hvRefuteAll(t, body, `data-claims="3"`, `data-claims="4"`)
	if len(backend.claimCalls) != 1 {
		t.Fatalf("claim list calls = %d, want exactly 1: %v", len(backend.claimCalls), backend.claimCalls)
	}
	if !strings.Contains(backend.claimCalls[0], "namespace=team") || strings.Contains(backend.claimCalls[0], "filter") {
		t.Errorf("claim query = %q, want namespace=team and no filter", backend.claimCalls[0])
	}
	if !strings.Contains(backend.listQueries[0], "namespace=team") || !strings.Contains(backend.listQueries[0], "type=host") {
		t.Errorf("list query = %q", backend.listQueries[0])
	}
}

func TestHostVolumeListHTMXReturnsOnlyTheTable(t *testing.T) {
	backend := &hvBackend{volumes: hvStubs()[:2], claims: hvClaims()}
	body := hvServe(t, hvTestHandler(t, backend), "/ui/host-volumes?namespace=team", true, "", http.StatusOK)

	assertContainsAll(t, body, "data-a", `data-claims="2"`)
	hvRefuteAll(t, body, "<!doctype", "<html", "Dynamic host volumes</h1>")
}

func TestHostVolumeListFiltersByNodeAndKeepsNodeOptions(t *testing.T) {
	backend := &hvBackend{volumes: hvStubs()[:2], claims: hvClaims()}
	handler := hvTestHandler(t, backend)

	body := hvServe(t, handler, "/ui/host-volumes?namespace=team&node="+hvNodeB, false, "", http.StatusOK)

	assertContainsAll(t, body, `href="/ui/host-volumes/`+hvVolB+`?namespace=team"`, "pool-b")
	hvRefuteAll(t, body, `href="/ui/host-volumes/`+hvVolA+`?namespace=team"`)
	// The node selector is derived from the unfiltered list.
	assertContainsAll(t, body, `value="`+hvNodeA+`"`, `value="`+hvNodeB+`" selected`)
	if strings.Contains(backend.listQueries[0], "node_id") {
		t.Errorf("node filter was sent to the API (%q); the handler filters instead", backend.listQueries[0])
	}
}

func TestHostVolumeListEmptyStates(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		want string
		not  string
	}{
		{name: "no volumes", path: "/ui/host-volumes?namespace=team", want: "are visible in this namespace", not: "match this node"},
		{name: "node filter without matches", path: "/ui/host-volumes?namespace=team&node=cccccccc", want: "No dynamic host volumes match this node", not: "are visible in this namespace"},
	} {
		t.Run(test.name, func(t *testing.T) {
			volumes := []*api.HostVolumeStub{}
			if strings.Contains(test.path, "node=") {
				volumes = hvStubs()[:1]
			}
			handler := hvTestHandler(t, &hvBackend{volumes: volumes, claims: []*api.HostVolumeClaim{}})
			body := hvServe(t, handler, test.path, false, "", http.StatusOK)
			assertContainsAll(t, body, test.want)
			hvRefuteAll(t, body, test.not)
		})
	}
}

func TestHostVolumeListWildcardPassesThroughAndPreChecksOnlyConcreteNamespaces(t *testing.T) {
	token := hvTeamToken
	for _, test := range []struct {
		name       string
		path       string
		wantStatus int
		wantCalls  int
	}{
		{name: "wildcard reaches the server", path: "/ui/host-volumes?namespace=*", wantStatus: http.StatusOK, wantCalls: 1},
		{name: "readable namespace", path: "/ui/host-volumes?namespace=team", wantStatus: http.StatusOK, wantCalls: 1},
		{name: "unreadable namespace is refused before the API", path: "/ui/host-volumes?namespace=other", wantStatus: http.StatusForbidden, wantCalls: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := &hvBackend{volumes: hvStubs(), claims: hvClaims()}
			body := hvServe(t, hvTestHandler(t, backend), test.path, false, token, test.wantStatus)
			if len(backend.listQueries) != test.wantCalls {
				t.Fatalf("list calls = %d, want %d", len(backend.listQueries), test.wantCalls)
			}
			if test.wantStatus == http.StatusForbidden {
				assertContainsAll(t, body, "host-volume-read")
			}
		})
	}

	backend := &hvBackend{volumes: hvStubs(), claims: hvClaims()}
	body := hvServe(t, hvTestHandler(t, backend), "/ui/host-volumes?namespace=*", false, token, http.StatusOK)
	assertContainsAll(t, body, `href="/ui/host-volumes/`+hvVolC+`?namespace=data"`, `href="/ui/host-volumes/`+hvVolA+`?namespace=team"`)
	if !strings.Contains(backend.listQueries[0], "namespace=%2A") && !strings.Contains(backend.listQueries[0], "namespace=*") {
		t.Errorf("wildcard was not passed to the API: %q", backend.listQueries[0])
	}
}

func TestCanReadHostVolume(t *testing.T) {
	team := hvTeamReader(t)
	for _, test := range []struct {
		name      string
		perms     *UserPermissions
		namespace string
		want      bool
	}{
		{name: "granted namespace", perms: team, namespace: "team", want: true},
		{name: "other namespace", perms: team, namespace: "other", want: false},
		{name: "wildcard does not match a scoped grant", perms: team, namespace: "*", want: false},
		{name: "acl disabled", perms: NewUserPermissions(&ACLTokenInfo{IsValid: true}), namespace: "any", want: true},
		{name: "management", perms: NewUserPermissions(&ACLTokenInfo{Token: &api.ACLToken{Type: "management"}, IsValid: true, ACLEnabled: true}), namespace: "any", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.perms.CanReadHostVolume(test.namespace); got != test.want {
				t.Errorf("CanReadHostVolume(%q) = %t, want %t", test.namespace, got, test.want)
			}
		})
	}
}

func TestHostVolumeListClaimFailureDoesNotFailThePage(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			backend := &hvBackend{volumes: hvStubs()[:2], claimStatus: status}
			body := hvServe(t, hvTestHandler(t, backend), "/ui/host-volumes?namespace=team", false, "", http.StatusOK)
			assertContainsAll(t, body, "data-a", "Claims unavailable", `data-claims="unavailable"`)
			hvRefuteAll(t, body, `data-claims="0"`)
		})
	}
}

func TestHostVolumeListBackendErrorsAreTruthful(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     int
		wantStatus int
	}{
		{name: "forbidden", status: http.StatusForbidden, wantStatus: http.StatusForbidden},
		{name: "unavailable", status: http.StatusInternalServerError, wantStatus: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := &hvBackend{volumes: hvStubs(), claims: hvClaims(), listStatus: test.status}
			body := hvServe(t, hvTestHandler(t, backend), "/ui/host-volumes?namespace=team", false, "", test.wantStatus)
			hvRefuteAll(t, body, "No dynamic host volumes")
		})
	}
}

func TestHostVolumeDetailRendersEverySection(t *testing.T) {
	backend := &hvBackend{info: map[string]*api.HostVolume{hvVolA: hvInfo()}, claims: hvClaims()}
	handler := hvTestHandler(t, backend)

	body := hvServe(t, handler, "/ui/host-volumes/"+hvVolA+"?namespace=team", false, "", http.StatusOK)

	assertContainsAll(t, body,
		"<title>data-a - Dynamic host volume", "Dynamic host volume",
		"Identity", "Status", "Diagnostics", "Placement", "Plugin", "Capacity",
		"Requested capabilities", "Parameters", "Constraints", "Attachments", "Claims", "Lifecycle",
		hvVolA, "team", "plugin-created", "pool-a", "mkdir",
		`href="/ui/clients/`+hvNodeA+`"`,
		"512.0 MB", "2.0 GB", "file-system", "single-node-writer",
		"${attr.kernel.name}", "linux",
		`href="/ui/allocations/`+hvAlloc+`"`,
		"claim-1", "claim-2",
		`href="/ui/jobs/web?namespace=team"`, "app",
		"4 Mar 2026 05:06:07 UTC", "4 Mar 2026 06:06:07 UTC", ">42<", ">57<",
		"ready",
	)
	// Claims of a different volume with the same name or a different
	// namespace are not shown.
	hvRefuteAll(t, body, "claim-3", "claim-4")
	if len(backend.infoCalls) != 1 || !strings.Contains(backend.infoCalls[0], "namespace=team") {
		t.Errorf("info calls = %v", backend.infoCalls)
	}
	if len(backend.claimCalls) != 1 || !strings.Contains(backend.claimCalls[0], "volume_name=data-a") || !strings.Contains(backend.claimCalls[0], "namespace=team") {
		t.Errorf("claim calls = %v", backend.claimCalls)
	}

	htmxBody := hvServe(t, handler, "/ui/host-volumes/"+hvVolA+"?namespace=team", true, "", http.StatusOK)
	assertContainsAll(t, htmxBody, "Requested capabilities", "Lifecycle")
	hvRefuteAll(t, htmxBody, "<!doctype")
}

func TestHostVolumeDetailEscapesOperatorSuppliedValues(t *testing.T) {
	backend := &hvBackend{info: map[string]*api.HostVolume{hvVolA: hvInfo()}, claims: []*api.HostVolumeClaim{}}
	body := hvServe(t, hvTestHandler(t, backend), "/ui/host-volumes/"+hvVolA+"?namespace=team", false, "", http.StatusOK)

	assertContainsAll(t, body,
		"/srv/&lt;script&gt;alert(&#34;path&#34;)&lt;/script&gt;",
		"&lt;script&gt;alert(&#34;param&#34;)&lt;/script&gt;",
		"&#34; onmouseover=&#34;alert(1)",
		"Plugin degraded &lt;b&gt;diag&lt;/b&gt;",
	)
	hvRefuteAll(t, body, "<script>alert", "<b>diag", `" onmouseover="alert(1)`)
	// Parameters render sorted by key.
	last := -1
	for _, key := range []string{"alpha", "beta", "delta", "epsilon", "eta", "gamma", "zeta"} {
		at := strings.Index(body, ">"+key+"<")
		if at <= last {
			t.Fatalf("parameter %q is out of order or missing (index %d after %d)", key, at, last)
		}
		last = at
	}
}

func TestHostVolumeDetailOmitsUnreportedHostPathAndParameters(t *testing.T) {
	info := hvInfo()
	info.HostPath, info.Parameters, info.PluginID, info.Attachments, info.Constraints = "", nil, "", nil, nil
	backend := &hvBackend{info: map[string]*api.HostVolume{hvVolA: info}, claims: []*api.HostVolumeClaim{}}
	body := hvServe(t, hvTestHandler(t, backend), "/ui/host-volumes/"+hvVolA+"?namespace=team", false, "", http.StatusOK)

	assertContainsAll(t, body, "Host path", "Not reported", "None (registered)", "No attachments", "No claims", "No constraints")
	hvRefuteAll(t, body, "Parameters</")
}

func TestHostVolumeDetailClaimFailureKeepsTheVolume(t *testing.T) {
	backend := &hvBackend{info: map[string]*api.HostVolume{hvVolA: hvInfo()}, claimStatus: http.StatusInternalServerError}
	body := hvServe(t, hvTestHandler(t, backend), "/ui/host-volumes/"+hvVolA+"?namespace=team", false, "", http.StatusOK)
	assertContainsAll(t, body, "data-a", "Claims unavailable")
	hvRefuteAll(t, body, "No claims")
}

func TestHostVolumeDetailErrorsAndGuards(t *testing.T) {
	for _, test := range []struct {
		name       string
		path       string
		token      string
		infoStatus int
		wantStatus int
		wantCalls  int
	}{
		{name: "missing", path: "/ui/host-volumes/missing?namespace=team", wantStatus: http.StatusNotFound, wantCalls: 1},
		{name: "denied by server", path: "/ui/host-volumes/missing?namespace=team", infoStatus: http.StatusForbidden, wantStatus: http.StatusForbidden, wantCalls: 1},
		{name: "unavailable", path: "/ui/host-volumes/missing?namespace=team", infoStatus: http.StatusInternalServerError, wantStatus: http.StatusServiceUnavailable, wantCalls: 1},
		{name: "denied before the API", path: "/ui/host-volumes/" + hvVolA + "?namespace=other", token: hvTeamToken, wantStatus: http.StatusForbidden},
		{name: "single namespace needed", path: "/ui/host-volumes/" + hvVolA + "?namespace=*", wantStatus: http.StatusNotFound},
		{name: "slash in id", path: "/ui/host-volumes/a%2Fb?namespace=team", wantStatus: http.StatusNotFound},
		{name: "invented subroute", path: "/ui/host-volumes/" + hvVolA + "/claims?namespace=team", wantStatus: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := &hvBackend{infoStatus: test.infoStatus, info: map[string]*api.HostVolume{}}
			body := hvServe(t, hvTestHandler(t, backend), test.path, false, test.token, test.wantStatus)
			if len(backend.infoCalls) != test.wantCalls {
				t.Errorf("info calls = %d, want %d", len(backend.infoCalls), test.wantCalls)
			}
			if test.name == "single namespace needed" {
				assertContainsAll(t, body, "single namespace")
			}
		})
	}
}

func TestHostVolumeRoutesRejectOtherMethods(t *testing.T) {
	handler := hvTestHandler(t, &hvBackend{})
	for _, path := range []string{"/ui/host-volumes", "/ui/host-volumes/" + hvVolA} {
		hvServeMethod(t, handler, http.MethodPost, path, false, "", http.StatusMethodNotAllowed)
	}
}

func TestHostVolumeRoutesAreRegisteredWithAndWithoutTrailingSlash(t *testing.T) {
	backend := &hvBackend{volumes: hvStubs()[:1], claims: []*api.HostVolumeClaim{}, info: map[string]*api.HostVolume{hvVolA: hvInfo()}}
	server := httptest.NewServer(backend)
	defer server.Close()
	t.Setenv("WANDER_ADDR", server.URL)
	getTokenCache().Clear()
	router := NewRouter(newJobsTestHandler(t, server.URL))

	for _, path := range []string{
		"/ui/host-volumes", "/ui/host-volumes/",
		"/ui/host-volumes/" + hvVolA + "?namespace=team", "/ui/host-volumes/" + hvVolA + "/?namespace=team",
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		assertRouteStatus(t, response, http.MethodGet, path, http.StatusOK)
	}
}

func writeJSONNoT(w http.ResponseWriter, value any) error {
	return json.NewEncoder(w).Encode(value)
}

func TestHostVolumeDetailMappingSortsParametersByKey(t *testing.T) {
	parameters := map[string]string{}
	for i := 0; i < 20; i++ {
		parameters[fmt.Sprintf("key-%02d", i)] = "value"
	}
	// Map iteration order is random, so repeat the mapping.
	for attempt := 0; attempt < 50; attempt++ {
		detail := hostVolumeDetail(&api.HostVolume{Parameters: parameters})
		for i, parameter := range detail.Parameters {
			if want := fmt.Sprintf("key-%02d", i); parameter.Key != want {
				t.Fatalf("attempt %d: parameter %d = %q, want %q", attempt, i, parameter.Key, want)
			}
		}
	}
}

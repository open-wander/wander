// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package e2e

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/open-wander/wander/api"
)

const smokeJobID = "wander-ui-smoke"

const (
	namespaceSmokeName      = "team-a"
	namespaceSmokeJobID     = "wander-ui-team-smoke"
	regionSmokeName         = "region-a"
	regionSmokeJobID        = "wander-ui-region-smoke"
	parameterizedSmokeNS    = "dispatch-test"
	parameterizedSmokeJobID = "wander-ui-parameterized"
)

// TestE2E is the main entry point for running Playwright e2e tests.
// It starts the test server, sets up the environment, runs Playwright tests,
// and cleans up after completion.
func TestE2E(t *testing.T) {
	requireReleaseE2E(t, "browser")

	// Skip if running unit tests only
	if testing.Short() {
		t.Skip("Skipping e2e tests in short mode")
	}

	// Check if npm and npx are available
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not found - skipping e2e tests")
	}
	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("npx not found - skipping e2e tests")
	}

	// Check if node_modules exists
	if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
		t.Skip("node_modules not found - run 'npm install' first")
	}

	clusterAddress, stopCluster := startTestCluster(t)
	defer stopCluster()
	t.Setenv("NOMAD_ADDR", clusterAddress)

	t.Logf("Testing embedded UI at %s", clusterAddress)
	t.Setenv("BASE_URL", clusterAddress)

	cmd := exec.Command("npx", "playwright", "test", "--project=chromium")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "." // Current directory (tests/e2e)

	if err := cmd.Run(); err != nil {
		t.Fatalf("Playwright tests failed: %v", err)
	}
}

func startTestCluster(t *testing.T) (string, func()) {
	t.Helper()

	if address := os.Getenv("WANDER_UI_E2E_CLUSTER_ADDR"); address != "" {
		waitForCluster(t, address)
		binary := os.Getenv("WANDER_UI_E2E_BINARY")
		registerSmokeJob(t, binary, address, "", nil)
		registerNamespaceSmokeJob(t, binary, address)
		registerParameterizedSmokeJob(t, binary, address)
		return address, func() {}
	}

	binary := buildTestBinary(t)
	requireAgentPortsAvailable(t)

	dataDir := filepath.Join(t.TempDir(), "cluster")
	startTestAgent(t, binary, "-dev", "-bind=127.0.0.1", "-data-dir="+dataDir, "-log-level=WARN")

	address := "http://127.0.0.1:4646"
	waitForCluster(t, address)
	regionAddress := startFederatedRegion(t, binary)
	join := exec.Command(binary, "server", "join", "-address="+address, "127.0.0.1:5648")
	runCommand(t, join, "federate disposable Wander regions")
	waitForRegion(t, address, regionSmokeName)
	registerSmokeJob(t, binary, address, "", nil)
	registerNamespaceSmokeJob(t, binary, address)
	registerParameterizedSmokeJob(t, binary, address)
	registerFixtureJob(t, binary, regionAddress, "", nil, "", regionSmokeJobID, "fixtures/region-smoke.nomad.hcl")
	t.Setenv("WANDER_UI_E2E_FEDERATED", "1")
	return address, func() {}
}

func startFederatedRegion(t *testing.T, binary string) string {
	t.Helper()
	config := fmt.Sprintf(`region = %q
data_dir = %q
bind_addr = "127.0.0.1"

ports {
  http = 5646
  rpc  = 5647
  serf = 5648
}
`, regionSmokeName, filepath.Join(t.TempDir(), "region-cluster"))
	configPath := filepath.Join(t.TempDir(), "region.hcl")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write disposable region configuration: %v", err)
	}
	startTestAgent(t, binary, "-dev", "-config="+configPath, "-log-level=WARN")
	address := "http://127.0.0.1:5646"
	waitForCluster(t, address)
	return address
}

func registerNamespaceSmokeJob(t *testing.T, binary, address string) {
	t.Helper()
	createTestNamespace(t, binary, address, namespaceSmokeName)
	registerFixtureJob(t, binary, address, "", nil, namespaceSmokeName, namespaceSmokeJobID, "fixtures/namespace-smoke.nomad.hcl")
}

func registerParameterizedSmokeJob(t *testing.T, binary, address string) {
	t.Helper()
	createTestNamespace(t, binary, address, parameterizedSmokeNS)
	registerParameterizedFixture(t, binary, address, "", parameterizedSmokeNS)
	waitForHTTP(t, "parameterized smoke job did not become visible", func(client *http.Client) bool {
		resp, err := client.Get(address + "/v1/job/" + parameterizedSmokeJobID + "?namespace=" + parameterizedSmokeNS)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})
}

func createTestNamespace(t *testing.T, binary, address, namespace string) {
	t.Helper()
	apply := exec.Command(binary, "namespace", "apply", "-address="+address, namespace)
	runCommand(t, apply, "create browser smoke namespace "+namespace)
}

func registerParameterizedFixture(t *testing.T, binary, address, token, namespace string) {
	t.Helper()
	runFixtureJob(t, binary, address, token, namespace, "fixtures/parameterized.nomad.hcl")
}

func requireAgentPortsAvailable(t *testing.T) {
	t.Helper()
	for _, address := range []string{
		"127.0.0.1:4646", "127.0.0.1:4647", "127.0.0.1:4648",
		"127.0.0.1:5646", "127.0.0.1:5647", "127.0.0.1:5648",
	} {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatalf("Wander end-to-end port %s is already in use: %v", address, err)
		}
		if err := listener.Close(); err != nil {
			t.Fatalf("release Wander end-to-end port %s: %v", address, err)
		}
	}
}

func startTestAgent(t *testing.T, binary string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	agentArgs := append([]string{"agent"}, args...)
	agent := exec.CommandContext(ctx, binary, agentArgs...)
	agent.Stdout = os.Stdout
	agent.Stderr = os.Stderr
	if err := agent.Start(); err != nil {
		cancel()
		t.Fatalf("start disposable Wander cluster: %v", err)
	}

	stopped := make(chan error, 1)
	go func() { stopped <- agent.Wait() }()
	cleanup := func() {
		cancel()
		select {
		case <-stopped:
		case <-time.After(10 * time.Second):
			_ = agent.Process.Kill()
			<-stopped
		}
	}
	t.Cleanup(cleanup)
}

func buildTestBinary(t *testing.T) string {
	t.Helper()
	repositoryRoot := testRepositoryRoot(t)
	binary := filepath.Join(t.TempDir(), "wander")
	build := exec.Command("go", "build", "-tags", "ui", "-o", binary, ".")
	build.Dir = repositoryRoot
	runCommand(t, build, "build Wander for release tests")
	return binary
}

func testRepositoryRoot(t *testing.T) string {
	t.Helper()
	repositoryRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return repositoryRoot
}

func waitForCluster(t *testing.T, address string) {
	t.Helper()
	waitForClusterWithClient(t, address, &http.Client{Timeout: time.Second})
}

func waitForClusterWithClient(t *testing.T, address string, client *http.Client) {
	t.Helper()
	waitForHTTP(t, "disposable Wander cluster did not become ready at "+address, func(_ *http.Client) bool {
		resp, err := client.Get(address + "/v1/status/leader")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		var leader string
		return resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&leader) == nil && leader != ""
	})
}

func waitForRegion(t *testing.T, address, region string) {
	t.Helper()
	waitForHTTP(t, "federated region did not become visible: "+region, func(client *http.Client) bool {
		resp, err := client.Get(address + "/v1/regions")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		var regions []string
		if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&regions) != nil {
			return false
		}
		for _, available := range regions {
			if available == region {
				return true
			}
		}
		return false
	})
}

func requireReleaseE2E(t *testing.T, suite string) {
	t.Helper()
	if os.Getenv("WANDER_UI_E2E") != "1" {
		t.Skipf("set WANDER_UI_E2E=1 to run %s end-to-end tests", suite)
	}
}

func registerSmokeJob(t *testing.T, binary, address, token string, requestClient *http.Client) {
	t.Helper()
	registerFixtureJob(t, binary, address, token, requestClient, "", smokeJobID, "fixtures/smoke.nomad.hcl")
}

func registerFixtureJob(t *testing.T, binary, address, token string, requestClient *http.Client, namespace, jobID, fixture string) {
	t.Helper()
	if binary == "" {
		t.Fatal("WANDER_UI_E2E_BINARY is required with WANDER_UI_E2E_CLUSTER_ADDR")
	}

	runFixtureJob(t, binary, address, token, namespace, fixture)

	if requestClient == nil {
		requestClient = &http.Client{Timeout: time.Second}
	}
	waitForHTTP(t, fmt.Sprintf("smoke job %q did not produce a running allocation", jobID), func(_ *http.Client) bool {
		endpoint := fmt.Sprintf("%s/v1/job/%s/allocations", address, jobID)
		if namespace != "" {
			endpoint += "?namespace=" + url.QueryEscape(namespace)
		}
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return false
		}
		if token != "" {
			req.Header.Set("X-Nomad-Token", token)
		}
		resp, err := requestClient.Do(req)
		if err != nil {
			return false
		}
		var allocations []struct {
			ClientStatus string
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&allocations)
		resp.Body.Close()
		return decodeErr == nil && len(allocations) > 0 && allocations[0].ClientStatus == "running"
	})
}

func runFixtureJob(t *testing.T, binary, address, token, namespace, fixture string) {
	t.Helper()
	jobFile, err := filepath.Abs(fixture)
	if err != nil {
		t.Fatalf("resolve smoke job path: %v", err)
	}
	args := []string{"job", "run", "-address=" + address, "-detach"}
	if token != "" {
		args = append(args, "-token="+token)
	}
	if namespace != "" {
		args = append(args, "-namespace="+namespace)
	}
	args = append(args, jobFile)
	command := exec.Command(binary, args...)
	runCommand(t, command, "register browser smoke job from "+fixture)
}

func runCommand(t *testing.T, command *exec.Cmd, failure string) {
	t.Helper()
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		t.Fatalf("%s: %v", failure, err)
	}
}

func waitForHTTP(t *testing.T, failure string, ready func(*http.Client) bool) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if ready(client) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal(failure)
}

type aclBootstrapResult struct {
	SecretID string
}

type testNode struct {
	ID string
}

type testAllocation struct {
	ID            string
	ClientStatus  string
	DesiredStatus string
}

type testDeployment struct {
	ID         string
	JobID      string
	JobVersion uint64
	Status     string
	TaskGroups map[string]struct {
		DesiredCanaries int
		HealthyAllocs   int
		Promoted        bool
	}
}

// TestACLMutations exercises every state-changing control exposed by the 0.1 UI
// against a disposable ACL-enabled cluster. It deliberately checks anonymous
// denials before using a management token for the corresponding success paths.
func TestACLMutations(t *testing.T) {
	requireReleaseE2E(t, "ACL")

	binary := buildTestBinary(t)
	address := "http://127.0.0.1:4646"
	startTestAgent(t, binary, "-dev", "-acl-enabled", "-bind=127.0.0.1", "-data-dir="+filepath.Join(t.TempDir(), "acl-cluster"), "-log-level=WARN")
	waitForCluster(t, address)

	bootstrap := exec.Command(binary, "acl", "bootstrap", "-json")
	bootstrap.Env = append(os.Environ(), "WANDER_ADDR="+address)
	output, err := bootstrap.CombinedOutput()
	if err != nil {
		t.Fatalf("bootstrap disposable ACL cluster: %v\n%s", err, output)
	}
	var root aclBootstrapResult
	if err := json.Unmarshal(output, &root); err != nil || root.SecretID == "" {
		t.Fatalf("decode ACL bootstrap response: %v\n%s", err, output)
	}

	registerSmokeJob(t, binary, address, root.SecretID, nil)
	apiConfig := api.DefaultConfig()
	apiConfig.Address = address
	apiConfig.SecretID = root.SecretID
	rootClient, err := api.NewClient(apiConfig)
	if err != nil {
		t.Fatalf("create root API client: %v", err)
	}
	registerParameterizedFixture(t, binary, address, root.SecretID, "default")
	if _, err := rootClient.Namespaces().Register(&api.Namespace{Name: namespaceSmokeName}, nil); err != nil {
		t.Fatalf("create ACL test namespace: %v", err)
	}
	registerFixtureJob(t, binary, address, root.SecretID, nil, namespaceSmokeName, namespaceSmokeJobID, "fixtures/namespace-smoke.nomad.hcl")
	policyName := "wander-ui-team-read"
	if _, err := rootClient.ACLPolicies().Upsert(&api.ACLPolicy{
		Name:  policyName,
		Rules: `namespace "team-a" { policy = "read" }`,
	}, nil); err != nil {
		t.Fatalf("create namespace-restricted policy: %v", err)
	}
	restrictedToken, _, err := rootClient.ACLTokens().Create(&api.ACLToken{
		Name:     "Wander UI team reader",
		Type:     "client",
		Policies: []string{policyName},
	}, nil)
	if err != nil {
		t.Fatalf("create namespace-restricted token: %v", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	status, body := request(t, client, http.MethodGet, address+"/ui/jobs?namespace="+namespaceSmokeName, restrictedToken.SecretID, "", "")
	requireResponseContains(t, status, body, "restricted namespace jobs", namespaceSmokeJobID)
	if strings.Contains(string(body), smokeJobID) {
		t.Fatalf("restricted namespace response leaked default job %q", smokeJobID)
	}
	assertRequestStatus(t, client, http.MethodPost, address+"/api/namespace/select?ns="+namespaceSmokeName, restrictedToken.SecretID, "", "", http.StatusOK)
	assertRequestStatus(t, client, http.MethodGet, address+"/ui/jobs?namespace=default", restrictedToken.SecretID, "", "", http.StatusForbidden)

	var nodes []testNode
	getJSON(t, client, address+"/v1/nodes", root.SecretID, &nodes)
	if len(nodes) == 0 {
		t.Fatal("ACL test cluster did not register a client node")
	}
	nodeID := nodes[0].ID
	allocationID := waitForRunningAllocation(t, client, address, root.SecretID, smokeJobID)
	monitorURL := address + "/ui/clients/" + nodeID + "/monitor"
	assertRequestStatus(t, client, http.MethodGet, monitorURL, restrictedToken.SecretID, "", "", http.StatusForbidden)
	assertRequestStatus(t, client, http.MethodGet, monitorURL+"/stream", restrictedToken.SecretID, "", "", http.StatusForbidden)
	assertRequestStatus(t, client, http.MethodGet, monitorURL, root.SecretID, "", "", http.StatusOK)
	assertMonitorStream(t, client, monitorURL+"/stream?level=warn", root.SecretID)
	var members api.ServerMembers
	getJSON(t, client, address+"/v1/agent/members", root.SecretID, &members)
	if len(members.Members) == 0 {
		t.Fatal("ACL test cluster did not register a server member")
	}
	serverMonitorBase := address + "/ui/servers/" + url.PathEscape(members.Members[0].Name) + "/monitor"
	serverRegion := url.QueryEscape(members.Members[0].Tags["region"])
	serverMonitorPage := serverMonitorBase + "?region=" + serverRegion
	serverMonitorStream := serverMonitorBase + "/stream?level=warn&region=" + serverRegion
	assertRequestStatus(t, client, http.MethodGet, serverMonitorPage, restrictedToken.SecretID, "", "", http.StatusForbidden)
	assertRequestStatus(t, client, http.MethodGet, serverMonitorPage, root.SecretID, "", "", http.StatusOK)
	assertRequestStatus(t, client, http.MethodGet, serverMonitorStream, restrictedToken.SecretID, "", "", http.StatusForbidden)
	assertMonitorStream(t, client, serverMonitorStream, root.SecretID)

	assertRequestStatus(t, client, http.MethodPost, address+"/ui/clients/"+nodeID+"/drain", "", "application/x-www-form-urlencoded", "deadline=5m", http.StatusForbidden)
	assertRequestStatus(t, client, http.MethodPost, address+"/ui/allocations/"+allocationID+"/restart", "", "", "", http.StatusForbidden)
	assertRequestStatus(t, client, http.MethodPost, address+"/ui/variables", "", "application/x-www-form-urlencoded", "path=acl-audit/test&namespace=default&key_0=answer&value_0=denied", http.StatusForbidden)
	dispatchForm := url.Values{"meta.environment": {"acl-test"}, "payload": {"acl-dispatch-payload"}}
	assertRequestStatus(t, client, http.MethodPost, address+"/api/jobs/"+parameterizedSmokeJobID+"/dispatch", "", "application/x-www-form-urlencoded", dispatchForm.Encode(), http.StatusForbidden)
	status, body = request(t, client, http.MethodPost, address+"/api/jobs/"+parameterizedSmokeJobID+"/dispatch", root.SecretID, "application/x-www-form-urlencoded", dispatchForm.Encode())
	if status != http.StatusOK {
		t.Fatalf("dispatch with management token returned %d: %s", status, body)
	}
	var dispatchResponse struct {
		DispatchedJobID string `json:"dispatchedJobId"`
	}
	if err := json.Unmarshal(body, &dispatchResponse); err != nil || dispatchResponse.DispatchedJobID == "" {
		t.Fatalf("decode dispatch response: %v; body: %s", err, body)
	}
	assertRequestStatus(t, client, http.MethodPost, address+"/api/jobs/"+dispatchResponse.DispatchedJobID+"/purge", "", "", "", http.StatusForbidden)
	assertRequestStatus(t, client, http.MethodPost, address+"/api/jobs/"+dispatchResponse.DispatchedJobID+"/purge", root.SecretID, "", "", http.StatusOK)

	assertACLMutation(t, client, http.MethodPost, address+"/api/clients/"+nodeID+"/eligibility", root.SecretID, "application/json", `{"Eligible":false}`)
	nodeActions := []struct {
		endpoint    string
		contentType string
		body        string
	}{
		{endpoint: address + "/api/clients/" + nodeID + "/eligibility", contentType: "application/json", body: `{"Eligible":true}`},
		{endpoint: address + "/ui/clients/" + nodeID + "/drain", contentType: "application/x-www-form-urlencoded", body: "deadline=5m&ignore_system=true"},
		{endpoint: address + "/ui/clients/" + nodeID + "/drain/cancel"},
	}
	for _, action := range nodeActions {
		assertRequestStatus(t, client, http.MethodPost, action.endpoint, root.SecretID, action.contentType, action.body, http.StatusOK)
	}

	allocationID = waitForRunningAllocation(t, client, address, root.SecretID, smokeJobID)
	assertRequestStatus(t, client, http.MethodPost, address+"/ui/allocations/"+allocationID+"/restart", root.SecretID, "", "", http.StatusOK)
	assertRequestStatus(t, client, http.MethodPost, address+"/ui/allocations/"+allocationID+"/stop", root.SecretID, "", "", http.StatusOK)

	variableForm := url.Values{
		"path":      {"acl-audit/test"},
		"namespace": {"default"},
		"key_0":     {"answer"},
		"value_0":   {"forty-two"},
		"key_1":     {"spacing"},
		"value_1":   {" preserved "},
	}
	assertRequestStatus(t, client, http.MethodPost, address+"/ui/variables", root.SecretID, "application/x-www-form-urlencoded", variableForm.Encode(), http.StatusCreated)
	var variable struct {
		Items map[string]string
	}
	getJSON(t, client, address+"/v1/var/acl-audit/test?namespace=default", root.SecretID, &variable)
	if variable.Items["answer"] != "forty-two" || variable.Items["spacing"] != " preserved " {
		t.Fatalf("variable create lost form items: %#v", variable.Items)
	}
	updateForm := url.Values{"namespace": {"default"}, "key_0": {"answer"}, "value_0": {"updated"}}
	assertRequestStatus(t, client, http.MethodPost, address+"/ui/variables/acl-audit/test?namespace=default", root.SecretID, "application/x-www-form-urlencoded", updateForm.Encode(), http.StatusOK)
	variable.Items = nil
	getJSON(t, client, address+"/v1/var/acl-audit/test?namespace=default", root.SecretID, &variable)
	if len(variable.Items) != 1 || variable.Items["answer"] != "updated" {
		t.Fatalf("variable update stored unexpected items: %#v", variable.Items)
	}
	assertRequestStatus(t, client, http.MethodDelete, address+"/ui/variables/acl-audit/test?namespace=default", root.SecretID, "", "", http.StatusOK)

	policyCreateForm := url.Values{
		"name":        {"wander-ui-audit"},
		"description": {"temporary UI audit policy"},
		"rules":       {`namespace "default" { policy = "read" }`},
	}
	assertACLMutation(t, client, http.MethodPost, address+"/ui/policies/new", root.SecretID, "application/x-www-form-urlencoded", policyCreateForm.Encode())
	policyForm := url.Values{"description": {"updated through Wander UI"}, "rules": {`namespace "default" { policy = "write" }`}}
	assertACLMutation(t, client, http.MethodPost, address+"/ui/policies/wander-ui-audit", root.SecretID, "application/x-www-form-urlencoded", policyForm.Encode())
	var policy struct {
		Description string
		Rules       string
	}
	getJSON(t, client, address+"/v1/acl/policy/wander-ui-audit", root.SecretID, &policy)
	if policy.Description != "updated through Wander UI" || !strings.Contains(policy.Rules, `policy = "write"`) {
		t.Fatalf("policy update was not persisted: %#v", policy)
	}

	roleCreateForm := url.Values{
		"name":        {"wander-ui-audit-role"},
		"description": {"temporary UI audit role"},
		"policies":    {"wander-ui-audit"},
	}
	assertACLMutation(t, client, http.MethodPost, address+"/ui/roles/new", root.SecretID, "application/x-www-form-urlencoded", roleCreateForm.Encode())
	roles, _, err := rootClient.ACLRoles().List(nil)
	if err != nil {
		t.Fatalf("list roles after UI creation: %v", err)
	}
	var auditRole *api.ACLRoleListStub
	for _, role := range roles {
		if role.Name == "wander-ui-audit-role" {
			auditRole = role
			break
		}
	}
	if auditRole == nil || len(auditRole.Policies) != 1 || auditRole.Policies[0].Name != "wander-ui-audit" {
		t.Fatalf("role created through UI has unexpected policy bindings: %#v", auditRole)
	}
	status, body = request(t, client, http.MethodGet, address+"/ui/roles", root.SecretID, "", "")
	requireResponseContains(t, status, body, "ACL role list", "wander-ui-audit-role")
	roleUpdateForm := url.Values{
		"name":        {"wander-ui-audit-role"},
		"description": {"updated through Wander UI"},
		"policies":    {"wander-ui-audit"},
	}
	assertACLMutation(t, client, http.MethodPost, address+"/ui/roles/"+auditRole.ID, root.SecretID, "application/x-www-form-urlencoded", roleUpdateForm.Encode())
	updatedRole, _, err := rootClient.ACLRoles().Get(auditRole.ID, nil)
	if err != nil || updatedRole.Description != "updated through Wander UI" {
		t.Fatalf("role update through UI was not persisted: role=%#v err=%v", updatedRole, err)
	}

	tokenCreateForm := url.Values{
		"name":     {"wander-ui-audit-token"},
		"type":     {"client"},
		"policies": {"wander-ui-audit"},
		"roles":    {auditRole.ID},
	}
	assertRequestStatus(t, client, http.MethodPost, address+"/ui/tokens/new", "", "application/x-www-form-urlencoded", tokenCreateForm.Encode(), http.StatusForbidden)
	status, body = request(t, client, http.MethodPost, address+"/ui/tokens/new", root.SecretID, "application/x-www-form-urlencoded", tokenCreateForm.Encode())
	if status != http.StatusOK {
		t.Fatalf("create ACL token through UI returned %d: %s", status, body)
	}
	tokenCreateBody := append([]byte(nil), body...)
	tokens, _, err := rootClient.ACLTokens().List(nil)
	if err != nil {
		t.Fatalf("list tokens after UI creation: %v", err)
	}
	var auditToken *api.ACLTokenListStub
	for _, token := range tokens {
		if token.Name == "wander-ui-audit-token" {
			auditToken = token
			break
		}
	}
	if auditToken == nil {
		t.Fatal("token created through UI was not returned by the API")
	}
	status, body = request(t, client, http.MethodGet, address+"/ui/tokens", root.SecretID, "", "")
	requireResponseContains(t, status, body, "ACL token list", "wander-ui-audit-token", auditToken.AccessorID)
	createdToken, _, err := rootClient.ACLTokens().Info(auditToken.AccessorID, nil)
	if err != nil {
		t.Fatalf("read token after UI creation: %v", err)
	}
	if createdToken.SecretID == "" || !bytes.Contains(tokenCreateBody, []byte(createdToken.SecretID)) {
		t.Fatal("token create response did not display the one-time secret")
	}
	status, body = request(t, client, http.MethodGet, address+"/ui/policies/wander-ui-audit", root.SecretID, "", "")
	requireResponseContains(t, status, body, "ACL policy relationships", "wander-ui-audit-token", "wander-ui-audit-role", "role wander-ui-audit-role")
	status, body = request(t, client, http.MethodGet, address+"/ui/tokens/"+auditToken.AccessorID, root.SecretID, "", "")
	requireResponseContains(t, status, body, "ACL token detail", "wander-ui-audit-token")
	if bytes.Contains(body, []byte(createdToken.SecretID)) {
		t.Fatal("token detail response leaked the token secret")
	}
	tokenUpdateForm := url.Values{
		"name":     {"wander-ui-audit-token-updated"},
		"type":     {"client"},
		"policies": {"wander-ui-audit"},
		"roles":    {auditRole.ID},
	}
	assertACLMutation(t, client, http.MethodPost, address+"/ui/tokens/"+auditToken.AccessorID, root.SecretID, "application/x-www-form-urlencoded", tokenUpdateForm.Encode())
	createdToken, _, err = rootClient.ACLTokens().Info(auditToken.AccessorID, nil)
	if err != nil || createdToken.Name != "wander-ui-audit-token-updated" || len(createdToken.Roles) != 1 {
		t.Fatalf("token update through UI was not persisted: token=%#v err=%v", createdToken, err)
	}
	assertACLMutation(t, client, http.MethodDelete, address+"/ui/tokens/"+auditToken.AccessorID, root.SecretID, "", "")
	assertACLMutation(t, client, http.MethodDelete, address+"/ui/roles/"+auditRole.ID, root.SecretID, "", "")
	assertACLMutation(t, client, http.MethodDelete, address+"/ui/policies/wander-ui-audit", root.SecretID, "", "")

	deploymentOne := writeDeploymentFixture(t, 1)
	runJobFile(t, binary, address, root.SecretID, deploymentOne)
	_ = waitForRunningAllocation(t, client, address, root.SecretID, "wander-ui-deployment-audit")
	promoteID := startDeploymentUpdate(t, client, binary, address, root.SecretID, 2, 1)

	status, body = request(t, client, http.MethodGet, address+"/ui/deployments/"+promoteID, root.SecretID, "", "")
	requireResponseContains(t, status, body, "deployment controls", "Promote canaries", "Fail deployment")
	assertACLMutation(t, client, http.MethodPost, address+"/api/deployments/"+promoteID+"/promote?namespace=default", root.SecretID, "", "")
	_ = waitForDeployment(t, client, address, root.SecretID, 1, "successful")

	failID := startDeploymentUpdate(t, client, binary, address, root.SecretID, 3, 2)
	assertACLMutation(t, client, http.MethodPost, address+"/api/deployments/"+failID+"/fail?namespace=default", root.SecretID, "", "")
}

// TestTLSTransports proves that allocation logs and exec work through the
// production HTTPS/WSS transport boundary with a real running task.
func TestTLSTransports(t *testing.T) {
	requireReleaseE2E(t, "TLS")

	repositoryRoot := testRepositoryRoot(t)
	caFile := filepath.Join(repositoryRoot, "helper/tlsutil/testdata/nomad-agent-ca.pem")
	certFile := filepath.Join(repositoryRoot, "helper/tlsutil/testdata/global-server-nomad.pem")
	keyFile := filepath.Join(repositoryRoot, "helper/tlsutil/testdata/global-server-nomad-key.pem")
	configPath := filepath.Join(t.TempDir(), "tls.hcl")
	config := fmt.Sprintf(`tls {
  http = true
  rpc = false
  verify_https_client = false
  ca_file = %q
  cert_file = %q
  key_file = %q
}
`, caFile, certFile, keyFile)
	writeTestFile(t, configPath, config)

	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		t.Fatalf("read TLS test CA: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("parse TLS test CA")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}

	address := "https://127.0.0.1:4646"
	t.Setenv("WANDER_ADDR", address)
	t.Setenv("WANDER_CACERT", caFile)
	binary := buildTestBinary(t)
	startTestAgent(t, binary, "-dev", "-bind=127.0.0.1", "-config="+configPath, "-data-dir="+filepath.Join(t.TempDir(), "tls-cluster"), "-log-level=WARN")
	waitForClusterWithClient(t, address, client)

	registerSmokeJob(t, binary, address, "", client)
	allocationID := waitForRunningAllocation(t, client, address, "", smokeJobID)
	status, logs := request(t, client, http.MethodGet, address+"/ui/allocations/"+allocationID+"/logs/stream?task=fixture&type=stdout&follow=false&origin=start", "", "", "")
	requireResponseContains(t, status, logs, "HTTPS log stream", "event: log", "wander-ui-smoke")

	dialer := websocket.Dialer{TLSClientConfig: tlsConfig}
	wsURL := "wss://127.0.0.1:4646/ui/allocations/" + allocationID + "/exec/ws?task=fixture&command=%2Fbin%2Fsh"
	connection, response, err := dialer.Dial(wsURL, nil)
	if err != nil {
		if response != nil {
			response.Body.Close()
		}
		t.Fatalf("connect to WSS exec endpoint: %v", err)
	}
	defer connection.Close()
	if err := connection.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		t.Fatalf("set WSS exec deadline: %v", err)
	}
	if err := connection.WriteJSON(map[string]string{"type": "input", "data": "echo wander-secure-exec; exit\n"}); err != nil {
		t.Fatalf("write WSS exec input: %v", err)
	}

	var execOutput strings.Builder
	for {
		var message struct {
			Type string `json:"type"`
			Data string `json:"data"`
			Code int    `json:"code"`
		}
		if err := connection.ReadJSON(&message); err != nil {
			t.Fatalf("read WSS exec response: %v (output %q)", err, execOutput.String())
		}
		execOutput.WriteString(message.Data)
		if message.Type == "exit" {
			if message.Code != 0 || !strings.Contains(execOutput.String(), "wander-secure-exec") {
				t.Fatalf("unexpected WSS exec result: code=%d output=%q", message.Code, execOutput.String())
			}
			break
		}
	}
}

func request(t *testing.T, client *http.Client, method, endpoint, token, contentType, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("create %s %s request: %v", method, endpoint, err)
	}
	if token != "" {
		req.Header.Set("X-Nomad-Token", token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send %s %s request: %v", method, endpoint, err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s %s response: %v", method, endpoint, err)
	}
	return resp.StatusCode, payload
}

func assertRequestStatus(t *testing.T, client *http.Client, method, endpoint, token, contentType, body string, expected int) {
	t.Helper()
	status, payload := request(t, client, method, endpoint, token, contentType, body)
	if status != expected {
		t.Fatalf("%s %s returned %d, want %d: %s", method, endpoint, status, expected, payload)
	}
}

func assertMonitorStream(t *testing.T, client *http.Client, endpoint, token string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		cancel()
		t.Fatalf("create monitor stream request: %v", err)
	}
	req.Header.Set("X-Nomad-Token", token)
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("open monitor stream: %v", err)
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		cancel()
		_ = resp.Body.Close()
		t.Fatalf("monitor stream returned %d with content type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	cancel()
	_ = resp.Body.Close()
}

func assertACLMutation(t *testing.T, client *http.Client, method, endpoint, token, contentType, body string) {
	t.Helper()
	assertRequestStatus(t, client, method, endpoint, "", contentType, body, http.StatusForbidden)
	assertRequestStatus(t, client, method, endpoint, token, contentType, body, http.StatusOK)
}

func requireResponseContains(t *testing.T, status int, body []byte, description string, fragments ...string) {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("%s returned %d: %s", description, status, body)
	}
	for _, fragment := range fragments {
		if !bytes.Contains(body, []byte(fragment)) {
			t.Fatalf("%s did not contain %q: %s", description, fragment, body)
		}
	}
}

func getJSON(t *testing.T, client *http.Client, endpoint, token string, target interface{}) {
	t.Helper()
	status, payload := request(t, client, http.MethodGet, endpoint, token, "", "")
	if status != http.StatusOK {
		t.Fatalf("GET %s returned %d: %s", endpoint, status, payload)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		t.Fatalf("decode GET %s response: %v\n%s", endpoint, err, payload)
	}
}

func waitForRunningAllocation(t *testing.T, client *http.Client, address, token, jobID string) string {
	t.Helper()
	var allocationID string
	waitForHTTP(t, "job "+jobID+" did not produce a running allocation", func(_ *http.Client) bool {
		var allocations []testAllocation
		getJSON(t, client, address+"/v1/job/"+jobID+"/allocations", token, &allocations)
		for _, allocation := range allocations {
			if allocation.ClientStatus == "running" && allocation.DesiredStatus == "run" {
				allocationID = allocation.ID
				return true
			}
		}
		return false
	})
	return allocationID
}

func waitForDeployment(t *testing.T, client *http.Client, address, token string, version uint64, status string) string {
	t.Helper()
	var deploymentID string
	waitForHTTP(t, fmt.Sprintf("deployment version %d did not reach %s", version, status), func(_ *http.Client) bool {
		var deployments []testDeployment
		getJSON(t, client, address+"/v1/deployments?namespace=default", token, &deployments)
		for _, deployment := range deployments {
			if deployment.JobID == "wander-ui-deployment-audit" && deployment.JobVersion == version && deployment.Status == status && deploymentReady(deployment, status) {
				deploymentID = deployment.ID
				return true
			}
		}
		return false
	})
	return deploymentID
}

func deploymentReady(deployment testDeployment, status string) bool {
	if status != "running" {
		return true
	}
	for _, group := range deployment.TaskGroups {
		if group.DesiredCanaries > 0 && !group.Promoted {
			return group.HealthyAllocs >= group.DesiredCanaries
		}
	}
	return false
}

func startDeploymentUpdate(t *testing.T, client *http.Client, binary, address, token string, fixtureVersion int, jobVersion uint64) string {
	t.Helper()
	fixture := writeDeploymentFixture(t, fixtureVersion)
	runJobFile(t, binary, address, token, fixture)
	return waitForDeployment(t, client, address, token, jobVersion, "running")
}

func runJobFile(t *testing.T, binary, address, token, path string) {
	t.Helper()
	command := exec.Command(binary, "job", "run", "-address="+address, "-token="+token, "-detach", path)
	runCommand(t, command, "register ACL mutation fixture")
}

func writeDeploymentFixture(t *testing.T, version int) string {
	t.Helper()
	contents := fmt.Sprintf(`job "wander-ui-deployment-audit" {
  datacenters = ["dc1"]
  type = "service"
  group "web" {
    count = 1
    update {
      max_parallel = 1
      canary = 1
      min_healthy_time = "1s"
      healthy_deadline = "1m"
      progress_deadline = "2m"
      auto_promote = false
    }
    task "fixture" {
      driver = "raw_exec"
      config {
        command = "/bin/sh"
        args = ["-c", "while true; do echo deployment-v%d; sleep 5; done"]
      }
      resources {
        cpu = 50
        memory = 32
      }
    }
  }
}
`, version)
	path := filepath.Join(t.TempDir(), fmt.Sprintf("deployment-v%d.nomad.hcl", version))
	writeTestFile(t, path, contents)
	return path
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
}

// TestServerStartup verifies that the test server can start and respond to requests.
func TestServerStartup(t *testing.T) {
	// Start server with random port
	srv, cleanup := StartTestServer(TestServerConfig{
		Port:     0, // Use random available port
		LogLevel: "error",
	})
	defer cleanup()

	if srv.Port() == 0 {
		t.Fatal("Server port should not be 0")
	}

	t.Logf("Server started on port %d", srv.Port())

	// Server should be accessible via URL
	expectedURL := srv.URL()
	if expectedURL == "" {
		t.Fatal("Server URL should not be empty")
	}

	t.Logf("Server URL: %s", expectedURL)
}

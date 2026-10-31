package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/partials/serverspartials"
)

func TestServerSortOptions(t *testing.T) {
	for _, test := range []struct {
		query      string
		wantColumn string
		wantDir    string
	}{
		{wantColumn: "leader", wantDir: "desc"},
		{query: "sort=name", wantColumn: "name", wantDir: "asc"},
		{query: "sort=isLeader&desc=true", wantColumn: "leader", wantDir: "desc"},
		{query: "sort=port&dir=desc", wantColumn: "port", wantDir: "desc"},
		{query: "sort=unknown&dir=asc", wantColumn: "leader", wantDir: "desc"},
	} {
		query, err := url.ParseQuery(test.query)
		if err != nil {
			t.Fatal(err)
		}
		column, direction := serverSortOptions(query)
		if column != test.wantColumn || direction != test.wantDir {
			t.Fatalf("serverSortOptions(%q) = (%q, %q), want (%q, %q)", test.query, column, direction, test.wantColumn, test.wantDir)
		}
	}
}

func TestSortAndPaginateServers(t *testing.T) {
	servers := []serverspartials.ServerSummary{
		{Name: "server-c", Leader: false, Port: "4649"},
		{Name: "server-a", Leader: true, Port: "4647"},
		{Name: "server-b", Leader: false, Port: "4648"},
	}
	sortServers(servers, "leader", "desc")
	if servers[0].Name != "server-a" {
		t.Fatalf("leader-first sort started with %q", servers[0].Name)
	}
	sortServers(servers, "port", "asc")
	page := paginateServers(servers, 2, 2)
	if len(page) != 1 || page[0].Name != "server-c" {
		t.Fatalf("second page = %#v, want server-c", page)
	}
	if got := boundedServerPage(99, len(servers), 2); got != 2 {
		t.Fatalf("boundedServerPage = %d, want 2", got)
	}
}

func TestServerAddressAndSortedTags(t *testing.T) {
	member := &api.AgentMember{Addr: "2001:db8::1", Port: 4648, Tags: map[string]string{"port": "4647", "z": "last", "a": "first"}}
	if got := serverRPCAddress(member); got != "[2001:db8::1]:4647" {
		t.Fatalf("serverRPCAddress = %q", got)
	}
	tags := sortedServerTags(member.Tags)
	if len(tags) != 3 || tags[0].Name != "a" || tags[2].Name != "z" {
		t.Fatalf("sorted tags = %#v", tags)
	}
}

func TestServerListURLPreservesRegion(t *testing.T) {
	if got := serverListURL("region with spaces"); got != "/ui/servers?region=region+with+spaces" {
		t.Fatalf("serverListURL = %q", got)
	}
}

func TestServersListAndDetailInheritedContract(t *testing.T) {
	handler := newServersTestHandler(t, `{"ServerRegion":"global","ServerDC":"dc1","Members":[{"Name":"server-1","Addr":"127.0.0.1","Port":4648,"Status":"alive","Tags":{"build":"1.6.5","dc":"dc1","port":"4647","region":"global"}}]}`)
	render := func(target string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		return response
	}

	listResponse := render("/ui/servers")
	for _, expected := range []string{"Port", "Version", "4648", "1.6.5", "True"} {
		assertHTTPResponse(t, listResponse, http.StatusOK, expected)
	}

	detailResponse := render("/ui/servers/server-1")
	assertHTTPResponse(t, detailResponse, http.StatusOK, "127.0.0.1:4647")
	body := detailResponse.Body.String()
	buildIndex := strings.Index(body, ">build<")
	regionIndex := strings.Index(body, ">region<")
	if buildIndex < 0 || regionIndex < 0 || buildIndex > regionIndex || !strings.Contains(body, "data-copy-button") {
		t.Fatalf("server detail did not render sorted copyable tags: %s", body)
	}
}

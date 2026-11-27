// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

// -------------------------------------------------------------------------------
// Consul Integration Tests
//
// Project: Wander
//
// Wander registers its agents and its jobs' services in Consul, removes a
// job's services when the job stops, and renders Consul KV and catalog data
// into task templates. Each test reads Consul's API to check the result.
// -------------------------------------------------------------------------------

//go:build integration

package integration

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/shoenig/test/must"

	"github.com/open-wander/wander/api"
)

const (
	consulTimeout = time.Minute
	templateKey   = "wander-it/message"
)

// TestConsul_AgentsRegistered proves each Wander agent registers itself in
// Consul with passing checks. nomad and nomad-client are Wander's default
// service names, kept so 1.6.5 clients still find upgraded servers.
func TestConsul_AgentsRegistered(t *testing.T) {
	cases := []struct {
		name, service, tag string
	}{
		{"server_http", "nomad", "http"},
		{"server_rpc", "nomad", "rpc"},
		{"server_serf", "nomad", "serf"},
		{"clients", "nomad-client", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			passingInstances(t, tc.service, tc.tag, 3)
		})
	}
}

// TestConsul_JobService proves a job's service registers one passing instance
// on each client, with the client's name interpolated into its meta, and
// leaves the Consul catalog when the job stops.
func TestConsul_JobService(t *testing.T) {
	client := servers["wander-server-1"]
	jobID := runJob(t, client, "consul-service")
	runningAllocs(t, client, jobID, 3)

	nodes := make(map[string]bool)
	for _, e := range passingInstances(t, "it-svc", "wander-it", 3) {
		must.Eq(t, e.Node.Node, e.Service.Meta["node"], must.Sprint("meta.node names the instance's own node"))
		nodes[e.Node.Node] = true
	}
	must.MapLen(t, 3, nodes, must.Sprint("one instance per client"))

	_, _, err := client.Jobs().Deregister(jobID, false, nil)
	must.NoError(t, err)

	// The catalog ignores health, so a failing instance still counts here.
	eventually(t, consulTimeout, func() error {
		left, _, err := consulClient.Catalog().Service("it-svc", "", nil)
		if err != nil {
			return err
		}
		if len(left) != 0 {
			return fmt.Errorf("%d it-svc instances still in the catalog", len(left))
		}
		return nil
	})
}

// TestConsul_Template proves a task template renders a Consul KV key and the
// consul service's instances exactly as Consul's API reports them, and renders
// the file again when the key changes.
func TestConsul_Template(t *testing.T) {
	client := servers["wander-server-1"]
	kv := consulClient.KV()

	_, err := kv.Put(&consulapi.KVPair{Key: templateKey, Value: []byte("hello-1")}, nil)
	must.NoError(t, err)
	t.Cleanup(func() {
		if _, err := kv.Delete(templateKey, nil); err != nil {
			t.Logf("delete %s: %v", templateKey, err)
		}
	})

	jobID := runJob(t, client, "consul-template")
	allocID := runningAllocs(t, client, jobID, 1)[0].ID
	want := consulAddrs(t)

	eventually(t, consulTimeout, func() error {
		return checkRendered(client, allocID, "message=hello-1", want)
	})

	_, err = kv.Put(&consulapi.KVPair{Key: templateKey, Value: []byte("hello-2")}, nil)
	must.NoError(t, err)
	eventually(t, consulTimeout, func() error {
		return checkRendered(client, allocID, "message=hello-2", want)
	})
}

// -------------------------------------------------------------------------
// HELPERS
// -------------------------------------------------------------------------

// passingInstances waits until Consul reports want passing instances of
// service, filtered by tag when tag is set, and returns them.
func passingInstances(t *testing.T, service, tag string, want int) []*consulapi.ServiceEntry {
	t.Helper()
	var entries []*consulapi.ServiceEntry
	eventually(t, consulTimeout, func() error {
		list, _, err := consulClient.Health().Service(service, tag, true, nil)
		if err != nil {
			return err
		}
		if len(list) != want {
			return fmt.Errorf("%d passing %s instances, want %d", len(list), service, want)
		}
		entries = list
		return nil
	})
	return entries
}

// consulAddrs returns the sorted address:port of each passing consul service
// instance, taken from Consul's health API.
func consulAddrs(t *testing.T) []string {
	t.Helper()
	var addrs []string
	for _, e := range passingInstances(t, "consul", "", 3) {
		// The template's .Address falls back to the node's when the service has none.
		addr := e.Service.Address
		if addr == "" {
			addr = e.Node.Address
		}
		addrs = append(addrs, fmt.Sprintf("%s:%d", addr, e.Service.Port))
	}
	slices.Sort(addrs)
	return addrs
}

// checkRendered reads the template job's out.txt and checks it holds the
// message line and exactly the consul= addresses in want, in any order.
func checkRendered(client *api.Client, allocID, message string, want []string) error {
	out, err := allocFile(client, allocID, "sleep/local/out.txt")
	if err != nil {
		return err
	}

	var got []string
	hasMessage := false
	for line := range strings.SplitSeq(out, "\n") {
		if line == message {
			hasMessage = true
		}
		if addr, ok := strings.CutPrefix(line, "consul="); ok {
			got = append(got, addr)
		}
	}
	if !hasMessage {
		return fmt.Errorf("no %q line in out.txt:\n%s", message, out)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		return fmt.Errorf("out.txt lists consul at %v, Consul's API reports %v", got, want)
	}
	return nil
}

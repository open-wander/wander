// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

// Node events carry static host volumes only, for every subscriber, because
// the broker shares one payload and cannot filter per token.
func TestEventsFromChanges_NodeStripsDynamicHostVolumes(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	node := mock.Node()
	node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
		"static-a":  {Name: "static-a", Path: "/synthetic/static-a"},
		"dynamic-a": {Name: "dynamic-a", Path: "/synthetic/dynamic-a", ID: uuid.Generate()},
	}

	must.NoError(t, s.UpsertNode(structs.NodeRegisterRequestType, 100, node))
	events := WaitForEvents(t, s, 100, 1, time.Second)
	must.Len(t, 1, events)
	payload := events[0].Payload.(*structs.NodeStreamEvent)
	must.MapLen(t, 1, payload.Node.HostVolumes)
	must.MapContainsKey(t, payload.Node.HostVolumes, "static-a")

	// A deregistration carries the node as it was before the delete.
	deleted := eventsFromChanges(nil, Changes{
		Index:   110,
		Changes: memdb.Changes{{Table: "nodes", Before: node, After: nil}},
		MsgType: structs.NodeDeregisterRequestType,
	})
	must.NotNil(t, deleted)
	must.Len(t, 1, deleted.Events)
	payload = deleted.Events[0].Payload.(*structs.NodeStreamEvent)
	must.MapLen(t, 1, payload.Node.HostVolumes)
	must.MapContainsKey(t, payload.Node.HostVolumes, "static-a")
	must.MapLen(t, 2, node.HostVolumes)
}

func TestStripNodeEventDynamicHostVolumes(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil node", func(t *testing.T) {
		must.Nil(t, stripNodeEventDynamicHostVolumes(nil))
	})

	t.Run("static only keeps the node", func(t *testing.T) {
		node := mock.Node()
		node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
			"static-a": {Name: "static-a", Path: "/synthetic/static-a"},
		}
		must.EqOp(t, node, stripNodeEventDynamicHostVolumes(node))
	})

	t.Run("dynamic entries removed without touching the input", func(t *testing.T) {
		node := mock.Node()
		node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
			"static-a":  {Name: "static-a", Path: "/synthetic/static-a"},
			"dynamic-a": {Name: "dynamic-a", Path: "/synthetic/dynamic-a", ID: uuid.Generate()},
		}
		out := stripNodeEventDynamicHostVolumes(node)
		must.MapLen(t, 1, out.HostVolumes)
		must.MapContainsKey(t, out.HostVolumes, "static-a")
		must.MapLen(t, 2, node.HostVolumes)
	})

	t.Run("only dynamic leaves an empty map", func(t *testing.T) {
		node := mock.Node()
		node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
			"dynamic-a": {Name: "dynamic-a", Path: "/synthetic/dynamic-a", ID: uuid.Generate()},
		}
		out := stripNodeEventDynamicHostVolumes(node)
		must.NotNil(t, out.HostVolumes)
		must.MapEmpty(t, out.HostVolumes)
	})
}

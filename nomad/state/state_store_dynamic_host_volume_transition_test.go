// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-wander/wander/nomad/structs"
	"github.com/stretchr/testify/require"
)

const transitionNode = "00000000-0000-0000-0000-00000000000a"

// transitionVolume builds a synthetic volume that is waiting for a node.
func transitionVolume(id string) *structs.DynamicHostVolume {
	return &structs.DynamicHostVolume{
		ID: id, Name: "data-" + id, Namespace: "default", Type: structs.VolumeTypeHost,
		PluginID: "plugin-a", State: structs.DynamicHostVolumeStateRequested,
		Origin:                    structs.DynamicHostVolumeOriginPluginCreated,
		RequestedCapacityMinBytes: 1, RequestedCapacityMaxBytes: 100,
		RequestedCapabilities: []*structs.DynamicHostVolumeCapability{{
			AttachmentMode: structs.DynamicHostVolumeAttachmentModeFilesystem,
			AccessMode:     structs.DynamicHostVolumeAccessModeSingleNodeWriter}},
		Parameters: map[string]string{"synthetic-key": "synthetic-value"},
		CreateTime: 100, ModifyTime: 100,
	}
}

// transitionPending is the volume of transitionVolume once a node is known.
func transitionPending(id string) *structs.DynamicHostVolume {
	v := transitionVolume(id)
	v.NodeID, v.State = transitionNode, structs.DynamicHostVolumeStatePending
	return v
}

func outcomeReady(id string) *structs.DynamicHostVolumeOutcomeRequest {
	return &structs.DynamicHostVolumeOutcomeRequest{
		Key: structs.DynamicHostVolumeKey{Namespace: "default", ID: id}, State: structs.DynamicHostVolumeStateReady,
		HostPath: "/srv/wander-synthetic/" + id, CapacityBytes: 50, ModifyTime: 200}
}

func outcomeFailed(id, diagnostic string) *structs.DynamicHostVolumeOutcomeRequest {
	return &structs.DynamicHostVolumeOutcomeRequest{
		Key: structs.DynamicHostVolumeKey{Namespace: "default", ID: id}, State: structs.DynamicHostVolumeStateFailed,
		Diagnostic: diagnostic, ModifyTime: 200}
}

func transitionKey(id string) structs.DynamicHostVolumeKey {
	return structs.DynamicHostVolumeKey{Namespace: "default", ID: id}
}

// transitionSeed drives a fresh volume into the wanted state through the real
// commands (unavailable has no command yet, so it is written as the later
// node-derived task will write it). It returns the next free Raft index.
func transitionSeed(t *testing.T, store *StateStore, id string, want structs.DynamicHostVolumeState) uint64 {
	t.Helper()
	index := uint64(10)
	next := func() uint64 { index++; return index }
	switch want {
	case structs.DynamicHostVolumeStateRequested:
		require.NoError(t, store.CreateDynamicHostVolumes(next(), []*structs.DynamicHostVolume{transitionVolume(id)}))
	case structs.DynamicHostVolumeStatePending:
		require.NoError(t, store.CreateDynamicHostVolumes(next(), []*structs.DynamicHostVolume{transitionPending(id)}))
	case structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeStateUnavailable:
		require.NoError(t, store.CreateDynamicHostVolumes(next(), []*structs.DynamicHostVolume{transitionPending(id)}))
		require.NoError(t, store.RecordDynamicHostVolumeOutcome(next(), outcomeReady(id)))
		if want == structs.DynamicHostVolumeStateUnavailable {
			cur := transitionGet(t, store, id)
			cur.State = structs.DynamicHostVolumeStateUnavailable
			require.NoError(t, store.UpsertDynamicHostVolumes(next(), []*structs.DynamicHostVolume{cur}))
		}
	case structs.DynamicHostVolumeStateFailed:
		require.NoError(t, store.CreateDynamicHostVolumes(next(), []*structs.DynamicHostVolume{transitionVolume(id)}))
		require.NoError(t, store.RecordDynamicHostVolumeOutcome(next(), outcomeFailed(id, "no node")))
	case structs.DynamicHostVolumeStateDeleting:
		require.NoError(t, store.CreateDynamicHostVolumes(next(), []*structs.DynamicHostVolume{transitionPending(id)}))
		require.NoError(t, store.MarkDynamicHostVolumesDeleting(next(), []structs.DynamicHostVolumeKey{transitionKey(id)}, 300))
	}
	require.Equal(t, want, transitionGet(t, store, id).State)
	return index
}

func transitionGet(t *testing.T, store *StateStore, id string) *structs.DynamicHostVolume {
	t.Helper()
	v, err := store.DynamicHostVolumeByID(nil, "default", id)
	require.NoError(t, err)
	require.NotNil(t, v)
	return v
}

func TestStateStore_DynamicHostVolumeTransitionTable(t *testing.T) {
	all := []structs.DynamicHostVolumeState{
		structs.DynamicHostVolumeStateUnknown, structs.DynamicHostVolumeStateRequested, structs.DynamicHostVolumeStatePending,
		structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeStateUnavailable,
		structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeStateFailed}
	allowed := map[[2]structs.DynamicHostVolumeState]bool{}
	for _, pair := range [][2]structs.DynamicHostVolumeState{
		{"", "requested"}, {"", "pending"},
		{"requested", "pending"}, {"requested", "failed"}, {"requested", "deleting"},
		{"pending", "ready"}, {"pending", "failed"}, {"pending", "deleting"},
		{"ready", "ready"}, {"ready", "unavailable"}, {"ready", "deleting"},
		{"unavailable", "ready"}, {"unavailable", "deleting"},
		{"failed", "deleting"},
		{"deleting", "deleting"},
	} {
		allowed[pair] = true
	}
	for _, from := range all {
		for _, to := range all {
			t.Run(fmt.Sprintf("%q_to_%q", from, to), func(t *testing.T) {
				require.Equal(t, allowed[[2]structs.DynamicHostVolumeState{from, to}], dynamicHostVolumeTransitionAllowed(from, to))
			})
		}
	}
}

// requireTransitionRefused checks the typed error and that the message names the
// current state.
func requireTransitionRefused(t *testing.T, err error, from structs.DynamicHostVolumeState) {
	t.Helper()
	require.ErrorIs(t, err, ErrDynamicHostVolumeStateTransition)
	name := string(from)
	if name == "" {
		name = "none"
	}
	require.Contains(t, err.Error(), fmt.Sprintf("current state %q", name))
}

func TestStateStore_DynamicHostVolumeCommandsPerState(t *testing.T) {
	states := []structs.DynamicHostVolumeState{
		structs.DynamicHostVolumeStateRequested, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeStateReady,
		structs.DynamicHostVolumeStateUnavailable, structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeStateFailed}
	type command struct {
		name string
		// allowedFrom lists the states the command is accepted in.
		allowedFrom []structs.DynamicHostVolumeState
		run         func(store *StateStore, index uint64, cur *structs.DynamicHostVolume) error
	}
	commands := []command{
		{"outcome ready", []structs.DynamicHostVolumeState{"pending"}, func(s *StateStore, i uint64, cur *structs.DynamicHostVolume) error {
			r := outcomeReady(cur.ID)
			r.CapacityBytes = 77 // never equal to a stored outcome, so a repeat is not mistaken for a no-op
			return s.RecordDynamicHostVolumeOutcome(i, r)
		}},
		{"outcome failed", []structs.DynamicHostVolumeState{"requested", "pending"}, func(s *StateStore, i uint64, cur *structs.DynamicHostVolume) error {
			return s.RecordDynamicHostVolumeOutcome(i, outcomeFailed(cur.ID, "different diagnostic"))
		}},
		{"update", []structs.DynamicHostVolumeState{"ready"}, func(s *StateStore, i uint64, cur *structs.DynamicHostVolume) error {
			next := cur.Copy()
			next.Parameters = map[string]string{"synthetic-key": "changed"}
			return s.UpdateDynamicHostVolume(i, next)
		}},
		{"mark deleting", []structs.DynamicHostVolumeState{"requested", "pending", "ready", "unavailable", "failed", "deleting"},
			func(s *StateStore, i uint64, cur *structs.DynamicHostVolume) error {
				return s.MarkDynamicHostVolumesDeleting(i, []structs.DynamicHostVolumeKey{transitionKey(cur.ID)}, 400)
			}},
		{"remove", []structs.DynamicHostVolumeState{"deleting"}, func(s *StateStore, i uint64, cur *structs.DynamicHostVolume) error {
			return s.RemoveDynamicHostVolumes(i, []structs.DynamicHostVolumeKey{transitionKey(cur.ID)})
		}},
	}
	for _, cmd := range commands {
		for _, from := range states {
			t.Run(cmd.name+" from "+string(from), func(t *testing.T) {
				store := TestStateStore(t)
				index := transitionSeed(t, store, "volume-a", from)
				cur := transitionGet(t, store, "volume-a")
				err := cmd.run(store, index+1, cur)
				accepted := false
				for _, ok := range cmd.allowedFrom {
					accepted = accepted || ok == from
				}
				if accepted {
					require.NoError(t, err)
					return
				}
				requireTransitionRefused(t, err, from)
				// A refusal changes nothing.
				after, getErr := store.DynamicHostVolumeByID(nil, "default", "volume-a")
				require.NoError(t, getErr)
				require.True(t, cur.Equal(after))
			})
		}
	}
}

func TestStateStore_DynamicHostVolumeCreateRules(t *testing.T) {
	t.Run("missing record is refused naming none", func(t *testing.T) {
		store := TestStateStore(t)
		requireTransitionRefused(t, store.RecordDynamicHostVolumeOutcome(11, outcomeReady("volume-a")), "")
		requireTransitionRefused(t, store.UpdateDynamicHostVolume(11, transitionPending("volume-a")), "")
		requireTransitionRefused(t, store.MarkDynamicHostVolumesDeleting(11, []structs.DynamicHostVolumeKey{transitionKey("volume-a")}, 1), "")
	})
	t.Run("create may only start requested or pending", func(t *testing.T) {
		for _, bad := range []structs.DynamicHostVolumeState{"", "ready", "unavailable", "deleting", "failed"} {
			store := TestStateStore(t)
			v := transitionPending("volume-a")
			v.State = bad
			err := store.CreateDynamicHostVolumes(11, []*structs.DynamicHostVolume{v})
			requireTransitionRefused(t, err, "")
		}
	})
	t.Run("pending needs a node and requested has none", func(t *testing.T) {
		store := TestStateStore(t)
		v := transitionVolume("volume-a")
		v.State = structs.DynamicHostVolumeStatePending
		require.ErrorIs(t, store.CreateDynamicHostVolumes(11, []*structs.DynamicHostVolume{v}), ErrDynamicHostVolumeStateInvalid)
		w := transitionPending("volume-b")
		w.State = structs.DynamicHostVolumeStateRequested
		require.ErrorIs(t, store.CreateDynamicHostVolumes(11, []*structs.DynamicHostVolume{w}), ErrDynamicHostVolumeStateInvalid)
	})
	t.Run("creation takes the create index and the request time", func(t *testing.T) {
		store := TestStateStore(t)
		require.NoError(t, store.CreateDynamicHostVolumes(11, []*structs.DynamicHostVolume{transitionVolume("volume-a")}))
		got := transitionGet(t, store, "volume-a")
		require.Equal(t, uint64(11), got.CreateIndex)
		require.Equal(t, uint64(11), got.ModifyIndex)
		require.Equal(t, int64(100), got.CreateTime)
	})
	t.Run("a batch with one refused volume changes nothing", func(t *testing.T) {
		store := TestStateStore(t)
		bad := transitionPending("volume-b")
		bad.State = structs.DynamicHostVolumeStateReady
		err := store.CreateDynamicHostVolumes(11, []*structs.DynamicHostVolume{transitionVolume("volume-a"), bad})
		requireTransitionRefused(t, err, "")
		got, getErr := store.DynamicHostVolumeByID(nil, "default", "volume-a")
		require.NoError(t, getErr)
		require.Nil(t, got)
		require.Zero(t, tableIndexOf(t, store, TableDynamicHostVolumes))
	})
	t.Run("a batch of mark deleting is atomic", func(t *testing.T) {
		store := TestStateStore(t)
		index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateReady)
		err := store.MarkDynamicHostVolumesDeleting(index+1,
			[]structs.DynamicHostVolumeKey{transitionKey("volume-a"), transitionKey("volume-missing")}, 5)
		requireTransitionRefused(t, err, "")
		require.Equal(t, structs.DynamicHostVolumeStateReady, transitionGet(t, store, "volume-a").State)
	})
}

func TestStateStore_DynamicHostVolumeIdempotentRepeats(t *testing.T) {
	t.Run("repeated create with new timestamps is a no-op", func(t *testing.T) {
		store := TestStateStore(t)
		require.NoError(t, store.CreateDynamicHostVolumes(11, []*structs.DynamicHostVolume{transitionVolume("volume-a")}))
		before := transitionGet(t, store, "volume-a")
		again := transitionVolume("volume-a")
		again.CreateTime, again.ModifyTime = 999, 999
		require.NoError(t, store.CreateDynamicHostVolumes(12, []*structs.DynamicHostVolume{again}))
		require.True(t, before.Equal(transitionGet(t, store, "volume-a")))
	})
	t.Run("repeated create after the volume advanced is a no-op", func(t *testing.T) {
		store := TestStateStore(t)
		index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateReady)
		before := transitionGet(t, store, "volume-a")
		require.NoError(t, store.CreateDynamicHostVolumes(index+1, []*structs.DynamicHostVolume{transitionPending("volume-a")}))
		require.True(t, before.Equal(transitionGet(t, store, "volume-a")))
	})
	t.Run("repeated create with different content is refused", func(t *testing.T) {
		store := TestStateStore(t)
		require.NoError(t, store.CreateDynamicHostVolumes(11, []*structs.DynamicHostVolume{transitionVolume("volume-a")}))
		other := transitionVolume("volume-a")
		other.Parameters = map[string]string{"synthetic-key": "other"}
		requireTransitionRefused(t, store.CreateDynamicHostVolumes(12, []*structs.DynamicHostVolume{other}), structs.DynamicHostVolumeStateRequested)
	})
	t.Run("repeated register with the same host path is a no-op and a different one is refused", func(t *testing.T) {
		store := TestStateStore(t)
		reg := transitionPending("volume-a")
		reg.Origin, reg.HostPath, reg.PluginID = structs.DynamicHostVolumeOriginRegistered, "/srv/wander-synthetic/a", ""
		require.NoError(t, store.CreateDynamicHostVolumes(11, []*structs.DynamicHostVolume{reg}))
		outcome := outcomeReady("volume-a")
		outcome.HostPath = reg.HostPath
		require.NoError(t, store.RecordDynamicHostVolumeOutcome(12, outcome))
		before := transitionGet(t, store, "volume-a")
		require.NoError(t, store.CreateDynamicHostVolumes(13, []*structs.DynamicHostVolume{reg}))
		require.True(t, before.Equal(transitionGet(t, store, "volume-a")))
		moved := reg.Copy()
		moved.HostPath = "/srv/wander-synthetic/other"
		requireTransitionRefused(t, store.CreateDynamicHostVolumes(14, []*structs.DynamicHostVolume{moved}), structs.DynamicHostVolumeStateReady)
	})
	t.Run("repeated outcome is a no-op", func(t *testing.T) {
		store := TestStateStore(t)
		index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateReady)
		before := transitionGet(t, store, "volume-a")
		require.NoError(t, store.RecordDynamicHostVolumeOutcome(index+1, outcomeReady("volume-a")))
		require.True(t, before.Equal(transitionGet(t, store, "volume-a")))
		failed := TestStateStore(t)
		index = transitionSeed(t, failed, "volume-a", structs.DynamicHostVolumeStateFailed)
		before = transitionGet(t, failed, "volume-a")
		require.NoError(t, failed.RecordDynamicHostVolumeOutcome(index+1, outcomeFailed("volume-a", "no node")))
		require.True(t, before.Equal(transitionGet(t, failed, "volume-a")))
	})
	t.Run("repeated mark deleting keeps the modify index", func(t *testing.T) {
		store := TestStateStore(t)
		index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateDeleting)
		before := transitionGet(t, store, "volume-a")
		require.NoError(t, store.MarkDynamicHostVolumesDeleting(index+1, []structs.DynamicHostVolumeKey{transitionKey("volume-a")}, 9999))
		after := transitionGet(t, store, "volume-a")
		require.Equal(t, before.ModifyIndex, after.ModifyIndex)
		require.Equal(t, before.ModifyTime, after.ModifyTime)
		require.True(t, before.Equal(after))
	})
	t.Run("repeated remove of a gone record is a no-op", func(t *testing.T) {
		store := TestStateStore(t)
		index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateDeleting)
		require.NoError(t, store.RemoveDynamicHostVolumes(index+1, []structs.DynamicHostVolumeKey{transitionKey("volume-a")}))
		require.NoError(t, store.RemoveDynamicHostVolumes(index+2, []structs.DynamicHostVolumeKey{transitionKey("volume-a")}))
		got, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
		require.NoError(t, err)
		require.Nil(t, got)
	})
}

func TestStateStore_DynamicHostVolumeOutcomeRules(t *testing.T) {
	t.Run("ready stores path and capacity and clears the diagnostic", func(t *testing.T) {
		store := TestStateStore(t)
		transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStatePending)
		require.NoError(t, store.RecordDynamicHostVolumeOutcome(20, outcomeReady("volume-a")))
		got := transitionGet(t, store, "volume-a")
		require.Equal(t, structs.DynamicHostVolumeStateReady, got.State)
		require.Equal(t, "/srv/wander-synthetic/volume-a", got.HostPath)
		require.Equal(t, int64(50), got.CapacityBytes)
		require.Equal(t, int64(200), got.ModifyTime)
		require.Equal(t, uint64(20), got.ModifyIndex)
		require.Empty(t, got.StatusDescription)
	})
	t.Run("ready needs a path and a capacity", func(t *testing.T) {
		store := TestStateStore(t)
		transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStatePending)
		noPath := outcomeReady("volume-a")
		noPath.HostPath = ""
		require.ErrorIs(t, store.RecordDynamicHostVolumeOutcome(20, noPath), ErrDynamicHostVolumeStateInvalid)
		relative := outcomeReady("volume-a")
		relative.HostPath = "relative/path"
		require.ErrorIs(t, store.RecordDynamicHostVolumeOutcome(20, relative), ErrDynamicHostVolumeStateInvalid)
		negative := outcomeReady("volume-a")
		negative.CapacityBytes = -1
		require.ErrorIs(t, store.RecordDynamicHostVolumeOutcome(20, negative), ErrDynamicHostVolumeStateInvalid)
		require.Equal(t, structs.DynamicHostVolumeStatePending, transitionGet(t, store, "volume-a").State)
	})
	t.Run("failed stores the diagnostic and needs one", func(t *testing.T) {
		store := TestStateStore(t)
		transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStatePending)
		require.ErrorIs(t, store.RecordDynamicHostVolumeOutcome(20, outcomeFailed("volume-a", "")), ErrDynamicHostVolumeStateInvalid)
		require.NoError(t, store.RecordDynamicHostVolumeOutcome(21, outcomeFailed("volume-a", "plugin exited 1")))
		got := transitionGet(t, store, "volume-a")
		require.Equal(t, structs.DynamicHostVolumeStateFailed, got.State)
		require.Equal(t, "plugin exited 1", got.StatusDescription)
	})
	t.Run("a long diagnostic is cut at a rune boundary", func(t *testing.T) {
		store := TestStateStore(t)
		transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStatePending)
		long := strings.Repeat("é", structs.DynamicHostVolumeMaxDiagnosticBytes)
		require.NoError(t, store.RecordDynamicHostVolumeOutcome(20, outcomeFailed("volume-a", long)))
		got := transitionGet(t, store, "volume-a").StatusDescription
		require.LessOrEqual(t, len(got), structs.DynamicHostVolumeMaxDiagnosticBytes)
		require.True(t, strings.HasPrefix(long, got))
		require.NotEmpty(t, got)
	})
	t.Run("only ready and failed are outcomes", func(t *testing.T) {
		store := TestStateStore(t)
		transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStatePending)
		bad := outcomeReady("volume-a")
		bad.State = structs.DynamicHostVolumeStateDeleting
		require.ErrorIs(t, store.RecordDynamicHostVolumeOutcome(20, bad), ErrDynamicHostVolumeStateInvalid)
	})
}

func TestStateStore_DynamicHostVolumeAssignNode(t *testing.T) {
	t.Run("requested to pending assigns the node", func(t *testing.T) {
		store := TestStateStore(t)
		transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateRequested)
		cur := transitionGet(t, store, "volume-a")
		next := transitionPending("volume-a")
		next.ModifyIndex, next.CreateIndex = cur.ModifyIndex, cur.CreateIndex
		require.NoError(t, store.CreateDynamicHostVolumes(20, []*structs.DynamicHostVolume{next}))
		got := transitionGet(t, store, "volume-a")
		require.Equal(t, transitionNode, got.NodeID)
		require.Equal(t, structs.DynamicHostVolumeStatePending, got.State)
		require.Equal(t, cur.CreateIndex, got.CreateIndex)
		require.Equal(t, uint64(20), got.ModifyIndex)
	})
	t.Run("a failed volume cannot be given a node", func(t *testing.T) {
		store := TestStateStore(t)
		transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateFailed)
		cur := transitionGet(t, store, "volume-a")
		next := transitionPending("volume-a")
		next.ModifyIndex, next.CreateIndex = cur.ModifyIndex, cur.CreateIndex
		requireTransitionRefused(t, store.CreateDynamicHostVolumes(20, []*structs.DynamicHostVolume{next}), structs.DynamicHostVolumeStateFailed)
	})
	t.Run("a stale modify index is a CAS conflict", func(t *testing.T) {
		store := TestStateStore(t)
		transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateRequested)
		next := transitionPending("volume-a")
		next.ModifyIndex = 1
		require.ErrorIs(t, store.CreateDynamicHostVolumes(20, []*structs.DynamicHostVolume{next}), ErrDynamicHostVolumeStateCAS)
	})
	t.Run("the move may change nothing but the node and the state", func(t *testing.T) {
		store := TestStateStore(t)
		transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateRequested)
		cur := transitionGet(t, store, "volume-a")
		for name, mutate := range map[string]func(*structs.DynamicHostVolume){
			"name":   func(v *structs.DynamicHostVolume) { v.Name = "other" },
			"origin": func(v *structs.DynamicHostVolume) { v.Origin = structs.DynamicHostVolumeOriginRegistered },
			"spec":   func(v *structs.DynamicHostVolume) { v.RequestedCapacityMaxBytes = 5 },
		} {
			next := transitionPending("volume-a")
			next.ModifyIndex, next.CreateIndex = cur.ModifyIndex, cur.CreateIndex
			mutate(next)
			require.Error(t, store.CreateDynamicHostVolumes(20, []*structs.DynamicHostVolume{next}), name)
		}
		require.True(t, cur.Equal(transitionGet(t, store, "volume-a")))
	})
}

// A request that named no pool, or all pools, is stored with that pool. Giving
// the volume its node fills in the node's own pool and nothing else; a named
// pool never changes.
func TestStateStore_DynamicHostVolumeAssignNodeFillsPool(t *testing.T) {
	seedPool := func(t *testing.T, pool string) (*StateStore, *structs.DynamicHostVolume) {
		store := TestStateStore(t)
		v := transitionVolume("volume-a")
		v.NodePool = pool
		require.NoError(t, store.CreateDynamicHostVolumes(11, []*structs.DynamicHostVolume{v}))
		return store, transitionGet(t, store, "volume-a")
	}
	assign := func(cur *structs.DynamicHostVolume, pool string) *structs.DynamicHostVolume {
		next := transitionPending("volume-a")
		next.ModifyIndex, next.CreateIndex = cur.ModifyIndex, cur.CreateIndex
		next.NodePool = pool
		return next
	}
	for _, stored := range []string{"", structs.NodePoolAll} {
		t.Run("pool "+stored+" takes the node pool", func(t *testing.T) {
			store, cur := seedPool(t, stored)
			require.NoError(t, store.CreateDynamicHostVolumes(20, []*structs.DynamicHostVolume{assign(cur, "synthetic-pool")}))
			got := transitionGet(t, store, "volume-a")
			require.Equal(t, "synthetic-pool", got.NodePool)
			require.Equal(t, transitionNode, got.NodeID)
			require.Equal(t, structs.DynamicHostVolumeStatePending, got.State)
		})
	}
	t.Run("a named pool cannot change", func(t *testing.T) {
		store, cur := seedPool(t, "synthetic-pool")
		require.ErrorIs(t, store.CreateDynamicHostVolumes(20, []*structs.DynamicHostVolume{assign(cur, "other-pool")}),
			ErrDynamicHostVolumeStateConflict)
		require.True(t, cur.Equal(transitionGet(t, store, "volume-a")))
	})
	t.Run("the fill still refuses any other change", func(t *testing.T) {
		store, cur := seedPool(t, "")
		next := assign(cur, "synthetic-pool")
		next.Name = "other"
		require.Error(t, store.CreateDynamicHostVolumes(20, []*structs.DynamicHostVolume{next}))
		next = assign(cur, "synthetic-pool")
		next.RequestedCapacityMaxBytes = 5
		require.ErrorIs(t, store.CreateDynamicHostVolumes(20, []*structs.DynamicHostVolume{next}), ErrDynamicHostVolumeStateConflict)
		require.True(t, cur.Equal(transitionGet(t, store, "volume-a")))
	})
}

func TestStateStore_DynamicHostVolumeUpdateRules(t *testing.T) {
	readyStore := func(t *testing.T) (*StateStore, *structs.DynamicHostVolume, uint64) {
		store := TestStateStore(t)
		index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateReady)
		return store, transitionGet(t, store, "volume-a"), index
	}
	t.Run("changes mutable fields with CAS", func(t *testing.T) {
		store, cur, index := readyStore(t)
		next := cur.Copy()
		next.Parameters = map[string]string{"synthetic-key": "changed"}
		next.ModifyTime = 500
		require.NoError(t, store.UpdateDynamicHostVolume(index+1, next))
		got := transitionGet(t, store, "volume-a")
		require.Equal(t, "changed", got.Parameters["synthetic-key"])
		require.Equal(t, index+1, got.ModifyIndex)
		require.Equal(t, cur.CreateIndex, got.CreateIndex)
		require.Equal(t, structs.DynamicHostVolumeStateReady, got.State)
		// The caller's copy is untouched.
		require.Equal(t, cur.ModifyIndex, cur.Copy().ModifyIndex)
	})
	t.Run("a stale modify index is a CAS conflict", func(t *testing.T) {
		store, cur, index := readyStore(t)
		next := cur.Copy()
		next.ModifyIndex = cur.ModifyIndex - 1
		require.ErrorIs(t, store.UpdateDynamicHostVolume(index+1, next), ErrDynamicHostVolumeStateCAS)
		next.ModifyIndex = 0
		require.ErrorIs(t, store.UpdateDynamicHostVolume(index+1, next), ErrDynamicHostVolumeStateCAS)
	})
	t.Run("immutable fields and state cannot change", func(t *testing.T) {
		store, cur, index := readyStore(t)
		for name, mutate := range map[string]func(*structs.DynamicHostVolume){
			"name":        func(v *structs.DynamicHostVolume) { v.Name = "other" },
			"node":        func(v *structs.DynamicHostVolume) { v.NodeID = "00000000-0000-0000-0000-00000000000b" },
			"empty node":  func(v *structs.DynamicHostVolume) { v.NodeID = "" },
			"origin":      func(v *structs.DynamicHostVolume) { v.Origin = structs.DynamicHostVolumeOriginRegistered },
			"create time": func(v *structs.DynamicHostVolume) { v.CreateTime = 1 },
			"create idx":  func(v *structs.DynamicHostVolume) { v.CreateIndex = 1 },
		} {
			next := cur.Copy()
			mutate(next)
			err := store.UpdateDynamicHostVolume(index+1, next)
			require.ErrorIs(t, err, structs.ErrDynamicHostVolumeImmutable, name)
		}
		next := cur.Copy()
		next.State = structs.DynamicHostVolumeStateDeleting
		requireTransitionRefused(t, store.UpdateDynamicHostVolume(index+1, next), structs.DynamicHostVolumeStateReady)
		require.True(t, cur.Equal(transitionGet(t, store, "volume-a")))
	})
}

// An update must not rewrite what the server and the client own.
func TestStateStore_DynamicHostVolumeUpdateKeepsServerOwnedFields(t *testing.T) {
	for name, mutate := range map[string]func(*structs.DynamicHostVolume){
		"host path":          func(v *structs.DynamicHostVolume) { v.HostPath = "/srv/wander-synthetic/other" },
		"capacity":           func(v *structs.DynamicHostVolume) { v.CapacityBytes = 999 },
		"status description": func(v *structs.DynamicHostVolume) { v.StatusDescription = "forged" },
		"attachments": func(v *structs.DynamicHostVolume) {
			v.Attachments = []*structs.DynamicHostVolumeAttachment{{AllocationID: "alloc-a", NodeID: transitionNode}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			store := TestStateStore(t)
			index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateReady)
			before := transitionGet(t, store, "volume-a")
			next := before.Copy()
			mutate(next)
			next.Parameters = map[string]string{"synthetic-key": "changed"}
			err := store.UpdateDynamicHostVolume(index+1, next)
			require.ErrorIs(t, err, structs.ErrDynamicHostVolumeImmutable)
			require.True(t, before.Equal(transitionGet(t, store, "volume-a")))
		})
	}
	t.Run("mutable fields still change and server-owned ones stay", func(t *testing.T) {
		store := TestStateStore(t)
		index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateReady)
		before := transitionGet(t, store, "volume-a")
		next := before.Copy()
		next.Parameters = map[string]string{"synthetic-key": "changed"}
		require.NoError(t, store.UpdateDynamicHostVolume(index+1, next))
		got := transitionGet(t, store, "volume-a")
		require.Equal(t, "changed", got.Parameters["synthetic-key"])
		require.Equal(t, before.HostPath, got.HostPath)
		require.Equal(t, before.CapacityBytes, got.CapacityBytes)
	})
}

func TestStateStore_DynamicHostVolumeCreateBoundsDiagnostic(t *testing.T) {
	store := TestStateStore(t)
	v := transitionVolume("volume-a")
	v.StatusDescription = strings.Repeat("x", structs.DynamicHostVolumeMaxDiagnosticBytes*2)
	require.NoError(t, store.CreateDynamicHostVolumes(11, []*structs.DynamicHostVolume{v}))
	require.Len(t, transitionGet(t, store, "volume-a").StatusDescription, structs.DynamicHostVolumeMaxDiagnosticBytes)
}

func TestStateStore_DynamicHostVolumeRemoveDeletesClaims(t *testing.T) {
	store := TestStateStore(t)
	index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateReady)
	// A claim of another volume stays.
	require.NoError(t, store.CreateDynamicHostVolumes(index+1, []*structs.DynamicHostVolume{transitionVolume("volume-b")}))
	require.NoError(t, store.UpsertDynamicHostVolumeClaims(index+2, []*structs.DynamicHostVolumeClaim{
		claimCleanupClaim("claim-a", "default", "job-a", "volume-a"),
		claimCleanupClaim("claim-b", "default", "job-a", "volume-b")}))

	requireTransitionRefused(t, store.RemoveDynamicHostVolumes(index+3, []structs.DynamicHostVolumeKey{transitionKey("volume-a")}),
		structs.DynamicHostVolumeStateReady)
	require.Equal(t, []string{"claim-a", "claim-b"}, claimIDsOf(t, store))

	require.NoError(t, store.MarkDynamicHostVolumesDeleting(index+4, []structs.DynamicHostVolumeKey{transitionKey("volume-a")}, 600))
	require.NoError(t, store.RemoveDynamicHostVolumes(index+5, []structs.DynamicHostVolumeKey{transitionKey("volume-a")}))

	gone, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
	require.NoError(t, err)
	require.Nil(t, gone)
	require.Equal(t, []string{"claim-b"}, claimIDsOf(t, store))
	require.Equal(t, index+5, tableIndexOf(t, store, TableDynamicHostVolumeClaims))
	require.Equal(t, index+5, tableIndexOf(t, store, TableDynamicHostVolumes))
}

func TestStateStore_DynamicHostVolumeRemoveRefusedWhileInUse(t *testing.T) {
	store := TestStateStore(t)
	require.NoError(t, store.CreateDynamicHostVolumes(10, []*structs.DynamicHostVolume{func() *structs.DynamicHostVolume {
		v := transitionPending("volume-a")
		v.NodeID = claimCleanupNodeA
		v.Name = "data-volume-a"
		return v
	}()}))
	// The allocation arrives after the mark, as one committed between the
	// mark and the removal would.
	require.NoError(t, store.MarkDynamicHostVolumesDeleting(15, []structs.DynamicHostVolumeKey{transitionKey("volume-a")}, 600))
	volumeUserAlloc(t, store, 20, nil)
	err := store.RemoveDynamicHostVolumes(31, []structs.DynamicHostVolumeKey{transitionKey("volume-a")})
	require.ErrorIs(t, err, ErrDynamicHostVolumeStateInUse)
	require.Equal(t, structs.DynamicHostVolumeStateDeleting, transitionGet(t, store, "volume-a").State)
}

// A volume a live allocation uses is never marked deleting: the client deletes
// its directory after the mark, so a mark that passed would destroy live data.
// The check runs in the apply, which Raft orders with the plan applier, so an
// allocation committed after the server looked is still seen.
func TestStateStore_DynamicHostVolumeMarkDeletingRefusedWhileInUse(t *testing.T) {
	store := TestStateStore(t)
	require.NoError(t, store.CreateDynamicHostVolumes(10, []*structs.DynamicHostVolume{func() *structs.DynamicHostVolume {
		v := transitionPending("volume-a")
		v.NodeID = claimCleanupNodeA
		v.Name = "data-volume-a"
		return v
	}()}))
	volumeUserAlloc(t, store, 20, nil)
	before := tableIndexOf(t, store, TableDynamicHostVolumes)

	err := store.MarkDynamicHostVolumesDeleting(30, []structs.DynamicHostVolumeKey{transitionKey("volume-a")}, 600)
	require.ErrorIs(t, err, ErrDynamicHostVolumeStateInUse)
	require.Equal(t, structs.DynamicHostVolumeStatePending, transitionGet(t, store, "volume-a").State)
	require.Equal(t, before, tableIndexOf(t, store, TableDynamicHostVolumes))
}

// A volume that is already deleting is not checked again: marking it is a
// no-op, and its removal is what refuses while an allocation uses it.
func TestStateStore_DynamicHostVolumeMarkDeletingRepeatSkipsInUse(t *testing.T) {
	store := TestStateStore(t)
	require.NoError(t, store.CreateDynamicHostVolumes(10, []*structs.DynamicHostVolume{func() *structs.DynamicHostVolume {
		v := transitionPending("volume-a")
		v.NodeID = claimCleanupNodeA
		v.Name = "data-volume-a"
		return v
	}()}))
	require.NoError(t, store.MarkDynamicHostVolumesDeleting(11, []structs.DynamicHostVolumeKey{transitionKey("volume-a")}, 600))
	volumeUserAlloc(t, store, 20, nil)

	require.NoError(t, store.MarkDynamicHostVolumesDeleting(30, []structs.DynamicHostVolumeKey{transitionKey("volume-a")}, 700))
	got := transitionGet(t, store, "volume-a")
	require.Equal(t, structs.DynamicHostVolumeStateDeleting, got.State)
	require.Equal(t, uint64(11), got.ModifyIndex)
}

// DynamicHostVolumeInUse is the read the server uses before it writes anything.
func TestStateStore_DynamicHostVolumeInUseRead(t *testing.T) {
	store := TestStateStore(t)
	vol := transitionPending("volume-a")
	vol.NodeID = claimCleanupNodeA
	vol.Name = "data-volume-a"
	require.NoError(t, store.CreateDynamicHostVolumes(10, []*structs.DynamicHostVolume{vol}))

	inUse, err := store.DynamicHostVolumeInUse(vol)
	require.NoError(t, err)
	require.False(t, inUse)

	volumeUserAlloc(t, store, 20, nil)
	inUse, err = store.DynamicHostVolumeInUse(vol)
	require.NoError(t, err)
	require.True(t, inUse)
}

func TestStateStore_DynamicHostVolumeReadyUnavailableNotBlocked(t *testing.T) {
	store := TestStateStore(t)
	index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateReady)
	cur := transitionGet(t, store, "volume-a")
	cur.State = structs.DynamicHostVolumeStateUnavailable
	require.NoError(t, store.UpsertDynamicHostVolumes(index+1, []*structs.DynamicHostVolume{cur}))
	cur = transitionGet(t, store, "volume-a")
	cur.State = structs.DynamicHostVolumeStateReady
	require.NoError(t, store.UpsertDynamicHostVolumes(index+2, []*structs.DynamicHostVolume{cur}))
}

func outcomeDeleting(id, code string) *structs.DynamicHostVolumeOutcomeRequest {
	return &structs.DynamicHostVolumeOutcomeRequest{
		Key: structs.DynamicHostVolumeKey{Namespace: "default", ID: id}, State: structs.DynamicHostVolumeStateDeleting,
		Diagnostic: code, ModifyTime: 999}
}

func TestStateStore_DynamicHostVolumeOutcomeDeleting(t *testing.T) {
	t.Run("stores the code and changes nothing else", func(t *testing.T) {
		store := TestStateStore(t)
		index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateDeleting)
		before := transitionGet(t, store, "volume-a")
		require.NoError(t, store.RecordDynamicHostVolumeOutcome(index+1, outcomeDeleting("volume-a", "client_unreachable")))
		got := transitionGet(t, store, "volume-a")
		require.Equal(t, structs.DynamicHostVolumeStateDeleting, got.State)
		require.Equal(t, "client_unreachable", got.StatusDescription)
		require.Equal(t, index+1, got.ModifyIndex)
		require.Equal(t, before.ModifyTime, got.ModifyTime)
		want := before.Copy()
		want.StatusDescription, want.ModifyIndex = got.StatusDescription, got.ModifyIndex
		require.True(t, want.Equal(got))
	})
	t.Run("a repeat is a no-op", func(t *testing.T) {
		store := TestStateStore(t)
		index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateDeleting)
		require.NoError(t, store.RecordDynamicHostVolumeOutcome(index+1, outcomeDeleting("volume-a", "volume_in_use")))
		require.NoError(t, store.RecordDynamicHostVolumeOutcome(index+2, outcomeDeleting("volume-a", "volume_in_use")))
		require.Equal(t, index+1, transitionGet(t, store, "volume-a").ModifyIndex)
	})
	t.Run("a new code replaces the old one", func(t *testing.T) {
		store := TestStateStore(t)
		index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateDeleting)
		require.NoError(t, store.RecordDynamicHostVolumeOutcome(index+1, outcomeDeleting("volume-a", "volume_in_use")))
		require.NoError(t, store.RecordDynamicHostVolumeOutcome(index+2, outcomeDeleting("volume-a", "node_not_found")))
		require.Equal(t, "node_not_found", transitionGet(t, store, "volume-a").StatusDescription)
	})
	t.Run("a code is needed", func(t *testing.T) {
		store := TestStateStore(t)
		index := transitionSeed(t, store, "volume-a", structs.DynamicHostVolumeStateDeleting)
		require.ErrorIs(t, store.RecordDynamicHostVolumeOutcome(index+1, outcomeDeleting("volume-a", "")), ErrDynamicHostVolumeStateInvalid)
	})
	t.Run("a missing record is refused", func(t *testing.T) {
		store := TestStateStore(t)
		requireTransitionRefused(t, store.RecordDynamicHostVolumeOutcome(5, outcomeDeleting("volume-a", "node_not_found")), "")
	})
	for _, from := range []structs.DynamicHostVolumeState{
		structs.DynamicHostVolumeStateRequested, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeStateReady,
		structs.DynamicHostVolumeStateUnavailable, structs.DynamicHostVolumeStateFailed,
	} {
		t.Run("refused from "+string(from), func(t *testing.T) {
			store := TestStateStore(t)
			index := transitionSeed(t, store, "volume-a", from)
			before := transitionGet(t, store, "volume-a")
			requireTransitionRefused(t, store.RecordDynamicHostVolumeOutcome(index+1, outcomeDeleting("volume-a", "node_not_found")), from)
			require.True(t, before.Equal(transitionGet(t, store, "volume-a")))
		})
	}
}

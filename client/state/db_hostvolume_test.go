// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"testing"
	"time"

	"github.com/open-wander/wander/ci"
	hvstate "github.com/open-wander/wander/client/hostvolume/state"
	"github.com/open-wander/wander/helper/boltdd"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/shoenig/test/must"
)

func syntheticHostVolume(id string) *hvstate.Record {
	return &hvstate.Record{
		VolumeID: id, Namespace: "ns-synthetic", Name: "name-" + id, NodeID: "node-synthetic",
		Kind: hvstate.KindPlugin, PluginID: "plugin-synthetic",
		CapacityMinBytes: 10, CapacityMaxBytes: 20,
		Parameters: map[string]string{"k": "v"},
		Path:       "/synthetic/" + id,
		Status:     hvstate.StatusReady,
		CheckedAt:  time.Unix(1700000000, 0).UTC(),
	}
}

func TestStateDB_HostVolumes(t *testing.T) {
	ci.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {
		// No records is not an error.
		recs, errs, err := db.GetHostVolumes()
		must.NoError(t, err)
		must.Len(t, 0, recs)
		must.MapLen(t, 0, errs)

		a, b := syntheticHostVolume("vol-a"), syntheticHostVolume("vol-b")
		must.NoError(t, db.PutHostVolume(a))
		must.NoError(t, db.PutHostVolume(b))

		// Mutating the value after put must not change the stored copy.
		a.Parameters["k"] = "mutated"

		recs, errs, err = db.GetHostVolumes()
		must.NoError(t, err)
		must.MapLen(t, 0, errs)
		must.Len(t, 2, recs)
		got := map[string]*hvstate.Record{}
		for _, r := range recs {
			got[r.VolumeID] = r
		}
		must.Eq(t, "v", got["vol-a"].Parameters["k"])
		must.True(t, got["vol-b"].CheckedAt.Equal(b.CheckedAt))
		must.Eq(t, "/synthetic/vol-b", got["vol-b"].Path)

		// Mutating a returned value must not change the stored copy.
		got["vol-b"].Path = "/changed"
		recs, _, _ = db.GetHostVolumes()
		for _, r := range recs {
			must.NotEq(t, "/changed", r.Path)
		}

		// Put replaces.
		b2 := syntheticHostVolume("vol-b")
		b2.Status = hvstate.StatusUnavailable
		b2.Reason = hvstate.ReasonMissingPath
		must.NoError(t, db.PutHostVolume(b2))
		recs, _, _ = db.GetHostVolumes()
		must.Len(t, 2, recs)

		must.NoError(t, db.DeleteHostVolume("vol-a"))
		must.NoError(t, db.DeleteHostVolume("vol-absent"))
		recs, errs, err = db.GetHostVolumes()
		must.NoError(t, err)
		must.MapLen(t, 0, errs)
		must.Len(t, 1, recs)
		must.Eq(t, "vol-b", recs[0].VolumeID)
		must.Eq(t, hvstate.ReasonMissingPath, recs[0].Reason)
	})
}

func TestStateDB_HostVolumes_BoltReopen(t *testing.T) {
	ci.Parallel(t)

	dir := t.TempDir()
	db, err := NewBoltStateDB(testlog.HCLogger(t), dir)
	must.NoError(t, err)
	must.NoError(t, db.PutHostVolume(syntheticHostVolume("vol-a")))
	must.NoError(t, db.PutHostVolume(syntheticHostVolume("vol-b")))
	must.NoError(t, db.DeleteHostVolume("vol-b"))
	must.NoError(t, db.Close())

	db, err = NewBoltStateDB(testlog.HCLogger(t), dir)
	must.NoError(t, err)
	defer db.Close()
	recs, errs, err := db.GetHostVolumes()
	must.NoError(t, err)
	must.MapLen(t, 0, errs)
	must.Len(t, 1, recs)
	must.Eq(t, "vol-a", recs[0].VolumeID)
}

// A store written before this feature has no bucket: reading must not fail
// and must not create the bucket.
func TestStateDB_HostVolumes_BoltNoBucket(t *testing.T) {
	ci.Parallel(t)

	db := setupBoltStateDB(t)
	recs, errs, err := db.GetHostVolumes()
	must.NoError(t, err)
	must.Len(t, 0, recs)
	must.MapLen(t, 0, errs)
	must.NoError(t, db.DeleteHostVolume("vol-absent"))

	must.NoError(t, db.db.View(func(tx *boltdd.Tx) error {
		must.Nil(t, tx.Bucket(hostVolumesBucket))
		return nil
	}))
}

func TestStateDB_HostVolumes_BoltCorruptRecord(t *testing.T) {
	ci.Parallel(t)

	db := setupBoltStateDB(t)
	must.NoError(t, db.PutHostVolume(syntheticHostVolume("vol-a")))
	must.NoError(t, db.PutHostVolume(syntheticHostVolume("vol-c")))
	must.NoError(t, db.db.Update(func(tx *boltdd.Tx) error {
		b, err := tx.CreateBucketIfNotExists(hostVolumesBucket)
		if err != nil {
			return err
		}
		return b.BoltBucket().Put([]byte("vol-b"), []byte{0xc1, 0xff, 0x00, 0x01})
	}))

	recs, errs, err := db.GetHostVolumes()
	must.NoError(t, err)
	must.Len(t, 2, recs)
	must.MapLen(t, 1, errs)
	must.MapContainsKey(t, errs, "vol-b")
}

func TestStateDB_HostVolumes_NoopAndErr(t *testing.T) {
	ci.Parallel(t)

	n := NoopDB{}
	must.NoError(t, n.PutHostVolume(syntheticHostVolume("vol-a")))
	recs, errs, err := n.GetHostVolumes()
	must.NoError(t, err)
	must.Len(t, 0, recs)
	must.MapLen(t, 0, errs)
	must.NoError(t, n.DeleteHostVolume("vol-a"))

	e := &ErrDB{}
	must.Error(t, e.PutHostVolume(syntheticHostVolume("vol-a")))
	_, _, err = e.GetHostVolumes()
	must.Error(t, err)
	must.Error(t, e.DeleteHostVolume("vol-a"))
}

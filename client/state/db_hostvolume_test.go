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

// A record stored under a key that is not its volume ID is reported by key
// and not returned. Returned as a record, it would outlive the delete of its
// volume and come back at the next client start.
func TestStateDB_HostVolumes_BoltRecordUnderOtherKey(t *testing.T) {
	ci.Parallel(t)

	db := setupBoltStateDB(t)
	must.NoError(t, db.PutHostVolume(syntheticHostVolume("vol-a")))
	stray := syntheticHostVolume("vol-a")
	stray.Path = "/synthetic/stray"
	must.NoError(t, db.db.Update(func(tx *boltdd.Tx) error {
		b, err := tx.CreateBucketIfNotExists(hostVolumesBucket)
		if err != nil {
			return err
		}
		return b.Put([]byte("vol-z"), stray)
	}))

	recs, errs, err := db.GetHostVolumes()
	must.NoError(t, err)
	must.Len(t, 1, recs)
	must.Eq(t, "/synthetic/vol-a", recs[0].Path)
	must.MapLen(t, 1, errs)
	must.MapContainsKey(t, errs, "vol-z")

	must.NoError(t, db.DeleteHostVolume("vol-a"))
	recs, _, err = db.GetHostVolumes()
	must.NoError(t, err)
	must.Len(t, 0, recs)
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

func TestStateDB_HostVolumeTombstones(t *testing.T) {
	ci.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {
		ts, errs, err := db.GetHostVolumeTombstones()
		must.NoError(t, err)
		must.Len(t, 0, ts)
		must.MapLen(t, 0, errs)

		at := time.Unix(1700000000, 0).UTC()
		must.NoError(t, db.PutHostVolumeTombstone(&hvstate.Tombstone{VolumeID: "vol-a", DeletedAt: at}))
		must.NoError(t, db.PutHostVolumeTombstone(&hvstate.Tombstone{VolumeID: "vol-b", DeletedAt: at}))
		// A tombstone is never a record, and a record is never a tombstone.
		must.NoError(t, db.PutHostVolume(syntheticHostVolume("vol-c")))
		recs, _, err := db.GetHostVolumes()
		must.NoError(t, err)
		must.Len(t, 1, recs)
		must.Eq(t, "vol-c", recs[0].VolumeID)

		ts, errs, err = db.GetHostVolumeTombstones()
		must.NoError(t, err)
		must.MapLen(t, 0, errs)
		must.Len(t, 2, ts)
		got := map[string]*hvstate.Tombstone{}
		for _, x := range ts {
			got[x.VolumeID] = x
		}
		must.True(t, got["vol-a"].DeletedAt.Equal(at))
		must.MapNotContainsKey(t, got, "vol-c")

		// Put replaces, which refreshes the time.
		later := at.Add(time.Hour)
		must.NoError(t, db.PutHostVolumeTombstone(&hvstate.Tombstone{VolumeID: "vol-a", DeletedAt: later}))
		ts, _, _ = db.GetHostVolumeTombstones()
		must.Len(t, 2, ts)
		for _, x := range ts {
			if x.VolumeID == "vol-a" {
				must.True(t, x.DeletedAt.Equal(later))
			}
		}

		must.NoError(t, db.DeleteHostVolumeTombstone("vol-a"))
		must.NoError(t, db.DeleteHostVolumeTombstone("vol-absent"))
		ts, _, err = db.GetHostVolumeTombstones()
		must.NoError(t, err)
		must.Len(t, 1, ts)
		must.Eq(t, "vol-b", ts[0].VolumeID)
	})
}

func TestStateDB_HostVolumeTombstones_BoltReopen(t *testing.T) {
	ci.Parallel(t)

	dir := t.TempDir()
	db, err := NewBoltStateDB(testlog.HCLogger(t), dir)
	must.NoError(t, err)
	at := time.Unix(1700000000, 0).UTC()
	must.NoError(t, db.PutHostVolumeTombstone(&hvstate.Tombstone{VolumeID: "vol-a", DeletedAt: at}))
	must.NoError(t, db.Close())

	db, err = NewBoltStateDB(testlog.HCLogger(t), dir)
	must.NoError(t, err)
	defer db.Close()
	ts, errs, err := db.GetHostVolumeTombstones()
	must.NoError(t, err)
	must.MapLen(t, 0, errs)
	must.Len(t, 1, ts)
	must.True(t, ts[0].DeletedAt.Equal(at))
}

// A store with no tombstone bucket reads as empty and the read does not
// create the bucket.
func TestStateDB_HostVolumeTombstones_BoltNoBucket(t *testing.T) {
	ci.Parallel(t)

	db := setupBoltStateDB(t)
	ts, errs, err := db.GetHostVolumeTombstones()
	must.NoError(t, err)
	must.Len(t, 0, ts)
	must.MapLen(t, 0, errs)
	must.NoError(t, db.DeleteHostVolumeTombstone("vol-absent"))
	must.NoError(t, db.db.View(func(tx *boltdd.Tx) error {
		must.Nil(t, tx.Bucket(hostVolumeTombstonesBucket))
		return nil
	}))
}

func TestStateDB_HostVolumeTombstones_BoltCorrupt(t *testing.T) {
	ci.Parallel(t)

	db := setupBoltStateDB(t)
	at := time.Unix(1700000000, 0).UTC()
	must.NoError(t, db.PutHostVolumeTombstone(&hvstate.Tombstone{VolumeID: "vol-a", DeletedAt: at}))
	must.NoError(t, db.db.Update(func(tx *boltdd.Tx) error {
		b, err := tx.CreateBucketIfNotExists(hostVolumeTombstonesBucket)
		if err != nil {
			return err
		}
		if err := b.Put([]byte("vol-z"), &hvstate.Tombstone{VolumeID: "vol-other", DeletedAt: at}); err != nil {
			return err
		}
		return b.BoltBucket().Put([]byte("vol-b"), []byte{0xc1, 0xff, 0x00, 0x01})
	}))

	ts, errs, err := db.GetHostVolumeTombstones()
	must.NoError(t, err)
	must.Len(t, 1, ts)
	must.MapLen(t, 2, errs)
	must.MapContainsKey(t, errs, "vol-b")
	must.MapContainsKey(t, errs, "vol-z")
}

func TestStateDB_HostVolumeTombstones_NoopAndErr(t *testing.T) {
	ci.Parallel(t)

	n := NoopDB{}
	ts := &hvstate.Tombstone{VolumeID: "vol-a", DeletedAt: time.Unix(1, 0)}
	must.NoError(t, n.PutHostVolumeTombstone(ts))
	got, errs, err := n.GetHostVolumeTombstones()
	must.NoError(t, err)
	must.Len(t, 0, got)
	must.MapLen(t, 0, errs)
	must.NoError(t, n.DeleteHostVolumeTombstone("vol-a"))

	e := &ErrDB{}
	must.Error(t, e.PutHostVolumeTombstone(ts))
	_, _, err = e.GetHostVolumeTombstones()
	must.Error(t, err)
	must.Error(t, e.DeleteHostVolumeTombstone("vol-a"))
}

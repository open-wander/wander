// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"strings"
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func validRecord() *Record {
	return &Record{
		VolumeID: "vol-1", Namespace: "ns", Name: "data", NodeID: "node-1",
		Kind: KindPlugin, PluginID: "plug", Path: "/synthetic/vol-1",
		Parameters: map[string]string{"k": "v"},
		Status:     StatusReady, CheckedAt: time.Unix(1, 0),
	}
}

func TestRecord_Validate(t *testing.T) {
	must.NoError(t, validRecord().Validate())

	cases := map[string]func(r *Record){
		"empty id":         func(r *Record) { r.VolumeID = "" },
		"id slash":         func(r *Record) { r.VolumeID = "a/b" },
		"id too long":      func(r *Record) { r.VolumeID = strings.Repeat("a", MaxIDLen+1) },
		"empty name":       func(r *Record) { r.Name = "" },
		"control in name":  func(r *Record) { r.Name = "a\nb" },
		"empty node":       func(r *Record) { r.NodeID = "" },
		"kind unknown":     func(r *Record) { r.Kind = "other" },
		"plugin no id":     func(r *Record) { r.PluginID = "" },
		"registered+plug":  func(r *Record) { r.Kind = KindRegistered },
		"negative cap":     func(r *Record) { r.CapacityMinBytes = -1 },
		"relative path":    func(r *Record) { r.Path = "a/b" },
		"unclean path":     func(r *Record) { r.Path = "/a/../b" },
		"root path":        func(r *Record) { r.Path = "/" },
		"nul in path":      func(r *Record) { r.Path = "/a\x00b" },
		"status unknown":   func(r *Record) { r.Status = "gone" },
		"message too long": func(r *Record) { r.Message = strings.Repeat("m", MaxMessageLen+1) },
		"reason too long":  func(r *Record) { r.Reason = strings.Repeat("r", MaxIDLen+1) },
	}
	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			r := validRecord()
			mod(r)
			must.Error(t, r.Validate())
		})
	}
}

func TestRecord_ValidateErrorsHoldNoParameterValue(t *testing.T) {
	r := validRecord()
	r.Parameters = map[string]string{"k": "SENTINEL-value"}
	r.Path = "relative"
	err := r.Validate()
	must.Error(t, err)
	must.StrNotContains(t, err.Error(), "SENTINEL")
}

func TestRecord_CopyIsDeep(t *testing.T) {
	r := validRecord()
	c := r.Copy()
	c.Parameters["k"] = "changed"
	must.Eq(t, "v", r.Parameters["k"])
	var nilRec *Record
	must.Nil(t, nilRec.Copy())
}

func TestTombstone_Validate(t *testing.T) {
	good := Tombstone{VolumeID: "vol-1", DeletedAt: time.Unix(1700000000, 0)}
	must.NoError(t, good.Validate())

	// Every ID the manager accepts must validate, or its tombstone would
	// reload invalid and never expire.
	for _, id := range []string{"vol.1", "a.b.c", "v-1_x.2", "vol."} {
		ts := good
		ts.VolumeID = id
		must.NoError(t, ts.Validate(), must.Sprint(id))
	}

	cases := map[string]func(ts *Tombstone){
		"leading dot": func(ts *Tombstone) { ts.VolumeID = ".vol" },
		"id space":    func(ts *Tombstone) { ts.VolumeID = "a b" },
		"empty id":    func(ts *Tombstone) { ts.VolumeID = "" },
		"id slash":    func(ts *Tombstone) { ts.VolumeID = "a/b" },
		"id too long": func(ts *Tombstone) { ts.VolumeID = strings.Repeat("a", MaxIDLen+1) },
		"zero time":   func(ts *Tombstone) { ts.DeletedAt = time.Time{} },
	}
	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			ts := good
			mod(&ts)
			must.Error(t, ts.Validate())
		})
	}
	must.Eq(t, "volume_deleted", ReasonVolumeDeleted)
}

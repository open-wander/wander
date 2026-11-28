// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/shoenig/test/must"
)

// fillHostVolumeFixture sets every field of v, nested ones included, to a
// value that is not the zero value. Strings carry the path of their field, so
// two swapped fields cannot go unseen.
func fillHostVolumeFixture(v reflect.Value, path string) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(path)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(len(path)) + 1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(len(path)) + 1)
	case reflect.Ptr:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		fillHostVolumeFixture(v.Elem(), path)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 2, 2))
		for i := range v.Len() {
			fillHostVolumeFixture(v.Index(i), fmt.Sprintf("%s[%d]", path, i))
		}
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		key := reflect.New(v.Type().Key()).Elem()
		fillHostVolumeFixture(key, path+".key")
		val := reflect.New(v.Type().Elem()).Elem()
		fillHostVolumeFixture(val, path+".value")
		v.SetMapIndex(key, val)
	case reflect.Struct:
		for i := range v.NumField() {
			f := v.Type().Field(i)
			if !f.IsExported() {
				continue
			}
			fillHostVolumeFixture(v.Field(i), path+"."+f.Name)
		}
	default:
		panic("unsupported kind in fixture: " + v.Kind().String())
	}
}

// requireHostVolumeFilled fails on any zero value left in v, so a fixture that
// hides a field behind a zero value, a nil slice or a nil pointer is refused.
func requireHostVolumeFilled(t *testing.T, v reflect.Value, path string) {
	t.Helper()
	switch v.Kind() {
	case reflect.Ptr:
		must.False(t, v.IsNil(), must.Sprint(path))
		requireHostVolumeFilled(t, v.Elem(), path)
	case reflect.Slice:
		must.Positive(t, v.Len(), must.Sprint(path))
		for i := range v.Len() {
			requireHostVolumeFilled(t, v.Index(i), fmt.Sprintf("%s[%d]", path, i))
		}
	case reflect.Map:
		must.Positive(t, v.Len(), must.Sprint(path))
	case reflect.Struct:
		for i := range v.NumField() {
			if f := v.Type().Field(i); f.IsExported() {
				requireHostVolumeFilled(t, v.Field(i), path+"."+f.Name)
			}
		}
	default:
		must.False(t, v.IsZero(), must.Sprint(path))
	}
}

// roundTripHostVolume encodes fixture the way the agent writes a reply, decodes
// it into the api type, encodes that again and compares the two documents as
// generic maps. A field the api type renames, drops or adds shows up as a key
// that only one side has. exempt names top-level keys that are removed from
// the server document first.
func roundTripHostVolume(t *testing.T, fixture any, apiType any, exempt ...string) {
	t.Helper()
	requireHostVolumeFilled(t, reflect.ValueOf(fixture), "fixture")

	var buf bytes.Buffer
	must.NoError(t, codec.NewEncoder(&buf, structs.JsonHandleWithExtensions).Encode(fixture))

	var server map[string]any
	must.NoError(t, json.Unmarshal(buf.Bytes(), &server))
	for _, k := range exempt {
		delete(server, k)
	}

	decoded := reflect.New(reflect.TypeOf(apiType))
	must.NoError(t, json.Unmarshal(buf.Bytes(), decoded.Interface()))
	again, err := json.Marshal(decoded.Interface())
	must.NoError(t, err)
	var client map[string]any
	must.NoError(t, json.Unmarshal(again, &client))

	must.Eq(t, server, client)
}

// TestHostVolumeAPI_RoundTrip checks that the public api types carry every
// field the server returns, with the same names and values.
func TestHostVolumeAPI_RoundTrip(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name    string
		fixture any
		apiType any
		exempt  []string
	}{
		{"volume", &structs.DynamicHostVolume{}, api.HostVolume{}, nil},
		{"stub", &structs.DynamicHostVolumeStub{}, api.HostVolumeStub{}, nil},
		{"claim", &hostVolumeClaim{}, api.HostVolumeClaim{}, nil},
		{"orphan", &structs.HostVolumeOrphan{}, api.HostVolumeOrphan{}, nil},
		{"write reply", &hostVolumeWriteResponse{}, api.HostVolumeWriteResponse{}, nil},
		// Name is the key of the map in a node reply, so the api type has no
		// field for it.
		{"node entry", &structs.ClientHostVolumeConfig{}, api.HostVolumeInfo{}, []string{"Name"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fillHostVolumeFixture(reflect.ValueOf(tc.fixture), "fixture")
			roundTripHostVolume(t, tc.fixture, tc.apiType, tc.exempt...)
		})
	}
}

// TestHostVolumeAPI_RoundTrip_NoWarnings covers the one difference the filled
// fixtures cannot: the server writes no warning as null and the api type has an
// empty string.
func TestHostVolumeAPI_RoundTrip_NoWarnings(t *testing.T) {
	ci.Parallel(t)

	var buf bytes.Buffer
	must.NoError(t, codec.NewEncoder(&buf, structs.JsonHandleWithExtensions).Encode(
		&hostVolumeWriteResponse{Volume: &structs.DynamicHostVolume{ID: "vol-1"}}))
	must.StrContains(t, buf.String(), `"Warnings":null`)

	var got api.HostVolumeWriteResponse
	must.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	must.Eq(t, "", got.Warnings)
	must.Eq(t, "vol-1", got.Volume.ID)
}

// TestHostVolumeAPI_RoundTrip_Detects checks that the comparison can fail: an
// api type that lacks a field of the fixture is reported.
func TestHostVolumeAPI_RoundTrip_Detects(t *testing.T) {
	ci.Parallel(t)

	type stubWithoutState struct {
		ID   string
		Name string
	}
	fixture := &structs.DynamicHostVolumeStub{}
	fillHostVolumeFixture(reflect.ValueOf(fixture), "fixture")

	var buf bytes.Buffer
	must.NoError(t, codec.NewEncoder(&buf, structs.JsonHandleWithExtensions).Encode(fixture))
	var server map[string]any
	must.NoError(t, json.Unmarshal(buf.Bytes(), &server))

	var decoded stubWithoutState
	must.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	again, err := json.Marshal(decoded)
	must.NoError(t, err)
	var client map[string]any
	must.NoError(t, json.Unmarshal(again, &client))

	must.NotEq(t, server, client)
}

// TestHostVolumeAPI_Integration drives the real agent HTTP server through the
// typed client: register, list, read, update, claims, orphans and deregister.
func TestHostVolumeAPI_Integration(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		nodeID := s.client.NodeID()
		testutil.WaitForResult(func() (bool, error) {
			node, err := s.Agent.Server().State().NodeByID(nil, nodeID)
			if err != nil || node == nil {
				return false, fmt.Errorf("node not stored: %v", err)
			}
			return node.Status == structs.NodeStatusReady, fmt.Errorf("node status %q", node.Status)
		}, func(err error) { must.NoError(t, err) })

		hv := s.Client().HostVolumes()
		dir := t.TempDir()

		reg, wm, err := hv.Register(&api.HostVolumeRegisterRequest{Volume: &api.HostVolume{
			Name: "data-a", NodeID: nodeID, HostPath: dir,
			RequestedCapabilities: []*api.HostVolumeCapability{{
				AttachmentMode: api.HostVolumeAttachmentModeFilesystem,
				AccessMode:     api.HostVolumeAccessModeSingleNodeWriter,
			}},
		}}, nil)
		must.NoError(t, err)
		must.Positive(t, wm.LastIndex)
		must.Eq(t, api.HostVolumeStateReady, reg.Volume.State)
		must.Eq(t, api.HostVolumeOriginRegistered, reg.Volume.Origin)
		must.Eq(t, "", reg.Warnings)
		id := reg.Volume.ID
		must.NotEq(t, "", id)

		stubs, qm, err := hv.List(&api.HostVolumeListRequest{NodeID: nodeID}, nil)
		must.NoError(t, err)
		must.Positive(t, qm.LastIndex)
		must.Len(t, 1, stubs)
		must.Eq(t, id, stubs[0].ID)
		must.Eq(t, "data-a", stubs[0].Name)

		got, _, err := hv.Info(id, nil)
		must.NoError(t, err)
		must.Eq(t, dir, got.HostPath)
		must.Eq(t, "default", got.Namespace)

		// An update is a register of the stored volume with its ID and a change.
		got.Parameters = map[string]string{"k": "v"}
		upd, _, err := hv.Register(&api.HostVolumeRegisterRequest{Volume: got}, nil)
		must.NoError(t, err)
		must.Eq(t, id, upd.Volume.ID)
		must.Eq(t, map[string]string{"k": "v"}, upd.Volume.Parameters)
		must.Greater(t, got.ModifyIndex, upd.Volume.ModifyIndex)

		claims, qm, err := hv.ListClaims(nil, nil)
		must.NoError(t, err)
		must.Len(t, 0, claims)
		must.NotNil(t, qm)

		orphans, _, err := hv.ListOrphans("", &api.QueryOptions{Namespace: api.AllNamespacesNamespace})
		must.NoError(t, err)
		must.Len(t, 0, orphans)

		_, err = hv.Deregister(id, false, nil)
		must.NoError(t, err)
		testutil.WaitForResult(func() (bool, error) {
			stubs, _, err := hv.List(nil, nil)
			if err != nil {
				return false, err
			}
			return len(stubs) == 0, fmt.Errorf("%d volumes remain", len(stubs))
		}, func(err error) { must.NoError(t, err) })
	})
}

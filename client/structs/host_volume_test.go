// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"testing"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/open-wander/wander/ci"
	nstructs "github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestHostVolumeRegister_MsgpackRoundTrip(t *testing.T) {
	ci.Parallel(t)

	var buf []byte
	req := HostVolumeRegisterRequest{VolumeID: "v", Namespace: "ns", Name: "n", NodeID: "node", NodePool: "p", HostPath: "/synthetic/p"}
	must.NoError(t, codec.NewEncoderBytes(&buf, nstructs.MsgpackHandle).Encode(req))
	var gotReq HostVolumeRegisterRequest
	must.NoError(t, codec.NewDecoderBytes(buf, nstructs.MsgpackHandle).Decode(&gotReq))
	must.Eq(t, req, gotReq)

	resp := HostVolumeRegisterResponse{VolumeID: "v", HostPath: "/synthetic/p", Status: "ready", Reason: "r", Message: "m"}
	buf = nil
	must.NoError(t, codec.NewEncoderBytes(&buf, nstructs.MsgpackHandle).Encode(resp))
	var gotResp HostVolumeRegisterResponse
	must.NoError(t, codec.NewDecoderBytes(buf, nstructs.MsgpackHandle).Decode(&gotResp))
	must.Eq(t, resp, gotResp)
}

func TestHostVolumeCreate_MsgpackRoundTrip(t *testing.T) {
	ci.Parallel(t)

	var buf []byte
	req := HostVolumeCreateRequest{
		VolumeID: "v", Namespace: "ns", Name: "n", NodeID: "node", NodePool: "p", PluginID: "plug",
		CapacityMinBytes: 1, CapacityMaxBytes: 2, Parameters: map[string]string{"k": "v"},
	}
	must.NoError(t, codec.NewEncoderBytes(&buf, nstructs.MsgpackHandle).Encode(req))
	var gotReq HostVolumeCreateRequest
	must.NoError(t, codec.NewDecoderBytes(buf, nstructs.MsgpackHandle).Decode(&gotReq))
	must.Eq(t, req, gotReq)

	resp := HostVolumeCreateResponse{VolumeID: "v", HostPath: "/synthetic/p", CapacityBytes: 7, Status: "ready", Reason: "r", Message: "m"}
	buf = nil
	must.NoError(t, codec.NewEncoderBytes(&buf, nstructs.MsgpackHandle).Encode(resp))
	var gotResp HostVolumeCreateResponse
	must.NoError(t, codec.NewDecoderBytes(buf, nstructs.MsgpackHandle).Decode(&gotResp))
	must.Eq(t, resp, gotResp)
}

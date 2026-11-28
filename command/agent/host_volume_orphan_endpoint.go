// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"net/http"
	"strings"

	"github.com/open-wander/wander/nomad/structs"
)

// hostVolumeOrphansList serves GET /v1/volumes/host/orphans: the dynamic
// volumes that nodes advertise and the servers hold no record of. node_id and
// namespace narrow the list; namespace=* lists every namespace the token may
// read. It is Wander policy.
func (s *HTTPServer) hostVolumeOrphansList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.HostVolumeOrphanListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}
	args.NodeID = req.URL.Query().Get("node_id")

	var out structs.HostVolumeOrphanListResponse
	if err := s.agent.RPC("HostVolume.ListOrphans", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Orphans == nil {
		out.Orphans = make([]*structs.HostVolumeOrphan, 0)
	}
	return out.Orphans, nil
}

// HostVolumeOrphanSpecificRequest serves
// DELETE /v1/volume/host/orphan/:node_id/:volume_id?mode=delete|forget, which
// removes an orphaned volume from its node. The mode is required and has no
// default. The namespace is the volume's namespace and defaults to "default"
// as everywhere in the API. It is Wander policy.
func (s *HTTPServer) HostVolumeOrphanSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	tokens := strings.Split(strings.TrimPrefix(req.URL.Path, "/v1/volume/host/orphan/"), "/")
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		return nil, CodedError(404, resourceNotFoundErr)
	}
	if req.Method != http.MethodDelete {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.HostVolumeOrphanRemoveRequest{
		NodeID: tokens[0], VolumeID: tokens[1], Mode: req.URL.Query().Get("mode"),
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.HostVolumeOrphanRemoveResponse
	if err := s.agent.RPC("HostVolume.RemoveOrphan", &args, &out); err != nil {
		return nil, err
	}
	return nil, nil
}

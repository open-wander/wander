// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"net/http"
	"strings"

	"github.com/open-wander/wander/nomad/structs"
)

// hostVolumesList serves GET /v1/volumes?type=host.
func (s *HTTPServer) hostVolumesList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.HostVolumeListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	query := req.URL.Query()
	args.NodeID = query.Get("node_id")
	args.NodePool = query.Get("node_pool")

	var out structs.HostVolumeListResponse
	if err := s.agent.RPC("HostVolume.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Volumes == nil {
		out.Volumes = make([]*structs.DynamicHostVolumeStub, 0)
	}
	return out.Volumes, nil
}

// HostVolumeSpecificRequest serves /v1/volume/host/:volume_id.
func (s *HTTPServer) HostVolumeSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/volume/host/")
	tokens := strings.Split(reqSuffix, "/")
	if len(tokens) != 1 || tokens[0] == "" {
		return nil, CodedError(404, resourceNotFoundErr)
	}

	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	return s.hostVolumeGet(tokens[0], resp, req)
}

func (s *HTTPServer) hostVolumeGet(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.HostVolumeGetRequest{ID: id}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.HostVolumeGetResponse
	if err := s.agent.RPC("HostVolume.Get", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Volume == nil {
		return nil, CodedError(404, "volume not found")
	}
	return out.Volume, nil
}

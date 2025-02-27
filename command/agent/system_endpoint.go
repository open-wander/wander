// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"net/http"

	"github.com/open-wander/wander/nomad/structs"
)

func (s *HTTPServer) GarbageCollectRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodPut {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var args structs.GenericRequest
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var gResp structs.GenericResponse
	if err := s.agent.RPC("System.GarbageCollect", &args, &gResp); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *HTTPServer) ReconcileJobSummaries(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodPut {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var args structs.GenericRequest
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var gResp structs.GenericResponse
	if err := s.agent.RPC("System.ReconcileJobSummaries", &args, &gResp); err != nil {
		return nil, err
	}
	return nil, nil
}

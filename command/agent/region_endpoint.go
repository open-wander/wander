// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"net/http"

	"github.com/open-wander/wander/nomad/structs"
)

func (s *HTTPServer) RegionListRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var args structs.GenericRequest
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var regions []string
	if err := s.agent.RPC("Region.List", &args, &regions); err != nil {
		return nil, err
	}
	return regions, nil
}

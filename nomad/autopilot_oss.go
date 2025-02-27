// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package nomad

import (
	autopilot "github.com/hashicorp/raft-autopilot"

	"github.com/open-wander/wander/nomad/structs"
)

func (s *Server) autopilotPromoter() autopilot.Promoter {
	return autopilot.DefaultPromoter()
}

// autopilotServerExt returns the autopilot-enterprise.Server extensions needed
// for ENT feature support, but this is the empty OSS implementation.
func (s *Server) autopilotServerExt(_ *serverParts) interface{} {
	return nil
}

// autopilotConfigExt returns the autopilot-enterprise.Config extensions needed
// for ENT feature support, but this is the empty OSS implementation.
func autopilotConfigExt(_ *structs.AutopilotConfig) interface{} {
	return nil
}

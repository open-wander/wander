// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-version"
	"github.com/open-wander/wander/nomad/structs"
)

// minWanderVersionDynamicHostVolumes is the first Wander version that knows the
// dynamic host volume Raft commands.
var minWanderVersionDynamicHostVolumes = version.Must(version.NewVersion("0.1.2"))

// ErrDynamicHostVolumeVersion is returned when a server of the region cannot
// apply the dynamic host volume commands yet.
var ErrDynamicHostVolumeVersion = errors.New("dynamic host volume commands refused")

func isDynamicHostVolumeMessage(t structs.MessageType) bool {
	switch t {
	case structs.DynamicHostVolumeUpsertRequestType,
		structs.DynamicHostVolumeOutcomeRequestType,
		structs.DynamicHostVolumeUpdateRequestType,
		structs.DynamicHostVolumeMarkDeletingRequestType,
		structs.DynamicHostVolumeRemoveRequestType:
		return true
	}
	return false
}

// dynamicHostVolumeRaftApply submits a dynamic host volume command to Raft once
// every server of the region advertises Wander 0.1.2 or later. A server that
// does not know the command would stop in Apply, or skip it and diverge, so the
// commands never reach the log while one is present. The check is made at
// submission: a server that joins between the check and the commit can still
// receive the entry, a window that is accepted. RPCs submit these commands only
// through this method.
func (s *Server) dynamicHostVolumeRaftApply(t structs.MessageType, msg any) (any, uint64, error) {
	if !isDynamicHostVolumeMessage(t) {
		return nil, 0, fmt.Errorf("message type %d is not a dynamic host volume command", t)
	}
	if !ServersMeetMinimumWanderVersion(s.serf.Members(), s.Region(), minWanderVersionDynamicHostVolumes) {
		return nil, 0, fmt.Errorf("%w: all servers in region %q must be running Wander %s or later",
			ErrDynamicHostVolumeVersion, s.Region(), minWanderVersionDynamicHostVolumes)
	}
	return s.raftApply(t, msg)
}

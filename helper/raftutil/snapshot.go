// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package raftutil

import (
	"fmt"
	"io"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"

	"github.com/open-wander/wander/helper/snapshot"
	"github.com/open-wander/wander/nomad"
	"github.com/open-wander/wander/nomad/state"
)

func RestoreFromArchive(archive io.Reader, filter *nomad.FSMFilter) (*state.StateStore, *raft.SnapshotMeta, error) {
	logger := hclog.L()

	fsm, err := dummyFSM(logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create FSM: %w", err)
	}

	// r is closed by RestoreFiltered, w is closed by CopySnapshot
	r, w := io.Pipe()

	errCh := make(chan error)
	metaCh := make(chan *raft.SnapshotMeta)

	go func() {
		meta, err := snapshot.CopySnapshot(archive, w)
		if err != nil {
			errCh <- fmt.Errorf("failed to read snapshot: %w", err)
		} else {
			metaCh <- meta
		}
	}()

	err = fsm.RestoreWithFilter(r, filter)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	select {
	case err := <-errCh:
		return nil, nil, err
	case meta := <-metaCh:
		return fsm.State(), meta, nil
	}
}

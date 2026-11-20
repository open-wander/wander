// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

// -------------------------------------------------------------------------------
// Variable Replication Tests
//
// Project: Wander
//
// A variable written through one server is readable on every server with stale
// reads allowed, so each server answers from its own copy of the Raft state
// instead of forwarding the read to the leader.
// -------------------------------------------------------------------------------

//go:build integration

package integration

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/shoenig/test/must"

	"github.com/open-wander/wander/api"
)

// replicationTimeout bounds how long a follower may take to apply a write.
const replicationTimeout = 30 * time.Second

// TestVariables_Replicated proves a variable written through wander-server-1
// reaches every server's local state.
func TestVariables_Replicated(t *testing.T) {
	now := time.Now().UnixNano()
	path := fmt.Sprintf("integration/variables-replicated-%d", now)
	marker := strconv.FormatInt(now, 36)

	writer := servers["wander-server-1"]
	_, _, err := writer.Variables().Create(&api.Variable{
		Path:  path,
		Items: api.VariableItems{"marker": marker},
	}, nil)
	must.NoError(t, err)
	t.Cleanup(func() {
		if _, err := writer.Variables().Delete(path, nil); err != nil {
			t.Logf("delete %s: %v", path, err)
		}
	})

	for _, name := range serverNames {
		t.Run(name, func(t *testing.T) {
			eventually(t, replicationTimeout, func() error {
				v, _, err := servers[name].Variables().Read(path, &api.QueryOptions{AllowStale: true})
				if err != nil {
					return err
				}
				if got := v.Items["marker"]; got != marker {
					return fmt.Errorf("marker is %q, want %q", got, marker)
				}
				return nil
			})
		})
	}
}

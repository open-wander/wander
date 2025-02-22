// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nsd

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/serviceregistration"
	"github.com/open-wander/wander/client/serviceregistration/checks/checkstore"
	"github.com/open-wander/wander/client/state"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

var _ serviceregistration.CheckStatusGetter = (*StatusGetter)(nil)

func TestStatusGetter_Get(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	db := state.NewMemDB(logger)
	s := checkstore.NewStore(logger, db)

	// setup some sample check results
	id1, id2, id3 := uuid.Short(), uuid.Short(), uuid.Short()
	must.NoError(t, s.Set("allocation1", &structs.CheckQueryResult{
		ID:     structs.CheckID(id1),
		Status: "passing",
	}))
	must.NoError(t, s.Set("allocation1", &structs.CheckQueryResult{
		ID:     structs.CheckID(id2),
		Status: "failing",
	}))
	must.NoError(t, s.Set("allocation2", &structs.CheckQueryResult{
		ID:     structs.CheckID(id3),
		Status: "passing",
	}))

	getter := StatusGetter{shim: s}
	snap, err := getter.Get()
	must.NoError(t, err)
	must.MapEq(t, map[string]string{
		id1: "passing",
		id2: "failing",
		id3: "passing",
	}, snap)
}

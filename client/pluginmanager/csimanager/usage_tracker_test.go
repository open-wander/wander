// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package csimanager

import (
	"testing"

	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestUsageTracker(t *testing.T) {
	mockAllocs := []*structs.Allocation{
		mock.Alloc(),
		mock.Alloc(),
		mock.Alloc(),
		mock.Alloc(),
		mock.Alloc(),
	}

	cases := []struct {
		Name string

		RegisterAllocs []*structs.Allocation
		FreeAllocs     []*structs.Allocation

		ExpectedResult bool
	}{
		{
			Name:           "Register and deregister all allocs",
			RegisterAllocs: mockAllocs,
			FreeAllocs:     mockAllocs,
			ExpectedResult: true,
		},
		{
			Name:           "Register all and deregister partial allocs",
			RegisterAllocs: mockAllocs,
			FreeAllocs:     mockAllocs[0:3],
			ExpectedResult: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tracker := newVolumeUsageTracker()

			volume := &structs.CSIVolume{
				ID: "foo",
			}
			for _, alloc := range tc.RegisterAllocs {
				tracker.Claim(alloc.ID, volume.ID, &UsageOptions{})
			}

			result := false

			for _, alloc := range tc.FreeAllocs {
				result = tracker.Free(alloc.ID, volume.ID, &UsageOptions{})
			}

			require.Equal(t, tc.ExpectedResult, result, "Tracker State: %#v", tracker.state)
		})
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"errors"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestSI_DeriveTokens(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	dFunc := func(alloc *structs.Allocation, taskNames []string) (map[string]string, error) {
		return map[string]string{"a": "b"}, nil
	}
	tc := NewIdentitiesClient(logger, dFunc)
	tokens, err := tc.DeriveSITokens(nil, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"a": "b"}, tokens)
}

func TestSI_DeriveTokens_error(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	dFunc := func(alloc *structs.Allocation, taskNames []string) (map[string]string, error) {
		return nil, errors.New("some failure")
	}
	tc := NewIdentitiesClient(logger, dFunc)
	_, err := tc.DeriveSITokens(&structs.Allocation{ID: "a1"}, nil)
	require.Error(t, err)
}

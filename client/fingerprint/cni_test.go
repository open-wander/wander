// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package fingerprint

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/stretchr/testify/require"
)

// Test that CNI fingerprinter is reloadable
var _ ReloadableFingerprint = &CNIFingerprint{}

func TestCNIFingerprint(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		req      *FingerprintRequest
		exp      *FingerprintResponse
		err      bool
		errMatch string
	}{
		{
			name: "cni config dir not set",
			req: &FingerprintRequest{
				Config: &config.Config{},
			},
			exp: &FingerprintResponse{
				Detected: false,
			},
		},
		{
			name: "cni config dir non-existent",
			req: &FingerprintRequest{
				Config: &config.Config{
					CNIConfigDir: "text_fixtures/cni_nonexistent",
				},
			},
			exp: &FingerprintResponse{
				Detected: false,
			},
		},
		{
			name: "two networks, no errors",
			req: &FingerprintRequest{
				Config: &config.Config{
					CNIConfigDir: "test_fixtures/cni",
				},
			},
			exp: &FingerprintResponse{
				NodeResources: &structs.NodeResources{
					Networks: []*structs.NetworkResource{
						{
							Mode: "cni/net1",
						},
						{
							Mode: "cni/net2",
						},
					},
				},
				Detected: true,
			},
			err: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)
			fp := NewCNIFingerprint(testlog.HCLogger(t))
			resp := &FingerprintResponse{}
			err := fp.Fingerprint(c.req, resp)
			if c.err {
				r.Error(err)
				r.Contains(err.Error(), c.errMatch)
			} else {
				r.NoError(err)
				r.Equal(c.exp.Detected, resp.Detected)
				if resp.NodeResources != nil || c.exp.NodeResources != nil {
					r.ElementsMatch(c.exp.NodeResources.Networks, resp.NodeResources.Networks)
				}
			}
		})
	}
}

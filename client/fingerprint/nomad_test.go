// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package fingerprint

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/version"
	"github.com/stretchr/testify/require"
)

func TestNomadFingerprint(t *testing.T) {
	ci.Parallel(t)

	f := NewNomadFingerprint(testlog.HCLogger(t))

	v := "foo"
	r := "123"
	h := "8.8.8.8:4646"
	c := &config.Config{
		Version: &version.VersionInfo{
			Revision: r,
			Version:  v,
		},
		NomadServiceDiscovery: true,
	}
	node := &structs.Node{
		Attributes: make(map[string]string),
		HTTPAddr:   h,
	}

	request := &FingerprintRequest{Config: c, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	if len(response.Attributes) == 0 {
		t.Fatalf("should apply")
	}

	if response.Attributes["nomad.version"] != v {
		t.Fatalf("incorrect version")
	}

	if response.Attributes["nomad.revision"] != r {
		t.Fatalf("incorrect revision")
	}

	if response.Attributes["nomad.advertise.address"] != h {
		t.Fatalf("incorrect advertise address")
	}

	serviceDisco := response.Attributes["nomad.service_discovery"]
	require.Equal(t, "true", serviceDisco, "service_discovery attr incorrect")
}

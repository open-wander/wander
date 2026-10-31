// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package fingerprint

import (
	"strconv"

	log "github.com/hashicorp/go-hclog"

	"github.com/open-wander/wander/version"
)

// NomadFingerprint is used to fingerprint the Nomad version
type NomadFingerprint struct {
	StaticFingerprinter
	logger log.Logger
}

// NewNomadFingerprint is used to create a Nomad fingerprint
func NewNomadFingerprint(logger log.Logger) Fingerprint {
	f := &NomadFingerprint{logger: logger.Named("nomad")}
	return f
}

func (f *NomadFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	resp.AddAttribute("nomad.advertise.address", req.Node.HTTPAddr)
	// nomad.version is a protocol compatibility attribute, not the Wander
	// product version. Keeping those values separate lets Wander use its own
	// release sequence without disabling legacy feature gates.
	resp.AddAttribute("nomad.version", version.ClusterCompatibilityVersion)
	resp.AddAttribute("nomad.revision", req.Config.Version.Revision)
	resp.AddAttribute("nomad.service_discovery", strconv.FormatBool(req.Config.NomadServiceDiscovery))
	resp.Detected = true
	return nil
}

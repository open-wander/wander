// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package volumewatcher

import (
	"github.com/open-wander/wander/nomad/structs"
)

// CSIVolumeRPC is a minimal interface of the Server, intended as an aid
// for testing logic surrounding server-to-server or server-to-client
// RPC calls and to avoid circular references between the nomad
// package and the volumewatcher
type CSIVolumeRPC interface {
	Unpublish(args *structs.CSIVolumeUnpublishRequest, reply *structs.CSIVolumeUnpublishResponse) error
}

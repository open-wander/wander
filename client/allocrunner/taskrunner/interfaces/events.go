// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package interfaces

import "github.com/open-wander/wander/nomad/structs"

type EventEmitter interface {
	EmitEvent(event *structs.TaskEvent)
}

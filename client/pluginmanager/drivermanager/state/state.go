// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import pstructs "github.com/open-wander/wander/plugins/shared/structs"

// PluginState is used to store the driver manager's state across restarts of the
// agent
type PluginState struct {
	// ReattachConfigs are the set of reattach configs for plugins launched by
	// the driver manager
	ReattachConfigs map[string]*pstructs.ReattachConfig
}

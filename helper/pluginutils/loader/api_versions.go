// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package loader

import (
	"github.com/open-wander/wander/plugins/base"
	"github.com/open-wander/wander/plugins/device"
	"github.com/open-wander/wander/plugins/drivers"
)

var (
	// AgentSupportedApiVersions is the set of API versions supported by the
	// Nomad agent by plugin type.
	AgentSupportedApiVersions = map[string][]string{
		base.PluginTypeDevice: {device.ApiVersion010},
		base.PluginTypeDriver: {drivers.ApiVersion010},
	}
)

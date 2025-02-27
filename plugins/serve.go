// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugins

import (
	"context"
	"fmt"

	log "github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/plugins/device"
	"github.com/open-wander/wander/plugins/drivers"
)

// PluginFactory returns a new plugin instance
type PluginFactory func(log log.Logger) interface{}

// PluginCtxFactory returns a new plugin instance, that takes in a context
type PluginCtxFactory func(ctx context.Context, log log.Logger) interface{}

// Serve is used to serve a new Nomad plugin
func Serve(f PluginFactory) {
	logger := log.New(&log.LoggerOptions{
		Level:      log.Trace,
		JSONFormat: true,
	})

	plugin := f(logger)
	serve(plugin, logger)
}

// ServeCtx is used to serve a new Nomad plugin
func ServeCtx(f PluginCtxFactory) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.New(&log.LoggerOptions{
		Level:      log.Trace,
		JSONFormat: true,
	})

	plugin := f(ctx, logger)
	serve(plugin, logger)
}
func serve(plugin interface{}, logger log.Logger) {
	switch p := plugin.(type) {
	case device.DevicePlugin:
		device.Serve(p, logger)
	case drivers.DriverPlugin:
		drivers.Serve(p, logger)
	default:
		fmt.Println("Unsupported plugin type")
	}
}

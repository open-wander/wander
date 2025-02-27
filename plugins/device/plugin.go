// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package device

import (
	"context"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/open-wander/wander/plugins/base"
	bproto "github.com/open-wander/wander/plugins/base/proto"
	"github.com/open-wander/wander/plugins/device/proto"
	"google.golang.org/grpc"
)

// PluginDevice is wraps a DevicePlugin and implements go-plugins GRPCPlugin
// interface to expose the interface over gRPC.
type PluginDevice struct {
	plugin.NetRPCUnsupportedPlugin
	Impl DevicePlugin
}

func (p *PluginDevice) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterDevicePluginServer(s, &devicePluginServer{
		impl:   p.Impl,
		broker: broker,
	})
	return nil
}

func (p *PluginDevice) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &devicePluginClient{
		doneCtx: ctx,
		client:  proto.NewDevicePluginClient(c),
		BasePluginClient: &base.BasePluginClient{
			Client:  bproto.NewBasePluginClient(c),
			DoneCtx: ctx,
		},
	}, nil
}

// Serve is used to serve a device plugin
func Serve(dev DevicePlugin, logger log.Logger) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: base.Handshake,
		Plugins: map[string]plugin.Plugin{
			base.PluginTypeBase:   &base.PluginBase{Impl: dev},
			base.PluginTypeDevice: &PluginDevice{Impl: dev},
		},
		GRPCServer: plugin.DefaultGRPCServer,
		Logger:     logger,
	})
}

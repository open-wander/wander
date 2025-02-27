// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package logmon

import (
	"context"
	"time"

	"github.com/open-wander/wander/client/logmon/proto"
	"github.com/open-wander/wander/helper/pluginutils/grpcutils"
)

type logmonClient struct {
	client proto.LogMonClient

	// doneCtx is closed when the plugin exits
	doneCtx context.Context
}

const logmonRPCTimeout = 1 * time.Minute

func (c *logmonClient) Start(cfg *LogConfig) error {
	req := &proto.StartRequest{
		LogDir:         cfg.LogDir,
		StdoutFileName: cfg.StdoutLogFile,
		StderrFileName: cfg.StderrLogFile,
		MaxFiles:       uint32(cfg.MaxFiles),
		MaxFileSizeMb:  uint32(cfg.MaxFileSizeMB),
		StdoutFifo:     cfg.StdoutFifo,
		StderrFifo:     cfg.StderrFifo,
	}
	ctx, cancel := context.WithTimeout(context.Background(), logmonRPCTimeout)
	defer cancel()

	_, err := c.client.Start(ctx, req)
	return grpcutils.HandleGrpcErr(err, c.doneCtx)
}

func (c *logmonClient) Stop() error {
	req := &proto.StopRequest{}
	ctx, cancel := context.WithTimeout(context.Background(), logmonRPCTimeout)
	defer cancel()

	_, err := c.client.Stop(ctx, req)
	return grpcutils.HandleGrpcErr(err, c.doneCtx)
}

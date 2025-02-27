// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"fmt"

	"github.com/hashicorp/go-hclog"

	"github.com/open-wander/wander/nomad/structs"
)

// System endpoint is used to call invoke system tasks.
type System struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewSystemEndpoint(srv *Server, ctx *RPCContext) *System {
	return &System{srv: srv, ctx: ctx, logger: srv.logger.Named("system")}
}

// GarbageCollect is used to trigger the system to immediately garbage collect nodes, evals
// and jobs.
func (s *System) GarbageCollect(args *structs.GenericRequest, reply *structs.GenericResponse) error {

	authErr := s.srv.Authenticate(s.ctx, args)
	if done, err := s.srv.forward("System.GarbageCollect", args, args, reply); done {
		return err
	}
	s.srv.MeasureRPCRate("system", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	// Check management level permissions
	if acl, err := s.srv.ResolveACL(args); err != nil {
		return err
	} else if acl != nil && !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Get the states current index
	snapshotIndex, err := s.srv.fsm.State().LatestIndex()
	if err != nil {
		return fmt.Errorf("failed to determine state store's index: %v", err)
	}

	s.srv.evalBroker.Enqueue(s.srv.coreJobEval(structs.CoreJobForceGC, snapshotIndex))
	return nil
}

// ReconcileJobSummaries reconciles the summaries of all the jobs in the state
// store
func (s *System) ReconcileJobSummaries(args *structs.GenericRequest, reply *structs.GenericResponse) error {

	authErr := s.srv.Authenticate(s.ctx, args)
	if done, err := s.srv.forward("System.ReconcileJobSummaries", args, args, reply); done {
		return err
	}
	s.srv.MeasureRPCRate("system", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	// Check management level permissions
	if acl, err := s.srv.ResolveACL(args); err != nil {
		return err
	} else if acl != nil && !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	_, index, err := s.srv.raftApply(structs.ReconcileJobSummariesRequestType, args)
	if err != nil {
		return fmt.Errorf("reconciliation of job summaries failed: %v", err)
	}
	reply.Index = index
	return nil
}

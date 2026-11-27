// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/state/paginator"
	"github.com/open-wander/wander/nomad/structs"
)

// HostVolume is the server endpoint for dynamic host volumes. It is separate
// from the client's HostVolume endpoint, which lives in the client RPC registry.
type HostVolume struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewHostVolumeEndpoint(srv *Server, ctx *RPCContext) *HostVolume {
	return &HostVolume{srv: srv, ctx: ctx, logger: srv.logger.Named("host_volume")}
}

// replySetIndex reports the dynamic host volumes table index. The
// claims table is deliberately not part of it. An index of zero means the table
// has never been written; it is reported as 1 so that blocking clients do not
// spin on an index of zero.
func (v *HostVolume) replySetIndex(reply *structs.QueryMeta) error {
	if err := v.srv.replySetIndex(state.TableDynamicHostVolumes, reply); err != nil {
		return err
	}
	if reply.Index < 1 {
		reply.Index = 1
	}
	return nil
}

// hostVolumeStateErr maps a rejected key to a fixed coded error. Errors never
// carry record or key values.
func hostVolumeStateErr(err error, msg string) error {
	if errors.Is(err, state.ErrDynamicHostVolumeStateInvalid) {
		return structs.NewErrRPCCoded(http.StatusBadRequest, msg)
	}
	return err
}

// List returns the dynamic host volumes the token may read. Reading requires
// host-volume-read in the namespace; host-volume-create, -register and -write
// imply it. A wildcard namespace returns only volumes in namespaces where the
// token holds that capability.
func (v *HostVolume) List(args *structs.HostVolumeListRequest, reply *structs.HostVolumeListResponse) error {
	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.List", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	ns := args.RequestNamespace()
	if !acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeRead)(aclObj, ns) {
		return structs.ErrPermissionDenied
	}
	allow := aclObj.AllowNsOpFunc(acl.NamespaceCapabilityHostVolumeRead)

	defer metrics.MeasureSince([]string{"nomad", "host_volume", "list"}, time.Now())

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			snap, err := store.Snapshot()
			if err != nil {
				return err
			}

			allowable, err := allowedNSes(aclObj, store, allow)
			if errors.Is(err, structs.ErrPermissionDenied) {
				// No readable namespace exists: reply with an empty list.
				reply.Volumes = []*structs.DynamicHostVolumeStub{}
				return v.replySetIndex(&reply.QueryMeta)
			} else if err != nil {
				return err
			}

			// Always iterate the ID index so that the order matches the
			// namespace and ID page token. The state helpers treat a namespace
			// as a literal key, so a wildcard cannot be passed to them.
			prefix := args.Prefix
			wildcard := ns == structs.AllNamespacesSentinel
			var iter memdb.ResultIterator
			switch {
			case wildcard:
				iter, err = snap.DynamicHostVolumes(ws)
			case prefix != "":
				iter, err = snap.DynamicHostVolumesByIDPrefix(ws, ns, prefix)
			default:
				iter, err = snap.DynamicHostVolumesByNamespace(ws, ns)
			}
			if err != nil {
				return hostVolumeStateErr(err, "invalid namespace or prefix")
			}

			tokenizer := paginator.NewStructsTokenizer(iter, paginator.StructsTokenizerOptions{
				WithNamespace: true,
				WithID:        true,
			})

			// The namespace filter comes first so that neither the other
			// filters nor a filter expression ever sees a volume the token
			// cannot read.
			filters := []paginator.Filter{
				paginator.NamespaceFilter{AllowableNamespaces: allowable},
				paginator.GenericFilter{Allow: func(raw interface{}) (bool, error) {
					vol := raw.(*structs.DynamicHostVolume)
					if wildcard && prefix != "" && !strings.HasPrefix(vol.ID, prefix) {
						return false, nil
					}
					if args.NodeID != "" && vol.NodeID != args.NodeID {
						return false, nil
					}
					if args.NodePool != "" && vol.NodePool != args.NodePool {
						return false, nil
					}
					return true, nil
				}},
			}

			stubs := []*structs.DynamicHostVolumeStub{}
			pager, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
				func(raw interface{}) error {
					stubs = append(stubs, raw.(*structs.DynamicHostVolume).Stub())
					return nil
				})
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to create result paginator: %v", err)
			}
			nextToken, err := pager.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to read result page: %v", err)
			}

			reply.QueryMeta.NextToken = nextToken
			reply.Volumes = stubs
			return v.replySetIndex(&reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}

// Get returns one dynamic host volume, including its host path, in the request
// namespace. A volume that does not exist is returned as nil.
func (v *HostVolume) Get(args *structs.HostVolumeGetRequest, reply *structs.HostVolumeGetResponse) error {
	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.Get", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	ns := args.RequestNamespace()
	if !acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeRead)(aclObj, ns) {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "host_volume", "get"}, time.Now())

	if args.ID == "" {
		return structs.NewErrRPCCoded(http.StatusBadRequest, "missing volume ID")
	}
	if ns == structs.AllNamespacesSentinel {
		return structs.NewErrRPCCoded(http.StatusBadRequest, "a volume read needs a single namespace")
	}

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			snap, err := store.Snapshot()
			if err != nil {
				return err
			}
			vol, err := snap.DynamicHostVolumeByID(ws, ns, args.ID)
			if err != nil {
				return hostVolumeStateErr(err, "invalid volume ID")
			}
			reply.Volume = vol
			return v.replySetIndex(&reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}

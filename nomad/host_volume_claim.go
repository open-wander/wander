// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-memdb"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/state/paginator"
	"github.com/open-wander/wander/nomad/structs"
)

const (
	hostVolumeErrClaimNotFound     = "claim not found"
	hostVolumeErrClaimInUse        = "claim in use"
	hostVolumeErrClaimSingleNS     = "a claim delete needs a single namespace"
	hostVolumeErrClaimMissingID    = "missing claim ID"
	hostVolumeErrClaimInvalidID    = "invalid claim ID"
	hostVolumeErrClaimInvalidQuery = "invalid namespace or prefix"
)

// replySetClaimsIndex reports the claims table index. The volumes table is
// deliberately not part of it: a claim list must wake on a claim delete, which
// does not touch the volumes table. An index of zero means the table has never
// been written; it is reported as 1 so that blocking clients do not spin.
func (v *HostVolume) replySetClaimsIndex(reply *structs.QueryMeta) error {
	if err := v.srv.replySetIndex(state.TableDynamicHostVolumeClaims, reply); err != nil {
		return err
	}
	if reply.Index < 1 {
		reply.Index = 1
	}
	return nil
}

// ListClaims returns the sticky claims the token may read. Reading requires
// host-volume-read in the namespace, as for volumes. A wildcard namespace
// returns only claims in namespaces where the token holds that capability.
func (v *HostVolume) ListClaims(args *structs.HostVolumeClaimListRequest, reply *structs.HostVolumeClaimListResponse) error {
	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.ListClaims", args, args, reply); done {
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

	defer metrics.MeasureSince([]string{"nomad", "host_volume", "list_claims"}, time.Now())

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
				reply.Claims = []*structs.DynamicHostVolumeClaim{}
				return v.replySetClaimsIndex(&reply.QueryMeta)
			} else if err != nil {
				return err
			}

			// Always iterate the ID index so that the order matches the
			// namespace and ID page token, whatever the filters are. The
			// state helpers treat a namespace as a literal key, so a
			// wildcard cannot be passed to them.
			prefix := args.Prefix
			wildcard := ns == structs.AllNamespacesSentinel
			var iter memdb.ResultIterator
			switch {
			case wildcard:
				iter, err = snap.DynamicHostVolumeClaims(ws)
			case prefix != "":
				iter, err = snap.DynamicHostVolumeClaimsByIDPrefix(ws, ns, prefix)
			default:
				iter, err = snap.DynamicHostVolumeClaimsByNamespace(ws, ns)
			}
			if err != nil {
				return hostVolumeStateErr(err, hostVolumeErrClaimInvalidQuery)
			}

			tokenizer := paginator.NewStructsTokenizer(iter, paginator.StructsTokenizerOptions{
				WithNamespace: true,
				WithID:        true,
			})

			// The namespace filter comes first so that no other filter
			// ever sees a claim the token cannot read.
			filters := []paginator.Filter{
				paginator.NamespaceFilter{AllowableNamespaces: allowable},
				paginator.GenericFilter{Allow: func(raw interface{}) (bool, error) {
					claim := raw.(*structs.DynamicHostVolumeClaim)
					if wildcard && prefix != "" && !strings.HasPrefix(claim.ID, prefix) {
						return false, nil
					}
					if args.JobID != "" && claim.JobID != args.JobID {
						return false, nil
					}
					if args.TaskGroup != "" && claim.TaskGroupName != args.TaskGroup {
						return false, nil
					}
					if args.VolumeName != "" && claim.VolumeName != args.VolumeName {
						return false, nil
					}
					return true, nil
				}},
			}

			claims := []*structs.DynamicHostVolumeClaim{}
			pager, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
				func(raw interface{}) error {
					claims = append(claims, raw.(*structs.DynamicHostVolumeClaim))
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
			reply.Claims = claims
			return v.replySetClaimsIndex(&reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}

// DeleteClaim removes one sticky claim in the request namespace. It needs
// host-volume-write there. A claim that an allocation still holds is refused
// with 409; the state store enforces that when the entry is applied, and the
// check here only gives the refusal without a log entry. A claim that does not
// exist is 404: the permission check comes first, so the answer reveals nothing
// about other namespaces.
func (v *HostVolume) DeleteClaim(args *structs.HostVolumeClaimDeleteRequest, reply *structs.HostVolumeClaimDeleteResponse) (retErr error) {
	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.DeleteClaim", args, args, reply); done {
		return err
	}
	audit := &hostVolumeAudit{op: hostVolumeAuditDeleteClaim, ns: args.RequestNamespace(), claimID: args.ID}
	defer func() { v.srv.auditHostVolumeCall(args, authErr, audit, retErr, nil) }()
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	ns := args.RequestNamespace()
	if !acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeWrite)(aclObj, ns) {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "host_volume", "delete_claim"}, time.Now())

	if ns == structs.AllNamespacesSentinel {
		return hostVolumeCoded(http.StatusBadRequest, hostVolumeErrClaimSingleNS)
	}
	if args.ID == "" {
		return hostVolumeCoded(http.StatusBadRequest, hostVolumeErrClaimMissingID)
	}
	if strings.ContainsRune(args.ID, 0) {
		return hostVolumeCoded(http.StatusBadRequest, hostVolumeErrClaimInvalidID)
	}

	store := v.srv.fsm.State()
	claim, err := store.DynamicHostVolumeClaimByID(nil, ns, args.ID)
	if err != nil {
		return hostVolumeStateErr(err, hostVolumeErrClaimInvalidID)
	}
	if claim == nil {
		return hostVolumeCoded(http.StatusNotFound, hostVolumeErrClaimNotFound)
	}
	held, err := store.DynamicHostVolumeClaimInUse(claim)
	if err != nil {
		return err
	}
	if held {
		return hostVolumeCoded(http.StatusConflict, hostVolumeErrClaimInUse)
	}

	// The compare-and-set index is the one read above, chosen before the
	// command is submitted. A claim rewritten since is refused as a conflict.
	_, index, err := v.srv.dynamicHostVolumeRaftApply(structs.DynamicHostVolumeClaimDeleteRequestType,
		&structs.DynamicHostVolumeClaimDeleteRequest{
			Claims:       []structs.DynamicHostVolumeClaimKey{{Namespace: ns, ID: claim.ID, ExpectedModifyIndex: claim.ModifyIndex}},
			WriteRequest: v.writeRequest(),
		})
	switch {
	case err == nil:
	case errors.Is(err, ErrDynamicHostVolumeVersion):
		return hostVolumeCoded(http.StatusServiceUnavailable, hostVolumeErrVersionGate)
	case errors.Is(err, state.ErrDynamicHostVolumeStateInUse):
		return hostVolumeCoded(http.StatusConflict, hostVolumeErrClaimInUse)
	case errors.Is(err, state.ErrDynamicHostVolumeStateCAS):
		// A claim removed since the read is a conflict as well; the caller
		// reads again and sees which.
		return hostVolumeCoded(http.StatusConflict, hostVolumeErrConcurrent)
	case errors.Is(err, state.ErrDynamicHostVolumeStateInvalid):
		return hostVolumeCoded(http.StatusBadRequest, hostVolumeErrClaimInvalidID)
	default:
		return err
	}
	reply.Index = index
	return nil
}

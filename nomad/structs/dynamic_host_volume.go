// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"maps"
	"slices"

	"github.com/open-wander/wander/helper"
	"github.com/open-wander/wander/helper/pointer"
)

// These labels belong to Wander's internal model requirements. They do not
// establish public wire compatibility, validation rules, or state transitions.
type DynamicHostVolumeState string

const (
	DynamicHostVolumeStateUnknown     DynamicHostVolumeState = ""
	DynamicHostVolumeStateRequested   DynamicHostVolumeState = "requested"
	DynamicHostVolumeStatePending     DynamicHostVolumeState = "pending"
	DynamicHostVolumeStateReady       DynamicHostVolumeState = "ready"
	DynamicHostVolumeStateUnavailable DynamicHostVolumeState = "unavailable"
	DynamicHostVolumeStateDeleting    DynamicHostVolumeState = "deleting"
	DynamicHostVolumeStateFailed      DynamicHostVolumeState = "failed"
)

type DynamicHostVolumeOrigin string

const (
	DynamicHostVolumeOriginUnknown       DynamicHostVolumeOrigin = ""
	DynamicHostVolumeOriginRegistered    DynamicHostVolumeOrigin = "registered"
	DynamicHostVolumeOriginPluginCreated DynamicHostVolumeOrigin = "plugin-created"
)

type DynamicHostVolumeAttachmentMode string

const (
	DynamicHostVolumeAttachmentModeUnknown    DynamicHostVolumeAttachmentMode = ""
	DynamicHostVolumeAttachmentModeFilesystem DynamicHostVolumeAttachmentMode = "file-system"
)

type DynamicHostVolumeAccessMode string

const (
	DynamicHostVolumeAccessModeUnknown                DynamicHostVolumeAccessMode = ""
	DynamicHostVolumeAccessModeSingleNodeReader       DynamicHostVolumeAccessMode = "single-node-reader-only"
	DynamicHostVolumeAccessModeSingleNodeWriter       DynamicHostVolumeAccessMode = "single-node-writer"
	DynamicHostVolumeAccessModeSingleNodeSingleWriter DynamicHostVolumeAccessMode = "single-node-single-writer"
	DynamicHostVolumeAccessModeSingleNodeMultiWriter  DynamicHostVolumeAccessMode = "single-node-multi-writer"
)

// DynamicHostVolumeAccessAllows reports whether a request with the given
// read-only setting may use a volume capability with the given access mode.
// A writer capability serves read/write requests only, a reader-only
// capability serves read-only requests only, and the single-writer and
// multi-writer capabilities serve both. Unknown modes serve nothing.
func DynamicHostVolumeAccessAllows(access DynamicHostVolumeAccessMode, readOnly bool) bool {
	switch access {
	case DynamicHostVolumeAccessModeSingleNodeReader:
		return readOnly
	case DynamicHostVolumeAccessModeSingleNodeWriter:
		return !readOnly
	case DynamicHostVolumeAccessModeSingleNodeSingleWriter, DynamicHostVolumeAccessModeSingleNodeMultiWriter:
		return true
	default:
		return false
	}
}

// DynamicHostVolume is storage metadata, independent of any operation request.
type DynamicHostVolume struct {
	ID                        string
	Name                      string
	Namespace                 string
	Type                      string
	NodeID                    string
	NodePool                  string
	HostPath                  string
	PluginID                  string
	CapacityBytes             int64
	RequestedCapacityMinBytes int64
	RequestedCapacityMaxBytes int64
	RequestedCapabilities     []*DynamicHostVolumeCapability
	Constraints               []*Constraint
	Parameters                map[string]string
	State                     DynamicHostVolumeState
	Origin                    DynamicHostVolumeOrigin
	Attachments               []*DynamicHostVolumeAttachment
	CreateTime                int64
	ModifyTime                int64
	CreateIndex               uint64
	ModifyIndex               uint64
}

type DynamicHostVolumeCapability struct {
	AttachmentMode DynamicHostVolumeAttachmentMode
	AccessMode     DynamicHostVolumeAccessMode
}

type DynamicHostVolumeAttachment struct {
	AllocationID   string
	NodeID         string
	AttachmentMode DynamicHostVolumeAttachmentMode
	AccessMode     DynamicHostVolumeAccessMode
	ReadOnly       bool
}

// DynamicHostVolumeClaim records a sticky allocation's volume identity.
type DynamicHostVolumeClaim struct {
	ID              string
	Namespace       string
	JobID           string
	TaskGroupName   string
	VolumeName      string
	VolumeID        string
	AllocationID    string
	AllocationIndex uint64
	CreateIndex     uint64
	ModifyIndex     uint64
}

func (v *DynamicHostVolume) Copy() *DynamicHostVolume {
	if v == nil {
		return nil
	}
	copied := *v
	copied.Parameters = maps.Clone(v.Parameters)
	copied.RequestedCapabilities = helper.CopySlice(v.RequestedCapabilities)
	copied.Constraints = helper.CopySlice(v.Constraints)
	copied.Attachments = helper.CopySlice(v.Attachments)
	return &copied
}

func (v *DynamicHostVolume) Equal(other *DynamicHostVolume) bool {
	if v == nil || other == nil {
		return v == other
	}
	return v.ID == other.ID && v.Name == other.Name && v.Namespace == other.Namespace &&
		v.Type == other.Type && v.NodeID == other.NodeID && v.NodePool == other.NodePool &&
		v.HostPath == other.HostPath && v.PluginID == other.PluginID &&
		v.CapacityBytes == other.CapacityBytes &&
		v.RequestedCapacityMinBytes == other.RequestedCapacityMinBytes &&
		v.RequestedCapacityMaxBytes == other.RequestedCapacityMaxBytes &&
		v.State == other.State && v.Origin == other.Origin &&
		v.CreateTime == other.CreateTime && v.ModifyTime == other.ModifyTime &&
		v.CreateIndex == other.CreateIndex && v.ModifyIndex == other.ModifyIndex &&
		(v.Parameters == nil) == (other.Parameters == nil) && maps.Equal(v.Parameters, other.Parameters) &&
		(v.RequestedCapabilities == nil) == (other.RequestedCapabilities == nil) &&
		slices.EqualFunc(v.RequestedCapabilities, other.RequestedCapabilities, (*DynamicHostVolumeCapability).Equal) &&
		(v.Constraints == nil) == (other.Constraints == nil) &&
		slices.EqualFunc(v.Constraints, other.Constraints, dynamicHostVolumeConstraintEqual) &&
		(v.Attachments == nil) == (other.Attachments == nil) &&
		slices.EqualFunc(v.Attachments, other.Attachments, (*DynamicHostVolumeAttachment).Equal)
}

func dynamicHostVolumeConstraintEqual(left, right *Constraint) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Equal(right)
}

func (c *DynamicHostVolumeCapability) Copy() *DynamicHostVolumeCapability {
	return pointer.Copy(c)
}

func (c *DynamicHostVolumeCapability) Equal(other *DynamicHostVolumeCapability) bool {
	return dynamicHostVolumeValueEqual(c, other)
}

func (a *DynamicHostVolumeAttachment) Copy() *DynamicHostVolumeAttachment {
	return pointer.Copy(a)
}

func (a *DynamicHostVolumeAttachment) Equal(other *DynamicHostVolumeAttachment) bool {
	return dynamicHostVolumeValueEqual(a, other)
}

func (c *DynamicHostVolumeClaim) Copy() *DynamicHostVolumeClaim {
	return pointer.Copy(c)
}

func (c *DynamicHostVolumeClaim) Equal(other *DynamicHostVolumeClaim) bool {
	return dynamicHostVolumeValueEqual(c, other)
}

func dynamicHostVolumeValueEqual[T comparable](left, right *T) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

// DynamicHostVolumeUse is the verdict on whether a dynamic host volume record
// can serve the host volume requests made against it on a node.
type DynamicHostVolumeUse int

const (
	DynamicHostVolumeUseOK DynamicHostVolumeUse = iota
	DynamicHostVolumeUseMissing
	DynamicHostVolumeUseNotReady
	DynamicHostVolumeUseNoCapability
)

// DynamicHostVolumeUsableBy reports whether the record, looked up in the
// namespace of the job, can serve the requests for source on the node. The
// record must belong to the node, carry the requested name, be ready, list a
// capability exactly equal to each request's effective modes, and allow the
// request's direction. Callers map the verdict to their own bounded text.
func DynamicHostVolumeUsableBy(vol *DynamicHostVolume, nodeID, source string, requests []*VolumeRequest) DynamicHostVolumeUse {
	if vol == nil || vol.NodeID != nodeID || vol.Name != source {
		return DynamicHostVolumeUseMissing
	}
	if vol.State != DynamicHostVolumeStateReady {
		return DynamicHostVolumeUseNotReady
	}

	for _, req := range requests {
		attachment, access := req.HostVolumeModes()
		matched := false
		for _, capability := range vol.RequestedCapabilities {
			if capability != nil && capability.AttachmentMode == attachment && capability.AccessMode == access {
				matched = true
				break
			}
		}
		// The direction rule applies to the matched mode, so an explicit
		// single-node-writer request that is read-only is refused rather than
		// falling through to another capability.
		if !matched || !DynamicHostVolumeAccessAllows(access, req.ReadOnly) {
			return DynamicHostVolumeUseNoCapability
		}
	}
	return DynamicHostVolumeUseOK
}

// HasSingleWriterCapability reports whether the record lists the
// single-node-single-writer access mode. Without it no user can hold the
// volume exclusively, so writer accounting can be skipped.
func (v *DynamicHostVolume) HasSingleWriterCapability() bool {
	for _, capability := range v.RequestedCapabilities {
		if capability != nil && capability.AccessMode == DynamicHostVolumeAccessModeSingleNodeSingleWriter {
			return true
		}
	}
	return false
}

// DynamicHostVolumeAllocRequests returns the host volume requests through
// which the allocation uses the volume: those of its task group whose source,
// with the per-allocation suffix applied, is the volume name. It returns none
// when the allocation has no job (and fallbackJob is nil), belongs to another
// namespace or names a task group the job lacks. The allocation's own
// namespace is preferred over the job's, because a plan allocation may not
// carry a job. Liveness is the caller's concern. Neither the allocation nor
// the job is modified.
func DynamicHostVolumeAllocRequests(vol *DynamicHostVolume, alloc *Allocation, fallbackJob *Job) []*VolumeRequest {
	if alloc == nil {
		return nil
	}
	job := alloc.Job
	if job == nil {
		job = fallbackJob
	}
	if job == nil {
		return nil
	}
	namespace := alloc.Namespace
	if namespace == "" {
		namespace = job.Namespace
	}
	if namespace != vol.Namespace {
		return nil
	}
	tg := job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return nil
	}
	return HostVolumeRequestsBySource(tg.Volumes, alloc.Name)[vol.Name]
}

// DynamicHostVolumeWriterConflict reports whether the request cannot be placed
// because another allocation already uses the volume in a way that excludes
// it. Two users conflict when at least one reaches the volume through
// single-node-single-writer and at least one of them is read/write; readers
// of that mode share the volume and the other modes never limit one another.
// The allocations in others are the proposed live allocations of the node.
// selfID names an allocation that never conflicts with itself, and
// fallbackJob stands in for an allocation whose Job is unset, as in a plan.
// Neither the allocations nor the jobs are modified.
func DynamicHostVolumeWriterConflict(vol *DynamicHostVolume, req *VolumeRequest, selfID string, others []*Allocation, fallbackJob *Job) bool {
	_, access := req.HostVolumeModes()

	for _, other := range others {
		if other == nil || (selfID != "" && other.ID == selfID) {
			continue
		}

		for _, otherReq := range DynamicHostVolumeAllocRequests(vol, other, fallbackJob) {
			_, otherAccess := otherReq.HostVolumeModes()
			exclusive := access == DynamicHostVolumeAccessModeSingleNodeSingleWriter ||
				otherAccess == DynamicHostVolumeAccessModeSingleNodeSingleWriter
			if exclusive && (!req.ReadOnly || !otherReq.ReadOnly) {
				return true
			}
		}
	}
	return false
}

// DynamicHostVolumeStickyAllowed applies the sticky claim rule to one
// candidate volume of a task group. When any of the requests for the source is
// sticky and the candidate is a dynamic volume (a non-empty ID), the claims of
// the group for the source whose volume record still exists and that no live
// allocation of the group uses are the free claims. If there are free claims
// the candidate must be one of them; with none, any candidate is allowed.
//
// claims may hold claims of other groups and sources, which are ignored.
// recordNodes maps the volume ID of each claimed volume that still has a record
// to the node of that record, and liveNodes holds the nodes carrying a live
// allocation of the group that uses the source. A table allows one record per
// namespace, node and name, so a claimed volume is used exactly when a live
// allocation of the group sits on the node of its record. Nothing is modified.
func DynamicHostVolumeStickyAllowed(requests []*VolumeRequest, group, source string, claims []*DynamicHostVolumeClaim,
	recordNodes map[string]string, liveNodes map[string]bool, candidateVolumeID string) bool {

	if candidateVolumeID == "" || !slices.ContainsFunc(requests, func(r *VolumeRequest) bool { return r != nil && r.Sticky }) {
		return true
	}

	free := false
	for _, claim := range claims {
		if claim == nil || claim.TaskGroupName != group || claim.VolumeName != source {
			continue
		}
		node, ok := recordNodes[claim.VolumeID]
		if !ok || liveNodes[node] {
			continue
		}
		free = true
		if claim.VolumeID == candidateVolumeID {
			return true
		}
	}
	return !free
}

// DynamicHostVolumeStickyLiveAllocs returns the live allocations of the group
// as the plan would leave them: the stored allocations that are not terminal on
// the server or the client, less those the plan stops or preempts on any node,
// plus the plan's placements for the group. A placement replaces a stored
// allocation with the same ID, and is then itself left out when it is terminal.
// The allocation named by selfID, which is the one being checked, is left out.
// A nil plan changes nothing. Neither the allocations nor the plan are
// modified.
//
// An allocation in client status unknown is on a disconnected client and is not
// live for this rule: its claim stays with its volume and is free, so every
// placement of the group, including the replacement of that allocation, is held
// to the volume until the client is back. Other rules, such as the writer rule
// and the volume delete, still count an unknown allocation as a user.
func DynamicHostVolumeStickyLiveAllocs(stored []*Allocation, plan *Plan, group, selfID string) []*Allocation {
	removed := make(map[string]struct{})
	if plan != nil {
		for _, allocs := range plan.NodeUpdate {
			for _, alloc := range allocs {
				removed[alloc.ID] = struct{}{}
			}
		}
		for _, allocs := range plan.NodePreemptions {
			for _, alloc := range allocs {
				removed[alloc.ID] = struct{}{}
			}
		}
	}

	live := make([]*Allocation, 0, len(stored))
	seen := make(map[string]struct{}, len(stored))
	for _, alloc := range stored {
		if alloc == nil || alloc.TaskGroup != group || stickyNotLive(alloc) || alloc.ID == selfID {
			continue
		}
		if _, ok := removed[alloc.ID]; ok {
			continue
		}
		seen[alloc.ID] = struct{}{}
		live = append(live, alloc)
	}

	if plan == nil {
		return live
	}
	// The map order does not matter: callers only collect nodes from the result.
	for _, allocs := range plan.NodeAllocation {
		for _, alloc := range allocs {
			if alloc.TaskGroup != group || alloc.ID == selfID {
				continue
			}
			if _, ok := seen[alloc.ID]; ok {
				// The stored copy of an updated allocation is replaced, even
				// when the new copy is not live.
				live = slices.DeleteFunc(live, func(a *Allocation) bool { return a.ID == alloc.ID })
			}
			if stickyNotLive(alloc) {
				continue
			}
			seen[alloc.ID] = struct{}{}
			live = append(live, alloc)
		}
	}
	return live
}

// DynamicHostVolumeWriterHolders returns the allocations of a node that count
// as using its volumes for the writer rule, with the plan applied. stored are
// the allocations of the node from the state. The result leaves out the stored
// allocations that are client-terminal and those the plan stops in NodeUpdate,
// and adds the plan's placements for the node, a placement replacing the stored
// allocation with the same ID.
//
// An allocation the plan preempts stays in the result. A preemption only asks
// the client to stop the allocation, which may still have the volume mounted,
// and a holder is never preempted to make room for another user of its volume.
// The stored copy is kept, because the preemption entries of a plan are stubs
// with no job and no task group. Neither the allocations nor the plan are
// modified, so the result may be built from a plan shared between evaluations.
func DynamicHostVolumeWriterHolders(stored []*Allocation, plan *Plan, nodeID string) []*Allocation {
	var stopped map[string]struct{}
	var placed []*Allocation
	if plan != nil {
		stopped = make(map[string]struct{}, len(plan.NodeUpdate[nodeID]))
		for _, alloc := range plan.NodeUpdate[nodeID] {
			stopped[alloc.ID] = struct{}{}
		}
		placed = plan.NodeAllocation[nodeID]
	}

	replaced := make(map[string]struct{}, len(placed))
	for _, alloc := range placed {
		replaced[alloc.ID] = struct{}{}
	}

	holders := make([]*Allocation, 0, len(stored)+len(placed))
	for _, alloc := range stored {
		if alloc == nil || alloc.ClientTerminalStatus() {
			continue
		}
		if _, ok := stopped[alloc.ID]; ok {
			continue
		}
		if _, ok := replaced[alloc.ID]; ok {
			continue
		}
		holders = append(holders, alloc)
	}
	return append(holders, placed...)
}

// stickyNotLive reports whether an allocation does not hold a sticky claim: it
// is terminal, or its client is disconnected.
func stickyNotLive(alloc *Allocation) bool {
	return alloc.TerminalStatus() || alloc.ClientStatus == AllocClientStatusUnknown
}

// DynamicHostVolumeStickyLiveNodes returns the nodes of the allocations that use
// the source, resolving the source per allocation because a per-alloc source
// carries the allocation index. fallbackJob stands in for an allocation whose
// Job is unset, as in a plan.
func DynamicHostVolumeStickyLiveNodes(allocs []*Allocation, source string, fallbackJob *Job) map[string]bool {
	nodes := make(map[string]bool)
	for _, alloc := range allocs {
		job := alloc.Job
		if job == nil {
			job = fallbackJob
		}
		if job == nil {
			continue
		}
		tg := job.LookupTaskGroup(alloc.TaskGroup)
		if tg == nil {
			continue
		}
		if _, ok := HostVolumeRequestsBySource(tg.Volumes, alloc.Name)[source]; ok {
			nodes[alloc.NodeID] = true
		}
	}
	return nodes
}

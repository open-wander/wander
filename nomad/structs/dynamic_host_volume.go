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

		job := other.Job
		if job == nil {
			job = fallbackJob
		}
		if job == nil {
			continue
		}
		// The allocation's own namespace is preferred, because a plan
		// allocation may not carry a job.
		namespace := other.Namespace
		if namespace == "" {
			namespace = job.Namespace
		}
		if namespace != vol.Namespace {
			continue
		}
		tg := job.LookupTaskGroup(other.TaskGroup)
		if tg == nil {
			continue
		}

		for _, otherReq := range HostVolumeRequestsBySource(tg.Volumes, other.Name)[vol.Name] {
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

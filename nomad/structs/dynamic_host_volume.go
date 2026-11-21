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

// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package structs

// HostVolumeListRequest lists dynamic host volumes. The prefix applies to the
// volume ID; NodeID and NodePool are exact-match filters.
type HostVolumeListRequest struct {
	NodeID   string
	NodePool string
	QueryOptions
}

type HostVolumeListResponse struct {
	Volumes []*DynamicHostVolumeStub
	QueryMeta
}

// HostVolumeGetRequest reads one dynamic host volume in the request namespace.
type HostVolumeGetRequest struct {
	ID string
	QueryOptions
}

type HostVolumeGetResponse struct {
	Volume *DynamicHostVolume
	QueryMeta
}

// HostVolumeCreateRequest asks the servers to create a dynamic host volume with
// a plugin, or, when Volume.ID names a stored volume, to update its mutable
// fields. PolicyOverride is accepted and ignored: Wander has no policy engine.
type HostVolumeCreateRequest struct {
	Volume         *DynamicHostVolume
	PolicyOverride bool
	WriteRequest
}

// HostVolumeCreateResponse carries the stored volume read back after the
// request, and the index of the last write made for it.
type HostVolumeCreateResponse struct {
	Volume *DynamicHostVolume
	WriteMeta
}

// HostVolumeRegisterRequest asks the servers to register an existing directory
// on a node as a dynamic host volume, or, when Volume.ID names a stored volume,
// to update its mutable fields. PolicyOverride is accepted and ignored.
type HostVolumeRegisterRequest struct {
	Volume         *DynamicHostVolume
	PolicyOverride bool
	WriteRequest
}

// HostVolumeRegisterResponse carries the stored volume read back after the
// request, and the index of the last write made for it.
type HostVolumeRegisterResponse struct {
	Volume *DynamicHostVolume
	WriteMeta
}

// DynamicHostVolumeStub is the listing form of a volume. It leaves out the
// host path, parameters, constraints, attachments and requested capabilities;
// a read returns the full record.
type DynamicHostVolumeStub struct {
	ID            string
	Name          string
	Namespace     string
	PluginID      string
	NodeID        string
	NodePool      string
	CapacityBytes int64
	State         DynamicHostVolumeState
	CreateIndex   uint64
	ModifyIndex   uint64
	CreateTime    int64
	ModifyTime    int64
}

// Stub returns the listing form of the volume.
func (v *DynamicHostVolume) Stub() *DynamicHostVolumeStub {
	if v == nil {
		return nil
	}
	return &DynamicHostVolumeStub{
		ID:            v.ID,
		Name:          v.Name,
		Namespace:     v.Namespace,
		PluginID:      v.PluginID,
		NodeID:        v.NodeID,
		NodePool:      v.NodePool,
		CapacityBytes: v.CapacityBytes,
		State:         v.State,
		CreateIndex:   v.CreateIndex,
		ModifyIndex:   v.ModifyIndex,
		CreateTime:    v.CreateTime,
		ModifyTime:    v.ModifyTime,
	}
}

// The getters let the list paginator tokenise and namespace-filter volumes.

func (v *DynamicHostVolume) GetID() string          { return v.ID }
func (v *DynamicHostVolume) GetNamespace() string   { return v.Namespace }
func (v *DynamicHostVolume) GetCreateIndex() uint64 { return v.CreateIndex }

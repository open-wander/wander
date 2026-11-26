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

// HostVolumeClaimListRequest lists sticky claims. The query prefix applies to
// the claim ID; the other fields are exact-match filters.
type HostVolumeClaimListRequest struct {
	JobID      string
	TaskGroup  string
	VolumeName string
	QueryOptions
}

// HostVolumeClaimListResponse returns the stored claims, including the
// allocation index that the public listing leaves out.
type HostVolumeClaimListResponse struct {
	Claims []*DynamicHostVolumeClaim
	QueryMeta
}

// HostVolumeClaimDeleteRequest removes one sticky claim in the request
// namespace.
type HostVolumeClaimDeleteRequest struct {
	ID string
	WriteRequest
}

type HostVolumeClaimDeleteResponse struct {
	WriteMeta
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

// The getters let the claim list paginator tokenise and namespace-filter claims.

func (c *DynamicHostVolumeClaim) GetID() string          { return c.ID }
func (c *DynamicHostVolumeClaim) GetNamespace() string   { return c.Namespace }
func (c *DynamicHostVolumeClaim) GetCreateIndex() uint64 { return c.CreateIndex }

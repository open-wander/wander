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

// HostVolumeDeleteRequest asks the servers to delete a plugin-created dynamic
// host volume. Force skips the call to the node only when the node is down,
// disconnected or unknown; it never skips the check for a volume in use.
type HostVolumeDeleteRequest struct {
	VolumeID string
	Force    bool
	WriteRequest
}

// HostVolumeDeleteResponse carries the index of the write that removed the
// volume.
type HostVolumeDeleteResponse struct {
	WriteMeta
}

// HostVolumeDeregisterRequest asks the servers to deregister a registered
// dynamic host volume. The directory is never touched. Force has the rules of
// HostVolumeDeleteRequest.
type HostVolumeDeregisterRequest struct {
	VolumeID string
	Force    bool
	WriteRequest
}

// HostVolumeDeregisterResponse carries the index of the write that removed the
// volume.
type HostVolumeDeregisterResponse struct {
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

// Modes of HostVolume.RemoveOrphan. There is no default: a request names one.
const (
	// HostVolumeOrphanModeDelete runs the plugin delete on the node and drops
	// its record.
	HostVolumeOrphanModeDelete = "delete"
	// HostVolumeOrphanModeForget drops the record on the node and keeps the
	// directory. No plugin runs.
	HostVolumeOrphanModeForget = "forget"
)

// HostVolumeOrphanListRequest lists the volumes that nodes advertise and the
// servers hold no record of. NodeID limits the list to one node. The query
// namespace limits it to one namespace, and a wildcard lists every namespace
// the token may read.
type HostVolumeOrphanListRequest struct {
	NodeID string
	QueryOptions
}

// HostVolumeOrphan is one advertised volume with no record. It carries no host
// path. Namespace is what the node advertises, and is empty for a node that
// does not say.
type HostVolumeOrphan struct {
	NodeID    string
	VolumeID  string
	Name      string
	Namespace string
}

type HostVolumeOrphanListResponse struct {
	Orphans []*HostVolumeOrphan
	QueryMeta
}

// HostVolumeOrphanRemoveRequest asks the servers to remove an orphaned volume
// from a node. The namespace is the request namespace. Mode is "delete" or
// "forget" and has no default.
type HostVolumeOrphanRemoveRequest struct {
	NodeID   string
	VolumeID string
	Mode     string
	WriteRequest
}

// HostVolumeOrphanRemoveResponse is empty: removing an orphan writes no
// record on the servers, so there is no index to report.
type HostVolumeOrphanRemoveResponse struct {
	WriteMeta
}

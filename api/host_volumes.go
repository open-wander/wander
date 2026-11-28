// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"errors"
	"net/url"
)

// HostVolumeState is the lifecycle state of a dynamic host volume.
type HostVolumeState string

const (
	HostVolumeStateUnknown     HostVolumeState = ""
	HostVolumeStateRequested   HostVolumeState = "requested"
	HostVolumeStatePending     HostVolumeState = "pending"
	HostVolumeStateReady       HostVolumeState = "ready"
	HostVolumeStateUnavailable HostVolumeState = "unavailable"
	HostVolumeStateDeleting    HostVolumeState = "deleting"
	HostVolumeStateFailed      HostVolumeState = "failed"
)

// HostVolumeOrigin says how a dynamic host volume came to exist.
type HostVolumeOrigin string

const (
	HostVolumeOriginUnknown       HostVolumeOrigin = ""
	HostVolumeOriginRegistered    HostVolumeOrigin = "registered"
	HostVolumeOriginPluginCreated HostVolumeOrigin = "plugin-created"
)

// HostVolumeAttachmentMode is how a volume is attached to a task.
type HostVolumeAttachmentMode string

const (
	HostVolumeAttachmentModeUnknown    HostVolumeAttachmentMode = ""
	HostVolumeAttachmentModeFilesystem HostVolumeAttachmentMode = "file-system"
)

// HostVolumeAccessMode is how many allocations may use a volume and how.
type HostVolumeAccessMode string

const (
	HostVolumeAccessModeUnknown                HostVolumeAccessMode = ""
	HostVolumeAccessModeSingleNodeReader       HostVolumeAccessMode = "single-node-reader-only"
	HostVolumeAccessModeSingleNodeWriter       HostVolumeAccessMode = "single-node-writer"
	HostVolumeAccessModeSingleNodeSingleWriter HostVolumeAccessMode = "single-node-single-writer"
	HostVolumeAccessModeSingleNodeMultiWriter  HostVolumeAccessMode = "single-node-multi-writer"
)

// HostVolumeOrphanMode says what removing an orphaned volume does. There is no
// default: the caller must choose.
type HostVolumeOrphanMode string

const (
	// HostVolumeOrphanModeDelete runs the plugin delete and drops the client
	// record.
	HostVolumeOrphanModeDelete HostVolumeOrphanMode = "delete"

	// HostVolumeOrphanModeForget drops the client record and keeps the
	// directory.
	HostVolumeOrphanModeForget HostVolumeOrphanMode = "forget"
)

// HostVolume is a dynamic host volume as the server stores and returns it.
// The same type is the volume of a create or register request.
type HostVolume struct {
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
	RequestedCapabilities     []*HostVolumeCapability
	Constraints               []*Constraint
	Parameters                map[string]string
	State                     HostVolumeState
	StatusDescription         string
	Origin                    HostVolumeOrigin
	Attachments               []*HostVolumeAttachment
	CreateTime                int64
	ModifyTime                int64
	CreateIndex               uint64
	ModifyIndex               uint64
}

// HostVolumeCapability is one way the volume is requested to be usable.
type HostVolumeCapability struct {
	AttachmentMode HostVolumeAttachmentMode
	AccessMode     HostVolumeAccessMode
}

// HostVolumeAttachment is an allocation that uses a volume. The allocation is
// named AllocationID here, as the server sends it; only a claim says AllocID.
type HostVolumeAttachment struct {
	AllocationID   string
	NodeID         string
	AttachmentMode HostVolumeAttachmentMode
	AccessMode     HostVolumeAccessMode
	ReadOnly       bool
}

// HostVolumeStub is the listing form of a volume.
type HostVolumeStub struct {
	ID            string
	Name          string
	Namespace     string
	PluginID      string
	NodeID        string
	NodePool      string
	CapacityBytes int64
	State         HostVolumeState
	CreateIndex   uint64
	ModifyIndex   uint64
	CreateTime    int64
	ModifyTime    int64
}

// HostVolumeClaim is a sticky claim of an allocation on a volume.
type HostVolumeClaim struct {
	AllocID       string
	CreateIndex   uint64
	ID            string
	JobID         string
	ModifyIndex   uint64
	Namespace     string
	TaskGroupName string
	VolumeID      string
	VolumeName    string
}

// HostVolumeOrphan is a dynamic volume that a node advertises and the servers
// hold no record of.
type HostVolumeOrphan struct {
	NodeID    string
	VolumeID  string
	Name      string
	Namespace string
}

// HostVolumeListRequest narrows a volume listing. An empty field is not sent.
// Namespace, prefix and paging come from the QueryOptions.
type HostVolumeListRequest struct {
	NodeID   string
	NodePool string
}

// HostVolumeClaimListRequest narrows a claim listing. An empty field is not
// sent. ClaimID is an ID prefix. Namespace and paging come from the
// QueryOptions.
type HostVolumeClaimListRequest struct {
	ClaimID    string
	JobID      string
	TaskGroup  string
	VolumeName string
}

// HostVolumeCreateRequest is the body of a create.
type HostVolumeCreateRequest struct {
	Volume *HostVolume

	// PolicyOverride is sent as given. Wander has no policy engine, so the
	// server only warns that it has no effect.
	PolicyOverride bool
}

// HostVolumeRegisterRequest is the body of a register.
type HostVolumeRegisterRequest struct {
	Volume         *HostVolume
	PolicyOverride bool
}

// HostVolumeWriteResponse is the reply of a create or register. Warnings is
// empty when the server has none.
type HostVolumeWriteResponse struct {
	Volume   *HostVolume
	Warnings string
}

// Errors the client returns before it sends anything. The texts are fixed and
// never echo the input.
var (
	errHostVolumeID       = errors.New("host volume ID is required")
	errHostVolumeClaimID  = errors.New("host volume claim ID is required")
	errHostVolumeNodeID   = errors.New("node ID is required")
	errHostVolumeRequired = errors.New("host volume is required")
	errHostVolumeMode     = errors.New("orphan mode must be delete or forget")
	errHostVolumeFilter   = errors.New("filter is not supported for claims")
	errHostVolumePrefix   = errors.New("prefix and claim ID differ")
)

// HostVolumes is used to access the dynamic host volume endpoints.
type HostVolumes struct {
	client *Client
}

// HostVolumes returns a handle on the dynamic host volume endpoints.
func (c *Client) HostVolumes() *HostVolumes {
	return &HostVolumes{client: c}
}

// withQuery appends the non-empty values to the endpoint. It builds a new
// query string and never touches the Params of the caller's options.
func withQuery(endpoint string, kv ...string) string {
	qp := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			qp.Set(kv[i], kv[i+1])
		}
	}
	if len(qp) == 0 {
		return endpoint
	}
	return endpoint + "?" + qp.Encode()
}

// List returns the dynamic host volumes. req may be nil. A nil q means the
// default options; the namespace, prefix and paging are read from it.
func (h *HostVolumes) List(req *HostVolumeListRequest, q *QueryOptions) ([]*HostVolumeStub, *QueryMeta, error) {
	if req == nil {
		req = &HostVolumeListRequest{}
	}
	endpoint := withQuery("/v1/volumes", "type", "host", "node_id", req.NodeID, "node_pool", req.NodePool)
	var out []*HostVolumeStub
	qm, err := h.client.query(endpoint, &out, q)
	if err != nil {
		return nil, nil, err
	}
	return out, qm, nil
}

// Info returns one volume by its full ID.
func (h *HostVolumes) Info(id string, q *QueryOptions) (*HostVolume, *QueryMeta, error) {
	if id == "" {
		return nil, nil, errHostVolumeID
	}
	var out HostVolume
	qm, err := h.client.query("/v1/volume/host/"+url.PathEscape(id), &out, q)
	if err != nil {
		return nil, nil, err
	}
	return &out, qm, nil
}

// Create asks for a volume to be created by a plugin on a node. A volume with
// an ID updates the stored volume; there is no separate update method.
//
// With no Namespace in w, the server takes the namespace of the volume. The
// client adds the namespace of its own configuration (or NOMAD_NAMESPACE) to
// every request, and the server refuses a request whose namespace differs from
// the namespace of the volume. The options are never overridden here.
func (h *HostVolumes) Create(req *HostVolumeCreateRequest, w *WriteOptions) (*HostVolumeWriteResponse, *WriteMeta, error) {
	if req == nil || req.Volume == nil {
		return nil, nil, errHostVolumeRequired
	}
	var out HostVolumeWriteResponse
	wm, err := h.client.put("/v1/volume/host/create", req, &out, w)
	if err != nil {
		return nil, nil, err
	}
	return &out, wm, nil
}

// Register records a volume that already exists on a node. A volume with an
// ID updates the stored volume. The namespace rule of Create applies.
func (h *HostVolumes) Register(req *HostVolumeRegisterRequest, w *WriteOptions) (*HostVolumeWriteResponse, *WriteMeta, error) {
	if req == nil || req.Volume == nil {
		return nil, nil, errHostVolumeRequired
	}
	var out HostVolumeWriteResponse
	wm, err := h.client.put("/v1/volume/host/register", req, &out, w)
	if err != nil {
		return nil, nil, err
	}
	return &out, wm, nil
}

// Delete deletes a volume and its data through the plugin. force is sent only
// when true.
func (h *HostVolumes) Delete(id string, force bool, w *WriteOptions) (*WriteMeta, error) {
	return h.remove(id, "delete", force, w)
}

// Deregister drops the record of a volume and keeps its data. force is sent
// only when true.
func (h *HostVolumes) Deregister(id string, force bool, w *WriteOptions) (*WriteMeta, error) {
	return h.remove(id, "deregister", force, w)
}

func (h *HostVolumes) remove(id, op string, force bool, w *WriteOptions) (*WriteMeta, error) {
	if id == "" {
		return nil, errHostVolumeID
	}
	endpoint := "/v1/volume/host/" + url.PathEscape(id) + "/" + op
	if force {
		endpoint = withQuery(endpoint, "force", "true")
	}
	return h.client.delete(endpoint, nil, nil, w)
}

// ListClaims returns the sticky claims. req may be nil. The namespace, paging
// and prefix come from q. The server refuses a filter expression and a prefix
// that differs from ClaimID, so the client refuses them before sending.
func (h *HostVolumes) ListClaims(req *HostVolumeClaimListRequest, q *QueryOptions) ([]*HostVolumeClaim, *QueryMeta, error) {
	if req == nil {
		req = &HostVolumeClaimListRequest{}
	}
	if q != nil {
		if q.Filter != "" {
			return nil, nil, errHostVolumeFilter
		}
		if q.Prefix != "" && req.ClaimID != "" && q.Prefix != req.ClaimID {
			return nil, nil, errHostVolumePrefix
		}
	}
	endpoint := withQuery("/v1/volumes/claims", "claim_id", req.ClaimID, "job_id", req.JobID,
		"task_group", req.TaskGroup, "volume_name", req.VolumeName)
	var out []*HostVolumeClaim
	qm, err := h.client.query(endpoint, &out, q)
	if err != nil {
		return nil, nil, err
	}
	return out, qm, nil
}

// DeleteClaim removes one sticky claim by its full ID.
func (h *HostVolumes) DeleteClaim(claimID string, w *WriteOptions) (*WriteMeta, error) {
	if claimID == "" {
		return nil, errHostVolumeClaimID
	}
	return h.client.delete("/v1/volumes/claim/"+url.PathEscape(claimID), nil, nil, w)
}

// ListOrphans returns the dynamic volumes that nodes advertise and the servers
// hold no record of. An empty nodeID lists every node. Use the wildcard
// namespace in q to list every namespace.
func (h *HostVolumes) ListOrphans(nodeID string, q *QueryOptions) ([]*HostVolumeOrphan, *QueryMeta, error) {
	var out []*HostVolumeOrphan
	qm, err := h.client.query(withQuery("/v1/volumes/host/orphans", "node_id", nodeID), &out, q)
	if err != nil {
		return nil, nil, err
	}
	return out, qm, nil
}

// RemoveOrphan removes an orphaned volume from its node. The mode is required
// and must be one of the two constants. The server writes no record for a
// removal, so the returned WriteMeta has LastIndex 0.
func (h *HostVolumes) RemoveOrphan(nodeID, volumeID string, mode HostVolumeOrphanMode, w *WriteOptions) (*WriteMeta, error) {
	if nodeID == "" {
		return nil, errHostVolumeNodeID
	}
	if volumeID == "" {
		return nil, errHostVolumeID
	}
	if mode != HostVolumeOrphanModeDelete && mode != HostVolumeOrphanModeForget {
		return nil, errHostVolumeMode
	}
	endpoint := withQuery("/v1/volume/host/orphan/"+url.PathEscape(nodeID)+"/"+url.PathEscape(volumeID),
		"mode", string(mode))
	return h.client.delete(endpoint, nil, nil, w)
}

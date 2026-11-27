// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package structs

// HostVolumeRegisterRequest asks a client to adopt an existing directory as a
// dynamic host volume. It is internal Wander design, sent by servers only.
type HostVolumeRegisterRequest struct {
	VolumeID  string
	Namespace string
	Name      string
	NodeID    string
	NodePool  string
	HostPath  string
}

// HostVolumeRegisterResponse reports the outcome of a registration. A
// rejected request returns no RPC error: Reason holds a machine-readable
// code, Message a short explanation and Status is empty. Status is the
// volume status ("ready" or "unavailable") on success.
type HostVolumeRegisterResponse struct {
	VolumeID string
	HostPath string
	Status   string
	Reason   string
	Message  string
}

// HostVolumeCreateRequest asks a client to provision a dynamic host volume
// with a plugin. It is internal Wander design, sent by servers only. Sending
// it again with the same VolumeID, Namespace, Name, NodeID and PluginID is
// safe: the client answers with the stored volume and runs nothing.
type HostVolumeCreateRequest struct {
	VolumeID         string
	Namespace        string
	Name             string
	NodeID           string
	NodePool         string
	PluginID         string
	CapacityMinBytes int64
	CapacityMaxBytes int64
	Parameters       map[string]string
}

// HostVolumeCreateResponse reports the outcome of a create. A rejected
// request returns no RPC error: Reason holds a machine-readable code, Message
// a short sanitised explanation and Status is empty. Status is the volume
// status ("ready" or "unavailable") on success. CapacityBytes is what the
// plugin reported, or zero when it is unknown.
type HostVolumeCreateResponse struct {
	VolumeID      string
	HostPath      string
	CapacityBytes int64
	Status        string
	Reason        string
	Message       string
}

// HostVolumeDeleteRequest asks a client to delete a plugin-created dynamic
// host volume. It is internal Wander design, sent by servers only. It names
// no plugin and no path: the client uses its own stored record. Sending it
// again is safe: a client that no longer holds the volume answers with
// success and runs nothing.
type HostVolumeDeleteRequest struct {
	VolumeID  string
	Namespace string
	NodeID    string
}

// HostVolumeDeleteResponse reports the outcome of a delete. A rejected
// request returns no RPC error: Reason holds a machine-readable code and
// Message a short sanitised explanation. Both are empty on success. VolumeID
// always echoes the request.
type HostVolumeDeleteResponse struct {
	VolumeID string
	Reason   string
	Message  string
}

// HostVolumeDeregisterRequest asks a client to drop the record of a
// registered dynamic host volume. It is internal Wander design, sent by
// servers only. The client never runs a plugin and never touches the
// directory. Sending it again is safe: a client that no longer holds the
// volume answers with success.
type HostVolumeDeregisterRequest struct {
	VolumeID  string
	Namespace string
	NodeID    string
}

// HostVolumeDeregisterResponse reports the outcome of a deregistration, with
// the same rules as HostVolumeDeleteResponse.
type HostVolumeDeregisterResponse struct {
	VolumeID string
	Reason   string
	Message  string
}

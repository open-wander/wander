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

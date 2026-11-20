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

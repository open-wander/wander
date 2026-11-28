// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"errors"
	"time"

	metrics "github.com/armon/go-metrics"

	"github.com/open-wander/wander/client/hostvolume"
	"github.com/open-wander/wander/client/structs"
)

// hostVolumeRegisterTimeout bounds one registration, which can wait on the
// per-volume lock while a plugin create for the same ID runs.
const hostVolumeRegisterTimeout = 2 * time.Minute

// hostVolumeCreateTimeout bounds one create, which includes waiting for the
// per-volume lock and running the plugin.
const hostVolumeCreateTimeout = 2 * time.Minute

// HostVolume is the client RPC endpoint for dynamic host volumes. It is
// Wander policy and only servers call it.
//
// Like the CSI client endpoint it adds no HTTP route and performs no ACL
// check: the server authorizes the request before it calls the node.
type HostVolume struct {
	c *Client
}

// Register adopts an existing directory as a registered volume without
// modifying it. A classified rejection returns a nil error with Reason set
// and nothing stored or published. Only transport or internal failures, such
// as a store error, a canceled wait or a missing manager, return an error.
func (h *HostVolume) Register(req *structs.HostVolumeRegisterRequest, resp *structs.HostVolumeRegisterResponse) error {
	defer metrics.MeasureSince([]string{"client", "host_volume", "register"}, time.Now())

	m := h.c.hostVolumeManager
	if m == nil {
		return errors.New("HostVolume.Register: host volume manager is not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), hostVolumeRegisterTimeout)
	defer cancel()
	go func() {
		select {
		case <-h.c.shutdownCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	st, err := m.AddRegistered(ctx, hostvolume.RegisterRequest{
		VolumeID: req.VolumeID, Namespace: req.Namespace, Name: req.Name,
		NodeID: req.NodeID, NodePool: req.NodePool, Path: req.HostPath,
	})
	if err != nil {
		var oe *hostvolume.OpError
		if errors.As(err, &oe) && oe.Reason != hostvolume.ReasonCanceled {
			*resp = structs.HostVolumeRegisterResponse{Reason: oe.Reason, Message: oe.Message}
			return nil
		}
		return errors.New("HostVolume.Register: " + err.Error())
	}
	*resp = structs.HostVolumeRegisterResponse{
		VolumeID: st.VolumeID, HostPath: st.Path, Status: string(st.Status),
	}
	return nil
}

// Create provisions a volume with the named plugin and records it. A repeat
// for a volume this client already holds, with the same namespace, name, node
// and plugin, returns the stored volume and runs no plugin. A classified
// rejection returns a nil error with Reason set and nothing stored or
// published. Only transport or internal failures, such as a store error, a
// canceled wait or a missing manager, return an error.
func (h *HostVolume) Create(req *structs.HostVolumeCreateRequest, resp *structs.HostVolumeCreateResponse) error {
	defer metrics.MeasureSince([]string{"client", "host_volume", "create"}, time.Now())

	m := h.c.hostVolumeManager
	if m == nil {
		return errors.New("HostVolume.Create: host volume manager is not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), hostVolumeCreateTimeout)
	defer cancel()
	go func() {
		select {
		case <-h.c.shutdownCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	st, err := m.Create(ctx, req.PluginID, hostvolume.OpRequest{
		VolumeID: req.VolumeID, Namespace: req.Namespace, Name: req.Name,
		NodeID: req.NodeID, NodePool: req.NodePool,
		CapacityMinBytes: req.CapacityMinBytes, CapacityMaxBytes: req.CapacityMaxBytes,
		Parameters: req.Parameters,
	})
	if err != nil {
		var oe *hostvolume.OpError
		if errors.As(err, &oe) && oe.Reason != hostvolume.ReasonCanceled {
			*resp = structs.HostVolumeCreateResponse{Reason: oe.Reason, Message: oe.Message}
			return nil
		}
		return errors.New("HostVolume.Create: " + err.Error())
	}
	*resp = structs.HostVolumeCreateResponse{
		VolumeID: st.VolumeID, HostPath: st.Path, CapacityBytes: st.CapacityBytes,
		Status: string(st.Status),
	}
	return nil
}

// hostVolumeDeleteTimeout bounds one delete or deregistration, which includes
// waiting for the per-volume lock and running the plugin.
const hostVolumeDeleteTimeout = 2 * time.Minute

// Delete runs the delete operation of a plugin-created volume and drops its
// record. A volume this client no longer holds answers success and runs
// nothing, so a repeat is safe. A classified rejection, including a failed
// plugin delete that keeps the record, returns a nil error with Reason set.
// A registered volume is refused with volume_kind_conflict: Deregister is the
// only call that drops one, and it never runs a plugin. Only transport or
// internal failures, such as a store error, a canceled wait or a missing
// manager, return an error.
func (h *HostVolume) Delete(req *structs.HostVolumeDeleteRequest, resp *structs.HostVolumeDeleteResponse) error {
	defer metrics.MeasureSince([]string{"client", "host_volume", "delete"}, time.Now())

	err := h.run("HostVolume.Delete", func(ctx context.Context, m *hostvolume.Manager) error {
		return m.Delete(ctx, hostvolume.DeleteRequest{
			VolumeID: req.VolumeID, Namespace: req.Namespace, NodeID: req.NodeID,
		})
	}, func(reason, message string) {
		*resp = structs.HostVolumeDeleteResponse{Reason: reason, Message: message}
	})
	if err != nil {
		return err
	}
	resp.VolumeID = req.VolumeID
	return nil
}

// Deregister drops the record of a registered volume. It never runs a plugin
// and never touches the directory. The reply rules are those of Delete, and a
// plugin-created volume is refused with volume_kind_conflict.
func (h *HostVolume) Deregister(req *structs.HostVolumeDeregisterRequest, resp *structs.HostVolumeDeregisterResponse) error {
	defer metrics.MeasureSince([]string{"client", "host_volume", "deregister"}, time.Now())

	err := h.run("HostVolume.Deregister", func(ctx context.Context, m *hostvolume.Manager) error {
		return m.Deregister(ctx, hostvolume.DeleteRequest{
			VolumeID: req.VolumeID, Namespace: req.Namespace, NodeID: req.NodeID,
		})
	}, func(reason, message string) {
		*resp = structs.HostVolumeDeregisterResponse{Reason: reason, Message: message}
	})
	if err != nil {
		return err
	}
	resp.VolumeID = req.VolumeID
	return nil
}

// Forget drops the record of a volume of either kind and keeps its directory.
// It never runs a plugin. The reply rules are those of Delete, and a volume
// that a live allocation mounts is refused with volume_in_use.
func (h *HostVolume) Forget(req *structs.HostVolumeForgetRequest, resp *structs.HostVolumeForgetResponse) error {
	defer metrics.MeasureSince([]string{"client", "host_volume", "forget"}, time.Now())

	err := h.run("HostVolume.Forget", func(ctx context.Context, m *hostvolume.Manager) error {
		return m.Forget(ctx, hostvolume.DeleteRequest{
			VolumeID: req.VolumeID, Namespace: req.Namespace, NodeID: req.NodeID,
		})
	}, func(reason, message string) {
		*resp = structs.HostVolumeForgetResponse{Reason: reason, Message: message}
	})
	if err != nil {
		return err
	}
	resp.VolumeID = req.VolumeID
	return nil
}

// run calls op under the delete timeout and the client shutdown. A classified
// rejection goes to reject and returns nil. Any other failure returns an
// error named after method.
func (h *HostVolume) run(method string, op func(context.Context, *hostvolume.Manager) error, reject func(reason, message string)) error {
	m := h.c.hostVolumeManager
	if m == nil {
		return errors.New(method + ": host volume manager is not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), hostVolumeDeleteTimeout)
	defer cancel()
	go func() {
		select {
		case <-h.c.shutdownCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	err := op(ctx, m)
	if err == nil {
		return nil
	}
	if errors.Is(err, hostvolume.ErrRegisteredVolume) {
		reject(hostvolume.ReasonVolumeKindConflict, "volume is registered and has no plugin to delete it")
		return nil
	}
	var oe *hostvolume.OpError
	if errors.As(err, &oe) && oe.Reason != hostvolume.ReasonCanceled {
		reject(oe.Reason, oe.Message)
		return nil
	}
	return errors.New(method + ": " + err.Error())
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/open-wander/wander/client/devicemanager"
	"github.com/open-wander/wander/client/pluginmanager/csimanager"
	"github.com/open-wander/wander/client/pluginmanager/drivermanager"
	"github.com/open-wander/wander/nomad/structs"
)

var (
	// batchFirstFingerprintsTimeout is the maximum amount of time to wait for
	// initial fingerprinting to complete before sending a batched Node update
	batchFirstFingerprintsTimeout = 50 * time.Second
)

// batchFirstFingerprints waits for the first fingerprint response from all
// plugin managers and sends a single Node update for all fingerprints. It
// should only ever be called once
func (c *Client) batchFirstFingerprints() {
	ctx, cancel := context.WithTimeout(context.Background(), batchFirstFingerprintsTimeout)
	defer cancel()

	ch, err := c.pluginManagers.WaitForFirstFingerprint(ctx)
	if err != nil {
		c.logger.Warn("failed to batch initial fingerprint updates, switching to incemental updates")
		goto SEND_BATCH
	}

	// Wait for fingerprinting to complete or timeout before processing batches
	select {
	case <-ch:
	case <-ctx.Done():
	}

SEND_BATCH:
	c.configLock.Lock()
	defer c.configLock.Unlock()

	newConfig := c.config.Copy()

	// csi updates
	var csiChanged bool
	c.batchNodeUpdates.batchCSIUpdates(func(name string, info *structs.CSIInfo) {
		if c.updateNodeFromCSIControllerLocked(name, info, newConfig.Node) {
			if newConfig.Node.CSIControllerPlugins[name].UpdateTime.IsZero() {
				newConfig.Node.CSIControllerPlugins[name].UpdateTime = time.Now()
			}
			csiChanged = true
		}
		if c.updateNodeFromCSINodeLocked(name, info, newConfig.Node) {
			if newConfig.Node.CSINodePlugins[name].UpdateTime.IsZero() {
				newConfig.Node.CSINodePlugins[name].UpdateTime = time.Now()
			}
			csiChanged = true
		}
	})

	// driver node updates
	var driverChanged bool
	c.batchNodeUpdates.batchDriverUpdates(func(driver string, info *structs.DriverInfo) {
		if c.applyNodeUpdatesFromDriver(driver, info, newConfig.Node) {
			newConfig.Node.Drivers[driver] = info
			if newConfig.Node.Drivers[driver].UpdateTime.IsZero() {
				newConfig.Node.Drivers[driver].UpdateTime = time.Now()
			}
			driverChanged = true
		}
	})

	// device node updates
	var devicesChanged bool
	c.batchNodeUpdates.batchDevicesUpdates(func(devices []*structs.NodeDeviceResource) {
		if c.updateNodeFromDevicesLocked(devices) {
			newConfig.Node.NodeResources.Devices = devices
			devicesChanged = true
		}
	})

	// only update the node if changes occurred
	if driverChanged || devicesChanged || csiChanged {
		c.config = newConfig
		c.updateNode()
	}

	close(c.fpInitialized)
}

// updateNodeFromCSI receives a CSIInfo struct for the plugin and updates the
// node accordingly
func (c *Client) updateNodeFromCSI(name string, info *structs.CSIInfo) {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	newConfig := c.config.Copy()

	changed := false

	if c.updateNodeFromCSIControllerLocked(name, info, newConfig.Node) {
		if newConfig.Node.CSIControllerPlugins[name].UpdateTime.IsZero() {
			newConfig.Node.CSIControllerPlugins[name].UpdateTime = time.Now()
		}
		changed = true
	}

	if c.updateNodeFromCSINodeLocked(name, info, newConfig.Node) {
		if newConfig.Node.CSINodePlugins[name].UpdateTime.IsZero() {
			newConfig.Node.CSINodePlugins[name].UpdateTime = time.Now()
		}
		changed = true
	}

	if changed {
		c.config = newConfig
		c.updateNode()
	}
}

// updateNodeFromCSIControllerLocked makes the changes to the node from a csi
// update but does not send the update to the server. c.configLock must be held
// before calling this func.
//
// It is safe to call for all CSI Updates, but will only perform changes when
// a ControllerInfo field is present.
func (c *Client) updateNodeFromCSIControllerLocked(name string, info *structs.CSIInfo, node *structs.Node) bool {
	var changed bool
	if info.ControllerInfo == nil {
		return false
	}
	i := info.Copy()
	i.NodeInfo = nil

	oldController, hadController := node.CSIControllerPlugins[name]
	if !hadController {
		// If the controller info has not yet been set, do that here
		changed = true
		node.CSIControllerPlugins[name] = i
	} else {
		// The controller info has already been set, fix it up
		if !oldController.Equal(i) {
			node.CSIControllerPlugins[name] = i
			changed = true
		}

		// If health state has changed, trigger node event
		if oldController.Healthy != i.Healthy || oldController.HealthDescription != i.HealthDescription {
			changed = true
			if i.HealthDescription != "" {
				event := structs.NewNodeEvent().
					SetSubsystem("CSI").
					SetMessage(i.HealthDescription).
					AddDetail("plugin", name).
					AddDetail("type", "controller")
				c.triggerNodeEvent(event)
			}
		}
	}

	return changed
}

// updateNodeFromCSINodeLocked makes the changes to the node from a csi
// update but does not send the update to the server. c.configLock must be hel
// before calling this func.
//
// It is safe to call for all CSI Updates, but will only perform changes when
// a NodeInfo field is present.
func (c *Client) updateNodeFromCSINodeLocked(name string, info *structs.CSIInfo, node *structs.Node) bool {
	var changed bool
	if info.NodeInfo == nil {
		return false
	}
	i := info.Copy()
	i.ControllerInfo = nil

	oldNode, hadNode := node.CSINodePlugins[name]
	if !hadNode {
		// If the Node info has not yet been set, do that here
		changed = true
		node.CSINodePlugins[name] = i
	} else {
		// The node info has already been set, fix it up
		if !oldNode.Equal(info) {
			node.CSINodePlugins[name] = i
			changed = true
		}

		// If health state has changed, trigger node event
		if oldNode.Healthy != i.Healthy || oldNode.HealthDescription != i.HealthDescription {
			changed = true
			if i.HealthDescription != "" {
				event := structs.NewNodeEvent().
					SetSubsystem("CSI").
					SetMessage(i.HealthDescription).
					AddDetail("plugin", name).
					AddDetail("type", "node")
				c.triggerNodeEvent(event)
			}
		}
	}

	return changed
}

// updateNodeFromDriver receives a DriverInfo struct for the driver and updates
// the node accordingly
func (c *Client) updateNodeFromDriver(name string, info *structs.DriverInfo) {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	newConfig := c.config.Copy()

	if c.applyNodeUpdatesFromDriver(name, info, newConfig.Node) {
		newConfig.Node.Drivers[name] = info
		if newConfig.Node.Drivers[name].UpdateTime.IsZero() {
			newConfig.Node.Drivers[name].UpdateTime = time.Now()
		}

		c.config = newConfig
		c.updateNode()
	}
}

// applyNodeUpdatesFromDriver applies changes to the passed in node. true is
// returned if the node has changed.
func (c *Client) applyNodeUpdatesFromDriver(name string, info *structs.DriverInfo, node *structs.Node) bool {
	var hasChanged bool

	hadDriver := node.Drivers[name] != nil
	if !hadDriver {
		// If the driver info has not yet been set, do that here
		hasChanged = true
		for attrName, newVal := range info.Attributes {
			node.Attributes[attrName] = newVal
		}
	} else {
		oldVal := node.Drivers[name]
		// The driver info has already been set, fix it up
		if oldVal.Detected != info.Detected {
			hasChanged = true
		}

		// If health state has change, trigger node event
		if oldVal.Healthy != info.Healthy || oldVal.HealthDescription != info.HealthDescription {
			hasChanged = true
			if info.HealthDescription != "" {
				event := structs.NewNodeEvent().
					SetSubsystem("Driver").
					SetMessage(info.HealthDescription).
					AddDetail("driver", name)
				c.triggerNodeEvent(event)
			}
		}

		for attrName, newVal := range info.Attributes {
			oldVal := node.Drivers[name].Attributes[attrName]
			if oldVal == newVal {
				continue
			}

			hasChanged = true
			if newVal == "" {
				delete(node.Attributes, attrName)
			} else {
				node.Attributes[attrName] = newVal
			}
		}
	}

	// COMPAT Remove in Nomad 0.10
	// We maintain the driver enabled attribute until all drivers expose
	// their attributes as DriverInfo
	driverName := fmt.Sprintf("driver.%s", name)
	if info.Detected {
		node.Attributes[driverName] = "1"
	} else {
		delete(node.Attributes, driverName)
	}

	return hasChanged
}

func (c *Client) updateNodeFromDevices(devices []*structs.NodeDeviceResource) {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	// Not updating node.Resources: the field is deprecated and includes
	// dispatched task resources and not appropriate for expressing
	// node available device resources
	if c.updateNodeFromDevicesLocked(devices) {
		c.updateNode()
	}
}

// updateNodeFromDevicesLocked updates the node with the results of devices,
// but does send the update to the server. c.configLock must be held before
// calling this func
func (c *Client) updateNodeFromDevicesLocked(devices []*structs.NodeDeviceResource) bool {
	if !structs.DevicesEquals(c.config.Node.NodeResources.Devices, devices) {
		c.logger.Debug("new devices detected", "devices", len(devices))
		newConfig := c.config.Copy()
		newConfig.Node.NodeResources.Devices = devices
		c.config = newConfig
		return true
	}

	return false
}

// updateNodeFromHostVolumes publishes the ready dynamic host volumes in the
// node's HostVolumes map next to the static ones, and triggers a node update
// only when the map changes. Static entries (empty ID) are kept and never
// replaced by a dynamic one. The map is rebuilt, never changed in place, as
// the registration and heartbeat code share it. This is Wander policy.
func (c *Client) updateNodeFromHostVolumes(dyn map[string]*structs.ClientHostVolumeConfig) {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	current := c.config.Node.HostVolumes
	next := make(map[string]*structs.ClientHostVolumeConfig, len(current)+len(dyn))
	for name, v := range current {
		if v != nil && v.ID == "" {
			next[name] = v.Copy()
		}
	}
	for name, v := range dyn {
		if _, static := next[name]; static || v == nil || v.ID == "" {
			continue
		}
		next[name] = v.Copy()
	}

	if hostVolumesEqual(current, next) {
		return
	}
	c.logger.Debug("dynamic host volumes changed", "dynamic", len(dyn))
	newConfig := c.config.Copy()
	if len(next) == 0 {
		next = nil
	}
	newConfig.Node.HostVolumes = next
	c.config = newConfig
	c.updateNode()
}

func hostVolumesEqual(a, b map[string]*structs.ClientHostVolumeConfig) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		w, ok := b[k]
		if !ok || v == nil || w == nil {
			if ok && v == nil && w == nil {
				continue
			}
			return false
		}
		if *v != *w {
			return false
		}
	}
	return true
}

// batchNodeUpdates allows for batching multiple Node updates from fingerprinting.
// Once ready, the batches can be flushed and toggled to stop batching and forward
// all updates to a configured callback to be performed incrementally
type batchNodeUpdates struct {
	// access to driver fields must hold driversMu lock
	drivers        map[string]*structs.DriverInfo
	driversBatched bool
	driverCB       drivermanager.UpdateNodeDriverInfoFn
	driversMu      sync.Mutex

	// access to devices fields must hold devicesMu lock
	devices        []*structs.NodeDeviceResource
	devicesBatched bool
	devicesCB      devicemanager.UpdateNodeDevicesFn
	devicesMu      sync.Mutex

	// access to csi fields must hold csiMu lock
	csiNodePlugins       map[string]*structs.CSIInfo
	csiControllerPlugins map[string]*structs.CSIInfo
	csiBatched           bool
	csiCB                csimanager.UpdateNodeCSIInfoFunc
	csiMu                sync.Mutex
}

func newBatchNodeUpdates(
	driverCB drivermanager.UpdateNodeDriverInfoFn,
	devicesCB devicemanager.UpdateNodeDevicesFn,
	csiCB csimanager.UpdateNodeCSIInfoFunc) *batchNodeUpdates {

	return &batchNodeUpdates{
		drivers:              make(map[string]*structs.DriverInfo),
		driverCB:             driverCB,
		devices:              []*structs.NodeDeviceResource{},
		devicesCB:            devicesCB,
		csiNodePlugins:       make(map[string]*structs.CSIInfo),
		csiControllerPlugins: make(map[string]*structs.CSIInfo),
		csiCB:                csiCB,
	}
}

// updateNodeFromCSI implements csimanager.UpdateNodeCSIInfoFunc and is used in
// the csi manager to send csi fingerprints to the server.
func (b *batchNodeUpdates) updateNodeFromCSI(plugin string, info *structs.CSIInfo) {
	b.csiMu.Lock()
	defer b.csiMu.Unlock()
	if b.csiBatched {
		b.csiCB(plugin, info)
		return
	}

	// Only one of these is expected to be set, but a future implementation that
	// explicitly models monolith plugins with a single fingerprinter may set both
	if info.ControllerInfo != nil {
		b.csiControllerPlugins[plugin] = info
	}

	if info.NodeInfo != nil {
		b.csiNodePlugins[plugin] = info
	}
}

// batchCSIUpdates sends all of the batched CSI updates by calling f  for each
// plugin batched
func (b *batchNodeUpdates) batchCSIUpdates(f csimanager.UpdateNodeCSIInfoFunc) error {
	b.csiMu.Lock()
	defer b.csiMu.Unlock()
	if b.csiBatched {
		return fmt.Errorf("csi updates already batched")
	}

	b.csiBatched = true
	for plugin, info := range b.csiNodePlugins {
		f(plugin, info)
	}
	for plugin, info := range b.csiControllerPlugins {
		f(plugin, info)
	}
	return nil
}

// updateNodeFromDriver implements drivermanager.UpdateNodeDriverInfoFn and is
// used in the driver manager to send driver fingerprints to
func (b *batchNodeUpdates) updateNodeFromDriver(driver string, info *structs.DriverInfo) {
	b.driversMu.Lock()
	defer b.driversMu.Unlock()
	if b.driversBatched {
		b.driverCB(driver, info)
		return
	}

	b.drivers[driver] = info
}

// batchDriverUpdates sends all of the batched driver node updates by calling f
// for each driver batched
func (b *batchNodeUpdates) batchDriverUpdates(f drivermanager.UpdateNodeDriverInfoFn) error {
	b.driversMu.Lock()
	defer b.driversMu.Unlock()
	if b.driversBatched {
		return fmt.Errorf("driver updates already batched")
	}

	b.driversBatched = true
	for driver, info := range b.drivers {
		f(driver, info)
	}
	return nil
}

// updateNodeFromDevices implements devicemanager.UpdateNodeDevicesFn and is
// used in the device manager to send device fingerprints to
func (b *batchNodeUpdates) updateNodeFromDevices(devices []*structs.NodeDeviceResource) {
	b.devicesMu.Lock()
	defer b.devicesMu.Unlock()
	if b.devicesBatched {
		b.devicesCB(devices)
		return
	}

	b.devices = devices
}

// batchDevicesUpdates sends the batched device node updates by calling f with
// the devices
func (b *batchNodeUpdates) batchDevicesUpdates(f devicemanager.UpdateNodeDevicesFn) error {
	b.devicesMu.Lock()
	defer b.devicesMu.Unlock()
	if b.devicesBatched {
		return fmt.Errorf("devices updates already batched")
	}

	b.devicesBatched = true
	f(b.devices)
	return nil
}

// hostVolumesFor returns the host volumes of the node as they are now that an
// allocation of the namespace may mount. Node.HostVolumes carries no
// namespace, so a dynamic entry is kept only when the record that the volume
// manager holds for its ID is in that namespace. An entry whose record is
// missing, for example because the volume was deleted since the node was
// updated, is dropped too. This is Wander policy: the server already places
// an allocation only on a volume of its namespace, and this keeps a client
// from mounting a volume of another namespace if that check were bypassed.
//
// The node's map is rebuilt for every update and never changed in place, so
// it is read without the config lock and is not modified here.
func (c *Client) hostVolumesFor(namespace string) map[string]*structs.ClientHostVolumeConfig {
	all := c.GetConfig().Node.HostVolumes
	if c.hostVolumeManager == nil {
		return all
	}

	out := make(map[string]*structs.ClientHostVolumeConfig, len(all))
	for name, v := range all {
		if v != nil && v.ID != "" {
			if ns, ok := c.hostVolumeManager.Namespace(v.ID); !ok || ns != namespace {
				continue
			}
		}
		out[name] = v
	}
	return out
}

var closedHostVolumesRestored = func() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()

// hostVolumesRestored returns a channel that is closed once the volume
// manager has ended its first restore. Without a manager there is nothing to
// restore, so the channel is already closed.
func (c *Client) hostVolumesRestored() <-chan struct{} {
	if c.hostVolumeManager == nil {
		return closedHostVolumesRestored
	}
	return c.hostVolumeManager.Restored()
}

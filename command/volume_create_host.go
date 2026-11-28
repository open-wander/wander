// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"fmt"

	humanize "github.com/dustin/go-humanize"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/open-wander/wander/api"
)

// hostCreate asks for a volume to be created by a plugin. A volume with an ID
// in its specification updates the stored volume. Unless detach is set, it
// follows the volume until it is ready or failed. There is no timeout: the
// monitor ends with the process, as the other inherited monitors do.
func (c *VolumeCreateCommand) hostCreate(client *api.Client, input *ast.File, detach, verbose, policyOverride bool) int {
	vol, err := hostDecodeVolume(input)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error decoding the volume definition: %s", err))
		return 1
	}

	resp, _, err := client.HostVolumes().Create(&api.HostVolumeCreateRequest{
		Volume:         vol,
		PolicyOverride: policyOverride,
	}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating volume: %s", err))
		return 1
	}
	if resp.Warnings != "" {
		c.Ui.Warn(c.FormatWarnings("Volume", resp.Warnings))
	}
	if resp.Volume == nil {
		c.Ui.Error("Error creating volume: the reply has no volume")
		return 1
	}

	created := resp.Volume
	c.Ui.Output(fmt.Sprintf("Created host volume %s", created.ID))
	if detach {
		return 0
	}
	return c.hostMonitor(client, created, verbose)
}

// hostMonitor prints each state change of the volume until it is ready or
// failed. A reply that is ready or failed already needs no query. The query is
// for the namespace of the volume, because the namespace of the client may
// differ, and blocks on the index of the last reply.
func (c *VolumeCreateCommand) hostMonitor(client *api.Client, vol *api.HostVolume, verbose bool) int {
	id, namespace, index := vol.ID, vol.Namespace, vol.ModifyIndex
	last := api.HostVolumeState("")
	started := false

	for {
		terminal := vol.State == api.HostVolumeStateReady || vol.State == api.HostVolumeStateFailed
		if vol.State != last && (terminal || started) {
			c.Ui.Output(fmt.Sprintf("Volume %s is %s", id, vol.State))
			if verbose {
				c.Ui.Output(formatHostVolumeVerbose(vol))
			}
		}
		last, started = vol.State, true

		switch vol.State {
		case api.HostVolumeStateReady:
			return 0
		case api.HostVolumeStateFailed:
			c.Ui.Error(fmt.Sprintf("Volume %s failed: %s", id, vol.StatusDescription))
			return 1
		}

		next, meta, err := client.HostVolumes().Info(id, &api.QueryOptions{
			Namespace: namespace,
			WaitIndex: index,
		})
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error monitoring volume: %s", err))
			return 1
		}
		if meta.LastIndex > index {
			index = meta.LastIndex
		}
		vol = next
	}
}

// formatHostVolumeVerbose lists the fields of a volume for -verbose.
func formatHostVolumeVerbose(vol *api.HostVolume) string {
	return formatKV([]string{
		fmt.Sprintf("ID|%s", vol.ID),
		fmt.Sprintf("Name|%s", vol.Name),
		fmt.Sprintf("Namespace|%s", vol.Namespace),
		fmt.Sprintf("Plugin ID|%s", vol.PluginID),
		fmt.Sprintf("Node ID|%s", vol.NodeID),
		fmt.Sprintf("Node Pool|%s", vol.NodePool),
		fmt.Sprintf("Capacity|%s", humanize.IBytes(uint64(max(vol.CapacityBytes, 0)))),
		fmt.Sprintf("State|%s", vol.State),
		fmt.Sprintf("Description|%s", vol.StatusDescription),
	})
}

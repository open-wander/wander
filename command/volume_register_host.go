// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"errors"
	"fmt"
	"math"

	humanize "github.com/dustin/go-humanize"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/hcl/hcl/token"
	"github.com/open-wander/wander/api"
)

// The errors of the host volume decoder have fixed texts. They never carry a
// host path, a parameter, an unknown key or a value from the specification.
var (
	errHostSpecRoot       = errors.New("volume specification must be an object")
	errHostSpecUnknownKey = errors.New("volume specification has an unknown key")
	errHostSpecTwice      = errors.New("volume specification gives a key twice")
)

// hostDecodeVolume decodes a host volume specification. The HCL and JSON forms
// parse to the same syntax tree, so one decoder serves both. Only the keys
// below are accepted, at every level, and a key that may be given once is
// refused when it is given twice.
func hostDecodeVolume(input *ast.File) (*api.HostVolume, error) {
	list, ok := input.Node.(*ast.ObjectList)
	if !ok {
		return nil, errHostSpecRoot
	}

	vol := &api.HostVolume{}
	scalars := map[string]*string{
		"id":        &vol.ID,
		"name":      &vol.Name,
		"namespace": &vol.Namespace,
		"plugin_id": &vol.PluginID,
		"node_id":   &vol.NodeID,
		"node_pool": &vol.NodePool,
		"host_path": &vol.HostPath,
	}
	sizes := map[string]*int64{
		"capacity":     &vol.CapacityBytes,
		"capacity_min": &vol.RequestedCapacityMinBytes,
		"capacity_max": &vol.RequestedCapacityMaxBytes,
	}

	seen := map[string]bool{}
	for _, item := range list.Items {
		if len(item.Keys) != 1 {
			return nil, errHostSpecUnknownKey
		}
		key := item.Keys[0].Token.Value().(string)

		_, isScalar := scalars[key]
		_, isSize := sizes[key]
		switch {
		case isScalar || isSize || key == "type" || key == "parameters":
			if seen[key] {
				return nil, errHostSpecTwice
			}
			seen[key] = true
		case key == "capability" || key == "constraint":
		default:
			return nil, errHostSpecUnknownKey
		}

		switch {
		case isScalar:
			s, ok := hostStringValue(item.Val)
			if !ok {
				return nil, fmt.Errorf("%s must be a string", key)
			}
			*scalars[key] = s
		case isSize:
			b, err := hostSizeValue(item.Val)
			if err != nil {
				return nil, fmt.Errorf("%s is not a valid size", key)
			}
			*sizes[key] = b
		case key == "type":
			// The dispatch has read the type already, and the server does
			// not take it from the request.
			if _, ok := hostStringValue(item.Val); !ok {
				return nil, errors.New("type must be a string")
			}
		case key == "parameters":
			params, err := hostStringMap(item.Val, "parameters", false)
			if err != nil {
				return nil, err
			}
			vol.Parameters = params
		case key == "capability":
			m, err := hostStringMap(item.Val, "capability", true, "access_mode", "attachment_mode")
			if err != nil {
				return nil, err
			}
			vol.RequestedCapabilities = append(vol.RequestedCapabilities, &api.HostVolumeCapability{
				AccessMode:     api.HostVolumeAccessMode(m["access_mode"]),
				AttachmentMode: api.HostVolumeAttachmentMode(m["attachment_mode"]),
			})
		case key == "constraint":
			m, err := hostStringMap(item.Val, "constraint", true, "attribute", "operator", "value")
			if err != nil {
				return nil, err
			}
			vol.Constraints = append(vol.Constraints, &api.Constraint{
				LTarget: m["attribute"],
				Operand: m["operator"],
				RTarget: m["value"],
			})
		}
	}

	return vol, nil
}

// hostStringValue returns the value of a quoted string.
func hostStringValue(n ast.Node) (string, bool) {
	lit, ok := n.(*ast.LiteralType)
	if !ok || lit.Token.Type != token.STRING {
		return "", false
	}
	s, ok := lit.Token.Value().(string)
	return s, ok
}

// hostSizeValue parses a size such as "10GiB". An empty string is zero, as for
// CSI volumes.
func hostSizeValue(n ast.Node) (int64, error) {
	s, ok := hostStringValue(n)
	if !ok {
		return 0, errors.New("not a size")
	}
	if s == "" {
		return 0, nil
	}
	b, err := humanize.ParseBytes(s)
	if err != nil || b > math.MaxInt64 {
		return 0, errors.New("not a size")
	}
	return int64(b), nil
}

// hostStringMap decodes a block of string values. With allowed set, only
// those keys are accepted and any other is an unknown key; without it, any
// key is a parameter. A key given twice is refused either way.
func hostStringMap(n ast.Node, name string, restrict bool, allowed ...string) (map[string]string, error) {
	obj, ok := n.(*ast.ObjectType)
	if !ok {
		return nil, fmt.Errorf("%s must be a block", name)
	}
	valueErr := fmt.Errorf("%s values must be strings", name)
	if !restrict {
		valueErr = fmt.Errorf("%s must be strings", name)
	}

	out := map[string]string{}
	for _, item := range obj.List.Items {
		if len(item.Keys) != 1 {
			return nil, fmt.Errorf("%s has an unknown key", name)
		}
		key := item.Keys[0].Token.Value().(string)
		if restrict {
			known := false
			for _, a := range allowed {
				known = known || a == key
			}
			if !known {
				return nil, fmt.Errorf("%s has an unknown key", name)
			}
		}
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("%s gives a key twice", name)
		}
		s, ok := hostStringValue(item.Val)
		if !ok {
			return nil, valueErr
		}
		out[key] = s
	}
	return out, nil
}

// hostRegister registers a volume that exists on a node already. A volume
// with an ID in its specification updates the stored volume. No storage is
// created or changed, and the command does not monitor the volume.
func (c *VolumeRegisterCommand) hostRegister(client *api.Client, input *ast.File, policyOverride bool) int {
	vol, err := hostDecodeVolume(input)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error decoding the volume definition: %s", err))
		return 1
	}

	resp, _, err := client.HostVolumes().Register(&api.HostVolumeRegisterRequest{
		Volume:         vol,
		PolicyOverride: policyOverride,
	}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error registering volume: %s", err))
		return 1
	}
	if resp.Warnings != "" {
		c.Ui.Warn(c.FormatWarnings("Volume", resp.Warnings))
	}

	id := vol.ID
	if resp.Volume != nil {
		id = resp.Volume.ID
	}
	c.Ui.Output(fmt.Sprintf("Registered host volume %s; no storage was created or modified", id))
	return 0
}

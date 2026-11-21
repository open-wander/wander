// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
)

// These codes describe internal validation failures, not HTTP status/envelopes.
var (
	ErrDynamicHostVolumeRequired    = errors.New("required")
	ErrDynamicHostVolumeUnsupported = errors.New("unsupported")
	ErrDynamicHostVolumeInvalid     = errors.New("invalid")
	ErrDynamicHostVolumeImmutable   = errors.New("immutable")
)

// ValidateCreate checks an already-resolved internal create record. HostPath is
// deliberately ignored; this method neither provisions storage nor sets defaults.
func (v *DynamicHostVolume) ValidateCreate() error {
	result := v.validateRecord()
	if v != nil {
		result = multierror.Append(result, dynamicHostVolumeRequired([2]string{"PluginID", v.PluginID}))
	}
	return result.ErrorOrNil()
}

// ValidateRegister checks an internal record for existing storage. HostPath is
// inspected lexically only; node existence and storage availability are deferred.
func (v *DynamicHostVolume) ValidateRegister() error {
	result := v.validateRecord()
	if v != nil {
		result = multierror.Append(result, dynamicHostVolumeRequired(
			[2]string{"NodeID", v.NodeID}, [2]string{"HostPath", v.HostPath}))
		if v.HostPath != "" && (!filepath.IsAbs(v.HostPath) || strings.ContainsRune(v.HostPath, 0)) {
			result.Errors = append(result.Errors, fmt.Errorf("HostPath: %w", ErrDynamicHostVolumeInvalid))
		}
	}
	return result.ErrorOrNil()
}

func (v *DynamicHostVolume) validateRecord() *multierror.Error {
	if v == nil {
		return dynamicHostVolumeRequired([2]string{"Volume", ""})
	}
	result := dynamicHostVolumeRequired(
		[2]string{"Name", v.Name}, [2]string{"Namespace", v.Namespace}, [2]string{"Type", v.Type})
	if v.Type != "" && v.Type != VolumeTypeHost {
		result.Errors = append(result.Errors, fmt.Errorf("Type: %w", ErrDynamicHostVolumeUnsupported))
	}
	for _, capacity := range []struct {
		field string
		value int64
	}{
		{"CapacityBytes", v.CapacityBytes},
		{"RequestedCapacityMinBytes", v.RequestedCapacityMinBytes},
		{"RequestedCapacityMaxBytes", v.RequestedCapacityMaxBytes},
	} {
		if capacity.value < 0 {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", capacity.field, ErrDynamicHostVolumeInvalid))
		}
	}
	// Zero is compared literally. This internal rule does not resolve any public
	// interpretation of an omitted maximum or authorize a capacity change.
	if v.RequestedCapacityMinBytes > v.RequestedCapacityMaxBytes {
		result.Errors = append(result.Errors, fmt.Errorf(
			"RequestedCapacityMinBytes: %w (exceeds RequestedCapacityMaxBytes)", ErrDynamicHostVolumeInvalid))
	}
	if len(v.RequestedCapabilities) == 0 {
		result.Errors = append(result.Errors, fmt.Errorf("RequestedCapabilities: %w", ErrDynamicHostVolumeRequired))
	}
	result = multierror.Append(result, dynamicHostVolumeValidateElements("RequestedCapabilities", v.RequestedCapabilities))
	for _, constraint := range v.Constraints {
		if constraint == nil {
			result.Errors = append(result.Errors, fmt.Errorf("Constraints: %w", ErrDynamicHostVolumeRequired))
		} else if constraint.Validate() != nil {
			// Underlying constraint errors may contain arbitrary input. Preserve
			// only our code and fixed field, including in the error chain.
			result.Errors = append(result.Errors, fmt.Errorf("Constraints: %w", ErrDynamicHostVolumeInvalid))
		}
	}
	result = multierror.Append(result, dynamicHostVolumeValidateElements("Attachments", v.Attachments))
	return result
}

func dynamicHostVolumeValidateElements[T interface{ Validate() error }](field string, values []T) *multierror.Error {
	var result multierror.Error
	for _, value := range values {
		if err := value.Validate(); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", field, err))
		}
	}
	return &result
}

func (c *DynamicHostVolumeCapability) Validate() error {
	if c == nil {
		return fmt.Errorf("Capability: %w", ErrDynamicHostVolumeRequired)
	}
	return dynamicHostVolumeValidateModes(c.AttachmentMode, c.AccessMode).ErrorOrNil()
}

func (a *DynamicHostVolumeAttachment) Validate() error {
	if a == nil {
		return fmt.Errorf("Attachment: %w", ErrDynamicHostVolumeRequired)
	}
	result := dynamicHostVolumeRequired(
		[2]string{"AllocationID", a.AllocationID}, [2]string{"NodeID", a.NodeID})
	result = multierror.Append(result, dynamicHostVolumeValidateModes(a.AttachmentMode, a.AccessMode))
	if a.AccessMode == DynamicHostVolumeAccessModeSingleNodeReader && !a.ReadOnly {
		result.Errors = append(result.Errors, fmt.Errorf("ReadOnly: %w", ErrDynamicHostVolumeInvalid))
	}
	return result.ErrorOrNil()
}

func (c *DynamicHostVolumeClaim) Validate() error {
	if c == nil {
		return fmt.Errorf("Claim: %w", ErrDynamicHostVolumeRequired)
	}
	return dynamicHostVolumeRequired(
		[2]string{"ID", c.ID}, [2]string{"Namespace", c.Namespace}, [2]string{"JobID", c.JobID},
		[2]string{"TaskGroupName", c.TaskGroupName}, [2]string{"VolumeName", c.VolumeName},
		[2]string{"VolumeID", c.VolumeID}).ErrorOrNil()
}

// ValidateImmutableFields rejects changes to identity and creation metadata.
// A nil error does not authorize any other update: operation-specific policy,
// ownership, capacity expansion, and lifecycle safety belong to later work.
func (v *DynamicHostVolume) ValidateImmutableFields(next *DynamicHostVolume) error {
	var result multierror.Error
	if v == nil {
		result.Errors = append(result.Errors, fmt.Errorf("Current: %w", ErrDynamicHostVolumeRequired))
	}
	if next == nil {
		result.Errors = append(result.Errors, fmt.Errorf("Next: %w", ErrDynamicHostVolumeRequired))
	}
	if v == nil || next == nil {
		return result.ErrorOrNil()
	}
	for _, field := range []struct {
		name string
		same bool
	}{
		{"ID", v.ID == next.ID},
		{"Namespace", v.Namespace == next.Namespace},
		{"Name", v.Name == next.Name},
		{"Type", v.Type == next.Type},
		{"NodeID", v.NodeID == next.NodeID},
		{"Origin", v.Origin == next.Origin},
		{"CreateIndex", v.CreateIndex == next.CreateIndex},
		{"CreateTime", v.CreateTime == next.CreateTime},
	} {
		if !field.same {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", field.name, ErrDynamicHostVolumeImmutable))
		}
	}
	return result.ErrorOrNil()
}

func dynamicHostVolumeRequired(fields ...[2]string) *multierror.Error {
	var result multierror.Error
	for _, field := range fields {
		if field[1] == "" {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", field[0], ErrDynamicHostVolumeRequired))
		}
	}
	return &result
}

func dynamicHostVolumeValidateModes(attachment DynamicHostVolumeAttachmentMode, access DynamicHostVolumeAccessMode) *multierror.Error {
	return multierror.Append(nil,
		dynamicHostVolumeModeError("AttachmentMode", string(attachment), string(DynamicHostVolumeAttachmentModeFilesystem)),
		dynamicHostVolumeModeError("AccessMode", string(access),
			string(DynamicHostVolumeAccessModeSingleNodeReader), string(DynamicHostVolumeAccessModeSingleNodeWriter),
			string(DynamicHostVolumeAccessModeSingleNodeSingleWriter), string(DynamicHostVolumeAccessModeSingleNodeMultiWriter)))
}

func dynamicHostVolumeModeError(field, value string, supported ...string) error {
	if value == "" {
		return fmt.Errorf("%s: %w", field, ErrDynamicHostVolumeRequired)
	}
	if slices.Contains(supported, value) {
		return nil
	}
	// Bound bytes before quoting so controls, invalid UTF-8, and large values
	// cannot escape diagnostics or produce unbounded per-value output.
	const limit = 32
	if len(value) > limit {
		value = value[:limit] + "..."
	}
	return fmt.Errorf("%s: %w (%s)", field, ErrDynamicHostVolumeUnsupported, strconv.QuoteToASCII(value))
}

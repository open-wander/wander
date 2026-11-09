// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/require"
)

func validDynamicHostVolumeForValidation() *DynamicHostVolume {
	return &DynamicHostVolume{
		Name: "data-synthetic-a", Namespace: "dhv-synthetic-a", Type: VolumeTypeHost,
		PluginID: "plugin-synthetic-a", NodeID: "node-synthetic-a", HostPath: "/srv/wander-synthetic/missing/../data-a",
		RequestedCapabilities: []*DynamicHostVolumeCapability{
			{AttachmentMode: DynamicHostVolumeAttachmentModeFilesystem, AccessMode: DynamicHostVolumeAccessModeSingleNodeWriter},
		},
		Constraints: []*Constraint{{LTarget: "synthetic-attribute", RTarget: "synthetic-value", Operand: "="}},
		Parameters:  map[string]string{"synthetic-key": "synthetic-value"},
	}
}

func requireDynamicHostVolumeValidationError(t *testing.T, err, code error, fields ...string) {
	t.Helper()
	require.Error(t, err)
	require.True(t, errors.Is(err, code), "missing sentinel %v in %v", code, err)
	for _, field := range fields {
		require.Contains(t, err.Error(), field)
	}
}

func dynamicHostVolumeValidationStringField[T interface{ Copy() T }](model T, field, value string) T {
	changed := model.Copy()
	reflect.ValueOf(changed).Elem().FieldByName(field).SetString(value)
	return changed
}

func TestDynamicHostVolumeOperationValidation(t *testing.T) {
	for _, operation := range []string{"create", "register"} {
		t.Run(operation, func(t *testing.T) {
			validate := func(volume *DynamicHostVolume) error {
				if operation == "create" {
					return volume.ValidateCreate()
				}
				return volume.ValidateRegister()
			}
			valid := validDynamicHostVolumeForValidation()
			if operation == "create" {
				valid.NodeID = ""
				valid.HostPath = "synthetic-ignored-relative\x00"
			} else {
				valid.PluginID = ""
			}
			attachment := &DynamicHostVolumeAttachment{AllocationID: "allocation-synthetic-a", NodeID: "node-synthetic-a",
				AttachmentMode: DynamicHostVolumeAttachmentModeFilesystem, AccessMode: DynamicHostVolumeAccessModeSingleNodeWriter}
			valid.Attachments = []*DynamicHostVolumeAttachment{attachment, attachment.Copy()}
			baseline := valid.Copy()
			require.NoError(t, validate(valid))
			require.Equal(t, baseline, valid, "operation validation is pure")
			required := []string{"Name", "Namespace", "Type"}
			if operation == "create" {
				required = append(required, "PluginID")
			} else {
				required = append(required, "NodeID", "HostPath")
			}
			for _, field := range required {
				t.Run(field, func(t *testing.T) {
					volume := dynamicHostVolumeValidationStringField(valid, field, "")
					before := volume.Copy()
					requireDynamicHostVolumeValidationError(t, validate(volume), ErrDynamicHostVolumeRequired, field)
					require.Equal(t, before, volume)
				})
			}
			requireDynamicHostVolumeValidationError(t, validate(nil), ErrDynamicHostVolumeRequired, "Volume")
			for _, field := range []string{"RequestedCapabilities", "Constraints", "Attachments"} {
				volume := valid.Copy()
				values := reflect.ValueOf(volume).Elem().FieldByName(field)
				values.Set(reflect.Append(values, reflect.Zero(values.Type().Elem())))
				requireDynamicHostVolumeValidationError(t, validate(volume), ErrDynamicHostVolumeRequired, field)
			}
			for _, change := range []struct {
				name   string
				mutate func(*DynamicHostVolume)
				field  string
				code   error
			}{
				{"type", func(v *DynamicHostVolume) { v.Type = "synthetic-other-type" }, "Type", ErrDynamicHostVolumeUnsupported},
				{"no-capabilities", func(v *DynamicHostVolume) { v.RequestedCapabilities = nil }, "RequestedCapabilities", ErrDynamicHostVolumeRequired},
				{"empty-capabilities", func(v *DynamicHostVolume) { v.RequestedCapabilities = []*DynamicHostVolumeCapability{} }, "RequestedCapabilities", ErrDynamicHostVolumeRequired},
				{"unsupported-capability", func(v *DynamicHostVolume) { v.RequestedCapabilities[0].AccessMode = "synthetic-future" }, "AccessMode", ErrDynamicHostVolumeUnsupported},
				{"bad-constraint", func(v *DynamicHostVolume) { v.Constraints[0].Operand = "synthetic-unsupported" }, "Constraints", ErrDynamicHostVolumeInvalid},
				{"invalid-attachment", func(v *DynamicHostVolume) { v.Attachments = []*DynamicHostVolumeAttachment{{}} }, "Attachments", ErrDynamicHostVolumeRequired},
			} {
				t.Run(change.name, func(t *testing.T) {
					volume := valid.Copy()
					change.mutate(volume)
					before := volume.Copy()
					requireDynamicHostVolumeValidationError(t, validate(volume), change.code, change.field)
					require.Equal(t, before, volume)
				})
			}
		})
	}
}

func TestDynamicHostVolumeCapacityValidation(t *testing.T) {
	for _, capacities := range []struct {
		name                     string
		actual, minimum, maximum int64
		invalidField             string
	}{
		{"zero", 0, 0, 0, ""},
		{"minimum-zero", 0, 0, 1048576, ""},
		{"equal-bounds", 0, 1048576, 1048576, ""},
		{"positive-minimum-zero-maximum", 0, 1048576, 0, "RequestedCapacityMinBytes"},
		{"inverted", 0, 2097152, 1048576, "RequestedCapacityMinBytes"},
		{"negative-actual", -1, 0, 0, "CapacityBytes"},
		{"negative-minimum", 0, -1, 0, "RequestedCapacityMinBytes"},
		{"negative-maximum", 0, 0, -1, "RequestedCapacityMaxBytes"},
	} {
		t.Run(capacities.name, func(t *testing.T) {
			volume := validDynamicHostVolumeForValidation()
			volume.CapacityBytes, volume.RequestedCapacityMinBytes, volume.RequestedCapacityMaxBytes =
				capacities.actual, capacities.minimum, capacities.maximum
			before := volume.Copy()
			for _, err := range []error{volume.ValidateCreate(), volume.ValidateRegister()} {
				if capacities.invalidField == "" {
					require.NoError(t, err)
				} else {
					requireDynamicHostVolumeValidationError(t, err, ErrDynamicHostVolumeInvalid, capacities.invalidField)
				}
			}
			require.Equal(t, before, volume)
		})
	}
}

func TestDynamicHostVolumePathValidation(t *testing.T) {
	for _, path := range []string{"/srv/wander-synthetic/does-not-exist", "/srv/wander-synthetic/../data-a", "//synthetic-root/data-a", "/"} {
		volume := validDynamicHostVolumeForValidation()
		volume.HostPath = path
		before := volume.Copy()
		require.NoError(t, volume.ValidateRegister(), "lexical checks must not inspect or normalize storage")
		require.Equal(t, before, volume)
	}
	for _, path := range []string{"relative-synthetic-path", "/srv/synthetic\x00hidden"} {
		volume := validDynamicHostVolumeForValidation()
		volume.HostPath = path
		err := volume.ValidateRegister()
		requireDynamicHostVolumeValidationError(t, err, ErrDynamicHostVolumeInvalid, "HostPath")
		require.NotContains(t, err.Error(), path)
		require.NoError(t, volume.ValidateCreate(), "create ignores HostPath")
	}
}

func TestDynamicHostVolumeCapabilityAndAttachmentValidation(t *testing.T) {
	for _, access := range []DynamicHostVolumeAccessMode{DynamicHostVolumeAccessModeSingleNodeReader, DynamicHostVolumeAccessModeSingleNodeWriter} {
		capability := &DynamicHostVolumeCapability{AttachmentMode: DynamicHostVolumeAttachmentModeFilesystem, AccessMode: access}
		require.NoError(t, capability.Validate())
		attachment := &DynamicHostVolumeAttachment{AllocationID: "allocation-synthetic-a", NodeID: "node-synthetic-a",
			AttachmentMode: capability.AttachmentMode, AccessMode: access, ReadOnly: true}
		baseline := attachment.Copy()
		require.NoError(t, attachment.Validate())
		require.Equal(t, baseline, attachment)
		attachment.ReadOnly = false
		if access == DynamicHostVolumeAccessModeSingleNodeReader {
			requireDynamicHostVolumeValidationError(t, attachment.Validate(), ErrDynamicHostVolumeInvalid, "ReadOnly")
		} else {
			require.NoError(t, attachment.Validate())
		}
		for _, field := range []string{"AllocationID", "NodeID", "AttachmentMode", "AccessMode"} {
			changed := dynamicHostVolumeValidationStringField(baseline, field, "")
			requireDynamicHostVolumeValidationError(t, changed.Validate(), ErrDynamicHostVolumeRequired, field)
			changed = dynamicHostVolumeValidationStringField(baseline, field, "synthetic-future")
			if field == "AttachmentMode" || field == "AccessMode" {
				requireDynamicHostVolumeValidationError(t, changed.Validate(), ErrDynamicHostVolumeUnsupported, field)
			} else {
				require.NoError(t, changed.Validate(), "identity fields require presence only")
			}
		}
	}
	var nilCapability *DynamicHostVolumeCapability
	var nilAttachment *DynamicHostVolumeAttachment
	requireDynamicHostVolumeValidationError(t, nilCapability.Validate(), ErrDynamicHostVolumeRequired, "Capability")
	requireDynamicHostVolumeValidationError(t, nilAttachment.Validate(), ErrDynamicHostVolumeRequired, "Attachment")
	for _, field := range []string{"AttachmentMode", "AccessMode"} {
		capability := &DynamicHostVolumeCapability{AttachmentMode: DynamicHostVolumeAttachmentModeFilesystem, AccessMode: DynamicHostVolumeAccessModeSingleNodeWriter}
		value := reflect.ValueOf(capability).Elem().FieldByName(field)
		value.SetString("")
		requireDynamicHostVolumeValidationError(t, capability.Validate(), ErrDynamicHostVolumeRequired, field)
		value.SetString("synthetic-future")
		err := capability.Validate()
		requireDynamicHostVolumeValidationError(t, err, ErrDynamicHostVolumeUnsupported, field)
		require.Contains(t, err.Error(), `"synthetic-future"`)
		value.SetString(strings.Repeat("synthetic-unsupported\n\x00", 100))
		err = capability.Validate()
		requireDynamicHostVolumeValidationError(t, err, ErrDynamicHostVolumeUnsupported, field)
		require.Less(t, len(err.Error()), 256)
		require.NotContains(t, err.Error(), "\x00")
		require.NotContains(t, err.Error(), "synthetic-unsupported\n")
	}
}

func TestDynamicHostVolumeClaimValidation(t *testing.T) {
	claim := syntheticDynamicHostVolumeClaim()
	claim.AllocationID, claim.AllocationIndex = "", 0
	baseline := claim.Copy()
	require.NoError(t, claim.Validate())
	require.Equal(t, baseline, claim)
	for _, field := range []string{"ID", "Namespace", "JobID", "TaskGroupName", "VolumeName", "VolumeID"} {
		t.Run(field, func(t *testing.T) {
			changed := dynamicHostVolumeValidationStringField(claim, field, "")
			requireDynamicHostVolumeValidationError(t, changed.Validate(), ErrDynamicHostVolumeRequired, field)
		})
	}
	var nilClaim *DynamicHostVolumeClaim
	requireDynamicHostVolumeValidationError(t, nilClaim.Validate(), ErrDynamicHostVolumeRequired, "Claim")
}

func TestDynamicHostVolumeImmutableValidation(t *testing.T) {
	current := validDynamicHostVolumeForValidation()
	current.ID, current.Origin, current.CreateIndex, current.CreateTime = "volume-synthetic-a", DynamicHostVolumeOriginRegistered, 11, 17
	for _, field := range []string{"ID", "Namespace", "Name", "Type", "NodeID", "Origin", "CreateIndex", "CreateTime"} {
		t.Run(field, func(t *testing.T) {
			for _, clear := range []bool{false, true} {
				next := current.Copy()
				value := reflect.ValueOf(next).Elem().FieldByName(field)
				if clear {
					value.SetZero()
				} else {
					switch value.Kind() {
					case reflect.String:
						value.SetString("synthetic-changed")
					case reflect.Int64:
						value.SetInt(value.Int() + 1)
					case reflect.Uint64:
						value.SetUint(value.Uint() + 1)
					}
				}
				beforeCurrent, beforeNext := current.Copy(), next.Copy()
				requireDynamicHostVolumeValidationError(t, current.ValidateImmutableFields(next), ErrDynamicHostVolumeImmutable, field)
				require.Equal(t, beforeCurrent, current)
				require.Equal(t, beforeNext, next)
			}
		})
	}
	var absent *DynamicHostVolume
	for _, pair := range [][2]*DynamicHostVolume{{nil, current}, {current, nil}, {absent, nil}} {
		requireDynamicHostVolumeValidationError(t, pair[0].ValidateImmutableFields(pair[1]), ErrDynamicHostVolumeRequired)
	}
	next := current.Copy()
	next.PluginID, next.HostPath, next.NodePool = "synthetic-other-plugin", "synthetic-other-path", "synthetic-other-pool"
	next.Parameters, next.RequestedCapabilities = nil, nil
	next.CapacityBytes, next.RequestedCapacityMinBytes, next.RequestedCapacityMaxBytes = 1, 2, 3
	next.ModifyIndex, next.ModifyTime, next.State, next.Attachments, next.Constraints = 19, 23, DynamicHostVolumeStateFailed, nil, nil
	baseline := next.Copy()
	require.NoError(t, current.ValidateImmutableFields(next), "this guard does not authorize unrelated changes")
	require.Equal(t, baseline, next)
}

func TestDynamicHostVolumeValidationAggregationAndRedaction(t *testing.T) {
	volume := &DynamicHostVolume{}
	err := volume.ValidateCreate()
	var aggregate *multierror.Error
	require.ErrorAs(t, err, &aggregate)
	requireDynamicHostVolumeValidationError(t, err, ErrDynamicHostVolumeRequired)
	messages := make([]string, len(aggregate.Errors))
	for index, inner := range aggregate.Errors {
		messages[index] = inner.Error()
	}
	require.Equal(t, []string{"Name: required", "Namespace: required", "Type: required", "RequestedCapabilities: required", "PluginID: required"}, messages)
	for index := 0; index < 10; index++ {
		require.Equal(t, err.Error(), volume.ValidateCreate().Error(), "aggregation order is deterministic")
	}
	registerErr := volume.ValidateRegister()
	require.ErrorAs(t, registerErr, &aggregate)
	messages = messages[:0]
	for _, inner := range aggregate.Errors {
		messages = append(messages, inner.Error())
	}
	require.Equal(t, []string{"Name: required", "Namespace: required", "Type: required", "RequestedCapabilities: required", "NodeID: required", "HostPath: required"}, messages)
	for _, constraint := range []*Constraint{
		{LTarget: "synthetic-hidden-left", Operand: "synthetic-hidden-operand", RTarget: "synthetic-hidden-right"},
		{LTarget: "synthetic-hidden-left", Operand: ConstraintRegex, RTarget: "[synthetic-hidden-right"},
	} {
		volume = validDynamicHostVolumeForValidation()
		volume.ID, volume.HostPath, volume.Parameters["synthetic-key"] = "synthetic-hidden-id", "/synthetic-hidden-path", "synthetic-hidden-parameter"
		volume.Constraints = []*Constraint{constraint}
		err = volume.ValidateRegister()
		requireDynamicHostVolumeValidationError(t, err, ErrDynamicHostVolumeInvalid, "Constraints")
		require.NotContains(t, err.Error(), "synthetic-hidden")
	}
}

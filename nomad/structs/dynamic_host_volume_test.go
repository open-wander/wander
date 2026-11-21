// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/stretchr/testify/require"
)

const dynamicHostVolumeLargeInteger = 1<<53 + 17

func syntheticDynamicHostVolume() *DynamicHostVolume {
	return &DynamicHostVolume{
		ID: "volume-synthetic-a", Name: "data-a", Namespace: "dhv-synthetic-a", Type: VolumeTypeHost,
		NodeID: "node-synthetic-a", NodePool: "pool-synthetic-a",
		HostPath: "/srv/wander-synthetic/data-a", PluginID: "mkdir",
		CapacityBytes: dynamicHostVolumeLargeInteger, RequestedCapacityMinBytes: 1048576,
		RequestedCapacityMaxBytes: dynamicHostVolumeLargeInteger + 1,
		RequestedCapabilities: []*DynamicHostVolumeCapability{
			{AttachmentMode: DynamicHostVolumeAttachmentModeFilesystem, AccessMode: DynamicHostVolumeAccessModeSingleNodeWriter},
			nil, {AttachmentMode: DynamicHostVolumeAttachmentModeFilesystem, AccessMode: DynamicHostVolumeAccessModeSingleNodeReader},
		},
		Constraints: []*Constraint{{LTarget: "synthetic-attribute", RTarget: "synthetic-value", Operand: "="}, nil},
		Parameters:  map[string]string{"mode": "0700", "synthetic-key": "synthetic-value"},
		State:       DynamicHostVolumeStateReady, Origin: DynamicHostVolumeOriginRegistered,
		Attachments: []*DynamicHostVolumeAttachment{
			{AllocationID: "allocation-synthetic-a", NodeID: "node-synthetic-a", ReadOnly: true,
				AttachmentMode: DynamicHostVolumeAttachmentModeFilesystem, AccessMode: DynamicHostVolumeAccessModeSingleNodeReader},
			nil, {AllocationID: "allocation-synthetic-b", NodeID: "node-synthetic-a"},
		},
		CreateTime: dynamicHostVolumeLargeInteger + 2, ModifyTime: dynamicHostVolumeLargeInteger + 3,
		CreateIndex: dynamicHostVolumeLargeInteger + 4, ModifyIndex: dynamicHostVolumeLargeInteger + 5,
	}
}

func syntheticDynamicHostVolumeClaim() *DynamicHostVolumeClaim {
	return &DynamicHostVolumeClaim{
		ID: "claim-synthetic-a", Namespace: "dhv-synthetic-a", JobID: "job-synthetic-a",
		TaskGroupName: "group-synthetic-a", VolumeName: "data-a", VolumeID: "volume-synthetic-a",
		AllocationID: "allocation-synthetic-a", AllocationIndex: dynamicHostVolumeLargeInteger + 6,
		CreateIndex: dynamicHostVolumeLargeInteger + 7, ModifyIndex: dynamicHostVolumeLargeInteger + 8,
	}
}

func TestDynamicHostVolumeCopyIsolation(t *testing.T) {
	original := syntheticDynamicHostVolume()
	baseline := syntheticDynamicHostVolume()
	copied := original.Copy()
	require.Equal(t, baseline, copied)
	for _, target := range []*DynamicHostVolume{original, copied} {
		target.Parameters["mode"] = "0750"
		target.RequestedCapabilities[0].AccessMode = "synthetic-other-access"
		target.Constraints[0].RTarget = "synthetic-other-value"
		target.Attachments[0].AllocationID = "allocation-synthetic-other"
		target.RequestedCapabilities[1] = &DynamicHostVolumeCapability{}
		target.Constraints[1] = &Constraint{}
		target.Attachments[1] = &DynamicHostVolumeAttachment{}
		if target == original {
			require.Equal(t, baseline, copied, "mutating original must not affect copy")
			original = syntheticDynamicHostVolume()
		} else {
			require.Equal(t, baseline, original, "mutating copy must not affect original")
		}
	}
}

func TestDynamicHostVolumeEqualityAndValueCopies(t *testing.T) {
	volume := syntheticDynamicHostVolume()
	models := []interface{}{volume, volume.RequestedCapabilities[0], volume.Attachments[0], syntheticDynamicHostVolumeClaim()}
	for _, model := range models {
		value := reflect.ValueOf(model)
		t.Run(value.Type().Elem().Name(), func(t *testing.T) {
			baseline := value.Elem().Interface()
			equal := func(left, right reflect.Value) bool {
				return left.MethodByName("Equal").Call([]reflect.Value{right})[0].Bool()
			}
			copyValue := func() reflect.Value { return value.MethodByName("Copy").Call(nil)[0] }
			copied := copyValue()
			require.True(t, equal(value, copied))
			require.True(t, equal(copied, value))
			require.NotEqual(t, value.Pointer(), copied.Pointer())
			nilValue := reflect.Zero(value.Type())
			require.True(t, nilValue.MethodByName("Copy").Call(nil)[0].IsNil())
			require.True(t, equal(nilValue, nilValue))
			require.False(t, equal(value, nilValue))
			require.False(t, equal(nilValue, value))
			zero := reflect.New(value.Type().Elem())
			require.False(t, equal(nilValue, zero))
			require.False(t, equal(zero, nilValue))
			for index := 0; index < value.Elem().NumField(); index++ {
				t.Run(value.Elem().Type().Field(index).Name, func(t *testing.T) {
					altered := copyValue()
					field := altered.Elem().Field(index)
					switch field.Kind() {
					case reflect.String:
						field.SetString(field.String() + "-synthetic-other")
					case reflect.Int64:
						field.SetInt(field.Int() + 1)
					case reflect.Uint64:
						field.SetUint(field.Uint() + 1)
					case reflect.Bool:
						field.SetBool(!field.Bool())
					case reflect.Slice:
						field.Set(field.Slice(1, field.Len()))
					case reflect.Map:
						field.Set(reflect.MakeMap(field.Type()))
					default:
						t.Fatalf("add a meaningful mutation for %s", field.Kind())
					}
					require.False(t, equal(value, altered), "every field participates in equality")
					require.False(t, equal(altered, value))
					require.Equal(t, baseline, value.Elem().Interface(), "value copy must not alias its original")
				})
			}
		})
	}
}

func TestDynamicHostVolumeCollections(t *testing.T) {
	for _, fieldName := range []string{"RequestedCapabilities", "Constraints", "Attachments", "Parameters"} {
		t.Run(fieldName, func(t *testing.T) {
			nilVolume := &DynamicHostVolume{}
			emptyVolume := &DynamicHostVolume{}
			field := reflect.ValueOf(emptyVolume).Elem().FieldByName(fieldName)
			if field.Kind() == reflect.Map {
				field.Set(reflect.MakeMap(field.Type()))
			} else {
				field.Set(reflect.MakeSlice(field.Type(), 0, 0))
			}
			require.False(t, nilVolume.Equal(emptyVolume))
			require.False(t, emptyVolume.Equal(nilVolume))
			require.Equal(t, emptyVolume, emptyVolume.Copy())
			require.Equal(t, nilVolume, nilVolume.Copy())
		})
	}
	volume := syntheticDynamicHostVolume()
	for _, fieldName := range []string{"RequestedCapabilities", "Constraints", "Attachments"} {
		t.Run(fieldName+"/order-and-nil", func(t *testing.T) {
			reordered := volume.Copy()
			slice := reflect.ValueOf(reordered).Elem().FieldByName(fieldName)
			reflect.Swapper(slice.Interface())(0, 1)
			require.False(t, volume.Equal(reordered))
			require.False(t, reordered.Equal(volume))
			withZero := volume.Copy()
			slice = reflect.ValueOf(withZero).Elem().FieldByName(fieldName)
			slice.Index(1).Set(reflect.New(slice.Type().Elem().Elem()))
			require.False(t, volume.Equal(withZero))
		})
	}
	mapReordered := volume.Copy()
	mapReordered.Parameters = map[string]string{"synthetic-key": "synthetic-value", "mode": "0700"}
	require.True(t, volume.Equal(mapReordered))
	contents := map[string]func(*DynamicHostVolume){
		"capability-value": func(changed *DynamicHostVolume) {
			changed.RequestedCapabilities[0].AccessMode = DynamicHostVolumeAccessModeSingleNodeReader
		},
		"constraint-value": func(changed *DynamicHostVolume) {
			changed.Constraints[0].RTarget = "synthetic-other-value"
		},
		"attachment-value": func(changed *DynamicHostVolume) {
			changed.Attachments[0].ReadOnly = false
		},
		"map-value": func(changed *DynamicHostVolume) {
			changed.Parameters["mode"] = "0750"
		},
		"map-key": func(changed *DynamicHostVolume) {
			delete(changed.Parameters, "mode")
			changed.Parameters["synthetic-other-key"] = "0700"
		},
	}
	for name, mutate := range contents {
		t.Run(name, func(t *testing.T) {
			changed := volume.Copy()
			mutate(changed)
			require.False(t, volume.Equal(changed), "equal-length collections compare their contents")
			require.False(t, changed.Equal(volume))
		})
	}
}

func TestDynamicHostVolumeSerialization(t *testing.T) {
	roundTrips := map[string]func(interface{}, interface{}){
		"json": func(source, target interface{}) {
			data, err := json.Marshal(source)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, target))
		},
		"msgpack": func(source, target interface{}) {
			var data []byte
			require.NoError(t, codec.NewEncoderBytes(&data, MsgpackHandle).Encode(source))
			require.NoError(t, codec.NewDecoderBytes(data, MsgpackHandle).Decode(target))
		},
	}
	for format, roundTrip := range roundTrips {
		t.Run(format, func(t *testing.T) {
			volume := syntheticDynamicHostVolume()
			var decoded DynamicHostVolume
			roundTrip(volume, &decoded)
			require.Equal(t, *volume, decoded, "inspect typed decoded fields independently of Equal")
			require.Equal(t, int64(dynamicHostVolumeLargeInteger), decoded.CapacityBytes)
			require.Equal(t, uint64(dynamicHostVolumeLargeInteger+5), decoded.ModifyIndex)
			require.Equal(t, int64(dynamicHostVolumeLargeInteger+3), decoded.ModifyTime)
			require.Equal(t, "allocation-synthetic-a", decoded.Attachments[0].AllocationID)
			require.Nil(t, decoded.Constraints[1])
			require.True(t, volume.Equal(&decoded))
			claim := syntheticDynamicHostVolumeClaim()
			var decodedClaim DynamicHostVolumeClaim
			roundTrip(claim, &decodedClaim)
			require.Equal(t, *claim, decodedClaim)
			require.Equal(t, uint64(dynamicHostVolumeLargeInteger+6), decodedClaim.AllocationIndex)
			require.Equal(t, "group-synthetic-a", decodedClaim.TaskGroupName)

			volume.State, volume.Origin = "synthetic-future-state", "synthetic-future-origin"
			volume.RequestedCapabilities[0].AccessMode = "synthetic-future-access"
			volume.RequestedCapabilities[0].AttachmentMode = "synthetic-future-attachment"
			volume.Attachments[0].AccessMode = "synthetic-future-access"
			volume.Attachments[0].AttachmentMode = "synthetic-future-attachment"
			roundTrip(volume, &decoded)
			require.Equal(t, *volume, decoded)
			var zero DynamicHostVolume
			roundTrip(map[string]interface{}{}, &zero)
			require.Equal(t, DynamicHostVolume{}, zero, "missing fields must not introduce defaults")
			var zeroClaim DynamicHostVolumeClaim
			roundTrip(map[string]interface{}{}, &zeroClaim)
			require.Equal(t, DynamicHostVolumeClaim{}, zeroClaim)
			for _, empty := range []*DynamicHostVolume{
				{}, {RequestedCapabilities: []*DynamicHostVolumeCapability{}, Constraints: []*Constraint{},
					Attachments: []*DynamicHostVolumeAttachment{}, Parameters: map[string]string{}},
			} {
				var result DynamicHostVolume
				roundTrip(empty, &result)
				require.Equal(t, *empty, result, "nil and empty representations must survive")
			}
		})
	}
}

func TestDynamicHostVolumeAccessAllows(t *testing.T) {
	for _, tc := range []struct {
		access   DynamicHostVolumeAccessMode
		readOnly bool
		allowed  bool
	}{
		{DynamicHostVolumeAccessModeSingleNodeReader, true, true},
		{DynamicHostVolumeAccessModeSingleNodeReader, false, false},
		{DynamicHostVolumeAccessModeSingleNodeWriter, true, false},
		{DynamicHostVolumeAccessModeSingleNodeWriter, false, true},
		{DynamicHostVolumeAccessModeSingleNodeSingleWriter, true, true},
		{DynamicHostVolumeAccessModeSingleNodeSingleWriter, false, true},
		{DynamicHostVolumeAccessModeSingleNodeMultiWriter, true, true},
		{DynamicHostVolumeAccessModeSingleNodeMultiWriter, false, true},
		{"synthetic-future", true, false},
		{"synthetic-future", false, false},
		{DynamicHostVolumeAccessModeUnknown, true, false},
		{DynamicHostVolumeAccessModeUnknown, false, false},
	} {
		t.Run(fmt.Sprintf("%q/read_only=%t", tc.access, tc.readOnly), func(t *testing.T) {
			require.Equal(t, tc.allowed, DynamicHostVolumeAccessAllows(tc.access, tc.readOnly))
		})
	}
}

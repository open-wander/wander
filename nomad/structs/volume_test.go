// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestVolumeRequest_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name           string
		expected       []string
		canariesCount  int
		taskGroupCount int
		req            *VolumeRequest
	}{
		{
			name:     "host volume with empty source",
			expected: []string{"volume has an empty source"},
			req: &VolumeRequest{
				Type: VolumeTypeHost,
			},
		},
		{
			name: "host volume with CSI volume config",
			expected: []string{
				"single-node-reader-only volumes must be read-only",
				"host volumes cannot have attachment mode block-device",
				"host volumes cannot have mount options",
				"volume cannot be per_alloc for system or sysbatch jobs",
				"volume cannot be per_alloc when canaries are in use",
			},
			canariesCount: 1,
			req: &VolumeRequest{
				Type:           VolumeTypeHost,
				ReadOnly:       false,
				AccessMode:     CSIVolumeAccessModeSingleNodeReader,
				AttachmentMode: CSIVolumeAttachmentModeBlockDevice,
				MountOptions: &CSIMountOptions{
					FSType:     "ext4",
					MountFlags: []string{"ro"},
				},
				PerAlloc: true,
			},
		},
		{
			name: "CSI volume multi-reader-single-writer access mode",
			expected: []string{
				"volume with multi-node-single-writer access mode allows only one writer",
			},
			taskGroupCount: 2,
			req: &VolumeRequest{
				Type:       VolumeTypeCSI,
				AccessMode: CSIVolumeAccessModeMultiNodeSingleWriter,
			},
		},
		{
			name: "CSI volume single reader access mode",
			expected: []string{
				"volume with single-node-reader-only access mode allows only one reader",
			},
			taskGroupCount: 2,
			req: &VolumeRequest{
				Type:       VolumeTypeCSI,
				AccessMode: CSIVolumeAccessModeSingleNodeReader,
				ReadOnly:   true,
			},
		},
		{
			name: "CSI volume per-alloc with canaries",
			expected: []string{
				"volume cannot be per_alloc for system or sysbatch jobs",
				"volume cannot be per_alloc when canaries are in use",
			},
			canariesCount: 1,
			req: &VolumeRequest{
				Type:     VolumeTypeCSI,
				PerAlloc: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate(JobTypeSystem, tc.taskGroupCount, tc.canariesCount)
			for _, expected := range tc.expected {
				require.Contains(t, err.Error(), expected)
			}
		})
	}

}

func TestVolumeRequest_Validate_HostModes(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		req      *VolumeRequest
		expected string // empty means valid
	}{
		{name: "no modes", req: &VolumeRequest{Source: "data-a"}},
		{name: "file-system", req: &VolumeRequest{Source: "data-a", AttachmentMode: CSIVolumeAttachmentModeFilesystem}},
		{name: "single-node-writer", req: &VolumeRequest{Source: "data-a", AccessMode: CSIVolumeAccessModeSingleNodeWriter}},
		{name: "reader-only read-only", req: &VolumeRequest{Source: "data-a", AccessMode: CSIVolumeAccessModeSingleNodeReader, ReadOnly: true}},
		{name: "reader-only writable", req: &VolumeRequest{Source: "data-a", AccessMode: CSIVolumeAccessModeSingleNodeReader}, expected: "single-node-reader-only volumes must be read-only"},
		{name: "single-writer read-write", req: &VolumeRequest{Source: "data-a", AccessMode: CSIVolumeAccessMode(DynamicHostVolumeAccessModeSingleNodeSingleWriter)}},
		{name: "single-writer read-only", req: &VolumeRequest{Source: "data-a", AccessMode: CSIVolumeAccessMode(DynamicHostVolumeAccessModeSingleNodeSingleWriter), ReadOnly: true}},
		{name: "multi-writer read-write", req: &VolumeRequest{Source: "data-a", AccessMode: CSIVolumeAccessMode(DynamicHostVolumeAccessModeSingleNodeMultiWriter)}},
		{name: "multi-writer read-only", req: &VolumeRequest{Source: "data-a", AccessMode: CSIVolumeAccessMode(DynamicHostVolumeAccessModeSingleNodeMultiWriter), ReadOnly: true}},
		{name: "explicit writer read-only is valid at submission", req: &VolumeRequest{Source: "data-a", AccessMode: CSIVolumeAccessModeSingleNodeWriter, ReadOnly: true}},
		{name: "unknown access mode", req: &VolumeRequest{Source: "data-a", AccessMode: "synthetic-future"}, expected: "host volumes cannot have access mode synthetic-future"},
		{name: "block-device", req: &VolumeRequest{Source: "data-a", AttachmentMode: CSIVolumeAttachmentModeBlockDevice}, expected: "host volumes cannot have attachment mode block-device"},
		{name: "multi-node-reader-only", req: &VolumeRequest{Source: "data-a", AccessMode: CSIVolumeAccessModeMultiNodeReader}, expected: "host volumes cannot have access mode multi-node-reader-only"},
		{name: "multi-node-single-writer", req: &VolumeRequest{Source: "data-a", AccessMode: CSIVolumeAccessModeMultiNodeSingleWriter}, expected: "host volumes cannot have access mode multi-node-single-writer"},
		{name: "multi-node-multi-writer", req: &VolumeRequest{Source: "data-a", AccessMode: CSIVolumeAccessModeMultiNodeMultiWriter}, expected: "host volumes cannot have access mode multi-node-multi-writer"},
		{name: "mount options", req: &VolumeRequest{Source: "data-a", MountOptions: &CSIMountOptions{FSType: "ext4"}}, expected: "host volumes cannot have mount options"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.Type = VolumeTypeHost
			err := tc.req.Validate(JobTypeService, 2, 0)
			if tc.expected == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.expected)
		})
	}
}

func TestVolumeRequest_HostVolumeModes(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name       string
		req        *VolumeRequest
		attachment DynamicHostVolumeAttachmentMode
		access     DynamicHostVolumeAccessMode
	}{
		{"defaults", &VolumeRequest{}, DynamicHostVolumeAttachmentModeFilesystem, DynamicHostVolumeAccessModeSingleNodeWriter},
		{"read-only default", &VolumeRequest{ReadOnly: true}, DynamicHostVolumeAttachmentModeFilesystem, DynamicHostVolumeAccessModeSingleNodeReader},
		{"explicit writer", &VolumeRequest{AccessMode: CSIVolumeAccessModeSingleNodeWriter}, DynamicHostVolumeAttachmentModeFilesystem, DynamicHostVolumeAccessModeSingleNodeWriter},
		{"explicit reader", &VolumeRequest{AccessMode: CSIVolumeAccessModeSingleNodeReader, ReadOnly: true}, DynamicHostVolumeAttachmentModeFilesystem, DynamicHostVolumeAccessModeSingleNodeReader},
		{"explicit single-writer", &VolumeRequest{AccessMode: CSIVolumeAccessMode(DynamicHostVolumeAccessModeSingleNodeSingleWriter)}, DynamicHostVolumeAttachmentModeFilesystem, DynamicHostVolumeAccessModeSingleNodeSingleWriter},
		{"explicit multi-writer read-only", &VolumeRequest{AccessMode: CSIVolumeAccessMode(DynamicHostVolumeAccessModeSingleNodeMultiWriter), ReadOnly: true}, DynamicHostVolumeAttachmentModeFilesystem, DynamicHostVolumeAccessModeSingleNodeMultiWriter},
		{"explicit file-system", &VolumeRequest{AttachmentMode: CSIVolumeAttachmentModeFilesystem}, DynamicHostVolumeAttachmentModeFilesystem, DynamicHostVolumeAccessModeSingleNodeWriter},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attachment, access := tc.req.HostVolumeModes()
			require.Equal(t, tc.attachment, attachment)
			require.Equal(t, tc.access, access)
		})
	}
}

func TestVolumeRequest_Equal(t *testing.T) {
	ci.Parallel(t)

	must.Equal[*VolumeRequest](t, nil, nil)
	must.NotEqual[*VolumeRequest](t, nil, new(VolumeRequest))

	must.StructEqual(t, &VolumeRequest{
		Name:           "name",
		Type:           "type",
		Source:         "source",
		ReadOnly:       true,
		AccessMode:     "access",
		AttachmentMode: "attachment",
		MountOptions: &CSIMountOptions{
			FSType:     "fs1",
			MountFlags: []string{"flag1"},
		},
		PerAlloc: true,
		Sticky:   true,
	}, []must.Tweak[*VolumeRequest]{{
		Field: "Name",
		Apply: func(vr *VolumeRequest) { vr.Name = "name2" },
	}, {
		Field: "Type",
		Apply: func(vr *VolumeRequest) { vr.Type = "type2" },
	}, {
		Field: "Source",
		Apply: func(vr *VolumeRequest) { vr.Source = "source2" },
	}, {
		Field: "ReadOnly",
		Apply: func(vr *VolumeRequest) { vr.ReadOnly = false },
	}, {
		Field: "AccessMode",
		Apply: func(vr *VolumeRequest) { vr.AccessMode = "access2" },
	}, {
		Field: "AttachmentMode",
		Apply: func(vr *VolumeRequest) { vr.AttachmentMode = "attachment2" },
	}, {
		Field: "MountOptions",
		Apply: func(vr *VolumeRequest) { vr.MountOptions = nil },
	}, {
		Field: "PerAlloc",
		Apply: func(vr *VolumeRequest) { vr.PerAlloc = false },
	}, {
		Field: "Sticky",
		Apply: func(vr *VolumeRequest) { vr.Sticky = false },
	}})
}

func TestVolumeRequest_Validate_Sticky(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		req      *VolumeRequest
		jobType  string
		count    int
		canaries int
		expected []string // empty means valid
	}{
		{name: "host sticky", req: &VolumeRequest{Type: VolumeTypeHost, Source: "data-a", Sticky: true}},
		{name: "host sticky with canaries", req: &VolumeRequest{Type: VolumeTypeHost, Source: "data-a", Sticky: true}, canaries: 2},
		{name: "host sticky with count", req: &VolumeRequest{Type: VolumeTypeHost, Source: "data-a", Sticky: true}, count: 3},
		{name: "host sticky read-only", req: &VolumeRequest{Type: VolumeTypeHost, Source: "data-a", Sticky: true, ReadOnly: true}},
		{name: "host sticky single-node-writer", req: &VolumeRequest{Type: VolumeTypeHost, Source: "data-a", Sticky: true, AccessMode: CSIVolumeAccessModeSingleNodeWriter}},
		{name: "host sticky single-node-reader-only", req: &VolumeRequest{Type: VolumeTypeHost, Source: "data-a", Sticky: true, ReadOnly: true, AccessMode: CSIVolumeAccessModeSingleNodeReader}},
		{name: "host sticky single-node-single-writer", req: &VolumeRequest{Type: VolumeTypeHost, Source: "data-a", Sticky: true, AccessMode: CSIVolumeAccessMode(DynamicHostVolumeAccessModeSingleNodeSingleWriter)}},
		{name: "host sticky single-node-multi-writer", req: &VolumeRequest{Type: VolumeTypeHost, Source: "data-a", Sticky: true, AccessMode: CSIVolumeAccessMode(DynamicHostVolumeAccessModeSingleNodeMultiWriter)}},
		{
			name:     "host sticky and per_alloc",
			req:      &VolumeRequest{Type: VolumeTypeHost, Source: "data-a", Sticky: true, PerAlloc: true},
			expected: []string{"volume cannot be per_alloc and sticky at the same time"},
		},
		{
			name: "csi sticky",
			req: &VolumeRequest{
				Type: VolumeTypeCSI, Source: "data-a", Sticky: true,
				AccessMode: CSIVolumeAccessModeMultiNodeMultiWriter, AttachmentMode: CSIVolumeAttachmentModeFilesystem,
			},
			expected: []string{"volume cannot be sticky unless its type is host"},
		},
		{
			name: "csi not sticky",
			req: &VolumeRequest{
				Type: VolumeTypeCSI, Source: "data-a",
				AccessMode: CSIVolumeAccessModeMultiNodeMultiWriter, AttachmentMode: CSIVolumeAttachmentModeFilesystem,
			},
		},
		{
			name: "csi sticky and per_alloc reports both",
			req: &VolumeRequest{
				Type: VolumeTypeCSI, Source: "data-a", Sticky: true, PerAlloc: true,
				AccessMode: CSIVolumeAccessModeMultiNodeMultiWriter, AttachmentMode: CSIVolumeAttachmentModeFilesystem,
			},
			expected: []string{
				"volume cannot be per_alloc and sticky at the same time",
				"volume cannot be sticky unless its type is host",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jobType := tc.jobType
			if jobType == "" {
				jobType = JobTypeService
			}
			err := tc.req.Validate(jobType, tc.count, tc.canaries)
			if len(tc.expected) == 0 {
				must.NoError(t, err)
				return
			}
			must.Error(t, err)
			for _, msg := range tc.expected {
				must.StrContains(t, err.Error(), msg)
			}
		})
	}
}

func TestVolumeRequest_Copy_Sticky(t *testing.T) {
	ci.Parallel(t)

	orig := &VolumeRequest{Name: "data-a", Type: VolumeTypeHost, Source: "data-a", Sticky: true}
	cp := orig.Copy()
	must.True(t, cp.Sticky)

	cp.Sticky = false
	must.True(t, orig.Sticky)
}

func TestVolumeMount_Equal(t *testing.T) {
	ci.Parallel(t)

	must.Equal[*VolumeMount](t, nil, nil)
	must.NotEqual[*VolumeMount](t, nil, new(VolumeMount))

	must.StructEqual(t, &VolumeMount{
		Volume:          "volume",
		Destination:     "destination",
		ReadOnly:        true,
		PropagationMode: "mode",
	}, []must.Tweak[*VolumeMount]{{
		Field: "Volume",
		Apply: func(vm *VolumeMount) { vm.Volume = "vol2" },
	}, {
		Field: "Destination",
		Apply: func(vm *VolumeMount) { vm.Destination = "dest2" },
	}, {
		Field: "ReadOnly",
		Apply: func(vm *VolumeMount) { vm.ReadOnly = false },
	}, {
		Field: "PropogationMode",
		Apply: func(vm *VolumeMount) { vm.PropagationMode = "mode2" },
	}})
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package components

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSidebarOnlyLinksManagementUsersToAccessControl(t *testing.T) {
	for _, test := range []struct {
		name string
		show bool
	}{
		{name: "ordinary user", show: false},
		{name: "management user", show: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := Sidebar(SidebarProps{ShowAccessControl: test.show}).Render(context.Background(), &output)
			if err != nil {
				t.Fatalf("render sidebar: %v", err)
			}
			if got := strings.Contains(output.String(), `href="/ui/access-control"`); got != test.show {
				t.Fatalf("access-control link present = %t, want %t", got, test.show)
			}
		})
	}
}

func TestSidebarLinksDynamicHostVolumesNextToCSI(t *testing.T) {
	var output bytes.Buffer
	if err := Sidebar(SidebarProps{}).Render(context.Background(), &output); err != nil {
		t.Fatalf("render sidebar: %v", err)
	}
	html := output.String()
	csi := strings.Index(html, `href="/ui/csi/volumes"`)
	hostVolumes := strings.Index(html, `href="/ui/host-volumes"`)
	if hostVolumes < 0 || !strings.Contains(html, "Host Volumes") {
		t.Fatal("sidebar has no Host Volumes link")
	}
	if csi < 0 || hostVolumes < csi {
		t.Fatalf("Host Volumes link (%d) must follow the unchanged CSI link (%d)", hostVolumes, csi)
	}
}

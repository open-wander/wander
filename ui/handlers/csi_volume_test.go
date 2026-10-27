package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-wander/wander/api"
)

func TestCSIVolumeListRendersLiveHealth(t *testing.T) {
	handler := csiTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if !requireCSIListRequest(t, w, r, "/v1/volumes") {
			return
		}
		for key, want := range map[string]string{"namespace": "team-a", "region": "west"} {
			if got := r.URL.Query().Get(key); got != want {
				t.Errorf("selection %s = %q, want %q", key, got, want)
			}
		}
		writeJSON(t, w, []*api.CSIVolumeListStub{{
			ID: "database", Name: "Database", Namespace: "team-a", PluginID: "storage", Provider: "org.example.storage",
			Schedulable: true, ControllerRequired: true, ControllersHealthy: 1, ControllersExpected: 2,
			NodesHealthy: 3, NodesExpected: 3, CurrentReaders: 1, CurrentWriters: 2,
		}})
	})

	body := serveCSIRequest(t, handler, "/ui/csi/volumes?namespace=team-a&region=west", false, http.StatusOK)
	assertContainsAll(t, body, "Database", "org.example.storage", "1 / 2", "3 / 3", ">3<")
}

func TestCSIVolumeDetailRendersCapabilitiesAndSeparateClaims(t *testing.T) {
	created := time.Date(2026, time.October, 27, 12, 0, 0, 0, time.UTC)
	volume := &api.CSIVolume{
		ID: "database", Name: "Database", Namespace: "team-a", ExternalID: "external-01",
		PluginID: "storage", Provider: "org.example.storage", Schedulable: true,
		RequestedCapabilities: []*api.CSIVolumeCapability{
			{AccessMode: api.CSIVolumeAccessModeSingleNodeWriter, AttachmentMode: api.CSIVolumeAttachmentModeFilesystem},
			{AccessMode: api.CSIVolumeAccessModeMultiNodeReader, AttachmentMode: api.CSIVolumeAttachmentModeBlockDevice},
		},
		ReadAllocs:  map[string]*api.Allocation{"read-alloc": nil},
		WriteAllocs: map[string]*api.Allocation{"write-alloc": nil},
		Allocations: []*api.AllocationListStub{
			{ID: "read-alloc", Name: "reader[0]", JobID: "reader", TaskGroup: "app", NodeID: "node-a", NodeName: "client-a", ClientStatus: api.AllocClientStatusRunning, CreateTime: created.UnixNano()},
			{ID: "write-alloc", Name: "writer[0]", JobID: "writer", TaskGroup: "app", NodeID: "node-b", NodeName: "client-b", ClientStatus: api.AllocClientStatusPending, CreateTime: created.UnixNano()},
		},
	}
	handler := csiTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/volume/csi/database" {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, volume)
	})

	body := serveCSIRequest(t, handler, "/ui/csi/volumes/database?namespace=team-a", false, http.StatusOK)
	assertContainsAll(t, body,
		"Capability Constraints", "Single Node Writer", "Multi Node Reader Only", "Block Device",
		"Read-only claims", "reader[0]", "Read/write claims", "writer[0]", "client-a", "client-b",
	)
	if strings.Index(body, "reader[0]") > strings.Index(body, "Read/write claims") {
		t.Error("read-only allocation was not rendered in the read-only claim table")
	}

	htmxBody := serveCSIRequest(t, handler, "/ui/csi/volumes/database?namespace=team-a", true, http.StatusOK)
	assertContainsAll(t, htmxBody, "Capability Constraints", "Read-only claims", "Read/write claims")
	if strings.Contains(strings.ToLower(htmxBody), "<!doctype html") {
		t.Error("HTMX response unexpectedly contains the document shell")
	}
}

func TestCSIVolumeRoutesPreserveTruthfulErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		path       string
		apiStatus  int
		wantStatus int
	}{
		{name: "list denied", path: "/ui/csi/volumes", apiStatus: http.StatusForbidden, wantStatus: http.StatusForbidden},
		{name: "list unavailable", path: "/ui/csi/volumes", apiStatus: http.StatusInternalServerError, wantStatus: http.StatusServiceUnavailable},
		{name: "detail missing", path: "/ui/csi/volumes/missing", apiStatus: http.StatusNotFound, wantStatus: http.StatusNotFound},
		{name: "detail denied", path: "/ui/csi/volumes/missing", apiStatus: http.StatusForbidden, wantStatus: http.StatusForbidden},
		{name: "detail unavailable", path: "/ui/csi/volumes/missing", apiStatus: http.StatusInternalServerError, wantStatus: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := csiTestHandler(t, func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, fmt.Sprintf("backend status %d", test.apiStatus), test.apiStatus)
			})
			body := serveCSIRequest(t, handler, test.path, false, test.wantStatus)
			if strings.Contains(body, "No volumes found") {
				t.Fatal("backend failure was rendered as an empty volume list")
			}
		})
	}
}

func TestCSIVolumeDetailRejectsInventedSubroutes(t *testing.T) {
	handler := csiTestHandler(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("invalid volume subroute reached the backend")
	})
	serveCSIRequest(t, handler, "/ui/csi/volumes/database/allocations", false, http.StatusNotFound)
}

// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package handlers

import (
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/hostvolumes"
	"github.com/open-wander/wander/ui/templates/partials/hostvolumespartials"
)

const (
	hostVolumesPath        = "/ui/host-volumes"
	hostVolumeCapability   = "host-volume-read"
	hostVolumesTableTarget = "host-volumes-table-container"
	allNamespaces          = "*"
)

// HostVolumesHandler serves the dynamic host volume list and detail pages.
// CSI volumes keep their own pages under /ui/csi.
type HostVolumesHandler struct {
	handler *Handler
}

// NewHostVolumesHandler creates a dynamic host volumes handler.
func NewHostVolumesHandler(h *Handler) *HostVolumesHandler {
	return &HostVolumesHandler{handler: h}
}

// ServeHTTP routes /ui/host-volumes and /ui/host-volumes/{id}.
func (h *HostVolumesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path != hostVolumesPath && !strings.HasPrefix(path, hostVolumesPath+"/") {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.Trim(path, "/") == strings.Trim(hostVolumesPath, "/") {
		h.List(w, r)
		return
	}
	volumeID, err := url.PathUnescape(strings.Trim(strings.TrimPrefix(path, hostVolumesPath+"/"), "/"))
	if err != nil || volumeID == "" || strings.Contains(volumeID, "/") {
		http.NotFound(w, r)
		return
	}
	h.Detail(w, r, volumeID)
}

// hostVolumeQueryOptions scopes an API call to the request's namespace, region
// and token. The wildcard namespace is passed through unchanged: the server
// already restricts a wildcard list to the namespaces the token may read.
func hostVolumeQueryOptions(r *http.Request, namespace string) *api.QueryOptions {
	return &api.QueryOptions{
		Namespace: namespace,
		Region:    GetRegionFromRequest(r),
		AuthToken: GetTokenFromRequest(r),
	}
}

// List renders the dynamic host volume list.
func (h *HostVolumesHandler) List(w http.ResponseWriter, r *http.Request) {
	namespace := GetNamespaceFromRequest(r)

	// A wildcard is not pre-checked: a token scoped to one namespace holds no
	// capability for "*", yet the server returns the readable subset for it.
	if perms := GetPermissionsFromRequest(r); perms != nil && namespace != allNamespaces && !perms.CanReadHostVolume(namespace) {
		h.renderHostVolumeForbidden(w, r, "You do not have permission to list dynamic host volumes", namespace)
		return
	}

	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.renderHostVolumeListError(w, r, namespace, errNomadClientUnavailable)
		return
	}
	query := hostVolumeQueryOptions(r, namespace)

	stubs, _, err := client.HostVolumes().List(nil, query)
	if err != nil {
		h.renderHostVolumeListError(w, r, namespace, err)
		return
	}

	// One claim list serves every row. A claim failure must not hide the
	// volumes, so the counts become "unavailable" instead.
	claimCounts, claimsKnown := map[hostVolumeKey]int{}, true
	claims, _, err := client.HostVolumes().ListClaims(nil, query)
	if err != nil {
		claimsKnown = false
		h.handler.logger.Warn("failed to fetch host volume claims", "namespace", namespace, "error", err)
	}
	for _, claim := range claims {
		if claim != nil {
			claimCounts[hostVolumeKey{namespace: claim.Namespace, id: claim.VolumeID}]++
		}
	}

	params := r.URL.Query()
	nodeFilter := params.Get("node")
	all := make([]hostvolumespartials.VolumeSummary, 0, len(stubs))
	nodeSet := map[string]struct{}{}
	for _, stub := range stubs {
		if stub == nil {
			continue
		}
		if stub.NodeID != "" {
			nodeSet[stub.NodeID] = struct{}{}
		}
		all = append(all, hostvolumespartials.VolumeSummary{
			ID: stub.ID, Name: stub.Name, Namespace: stub.Namespace, NodeID: stub.NodeID,
			NodePool: stub.NodePool, PluginID: stub.PluginID, State: string(stub.State),
			CapacityBytes: stub.CapacityBytes,
			Claims:        claimCounts[hostVolumeKey{namespace: stub.Namespace, id: stub.ID}],
			ClaimsKnown:   claimsKnown,
		})
	}

	// The node filter is applied here, not by the API, so the node selector
	// keeps offering every node seen in the unfiltered list.
	nodes := make([]string, 0, len(nodeSet))
	for node := range nodeSet {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	filtered := all[:0:0]
	for _, volume := range all {
		if nodeFilter == "" || volume.NodeID == nodeFilter {
			filtered = append(filtered, volume)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.NodeID != b.NodeID {
			return a.NodeID < b.NodeID
		}
		return a.ID < b.ID
	})

	page := positiveBoundedInt(params.Get("page"), 1, math.MaxInt)
	perPage := positiveBoundedInt(params.Get("per_page"), 25, 100)
	page, _, start, end := paginationBounds(len(filtered), page, perPage)

	props := hostvolumespartials.ListProps{
		Volumes:        filtered[start:end],
		Nodes:          nodes,
		NodeFilter:     nodeFilter,
		NamespaceParam: params.Get("namespace"),
		Pagination:     hostvolumespartials.PaginationMeta{Page: page, PerPage: perPage, Total: len(filtered)},
	}

	if IsHTMXFromRequest(r) && r.Header.Get("HX-Target") == hostVolumesTableTarget {
		if err := RenderPartial(w, r, hostvolumespartials.VolumesTable(props)); err != nil {
			h.handler.logger.Error("failed to render host volumes table", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	layoutProps := h.handler.BaseLayoutProps(r,
		"Dynamic host volumes - Wander",
		"Dynamic host volumes",
		hostVolumesPath,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Storage", URL: ""},
			{Label: "Host volumes", URL: ""},
		},
	)
	if err := Render(w, r, hostvolumes.List(layoutProps, props), hostvolumespartials.ListContent(props)); err != nil {
		h.handler.logger.Error("failed to render host volumes list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// hostVolumeKey identifies a volume across namespaces; names repeat across
// nodes, so claims are never matched by name.
type hostVolumeKey struct {
	namespace string
	id        string
}

// Detail renders one dynamic host volume.
func (h *HostVolumesHandler) Detail(w http.ResponseWriter, r *http.Request, volumeID string) {
	namespace := GetNamespaceFromRequest(r)

	// The server refuses a single read across namespaces, so say so here
	// instead of sending a request that can only fail.
	if namespace == allNamespaces {
		h.handler.RenderNotFound(w, r, NotFoundError{
			Message:     "Viewing a dynamic host volume needs a single namespace. Open the volume from the list or add a namespace to the address.",
			RequestPath: r.URL.Path,
		})
		return
	}
	if perms := GetPermissionsFromRequest(r); perms != nil && !perms.CanReadHostVolume(namespace) {
		h.renderHostVolumeForbidden(w, r, "You do not have permission to view this dynamic host volume", namespace)
		return
	}

	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.renderHostVolumeDetailError(w, r, volumeID, namespace, errNomadClientUnavailable)
		return
	}
	volume, _, err := client.HostVolumes().Info(volumeID, hostVolumeQueryOptions(r, namespace))
	if err != nil {
		h.renderHostVolumeDetailError(w, r, volumeID, namespace, err)
		return
	}

	detail := hostVolumeDetail(volume)
	claimNamespace := firstNonEmpty(volume.Namespace, namespace)
	claims, _, err := client.HostVolumes().ListClaims(
		&api.HostVolumeClaimListRequest{VolumeName: volume.Name},
		hostVolumeQueryOptions(r, claimNamespace),
	)
	if err != nil {
		detail.ClaimsUnavailable = true
		h.handler.logger.Warn("failed to fetch host volume claims", "volumeID", volumeID, "error", err)
	}
	for _, claim := range claims {
		// The claim query filters by name, which repeats across nodes.
		if claim == nil || claim.VolumeID != volume.ID || claim.Namespace != claimNamespace {
			continue
		}
		detail.Claims = append(detail.Claims, hostvolumespartials.Claim{
			ID: claim.ID, JobID: claim.JobID, TaskGroup: claim.TaskGroupName,
			AllocID: claim.AllocID, Namespace: claim.Namespace,
		})
	}
	sort.SliceStable(detail.Claims, func(i, j int) bool { return detail.Claims[i].ID < detail.Claims[j].ID })

	layoutProps := h.handler.BaseLayoutProps(r,
		volume.Name+" - Dynamic host volume - Wander",
		"Dynamic host volume "+volume.Name,
		hostVolumesPath,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Storage", URL: ""},
			{Label: "Host volumes", URL: hostVolumesPath},
			{Label: volume.Name, URL: ""},
		},
	)
	if err := Render(w, r, hostvolumes.Detail(layoutProps, detail), hostvolumespartials.DetailContent(detail)); err != nil {
		h.handler.logger.Error("failed to render host volume detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// hostVolumeDetail maps the API record without interpreting it: HostPath and
// Parameters are passed through exactly as returned.
func hostVolumeDetail(volume *api.HostVolume) hostvolumespartials.VolumeDetail {
	detail := hostvolumespartials.VolumeDetail{
		ID: volume.ID, Name: volume.Name, Namespace: volume.Namespace, Type: volume.Type,
		Origin: string(volume.Origin), State: string(volume.State), Diagnostics: volume.StatusDescription,
		NodeID: volume.NodeID, NodePool: volume.NodePool, PluginID: volume.PluginID, HostPath: volume.HostPath,
		CapacityBytes: volume.CapacityBytes, MinBytes: volume.RequestedCapacityMinBytes, MaxBytes: volume.RequestedCapacityMaxBytes,
		CreateIndex: volume.CreateIndex, ModifyIndex: volume.ModifyIndex,
		Created: unixNanoTime(volume.CreateTime), Modified: unixNanoTime(volume.ModifyTime),
	}
	for _, capability := range volume.RequestedCapabilities {
		if capability != nil {
			detail.Capabilities = append(detail.Capabilities, hostvolumespartials.Capability{
				AttachmentMode: string(capability.AttachmentMode), AccessMode: string(capability.AccessMode),
			})
		}
	}
	for _, constraint := range volume.Constraints {
		if constraint != nil {
			detail.Constraints = append(detail.Constraints, hostvolumespartials.Constraint{
				LTarget: constraint.LTarget, Operand: constraint.Operand, RTarget: constraint.RTarget,
			})
		}
	}
	for _, attachment := range volume.Attachments {
		if attachment != nil {
			detail.Attachments = append(detail.Attachments, hostvolumespartials.Attachment{
				AllocationID: attachment.AllocationID, NodeID: attachment.NodeID,
				AttachmentMode: string(attachment.AttachmentMode), AccessMode: string(attachment.AccessMode),
				ReadOnly: attachment.ReadOnly,
			})
		}
	}
	for key, value := range volume.Parameters {
		detail.Parameters = append(detail.Parameters, hostvolumespartials.Parameter{Key: key, Value: value})
	}
	sort.Slice(detail.Parameters, func(i, j int) bool { return detail.Parameters[i].Key < detail.Parameters[j].Key })
	return detail
}

func (h *HostVolumesHandler) renderHostVolumeForbidden(w http.ResponseWriter, r *http.Request, message, namespace string) {
	h.handler.RenderForbidden(w, r, ForbiddenError{
		Message: message, Capability: hostVolumeCapability, Namespace: namespace,
		Suggestions: []string{"Contact your administrator to request the 'host-volume-read' capability"},
	})
}

func (h *HostVolumesHandler) renderHostVolumeListError(w http.ResponseWriter, r *http.Request, namespace string, err error) {
	if apiResponseHasStatus(err, http.StatusForbidden) {
		h.renderHostVolumeForbidden(w, r, "You do not have permission to list dynamic host volumes", namespace)
		return
	}
	h.handler.logger.Error("failed to fetch host volumes", "namespace", namespace, "error", err)
	h.handler.RenderClusterUnavailable(w, r, ClusterUnavailableError{
		Message: "Wander could not load dynamic host volumes from the cluster.",
	})
}

func (h *HostVolumesHandler) renderHostVolumeDetailError(w http.ResponseWriter, r *http.Request, volumeID, namespace string, err error) {
	switch {
	case apiResponseHasStatus(err, http.StatusNotFound):
		h.handler.RenderNotFound(w, r, NotFoundError{
			Message: "The requested dynamic host volume was not found.", RequestPath: r.URL.Path,
		})
	case apiResponseHasStatus(err, http.StatusForbidden):
		h.renderHostVolumeForbidden(w, r, "You do not have permission to view this dynamic host volume", namespace)
	default:
		h.handler.logger.Error("failed to fetch host volume", "volumeID", volumeID, "namespace", namespace, "error", err)
		h.handler.RenderClusterUnavailable(w, r, ClusterUnavailableError{
			Message: "Wander could not load the dynamic host volume from the cluster.",
		})
	}
}

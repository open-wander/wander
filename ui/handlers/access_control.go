package handlers

import (
	"net/http"
	"strings"

	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/accesscontrol"
	"github.com/open-wander/wander/ui/templates/partials/accesscontrolpartials"
)

const (
	aclBasePath     = "/ui/access-control"
	aclPoliciesPath = aclBasePath + "/policies"
	aclTokensPath   = aclBasePath + "/tokens"
	aclRolesPath    = aclBasePath + "/roles"

	legacyPoliciesPath = "/ui/policies"
	legacyTokensPath   = "/ui/tokens"
	legacyRolesPath    = "/ui/roles"
)

// AccessControlHandler provides the Nomad 1.6.5-compatible ACL overview and
// delegates resource pages to Wander's independently implemented handlers.
type AccessControlHandler struct {
	handler    *Handler
	policies   *PoliciesHandler
	management *ACLManagementHandler
}

func NewAccessControlHandler(handler *Handler) *AccessControlHandler {
	return &AccessControlHandler{
		handler:    handler,
		policies:   NewPoliciesHandler(handler),
		management: NewACLManagementHandler(handler),
	}
}

func (h *AccessControlHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == aclBasePath:
		h.serveOverview(w, r)
	case path == aclPoliciesPath || strings.HasPrefix(path, aclPoliciesPath+"/"):
		h.policies.ServeHTTP(w, r)
	case path == aclTokensPath || strings.HasPrefix(path, aclTokensPath+"/"):
		h.management.ServeTokensHTTP(w, r)
	case path == aclRolesPath || strings.HasPrefix(path, aclRolesPath+"/"):
		h.management.ServeRolesHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *AccessControlHandler) serveOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.management.requireManagement(w, r) {
		return
	}

	client, queryOptions, ok := h.management.readableClient(w, r)
	if !ok {
		return
	}
	policies, ok := h.management.listPolicies(w, r, client, queryOptions, "list ACL policies")
	if !ok {
		return
	}
	tokens, _, err := client.ACLTokens().List(queryOptions)
	if err != nil {
		h.management.renderAPIError(w, r, err, "list ACL tokens")
		return
	}
	roles, _, err := client.ACLRoles().List(queryOptions)
	if err != nil {
		h.management.renderAPIError(w, r, err, "list ACL roles")
		return
	}

	counts := accesscontrolpartials.Counts{
		Policies: len(policies),
		Tokens:   len(tokens),
		Roles:    len(roles),
	}
	props := h.handler.BaseLayoutProps(r, "Access Control - Wander", "Manage Wander access control", aclBasePath, []components.Breadcrumb{
		{Label: "Home", URL: "/ui/"},
		{Label: "Access Control"},
	})
	h.management.renderComponents(w, r, accesscontrol.Overview(props, counts), accesscontrolpartials.Overview(counts), "render access control overview")
}

func rewritePathPrefix(from, to string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clone := r.Clone(r.Context())
		urlCopy := *r.URL
		urlCopy.Path = to + strings.TrimPrefix(r.URL.Path, from)
		urlCopy.RawPath = ""
		clone.URL = &urlCopy
		next.ServeHTTP(w, clone)
	})
}

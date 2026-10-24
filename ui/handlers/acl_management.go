package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/roles"
	"github.com/open-wander/wander/ui/templates/pages/tokens"
	"github.com/open-wander/wander/ui/templates/partials/rolespartials"
	"github.com/open-wander/wander/ui/templates/partials/tokenspartials"
)

// ACLManagementHandler provides clean-room UI management for ACL tokens and
// roles using the public Wander API client.
type ACLManagementHandler struct {
	handler *Handler
}

func NewACLManagementHandler(handler *Handler) *ACLManagementHandler {
	return &ACLManagementHandler{handler: handler}
}

func (h *ACLManagementHandler) ServeTokensHTTP(w http.ResponseWriter, r *http.Request) {
	h.serveResource(w, r, "/ui/tokens", h.listTokens, h.newToken, h.tokenDetail)
}

func (h *ACLManagementHandler) ServeRolesHTTP(w http.ResponseWriter, r *http.Request) {
	h.serveResource(w, r, "/ui/roles", h.listRoles, h.newRole, h.roleDetail)
}

func (h *ACLManagementHandler) serveResource(
	w http.ResponseWriter,
	r *http.Request,
	basePath string,
	list func(http.ResponseWriter, *http.Request),
	create func(http.ResponseWriter, *http.Request),
	detail func(http.ResponseWriter, *http.Request, string),
) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == basePath:
		if r.Method != http.MethodGet {
			h.methodNotAllowed(w)
			return
		}
		if !h.requireManagement(w, r) {
			return
		}
		list(w, r)
	case path == basePath+"/new":
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			h.methodNotAllowed(w)
			return
		}
		if !h.requireManagement(w, r) {
			return
		}
		create(w, r)
	case strings.HasPrefix(path, basePath+"/"):
		identifier, err := url.PathUnescape(strings.TrimPrefix(path, basePath+"/"))
		if err != nil || identifier == "" || strings.Contains(identifier, "/") {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodDelete {
			h.methodNotAllowed(w)
			return
		}
		if !h.requireManagement(w, r) {
			return
		}
		detail(w, r, identifier)
	default:
		http.NotFound(w, r)
	}
}

func (h *ACLManagementHandler) requireManagement(w http.ResponseWriter, r *http.Request) bool {
	permissions := GetPermissionsFromRequest(r)
	if permissions != nil && permissions.IsManagement() {
		return true
	}
	h.handler.RenderForbidden(w, r, ForbiddenError{
		Message:    "You do not have permission to manage ACL resources",
		Capability: "management token",
		Suggestions: []string{
			"ACL token and role management requires a management token",
			"Sign in with a management token or contact your administrator",
		},
	})
	return false
}

func (h *ACLManagementHandler) methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, POST, DELETE")
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (h *ACLManagementHandler) client(r *http.Request) (*api.Client, *api.QueryOptions, *api.WriteOptions) {
	client := h.handler.NomadClientForRequest(r)
	token := GetTokenFromRequest(r)
	region := GetRegionFromRequest(r)
	return client,
		&api.QueryOptions{Region: region, AuthToken: token},
		&api.WriteOptions{Region: region, AuthToken: token}
}

func (h *ACLManagementHandler) listTokens(w http.ResponseWriter, r *http.Request) {
	client, queryOptions, _ := h.client(r)
	if client == nil {
		h.renderUnavailable(w, r)
		return
	}

	apiTokens, _, err := client.ACLTokens().List(queryOptions)
	if err != nil {
		h.renderAPIError(w, r, err, "list ACL tokens")
		return
	}

	search := normalizedSearch(r)
	items := filterMapSort(apiTokens, search,
		func(token *api.ACLTokenListStub, query string) bool {
			return containsFold(token.Name, query) || containsFold(token.AccessorID, query)
		},
		tokenSummary,
		func(left, right tokenspartials.TokenSummary) bool {
			if left.Name == right.Name {
				return left.AccessorID < right.AccessorID
			}
			return left.Name < right.Name
		},
	)

	props := h.handler.BaseLayoutProps(r, "ACL Tokens - Wander", "Manage Wander ACL tokens", "/ui/tokens", aclBreadcrumbs("Tokens"))
	h.renderComponents(w, r, tokens.List(props, items, search), tokenspartials.ListContent(items, search), "render ACL token list")
}

func (h *ACLManagementHandler) newToken(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.renderTokenForm(w, r, tokenspartials.TokenForm{IsNew: true})
		return
	}

	token, validationErr := parseTokenForm(r, "")
	if validationErr != "" {
		h.renderTokenForm(w, r, tokenspartials.TokenForm{IsNew: true, Token: tokenView(token), Error: validationErr})
		return
	}

	client, _, writeOptions := h.client(r)
	if client == nil {
		h.renderUnavailable(w, r)
		return
	}
	created, _, err := client.ACLTokens().Create(token, writeOptions)
	if err != nil {
		h.renderAPIError(w, r, err, "create ACL token")
		return
	}

	target := "/ui/tokens/" + url.PathEscape(created.AccessorID)
	h.renderTokenForm(w, requestWithURL(r, target), tokenspartials.TokenForm{Token: tokenView(created), Secret: created.SecretID})
}

func (h *ACLManagementHandler) tokenDetail(w http.ResponseWriter, r *http.Request, accessorID string) {
	switch r.Method {
	case http.MethodGet:
		token, ok := h.getToken(w, r, accessorID)
		if ok {
			h.renderTokenForm(w, r, tokenspartials.TokenForm{Token: tokenView(token)})
		}
	case http.MethodPost:
		token, validationErr := parseTokenForm(r, accessorID)
		if validationErr != "" {
			h.renderTokenForm(w, r, tokenspartials.TokenForm{Token: tokenView(token), Error: validationErr})
			return
		}
		client, _, writeOptions := h.client(r)
		if client == nil {
			h.renderUnavailable(w, r)
			return
		}
		updated, _, err := client.ACLTokens().Update(token, writeOptions)
		if err != nil {
			h.renderAPIError(w, r, err, "update ACL token")
			return
		}
		w.Header().Set("HX-Trigger", `{"showToast":{"message":"Token updated successfully","type":"success"}}`)
		h.renderTokenForm(w, r, tokenspartials.TokenForm{Token: tokenView(updated)})
	case http.MethodDelete:
		client, _, writeOptions := h.client(r)
		if client == nil {
			h.renderUnavailable(w, r)
			return
		}
		if _, err := client.ACLTokens().Delete(accessorID, writeOptions); err != nil {
			h.renderAPIError(w, r, err, "delete ACL token")
			return
		}
		h.redirectAfterMutation(w, r, "/ui/tokens", "Token deleted successfully")
	default:
		h.methodNotAllowed(w)
	}
}

func (h *ACLManagementHandler) getToken(w http.ResponseWriter, r *http.Request, accessorID string) (*api.ACLToken, bool) {
	client, queryOptions, _ := h.client(r)
	if client == nil {
		h.renderUnavailable(w, r)
		return nil, false
	}
	token, _, err := client.ACLTokens().Info(accessorID, queryOptions)
	if err != nil {
		h.renderAPIError(w, r, err, "read ACL token")
		return nil, false
	}
	return token, true
}

func (h *ACLManagementHandler) renderTokenForm(w http.ResponseWriter, r *http.Request, form tokenspartials.TokenForm) {
	client, queryOptions, _ := h.client(r)
	if client == nil {
		h.renderUnavailable(w, r)
		return
	}
	policiesList, _, policyErr := client.ACLPolicies().List(queryOptions)
	rolesList, _, roleErr := client.ACLRoles().List(queryOptions)
	if policyErr != nil {
		h.renderAPIError(w, r, policyErr, "load ACL policies")
		return
	}
	if roleErr != nil {
		h.renderAPIError(w, r, roleErr, "load ACL roles")
		return
	}
	form.Policies = buildOptions(policiesList, form.Token.Policies, policyOptionFields)
	form.Roles = buildOptions(rolesList, form.Token.Roles, roleOptionFields)
	if permissions := GetPermissionsFromRequest(r); permissions != nil {
		currentToken := permissions.GetToken()
		form.IsCurrent = currentToken != nil && currentToken.AccessorID != "" && currentToken.AccessorID == form.Token.AccessorID
	}

	title := "New ACL Token"
	currentPath := "/ui/tokens/new"
	if !form.IsNew {
		title = form.Token.Name
		if title == "" {
			title = form.Token.AccessorID
		}
		currentPath = "/ui/tokens/" + url.PathEscape(form.Token.AccessorID)
	}
	props := h.handler.BaseLayoutProps(r, title+" - Wander", "Manage Wander ACL token", currentPath, aclBreadcrumbs(title))
	h.renderComponents(w, r, tokens.Detail(props, form), tokenspartials.DetailContent(form), "render ACL token")
}

func (h *ACLManagementHandler) listRoles(w http.ResponseWriter, r *http.Request) {
	client, queryOptions, _ := h.client(r)
	if client == nil {
		h.renderUnavailable(w, r)
		return
	}

	apiRoles, _, err := client.ACLRoles().List(queryOptions)
	if err != nil {
		h.renderAPIError(w, r, err, "list ACL roles")
		return
	}
	search := normalizedSearch(r)
	items := filterMapSort(apiRoles, search,
		func(role *api.ACLRoleListStub, query string) bool {
			return containsFold(role.Name, query) || containsFold(role.Description, query)
		},
		roleSummary,
		func(left, right rolespartials.RoleSummary) bool { return left.Name < right.Name },
	)

	props := h.handler.BaseLayoutProps(r, "ACL Roles - Wander", "Manage Wander ACL roles", "/ui/roles", aclBreadcrumbs("Roles"))
	h.renderComponents(w, r, roles.List(props, items, search), rolespartials.ListContent(items, search), "render ACL role list")
}

func (h *ACLManagementHandler) newRole(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.renderRoleForm(w, r, rolespartials.RoleForm{IsNew: true})
		return
	}
	role, validationErr := parseRoleForm(r, "")
	if validationErr != "" {
		h.renderRoleForm(w, r, rolespartials.RoleForm{IsNew: true, Role: roleView(role), Error: validationErr})
		return
	}
	client, _, writeOptions := h.client(r)
	if client == nil {
		h.renderUnavailable(w, r)
		return
	}
	created, _, err := client.ACLRoles().Create(role, writeOptions)
	if err != nil {
		h.renderAPIError(w, r, err, "create ACL role")
		return
	}
	target := "/ui/roles/" + url.PathEscape(created.ID)
	h.renderRoleForm(w, requestWithURL(r, target), rolespartials.RoleForm{Role: roleView(created)})
}

func (h *ACLManagementHandler) roleDetail(w http.ResponseWriter, r *http.Request, roleID string) {
	switch r.Method {
	case http.MethodGet:
		role, ok := h.getRole(w, r, roleID)
		if ok {
			h.renderRoleForm(w, r, rolespartials.RoleForm{Role: roleView(role)})
		}
	case http.MethodPost:
		role, validationErr := parseRoleForm(r, roleID)
		if validationErr != "" {
			h.renderRoleForm(w, r, rolespartials.RoleForm{Role: roleView(role), Error: validationErr})
			return
		}
		client, _, writeOptions := h.client(r)
		if client == nil {
			h.renderUnavailable(w, r)
			return
		}
		updated, _, err := client.ACLRoles().Update(role, writeOptions)
		if err != nil {
			h.renderAPIError(w, r, err, "update ACL role")
			return
		}
		w.Header().Set("HX-Trigger", `{"showToast":{"message":"Role updated successfully","type":"success"}}`)
		h.renderRoleForm(w, r, rolespartials.RoleForm{Role: roleView(updated)})
	case http.MethodDelete:
		client, _, writeOptions := h.client(r)
		if client == nil {
			h.renderUnavailable(w, r)
			return
		}
		if _, err := client.ACLRoles().Delete(roleID, writeOptions); err != nil {
			h.renderAPIError(w, r, err, "delete ACL role")
			return
		}
		h.redirectAfterMutation(w, r, "/ui/roles", "Role deleted successfully")
	default:
		h.methodNotAllowed(w)
	}
}

func (h *ACLManagementHandler) getRole(w http.ResponseWriter, r *http.Request, roleID string) (*api.ACLRole, bool) {
	client, queryOptions, _ := h.client(r)
	if client == nil {
		h.renderUnavailable(w, r)
		return nil, false
	}
	role, _, err := client.ACLRoles().Get(roleID, queryOptions)
	if err != nil {
		h.renderAPIError(w, r, err, "read ACL role")
		return nil, false
	}
	return role, true
}

func (h *ACLManagementHandler) renderRoleForm(w http.ResponseWriter, r *http.Request, form rolespartials.RoleForm) {
	client, queryOptions, _ := h.client(r)
	if client == nil {
		h.renderUnavailable(w, r)
		return
	}
	policiesList, _, err := client.ACLPolicies().List(queryOptions)
	if err != nil {
		h.renderAPIError(w, r, err, "load ACL policies")
		return
	}
	form.Policies = buildOptions(policiesList, form.Role.Policies, policyOptionFields)

	title := "New ACL Role"
	currentPath := "/ui/roles/new"
	if !form.IsNew {
		title = form.Role.Name
		currentPath = "/ui/roles/" + url.PathEscape(form.Role.ID)
	}
	props := h.handler.BaseLayoutProps(r, title+" - Wander", "Manage Wander ACL role", currentPath, aclBreadcrumbs(title))
	h.renderComponents(w, r, roles.Detail(props, form), rolespartials.DetailContent(form), "render ACL role")
}

func (h *ACLManagementHandler) renderAPIError(w http.ResponseWriter, r *http.Request, err error, action string) {
	h.handler.logger.Warn("ACL management API request failed", "action", action, "error", err)
	if h.handler.CheckClusterError(w, r, err) {
		return
	}
	if apiResponseHasStatus(err, http.StatusNotFound) {
		h.handler.RenderNotFound(w, r, NotFoundError{Message: "The requested ACL resource was not found", RequestPath: r.URL.Path})
		return
	}
	status := http.StatusInternalServerError
	if apiResponseHasStatus(err, http.StatusBadRequest) {
		status = http.StatusBadRequest
	} else if apiResponseHasStatus(err, http.StatusForbidden) {
		status = http.StatusForbidden
	}
	http.Error(w, "Unable to "+action, status)
}

func (h *ACLManagementHandler) renderUnavailable(w http.ResponseWriter, r *http.Request) {
	h.handler.RenderClusterUnavailable(w, r, ClusterUnavailableError{Message: "The Wander API client is unavailable."})
}

func (h *ACLManagementHandler) renderFailure(w http.ResponseWriter, action string, err error) {
	h.handler.logger.Error(action, "error", err)
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}

func (h *ACLManagementHandler) renderComponents(w http.ResponseWriter, r *http.Request, fullPage, partial templ.Component, action string) {
	if err := Render(w, r, fullPage, partial); err != nil {
		h.renderFailure(w, action, err)
	}
}

func (h *ACLManagementHandler) redirectAfterMutation(w http.ResponseWriter, r *http.Request, target, message string) {
	if IsHTMXFromRequest(r) {
		w.Header().Set("HX-Redirect", target)
		w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast":{"message":%q,"type":"success"}}`, message))
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func parseTokenForm(r *http.Request, accessorID string) (*api.ACLToken, string) {
	if err := r.ParseForm(); err != nil {
		return &api.ACLToken{AccessorID: accessorID}, "The submitted form is invalid."
	}
	tokenType := strings.TrimSpace(r.FormValue("type"))
	if tokenType != "client" && tokenType != "management" {
		return &api.ACLToken{AccessorID: accessorID}, "Token type must be client or management."
	}
	token := &api.ACLToken{
		AccessorID: accessorID,
		Name:       strings.TrimSpace(r.FormValue("name")),
		Type:       tokenType,
		Global:     r.Form.Has("global"),
		Policies:   uniqueValues(r.Form["policies"]),
	}
	for _, roleID := range uniqueValues(r.Form["roles"]) {
		token.Roles = append(token.Roles, &api.ACLTokenRoleLink{ID: roleID})
	}
	if accessorID == "" {
		ttl := strings.TrimSpace(r.FormValue("expiration_ttl"))
		if ttl != "" {
			duration, err := time.ParseDuration(ttl)
			if err != nil || duration <= 0 {
				return token, "Expiration TTL must be a positive duration such as 24h or 30m."
			}
			token.ExpirationTTL = duration
		}
	}
	return token, ""
}

func parseRoleForm(r *http.Request, roleID string) (*api.ACLRole, string) {
	if err := r.ParseForm(); err != nil {
		return &api.ACLRole{ID: roleID}, "The submitted form is invalid."
	}
	role := &api.ACLRole{
		ID:          roleID,
		Name:        strings.TrimSpace(r.FormValue("name")),
		Description: strings.TrimSpace(r.FormValue("description")),
	}
	if role.Name == "" {
		return role, "Role name is required."
	}
	for _, policyName := range uniqueValues(r.Form["policies"]) {
		role.Policies = append(role.Policies, &api.ACLRolePolicyLink{Name: policyName})
	}
	if len(role.Policies) == 0 {
		return role, "Select at least one policy."
	}
	return role, ""
}

func uniqueValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func tokenSummary(token *api.ACLTokenListStub) tokenspartials.TokenSummary {
	return tokenspartials.TokenSummary{
		AccessorID:     token.AccessorID,
		Name:           token.Name,
		Type:           token.Type,
		Global:         token.Global,
		PolicyCount:    len(token.Policies),
		RoleCount:      len(token.Roles),
		ExpirationTime: token.ExpirationTime,
	}
}

func tokenView(token *api.ACLToken) tokenspartials.TokenView {
	view := tokenspartials.TokenView{
		AccessorID:     token.AccessorID,
		Name:           token.Name,
		Type:           token.Type,
		Global:         token.Global,
		Policies:       append([]string(nil), token.Policies...),
		ExpirationTime: token.ExpirationTime,
		CreateTime:     token.CreateTime,
		CreateIndex:    token.CreateIndex,
		ModifyIndex:    token.ModifyIndex,
	}
	for _, role := range token.Roles {
		view.Roles = append(view.Roles, role.ID)
	}
	return view
}

func roleSummary(role *api.ACLRoleListStub) rolespartials.RoleSummary {
	return rolespartials.RoleSummary{ID: role.ID, Name: role.Name, Description: role.Description, PolicyCount: len(role.Policies)}
}

func roleView(role *api.ACLRole) rolespartials.RoleView {
	view := rolespartials.RoleView{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		CreateIndex: role.CreateIndex,
		ModifyIndex: role.ModifyIndex,
	}
	for _, policy := range role.Policies {
		view.Policies = append(view.Policies, policy.Name)
	}
	return view
}

func policyOptionFields(policy *api.ACLPolicyListStub) (string, string, string) {
	return policy.Name, policy.Name, policy.Description
}

func roleOptionFields(role *api.ACLRoleListStub) (string, string, string) {
	return role.ID, role.Name, role.Description
}

func buildOptions[T any](items []T, selected []string, fields func(T) (string, string, string)) []tokenspartials.Option {
	selectedSet := stringSet(selected)
	options := make([]tokenspartials.Option, 0, len(items))
	for _, item := range items {
		value, label, description := fields(item)
		_, checked := selectedSet[value]
		options = append(options, tokenspartials.Option{Value: value, Label: label, Description: description, Selected: checked})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].Label < options[j].Label })
	return options
}

func normalizedSearch(r *http.Request) string {
	return strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
}

func containsFold(value, normalizedQuery string) bool {
	return strings.Contains(strings.ToLower(value), normalizedQuery)
}

func filterMapSort[T any, U any](source []T, search string, matches func(T, string) bool, convert func(T) U, less func(U, U) bool) []U {
	items := make([]U, 0, len(source))
	for _, sourceItem := range source {
		if search != "" && !matches(sourceItem, search) {
			continue
		}
		items = append(items, convert(sourceItem))
	}
	sort.Slice(items, func(i, j int) bool { return less(items[i], items[j]) })
	return items
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func requestWithURL(r *http.Request, target string) *http.Request {
	clone := r.Clone(r.Context())
	urlCopy := *r.URL
	urlCopy.Path = target
	urlCopy.RawPath = ""
	urlCopy.RawQuery = ""
	clone.URL = &urlCopy
	return clone
}

func aclBreadcrumbs(label string) []components.Breadcrumb {
	return []components.Breadcrumb{{Label: "Home", URL: "/ui/"}, {Label: "Access Control", URL: "/ui/policies"}, {Label: label}}
}

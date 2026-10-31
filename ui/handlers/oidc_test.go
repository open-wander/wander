// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/api"
	"github.com/shoenig/test/must"
)

func TestSettingsOIDCMethodsFiltersJWT(t *testing.T) {
	backend := newOIDCMethodsTestServer(t, []map[string]any{
		{"Name": "jwt-only", "Type": "JWT", "Default": true},
		{"Name": "secondary", "Type": "OIDC"},
		{"Name": "primary", "Type": "OIDC", "Default": true},
	}, nil)

	handler := newOIDCTestSettingsHandler(t, backend.URL)
	methods := handler.oidcMethods(httptest.NewRequest(http.MethodGet, oidcCallbackPath, nil))
	must.Len(t, 2, methods)
	must.Eq(t, "primary", methods[0].Name)
	must.True(t, methods[0].Default)
	must.Eq(t, "secondary", methods[1].Name)
}

func TestSettingsTokensRendersOIDCWithoutJWTAction(t *testing.T) {
	backend := newOIDCMethodsTestServer(t, []map[string]any{
		{"Name": "company-sso", "Type": "OIDC"},
		{"Name": "automation-jwt", "Type": "JWT"},
	}, nil)

	settingsHandler := newOIDCTestSettingsHandler(t, backend.URL)
	response := httptest.NewRecorder()
	settingsHandler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, oidcCallbackPath, nil))

	must.Eq(t, http.StatusOK, response.Code)
	must.StrContains(t, response.Body.String(), "Sign in with SSO")
	must.StrContains(t, response.Body.String(), "Sign in with company-sso")
	must.StrNotContains(t, response.Body.String(), "automation-jwt")
}

func TestSettingsStartOIDC(t *testing.T) {
	var authRequest api.ACLOIDCAuthURLRequest
	backend := newOIDCMethodsTestServer(t,
		[]map[string]any{{"Name": "company-sso", "Type": "OIDC"}},
		func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != "/v1/acl/oidc/auth-url" {
				return false
			}
			must.Eq(t, http.MethodPut, r.Method)
			must.NoError(t, json.NewDecoder(r.Body).Decode(&authRequest))
			writeJSON(t, w, map[string]string{"AuthURL": "https://identity.example/authorize"})
			return true
		})

	handler := newOIDCTestSettingsHandler(t, backend.URL)
	response := performOIDCStart(handler, "https://wander.example/ui/settings/oidc", "company-sso")

	must.Eq(t, http.StatusSeeOther, response.Code)
	must.Eq(t, "https://identity.example/authorize", response.Header().Get("Location"))
	must.Eq(t, "company-sso", authRequest.AuthMethodName)
	must.Eq(t, "https://wander.example/ui/settings/tokens", authRequest.RedirectURI)
	must.NotEq(t, "", authRequest.ClientNonce)

	transactionCookie := responseCookie(t, response, oidcTransactionCookie)
	assertCookieSecurity(t, transactionCookie, true, http.SameSiteLaxMode)
	transaction, err := handler.handler.verifyOIDCTransaction(transactionCookie.Value)
	must.NoError(t, err)
	must.Eq(t, authRequest.ClientNonce, transaction.ClientNonce)
	must.Eq(t, authRequest.RedirectURI, transaction.RedirectURI)
}

func TestSettingsStartOIDCRejectsJWTMethod(t *testing.T) {
	backend := newOIDCMethodsTestServer(t, []map[string]any{{"Name": "jwt-only", "Type": "JWT"}}, nil)

	handler := newOIDCTestSettingsHandler(t, backend.URL)
	response := performOIDCStart(handler, "http://wander.example/ui/settings/oidc", "jwt-only")

	must.Eq(t, http.StatusBadRequest, response.Code)
	must.StrContains(t, response.Body.String(), "selected SSO method is unavailable")
}

func TestSettingsCompleteOIDC(t *testing.T) {
	const secretID = "oidc-generated-secret"
	var completeRequest api.ACLOIDCCompleteAuthRequest
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/acl/oidc/complete-auth" {
			http.NotFound(w, r)
			return
		}
		must.NoError(t, json.NewDecoder(r.Body).Decode(&completeRequest))
		writeJSON(t, w, map[string]any{
			"AccessorID": "accessor",
			"SecretID":   secretID,
			"Name":       "OIDC-company-sso",
			"Type":       "client",
		})
	}))
	t.Cleanup(backend.Close)

	handler := newOIDCTestSettingsHandler(t, backend.URL)
	encoded := signedOIDCTestTransaction(t, handler.handler, "https://wander.example/ui/settings/tokens", time.Now().Add(time.Minute))
	response := performOIDCComplete(handler, encoded, "provider-state", "provider-code")

	must.Eq(t, http.StatusSeeOther, response.Code)
	must.Eq(t, oidcCallbackPath, response.Header().Get("Location"))
	must.Eq(t, "company-sso", completeRequest.AuthMethodName)
	must.Eq(t, "browser-nonce", completeRequest.ClientNonce)
	must.Eq(t, "provider-state", completeRequest.State)
	must.Eq(t, "provider-code", completeRequest.Code)
	must.Eq(t, "https://wander.example/ui/settings/tokens", completeRequest.RedirectURI)

	tokenCookie := responseCookie(t, response, WanderTokenCookie)
	must.Eq(t, secretID, tokenCookie.Value)
	assertCookieSecurity(t, tokenCookie, true, http.SameSiteStrictMode)
	must.StrNotContains(t, response.Body.String(), secretID)
	clearedTransaction := responseCookie(t, response, oidcTransactionCookie)
	must.Eq(t, -1, clearedTransaction.MaxAge)
}

func TestSettingsCompleteOIDCRejectsTamperedTransaction(t *testing.T) {
	handler := newOIDCTestSettingsHandler(t, "http://127.0.0.1:1")
	encoded := signedOIDCTestTransaction(t, handler.handler, "http://wander.example/ui/settings/tokens", time.Now().Add(time.Minute))
	encoded = "A" + encoded[1:]
	response := performOIDCComplete(handler, encoded, "state", "code")

	must.Eq(t, http.StatusBadRequest, response.Code)
	must.StrContains(t, response.Body.String(), "session is invalid or has expired")
	must.StrNotContains(t, response.Body.String(), encoded)
}

func TestSettingsOIDCCallbackRejectsMissingTransaction(t *testing.T) {
	backend := newOIDCMethodsTestServer(t, nil, nil)
	handler := newOIDCTestSettingsHandler(t, backend.URL)
	request := httptest.NewRequest(http.MethodGet, oidcCallbackPath+"?state=state&code=code", nil)
	response := serveSettingsRequest(handler, request)

	must.Eq(t, http.StatusBadRequest, response.Code)
	must.StrContains(t, response.Body.String(), "session is missing or has expired")
}

func TestSettingsOIDCCallbackHandlesProviderError(t *testing.T) {
	backend := newOIDCMethodsTestServer(t, nil, nil)
	handler := newOIDCTestSettingsHandler(t, backend.URL)
	encoded := signedOIDCTestTransaction(t, handler.handler, "https://wander.example/ui/settings/tokens", time.Now().Add(time.Minute))
	request := httptest.NewRequest(http.MethodGet, oidcCallbackPath+"?error=access_denied&error_description=sensitive", nil)
	request.AddCookie(&http.Cookie{Name: oidcTransactionCookie, Value: encoded})
	response := serveSettingsRequest(handler, request)

	must.Eq(t, http.StatusBadRequest, response.Code)
	must.StrContains(t, response.Body.String(), "SSO sign-in was not completed")
	must.StrNotContains(t, response.Body.String(), "sensitive")
	must.Eq(t, -1, responseCookie(t, response, oidcTransactionCookie).MaxAge)
}

func TestVerifyOIDCTransactionRejectsExpiredValue(t *testing.T) {
	handler := NewHandler(hclog.NewNullLogger())
	encoded := signedOIDCTestTransaction(t, handler, "http://wander.example/ui/settings/tokens", time.Now().Add(-time.Second))
	_, err := handler.verifyOIDCTransaction(encoded)
	must.ErrorIs(t, err, errInvalidOIDCTransaction)
}

func newOIDCStartRequest(target, authMethod string) *http.Request {
	form := url.Values{"auth_method": {authMethod}}
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", request.URL.Scheme+"://"+request.URL.Host)
	return request
}

func performOIDCStart(handler *SettingsHandler, target, authMethod string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.StartOIDC(response, newOIDCStartRequest(target, authMethod))
	return response
}

func newOIDCCallbackRequest(transaction, state, code string) *http.Request {
	query := url.Values{"state": {state}, "code": {code}}
	request := httptest.NewRequest(http.MethodGet, oidcCallbackPath+"?"+query.Encode(), nil)
	request.AddCookie(&http.Cookie{Name: oidcTransactionCookie, Value: transaction})
	return request
}

func performOIDCComplete(handler *SettingsHandler, transaction, state, code string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.CompleteOIDC(response, newOIDCCallbackRequest(transaction, state, code))
	return response
}

func serveSettingsRequest(handler *SettingsHandler, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func signedOIDCTestTransaction(t *testing.T, handler *Handler, redirectURI string, expiresAt time.Time) string {
	t.Helper()
	encoded, err := handler.signOIDCTransaction(oidcTransaction{
		AuthMethod:  "company-sso",
		ClientNonce: "browser-nonce",
		RedirectURI: redirectURI,
		ExpiresAt:   expiresAt.Unix(),
	})
	must.NoError(t, err)
	return encoded
}

func TestOIDCRedirectURI(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "https://wander.example:8443/ui/settings/oidc", nil)
	request.Header.Set("Origin", "https://wander.example:8443")
	redirectURI, err := oidcRedirectURI(request)
	must.NoError(t, err)
	must.Eq(t, "https://wander.example:8443/ui/settings/tokens", redirectURI)

	for _, invalidOrigin := range []string{"https://wander.example/unsafe", "https://other.example:8443"} {
		request.Header.Set("Origin", invalidOrigin)
		_, err = oidcRedirectURI(request)
		must.Error(t, err)
	}
}

func newOIDCMethodsTestServer(t *testing.T, methods []map[string]any, extra func(http.ResponseWriter, *http.Request) bool) *httptest.Server {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/acl/auth-methods":
			writeJSON(t, w, methods)
		case "/v1/acl/token/self":
			http.Error(w, "ACL token not found", http.StatusForbidden)
		case "/v1/namespaces", "/v1/regions":
			writeJSON(t, w, []string{})
		default:
			if extra == nil || !extra(w, r) {
				http.NotFound(w, r)
			}
		}
	}))
	t.Cleanup(backend.Close)
	return backend
}

func newOIDCTestSettingsHandler(t *testing.T, address string) *SettingsHandler {
	t.Helper()
	config := api.DefaultConfig()
	config.Address = address
	return NewSettingsHandler(NewHandlerWithAPIConfig(hclog.NewNullLogger(), config))
}

func responseCookie(t *testing.T, response *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response did not set cookie %q", name)
	return nil
}

func assertCookieSecurity(t *testing.T, cookie *http.Cookie, secure bool, sameSite http.SameSite) {
	t.Helper()
	must.True(t, cookie.HttpOnly)
	must.Eq(t, secure, cookie.Secure)
	must.Eq(t, sameSite, cookie.SameSite)
}

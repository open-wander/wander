// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/partials/settingspartials"
)

const (
	oidcTransactionCookie = "wander_oidc_transaction" // #nosec G101 -- cookie name, not a credential.
	oidcCallbackPath      = "/ui/settings/tokens"
	oidcTransactionTTL    = 10 * time.Minute
)

var errInvalidOIDCTransaction = errors.New("invalid or expired OIDC login transaction")

type oidcTransaction struct {
	AuthMethod  string `json:"auth_method"`
	ClientNonce string `json:"client_nonce"`
	RedirectURI string `json:"redirect_uri"`
	ExpiresAt   int64  `json:"expires_at"`
}

func (h *SettingsHandler) oidcMethods(r *http.Request) []settingspartials.OIDCAuthMethod {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil
	}

	methods, _, err := client.ACLAuthMethods().List(&api.QueryOptions{
		Region:    GetRegionFromRequest(r),
		AuthToken: GetTokenFromRequest(r),
	})
	if err != nil {
		h.handler.logger.Debug("failed to list OIDC auth methods", "error", err)
		return nil
	}

	oidcMethods := make([]settingspartials.OIDCAuthMethod, 0, len(methods))
	for _, method := range methods {
		if method == nil || !strings.EqualFold(method.Type, api.ACLAuthMethodTypeOIDC) {
			continue
		}
		oidcMethods = append(oidcMethods, settingspartials.OIDCAuthMethod{
			Name:    method.Name,
			Default: method.Default,
		})
	}
	sort.SliceStable(oidcMethods, func(i, j int) bool {
		if oidcMethods[i].Default != oidcMethods[j].Default {
			return oidcMethods[i].Default
		}
		return oidcMethods[i].Name < oidcMethods[j].Name
	})
	return oidcMethods
}

func (h *SettingsHandler) StartOIDC(w http.ResponseWriter, r *http.Request) {
	if len(h.handler.oidcCookieKey) == 0 {
		h.renderOIDCError(w, r, "SSO sign-in is unavailable. Please use an ACL token.")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderOIDCError(w, r, "The SSO sign-in request was invalid.")
		return
	}

	methodName := r.FormValue("auth_method")
	if !containsOIDCMethod(h.oidcMethods(r), methodName) {
		h.renderOIDCError(w, r, "The selected SSO method is unavailable.")
		return
	}

	redirectURI, err := oidcRedirectURI(r)
	if err != nil {
		h.renderOIDCError(w, r, "Wander could not determine a safe SSO callback address.")
		return
	}
	nonce, err := randomURLToken(32)
	if err != nil {
		h.handler.logger.Error("failed to create OIDC client nonce", "error", err)
		h.renderOIDCError(w, r, "SSO sign-in is temporarily unavailable.")
		return
	}

	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.renderOIDCError(w, r, "Wander could not contact the cluster.")
		return
	}
	authURL, _, err := client.ACLAuth().GetAuthURL(&api.ACLOIDCAuthURLRequest{
		AuthMethodName: methodName,
		RedirectURI:    redirectURI,
		ClientNonce:    nonce,
	}, &api.WriteOptions{Region: GetRegionFromRequest(r)})
	if err != nil || authURL == nil || authURL.AuthURL == "" {
		h.handler.logger.Warn("failed to start OIDC authentication", "auth_method", methodName, "error", err)
		h.renderOIDCError(w, r, "Wander could not start SSO sign-in.")
		return
	}

	transaction := oidcTransaction{
		AuthMethod:  methodName,
		ClientNonce: nonce,
		RedirectURI: redirectURI,
		ExpiresAt:   time.Now().Add(oidcTransactionTTL).Unix(),
	}
	encoded, err := h.handler.signOIDCTransaction(transaction)
	if err != nil {
		h.handler.logger.Error("failed to encode OIDC transaction", "error", err)
		h.renderOIDCError(w, r, "SSO sign-in is temporarily unavailable.")
		return
	}
	secure := strings.HasPrefix(redirectURI, "https://")
	http.SetCookie(w, oidcCookie(oidcTransactionCookie, encoded, int(oidcTransactionTTL.Seconds()), secure))
	http.Redirect(w, r, authURL.AuthURL, http.StatusSeeOther)
}

func (h *SettingsHandler) CompleteOIDC(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(oidcTransactionCookie)
	if err != nil {
		h.renderOIDCError(w, r, "The SSO sign-in session is missing or has expired. Please try again.")
		return
	}
	transaction, err := h.handler.verifyOIDCTransaction(cookie.Value)
	if err != nil {
		h.clearOIDCTransaction(w, requestUsesTLS(r))
		h.renderOIDCError(w, r, "The SSO sign-in session is invalid or has expired. Please try again.")
		return
	}
	secure := strings.HasPrefix(transaction.RedirectURI, "https://")
	h.clearOIDCTransaction(w, secure)

	if r.URL.Query().Get("error") != "" {
		h.renderOIDCError(w, r, "SSO sign-in was not completed. Please try again.")
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		h.renderOIDCError(w, r, "The SSO provider returned an incomplete response. Please try again.")
		return
	}

	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.renderOIDCError(w, r, "Wander could not contact the cluster.")
		return
	}
	token, _, err := client.ACLAuth().CompleteAuth(&api.ACLOIDCCompleteAuthRequest{
		AuthMethodName: transaction.AuthMethod,
		ClientNonce:    transaction.ClientNonce,
		State:          state,
		Code:           code,
		RedirectURI:    transaction.RedirectURI,
	}, &api.WriteOptions{Region: GetRegionFromRequest(r)})
	if err != nil || token == nil || token.SecretID == "" {
		h.handler.logger.Warn("failed to complete OIDC authentication", "auth_method", transaction.AuthMethod, "error", err)
		h.renderOIDCError(w, r, "SSO sign-in could not be completed. Please try again.")
		return
	}

	http.SetCookie(w, browserCookieWithSecure(WanderTokenCookie, token.SecretID, 86400*7, http.SameSiteStrictMode, secure))
	http.Redirect(w, r, oidcCallbackPath, http.StatusSeeOther)
}

func (h *SettingsHandler) renderOIDCError(w http.ResponseWriter, r *http.Request, message string) {
	w.WriteHeader(http.StatusBadRequest)
	h.ShowWithError(w, r, message)
}

func (h *SettingsHandler) clearOIDCTransaction(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, oidcCookie(oidcTransactionCookie, "", -1, secure))
}

func containsOIDCMethod(methods []settingspartials.OIDCAuthMethod, name string) bool {
	for _, method := range methods {
		if method.Name == name {
			return true
		}
	}
	return false
}

func oidcRedirectURI(r *http.Request) (string, error) {
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
			parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", fmt.Errorf("invalid request origin")
		}
		if !strings.EqualFold(parsed.Host, r.Host) {
			return "", fmt.Errorf("request origin does not match host")
		}
		return parsed.String() + oidcCallbackPath, nil
	}
	if r.Host == "" {
		return "", fmt.Errorf("missing request host")
	}
	scheme := "http"
	if requestUsesTLS(r) {
		scheme = "https"
	}
	return (&url.URL{Scheme: scheme, Host: r.Host, Path: oidcCallbackPath}).String(), nil
}

func randomURLToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (h *Handler) signOIDCTransaction(transaction oidcTransaction) (string, error) {
	if len(h.oidcCookieKey) == 0 {
		return "", errInvalidOIDCTransaction
	}
	payload, err := json.Marshal(transaction)
	if err != nil {
		return "", err
	}
	signature := hmac.New(sha256.New, h.oidcCookieKey)
	_, _ = signature.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(signature.Sum(nil)), nil
}

func (h *Handler) verifyOIDCTransaction(encoded string) (oidcTransaction, error) {
	var transaction oidcTransaction
	parts := strings.Split(encoded, ".")
	if len(parts) != 2 || len(h.oidcCookieKey) == 0 {
		return transaction, errInvalidOIDCTransaction
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return transaction, errInvalidOIDCTransaction
	}
	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return transaction, errInvalidOIDCTransaction
	}
	expectedSignature := hmac.New(sha256.New, h.oidcCookieKey)
	_, _ = expectedSignature.Write(payload)
	if !hmac.Equal(providedSignature, expectedSignature.Sum(nil)) {
		return transaction, errInvalidOIDCTransaction
	}
	if err := json.Unmarshal(payload, &transaction); err != nil {
		return transaction, errInvalidOIDCTransaction
	}
	if transaction.AuthMethod == "" || transaction.ClientNonce == "" || transaction.RedirectURI == "" ||
		time.Now().Unix() >= transaction.ExpiresAt {
		return oidcTransaction{}, errInvalidOIDCTransaction
	}
	return transaction, nil
}

func oidcCookie(name, value string, maxAge int, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

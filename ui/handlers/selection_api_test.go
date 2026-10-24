package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSelectionEndpointsSetScopedCookiesAndExplicitRedirects(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/namespace/team-a":
			_ = json.NewEncoder(w).Encode(map[string]string{"Name": "team-a"})
		case "/v1/regions":
			_ = json.NewEncoder(w).Encode([]string{"global", "region-a"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()
	selectionHandler := NewSelectionAPIHandler(newJobsTestHandler(t, backend.URL))

	tests := []struct {
		name       string
		target     string
		currentURL string
		cookieName string
		cookieWant string
		queryWant  string
		handler    http.Handler
	}{
		{
			name:       "namespace",
			target:     "/api/namespace/select?ns=team-a",
			currentURL: "http://wander.test/ui/jobs?status=running&namespace=old",
			cookieName: WanderNamespaceCookie,
			cookieWant: "team-a",
			queryWant:  "namespace=team-a",
			handler:    selectionHandler,
		},
		{
			name:       "region",
			target:     "/api/region/select?region=region-a",
			currentURL: "http://wander.test/ui/topology?region=old",
			cookieName: WanderRegionCookie,
			cookieWant: "region-a",
			queryWant:  "region=region-a",
			handler:    selectionHandler,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.target, nil)
			request.Header.Set(HXCurrentURL, test.currentURL)
			response := httptest.NewRecorder()
			test.handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			cookies := response.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("cookie count = %d, want 1", len(cookies))
			}
			cookie := cookies[0]
			if cookie.Name != test.cookieName || cookie.Value != test.cookieWant {
				t.Fatalf("cookie = %s=%s, want %s=%s", cookie.Name, cookie.Value, test.cookieName, test.cookieWant)
			}
			if !cookie.HttpOnly || cookie.Path != "/" || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge <= 0 {
				t.Fatalf("cookie security attributes missing: %#v", cookie)
			}
			redirect := response.Header().Get("HX-Redirect")
			if !strings.HasPrefix(redirect, "/ui/") || !strings.Contains(redirect, test.queryWant) {
				t.Fatalf("HX-Redirect = %q, want same-origin UI URL containing %q", redirect, test.queryWant)
			}
			if response.Header().Get("HX-Refresh") != "" {
				t.Fatal("unexpected HX-Refresh when current URL is available")
			}
		})
	}
}

func TestSelectionEndpointsRejectUnknownValues(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/regions" {
			_ = json.NewEncoder(w).Encode([]string{"global"})
			return
		}
		http.NotFound(w, r)
	}))
	defer backend.Close()
	handler := NewSelectionAPIHandler(newJobsTestHandler(t, backend.URL))

	for _, target := range []string{
		"/api/namespace/select?ns=missing",
		"/api/region/select?region=missing",
	} {
		request := httptest.NewRequest(http.MethodPost, target, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, want %d", target, response.Code, http.StatusBadRequest)
		}
		if len(response.Result().Cookies()) != 0 {
			t.Errorf("%s unexpectedly set a cookie", target)
		}
	}
}

func TestSelectionEndpointsRejectInvalidValues(t *testing.T) {
	tests := []struct {
		target  string
		handler http.Handler
	}{
		{target: "/api/namespace/select?ns=bad%2Fnamespace", handler: NewSelectionAPIHandler(NewHandler(nil))},
		{target: "/api/region/select?region=bad%20region", handler: NewSelectionAPIHandler(NewHandler(nil))},
	}

	for _, test := range tests {
		request := httptest.NewRequest(http.MethodPost, test.target, nil)
		response := httptest.NewRecorder()
		test.handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, want %d", test.target, response.Code, http.StatusBadRequest)
		}
		if len(response.Result().Cookies()) != 0 {
			t.Errorf("%s unexpectedly set a cookie", test.target)
		}
	}
}

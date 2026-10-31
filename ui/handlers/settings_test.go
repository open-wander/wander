// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

package handlers

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shoenig/test/must"
)

func TestRequestUsesTLS(t *testing.T) {
	httpRequest := httptest.NewRequest("GET", "http://wander.test/ui/settings", nil)
	must.False(t, requestUsesTLS(httpRequest))

	httpsRequest := httptest.NewRequest("GET", "https://wander.test/ui/settings", nil)
	if httpsRequest.TLS == nil {
		httpsRequest.TLS = &tls.ConnectionState{}
	}
	must.True(t, requestUsesTLS(httpsRequest))
}

func TestBrowserCookieSecurityAttributes(t *testing.T) {
	for _, test := range []struct {
		name   string
		target string
		secure bool
	}{
		{name: "http development", target: "http://wander.test/ui/settings"},
		{name: "https", target: "https://wander.test/ui/settings", secure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			cookie := browserCookie(request, WanderTokenCookie, "value", 60, http.SameSiteStrictMode)
			must.Eq(t, test.secure, cookie.Secure)
			must.True(t, cookie.HttpOnly)
			must.Eq(t, http.SameSiteStrictMode, cookie.SameSite)
			must.Eq(t, "/", cookie.Path)
			must.Eq(t, 60, cookie.MaxAge)
		})
	}
}

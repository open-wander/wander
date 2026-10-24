// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

package handlers

import (
	"crypto/tls"
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

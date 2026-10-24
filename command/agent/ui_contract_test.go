// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

//go:build ui

package agent

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const defaultUIContentSecurityPolicy = "default-src 'none'; connect-src *; img-src 'self' data:; script-src 'self'; style-src 'self' 'unsafe-inline'; form-action 'none'; frame-ancestors 'none'"

func TestUIRuntimeContract(t *testing.T) {
	t.Run("enabled by default with embedded assets", func(t *testing.T) {
		s := makeHTTPServer(t, nil)
		defer s.Shutdown()

		baseURL := "http://" + s.Agent.config.AdvertiseAddrs.HTTP
		assertUIResponse(t, baseURL+"/ui/jobs", "Wander")
		assertUIResponse(t, baseURL+"/ui/static/css/output.css", "--tw-")
	})

	t.Run("disabled by inherited ui configuration", func(t *testing.T) {
		s := makeHTTPServer(t, func(c *Config) {
			c.UI.Enabled = false
		})
		defer s.Shutdown()

		resp, err := noProxyHTTPClient().Get("http://" + s.Agent.config.AdvertiseAddrs.HTTP + "/ui/")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "Wander UI is disabled")
	})
}

func assertUIResponse(t *testing.T, url, bodyFragment string) {
	t.Helper()

	resp, err := noProxyHTTPClient().Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "response body: %s", body)
	require.Equal(t, defaultUIContentSecurityPolicy, resp.Header.Get("Content-Security-Policy"))
	require.True(t, strings.Contains(string(body), bodyFragment), "response body does not contain %q", bodyFragment)
}

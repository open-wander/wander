// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestLinkCheckerUsesGETAndWaitsForChildPages(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ui/", getOnlyHTML(t, `<a href="/ui/child">child</a><a href="/ui/child">child again</a><script src="/ui/static/app.js"></script><script src="/ui/static/app.js"></script>`))
	mux.HandleFunc("/ui/child", getOnlyHTML(t, `<main>child</main>`))
	mux.HandleFunc("/ui/static/app.js", getOnlyHTML(t, `console.log("loaded")`))

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	checker := NewLinkChecker(server.URL, hclog.NewNullLogger())
	results := checker.Check()

	require.Empty(t, results.BrokenLinks)
	require.Empty(t, results.MissingAssets)
	require.Equal(t, 2, results.TotalPages)
	require.Equal(t, 1, results.TotalLinks)
	require.Equal(t, 1, results.TotalAssets)
	require.ElementsMatch(t, []string{"/ui/", "/ui/child"}, results.VisitedPages)
}

func getOnlyHTML(t *testing.T, body string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write([]byte(body))
		require.NoError(t, err)
	}
}

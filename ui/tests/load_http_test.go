package tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// These benchmarks test HTTP handler performance without template dependencies.
// They demonstrate the load testing patterns for when templates are fixed.

// mockJobsHandler simulates a jobs list handler
func mockJobsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simulate filtering
	search := r.URL.Query().Get("search")
	if search != "" {
		// Filter logic simulation
		_ = strings.Contains("web-frontend", search)
	}

	// Simulate response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `<!DOCTYPE html><html><body><h1>Jobs</h1><table>`)
	for i := 0; i < 50; i++ {
		io.WriteString(w, `<tr><td>job-`)
		io.WriteString(w, string(rune('0'+i%10)))
		io.WriteString(w, `</td></tr>`)
	}
	io.WriteString(w, `</table></body></html>`)
}

// mockAllocationsHandler simulates an allocations list handler
func mockAllocationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `<!DOCTYPE html><html><body><h1>Allocations</h1><table>`)
	for i := 0; i < 100; i++ {
		io.WriteString(w, `<tr><td>alloc-`)
		io.WriteString(w, string(rune('0'+i%10)))
		io.WriteString(w, `</td></tr>`)
	}
	io.WriteString(w, `</table></body></html>`)
}

// mockStaticHandler simulates static file serving
func mockStaticHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simulate CSS file
	if strings.HasSuffix(r.URL.Path, ".css") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "body { margin: 0; padding: 0; }")
		return
	}

	// Simulate JS file
	if strings.HasSuffix(r.URL.Path, ".js") {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "(function() { console.log('loaded'); })();")
		return
	}

	http.NotFound(w, r)
}

// BenchmarkMockJobsList benchmarks concurrent requests to jobs list.
func BenchmarkMockJobsList(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/jobs", nil)
			w := httptest.NewRecorder()
			mockJobsHandler(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkMockJobsListWithSearch benchmarks jobs list with search filtering.
func BenchmarkMockJobsListWithSearch(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/jobs?search=web", nil)
			w := httptest.NewRecorder()
			mockJobsHandler(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkMockAllocations benchmarks concurrent requests to allocations list.
func BenchmarkMockAllocations(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/allocations", nil)
			w := httptest.NewRecorder()
			mockAllocationsHandler(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkMockStaticCSS benchmarks concurrent static CSS requests.
func BenchmarkMockStaticCSS(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/static/css/output.css", nil)
			w := httptest.NewRecorder()
			mockStaticHandler(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkMockStaticJS benchmarks concurrent static JS requests.
func BenchmarkMockStaticJS(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/static/js/htmx.min.js", nil)
			w := httptest.NewRecorder()
			mockStaticHandler(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkMockRouter benchmarks routing overhead.
func BenchmarkMockRouter(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ui/jobs", mockJobsHandler)
	mux.HandleFunc("/ui/allocations", mockAllocationsHandler)
	mux.HandleFunc("/ui/static/", mockStaticHandler)

	routes := []string{
		"/ui/jobs",
		"/ui/allocations",
		"/ui/static/css/output.css",
		"/ui/static/js/htmx.min.js",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			route := routes[i%len(routes)]
			req := httptest.NewRequest(http.MethodGet, route, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			i++
		}
	})
}

// BenchmarkMockMixedWorkload simulates realistic mixed traffic.
func BenchmarkMockMixedWorkload(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ui/jobs", mockJobsHandler)
	mux.HandleFunc("/ui/allocations", mockAllocationsHandler)
	mux.HandleFunc("/ui/static/", mockStaticHandler)

	requests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/ui/jobs"},
		{http.MethodGet, "/ui/jobs?search=web"},
		{http.MethodGet, "/ui/allocations"},
		{http.MethodGet, "/ui/allocations?search=running"},
		{http.MethodGet, "/ui/static/css/output.css"},
		{http.MethodGet, "/ui/static/js/htmx.min.js"},
		{http.MethodGet, "/ui/static/css/output.css"},
		{http.MethodGet, "/ui/jobs"},
		{http.MethodGet, "/ui/allocations"},
		{http.MethodGet, "/ui/static/js/htmx.min.js"},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := requests[i%len(requests)]
			httpReq := httptest.NewRequest(req.method, req.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httpReq)
			i++
		}
	})
}

// BenchmarkHTTPMiddleware benchmarks middleware overhead.
func BenchmarkHTTPMiddleware(b *testing.B) {
	// Simple middleware that adds headers
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate HTMX middleware
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Request", "true")
			}
			next.ServeHTTP(w, r)
		})
	}

	handler := middleware(http.HandlerFunc(mockJobsHandler))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/jobs", nil)
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

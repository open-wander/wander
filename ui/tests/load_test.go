package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/ui/handlers"
)

// BenchmarkJobsList benchmarks concurrent requests to the jobs list endpoint.
// This simulates multiple users loading the jobs list page simultaneously.
func BenchmarkJobsList(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/jobs", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkJobsListWithSearch benchmarks concurrent requests with search filtering.
func BenchmarkJobsListWithSearch(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/jobs?search=web", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkJobsListHTMX benchmarks HTMX partial requests to jobs list.
func BenchmarkJobsListHTMX(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

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

// BenchmarkJobsDetail benchmarks concurrent requests to job detail pages.
func BenchmarkJobsDetail(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/jobs/web-frontend", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkAllocations benchmarks concurrent requests to the allocations list.
func BenchmarkAllocations(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/allocations", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkAllocationsWithSearch benchmarks concurrent requests with search filtering.
func BenchmarkAllocationsWithSearch(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/allocations?search=running", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkAllocationsDetail benchmarks concurrent requests to allocation detail pages.
func BenchmarkAllocationsDetail(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/allocations/abc123", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkStaticCSS benchmarks concurrent requests to static CSS files.
func BenchmarkStaticCSS(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/static/css/output.css", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkStaticJS benchmarks concurrent requests to static JavaScript files.
func BenchmarkStaticJS(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/static/js/htmx.min.js", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkHome benchmarks concurrent requests to the home/dashboard page.
func BenchmarkHome(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkClients benchmarks concurrent requests to the clients list.
func BenchmarkClients(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/clients", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkServers benchmarks concurrent requests to the servers list.
func BenchmarkServers(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkMixedWorkload simulates a realistic mixed workload with different pages.
func BenchmarkMixedWorkload(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	// Different request patterns
	requests := []struct {
		method string
		path   string
		htmx   bool
	}{
		{http.MethodGet, "/ui/", false},
		{http.MethodGet, "/ui/jobs", false},
		{http.MethodGet, "/ui/jobs", true},
		{http.MethodGet, "/ui/jobs/web-frontend", false},
		{http.MethodGet, "/ui/allocations", false},
		{http.MethodGet, "/ui/allocations?search=running", true},
		{http.MethodGet, "/ui/clients", false},
		{http.MethodGet, "/ui/servers", false},
		{http.MethodGet, "/ui/static/css/output.css", false},
		{http.MethodGet, "/ui/static/js/htmx.min.js", false},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := requests[i%len(requests)]
			httpReq := httptest.NewRequest(req.method, req.path, nil)
			if req.htmx {
				httpReq.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httpReq)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d for %s", w.Code, req.path)
			}
			i++
		}
	})
}

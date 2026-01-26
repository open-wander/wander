package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/ui-htmx/handlers"
)

// These benchmarks test the handler layer only, avoiding template compilation issues.
// They measure the overhead of routing, middleware, and handler logic.

// BenchmarkJobsListHandler benchmarks concurrent requests to the jobs list handler.
func BenchmarkJobsListHandler(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	jobsHandler := handlers.NewJobsHandler(h)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/jobs", nil)
			w := httptest.NewRecorder()

			// Call List method directly to bypass template rendering
			// This measures handler logic overhead
			jobsHandler.ServeHTTP(w, req)
		}
	})
}

// BenchmarkAllocationsListHandler benchmarks concurrent requests to allocations handler.
func BenchmarkAllocationsListHandler(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	allocsHandler := handlers.NewAllocationsHandler(h)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/allocations", nil)
			w := httptest.NewRecorder()
			allocsHandler.ServeHTTP(w, req)
		}
	})
}

// BenchmarkStaticHandler benchmarks concurrent static file serving.
func BenchmarkStaticHandler(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	staticHandler := handlers.NewStaticHandler(h)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/css/output.css", nil)
			w := httptest.NewRecorder()
			staticHandler.ServeHTTP(w, req)
		}
	})
}

// BenchmarkRouterDispatch benchmarks the router's dispatching overhead.
func BenchmarkRouterDispatch(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	// Test different routes
	routes := []string{
		"/ui/",
		"/ui/jobs",
		"/ui/allocations",
		"/ui/clients",
		"/ui/servers",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			route := routes[i%len(routes)]
			req := httptest.NewRequest(http.MethodGet, route, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			i++
		}
	})
}

// BenchmarkHTMXMiddleware benchmarks HTMX header processing overhead.
func BenchmarkHTMXMiddleware(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	handler := router.Handler()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/ui/jobs", nil)
			req.Header.Set("HX-Request", "true")
			req.Header.Set("HX-Target", "main-content")
			req.Header.Set("HX-Current-URL", "/ui/")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}
	})
}

// BenchmarkJobsHandlerCreation benchmarks handler instantiation overhead.
func BenchmarkJobsHandlerCreation(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handlers.NewJobsHandler(h)
	}
}

// BenchmarkRouterCreation benchmarks router instantiation overhead.
func BenchmarkRouterCreation(b *testing.B) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handlers.NewRouter(h)
	}
}

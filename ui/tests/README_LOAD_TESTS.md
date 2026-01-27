# Load Testing Guide

## Overview

This directory contains load tests for the UI-HTMX application. The tests measure concurrent request handling, memory usage, and throughput under various workloads.

## Quick Start

### Run All Load Tests
```bash
# From ui-htmx directory
./scripts/load-test.sh
```

### Run Specific Benchmarks
```bash
# Run jobs list benchmark
go test -bench=BenchmarkMockJobsList -benchmem ./tests/load_http_test.go

# Run all mock benchmarks with 4 CPU cores
go test -bench=BenchmarkMock -benchmem -cpu=4 ./tests/load_http_test.go

# Run with longer duration for more accurate results
go test -bench=BenchmarkMock -benchmem -benchtime=10s ./tests/load_http_test.go
```

## Test Files

### `load_http_test.go`
Mock HTTP handler benchmarks that test routing and handler logic without template dependencies.

**Benchmarks:**
- `BenchmarkMockJobsList` - Jobs list page
- `BenchmarkMockJobsListWithSearch` - Jobs list with filtering
- `BenchmarkMockAllocations` - Allocations list page
- `BenchmarkMockStaticCSS` - Static CSS serving
- `BenchmarkMockStaticJS` - Static JS serving
- `BenchmarkMockRouter` - HTTP routing overhead
- `BenchmarkMockMixedWorkload` - Realistic mixed traffic
- `BenchmarkHTTPMiddleware` - Middleware overhead

### `load_test.go` (TODO)
Real handler benchmarks with template rendering. Currently blocked by template compilation issues.

**Pending Benchmarks:**
- `BenchmarkJobsList` - Real jobs handler with templates
- `BenchmarkJobsDetail` - Job detail page
- `BenchmarkAllocations` - Real allocations handler
- `BenchmarkStaticFiles` - Embedded static files
- `BenchmarkMixedWorkload` - Full stack test

### `load_test_standalone.go` (TODO)
Standalone handler benchmarks for testing specific components.

## Understanding Results

### Benchmark Output Format
```
BenchmarkMockJobsList-4    2089929    1731 ns/op    10338 B/op    74 allocs/op
                      |          |           |            |              |
                      |          |           |            |              +- Allocations per operation
                      |          |           |            +- Bytes allocated per operation
                      |          |           +- Nanoseconds per operation (latency)
                      |          +- Number of iterations
                      +- CPU cores used
```

### Key Metrics

**Throughput (req/sec)**:
- Calculate from ns/op: `1,000,000,000 / ns_op = req/sec`
- Higher is better
- Target: >10,000 req/sec for dynamic pages

**Latency (ns/op)**:
- Time to handle one request
- Lower is better
- Target: <100,000 ns (0.1ms) for handler logic

**Memory (B/op)**:
- Bytes allocated per request
- Lower is better
- Target: <50KB per request

**Allocations (allocs/op)**:
- Number of memory allocations
- Lower is better (reduces GC pressure)
- Target: <200 allocations per request

## CPU Core Scaling

The tests automatically run with 1, 2, 4, and 8 CPU cores to measure scalability:

```bash
# Manual CPU scaling test
go test -bench=BenchmarkMockJobsList -cpu=1,2,4,8 ./tests/load_http_test.go
```

**Expected Scaling:**
- 1→2 cores: ~1.6-1.8x throughput
- 2→4 cores: ~1.3-1.5x throughput
- 4→8 cores: ~0.9-1.1x throughput (may regress)

## Profiling

### CPU Profiling
```bash
# Generate CPU profile
go test -bench=BenchmarkMockMixedWorkload -cpuprofile=cpu.prof ./tests/load_http_test.go

# Analyze profile
go tool pprof cpu.prof
> top10        # Show top 10 CPU consumers
> web          # Open in browser (requires graphviz)
```

### Memory Profiling
```bash
# Generate memory profile
go test -bench=BenchmarkMockMixedWorkload -memprofile=mem.prof ./tests/load_http_test.go

# Analyze profile
go tool pprof mem.prof
> top10        # Show top 10 memory allocators
> list functionName  # Show source for specific function
```

### Trace Analysis
```bash
# Generate execution trace
go test -bench=BenchmarkMockMixedWorkload -trace=trace.out ./tests/load_http_test.go

# View trace
go tool trace trace.out
```

## HTTP Load Testing

### Using hey (recommended)
```bash
# Install hey
go install github.com/rakyll/hey@latest

# Test jobs endpoint
hey -n 10000 -c 50 http://localhost:3000/ui/jobs

# Test with rate limiting
hey -n 10000 -c 50 -q 100 http://localhost:3000/ui/jobs

# Test allocations with high concurrency
hey -n 20000 -c 200 http://localhost:3000/ui/allocations
```

### Using wrk
```bash
# Install wrk (macOS)
brew install wrk

# 30-second test with 100 connections
wrk -t4 -c100 -d30s http://localhost:3000/ui/jobs

# 1-minute stress test
wrk -t8 -c200 -d60s http://localhost:3000/ui/allocations
```

## Performance Targets

### Current Results (Mock Handlers, 4 cores)
- Jobs List: ~574,000 req/sec @ 1.7µs latency
- Allocations: ~393,000 req/sec @ 2.5µs latency
- Static Files: ~1,100,000 req/sec @ 0.9µs latency
- Mixed Workload: ~600,000 req/sec @ 1.7µs latency

### Production Targets
- Dynamic Pages: >10,000 req/sec
- Static Files: >50,000 req/sec
- P50 Latency: <50ms (including network)
- P95 Latency: <100ms
- P99 Latency: <200ms
- Concurrent Users: 1,000+

## Troubleshooting

### Template Compilation Errors
If you see errors like:
```
templates/partials/jobspartials/detail_templ.go:77:6: too many arguments in call to components.Badge
```

**Solution**: Fix the template component signatures first, then re-run benchmarks with real handlers.

### Out of Memory
If tests crash with OOM:
```bash
# Reduce benchmark time
go test -bench=. -benchtime=1s ./tests/...

# Test one benchmark at a time
go test -bench=BenchmarkMockJobsList ./tests/load_http_test.go
```

### Inconsistent Results
Benchmark results can vary due to:
- Background processes (close other apps)
- Thermal throttling (let CPU cool between runs)
- GC timing (run multiple times and average)

**Solution**:
```bash
# Run multiple times and use benchstat
go test -bench=. -count=5 ./tests/... > results.txt
benchstat results.txt
```

## Next Steps

1. **Fix Template Compilation**: Resolve component signature issues
2. **Run Real Handler Tests**: Test with actual templates
3. **HTTP Load Testing**: Test against running server
4. **Optimize Hotspots**: Profile and improve slow paths
5. **Production Testing**: Test with real Nomad cluster

## Resources

- [Go Benchmark Guide](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)
- [Profiling Go Programs](https://go.dev/blog/pprof)
- [hey Documentation](https://github.com/rakyll/hey)
- [wrk Documentation](https://github.com/wg/wrk)

---

**Last Updated**: 2026-01-26
**Test Environment**: Apple M3 Max, Go 1.25.5

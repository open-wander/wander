#!/bin/bash
set -e

# Load Test Runner Script
# This script runs Go benchmarks with different concurrency levels
# and optionally uses external tools like hey or wrk for additional testing.

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}UI-HTMX Load Test Suite${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Change to ui-htmx directory
cd "$(dirname "$0")/.."

# Check if Go is available
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed or not in PATH${NC}"
    exit 1
fi

# Create results directory
mkdir -p test-results

# Timestamp for results
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
RESULTS_FILE="test-results/load_test_${TIMESTAMP}.txt"

echo -e "${GREEN}Starting Go benchmark tests...${NC}"
echo ""

# Function to run benchmarks with different CPU counts
run_benchmark() {
    local name=$1
    local pattern=$2
    local cpus=$3

    echo -e "${YELLOW}Running: $name (CPUs: $cpus)${NC}"
    go test -bench="$pattern" -benchmem -benchtime=5s -cpu="$cpus" ./tests/... | tee -a "$RESULTS_FILE"
    echo ""
}

# Note: Currently using mock tests due to template compilation issues
# Once templates are fixed, uncomment the real handler benchmarks below

echo -e "${BLUE}--- Running Mock HTTP Benchmarks ---${NC}"
echo -e "${YELLOW}Note: Using mock handlers (template compilation issues)${NC}"
echo ""

# Run comprehensive benchmark for all mock tests with different CPU counts
echo -e "${BLUE}--- Scaling Test (1, 2, 4, 8 CPUs) ---${NC}"
echo -e "${YELLOW}Running: All mock benchmarks across CPU counts${NC}"
go test -bench=BenchmarkMock -benchmem -benchtime=3s -cpu=1,2,4,8 ./tests/load_http_test.go | tee -a "$RESULTS_FILE"

# TODO: Uncomment these when template compilation issues are fixed
# echo -e "${BLUE}--- Real Handler Benchmarks (4 CPUs) ---${NC}"
# run_benchmark "Jobs List" "BenchmarkJobsList$" 4
# run_benchmark "Jobs Detail" "BenchmarkJobsDetail" 4
# run_benchmark "Allocations" "BenchmarkAllocations$" 4
# run_benchmark "Static Files" "BenchmarkStatic" 4
# run_benchmark "Mixed Workload" "BenchmarkMixedWorkload" 4

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Benchmark Results Summary${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Results saved to: $RESULTS_FILE"
echo ""

# Extract key metrics from results
echo -e "${BLUE}Key Performance Metrics (4 CPU cores - optimal):${NC}"
echo ""
echo "Endpoint                    | ns/op    | B/op      | allocs/op"
echo "----------------------------|----------|-----------|----------"
grep "BenchmarkMock" "$RESULTS_FILE" | grep "\-4" | awk '{printf "%-27s | %-8s | %-9s | %s\n", $1, $3, $5, $7}'

echo ""
echo -e "${YELLOW}Note: Lower ns/op is better (faster), lower B/op is better (less memory)${NC}"
echo ""

# Calculate requests per second for 4-core runs
echo -e "${BLUE}Estimated Throughput (4 CPU cores):${NC}"
echo ""
grep "BenchmarkMock" "$RESULTS_FILE" | grep "\-4" | awk '{
    ops = $2
    ns_per_op = $3
    req_per_sec = 1000000000 / ns_per_op
    printf "%-30s: %'"'"'10.0f req/sec\n", $1, req_per_sec
}'

echo ""

# Check if hey is available for HTTP load testing
if command -v hey &> /dev/null; then
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}HTTP Load Testing with 'hey'${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    echo -e "${YELLOW}Note: This requires the server to be running${NC}"
    echo -e "${YELLOW}Start the server with: make dev${NC}"
    echo -e "${YELLOW}Then run this script again or run hey manually:${NC}"
    echo -e "${YELLOW}  hey -n 10000 -c 50 http://localhost:3000/ui/jobs${NC}"
    echo ""
else
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}Optional: Install 'hey' for HTTP testing${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""
    echo "You can install 'hey' for additional HTTP load testing:"
    echo "  go install github.com/rakyll/hey@latest"
    echo ""
    echo "Example usage:"
    echo "  hey -n 10000 -c 50 -q 10 http://localhost:3000/ui/jobs"
    echo "  hey -n 5000 -c 100 http://localhost:3000/ui/allocations"
    echo ""
fi

# Check if wrk is available
if command -v wrk &> /dev/null; then
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}HTTP Load Testing with 'wrk'${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    echo -e "${YELLOW}Note: This requires the server to be running${NC}"
    echo -e "${YELLOW}Start the server with: make dev${NC}"
    echo -e "${YELLOW}Then run this script again or run wrk manually:${NC}"
    echo -e "${YELLOW}  wrk -t4 -c100 -d30s http://localhost:3000/ui/jobs${NC}"
    echo ""
else
    echo -e "${YELLOW}Optional: Install 'wrk' for HTTP testing${NC}"
    echo ""
    echo "You can install 'wrk' for additional HTTP load testing:"
    echo "  brew install wrk  # macOS"
    echo "  apt install wrk   # Ubuntu/Debian"
    echo ""
    echo "Example usage:"
    echo "  wrk -t4 -c100 -d30s http://localhost:3000/ui/jobs"
    echo "  wrk -t8 -c200 -d60s http://localhost:3000/ui/allocations"
    echo ""
fi

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Load Test Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Next steps:"
echo "  1. Review results in: $RESULTS_FILE"
echo "  2. Update LOAD_TEST_RESULTS.md with findings"
echo "  3. Compare against performance targets"
echo "  4. Identify and address any bottlenecks"
echo ""

#!/usr/bin/env bash

# Performance measurement script for UI-HTMX
# Measures TTFB (Time To First Byte) for key pages and assets
# Usage: ./measure-performance.sh [base_url]
# Example: ./measure-performance.sh http://localhost:3000

set -euo pipefail

# Default base URL if not provided
BASE_URL="${1:-http://localhost:3000}"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Performance targets (in milliseconds)
TARGET_TTFB=100
TARGET_FCP=500
TARGET_TTI=1000

# Array of pages to test
declare -a PAGES=(
    "/ui/"
    "/ui/jobs"
    "/ui/allocations"
    "/ui/clients"
    "/ui/servers"
    "/ui/static/css/output.css"
    "/ui/static/js/htmx.min.js"
    "/ui/static/js/alpine.min.js"
)

# Function to check if server is running
check_server() {
    local url="$1"
    echo "Checking if server is running at $url..."
    if ! curl -s -f -o /dev/null --connect-timeout 5 "$url/ui/" 2>/dev/null; then
        echo -e "${RED}Error: Server not responding at $url${NC}"
        echo ""
        echo "To start the dev server, run:"
        echo "  cd ui-htmx && make dev"
        echo ""
        echo "Or in separate terminals:"
        echo "  Terminal 1: cd ui-htmx && air"
        echo "  Terminal 2: cd ui-htmx && make watch-css"
        echo ""
        exit 1
    fi
    echo -e "${GREEN}Server is running${NC}"
    echo ""
}

# Function to measure TTFB using curl
measure_ttfb() {
    local url="$1"
    local runs="${2:-3}"
    local total=0
    local min=999999
    local max=0

    for ((i=1; i<=runs; i++)); do
        # Use curl to measure time_starttransfer (TTFB)
        local ttfb
        ttfb=$(curl -o /dev/null -s -w '%{time_starttransfer}' "$url" 2>/dev/null || echo "0")

        # Convert to milliseconds (curl outputs seconds with decimals)
        local ttfb_ms
        ttfb_ms=$(echo "$ttfb * 1000" | bc -l | awk '{printf "%.0f", $0}')

        total=$((total + ttfb_ms))

        if [ "$ttfb_ms" -lt "$min" ]; then
            min=$ttfb_ms
        fi

        if [ "$ttfb_ms" -gt "$max" ]; then
            max=$ttfb_ms
        fi

        # Small delay between runs
        sleep 0.1
    done

    # Calculate average
    local avg=$((total / runs))

    echo "$avg|$min|$max"
}

# Function to get content size
get_size() {
    local url="$1"
    local size
    size=$(curl -s -o /dev/null -w '%{size_download}' "$url" 2>/dev/null || echo "0")

    # Convert to KB
    echo "scale=1; $size / 1024" | bc -l | awk '{printf "%.1f", $0}'
}

# Function to check status
check_status() {
    local url="$1"
    curl -s -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || echo "000"
}

# Function to format TTFB with color
format_ttfb() {
    local ttfb="$1"
    if [ "$ttfb" -le "$TARGET_TTFB" ]; then
        echo -e "${GREEN}${ttfb}ms${NC}"
    elif [ "$ttfb" -le "$((TARGET_TTFB * 2))" ]; then
        echo -e "${YELLOW}${ttfb}ms${NC}"
    else
        echo -e "${RED}${ttfb}ms${NC}"
    fi
}

# Main execution
echo "=========================================="
echo "UI-HTMX Performance Measurement"
echo "=========================================="
echo ""
echo "Base URL: $BASE_URL"
echo "Target TTFB: ${TARGET_TTFB}ms"
echo "Runs per page: 3"
echo ""

# Check if server is running
check_server "$BASE_URL"

# Results file
RESULTS_FILE="./test-results/performance-results-$(date +%Y%m%d-%H%M%S).txt"
mkdir -p ./test-results

echo "=========================================="
echo "Measuring performance..."
echo "=========================================="
echo ""

# Markdown table header
echo "| Page | Avg TTFB | Min TTFB | Max TTFB | Size | Status |" > "$RESULTS_FILE"
echo "|------|----------|----------|----------|------|--------|" >> "$RESULTS_FILE"

# Process each page
for page in "${PAGES[@]}"; do
    url="${BASE_URL}${page}"

    echo -n "Testing: $page ... "

    # Measure TTFB
    result=$(measure_ttfb "$url" 3)
    IFS='|' read -r avg min max <<< "$result"

    # Get size
    size=$(get_size "$url")

    # Get status
    status=$(check_status "$url")

    # Format output
    echo "$(format_ttfb "$avg") (min: ${min}ms, max: ${max}ms) | ${size}KB | ${status}"

    # Write to results file
    echo "| $page | ${avg}ms | ${min}ms | ${max}ms | ${size}KB | $status |" >> "$RESULTS_FILE"
done

echo ""
echo "=========================================="
echo "Results saved to: $RESULTS_FILE"
echo "=========================================="
echo ""

# Display results
cat "$RESULTS_FILE"

echo ""
echo "=========================================="
echo "Summary"
echo "=========================================="
echo ""
echo "TTFB Target: ${TARGET_TTFB}ms"
echo -e "${GREEN}✓ Pass${NC}  - TTFB <= ${TARGET_TTFB}ms"
echo -e "${YELLOW}⚠ Warn${NC}  - TTFB <= $((TARGET_TTFB * 2))ms"
echo -e "${RED}✗ Fail${NC}  - TTFB > $((TARGET_TTFB * 2))ms"
echo ""
echo "Note: TTFB is measured from curl. For FCP and TTI metrics,"
echo "use browser-based tools like Lighthouse or Chrome DevTools."
echo ""
echo "To update PERFORMANCE_METRICS.md, copy the results table above."
echo ""

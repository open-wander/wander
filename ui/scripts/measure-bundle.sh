#!/bin/bash
set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Target size in bytes (100KB gzipped)
TARGET_SIZE=$((100 * 1024))

# Directory containing JS files
JS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/static/js"

echo -e "${BLUE}=== JavaScript Bundle Size Analysis ===${NC}\n"

# Check if directory exists
if [ ! -d "$JS_DIR" ]; then
    echo -e "${RED}Error: JS directory not found at $JS_DIR${NC}"
    exit 1
fi

# Arrays to store results
declare -a files
declare -a raw_sizes
declare -a gzip_sizes

total_raw=0
total_gzip=0

# Process each JS file
while IFS= read -r -d '' file; do
    filename=$(basename "$file")

    # Get raw size
    raw_size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null)

    # Get gzipped size
    gzip_size=$(gzip -c "$file" | wc -c | tr -d ' ')

    files+=("$filename")
    raw_sizes+=("$raw_size")
    gzip_sizes+=("$gzip_size")

    total_raw=$((total_raw + raw_size))
    total_gzip=$((total_gzip + gzip_size))
done < <(find "$JS_DIR" -maxdepth 1 -name "*.js" -type f -print0 | sort -z)

# Print table header
printf "%-40s %12s %12s %10s\n" "File" "Raw Size" "Gzipped" "Ratio"
printf "%-40s %12s %12s %10s\n" "----" "--------" "--------" "-----"

# Print each file
for i in "${!files[@]}"; do
    file="${files[$i]}"
    raw="${raw_sizes[$i]}"
    gzip="${gzip_sizes[$i]}"

    # Calculate compression ratio
    ratio=$(echo "$gzip $raw" | awk '{printf "%.1f%%", ($1 / $2) * 100}')

    # Format sizes (KB with 2 decimals)
    raw_kb=$(echo "$raw" | awk '{printf "%.2f KB", $1 / 1024}')
    gzip_kb=$(echo "$gzip" | awk '{printf "%.2f KB", $1 / 1024}')

    printf "%-40s %12s %12s %10s\n" "$file" "$raw_kb" "$gzip_kb" "$ratio"
done

echo ""
printf "%-40s %12s %12s\n" "----------------------------------------" "------------" "------------"

# Format totals
total_raw_kb=$(echo "$total_raw" | awk '{printf "%.2f KB", $1 / 1024}')
total_gzip_kb=$(echo "$total_gzip" | awk '{printf "%.2f KB", $1 / 1024}')
total_gzip_ratio=$(echo "$total_gzip $total_raw" | awk '{printf "%.1f%%", ($1 / $2) * 100}')

printf "%-40s %12s %12s %10s\n" "TOTAL" "$total_raw_kb" "$total_gzip_kb" "$total_gzip_ratio"

echo ""
echo -e "${BLUE}=== Summary ===${NC}"
echo "Total Bundle Size (Raw):     $total_raw_kb"
echo "Total Bundle Size (Gzipped): $total_gzip_kb"
echo "Target Size:                 $(echo "$TARGET_SIZE" | awk '{printf "%.2f KB", $1 / 1024}')"

# Calculate difference from target
diff=$((total_gzip - TARGET_SIZE))
diff_abs=${diff#-}  # Absolute value
diff_kb=$(echo "$diff_abs" | awk '{printf "%.2f KB", $1 / 1024}')
diff_percent=$(echo "$diff_abs $TARGET_SIZE" | awk '{printf "%.1f%%", ($1 / $2) * 100}')

echo ""
if [ $total_gzip -le $TARGET_SIZE ]; then
    under=$((TARGET_SIZE - total_gzip))
    under_kb=$(echo "$under" | awk '{printf "%.2f KB", $1 / 1024}')
    echo -e "${GREEN}✓ PASSED${NC} - Bundle is ${under_kb} (${diff_percent}) under budget"
    exit 0
else
    echo -e "${RED}✗ FAILED${NC} - Bundle is ${diff_kb} (${diff_percent}) over budget"
    echo ""
    echo -e "${YELLOW}Recommendations:${NC}"
    echo "  1. Review large vendor files (htmx, alpine, xterm, codemirror)"
    echo "  2. Consider lazy-loading non-critical JavaScript"
    echo "  3. Check if all chart components are necessary on every page"
    echo "  4. Minify custom JavaScript files if not already minified"
    echo "  5. Consider code splitting for terminal/editor features"
    exit 1
fi

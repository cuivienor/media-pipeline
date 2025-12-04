#!/bin/bash
# Contract test runner - validates bash scripts produce scanner-compatible state files
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MOCK_DIR="$PROJECT_ROOT/testdata/mocks"
VALIDATOR="$PROJECT_ROOT/bin/validate-state"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track results
PASSED=0
FAILED=0

# Build validator if needed
if [ ! -f "$VALIDATOR" ]; then
    echo "Building state validator..."
    (cd "$PROJECT_ROOT" && go build -o bin/validate-state ./test/validate-state)
fi

# Create temp staging area
STAGING_DIR=$(mktemp -d)
trap "rm -rf $STAGING_DIR" EXIT

echo "================================"
echo "Contract Tests"
echo "================================"
echo "Staging: $STAGING_DIR"
echo ""

# Test rip-disc.sh
test_rip_script() {
    local test_name="rip-disc.sh movie"
    echo -n "Testing $test_name... "

    # Set up environment
    export PATH="$MOCK_DIR:$PATH"
    export MEDIA_BASE="$STAGING_DIR"
    mkdir -p "$STAGING_DIR/staging"

    # Run script (suppress interactive prompts)
    cd "$PROJECT_ROOT"
    echo "n" | timeout 10 ./cmd/rip/rip-disc.sh -t movie -n "Test Movie" 2>/dev/null || true

    # Find and validate state directory
    local state_dir=$(find "$STAGING_DIR" -name ".rip" -type d 2>/dev/null | head -1)
    if [ -z "$state_dir" ]; then
        echo -e "${RED}FAIL${NC} - no .rip directory created"
        FAILED=$((FAILED + 1))
        return
    fi

    if $VALIDATOR "$state_dir" >/dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}FAIL${NC}"
        $VALIDATOR "$state_dir"
        FAILED=$((FAILED + 1))
    fi
}

# Run tests
test_rip_script

# Summary
echo ""
echo "================================"
echo "Results: $PASSED passed, $FAILED failed"
echo "================================"

if [ $FAILED -gt 0 ]; then
    exit 1
fi

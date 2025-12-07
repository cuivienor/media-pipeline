#!/bin/bash
set -e

# Get script directory for finding binaries
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Generate unique test directory or use provided MEDIA_BASE
if [ -n "$MEDIA_BASE" ]; then
    TEST_MEDIA_BASE="$MEDIA_BASE"
else
    TIMESTAMP=$(date +%s)
    TEST_MEDIA_BASE="/tmp/media-test-$TIMESTAMP"
fi

echo "Setting up test environment at $TEST_MEDIA_BASE"
echo ""

# Create directory structure
mkdir -p "$TEST_MEDIA_BASE/pipeline/logs/jobs"
mkdir -p "$TEST_MEDIA_BASE/staging/1-ripped/movies"
mkdir -p "$TEST_MEDIA_BASE/staging/1-ripped/tv"
mkdir -p "$TEST_MEDIA_BASE/staging/2-remuxed/movies"
mkdir -p "$TEST_MEDIA_BASE/staging/2-remuxed/tv"
mkdir -p "$TEST_MEDIA_BASE/staging/3-transcoded/movies"
mkdir -p "$TEST_MEDIA_BASE/staging/3-transcoded/tv"
mkdir -p "$TEST_MEDIA_BASE/library/movies"
mkdir -p "$TEST_MEDIA_BASE/library/tv"
mkdir -p "$TEST_MEDIA_BASE/bin"

# Create config file
cat > "$TEST_MEDIA_BASE/pipeline/config.yaml" << EOF
staging_base: $TEST_MEDIA_BASE/staging
library_base: $TEST_MEDIA_BASE/library

dispatch:
  rip: ""
  remux: ""
  transcode: ""
  publish: ""
EOF

echo "Created directory structure"
echo "Created config at $TEST_MEDIA_BASE/pipeline/config.yaml"

# Copy binaries if they exist
BINARIES="media-pipeline ripper remux transcode publish mock-makemkv"
COPIED_BINS=""
for bin in $BINARIES; do
    if [ -f "$PROJECT_ROOT/bin/$bin" ]; then
        cp "$PROJECT_ROOT/bin/$bin" "$TEST_MEDIA_BASE/bin/"
        COPIED_BINS="$COPIED_BINS $bin"
    fi
done

if [ -n "$COPIED_BINS" ]; then
    echo "Copied binaries:$COPIED_BINS"
fi

# Initialize database
DB_PATH="$TEST_MEDIA_BASE/pipeline/pipeline.db"
if [ -f "$TEST_MEDIA_BASE/bin/media-pipeline" ]; then
    # Use the TUI binary to initialize DB (it creates schema on startup)
    # For now just touch the file - the app will initialize it
    touch "$DB_PATH"
    echo "Created database at $DB_PATH"
fi

echo ""
echo "=========================================="
echo "Test environment ready!"
echo "=========================================="
echo ""
echo "To use:"
echo "  export MEDIA_BASE=$TEST_MEDIA_BASE"
echo "  export MAKEMKVCON_PATH=$TEST_MEDIA_BASE/bin/mock-makemkv"
echo "  $TEST_MEDIA_BASE/bin/media-pipeline"
echo ""
echo "Or run directly:"
echo "  MEDIA_BASE=$TEST_MEDIA_BASE MAKEMKVCON_PATH=$TEST_MEDIA_BASE/bin/mock-makemkv $TEST_MEDIA_BASE/bin/media-pipeline"
echo ""
echo "To clean up:"
echo "  rm -rf $TEST_MEDIA_BASE"

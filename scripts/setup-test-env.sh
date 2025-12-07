#!/bin/bash
set -e

# Default test MEDIA_BASE
TEST_MEDIA_BASE="${MEDIA_BASE:-/tmp/test-media}"

echo "Setting up test environment at $TEST_MEDIA_BASE"

# Create directory structure
mkdir -p "$TEST_MEDIA_BASE/pipeline"
mkdir -p "$TEST_MEDIA_BASE/staging/1-ripped/movies"
mkdir -p "$TEST_MEDIA_BASE/staging/1-ripped/tv"
mkdir -p "$TEST_MEDIA_BASE/staging/2-remuxed/movies"
mkdir -p "$TEST_MEDIA_BASE/staging/2-remuxed/tv"
mkdir -p "$TEST_MEDIA_BASE/staging/3-transcoded/movies"
mkdir -p "$TEST_MEDIA_BASE/staging/3-transcoded/tv"
mkdir -p "$TEST_MEDIA_BASE/library/movies"
mkdir -p "$TEST_MEDIA_BASE/library/tv"

# Create config file
cat > "$TEST_MEDIA_BASE/pipeline/config.yaml" << EOF
staging_base: $TEST_MEDIA_BASE/staging
library_base: $TEST_MEDIA_BASE/library

dispatch:
  rip: ""        # all local for testing
  remux: ""
  transcode: ""
  publish: ""
EOF

echo "Config written to $TEST_MEDIA_BASE/pipeline/config.yaml"
echo ""
echo "To use this environment:"
echo "  export MEDIA_BASE=$TEST_MEDIA_BASE"
echo "  export MAKEMKVCON_PATH=\$(pwd)/bin/mock-makemkv"
echo "  ./bin/media-pipeline"

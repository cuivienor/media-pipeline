#!/bin/bash
# MakeMKV Disc Ripper Helper
# Usage: ./rip-disc.sh -t <type> -n <name> [-s <season>] [-d <disc>]
#
# Examples:
#   ./rip-disc.sh -t movie -n "The Matrix"
#   ./rip-disc.sh --type movie --name "The Matrix Reloaded"
#   ./rip-disc.sh -t show -n "Avatar The Last Airbender" -s 1 -d 1
#   ./rip-disc.sh --type show --name "Breaking Bad" --season 2 --disc 3
#
# State Management:
#   - Creates .rip/ directory with status, logs, and metadata
#   - Creates symlink in ~/active-jobs/ for global visibility
#   - Status: in_progress → completed or failed
#
# Monitoring:
#   ls ~/active-jobs/                    # See all active jobs
#   cat ~/active-jobs/*/status           # Check status
#   tail -f ~/active-jobs/*/rip.log      # Follow logs

set -e

# Default values
TYPE=""
NAME=""
SEASON=""
DISC=""

# Global state directory for active jobs
ACTIVE_JOBS_DIR="$HOME/active-jobs"

# Help function
show_help() {
    cat << EOF
Usage: $0 -t <type> -n <name> [-s <season>] [-d <disc>]

Options:
  -t, --type <type>       Media type: 'movie' or 'show' (required)
  -n, --name <name>       Title of the movie or show (required)
  -s, --season <number>   Season number (required for shows)
  -d, --disc <number>     Disc number (required for shows)
  -h, --help              Show this help message

Examples:
  $0 -t movie -n "The Matrix"
  $0 -t show -n "Avatar The Last Airbender" -s 1 -d 1
  $0 --type show --name "Breaking Bad" --season 2 --disc 3

State Management:
  Job state is tracked in OUTPUT_DIR/.rip/
  Active jobs are symlinked in ~/active-jobs/

Monitoring:
  ls ~/active-jobs/                    # See all active jobs
  cat ~/active-jobs/*/status           # Check status of all jobs
  tail -f ~/active-jobs/*/rip.log      # Follow logs of all jobs
EOF
    exit 0
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--type)
            TYPE="$2"
            shift 2
            ;;
        -n|--name)
            NAME="$2"
            shift 2
            ;;
        -s|--season)
            SEASON="$2"
            shift 2
            ;;
        -d|--disc)
            DISC="$2"
            shift 2
            ;;
        -h|--help)
            show_help
            ;;
        *)
            echo "Error: Unknown option $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Validate required arguments
if [ -z "$TYPE" ] || [ -z "$NAME" ]; then
    echo "Error: --type and --name are required"
    echo "Use -h or --help for usage information"
    exit 1
fi

# Validate type
if [[ ! "$TYPE" =~ ^(movie|show)$ ]]; then
    echo "Error: Type must be 'movie' or 'show'"
    exit 1
fi

# Require season and disc for shows
if [[ "$TYPE" == "show" ]]; then
    if [ -z "$SEASON" ] || [ -z "$DISC" ]; then
        echo "Error: --season and --disc are required for TV shows"
        echo "Example: $0 -t show -n \"$NAME\" -s 1 -d 1"
        exit 1
    fi

    # Validate season and disc are numbers
    if ! [[ "$SEASON" =~ ^[0-9]+$ ]]; then
        echo "Error: Season must be a number"
        exit 1
    fi

    if ! [[ "$DISC" =~ ^[0-9]+$ ]]; then
        echo "Error: Disc must be a number"
        exit 1
    fi
fi

# Create safe directory names
SAFE_NAME=$(echo "$NAME" | tr ' ' '_' | tr -cd '[:alnum:]_-')

# Standardized media path (consistent across all containers)
# Can be overridden via environment for testing
MEDIA_BASE="${MEDIA_BASE:-/mnt/media}"

# Verify mount exists
if [ ! -d "$MEDIA_BASE/staging" ]; then
    echo "Error: Media mount not found at $MEDIA_BASE/staging"
    echo "Expected mount: $MEDIA_BASE → /mnt/storage/media"
    exit 1
fi

case "$TYPE" in
    movie)
        OUTPUT_DIR="${MEDIA_BASE}/staging/1-ripped/movies/${SAFE_NAME}"
        DISPLAY_INFO="Movie: $NAME"
        JOB_NAME="rip_movie_${SAFE_NAME}"
        ;;
    show)
        # Format season with leading zero (S01, S02, etc.)
        SEASON_DIR=$(printf "S%02d" "$SEASON")
        DISC_DIR="Disc${DISC}"
        OUTPUT_DIR="${MEDIA_BASE}/staging/1-ripped/tv/${SAFE_NAME}/${SEASON_DIR}/${DISC_DIR}"
        DISPLAY_INFO="Show: $NAME | Season: $SEASON | Disc: $DISC"
        JOB_NAME="rip_show_${SAFE_NAME}_${SEASON_DIR}_Disc${DISC}"
        ;;
esac

# Check if output directory already exists
if [ -d "$OUTPUT_DIR" ]; then
    echo "Warning: Output directory already exists: $OUTPUT_DIR"
    read -p "Continue and potentially overwrite? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
fi

# Create output directory and state tracking
mkdir -p "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR/.rip"
mkdir -p "$ACTIVE_JOBS_DIR"

# Initialize state
STATE_DIR="$OUTPUT_DIR/.rip"
echo "in_progress" > "$STATE_DIR/status"
echo "$$" > "$STATE_DIR/pid"
date -Iseconds > "$STATE_DIR/started_at"

# Store job metadata
cat > "$STATE_DIR/metadata.json" << EOF
{
  "type": "$TYPE",
  "name": "$NAME",
  "safe_name": "$SAFE_NAME",
  "season": "$SEASON",
  "disc": "$DISC",
  "output_dir": "$OUTPUT_DIR",
  "started_at": "$(date -Iseconds)",
  "pid": $$
}
EOF

# Create symlink for global job tracking
ln -sf "$STATE_DIR" "$ACTIVE_JOBS_DIR/$JOB_NAME"

# Set up logging - redirect all output to log file AND stdout
LOG_FILE="$STATE_DIR/rip.log"
exec > >(tee -a "$LOG_FILE") 2>&1

# Cleanup function to handle exit (success or failure)
cleanup() {
    local exit_code=$?

    if [ $exit_code -eq 0 ]; then
        echo "completed" > "$STATE_DIR/status"
        date -Iseconds > "$STATE_DIR/completed_at"
        echo ""
        echo "=========================================="
        echo "✓ Rip completed successfully"
        echo "=========================================="
    else
        echo "failed" > "$STATE_DIR/status"
        echo "Exit code: $exit_code" > "$STATE_DIR/error"
        date -Iseconds > "$STATE_DIR/failed_at"
        echo ""
        echo "=========================================="
        echo "✗ Rip failed with exit code: $exit_code"
        echo "Check: $STATE_DIR/rip.log"
        echo "=========================================="
    fi

    # Remove PID file (process is done)
    rm -f "$STATE_DIR/pid"

    # Remove symlink from active jobs (job is no longer active)
    rm -f "$ACTIVE_JOBS_DIR/$JOB_NAME"
}

# Register cleanup to run on exit
trap cleanup EXIT

echo "=========================================="
echo "MakeMKV Disc Ripper"
echo "=========================================="
echo "$DISPLAY_INFO"
echo "Output: $OUTPUT_DIR"
echo "State: $STATE_DIR"
echo "Log: $LOG_FILE"
echo "PID: $$"
echo "=========================================="
echo ""

# Get disc info first
echo "Analyzing disc..."
makemkvcon info disc:0

echo ""
echo "Starting rip of ALL titles..."
echo "This will take 30-90 minutes depending on disc size"
echo ""

# Rip all titles
makemkvcon mkv disc:0 all "$OUTPUT_DIR"

echo ""
echo "=========================================="
echo "Rip complete!"
echo "Files saved to: $OUTPUT_DIR"
echo "=========================================="

# Create organization scaffolding for manual review
echo ""
echo "Creating organization directories..."

mkdir -p "$OUTPUT_DIR/_discarded"
mkdir -p "$OUTPUT_DIR/_extras/behind the scenes"
mkdir -p "$OUTPUT_DIR/_extras/deleted scenes"
mkdir -p "$OUTPUT_DIR/_extras/featurettes"
mkdir -p "$OUTPUT_DIR/_extras/interviews"
mkdir -p "$OUTPUT_DIR/_extras/scenes"
mkdir -p "$OUTPUT_DIR/_extras/shorts"
mkdir -p "$OUTPUT_DIR/_extras/trailers"
mkdir -p "$OUTPUT_DIR/_extras/other"

if [ "$TYPE" == "show" ]; then
    echo ""
    echo "Adding disc identifier to filenames..."

    cd "$OUTPUT_DIR"

    # Rename all files to include show name and disc ID
    # Pattern: title_t00.mkv → ShowName_S01_Disc1_t00.mkv
    for file in *.mkv; do
        if [ -f "$file" ]; then
            # Extract track number from filename
            if [[ "$file" =~ _t([0-9]+)\.mkv ]]; then
                track_num="${BASH_REMATCH[1]}"
                new_name="${SAFE_NAME}_${SEASON_DIR}_Disc${DISC}_t${track_num}.mkv"

                if [ "$file" != "$new_name" ]; then
                    mv "$file" "$new_name"
                    echo "  Renamed: $file → $new_name"
                fi
            fi
        fi
    done

    cd - > /dev/null

    echo ""
    echo "✓ Files renamed with disc identifier"

    mkdir -p "$OUTPUT_DIR/_episodes"

    # Create review notes template for TV shows
    cat > "$OUTPUT_DIR/_REVIEW.txt" << EOF
# Manual Review Notes
# Show: $NAME
# Season: $SEASON
# Disc: $DISC
# Ripped: $(date '+%Y-%m-%d %H:%M')

## Disc Info
- Blu-ray.com URL:
- Total titles ripped: $(ls -1 "$OUTPUT_DIR"/*.mkv 2>/dev/null | wc -l)

## Episode Mapping
# Rename files to: 01.mkv, 02.mkv, etc. (or 01_Episode_Name.mkv)
# Then move to _episodes/

## Extras Found
# Move to appropriate _extras/ subdirectory with descriptive names
# Example: Making_Of_Season_1.mkv → _extras/behind the scenes/

## Discarded
# Move duplicates/unwanted to _discarded/

## Notes

EOF

    echo "✓ Organization directories created"
    echo ""
    echo "Next steps:"
    echo "  1. Review each .mkv file (use mediainfo or play briefly)"
    echo "  2. Rename episodes: 01.mkv, 02.mkv, etc."
    echo "  3. Move episodes to _episodes/"
    echo "  4. Rename extras descriptively and move to _extras/{category}/"
    echo "  5. Move unwanted files to _discarded/"
    echo "  6. Update _REVIEW.txt with your notes"
else
    # Movie scaffolding
    mkdir -p "$OUTPUT_DIR/_main"

    # Create review notes template for movies
    cat > "$OUTPUT_DIR/_REVIEW.txt" << EOF
# Manual Review Notes
# Movie: $NAME
# Ripped: $(date '+%Y-%m-%d %H:%M')

## Disc Info
- Blu-ray.com URL:
- Total titles ripped: $(ls -1 "$OUTPUT_DIR"/*.mkv 2>/dev/null | wc -l)

## Main Feature
# Identify the main movie file and move to _main/
# Rename to: ${SAFE_NAME}.mkv

## Extras Found
# Move to appropriate _extras/ subdirectory with descriptive names
# Example: Making_Of.mkv → _extras/behind the scenes/

## Discarded
# Move duplicates/unwanted to _discarded/

## Notes

EOF

    echo "✓ Organization directories created"
    echo ""
    echo "Next steps:"
    echo "  1. Review each .mkv file (use mediainfo or play briefly)"
    echo "  2. Identify main feature and move to _main/"
    echo "  3. Rename extras descriptively and move to _extras/{category}/"
    echo "  4. Move unwanted files to _discarded/"
    echo "  5. Update _REVIEW.txt with your notes"
fi

echo ""
ls -lh "$OUTPUT_DIR"
echo ""

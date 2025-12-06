# Media Pipeline Test Container Requirements

## Purpose

Container for testing the media-pipeline project interactively, including the TUI, ripper CLI, and mock-makemkv for simulating disc rips without real hardware.

## Binary Installation

Download and extract the latest release:

```bash
VERSION=v0.1.0
curl -L "https://github.com/cuivienor/media-pipeline/releases/download/${VERSION}/media-pipeline-linux-amd64.tar.gz" | tar -xz -C /home/media/bin --strip-components=1
```

This installs:
- `media-pipeline` - TUI for monitoring pipeline status
- `ripper` - CLI for starting rip jobs
- `mock-makemkv` - Mock MakeMKV that generates synthetic MKV files for testing

## System Dependencies

| Package | Purpose |
|---------|---------|
| ffmpeg | Required by mock-makemkv to generate synthetic MKV files |
| mkvtoolnix | MKV manipulation (mkvmerge, mkvextract) for remuxing stage |
| jq | JSON parsing in bash scripts |
| bc | Arithmetic in bash scripts |

## Directory Structure

```
/mnt/media/
├── staging/
│   ├── 1-ripped/
│   │   ├── movies/
│   │   └── tv/
│   ├── 2-remuxed/
│   │   ├── movies/
│   │   └── tv/
│   └── 3-transcoded/
│       ├── movies/
│       └── tv/
└── library/
    ├── movies/
    └── tv/
```

Create all directories with appropriate permissions for the media user.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| MEDIA_BASE | /mnt/media | Root media directory |
| MAKEMKVCON_PATH | makemkvcon | Path to MakeMKV or mock-makemkv |

For testing with mock-makemkv:
```bash
export MAKEMKVCON_PATH=/home/media/bin/mock-makemkv
```

## User Setup

- User: `media`
- Home: `/home/media`
- PATH must include: `/home/media/bin`

## Quick Verification

After provisioning, verify the setup:

```bash
# Check binaries are installed
which media-pipeline ripper mock-makemkv

# Test mock-makemkv
mock-makemkv info disc:0

# Test ripper with mock
MAKEMKVCON_PATH=mock-makemkv ripper -t movie -n "Test Movie"

# Run TUI
media-pipeline
```

## Testing Workflow

1. **Rip a mock movie**:
   ```bash
   ripper -t movie -n "Big Buck Bunny"
   ```

2. **Rip a mock TV disc**:
   ```bash
   ripper -t tv -n "The Simpsons" -s 1 -d 1
   ```

3. **View pipeline status**:
   ```bash
   media-pipeline
   ```
   - Tab: Toggle between overview and action views
   - Enter: Drill into items
   - Esc: Go back
   - q: Quit

## Mock-MakeMKV Profiles

The mock-makemkv tool simulates different disc types:

| Profile | Description |
|---------|-------------|
| big_buck_bunny | Single-title movie disc (default) |
| simpsons_s01d01 | Multi-episode TV disc |
| problem_disc | Simulates read failure at 30% |

Use with `--profile`:
```bash
mock-makemkv --profile simpsons_s01d01 info disc:0
```

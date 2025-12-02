# Media Pipeline TUI

A Go + Bubbletea terminal UI for monitoring your media ripping pipeline.

## Features

- **Pipeline Overview**: See all items grouped by stage with progress bars
- **Stage Details**: Drill into each stage to see individual items
- **Action Needed**: Quick view of items requiring attention
- **Item Detail**: Full pipeline history and file listing

## Requirements

- Go 1.21+ (for building)
- Access to `/mnt/media/staging` filesystem (for running)

## Building

```bash
# Build for local machine
make build-local

# Build for Linux (production)
make build
```

## Deployment

```bash
# Deploy to analyzer container
make deploy

# Or manually:
scp bin/media-pipeline analyzer:/home/media/bin/
```

## Running

```bash
# Run on analyzer container (interactive)
make run-remote

# Or directly:
ssh -t analyzer '/home/media/bin/media-pipeline'
```

## Keyboard Controls

| Key | Action |
|-----|--------|
| `↑`/`k` | Move up |
| `↓`/`j` | Move down |
| `Enter` | Select / Drill down |
| `Esc` | Go back |
| `Tab` | Toggle Overview / Action view |
| `r` | Refresh (rescan filesystem) |
| `q` | Quit |

## Views

### Pipeline Overview
Shows a bar chart of items at each stage:
- 1-Ripped: Items freshly ripped from disc
- 2-Remuxed: Items with tracks filtered
- 3-Transcoded: Items encoded to H.265
- Library: Items organized by FileBot

### Action Needed
Groups items by what needs attention:
- Ready for next stage (completed items)
- In progress (currently processing)
- Failed (errors occurred)

### Item Detail
Shows full information about a single item:
- Type and current stage
- Pipeline history with timestamps
- List of media files with sizes

## Architecture

```
media-pipeline/
├── cmd/media-pipeline/    # Entry point
├── internal/
│   ├── model/            # Data types (MediaItem, Stage, etc.)
│   ├── scanner/          # Filesystem scanner
│   └── tui/              # Bubbletea views
└── Makefile
```

The scanner reads state files created by the bash scripts:
- `.rip/metadata.json` + `status`
- `.remux/metadata.json` + `status`
- `.transcode/metadata.json` + `status`
- `.filebot/metadata.json` + `status`

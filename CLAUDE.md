# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go + Bubbletea terminal UI for monitoring a media ripping pipeline. The TUI reads state from filesystem directories created by bash scripts that handle ripping, remuxing, transcoding, and organizing media files with FileBot.

## Build & Development Commands

```bash
# Build for local development
make build-local

# Build for Linux (production target)
make build

# Run tests
make test

# Format code
make fmt

# Vet code
make vet

# Deploy to analyzer container
make deploy

# Build, deploy, and run
make run
```

## Architecture

### Pipeline Stages

Media items flow through four stages, tracked via state directories:

1. **1-ripped** (`.rip/`) - Raw disc rip from MakeMKV
2. **2-remuxed** (`.remux/`) - Track filtering (select audio/subtitles)
3. **3-transcoded** (`.transcode/`) - H.265 encoding
4. **Library** (`.filebot/`) - Organized by FileBot into final library structure

### State Files

Each stage creates a state directory (e.g., `.rip/`, `.remux/`) containing:
- `metadata.json` - Job metadata (type, name, safe_name, season, timestamps)
- `status` - Current status: `pending`, `in_progress`, `completed`, `failed`
- `*.log` - Processing logs

### Code Structure

- `internal/model/media.go` - Core domain types: `Stage`, `Status`, `MediaType`, `MediaItem`, `PipelineState`
- `internal/scanner/scanner.go` - Filesystem scanner that reads state directories and builds `PipelineState`
- `internal/tui/app.go` - Main Bubbletea model with navigation state machine
- `internal/tui/*.go` - View renderers (overview, stagelist, actionlist, itemdetail)
- `cmd/media-pipeline/main.go` - Entry point
- `cmd/*/` - Bash scripts for each pipeline stage

### TUI Navigation

The app has four views managed by `currentView`:
- `ViewOverview` - Bar chart of items per stage
- `ViewStageList` - Items at a specific stage
- `ViewActionNeeded` - Items grouped by status (ready, in-progress, failed)
- `ViewItemDetail` - Full details for one item

Navigation: Enter to drill down, Esc to go back, Tab to toggle overview/action view.

## Filesystem Layout (Production)

```
/mnt/media/staging/
├── 1-ripped/{movies,tv}/     # Raw rips with .rip/ state
├── 2-remuxed/{movies,tv}/    # Remuxed with .remux/ state
├── 3-transcoded/{movies,tv}/ # Transcoded with .transcode/ and .filebot/ state
/mnt/media/library/           # Final organized library
```

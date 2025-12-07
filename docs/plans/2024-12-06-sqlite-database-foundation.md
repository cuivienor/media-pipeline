# SQLite Database Foundation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform the media-pipeline TUI from filesystem-based to SQLite-backed, implementing Phase 1 (Database Foundation) with dual-write ripper and organize stage validation.

**Architecture:** SQLite database as source of truth with dual-write to filesystem for debugging. Repository pattern with interface for testability. 5-stage pipeline: rip → organize → remux → transcode → publish.

**Tech Stack:** Go, SQLite (modernc.org/sqlite - pure Go), existing testenv E2E infrastructure

---

## Design Decisions Summary

| Decision | Choice |
|----------|--------|
| Storage | Local SQLite on same-host mount (locking reliable) |
| tv_discs table | Removed - track via jobs where stage='rip' AND disc IS NOT NULL |
| Log storage | Files in filesystem, DB stores path + significant events only |
| Stages | 5 stages: rip, organize, remux, transcode, publish |
| Stage enum naming | Action-oriented (StageRip not StageRipped) |
| Season format | Integer everywhere, format "S02" at display layer only |
| Organize completion | Manual button with validation (root empty, _main or _episodes exists) |
| CLI interface | Dual-mode: standalone with `-t/-n` args OR `-job-id` for pre-created jobs |
| TUI dispatch | SSH fire-and-forget (`ssh host "nohup cmd &"`), no daemons |
| SSH targets | Config file maps stage → host; local mode for E2E testing |

---

## Execution Architecture

### CLI Dual-Mode Interface

Each stage CLI (ripper, etc.) supports two modes:

```bash
# Mode 1: Standalone (creates job on-the-fly)
ripper -t movie -n "Movie Name" -db /mnt/media/pipeline.db

# Mode 2: Pre-created job (TUI dispatch)
ripper -job-id 123 -db /mnt/media/pipeline.db
```

Both modes:
1. Create/load MediaItem and Job in DB
2. Execute the stage work
3. Update job status in DB when complete
4. Exit (fire-and-forget, no daemon)

### TUI Dispatch Flow

```
User clicks "Rip" in TUI
    ↓
TUI creates Job record (status=pending)
    ↓
TUI reads config for SSH target (e.g., "ripper")
    ↓
TUI runs: ssh ripper "nohup ripper -job-id 123 -db /mnt/media/pipeline.db &"
    ↓
TUI updates Job status to in_progress
    ↓
CLI runs on remote, updates DB directly when done
    ↓
TUI polls DB to show status (no persistent connection)
```

### Directory Structure

```
/mnt/media/
├── pipeline/                  # App data directory (BACK THIS UP!)
│   ├── pipeline.db            # SQLite database
│   ├── config.yaml            # Config file
│   └── logs/
│       └── jobs/
│           ├── 123/           # Per-job directory
│           │   ├── job.log    # Wrapper log (INFO/PROGRESS/WARN/ERROR)
│           │   └── makemkv.log # Raw tool output
│           ├── 124/
│           │   ├── job.log
│           │   └── ffmpeg.log
│           └── ...
├── staging/                   # Transient media processing
│   ├── 1-ripped/
│   │   └── movies/Movie_Name/
│   │       └── .rip/
│   │           ├── status
│   │           ├── metadata.json
│   │           ├── job.log -> /mnt/media/pipeline/logs/jobs/123/job.log
│   │           └── makemkv.log -> /mnt/media/pipeline/logs/jobs/123/makemkv.log
│   ├── 2-remuxed/
│   └── 3-transcoded/
└── library/                   # Final organized media
```

### Config File Format

```yaml
# /mnt/media/pipeline/config.yaml (production)
# or ~/.config/media-pipeline/config.yaml (development)

data_dir: /mnt/media/pipeline  # App data directory (db, logs)
staging_base: /mnt/media/staging
library_base: /mnt/media/library

# SSH targets per stage (empty = local execution for testing)
dispatch:
  rip: ripper          # ssh ripper "ripper ..."
  remux: analyzer      # ssh analyzer "remux ..."
  transcode: transcoder
  publish: analyzer

# E2E testing mode: set all to empty for local execution
# dispatch:
#   rip: ""
#   remux: ""
#   transcode: ""
#   publish: ""
```

Config resolution order:
1. `-config` flag if provided
2. `/mnt/media/pipeline/config.yaml` (production default)
3. `~/.config/media-pipeline/config.yaml` (development fallback)

### E2E Testing Mode

For E2E tests, config uses empty dispatch targets = local subprocess execution:

```go
// When dispatch target is empty, run locally instead of SSH
if target == "" {
    cmd := exec.Command("ripper", "-job-id", jobID, "-db", dbPath)
} else {
    cmd := exec.Command("ssh", target, fmt.Sprintf("nohup ripper -job-id %s -db %s &", jobID, dbPath))
}
```

---

## Logging Architecture

### Log Levels

Five levels: `DEBUG`, `INFO`, `PROGRESS`, `WARN`, `ERROR`

- **DEBUG**: Verbose output for troubleshooting
- **INFO**: Normal operational messages
- **PROGRESS**: Status updates (10%, 20%, etc.) for long-running operations
- **WARN**: Non-fatal issues
- **ERROR**: Failures

### Log Destinations

| Context | Stdout | File | DB Events |
|---------|--------|------|-----------|
| CLI (DB available) | ✓ | ✓ | ✓ |
| CLI (no DB configured) | ✓ | ✓ | - |
| CLI via SSH dispatch | - | ✓ | ✓ |
| E2E tests | captured | ✓ | ✓ |

### DB Path Resolution

1. If `-db` flag provided → use that
2. Else if config file exists → use `database` from config
3. Else → no DB (filesystem-only mode, for testing/one-offs)

### Log Format

Human-readable format:
```
2024-01-15 10:30:45 [INFO] Starting rip for "Movie Name"
2024-01-15 10:30:46 [PROGRESS] Scanning disc...
2024-01-15 10:31:00 [PROGRESS] Ripping: 10% complete
2024-01-15 10:45:00 [INFO] Rip completed successfully
```

### DB Events

Only significant events go to `log_events` table:
- Job started
- Job completed
- Job failed (with error message)
- Stage transitions

Progress updates (10%, 20%) go to log file only - NOT to DB.

### Log File Location

**Persistent logs** (survive media cleanup):
```
/mnt/media/pipeline/logs/jobs/{job_id}/
├── job.log       # Wrapper log (human-readable)
├── makemkv.log   # Raw tool output (stage-specific)
├── ffmpeg.log
└── filebot.log
```

**Transient symlinks** (convenience during active work):
```
.rip/job.log -> /mnt/media/pipeline/logs/jobs/{job_id}/job.log
.rip/makemkv.log -> /mnt/media/pipeline/logs/jobs/{job_id}/makemkv.log
```

When a job starts:
1. Create job log directory at `{data_dir}/logs/jobs/{job_id}/`
2. Create `job.log` for wrapper output
3. Create tool-specific log (e.g., `makemkv.log`) for raw output
4. Create symlinks in state directory (`.rip/`, `.remux/`, etc.)
5. All wrapper logging goes to `job.log`, tool output to tool-specific files

When media is cleaned up, symlinks are deleted but logs persist.

### TUI Log Viewing

TUI tails the log file directly for live output:
- Uses path from `jobs.log_path` (persistent location)
- For completed jobs, read full file
- For running jobs, tail -f equivalent

---

## Downstream Tool Log Capture

Each pipeline stage invokes external tools (MakeMKV, ffmpeg, FileBot). Their output must be captured for debugging and progress tracking.

### Log Directory Structure

```
/mnt/media/pipeline/logs/jobs/{job_id}/
├── job.log          # Our wrapper's log (INFO/WARN/ERROR/PROGRESS)
├── makemkv.log      # Raw makemkv stdout (robot mode) - rip stage only
├── ffmpeg.log       # Raw ffmpeg stderr - transcode stage only
└── filebot.log      # Raw filebot output - publish stage only
```

### Tool Output Characteristics

| Tool | Output Stream | Format | Progress Source |
|------|--------------|--------|-----------------|
| MakeMKV | stdout (with `-r`) | Machine-parseable: `PRGV:cur,tot,max` | `tot/max * 100` |
| ffmpeg | stderr | Key-value: `frame=N out_time=HH:MM:SS speed=Nx` | `out_time/duration * 100` |
| FileBot | mixed stdout/stderr | Human-readable with `[TEST]`, `[COPY]` prefixes | N/A (fast) |

### MakeMKV Output ([docs](https://www.makemkv.com/developers/usage.txt))

Robot mode (`-r`) produces machine-parseable output on stdout:
- `PRGV:current,total,max` - Progress values (total/max = overall %)
- `PRGT:code,id,name` - Current operation name
- `MSG:id,flag,count,message,...` - Status/error messages
- `TCOUT`, `CINFO`, `TINFO`, `SINFO` - Disc/title metadata

**Capture strategy:**
```go
// Pipe stdout to both parser (for progress) and raw log file
stdout, _ := cmd.StdoutPipe()
stderr, _ := cmd.StderrPipe()

// Tee stdout to raw log + parser
tee := io.TeeReader(stdout, rawLogFile)
scanner := bufio.NewScanner(tee)
for scanner.Scan() {
    line := scanner.Text()
    if progress := parsePRGV(line); progress != nil {
        logger.Progress("Ripping: %d%% complete", progress.Percent())
    }
}

// Capture stderr to raw log (errors)
io.Copy(rawLogFile, stderr)
```

### ffmpeg Output ([docs](https://ffmpeg.org/ffmpeg.html))

All logging goes to stderr (stdout is for media data). With `-progress - -nostats`:
```
frame=2675
fps=120.5
out_time=00:01:47.000000
speed=2.1x
progress=continue
```

**Capture strategy:**
```go
// Redirect stderr to both raw log and parser
cmd.Stdout = nil  // Don't capture media data!
stderr, _ := cmd.StderrPipe()

tee := io.TeeReader(stderr, rawLogFile)
scanner := bufio.NewScanner(tee)

var outTime time.Duration
for scanner.Scan() {
    line := scanner.Text()
    if strings.HasPrefix(line, "out_time=") {
        outTime = parseOutTime(line)
        percent := float64(outTime) / float64(totalDuration) * 100
        logger.Progress("Transcoding: %.1f%% complete (speed: %s)", percent, speed)
    }
}
```

**Note:** ffmpeg uses `\r` to overwrite status lines. Scanner handles this by reading complete lines.

### FileBot Output ([docs](https://www.filebot.net/cli.html))

Human-readable output with log levels indicated by color (stripped in capture):
- `[TEST] from [src] to [dest]` - Preview mode
- `[COPY] from [src] to [dest]` - Actual operations
- `[MOVE]`, `[HARDLINK]` - Other actions

**Capture strategy:**
```go
// Capture combined stdout/stderr
cmd.Stdout = rawLogFile
cmd.Stderr = rawLogFile

// FileBot is fast enough that we don't need progress parsing
// Just log start/end events
logger.Info("Starting FileBot rename")
err := cmd.Run()
if err != nil {
    logger.Error("FileBot failed: %v", err)
} else {
    logger.Info("FileBot completed successfully")
}
```

### Implementation Notes

1. **Raw logs are verbose** - Only written to per-tool files, not job.log
2. **Progress goes to job.log** - Parsed from raw output, human-readable format
3. **TUI shows job.log** - Clean, consistent format across all stages
4. **Drill-down available** - TUI can display raw tool logs for debugging
5. **Input duration for ffmpeg** - Must probe input file first to calculate progress percentage

### Symlinks for Active Jobs

During job execution, create convenience symlinks in the state directory:

```
.rip/
├── status
├── metadata.json
├── job.log -> /mnt/media/pipeline/logs/jobs/123/job.log
├── makemkv.log -> /mnt/media/pipeline/logs/jobs/123/makemkv.log
```

This allows `tail -f .rip/makemkv.log` during active rips without knowing the job ID.

---

### Logger Interface

```go
package logging

type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Progress(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)

    // Event logs to DB (if available) and file
    Event(level string, msg string)
}

// NewLogger creates a logger with the specified outputs
func NewLogger(opts LoggerOptions) Logger

type LoggerOptions struct {
    Stdout    bool        // Write to stdout
    FilePath  string      // Write to file (empty = no file)
    DBRepo    Repository  // Write events to DB (nil = no DB)
    JobID     int64       // Job ID for DB events
    MinLevel  Level       // Minimum level to log
}
```

---

## Database Schema

```sql
-- File: internal/db/migrations/001_initial.sql

CREATE TABLE media_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK (type IN ('movie', 'tv')),
    name TEXT NOT NULL,
    safe_name TEXT NOT NULL,
    season INTEGER,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(safe_name, season)
);

CREATE TABLE jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    stage TEXT NOT NULL CHECK (stage IN ('rip', 'organize', 'remux', 'transcode', 'publish')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'in_progress', 'completed', 'failed')),
    disc INTEGER,
    worker_id TEXT,
    pid INTEGER,
    input_dir TEXT,
    output_dir TEXT,
    log_path TEXT,
    error_message TEXT,
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(media_item_id, stage, disc)
);

CREATE TABLE log_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    level TEXT NOT NULL CHECK (level IN ('info', 'warn', 'error')),
    message TEXT NOT NULL,
    timestamp TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX idx_jobs_media_item ON jobs(media_item_id);
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_log_events_job ON log_events(job_id);
```

---

## Critical Files

| File | Changes |
|------|---------|
| `internal/model/media.go` | Rename Stage enum, add ID fields, add Job/LogEvent types, change Season to *int |
| `internal/ripper/state.go` | Add DualWriteStateManager wrapping DefaultStateManager |
| `internal/scanner/scanner.go` | Update for new Stage names, handle Season as int |
| `tests/e2e/testenv/environment.go` | Add DB fixture support |

---

## Task 1: Add SQLite Dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add modernc.org/sqlite dependency**

```bash
go get modernc.org/sqlite
```

**Step 2: Verify dependency added**

```bash
go mod tidy && grep sqlite go.mod
```

Expected: `modernc.org/sqlite v...`

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add modernc.org/sqlite for pure-Go SQLite"
```

---

## Task 2: Update Stage Enum

**Files:**
- Modify: `internal/model/media.go`
- Modify: `internal/model/media_test.go` (if exists)

**Step 2.1: Write failing test for new stage names**

Create `internal/model/stage_test.go`:

```go
package model

import "testing"

func TestStage_String(t *testing.T) {
    tests := []struct {
        stage Stage
        want  string
    }{
        {StageRip, "rip"},
        {StageOrganize, "organize"},
        {StageRemux, "remux"},
        {StageTranscode, "transcode"},
        {StagePublish, "publish"},
    }
    for _, tt := range tests {
        if got := tt.stage.String(); got != tt.want {
            t.Errorf("Stage(%d).String() = %q, want %q", tt.stage, got, tt.want)
        }
    }
}

func TestStage_DisplayName(t *testing.T) {
    tests := []struct {
        stage Stage
        want  string
    }{
        {StageRip, "1-Ripped"},
        {StageOrganize, "2-Organized"},
        {StageRemux, "3-Remuxed"},
        {StageTranscode, "4-Transcoded"},
        {StagePublish, "Library"},
    }
    for _, tt := range tests {
        if got := tt.stage.DisplayName(); got != tt.want {
            t.Errorf("Stage(%d).DisplayName() = %q, want %q", tt.stage, got, tt.want)
        }
    }
}

func TestStage_NextStage(t *testing.T) {
    tests := []struct {
        stage Stage
        want  Stage
    }{
        {StageRip, StageOrganize},
        {StageOrganize, StageRemux},
        {StageRemux, StageTranscode},
        {StageTranscode, StagePublish},
        {StagePublish, StagePublish}, // terminal
    }
    for _, tt := range tests {
        if got := tt.stage.NextStage(); got != tt.want {
            t.Errorf("Stage(%d).NextStage() = %d, want %d", tt.stage, got, tt.want)
        }
    }
}
```

**Step 2.2: Run test to verify it fails**

```bash
go test ./internal/model/... -run TestStage -v
```

Expected: FAIL (StageOrganize not defined, names don't match)

**Step 2.3: Update Stage enum in media.go**

Replace Stage constants and methods:

```go
// Stage represents a step in the media processing pipeline
type Stage int

const (
    StageRip Stage = iota
    StageOrganize
    StageRemux
    StageTranscode
    StagePublish
)

func (s Stage) String() string {
    switch s {
    case StageRip:
        return "rip"
    case StageOrganize:
        return "organize"
    case StageRemux:
        return "remux"
    case StageTranscode:
        return "transcode"
    case StagePublish:
        return "publish"
    default:
        return "unknown"
    }
}

func (s Stage) DisplayName() string {
    switch s {
    case StageRip:
        return "1-Ripped"
    case StageOrganize:
        return "2-Organized"
    case StageRemux:
        return "3-Remuxed"
    case StageTranscode:
        return "4-Transcoded"
    case StagePublish:
        return "Library"
    default:
        return "Unknown"
    }
}

func (s Stage) NextStage() Stage {
    switch s {
    case StageRip:
        return StageOrganize
    case StageOrganize:
        return StageRemux
    case StageRemux:
        return StageTranscode
    case StageTranscode:
        return StagePublish
    default:
        return StagePublish
    }
}

func (s Stage) NextAction() string {
    switch s {
    case StageRip:
        return "needs organize"
    case StageOrganize:
        return "needs remux"
    case StageRemux:
        return "needs transcode"
    case StageTranscode:
        return "needs publish"
    case StagePublish:
        return "complete"
    default:
        return "unknown"
    }
}
```

**Step 2.4: Run tests to verify they pass**

```bash
go test ./internal/model/... -run TestStage -v
```

Expected: PASS

**Step 2.5: Commit**

```bash
git add internal/model/
git commit -m "model: rename stages to action-oriented names, add organize stage"
```

---

## Task 3: Fix Compilation Errors from Stage Rename

**Files:**
- Modify: `internal/scanner/scanner.go`
- Modify: `internal/tui/*.go` (as needed)
- Modify: Any files referencing old stage names

**Step 3.1: Find all references to old stage names**

```bash
grep -rn "StageRipped\|StageRemuxed\|StageTranscoded\|StageInLibrary" --include="*.go"
```

**Step 3.2: Update scanner stage mapping**

In `internal/scanner/scanner.go`, update the stage directory mapping:

```go
// Old:
// case "1-ripped": stage = model.StageRipped
// New:
case "1-ripped":
    stage = model.StageRip
case "2-remuxed":
    stage = model.StageRemux
case "3-transcoded":
    stage = model.StageTranscode
```

**Step 3.3: Update TUI files**

Replace all occurrences:
- `StageRipped` → `StageRip`
- `StageRemuxed` → `StageRemux`
- `StageTranscoded` → `StageTranscode`
- `StageInLibrary` → `StagePublish`

**Step 3.4: Verify all tests pass**

```bash
go test ./... -v
```

**Step 3.5: Commit**

```bash
git add .
git commit -m "refactor: update all code to use new stage names"
```

---

## Task 4: Change Season from String to *int

**Files:**
- Modify: `internal/model/media.go`
- Modify: `internal/scanner/scanner.go`
- Modify: `internal/ripper/ripper.go` and `types.go`
- Modify: Tests as needed

**Step 4.1: Write failing test for Season as int**

Add to `internal/model/media_test.go`:

```go
func TestMediaItem_UniqueKey(t *testing.T) {
    tests := []struct {
        name string
        item MediaItem
        want string
    }{
        {
            name: "movie has no season",
            item: MediaItem{Type: MediaTypeMovie, SafeName: "Movie_Name"},
            want: "Movie_Name",
        },
        {
            name: "TV with season",
            item: MediaItem{Type: MediaTypeTV, SafeName: "Show_Name", Season: intPtr(2)},
            want: "Show_Name_S02",
        },
        {
            name: "TV with nil season",
            item: MediaItem{Type: MediaTypeTV, SafeName: "Show_Name", Season: nil},
            want: "Show_Name",
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := tt.item.UniqueKey(); got != tt.want {
                t.Errorf("UniqueKey() = %q, want %q", got, tt.want)
            }
        })
    }
}

func intPtr(i int) *int {
    return &i
}
```

**Step 4.2: Run test to verify it fails**

```bash
go test ./internal/model/... -run TestMediaItem_UniqueKey -v
```

**Step 4.3: Update MediaItem.Season to *int**

```go
type MediaItem struct {
    ID       int64
    Type     MediaType
    Name     string
    SafeName string
    Season   *int  // Changed from string, nil for movies

    Stages  []StageInfo
    Current Stage
    Status  Status
}

func (m *MediaItem) UniqueKey() string {
    if m.Type == MediaTypeTV && m.Season != nil {
        return fmt.Sprintf("%s_S%02d", m.SafeName, *m.Season)
    }
    return m.SafeName
}
```

**Step 4.4: Update scanner to parse season as int**

In `internal/scanner/scanner.go`, the season parsing from metadata needs updating.

**Step 4.5: Update ripper types**

In `internal/ripper/types.go`, ensure Season is int:

```go
type RipRequest struct {
    Type     MediaType
    Name     string
    Season   int  // 0 for movies
    Disc     int
    DiscPath string
}
```

**Step 4.6: Run all tests**

```bash
go test ./... -v
```

**Step 4.7: Commit**

```bash
git add .
git commit -m "model: change Season from string to *int, format at display layer"
```

---

## Task 5: Create Database Package Structure

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/migrations.go`
- Create: `internal/db/migrations/001_initial.sql`

**Step 5.1: Create migrations directory and SQL file**

```bash
mkdir -p internal/db/migrations
```

Create `internal/db/migrations/001_initial.sql` with schema from above.

**Step 5.2: Write test for database opening and migration**

Create `internal/db/db_test.go`:

```go
package db

import (
    "testing"
)

func TestOpen_InMemory(t *testing.T) {
    database, err := OpenInMemory()
    if err != nil {
        t.Fatalf("OpenInMemory() error = %v", err)
    }
    defer database.Close()

    // Verify tables exist
    var count int
    err = database.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='media_items'").Scan(&count)
    if err != nil {
        t.Fatalf("Query error: %v", err)
    }
    if count != 1 {
        t.Errorf("media_items table not created")
    }
}

func TestOpen_MigrationsApplied(t *testing.T) {
    database, err := OpenInMemory()
    if err != nil {
        t.Fatalf("OpenInMemory() error = %v", err)
    }
    defer database.Close()

    // Verify all expected tables
    tables := []string{"media_items", "jobs", "log_events"}
    for _, table := range tables {
        var count int
        err = database.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
        if err != nil {
            t.Fatalf("Query error for %s: %v", table, err)
        }
        if count != 1 {
            t.Errorf("table %s not created", table)
        }
    }
}
```

**Step 5.3: Run test to verify it fails**

```bash
go test ./internal/db/... -v
```

**Step 5.4: Implement db.go**

```go
package db

import (
    "database/sql"
    "embed"
    "fmt"
    "io/fs"
    "sort"
    "strings"

    _ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// DB wraps a SQLite database connection
type DB struct {
    db *sql.DB
}

// Open opens a SQLite database at the given path
func Open(path string) (*DB, error) {
    db, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Enable foreign keys
    if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
    }

    database := &DB{db: db}
    if err := database.migrate(); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to run migrations: %w", err)
    }

    return database, nil
}

// OpenInMemory opens an in-memory SQLite database for testing
func OpenInMemory() (*DB, error) {
    return Open(":memory:")
}

// Close closes the database connection
func (d *DB) Close() error {
    return d.db.Close()
}

// migrate runs all SQL migrations
func (d *DB) migrate() error {
    entries, err := fs.ReadDir(migrations, "migrations")
    if err != nil {
        return fmt.Errorf("failed to read migrations: %w", err)
    }

    // Sort by filename to ensure order
    var files []string
    for _, entry := range entries {
        if strings.HasSuffix(entry.Name(), ".sql") {
            files = append(files, entry.Name())
        }
    }
    sort.Strings(files)

    for _, file := range files {
        content, err := fs.ReadFile(migrations, "migrations/"+file)
        if err != nil {
            return fmt.Errorf("failed to read %s: %w", file, err)
        }

        if _, err := d.db.Exec(string(content)); err != nil {
            return fmt.Errorf("failed to execute %s: %w", file, err)
        }
    }

    return nil
}
```

**Step 5.5: Run tests**

```bash
go test ./internal/db/... -v
```

**Step 5.6: Commit**

```bash
git add internal/db/
git commit -m "db: add SQLite database package with migrations"
```

---

## Task 6: Add Job and LogEvent Types to Model

**Files:**
- Modify: `internal/model/media.go`
- Create: `internal/model/job.go`

**Step 6.1: Write test for Job type**

Create `internal/model/job_test.go`:

```go
package model

import (
    "testing"
    "time"
)

func TestJob_IsActive(t *testing.T) {
    tests := []struct {
        status JobStatus
        want   bool
    }{
        {JobStatusPending, true},
        {JobStatusInProgress, true},
        {JobStatusCompleted, false},
        {JobStatusFailed, false},
    }
    for _, tt := range tests {
        job := Job{Status: tt.status}
        if got := job.IsActive(); got != tt.want {
            t.Errorf("Job{Status: %q}.IsActive() = %v, want %v", tt.status, got, tt.want)
        }
    }
}

func TestJob_Duration(t *testing.T) {
    start := time.Now().Add(-1 * time.Hour)
    end := time.Now()

    job := Job{
        StartedAt:   &start,
        CompletedAt: &end,
    }

    duration := job.Duration()
    if duration < 59*time.Minute || duration > 61*time.Minute {
        t.Errorf("Duration() = %v, want ~1 hour", duration)
    }
}
```

**Step 6.2: Run test to verify it fails**

```bash
go test ./internal/model/... -run TestJob -v
```

**Step 6.3: Create job.go**

```go
package model

import "time"

// JobStatus represents the current state of a job
type JobStatus string

const (
    JobStatusPending    JobStatus = "pending"
    JobStatusInProgress JobStatus = "in_progress"
    JobStatusCompleted  JobStatus = "completed"
    JobStatusFailed     JobStatus = "failed"
)

// Job represents a single stage execution attempt
type Job struct {
    ID           int64
    MediaItemID  int64
    Stage        Stage
    Status       JobStatus
    Disc         *int
    WorkerID     string
    PID          int
    InputDir     string
    OutputDir    string
    LogPath      string
    ErrorMessage string
    StartedAt    *time.Time
    CompletedAt  *time.Time
    CreatedAt    time.Time
}

// IsActive returns true if the job is pending or in progress
func (j *Job) IsActive() bool {
    return j.Status == JobStatusPending || j.Status == JobStatusInProgress
}

// Duration returns the job duration, or zero if not completed
func (j *Job) Duration() time.Duration {
    if j.StartedAt == nil || j.CompletedAt == nil {
        return 0
    }
    return j.CompletedAt.Sub(*j.StartedAt)
}

// LogEvent represents a significant event during job execution
type LogEvent struct {
    ID        int64
    JobID     int64
    Level     string // "info", "warn", "error"
    Message   string
    Timestamp time.Time
}

// DiscProgress tracks rip status for a TV disc
type DiscProgress struct {
    Disc   int
    Status JobStatus
    JobID  int64
}
```

**Step 6.4: Run tests**

```bash
go test ./internal/model/... -v
```

**Step 6.5: Commit**

```bash
git add internal/model/
git commit -m "model: add Job, LogEvent, DiscProgress types"
```

---

## Task 7: Implement Repository Interface

**Files:**
- Create: `internal/db/repository.go`
- Create: `internal/db/sqlite.go`
- Create: `internal/db/sqlite_test.go`

**Step 7.1: Define Repository interface**

```go
package db

import (
    "context"
    "github.com/cuivienor/media-pipeline/internal/model"
)

// Repository defines persistence operations for the pipeline
type Repository interface {
    // Media items
    CreateMediaItem(ctx context.Context, item *model.MediaItem) error
    GetMediaItem(ctx context.Context, id int64) (*model.MediaItem, error)
    GetMediaItemBySafeName(ctx context.Context, safeName string, season *int) (*model.MediaItem, error)
    ListMediaItems(ctx context.Context, opts ListOptions) ([]model.MediaItem, error)

    // Jobs
    CreateJob(ctx context.Context, job *model.Job) error
    GetJob(ctx context.Context, id int64) (*model.Job, error)
    GetActiveJobForStage(ctx context.Context, mediaItemID int64, stage model.Stage, disc *int) (*model.Job, error)
    UpdateJobStatus(ctx context.Context, id int64, status model.JobStatus, errorMsg string) error
    ListJobsForMedia(ctx context.Context, mediaItemID int64) ([]model.Job, error)

    // Log events
    CreateLogEvent(ctx context.Context, event *model.LogEvent) error
    ListLogEvents(ctx context.Context, jobID int64, limit int) ([]model.LogEvent, error)

    // Disc progress (TV shows)
    GetDiscProgress(ctx context.Context, mediaItemID int64) ([]model.DiscProgress, error)
}

// ListOptions configures media item listing
type ListOptions struct {
    Type       *model.MediaType
    ActiveOnly bool
    Limit      int
    Offset     int
}
```

**Step 7.2: Write tests for SQLite repository**

Create `internal/db/sqlite_test.go` with table-driven tests for each method.

**Step 7.3: Implement SQLiteRepository**

Create `internal/db/sqlite.go` implementing each method.

**Step 7.4: Run tests**

```bash
go test ./internal/db/... -v
```

**Step 7.5: Commit**

```bash
git add internal/db/
git commit -m "db: implement SQLite repository with full CRUD operations"
```

---

## Task 8: Create Organization Validator

**Files:**
- Create: `internal/organize/validator.go`
- Create: `internal/organize/validator_test.go`

**Step 8.1: Write failing tests**

```go
package organize

import (
    "os"
    "path/filepath"
    "testing"
)

func TestValidator_ValidateMovie(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(dir string)
        wantOK  bool
        wantErr string
    }{
        {
            name: "valid: _main has files, root empty",
            setup: func(dir string) {
                os.MkdirAll(filepath.Join(dir, "_main"), 0755)
                os.WriteFile(filepath.Join(dir, "_main", "movie.mkv"), []byte{}, 0644)
            },
            wantOK: true,
        },
        {
            name: "invalid: root has loose files",
            setup: func(dir string) {
                os.MkdirAll(filepath.Join(dir, "_main"), 0755)
                os.WriteFile(filepath.Join(dir, "_main", "movie.mkv"), []byte{}, 0644)
                os.WriteFile(filepath.Join(dir, "title_t00.mkv"), []byte{}, 0644)
            },
            wantOK:  false,
            wantErr: "root directory not empty",
        },
        {
            name: "invalid: _main missing",
            setup: func(dir string) {
                os.MkdirAll(filepath.Join(dir, "_extras"), 0755)
            },
            wantOK:  false,
            wantErr: "_main directory not found",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            dir := t.TempDir()
            tt.setup(dir)

            v := &Validator{}
            result := v.ValidateMovie(dir)

            if result.Valid != tt.wantOK {
                t.Errorf("Valid = %v, want %v", result.Valid, tt.wantOK)
            }
            if !tt.wantOK && len(result.Errors) == 0 {
                t.Error("expected errors but got none")
            }
        })
    }
}
```

**Step 8.2: Implement validator**

```go
package organize

import (
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strconv"
)

type ValidationResult struct {
    Valid    bool
    Errors   []string
    Warnings []string
}

type Validator struct{}

func (v *Validator) ValidateMovie(outputDir string) ValidationResult {
    result := ValidationResult{Valid: true}

    // Check root is empty (except _ dirs and .rip)
    if errs := v.checkRootEmpty(outputDir); len(errs) > 0 {
        result.Valid = false
        result.Errors = append(result.Errors, errs...)
    }

    // Check _main exists and has files
    mainDir := filepath.Join(outputDir, "_main")
    if _, err := os.Stat(mainDir); os.IsNotExist(err) {
        result.Valid = false
        result.Errors = append(result.Errors, "_main directory not found")
    } else {
        files, _ := filepath.Glob(filepath.Join(mainDir, "*.mkv"))
        if len(files) == 0 {
            result.Valid = false
            result.Errors = append(result.Errors, "_main has no .mkv files")
        }
    }

    return result
}

func (v *Validator) ValidateTV(outputDir string) ValidationResult {
    result := ValidationResult{Valid: true}

    // Check root is empty
    if errs := v.checkRootEmpty(outputDir); len(errs) > 0 {
        result.Valid = false
        result.Errors = append(result.Errors, errs...)
    }

    // Check _episodes exists
    episodesDir := filepath.Join(outputDir, "_episodes")
    if _, err := os.Stat(episodesDir); os.IsNotExist(err) {
        result.Valid = false
        result.Errors = append(result.Errors, "_episodes directory not found")
        return result
    }

    // Check episode naming and sequence
    files, _ := filepath.Glob(filepath.Join(episodesDir, "*.mkv"))
    episodes := v.parseEpisodeNumbers(files)

    if len(episodes) == 0 {
        result.Valid = false
        result.Errors = append(result.Errors, "_episodes has no valid episode files")
        return result
    }

    // Check for gaps
    if gaps := v.findGaps(episodes); len(gaps) > 0 {
        result.Valid = false
        for _, gap := range gaps {
            result.Errors = append(result.Errors, fmt.Sprintf("missing episode %d", gap))
        }
    }

    return result
}

func (v *Validator) checkRootEmpty(dir string) []string {
    var errors []string
    entries, _ := os.ReadDir(dir)
    for _, entry := range entries {
        name := entry.Name()
        // Allow _ prefixed dirs and .rip state dir
        if name[0] != '_' && name != ".rip" {
            errors = append(errors, fmt.Sprintf("root directory not empty: found %s", name))
        }
    }
    return errors
}

var episodePattern = regexp.MustCompile(`^(\d+)(?:-\d+)?(?:_.*)?\.mkv$`)

func (v *Validator) parseEpisodeNumbers(files []string) []int {
    var episodes []int
    for _, file := range files {
        base := filepath.Base(file)
        if matches := episodePattern.FindStringSubmatch(base); matches != nil {
            if num, err := strconv.Atoi(matches[1]); err == nil {
                episodes = append(episodes, num)
            }
        }
    }
    sort.Ints(episodes)
    return episodes
}

func (v *Validator) findGaps(episodes []int) []int {
    if len(episodes) == 0 {
        return nil
    }
    var gaps []int
    for i := episodes[0]; i < episodes[len(episodes)-1]; i++ {
        found := false
        for _, ep := range episodes {
            if ep == i {
                found = true
                break
            }
        }
        if !found {
            gaps = append(gaps, i)
        }
    }
    return gaps
}
```

**Step 8.3: Run tests**

```bash
go test ./internal/organize/... -v
```

**Step 8.4: Commit**

```bash
git add internal/organize/
git commit -m "organize: add validation logic for movie and TV organization"
```

---

## Task 9: Add DB Fixture Support to Test Environment

**Files:**
- Create: `tests/e2e/testenv/db.go`
- Modify: `tests/e2e/testenv/environment.go`

**Step 9.1: Create DBFixture**

```go
package testenv

import (
    "context"
    "testing"

    "github.com/cuivienor/media-pipeline/internal/db"
    "github.com/cuivienor/media-pipeline/internal/model"
)

// DBFixture provides database test helpers
type DBFixture struct {
    t    *testing.T
    DB   *db.DB
    Repo db.Repository
}

// NewDBFixture creates an in-memory database for testing
func NewDBFixture(t *testing.T) *DBFixture {
    t.Helper()
    database, err := db.OpenInMemory()
    if err != nil {
        t.Fatalf("failed to open in-memory database: %v", err)
    }

    t.Cleanup(func() {
        database.Close()
    })

    return &DBFixture{
        t:    t,
        DB:   database,
        Repo: db.NewSQLiteRepository(database),
    }
}

// CreateMovie creates a movie MediaItem for testing
func (f *DBFixture) CreateMovie(name, safeName string) *model.MediaItem {
    f.t.Helper()
    item := &model.MediaItem{
        Type:     model.MediaTypeMovie,
        Name:     name,
        SafeName: safeName,
    }
    if err := f.Repo.CreateMediaItem(context.Background(), item); err != nil {
        f.t.Fatalf("CreateMovie failed: %v", err)
    }
    return item
}

// CreateTVSeason creates a TV season MediaItem for testing
func (f *DBFixture) CreateTVSeason(name, safeName string, season int) *model.MediaItem {
    f.t.Helper()
    item := &model.MediaItem{
        Type:     model.MediaTypeTV,
        Name:     name,
        SafeName: safeName,
        Season:   &season,
    }
    if err := f.Repo.CreateMediaItem(context.Background(), item); err != nil {
        f.t.Fatalf("CreateTVSeason failed: %v", err)
    }
    return item
}

// CreateRipJob creates a rip job for testing
func (f *DBFixture) CreateRipJob(mediaItemID int64, disc *int, status model.JobStatus) *model.Job {
    f.t.Helper()
    job := &model.Job{
        MediaItemID: mediaItemID,
        Stage:       model.StageRip,
        Status:      status,
        Disc:        disc,
    }
    if err := f.Repo.CreateJob(context.Background(), job); err != nil {
        f.t.Fatalf("CreateRipJob failed: %v", err)
    }
    return job
}
```

**Step 9.2: Extend Environment with optional DB**

```go
// In environment.go, add:

func (e *Environment) WithDB(t *testing.T) (*Environment, *DBFixture) {
    t.Helper()
    dbFixture := NewDBFixture(t)
    return e, dbFixture
}
```

**Step 9.3: Commit**

```bash
git add tests/e2e/testenv/
git commit -m "testenv: add database fixture support for E2E tests"
```

---

## Task 10: Implement DualWriteStateManager

**Files:**
- Modify: `internal/ripper/state.go`
- Create: `internal/ripper/state_dual_test.go`

**Step 10.1: Write failing test**

```go
func TestDualWriteStateManager_Initialize(t *testing.T) {
    env := testenv.New(t)
    dbFixture := testenv.NewDBFixture(t)

    dm := NewDualWriteStateManager(
        NewStateManager(),
        dbFixture.Repo,
    )

    req := &RipRequest{
        Type: MediaTypeMovie,
        Name: "Test Movie",
    }

    outputDir := filepath.Join(env.RippedMoviesDir(), "Test_Movie")
    err := dm.Initialize(outputDir, req)
    if err != nil {
        t.Fatalf("Initialize failed: %v", err)
    }

    // Verify filesystem state created
    stateDir, err := testenv.FindStateDir(outputDir, ".rip")
    if err != nil {
        t.Fatal("filesystem state not created")
    }
    stateDir.AssertStatus(t, model.StatusInProgress)

    // Verify database state created
    item, err := dbFixture.Repo.GetMediaItemBySafeName(context.Background(), "Test_Movie", nil)
    if err != nil {
        t.Fatalf("media item not in database: %v", err)
    }
    if item == nil {
        t.Fatal("media item not found")
    }

    jobs, err := dbFixture.Repo.ListJobsForMedia(context.Background(), item.ID)
    if err != nil {
        t.Fatalf("failed to list jobs: %v", err)
    }
    if len(jobs) != 1 {
        t.Errorf("expected 1 job, got %d", len(jobs))
    }
    if jobs[0].Status != model.JobStatusInProgress {
        t.Errorf("job status = %v, want in_progress", jobs[0].Status)
    }
}
```

**Step 10.2: Implement DualWriteStateManager**

```go
// DualWriteStateManager writes to both database and filesystem
type DualWriteStateManager struct {
    fs     StateManager
    repo   db.Repository
    jobID  int64
    itemID int64
}

func NewDualWriteStateManager(fs StateManager, repo db.Repository) *DualWriteStateManager {
    return &DualWriteStateManager{
        fs:   fs,
        repo: repo,
    }
}

func (d *DualWriteStateManager) Initialize(outputDir string, req *RipRequest) error {
    ctx := context.Background()

    // 1. Find or create media item
    var season *int
    if req.Type == MediaTypeTV {
        season = &req.Season
    }

    item, err := d.repo.GetMediaItemBySafeName(ctx, req.SafeName(), season)
    if err != nil {
        return fmt.Errorf("failed to lookup media item: %w", err)
    }

    if item == nil {
        item = &model.MediaItem{
            Type:     model.MediaType(req.Type),
            Name:     req.Name,
            SafeName: req.SafeName(),
            Season:   season,
        }
        if err := d.repo.CreateMediaItem(ctx, item); err != nil {
            return fmt.Errorf("failed to create media item: %w", err)
        }
    }
    d.itemID = item.ID

    // 2. Create job
    var disc *int
    if req.Type == MediaTypeTV && req.Disc > 0 {
        disc = &req.Disc
    }

    job := &model.Job{
        MediaItemID: item.ID,
        Stage:       model.StageRip,
        Status:      model.JobStatusInProgress,
        Disc:        disc,
        OutputDir:   outputDir,
        WorkerID:    hostname(),
        PID:         os.Getpid(),
    }
    if err := d.repo.CreateJob(ctx, job); err != nil {
        return fmt.Errorf("failed to create job: %w", err)
    }
    d.jobID = job.ID

    // 3. Write filesystem state
    if err := d.fs.Initialize(outputDir, req); err != nil {
        // Log warning but don't fail - DB is authoritative
        // TODO: log event
        return nil
    }

    return nil
}

func (d *DualWriteStateManager) Complete(outputDir string) error {
    ctx := context.Background()

    // Update DB first
    if err := d.repo.UpdateJobStatus(ctx, d.jobID, model.JobStatusCompleted, ""); err != nil {
        return fmt.Errorf("failed to update job status: %w", err)
    }

    // Update filesystem
    if err := d.fs.Complete(outputDir); err != nil {
        // Log warning but don't fail
    }

    return nil
}

// SetStatus, SetError similar pattern...
```

**Step 10.3: Run tests**

```bash
go test ./internal/ripper/... -v
```

**Step 10.4: Commit**

```bash
git add internal/ripper/
git commit -m "ripper: add DualWriteStateManager for DB + filesystem writes"
```

---

## Task 11: E2E Integration Test

**Files:**
- Create: `tests/e2e/suites/db_integration_test.go`

**Step 11.1: Write E2E test**

```go
func TestRipper_E2E_DualWrite(t *testing.T) {
    requireFFmpeg(t)
    mockPath := findMockMakeMKV(t)

    env := testenv.New(t)
    dbFixture := testenv.NewDBFixture(t)

    // Create ripper with dual-write state manager
    runner := ripper.NewMakeMKVRunner(mockPath)
    stateManager := ripper.NewDualWriteStateManager(
        ripper.NewStateManager(),
        dbFixture.Repo,
    )
    r := ripper.NewRipperWithStateManager(env.StagingBase, runner, nil, stateManager)

    req := &ripper.RipRequest{
        Type:     ripper.MediaTypeMovie,
        Name:     "Dual Write Test",
        DiscPath: "disc:0",
    }

    result, err := r.Rip(context.Background(), req)
    if err != nil {
        t.Fatalf("Rip failed: %v", err)
    }

    // Verify filesystem
    stateDir, err := testenv.FindStateDir(result.OutputDir, ".rip")
    if err != nil {
        t.Fatal("filesystem state not created")
    }
    stateDir.AssertStatus(t, model.StatusCompleted)

    // Verify database
    item, err := dbFixture.Repo.GetMediaItemBySafeName(
        context.Background(),
        "Dual_Write_Test",
        nil,
    )
    if err != nil || item == nil {
        t.Fatalf("media item not in database: %v", err)
    }

    jobs, _ := dbFixture.Repo.ListJobsForMedia(context.Background(), item.ID)
    if len(jobs) != 1 {
        t.Fatalf("expected 1 job, got %d", len(jobs))
    }
    if jobs[0].Status != model.JobStatusCompleted {
        t.Errorf("job status = %v, want completed", jobs[0].Status)
    }
}
```

**Step 11.2: Run E2E tests**

```bash
make build-mock-makemkv
go test ./tests/e2e/... -v
```

**Step 11.3: Commit**

```bash
git add tests/e2e/
git commit -m "test: add E2E integration test for dual-write ripper"
```

---

## Task 12: Config Package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 12.1: Write test for config loading**

```go
package config

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLoad_FromFile(t *testing.T) {
    dir := t.TempDir()
    configPath := filepath.Join(dir, "config.yaml")

    content := `
data_dir: /mnt/media/pipeline
staging_base: /mnt/media/staging
library_base: /mnt/media/library
dispatch:
  rip: ripper
  remux: analyzer
  transcode: transcoder
  publish: analyzer
`
    os.WriteFile(configPath, []byte(content), 0644)

    cfg, err := Load(configPath)
    if err != nil {
        t.Fatalf("Load() error = %v", err)
    }

    if cfg.DataDir != "/mnt/media/pipeline" {
        t.Errorf("DataDir = %q, want /mnt/media/pipeline", cfg.DataDir)
    }
    if cfg.DatabasePath() != "/mnt/media/pipeline/pipeline.db" {
        t.Errorf("DatabasePath() = %q, want /mnt/media/pipeline/pipeline.db", cfg.DatabasePath())
    }
    if cfg.Dispatch["rip"] != "ripper" {
        t.Errorf("Dispatch[rip] = %q, want ripper", cfg.Dispatch["rip"])
    }
}

func TestConfig_JobLogPath(t *testing.T) {
    cfg := &Config{DataDir: "/mnt/media/pipeline"}

    got := cfg.JobLogPath(123)
    want := "/mnt/media/pipeline/logs/jobs/123/job.log"
    if got != want {
        t.Errorf("JobLogPath(123) = %q, want %q", got, want)
    }
}

func TestConfig_ToolLogPath(t *testing.T) {
    cfg := &Config{DataDir: "/mnt/media/pipeline"}

    got := cfg.ToolLogPath(123, "makemkv")
    want := "/mnt/media/pipeline/logs/jobs/123/makemkv.log"
    if got != want {
        t.Errorf("ToolLogPath(123, makemkv) = %q, want %q", got, want)
    }
}

func TestConfig_DispatchTarget(t *testing.T) {
    cfg := &Config{
        Dispatch: map[string]string{
            "rip": "ripper",
            "remux": "",  // empty = local
        },
    }

    if target := cfg.DispatchTarget("rip"); target != "ripper" {
        t.Errorf("DispatchTarget(rip) = %q, want ripper", target)
    }
    if target := cfg.DispatchTarget("remux"); target != "" {
        t.Errorf("DispatchTarget(remux) = %q, want empty", target)
    }
}
```

**Step 12.2: Implement config.go**

```go
package config

import (
    "fmt"
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

// Config holds application configuration
type Config struct {
    DataDir     string            `yaml:"data_dir"`      // App data: db, logs
    StagingBase string            `yaml:"staging_base"`
    LibraryBase string            `yaml:"library_base"`
    Dispatch    map[string]string `yaml:"dispatch"`
}

// DatabasePath returns the path to the SQLite database
func (c *Config) DatabasePath() string {
    return filepath.Join(c.DataDir, "pipeline.db")
}

// JobLogDir returns the directory for a job's log files
func (c *Config) JobLogDir(jobID int64) string {
    return filepath.Join(c.DataDir, "logs", "jobs", fmt.Sprintf("%d", jobID))
}

// JobLogPath returns the path for a job's main log file
func (c *Config) JobLogPath(jobID int64) string {
    return filepath.Join(c.JobLogDir(jobID), "job.log")
}

// ToolLogPath returns the path for a tool's raw log file
func (c *Config) ToolLogPath(jobID int64, tool string) string {
    return filepath.Join(c.JobLogDir(jobID), fmt.Sprintf("%s.log", tool))
}

// EnsureJobLogDir creates the log directory for a specific job
func (c *Config) EnsureJobLogDir(jobID int64) error {
    return os.MkdirAll(c.JobLogDir(jobID), 0755)
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config: %w", err)
    }

    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }

    return &cfg, nil
}

// LoadDefault loads config from default location
func LoadDefault() (*Config, error) {
    // Check XDG config dir first
    if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
        path := filepath.Join(xdg, "media-pipeline", "config.yaml")
        if _, err := os.Stat(path); err == nil {
            return Load(path)
        }
    }

    // Fall back to ~/.config
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("failed to get home dir: %w", err)
    }

    path := filepath.Join(home, ".config", "media-pipeline", "config.yaml")
    return Load(path)
}

// DispatchTarget returns the SSH target for a stage, or empty for local execution
func (c *Config) DispatchTarget(stage string) string {
    if c.Dispatch == nil {
        return ""
    }
    return c.Dispatch[stage]
}

// IsLocal returns true if the stage should run locally (no SSH)
func (c *Config) IsLocal(stage string) bool {
    return c.DispatchTarget(stage) == ""
}
```

**Step 12.3: Run tests**

```bash
go get gopkg.in/yaml.v3
go test ./internal/config/... -v
```

**Step 12.4: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "config: add YAML config package with dispatch targets"
```

---

## Task 13: Logging Package

**Files:**
- Create: `internal/logging/logger.go`
- Create: `internal/logging/logger_test.go`

**Step 13.1: Write test for logger**

```go
package logging

import (
    "bytes"
    "strings"
    "testing"
)

func TestLogger_WritesToMultipleDestinations(t *testing.T) {
    var stdout, file bytes.Buffer

    logger := New(Options{
        Stdout:   &stdout,
        File:     &file,
        MinLevel: LevelInfo,
    })

    logger.Info("test message")

    if !strings.Contains(stdout.String(), "test message") {
        t.Error("stdout missing message")
    }
    if !strings.Contains(file.String(), "test message") {
        t.Error("file missing message")
    }
}

func TestLogger_RespectsMinLevel(t *testing.T) {
    var buf bytes.Buffer

    logger := New(Options{
        Stdout:   &buf,
        MinLevel: LevelWarn,
    })

    logger.Debug("debug msg")
    logger.Info("info msg")
    logger.Warn("warn msg")

    output := buf.String()
    if strings.Contains(output, "debug msg") {
        t.Error("debug should be filtered")
    }
    if strings.Contains(output, "info msg") {
        t.Error("info should be filtered")
    }
    if !strings.Contains(output, "warn msg") {
        t.Error("warn should be included")
    }
}

func TestLogger_ProgressLevel(t *testing.T) {
    var buf bytes.Buffer

    logger := New(Options{
        Stdout:   &buf,
        MinLevel: LevelProgress,
    })

    logger.Progress("50%% complete")

    if !strings.Contains(buf.String(), "[PROGRESS]") {
        t.Error("progress level not formatted correctly")
    }
}
```

**Step 13.2: Implement logger.go**

```go
package logging

import (
    "fmt"
    "io"
    "os"
    "sync"
    "time"
)

type Level int

const (
    LevelDebug Level = iota
    LevelInfo
    LevelProgress
    LevelWarn
    LevelError
)

func (l Level) String() string {
    switch l {
    case LevelDebug:
        return "DEBUG"
    case LevelInfo:
        return "INFO"
    case LevelProgress:
        return "PROGRESS"
    case LevelWarn:
        return "WARN"
    case LevelError:
        return "ERROR"
    default:
        return "UNKNOWN"
    }
}

type Logger struct {
    mu       sync.Mutex
    stdout   io.Writer
    file     io.Writer
    minLevel Level

    // For DB event logging
    eventFn  func(level, msg string)
}

type Options struct {
    Stdout   io.Writer  // nil = no stdout
    File     io.Writer  // nil = no file
    MinLevel Level
    EventFn  func(level, msg string)  // Called for significant events
}

func New(opts Options) *Logger {
    return &Logger{
        stdout:   opts.Stdout,
        file:     opts.File,
        minLevel: opts.MinLevel,
        eventFn:  opts.EventFn,
    }
}

// NewForJob creates a logger configured for a job execution
func NewForJob(logPath string, stdout bool, eventFn func(level, msg string)) (*Logger, error) {
    var stdoutWriter io.Writer
    if stdout {
        stdoutWriter = os.Stdout
    }

    var fileWriter io.Writer
    if logPath != "" {
        f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
        if err != nil {
            return nil, fmt.Errorf("failed to open log file: %w", err)
        }
        fileWriter = f
    }

    return New(Options{
        Stdout:   stdoutWriter,
        File:     fileWriter,
        MinLevel: LevelInfo,
        EventFn:  eventFn,
    }), nil
}

func (l *Logger) log(level Level, msg string, args ...any) {
    if level < l.minLevel {
        return
    }

    formatted := fmt.Sprintf(msg, args...)
    line := fmt.Sprintf("%s [%s] %s\n",
        time.Now().Format("2006-01-02 15:04:05"),
        level.String(),
        formatted,
    )

    l.mu.Lock()
    defer l.mu.Unlock()

    if l.stdout != nil {
        l.stdout.Write([]byte(line))
    }
    if l.file != nil {
        l.file.Write([]byte(line))
    }
}

func (l *Logger) Debug(msg string, args ...any)    { l.log(LevelDebug, msg, args...) }
func (l *Logger) Info(msg string, args ...any)     { l.log(LevelInfo, msg, args...) }
func (l *Logger) Progress(msg string, args ...any) { l.log(LevelProgress, msg, args...) }
func (l *Logger) Warn(msg string, args ...any)     { l.log(LevelWarn, msg, args...) }
func (l *Logger) Error(msg string, args ...any)    { l.log(LevelError, msg, args...) }

// Event logs a significant event to file AND DB (if configured)
func (l *Logger) Event(level Level, msg string) {
    l.log(level, msg)
    if l.eventFn != nil {
        l.eventFn(level.String(), msg)
    }
}
```

**Step 13.3: Run tests**

```bash
go test ./internal/logging/... -v
```

**Step 13.4: Commit**

```bash
git add internal/logging/
git commit -m "logging: add multi-destination logger with level filtering"
```

---

## Task 14: Update Ripper CLI for Dual-Mode

**Files:**
- Modify: `cmd/ripper/main.go`

**Step 14.1: Update CLI to support -job-id and -db flags**

The ripper CLI should support:
- Existing: `-t movie -n "Name"` (standalone mode)
- New: `-job-id 123 -db /path/to/db` (TUI dispatch mode)
- New: `-db /path/to/db` with existing args (standalone with DB tracking)

```go
// Add new flags
var (
    jobID  = flag.Int64("job-id", 0, "Job ID to resume (TUI dispatch mode)")
    dbPath = flag.String("db", "", "Path to SQLite database")
)

func main() {
    flag.Parse()

    // Determine mode
    if *jobID > 0 {
        // TUI dispatch mode: load job from DB
        runFromJob(*jobID, *dbPath)
    } else {
        // Standalone mode: create job if DB specified
        runStandalone(*dbPath)
    }
}
```

**Step 14.2: Run tests**

```bash
go test ./... -v
make build-all
```

**Step 14.3: Commit**

```bash
git add cmd/ripper/
git commit -m "ripper: add -job-id and -db flags for dual-mode operation"
```

---

## Task 15: Final Verification

**Step 15.1: Run all tests**

```bash
make test
```

**Step 15.2: Run E2E tests**

```bash
make build-mock-makemkv
go test ./tests/e2e/... -v
```

**Step 15.3: Verify build**

```bash
make build-all
```

**Step 15.4: Test CLI dual-mode manually**

```bash
# Standalone mode (no DB)
./bin/ripper -t movie -n "Test Movie"

# Standalone mode with DB tracking
./bin/ripper -t movie -n "Test Movie" -db /tmp/test.db

# Verify DB has record
sqlite3 /tmp/test.db "SELECT * FROM jobs"
```

---

## Summary

| Task | Description | Dependencies |
|------|-------------|--------------|
| 1 | Add SQLite dependency | None |
| 2 | Update Stage enum | None |
| 3 | Fix compilation from Stage rename | Task 2 |
| 4 | Change Season to *int | Task 2 |
| 5 | Create DB package structure | Task 1 |
| 6 | Add Job/LogEvent types | Task 2 |
| 7 | Implement Repository | Tasks 5, 6 |
| 8 | Create Organization Validator | None |
| 9 | Add DB fixture to testenv | Task 7 |
| 10 | Implement DualWriteStateManager | Tasks 7, 9 |
| 11 | E2E integration test | Task 10 |
| 12 | Config package | None |
| 13 | Logging package | None |
| 14 | Update Ripper CLI for dual-mode | Tasks 7, 12, 13 |
| 15 | Final verification | All |

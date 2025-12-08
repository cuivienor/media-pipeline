# Item-Centric UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor the TUI from a pipeline-centric view (stages with items) to an item-centric view (items with their pipeline journey).

**Architecture:** The main view becomes a list of media items (movies/TV shows) grouped by status. TV shows contain seasons as nested entities. Drilling into an item shows current state + next action. Movies and seasons move through pipeline stages independently.

**Tech Stack:** Go, Bubbletea TUI framework, SQLite database

---

## Phase 1: Database Schema Changes

### Task 1: Create migration for new schema

**Files:**
- Create: `internal/db/migrations/002_item_centric.sql`

**Step 1: Write the migration SQL**

```sql
-- File: internal/db/migrations/002_item_centric.sql

-- Add status column to media_items for item-level status (not_started, active, completed)
ALTER TABLE media_items ADD COLUMN status TEXT NOT NULL DEFAULT 'not_started' CHECK (status IN ('not_started', 'active', 'completed'));

-- Create seasons table for TV shows
CREATE TABLE IF NOT EXISTS seasons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id INTEGER NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    number INTEGER NOT NULL,
    current_stage TEXT NOT NULL DEFAULT 'rip' CHECK (current_stage IN ('rip', 'organize', 'remux', 'transcode', 'publish')),
    stage_status TEXT NOT NULL DEFAULT 'pending' CHECK (stage_status IN ('pending', 'in_progress', 'completed', 'failed')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(item_id, number)
);

CREATE INDEX IF NOT EXISTS idx_seasons_item ON seasons(item_id);

-- Add season_id to jobs table (nullable - NULL for movies, set for TV)
ALTER TABLE jobs ADD COLUMN season_id INTEGER REFERENCES seasons(id) ON DELETE CASCADE;

-- Migrate existing TV show data: create seasons from media_items with season != NULL
INSERT INTO seasons (item_id, number, created_at, updated_at)
SELECT id, season, created_at, updated_at
FROM media_items
WHERE season IS NOT NULL;

-- Update jobs to reference the new seasons
UPDATE jobs
SET season_id = (
    SELECT s.id FROM seasons s
    JOIN media_items m ON s.item_id = m.id AND s.number = m.season
    WHERE m.id = jobs.media_item_id AND m.season IS NOT NULL
)
WHERE media_item_id IN (SELECT id FROM media_items WHERE season IS NOT NULL);

-- Update media_items status based on existing jobs
UPDATE media_items SET status = 'active'
WHERE id IN (SELECT DISTINCT media_item_id FROM jobs);

-- Update media_items status to completed if latest job is publish + completed
UPDATE media_items SET status = 'completed'
WHERE id IN (
    SELECT media_item_id FROM jobs
    WHERE stage = 'publish' AND status = 'completed'
);

-- For TV shows, consolidate to single row per show (remove season from media_items)
-- First, update jobs to point to the "parent" item (lowest ID for same safe_name)
-- This is complex, so we'll handle TV show consolidation in application code during migration

-- Remove season column from media_items (SQLite doesn't support DROP COLUMN easily)
-- We'll handle this by creating a new table and copying data

-- Create new media_items table without season column
CREATE TABLE media_items_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK (type IN ('movie', 'tv')),
    name TEXT NOT NULL,
    safe_name TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'not_started' CHECK (status IN ('not_started', 'active', 'completed')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- For movies, copy directly
INSERT INTO media_items_new (id, type, name, safe_name, status, created_at, updated_at)
SELECT id, type, name, safe_name, status, created_at, updated_at
FROM media_items
WHERE type = 'movie';

-- For TV shows, insert one row per unique safe_name (the earliest created one)
INSERT INTO media_items_new (id, type, name, safe_name, status, created_at, updated_at)
SELECT MIN(id), type, name, safe_name,
       CASE WHEN EXISTS (SELECT 1 FROM jobs j WHERE j.media_item_id = media_items.id) THEN 'active' ELSE 'not_started' END,
       MIN(created_at), MAX(updated_at)
FROM media_items
WHERE type = 'tv'
GROUP BY safe_name;

-- Update seasons to point to the consolidated TV show item
UPDATE seasons SET item_id = (
    SELECT mn.id FROM media_items_new mn
    JOIN media_items m ON mn.safe_name = m.safe_name
    WHERE m.id = (SELECT item_id FROM seasons WHERE seasons.id = seasons.id LIMIT 1)
);

-- This migration is getting complex. Let's simplify by doing a two-phase approach:
-- Phase 1: Add new columns/tables
-- Phase 2: Application code handles data migration on first run
```

**WAIT - This migration is too complex for a single SQL file. Let me revise the approach.**

**Step 1 (revised): Write a simpler migration that adds new structures**

```sql
-- File: internal/db/migrations/002_item_centric.sql

-- Add status column to media_items for item-level status
-- Using ALTER TABLE which SQLite supports
ALTER TABLE media_items ADD COLUMN status TEXT DEFAULT 'active' CHECK (status IN ('not_started', 'active', 'completed'));

-- Create seasons table for TV shows
CREATE TABLE IF NOT EXISTS seasons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id INTEGER NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    number INTEGER NOT NULL,
    current_stage TEXT NOT NULL DEFAULT 'rip' CHECK (current_stage IN ('rip', 'organize', 'remux', 'transcode', 'publish')),
    stage_status TEXT NOT NULL DEFAULT 'pending' CHECK (stage_status IN ('pending', 'in_progress', 'completed', 'failed')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(item_id, number)
);

CREATE INDEX IF NOT EXISTS idx_seasons_item ON seasons(item_id);
CREATE INDEX IF NOT EXISTS idx_seasons_status ON seasons(stage_status);

-- Add season_id to jobs table (nullable - NULL for movies, set for TV)
ALTER TABLE jobs ADD COLUMN season_id INTEGER REFERENCES seasons(id) ON DELETE CASCADE;

-- Add current_stage and stage_status to media_items for movies (TV uses seasons table)
ALTER TABLE media_items ADD COLUMN current_stage TEXT DEFAULT 'rip' CHECK (current_stage IN ('rip', 'organize', 'remux', 'transcode', 'publish'));
ALTER TABLE media_items ADD COLUMN stage_status TEXT DEFAULT 'pending' CHECK (stage_status IN ('pending', 'in_progress', 'completed', 'failed'));
```

**Step 2: Verify migration works**

Run: `go test ./internal/db/... -v -run TestMigrations`
Expected: PASS (or create this test if it doesn't exist)

**Step 3: Commit**

```bash
git add internal/db/migrations/002_item_centric.sql
git commit -m "db: add item-centric schema migration"
```

---

### Task 2: Update model types

**Files:**
- Modify: `internal/model/media.go`
- Create: `internal/model/season.go`

**Step 1: Create Season model**

Create file `internal/model/season.go`:

```go
package model

import "time"

// Season represents a TV show season that moves through the pipeline
type Season struct {
    ID           int64
    ItemID       int64      // Foreign key to Item (TV show)
    Number       int        // Season number (1, 2, 3...)
    CurrentStage Stage      // Current pipeline stage
    StageStatus  Status     // Status of current stage
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// IsReadyForNextStage returns true if the season has completed its current stage
func (s *Season) IsReadyForNextStage() bool {
    return s.StageStatus == StatusCompleted && s.CurrentStage != StagePublish
}

// IsFailed returns true if the season is in a failed state
func (s *Season) IsFailed() bool {
    return s.StageStatus == StatusFailed
}

// IsInProgress returns true if the season is currently being processed
func (s *Season) IsInProgress() bool {
    return s.StageStatus == StatusInProgress
}

// IsComplete returns true if the season has been published
func (s *Season) IsComplete() bool {
    return s.CurrentStage == StagePublish && s.StageStatus == StatusCompleted
}

// NextAction returns a human-readable description of what needs to happen next
func (s *Season) NextAction() string {
    if s.StageStatus == StatusFailed {
        return "retry " + s.CurrentStage.String()
    }
    if s.StageStatus == StatusInProgress {
        return s.CurrentStage.String() + " in progress"
    }
    if s.StageStatus == StatusCompleted {
        return s.CurrentStage.NextStage().String()
    }
    return "start " + s.CurrentStage.String()
}
```

**Step 2: Update MediaItem model**

Modify `internal/model/media.go` to add item-level fields:

```go
// Add these fields to MediaItem struct (around line 114):

// ItemStatus represents the overall status of an item
type ItemStatus string

const (
    ItemStatusNotStarted ItemStatus = "not_started"
    ItemStatusActive     ItemStatus = "active"
    ItemStatusCompleted  ItemStatus = "completed"
)

// MediaItem represents a single media item (movie or TV show)
type MediaItem struct {
    ID       int64
    Type     MediaType
    Name     string
    SafeName string

    // Item-level status
    ItemStatus ItemStatus

    // For Movies: pipeline state lives here
    CurrentStage Stage  // Only used for movies
    StageStatus  Status // Only used for movies

    // For TV Shows: seasons contain the pipeline state
    Seasons []Season // Populated for TV shows

    // Legacy fields (to be removed after migration)
    Season  *int        // DEPRECATED: Season number for TV
    Stages  []StageInfo // DEPRECATED: History of stages
    Current Stage       // DEPRECATED: Use CurrentStage
    Status  Status      // DEPRECATED: Use StageStatus
}
```

**Step 3: Run tests**

Run: `go test ./internal/model/... -v`
Expected: PASS (some tests may need updates)

**Step 4: Commit**

```bash
git add internal/model/media.go internal/model/season.go
git commit -m "model: add Season type and update MediaItem for item-centric design"
```

---

### Task 3: Update repository interface and implementation

**Files:**
- Modify: `internal/db/repository.go`
- Modify: `internal/db/sqlite.go`

**Step 1: Add Season methods to repository interface**

Add to `internal/db/repository.go`:

```go
// Add these methods to the Repository interface:

    // Seasons
    CreateSeason(ctx context.Context, season *model.Season) error
    GetSeason(ctx context.Context, id int64) (*model.Season, error)
    ListSeasonsForItem(ctx context.Context, itemID int64) ([]model.Season, error)
    UpdateSeason(ctx context.Context, season *model.Season) error
    UpdateSeasonStage(ctx context.Context, id int64, stage model.Stage, status model.Status) error

    // Updated item methods
    UpdateMediaItemStatus(ctx context.Context, id int64, status model.ItemStatus) error
    ListActiveItems(ctx context.Context) ([]model.MediaItem, error)
```

**Step 2: Implement Season methods in SQLite repository**

Add to `internal/db/sqlite.go`:

```go
// CreateSeason creates a new season
func (r *SQLiteRepository) CreateSeason(ctx context.Context, season *model.Season) error {
    query := `
        INSERT INTO seasons (item_id, number, current_stage, stage_status, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?)
    `
    now := time.Now().UTC().Format(time.RFC3339)
    result, err := r.db.db.ExecContext(ctx, query,
        season.ItemID,
        season.Number,
        season.CurrentStage.String(),
        season.StageStatus,
        now,
        now,
    )
    if err != nil {
        return fmt.Errorf("failed to insert season: %w", err)
    }
    id, err := result.LastInsertId()
    if err != nil {
        return fmt.Errorf("failed to get last insert id: %w", err)
    }
    season.ID = id
    return nil
}

// GetSeason retrieves a season by ID
func (r *SQLiteRepository) GetSeason(ctx context.Context, id int64) (*model.Season, error) {
    query := `
        SELECT id, item_id, number, current_stage, stage_status, created_at, updated_at
        FROM seasons
        WHERE id = ?
    `
    var season model.Season
    var stageStr, statusStr string
    var createdAt, updatedAt string

    err := r.db.db.QueryRowContext(ctx, query, id).Scan(
        &season.ID,
        &season.ItemID,
        &season.Number,
        &stageStr,
        &statusStr,
        &createdAt,
        &updatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get season: %w", err)
    }

    season.CurrentStage = parseStage(stageStr)
    season.StageStatus = model.Status(statusStr)
    season.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
    season.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

    return &season, nil
}

// ListSeasonsForItem lists all seasons for a TV show item
func (r *SQLiteRepository) ListSeasonsForItem(ctx context.Context, itemID int64) ([]model.Season, error) {
    query := `
        SELECT id, item_id, number, current_stage, stage_status, created_at, updated_at
        FROM seasons
        WHERE item_id = ?
        ORDER BY number ASC
    `
    rows, err := r.db.db.QueryContext(ctx, query, itemID)
    if err != nil {
        return nil, fmt.Errorf("failed to list seasons: %w", err)
    }
    defer rows.Close()

    var seasons []model.Season
    for rows.Next() {
        var season model.Season
        var stageStr, statusStr string
        var createdAt, updatedAt string

        err := rows.Scan(
            &season.ID,
            &season.ItemID,
            &season.Number,
            &stageStr,
            &statusStr,
            &createdAt,
            &updatedAt,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan season: %w", err)
        }

        season.CurrentStage = parseStage(stageStr)
        season.StageStatus = model.Status(statusStr)
        season.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
        season.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

        seasons = append(seasons, season)
    }

    return seasons, rows.Err()
}

// UpdateSeason updates a season
func (r *SQLiteRepository) UpdateSeason(ctx context.Context, season *model.Season) error {
    query := `
        UPDATE seasons
        SET current_stage = ?, stage_status = ?, updated_at = ?
        WHERE id = ?
    `
    now := time.Now().UTC().Format(time.RFC3339)
    _, err := r.db.db.ExecContext(ctx, query,
        season.CurrentStage.String(),
        season.StageStatus,
        now,
        season.ID,
    )
    if err != nil {
        return fmt.Errorf("failed to update season: %w", err)
    }
    return nil
}

// UpdateSeasonStage updates a season's stage and status
func (r *SQLiteRepository) UpdateSeasonStage(ctx context.Context, id int64, stage model.Stage, status model.Status) error {
    query := `
        UPDATE seasons
        SET current_stage = ?, stage_status = ?, updated_at = ?
        WHERE id = ?
    `
    now := time.Now().UTC().Format(time.RFC3339)
    _, err := r.db.db.ExecContext(ctx, query, stage.String(), status, now, id)
    if err != nil {
        return fmt.Errorf("failed to update season stage: %w", err)
    }
    return nil
}

// UpdateMediaItemStatus updates an item's overall status
func (r *SQLiteRepository) UpdateMediaItemStatus(ctx context.Context, id int64, status model.ItemStatus) error {
    query := `UPDATE media_items SET status = ?, updated_at = ? WHERE id = ?`
    now := time.Now().UTC().Format(time.RFC3339)
    _, err := r.db.db.ExecContext(ctx, query, status, now, id)
    if err != nil {
        return fmt.Errorf("failed to update media item status: %w", err)
    }
    return nil
}

// ListActiveItems lists all items that are not completed
func (r *SQLiteRepository) ListActiveItems(ctx context.Context) ([]model.MediaItem, error) {
    query := `
        SELECT id, type, name, safe_name, status, current_stage, stage_status, created_at, updated_at
        FROM media_items
        WHERE status != 'completed'
        ORDER BY updated_at DESC
    `
    rows, err := r.db.db.QueryContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("failed to list active items: %w", err)
    }
    defer rows.Close()

    var items []model.MediaItem
    for rows.Next() {
        var item model.MediaItem
        var stageStr, stageStatusStr sql.NullString
        var createdAt, updatedAt string

        err := rows.Scan(
            &item.ID,
            &item.Type,
            &item.Name,
            &item.SafeName,
            &item.ItemStatus,
            &stageStr,
            &stageStatusStr,
            &createdAt,
            &updatedAt,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan media item: %w", err)
        }

        if stageStr.Valid {
            item.CurrentStage = parseStage(stageStr.String)
        }
        if stageStatusStr.Valid {
            item.StageStatus = model.Status(stageStatusStr.String)
        }

        items = append(items, item)
    }

    return items, rows.Err()
}
```

**Step 3: Run tests**

Run: `go test ./internal/db/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/db/repository.go internal/db/sqlite.go
git commit -m "db: add Season repository methods and item status updates"
```

---

## Phase 2: TUI State Management

### Task 4: Create new TUI state loader

**Files:**
- Modify: `internal/tui/state.go`

**Step 1: Update state loading to use new model**

Replace the content of `internal/tui/state.go`:

```go
package tui

import (
    "context"
    "fmt"

    "github.com/cuivienor/media-pipeline/internal/db"
    "github.com/cuivienor/media-pipeline/internal/model"
)

// AppState holds the current application state
type AppState struct {
    Items []model.MediaItem
    Jobs  map[int64][]model.Job // itemID or seasonID -> jobs
}

// LoadState loads application state from the database
func LoadState(repo db.Repository) (*AppState, error) {
    ctx := context.Background()

    items, err := repo.ListActiveItems(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to list active items: %w", err)
    }

    state := &AppState{
        Items: items,
        Jobs:  make(map[int64][]model.Job),
    }

    // Load seasons for TV shows, jobs for all
    for i := range items {
        item := &items[i]

        if item.Type == model.MediaTypeTV {
            seasons, err := repo.ListSeasonsForItem(ctx, item.ID)
            if err != nil {
                return nil, fmt.Errorf("failed to list seasons for %s: %w", item.Name, err)
            }
            item.Seasons = seasons

            // Load jobs for each season
            for _, season := range seasons {
                jobs, err := repo.ListJobsForMedia(ctx, item.ID)
                if err != nil {
                    return nil, fmt.Errorf("failed to list jobs for season: %w", err)
                }
                // Filter to jobs for this season
                var seasonJobs []model.Job
                for _, job := range jobs {
                    // TODO: After job.SeasonID is populated, filter by it
                    seasonJobs = append(seasonJobs, job)
                }
                state.Jobs[season.ID] = seasonJobs
            }
        } else {
            // Movie - load jobs directly
            jobs, err := repo.ListJobsForMedia(ctx, item.ID)
            if err != nil {
                return nil, fmt.Errorf("failed to list jobs for %s: %w", item.Name, err)
            }
            state.Jobs[item.ID] = jobs

            // Update movie's current stage from jobs
            if len(jobs) > 0 {
                latestJob := jobs[len(jobs)-1]
                item.CurrentStage = latestJob.Stage
                item.StageStatus = jobStatusToStatus(latestJob.Status)
            }
        }
    }

    return state, nil
}

// ItemsNeedingAction returns items/seasons that need user action
func (s *AppState) ItemsNeedingAction() []model.MediaItem {
    var result []model.MediaItem
    for _, item := range s.Items {
        if item.Type == model.MediaTypeMovie {
            if item.StageStatus == model.StatusCompleted && item.CurrentStage != model.StagePublish {
                result = append(result, item)
            }
        }
        // TV shows with any season needing action are included
        // (handled in display logic)
    }
    return result
}

// ItemsInProgress returns items/seasons currently being processed
func (s *AppState) ItemsInProgress() []model.MediaItem {
    var result []model.MediaItem
    for _, item := range s.Items {
        if item.Type == model.MediaTypeMovie {
            if item.StageStatus == model.StatusInProgress {
                result = append(result, item)
            }
        }
    }
    return result
}

// ItemsFailed returns items/seasons in a failed state
func (s *AppState) ItemsFailed() []model.MediaItem {
    var result []model.MediaItem
    for _, item := range s.Items {
        if item.Type == model.MediaTypeMovie {
            if item.StageStatus == model.StatusFailed {
                result = append(result, item)
            }
        }
    }
    return result
}

// jobStatusToStatus converts JobStatus to Status
func jobStatusToStatus(js model.JobStatus) model.Status {
    switch js {
    case model.JobStatusCompleted:
        return model.StatusCompleted
    case model.JobStatusInProgress:
        return model.StatusInProgress
    case model.JobStatusFailed:
        return model.StatusFailed
    default:
        return model.StatusPending
    }
}
```

**Step 2: Run tests**

Run: `go test ./internal/tui/... -v`
Expected: Some tests may fail - update them as needed

**Step 3: Commit**

```bash
git add internal/tui/state.go
git commit -m "tui: update state loading for item-centric model"
```

---

## Phase 3: New TUI Views

### Task 5: Create Item List view (main view)

**Files:**
- Create: `internal/tui/itemlist.go`
- Modify: `internal/tui/app.go`

**Step 1: Create the item list view**

Create `internal/tui/itemlist.go`:

```go
package tui

import (
    "fmt"
    "strings"

    "github.com/charmbracelet/lipgloss"
    "github.com/cuivienor/media-pipeline/internal/model"
)

// renderItemList renders the main item list view
func (a *App) renderItemList() string {
    var b strings.Builder

    b.WriteString(titleStyle.Render("Media Pipeline"))
    b.WriteString("\n\n")

    if a.state == nil || len(a.state.Items) == 0 {
        b.WriteString(mutedItemStyle.Render("No active items. Press [n] to add one."))
        b.WriteString("\n\n")
        b.WriteString(helpStyle.Render("[n] New Item  [h] History  [q] Quit"))
        return b.String()
    }

    // Group items by status
    needsAction := a.filterItemsByStatus(model.StatusCompleted)
    inProgress := a.filterItemsByStatus(model.StatusInProgress)
    failed := a.filterItemsByStatus(model.StatusFailed)
    notStarted := a.filterItemsByItemStatus(model.ItemStatusNotStarted)

    cursorIndex := 0

    // Needs Action section
    if len(needsAction) > 0 {
        b.WriteString(sectionHeaderStyle.Render("NEEDS ACTION"))
        b.WriteString("\n")
        for _, item := range needsAction {
            selected := cursorIndex == a.cursor
            b.WriteString(a.renderItemRow(item, selected))
            b.WriteString("\n")
            cursorIndex++
        }
        b.WriteString("\n")
    }

    // In Progress section
    if len(inProgress) > 0 {
        b.WriteString(sectionHeaderStyle.Render("IN PROGRESS"))
        b.WriteString("\n")
        for _, item := range inProgress {
            selected := cursorIndex == a.cursor
            b.WriteString(a.renderItemRow(item, selected))
            b.WriteString("\n")
            cursorIndex++
        }
        b.WriteString("\n")
    }

    // Failed section
    if len(failed) > 0 {
        b.WriteString(sectionHeaderStyle.Render("FAILED"))
        b.WriteString("\n")
        for _, item := range failed {
            selected := cursorIndex == a.cursor
            b.WriteString(a.renderItemRow(item, selected))
            b.WriteString("\n")
            cursorIndex++
        }
        b.WriteString("\n")
    }

    // Not Started section
    if len(notStarted) > 0 {
        b.WriteString(sectionHeaderStyle.Render("NOT STARTED"))
        b.WriteString("\n")
        for _, item := range notStarted {
            selected := cursorIndex == a.cursor
            b.WriteString(a.renderItemRow(item, selected))
            b.WriteString("\n")
            cursorIndex++
        }
        b.WriteString("\n")
    }

    b.WriteString(helpStyle.Render("[Enter] View  [n] New Item  [r] Refresh  [h] History  [q] Quit"))

    return b.String()
}

// renderItemRow renders a single item row
func (a *App) renderItemRow(item model.MediaItem, selected bool) string {
    prefix := "  "
    if selected {
        prefix = "> "
    }

    // Status indicator
    var statusIcon string
    var statusStyle lipgloss.Style
    switch item.StageStatus {
    case model.StatusCompleted:
        statusIcon = "●"
        statusStyle = lipgloss.NewStyle().Foreground(colorSuccess)
    case model.StatusInProgress:
        statusIcon = "◐"
        statusStyle = lipgloss.NewStyle().Foreground(colorWarning)
    case model.StatusFailed:
        statusIcon = "✗"
        statusStyle = lipgloss.NewStyle().Foreground(colorError)
    default:
        statusIcon = "○"
        statusStyle = lipgloss.NewStyle().Foreground(colorMuted)
    }

    // Type badge
    typeBadge := "[M]"
    if item.Type == model.MediaTypeTV {
        typeBadge = "[TV]"
    }

    // Build row
    name := item.Name
    if item.Type == model.MediaTypeTV && len(item.Seasons) > 0 {
        name = fmt.Sprintf("%s (%d seasons)", item.Name, len(item.Seasons))
    }

    // Next action hint
    var actionHint string
    if item.Type == model.MediaTypeMovie {
        if item.StageStatus == model.StatusCompleted && item.CurrentStage != model.StagePublish {
            actionHint = mutedItemStyle.Render(fmt.Sprintf(" → %s", item.CurrentStage.NextAction()))
        } else if item.StageStatus == model.StatusInProgress {
            actionHint = mutedItemStyle.Render(fmt.Sprintf(" [%s]", item.CurrentStage.String()))
        }
    }

    return fmt.Sprintf("%s%s %s %s%s",
        prefix,
        statusStyle.Render(statusIcon),
        typeBadge,
        name,
        actionHint,
    )
}

// filterItemsByStatus returns items with the given stage status (for movies)
func (a *App) filterItemsByStatus(status model.Status) []model.MediaItem {
    var result []model.MediaItem
    for _, item := range a.state.Items {
        if item.Type == model.MediaTypeMovie && item.StageStatus == status {
            result = append(result, item)
        }
        // For TV shows, check if any season matches
        if item.Type == model.MediaTypeTV {
            for _, season := range item.Seasons {
                if season.StageStatus == status {
                    result = append(result, item)
                    break
                }
            }
        }
    }
    return result
}

// filterItemsByItemStatus returns items with the given item status
func (a *App) filterItemsByItemStatus(status model.ItemStatus) []model.MediaItem {
    var result []model.MediaItem
    for _, item := range a.state.Items {
        if item.ItemStatus == status {
            result = append(result, item)
        }
    }
    return result
}
```

**Step 2: Update app.go View enum and routing**

In `internal/tui/app.go`, update the View constants and View() method:

```go
// Update View constants (around line 14):
const (
    ViewItemList View = iota  // NEW: Main view
    ViewItemDetail            // NEW: Movie or TV show detail
    ViewSeasonDetail          // NEW: Season detail for TV
    ViewOrganize
    ViewNewItem               // Renamed from ViewNewRip
    // Remove: ViewOverview, ViewStageList, ViewActionNeeded
)
```

**Step 3: Run tests**

Run: `go build ./...`
Expected: Build succeeds (tests may need updates)

**Step 4: Commit**

```bash
git add internal/tui/itemlist.go internal/tui/app.go
git commit -m "tui: add item list view as main view"
```

---

### Task 6: Create Item Detail view

**Files:**
- Create: `internal/tui/itemdetail_new.go`

**Step 1: Create the item detail view**

Create `internal/tui/itemdetail_new.go`:

```go
package tui

import (
    "fmt"
    "strings"

    "github.com/charmbracelet/lipgloss"
    "github.com/cuivienor/media-pipeline/internal/model"
)

// renderItemDetailNew renders the detail view for a movie or TV show
func (a *App) renderItemDetailNew() string {
    if a.selectedItem == nil {
        return "No item selected"
    }

    item := a.selectedItem

    if item.Type == model.MediaTypeTV {
        return a.renderTVShowDetail(item)
    }
    return a.renderMovieDetail(item)
}

// renderMovieDetail renders detail view for a movie
func (a *App) renderMovieDetail(item *model.MediaItem) string {
    var b strings.Builder

    // Title
    b.WriteString(titleStyle.Render(item.Name))
    b.WriteString("\n")
    b.WriteString(mutedItemStyle.Render("Movie"))
    b.WriteString("\n\n")

    // Current State
    b.WriteString(sectionHeaderStyle.Render("CURRENT STATE"))
    b.WriteString("\n")

    stageStyle := lipgloss.NewStyle()
    switch item.StageStatus {
    case model.StatusCompleted:
        stageStyle = stageStyle.Foreground(colorSuccess)
    case model.StatusInProgress:
        stageStyle = stageStyle.Foreground(colorWarning)
    case model.StatusFailed:
        stageStyle = stageStyle.Foreground(colorError)
    }

    b.WriteString(fmt.Sprintf("  Stage: %s\n", item.CurrentStage.DisplayName()))
    b.WriteString(fmt.Sprintf("  Status: %s\n", stageStyle.Render(string(item.StageStatus))))
    b.WriteString("\n")

    // Next Action
    if item.StageStatus == model.StatusCompleted && item.CurrentStage != model.StagePublish {
        b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
        b.WriteString("\n")
        nextStage := item.CurrentStage.NextStage()
        b.WriteString(fmt.Sprintf("  Press [Enter] to %s\n", nextStage.String()))
        b.WriteString("\n")
    } else if item.StageStatus == model.StatusFailed {
        b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
        b.WriteString("\n")
        b.WriteString("  Press [Enter] to retry\n")
        b.WriteString("\n")
    }

    // Job History (collapsible in future)
    jobs := a.state.Jobs[item.ID]
    if len(jobs) > 0 {
        b.WriteString(sectionHeaderStyle.Render("HISTORY"))
        b.WriteString("\n")
        for _, job := range jobs {
            statusIcon := "○"
            switch job.Status {
            case model.JobStatusCompleted:
                statusIcon = "✓"
            case model.JobStatusInProgress:
                statusIcon = "◐"
            case model.JobStatusFailed:
                statusIcon = "✗"
            }
            b.WriteString(fmt.Sprintf("  %s %s\n", statusIcon, job.Stage.DisplayName()))
        }
        b.WriteString("\n")
    }

    // Help
    helpText := "[Enter] Next Action  [l] View Logs  [f] View Files  [Esc] Back  [q] Quit"
    if item.CurrentStage == model.StageRip && item.StageStatus == model.StatusCompleted {
        helpText = "[o] Organize  [l] View Logs  [f] View Files  [Esc] Back  [q] Quit"
    }
    b.WriteString(helpStyle.Render(helpText))

    return b.String()
}

// renderTVShowDetail renders detail view for a TV show
func (a *App) renderTVShowDetail(item *model.MediaItem) string {
    var b strings.Builder

    // Title
    b.WriteString(titleStyle.Render(item.Name))
    b.WriteString("\n")
    b.WriteString(mutedItemStyle.Render("TV Show"))
    b.WriteString("\n\n")

    // Seasons list
    b.WriteString(sectionHeaderStyle.Render("SEASONS"))
    b.WriteString("\n")

    if len(item.Seasons) == 0 {
        b.WriteString(mutedItemStyle.Render("  No seasons. Press [a] to add a season."))
        b.WriteString("\n")
    } else {
        for i, season := range item.Seasons {
            selected := i == a.cursor
            prefix := "  "
            if selected {
                prefix = "> "
            }

            statusIcon := "○"
            switch season.StageStatus {
            case model.StatusCompleted:
                if season.CurrentStage == model.StagePublish {
                    statusIcon = "✓"
                } else {
                    statusIcon = "●"
                }
            case model.StatusInProgress:
                statusIcon = "◐"
            case model.StatusFailed:
                statusIcon = "✗"
            }

            stageName := season.CurrentStage.DisplayName()
            b.WriteString(fmt.Sprintf("%s%s Season %d - %s (%s)\n",
                prefix, statusIcon, season.Number, stageName, season.StageStatus))
        }
    }
    b.WriteString("\n")

    // Help
    b.WriteString(helpStyle.Render("[Enter] View Season  [a] Add Season  [r] Start Rip  [Esc] Back  [q] Quit"))

    return b.String()
}
```

**Step 2: Run build**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/tui/itemdetail_new.go
git commit -m "tui: add new item detail views for movies and TV shows"
```

---

### Task 7: Create Season Detail view

**Files:**
- Create: `internal/tui/seasondetail.go`

**Step 1: Create the season detail view**

Create `internal/tui/seasondetail.go`:

```go
package tui

import (
    "fmt"
    "strings"

    "github.com/charmbracelet/lipgloss"
    "github.com/cuivienor/media-pipeline/internal/model"
)

// renderSeasonDetail renders the detail view for a TV season
func (a *App) renderSeasonDetail() string {
    if a.selectedItem == nil || a.selectedSeason == nil {
        return "No season selected"
    }

    item := a.selectedItem
    season := a.selectedSeason

    var b strings.Builder

    // Title
    title := fmt.Sprintf("%s - Season %d", item.Name, season.Number)
    b.WriteString(titleStyle.Render(title))
    b.WriteString("\n\n")

    // Current State
    b.WriteString(sectionHeaderStyle.Render("CURRENT STATE"))
    b.WriteString("\n")

    stageStyle := lipgloss.NewStyle()
    switch season.StageStatus {
    case model.StatusCompleted:
        stageStyle = stageStyle.Foreground(colorSuccess)
    case model.StatusInProgress:
        stageStyle = stageStyle.Foreground(colorWarning)
    case model.StatusFailed:
        stageStyle = stageStyle.Foreground(colorError)
    }

    b.WriteString(fmt.Sprintf("  Stage: %s\n", season.CurrentStage.DisplayName()))
    b.WriteString(fmt.Sprintf("  Status: %s\n", stageStyle.Render(string(season.StageStatus))))
    b.WriteString("\n")

    // Next Action
    if season.StageStatus == model.StatusCompleted && season.CurrentStage != model.StagePublish {
        b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
        b.WriteString("\n")
        nextStage := season.CurrentStage.NextStage()
        b.WriteString(fmt.Sprintf("  Press [Enter] to %s\n", nextStage.String()))
        b.WriteString("\n")
    } else if season.StageStatus == model.StatusFailed {
        b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
        b.WriteString("\n")
        b.WriteString("  Press [Enter] to retry\n")
        b.WriteString("\n")
    } else if season.StageStatus == model.StatusPending ||
              (season.CurrentStage == model.StageRip && season.StageStatus != model.StatusInProgress) {
        b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
        b.WriteString("\n")
        b.WriteString("  Press [r] to rip a disc\n")
        b.WriteString("\n")
    }

    // Rip Jobs (for TV seasons, multiple discs)
    jobs := a.state.Jobs[season.ID]
    ripJobs := filterJobsByStage(jobs, model.StageRip)
    if len(ripJobs) > 0 {
        b.WriteString(sectionHeaderStyle.Render("DISC RIPS"))
        b.WriteString("\n")
        for _, job := range ripJobs {
            statusIcon := "○"
            switch job.Status {
            case model.JobStatusCompleted:
                statusIcon = "✓"
            case model.JobStatusInProgress:
                statusIcon = "◐"
            case model.JobStatusFailed:
                statusIcon = "✗"
            }
            discLabel := "Disc"
            if job.Disc != nil {
                discLabel = fmt.Sprintf("Disc %d", *job.Disc)
            }
            b.WriteString(fmt.Sprintf("  %s %s\n", statusIcon, discLabel))
        }
        b.WriteString("\n")
    }

    // Other Job History
    otherJobs := filterJobsExcludingStage(jobs, model.StageRip)
    if len(otherJobs) > 0 {
        b.WriteString(sectionHeaderStyle.Render("HISTORY"))
        b.WriteString("\n")
        for _, job := range otherJobs {
            statusIcon := "○"
            switch job.Status {
            case model.JobStatusCompleted:
                statusIcon = "✓"
            case model.JobStatusInProgress:
                statusIcon = "◐"
            case model.JobStatusFailed:
                statusIcon = "✗"
            }
            b.WriteString(fmt.Sprintf("  %s %s\n", statusIcon, job.Stage.DisplayName()))
        }
        b.WriteString("\n")
    }

    // Help
    helpText := "[r] Rip Disc  [l] View Logs  [f] View Files  [Esc] Back  [q] Quit"
    if season.CurrentStage == model.StageRip && season.StageStatus == model.StatusCompleted {
        helpText = "[o] Organize  [r] Rip Another Disc  [l] View Logs  [Esc] Back  [q] Quit"
    }
    b.WriteString(helpStyle.Render(helpText))

    return b.String()
}

// filterJobsByStage returns jobs for a specific stage
func filterJobsByStage(jobs []model.Job, stage model.Stage) []model.Job {
    var result []model.Job
    for _, job := range jobs {
        if job.Stage == stage {
            result = append(result, job)
        }
    }
    return result
}

// filterJobsExcludingStage returns jobs excluding a specific stage
func filterJobsExcludingStage(jobs []model.Job, stage model.Stage) []model.Job {
    var result []model.Job
    for _, job := range jobs {
        if job.Stage != stage {
            result = append(result, job)
        }
    }
    return result
}
```

**Step 2: Update App struct to add selectedSeason**

In `internal/tui/app.go`, add to App struct:

```go
// Add this field to App struct (around line 34):
    selectedSeason *model.Season
```

**Step 3: Run build**

Run: `go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add internal/tui/seasondetail.go internal/tui/app.go
git commit -m "tui: add season detail view"
```

---

### Task 8: Create New Item form

**Files:**
- Create: `internal/tui/newitem.go`

**Step 1: Create the new item form**

Create `internal/tui/newitem.go`:

```go
package tui

import (
    "context"
    "fmt"
    "strconv"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/cuivienor/media-pipeline/internal/model"
)

// NewItemForm holds the form state for creating a new item
type NewItemForm struct {
    Type       string // "movie" or "tv"
    Name       string
    Seasons    string // For TV: "1-5" or "1,2,3" or "1"
    focusIndex int
    err        string
}

// fields returns the list of field names in order
func (f *NewItemForm) fields() []string {
    if f.Type == "tv" {
        return []string{"type", "name", "seasons"}
    }
    return []string{"type", "name"}
}

// Validate returns an error message if the form is invalid
func (f *NewItemForm) Validate() string {
    if f.Name == "" {
        return "Name is required"
    }
    if f.Type == "tv" && f.Seasons == "" {
        return "Seasons is required for TV shows (e.g., '1-5' or '1,2,3')"
    }
    if f.Type == "tv" {
        if _, err := parseSeasons(f.Seasons); err != nil {
            return err.Error()
        }
    }
    return ""
}

// parseSeasons parses a season string like "1-5" or "1,2,3" into a slice of ints
func parseSeasons(s string) ([]int, error) {
    s = strings.TrimSpace(s)
    if s == "" {
        return nil, fmt.Errorf("seasons cannot be empty")
    }

    // Handle range: "1-5"
    if strings.Contains(s, "-") {
        parts := strings.Split(s, "-")
        if len(parts) != 2 {
            return nil, fmt.Errorf("invalid range format, use '1-5'")
        }
        start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
        if err != nil {
            return nil, fmt.Errorf("invalid start number")
        }
        end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
        if err != nil {
            return nil, fmt.Errorf("invalid end number")
        }
        if start > end {
            return nil, fmt.Errorf("start must be less than end")
        }
        var seasons []int
        for i := start; i <= end; i++ {
            seasons = append(seasons, i)
        }
        return seasons, nil
    }

    // Handle comma-separated: "1,2,3"
    if strings.Contains(s, ",") {
        parts := strings.Split(s, ",")
        var seasons []int
        for _, p := range parts {
            n, err := strconv.Atoi(strings.TrimSpace(p))
            if err != nil {
                return nil, fmt.Errorf("invalid season number: %s", p)
            }
            seasons = append(seasons, n)
        }
        return seasons, nil
    }

    // Single number
    n, err := strconv.Atoi(s)
    if err != nil {
        return nil, fmt.Errorf("invalid season number")
    }
    return []int{n}, nil
}

// renderNewItemForm renders the new item form view
func (a *App) renderNewItemForm() string {
    var b strings.Builder

    b.WriteString(titleStyle.Render("New Item"))
    b.WriteString("\n\n")

    form := a.newItemForm
    fields := form.fields()

    for i, field := range fields {
        prefix := "  "
        if i == form.focusIndex {
            prefix = "> "
        }

        switch field {
        case "type":
            typeStr := "[movie]  tv"
            if form.Type == "tv" {
                typeStr = " movie  [tv]"
            }
            b.WriteString(fmt.Sprintf("%sType: %s\n", prefix, typeStr))
        case "name":
            b.WriteString(fmt.Sprintf("%sName: %s\n", prefix, form.Name))
        case "seasons":
            b.WriteString(fmt.Sprintf("%sSeasons: %s\n", prefix, form.Seasons))
            b.WriteString(mutedItemStyle.Render("        (e.g., '1-5' or '1,2,3')"))
            b.WriteString("\n")
        }
    }

    b.WriteString("\n")

    if form.err != "" {
        b.WriteString(errorStyle.Render(form.err))
        b.WriteString("\n\n")
    }

    b.WriteString(helpStyle.Render("[Enter] Create  [Tab] Next field  [Esc] Cancel"))

    return b.String()
}

// handleNewItemKey handles key presses in the new item form
func (a *App) handleNewItemKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    form := a.newItemForm
    fields := form.fields()

    switch msg.String() {
    case "tab", "down":
        form.focusIndex = (form.focusIndex + 1) % len(fields)
        return a, nil

    case "shift+tab", "up":
        form.focusIndex--
        if form.focusIndex < 0 {
            form.focusIndex = len(fields) - 1
        }
        return a, nil

    case "left", "right":
        if fields[form.focusIndex] == "type" {
            if form.Type == "movie" {
                form.Type = "tv"
            } else {
                form.Type = "movie"
            }
        }
        return a, nil

    case "enter":
        if err := form.Validate(); err != "" {
            form.err = err
            return a, nil
        }
        return a, a.createNewItem()

    case "backspace":
        field := fields[form.focusIndex]
        switch field {
        case "name":
            if len(form.Name) > 0 {
                form.Name = form.Name[:len(form.Name)-1]
            }
        case "seasons":
            if len(form.Seasons) > 0 {
                form.Seasons = form.Seasons[:len(form.Seasons)-1]
            }
        }
        return a, nil

    case "esc":
        a.currentView = ViewItemList
        a.newItemForm = nil
        return a, nil

    default:
        if len(msg.String()) == 1 {
            field := fields[form.focusIndex]
            char := msg.String()
            switch field {
            case "name":
                form.Name += char
            case "seasons":
                // Allow digits, comma, dash
                if (char >= "0" && char <= "9") || char == "," || char == "-" {
                    form.Seasons += char
                }
            }
        }
        return a, nil
    }
}

// itemCreatedMsg is sent when item creation completes
type itemCreatedMsg struct {
    item *model.MediaItem
    err  error
}

// createNewItem creates a new item in the database
func (a *App) createNewItem() tea.Cmd {
    return func() tea.Msg {
        form := a.newItemForm
        ctx := context.Background()

        safeName := strings.ReplaceAll(form.Name, " ", "_")

        item := &model.MediaItem{
            Type:       model.MediaType(form.Type),
            Name:       form.Name,
            SafeName:   safeName,
            ItemStatus: model.ItemStatusNotStarted,
        }

        // For movies, set initial stage
        if form.Type == "movie" {
            item.CurrentStage = model.StageRip
            item.StageStatus = model.StatusPending
        }

        if err := a.repo.CreateMediaItem(ctx, item); err != nil {
            return itemCreatedMsg{err: err}
        }

        // For TV shows, create seasons
        if form.Type == "tv" {
            seasons, _ := parseSeasons(form.Seasons)
            for _, num := range seasons {
                season := &model.Season{
                    ItemID:       item.ID,
                    Number:       num,
                    CurrentStage: model.StageRip,
                    StageStatus:  model.StatusPending,
                }
                if err := a.repo.CreateSeason(ctx, season); err != nil {
                    return itemCreatedMsg{err: fmt.Errorf("failed to create season %d: %w", num, err)}
                }
            }
        }

        return itemCreatedMsg{item: item}
    }
}
```

**Step 2: Add newItemForm to App struct**

In `internal/tui/app.go`, add:

```go
// Add this field to App struct:
    newItemForm *NewItemForm
```

**Step 3: Run build**

Run: `go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add internal/tui/newitem.go internal/tui/app.go
git commit -m "tui: add new item creation form"
```

---

## Phase 4: Wire Everything Together

### Task 9: Update App to use new views

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Update App struct with new state type**

Replace the App struct and related code in `internal/tui/app.go`:

```go
package tui

import (
    "fmt"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/cuivienor/media-pipeline/internal/config"
    "github.com/cuivienor/media-pipeline/internal/db"
    "github.com/cuivienor/media-pipeline/internal/model"
)

// View represents the current view
type View int

const (
    ViewItemList View = iota
    ViewItemDetail
    ViewSeasonDetail
    ViewOrganize
    ViewNewItem
)

// App is the main application model
type App struct {
    config *config.Config
    repo   db.Repository
    state  *AppState
    err    error

    // Navigation state
    currentView    View
    selectedItem   *model.MediaItem
    selectedSeason *model.Season
    cursor         int

    // Window size
    width  int
    height int

    // Form state
    newItemForm  *NewItemForm
    organizeView *OrganizeView
}

// NewApp creates a new application instance
func NewApp(cfg *config.Config, repo db.Repository) *App {
    return &App{
        config:      cfg,
        repo:        repo,
        currentView: ViewItemList,
    }
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
    return a.loadState
}

// stateMsg is sent when state loading completes
type stateMsg struct {
    state *AppState
    err   error
}

// loadState loads application state from the database
func (a *App) loadState() tea.Msg {
    state, err := LoadState(a.repo)
    return stateMsg{state: state, err: err}
}

// Update implements tea.Model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return a.handleKeyPress(msg)

    case tea.WindowSizeMsg:
        a.width = msg.Width
        a.height = msg.Height
        return a, nil

    case stateMsg:
        a.state = msg.state
        a.err = msg.err
        return a, nil

    case itemCreatedMsg:
        if msg.err != nil {
            a.err = msg.err
            return a, nil
        }
        a.currentView = ViewItemList
        a.newItemForm = nil
        return a, a.loadState

    case organizeLoadedMsg:
        if msg.err != nil {
            a.err = msg.err
            return a, nil
        }
        a.organizeView = &OrganizeView{
            item:  msg.item,
            path:  msg.path,
            files: msg.files,
        }
        a.currentView = ViewOrganize
        return a, nil

    case validateMsg:
        if msg.err != nil {
            a.err = msg.err
            return a, nil
        }
        if a.organizeView != nil {
            a.organizeView.validation = msg.result
        }
        return a, nil

    case organizeCompleteMsg:
        if msg.err != nil {
            a.err = msg.err
            return a, nil
        }
        a.currentView = ViewItemDetail
        a.organizeView = nil
        return a, a.loadState
    }

    return a, nil
}

// handleKeyPress handles keyboard input
func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    // Route to form handlers
    if a.currentView == ViewNewItem && a.newItemForm != nil {
        return a.handleNewItemKey(msg)
    }
    if a.currentView == ViewOrganize {
        return a.handleOrganizeKey(msg)
    }

    switch msg.String() {
    case "q", "ctrl+c":
        return a, tea.Quit

    case "r":
        return a, a.loadState

    case "n":
        if a.currentView == ViewItemList {
            a.currentView = ViewNewItem
            a.newItemForm = &NewItemForm{Type: "movie"}
            return a, nil
        }

    case "esc":
        switch a.currentView {
        case ViewItemDetail:
            a.currentView = ViewItemList
            a.selectedItem = nil
            a.cursor = 0
        case ViewSeasonDetail:
            a.currentView = ViewItemDetail
            a.selectedSeason = nil
            a.cursor = 0
        case ViewOrganize:
            a.currentView = ViewItemDetail
            a.organizeView = nil
        }
        return a, nil

    case "up", "k":
        if a.cursor > 0 {
            a.cursor--
        }
        return a, nil

    case "down", "j":
        a.cursor++
        maxCursor := a.getMaxCursor()
        if a.cursor > maxCursor {
            a.cursor = maxCursor
        }
        return a, nil

    case "enter":
        return a.handleEnter()

    case "o":
        // Organize action
        if a.currentView == ViewItemDetail && a.selectedItem != nil {
            if a.selectedItem.Type == model.MediaTypeMovie &&
                a.selectedItem.CurrentStage == model.StageRip &&
                a.selectedItem.StageStatus == model.StatusCompleted {
                return a, a.loadOrganizeView(a.selectedItem)
            }
        }
        if a.currentView == ViewSeasonDetail && a.selectedSeason != nil {
            if a.selectedSeason.CurrentStage == model.StageRip &&
                a.selectedSeason.StageStatus == model.StatusCompleted {
                return a, a.loadOrganizeViewForSeason(a.selectedItem, a.selectedSeason)
            }
        }
    }

    return a, nil
}

// getMaxCursor returns the maximum cursor position for the current view
func (a *App) getMaxCursor() int {
    if a.state == nil {
        return 0
    }

    switch a.currentView {
    case ViewItemList:
        if len(a.state.Items) == 0 {
            return 0
        }
        return len(a.state.Items) - 1
    case ViewItemDetail:
        if a.selectedItem != nil && a.selectedItem.Type == model.MediaTypeTV {
            if len(a.selectedItem.Seasons) == 0 {
                return 0
            }
            return len(a.selectedItem.Seasons) - 1
        }
        return 0
    default:
        return 0
    }
}

// handleEnter handles the enter key
func (a *App) handleEnter() (tea.Model, tea.Cmd) {
    if a.state == nil {
        return a, nil
    }

    switch a.currentView {
    case ViewItemList:
        if a.cursor < len(a.state.Items) {
            a.selectedItem = &a.state.Items[a.cursor]
            a.currentView = ViewItemDetail
            a.cursor = 0
        }
        return a, nil

    case ViewItemDetail:
        if a.selectedItem != nil && a.selectedItem.Type == model.MediaTypeTV {
            if a.cursor < len(a.selectedItem.Seasons) {
                a.selectedSeason = &a.selectedItem.Seasons[a.cursor]
                a.currentView = ViewSeasonDetail
                a.cursor = 0
            }
        }
        return a, nil
    }

    return a, nil
}

// View implements tea.Model
func (a *App) View() string {
    if a.err != nil {
        return fmt.Sprintf("Error: %v\n\nPress 'r' to retry or 'q' to quit.", a.err)
    }

    if a.state == nil {
        return "Loading..."
    }

    switch a.currentView {
    case ViewItemList:
        return a.renderItemList()
    case ViewItemDetail:
        return a.renderItemDetailNew()
    case ViewSeasonDetail:
        return a.renderSeasonDetail()
    case ViewOrganize:
        return a.renderOrganizeView()
    case ViewNewItem:
        return a.renderNewItemForm()
    default:
        return "Unknown view"
    }
}

// loadOrganizeViewForSeason loads organize view for a TV season
func (a *App) loadOrganizeViewForSeason(item *model.MediaItem, season *model.Season) tea.Cmd {
    // TODO: Implement season-specific organize loading
    // For now, reuse the item-based loading
    return a.loadOrganizeView(item)
}
```

**Step 2: Run build and tests**

Run: `go build ./... && go test ./... -v`
Expected: Build succeeds, tests may need updates

**Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "tui: wire up new item-centric views"
```

---

### Task 10: Clean up old views

**Files:**
- Delete: `internal/tui/overview.go`
- Delete: `internal/tui/stagelist.go`
- Delete: `internal/tui/actionlist.go`
- Delete: `internal/tui/itemdetail.go` (old one)
- Delete: `internal/tui/newrip.go`
- Rename: `internal/tui/itemdetail_new.go` → `internal/tui/itemdetail.go`

**Step 1: Remove old files**

```bash
rm internal/tui/overview.go
rm internal/tui/stagelist.go
rm internal/tui/actionlist.go
rm internal/tui/itemdetail.go
rm internal/tui/newrip.go
mv internal/tui/itemdetail_new.go internal/tui/itemdetail.go
```

**Step 2: Update itemdetail.go function name**

In `internal/tui/itemdetail.go`, rename `renderItemDetailNew` to `renderItemDetail` and update the call site in `app.go`.

**Step 3: Run build and tests**

Run: `go build ./... && go test ./... -v`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add -A
git commit -m "tui: remove old pipeline-centric views"
```

---

### Task 11: Update tests

**Files:**
- Modify: `internal/tui/state_test.go`
- Modify: `internal/db/sqlite_test.go`

**Step 1: Update state tests**

Update `internal/tui/state_test.go` to test new AppState structure.

**Step 2: Add Season repository tests**

Add tests to `internal/db/sqlite_test.go` for Season CRUD operations.

**Step 3: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 4: Commit**

```bash
git add internal/tui/state_test.go internal/db/sqlite_test.go
git commit -m "test: update tests for item-centric model"
```

---

## Phase 5: Data Migration

### Task 12: Write data migration logic

**Files:**
- Create: `internal/db/migrate.go`

**Step 1: Create migration helper**

Create `internal/db/migrate.go`:

```go
package db

import (
    "context"
    "database/sql"
    "fmt"
    "time"

    "github.com/cuivienor/media-pipeline/internal/model"
)

// MigrateToItemCentric migrates existing TV show data to the new schema.
// This consolidates multiple media_items rows (one per season) into
// a single item with multiple seasons.
func (r *SQLiteRepository) MigrateToItemCentric(ctx context.Context) error {
    // Find TV shows that need migration (have season in media_items but no seasons table entries)
    query := `
        SELECT DISTINCT safe_name
        FROM media_items
        WHERE type = 'tv' AND season IS NOT NULL
        AND NOT EXISTS (
            SELECT 1 FROM seasons s
            JOIN media_items m2 ON s.item_id = m2.id
            WHERE m2.safe_name = media_items.safe_name
        )
    `

    rows, err := r.db.db.QueryContext(ctx, query)
    if err != nil {
        return fmt.Errorf("failed to find TV shows to migrate: %w", err)
    }
    defer rows.Close()

    var showsToMigrate []string
    for rows.Next() {
        var safeName string
        if err := rows.Scan(&safeName); err != nil {
            return fmt.Errorf("failed to scan safe_name: %w", err)
        }
        showsToMigrate = append(showsToMigrate, safeName)
    }

    for _, safeName := range showsToMigrate {
        if err := r.migrateShow(ctx, safeName); err != nil {
            return fmt.Errorf("failed to migrate show %s: %w", safeName, err)
        }
    }

    return nil
}

func (r *SQLiteRepository) migrateShow(ctx context.Context, safeName string) error {
    // Get all media_items for this show
    query := `
        SELECT id, name, season, created_at
        FROM media_items
        WHERE safe_name = ? AND type = 'tv'
        ORDER BY season ASC
    `

    rows, err := r.db.db.QueryContext(ctx, query, safeName)
    if err != nil {
        return err
    }
    defer rows.Close()

    var items []struct {
        id        int64
        name      string
        season    int
        createdAt string
    }

    for rows.Next() {
        var item struct {
            id        int64
            name      string
            season    int
            createdAt string
        }
        var seasonNull sql.NullInt64
        if err := rows.Scan(&item.id, &item.name, &seasonNull, &item.createdAt); err != nil {
            return err
        }
        if seasonNull.Valid {
            item.season = int(seasonNull.Int64)
        }
        items = append(items, item)
    }

    if len(items) == 0 {
        return nil
    }

    // Use the first item as the "parent" item
    parentID := items[0].id

    // Create seasons for each item
    for _, item := range items {
        // Get latest job to determine stage/status
        var stage, status string
        err := r.db.db.QueryRowContext(ctx, `
            SELECT stage, status FROM jobs
            WHERE media_item_id = ?
            ORDER BY created_at DESC LIMIT 1
        `, item.id).Scan(&stage, &status)

        if err == sql.ErrNoRows {
            stage = "rip"
            status = "pending"
        } else if err != nil {
            return err
        }

        now := time.Now().UTC().Format(time.RFC3339)
        _, err = r.db.db.ExecContext(ctx, `
            INSERT INTO seasons (item_id, number, current_stage, stage_status, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?)
        `, parentID, item.season, stage, status, item.createdAt, now)
        if err != nil {
            return fmt.Errorf("failed to create season: %w", err)
        }

        // Update jobs to reference parent item
        if item.id != parentID {
            _, err = r.db.db.ExecContext(ctx, `
                UPDATE jobs SET media_item_id = ? WHERE media_item_id = ?
            `, parentID, item.id)
            if err != nil {
                return fmt.Errorf("failed to update jobs: %w", err)
            }

            // Delete the old media_item
            _, err = r.db.db.ExecContext(ctx, `
                DELETE FROM media_items WHERE id = ?
            `, item.id)
            if err != nil {
                return fmt.Errorf("failed to delete old media item: %w", err)
            }
        }
    }

    // Update parent item to remove season field and set status
    _, err = r.db.db.ExecContext(ctx, `
        UPDATE media_items SET season = NULL, status = 'active' WHERE id = ?
    `, parentID)
    if err != nil {
        return fmt.Errorf("failed to update parent item: %w", err)
    }

    return nil
}
```

**Step 2: Call migration on startup**

In `cmd/media-pipeline/main.go`, call the migration after opening the database.

**Step 3: Run tests**

Run: `go test ./internal/db/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/db/migrate.go cmd/media-pipeline/main.go
git commit -m "db: add data migration for existing TV shows"
```

---

## Summary

This plan covers:

1. **Database changes**: New `seasons` table, updated `media_items` with status fields
2. **Model changes**: New `Season` type, updated `MediaItem` with item-level status
3. **Repository changes**: CRUD for seasons, active item listing
4. **TUI changes**:
   - Item List (main view) grouped by status
   - Item Detail for movies and TV shows
   - Season Detail for TV seasons
   - New Item form supporting TV show season ranges
5. **Migration**: Logic to convert existing data to new schema

The implementation follows TDD where applicable and commits frequently.

# Media-Centric SQLite Architecture

## Summary

Transform the TUI from a pipeline-centric filesystem scanner to a media-centric SQLite-backed application. Users create movie/show entries, then trigger pipeline stages from the TUI. Database is source of truth; filesystem state remains for debugging.

---

## User Requirements

- **Media-centric view**: Landing shows movies/shows with pipeline progress, not stages with items
- **Active only**: Show items in progress or needing action (toggle for all)
- **SQLite state**: Persistent history even after file cleanup
- **TUI-initiated workflow**: Create entries, trigger rips from TUI
- **Logs viewable**: Drill into item → job → logs
- **Manual stage transitions**: User triggers each stage explicitly
- **TV shows**: Season is the unit, discs are progress markers
- **Distributed**: TUI on analyzer, dispatch to ripper/transcoder containers
- **Keep filesystem state**: `.rip/` dirs remain as debugging aid

---

## Database Schema

```sql
-- Media items (movies or TV seasons)
CREATE TABLE media_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK (type IN ('movie', 'tv')),
    name TEXT NOT NULL,
    safe_name TEXT NOT NULL,
    season INTEGER,  -- NULL for movies
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(safe_name, season)
);

-- Jobs (one per stage attempt)
CREATE TABLE jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL REFERENCES media_items(id),
    stage TEXT NOT NULL CHECK (stage IN ('rip', 'remux', 'transcode', 'filebot')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'in_progress', 'completed', 'failed')),
    disc INTEGER,  -- For TV rips
    worker_id TEXT,
    pid INTEGER,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    input_dir TEXT,
    output_dir TEXT,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Centralized logs
CREATE TABLE job_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL REFERENCES jobs(id),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    level TEXT CHECK (level IN ('debug', 'info', 'warn', 'error')),
    message TEXT NOT NULL
);

-- TV disc tracking
CREATE TABLE tv_discs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL REFERENCES media_items(id),
    disc_number INTEGER NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'ripped', 'failed')),
    rip_job_id INTEGER REFERENCES jobs(id),
    UNIQUE(media_item_id, disc_number)
);
```

**Location**: `/mnt/media/pipeline.db` (shared mount, accessible from all containers)

---

## New Package Structure

```
internal/
  db/
    db.go           -- Connection, migrations
    repository.go   -- Interface definition
    sqlite.go       -- SQLite implementation
  job/
    dispatcher.go   -- Job dispatch abstraction
    local.go        -- Local execution (MVP)
    ssh.go          -- Remote execution (future)
  model/
    media.go        -- Extended with Job, LogEntry, TVDisc types
  tui/
    app.go          -- Refactored to use repository
    media_list.go   -- Landing view (replaces overview)
    media_detail.go -- Item detail with pipeline progress
    job_detail.go   -- Job info + log viewer
    create_media.go -- Create movie/show form
    add_disc.go     -- Add disc to season (TV)
```

---

## TUI View Hierarchy

```
ViewMediaList (Landing - active items only)
    |
    +-- [N] New --> ViewCreateMedia
    |                   +-- Form: type, name, season (TV)
    |                   +-- [Enter] creates, returns to list
    |
    +-- [Tab] Toggle all/active
    |
    +-- [Enter] --> ViewMediaDetail
                        +-- Pipeline progress bar
                        +-- TV disc grid (if TV)
                        +-- [R] Rip, [M] Remux, [T] Transcode
                        +-- [A] Add Disc (TV only)
                        |
                        +-- [Enter] on stage --> ViewJobDetail
                                                    +-- Job metadata
                                                    +-- Scrollable logs
                                                    +-- [R] Retry if failed
```

---

## Sync Strategy

- **Database is authoritative** for TUI
- **Dual-write during jobs**: DB updated first, then filesystem `.rip/` dirs
- **Scanner becomes recovery tool**: Import orphaned filesystem state if needed
- Filesystem state is debugging aid, not source of truth

---

## Implementation Phases

### Phase 1: Database Foundation
- Create `internal/db` package with SQLite repository
- Add migration system
- Extend `internal/model` with Job, LogEntry, TVDisc types
- CLI to initialize database
- Repository tests

**Milestone**: CRUD operations work, can query media items/jobs

### Phase 2: TUI Restructure
- Refactor `internal/tui/app.go` to use repository
- Implement ViewMediaList (active items, grouped by type)
- Implement ViewMediaDetail (pipeline progress, actions)
- Implement ViewJobDetail (logs viewer)
- Implement ViewCreateMedia form

**Milestone**: TUI reads from DB, shows media items, can create new ones

### Phase 3: Job Dispatch
- Create `internal/job` package with LocalDispatcher
- Modify ripper to write to DB + filesystem
- Stream logs to job_logs table
- Wire TUI actions to dispatcher
- Job status refresh

**Milestone**: Can rip from TUI, see progress/logs

### Phase 4: TV Show Support
- ViewAddDisc form
- Disc grid in ViewMediaDetail
- Disc-specific rip jobs
- Season progress aggregation

**Milestone**: Full TV workflow with multiple discs

### Phase 5: Distributed (Future)
- SSHDispatcher for remote job execution
- Worker health monitoring

---

## Critical Files to Modify

| File | Changes |
|------|---------|
| `internal/model/media.go` | Add Job, LogEntry, TVDisc types; add ID fields |
| `internal/tui/app.go` | Replace scanner with repository |
| `internal/ripper/ripper.go` | Add DB writes alongside filesystem |
| `internal/ripper/state.go` | Extend StateManager for DB |

## New Files to Create

| File | Purpose |
|------|---------|
| `internal/db/db.go` | SQLite connection, migration runner |
| `internal/db/repository.go` | Repository interface |
| `internal/db/sqlite.go` | SQLite implementation |
| `internal/job/dispatcher.go` | Job dispatch interface |
| `internal/job/local.go` | Local execution |
| `internal/tui/media_list.go` | Landing view |
| `internal/tui/media_detail.go` | Item detail |
| `internal/tui/job_detail.go` | Job + logs |
| `internal/tui/create_media.go` | Create form |

---

## Key Decisions

1. **SQLite on shared mount** - Simple, low write volume acceptable
2. **Logs in DB** - Enables TUI viewing without filesystem access
3. **Manual stage transitions** - User controls pipeline flow
4. **Season as unit** - Discs are tracked within, not separate items
5. **Local dispatcher first** - Remote execution is Phase 5

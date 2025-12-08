-- File: internal/db/migrations/003_fix_jobs_unique.sql
-- Fix jobs unique constraint to include season_id for TV shows
-- Without this, disc 1 of season 1 conflicts with disc 1 of season 2

-- SQLite requires recreating the table to modify constraints
-- Note: SQLite treats NULL as distinct in UNIQUE constraints (NULL != NULL)
-- So we need two partial indexes: one for movies (season_id IS NULL) and one for TV (season_id IS NOT NULL)

-- Step 1: Create new table WITHOUT the old constraint
CREATE TABLE jobs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    season_id INTEGER REFERENCES seasons(id) ON DELETE CASCADE,
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
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Step 2: Copy data from old table
INSERT INTO jobs_new (id, media_item_id, season_id, stage, status, disc, worker_id, pid, input_dir, output_dir, log_path, error_message, started_at, completed_at, created_at)
SELECT id, media_item_id, season_id, stage, status, disc, worker_id, pid, input_dir, output_dir, log_path, error_message, started_at, completed_at, created_at
FROM jobs;

-- Step 3: Drop old table
DROP TABLE jobs;

-- Step 4: Rename new table
ALTER TABLE jobs_new RENAME TO jobs;

-- Step 5: Create indexes
CREATE INDEX IF NOT EXISTS idx_jobs_media_item ON jobs(media_item_id);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_season ON jobs(season_id);

-- Step 6: Create partial unique indexes
-- For movies (season_id IS NULL): unique on (media_item_id, stage, disc)
CREATE UNIQUE INDEX idx_jobs_unique_movie ON jobs(media_item_id, stage, disc) WHERE season_id IS NULL;
-- For TV seasons (season_id IS NOT NULL): unique on (media_item_id, season_id, stage, disc)
CREATE UNIQUE INDEX idx_jobs_unique_tv ON jobs(media_item_id, season_id, stage, disc) WHERE season_id IS NOT NULL;

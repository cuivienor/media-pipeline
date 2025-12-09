-- File: internal/db/migrations/004_transcode.sql
-- Transcode support: per-file tracking and job options

-- Create transcode_files table for per-file progress tracking
CREATE TABLE IF NOT EXISTS transcode_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    relative_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'completed', 'failed', 'skipped')),
    input_size INTEGER,
    output_size INTEGER,
    progress INTEGER DEFAULT 0,
    duration_secs REAL,
    started_at TEXT,
    completed_at TEXT,
    error_message TEXT,
    UNIQUE(job_id, relative_path)
);

CREATE INDEX IF NOT EXISTS idx_transcode_files_job ON transcode_files(job_id);
CREATE INDEX IF NOT EXISTS idx_transcode_files_status ON transcode_files(status);

-- Add options column to jobs table for per-job configuration overrides
-- Stores JSON: {"crf": 18, "mode": "hardware"}
ALTER TABLE jobs ADD COLUMN options TEXT;

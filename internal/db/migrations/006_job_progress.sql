-- Add progress column to jobs table for tracking job completion percentage
ALTER TABLE jobs ADD COLUMN progress INTEGER DEFAULT 0;

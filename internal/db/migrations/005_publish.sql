-- File: internal/db/migrations/005_publish.sql
-- Publish support: TMDB/TVDB IDs for reliable FileBot matching

-- Add tmdb_id for movies (TheMovieDB)
ALTER TABLE media_items ADD COLUMN tmdb_id INTEGER;

-- Add tvdb_id for TV shows (TheTVDB)
ALTER TABLE media_items ADD COLUMN tvdb_id INTEGER;

CREATE INDEX IF NOT EXISTS idx_media_items_tmdb ON media_items(tmdb_id);
CREATE INDEX IF NOT EXISTS idx_media_items_tvdb ON media_items(tvdb_id);

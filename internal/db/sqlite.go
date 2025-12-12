package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cuivienor/media-pipeline/internal/model"
)

// SQLiteRepository implements Repository using SQLite
type SQLiteRepository struct {
	db *DB
}

// NewSQLiteRepository creates a new SQLite repository
func NewSQLiteRepository(db *DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// CreateMediaItem creates a new media item
func (r *SQLiteRepository) CreateMediaItem(ctx context.Context, item *model.MediaItem) error {
	query := `
		INSERT INTO media_items (type, name, safe_name, season, tmdb_id, tvdb_id, status, current_stage, stage_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Set defaults if not provided
	itemStatus := item.ItemStatus
	if itemStatus == "" {
		itemStatus = model.ItemStatusNotStarted
	}

	stageStatus := item.StageStatus
	if stageStatus == "" {
		stageStatus = model.StatusPending
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := r.db.db.ExecContext(ctx, query,
		item.Type,
		item.Name,
		item.SafeName,
		item.Season,
		item.TmdbID,
		item.TvdbID,
		itemStatus,
		item.CurrentStage.String(),
		stageStatus,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert media item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	item.ID = id
	return nil
}

// GetMediaItem retrieves a media item by ID
func (r *SQLiteRepository) GetMediaItem(ctx context.Context, id int64) (*model.MediaItem, error) {
	query := `
		SELECT id, type, name, safe_name, season, tmdb_id, tvdb_id, created_at, updated_at
		FROM media_items
		WHERE id = ?
	`

	var item model.MediaItem
	var season, tmdbID, tvdbID sql.NullInt64
	var createdAt, updatedAt string

	err := r.db.db.QueryRowContext(ctx, query, id).Scan(
		&item.ID,
		&item.Type,
		&item.Name,
		&item.SafeName,
		&season,
		&tmdbID,
		&tvdbID,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get media item: %w", err)
	}

	if season.Valid {
		s := int(season.Int64)
		item.Season = &s
	}
	if tmdbID.Valid {
		id := int(tmdbID.Int64)
		item.TmdbID = &id
	}
	if tvdbID.Valid {
		id := int(tvdbID.Int64)
		item.TvdbID = &id
	}

	return &item, nil
}

// GetMediaItemBySafeName retrieves a media item by safe name and season
func (r *SQLiteRepository) GetMediaItemBySafeName(ctx context.Context, safeName string, season *int) (*model.MediaItem, error) {
	query := `
		SELECT id, type, name, safe_name, season, tmdb_id, tvdb_id, created_at, updated_at
		FROM media_items
		WHERE safe_name = ? AND (? IS NULL AND season IS NULL OR season = ?)
	`

	var item model.MediaItem
	var dbSeason, tmdbID, tvdbID sql.NullInt64
	var createdAt, updatedAt string

	var seasonVal interface{}
	if season != nil {
		seasonVal = *season
	}

	err := r.db.db.QueryRowContext(ctx, query, safeName, seasonVal, seasonVal).Scan(
		&item.ID,
		&item.Type,
		&item.Name,
		&item.SafeName,
		&dbSeason,
		&tmdbID,
		&tvdbID,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get media item by safe name: %w", err)
	}

	if dbSeason.Valid {
		s := int(dbSeason.Int64)
		item.Season = &s
	}
	if tmdbID.Valid {
		id := int(tmdbID.Int64)
		item.TmdbID = &id
	}
	if tvdbID.Valid {
		id := int(tvdbID.Int64)
		item.TvdbID = &id
	}

	return &item, nil
}

// ListMediaItems lists media items with optional filters
func (r *SQLiteRepository) ListMediaItems(ctx context.Context, opts ListOptions) ([]model.MediaItem, error) {
	query := `
		SELECT id, type, name, safe_name, season, tmdb_id, tvdb_id, created_at, updated_at
		FROM media_items
		WHERE 1=1
	`
	args := []interface{}{}

	if opts.Type != nil {
		query += " AND type = ?"
		args = append(args, *opts.Type)
	}

	if opts.ActiveOnly {
		query += `
			AND EXISTS (
				SELECT 1 FROM jobs
				WHERE jobs.media_item_id = media_items.id
				  AND jobs.status IN ('pending', 'in_progress')
			)`
	}

	query += " ORDER BY created_at DESC"

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	if opts.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, opts.Offset)
	}

	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list media items: %w", err)
	}
	defer rows.Close()

	var items []model.MediaItem
	for rows.Next() {
		var item model.MediaItem
		var season, tmdbID, tvdbID sql.NullInt64
		var createdAt, updatedAt string

		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Name,
			&item.SafeName,
			&season,
			&tmdbID,
			&tvdbID,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan media item: %w", err)
		}

		if season.Valid {
			s := int(season.Int64)
			item.Season = &s
		}
		if tmdbID.Valid {
			id := int(tmdbID.Int64)
			item.TmdbID = &id
		}
		if tvdbID.Valid {
			id := int(tvdbID.Int64)
			item.TvdbID = &id
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating media items: %w", err)
	}

	return items, nil
}

// CreateJob creates a new job
func (r *SQLiteRepository) CreateJob(ctx context.Context, job *model.Job) error {
	query := `
		INSERT INTO jobs (
			media_item_id, season_id, stage, status, disc, worker_id, pid,
			input_dir, output_dir, log_path, error_message,
			started_at, completed_at, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC().Format(time.RFC3339)

	var startedAt, completedAt interface{}
	if job.StartedAt != nil {
		startedAt = job.StartedAt.UTC().Format(time.RFC3339)
	}
	if job.CompletedAt != nil {
		completedAt = job.CompletedAt.UTC().Format(time.RFC3339)
	}

	result, err := r.db.db.ExecContext(ctx, query,
		job.MediaItemID,
		job.SeasonID,
		job.Stage.String(),
		job.Status,
		job.Disc,
		job.WorkerID,
		job.PID,
		job.InputDir,
		job.OutputDir,
		job.LogPath,
		job.ErrorMessage,
		startedAt,
		completedAt,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	job.ID = id
	return nil
}

// GetJob retrieves a job by ID
func (r *SQLiteRepository) GetJob(ctx context.Context, id int64) (*model.Job, error) {
	query := `
		SELECT id, media_item_id, season_id, stage, status, disc, worker_id, pid,
		       input_dir, output_dir, log_path, error_message,
		       started_at, completed_at, created_at
		FROM jobs
		WHERE id = ?
	`

	var job model.Job
	var stageStr string
	var seasonID, disc sql.NullInt64
	var workerID, inputDir, outputDir, logPath, errorMessage sql.NullString
	var pid sql.NullInt64
	var startedAt, completedAt, createdAt sql.NullString

	err := r.db.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID,
		&job.MediaItemID,
		&seasonID,
		&stageStr,
		&job.Status,
		&disc,
		&workerID,
		&pid,
		&inputDir,
		&outputDir,
		&logPath,
		&errorMessage,
		&startedAt,
		&completedAt,
		&createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Parse stage
	job.Stage = parseStage(stageStr)

	// Handle nullable fields
	if seasonID.Valid {
		s := seasonID.Int64
		job.SeasonID = &s
	}
	if disc.Valid {
		d := int(disc.Int64)
		job.Disc = &d
	}
	if workerID.Valid {
		job.WorkerID = workerID.String
	}
	if pid.Valid {
		job.PID = int(pid.Int64)
	}
	if inputDir.Valid {
		job.InputDir = inputDir.String
	}
	if outputDir.Valid {
		job.OutputDir = outputDir.String
	}
	if logPath.Valid {
		job.LogPath = logPath.String
	}
	if errorMessage.Valid {
		job.ErrorMessage = errorMessage.String
	}
	if startedAt.Valid {
		t, err := time.Parse(time.RFC3339, startedAt.String)
		if err == nil {
			job.StartedAt = &t
		}
	}
	if completedAt.Valid {
		t, err := time.Parse(time.RFC3339, completedAt.String)
		if err == nil {
			job.CompletedAt = &t
		}
	}
	if createdAt.Valid {
		t, err := time.Parse(time.RFC3339, createdAt.String)
		if err == nil {
			job.CreatedAt = t
		}
	}

	return &job, nil
}

// GetActiveJobForStage retrieves an active job for a specific stage
func (r *SQLiteRepository) GetActiveJobForStage(ctx context.Context, mediaItemID int64, stage model.Stage, disc *int) (*model.Job, error) {
	query := `
		SELECT id, media_item_id, stage, status, disc, worker_id, pid,
		       input_dir, output_dir, log_path, error_message,
		       started_at, completed_at, created_at
		FROM jobs
		WHERE media_item_id = ?
		  AND stage = ?
		  AND status IN ('pending', 'in_progress')
		  AND (? IS NULL AND disc IS NULL OR disc = ?)
		LIMIT 1
	`

	var discVal interface{}
	if disc != nil {
		discVal = *disc
	}

	var job model.Job
	var stageStr string
	var dbDisc sql.NullInt64
	var workerID, inputDir, outputDir, logPath, errorMessage sql.NullString
	var pid sql.NullInt64
	var startedAt, completedAt, createdAt sql.NullString

	err := r.db.db.QueryRowContext(ctx, query, mediaItemID, stage.String(), discVal, discVal).Scan(
		&job.ID,
		&job.MediaItemID,
		&stageStr,
		&job.Status,
		&dbDisc,
		&workerID,
		&pid,
		&inputDir,
		&outputDir,
		&logPath,
		&errorMessage,
		&startedAt,
		&completedAt,
		&createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active job: %w", err)
	}

	// Parse stage
	job.Stage = parseStage(stageStr)

	// Handle nullable fields
	if dbDisc.Valid {
		d := int(dbDisc.Int64)
		job.Disc = &d
	}
	if workerID.Valid {
		job.WorkerID = workerID.String
	}
	if pid.Valid {
		job.PID = int(pid.Int64)
	}
	if inputDir.Valid {
		job.InputDir = inputDir.String
	}
	if outputDir.Valid {
		job.OutputDir = outputDir.String
	}
	if logPath.Valid {
		job.LogPath = logPath.String
	}
	if errorMessage.Valid {
		job.ErrorMessage = errorMessage.String
	}
	if startedAt.Valid {
		t, err := time.Parse(time.RFC3339, startedAt.String)
		if err == nil {
			job.StartedAt = &t
		}
	}
	if completedAt.Valid {
		t, err := time.Parse(time.RFC3339, completedAt.String)
		if err == nil {
			job.CompletedAt = &t
		}
	}
	if createdAt.Valid {
		t, err := time.Parse(time.RFC3339, createdAt.String)
		if err == nil {
			job.CreatedAt = t
		}
	}

	return &job, nil
}

// UpdateJob updates all fields of a job
func (r *SQLiteRepository) UpdateJob(ctx context.Context, job *model.Job) error {
	query := `
		UPDATE jobs
		SET media_item_id = ?, stage = ?, status = ?, disc = ?,
		    worker_id = ?, pid = ?, input_dir = ?, output_dir = ?,
		    log_path = ?, error_message = ?, started_at = ?, completed_at = ?
		WHERE id = ?
	`

	var startedAt, completedAt interface{}
	if job.StartedAt != nil {
		startedAt = job.StartedAt.UTC().Format(time.RFC3339)
	}
	if job.CompletedAt != nil {
		completedAt = job.CompletedAt.UTC().Format(time.RFC3339)
	}

	_, err := r.db.db.ExecContext(ctx, query,
		job.MediaItemID,
		job.Stage.String(),
		job.Status,
		job.Disc,
		job.WorkerID,
		job.PID,
		job.InputDir,
		job.OutputDir,
		job.LogPath,
		job.ErrorMessage,
		startedAt,
		completedAt,
		job.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	return nil
}

// UpdateJobStatus updates a job's status and optionally sets error message and completion time
func (r *SQLiteRepository) UpdateJobStatus(ctx context.Context, id int64, status model.JobStatus, errorMsg string) error {
	query := `
		UPDATE jobs
		SET status = ?, error_message = ?, completed_at = ?
		WHERE id = ?
	`

	var completedAt interface{}
	if status == model.JobStatusCompleted || status == model.JobStatusFailed {
		completedAt = time.Now().UTC().Format(time.RFC3339)
	}

	_, err := r.db.db.ExecContext(ctx, query, status, errorMsg, completedAt, id)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	return nil
}

// UpdateJobProgress updates a job's progress percentage (0-100)
func (r *SQLiteRepository) UpdateJobProgress(ctx context.Context, id int64, progress int) error {
	query := `UPDATE jobs SET progress = ? WHERE id = ?`

	_, err := r.db.db.ExecContext(ctx, query, progress, id)
	if err != nil {
		return fmt.Errorf("failed to update job progress: %w", err)
	}

	return nil
}

// ListJobsForMedia lists all jobs for a media item
func (r *SQLiteRepository) ListJobsForMedia(ctx context.Context, mediaItemID int64) ([]model.Job, error) {
	query := `
		SELECT id, media_item_id, season_id, stage, status, disc, worker_id, pid,
		       input_dir, output_dir, log_path, error_message,
		       started_at, completed_at, created_at
		FROM jobs
		WHERE media_item_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.db.QueryContext(ctx, query, mediaItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []model.Job
	for rows.Next() {
		var job model.Job
		var stageStr string
		var seasonID, disc sql.NullInt64
		var workerID, inputDir, outputDir, logPath, errorMessage sql.NullString
		var pid sql.NullInt64
		var startedAt, completedAt, createdAt sql.NullString

		err := rows.Scan(
			&job.ID,
			&job.MediaItemID,
			&seasonID,
			&stageStr,
			&job.Status,
			&disc,
			&workerID,
			&pid,
			&inputDir,
			&outputDir,
			&logPath,
			&errorMessage,
			&startedAt,
			&completedAt,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		// Parse stage
		job.Stage = parseStage(stageStr)

		// Handle nullable fields
		if seasonID.Valid {
			s := seasonID.Int64
			job.SeasonID = &s
		}
		if disc.Valid {
			d := int(disc.Int64)
			job.Disc = &d
		}
		if workerID.Valid {
			job.WorkerID = workerID.String
		}
		if pid.Valid {
			job.PID = int(pid.Int64)
		}
		if inputDir.Valid {
			job.InputDir = inputDir.String
		}
		if outputDir.Valid {
			job.OutputDir = outputDir.String
		}
		if logPath.Valid {
			job.LogPath = logPath.String
		}
		if errorMessage.Valid {
			job.ErrorMessage = errorMessage.String
		}
		if startedAt.Valid {
			t, err := time.Parse(time.RFC3339, startedAt.String)
			if err == nil {
				job.StartedAt = &t
			}
		}
		if completedAt.Valid {
			t, err := time.Parse(time.RFC3339, completedAt.String)
			if err == nil {
				job.CompletedAt = &t
			}
		}
		if createdAt.Valid {
			t, err := time.Parse(time.RFC3339, createdAt.String)
			if err == nil {
				job.CreatedAt = t
			}
		}

		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating jobs: %w", err)
	}

	return jobs, nil
}

// CreateLogEvent creates a new log event
func (r *SQLiteRepository) CreateLogEvent(ctx context.Context, event *model.LogEvent) error {
	query := `
		INSERT INTO log_events (job_id, level, message, timestamp)
		VALUES (?, ?, ?, ?)
	`

	now := time.Now().UTC().Format(time.RFC3339)

	result, err := r.db.db.ExecContext(ctx, query,
		event.JobID,
		event.Level,
		event.Message,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert log event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	event.ID = id
	return nil
}

// ListLogEvents lists log events for a job
func (r *SQLiteRepository) ListLogEvents(ctx context.Context, jobID int64, limit int) ([]model.LogEvent, error) {
	query := `
		SELECT id, job_id, level, message, timestamp
		FROM log_events
		WHERE job_id = ?
		ORDER BY timestamp DESC
	`

	args := []interface{}{jobID}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list log events: %w", err)
	}
	defer rows.Close()

	var events []model.LogEvent
	for rows.Next() {
		var event model.LogEvent
		var timestampStr string

		err := rows.Scan(
			&event.ID,
			&event.JobID,
			&event.Level,
			&event.Message,
			&timestampStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log event: %w", err)
		}

		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err == nil {
			event.Timestamp = timestamp
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating log events: %w", err)
	}

	return events, nil
}

// GetDiscProgress gets progress for all discs of a TV show
func (r *SQLiteRepository) GetDiscProgress(ctx context.Context, mediaItemID int64) ([]model.DiscProgress, error) {
	query := `
		SELECT disc, status, id
		FROM jobs
		WHERE media_item_id = ?
		  AND stage = 'rip'
		  AND disc IS NOT NULL
		ORDER BY disc ASC
	`

	rows, err := r.db.db.QueryContext(ctx, query, mediaItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get disc progress: %w", err)
	}
	defer rows.Close()

	var progress []model.DiscProgress
	for rows.Next() {
		var p model.DiscProgress
		var disc int64

		err := rows.Scan(&disc, &p.Status, &p.JobID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan disc progress: %w", err)
		}

		p.Disc = int(disc)
		progress = append(progress, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating disc progress: %w", err)
	}

	return progress, nil
}

// parseStage converts a stage string to Stage enum
func parseStage(s string) model.Stage {
	switch s {
	case "rip":
		return model.StageRip
	case "organize":
		return model.StageOrganize
	case "remux":
		return model.StageRemux
	case "transcode":
		return model.StageTranscode
	case "publish":
		return model.StagePublish
	default:
		return model.StageRip
	}
}

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

// UpdateMediaItemStage updates a media item's current stage and stage status
func (r *SQLiteRepository) UpdateMediaItemStage(ctx context.Context, id int64, stage model.Stage, status model.Status) error {
	query := `UPDATE media_items SET current_stage = ?, stage_status = ?, updated_at = ? WHERE id = ?`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.db.ExecContext(ctx, query, stage.String(), status, now, id)
	if err != nil {
		return fmt.Errorf("failed to update media item stage: %w", err)
	}
	return nil
}

// ListActiveItems lists all items (including completed - history filtering will be added later)
func (r *SQLiteRepository) ListActiveItems(ctx context.Context) ([]model.MediaItem, error) {
	query := `
		SELECT id, type, name, safe_name, tmdb_id, tvdb_id, status, current_stage, stage_status, created_at, updated_at
		FROM media_items
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
		var tmdbID, tvdbID sql.NullInt64
		var stageStr, stageStatusStr sql.NullString
		var createdAt, updatedAt string

		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Name,
			&item.SafeName,
			&tmdbID,
			&tvdbID,
			&item.ItemStatus,
			&stageStr,
			&stageStatusStr,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan media item: %w", err)
		}

		if tmdbID.Valid {
			id := int(tmdbID.Int64)
			item.TmdbID = &id
		}
		if tvdbID.Valid {
			id := int(tvdbID.Int64)
			item.TvdbID = &id
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

// CreateTranscodeFile creates a new transcode file record
func (r *SQLiteRepository) CreateTranscodeFile(ctx context.Context, file *model.TranscodeFile) error {
	query := `
		INSERT INTO transcode_files (job_id, relative_path, status, input_size, duration_secs)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := r.db.db.ExecContext(ctx, query,
		file.JobID,
		file.RelativePath,
		file.Status,
		file.InputSize,
		file.DurationSecs,
	)
	if err != nil {
		return fmt.Errorf("failed to create transcode file: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	file.ID = id
	return nil
}

// GetTranscodeFile retrieves a transcode file by ID
func (r *SQLiteRepository) GetTranscodeFile(ctx context.Context, id int64) (*model.TranscodeFile, error) {
	query := `
		SELECT id, job_id, relative_path, status, input_size, output_size,
		       progress, duration_secs, started_at, completed_at, error_message
		FROM transcode_files
		WHERE id = ?
	`
	var file model.TranscodeFile
	var startedAt, completedAt sql.NullString
	var outputSize sql.NullInt64
	var errorMsg sql.NullString

	err := r.db.db.QueryRowContext(ctx, query, id).Scan(
		&file.ID,
		&file.JobID,
		&file.RelativePath,
		&file.Status,
		&file.InputSize,
		&outputSize,
		&file.Progress,
		&file.DurationSecs,
		&startedAt,
		&completedAt,
		&errorMsg,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transcode file: %w", err)
	}

	if outputSize.Valid {
		file.OutputSize = outputSize.Int64
	}
	if startedAt.Valid {
		if t, err := time.Parse(time.RFC3339, startedAt.String); err == nil {
			file.StartedAt = &t
		}
	}
	if completedAt.Valid {
		if t, err := time.Parse(time.RFC3339, completedAt.String); err == nil {
			file.CompletedAt = &t
		}
	}
	if errorMsg.Valid {
		file.ErrorMessage = errorMsg.String
	}

	return &file, nil
}

// ListTranscodeFiles lists all transcode files for a job
func (r *SQLiteRepository) ListTranscodeFiles(ctx context.Context, jobID int64) ([]model.TranscodeFile, error) {
	query := `
		SELECT id, job_id, relative_path, status, input_size, output_size,
		       progress, duration_secs, started_at, completed_at, error_message
		FROM transcode_files
		WHERE job_id = ?
		ORDER BY relative_path
	`
	rows, err := r.db.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to list transcode files: %w", err)
	}
	defer rows.Close()

	var files []model.TranscodeFile
	for rows.Next() {
		var file model.TranscodeFile
		var startedAt, completedAt sql.NullString
		var outputSize sql.NullInt64
		var errorMsg sql.NullString

		if err := rows.Scan(
			&file.ID,
			&file.JobID,
			&file.RelativePath,
			&file.Status,
			&file.InputSize,
			&outputSize,
			&file.Progress,
			&file.DurationSecs,
			&startedAt,
			&completedAt,
			&errorMsg,
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcode file: %w", err)
		}

		if outputSize.Valid {
			file.OutputSize = outputSize.Int64
		}
		if startedAt.Valid {
			if t, err := time.Parse(time.RFC3339, startedAt.String); err == nil {
				file.StartedAt = &t
			}
		}
		if completedAt.Valid {
			if t, err := time.Parse(time.RFC3339, completedAt.String); err == nil {
				file.CompletedAt = &t
			}
		}
		if errorMsg.Valid {
			file.ErrorMessage = errorMsg.String
		}

		files = append(files, file)
	}

	return files, rows.Err()
}

// UpdateTranscodeFile updates a transcode file record
func (r *SQLiteRepository) UpdateTranscodeFile(ctx context.Context, file *model.TranscodeFile) error {
	query := `
		UPDATE transcode_files
		SET status = ?, input_size = ?, output_size = ?, progress = ?,
		    duration_secs = ?, started_at = ?, completed_at = ?, error_message = ?
		WHERE id = ?
	`
	var startedAt, completedAt *string
	if file.StartedAt != nil {
		s := file.StartedAt.UTC().Format(time.RFC3339)
		startedAt = &s
	}
	if file.CompletedAt != nil {
		s := file.CompletedAt.UTC().Format(time.RFC3339)
		completedAt = &s
	}

	_, err := r.db.db.ExecContext(ctx, query,
		file.Status,
		file.InputSize,
		file.OutputSize,
		file.Progress,
		file.DurationSecs,
		startedAt,
		completedAt,
		file.ErrorMessage,
		file.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update transcode file: %w", err)
	}
	return nil
}

// UpdateTranscodeFileProgress updates just the progress percentage
func (r *SQLiteRepository) UpdateTranscodeFileProgress(ctx context.Context, id int64, progress int) error {
	query := `UPDATE transcode_files SET progress = ? WHERE id = ?`
	_, err := r.db.db.ExecContext(ctx, query, progress, id)
	if err != nil {
		return fmt.Errorf("failed to update transcode file progress: %w", err)
	}
	return nil
}

// UpdateTranscodeFileStatus updates status and optionally error message
func (r *SQLiteRepository) UpdateTranscodeFileStatus(ctx context.Context, id int64, status model.TranscodeFileStatus, errorMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var query string
	var args []interface{}

	if status == model.TranscodeFileStatusInProgress {
		query = `UPDATE transcode_files SET status = ?, started_at = ? WHERE id = ?`
		args = []interface{}{status, now, id}
	} else if status == model.TranscodeFileStatusCompleted || status == model.TranscodeFileStatusFailed {
		query = `UPDATE transcode_files SET status = ?, completed_at = ?, error_message = ? WHERE id = ?`
		args = []interface{}{status, now, errorMsg, id}
	} else {
		query = `UPDATE transcode_files SET status = ?, error_message = ? WHERE id = ?`
		args = []interface{}{status, errorMsg, id}
	}

	_, err := r.db.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update transcode file status: %w", err)
	}
	return nil
}

// GetJobOptions retrieves the JSON options for a job
func (r *SQLiteRepository) GetJobOptions(ctx context.Context, jobID int64) (map[string]interface{}, error) {
	query := `SELECT options FROM jobs WHERE id = ?`
	var optionsJSON sql.NullString

	err := r.db.db.QueryRowContext(ctx, query, jobID).Scan(&optionsJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job options: %w", err)
	}

	if !optionsJSON.Valid || optionsJSON.String == "" {
		return nil, nil
	}

	var options map[string]interface{}
	if err := json.Unmarshal([]byte(optionsJSON.String), &options); err != nil {
		return nil, fmt.Errorf("failed to parse job options: %w", err)
	}

	return options, nil
}

// SetJobOptions sets the JSON options for a job
func (r *SQLiteRepository) SetJobOptions(ctx context.Context, jobID int64, options map[string]interface{}) error {
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("failed to marshal job options: %w", err)
	}

	query := `UPDATE jobs SET options = ? WHERE id = ?`
	_, err = r.db.db.ExecContext(ctx, query, string(optionsJSON), jobID)
	if err != nil {
		return fmt.Errorf("failed to set job options: %w", err)
	}
	return nil
}

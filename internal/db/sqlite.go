package db

import (
	"context"
	"database/sql"
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
		INSERT INTO media_items (type, name, safe_name, season, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := r.db.db.ExecContext(ctx, query,
		item.Type,
		item.Name,
		item.SafeName,
		item.Season,
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
		SELECT id, type, name, safe_name, season, created_at, updated_at
		FROM media_items
		WHERE id = ?
	`

	var item model.MediaItem
	var season sql.NullInt64
	var createdAt, updatedAt string

	err := r.db.db.QueryRowContext(ctx, query, id).Scan(
		&item.ID,
		&item.Type,
		&item.Name,
		&item.SafeName,
		&season,
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

	return &item, nil
}

// GetMediaItemBySafeName retrieves a media item by safe name and season
func (r *SQLiteRepository) GetMediaItemBySafeName(ctx context.Context, safeName string, season *int) (*model.MediaItem, error) {
	query := `
		SELECT id, type, name, safe_name, season, created_at, updated_at
		FROM media_items
		WHERE safe_name = ? AND (? IS NULL AND season IS NULL OR season = ?)
	`

	var item model.MediaItem
	var dbSeason sql.NullInt64
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

	return &item, nil
}

// ListMediaItems lists media items with optional filters
func (r *SQLiteRepository) ListMediaItems(ctx context.Context, opts ListOptions) ([]model.MediaItem, error) {
	query := `
		SELECT id, type, name, safe_name, season, created_at, updated_at
		FROM media_items
		WHERE 1=1
	`
	args := []interface{}{}

	if opts.Type != nil {
		query += " AND type = ?"
		args = append(args, *opts.Type)
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
		var season sql.NullInt64
		var createdAt, updatedAt string

		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Name,
			&item.SafeName,
			&season,
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
			media_item_id, stage, status, disc, worker_id, pid,
			input_dir, output_dir, log_path, error_message,
			started_at, completed_at, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		SELECT id, media_item_id, stage, status, disc, worker_id, pid,
		       input_dir, output_dir, log_path, error_message,
		       started_at, completed_at, created_at
		FROM jobs
		WHERE id = ?
	`

	var job model.Job
	var stageStr string
	var disc sql.NullInt64
	var workerID, inputDir, outputDir, logPath, errorMessage sql.NullString
	var pid sql.NullInt64
	var startedAt, completedAt, createdAt sql.NullString

	err := r.db.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID,
		&job.MediaItemID,
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

// ListJobsForMedia lists all jobs for a media item
func (r *SQLiteRepository) ListJobsForMedia(ctx context.Context, mediaItemID int64) ([]model.Job, error) {
	query := `
		SELECT id, media_item_id, stage, status, disc, worker_id, pid,
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
		var disc sql.NullInt64
		var workerID, inputDir, outputDir, logPath, errorMessage sql.NullString
		var pid sql.NullInt64
		var startedAt, completedAt, createdAt sql.NullString

		err := rows.Scan(
			&job.ID,
			&job.MediaItemID,
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

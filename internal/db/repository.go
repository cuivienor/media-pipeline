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
	UpdateJob(ctx context.Context, job *model.Job) error
	UpdateJobStatus(ctx context.Context, id int64, status model.JobStatus, errorMsg string) error
	UpdateJobProgress(ctx context.Context, id int64, progress int) error
	ListJobsForMedia(ctx context.Context, mediaItemID int64) ([]model.Job, error)

	// Log events
	CreateLogEvent(ctx context.Context, event *model.LogEvent) error
	ListLogEvents(ctx context.Context, jobID int64, limit int) ([]model.LogEvent, error)

	// Disc progress (TV shows)
	GetDiscProgress(ctx context.Context, mediaItemID int64) ([]model.DiscProgress, error)

	// Seasons
	CreateSeason(ctx context.Context, season *model.Season) error
	GetSeason(ctx context.Context, id int64) (*model.Season, error)
	ListSeasonsForItem(ctx context.Context, itemID int64) ([]model.Season, error)
	UpdateSeason(ctx context.Context, season *model.Season) error
	UpdateSeasonStage(ctx context.Context, id int64, stage model.Stage, status model.Status) error

	// Updated item methods
	UpdateMediaItemStatus(ctx context.Context, id int64, status model.ItemStatus) error
	UpdateMediaItemStage(ctx context.Context, id int64, stage model.Stage, status model.Status) error
	ListActiveItems(ctx context.Context) ([]model.MediaItem, error)

	// Transcode files
	CreateTranscodeFile(ctx context.Context, file *model.TranscodeFile) error
	GetTranscodeFile(ctx context.Context, id int64) (*model.TranscodeFile, error)
	ListTranscodeFiles(ctx context.Context, jobID int64) ([]model.TranscodeFile, error)
	UpdateTranscodeFile(ctx context.Context, file *model.TranscodeFile) error
	UpdateTranscodeFileProgress(ctx context.Context, id int64, progress int) error
	UpdateTranscodeFileStatus(ctx context.Context, id int64, status model.TranscodeFileStatus, errorMsg string) error

	// Job options
	GetJobOptions(ctx context.Context, jobID int64) (map[string]interface{}, error)
	SetJobOptions(ctx context.Context, jobID int64, options map[string]interface{}) error
}

// ListOptions configures media item listing
type ListOptions struct {
	Type       *model.MediaType
	ActiveOnly bool
	Limit      int
	Offset     int
}

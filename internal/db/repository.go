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

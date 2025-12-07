package tui

import (
	"context"
	"fmt"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// PipelineState holds the current state loaded from the database.
// Items contain media items with their current stage/status updated from the latest job.
// Jobs maps media item IDs to their job history.
type PipelineState struct {
	Items []model.MediaItem
	Jobs  map[int64][]model.Job // mediaItemID -> jobs
}

// LoadState loads pipeline state from the database.
// It fetches all media items and their jobs, updating each item's current stage
// and status based on the latest job.
func LoadState(repo db.Repository) (*PipelineState, error) {
	ctx := context.Background()

	items, err := repo.ListMediaItems(ctx, db.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list media items: %w", err)
	}

	state := &PipelineState{
		Items: items,
		Jobs:  make(map[int64][]model.Job),
	}

	// Load jobs for each item and update item state.
	// Note: items[i] is modified in-place, so no need to reassign the slice.
	for i := range items {
		jobs, err := repo.ListJobsForMedia(ctx, items[i].ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list jobs for media item %d: %w", items[i].ID, err)
		}
		state.Jobs[items[i].ID] = jobs

		// Update item's current stage/status from latest job
		if len(jobs) > 0 {
			latestJob := jobs[len(jobs)-1]
			state.updateItemFromJob(&items[i], latestJob)
		}
	}

	return state, nil
}

// updateItemFromJob updates a MediaItem's Current/Status from its latest job
func (s *PipelineState) updateItemFromJob(item *model.MediaItem, job model.Job) {
	item.Current = job.Stage
	switch job.Status {
	case model.JobStatusCompleted:
		item.Status = model.StatusCompleted
	case model.JobStatusInProgress:
		item.Status = model.StatusInProgress
	case model.JobStatusFailed:
		item.Status = model.StatusFailed
	default:
		item.Status = model.StatusPending
	}
}

// CountByStage returns the number of items at each stage
func (s *PipelineState) CountByStage() map[model.Stage]int {
	counts := make(map[model.Stage]int)
	for _, item := range s.Items {
		counts[item.Current]++
	}
	return counts
}

// ItemsAtStage returns all items currently at the specified stage
func (s *PipelineState) ItemsAtStage(stage model.Stage) []model.MediaItem {
	var result []model.MediaItem
	for _, item := range s.Items {
		if item.Current == stage {
			result = append(result, item)
		}
	}
	return result
}

// ItemsReadyForNextStage returns all items that have completed their current stage
func (s *PipelineState) ItemsReadyForNextStage() []model.MediaItem {
	var result []model.MediaItem
	for _, item := range s.Items {
		if item.Status == model.StatusCompleted && item.Current != model.StagePublish {
			result = append(result, item)
		}
	}
	return result
}

// ItemsInProgress returns all items currently being processed
func (s *PipelineState) ItemsInProgress() []model.MediaItem {
	var result []model.MediaItem
	for _, item := range s.Items {
		if item.Status == model.StatusInProgress {
			result = append(result, item)
		}
	}
	return result
}

// ItemsFailed returns all items in a failed state
func (s *PipelineState) ItemsFailed() []model.MediaItem {
	var result []model.MediaItem
	for _, item := range s.Items {
		if item.Status == model.StatusFailed {
			result = append(result, item)
		}
	}
	return result
}

// GetJobsForItem returns all jobs for a specific media item
func (s *PipelineState) GetJobsForItem(itemID int64) []model.Job {
	return s.Jobs[itemID]
}

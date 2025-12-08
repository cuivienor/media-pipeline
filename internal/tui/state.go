package tui

import (
	"context"
	"fmt"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// AppState holds the current application state
type AppState struct {
	Items      []model.MediaItem
	MovieJobs  map[int64][]model.Job  // itemID -> jobs (for movies)
	SeasonJobs map[int64][]model.Job  // seasonID -> jobs (for TV seasons)
}

// LoadState loads application state from the database
func LoadState(repo db.Repository) (*AppState, error) {
	ctx := context.Background()

	items, err := repo.ListActiveItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list active items: %w", err)
	}

	state := &AppState{
		Items:      items,
		MovieJobs:  make(map[int64][]model.Job),
		SeasonJobs: make(map[int64][]model.Job),
	}

	// Load seasons for TV shows, jobs for all
	for i := range items {
		item := &items[i]

		if item.Type == model.MediaTypeTV {
			seasons, err := repo.ListSeasonsForItem(ctx, item.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to list seasons for %s: %w", item.Name, err)
			}
			item.Seasons = seasons

			// Load jobs once for the entire TV show
			jobs, err := repo.ListJobsForMedia(ctx, item.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to list jobs for %s: %w", item.Name, err)
			}

			// Assign jobs to each season
			for _, season := range seasons {
				// Filter to jobs for this season
				var seasonJobs []model.Job
				for _, job := range jobs {
					if job.SeasonID != nil && *job.SeasonID == season.ID {
						seasonJobs = append(seasonJobs, job)
					}
				}
				state.SeasonJobs[season.ID] = seasonJobs
			}
		} else {
			// Movie - load jobs directly
			jobs, err := repo.ListJobsForMedia(ctx, item.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to list jobs for %s: %w", item.Name, err)
			}
			state.MovieJobs[item.ID] = jobs

			// Update movie's current stage from jobs
			if len(jobs) > 0 {
				latestJob := jobs[len(jobs)-1]
				item.CurrentStage = latestJob.Stage
				item.StageStatus = jobStatusToStatus(latestJob.Status)
			}
		}
	}

	return state, nil
}

// ItemsNeedingAction returns movies that need user action.
// Note: Currently only handles movies. TV show seasons are handled in the display logic
// (Task 5 itemlist.go) by checking season.StageStatus directly.
func (s *AppState) ItemsNeedingAction() []model.MediaItem {
	var result []model.MediaItem
	for _, item := range s.Items {
		if item.Type == model.MediaTypeMovie {
			if item.StageStatus == model.StatusCompleted && item.CurrentStage != model.StagePublish {
				result = append(result, item)
			}
		}
	}
	return result
}

// ItemsInProgress returns movies currently being processed.
// Note: Currently only handles movies. TV show seasons are checked via season.StageStatus.
func (s *AppState) ItemsInProgress() []model.MediaItem {
	var result []model.MediaItem
	for _, item := range s.Items {
		if item.Type == model.MediaTypeMovie {
			if item.StageStatus == model.StatusInProgress {
				result = append(result, item)
			}
		}
	}
	return result
}

// ItemsFailed returns movies in a failed state.
// Note: Currently only handles movies. TV show seasons are checked via season.StageStatus.
func (s *AppState) ItemsFailed() []model.MediaItem {
	var result []model.MediaItem
	for _, item := range s.Items {
		if item.Type == model.MediaTypeMovie {
			if item.StageStatus == model.StatusFailed {
				result = append(result, item)
			}
		}
	}
	return result
}

// jobStatusToStatus converts JobStatus to Status
func jobStatusToStatus(js model.JobStatus) model.Status {
	switch js {
	case model.JobStatusCompleted:
		return model.StatusCompleted
	case model.JobStatusInProgress:
		return model.StatusInProgress
	case model.JobStatusFailed:
		return model.StatusFailed
	default:
		return model.StatusPending
	}
}

// Legacy compatibility methods for old views.
// These methods support ViewOverview, ViewStageList, ViewActionNeeded (removed in Task 10).

// CountByStage returns the number of items at each stage (legacy method)
func (s *AppState) CountByStage() map[model.Stage]int {
	counts := make(map[model.Stage]int)
	for _, item := range s.Items {
		if item.Type == model.MediaTypeMovie {
			counts[item.CurrentStage]++
		}
	}
	return counts
}

// ItemsAtStage returns all items currently at the specified stage (legacy method)
func (s *AppState) ItemsAtStage(stage model.Stage) []model.MediaItem {
	var result []model.MediaItem
	for _, item := range s.Items {
		if item.Type == model.MediaTypeMovie && item.CurrentStage == stage {
			result = append(result, item)
		}
	}
	return result
}

// ItemsReadyForNextStage returns all items that have completed their current stage (legacy method)
func (s *AppState) ItemsReadyForNextStage() []model.MediaItem {
	var result []model.MediaItem
	for _, item := range s.Items {
		if item.Type == model.MediaTypeMovie && item.StageStatus == model.StatusCompleted && item.CurrentStage != model.StagePublish {
			result = append(result, item)
		}
	}
	return result
}

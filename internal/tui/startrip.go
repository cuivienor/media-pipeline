package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// ripStartedMsg is sent when a rip job is dispatched
type ripStartedMsg struct {
	err error
}

// startRipForItem starts a rip job for an existing media item
func (a *App) startRipForItem(item *model.MediaItem) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Create pending job
		job := &model.Job{
			MediaItemID: item.ID,
			Stage:       model.StageRip,
			Status:      model.JobStatusPending,
		}
		if err := a.repo.CreateJob(ctx, job); err != nil {
			return ripStartedMsg{err: err}
		}

		// Update item status to in_progress (if not already)
		if item.StageStatus == model.StatusPending {
			if err := a.repo.UpdateMediaItemStage(ctx, item.ID, model.StageRip, model.StatusInProgress); err != nil {
				return ripStartedMsg{err: fmt.Errorf("failed to update item status: %w", err)}
			}
		}

		// Build command args
		args := []string{
			"-job-id", fmt.Sprintf("%d", job.ID),
			"-db", a.config.DatabasePath(),
		}

		// Find ripper binary - look in same directory as current executable
		ripperPath := "ripper"
		if exe, err := os.Executable(); err == nil {
			siblingPath := filepath.Join(filepath.Dir(exe), "ripper")
			if _, err := os.Stat(siblingPath); err == nil {
				ripperPath = siblingPath
			}
		}

		target := a.config.DispatchTarget("rip")
		if target == "" {
			// Local execution
			cmd := exec.Command(ripperPath, args...)
			if err := cmd.Start(); err != nil {
				return ripStartedMsg{err: fmt.Errorf("failed to start ripper: %w", err)}
			}
		} else {
			// SSH dispatch - assume ripper is in PATH on remote
			sshArgs := append([]string{target, "ripper"}, args...)
			cmd := exec.Command("ssh", sshArgs...)
			if err := cmd.Start(); err != nil {
				return ripStartedMsg{err: fmt.Errorf("failed to SSH dispatch ripper: %w", err)}
			}
		}

		return ripStartedMsg{err: nil}
	}
}

// seasonAddedMsg is sent when a season is added
type seasonAddedMsg struct {
	err error
}

// addSeasonToItem adds the next season number to a TV show
func (a *App) addSeasonToItem(item *model.MediaItem) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Determine next season number
		nextSeasonNum := 1
		for _, s := range item.Seasons {
			if s.Number >= nextSeasonNum {
				nextSeasonNum = s.Number + 1
			}
		}

		// Create new season
		season := &model.Season{
			ItemID:       item.ID,
			Number:       nextSeasonNum,
			CurrentStage: model.StageRip,
			StageStatus:  model.StatusPending,
		}
		if err := a.repo.CreateSeason(ctx, season); err != nil {
			return seasonAddedMsg{err: err}
		}

		return seasonAddedMsg{err: nil}
	}
}

// seasonRipsDoneMsg is sent when ripping is marked done for a season
type seasonRipsDoneMsg struct {
	err error
}

// markSeasonRipsDone marks all rip jobs as complete and updates season status
func (a *App) markSeasonRipsDone(item *model.MediaItem, season *model.Season) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Get all jobs for this media item
		jobs, err := a.repo.ListJobsForMedia(ctx, item.ID)
		if err != nil {
			return seasonRipsDoneMsg{err: fmt.Errorf("failed to list jobs: %w", err)}
		}

		// Check that there's at least one completed rip job for this season
		hasCompletedRip := false
		for _, job := range jobs {
			if job.Stage == model.StageRip && job.SeasonID != nil && *job.SeasonID == season.ID {
				if job.Status == model.JobStatusCompleted {
					hasCompletedRip = true
					break
				}
			}
		}

		if !hasCompletedRip {
			return seasonRipsDoneMsg{err: fmt.Errorf("no completed rip jobs for this season")}
		}

		// Update season status to completed (for rip stage)
		if err := a.repo.UpdateSeasonStage(ctx, season.ID, model.StageRip, model.StatusCompleted); err != nil {
			return seasonRipsDoneMsg{err: fmt.Errorf("failed to update season status: %w", err)}
		}

		return seasonRipsDoneMsg{err: nil}
	}
}

// startRipForSeason starts a rip job for a TV season
// It auto-determines the next disc number based on existing rip jobs
func (a *App) startRipForSeason(item *model.MediaItem, season *model.Season) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Determine next disc number by counting existing rip jobs for this season
		jobs, err := a.repo.ListJobsForMedia(ctx, item.ID)
		if err != nil {
			return ripStartedMsg{err: fmt.Errorf("failed to list jobs: %w", err)}
		}

		// Count rip jobs for this season
		discNum := 1
		for _, job := range jobs {
			if job.Stage == model.StageRip && job.SeasonID != nil && *job.SeasonID == season.ID {
				if job.Disc != nil && *job.Disc >= discNum {
					discNum = *job.Disc + 1
				}
			}
		}

		// Create pending job with season and disc info
		job := &model.Job{
			MediaItemID: item.ID,
			SeasonID:    &season.ID,
			Stage:       model.StageRip,
			Status:      model.JobStatusPending,
			Disc:        &discNum,
		}
		if err := a.repo.CreateJob(ctx, job); err != nil {
			return ripStartedMsg{err: err}
		}

		// Update season status to in_progress (if not already)
		if season.StageStatus == model.StatusPending {
			if err := a.repo.UpdateSeasonStage(ctx, season.ID, model.StageRip, model.StatusInProgress); err != nil {
				return ripStartedMsg{err: fmt.Errorf("failed to update season status: %w", err)}
			}
		}

		// Build command args
		args := []string{
			"-job-id", fmt.Sprintf("%d", job.ID),
			"-db", a.config.DatabasePath(),
		}

		// Find ripper binary - look in same directory as current executable
		ripperPath := "ripper"
		if exe, err := os.Executable(); err == nil {
			siblingPath := filepath.Join(filepath.Dir(exe), "ripper")
			if _, err := os.Stat(siblingPath); err == nil {
				ripperPath = siblingPath
			}
		}

		target := a.config.DispatchTarget("rip")
		if target == "" {
			// Local execution
			cmd := exec.Command(ripperPath, args...)
			if err := cmd.Start(); err != nil {
				return ripStartedMsg{err: fmt.Errorf("failed to start ripper: %w", err)}
			}
		} else {
			// SSH dispatch - assume ripper is in PATH on remote
			sshArgs := append([]string{target, "ripper"}, args...)
			cmd := exec.Command("ssh", sshArgs...)
			if err := cmd.Start(); err != nil {
				return ripStartedMsg{err: fmt.Errorf("failed to SSH dispatch ripper: %w", err)}
			}
		}

		return ripStartedMsg{err: nil}
	}
}

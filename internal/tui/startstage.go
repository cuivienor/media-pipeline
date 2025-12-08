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

// stageStartedMsg is sent when a stage job is dispatched
type stageStartedMsg struct {
	stage model.Stage
	err   error
}

// startStageForItem starts a stage job for a movie
func (a *App) startStageForItem(item *model.MediaItem, stage model.Stage) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// For rip stage, use the existing rip function
		if stage == model.StageRip {
			// This shouldn't happen as rip has its own handler, but handle gracefully
			return a.startRipForItem(item)()
		}

		// Create pending job
		job := &model.Job{
			MediaItemID: item.ID,
			Stage:       stage,
			Status:      model.JobStatusPending,
		}
		if err := a.repo.CreateJob(ctx, job); err != nil {
			return stageStartedMsg{stage: stage, err: fmt.Errorf("failed to create job: %w", err)}
		}

		// Update item stage and status
		if err := a.repo.UpdateMediaItemStage(ctx, item.ID, stage, model.StatusInProgress); err != nil {
			return stageStartedMsg{stage: stage, err: fmt.Errorf("failed to update item stage: %w", err)}
		}

		// Find the binary for this stage
		binaryName := stage.String()
		binaryPath := binaryName
		if exe, err := os.Executable(); err == nil {
			siblingPath := filepath.Join(filepath.Dir(exe), binaryName)
			if _, err := os.Stat(siblingPath); err == nil {
				binaryPath = siblingPath
			}
		}

		// Build command args
		args := []string{
			"-job-id", fmt.Sprintf("%d", job.ID),
			"-db", a.config.DatabasePath(),
		}

		// Check for SSH dispatch target
		target := a.config.DispatchTarget(stage.String())
		if target == "" {
			// Local execution
			cmd := exec.Command(binaryPath, args...)
			if err := cmd.Start(); err != nil {
				return stageStartedMsg{stage: stage, err: fmt.Errorf("failed to start %s: %w", binaryName, err)}
			}
		} else {
			// SSH dispatch
			sshArgs := append([]string{target, binaryName}, args...)
			cmd := exec.Command("ssh", sshArgs...)
			if err := cmd.Start(); err != nil {
				return stageStartedMsg{stage: stage, err: fmt.Errorf("failed to SSH dispatch %s: %w", binaryName, err)}
			}
		}

		return stageStartedMsg{stage: stage, err: nil}
	}
}

// startStageForSeason starts a stage job for a TV season
func (a *App) startStageForSeason(item *model.MediaItem, season *model.Season, stage model.Stage) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// For rip stage, use the existing rip function
		if stage == model.StageRip {
			// This shouldn't happen as rip has its own handler, but handle gracefully
			return a.startRipForSeason(item, season)()
		}

		// Create pending job with season reference
		job := &model.Job{
			MediaItemID: item.ID,
			SeasonID:    &season.ID,
			Stage:       stage,
			Status:      model.JobStatusPending,
		}
		if err := a.repo.CreateJob(ctx, job); err != nil {
			return stageStartedMsg{stage: stage, err: fmt.Errorf("failed to create job: %w", err)}
		}

		// Update season stage and status
		if err := a.repo.UpdateSeasonStage(ctx, season.ID, stage, model.StatusInProgress); err != nil {
			return stageStartedMsg{stage: stage, err: fmt.Errorf("failed to update season stage: %w", err)}
		}

		// Find the binary for this stage
		binaryName := stage.String()
		binaryPath := binaryName
		if exe, err := os.Executable(); err == nil {
			siblingPath := filepath.Join(filepath.Dir(exe), binaryName)
			if _, err := os.Stat(siblingPath); err == nil {
				binaryPath = siblingPath
			}
		}

		// Build command args
		args := []string{
			"-job-id", fmt.Sprintf("%d", job.ID),
			"-db", a.config.DatabasePath(),
		}

		// Check for SSH dispatch target
		target := a.config.DispatchTarget(stage.String())
		if target == "" {
			// Local execution
			cmd := exec.Command(binaryPath, args...)
			if err := cmd.Start(); err != nil {
				return stageStartedMsg{stage: stage, err: fmt.Errorf("failed to start %s: %w", binaryName, err)}
			}
		} else {
			// SSH dispatch
			sshArgs := append([]string{target, binaryName}, args...)
			cmd := exec.Command("ssh", sshArgs...)
			if err := cmd.Start(); err != nil {
				return stageStartedMsg{stage: stage, err: fmt.Errorf("failed to SSH dispatch %s: %w", binaryName, err)}
			}
		}

		return stageStartedMsg{stage: stage, err: nil}
	}
}

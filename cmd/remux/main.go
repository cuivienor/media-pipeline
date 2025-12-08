package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cuivienor/media-pipeline/internal/config"
	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/logging"
	"github.com/cuivienor/media-pipeline/internal/model"
	"github.com/cuivienor/media-pipeline/internal/remux"
)

func main() {
	var jobID int64
	var dbPath string

	flag.Int64Var(&jobID, "job-id", 0, "Job ID to execute")
	flag.StringVar(&dbPath, "db", "", "Path to database")
	flag.Parse()

	if jobID == 0 || dbPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: remux -job-id <id> -db <path>")
		os.Exit(1)
	}

	if err := run(jobID, dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(jobID int64, dbPath string) error {
	ctx := context.Background()

	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	repo := db.NewSQLiteRepository(database)

	// Get job
	job, err := repo.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Get media item
	item, err := repo.GetMediaItem(ctx, job.MediaItemID)
	if err != nil {
		return fmt.Errorf("failed to get media item: %w", err)
	}

	// Load config for languages
	cfg, err := config.LoadFromMediaBase()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Set up logging
	if err := cfg.EnsureJobLogDir(jobID); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	logPath := cfg.JobLogPath(jobID)
	logger, err := logging.NewForJob(logPath, true, nil)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Close()

	logger.Info("Starting remux: type=%s name=%q", item.Type, item.Name)

	// Find input directory from organize job
	inputDir, err := findOrganizeOutput(ctx, repo, job)
	if err != nil {
		logger.Error("Failed to find input: %v", err)
		return fmt.Errorf("failed to find input: %w", err)
	}

	// Determine output directory
	outputDir, err := buildOutputPath(ctx, repo, cfg, item, job)
	if err != nil {
		logger.Error("Failed to build output path: %v", err)
		return fmt.Errorf("failed to build output path: %w", err)
	}

	logger.Info("Input directory: %s", inputDir)
	logger.Info("Output directory: %s", outputDir)
	logger.Info("Languages to keep: %v", cfg.RemuxLanguages())

	// Update job to in_progress with input/output paths
	job.Status = model.JobStatusInProgress
	job.InputDir = inputDir
	job.OutputDir = outputDir
	now := time.Now()
	job.StartedAt = &now
	if err := repo.UpdateJob(ctx, job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Create remuxer and process
	remuxer := remux.NewRemuxer(cfg.RemuxLanguages())
	isTV := item.Type == model.MediaTypeTV

	logger.Info("Starting track filtering...")

	results, err := remuxer.RemuxDirectory(ctx, inputDir, outputDir, isTV)
	if err != nil {
		logger.Error("Remux failed: %v", err)
		// Mark job as failed
		if updateErr := repo.UpdateJobStatus(ctx, jobID, model.JobStatusFailed, err.Error()); updateErr != nil {
			logger.Error("Failed to update job status: %v", updateErr)
		}
		return err
	}

	// Log results
	totalRemoved := 0
	for _, r := range results {
		logger.Info("Processed: %s (input: %d audio, %d subs -> output: %d audio, %d subs, %d tracks removed)",
			filepath.Base(r.InputPath),
			r.InputTracks.Audio, r.InputTracks.Subtitles,
			r.OutputTracks.Audio, r.OutputTracks.Subtitles,
			r.TracksRemoved)
		totalRemoved += r.TracksRemoved
	}
	logger.Info("Total: %d files processed, %d tracks removed", len(results), totalRemoved)

	// Mark job as complete
	if err := repo.UpdateJobStatus(ctx, jobID, model.JobStatusCompleted, ""); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update media item stage
	if err := repo.UpdateMediaItemStage(ctx, item.ID, model.StageRemux, model.StatusCompleted); err != nil {
		return fmt.Errorf("failed to update item stage: %w", err)
	}

	logger.Info("Remux finished successfully")
	return nil
}

// findOrganizeOutput finds the output directory from the organize stage
func findOrganizeOutput(ctx context.Context, repo db.Repository, job *model.Job) (string, error) {
	// Look for completed organize job for this media item
	jobs, err := repo.ListJobsForMedia(ctx, job.MediaItemID)
	if err != nil {
		return "", err
	}

	// Find the most recent completed organize job
	for i := len(jobs) - 1; i >= 0; i-- {
		j := jobs[i]
		if j.Stage == model.StageOrganize && j.Status == model.JobStatusCompleted {
			if j.OutputDir != "" {
				return j.OutputDir, nil
			}
		}
	}

	return "", fmt.Errorf("no completed organize job found for media item %d", job.MediaItemID)
}

// buildOutputPath constructs the output directory for remuxed files
func buildOutputPath(ctx context.Context, repo db.Repository, cfg *config.Config, item *model.MediaItem, job *model.Job) (string, error) {
	// Output goes to staging/2-remuxed/{movies,tv}/{safe_name}
	mediaTypeDir := "movies"
	if item.Type == model.MediaTypeTV {
		mediaTypeDir = "tv"
	}

	baseName := item.SafeName
	if item.Type == model.MediaTypeTV && job.SeasonID != nil {
		// Include season in path for TV
		season, err := repo.GetSeason(ctx, *job.SeasonID)
		if err != nil {
			return "", fmt.Errorf("failed to get season: %w", err)
		}
		baseName = fmt.Sprintf("%s/Season_%02d", item.SafeName, season.Number)
	}

	return filepath.Join(cfg.StagingBase, "2-remuxed", mediaTypeDir, baseName), nil
}

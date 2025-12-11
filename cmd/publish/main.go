package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cuivienor/media-pipeline/internal/config"
	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/logging"
	"github.com/cuivienor/media-pipeline/internal/model"
	"github.com/cuivienor/media-pipeline/internal/publish"
)

func main() {
	var jobID int64
	var dbPath string

	flag.Int64Var(&jobID, "job-id", 0, "Job ID to execute")
	flag.StringVar(&dbPath, "db", "", "Path to database")
	flag.Parse()

	if jobID == 0 || dbPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: publish -job-id <id> -db <path>")
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

	// Helper to mark job as failed
	markFailed := func(errMsg string) {
		if updateErr := repo.UpdateJobStatus(ctx, jobID, model.JobStatusFailed, errMsg); updateErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to update job status: %v\n", updateErr)
		}
	}

	// Get job
	job, err := repo.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Get media item
	item, err := repo.GetMediaItem(ctx, job.MediaItemID)
	if err != nil {
		markFailed(err.Error())
		return fmt.Errorf("failed to get media item: %w", err)
	}

	// Validate database ID is set
	if item.DatabaseID() == 0 {
		errMsg := "media item missing database ID (set tmdb_id for movies, tvdb_id for TV)"
		markFailed(errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// Load config
	cfg, err := config.LoadFromMediaBase()
	if err != nil {
		markFailed(err.Error())
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Set up logging
	if err := cfg.EnsureJobLogDir(jobID); err != nil {
		markFailed(err.Error())
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	logPath := cfg.JobLogPath(jobID)
	logger, err := logging.NewForJob(logPath, true, nil)
	if err != nil {
		markFailed(err.Error())
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Close()

	logger.Info("Starting publish: type=%s name=%q dbID=%d", item.Type, item.Name, item.DatabaseID())

	// Find input directory from transcode job
	inputDir, err := findTranscodeOutput(ctx, repo, job)
	if err != nil {
		logger.Error("Failed to find input: %v", err)
		markFailed(err.Error())
		return fmt.Errorf("failed to find input: %w", err)
	}

	logger.Info("Input directory: %s", inputDir)

	// Update job to in_progress
	job.Status = model.JobStatusInProgress
	job.InputDir = inputDir
	now := time.Now()
	job.StartedAt = &now
	if err := repo.UpdateJob(ctx, job); err != nil {
		markFailed(err.Error())
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Create publisher
	opts := publish.PublishOptions{
		LibraryMovies: cfg.LibraryMoviesPath(),
		LibraryTV:     cfg.LibraryTVPath(),
	}
	publisher := publish.NewPublisher(repo, logger, opts)

	// Execute publish
	result, err := publisher.Publish(ctx, item, inputDir)
	if err != nil {
		logger.Error("Publish failed: %v", err)
		markFailed(err.Error())
		return err
	}

	logger.Info("Published to: %s", result.LibraryPath)
	logger.Info("Main files: %d, Extras: %d", result.MainFiles, result.ExtrasFiles)

	// Update job output directory
	job.OutputDir = result.LibraryPath
	if err := repo.UpdateJob(ctx, job); err != nil {
		logger.Error("Failed to update job output: %v", err)
	}

	// Mark job as complete
	if err := repo.UpdateJobStatus(ctx, jobID, model.JobStatusCompleted, ""); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update media item stage
	if err := repo.UpdateMediaItemStage(ctx, item.ID, model.StagePublish, model.StatusCompleted); err != nil {
		return fmt.Errorf("failed to update item stage: %w", err)
	}

	// Update item status to completed
	if err := repo.UpdateMediaItemStatus(ctx, item.ID, model.ItemStatusCompleted); err != nil {
		logger.Error("Failed to update item status: %v", err)
	}

	logger.Info("Publish finished successfully")
	return nil
}

// findTranscodeOutput finds the output directory from the transcode stage
func findTranscodeOutput(ctx context.Context, repo db.Repository, job *model.Job) (string, error) {
	jobs, err := repo.ListJobsForMedia(ctx, job.MediaItemID)
	if err != nil {
		return "", err
	}

	// Find the most recent completed transcode job
	for i := len(jobs) - 1; i >= 0; i-- {
		j := jobs[i]
		if j.Stage == model.StageTranscode && j.Status == model.JobStatusCompleted {
			if j.OutputDir != "" {
				return j.OutputDir, nil
			}
		}
	}

	return "", fmt.Errorf("no completed transcode job found for media item %d", job.MediaItemID)
}

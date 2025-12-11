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
	"github.com/cuivienor/media-pipeline/internal/transcode"
)

func main() {
	var jobID int64
	var dbPath string

	flag.Int64Var(&jobID, "job-id", 0, "Job ID to execute")
	flag.StringVar(&dbPath, "db", "", "Path to database")
	flag.Parse()

	if jobID == 0 || dbPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: transcode -job-id <id> -db <path>")
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

	logger.Info("Starting transcode: type=%s name=%q", item.Type, item.Name)

	// Get transcode options (defaults from config, overridable per-job)
	opts := transcode.TranscodeOptions{
		CRF:      cfg.TranscodeCRF(),
		Mode:     cfg.TranscodeMode(),
		Preset:   cfg.TranscodePreset(),
		HWPreset: cfg.TranscodeHWPreset(),
	}

	// Check for per-job overrides
	jobOpts, err := repo.GetJobOptions(ctx, jobID)
	if err == nil && jobOpts != nil {
		if crf, ok := jobOpts["crf"].(float64); ok {
			opts.CRF = int(crf)
		}
		if mode, ok := jobOpts["mode"].(string); ok {
			opts.Mode = mode
		}
	}

	logger.Info("Transcode options: CRF=%d, mode=%s, preset=%s", opts.CRF, opts.Mode, opts.Preset)

	// Check hardware support if requested
	if opts.Mode == "hardware" {
		if err := transcode.CheckHardwareSupport(); err != nil {
			logger.Error("Hardware encoding requested but not available: %v", err)
			markFailed(fmt.Sprintf("hardware encoding not available: %v", err))
			return fmt.Errorf("hardware encoding not available: %w", err)
		}
		logger.Info("Hardware encoding (QSV) available")
	}

	// Find input directory from remux job
	inputDir, err := findRemuxOutput(ctx, repo, job)
	if err != nil {
		logger.Error("Failed to find input: %v", err)
		markFailed(err.Error())
		return fmt.Errorf("failed to find input: %w", err)
	}

	// Determine output directory
	outputDir, err := buildOutputPath(ctx, repo, cfg, item, job)
	if err != nil {
		logger.Error("Failed to build output path: %v", err)
		markFailed(err.Error())
		return fmt.Errorf("failed to build output path: %w", err)
	}

	logger.Info("Input directory: %s", inputDir)
	logger.Info("Output directory: %s", outputDir)

	// Update job to in_progress
	job.Status = model.JobStatusInProgress
	job.InputDir = inputDir
	job.OutputDir = outputDir
	now := time.Now()
	job.StartedAt = &now
	if err := repo.UpdateJob(ctx, job); err != nil {
		markFailed(err.Error())
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Create transcoder and process
	transcoder := transcode.NewTranscoder(repo, logger, opts)
	isTV := item.Type == model.MediaTypeTV

	err = transcoder.TranscodeJob(ctx, job, inputDir, outputDir, isTV)
	if err != nil {
		logger.Error("Transcode failed: %v", err)
		markFailed(err.Error())
		return err
	}

	// Mark job as complete
	if err := repo.UpdateJobStatus(ctx, jobID, model.JobStatusCompleted, ""); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update media item stage
	if err := repo.UpdateMediaItemStage(ctx, item.ID, model.StageTranscode, model.StatusCompleted); err != nil {
		return fmt.Errorf("failed to update item stage: %w", err)
	}

	logger.Info("Transcode finished successfully")
	return nil
}

// findRemuxOutput finds the output directory from the remux stage
func findRemuxOutput(ctx context.Context, repo db.Repository, job *model.Job) (string, error) {
	jobs, err := repo.ListJobsForMedia(ctx, job.MediaItemID)
	if err != nil {
		return "", err
	}

	// Find the most recent completed remux job
	for i := len(jobs) - 1; i >= 0; i-- {
		j := jobs[i]
		if j.Stage == model.StageRemux && j.Status == model.JobStatusCompleted {
			if j.OutputDir != "" {
				return j.OutputDir, nil
			}
		}
	}

	return "", fmt.Errorf("no completed remux job found for media item %d", job.MediaItemID)
}

// buildOutputPath constructs the output directory for transcoded files
func buildOutputPath(ctx context.Context, repo db.Repository, cfg *config.Config, item *model.MediaItem, job *model.Job) (string, error) {
	mediaTypeDir := "movies"
	if item.Type == model.MediaTypeTV {
		mediaTypeDir = "tv"
	}

	baseName := item.SafeName
	if item.Type == model.MediaTypeTV && job.SeasonID != nil {
		season, err := repo.GetSeason(ctx, *job.SeasonID)
		if err != nil {
			return "", fmt.Errorf("failed to get season: %w", err)
		}
		baseName = fmt.Sprintf("%s/Season_%02d", item.SafeName, season.Number)
	}

	return filepath.Join(cfg.StagingBase, "3-transcoded", mediaTypeDir, baseName), nil
}

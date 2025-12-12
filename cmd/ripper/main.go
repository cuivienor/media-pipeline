package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/logging"
	"github.com/cuivienor/media-pipeline/internal/model"
	"github.com/cuivienor/media-pipeline/internal/ripper"
)

const defaultMediaBase = "/mnt/media"

func main() {
	var jobID int64
	var dbPath string
	var discPath string

	flag.Int64Var(&jobID, "job-id", 0, "Job ID to execute")
	flag.StringVar(&dbPath, "db", "", "Path to database")
	flag.StringVar(&discPath, "disc-path", "disc:0", "Path to disc device")
	flag.Parse()

	if jobID == 0 || dbPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: ripper -job-id <id> -db <path> [--disc-path <path>]")
		os.Exit(1)
	}

	if err := run(jobID, dbPath, discPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(jobID int64, dbPath string, discPath string) error {
	ctx := context.Background()

	// Get config from environment
	mediaBase := os.Getenv("MEDIA_BASE")
	if mediaBase == "" {
		mediaBase = defaultMediaBase
	}
	makeMKVConPath := os.Getenv("MAKEMKVCON_PATH")

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

	// Build rip request from job
	req, err := buildRipRequest(ctx, repo, job, item, discPath)
	if err != nil {
		markFailed(err.Error())
		return fmt.Errorf("failed to build rip request: %w", err)
	}

	// Set up logging
	logDir := filepath.Join(mediaBase, "pipeline", "logs", "jobs", fmt.Sprintf("%d", jobID))
	if err := os.MkdirAll(logDir, 0755); err != nil {
		markFailed(err.Error())
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	logPath := filepath.Join(logDir, "job.log")

	logger, err := logging.NewForJob(logPath, true, nil)
	if err != nil {
		markFailed(err.Error())
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Close()

	logger.Info("Starting rip: type=%s name=%q", item.Type, item.Name)
	if item.Type == model.MediaTypeTV {
		logger.Info("TV show: season=%d disc=%d", req.Season, req.Disc)
	}

	// Build output directory
	stagingBase := filepath.Join(mediaBase, "staging")
	outputDir := buildOutputDir(stagingBase, req)
	logger.Info("Output directory: %s", outputDir)

	// Update job to in_progress
	job.Status = model.JobStatusInProgress
	job.OutputDir = outputDir
	now := time.Now()
	job.StartedAt = &now
	if err := repo.UpdateJob(ctx, job); err != nil {
		markFailed(err.Error())
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Create ripper and run
	runner := ripper.NewMakeMKVRunner(makeMKVConPath)
	r := ripper.NewRipper(stagingBase, runner, &loggerAdapter{logger})

	// Create callbacks for line logging and progress updates
	onLine := func(line string) {
		// Log raw MakeMKV output to the job log
		logger.Info("[makemkv] %s", line)
	}

	lastProgress := 0
	onProgress := func(p ripper.Progress) {
		percent := int(p.Percent)
		// Only update on 1% increments to avoid excessive DB writes
		if percent > lastProgress {
			lastProgress = percent
			repo.UpdateJobProgress(ctx, jobID, percent)
		}
	}

	result, err := r.Rip(ctx, req, outputDir, onLine, onProgress)
	if err != nil {
		logger.Error("Rip failed: %v", err)
		markFailed(err.Error())
		return err
	}

	// Mark job as complete
	if err := repo.UpdateJobStatus(ctx, jobID, model.JobStatusCompleted, ""); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update media item stage
	if err := repo.UpdateMediaItemStage(ctx, item.ID, model.StageRip, model.StatusCompleted); err != nil {
		return fmt.Errorf("failed to update item stage: %w", err)
	}

	logger.Info("Rip finished successfully in %s", result.Duration())
	return nil
}

// buildRipRequest creates a RipRequest from job and media item
func buildRipRequest(ctx context.Context, repo db.Repository, job *model.Job, item *model.MediaItem, discPath string) (*ripper.RipRequest, error) {
	req := &ripper.RipRequest{
		Name:     item.Name,
		DiscPath: discPath,
	}

	// Set type
	switch item.Type {
	case model.MediaTypeMovie:
		req.Type = ripper.MediaTypeMovie
	case model.MediaTypeTV:
		req.Type = ripper.MediaTypeTV
	default:
		return nil, fmt.Errorf("unknown media type: %s", item.Type)
	}

	// Set TV-specific fields
	if req.Type == ripper.MediaTypeTV {
		if job.SeasonID == nil {
			return nil, fmt.Errorf("TV show job missing season_id")
		}
		season, err := repo.GetSeason(ctx, *job.SeasonID)
		if err != nil {
			return nil, fmt.Errorf("failed to get season: %w", err)
		}
		req.Season = season.Number

		if job.Disc == nil {
			return nil, fmt.Errorf("TV show job missing disc number")
		}
		req.Disc = *job.Disc
	}

	return req, nil
}

// buildOutputDir constructs the output directory path
func buildOutputDir(stagingBase string, req *ripper.RipRequest) string {
	safeName := req.SafeName()

	switch req.Type {
	case ripper.MediaTypeMovie:
		return filepath.Join(stagingBase, "1-ripped", "movies", safeName)
	case ripper.MediaTypeTV:
		season := fmt.Sprintf("S%02d", req.Season)
		disc := fmt.Sprintf("Disc%d", req.Disc)
		return filepath.Join(stagingBase, "1-ripped", "tv", safeName, season, disc)
	default:
		return filepath.Join(stagingBase, "1-ripped", "other", safeName)
	}
}

// loggerAdapter adapts logging.Logger to ripper.Logger interface
type loggerAdapter struct {
	*logging.Logger
}

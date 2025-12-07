package ripper

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/model"
)

const (
	stateDirName = ".rip"
)

// DefaultStateManager implements StateManager for rip operations
type DefaultStateManager struct{}

// NewStateManager creates a new DefaultStateManager
func NewStateManager() *DefaultStateManager {
	return &DefaultStateManager{}
}

// stateDir returns the path to the state directory
func (s *DefaultStateManager) stateDir(outputDir string) string {
	return filepath.Join(outputDir, stateDirName)
}

// Initialize creates the state directory and initial state files
func (s *DefaultStateManager) Initialize(outputDir string, request *RipRequest) error {
	stateDir := s.stateDir(outputDir)

	// Create state directory
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	now := time.Now()

	// Build metadata
	metadata := map[string]interface{}{
		"type":       string(request.Type),
		"name":       request.Name,
		"safe_name":  request.SafeName(),
		"output_dir": outputDir,
		"started_at": now.Format(time.RFC3339),
		"pid":        os.Getpid(),
	}

	// Add TV-specific fields
	if request.Type == MediaTypeTV {
		metadata["season"] = request.Season
		metadata["disc"] = request.Disc
	}

	// Write metadata.json
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "metadata.json"), metadataBytes, 0644); err != nil {
		return fmt.Errorf("failed to write metadata.json: %w", err)
	}

	// Write status file
	if err := os.WriteFile(filepath.Join(stateDir, "status"), []byte(string(model.StatusInProgress)), 0644); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	// Write started_at file
	if err := os.WriteFile(filepath.Join(stateDir, "started_at"), []byte(now.Format(time.RFC3339)), 0644); err != nil {
		return fmt.Errorf("failed to write started_at: %w", err)
	}

	// Write PID file
	if err := os.WriteFile(filepath.Join(stateDir, "pid"), []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("failed to write pid: %w", err)
	}

	return nil
}

// SetStatus updates the status file
func (s *DefaultStateManager) SetStatus(outputDir string, status model.Status) error {
	statusPath := filepath.Join(s.stateDir(outputDir), "status")
	if err := os.WriteFile(statusPath, []byte(string(status)), 0644); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}
	return nil
}

// SetError records an error in the state directory
func (s *DefaultStateManager) SetError(outputDir string, err error) error {
	errorPath := filepath.Join(s.stateDir(outputDir), "error")
	if writeErr := os.WriteFile(errorPath, []byte(err.Error()), 0644); writeErr != nil {
		return fmt.Errorf("failed to write error: %w", writeErr)
	}
	return nil
}

// Complete marks the operation as complete
func (s *DefaultStateManager) Complete(outputDir string) error {
	stateDir := s.stateDir(outputDir)

	// Update status to completed
	if err := s.SetStatus(outputDir, model.StatusCompleted); err != nil {
		return err
	}

	// Write completed_at timestamp
	completedAt := time.Now().Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(stateDir, "completed_at"), []byte(completedAt), 0644); err != nil {
		return fmt.Errorf("failed to write completed_at: %w", err)
	}

	// Remove PID file (job is no longer running)
	pidPath := filepath.Join(stateDir, "pid")
	os.Remove(pidPath) // Ignore error - file might not exist

	return nil
}

// DualWriteStateManager writes to both database and filesystem
// Database is the authoritative source, filesystem is for debugging/compatibility
type DualWriteStateManager struct {
	fs            StateManager
	repo          db.Repository
	jobID         int64
	itemID        int64
	existingJobID int64 // Set when resuming an existing job (TUI dispatch mode)
}

// NewDualWriteStateManager creates a new DualWriteStateManager
func NewDualWriteStateManager(fs StateManager, repo db.Repository) *DualWriteStateManager {
	return &DualWriteStateManager{
		fs:   fs,
		repo: repo,
	}
}

// WithJobID sets the existing job ID for TUI dispatch mode
// Returns the manager for chaining
func (d *DualWriteStateManager) WithJobID(jobID int64) *DualWriteStateManager {
	d.existingJobID = jobID
	return d
}

// Initialize creates database records and filesystem state
func (d *DualWriteStateManager) Initialize(outputDir string, req *RipRequest) error {
	ctx := context.Background()

	// If we have an existingJobID, we're resuming a job (TUI dispatch mode)
	if d.existingJobID > 0 {
		return d.resumeExistingJob(ctx, outputDir, req)
	}

	// Otherwise, create new job (standalone mode)
	return d.createNewJob(ctx, outputDir, req)
}

// resumeExistingJob updates an existing job for TUI dispatch mode
func (d *DualWriteStateManager) resumeExistingJob(ctx context.Context, outputDir string, req *RipRequest) error {
	// Load the existing job
	job, err := d.repo.GetJob(ctx, d.existingJobID)
	if err != nil {
		return fmt.Errorf("failed to load existing job: %w", err)
	}
	if job == nil {
		return fmt.Errorf("job %d not found", d.existingJobID)
	}

	// Store job ID and media item ID
	d.jobID = job.ID
	d.itemID = job.MediaItemID

	// Update job to in_progress with current worker and output_dir
	now := time.Now()
	job.Status = model.JobStatusInProgress
	job.OutputDir = outputDir
	job.WorkerID = hostname()
	job.PID = os.Getpid()
	job.StartedAt = &now
	job.ErrorMessage = "" // Clear any previous error

	// Update the job in the database
	if err := d.repo.UpdateJob(ctx, job); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	// Write filesystem state (best-effort)
	if err := d.fs.Initialize(outputDir, req); err != nil {
		log.Printf("WARNING: failed to write filesystem state: %v", err)
	}

	return nil
}

// createNewJob creates a new job and media item (standalone mode)
func (d *DualWriteStateManager) createNewJob(ctx context.Context, outputDir string, req *RipRequest) error {
	// 1. Find or create media item
	var season *int
	if req.Type == MediaTypeTV {
		season = &req.Season
	}

	item, err := d.repo.GetMediaItemBySafeName(ctx, req.SafeName(), season)
	if err != nil {
		return fmt.Errorf("failed to lookup media item: %w", err)
	}

	if item == nil {
		// Create new media item
		item = &model.MediaItem{
			Type:     model.MediaType(req.Type),
			Name:     req.Name,
			SafeName: req.SafeName(),
			Season:   season,
		}
		if err := d.repo.CreateMediaItem(ctx, item); err != nil {
			return fmt.Errorf("failed to create media item: %w", err)
		}
	}
	d.itemID = item.ID

	// 2. Create job record
	var disc *int
	if req.Type == MediaTypeTV && req.Disc > 0 {
		disc = &req.Disc
	}

	now := time.Now()
	job := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusInProgress,
		Disc:        disc,
		OutputDir:   outputDir,
		WorkerID:    hostname(),
		PID:         os.Getpid(),
		StartedAt:   &now,
	}
	if err := d.repo.CreateJob(ctx, job); err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	d.jobID = job.ID

	// 3. Write filesystem state (best-effort)
	if err := d.fs.Initialize(outputDir, req); err != nil {
		// Log warning but don't fail - DB is authoritative
		log.Printf("WARNING: failed to write filesystem state: %v", err)
	}

	return nil
}

// SetStatus updates filesystem status (best-effort)
func (d *DualWriteStateManager) SetStatus(outputDir string, status model.Status) error {
	// Update filesystem only - job status in DB is managed via Complete/SetError
	if err := d.fs.SetStatus(outputDir, status); err != nil {
		log.Printf("WARNING: failed to update filesystem status: %v", err)
	}
	return nil
}

// SetError records error in both DB and filesystem
func (d *DualWriteStateManager) SetError(outputDir string, err error) error {
	ctx := context.Background()

	// Update job status in DB to failed
	if updateErr := d.repo.UpdateJobStatus(ctx, d.jobID, model.JobStatusFailed, err.Error()); updateErr != nil {
		return fmt.Errorf("failed to update job status: %w", updateErr)
	}

	// Update filesystem (best-effort)
	if fsErr := d.fs.SetError(outputDir, err); fsErr != nil {
		log.Printf("WARNING: failed to write filesystem error: %v", fsErr)
	}

	return nil
}

// Complete marks the operation as complete in both DB and filesystem
func (d *DualWriteStateManager) Complete(outputDir string) error {
	ctx := context.Background()

	// Update DB first (authoritative)
	if err := d.repo.UpdateJobStatus(ctx, d.jobID, model.JobStatusCompleted, ""); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update filesystem (best-effort)
	if err := d.fs.Complete(outputDir); err != nil {
		log.Printf("WARNING: failed to complete filesystem state: %v", err)
	}

	return nil
}

// hostname returns the current hostname, or "unknown" if it can't be determined
func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}


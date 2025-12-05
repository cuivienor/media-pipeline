package ripper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
		metadata["season"] = formatSeason(request.Season)
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

// formatSeason formats a season number as "S01", "S02", etc.
func formatSeason(season int) string {
	if season <= 0 {
		return ""
	}
	return fmt.Sprintf("S%02d", season)
}

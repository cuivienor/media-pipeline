package ripper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cuivienor/media-pipeline/internal/model"
)

// Ripper orchestrates the disc ripping process
type Ripper struct {
	stagingBase string
	runner      MakeMKVRunner
	state       StateManager
}

// NewRipper creates a new Ripper instance
// If runner is nil, creates a DefaultMakeMKVRunner
// If state is nil, creates a DefaultStateManager
func NewRipper(stagingBase string, runner MakeMKVRunner, state StateManager) *Ripper {
	if runner == nil {
		runner = NewMakeMKVRunner("")
	}
	if state == nil {
		state = NewStateManager()
	}
	return &Ripper{
		stagingBase: stagingBase,
		runner:      runner,
		state:       state,
	}
}

// Rip performs the disc ripping operation
func (r *Ripper) Rip(ctx context.Context, req *RipRequest) (*RipResult, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	result := &RipResult{
		StartedAt: time.Now(),
	}

	// Build output directory
	outputDir := r.BuildOutputDir(req)
	result.OutputDir = outputDir

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Initialize state
	if err := r.state.Initialize(outputDir, req); err != nil {
		return nil, fmt.Errorf("failed to initialize state: %w", err)
	}

	// Run ripping
	err := r.runner.RipTitles(ctx, req.DiscPath, outputDir, nil, nil)
	if err != nil {
		// Record failure
		r.state.SetStatus(outputDir, model.StatusFailed)
		r.state.SetError(outputDir, err)
		result.Status = model.StatusFailed
		result.Error = err
		result.CompletedAt = time.Now()
		return result, err
	}

	// Create organization scaffolding for manual review
	if err := CreateOrganizationScaffolding(outputDir, req); err != nil {
		return nil, fmt.Errorf("failed to create organization scaffolding: %w", err)
	}

	// Complete state
	if err := r.state.Complete(outputDir); err != nil {
		return nil, fmt.Errorf("failed to complete state: %w", err)
	}

	result.Status = model.StatusCompleted
	result.CompletedAt = time.Now()

	return result, nil
}

// BuildOutputDir builds the output directory path for a rip request
func (r *Ripper) BuildOutputDir(req *RipRequest) string {
	safeName := req.SafeName()

	switch req.Type {
	case MediaTypeMovie:
		return filepath.Join(r.stagingBase, "1-ripped", "movies", safeName)
	case MediaTypeTV:
		season := fmt.Sprintf("S%02d", req.Season)
		disc := fmt.Sprintf("Disc%d", req.Disc)
		return filepath.Join(r.stagingBase, "1-ripped", "tv", safeName, season, disc)
	default:
		// Fallback for unknown type
		return filepath.Join(r.stagingBase, "1-ripped", "other", safeName)
	}
}

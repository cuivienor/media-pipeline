package ripper

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/cuivienor/media-pipeline/internal/model"
)

// MediaType for ripper operations
type MediaType string

const (
	MediaTypeMovie MediaType = "movie"
	MediaTypeTV    MediaType = "tv"
)

// RipRequest contains all information needed to rip a disc
type RipRequest struct {
	Type     MediaType // movie or tv
	Name     string    // Human readable name
	Season   int       // Season number (TV only, 0 for movies)
	Disc     int       // Disc number (TV only, 0 for movies)
	DiscPath string    // e.g., "disc:0" or "/dev/sr0"
}

// Validate checks that the request has all required fields
func (r *RipRequest) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	if r.Type == "" {
		return errors.New("type is required")
	}
	if r.Type == MediaTypeTV {
		if r.Season <= 0 {
			return errors.New("season is required for TV shows")
		}
		if r.Disc <= 0 {
			return errors.New("disc is required for TV shows")
		}
	}
	return nil
}

// SafeName returns a filesystem-safe version of the name
func (r *RipRequest) SafeName() string {
	return toSafeName(r.Name)
}

// toSafeName converts a name to a filesystem-safe format
func toSafeName(name string) string {
	var result strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			result.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			result.WriteRune('_')
		// Skip other characters (punctuation, etc.)
		}
	}
	// Clean up multiple underscores
	s := result.String()
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return strings.Trim(s, "_")
}

// RipResult contains the outcome of a rip operation
type RipResult struct {
	OutputDir   string       // Directory where files were saved
	OutputFiles []string     // List of created MKV files
	Status      model.Status // Final status
	StartedAt   time.Time    // When the rip started
	CompletedAt time.Time    // When the rip finished
	Error       error        // Error if failed
}

// Duration returns how long the rip took
func (r *RipResult) Duration() time.Duration {
	return r.CompletedAt.Sub(r.StartedAt)
}

// IsSuccess returns true if the rip completed successfully
func (r *RipResult) IsSuccess() bool {
	return r.Status == model.StatusCompleted
}

// TitleInfo represents a title found on the disc
type TitleInfo struct {
	Index    int           // Title index (0-based)
	Name     string        // Title name
	Duration time.Duration // Duration of the title
	Size     int64         // Size in bytes
	Filename string        // Suggested output filename
}

// DiscInfo represents information about a disc
type DiscInfo struct {
	Name       string      // Disc name
	ID         string      // Disc ID / volume name
	TitleCount int         // Number of titles on disc
	Titles     []TitleInfo // Information about each title
}

// Progress represents ripping progress
type Progress struct {
	CurrentTitle int     // Current title being ripped (0-based)
	TotalTitles  int     // Total number of titles
	TitleName    string  // Name of current title
	Percent      float64 // Progress percentage (0-100)
	BytesWritten int64   // Bytes written so far
}

// ProgressCallback is called with progress updates during ripping
type ProgressCallback func(Progress)

// MakeMKVRunner abstracts makemkvcon execution for testing
type MakeMKVRunner interface {
	// GetDiscInfo retrieves information about a disc
	GetDiscInfo(ctx context.Context, discPath string) (*DiscInfo, error)

	// RipTitles rips specified titles from a disc
	// If titleIndices is nil or empty, rips all titles
	RipTitles(ctx context.Context, discPath, outputDir string, titleIndices []int, progress ProgressCallback) error
}

// StateManager handles state directory operations (.rip/, .remux/, etc.)
type StateManager interface {
	// Initialize creates state directory and sets status to in_progress
	Initialize(outputDir string, request *RipRequest) error

	// SetStatus updates the status file
	SetStatus(outputDir string, status model.Status) error

	// SetError records an error in the state directory
	SetError(outputDir string, err error) error

	// Complete marks the operation as complete
	Complete(outputDir string) error
}

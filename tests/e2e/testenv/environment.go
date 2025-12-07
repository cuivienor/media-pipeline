package testenv

import (
	"os"
	"path/filepath"
	"testing"
)

// Environment manages test filesystem state for E2E tests
type Environment struct {
	t           *testing.T
	BaseDir     string // Root temp directory
	StagingBase string // Staging directory (contains 1-ripped, 2-remuxed, etc.)
	LibraryBase string // Library directory
	MockBinDir  string // Directory for mock binaries (prepended to PATH)
}

// New creates a new test environment with temp directories
// The environment is automatically cleaned up when the test completes
func New(t *testing.T) *Environment {
	t.Helper()

	baseDir, err := os.MkdirTemp("", "media-pipeline-e2e-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	env := &Environment{
		t:           t,
		BaseDir:     baseDir,
		StagingBase: filepath.Join(baseDir, "staging"),
		LibraryBase: filepath.Join(baseDir, "library"),
		MockBinDir:  filepath.Join(baseDir, "bin"),
	}

	// Create directory structure matching production layout
	dirs := []string{
		filepath.Join(env.StagingBase, "1-ripped", "movies"),
		filepath.Join(env.StagingBase, "1-ripped", "tv"),
		filepath.Join(env.StagingBase, "2-remuxed", "movies"),
		filepath.Join(env.StagingBase, "2-remuxed", "tv"),
		filepath.Join(env.StagingBase, "3-transcoded", "movies"),
		filepath.Join(env.StagingBase, "3-transcoded", "tv"),
		env.LibraryBase,
		env.MockBinDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Register cleanup
	t.Cleanup(func() {
		os.RemoveAll(baseDir)
	})

	return env
}

// EnvVars returns environment variables for subprocess execution
// This sets up MEDIA_BASE and prepends MockBinDir to PATH
func (e *Environment) EnvVars() []string {
	// Get existing PATH
	existingPath := os.Getenv("PATH")
	newPath := e.MockBinDir + ":" + existingPath

	return []string{
		"MEDIA_BASE=" + e.BaseDir,
		"PATH=" + newPath,
	}
}

// RippedMoviesDir returns the path to the ripped movies directory
func (e *Environment) RippedMoviesDir() string {
	return filepath.Join(e.StagingBase, "1-ripped", "movies")
}

// RippedTVDir returns the path to the ripped TV directory
func (e *Environment) RippedTVDir() string {
	return filepath.Join(e.StagingBase, "1-ripped", "tv")
}

// RemuxedMoviesDir returns the path to the remuxed movies directory
func (e *Environment) RemuxedMoviesDir() string {
	return filepath.Join(e.StagingBase, "2-remuxed", "movies")
}

// RemuxedTVDir returns the path to the remuxed TV directory
func (e *Environment) RemuxedTVDir() string {
	return filepath.Join(e.StagingBase, "2-remuxed", "tv")
}

// TranscodedMoviesDir returns the path to the transcoded movies directory
func (e *Environment) TranscodedMoviesDir() string {
	return filepath.Join(e.StagingBase, "3-transcoded", "movies")
}

// TranscodedTVDir returns the path to the transcoded TV directory
func (e *Environment) TranscodedTVDir() string {
	return filepath.Join(e.StagingBase, "3-transcoded", "tv")
}

// CreateMovieDir creates a movie directory in the ripped stage
// Returns the full path to the created directory
func (e *Environment) CreateMovieDir(safeName string) string {
	e.t.Helper()
	dir := filepath.Join(e.RippedMoviesDir(), safeName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		e.t.Fatalf("failed to create movie dir: %v", err)
	}
	return dir
}

// CreateTVDir creates a TV show directory structure in the ripped stage
// Returns the full path to the disc directory
func (e *Environment) CreateTVDir(safeName, season string, disc int) string {
	e.t.Helper()
	dir := filepath.Join(e.RippedTVDir(), safeName, season, formatDisc(disc))
	if err := os.MkdirAll(dir, 0755); err != nil {
		e.t.Fatalf("failed to create TV dir: %v", err)
	}
	return dir
}

func formatDisc(disc int) string {
	if disc <= 0 {
		return "Disc1"
	}
	return "Disc" + string(rune('0'+disc))
}

// WithDB creates a DBFixture and returns both the environment and fixture
func (e *Environment) WithDB(t *testing.T) (*Environment, *DBFixture) {
	t.Helper()
	dbFixture := NewDBFixture(t)
	return e, dbFixture
}

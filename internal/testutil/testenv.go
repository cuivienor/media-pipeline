package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// TestEnv provides an isolated test environment with temp directories and in-memory database
type TestEnv struct {
	t          *testing.T
	BaseDir    string
	StagingDir string
	LibraryDir string
	DB         *db.DB
	Repo       *db.SQLiteRepository
}

// NewTestEnv creates a new isolated test environment
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	baseDir := t.TempDir()

	// Create staging directories
	stagingDirs := []string{
		"staging/1-ripped/movies",
		"staging/1-ripped/tv",
		"staging/2-remuxed/movies",
		"staging/2-remuxed/tv",
		"staging/3-transcoded/movies",
		"staging/3-transcoded/tv",
	}
	for _, dir := range stagingDirs {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0755); err != nil {
			t.Fatalf("failed to create staging dir %s: %v", dir, err)
		}
	}

	// Create library directories
	libraryDirs := []string{
		"library/movies",
		"library/tv",
	}
	for _, dir := range libraryDirs {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0755); err != nil {
			t.Fatalf("failed to create library dir %s: %v", dir, err)
		}
	}

	// Open in-memory database
	database, err := db.OpenInMemory()
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}

	repo := db.NewSQLiteRepository(database)

	env := &TestEnv{
		t:          t,
		BaseDir:    baseDir,
		StagingDir: filepath.Join(baseDir, "staging"),
		LibraryDir: filepath.Join(baseDir, "library"),
		DB:         database,
		Repo:       repo,
	}

	t.Cleanup(func() {
		database.Close()
	})

	return env
}

// CreateMediaItem creates a media item in the test database
func (e *TestEnv) CreateMediaItem(safeName string, mediaType model.MediaType) *model.MediaItem {
	e.t.Helper()
	ctx := context.Background()

	// Derive human-readable name from safe name
	name := strings.ReplaceAll(safeName, "_", " ")

	item := &model.MediaItem{
		Type:       mediaType,
		Name:       name,
		SafeName:   safeName,
		ItemStatus: model.ItemStatusActive,
	}

	if err := e.Repo.CreateMediaItem(ctx, item); err != nil {
		e.t.Fatalf("CreateMediaItem failed: %v", err)
	}

	return item
}

// CreateJob creates a pending job for a media item
func (e *TestEnv) CreateJob(mediaItemID int64, stage model.Stage) *model.Job {
	e.t.Helper()
	ctx := context.Background()

	job := &model.Job{
		MediaItemID: mediaItemID,
		Stage:       stage,
		Status:      model.JobStatusPending,
	}

	if err := e.Repo.CreateJob(ctx, job); err != nil {
		e.t.Fatalf("CreateJob failed: %v", err)
	}

	return job
}

// CreateCompletedJob creates a completed job with the specified output directory
func (e *TestEnv) CreateCompletedJob(mediaItemID int64, stage model.Stage, outputDir string) *model.Job {
	e.t.Helper()
	ctx := context.Background()

	now := time.Now()
	job := &model.Job{
		MediaItemID: mediaItemID,
		Stage:       stage,
		Status:      model.JobStatusCompleted,
		OutputDir:   outputDir,
		Progress:    100,
		StartedAt:   &now,
		CompletedAt: &now,
	}

	if err := e.Repo.CreateJob(ctx, job); err != nil {
		e.t.Fatalf("CreateCompletedJob failed: %v", err)
	}

	return job
}

// CreateMovieStructure creates an organized movie directory with a test MKV
// stage is one of: "1-ripped", "2-remuxed", "3-transcoded"
// Returns the full path to the movie directory
func (e *TestEnv) CreateMovieStructure(safeName, stage string) string {
	e.t.Helper()

	movieDir := filepath.Join(e.StagingDir, stage, "movies", safeName)
	mainDir := filepath.Join(movieDir, "_main")

	if err := os.MkdirAll(mainDir, 0755); err != nil {
		e.t.Fatalf("failed to create _main dir: %v", err)
	}

	// Generate test MKV
	mkvPath := filepath.Join(mainDir, "movie.mkv")
	if err := GenerateTestMKV(mkvPath, MKVOptions{
		DurationSec: 1,
		AudioLangs:  []string{"eng", "spa", "fra"},
		SubLangs:    []string{"eng", "spa"},
	}); err != nil {
		e.t.Fatalf("failed to generate test MKV: %v", err)
	}

	return movieDir
}

// CreateTVStructure creates an organized TV season directory with numbered episode MKVs
// stage is one of: "1-ripped", "2-remuxed", "3-transcoded"
// Returns the full path to the season directory
func (e *TestEnv) CreateTVStructure(safeName string, season, episodeCount int, stage string) string {
	e.t.Helper()

	seasonDir := filepath.Join(e.StagingDir, stage, "tv", safeName, fmt.Sprintf("Season_%02d", season))
	episodesDir := filepath.Join(seasonDir, "_episodes")

	if err := os.MkdirAll(episodesDir, 0755); err != nil {
		e.t.Fatalf("failed to create _episodes dir: %v", err)
	}

	// Generate episode MKVs
	for i := 1; i <= episodeCount; i++ {
		mkvPath := filepath.Join(episodesDir, fmt.Sprintf("%02d.mkv", i))
		if err := GenerateTestMKV(mkvPath, MKVOptions{
			DurationSec: 1,
			AudioLangs:  []string{"eng", "spa"},
			SubLangs:    []string{"eng"},
		}); err != nil {
			e.t.Fatalf("failed to generate episode %d MKV: %v", i, err)
		}
	}

	return seasonDir
}

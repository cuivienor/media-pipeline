package testutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/model"
)

func TestNewTestEnv_CreatesStagingDirs(t *testing.T) {
	env := NewTestEnv(t)

	// Verify staging directories exist
	dirs := []string{
		"staging/1-ripped/movies",
		"staging/1-ripped/tv",
		"staging/2-remuxed/movies",
		"staging/2-remuxed/tv",
		"staging/3-transcoded/movies",
		"staging/3-transcoded/tv",
		"library/movies",
		"library/tv",
	}

	for _, dir := range dirs {
		path := filepath.Join(env.BaseDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("directory not created: %s", dir)
		}
	}
}

func TestNewTestEnv_ProvidesInMemoryDB(t *testing.T) {
	env := NewTestEnv(t)

	if env.DB == nil {
		t.Fatal("DB is nil")
	}
	if env.Repo == nil {
		t.Fatal("Repo is nil")
	}
}

func TestTestEnv_CreateMediaItem(t *testing.T) {
	env := NewTestEnv(t)
	ctx := context.Background()

	item := env.CreateMediaItem("Test_Movie", model.MediaTypeMovie)

	if item.ID == 0 {
		t.Error("item ID not set")
	}
	if item.SafeName != "Test_Movie" {
		t.Errorf("SafeName = %q, want %q", item.SafeName, "Test_Movie")
	}

	// Verify persisted
	fetched, err := env.Repo.GetMediaItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetMediaItem error: %v", err)
	}
	if fetched == nil {
		t.Fatal("item not found in database")
	}
}

func TestTestEnv_CreateJob(t *testing.T) {
	env := NewTestEnv(t)
	ctx := context.Background()

	item := env.CreateMediaItem("Test_Movie", model.MediaTypeMovie)
	job := env.CreateJob(item.ID, model.StageOrganize)

	if job.ID == 0 {
		t.Error("job ID not set")
	}
	if job.MediaItemID != item.ID {
		t.Errorf("MediaItemID = %d, want %d", job.MediaItemID, item.ID)
	}
	if job.Stage != model.StageOrganize {
		t.Errorf("Stage = %v, want %v", job.Stage, model.StageOrganize)
	}

	// Verify persisted
	fetched, err := env.Repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob error: %v", err)
	}
	if fetched == nil {
		t.Fatal("job not found in database")
	}
}

func TestTestEnv_CreateCompletedJob(t *testing.T) {
	env := NewTestEnv(t)

	item := env.CreateMediaItem("Test_Movie", model.MediaTypeMovie)
	job := env.CreateCompletedJob(item.ID, model.StageOrganize, "/some/output/dir")

	if job.Status != model.JobStatusCompleted {
		t.Errorf("Status = %v, want %v", job.Status, model.JobStatusCompleted)
	}
	if job.OutputDir != "/some/output/dir" {
		t.Errorf("OutputDir = %q, want %q", job.OutputDir, "/some/output/dir")
	}
	if job.Progress != 100 {
		t.Errorf("Progress = %d, want 100", job.Progress)
	}
}

func TestGenerateTestMKV(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test.mkv")

	err := GenerateTestMKV(outputPath, MKVOptions{
		DurationSec: 1,
		AudioLangs:  []string{"eng", "spa"},
		SubLangs:    []string{"eng"},
	})
	if err != nil {
		t.Fatalf("GenerateTestMKV error: %v", err)
	}

	// Verify file exists and is non-empty
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}
}

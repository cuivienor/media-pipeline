package transcode

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// testLogger implements Logger for tests
type testLogger struct {
	t *testing.T
}

func (l *testLogger) Info(format string, args ...interface{}) {
	l.t.Logf("[INFO] "+format, args...)
}

func (l *testLogger) Error(format string, args ...interface{}) {
	l.t.Logf("[ERROR] "+format, args...)
}

func TestTranscoder_Integration(t *testing.T) {
	// Skip if ffmpeg not available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}

	// Create temp directories
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(filepath.Join(inputDir, "_main"), 0755)

	// Generate a short test video
	testVideo := filepath.Join(inputDir, "_main", "test.mkv")
	err := exec.Command("ffmpeg",
		"-f", "lavfi",
		"-i", "testsrc=duration=2:size=320x240:rate=24",
		"-f", "lavfi",
		"-i", "anullsrc=r=48000:cl=stereo:d=2",
		"-c:v", "libx264", "-preset", "ultrafast",
		"-c:a", "aac",
		"-shortest",
		"-y", testVideo,
	).Run()
	if err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	// Set up database
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	repo := db.NewSQLiteRepository(database)
	ctx := context.Background()

	// Create media item and job
	item := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("Failed to create media item: %v", err)
	}

	job := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageTranscode,
		Status:      model.JobStatusInProgress,
		InputDir:    inputDir,
		OutputDir:   outputDir,
	}
	now := time.Now()
	job.StartedAt = &now
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Create transcoder
	opts := TranscodeOptions{
		CRF:    28, // Higher CRF for faster test
		Mode:   "software",
		Preset: "ultrafast",
	}
	logger := &testLogger{t}
	transcoder := NewTranscoder(repo, logger, opts)

	// Run transcode
	err = transcoder.TranscodeJob(ctx, job, inputDir, outputDir, false)
	if err != nil {
		t.Fatalf("TranscodeJob failed: %v", err)
	}

	// Verify output exists
	outputVideo := filepath.Join(outputDir, "_main", "test.mkv")
	if _, err := os.Stat(outputVideo); os.IsNotExist(err) {
		t.Error("Output video not created")
	}

	// Verify database records
	files, err := repo.ListTranscodeFiles(ctx, job.ID)
	if err != nil {
		t.Fatalf("Failed to list transcode files: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 transcode file, got %d", len(files))
	}
	if files[0].Status != model.TranscodeFileStatusCompleted {
		t.Errorf("Expected completed status, got %v", files[0].Status)
	}
	if files[0].Progress != 100 {
		t.Errorf("Expected 100%% progress, got %d%%", files[0].Progress)
	}
	if files[0].OutputSize == 0 {
		t.Error("Expected non-zero output size")
	}

	t.Logf("Input size: %d, Output size: %d, Ratio: %.1f%%",
		files[0].InputSize, files[0].OutputSize,
		float64(files[0].OutputSize)/float64(files[0].InputSize)*100)
}

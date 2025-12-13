//go:build integration

package contracts

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuivienor/media-pipeline/internal/model"
	"github.com/cuivienor/media-pipeline/internal/testutil"
	"github.com/cuivienor/media-pipeline/internal/transcode"
)

// testLogger implements transcode.Logger for tests
type testLogger struct {
	t *testing.T
}

func (l *testLogger) Info(format string, args ...interface{}) {
	l.t.Logf("[INFO] "+format, args...)
}

func (l *testLogger) Error(format string, args ...interface{}) {
	l.t.Logf("[ERROR] "+format, args...)
}

func TestTranscodeContract_MovieHappyPath(t *testing.T) {
	// Skip if ffmpeg/ffprobe not available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}

	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Setup: Create remuxed movie structure
	inputDir := env.CreateMovieStructure("Test_Movie", "2-remuxed")

	// Create database records
	item := env.CreateMediaItem("Test_Movie", model.MediaTypeMovie)
	env.CreateCompletedJob(item.ID, model.StageRemux, inputDir)

	// Create transcode job (in_progress since transcode tracks per-file)
	now := time.Now()
	transcodeJob := env.CreateJob(item.ID, model.StageTranscode)
	transcodeJob.Status = model.JobStatusInProgress
	transcodeJob.StartedAt = &now
	transcodeJob.InputDir = inputDir
	env.Repo.UpdateJobStatus(ctx, transcodeJob.ID, model.JobStatusInProgress, "")

	// Execute transcode
	outputDir := filepath.Join(env.StagingDir, "3-transcoded", "movies", "Test_Movie")
	opts := transcode.TranscodeOptions{
		CRF:    28, // Higher CRF for faster test
		Mode:   "software",
		Preset: "ultrafast",
	}
	logger := &testLogger{t}
	transcoder := transcode.NewTranscoder(env.Repo, logger, opts)

	err := transcoder.TranscodeJob(ctx, transcodeJob, inputDir, outputDir, false)
	if err != nil {
		t.Fatalf("TranscodeJob error: %v", err)
	}

	// Verify output structure
	env.AssertDirExists("staging/3-transcoded/movies/Test_Movie/_main")
	env.AssertFileNonEmpty("staging/3-transcoded/movies/Test_Movie/_main/movie.mkv")

	// Verify transcode file records
	files, err := env.Repo.ListTranscodeFiles(ctx, transcodeJob.ID)
	if err != nil {
		t.Fatalf("ListTranscodeFiles error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 transcode file, got %d", len(files))
	}
	if files[0].Status != model.TranscodeFileStatusCompleted {
		t.Errorf("status = %v, want completed", files[0].Status)
	}
	if files[0].Progress != 100 {
		t.Errorf("progress = %d, want 100", files[0].Progress)
	}
	if files[0].OutputSize == 0 {
		t.Error("output size should be non-zero")
	}
	if files[0].OutputSize >= files[0].InputSize {
		t.Logf("Warning: output size %d >= input size %d (expected compression)",
			files[0].OutputSize, files[0].InputSize)
	}

	// Mark job completed
	if err := env.Repo.UpdateJobProgress(ctx, transcodeJob.ID, 100); err != nil {
		t.Fatalf("UpdateJobProgress error: %v", err)
	}
	if err := env.Repo.UpdateJobStatus(ctx, transcodeJob.ID, model.JobStatusCompleted, ""); err != nil {
		t.Fatalf("UpdateJobStatus error: %v", err)
	}

	env.AssertJobCompleted(transcodeJob.ID)
	env.AssertInvariants()
}

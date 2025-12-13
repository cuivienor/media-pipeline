//go:build integration

package contracts

import (
	"context"
	"fmt"
	"os"
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

func TestTranscodeContract_TVHappyPath(t *testing.T) {
	// Skip if ffmpeg/ffprobe not available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}

	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Setup: Create remuxed TV structure with 2 episodes
	inputDir := env.CreateTVStructure("Test_Show", 1, 2, "2-remuxed")

	// Create database records
	item := env.CreateMediaItem("Test_Show", model.MediaTypeTV)
	env.CreateCompletedJob(item.ID, model.StageRemux, inputDir)

	// Create transcode job
	now := time.Now()
	transcodeJob := env.CreateJob(item.ID, model.StageTranscode)
	transcodeJob.Status = model.JobStatusInProgress
	transcodeJob.StartedAt = &now
	transcodeJob.InputDir = inputDir
	env.Repo.UpdateJobStatus(ctx, transcodeJob.ID, model.JobStatusInProgress, "")

	// Execute transcode
	outputDir := filepath.Join(env.StagingDir, "3-transcoded", "tv", "Test_Show", "Season_01")
	opts := transcode.TranscodeOptions{
		CRF:    28,
		Mode:   "software",
		Preset: "ultrafast",
	}
	logger := &testLogger{t}
	transcoder := transcode.NewTranscoder(env.Repo, logger, opts)

	err := transcoder.TranscodeJob(ctx, transcodeJob, inputDir, outputDir, true)
	if err != nil {
		t.Fatalf("TranscodeJob error: %v", err)
	}

	// Verify output structure - should have 2 episode files
	env.AssertDirExists("staging/3-transcoded/tv/Test_Show/Season_01/_episodes")
	env.AssertFileNonEmpty("staging/3-transcoded/tv/Test_Show/Season_01/_episodes/01.mkv")
	env.AssertFileNonEmpty("staging/3-transcoded/tv/Test_Show/Season_01/_episodes/02.mkv")

	// Verify transcode file records
	files, err := env.Repo.ListTranscodeFiles(ctx, transcodeJob.ID)
	if err != nil {
		t.Fatalf("ListTranscodeFiles error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 transcode files, got %d", len(files))
	}

	for i, f := range files {
		if f.Status != model.TranscodeFileStatusCompleted {
			t.Errorf("file %d: status = %v, want completed", i, f.Status)
		}
		if f.Progress != 100 {
			t.Errorf("file %d: progress = %d, want 100", i, f.Progress)
		}
		if f.OutputSize == 0 {
			t.Errorf("file %d: output size should be non-zero", i)
		}
		if f.OutputSize >= f.InputSize {
			t.Logf("Warning: file %d output size %d >= input size %d (expected compression)",
				i, f.OutputSize, f.InputSize)
		}
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

func TestTranscodeContract_ResumeAfterInterruption(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}

	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Setup: Create TV structure with 3 episodes
	inputDir := env.CreateTVStructure("Test_Show", 1, 3, "2-remuxed")

	// Create database records
	item := env.CreateMediaItem("Test_Show", model.MediaTypeTV)
	env.CreateCompletedJob(item.ID, model.StageRemux, inputDir)

	// Create transcode job
	now := time.Now()
	transcodeJob := env.CreateJob(item.ID, model.StageTranscode)
	transcodeJob.Status = model.JobStatusInProgress
	transcodeJob.StartedAt = &now
	transcodeJob.InputDir = inputDir
	if err := env.Repo.UpdateJob(ctx, transcodeJob); err != nil {
		t.Fatalf("UpdateJob error: %v", err)
	}

	outputDir := filepath.Join(env.StagingDir, "3-transcoded", "tv", "Test_Show", "Season_01")
	opts := transcode.TranscodeOptions{
		CRF:    28,
		Mode:   "software",
		Preset: "ultrafast",
	}
	logger := &testLogger{t}
	transcoder := transcode.NewTranscoder(env.Repo, logger, opts)

	// First run: Complete transcoding
	err := transcoder.TranscodeJob(ctx, transcodeJob, inputDir, outputDir, true)
	if err != nil {
		t.Fatalf("First TranscodeJob error: %v", err)
	}

	// Verify 3 files completed
	files, err := env.Repo.ListTranscodeFiles(ctx, transcodeJob.ID)
	if err != nil {
		t.Fatalf("ListTranscodeFiles error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 transcode files, got %d", len(files))
	}

	completedCount := 0
	for _, f := range files {
		if f.Status == model.TranscodeFileStatusCompleted {
			completedCount++
		}
	}
	if completedCount != 3 {
		t.Fatalf("expected 3 completed files, got %d", completedCount)
	}

	// "Interrupt" by resetting one file to pending and deleting its output
	file := &files[1]
	file.Status = model.TranscodeFileStatusPending
	file.Progress = 0
	file.OutputSize = 0
	if err := env.Repo.UpdateTranscodeFile(ctx, file); err != nil {
		t.Fatalf("UpdateTranscodeFile error: %v", err)
	}

	outputPath := filepath.Join(outputDir, file.RelativePath)
	os.Remove(outputPath)

	// Re-fetch job to get current state
	resumeJob, err := env.Repo.GetJob(ctx, transcodeJob.ID)
	if err != nil {
		t.Fatalf("GetJob error: %v", err)
	}
	resumeJob.InputDir = inputDir

	// Second run: Should only process the reset file
	transcoder2 := transcode.NewTranscoder(env.Repo, logger, opts)
	err = transcoder2.TranscodeJob(ctx, resumeJob, inputDir, outputDir, true)
	if err != nil {
		t.Fatalf("Resume TranscodeJob error: %v", err)
	}

	// Verify all files are completed again
	files, err = env.Repo.ListTranscodeFiles(ctx, transcodeJob.ID)
	if err != nil {
		t.Fatalf("ListTranscodeFiles error: %v", err)
	}
	completedCount = 0
	for _, f := range files {
		if f.Status == model.TranscodeFileStatusCompleted {
			completedCount++
		}
	}
	if completedCount != 3 {
		t.Errorf("after resume: expected 3 completed files, got %d", completedCount)
	}

	// Verify output files exist
	for i := 1; i <= 3; i++ {
		relPath := filepath.Join("staging/3-transcoded/tv/Test_Show/Season_01/_episodes",
			fmt.Sprintf("%02d.mkv", i))
		env.AssertFileNonEmpty(relPath)
	}

	env.AssertInvariants()
}

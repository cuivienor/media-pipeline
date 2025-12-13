// internal/pipeline/contracts/remux_test.go
//go:build integration

package contracts

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/model"
	"github.com/cuivienor/media-pipeline/internal/remux"
	"github.com/cuivienor/media-pipeline/internal/testutil"
)

func TestRemuxContract_MovieHappyPath(t *testing.T) {
	// Skip if tools not installed
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("mkvmerge"); err != nil {
		t.Skip("mkvmerge not installed")
	}

	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Setup: Create organized movie with multi-language tracks
	inputDir := env.CreateMovieStructure("Test_Movie", "1-ripped")

	// Create database records
	item := env.CreateMediaItem("Test_Movie", model.MediaTypeMovie)
	env.CreateCompletedJob(item.ID, model.StageOrganize, inputDir)

	// Create pending remux job
	remuxJob := env.CreateJob(item.ID, model.StageRemux)

	// Execute remux
	outputDir := filepath.Join(env.StagingDir, "2-remuxed", "movies", "Test_Movie")
	remuxer := remux.NewRemuxer([]string{"eng"}) // Keep only English

	results, err := remuxer.RemuxDirectory(ctx, inputDir, outputDir, false)
	if err != nil {
		t.Fatalf("RemuxDirectory error: %v", err)
	}

	// Verify output structure
	env.AssertDirExists("staging/2-remuxed/movies/Test_Movie/_main")
	env.AssertFileNonEmpty("staging/2-remuxed/movies/Test_Movie/_main/movie.mkv")

	// Verify tracks were filtered (input had eng, spa, fra; output should only have eng)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	result := results[0]
	if result.TracksRemoved == 0 {
		t.Error("expected some tracks to be removed")
	}
	t.Logf("Tracks removed: %d", result.TracksRemoved)

	// Mark job completed (simulating what cmd/remux would do)
	if err := env.Repo.UpdateJobStatus(ctx, remuxJob.ID, model.JobStatusCompleted, ""); err != nil {
		t.Fatalf("UpdateJobStatus error: %v", err)
	}

	// Verify job state
	env.AssertJobCompleted(remuxJob.ID)
}

func TestRemuxContract_TVHappyPath(t *testing.T) {
	// Skip if tools not installed
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("mkvmerge"); err != nil {
		t.Skip("mkvmerge not installed")
	}

	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Setup: Create organized TV season with 3 episodes
	inputDir := env.CreateTVStructure("Test_Show", 1, 3, "1-ripped")

	// Create database records
	item := env.CreateMediaItem("Test_Show", model.MediaTypeTV)
	env.CreateCompletedJob(item.ID, model.StageOrganize, inputDir)

	// Create pending remux job
	remuxJob := env.CreateJob(item.ID, model.StageRemux)

	// Execute remux
	outputDir := filepath.Join(env.StagingDir, "2-remuxed", "tv", "Test_Show", "Season_01")
	remuxer := remux.NewRemuxer([]string{"eng"})

	results, err := remuxer.RemuxDirectory(ctx, inputDir, outputDir, true) // isTV=true
	if err != nil {
		t.Fatalf("RemuxDirectory error: %v", err)
	}

	// Verify output structure - should have 3 episode files
	env.AssertDirExists("staging/2-remuxed/tv/Test_Show/Season_01/_episodes")

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify each episode was processed
	for i := 1; i <= 3; i++ {
		relPath := filepath.Join("staging/2-remuxed/tv/Test_Show/Season_01/_episodes",
			fmt.Sprintf("%02d.mkv", i))
		env.AssertFileNonEmpty(relPath)
	}

	// Mark job completed
	if err := env.Repo.UpdateJobStatus(ctx, remuxJob.ID, model.JobStatusCompleted, ""); err != nil {
		t.Fatalf("UpdateJobStatus error: %v", err)
	}

	env.AssertJobCompleted(remuxJob.ID)
}

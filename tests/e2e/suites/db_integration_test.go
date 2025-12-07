package suites

import (
	"context"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/model"
	"github.com/cuivienor/media-pipeline/internal/ripper"
	"github.com/cuivienor/media-pipeline/tests/e2e/testenv"
)

func TestRipper_E2E_DualWrite(t *testing.T) {
	requireFFmpeg(t)
	mockPath := findMockMakeMKV(t)

	env := testenv.New(t)
	dbFixture := testenv.NewDBFixture(t)

	// Create ripper with dual-write state manager
	runner := ripper.NewMakeMKVRunner(mockPath)
	stateManager := ripper.NewDualWriteStateManager(
		ripper.NewStateManager(),
		dbFixture.Repo,
	)
	r := ripper.NewRipper(env.StagingBase, runner, stateManager)

	req := &ripper.RipRequest{
		Type:     ripper.MediaTypeMovie,
		Name:     "Dual Write Test",
		DiscPath: "disc:0",
	}

	result, err := r.Rip(context.Background(), req)
	if err != nil {
		t.Fatalf("Rip failed: %v", err)
	}

	// Verify filesystem
	stateDir, err := testenv.FindStateDir(result.OutputDir, ".rip")
	if err != nil {
		t.Fatal("filesystem state not created")
	}
	stateDir.AssertStatus(t, model.StatusCompleted)

	// Verify database
	item, err := dbFixture.Repo.GetMediaItemBySafeName(
		context.Background(),
		"Dual_Write_Test",
		nil,
	)
	if err != nil {
		t.Fatalf("failed to query media item: %v", err)
	}
	if item == nil {
		t.Fatal("media item not in database")
	}

	// Verify media item fields
	if item.Type != model.MediaTypeMovie {
		t.Errorf("item.Type = %v, want %v", item.Type, model.MediaTypeMovie)
	}
	if item.Name != "Dual Write Test" {
		t.Errorf("item.Name = %q, want %q", item.Name, "Dual Write Test")
	}
	if item.SafeName != "Dual_Write_Test" {
		t.Errorf("item.SafeName = %q, want %q", item.SafeName, "Dual_Write_Test")
	}

	// Verify job records
	jobs, err := dbFixture.Repo.ListJobsForMedia(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	job := jobs[0]
	if job.Status != model.JobStatusCompleted {
		t.Errorf("job.Status = %v, want completed", job.Status)
	}
	if job.Stage != model.StageRip {
		t.Errorf("job.Stage = %v, want rip", job.Stage)
	}
	if job.MediaItemID != item.ID {
		t.Errorf("job.MediaItemID = %d, want %d", job.MediaItemID, item.ID)
	}
	if job.OutputDir != result.OutputDir {
		t.Errorf("job.OutputDir = %q, want %q", job.OutputDir, result.OutputDir)
	}
	if job.Disc != nil {
		t.Errorf("job.Disc = %v, want nil for movie", job.Disc)
	}
}

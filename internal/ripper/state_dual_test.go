package ripper

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/model"
	"github.com/cuivienor/media-pipeline/tests/e2e/testenv"
)

func TestDualWriteStateManager_Initialize_Movie(t *testing.T) {
	env := testenv.New(t)
	dbFixture := testenv.NewDBFixture(t)

	dm := NewDualWriteStateManager(
		NewStateManager(),
		dbFixture.Repo,
	)

	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Test Movie",
	}

	outputDir := filepath.Join(env.RippedMoviesDir(), "Test_Movie")
	err := dm.Initialize(outputDir, req)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify filesystem state created
	stateDir, err := testenv.FindStateDir(outputDir, ".rip")
	if err != nil {
		t.Fatal("filesystem state not created")
	}
	stateDir.AssertStatus(t, model.StatusInProgress)

	// Verify database state created
	item, err := dbFixture.Repo.GetMediaItemBySafeName(context.Background(), "Test_Movie", nil)
	if err != nil {
		t.Fatalf("media item not in database: %v", err)
	}
	if item == nil {
		t.Fatal("media item not found")
	}

	jobs, err := dbFixture.Repo.ListJobsForMedia(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Status != model.JobStatusInProgress {
		t.Errorf("job status = %v, want in_progress", jobs[0].Status)
	}
	if jobs[0].Stage != model.StageRip {
		t.Errorf("job stage = %v, want rip", jobs[0].Stage)
	}
}

func TestDualWriteStateManager_Initialize_TV(t *testing.T) {
	env := testenv.New(t)
	dbFixture := testenv.NewDBFixture(t)

	dm := NewDualWriteStateManager(
		NewStateManager(),
		dbFixture.Repo,
	)

	req := &RipRequest{
		Type:   MediaTypeTV,
		Name:   "Test Show",
		Season: 2,
		Disc:   1,
	}

	outputDir := filepath.Join(env.RippedTVDir(), "Test_Show_S02_D01")
	err := dm.Initialize(outputDir, req)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify filesystem state created
	stateDir, err := testenv.FindStateDir(outputDir, ".rip")
	if err != nil {
		t.Fatal("filesystem state not created")
	}
	stateDir.AssertStatus(t, model.StatusInProgress)

	// Verify database state created with correct season and disc
	season := 2
	item, err := dbFixture.Repo.GetMediaItemBySafeName(context.Background(), "Test_Show", &season)
	if err != nil {
		t.Fatalf("media item not in database: %v", err)
	}
	if item == nil {
		t.Fatal("media item not found")
	}
	if item.Season == nil || *item.Season != 2 {
		t.Errorf("item season = %v, want 2", item.Season)
	}

	jobs, err := dbFixture.Repo.ListJobsForMedia(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Disc == nil || *jobs[0].Disc != 1 {
		t.Errorf("job disc = %v, want 1", jobs[0].Disc)
	}
}

func TestDualWriteStateManager_Complete(t *testing.T) {
	env := testenv.New(t)
	dbFixture := testenv.NewDBFixture(t)

	dm := NewDualWriteStateManager(
		NewStateManager(),
		dbFixture.Repo,
	)

	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Complete Test",
	}

	outputDir := filepath.Join(env.RippedMoviesDir(), "Complete_Test")
	if err := dm.Initialize(outputDir, req); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Complete the operation
	if err := dm.Complete(outputDir); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Verify filesystem status updated
	stateDir, err := testenv.FindStateDir(outputDir, ".rip")
	if err != nil {
		t.Fatal("filesystem state not found")
	}
	stateDir.AssertStatus(t, model.StatusCompleted)

	// Verify database job status updated
	item, _ := dbFixture.Repo.GetMediaItemBySafeName(context.Background(), "Complete_Test", nil)
	jobs, _ := dbFixture.Repo.ListJobsForMedia(context.Background(), item.ID)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Status != model.JobStatusCompleted {
		t.Errorf("job status = %v, want completed", jobs[0].Status)
	}
	if jobs[0].CompletedAt == nil {
		t.Error("job completed_at should be set")
	}
}

func TestDualWriteStateManager_SetStatus(t *testing.T) {
	env := testenv.New(t)
	dbFixture := testenv.NewDBFixture(t)

	dm := NewDualWriteStateManager(
		NewStateManager(),
		dbFixture.Repo,
	)

	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Status Test",
	}

	outputDir := filepath.Join(env.RippedMoviesDir(), "Status_Test")
	if err := dm.Initialize(outputDir, req); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Update status to pending
	if err := dm.SetStatus(outputDir, model.StatusPending); err != nil {
		t.Fatalf("SetStatus failed: %v", err)
	}

	// Verify filesystem status
	stateDir, err := testenv.FindStateDir(outputDir, ".rip")
	if err != nil {
		t.Fatal("filesystem state not found")
	}
	stateDir.AssertStatus(t, model.StatusPending)

	// Verify database status (note: SetStatus doesn't update job status in DB)
	// It only updates filesystem - job status changes via Complete/SetError
}

func TestDualWriteStateManager_SetError(t *testing.T) {
	env := testenv.New(t)
	dbFixture := testenv.NewDBFixture(t)

	dm := NewDualWriteStateManager(
		NewStateManager(),
		dbFixture.Repo,
	)

	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Error Test",
	}

	outputDir := filepath.Join(env.RippedMoviesDir(), "Error_Test")
	if err := dm.Initialize(outputDir, req); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Set error
	testErr := os.ErrNotExist
	if err := dm.SetError(outputDir, testErr); err != nil {
		t.Fatalf("SetError failed: %v", err)
	}

	// Verify filesystem error file created
	errorPath := filepath.Join(outputDir, ".rip", "error")
	if _, err := os.Stat(errorPath); err != nil {
		t.Error("filesystem error file not created")
	}

	// Verify database job marked as failed
	item, _ := dbFixture.Repo.GetMediaItemBySafeName(context.Background(), "Error_Test", nil)
	jobs, _ := dbFixture.Repo.ListJobsForMedia(context.Background(), item.ID)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Status != model.JobStatusFailed {
		t.Errorf("job status = %v, want failed", jobs[0].Status)
	}
	if jobs[0].ErrorMessage == "" {
		t.Error("job error message should be set")
	}
}

func TestDualWriteStateManager_ReuseExistingMediaItem(t *testing.T) {
	env := testenv.New(t)
	dbFixture := testenv.NewDBFixture(t)

	// Pre-create a media item
	existingItem := dbFixture.CreateMovie("Existing Movie", "Existing_Movie")

	dm := NewDualWriteStateManager(
		NewStateManager(),
		dbFixture.Repo,
	)

	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Existing Movie",
	}

	outputDir := filepath.Join(env.RippedMoviesDir(), "Existing_Movie")
	if err := dm.Initialize(outputDir, req); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify media item was reused (not duplicated)
	items, err := dbFixture.Repo.ListMediaItems(context.Background(), db.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list items: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 media item (reused), got %d", len(items))
	}

	// Verify job was created for existing item
	jobs, _ := dbFixture.Repo.ListJobsForMedia(context.Background(), existingItem.ID)
	if len(jobs) != 1 {
		t.Errorf("expected 1 job for existing item, got %d", len(jobs))
	}
}

func TestDualWriteStateManager_FilesystemFailureDoesNotFailOperation(t *testing.T) {
	_ = testenv.New(t)
	dbFixture := testenv.NewDBFixture(t)

	dm := NewDualWriteStateManager(
		NewStateManager(),
		dbFixture.Repo,
	)

	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "FS Fail Test",
	}

	// Use an invalid path that will cause filesystem write to fail
	// but DB writes should still succeed
	outputDir := "/proc/invalid/path/that/cannot/be/created"

	// Initialize should succeed even if filesystem write fails
	// (DB is authoritative, filesystem is best-effort)
	err := dm.Initialize(outputDir, req)
	// We expect this to succeed because DB write succeeds
	if err != nil {
		t.Fatalf("Initialize should not fail when filesystem write fails: %v", err)
	}

	// Verify database state was created despite filesystem failure
	item, err := dbFixture.Repo.GetMediaItemBySafeName(context.Background(), "FS_Fail_Test", nil)
	if err != nil {
		t.Fatalf("media item not in database: %v", err)
	}
	if item == nil {
		t.Fatal("media item should exist in DB even with filesystem failure")
	}

	jobs, _ := dbFixture.Repo.ListJobsForMedia(context.Background(), item.ID)
	if len(jobs) != 1 {
		t.Errorf("expected 1 job in DB, got %d", len(jobs))
	}
}

func TestDualWriteStateManager_WithJobID_ResumesExistingJob(t *testing.T) {
	env := testenv.New(t)
	dbFixture := testenv.NewDBFixture(t)

	ctx := context.Background()

	// Pre-create a media item and pending job (simulating TUI dispatch)
	item := dbFixture.CreateMovie("Resume Test", "Resume_Test")
	job := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusPending,
	}
	if err := dbFixture.Repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("failed to create pending job: %v", err)
	}

	// Create state manager with job ID
	dm := NewDualWriteStateManager(
		NewStateManager(),
		dbFixture.Repo,
	).WithJobID(job.ID)

	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Resume Test",
	}

	outputDir := filepath.Join(env.RippedMoviesDir(), "Resume_Test")

	// Initialize should resume the existing job, not create a new one
	if err := dm.Initialize(outputDir, req); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify filesystem state created
	stateDir, err := testenv.FindStateDir(outputDir, ".rip")
	if err != nil {
		t.Fatal("filesystem state not created")
	}
	stateDir.AssertStatus(t, model.StatusInProgress)

	// Verify job was updated (not duplicated)
	jobs, err := dbFixture.Repo.ListJobsForMedia(ctx, item.ID)
	if err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected 1 job (resumed), got %d", len(jobs))
	}

	// Verify job was updated to in_progress
	updatedJob := jobs[0]
	if updatedJob.Status != model.JobStatusInProgress {
		t.Errorf("job status = %v, want in_progress", updatedJob.Status)
	}
	if updatedJob.OutputDir != outputDir {
		t.Errorf("job output_dir = %q, want %q", updatedJob.OutputDir, outputDir)
	}
	if updatedJob.StartedAt == nil {
		t.Error("job started_at should be set")
	}
}

func TestDualWriteStateManager_WithJobID_TVShow(t *testing.T) {
	env := testenv.New(t)
	dbFixture := testenv.NewDBFixture(t)

	ctx := context.Background()

	// Pre-create a TV show item and pending job
	season := 2
	disc := 3
	item := dbFixture.CreateTVSeason("Test Show", "Test_Show", season)
	job := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusPending,
		Disc:        &disc,
	}
	if err := dbFixture.Repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("failed to create pending job: %v", err)
	}

	// Create state manager with job ID
	dm := NewDualWriteStateManager(
		NewStateManager(),
		dbFixture.Repo,
	).WithJobID(job.ID)

	req := &RipRequest{
		Type:   MediaTypeTV,
		Name:   "Test Show",
		Season: 2,
		Disc:   3,
	}

	outputDir := filepath.Join(env.RippedTVDir(), "Test_Show_S02_D03")

	// Initialize should resume the existing job
	if err := dm.Initialize(outputDir, req); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify no duplicate jobs
	jobs, err := dbFixture.Repo.ListJobsForMedia(ctx, item.ID)
	if err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}

	// Verify job has correct disc
	if jobs[0].Disc == nil || *jobs[0].Disc != 3 {
		t.Errorf("job disc = %v, want 3", jobs[0].Disc)
	}
}

func TestDualWriteStateManager_WithJobID_JobNotFound(t *testing.T) {
	env := testenv.New(t)
	dbFixture := testenv.NewDBFixture(t)

	// Create state manager with non-existent job ID
	dm := NewDualWriteStateManager(
		NewStateManager(),
		dbFixture.Repo,
	).WithJobID(999)

	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Test",
	}

	outputDir := filepath.Join(env.RippedMoviesDir(), "Test")

	// Initialize should fail because job doesn't exist
	err := dm.Initialize(outputDir, req)
	if err == nil {
		t.Error("expected error for non-existent job")
	}
}

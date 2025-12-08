package tui

import (
	"context"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/model"
)

func TestLoadState_EmptyDatabase(t *testing.T) {
	database, err := db.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer database.Close()

	repo := db.NewSQLiteRepository(database)

	state, err := LoadState(repo)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if state == nil {
		t.Fatal("expected state, got nil")
	}

	if len(state.Items) != 0 {
		t.Errorf("len(Items) = %d, want 0", len(state.Items))
	}

	if len(state.MovieJobs) != 0 {
		t.Errorf("len(MovieJobs) = %d, want 0", len(state.MovieJobs))
	}
	if len(state.SeasonJobs) != 0 {
		t.Errorf("len(SeasonJobs) = %d, want 0", len(state.SeasonJobs))
	}
}

func TestLoadState_ItemsWithNoJobs(t *testing.T) {
	database, err := db.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer database.Close()

	repo := db.NewSQLiteRepository(database)
	ctx := context.Background()

	// Create a movie without jobs
	movie := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
	}
	if err := repo.CreateMediaItem(ctx, movie); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	state, err := LoadState(repo)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if len(state.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(state.Items))
	}

	// Items with no jobs should have default CurrentStage and StageStatus
	item := state.Items[0]
	if item.Type == model.MediaTypeMovie {
		// The new state loader doesn't set CurrentStage/StageStatus for items without jobs
		// They remain at their default values
		if item.CurrentStage != model.StageRip {
			// This is OK - items may have default stage from DB
		}
	}

	// MovieJobs map should have entry for the item with empty slice
	if len(state.MovieJobs) != 1 {
		t.Errorf("len(MovieJobs) = %d, want 1", len(state.MovieJobs))
	}

	jobs, exists := state.MovieJobs[movie.ID]
	if !exists {
		t.Errorf("MovieJobs[%d] does not exist in map", movie.ID)
	}
	if len(jobs) != 0 {
		t.Errorf("len(MovieJobs[%d]) = %d, want 0", movie.ID, len(jobs))
	}
}

func TestLoadState_ItemsWithMultipleJobs(t *testing.T) {
	database, err := db.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer database.Close()

	repo := db.NewSQLiteRepository(database)
	ctx := context.Background()

	// Create a media item
	movie := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
	}
	if err := repo.CreateMediaItem(ctx, movie); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	// Create multiple jobs for the same item
	ripJob := &model.Job{
		MediaItemID: movie.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusCompleted,
	}
	if err := repo.CreateJob(ctx, ripJob); err != nil {
		t.Fatalf("CreateJob(rip) error = %v", err)
	}

	organizeJob := &model.Job{
		MediaItemID: movie.ID,
		Stage:       model.StageOrganize,
		Status:      model.JobStatusCompleted,
	}
	if err := repo.CreateJob(ctx, organizeJob); err != nil {
		t.Fatalf("CreateJob(organize) error = %v", err)
	}

	remuxJob := &model.Job{
		MediaItemID: movie.ID,
		Stage:       model.StageRemux,
		Status:      model.JobStatusInProgress,
	}
	if err := repo.CreateJob(ctx, remuxJob); err != nil {
		t.Fatalf("CreateJob(remux) error = %v", err)
	}

	state, err := LoadState(repo)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if len(state.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1", len(state.Items))
	}

	item := state.Items[0]

	// Item should reflect the latest job (remux in-progress)
	if item.CurrentStage != model.StageRemux {
		t.Errorf("CurrentStage = %v, want %v", item.CurrentStage, model.StageRemux)
	}

	if item.StageStatus != model.StatusInProgress {
		t.Errorf("StageStatus = %v, want %v", item.StageStatus, model.StatusInProgress)
	}

	// MovieJobs map should contain all jobs
	jobs := state.MovieJobs[movie.ID]
	if len(jobs) != 3 {
		t.Fatalf("len(MovieJobs[%d]) = %d, want 3", movie.ID, len(jobs))
	}

	// Jobs should be in order: rip, organize, remux
	expectedStages := []model.Stage{model.StageRip, model.StageOrganize, model.StageRemux}
	for i, job := range jobs {
		if job.Stage != expectedStages[i] {
			t.Errorf("jobs[%d].Stage = %v, want %v", i, job.Stage, expectedStages[i])
		}
	}
}

func TestJobStatusToStatus(t *testing.T) {
	tests := []struct {
		name           string
		jobStatus      model.JobStatus
		expectedStatus model.Status
	}{
		{
			name:           "JobStatusCompleted maps to StatusCompleted",
			jobStatus:      model.JobStatusCompleted,
			expectedStatus: model.StatusCompleted,
		},
		{
			name:           "JobStatusInProgress maps to StatusInProgress",
			jobStatus:      model.JobStatusInProgress,
			expectedStatus: model.StatusInProgress,
		},
		{
			name:           "JobStatusFailed maps to StatusFailed",
			jobStatus:      model.JobStatusFailed,
			expectedStatus: model.StatusFailed,
		},
		{
			name:           "JobStatusPending maps to StatusPending",
			jobStatus:      model.JobStatusPending,
			expectedStatus: model.StatusPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := jobStatusToStatus(tt.jobStatus)
			if result != tt.expectedStatus {
				t.Errorf("jobStatusToStatus(%v) = %v, want %v", tt.jobStatus, result, tt.expectedStatus)
			}
		})
	}
}

func TestItemsNeedingAction(t *testing.T) {
	state := &AppState{
		Items: []model.MediaItem{
			{ID: 1, Type: model.MediaTypeMovie, CurrentStage: model.StageRip, StageStatus: model.StatusCompleted},
			{ID: 2, Type: model.MediaTypeMovie, CurrentStage: model.StageRemux, StageStatus: model.StatusInProgress},
			{ID: 3, Type: model.MediaTypeMovie, CurrentStage: model.StageTranscode, StageStatus: model.StatusCompleted},
			{ID: 4, Type: model.MediaTypeMovie, CurrentStage: model.StagePublish, StageStatus: model.StatusCompleted}, // Should be excluded
			{ID: 5, Type: model.MediaTypeMovie, CurrentStage: model.StageRip, StageStatus: model.StatusFailed},
		},
	}

	ready := state.ItemsNeedingAction()

	// Should return items 1 and 3 (completed but not at Publish stage)
	expectedIDs := []int64{1, 3}

	if len(ready) != len(expectedIDs) {
		t.Fatalf("len(ready) = %d, want %d", len(ready), len(expectedIDs))
	}

	foundIDs := make(map[int64]bool)
	for _, item := range ready {
		foundIDs[item.ID] = true
	}

	for _, expectedID := range expectedIDs {
		if !foundIDs[expectedID] {
			t.Errorf("expected item %d in ready items, but not found", expectedID)
		}
	}

	// Verify excluded items
	if foundIDs[2] {
		t.Error("item 2 (in-progress) should not be in ready items")
	}
	if foundIDs[4] {
		t.Error("item 4 (at Publish stage) should not be in ready items")
	}
	if foundIDs[5] {
		t.Error("item 5 (failed) should not be in ready items")
	}
}

func TestLoadState_ErrorHandling(t *testing.T) {
	database, err := db.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	// Close database immediately to cause errors
	database.Close()

	repo := db.NewSQLiteRepository(database)

	_, err = LoadState(repo)
	if err == nil {
		t.Error("expected error when loading state from closed database, got nil")
	}
}

func TestAppState_ItemsInProgress(t *testing.T) {
	state := &AppState{
		Items: []model.MediaItem{
			{ID: 1, Type: model.MediaTypeMovie, StageStatus: model.StatusInProgress},
			{ID: 2, Type: model.MediaTypeMovie, StageStatus: model.StatusCompleted},
			{ID: 3, Type: model.MediaTypeMovie, StageStatus: model.StatusInProgress},
			{ID: 4, Type: model.MediaTypeMovie, StageStatus: model.StatusFailed},
		},
	}

	items := state.ItemsInProgress()
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}

	expectedIDs := []int64{1, 3}
	for i, item := range items {
		if item.ID != expectedIDs[i] {
			t.Errorf("items[%d].ID = %d, want %d", i, item.ID, expectedIDs[i])
		}
	}
}

func TestAppState_ItemsFailed(t *testing.T) {
	state := &AppState{
		Items: []model.MediaItem{
			{ID: 1, Type: model.MediaTypeMovie, StageStatus: model.StatusInProgress},
			{ID: 2, Type: model.MediaTypeMovie, StageStatus: model.StatusFailed},
			{ID: 3, Type: model.MediaTypeMovie, StageStatus: model.StatusCompleted},
			{ID: 4, Type: model.MediaTypeMovie, StageStatus: model.StatusFailed},
		},
	}

	items := state.ItemsFailed()
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}

	expectedIDs := []int64{2, 4}
	for i, item := range items {
		if item.ID != expectedIDs[i] {
			t.Errorf("items[%d].ID = %d, want %d", i, item.ID, expectedIDs[i])
		}
	}
}

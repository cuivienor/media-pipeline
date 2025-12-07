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

	if len(state.Jobs) != 0 {
		t.Errorf("len(Jobs) = %d, want 0", len(state.Jobs))
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

	// Create media items without jobs
	movie := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
	}
	if err := repo.CreateMediaItem(ctx, movie); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	season := 1
	tv := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Test Show",
		SafeName: "Test_Show",
		Season:   &season,
	}
	if err := repo.CreateMediaItem(ctx, tv); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	state, err := LoadState(repo)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if len(state.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(state.Items))
	}

	// Items with no jobs should have default stage (StageRip = 0) and status
	for _, item := range state.Items {
		if item.Current != model.StageRip {
			t.Errorf("item %d Current = %v, want %v", item.ID, item.Current, model.StageRip)
		}
		// Status should remain as zero value (empty string) since no jobs exist
		if item.Status != "" {
			t.Errorf("item %d Status = %q, want empty string", item.ID, item.Status)
		}
	}

	// Jobs map should have entries for each item with empty slices
	if len(state.Jobs) != 2 {
		t.Errorf("len(Jobs) = %d, want 2", len(state.Jobs))
	}

	for _, item := range state.Items {
		jobs, exists := state.Jobs[item.ID]
		if !exists {
			t.Errorf("Jobs[%d] does not exist in map", item.ID)
		}
		if len(jobs) != 0 {
			t.Errorf("len(Jobs[%d]) = %d, want 0", item.ID, len(jobs))
		}
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
	if item.Current != model.StageRemux {
		t.Errorf("Current = %v, want %v", item.Current, model.StageRemux)
	}

	if item.Status != model.StatusInProgress {
		t.Errorf("Status = %v, want %v", item.Status, model.StatusInProgress)
	}

	// Jobs map should contain all jobs
	jobs := state.Jobs[movie.ID]
	if len(jobs) != 3 {
		t.Fatalf("len(Jobs[%d]) = %d, want 3", movie.ID, len(jobs))
	}

	// Jobs should be in order: rip, organize, remux
	expectedStages := []model.Stage{model.StageRip, model.StageOrganize, model.StageRemux}
	for i, job := range jobs {
		if job.Stage != expectedStages[i] {
			t.Errorf("jobs[%d].Stage = %v, want %v", i, job.Stage, expectedStages[i])
		}
	}
}

func TestUpdateItemFromJob_StatusMapping(t *testing.T) {
	database, err := db.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer database.Close()

	repo := db.NewSQLiteRepository(database)
	state := &PipelineState{
		Items: []model.MediaItem{},
		Jobs:  make(map[int64][]model.Job),
	}

	tests := []struct {
		name           string
		jobStatus      model.JobStatus
		expectedStatus model.Status
		stage          model.Stage
	}{
		{
			name:           "JobStatusCompleted maps to StatusCompleted",
			jobStatus:      model.JobStatusCompleted,
			expectedStatus: model.StatusCompleted,
			stage:          model.StageRip,
		},
		{
			name:           "JobStatusInProgress maps to StatusInProgress",
			jobStatus:      model.JobStatusInProgress,
			expectedStatus: model.StatusInProgress,
			stage:          model.StageRemux,
		},
		{
			name:           "JobStatusFailed maps to StatusFailed",
			jobStatus:      model.JobStatusFailed,
			expectedStatus: model.StatusFailed,
			stage:          model.StageTranscode,
		},
		{
			name:           "JobStatusPending maps to StatusPending",
			jobStatus:      model.JobStatusPending,
			expectedStatus: model.StatusPending,
			stage:          model.StageOrganize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &model.MediaItem{
				Type:     model.MediaTypeMovie,
				Name:     "Test",
				SafeName: "Test",
			}

			job := model.Job{
				MediaItemID: 1,
				Stage:       tt.stage,
				Status:      tt.jobStatus,
			}

			state.updateItemFromJob(item, job)

			if item.Status != tt.expectedStatus {
				t.Errorf("Status = %v, want %v", item.Status, tt.expectedStatus)
			}

			if item.Current != tt.stage {
				t.Errorf("Current = %v, want %v", item.Current, tt.stage)
			}
		})
	}

	_ = repo // Keep repo for consistency with test pattern
}

func TestCountByStage(t *testing.T) {
	state := &PipelineState{
		Items: []model.MediaItem{
			{Current: model.StageRip, Status: model.StatusInProgress},
			{Current: model.StageRip, Status: model.StatusCompleted},
			{Current: model.StageRemux, Status: model.StatusInProgress},
			{Current: model.StageTranscode, Status: model.StatusCompleted},
			{Current: model.StageTranscode, Status: model.StatusFailed},
			{Current: model.StagePublish, Status: model.StatusCompleted},
		},
	}

	counts := state.CountByStage()

	expected := map[model.Stage]int{
		model.StageRip:       2,
		model.StageRemux:     1,
		model.StageTranscode: 2,
		model.StagePublish:   1,
	}

	if len(counts) != len(expected) {
		t.Errorf("len(counts) = %d, want %d", len(counts), len(expected))
	}

	for stage, expectedCount := range expected {
		if counts[stage] != expectedCount {
			t.Errorf("counts[%v] = %d, want %d", stage, counts[stage], expectedCount)
		}
	}

	// Verify stages with no items are not in the map
	if _, exists := counts[model.StageOrganize]; exists {
		t.Errorf("counts[StageOrganize] should not exist, got %d", counts[model.StageOrganize])
	}
}

func TestItemsReadyForNextStage(t *testing.T) {
	state := &PipelineState{
		Items: []model.MediaItem{
			{ID: 1, Current: model.StageRip, Status: model.StatusCompleted},
			{ID: 2, Current: model.StageRemux, Status: model.StatusInProgress},
			{ID: 3, Current: model.StageTranscode, Status: model.StatusCompleted},
			{ID: 4, Current: model.StagePublish, Status: model.StatusCompleted}, // Should be excluded
			{ID: 5, Current: model.StageRip, Status: model.StatusFailed},
			{ID: 6, Current: model.StageOrganize, Status: model.StatusCompleted},
		},
	}

	ready := state.ItemsReadyForNextStage()

	// Should return items 1, 3, and 6 (completed but not at Publish stage)
	expectedIDs := []int64{1, 3, 6}

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

func TestGetJobsForItem(t *testing.T) {
	jobs1 := []model.Job{
		{ID: 1, MediaItemID: 100, Stage: model.StageRip},
		{ID: 2, MediaItemID: 100, Stage: model.StageRemux},
	}

	jobs2 := []model.Job{
		{ID: 3, MediaItemID: 200, Stage: model.StageRip},
	}

	state := &PipelineState{
		Jobs: map[int64][]model.Job{
			100: jobs1,
			200: jobs2,
		},
	}

	t.Run("get existing item jobs", func(t *testing.T) {
		retrieved := state.GetJobsForItem(100)
		if len(retrieved) != 2 {
			t.Errorf("len(retrieved) = %d, want 2", len(retrieved))
		}
	})

	t.Run("get non-existent item jobs", func(t *testing.T) {
		retrieved := state.GetJobsForItem(999)
		if retrieved != nil {
			t.Errorf("expected nil for non-existent item, got %v", retrieved)
		}
	})
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

func TestPipelineState_ItemsAtStage(t *testing.T) {
	state := &PipelineState{
		Items: []model.MediaItem{
			{ID: 1, Current: model.StageRip},
			{ID: 2, Current: model.StageRip},
			{ID: 3, Current: model.StageRemux},
			{ID: 4, Current: model.StageTranscode},
		},
	}

	t.Run("stage with multiple items", func(t *testing.T) {
		items := state.ItemsAtStage(model.StageRip)
		if len(items) != 2 {
			t.Errorf("len(items) = %d, want 2", len(items))
		}
	})

	t.Run("stage with one item", func(t *testing.T) {
		items := state.ItemsAtStage(model.StageRemux)
		if len(items) != 1 {
			t.Errorf("len(items) = %d, want 1", len(items))
		}
	})

	t.Run("stage with no items", func(t *testing.T) {
		items := state.ItemsAtStage(model.StagePublish)
		if len(items) != 0 {
			t.Errorf("len(items) = %d, want 0", len(items))
		}
	})
}

func TestPipelineState_ItemsInProgress(t *testing.T) {
	state := &PipelineState{
		Items: []model.MediaItem{
			{ID: 1, Status: model.StatusInProgress},
			{ID: 2, Status: model.StatusCompleted},
			{ID: 3, Status: model.StatusInProgress},
			{ID: 4, Status: model.StatusFailed},
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

func TestPipelineState_ItemsFailed(t *testing.T) {
	state := &PipelineState{
		Items: []model.MediaItem{
			{ID: 1, Status: model.StatusInProgress},
			{ID: 2, Status: model.StatusFailed},
			{ID: 3, Status: model.StatusCompleted},
			{ID: 4, Status: model.StatusFailed},
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

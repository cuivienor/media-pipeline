package db

import (
	"context"
	"testing"
	"time"

	"github.com/cuivienor/media-pipeline/internal/model"
)

func TestSQLiteRepository_CreateMediaItem(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	t.Run("create movie", func(t *testing.T) {
		item := &model.MediaItem{
			Type:     model.MediaTypeMovie,
			Name:     "Test Movie",
			SafeName: "Test_Movie",
		}

		err := repo.CreateMediaItem(ctx, item)
		if err != nil {
			t.Fatalf("CreateMediaItem() error = %v", err)
		}

		if item.ID == 0 {
			t.Error("ID not set after creation")
		}
	})

	t.Run("create TV season", func(t *testing.T) {
		season := 1
		item := &model.MediaItem{
			Type:     model.MediaTypeTV,
			Name:     "Test Show",
			SafeName: "Test_Show",
			Season:   &season,
		}

		err := repo.CreateMediaItem(ctx, item)
		if err != nil {
			t.Fatalf("CreateMediaItem() error = %v", err)
		}

		if item.ID == 0 {
			t.Error("ID not set after creation")
		}
	})

	t.Run("duplicate safename and season fails", func(t *testing.T) {
		season := 2
		item1 := &model.MediaItem{
			Type:     model.MediaTypeTV,
			Name:     "Duplicate Test",
			SafeName: "Duplicate_Test",
			Season:   &season,
		}

		err := repo.CreateMediaItem(ctx, item1)
		if err != nil {
			t.Fatalf("CreateMediaItem() error = %v", err)
		}

		item2 := &model.MediaItem{
			Type:     model.MediaTypeTV,
			Name:     "Duplicate Test",
			SafeName: "Duplicate_Test",
			Season:   &season,
		}

		err = repo.CreateMediaItem(ctx, item2)
		if err == nil {
			t.Error("expected error for duplicate safename/season, got nil")
		}
	})
}

func TestSQLiteRepository_GetMediaItem(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create a test item
	season := 1
	created := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Test Show",
		SafeName: "Test_Show",
		Season:   &season,
	}
	if err := repo.CreateMediaItem(ctx, created); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	t.Run("get existing item", func(t *testing.T) {
		item, err := repo.GetMediaItem(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetMediaItem() error = %v", err)
		}

		if item == nil {
			t.Fatal("expected item, got nil")
		}

		if item.ID != created.ID {
			t.Errorf("ID = %d, want %d", item.ID, created.ID)
		}
		if item.Type != created.Type {
			t.Errorf("Type = %q, want %q", item.Type, created.Type)
		}
		if item.Name != created.Name {
			t.Errorf("Name = %q, want %q", item.Name, created.Name)
		}
		if item.SafeName != created.SafeName {
			t.Errorf("SafeName = %q, want %q", item.SafeName, created.SafeName)
		}
		if item.Season == nil || *item.Season != *created.Season {
			t.Errorf("Season = %v, want %v", item.Season, created.Season)
		}
	})

	t.Run("get non-existent item", func(t *testing.T) {
		item, err := repo.GetMediaItem(ctx, 99999)
		if err != nil {
			t.Fatalf("GetMediaItem() error = %v", err)
		}
		if item != nil {
			t.Errorf("expected nil, got %+v", item)
		}
	})
}

func TestSQLiteRepository_GetMediaItemBySafeName(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create test items
	movie := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
	}
	if err := repo.CreateMediaItem(ctx, movie); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	season1 := 1
	tv1 := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Test Show",
		SafeName: "Test_Show",
		Season:   &season1,
	}
	if err := repo.CreateMediaItem(ctx, tv1); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	season2 := 2
	tv2 := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Test Show",
		SafeName: "Test_Show",
		Season:   &season2,
	}
	if err := repo.CreateMediaItem(ctx, tv2); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	t.Run("find movie by safename", func(t *testing.T) {
		item, err := repo.GetMediaItemBySafeName(ctx, "Test_Movie", nil)
		if err != nil {
			t.Fatalf("GetMediaItemBySafeName() error = %v", err)
		}
		if item == nil {
			t.Fatal("expected item, got nil")
		}
		if item.ID != movie.ID {
			t.Errorf("ID = %d, want %d", item.ID, movie.ID)
		}
	})

	t.Run("find TV by safename and season", func(t *testing.T) {
		item, err := repo.GetMediaItemBySafeName(ctx, "Test_Show", &season1)
		if err != nil {
			t.Fatalf("GetMediaItemBySafeName() error = %v", err)
		}
		if item == nil {
			t.Fatal("expected item, got nil")
		}
		if item.ID != tv1.ID {
			t.Errorf("ID = %d, want %d", item.ID, tv1.ID)
		}
	})

	t.Run("find different season", func(t *testing.T) {
		item, err := repo.GetMediaItemBySafeName(ctx, "Test_Show", &season2)
		if err != nil {
			t.Fatalf("GetMediaItemBySafeName() error = %v", err)
		}
		if item == nil {
			t.Fatal("expected item, got nil")
		}
		if item.ID != tv2.ID {
			t.Errorf("ID = %d, want %d", item.ID, tv2.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		item, err := repo.GetMediaItemBySafeName(ctx, "Nonexistent", nil)
		if err != nil {
			t.Fatalf("GetMediaItemBySafeName() error = %v", err)
		}
		if item != nil {
			t.Errorf("expected nil, got %+v", item)
		}
	})
}

func TestSQLiteRepository_ListMediaItems(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create test items
	movie1 := &model.MediaItem{Type: model.MediaTypeMovie, Name: "Movie 1", SafeName: "Movie_1"}
	movie2 := &model.MediaItem{Type: model.MediaTypeMovie, Name: "Movie 2", SafeName: "Movie_2"}
	season := 1
	tv1 := &model.MediaItem{Type: model.MediaTypeTV, Name: "Show 1", SafeName: "Show_1", Season: &season}

	for _, item := range []*model.MediaItem{movie1, movie2, tv1} {
		if err := repo.CreateMediaItem(ctx, item); err != nil {
			t.Fatalf("CreateMediaItem() error = %v", err)
		}
	}

	t.Run("list all", func(t *testing.T) {
		items, err := repo.ListMediaItems(ctx, ListOptions{})
		if err != nil {
			t.Fatalf("ListMediaItems() error = %v", err)
		}
		if len(items) != 3 {
			t.Errorf("len(items) = %d, want 3", len(items))
		}
	})

	t.Run("filter by type movie", func(t *testing.T) {
		movieType := model.MediaTypeMovie
		items, err := repo.ListMediaItems(ctx, ListOptions{Type: &movieType})
		if err != nil {
			t.Fatalf("ListMediaItems() error = %v", err)
		}
		if len(items) != 2 {
			t.Errorf("len(items) = %d, want 2", len(items))
		}
		for _, item := range items {
			if item.Type != model.MediaTypeMovie {
				t.Errorf("item.Type = %q, want movie", item.Type)
			}
		}
	})

	t.Run("filter by type TV", func(t *testing.T) {
		tvType := model.MediaTypeTV
		items, err := repo.ListMediaItems(ctx, ListOptions{Type: &tvType})
		if err != nil {
			t.Fatalf("ListMediaItems() error = %v", err)
		}
		if len(items) != 1 {
			t.Errorf("len(items) = %d, want 1", len(items))
		}
		if items[0].Type != model.MediaTypeTV {
			t.Errorf("item.Type = %q, want tv", items[0].Type)
		}
	})

	t.Run("limit and offset", func(t *testing.T) {
		items, err := repo.ListMediaItems(ctx, ListOptions{Limit: 2, Offset: 1})
		if err != nil {
			t.Fatalf("ListMediaItems() error = %v", err)
		}
		if len(items) != 2 {
			t.Errorf("len(items) = %d, want 2", len(items))
		}
	})

	t.Run("filter by active only", func(t *testing.T) {
		// Create jobs for some items
		// movie1 has an active job
		job1 := &model.Job{
			MediaItemID: movie1.ID,
			Stage:       model.StageRip,
			Status:      model.JobStatusInProgress,
		}
		if err := repo.CreateJob(ctx, job1); err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}

		// movie2 has a completed job
		job2 := &model.Job{
			MediaItemID: movie2.ID,
			Stage:       model.StageRip,
			Status:      model.JobStatusCompleted,
		}
		if err := repo.CreateJob(ctx, job2); err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}

		// tv1 has a pending job
		job3 := &model.Job{
			MediaItemID: tv1.ID,
			Stage:       model.StageRip,
			Status:      model.JobStatusPending,
		}
		if err := repo.CreateJob(ctx, job3); err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}

		// Query for active only - should get movie1 and tv1
		items, err := repo.ListMediaItems(ctx, ListOptions{ActiveOnly: true})
		if err != nil {
			t.Fatalf("ListMediaItems() error = %v", err)
		}

		if len(items) != 2 {
			t.Errorf("len(items) = %d, want 2", len(items))
		}

		// Verify we got the right items
		foundMovie1 := false
		foundTV1 := false
		for _, item := range items {
			if item.ID == movie1.ID {
				foundMovie1 = true
			}
			if item.ID == tv1.ID {
				foundTV1 = true
			}
			if item.ID == movie2.ID {
				t.Errorf("movie2 should not be in active items (has completed job)")
			}
		}

		if !foundMovie1 {
			t.Error("movie1 not found in active items")
		}
		if !foundTV1 {
			t.Error("tv1 not found in active items")
		}
	})
}

func TestSQLiteRepository_CreateJob(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create a media item first
	item := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	t.Run("create job", func(t *testing.T) {
		job := &model.Job{
			MediaItemID: item.ID,
			Stage:       model.StageRip,
			Status:      model.JobStatusPending,
			WorkerID:    "test-worker",
			PID:         12345,
			OutputDir:   "/tmp/test",
			LogPath:     "/tmp/test.log",
		}

		err := repo.CreateJob(ctx, job)
		if err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}

		if job.ID == 0 {
			t.Error("ID not set after creation")
		}
	})

	t.Run("create job with disc", func(t *testing.T) {
		disc := 1
		job := &model.Job{
			MediaItemID: item.ID,
			Stage:       model.StageRip,
			Status:      model.JobStatusPending,
			Disc:        &disc,
		}

		err := repo.CreateJob(ctx, job)
		if err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}

		if job.ID == 0 {
			t.Error("ID not set after creation")
		}
	})

	t.Run("duplicate job fails", func(t *testing.T) {
		disc := 2
		job1 := &model.Job{
			MediaItemID: item.ID,
			Stage:       model.StageRip,
			Status:      model.JobStatusPending,
			Disc:        &disc,
		}

		err := repo.CreateJob(ctx, job1)
		if err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}

		job2 := &model.Job{
			MediaItemID: item.ID,
			Stage:       model.StageRip,
			Status:      model.JobStatusInProgress,
			Disc:        &disc,
		}

		err = repo.CreateJob(ctx, job2)
		if err == nil {
			t.Error("expected error for duplicate job, got nil")
		}
	})
}

func TestSQLiteRepository_GetJob(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create a media item and job
	item := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	now := time.Now()
	created := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusInProgress,
		WorkerID:    "test-worker",
		PID:         12345,
		OutputDir:   "/tmp/test",
		LogPath:     "/tmp/test.log",
		StartedAt:   &now,
	}
	if err := repo.CreateJob(ctx, created); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	t.Run("get existing job", func(t *testing.T) {
		job, err := repo.GetJob(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetJob() error = %v", err)
		}

		if job == nil {
			t.Fatal("expected job, got nil")
		}

		if job.ID != created.ID {
			t.Errorf("ID = %d, want %d", job.ID, created.ID)
		}
		if job.MediaItemID != created.MediaItemID {
			t.Errorf("MediaItemID = %d, want %d", job.MediaItemID, created.MediaItemID)
		}
		if job.Stage != created.Stage {
			t.Errorf("Stage = %v, want %v", job.Stage, created.Stage)
		}
		if job.Status != created.Status {
			t.Errorf("Status = %v, want %v", job.Status, created.Status)
		}
		if job.WorkerID != created.WorkerID {
			t.Errorf("WorkerID = %q, want %q", job.WorkerID, created.WorkerID)
		}
		if job.PID != created.PID {
			t.Errorf("PID = %d, want %d", job.PID, created.PID)
		}
		if job.StartedAt == nil {
			t.Error("StartedAt is nil")
		}
	})

	t.Run("get non-existent job", func(t *testing.T) {
		job, err := repo.GetJob(ctx, 99999)
		if err != nil {
			t.Fatalf("GetJob() error = %v", err)
		}
		if job != nil {
			t.Errorf("expected nil, got %+v", job)
		}
	})
}

func TestSQLiteRepository_GetActiveJobForStage(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create a media item
	item := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	// Create jobs in different states
	completedJob := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusCompleted,
	}
	if err := repo.CreateJob(ctx, completedJob); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	activeJob := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRemux,
		Status:      model.JobStatusInProgress,
	}
	if err := repo.CreateJob(ctx, activeJob); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	t.Run("find active job", func(t *testing.T) {
		job, err := repo.GetActiveJobForStage(ctx, item.ID, model.StageRemux, nil)
		if err != nil {
			t.Fatalf("GetActiveJobForStage() error = %v", err)
		}

		if job == nil {
			t.Fatal("expected job, got nil")
		}

		if job.ID != activeJob.ID {
			t.Errorf("ID = %d, want %d", job.ID, activeJob.ID)
		}
	})

	t.Run("no active job for completed stage", func(t *testing.T) {
		job, err := repo.GetActiveJobForStage(ctx, item.ID, model.StageRip, nil)
		if err != nil {
			t.Fatalf("GetActiveJobForStage() error = %v", err)
		}

		if job != nil {
			t.Errorf("expected nil, got %+v", job)
		}
	})

	t.Run("no active job for non-existent stage", func(t *testing.T) {
		job, err := repo.GetActiveJobForStage(ctx, item.ID, model.StageTranscode, nil)
		if err != nil {
			t.Fatalf("GetActiveJobForStage() error = %v", err)
		}

		if job != nil {
			t.Errorf("expected nil, got %+v", job)
		}
	})
}

func TestSQLiteRepository_UpdateJobStatus(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create a media item and job
	item := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	job := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusPending,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	t.Run("update to completed", func(t *testing.T) {
		err := repo.UpdateJobStatus(ctx, job.ID, model.JobStatusCompleted, "")
		if err != nil {
			t.Fatalf("UpdateJobStatus() error = %v", err)
		}

		updated, err := repo.GetJob(ctx, job.ID)
		if err != nil {
			t.Fatalf("GetJob() error = %v", err)
		}

		if updated.Status != model.JobStatusCompleted {
			t.Errorf("Status = %v, want %v", updated.Status, model.JobStatusCompleted)
		}

		if updated.CompletedAt == nil {
			t.Error("CompletedAt should be set")
		}
	})

	t.Run("update to failed with error", func(t *testing.T) {
		failedJob := &model.Job{
			MediaItemID: item.ID,
			Stage:       model.StageRemux,
			Status:      model.JobStatusInProgress,
		}
		if err := repo.CreateJob(ctx, failedJob); err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}

		err := repo.UpdateJobStatus(ctx, failedJob.ID, model.JobStatusFailed, "test error")
		if err != nil {
			t.Fatalf("UpdateJobStatus() error = %v", err)
		}

		updated, err := repo.GetJob(ctx, failedJob.ID)
		if err != nil {
			t.Fatalf("GetJob() error = %v", err)
		}

		if updated.Status != model.JobStatusFailed {
			t.Errorf("Status = %v, want %v", updated.Status, model.JobStatusFailed)
		}

		if updated.ErrorMessage != "test error" {
			t.Errorf("ErrorMessage = %q, want %q", updated.ErrorMessage, "test error")
		}
	})
}

func TestSQLiteRepository_ListJobsForMedia(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create two media items
	item1 := &model.MediaItem{Type: model.MediaTypeMovie, Name: "Movie 1", SafeName: "Movie_1"}
	item2 := &model.MediaItem{Type: model.MediaTypeMovie, Name: "Movie 2", SafeName: "Movie_2"}

	for _, item := range []*model.MediaItem{item1, item2} {
		if err := repo.CreateMediaItem(ctx, item); err != nil {
			t.Fatalf("CreateMediaItem() error = %v", err)
		}
	}

	// Create jobs for item1
	for _, stage := range []model.Stage{model.StageRip, model.StageRemux} {
		job := &model.Job{
			MediaItemID: item1.ID,
			Stage:       stage,
			Status:      model.JobStatusCompleted,
		}
		if err := repo.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}
	}

	// Create job for item2
	job2 := &model.Job{
		MediaItemID: item2.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusCompleted,
	}
	if err := repo.CreateJob(ctx, job2); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	t.Run("list jobs for item1", func(t *testing.T) {
		jobs, err := repo.ListJobsForMedia(ctx, item1.ID)
		if err != nil {
			t.Fatalf("ListJobsForMedia() error = %v", err)
		}

		if len(jobs) != 2 {
			t.Errorf("len(jobs) = %d, want 2", len(jobs))
		}

		for _, job := range jobs {
			if job.MediaItemID != item1.ID {
				t.Errorf("MediaItemID = %d, want %d", job.MediaItemID, item1.ID)
			}
		}
	})

	t.Run("list jobs for item2", func(t *testing.T) {
		jobs, err := repo.ListJobsForMedia(ctx, item2.ID)
		if err != nil {
			t.Fatalf("ListJobsForMedia() error = %v", err)
		}

		if len(jobs) != 1 {
			t.Errorf("len(jobs) = %d, want 1", len(jobs))
		}
	})

	t.Run("list jobs for non-existent media", func(t *testing.T) {
		jobs, err := repo.ListJobsForMedia(ctx, 99999)
		if err != nil {
			t.Fatalf("ListJobsForMedia() error = %v", err)
		}

		if len(jobs) != 0 {
			t.Errorf("len(jobs) = %d, want 0", len(jobs))
		}
	})
}

func TestSQLiteRepository_CreateLogEvent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create a media item and job
	item := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	job := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusInProgress,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	t.Run("create log event", func(t *testing.T) {
		event := &model.LogEvent{
			JobID:   job.ID,
			Level:   "info",
			Message: "Test message",
		}

		err := repo.CreateLogEvent(ctx, event)
		if err != nil {
			t.Fatalf("CreateLogEvent() error = %v", err)
		}

		if event.ID == 0 {
			t.Error("ID not set after creation")
		}
	})
}

func TestSQLiteRepository_ListLogEvents(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create a media item and job
	item := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	job := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusInProgress,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	// Create log events
	for i := 0; i < 5; i++ {
		event := &model.LogEvent{
			JobID:   job.ID,
			Level:   "info",
			Message: "Test message",
		}
		if err := repo.CreateLogEvent(ctx, event); err != nil {
			t.Fatalf("CreateLogEvent() error = %v", err)
		}
	}

	t.Run("list all events", func(t *testing.T) {
		events, err := repo.ListLogEvents(ctx, job.ID, 0)
		if err != nil {
			t.Fatalf("ListLogEvents() error = %v", err)
		}

		if len(events) != 5 {
			t.Errorf("len(events) = %d, want 5", len(events))
		}
	})

	t.Run("list with limit", func(t *testing.T) {
		events, err := repo.ListLogEvents(ctx, job.ID, 3)
		if err != nil {
			t.Fatalf("ListLogEvents() error = %v", err)
		}

		if len(events) != 3 {
			t.Errorf("len(events) = %d, want 3", len(events))
		}
	})

	t.Run("list for non-existent job", func(t *testing.T) {
		events, err := repo.ListLogEvents(ctx, 99999, 0)
		if err != nil {
			t.Fatalf("ListLogEvents() error = %v", err)
		}

		if len(events) != 0 {
			t.Errorf("len(events) = %d, want 0", len(events))
		}
	})
}

func TestSQLiteRepository_GetDiscProgress(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create a TV show
	season := 1
	item := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Test Show",
		SafeName: "Test_Show",
		Season:   &season,
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	// Create jobs for different discs
	disc1 := 1
	job1 := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusCompleted,
		Disc:        &disc1,
	}
	if err := repo.CreateJob(ctx, job1); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	disc2 := 2
	job2 := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusInProgress,
		Disc:        &disc2,
	}
	if err := repo.CreateJob(ctx, job2); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	disc3 := 3
	job3 := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusFailed,
		Disc:        &disc3,
	}
	if err := repo.CreateJob(ctx, job3); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	t.Run("get disc progress", func(t *testing.T) {
		progress, err := repo.GetDiscProgress(ctx, item.ID)
		if err != nil {
			t.Fatalf("GetDiscProgress() error = %v", err)
		}

		if len(progress) != 3 {
			t.Errorf("len(progress) = %d, want 3", len(progress))
		}

		// Verify disc 1 is completed
		found := false
		for _, p := range progress {
			if p.Disc == 1 {
				found = true
				if p.Status != model.JobStatusCompleted {
					t.Errorf("disc 1 status = %v, want completed", p.Status)
				}
				if p.JobID != job1.ID {
					t.Errorf("disc 1 jobID = %d, want %d", p.JobID, job1.ID)
				}
			}
		}
		if !found {
			t.Error("disc 1 not found in progress")
		}
	})

	t.Run("no disc progress for movie", func(t *testing.T) {
		movieItem := &model.MediaItem{
			Type:     model.MediaTypeMovie,
			Name:     "Test Movie",
			SafeName: "Test_Movie2",
		}
		if err := repo.CreateMediaItem(ctx, movieItem); err != nil {
			t.Fatalf("CreateMediaItem() error = %v", err)
		}

		progress, err := repo.GetDiscProgress(ctx, movieItem.ID)
		if err != nil {
			t.Fatalf("GetDiscProgress() error = %v", err)
		}

		if len(progress) != 0 {
			t.Errorf("len(progress) = %d, want 0", len(progress))
		}
	})
}

func TestSQLiteRepository_CreateSeason(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create a TV show first
	show := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Test Show",
		SafeName: "Test_Show",
	}
	if err := repo.CreateMediaItem(ctx, show); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	t.Run("create season", func(t *testing.T) {
		season := &model.Season{
			ItemID:       show.ID,
			Number:       1,
			CurrentStage: model.StageRip,
			StageStatus:  model.StatusPending,
		}

		err := repo.CreateSeason(ctx, season)
		if err != nil {
			t.Fatalf("CreateSeason() error = %v", err)
		}

		if season.ID == 0 {
			t.Error("ID not set after creation")
		}
	})

	t.Run("duplicate season number fails", func(t *testing.T) {
		season := &model.Season{
			ItemID:       show.ID,
			Number:       1, // Same as above
			CurrentStage: model.StageRip,
			StageStatus:  model.StatusPending,
		}

		err := repo.CreateSeason(ctx, season)
		if err == nil {
			t.Error("expected error for duplicate season number, got nil")
		}
	})
}

func TestSQLiteRepository_GetSeason(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create a TV show and season
	show := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Test Show",
		SafeName: "Test_Show",
	}
	if err := repo.CreateMediaItem(ctx, show); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	created := &model.Season{
		ItemID:       show.ID,
		Number:       1,
		CurrentStage: model.StageRemux,
		StageStatus:  model.StatusInProgress,
	}
	if err := repo.CreateSeason(ctx, created); err != nil {
		t.Fatalf("CreateSeason() error = %v", err)
	}

	t.Run("get existing season", func(t *testing.T) {
		season, err := repo.GetSeason(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetSeason() error = %v", err)
		}

		if season.ID != created.ID {
			t.Errorf("ID = %d, want %d", season.ID, created.ID)
		}
		if season.ItemID != show.ID {
			t.Errorf("ItemID = %d, want %d", season.ItemID, show.ID)
		}
		if season.Number != 1 {
			t.Errorf("Number = %d, want 1", season.Number)
		}
		if season.CurrentStage != model.StageRemux {
			t.Errorf("CurrentStage = %v, want %v", season.CurrentStage, model.StageRemux)
		}
		if season.StageStatus != model.StatusInProgress {
			t.Errorf("StageStatus = %v, want %v", season.StageStatus, model.StatusInProgress)
		}
	})

	t.Run("get nonexistent season", func(t *testing.T) {
		season, err := repo.GetSeason(ctx, 99999)
		if err != nil {
			t.Fatalf("GetSeason() error = %v", err)
		}
		if season != nil {
			t.Error("expected nil for nonexistent season")
		}
	})
}

func TestSQLiteRepository_ListSeasonsForItem(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create a TV show with multiple seasons
	show := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Test Show",
		SafeName: "Test_Show",
	}
	if err := repo.CreateMediaItem(ctx, show); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	// Create seasons out of order
	for _, num := range []int{3, 1, 2} {
		season := &model.Season{
			ItemID:       show.ID,
			Number:       num,
			CurrentStage: model.StageRip,
			StageStatus:  model.StatusPending,
		}
		if err := repo.CreateSeason(ctx, season); err != nil {
			t.Fatalf("CreateSeason(season %d) error = %v", num, err)
		}
	}

	t.Run("list seasons returns in order", func(t *testing.T) {
		seasons, err := repo.ListSeasonsForItem(ctx, show.ID)
		if err != nil {
			t.Fatalf("ListSeasonsForItem() error = %v", err)
		}

		if len(seasons) != 3 {
			t.Fatalf("len(seasons) = %d, want 3", len(seasons))
		}

		// Should be ordered by number: 1, 2, 3
		for i, season := range seasons {
			expectedNum := i + 1
			if season.Number != expectedNum {
				t.Errorf("seasons[%d].Number = %d, want %d", i, season.Number, expectedNum)
			}
		}
	})

	t.Run("list seasons for nonexistent item", func(t *testing.T) {
		seasons, err := repo.ListSeasonsForItem(ctx, 99999)
		if err != nil {
			t.Fatalf("ListSeasonsForItem() error = %v", err)
		}
		if len(seasons) != 0 {
			t.Errorf("len(seasons) = %d, want 0", len(seasons))
		}
	})
}

func TestSQLiteRepository_ListActiveItems(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() error = %v", err)
	}
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create items with different statuses
	activeMovie := &model.MediaItem{
		Type:       model.MediaTypeMovie,
		Name:       "Active Movie",
		SafeName:   "Active_Movie",
		ItemStatus: model.ItemStatusActive,
	}
	if err := repo.CreateMediaItem(ctx, activeMovie); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	completedMovie := &model.MediaItem{
		Type:       model.MediaTypeMovie,
		Name:       "Completed Movie",
		SafeName:   "Completed_Movie",
		ItemStatus: model.ItemStatusCompleted,
	}
	if err := repo.CreateMediaItem(ctx, completedMovie); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	notStartedShow := &model.MediaItem{
		Type:       model.MediaTypeTV,
		Name:       "Not Started Show",
		SafeName:   "Not_Started_Show",
		ItemStatus: model.ItemStatusNotStarted,
	}
	if err := repo.CreateMediaItem(ctx, notStartedShow); err != nil {
		t.Fatalf("CreateMediaItem() error = %v", err)
	}

	t.Run("excludes completed items", func(t *testing.T) {
		items, err := repo.ListActiveItems(ctx)
		if err != nil {
			t.Fatalf("ListActiveItems() error = %v", err)
		}

		// Should include active and not_started, exclude completed
		if len(items) != 2 {
			t.Fatalf("len(items) = %d, want 2", len(items))
		}

		// Verify completed item is not in the list
		for _, item := range items {
			if item.ItemStatus == model.ItemStatusCompleted {
				t.Error("completed item should not be in active items")
			}
		}
	})
}

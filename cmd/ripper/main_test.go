package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/model"
	"github.com/cuivienor/media-pipeline/internal/ripper"
)

func TestBuildRipRequest_Movie(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	repo := db.NewSQLiteRepository(database)

	// Create a media item
	item := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "The Matrix",
		SafeName: "The_Matrix",
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("Failed to create media item: %v", err)
	}

	// Create a job
	job := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusPending,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Build RipRequest
	req, err := buildRipRequest(ctx, repo, job, item, "disc:0")
	if err != nil {
		t.Fatalf("buildRipRequest failed: %v", err)
	}

	if req.Type != ripper.MediaTypeMovie {
		t.Errorf("Type = %v, want movie", req.Type)
	}
	if req.Name != "The Matrix" {
		t.Errorf("Name = %q, want 'The Matrix'", req.Name)
	}
	if req.DiscPath != "disc:0" {
		t.Errorf("DiscPath = %q, want 'disc:0'", req.DiscPath)
	}
}

func TestBuildRipRequest_TVShow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	repo := db.NewSQLiteRepository(database)

	// Create a media item (TV show)
	item := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Breaking Bad",
		SafeName: "Breaking_Bad",
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("Failed to create media item: %v", err)
	}

	// Create a season
	season := &model.Season{
		ItemID:       item.ID,
		Number:       2,
		CurrentStage: model.StageRip,
		StageStatus:  model.StatusPending,
	}
	if err := repo.CreateSeason(ctx, season); err != nil {
		t.Fatalf("Failed to create season: %v", err)
	}

	// Create a job with SeasonID
	disc := 3
	job := &model.Job{
		MediaItemID: item.ID,
		SeasonID:    &season.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusPending,
		Disc:        &disc,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Build RipRequest
	req, err := buildRipRequest(ctx, repo, job, item, "/dev/sr0")
	if err != nil {
		t.Fatalf("buildRipRequest failed: %v", err)
	}

	if req.Type != ripper.MediaTypeTV {
		t.Errorf("Type = %v, want tv", req.Type)
	}
	if req.Name != "Breaking Bad" {
		t.Errorf("Name = %q, want 'Breaking Bad'", req.Name)
	}
	if req.Season != 2 {
		t.Errorf("Season = %d, want 2", req.Season)
	}
	if req.Disc != 3 {
		t.Errorf("Disc = %d, want 3", req.Disc)
	}
	if req.DiscPath != "/dev/sr0" {
		t.Errorf("DiscPath = %q, want '/dev/sr0'", req.DiscPath)
	}
}

func TestBuildRipRequest_TVMissingSeason(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	repo := db.NewSQLiteRepository(database)

	// Create a TV media item
	item := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Breaking Bad",
		SafeName: "Breaking_Bad",
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("Failed to create media item: %v", err)
	}

	// Create a job without SeasonID (invalid)
	disc := 1
	job := &model.Job{
		MediaItemID: item.ID,
		SeasonID:    nil, // Missing season
		Stage:       model.StageRip,
		Status:      model.JobStatusPending,
		Disc:        &disc,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// buildRipRequest should fail
	_, err = buildRipRequest(ctx, repo, job, item, "disc:0")
	if err == nil {
		t.Error("Expected error for TV show missing season")
	}
}

func TestBuildRipRequest_TVMissingDisc(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	repo := db.NewSQLiteRepository(database)

	// Create a TV media item
	item := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Breaking Bad",
		SafeName: "Breaking_Bad",
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("Failed to create media item: %v", err)
	}

	// Create a season
	season := &model.Season{
		ItemID:       item.ID,
		Number:       1,
		CurrentStage: model.StageRip,
		StageStatus:  model.StatusPending,
	}
	if err := repo.CreateSeason(ctx, season); err != nil {
		t.Fatalf("Failed to create season: %v", err)
	}

	// Create a job without Disc (invalid)
	job := &model.Job{
		MediaItemID: item.ID,
		SeasonID:    &season.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusPending,
		Disc:        nil, // Missing disc
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// buildRipRequest should fail
	_, err = buildRipRequest(ctx, repo, job, item, "disc:0")
	if err == nil {
		t.Error("Expected error for TV show job missing disc")
	}
}

func TestBuildOutputDir_Movie(t *testing.T) {
	req := &ripper.RipRequest{
		Type: ripper.MediaTypeMovie,
		Name: "The Matrix",
	}

	outputDir := buildOutputDir("/mnt/media/staging", req)
	expected := "/mnt/media/staging/1-ripped/movies/The_Matrix"

	if outputDir != expected {
		t.Errorf("outputDir = %q, want %q", outputDir, expected)
	}
}

func TestBuildOutputDir_TVShow(t *testing.T) {
	req := &ripper.RipRequest{
		Type:   ripper.MediaTypeTV,
		Name:   "Breaking Bad",
		Season: 2,
		Disc:   3,
	}

	outputDir := buildOutputDir("/mnt/media/staging", req)
	expected := "/mnt/media/staging/1-ripped/tv/Breaking_Bad/S02/Disc3"

	if outputDir != expected {
		t.Errorf("outputDir = %q, want %q", outputDir, expected)
	}
}

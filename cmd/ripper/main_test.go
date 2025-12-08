package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/model"
	"github.com/cuivienor/media-pipeline/internal/ripper"
)

func TestParseArgs_MovieShort(t *testing.T) {
	args := []string{"-t", "movie", "-n", "The Matrix"}

	opts, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if opts.Type != ripper.MediaTypeMovie {
		t.Errorf("Type = %v, want movie", opts.Type)
	}
	if opts.Name != "The Matrix" {
		t.Errorf("Name = %q, want 'The Matrix'", opts.Name)
	}
}

func TestParseArgs_MovieLong(t *testing.T) {
	args := []string{"--type", "movie", "--name", "The Matrix"}

	opts, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if opts.Type != ripper.MediaTypeMovie {
		t.Errorf("Type = %v, want movie", opts.Type)
	}
}

func TestParseArgs_TVShow(t *testing.T) {
	args := []string{"-t", "tv", "-n", "Breaking Bad", "-s", "1", "-d", "2"}

	opts, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if opts.Type != ripper.MediaTypeTV {
		t.Errorf("Type = %v, want tv", opts.Type)
	}
	if opts.Name != "Breaking Bad" {
		t.Errorf("Name = %q, want 'Breaking Bad'", opts.Name)
	}
	if opts.Season != 1 {
		t.Errorf("Season = %d, want 1", opts.Season)
	}
	if opts.Disc != 2 {
		t.Errorf("Disc = %d, want 2", opts.Disc)
	}
}

func TestParseArgs_TVShowLong(t *testing.T) {
	args := []string{"--type", "tv", "--name", "Avatar", "--season", "2", "--disc", "3"}

	opts, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if opts.Season != 2 {
		t.Errorf("Season = %d, want 2", opts.Season)
	}
	if opts.Disc != 3 {
		t.Errorf("Disc = %d, want 3", opts.Disc)
	}
}

func TestParseArgs_MissingType(t *testing.T) {
	args := []string{"-n", "The Matrix"}

	_, err := ParseArgs(args)
	if err == nil {
		t.Error("Expected error for missing type")
	}
}

func TestParseArgs_MissingName(t *testing.T) {
	args := []string{"-t", "movie"}

	_, err := ParseArgs(args)
	if err == nil {
		t.Error("Expected error for missing name")
	}
}

func TestParseArgs_TVMissingSeason(t *testing.T) {
	args := []string{"-t", "tv", "-n", "Breaking Bad", "-d", "1"}

	_, err := ParseArgs(args)
	if err == nil {
		t.Error("Expected error for TV show missing season")
	}
}

func TestParseArgs_TVMissingDisc(t *testing.T) {
	args := []string{"-t", "tv", "-n", "Breaking Bad", "-s", "1"}

	_, err := ParseArgs(args)
	if err == nil {
		t.Error("Expected error for TV show missing disc")
	}
}

func TestParseArgs_DiscPath(t *testing.T) {
	args := []string{"-t", "movie", "-n", "The Matrix", "--disc-path", "/dev/sr0"}

	opts, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if opts.DiscPath != "/dev/sr0" {
		t.Errorf("DiscPath = %q, want '/dev/sr0'", opts.DiscPath)
	}
}

func TestParseArgs_DefaultDiscPath(t *testing.T) {
	args := []string{"-t", "movie", "-n", "The Matrix"}

	opts, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if opts.DiscPath != "disc:0" {
		t.Errorf("DiscPath = %q, want 'disc:0'", opts.DiscPath)
	}
}

func TestParseArgs_InvalidType(t *testing.T) {
	args := []string{"-t", "invalid", "-n", "The Matrix"}

	_, err := ParseArgs(args)
	if err == nil {
		t.Error("Expected error for invalid type")
	}
}

func TestParseArgs_TypeAliases(t *testing.T) {
	// "show" should be accepted as alias for "tv"
	args := []string{"-t", "show", "-n", "Breaking Bad", "-s", "1", "-d", "1"}

	opts, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if opts.Type != ripper.MediaTypeTV {
		t.Errorf("Type = %v, want tv", opts.Type)
	}
}

func TestBuildConfig_DefaultMediaBase(t *testing.T) {
	opts := &Options{
		Type: ripper.MediaTypeMovie,
		Name: "Test",
	}

	config := BuildConfig(opts, nil)

	if config.MediaBase == "" {
		t.Error("MediaBase should have a default value")
	}
}

func TestBuildConfig_EnvOverride(t *testing.T) {
	opts := &Options{
		Type: ripper.MediaTypeMovie,
		Name: "Test",
	}

	env := map[string]string{
		"MEDIA_BASE": "/custom/path",
	}

	config := BuildConfig(opts, env)

	if config.MediaBase != "/custom/path" {
		t.Errorf("MediaBase = %q, want '/custom/path'", config.MediaBase)
	}
}

func TestBuildConfig_MakeMKVConPath(t *testing.T) {
	opts := &Options{
		Type: ripper.MediaTypeMovie,
		Name: "Test",
	}

	env := map[string]string{
		"MAKEMKVCON_PATH": "/usr/local/bin/mock-makemkv",
	}

	config := BuildConfig(opts, env)

	if config.MakeMKVConPath != "/usr/local/bin/mock-makemkv" {
		t.Errorf("MakeMKVConPath = %q, want '/usr/local/bin/mock-makemkv'", config.MakeMKVConPath)
	}
}

func TestBuildRipRequest(t *testing.T) {
	opts := &Options{
		Type:     ripper.MediaTypeTV,
		Name:     "Breaking Bad",
		Season:   2,
		Disc:     3,
		DiscPath: "disc:0",
	}

	req := BuildRipRequest(opts)

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
}

func TestParseArgs_DBPath(t *testing.T) {
	args := []string{"-t", "movie", "-n", "The Matrix", "-db", "/path/to/test.db"}

	opts, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if opts.DBPath != "/path/to/test.db" {
		t.Errorf("DBPath = %q, want '/path/to/test.db'", opts.DBPath)
	}
}

func TestParseArgs_JobID(t *testing.T) {
	args := []string{"-job-id", "123", "-db", "/path/to/test.db"}

	opts, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if opts.JobID != 123 {
		t.Errorf("JobID = %d, want 123", opts.JobID)
	}
	if opts.DBPath != "/path/to/test.db" {
		t.Errorf("DBPath = %q, want '/path/to/test.db'", opts.DBPath)
	}
}

func TestParseArgs_JobIDWithoutDB(t *testing.T) {
	args := []string{"-job-id", "123"}

	_, err := ParseArgs(args)
	if err == nil {
		t.Error("Expected error when job-id specified without db path")
	}
}

func TestParseArgs_JobIDMode(t *testing.T) {
	// In job-id mode, -t and -n are not required (they come from the job)
	args := []string{"-job-id", "123", "-db", "/path/to/test.db"}

	opts, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	// Should be in job-id mode
	if opts.JobID != 123 {
		t.Errorf("JobID = %d, want 123", opts.JobID)
	}
}

func TestParseArgs_StandaloneWithDB(t *testing.T) {
	// Standalone mode with DB tracking
	args := []string{"-t", "movie", "-n", "The Matrix", "-db", "/path/to/test.db"}

	opts, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if opts.Type != ripper.MediaTypeMovie {
		t.Errorf("Type = %v, want movie", opts.Type)
	}
	if opts.DBPath != "/path/to/test.db" {
		t.Errorf("DBPath = %q, want '/path/to/test.db'", opts.DBPath)
	}
}

func TestDetermineMode_JobID(t *testing.T) {
	opts := &Options{
		JobID:  123,
		DBPath: "/path/to/test.db",
	}

	mode := DetermineMode(opts)
	if mode != ModeJobDispatch {
		t.Errorf("mode = %v, want ModeJobDispatch", mode)
	}
}

func TestDetermineMode_StandaloneWithDB(t *testing.T) {
	opts := &Options{
		Type:   ripper.MediaTypeMovie,
		Name:   "The Matrix",
		DBPath: "/path/to/test.db",
	}

	mode := DetermineMode(opts)
	if mode != ModeStandaloneWithDB {
		t.Errorf("mode = %v, want ModeStandaloneWithDB", mode)
	}
}

func TestDetermineMode_StandaloneNoDB(t *testing.T) {
	opts := &Options{
		Type: ripper.MediaTypeMovie,
		Name: "The Matrix",
	}

	mode := DetermineMode(opts)
	if mode != ModeStandaloneNoDB {
		t.Errorf("mode = %v, want ModeStandaloneNoDB", mode)
	}
}

func TestBuildStateManager_WithDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	opts := &Options{
		Type:   ripper.MediaTypeMovie,
		Name:   "Test Movie",
		DBPath: dbPath,
	}

	sm, database, err := BuildStateManager(opts)
	if err != nil {
		t.Fatalf("BuildStateManager failed: %v", err)
	}
	defer database.Close()

	if sm == nil {
		t.Error("expected DualWriteStateManager, got nil")
	}
	if database == nil {
		t.Error("expected DB, got nil")
	}
}

func TestBuildStateManager_NoDB(t *testing.T) {
	opts := &Options{
		Type: ripper.MediaTypeMovie,
		Name: "Test Movie",
	}

	sm, database, err := BuildStateManager(opts)
	if err != nil {
		t.Fatalf("BuildStateManager failed: %v", err)
	}

	if sm == nil {
		t.Error("expected DefaultStateManager, got nil")
	}
	if database != nil {
		t.Error("expected nil DB for no-DB mode, got non-nil")
	}
}

func TestLoadRipRequestFromJob_Movie(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database
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

	// Load RipRequest from job
	req, err := LoadRipRequestFromJob(database, job.ID)
	if err != nil {
		t.Fatalf("LoadRipRequestFromJob failed: %v", err)
	}

	if req.Type != ripper.MediaTypeMovie {
		t.Errorf("Type = %v, want movie", req.Type)
	}
	if req.Name != "The Matrix" {
		t.Errorf("Name = %q, want 'The Matrix'", req.Name)
	}
	if req.Season != 0 {
		t.Errorf("Season = %d, want 0", req.Season)
	}
	if req.Disc != 0 {
		t.Errorf("Disc = %d, want 0", req.Disc)
	}
}

func TestLoadRipRequestFromJob_TVShow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database
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

	// Load RipRequest from job
	req, err := LoadRipRequestFromJob(database, job.ID)
	if err != nil {
		t.Fatalf("LoadRipRequestFromJob failed: %v", err)
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
}

func TestLoadRipRequestFromJob_JobNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Try to load non-existent job
	_, err = LoadRipRequestFromJob(database, 999)
	if err == nil {
		t.Error("Expected error for non-existent job")
	}
}

func TestLoadRipRequestFromJob_TVMissingSeason(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	repo := db.NewSQLiteRepository(database)

	// Create a TV media item without season (invalid)
	item := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Breaking Bad",
		SafeName: "Breaking_Bad",
		Season:   nil, // Missing season
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("Failed to create media item: %v", err)
	}

	// Create a job
	disc := 1
	job := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusPending,
		Disc:        &disc,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Load RipRequest from job should fail
	_, err = LoadRipRequestFromJob(database, job.ID)
	if err == nil {
		t.Error("Expected error for TV show missing season")
	}
}

func TestLoadRipRequestFromJob_TVMissingDisc(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	repo := db.NewSQLiteRepository(database)

	// Create a TV media item
	season := 1
	item := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Breaking Bad",
		SafeName: "Breaking_Bad",
		Season:   &season,
	}
	if err := repo.CreateMediaItem(ctx, item); err != nil {
		t.Fatalf("Failed to create media item: %v", err)
	}

	// Create a job without disc (invalid)
	job := &model.Job{
		MediaItemID: item.ID,
		Stage:       model.StageRip,
		Status:      model.JobStatusPending,
		Disc:        nil, // Missing disc
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Load RipRequest from job should fail
	_, err = LoadRipRequestFromJob(database, job.ID)
	if err == nil {
		t.Error("Expected error for TV show job missing disc")
	}
}

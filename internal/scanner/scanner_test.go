package scanner

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/model"
)

// getTestdataPath returns absolute path to testdata directory
func getTestdataPath(t *testing.T, fixture string) string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get current file path")
	}
	return filepath.Join(filepath.Dir(filename), "testdata", fixture, "staging")
}

func TestScanPipeline_EmptyPipeline(t *testing.T) {
	stagingPath := getTestdataPath(t, "empty_pipeline")
	config := Config{
		StagingBase: stagingPath,
		LibraryBase: filepath.Join(stagingPath, "..", "library"),
	}
	scanner := New(config)

	state, err := scanner.ScanPipeline()
	if err != nil {
		t.Fatalf("ScanPipeline() error = %v", err)
	}
	if len(state.Items) != 0 {
		t.Errorf("ScanPipeline() returned %d items, want 0", len(state.Items))
	}
}

func TestScanPipeline_SingleMovieRipped(t *testing.T) {
	stagingPath := getTestdataPath(t, "single_movie_ripped")
	config := Config{
		StagingBase: stagingPath,
		LibraryBase: filepath.Join(stagingPath, "..", "library"),
	}
	scanner := New(config)

	state, err := scanner.ScanPipeline()
	if err != nil {
		t.Fatalf("ScanPipeline() error = %v", err)
	}
	if len(state.Items) != 1 {
		t.Fatalf("ScanPipeline() returned %d items, want 1", len(state.Items))
	}

	item := state.Items[0]
	if item.Type != model.MediaTypeMovie {
		t.Errorf("item.Type = %v, want movie", item.Type)
	}
	if item.Name != "Test Movie" {
		t.Errorf("item.Name = %v, want 'Test Movie'", item.Name)
	}
	if item.SafeName != "Test_Movie" {
		t.Errorf("item.SafeName = %v, want 'Test_Movie'", item.SafeName)
	}
	if item.Current != model.StageRip {
		t.Errorf("item.Current = %v, want StageRip", item.Current)
	}
	if item.Status != model.StatusCompleted {
		t.Errorf("item.Status = %v, want StatusCompleted", item.Status)
	}
}

func TestScanPipeline_TVShowMultiSeason(t *testing.T) {
	stagingPath := getTestdataPath(t, "tv_show_multi_season")
	config := Config{
		StagingBase: stagingPath,
		LibraryBase: filepath.Join(stagingPath, "..", "library"),
	}
	scanner := New(config)

	state, err := scanner.ScanPipeline()
	if err != nil {
		t.Fatalf("ScanPipeline() error = %v", err)
	}

	// Should merge ripped + remuxed into single item at furthest stage
	if len(state.Items) != 1 {
		t.Fatalf("ScanPipeline() returned %d items, want 1 (merged)", len(state.Items))
	}

	item := state.Items[0]
	if item.Type != model.MediaTypeTV {
		t.Errorf("item.Type = %v, want tv", item.Type)
	}
	if item.SafeName != "Breaking_Bad" {
		t.Errorf("item.SafeName = %v, want 'Breaking_Bad'", item.SafeName)
	}
	// Should be at remux stage (furthest completed)
	if item.Current != model.StageRemux {
		t.Errorf("item.Current = %v, want StageRemux", item.Current)
	}
	// Should have 2 stage entries in history
	if len(item.Stages) != 2 {
		t.Errorf("len(item.Stages) = %d, want 2", len(item.Stages))
	}
}

func TestScanPipeline_MixedStatuses(t *testing.T) {
	stagingPath := getTestdataPath(t, "mixed_statuses")
	config := Config{
		StagingBase: stagingPath,
		LibraryBase: filepath.Join(stagingPath, "..", "library"),
	}
	scanner := New(config)

	state, err := scanner.ScanPipeline()
	if err != nil {
		t.Fatalf("ScanPipeline() error = %v", err)
	}
	if len(state.Items) != 3 {
		t.Fatalf("ScanPipeline() returned %d items, want 3", len(state.Items))
	}

	// Check filter methods work
	ready := state.ItemsReadyForNextStage()
	if len(ready) != 1 {
		t.Errorf("ItemsReadyForNextStage() = %d, want 1", len(ready))
	}

	inProgress := state.ItemsInProgress()
	if len(inProgress) != 1 {
		t.Errorf("ItemsInProgress() = %d, want 1", len(inProgress))
	}

	failed := state.ItemsFailed()
	if len(failed) != 1 {
		t.Errorf("ItemsFailed() = %d, want 1", len(failed))
	}
}

func TestScanPipeline_NonExistentPath(t *testing.T) {
	config := Config{
		StagingBase: "/nonexistent/path/that/does/not/exist",
		LibraryBase: "/nonexistent/library",
	}
	scanner := New(config)

	state, err := scanner.ScanPipeline()
	// Should not error, just return empty state
	if err != nil {
		t.Fatalf("ScanPipeline() error = %v, want nil", err)
	}
	if len(state.Items) != 0 {
		t.Errorf("ScanPipeline() returned %d items, want 0", len(state.Items))
	}
}

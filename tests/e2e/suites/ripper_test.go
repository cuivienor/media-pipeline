package suites

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/model"
	"github.com/cuivienor/media-pipeline/internal/ripper"
	"github.com/cuivienor/media-pipeline/tests/e2e/testenv"
)

// findMockMakeMKV locates the mock-makemkv binary
func findMockMakeMKV(t *testing.T) string {
	// Try common locations
	paths := []string{
		filepath.Join("bin", "mock-makemkv"),
		filepath.Join("..", "..", "..", "bin", "mock-makemkv"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			absPath, _ := filepath.Abs(p)
			return absPath
		}
	}

	// Try PATH
	path, err := exec.LookPath("mock-makemkv")
	if err == nil {
		return path
	}

	t.Skip("mock-makemkv not found, skipping E2E test")
	return ""
}

// requireFFmpeg skips test if ffmpeg is not available
func requireFFmpeg(t *testing.T) {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not found, skipping E2E test")
	}
}

func TestRipper_E2E_MovieRip(t *testing.T) {
	requireFFmpeg(t)
	mockPath := findMockMakeMKV(t)

	// Set up test environment
	env := testenv.New(t)

	// Create ripper with mock-makemkv
	runner := ripper.NewMakeMKVRunner(mockPath)
	r := ripper.NewRipper(env.StagingBase, runner, nil)

	// Create request
	req := &ripper.RipRequest{
		Type:     ripper.MediaTypeMovie,
		Name:     "Big Buck Bunny",
		DiscPath: "disc:0", // mock-makemkv uses profile based on disc path
	}

	// Build output directory
	outputDir := r.BuildOutputDir(req)

	// Run the rip
	result, err := r.Rip(context.Background(), req, outputDir, nil, nil)
	if err != nil {
		t.Fatalf("Rip failed: %v", err)
	}

	// Verify result
	if result.Status != model.StatusCompleted {
		t.Errorf("Status = %v, want completed", result.Status)
	}

	// Verify output directory exists
	if _, err := os.Stat(result.OutputDir); os.IsNotExist(err) {
		t.Error("Output directory does not exist")
	}

	// Verify organization scaffolding was created
	expectedDirs := []string{"_discarded", "_main", "_extras/trailers"}
	for _, dir := range expectedDirs {
		path := filepath.Join(result.OutputDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected scaffolding directory %q to exist", dir)
		}
	}
}

func TestRipper_E2E_TVShowRip(t *testing.T) {
	requireFFmpeg(t)
	mockPath := findMockMakeMKV(t)

	// Set up test environment
	env := testenv.New(t)

	// Create ripper with mock-makemkv
	runner := ripper.NewMakeMKVRunner(mockPath)
	r := ripper.NewRipper(env.StagingBase, runner, nil)

	// Create request
	req := &ripper.RipRequest{
		Type:     ripper.MediaTypeTV,
		Name:     "The Simpsons",
		Season:   1,
		Disc:     1,
		DiscPath: "disc:0",
	}

	// Build output directory
	outputDir := r.BuildOutputDir(req)

	// Run the rip
	result, err := r.Rip(context.Background(), req, outputDir, nil, nil)
	if err != nil {
		t.Fatalf("Rip failed: %v", err)
	}

	// Verify result
	if result.Status != model.StatusCompleted {
		t.Errorf("Status = %v, want completed", result.Status)
	}

	// Verify output directory structure
	expectedDir := filepath.Join(env.StagingBase, "1-ripped", "tv", "The_Simpsons", "S01", "Disc1")
	if result.OutputDir != expectedDir {
		t.Errorf("OutputDir = %q, want %q", result.OutputDir, expectedDir)
	}

	// Verify organization scaffolding was created (TV shows get _episodes, not _main)
	expectedDirs := []string{"_discarded", "_episodes", "_extras/trailers"}
	for _, dir := range expectedDirs {
		path := filepath.Join(result.OutputDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected scaffolding directory %q to exist", dir)
		}
	}
}

// TestRipper_E2E_StateCompatibleWithScanner has been removed as scanner was removed
// in favor of database-backed state. See Task 3 of Phase 2 TUI Database Integration.

func TestRipper_E2E_MultipleTVShowDiscs(t *testing.T) {
	requireFFmpeg(t)
	mockPath := findMockMakeMKV(t)

	// Set up test environment
	env := testenv.New(t)

	// Create ripper with mock-makemkv
	runner := ripper.NewMakeMKVRunner(mockPath)
	r := ripper.NewRipper(env.StagingBase, runner, nil)

	// Rip two discs of the same show
	for disc := 1; disc <= 2; disc++ {
		req := &ripper.RipRequest{
			Type:     ripper.MediaTypeTV,
			Name:     "Multi Disc Show",
			Season:   1,
			Disc:     disc,
			DiscPath: "disc:0",
		}

		outputDir := r.BuildOutputDir(req)
		_, err := r.Rip(context.Background(), req, outputDir, nil, nil)
		if err != nil {
			t.Fatalf("Rip of disc %d failed: %v", disc, err)
		}
	}

	// Verify both disc directories exist
	for disc := 1; disc <= 2; disc++ {
		discDir := filepath.Join(env.StagingBase, "1-ripped", "tv", "Multi_Disc_Show", "S01", fmt.Sprintf("Disc%d", disc))
		if _, err := os.Stat(discDir); os.IsNotExist(err) {
			t.Errorf("Disc%d directory does not exist: %s", disc, discDir)
		}

		// Verify organization scaffolding was created for each disc (TV shows get _episodes, not _main)
		expectedDirs := []string{"_discarded", "_episodes", "_extras/trailers"}
		for _, dir := range expectedDirs {
			path := filepath.Join(discDir, dir)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("Disc%d: Expected scaffolding directory %q to exist", disc, dir)
			}
		}
	}
}

func TestRipper_E2E_CLIExecution(t *testing.T) {
	// Skip this test - ripper CLI now requires -job-id and -db flags
	// and cannot run standalone without a database
	t.Skip("Ripper CLI now requires database mode with -job-id and -db flags")
}

// findRipperBinary locates the ripper binary
func findRipperBinary(t *testing.T) string {
	paths := []string{
		filepath.Join("bin", "ripper"),
		filepath.Join("..", "..", "..", "bin", "ripper"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			absPath, _ := filepath.Abs(p)
			return absPath
		}
	}

	// Try PATH
	path, err := exec.LookPath("ripper")
	if err == nil {
		return path
	}

	t.Skip("ripper binary not found, skipping CLI test")
	return ""
}

// filterByType filters items by media type
func filterByType(items []model.MediaItem, mediaType model.MediaType) []model.MediaItem {
	var filtered []model.MediaItem
	for _, item := range items {
		if item.Type == mediaType {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

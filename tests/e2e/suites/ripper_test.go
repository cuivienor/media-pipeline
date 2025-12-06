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
	"github.com/cuivienor/media-pipeline/internal/scanner"
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

	// Run the rip
	result, err := r.Rip(context.Background(), req)
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

	// Verify .rip state directory
	stateDir, err := testenv.FindStateDir(result.OutputDir, ".rip")
	if err != nil {
		t.Fatalf("Failed to find state dir: %v", err)
	}

	stateDir.AssertStatus(t, model.StatusCompleted)
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

	// Run the rip
	result, err := r.Rip(context.Background(), req)
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

	// Verify .rip state directory
	stateDir, err := testenv.FindStateDir(result.OutputDir, ".rip")
	if err != nil {
		t.Fatalf("Failed to find state dir: %v", err)
	}

	stateDir.AssertStatus(t, model.StatusCompleted)
}

func TestRipper_E2E_StateCompatibleWithScanner(t *testing.T) {
	requireFFmpeg(t)
	mockPath := findMockMakeMKV(t)

	// Set up test environment
	env := testenv.New(t)

	// Create ripper with mock-makemkv
	runner := ripper.NewMakeMKVRunner(mockPath)
	r := ripper.NewRipper(env.StagingBase, runner, nil)

	// Create and run request
	req := &ripper.RipRequest{
		Type:     ripper.MediaTypeMovie,
		Name:     "Scanner Test Movie",
		DiscPath: "disc:0",
	}

	_, err := r.Rip(context.Background(), req)
	if err != nil {
		t.Fatalf("Rip failed: %v", err)
	}

	// Use the scanner to read the state
	config := env.ScannerConfig()
	s := scanner.New(config)

	state, err := s.ScanPipeline()
	if err != nil {
		t.Fatalf("Scanner.ScanPipeline failed: %v", err)
	}

	// Filter to get only movies
	movies := filterByType(state.Items, model.MediaTypeMovie)

	// Verify scanner found the movie
	if len(movies) != 1 {
		t.Errorf("Expected 1 movie, got %d", len(movies))
	}

	if len(movies) > 0 {
		movie := movies[0]
		if movie.Name != "Scanner Test Movie" {
			t.Errorf("Movie name = %q, want 'Scanner Test Movie'", movie.Name)
		}
		if movie.Current != model.StageRipped {
			t.Errorf("Current = %v, want %v", movie.Current, model.StageRipped)
		}
		if movie.Status != model.StatusCompleted {
			t.Errorf("Status = %v, want completed", movie.Status)
		}
	}
}

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

		_, err := r.Rip(context.Background(), req)
		if err != nil {
			t.Fatalf("Rip of disc %d failed: %v", disc, err)
		}
	}

	// Verify both disc directories exist with state files
	for disc := 1; disc <= 2; disc++ {
		discDir := filepath.Join(env.StagingBase, "1-ripped", "tv", "Multi_Disc_Show", "S01", fmt.Sprintf("Disc%d", disc))
		if _, err := os.Stat(discDir); os.IsNotExist(err) {
			t.Errorf("Disc%d directory does not exist: %s", disc, discDir)
		}

		// Verify state directory exists for each disc
		stateDir, err := testenv.FindStateDir(discDir, ".rip")
		if err != nil {
			t.Errorf("Disc%d has no .rip state dir: %v", disc, err)
			continue
		}
		stateDir.AssertStatus(t, model.StatusCompleted)
	}
}

func TestRipper_E2E_CLIExecution(t *testing.T) {
	requireFFmpeg(t)
	mockPath := findMockMakeMKV(t)

	// Find the ripper binary
	ripperPath := findRipperBinary(t)

	// Set up test environment
	env := testenv.New(t)

	// Run ripper CLI with mock-makemkv
	cmd := exec.Command(ripperPath, "-t", "movie", "-n", "CLI Test Movie")
	cmd.Env = append(os.Environ(),
		"MEDIA_BASE="+env.BaseDir,
		"MAKEMKVCON_PATH="+mockPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Ripper CLI failed: %v\nOutput: %s", err, output)
	}

	// Verify output directory was created
	expectedDir := filepath.Join(env.StagingBase, "1-ripped", "movies", "CLI_Test_Movie")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("Expected output directory does not exist: %s", expectedDir)
	}
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

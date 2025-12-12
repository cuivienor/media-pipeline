package ripper

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/model"
)

func TestRipper_BuildOutputDir_Movie(t *testing.T) {
	ripper := &Ripper{
		stagingBase: "/mnt/media/staging",
	}

	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "The Matrix",
	}

	outputDir := ripper.BuildOutputDir(req)
	expected := "/mnt/media/staging/1-ripped/movies/The_Matrix"

	if outputDir != expected {
		t.Errorf("BuildOutputDir = %q, want %q", outputDir, expected)
	}
}

func TestRipper_BuildOutputDir_TVShow(t *testing.T) {
	ripper := &Ripper{
		stagingBase: "/mnt/media/staging",
	}

	req := &RipRequest{
		Type:   MediaTypeTV,
		Name:   "Breaking Bad",
		Season: 1,
		Disc:   2,
	}

	outputDir := ripper.BuildOutputDir(req)
	expected := "/mnt/media/staging/1-ripped/tv/Breaking_Bad/S01/Disc2"

	if outputDir != expected {
		t.Errorf("BuildOutputDir = %q, want %q", outputDir, expected)
	}
}

func TestRipper_Rip_CreatesOutputDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{}

	ripper := NewRipper(tmpDir, mockRunner, nil)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	outputDir := filepath.Join(tmpDir, "1-ripped", "movies", "Test_Movie")
	_, err := ripper.Rip(context.Background(), req, outputDir, nil, nil)
	if err != nil {
		t.Fatalf("Rip failed: %v", err)
	}

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("Output directory was not created")
	}
}

func TestRipper_Rip_CallsMakeMKVRunner(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{}

	ripper := NewRipper(tmpDir, mockRunner, nil)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	outputDir := filepath.Join(tmpDir, "1-ripped", "movies", "Test_Movie")
	ripper.Rip(context.Background(), req, outputDir, nil, nil)

	if !mockRunner.ripTitlesCalled {
		t.Error("MakeMKVRunner.RipTitles was not called")
	}
}

func TestRipper_Rip_ReturnsCompletedStatusOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{}

	ripper := NewRipper(tmpDir, mockRunner, nil)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	outputDir := filepath.Join(tmpDir, "1-ripped", "movies", "Test_Movie")
	result, err := ripper.Rip(context.Background(), req, outputDir, nil, nil)
	if err != nil {
		t.Fatalf("Rip failed: %v", err)
	}

	if result.Status != model.StatusCompleted {
		t.Errorf("Status = %v, want %v", result.Status, model.StatusCompleted)
	}
}

func TestRipper_Rip_ReturnsFailedStatusOnError(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{
		ripError: errors.New("rip failed"),
	}

	ripper := NewRipper(tmpDir, mockRunner, nil)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	outputDir := filepath.Join(tmpDir, "1-ripped", "movies", "Test_Movie")
	result, err := ripper.Rip(context.Background(), req, outputDir, nil, nil)

	if err == nil {
		t.Error("Expected error from Rip")
	}

	if result.Status != model.StatusFailed {
		t.Errorf("Status = %v, want %v", result.Status, model.StatusFailed)
	}
}

func TestRipper_Rip_ReturnsRipResult(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{}

	ripper := NewRipper(tmpDir, mockRunner, nil)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	outputDir := filepath.Join(tmpDir, "1-ripped", "movies", "Test_Movie")
	result, err := ripper.Rip(context.Background(), req, outputDir, nil, nil)
	if err != nil {
		t.Fatalf("Rip failed: %v", err)
	}

	if result.Status != model.StatusCompleted {
		t.Errorf("Status = %v, want %v", result.Status, model.StatusCompleted)
	}

	if result.OutputDir != outputDir {
		t.Errorf("OutputDir = %q, want %q", result.OutputDir, outputDir)
	}
}

func TestRipper_Rip_ValidatesRequest(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{}

	ripper := NewRipper(tmpDir, mockRunner, nil)

	// Missing name
	req := &RipRequest{
		Type:     MediaTypeMovie,
		DiscPath: "disc:0",
	}

	outputDir := filepath.Join(tmpDir, "1-ripped", "movies", "")
	_, err := ripper.Rip(context.Background(), req, outputDir, nil, nil)
	if err == nil {
		t.Error("Expected validation error for missing name")
	}
}

func TestNewRipper_DefaultRunner(t *testing.T) {
	ripper := NewRipper("/tmp", nil, nil)

	// Should have default runner
	if ripper.runner == nil {
		t.Error("runner should not be nil with default")
	}
}

func TestRipper_Rip_CreatesOrganizationScaffolding(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{}

	ripper := NewRipper(tmpDir, mockRunner, nil)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	outputDir := filepath.Join(tmpDir, "1-ripped", "movies", "Test_Movie")
	_, err := ripper.Rip(context.Background(), req, outputDir, nil, nil)
	if err != nil {
		t.Fatalf("Rip failed: %v", err)
	}

	// Check scaffolding was created
	expectedDirs := []string{"_discarded", "_main", "_extras/trailers"}
	for _, dir := range expectedDirs {
		path := filepath.Join(outputDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected scaffolding directory %q to exist", dir)
		}
	}

	// Check _REVIEW.txt exists
	reviewPath := filepath.Join(outputDir, "_REVIEW.txt")
	if _, err := os.Stat(reviewPath); os.IsNotExist(err) {
		t.Error("Expected _REVIEW.txt to exist")
	}
}

// Test helper implementations
type testMakeMKVRunner struct {
	discInfo        *DiscInfo
	ripError        error
	ripTitlesCalled bool
}

func (m *testMakeMKVRunner) GetDiscInfo(ctx context.Context, discPath string) (*DiscInfo, error) {
	if m.discInfo != nil {
		return m.discInfo, nil
	}
	return &DiscInfo{
		Name:       "Test Disc",
		TitleCount: 1,
		Titles: []TitleInfo{
			{Index: 0, Name: "Main"},
		},
	}, nil
}

func (m *testMakeMKVRunner) RipTitles(ctx context.Context, discPath, outputDir string, titleIndices []int, onLine LineCallback, onProgress ProgressCallback) error {
	m.ripTitlesCalled = true
	return m.ripError
}

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
	mockState := &testStateManager{}

	ripper := NewRipper(tmpDir, mockRunner, mockState)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	_, err := ripper.Rip(context.Background(), req)
	if err != nil {
		t.Fatalf("Rip failed: %v", err)
	}

	expectedDir := filepath.Join(tmpDir, "1-ripped", "movies", "Test_Movie")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Error("Output directory was not created")
	}
}

func TestRipper_Rip_InitializesState(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{}
	mockState := &testStateManager{}

	ripper := NewRipper(tmpDir, mockRunner, mockState)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	ripper.Rip(context.Background(), req)

	if !mockState.initializeCalled {
		t.Error("StateManager.Initialize was not called")
	}
}

func TestRipper_Rip_CallsMakeMKVRunner(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{}
	mockState := &testStateManager{}

	ripper := NewRipper(tmpDir, mockRunner, mockState)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	ripper.Rip(context.Background(), req)

	if !mockRunner.ripTitlesCalled {
		t.Error("MakeMKVRunner.RipTitles was not called")
	}
}

func TestRipper_Rip_CompletesStateOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{}
	mockState := &testStateManager{}

	ripper := NewRipper(tmpDir, mockRunner, mockState)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	ripper.Rip(context.Background(), req)

	if !mockState.completeCalled {
		t.Error("StateManager.Complete was not called")
	}
}

func TestRipper_Rip_SetsFailedStatusOnError(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{
		ripError: errors.New("rip failed"),
	}
	mockState := &testStateManager{}

	ripper := NewRipper(tmpDir, mockRunner, mockState)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	_, err := ripper.Rip(context.Background(), req)

	if err == nil {
		t.Error("Expected error from Rip")
	}

	if mockState.lastStatus != model.StatusFailed {
		t.Errorf("lastStatus = %v, want %v", mockState.lastStatus, model.StatusFailed)
	}
}

func TestRipper_Rip_ReturnsRipResult(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{}
	mockState := &testStateManager{}

	ripper := NewRipper(tmpDir, mockRunner, mockState)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	result, err := ripper.Rip(context.Background(), req)
	if err != nil {
		t.Fatalf("Rip failed: %v", err)
	}

	if result.Status != model.StatusCompleted {
		t.Errorf("Status = %v, want %v", result.Status, model.StatusCompleted)
	}

	expectedDir := filepath.Join(tmpDir, "1-ripped", "movies", "Test_Movie")
	if result.OutputDir != expectedDir {
		t.Errorf("OutputDir = %q, want %q", result.OutputDir, expectedDir)
	}
}

func TestRipper_Rip_ValidatesRequest(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{}
	mockState := &testStateManager{}

	ripper := NewRipper(tmpDir, mockRunner, mockState)

	// Missing name
	req := &RipRequest{
		Type:     MediaTypeMovie,
		DiscPath: "disc:0",
	}

	_, err := ripper.Rip(context.Background(), req)
	if err == nil {
		t.Error("Expected validation error for missing name")
	}
}

func TestRipper_Rip_SetsErrorOnFailure(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{
		ripError: errors.New("makemkv crashed"),
	}
	mockState := &testStateManager{}

	ripper := NewRipper(tmpDir, mockRunner, mockState)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	ripper.Rip(context.Background(), req)

	if !mockState.setErrorCalled {
		t.Error("StateManager.SetError was not called")
	}
}

func TestNewRipper_DefaultStateManager(t *testing.T) {
	ripper := NewRipper("/tmp", nil, nil)

	// Should have default state manager
	if ripper.state == nil {
		t.Error("state should not be nil with default")
	}
}

func TestRipper_Rip_CreatesOrganizationScaffolding(t *testing.T) {
	tmpDir := t.TempDir()

	mockRunner := &testMakeMKVRunner{}
	mockState := &testStateManager{}

	ripper := NewRipper(tmpDir, mockRunner, mockState)

	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	_, err := ripper.Rip(context.Background(), req)
	if err != nil {
		t.Fatalf("Rip failed: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "1-ripped", "movies", "Test_Movie")

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

func (m *testMakeMKVRunner) RipTitles(ctx context.Context, discPath, outputDir string, titleIndices []int, progress ProgressCallback) error {
	m.ripTitlesCalled = true
	return m.ripError
}

type testStateManager struct {
	initializeCalled bool
	completeCalled   bool
	setErrorCalled   bool
	lastStatus       model.Status
}

func (s *testStateManager) Initialize(outputDir string, request *RipRequest) error {
	s.initializeCalled = true
	return nil
}

func (s *testStateManager) SetStatus(outputDir string, status model.Status) error {
	s.lastStatus = status
	return nil
}

func (s *testStateManager) SetError(outputDir string, err error) error {
	s.setErrorCalled = true
	return nil
}

func (s *testStateManager) Complete(outputDir string) error {
	s.completeCalled = true
	return nil
}

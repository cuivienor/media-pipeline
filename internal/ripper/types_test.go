package ripper

import (
	"context"
	"testing"
	"time"

	"github.com/cuivienor/media-pipeline/internal/model"
)

func TestMediaType_Constants(t *testing.T) {
	// Verify media type constants match model package
	if MediaTypeMovie != "movie" {
		t.Errorf("MediaTypeMovie = %q, want movie", MediaTypeMovie)
	}
	if MediaTypeTV != "tv" {
		t.Errorf("MediaTypeTV = %q, want tv", MediaTypeTV)
	}
}

func TestRipRequest_Validate_Movie(t *testing.T) {
	req := &RipRequest{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		DiscPath: "disc:0",
	}

	if err := req.Validate(); err != nil {
		t.Errorf("Valid movie request failed validation: %v", err)
	}
}

func TestRipRequest_Validate_TV(t *testing.T) {
	req := &RipRequest{
		Type:     MediaTypeTV,
		Name:     "Test Show",
		Season:   1,
		Disc:     1,
		DiscPath: "disc:0",
	}

	if err := req.Validate(); err != nil {
		t.Errorf("Valid TV request failed validation: %v", err)
	}
}

func TestRipRequest_Validate_TVMissingSeason(t *testing.T) {
	req := &RipRequest{
		Type:     MediaTypeTV,
		Name:     "Test Show",
		Disc:     1,
		DiscPath: "disc:0",
	}

	if err := req.Validate(); err == nil {
		t.Error("TV request without season should fail validation")
	}
}

func TestRipRequest_Validate_TVMissingDisc(t *testing.T) {
	req := &RipRequest{
		Type:     MediaTypeTV,
		Name:     "Test Show",
		Season:   1,
		DiscPath: "disc:0",
	}

	if err := req.Validate(); err == nil {
		t.Error("TV request without disc should fail validation")
	}
}

func TestRipRequest_Validate_MissingName(t *testing.T) {
	req := &RipRequest{
		Type:     MediaTypeMovie,
		DiscPath: "disc:0",
	}

	if err := req.Validate(); err == nil {
		t.Error("Request without name should fail validation")
	}
}

func TestRipRequest_SafeName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Test Movie", "Test_Movie"},
		{"The Matrix: Reloaded", "The_Matrix_Reloaded"},
		{"Movie (2024)", "Movie_2024"},
		{"It's a Test!", "Its_a_Test"},
	}

	for _, tt := range tests {
		req := &RipRequest{Name: tt.name}
		if got := req.SafeName(); got != tt.want {
			t.Errorf("SafeName(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestRipResult_Duration(t *testing.T) {
	result := &RipResult{
		StartedAt:   time.Now().Add(-5 * time.Minute),
		CompletedAt: time.Now(),
	}

	duration := result.Duration()
	if duration < 4*time.Minute || duration > 6*time.Minute {
		t.Errorf("Duration = %v, want ~5 minutes", duration)
	}
}

func TestRipResult_IsSuccess(t *testing.T) {
	success := &RipResult{Status: model.StatusCompleted}
	if !success.IsSuccess() {
		t.Error("Completed result should be success")
	}

	failed := &RipResult{Status: model.StatusFailed}
	if failed.IsSuccess() {
		t.Error("Failed result should not be success")
	}
}

// Test that interfaces can be implemented (compile-time check)
func TestMakeMKVRunner_Interface(t *testing.T) {
	var _ MakeMKVRunner = (*mockRunner)(nil)
}

func TestStateManager_Interface(t *testing.T) {
	var _ StateManager = (*mockStateManager)(nil)
}

// Mock implementations for interface testing
type mockRunner struct{}

func (m *mockRunner) GetDiscInfo(ctx context.Context, discPath string) (*DiscInfo, error) {
	return &DiscInfo{}, nil
}

func (m *mockRunner) RipTitles(ctx context.Context, discPath, outputDir string, titleIndices []int, progress ProgressCallback) error {
	return nil
}

type mockStateManager struct{}

func (m *mockStateManager) Initialize(outputDir string, request *RipRequest) error {
	return nil
}

func (m *mockStateManager) SetStatus(outputDir string, status model.Status) error {
	return nil
}

func (m *mockStateManager) SetError(outputDir string, err error) error {
	return nil
}

func (m *mockStateManager) Complete(outputDir string) error {
	return nil
}

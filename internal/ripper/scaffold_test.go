package ripper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateOrganizationScaffolding_Movie_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "The Matrix",
	}

	err := CreateOrganizationScaffolding(tmpDir, req)
	if err != nil {
		t.Fatalf("CreateOrganizationScaffolding failed: %v", err)
	}

	// Check common directories
	expectedDirs := []string{
		"_discarded",
		"_extras/behind the scenes",
		"_extras/deleted scenes",
		"_extras/featurettes",
		"_extras/interviews",
		"_extras/scenes",
		"_extras/shorts",
		"_extras/trailers",
		"_extras/other",
		"_main", // Movie-specific
	}

	for _, dir := range expectedDirs {
		path := filepath.Join(tmpDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected directory %q to exist", dir)
		}
	}
}

func TestCreateOrganizationScaffolding_Movie_CreatesReviewFile(t *testing.T) {
	tmpDir := t.TempDir()

	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "The Matrix",
	}

	err := CreateOrganizationScaffolding(tmpDir, req)
	if err != nil {
		t.Fatalf("CreateOrganizationScaffolding failed: %v", err)
	}

	reviewPath := filepath.Join(tmpDir, "_REVIEW.txt")
	if _, err := os.Stat(reviewPath); os.IsNotExist(err) {
		t.Error("Expected _REVIEW.txt to exist")
	}

	// Check content contains movie name
	content, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("Failed to read _REVIEW.txt: %v", err)
	}

	if !contains(string(content), "The Matrix") {
		t.Error("_REVIEW.txt should contain movie name")
	}

	if !contains(string(content), "Movie:") {
		t.Error("_REVIEW.txt should indicate it's a movie")
	}
}

func TestCreateOrganizationScaffolding_TV_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	req := &RipRequest{
		Type:   MediaTypeTV,
		Name:   "Breaking Bad",
		Season: 2,
		Disc:   3,
	}

	err := CreateOrganizationScaffolding(tmpDir, req)
	if err != nil {
		t.Fatalf("CreateOrganizationScaffolding failed: %v", err)
	}

	// Check common directories
	expectedDirs := []string{
		"_discarded",
		"_extras/behind the scenes",
		"_extras/deleted scenes",
		"_extras/featurettes",
		"_extras/interviews",
		"_extras/scenes",
		"_extras/shorts",
		"_extras/trailers",
		"_extras/other",
		"_episodes", // TV-specific
	}

	for _, dir := range expectedDirs {
		path := filepath.Join(tmpDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected directory %q to exist", dir)
		}
	}

	// Verify _main does NOT exist for TV shows
	mainPath := filepath.Join(tmpDir, "_main")
	if _, err := os.Stat(mainPath); err == nil {
		t.Error("_main should not exist for TV shows")
	}
}

func TestCreateOrganizationScaffolding_TV_CreatesReviewFile(t *testing.T) {
	tmpDir := t.TempDir()

	req := &RipRequest{
		Type:   MediaTypeTV,
		Name:   "Breaking Bad",
		Season: 2,
		Disc:   3,
	}

	err := CreateOrganizationScaffolding(tmpDir, req)
	if err != nil {
		t.Fatalf("CreateOrganizationScaffolding failed: %v", err)
	}

	reviewPath := filepath.Join(tmpDir, "_REVIEW.txt")
	content, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("Failed to read _REVIEW.txt: %v", err)
	}

	// Check TV-specific content
	if !contains(string(content), "Breaking Bad") {
		t.Error("_REVIEW.txt should contain show name")
	}

	if !contains(string(content), "Season: 2") {
		t.Error("_REVIEW.txt should contain season number")
	}

	if !contains(string(content), "Disc: 3") {
		t.Error("_REVIEW.txt should contain disc number")
	}

	if !contains(string(content), "Show:") {
		t.Error("_REVIEW.txt should indicate it's a TV show")
	}
}

func TestCreateOrganizationScaffolding_ExistingDirectory_NoError(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create a directory
	os.MkdirAll(filepath.Join(tmpDir, "_extras", "trailers"), 0755)

	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Test",
	}

	// Should not error on existing directories
	err := CreateOrganizationScaffolding(tmpDir, req)
	if err != nil {
		t.Errorf("Should not error on existing directories: %v", err)
	}
}

func TestCreateOrganizationScaffolding_InvalidOutputDir_ReturnsError(t *testing.T) {
	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Test",
	}

	// Non-existent path that can't be created
	err := CreateOrganizationScaffolding("/nonexistent/path/that/should/fail", req)
	if err == nil {
		t.Error("Expected error for invalid output directory")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

package testenv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/model"
)

func TestFindStateDir_FindsRipDir(t *testing.T) {
	env := New(t)

	// Create a movie with .rip state directory
	movieDir := env.CreateMovieDir("Test_Movie")
	ripDir := filepath.Join(movieDir, ".rip")
	if err := os.MkdirAll(ripDir, 0755); err != nil {
		t.Fatalf("failed to create .rip dir: %v", err)
	}

	// Should find the state directory
	stateDir, err := FindStateDir(movieDir, ".rip")
	if err != nil {
		t.Fatalf("FindStateDir failed: %v", err)
	}
	if stateDir.Path != ripDir {
		t.Errorf("Path = %q, want %q", stateDir.Path, ripDir)
	}
}

func TestFindStateDir_ReturnsErrorIfNotFound(t *testing.T) {
	env := New(t)
	movieDir := env.CreateMovieDir("Test_Movie")

	_, err := FindStateDir(movieDir, ".rip")
	if err == nil {
		t.Error("Expected error when state dir not found")
	}
}

func TestStateDir_ReadStatus(t *testing.T) {
	env := New(t)
	movieDir := env.CreateMovieDir("Test_Movie")
	ripDir := filepath.Join(movieDir, ".rip")
	os.MkdirAll(ripDir, 0755)

	// Write status file
	os.WriteFile(filepath.Join(ripDir, "status"), []byte("completed"), 0644)

	stateDir := &StateDir{Path: ripDir}
	status, err := stateDir.ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus failed: %v", err)
	}
	if status != model.StatusCompleted {
		t.Errorf("status = %q, want completed", status)
	}
}

func TestStateDir_ReadMetadata(t *testing.T) {
	env := New(t)
	movieDir := env.CreateMovieDir("Test_Movie")
	ripDir := filepath.Join(movieDir, ".rip")
	os.MkdirAll(ripDir, 0755)

	// Write metadata.json
	metadata := map[string]interface{}{
		"type":      "movie",
		"name":      "Test Movie",
		"safe_name": "Test_Movie",
	}
	data, _ := json.Marshal(metadata)
	os.WriteFile(filepath.Join(ripDir, "metadata.json"), data, 0644)

	stateDir := &StateDir{Path: ripDir}
	meta, err := stateDir.ReadMetadata()
	if err != nil {
		t.Fatalf("ReadMetadata failed: %v", err)
	}
	if meta["type"] != "movie" {
		t.Errorf("type = %v, want movie", meta["type"])
	}
	if meta["name"] != "Test Movie" {
		t.Errorf("name = %v, want 'Test Movie'", meta["name"])
	}
}

func TestAssertStatus_PassesForCorrectStatus(t *testing.T) {
	env := New(t)
	movieDir := env.CreateMovieDir("Test_Movie")
	ripDir := filepath.Join(movieDir, ".rip")
	os.MkdirAll(ripDir, 0755)
	os.WriteFile(filepath.Join(ripDir, "status"), []byte("completed"), 0644)

	stateDir := &StateDir{Path: ripDir}
	// Should not fail
	stateDir.AssertStatus(t, model.StatusCompleted)
}

func TestAssertMetadataContains_PassesForMatchingFields(t *testing.T) {
	env := New(t)
	movieDir := env.CreateMovieDir("Test_Movie")
	ripDir := filepath.Join(movieDir, ".rip")
	os.MkdirAll(ripDir, 0755)

	metadata := map[string]interface{}{
		"type":      "movie",
		"name":      "Test Movie",
		"safe_name": "Test_Movie",
	}
	data, _ := json.Marshal(metadata)
	os.WriteFile(filepath.Join(ripDir, "metadata.json"), data, 0644)

	stateDir := &StateDir{Path: ripDir}
	// Should not fail
	stateDir.AssertMetadataContains(t, "type", "movie")
	stateDir.AssertMetadataContains(t, "name", "Test Movie")
}

func TestAssertHasFile_PassesWhenFileExists(t *testing.T) {
	env := New(t)
	movieDir := env.CreateMovieDir("Test_Movie")
	ripDir := filepath.Join(movieDir, ".rip")
	os.MkdirAll(ripDir, 0755)
	os.WriteFile(filepath.Join(ripDir, "rip.log"), []byte("log content"), 0644)

	stateDir := &StateDir{Path: ripDir}
	// Should not fail
	stateDir.AssertHasFile(t, "rip.log")
}

func TestAssertNoFile_PassesWhenFileMissing(t *testing.T) {
	env := New(t)
	movieDir := env.CreateMovieDir("Test_Movie")
	ripDir := filepath.Join(movieDir, ".rip")
	os.MkdirAll(ripDir, 0755)

	stateDir := &StateDir{Path: ripDir}
	// Should not fail - pid file should not exist
	stateDir.AssertNoFile(t, "pid")
}

func TestCreateStateFixture_CreatesValidState(t *testing.T) {
	env := New(t)
	movieDir := env.CreateMovieDir("Test_Movie")

	fixture := StateFixture{
		Type:     "movie",
		Name:     "Test Movie",
		SafeName: "Test_Movie",
		Status:   model.StatusCompleted,
	}

	stateDir := CreateStateFixture(t, movieDir, ".rip", fixture)

	// Verify status
	status, _ := stateDir.ReadStatus()
	if status != model.StatusCompleted {
		t.Errorf("status = %q, want completed", status)
	}

	// Verify metadata
	meta, _ := stateDir.ReadMetadata()
	if meta["type"] != "movie" {
		t.Errorf("type = %v, want movie", meta["type"])
	}
}

package testenv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuivienor/media-pipeline/internal/model"
)

// StateDir represents a state directory (.rip/, .remux/, etc.) for assertions
type StateDir struct {
	Path string
}

// StateFixture defines data for creating test state directories
type StateFixture struct {
	Type      string       // "movie" or "tv"
	Name      string       // Human-readable name
	SafeName  string       // Filesystem-safe name
	Season    string       // Season identifier (TV only)
	Status    model.Status // Current status
	StartedAt time.Time    // When the job started (optional, defaults to now)
}

// FindStateDir locates a state directory under a base path
func FindStateDir(basePath, stateDirName string) (*StateDir, error) {
	statePath := filepath.Join(basePath, stateDirName)
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("state directory %s not found under %s", stateDirName, basePath)
	}
	return &StateDir{Path: statePath}, nil
}

// ReadStatus reads and returns the status from the status file
func (s *StateDir) ReadStatus() (model.Status, error) {
	content, err := os.ReadFile(filepath.Join(s.Path, "status"))
	if err != nil {
		return "", fmt.Errorf("failed to read status file: %w", err)
	}
	return model.Status(strings.TrimSpace(string(content))), nil
}

// ReadMetadata reads and returns the metadata from metadata.json
func (s *StateDir) ReadMetadata() (map[string]interface{}, error) {
	content, err := os.ReadFile(filepath.Join(s.Path, "metadata.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata.json: %w", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(content, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata.json: %w", err)
	}

	return metadata, nil
}

// HasFile returns true if the specified file exists in the state directory
func (s *StateDir) HasFile(name string) bool {
	_, err := os.Stat(filepath.Join(s.Path, name))
	return err == nil
}

// ReadFile reads and returns the contents of a file in the state directory
func (s *StateDir) ReadFile(name string) (string, error) {
	content, err := os.ReadFile(filepath.Join(s.Path, name))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// --- Assertion helpers ---

// AssertStatus verifies the status file contains the expected value
func (s *StateDir) AssertStatus(t *testing.T, expected model.Status) {
	t.Helper()
	status, err := s.ReadStatus()
	if err != nil {
		t.Fatalf("failed to read status: %v", err)
	}
	if status != expected {
		t.Errorf("status = %q, want %q", status, expected)
	}
}

// AssertMetadataContains verifies metadata.json contains a specific key-value pair
func (s *StateDir) AssertMetadataContains(t *testing.T, key string, expected interface{}) {
	t.Helper()
	meta, err := s.ReadMetadata()
	if err != nil {
		t.Fatalf("failed to read metadata: %v", err)
	}

	actual, exists := meta[key]
	if !exists {
		t.Errorf("metadata missing key %q", key)
		return
	}

	// Compare as strings for simplicity
	if fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected) {
		t.Errorf("metadata[%q] = %v, want %v", key, actual, expected)
	}
}

// AssertHasFile verifies a file exists in the state directory
func (s *StateDir) AssertHasFile(t *testing.T, name string) {
	t.Helper()
	if !s.HasFile(name) {
		t.Errorf("expected file %s to exist in %s", name, s.Path)
	}
}

// AssertNoFile verifies a file does NOT exist in the state directory
func (s *StateDir) AssertNoFile(t *testing.T, name string) {
	t.Helper()
	if s.HasFile(name) {
		t.Errorf("expected file %s to NOT exist in %s", name, s.Path)
	}
}

// AssertFileContains verifies a file contains expected content
func (s *StateDir) AssertFileContains(t *testing.T, name, expected string) {
	t.Helper()
	content, err := s.ReadFile(name)
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}
	if !strings.Contains(content, expected) {
		t.Errorf("file %s does not contain %q", name, expected)
	}
}

// --- Fixture creation ---

// CreateStateFixture creates a state directory with the specified fixture data
func CreateStateFixture(t *testing.T, basePath, stateDirName string, fixture StateFixture) *StateDir {
	t.Helper()

	stateDir := filepath.Join(basePath, stateDirName)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("failed to create state directory: %v", err)
	}

	// Set default started time if not provided
	startedAt := fixture.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}

	// Write metadata.json
	metadata := map[string]interface{}{
		"type":       fixture.Type,
		"name":       fixture.Name,
		"safe_name":  fixture.SafeName,
		"started_at": startedAt.Format(time.RFC3339),
	}
	if fixture.Season != "" {
		metadata["season"] = fixture.Season
	}

	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "metadata.json"), metadataBytes, 0644); err != nil {
		t.Fatalf("failed to write metadata.json: %v", err)
	}

	// Write status file
	if err := os.WriteFile(filepath.Join(stateDir, "status"), []byte(string(fixture.Status)), 0644); err != nil {
		t.Fatalf("failed to write status: %v", err)
	}

	// Write started_at file
	if err := os.WriteFile(filepath.Join(stateDir, "started_at"), []byte(startedAt.Format(time.RFC3339)), 0644); err != nil {
		t.Fatalf("failed to write started_at: %v", err)
	}

	return &StateDir{Path: stateDir}
}

// CreateCompletedStateFixture creates a completed state fixture with completion timestamp
func CreateCompletedStateFixture(t *testing.T, basePath, stateDirName string, fixture StateFixture) *StateDir {
	t.Helper()

	fixture.Status = model.StatusCompleted
	stateDir := CreateStateFixture(t, basePath, stateDirName, fixture)

	// Add completed_at timestamp
	completedAt := time.Now()
	if err := os.WriteFile(filepath.Join(stateDir.Path, "completed_at"), []byte(completedAt.Format(time.RFC3339)), 0644); err != nil {
		t.Fatalf("failed to write completed_at: %v", err)
	}

	return stateDir
}

// CreateFailedStateFixture creates a failed state fixture with error message
func CreateFailedStateFixture(t *testing.T, basePath, stateDirName string, fixture StateFixture, errorMsg string) *StateDir {
	t.Helper()

	fixture.Status = model.StatusFailed
	stateDir := CreateStateFixture(t, basePath, stateDirName, fixture)

	// Add error file
	if err := os.WriteFile(filepath.Join(stateDir.Path, "error"), []byte(errorMsg), 0644); err != nil {
		t.Fatalf("failed to write error: %v", err)
	}

	return stateDir
}

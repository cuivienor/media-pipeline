package ripper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/model"
)

func TestDefaultStateManager_Initialize_CreatesStateDir(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "Test_Movie")
	os.MkdirAll(outputDir, 0755)

	sm := NewStateManager()
	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Test Movie",
	}

	err := sm.Initialize(outputDir, req)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	stateDir := filepath.Join(outputDir, ".rip")
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		t.Error("State directory was not created")
	}
}

func TestDefaultStateManager_Initialize_WritesMetadataJSON(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "Test_Movie")
	os.MkdirAll(outputDir, 0755)

	sm := NewStateManager()
	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Test Movie",
	}

	sm.Initialize(outputDir, req)

	metadataPath := filepath.Join(outputDir, ".rip", "metadata.json")
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata.json: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(content, &metadata); err != nil {
		t.Fatalf("Failed to parse metadata.json: %v", err)
	}

	if metadata["type"] != "movie" {
		t.Errorf("type = %v, want movie", metadata["type"])
	}
	if metadata["name"] != "Test Movie" {
		t.Errorf("name = %v, want 'Test Movie'", metadata["name"])
	}
	if metadata["safe_name"] != "Test_Movie" {
		t.Errorf("safe_name = %v, want Test_Movie", metadata["safe_name"])
	}
}

func TestDefaultStateManager_Initialize_WritesStatusInProgress(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "Test_Movie")
	os.MkdirAll(outputDir, 0755)

	sm := NewStateManager()
	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Test Movie",
	}

	sm.Initialize(outputDir, req)

	statusPath := filepath.Join(outputDir, ".rip", "status")
	content, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("Failed to read status: %v", err)
	}

	if strings.TrimSpace(string(content)) != "in_progress" {
		t.Errorf("status = %q, want in_progress", content)
	}
}

func TestDefaultStateManager_Initialize_WritesStartedAt(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "Test_Movie")
	os.MkdirAll(outputDir, 0755)

	sm := NewStateManager()
	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Test Movie",
	}

	sm.Initialize(outputDir, req)

	startedAtPath := filepath.Join(outputDir, ".rip", "started_at")
	if _, err := os.Stat(startedAtPath); os.IsNotExist(err) {
		t.Error("started_at file was not created")
	}
}

func TestDefaultStateManager_Initialize_WritesPID(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "Test_Movie")
	os.MkdirAll(outputDir, 0755)

	sm := NewStateManager()
	req := &RipRequest{
		Type: MediaTypeMovie,
		Name: "Test Movie",
	}

	sm.Initialize(outputDir, req)

	pidPath := filepath.Join(outputDir, ".rip", "pid")
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Error("pid file was not created")
	}
}

func TestDefaultStateManager_Initialize_TVShow(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "Test_Show", "S01", "Disc1")
	os.MkdirAll(outputDir, 0755)

	sm := NewStateManager()
	req := &RipRequest{
		Type:   MediaTypeTV,
		Name:   "Test Show",
		Season: 1,
		Disc:   1,
	}

	sm.Initialize(outputDir, req)

	metadataPath := filepath.Join(outputDir, ".rip", "metadata.json")
	content, _ := os.ReadFile(metadataPath)
	var metadata map[string]interface{}
	json.Unmarshal(content, &metadata)

	if metadata["type"] != "tv" {
		t.Errorf("type = %v, want tv", metadata["type"])
	}
	if metadata["season"] != "S01" {
		t.Errorf("season = %v, want S01", metadata["season"])
	}
}

func TestDefaultStateManager_SetStatus_UpdatesStatusFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "Test_Movie")
	os.MkdirAll(outputDir, 0755)

	sm := NewStateManager()
	req := &RipRequest{Type: MediaTypeMovie, Name: "Test Movie"}
	sm.Initialize(outputDir, req)

	err := sm.SetStatus(outputDir, model.StatusFailed)
	if err != nil {
		t.Fatalf("SetStatus failed: %v", err)
	}

	statusPath := filepath.Join(outputDir, ".rip", "status")
	content, _ := os.ReadFile(statusPath)
	if strings.TrimSpace(string(content)) != "failed" {
		t.Errorf("status = %q, want failed", content)
	}
}

func TestDefaultStateManager_SetError_WritesErrorFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "Test_Movie")
	os.MkdirAll(outputDir, 0755)

	sm := NewStateManager()
	req := &RipRequest{Type: MediaTypeMovie, Name: "Test Movie"}
	sm.Initialize(outputDir, req)

	testErr := os.ErrNotExist
	err := sm.SetError(outputDir, testErr)
	if err != nil {
		t.Fatalf("SetError failed: %v", err)
	}

	errorPath := filepath.Join(outputDir, ".rip", "error")
	content, _ := os.ReadFile(errorPath)
	if !strings.Contains(string(content), "not exist") {
		t.Errorf("error file should contain error message, got: %s", content)
	}
}

func TestDefaultStateManager_Complete_SetsStatusCompleted(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "Test_Movie")
	os.MkdirAll(outputDir, 0755)

	sm := NewStateManager()
	req := &RipRequest{Type: MediaTypeMovie, Name: "Test Movie"}
	sm.Initialize(outputDir, req)

	err := sm.Complete(outputDir)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	statusPath := filepath.Join(outputDir, ".rip", "status")
	content, _ := os.ReadFile(statusPath)
	if strings.TrimSpace(string(content)) != "completed" {
		t.Errorf("status = %q, want completed", content)
	}
}

func TestDefaultStateManager_Complete_WritesCompletedAt(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "Test_Movie")
	os.MkdirAll(outputDir, 0755)

	sm := NewStateManager()
	req := &RipRequest{Type: MediaTypeMovie, Name: "Test Movie"}
	sm.Initialize(outputDir, req)
	sm.Complete(outputDir)

	completedAtPath := filepath.Join(outputDir, ".rip", "completed_at")
	if _, err := os.Stat(completedAtPath); os.IsNotExist(err) {
		t.Error("completed_at file was not created")
	}
}

func TestDefaultStateManager_Complete_RemovesPID(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "Test_Movie")
	os.MkdirAll(outputDir, 0755)

	sm := NewStateManager()
	req := &RipRequest{Type: MediaTypeMovie, Name: "Test Movie"}
	sm.Initialize(outputDir, req)

	// Verify PID exists before complete
	pidPath := filepath.Join(outputDir, ".rip", "pid")
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Fatal("pid file should exist after Initialize")
	}

	sm.Complete(outputDir)

	// Verify PID removed after complete
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("pid file should be removed after Complete")
	}
}

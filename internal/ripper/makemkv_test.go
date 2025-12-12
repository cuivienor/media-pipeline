package ripper

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultMakeMKVRunner_GetDiscInfo_ParsesOutput(t *testing.T) {
	// Test that GetDiscInfo correctly parses makemkvcon info output
	// using a simulated output

	runner := &DefaultMakeMKVRunner{
		execCommand: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Return a command that outputs test data
			return exec.CommandContext(ctx, "echo", `CINFO:2,0,"Test Disc"
CINFO:32,0,"TESTID"
TCOUT:2
TINFO:0,2,0,"Main Feature"
TINFO:0,9,0,"1:30:00"
TINFO:1,2,0,"Bonus"
TINFO:1,9,0,"0:10:00"`)
		},
	}

	info, err := runner.GetDiscInfo(context.Background(), "disc:0")
	if err != nil {
		t.Fatalf("GetDiscInfo failed: %v", err)
	}

	if info.Name != "Test Disc" {
		t.Errorf("Name = %q, want 'Test Disc'", info.Name)
	}
	if info.ID != "TESTID" {
		t.Errorf("ID = %q, want 'TESTID'", info.ID)
	}
	if info.TitleCount != 2 {
		t.Errorf("TitleCount = %d, want 2", info.TitleCount)
	}
	if len(info.Titles) != 2 {
		t.Errorf("len(Titles) = %d, want 2", len(info.Titles))
	}
}

func TestDefaultMakeMKVRunner_RipTitles_CallsProgressCallback(t *testing.T) {
	tmpDir := t.TempDir()

	progressCalls := 0
	var lastProgress Progress

	runner := &DefaultMakeMKVRunner{
		execCommand: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Return a command that outputs progress
			return exec.CommandContext(ctx, "echo", `PRGV:0,0,65536
PRGV:32768,0,65536
PRGV:65536,0,65536`)
		},
	}

	err := runner.RipTitles(context.Background(), "disc:0", tmpDir, nil, nil, func(p Progress) {
		progressCalls++
		lastProgress = p
	})

	if err != nil {
		t.Fatalf("RipTitles failed: %v", err)
	}

	if progressCalls != 3 {
		t.Errorf("progressCalls = %d, want 3", progressCalls)
	}

	// Last progress should be 100%
	if lastProgress.Percent != 100.0 {
		t.Errorf("lastProgress.Percent = %v, want 100.0", lastProgress.Percent)
	}
}

func TestDefaultMakeMKVRunner_BuildInfoArgs(t *testing.T) {
	runner := NewMakeMKVRunner("")

	args := runner.buildInfoArgs("disc:0")

	expected := []string{"-r", "--noscan", "info", "disc:0"}
	if !stringSliceEqual(args, expected) {
		t.Errorf("buildInfoArgs = %v, want %v", args, expected)
	}
}

func TestDefaultMakeMKVRunner_BuildMkvArgs_AllTitles(t *testing.T) {
	runner := NewMakeMKVRunner("")

	args := runner.buildMkvArgs("disc:0", "/output", nil)

	expected := []string{"-r", "--noscan", "mkv", "disc:0", "all", "/output"}
	if !stringSliceEqual(args, expected) {
		t.Errorf("buildMkvArgs = %v, want %v", args, expected)
	}
}

func TestDefaultMakeMKVRunner_BuildMkvArgs_SpecificTitles(t *testing.T) {
	runner := NewMakeMKVRunner("")

	args := runner.buildMkvArgs("disc:0", "/output", []int{0, 2, 5})

	// Should have three separate title arguments
	if !strings.Contains(strings.Join(args, " "), "0") {
		t.Error("args should contain title index 0")
	}
}

func TestDefaultMakeMKVRunner_ContextCancellation(t *testing.T) {
	runner := &DefaultMakeMKVRunner{
		execCommand: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Return a command that takes a while
			return exec.CommandContext(ctx, "sleep", "10")
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := runner.GetDiscInfo(ctx, "disc:0")

	// Should fail due to context cancellation
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
}

func TestNewMakeMKVRunner_DefaultPath(t *testing.T) {
	runner := NewMakeMKVRunner("")

	if runner.makemkvconPath != "makemkvcon" {
		t.Errorf("makemkvconPath = %q, want 'makemkvcon'", runner.makemkvconPath)
	}
}

func TestNewMakeMKVRunner_CustomPath(t *testing.T) {
	runner := NewMakeMKVRunner("/usr/local/bin/makemkvcon")

	if runner.makemkvconPath != "/usr/local/bin/makemkvcon" {
		t.Errorf("makemkvconPath = %q, want '/usr/local/bin/makemkvcon'", runner.makemkvconPath)
	}
}

// Integration test - only runs if mock-makemkv is available
func TestDefaultMakeMKVRunner_Integration_WithMock(t *testing.T) {
	// Look for mock-makemkv in bin/ or PATH
	mockPath := findMockMakeMKV()
	if mockPath == "" {
		t.Skip("mock-makemkv not found, skipping integration test")
	}

	runner := NewMakeMKVRunner(mockPath)

	// Test GetDiscInfo
	info, err := runner.GetDiscInfo(context.Background(), "disc:0")
	if err != nil {
		t.Fatalf("GetDiscInfo failed: %v", err)
	}

	if info.Name == "" {
		t.Error("Expected disc name from mock")
	}
}

// Helper to find mock-makemkv
func findMockMakeMKV() string {
	// Check bin/ directory first
	binPath := filepath.Join(".", "bin", "mock-makemkv")
	if _, err := os.Stat(binPath); err == nil {
		return binPath
	}

	// Check two levels up (when running from internal/ripper/)
	binPath = filepath.Join("..", "..", "bin", "mock-makemkv")
	if _, err := os.Stat(binPath); err == nil {
		return binPath
	}

	// Check PATH
	path, err := exec.LookPath("mock-makemkv")
	if err == nil {
		return path
	}

	return ""
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

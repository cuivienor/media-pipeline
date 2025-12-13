//go:build integration

package integration

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemuxBinary_BuildsAndShowsUsage(t *testing.T) {
	// Build the binary
	binPath := filepath.Join(t.TempDir(), "remux")
	buildCmd := exec.Command("go", "build", "-o", binPath, "../../cmd/remux")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build remux binary: %v\nOutput: %s", err, output)
	}

	// Run without required flags - should show usage
	cmd := exec.Command(binPath)
	output, err := cmd.CombinedOutput()

	// Binary should exit with error (non-zero) when missing flags
	if err == nil {
		t.Fatal("Expected error when running without required flags")
	}

	// Should show usage information
	if !strings.Contains(string(output), "Usage:") || !strings.Contains(string(output), "job-id") {
		t.Errorf("Expected usage message with job-id, got: %s", output)
	}
}

func TestTranscodeBinary_BuildsAndShowsUsage(t *testing.T) {
	// Build the binary
	binPath := filepath.Join(t.TempDir(), "transcode")
	buildCmd := exec.Command("go", "build", "-o", binPath, "../../cmd/transcode")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build transcode binary: %v\nOutput: %s", err, output)
	}

	// Run without required flags - should show usage
	cmd := exec.Command(binPath)
	output, err := cmd.CombinedOutput()

	// Binary should exit with error (non-zero) when missing flags
	if err == nil {
		t.Fatal("Expected error when running without required flags")
	}

	// Should show usage information
	if !strings.Contains(string(output), "Usage:") || !strings.Contains(string(output), "job-id") {
		t.Errorf("Expected usage message with job-id, got: %s", output)
	}
}

func TestPublishBinary_BuildsAndShowsUsage(t *testing.T) {
	// Build the binary
	binPath := filepath.Join(t.TempDir(), "publish")
	buildCmd := exec.Command("go", "build", "-o", binPath, "../../cmd/publish")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build publish binary: %v\nOutput: %s", err, output)
	}

	// Run without required flags - should show usage
	cmd := exec.Command(binPath)
	output, err := cmd.CombinedOutput()

	// Binary should exit with error (non-zero) when missing flags
	if err == nil {
		t.Fatal("Expected error when running without required flags")
	}

	// Should show usage information
	if !strings.Contains(string(output), "Usage:") || !strings.Contains(string(output), "job-id") {
		t.Errorf("Expected usage message with job-id, got: %s", output)
	}
}

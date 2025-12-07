package testenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_CreatesDirectoryStructure(t *testing.T) {
	env := New(t)

	// Check staging directories exist
	stagingDirs := []string{
		filepath.Join(env.StagingBase, "1-ripped", "movies"),
		filepath.Join(env.StagingBase, "1-ripped", "tv"),
		filepath.Join(env.StagingBase, "2-remuxed", "movies"),
		filepath.Join(env.StagingBase, "2-remuxed", "tv"),
		filepath.Join(env.StagingBase, "3-transcoded", "movies"),
		filepath.Join(env.StagingBase, "3-transcoded", "tv"),
	}

	for _, dir := range stagingDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory %s to exist", dir)
		}
	}

	// Check library and bin directories
	if _, err := os.Stat(env.LibraryBase); os.IsNotExist(err) {
		t.Error("Expected LibraryBase directory to exist")
	}
	if _, err := os.Stat(env.MockBinDir); os.IsNotExist(err) {
		t.Error("Expected MockBinDir directory to exist")
	}
}

func TestNew_SetsPathsCorrectly(t *testing.T) {
	env := New(t)

	// All paths should be under the same base
	if !strings.HasPrefix(env.StagingBase, env.BaseDir) {
		t.Errorf("StagingBase %q not under BaseDir %q", env.StagingBase, env.BaseDir)
	}
	if !strings.HasPrefix(env.LibraryBase, env.BaseDir) {
		t.Errorf("LibraryBase %q not under BaseDir %q", env.LibraryBase, env.BaseDir)
	}
	if !strings.HasPrefix(env.MockBinDir, env.BaseDir) {
		t.Errorf("MockBinDir %q not under BaseDir %q", env.MockBinDir, env.BaseDir)
	}
}

func TestEnvVars_ContainsMediaBase(t *testing.T) {
	env := New(t)
	vars := env.EnvVars()

	found := false
	for _, v := range vars {
		if strings.HasPrefix(v, "MEDIA_BASE=") {
			found = true
			break
		}
	}
	if !found {
		t.Error("EnvVars should contain MEDIA_BASE")
	}
}

func TestEnvVars_PrependsMockBinDir(t *testing.T) {
	env := New(t)
	vars := env.EnvVars()

	var pathVar string
	for _, v := range vars {
		if strings.HasPrefix(v, "PATH=") {
			pathVar = v
			break
		}
	}

	if pathVar == "" {
		t.Fatal("EnvVars should contain PATH")
	}

	// MockBinDir should be at the start of PATH
	pathValue := strings.TrimPrefix(pathVar, "PATH=")
	if !strings.HasPrefix(pathValue, env.MockBinDir) {
		t.Errorf("PATH should start with MockBinDir, got: %s", pathValue)
	}
}

// TestScannerConfig_ReturnsValidConfig has been removed as scanner was removed
// in favor of database-backed state. See Task 3 of Phase 2 TUI Database Integration.

func TestCleanup_RemovesAllFiles(t *testing.T) {
	// Create a nested test to capture the cleanup behavior
	var baseDir string

	t.Run("inner", func(t *testing.T) {
		env := New(t)
		baseDir = env.BaseDir

		// Create a test file
		testFile := filepath.Join(env.StagingBase, "test.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	})

	// After inner test completes, cleanup should have run
	if _, err := os.Stat(baseDir); !os.IsNotExist(err) {
		t.Errorf("BaseDir %s should have been cleaned up", baseDir)
		os.RemoveAll(baseDir) // Clean up manually
	}
}

func TestRippedMoviesDir_ReturnsCorrectPath(t *testing.T) {
	env := New(t)

	expected := filepath.Join(env.StagingBase, "1-ripped", "movies")
	if got := env.RippedMoviesDir(); got != expected {
		t.Errorf("RippedMoviesDir() = %q, want %q", got, expected)
	}
}

func TestRippedTVDir_ReturnsCorrectPath(t *testing.T) {
	env := New(t)

	expected := filepath.Join(env.StagingBase, "1-ripped", "tv")
	if got := env.RippedTVDir(); got != expected {
		t.Errorf("RippedTVDir() = %q, want %q", got, expected)
	}
}

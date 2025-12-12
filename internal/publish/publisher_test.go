package publish

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/model"
)

// Aliases for readability in tests
type MediaItem = model.MediaItem
type MediaType = model.MediaType

const (
	MediaTypeMovie = model.MediaTypeMovie
	MediaTypeTV    = model.MediaTypeTV
)

func TestNewPublisher(t *testing.T) {
	p := NewPublisher(nil, nil, PublishOptions{
		LibraryMovies: "/mnt/media/library/movies",
		LibraryTV:     "/mnt/media/library/tv",
	})

	if p.opts.LibraryMovies != "/mnt/media/library/movies" {
		t.Errorf("unexpected LibraryMovies: %s", p.opts.LibraryMovies)
	}
}

func TestPublisher_BuildFilebotArgs_Movie(t *testing.T) {
	p := NewPublisher(nil, nil, PublishOptions{
		LibraryMovies: "/mnt/media/library/movies",
	})

	args := p.buildFilebotArgs("/input/dir", "movie", 12345)

	expected := []string{
		"-rename", "/input/dir",
		"--db", "TheMovieDB",
		"--q", "12345",
		"--output", "/mnt/media/library/movies",
		"--format", "{n} ({y})/{n} ({y})",
		"-non-strict",
		"--action", "copy",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Errorf("unexpected args:\ngot:  %v\nwant: %v", args, expected)
	}
}

func TestPublisher_BuildFilebotArgs_TV(t *testing.T) {
	p := NewPublisher(nil, nil, PublishOptions{
		LibraryTV: "/mnt/media/library/tv",
	})

	args := p.buildFilebotArgs("/input/dir", "tv", 67890)

	expected := []string{
		"-rename", "/input/dir",
		"--db", "TheTVDB",
		"--q", "67890",
		"--output", "/mnt/media/library/tv",
		"--format", "{n}/Season {s.pad(2)}/{n} - {s00e00} - {t}",
		"-non-strict",
		"--action", "copy",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Errorf("unexpected args:\ngot:  %v\nwant: %v", args, expected)
	}
}

func TestPublisher_FindExtras(t *testing.T) {
	// Create temp directory structure matching transcode output (_extras/<type>/)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "_extras", "featurettes"), 0755)
	os.MkdirAll(filepath.Join(dir, "_extras", "deleted scenes"), 0755)
	os.WriteFile(filepath.Join(dir, "_extras", "featurettes", "making_of.mkv"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "_extras", "deleted scenes", "scene1.mkv"), []byte{}, 0644)

	p := NewPublisher(nil, nil, PublishOptions{})
	extras := p.findExtras(dir)

	if len(extras) != 2 {
		t.Errorf("expected 2 extras dirs, got %d", len(extras))
	}
}

func TestPublisher_RunFilebot_ParsesOutput(t *testing.T) {
	// Test that we correctly parse FileBot output to extract destination paths
	output := `Rename movies using [TheMovieDB]
[COPY] from [/input/Movie.mkv] to [/library/movies/Movie (2024)/Movie (2024).mkv]`

	dest := parseFilebotDestination(output)
	expected := "/library/movies/Movie (2024)"

	if dest != expected {
		t.Errorf("expected %q, got %q", expected, dest)
	}
}

func TestPublisher_CopyExtras(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source extras
	os.MkdirAll(filepath.Join(srcDir, "featurettes"), 0755)
	os.WriteFile(filepath.Join(srcDir, "featurettes", "making_of.mkv"), []byte("test"), 0644)

	p := NewPublisher(nil, nil, PublishOptions{})
	extras := []ExtraDir{{
		Type:  "featurettes",
		Path:  filepath.Join(srcDir, "featurettes"),
		Files: []string{filepath.Join(srcDir, "featurettes", "making_of.mkv")},
	}}

	copied, err := p.copyExtras(extras, dstDir)
	if err != nil {
		t.Errorf("copyExtras failed: %v", err)
	}
	if copied != 1 {
		t.Errorf("expected 1 copied file, got %d", copied)
	}

	// Verify file exists in destination
	destFile := filepath.Join(dstDir, "featurettes", "making_of.mkv")
	if _, err := os.Stat(destFile); err != nil {
		t.Errorf("expected file at %s, but got error: %v", destFile, err)
	}
}

func TestPublisher_VerifyFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "movie.mkv"), []byte("test"), 0644)

	p := NewPublisher(nil, nil, PublishOptions{})

	err := p.verifyFiles(dir)
	if err != nil {
		t.Errorf("verifyFiles should pass with valid directory: %v", err)
	}

	err = p.verifyFiles("/nonexistent")
	if err == nil {
		t.Error("verifyFiles should fail with nonexistent directory")
	}
}

func TestPublisher_Publish_RequiresDatabaseID(t *testing.T) {
	p := NewPublisher(nil, nil, PublishOptions{})

	item := &MediaItem{
		Type:     MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
		// No TmdbID set
	}

	result, err := p.Publish(context.Background(), item, "/input")
	if err == nil {
		t.Error("expected error for missing database ID")
	}
	if err != nil && !contains(err.Error(), "database ID") {
		t.Errorf("expected error to mention 'database ID', got: %v", err)
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

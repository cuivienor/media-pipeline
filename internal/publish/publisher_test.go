package publish

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cuivienor/media-pipeline/internal/db"
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

// mockFilebotRunner simulates FileBot for tests
type mockFilebotRunner struct {
	copyFunc func(inputDir, outputDir string) error
	destDir  string
}

func (m *mockFilebotRunner) Run(args []string) (string, error) {
	// Parse args to find input and output
	var inputDir, outputDir string
	for i, arg := range args {
		if arg == "-rename" && i+1 < len(args) {
			inputDir = args[i+1]
		}
		if arg == "--output" && i+1 < len(args) {
			outputDir = args[i+1]
		}
	}

	if inputDir == "" || outputDir == "" {
		return "", fmt.Errorf("missing input or output directory")
	}

	// Find MKV files in input
	files, _ := filepath.Glob(filepath.Join(inputDir, "*.mkv"))
	if len(files) == 0 {
		return "", fmt.Errorf("no MKV files in input")
	}

	// Create output directory (simulate FileBot naming)
	destDir := filepath.Join(outputDir, "Test Movie (2024)")
	m.destDir = destDir
	os.MkdirAll(destDir, 0755)

	// Copy files and generate FileBot-style output
	var output string
	for _, src := range files {
		dst := filepath.Join(destDir, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			return "", err
		}
		output += fmt.Sprintf("[COPY] from [%s] to [%s]\n", src, dst)
	}

	return output, nil
}

// mockTVFilebotRunner simulates FileBot for TV shows
type mockTVFilebotRunner struct {
	destDir string
}

func (m *mockTVFilebotRunner) Run(args []string) (string, error) {
	// Parse args to find input and output
	var inputDir, outputDir string
	for i, arg := range args {
		if arg == "-rename" && i+1 < len(args) {
			inputDir = args[i+1]
		}
		if arg == "--output" && i+1 < len(args) {
			outputDir = args[i+1]
		}
	}

	if inputDir == "" || outputDir == "" {
		return "", fmt.Errorf("missing input or output directory")
	}

	// Find MKV files in input
	files, _ := filepath.Glob(filepath.Join(inputDir, "*.mkv"))
	if len(files) == 0 {
		return "", fmt.Errorf("no MKV files in input")
	}

	// Create output directory (simulate FileBot naming for TV shows)
	destDir := filepath.Join(outputDir, "Test Show", "Season 01")
	m.destDir = destDir
	os.MkdirAll(destDir, 0755)

	// Copy files and generate FileBot-style output
	var output string
	for i, src := range files {
		episodeNum := i + 1
		dst := filepath.Join(destDir, fmt.Sprintf("Test Show - S01E%02d - Episode.mkv", episodeNum))
		if err := copyFile(src, dst); err != nil {
			return "", err
		}
		output += fmt.Sprintf("[COPY] from [%s] to [%s]\n", src, dst)
	}

	return output, nil
}

func TestPublisher_MovieHappyPath(t *testing.T) {
	// Create temp dirs
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	libraryDir := filepath.Join(tmpDir, "library", "movies")
	os.MkdirAll(filepath.Join(inputDir, "_main"), 0755)
	os.MkdirAll(libraryDir, 0755)

	// Create a test file
	testFile := filepath.Join(inputDir, "_main", "movie.mkv")
	os.WriteFile(testFile, []byte("test content"), 0644)

	// Setup database
	database, err := db.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory error: %v", err)
	}
	defer database.Close()
	repo := db.NewSQLiteRepository(database)

	// Create media item with TMDB ID
	tmdbID := 12345
	item := &model.MediaItem{
		Type:     model.MediaTypeMovie,
		Name:     "Test Movie",
		SafeName: "Test_Movie",
		TmdbID:   &tmdbID,
	}
	if err := repo.CreateMediaItem(context.Background(), item); err != nil {
		t.Fatalf("CreateMediaItem error: %v", err)
	}

	// Create publisher with mock
	pub := NewPublisher(repo, nil, PublishOptions{
		LibraryMovies: libraryDir,
		LibraryTV:     filepath.Join(tmpDir, "library", "tv"),
	})
	mock := &mockFilebotRunner{}
	pub.SetFilebotRunner(mock)

	// Execute publish
	result, err := pub.Publish(context.Background(), item, inputDir)
	if err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	// Verify result
	if result.MainFiles != 1 {
		t.Errorf("MainFiles = %d, want 1", result.MainFiles)
	}
	if result.LibraryPath == "" {
		t.Error("LibraryPath should not be empty")
	}

	// Verify file was copied
	destFile := filepath.Join(mock.destDir, "movie.mkv")
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		t.Errorf("destination file not created: %s", destFile)
	}
}

func TestPublisher_TVHappyPath(t *testing.T) {
	// Create temp dirs
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	libraryDir := filepath.Join(tmpDir, "library", "tv")
	os.MkdirAll(filepath.Join(inputDir, "_main"), 0755)
	os.MkdirAll(libraryDir, 0755)

	// Create 3 test episode files
	for i := 1; i <= 3; i++ {
		testFile := filepath.Join(inputDir, "_main", fmt.Sprintf("episode%d.mkv", i))
		os.WriteFile(testFile, []byte("test content"), 0644)
	}

	// Setup database
	database, err := db.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory error: %v", err)
	}
	defer database.Close()
	repo := db.NewSQLiteRepository(database)

	// Create media item with TVDB ID
	tvdbID := 67890
	item := &model.MediaItem{
		Type:     model.MediaTypeTV,
		Name:     "Test Show",
		SafeName: "Test_Show",
		TvdbID:   &tvdbID,
	}
	if err := repo.CreateMediaItem(context.Background(), item); err != nil {
		t.Fatalf("CreateMediaItem error: %v", err)
	}

	// Create publisher with mock
	pub := NewPublisher(repo, nil, PublishOptions{
		LibraryMovies: filepath.Join(tmpDir, "library", "movies"),
		LibraryTV:     libraryDir,
	})
	mock := &mockTVFilebotRunner{}
	pub.SetFilebotRunner(mock)

	// Execute publish
	result, err := pub.Publish(context.Background(), item, inputDir)
	if err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	// Verify 3 files were copied
	if result.MainFiles != 3 {
		t.Errorf("MainFiles = %d, want 3", result.MainFiles)
	}
	if result.LibraryPath == "" {
		t.Error("LibraryPath should not be empty")
	}

	// Verify output files exist
	for i := 1; i <= 3; i++ {
		destFile := filepath.Join(mock.destDir, fmt.Sprintf("Test Show - S01E%02d - Episode.mkv", i))
		if _, err := os.Stat(destFile); os.IsNotExist(err) {
			t.Errorf("destination file not created: %s", destFile)
		}
	}
}

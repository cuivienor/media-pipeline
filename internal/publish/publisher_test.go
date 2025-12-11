package publish

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
		"--output", "/mnt/media/library/movies",
		"--format", "{n} ({y})/{n} ({y})",
		"-non-strict",
		"--filter", "id == 12345",
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
		"--output", "/mnt/media/library/tv",
		"--format", "{n}/Season {s.pad(2)}/{n} - {s00e00} - {t}",
		"-non-strict",
		"--filter", "id == 67890",
		"--action", "copy",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Errorf("unexpected args:\ngot:  %v\nwant: %v", args, expected)
	}
}

func TestPublisher_FindExtras(t *testing.T) {
	// Create temp directory structure
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "featurettes"), 0755)
	os.MkdirAll(filepath.Join(dir, "deleted scenes"), 0755)
	os.WriteFile(filepath.Join(dir, "featurettes", "making_of.mkv"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "deleted scenes", "scene1.mkv"), []byte{}, 0644)

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

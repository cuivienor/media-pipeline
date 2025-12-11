package publish

import (
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

package publish

import (
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

package publish

import (
	"fmt"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/logging"
)

// PublishOptions configures the publisher
type PublishOptions struct {
	LibraryMovies string // Destination for movies
	LibraryTV     string // Destination for TV shows
}

// Publisher handles copying media to the library using FileBot
type Publisher struct {
	repo   db.Repository
	logger *logging.Logger
	opts   PublishOptions
}

// NewPublisher creates a new Publisher
func NewPublisher(repo db.Repository, logger *logging.Logger, opts PublishOptions) *Publisher {
	return &Publisher{
		repo:   repo,
		logger: logger,
		opts:   opts,
	}
}

// buildFilebotArgs constructs FileBot CLI arguments
func (p *Publisher) buildFilebotArgs(inputDir string, mediaType string, dbID int) []string {
	var db, output, format string

	if mediaType == "movie" {
		db = "TheMovieDB"
		output = p.opts.LibraryMovies
		format = "{n} ({y})/{n} ({y})"
	} else {
		db = "TheTVDB"
		output = p.opts.LibraryTV
		format = "{n}/Season {s.pad(2)}/{n} - {s00e00} - {t}"
	}

	return []string{
		"-rename", inputDir,
		"--db", db,
		"--output", output,
		"--format", format,
		"-non-strict",
		"--filter", fmt.Sprintf("id == %d", dbID),
		"--action", "copy",
	}
}

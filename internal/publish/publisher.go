package publish

import (
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

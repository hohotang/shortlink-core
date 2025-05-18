package storage

import (
	"context"
	"errors"
)

var (
	// ErrNotFound is returned when a URL is not found
	ErrNotFound = errors.New("url not found")
	// ErrInvalidURL is returned when a URL is invalid
	ErrInvalidURL = errors.New("invalid url")
)

// URLStorage defines the interface for URL storage operations
type URLStorage interface {
	// Find saves a URL and returns a short ID
	// Note: Some implementations may not support this method directly
	Find(ctx context.Context, originalURL string) (string, error)

	// StoreWithID saves a URL with a specific short ID
	// Returns an error if the operation fails
	StoreWithID(ctx context.Context, shortID string, originalURL string) error

	// Get retrieves the original URL for a short ID
	Get(ctx context.Context, shortID string) (string, error)

	// Close closes any connections
	Close() error
}

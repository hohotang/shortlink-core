package storage

import "errors"

var (
	// ErrNotFound is returned when a URL is not found
	ErrNotFound = errors.New("url not found")
	// ErrInvalidURL is returned when a URL is invalid
	ErrInvalidURL = errors.New("invalid url")
)

// URLStorage defines the interface for URL storage
type URLStorage interface {
	// Store saves a URL and returns a short ID
	// Note: Some implementations may not support this method directly
	Store(originalURL string) (string, error)

	// StoreWithID saves a URL with a specific short ID
	// Returns an error if the operation fails
	StoreWithID(shortID string, originalURL string) error

	// Get retrieves the original URL for a short ID
	Get(shortID string) (string, error)

	// Close closes any connections
	Close() error
}

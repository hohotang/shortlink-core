package storage

import (
	"sync"
)

// MemoryStorage implements URLStorage with an in-memory map
type MemoryStorage struct {
	urls        map[string]string // shortID -> originalURL
	reverseUrls map[string]string // originalURL -> shortID
	mutex       sync.RWMutex
}

// NewMemoryStorage creates a new MemoryStorage instance
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		urls:        make(map[string]string),
		reverseUrls: make(map[string]string),
	}
}

// Store implements URLStorage.Store
func (s *MemoryStorage) Store(originalURL string) (string, error) {
	if originalURL == "" {
		return "", ErrInvalidURL
	}

	// Check if the URL has already been shortened
	s.mutex.RLock()
	if shortID, exists := s.reverseUrls[originalURL]; exists {
		s.mutex.RUnlock()
		return shortID, nil
	}
	s.mutex.RUnlock()

	// If URL doesn't exist, return not found error
	// The ID will be generated at service layer
	return "", ErrNotFound
}

// StoreWithID implements URLStorage.StoreWithID
func (s *MemoryStorage) StoreWithID(shortID string, originalURL string) error {
	if originalURL == "" {
		return ErrInvalidURL
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if this URL already has a different short ID
	if existingShortID, exists := s.reverseUrls[originalURL]; exists && existingShortID != shortID {
		// We already have a different shortID for this URL, but we'll override it as requested
		// Remove the old mapping first
		delete(s.urls, existingShortID)
	}

	// Check if this shortID is already used for a different URL
	if existingURL, exists := s.urls[shortID]; exists && existingURL != originalURL {
		// Remove the old reverse mapping
		delete(s.reverseUrls, existingURL)
	}

	// Insert or update both mappings
	s.urls[shortID] = originalURL
	s.reverseUrls[originalURL] = shortID

	return nil
}

// Get implements URLStorage.Get
func (s *MemoryStorage) Get(shortID string) (string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	originalURL, exists := s.urls[shortID]
	if !exists {
		return "", ErrNotFound
	}

	return originalURL, nil
}

// Close is a no-op for memory storage
func (s *MemoryStorage) Close() error {
	// Nothing to close for in-memory storage
	return nil
}

package storage

import (
	"sync"

	"github.com/hohotang/shortlink-core/internal/logger"
	"go.uber.org/zap"
)

// MemoryStorage implements URLStorage with an in-memory map
type MemoryStorage struct {
	urls        map[string]string // shortID -> originalURL
	reverseUrls map[string]string // originalURL -> shortID
	mutex       sync.RWMutex
}

// NewMemoryStorage creates a new MemoryStorage instance
func NewMemoryStorage() *MemoryStorage {
	log := logger.L()
	log.Info("Initializing in-memory storage")

	return &MemoryStorage{
		urls:        make(map[string]string),
		reverseUrls: make(map[string]string),
	}
}

// Store implements URLStorage.Store
func (s *MemoryStorage) Store(originalURL string) (string, error) {
	log := logger.L()

	if originalURL == "" {
		return "", ErrInvalidURL
	}

	// Check if the URL has already been shortened
	s.mutex.RLock()
	if shortID, exists := s.reverseUrls[originalURL]; exists {
		s.mutex.RUnlock()
		log.Debug("Found existing short ID in memory",
			zap.String("shortID", shortID),
			zap.String("url", originalURL))
		return shortID, nil
	}
	s.mutex.RUnlock()

	// If URL doesn't exist, return not found error
	// The ID will be generated at service layer
	log.Debug("No existing short ID found in memory", zap.String("url", originalURL))
	return "", ErrNotFound
}

// StoreWithID implements URLStorage.StoreWithID
func (s *MemoryStorage) StoreWithID(shortID string, originalURL string) error {
	log := logger.L()

	if originalURL == "" {
		return ErrInvalidURL
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if this URL already has a different short ID
	if existingShortID, exists := s.reverseUrls[originalURL]; exists && existingShortID != shortID {
		// We already have a different shortID for this URL, but we'll override it as requested
		// Remove the old mapping first
		log.Info("URL already exists with different short ID, updating",
			zap.String("existingID", existingShortID),
			zap.String("newID", shortID),
			zap.String("url", originalURL))
		delete(s.urls, existingShortID)
	}

	// Check if this shortID is already used for a different URL
	if existingURL, exists := s.urls[shortID]; exists && existingURL != originalURL {
		// Remove the old reverse mapping
		log.Info("Short ID already used for different URL, updating mapping",
			zap.String("shortID", shortID),
			zap.String("oldURL", existingURL),
			zap.String("newURL", originalURL))
		delete(s.reverseUrls, existingURL)
	}

	// Insert or update both mappings
	s.urls[shortID] = originalURL
	s.reverseUrls[originalURL] = shortID

	log.Debug("Stored URL in memory",
		zap.String("shortID", shortID),
		zap.String("url", originalURL))

	return nil
}

// Get implements URLStorage.Get
func (s *MemoryStorage) Get(shortID string) (string, error) {
	log := logger.L()

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	originalURL, exists := s.urls[shortID]
	if !exists {
		log.Debug("Short ID not found in memory", zap.String("shortID", shortID))
		return "", ErrNotFound
	}

	log.Debug("Retrieved URL from memory",
		zap.String("shortID", shortID),
		zap.String("url", originalURL))
	return originalURL, nil
}

// Close is a no-op for memory storage
func (s *MemoryStorage) Close() error {
	log := logger.L()
	log.Info("Closing memory storage (no-op)")
	return nil
}

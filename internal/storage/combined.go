package storage

import (
	"fmt"
)

// CombinedStorage implements URLStorage with PostgreSQL as primary storage and Redis as cache
type CombinedStorage struct {
	postgres *PostgresStorage
	redis    *RedisStorage
}

// NewCombinedStorage creates a new CombinedStorage instance
func NewCombinedStorage(postgresURL, redisURL string, cacheTTL int) (*CombinedStorage, error) {
	// Initialize PostgreSQL storage
	postgres, err := NewPostgresStorage(postgresURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PostgreSQL storage: %w", err)
	}

	// Initialize Redis storage
	redis, err := NewRedisStorage(redisURL, cacheTTL)
	if err != nil {
		// Close PostgreSQL connection if Redis fails
		postgres.Close()
		return nil, fmt.Errorf("failed to initialize Redis storage: %w", err)
	}

	return &CombinedStorage{
		postgres: postgres,
		redis:    redis,
	}, nil
}

// Store implements URLStorage.Store
func (s *CombinedStorage) Store(originalURL string) (string, error) {
	if originalURL == "" {
		return "", ErrInvalidURL
	}

	// First check in Redis (cache) if the URL already exists
	shortID, err := s.redis.FindShortIDByURL(originalURL)
	if err == nil {
		// URL already exists in Redis cache
		return shortID, nil
	} else if err != ErrNotFound {
		// Redis error other than "not found" - non-critical, continue with PostgreSQL
		fmt.Printf("Warning: error checking Redis for existing URL: %v\n", err)
	}

	// Not found in Redis or Redis error, check PostgreSQL
	shortID, err = s.postgres.FindShortIDByURL(originalURL)
	if err == nil {
		// URL exists in PostgreSQL but not in Redis - update Redis cache
		if cacheErr := s.redis.StoreWithID(shortID, originalURL); cacheErr != nil {
			// Log error but don't fail if Redis fails
			fmt.Printf("Warning: failed to update Redis cache: %v\n", cacheErr)
		}
		return shortID, nil
	} else if err != ErrNotFound {
		// PostgreSQL error other than "not found"
		return "", err
	}

	// Not found in either storage
	return "", fmt.Errorf("combined storage requires specifying short ID, use StoreWithID instead")
}

// StoreWithID stores a URL with a specific ID in both PostgreSQL and Redis
func (s *CombinedStorage) StoreWithID(shortID string, originalURL string) error {
	if originalURL == "" {
		return ErrInvalidURL
	}

	// Store in PostgreSQL first (persistent storage)
	if err := s.postgres.StoreWithID(shortID, originalURL); err != nil {
		return fmt.Errorf("failed to store in PostgreSQL: %w", err)
	}

	// Store in Redis (cache)
	if err := s.redis.StoreWithID(shortID, originalURL); err != nil {
		// Log error but don't fail if Redis fails
		fmt.Printf("Warning: failed to store in Redis: %v\n", err)
	}

	return nil
}

// Get retrieves a URL from Redis first, falling back to PostgreSQL
func (s *CombinedStorage) Get(shortID string) (string, error) {
	// Try to get from Redis first
	originalURL, err := s.redis.Get(shortID)
	if err == nil {
		// Found in Redis
		return originalURL, nil
	}

	// Not found in Redis or Redis error, try PostgreSQL
	originalURL, err = s.postgres.Get(shortID)
	if err != nil {
		return "", err
	}

	// Found in PostgreSQL, update Redis cache
	if cacheErr := s.redis.StoreWithID(shortID, originalURL); cacheErr != nil {
		// Log error but don't fail if Redis fails
		fmt.Printf("Warning: failed to update Redis cache: %v\n", cacheErr)
	}

	return originalURL, nil
}

// Close closes both PostgreSQL and Redis connections
func (s *CombinedStorage) Close() error {
	pgErr := s.postgres.Close()
	redisErr := s.redis.Close()

	if pgErr != nil {
		return pgErr
	}
	return redisErr
}

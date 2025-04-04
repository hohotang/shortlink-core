package storage

import (
	"fmt"

	"github.com/hohotang/shortlink-core/internal/config"
)

// CombinedStorage combines PostgreSQL and Redis for efficient storage
// It uses Redis as a cache and PostgreSQL as the primary storage
type CombinedStorage struct {
	postgres *PostgresStorage
	redis    *RedisStorage
}

// NewCombinedStorage creates a combined Redis+PostgreSQL storage
func NewCombinedStorage(redisURL string, cacheTTL int, cfg *config.Config) (*CombinedStorage, error) {
	redis, err := NewRedisStorage(redisURL, cacheTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis storage: %w", err)
	}

	postgres, err := NewPostgresStorage(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PostgreSQL storage: %w", err)
	}

	return &CombinedStorage{
		redis:    redis,
		postgres: postgres,
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
	} else {
		// Not found in either storage
		return "", ErrNotFound
	}
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

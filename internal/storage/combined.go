package storage

import (
	"fmt"

	"github.com/hohotang/shortlink-core/internal/config"
	"github.com/hohotang/shortlink-core/internal/logger"
	"go.uber.org/zap"
)

// CombinedStorage combines PostgreSQL and Redis for efficient storage
// It uses Redis as a cache and PostgreSQL as the primary storage
type CombinedStorage struct {
	postgres *PostgresStorage
	redis    *RedisStorage
}

// NewCombinedStorage creates a combined Redis+PostgreSQL storage
func NewCombinedStorage(redisURL string, cacheTTL int, cfg *config.Config) (*CombinedStorage, error) {
	log := logger.L()

	redis, err := NewRedisStorage(redisURL, cacheTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis storage: %w", err)
	}

	postgres, err := NewPostgresStorage(cfg)
	if err != nil {
		// Close Redis connection if PostgreSQL fails
		if closeErr := redis.Close(); closeErr != nil {
			log.Warn("Failed to close Redis connection", zap.Error(closeErr))
		}
		return nil, fmt.Errorf("failed to initialize PostgreSQL storage: %w", err)
	}

	return &CombinedStorage{
		redis:    redis,
		postgres: postgres,
	}, nil
}

// Store implements URLStorage.Store
func (s *CombinedStorage) Find(originalURL string) (string, error) {
	log := logger.L()

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
		log.Warn("Error checking Redis for existing URL", zap.Error(err))
	}

	// Not found in Redis or Redis error, check PostgreSQL
	shortID, err = s.postgres.FindShortIDByURL(originalURL)
	if err == nil {
		// URL exists in PostgreSQL but not in Redis - update Redis cache
		if cacheErr := s.redis.StoreWithID(shortID, originalURL); cacheErr != nil {
			// Log error but don't fail if Redis fails
			log.Warn("Failed to update Redis cache", zap.Error(cacheErr))
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
	log := logger.L()

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
		log.Warn("Failed to store in Redis", zap.Error(err))
	}

	return nil
}

// Get retrieves a URL from Redis first, falling back to PostgreSQL
func (s *CombinedStorage) Get(shortID string) (string, error) {
	log := logger.L()

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
		log.Warn("Failed to update Redis cache", zap.Error(cacheErr))
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

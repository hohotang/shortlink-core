package storage

import (
	"context"
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
	logger   *zap.Logger
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
		logger:   log,
	}, nil
}

// Find implements URLStorage.Find
func (s *CombinedStorage) Find(ctx context.Context, originalURL string) (string, error) {
	if originalURL == "" {
		return "", ErrInvalidURL
	}

	// Try Redis first
	shortID, err := s.redis.Find(ctx, originalURL)
	if err == nil {
		return shortID, nil
	} else if err != ErrNotFound {
		// Redis error other than "not found" - non-critical, continue with PostgreSQL
		s.logger.Warn("Error checking Redis for existing URL", zap.Error(err))
	}

	// Try PostgreSQL
	shortID, err = s.postgres.Find(ctx, originalURL)
	if err != nil {
		return "", err
	}

	// Found in PostgreSQL, update Redis cache
	if cacheErr := s.redis.StoreWithID(ctx, shortID, originalURL); cacheErr != nil {
		// Log error but don't fail if Redis fails
		s.logger.Warn("Failed to update Redis cache", zap.Error(cacheErr))
	}
	return shortID, nil
}

// StoreWithID implements URLStorage.StoreWithID
func (s *CombinedStorage) StoreWithID(ctx context.Context, shortID string, originalURL string) error {
	if originalURL == "" {
		return ErrInvalidURL
	}

	// Store in PostgreSQL
	if err := s.postgres.StoreWithID(ctx, shortID, originalURL); err != nil {
		return err
	}

	// Try to store in Redis
	if err := s.redis.StoreWithID(ctx, shortID, originalURL); err != nil {
		// Log error but don't fail if Redis fails
		s.logger.Warn("Failed to store in Redis", zap.Error(err))
	}

	return nil
}

// Get implements URLStorage.Get
func (s *CombinedStorage) Get(ctx context.Context, shortID string) (string, error) {
	// Try cache first
	url, err := s.redis.Get(ctx, shortID)
	if err == nil {
		return url, nil
	}

	// Not found in Redis or Redis error, try PostgreSQL
	url, err = s.postgres.Get(ctx, shortID)
	if err != nil {
		return "", err
	}

	// Found in PostgreSQL, update Redis cache
	if cacheErr := s.redis.StoreWithID(ctx, shortID, url); cacheErr != nil {
		// Log error but don't fail if Redis fails
		s.logger.Warn("Failed to update Redis cache", zap.Error(cacheErr))
	}

	return url, nil
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

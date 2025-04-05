package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hohotang/shortlink-core/internal/logger"
	"github.com/hohotang/shortlink-core/internal/models"
	"go.uber.org/zap"
)

// RedisStorage implements URLStorage with Redis
type RedisStorage struct {
	client *redis.Client
	ttl    time.Duration
	ctx    context.Context
}

// NewRedisStorage creates a new RedisStorage instance
func NewRedisStorage(redisURL string, ttl int) (*RedisStorage, error) {
	log := logger.L()

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opts)
	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Use default TTL if not specified
	if ttl <= 0 {
		ttl = 3600 // 1 hour
		log.Info("Using default TTL for Redis cache", zap.Int("ttl", ttl))
	}

	log.Info("Redis connection established",
		zap.String("address", opts.Addr),
		zap.Int("database", opts.DB),
		zap.Int("ttl", ttl))

	return &RedisStorage{
		client: client,
		ttl:    time.Duration(ttl) * time.Second,
		ctx:    ctx,
	}, nil
}

// FindShortIDByURL checks if a URL already has a short ID in Redis
func (s *RedisStorage) FindShortIDByURL(originalURL string) (string, error) {
	log := logger.L()

	shortID, err := s.client.HGet(s.ctx, models.ReverseURLsKey, originalURL).Result()
	if err != nil {
		if err == redis.Nil {
			log.Debug("No existing short ID found in Redis", zap.String("url", originalURL))
			return "", ErrNotFound
		}
		log.Error("Failed to query Redis for existing URL", zap.Error(err))
		return "", fmt.Errorf("failed to query for existing URL: %w", err)
	}

	// Check if the shortID actually exists (in case of inconsistency)
	exists, err := s.client.Exists(s.ctx, models.ShortIDKeyPrefix+shortID).Result()
	if err != nil {
		log.Error("Failed to check if short ID exists in Redis", zap.Error(err))
		return "", fmt.Errorf("failed to check if short ID exists: %w", err)
	}

	if exists == 0 {
		// The reverse mapping exists but the actual key doesn't
		// Let's clean up the inconsistency
		log.Warn("Inconsistent Redis state: cleaning up stale reverse mapping",
			zap.String("shortID", shortID),
			zap.String("url", originalURL))
		s.client.HDel(s.ctx, models.ReverseURLsKey, originalURL)
		return "", ErrNotFound
	}

	log.Debug("Found existing short ID in Redis",
		zap.String("shortID", shortID),
		zap.String("url", originalURL))
	return shortID, nil
}

func (s *RedisStorage) Find(originalURL string) (string, error) {
	// This method simply calls FindShortIDByURL to check if the URL already exists
	return s.FindShortIDByURL(originalURL)
}

// StoreWithID implements URLStorage.StoreWithID
func (s *RedisStorage) StoreWithID(shortID string, originalURL string) error {
	log := logger.L()

	if originalURL == "" {
		return ErrInvalidURL
	}

	// First check if this URL already exists in Redis with a different short ID
	existingShortID, err := s.FindShortIDByURL(originalURL)
	if err == nil && existingShortID != shortID {
		// URL exists with a different short ID - need to update both mappings
		log.Info("URL already exists in Redis with different short ID, updating",
			zap.String("existingShortID", existingShortID),
			zap.String("newShortID", shortID),
			zap.String("url", originalURL))

		// Use a pipeline for atomic updates
		pipe := s.client.Pipeline()

		// Remove old short ID entry
		pipe.Del(s.ctx, models.ShortIDKeyPrefix+existingShortID)

		// Add bidirectional mapping for new short ID
		pipe.Set(s.ctx, models.ShortIDKeyPrefix+shortID, originalURL, s.ttl)
		pipe.HSet(s.ctx, models.ReverseURLsKey, originalURL, shortID)

		// Execute pipeline
		_, err = pipe.Exec(s.ctx)
		if err != nil {
			log.Error("Failed to update Redis mappings", zap.Error(err))
			return fmt.Errorf("failed to update Redis mappings: %w", err)
		}

		return nil
	} else if err != nil && err != ErrNotFound {
		// Real error occurred, not just "not found"
		return err
	}

	// URL doesn't exist or already has the same short ID
	// In either case, just store the bidirectional mapping
	pipe := s.client.Pipeline()
	pipe.Set(s.ctx, models.ShortIDKeyPrefix+shortID, originalURL, s.ttl)
	pipe.HSet(s.ctx, models.ReverseURLsKey, originalURL, shortID)

	_, err = pipe.Exec(s.ctx)
	if err != nil {
		log.Error("Failed to store URL in Redis", zap.Error(err))
		return fmt.Errorf("failed to store URL in Redis: %w", err)
	}

	log.Debug("Stored URL in Redis",
		zap.String("shortID", shortID),
		zap.String("url", originalURL))
	return nil
}

// Get implements URLStorage.Get
func (s *RedisStorage) Get(shortID string) (string, error) {
	log := logger.L()

	originalURL, err := s.client.Get(s.ctx, models.ShortIDKeyPrefix+shortID).Result()
	if err != nil {
		if err == redis.Nil {
			log.Debug("Short ID not found in Redis", zap.String("shortID", shortID))
			return "", ErrNotFound
		}
		log.Error("Failed to get URL from Redis", zap.Error(err), zap.String("shortID", shortID))
		return "", fmt.Errorf("failed to get URL from Redis: %w", err)
	}

	// Refresh the TTL
	if err := s.client.Expire(s.ctx, models.ShortIDKeyPrefix+shortID, s.ttl).Err(); err != nil {
		// Non-fatal error, just log it
		log.Warn("Failed to refresh TTL in Redis",
			zap.Error(err),
			zap.String("shortID", shortID))
	}

	log.Debug("Retrieved URL from Redis",
		zap.String("shortID", shortID),
		zap.String("url", originalURL))

	return originalURL, nil
}

// Close implements URLStorage.Close
func (s *RedisStorage) Close() error {
	log := logger.L()
	log.Info("Closing Redis connection")
	return s.client.Close()
}

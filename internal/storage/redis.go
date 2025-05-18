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

// Find implements URLStorage.Find
func (s *RedisStorage) Find(ctx context.Context, originalURL string) (string, error) {
	if originalURL == "" {
		return "", ErrInvalidURL
	}

	shortID, err := s.client.Get(ctx, originalURL).Result()
	if err == redis.Nil {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to get URL from Redis: %w", err)
	}
	return shortID, nil
}

// StoreWithID implements URLStorage.StoreWithID
func (s *RedisStorage) StoreWithID(ctx context.Context, shortID string, originalURL string) error {
	if originalURL == "" {
		return ErrInvalidURL
	}

	err := s.client.Set(ctx, shortID, originalURL, s.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to store URL in Redis: %w", err)
	}
	return nil
}

// Get implements URLStorage.Get
func (s *RedisStorage) Get(ctx context.Context, shortID string) (string, error) {
	originalURL, err := s.client.Get(ctx, shortID).Result()
	if err == redis.Nil {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to get URL from Redis: %w", err)
	}
	return originalURL, nil
}

// Close implements URLStorage.Close
func (s *RedisStorage) Close() error {
	log := logger.L()
	log.Info("Closing Redis connection")
	return s.client.Close()
}

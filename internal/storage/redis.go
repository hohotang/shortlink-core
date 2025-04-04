package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hohotang/shortlink-core/internal/models"
)

// RedisStorage implements URLStorage with Redis
type RedisStorage struct {
	client *redis.Client
	ttl    time.Duration
	ctx    context.Context
}

// NewRedisStorage creates a new RedisStorage instance
func NewRedisStorage(redisURL string, ttl int) (*RedisStorage, error) {
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
	}

	return &RedisStorage{
		client: client,
		ttl:    time.Duration(ttl) * time.Second,
		ctx:    ctx,
	}, nil
}

// FindShortIDByURL checks if a URL already has a short ID in Redis
func (s *RedisStorage) FindShortIDByURL(originalURL string) (string, error) {
	shortID, err := s.client.HGet(s.ctx, models.ReverseURLsKey, originalURL).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to query for existing URL: %w", err)
	}

	// Check if the shortID actually exists (in case of inconsistency)
	exists, err := s.client.Exists(s.ctx, models.ShortIDKeyPrefix+shortID).Result()
	if err != nil {
		return "", fmt.Errorf("failed to check if short ID exists: %w", err)
	}

	if exists == 0 {
		// The reverse mapping exists but the actual key doesn't
		// Let's clean up the inconsistency
		s.client.HDel(s.ctx, models.ReverseURLsKey, originalURL)
		return "", ErrNotFound
	}

	return shortID, nil
}

// Store implements URLStorage.Store
func (s *RedisStorage) Store(originalURL string) (string, error) {
	if originalURL == "" {
		return "", ErrInvalidURL
	}

	// First check if this URL already exists
	shortID, err := s.FindShortIDByURL(originalURL)
	if err == nil {
		// URL already exists, return the existing short ID
		// Reset TTL on access
		s.client.Expire(s.ctx, models.ShortIDKeyPrefix+shortID, s.ttl)
		return shortID, nil
	} else if err != ErrNotFound {
		// An error other than "not found" occurred
		return "", err
	} else {
		// URL doesn't exist yet, but we can't generate a new ID here
		return "", ErrNotFound
	}
}

// StoreWithID stores a URL with a specific ID
func (s *RedisStorage) StoreWithID(shortID string, originalURL string) error {
	if originalURL == "" {
		return ErrInvalidURL
	}

	// Store in both directions
	// 1. Key: shortID, Value: originalURL
	if err := s.client.Set(s.ctx, models.ShortIDKeyPrefix+shortID, originalURL, s.ttl).Err(); err != nil {
		return fmt.Errorf("failed to store shortID->URL mapping: %w", err)
	}

	// 2. URL to ID mapping in a hash
	if err := s.client.HSet(s.ctx, models.ReverseURLsKey, originalURL, shortID).Err(); err != nil {
		return fmt.Errorf("failed to store URL->shortID mapping: %w", err)
	}

	return nil
}

// Get implements URLStorage.Get
func (s *RedisStorage) Get(shortID string) (string, error) {
	originalURL, err := s.client.Get(s.ctx, models.ShortIDKeyPrefix+shortID).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to get URL: %w", err)
	}

	// refresh TTL when accessed
	s.client.Expire(s.ctx, models.ShortIDKeyPrefix+shortID, s.ttl)

	return originalURL, nil
}

// Close closes the Redis client
func (s *RedisStorage) Close() error {
	return s.client.Close()
}

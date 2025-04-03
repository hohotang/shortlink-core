package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisStorage implements URLStorage with Redis
type RedisStorage struct {
	client *redis.Client
	ttl    time.Duration
	ctx    context.Context
}

// Constants for Redis keys
const (
	// URL to short ID mapping hash
	reverseURLsKey = "reverse_urls"
)

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
	shortID, err := s.client.HGet(s.ctx, reverseURLsKey, originalURL).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to query for existing URL: %w", err)
	}

	// Check if the shortID actually exists (in case of inconsistency)
	exists, err := s.client.Exists(s.ctx, shortID).Result()
	if err != nil {
		return "", fmt.Errorf("failed to check if short ID exists: %w", err)
	}

	if exists == 0 {
		// The reverse mapping exists but the actual key doesn't
		// Let's clean up the inconsistency
		s.client.HDel(s.ctx, reverseURLsKey, originalURL)
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
		s.client.Expire(s.ctx, shortID, s.ttl)
		return shortID, nil
	} else if err != ErrNotFound {
		// An error other than "not found" occurred
		return "", err
	}

	// URL doesn't exist yet, but we can't generate a new ID here
	return "", fmt.Errorf("redis storage requires specifying short ID, use StoreWithID instead")
}

// StoreWithID stores a URL with a specific ID
func (s *RedisStorage) StoreWithID(shortID string, originalURL string) error {
	if originalURL == "" {
		return ErrInvalidURL
	}

	// First check if this URL already exists with a different short ID
	existingShortID, err := s.FindShortIDByURL(originalURL)
	if err == nil && existingShortID != shortID {
		// The URL exists with a different short ID
		// Delete the old mapping
		s.client.Del(s.ctx, existingShortID)
		s.client.HDel(s.ctx, reverseURLsKey, originalURL)
	} else if err != nil && err != ErrNotFound {
		// An error other than "not found" occurred
		return err
	}

	// Check if this short ID is used for a different URL
	existingURL, err := s.client.Get(s.ctx, shortID).Result()
	if err == nil && existingURL != originalURL {
		// The short ID exists but points to a different URL
		// Find and remove any reverse mapping for the existing URL
		s.client.HDel(s.ctx, reverseURLsKey, existingURL)
	} else if err != nil && err != redis.Nil {
		// An error other than "not found" occurred
		return fmt.Errorf("failed to check if short ID exists: %w", err)
	}

	// Store URL in Redis with TTL
	err = s.client.Set(s.ctx, shortID, originalURL, s.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to store URL in Redis: %w", err)
	}

	// Store reverse mapping (URL to shortID) in a hash
	// This hash doesn't expire, but we clean up entries if they become invalid
	err = s.client.HSet(s.ctx, reverseURLsKey, originalURL, shortID).Err()
	if err != nil {
		return fmt.Errorf("failed to store reverse mapping in Redis: %w", err)
	}

	return nil
}

// Get implements URLStorage.Get
func (s *RedisStorage) Get(shortID string) (string, error) {
	// Get URL from Redis
	originalURL, err := s.client.Get(s.ctx, shortID).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to get URL from Redis: %w", err)
	}

	// Reset TTL on access
	s.client.Expire(s.ctx, shortID, s.ttl)

	return originalURL, nil
}

// Close closes the Redis client
func (s *RedisStorage) Close() error {
	return s.client.Close()
}

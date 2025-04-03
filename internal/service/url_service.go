package service

import (
	"context"
	"fmt"
	"log"
	"net/url"

	"github.com/hohotang/shortlink-core/internal/config"
	"github.com/hohotang/shortlink-core/internal/storage"
	"github.com/hohotang/shortlink-core/internal/utils"
	"github.com/hohotang/shortlink-core/proto"
)

// URLService implements the gRPC URLService interface
type URLService struct {
	proto.UnimplementedURLServiceServer
	storage   storage.URLStorage
	baseURL   string
	generator *utils.SnowflakeGenerator
}

// NewURLService creates a new URLService instance
func NewURLService(cfg *config.Config) *URLService {
	// Initialize snowflake generator
	generator, err := utils.NewSnowflakeGenerator(cfg.Snowflake.MachineID)
	if err != nil {
		log.Printf("Warning: Failed to initialize snowflake generator: %v. Using default.", err)
		generator, _ = utils.NewSnowflakeGenerator(1) // Default to 1
	}

	// Create storage based on configuration
	var store storage.URLStorage
	switch cfg.Storage.Type {
	case "redis":
		store, err = storage.NewRedisStorage(cfg.Storage.RedisURL, cfg.Storage.CacheTTL)
		if err != nil {
			log.Printf("Warning: Failed to initialize Redis storage: %v. Falling back to memory.", err)
			store = storage.NewMemoryStorage()
		}
	case "postgres":
		store, err = storage.NewPostgresStorage(cfg.Storage.PostgresURL)
		if err != nil {
			log.Printf("Warning: Failed to initialize PostgreSQL storage: %v. Falling back to memory.", err)
			store = storage.NewMemoryStorage()
		}
	case "both":
		store, err = storage.NewCombinedStorage(cfg.Storage.PostgresURL, cfg.Storage.RedisURL, cfg.Storage.CacheTTL)
		if err != nil {
			log.Printf("Warning: Failed to initialize combined storage: %v. Falling back to memory.", err)
			store = storage.NewMemoryStorage()
		}
	default:
		store = storage.NewMemoryStorage()
	}

	baseURL := cfg.Server.BaseURL // 使用配置中的 base_url

	return &URLService{
		storage:   store,
		baseURL:   baseURL,
		generator: generator,
	}
}

// ShortenURL implements the ShortenURL RPC method
func (s *URLService) ShortenURL(ctx context.Context, req *proto.ShortenURLRequest) (*proto.ShortenURLResponse, error) {
	// Validate URL
	originalURL := req.OriginalUrl
	if _, err := url.ParseRequestURI(originalURL); err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Generate a new snowflake ID
	snowflakeID, err := s.generator.NextID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ID: %w", err)
	}

	// Encode to base62
	shortID := s.generator.Encode(snowflakeID)

	// Store URL with the generated ID
	var storeErr error

	// If storage supports StoreWithID, use it
	if storageWithID, ok := s.storage.(interface {
		StoreWithID(shortID string, originalURL string) error
	}); ok {
		storeErr = storageWithID.StoreWithID(shortID, originalURL)
	} else {
		// Fall back to standard Store method, which might not be ideal
		var genID string
		genID, storeErr = s.storage.Store(originalURL)
		if genID != shortID {
			log.Printf("Warning: Generated ID (%s) differs from snowflake ID (%s)", genID, shortID)
		}
	}

	if storeErr != nil {
		return nil, fmt.Errorf("failed to store URL: %w", storeErr)
	}

	// Build full short URL
	shortURL := s.baseURL + shortID

	return &proto.ShortenURLResponse{
		ShortId:  shortID,
		ShortUrl: shortURL,
	}, nil
}

// ExpandURL implements the ExpandURL RPC method
func (s *URLService) ExpandURL(ctx context.Context, req *proto.ExpandURLRequest) (*proto.ExpandURLResponse, error) {
	// Get original URL from storage
	originalURL, err := s.storage.Get(req.ShortId)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, fmt.Errorf("short URL not found: %s", req.ShortId)
		}
		return nil, fmt.Errorf("failed to retrieve URL: %w", err)
	}

	return &proto.ExpandURLResponse{
		OriginalUrl: originalURL,
	}, nil
}

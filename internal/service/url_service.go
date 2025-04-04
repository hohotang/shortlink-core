package service

import (
	"context"
	"fmt"
	"log"
	"net/url"

	"github.com/hohotang/shortlink-core/internal/config"
	"github.com/hohotang/shortlink-core/internal/models"
	"github.com/hohotang/shortlink-core/internal/storage"
	"github.com/hohotang/shortlink-core/internal/utils"
	"github.com/hohotang/shortlink-core/proto"
)

// URLService implements the gRPC URLService interface
type URLService struct {
	proto.UnimplementedURLServiceServer
	storage   storage.URLStorage
	baseURL   string
	generator utils.IDGenerator
}

// NewURLService creates a new URLService instance
func NewURLService(cfg *config.Config) (*URLService, error) {
	var store storage.URLStorage
	var err error
	var generator utils.IDGenerator

	// Create a snowflake generator for ID generation
	generator, err = utils.NewSnowflakeGenerator(cfg.Snowflake.MachineID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Snowflake generator: %w", err)
	}

	// Initialize the storage based on configuration
	switch cfg.Storage.Type {
	case models.Memory:
		store = storage.NewMemoryStorage()

	case models.Redis:
		store, err = storage.NewRedisStorage(cfg.Storage.RedisURL, cfg.Storage.CacheTTL)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Redis storage: %w", err)
		}

	case models.Postgres:
		store, err = storage.NewPostgresStorage(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize PostgreSQL storage: %w", err)
		}

	case models.Combined:
		store, err = storage.NewCombinedStorage(cfg.Storage.RedisURL, cfg.Storage.CacheTTL, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize combined storage: %w", err)
		}

	default:
		return nil, fmt.Errorf("unknown storage type: %s", cfg.Storage.Type)
	}

	// Default base URL from config
	baseURL := cfg.Server.BaseURL

	return &URLService{
		storage:   store,
		baseURL:   baseURL,
		generator: generator, // The generator is now only in the service layer
	}, nil
}

// ShortenURL implements the ShortenURL RPC method
func (s *URLService) ShortenURL(ctx context.Context, req *proto.ShortenURLRequest) (*proto.ShortenURLResponse, error) {
	originalURL := req.OriginalUrl

	if err := s.validateURL(originalURL); err != nil {
		return nil, err
	}

	shortID, err := s.findExistingShortID(originalURL)
	if err != nil && err != storage.ErrNotFound {
		return nil, err
	}

	if err == storage.ErrNotFound {
		shortID, err = s.generateAndStoreShortID(originalURL)
		if err != nil {
			return nil, err
		}
	}

	return s.buildResponse(shortID), nil
}

// validateURL checks if the URL is valid
func (s *URLService) validateURL(originalURL string) error {
	if _, err := url.ParseRequestURI(originalURL); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	return nil
}

// findExistingShortID checks if a short link already exists for the URL
func (s *URLService) findExistingShortID(originalURL string) (string, error) {
	shortID, err := s.storage.Store(originalURL)
	if err == nil {
		// Found existing short ID, log and return
		log.Printf("Found existing short ID %s for URL %s", shortID, originalURL)
		return shortID, nil
	}
	return "", err
}

// generateAndStoreShortID creates a new short ID and stores it
func (s *URLService) generateAndStoreShortID(originalURL string) (string, error) {
	// Use the generator's method to generate short ID
	shortID := s.generator.GenerateShortID()

	// Store the URL and generated short ID
	if err := s.storage.StoreWithID(shortID, originalURL); err != nil {
		return "", fmt.Errorf("failed to store URL: %w", err)
	}

	return shortID, nil
}

// buildResponse creates the response object
func (s *URLService) buildResponse(shortID string) *proto.ShortenURLResponse {
	return &proto.ShortenURLResponse{
		ShortId:  shortID,
		ShortUrl: s.baseURL + shortID,
	}
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

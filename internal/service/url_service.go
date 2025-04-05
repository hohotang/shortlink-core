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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Tracer 名稱
const tracerName = "github.com/hohotang/shortlink-core/internal/service"

// URLService implements the gRPC URLService interface
type URLService struct {
	proto.UnimplementedURLServiceServer
	storage   storage.URLStorage
	baseURL   string
	generator utils.IDGenerator
	tracer    trace.Tracer
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

	// Initialize tracer
	tracer := otel.Tracer(tracerName)

	return &URLService{
		storage:   store,
		baseURL:   baseURL,
		generator: generator,
		tracer:    tracer,
	}, nil
}

// ShortenURL implements the ShortenURL RPC method
func (s *URLService) ShortenURL(ctx context.Context, req *proto.ShortenURLRequest) (*proto.ShortenURLResponse, error) {
	// Check if incoming context already has trace information
	parentSpanContext := trace.SpanContextFromContext(ctx)
	if parentSpanContext.IsValid() {
		log.Printf("INCOMING context already has trace ID: %s, remote: %t",
			parentSpanContext.TraceID().String(),
			parentSpanContext.IsRemote())
	} else {
		log.Printf("INCOMING context has NO valid trace information")
	}

	// Create span with the correct trace option to ensure it links to the parent span
	ctx, span := s.tracer.Start(ctx, "URLService.ShortenURL",
		trace.WithAttributes(attribute.String("original_url", req.OriginalUrl)))
	defer span.End()

	// Show current trace ID for debugging
	spanCtx := span.SpanContext()
	if spanCtx.HasTraceID() {
		log.Printf("Current trace ID: %s, parent: %t", spanCtx.TraceID().String(), spanCtx.IsRemote())
	}

	originalURL := req.OriginalUrl

	// Validate URL and record to span
	if err := s.validateURL(ctx, originalURL); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Find existing shortID
	shortID, err := s.findExistingShortID(ctx, originalURL)
	if err != nil && err != storage.ErrNotFound {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// If needed, generate new shortID
	if err == storage.ErrNotFound {
		shortID, err = s.generateAndStoreShortID(ctx, originalURL)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		span.SetAttributes(attribute.Bool("new_short_id_generated", true))
	} else {
		span.SetAttributes(attribute.Bool("existing_short_id_used", true))
	}

	// Build response
	response := s.buildResponse(shortID)
	span.SetAttributes(attribute.String("short_id", response.ShortId))
	return response, nil
}

// validateURL checks if the URL is valid
func (s *URLService) validateURL(ctx context.Context, originalURL string) error {
	_, span := s.tracer.Start(ctx, "URLService.validateURL")
	defer span.End()

	if _, err := url.ParseRequestURI(originalURL); err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid URL: %w", err)
	}
	return nil
}

// findExistingShortID checks if a short link already exists for the URL
func (s *URLService) findExistingShortID(ctx context.Context, originalURL string) (string, error) {
	_, span := s.tracer.Start(ctx, "URLService.findExistingShortID")
	defer span.End()

	shortID, err := s.storage.Store(originalURL)
	if err == nil {
		// Found existing short ID, log and return
		log.Printf("Found existing short ID %s for URL %s", shortID, originalURL)
		span.SetAttributes(attribute.String("existing_short_id", shortID))
		return shortID, nil
	}

	if err != storage.ErrNotFound {
		span.RecordError(err)
	}
	return "", err
}

// generateAndStoreShortID creates a new short ID and stores it
func (s *URLService) generateAndStoreShortID(ctx context.Context, originalURL string) (string, error) {
	_, span := s.tracer.Start(ctx, "URLService.generateAndStoreShortID")
	defer span.End()

	// Use the generator's method to generate short ID
	shortID := s.generator.GenerateShortID()
	span.SetAttributes(attribute.String("generated_short_id", shortID))

	// Store the URL and generated short ID
	if err := s.storage.StoreWithID(shortID, originalURL); err != nil {
		span.RecordError(err)
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
	_, span := s.tracer.Start(ctx, "URLService.ExpandURL",
		trace.WithAttributes(attribute.String("short_id", req.ShortId)))
	defer span.End()

	// Show current trace ID for debugging
	spanCtx := span.SpanContext()
	if spanCtx.HasTraceID() {
		log.Printf("ExpandURL trace ID: %s, parent: %t", spanCtx.TraceID().String(), spanCtx.IsRemote())
	}

	// Get original URL from storage
	originalURL, err := s.storage.Get(req.ShortId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		if err == storage.ErrNotFound {
			return nil, fmt.Errorf("short URL not found: %s", req.ShortId)
		}
		return nil, fmt.Errorf("failed to retrieve URL: %w", err)
	}

	span.SetAttributes(attribute.String("original_url", originalURL))
	return &proto.ExpandURLResponse{
		OriginalUrl: originalURL,
	}, nil
}

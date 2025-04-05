package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hohotang/shortlink-core/internal/config"
	"github.com/hohotang/shortlink-core/internal/logger"
	"github.com/hohotang/shortlink-core/internal/otel"
	"github.com/hohotang/shortlink-core/internal/service"
	"github.com/hohotang/shortlink-core/proto"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		// Use standard log here since logger is not initialized yet
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger.Init("shortlink-core", cfg.Telemetry.Environment)
	defer logger.Sync()

	log := logger.L()

	// Initialize OpenTelemetry if enabled
	var shutdown func(context.Context) error
	if cfg.Telemetry.Enabled {
		log.Info("Initializing OpenTelemetry",
			zap.String("endpoint", cfg.Telemetry.OTLPEndpoint))

		shutdown, err = otel.InitTracer(otel.Config{
			OTLPEndpoint:   cfg.Telemetry.OTLPEndpoint,
			ServiceName:    cfg.Telemetry.ServiceName,
			ServiceVersion: "1.0.0", // TODO: Make this configurable
			Environment:    cfg.Telemetry.Environment,
		})
		if err != nil {
			log.Warn("Failed to initialize OpenTelemetry", zap.Error(err))
		} else {
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := shutdown(ctx); err != nil {
					log.Warn("Error shutting down OpenTelemetry", zap.Error(err))
				}
			}()
		}
	}

	// Initialize server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		log.Fatal("Failed to listen", zap.Error(err))
	}

	// Create gRPC server with OpenTelemetry integration if enabled
	var grpcServer *grpc.Server
	if cfg.Telemetry.Enabled {
		// With OpenTelemetry - configure to propagate trace context properly
		grpcServer = grpc.NewServer(
			grpc.StatsHandler(otelgrpc.NewServerHandler(
				otelgrpc.WithPropagators(propagation.NewCompositeTextMapPropagator(
					propagation.TraceContext{},
					propagation.Baggage{},
				)),
			)),
		)
		log.Info("gRPC server created with OpenTelemetry integration")
	} else {
		// Without OpenTelemetry
		grpcServer = grpc.NewServer()
		log.Info("gRPC server created without OpenTelemetry integration")
	}

	// Create URL service
	urlService, err := service.NewURLService(cfg)
	if err != nil {
		log.Fatal("Failed to create URL service", zap.Error(err))
	}

	// Register service
	proto.RegisterURLServiceServer(grpcServer, urlService)

	// Start server
	log.Info("Starting gRPC server", zap.Int("port", cfg.Server.Port))
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("Failed to serve", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")
	grpcServer.GracefulStop()
	log.Info("Server stopped")
}

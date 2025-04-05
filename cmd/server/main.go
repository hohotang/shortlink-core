package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hohotang/shortlink-core/internal/config"
	"github.com/hohotang/shortlink-core/internal/otel"
	"github.com/hohotang/shortlink-core/internal/service"
	"github.com/hohotang/shortlink-core/proto"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize OpenTelemetry if enabled
	var shutdown func(context.Context) error
	if cfg.Telemetry.Enabled {
		log.Printf("Initializing OpenTelemetry with endpoint: %s", cfg.Telemetry.OTLPEndpoint)
		shutdown, err = otel.InitTracer(otel.Config{
			OTLPEndpoint:   cfg.Telemetry.OTLPEndpoint,
			ServiceName:    cfg.Telemetry.ServiceName,
			ServiceVersion: "1.0.0", // TODO: Make this configurable
			Environment:    cfg.Telemetry.Environment,
		})
		if err != nil {
			log.Printf("Failed to initialize OpenTelemetry: %v", err)
		} else {
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := shutdown(ctx); err != nil {
					log.Printf("Error shutting down OpenTelemetry: %v", err)
				}
			}()
		}
	}

	// Initialize server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
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
		log.Println("gRPC server created with OpenTelemetry integration")
	} else {
		// Without OpenTelemetry
		grpcServer = grpc.NewServer()
		log.Println("gRPC server created without OpenTelemetry integration")
	}

	// Create URL service
	urlService, err := service.NewURLService(cfg)
	if err != nil {
		log.Fatalf("Failed to create URL service: %v", err)
	}

	// Register service
	proto.RegisterURLServiceServer(grpcServer, urlService)

	// Start server
	log.Printf("Starting gRPC server on port %d", cfg.Server.Port)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	grpcServer.GracefulStop()
	log.Println("Server stopped")
}

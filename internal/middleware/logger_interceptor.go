// internal/middleware/logger_interceptor.go

package middleware

import (
	"context"
	"time"

	"github.com/hohotang/shortlink-core/internal/logger"
	"github.com/hohotang/shortlink-core/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// LoggerInterceptor creates a gRPC interceptor that injects a request-scoped logger
// into the context and logs request details
func LoggerInterceptor(baseLogger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Extract request metadata
		requestID := extractRequestID(ctx)

		// Create a request-scoped logger with additional fields
		reqLogger := baseLogger.With(
			zap.String("requestID", requestID),
			zap.String("method", info.FullMethod),
		)

		// Add request type and basic info
		reqLogger = addRequestInfo(reqLogger, req)

		// Start timing the request
		startTime := time.Now()

		// Log request start
		reqLogger.Debug("Processing request")

		// Inject logger into context for the handler to use
		ctxWithLogger := logger.WithContext(ctx, reqLogger)

		// Execute the handler
		resp, err := handler(ctxWithLogger, req)

		// Calculate duration
		duration := time.Since(startTime)

		// Determine status code
		statusCode := "OK"
		if err != nil {
			st, _ := status.FromError(err)
			statusCode = st.Code().String()
		}

		// Log completion with appropriate level based on error
		if err != nil {
			reqLogger.Error("Request failed",
				zap.Error(err),
				zap.String("status", statusCode),
				zap.Duration("duration", duration),
			)
		} else {
			reqLogger.Info("Request completed",
				zap.String("status", statusCode),
				zap.Duration("duration", duration),
			)
		}

		return resp, err
	}
}

// extractRequestID gets the request ID from context metadata
func extractRequestID(ctx context.Context) string {
	requestID := "unknown"
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get("x-request-id"); len(values) > 0 {
			requestID = values[0]
		}
	}
	return requestID
}

// addRequestInfo adds basic information about the request to the logger
func addRequestInfo(log *zap.Logger, req interface{}) *zap.Logger {
	// Add type-specific logging based on the request type
	switch typedReq := req.(type) {
	case *proto.ShortenURLRequest:
		if typedReq.OriginalUrl != "" {
			log = log.With(zap.String("originalUrl", typedReq.OriginalUrl))
		}
	case *proto.ExpandURLRequest:
		if typedReq.ShortId != "" {
			log = log.With(zap.String("shortId", typedReq.ShortId))
		}
	}

	return log
}

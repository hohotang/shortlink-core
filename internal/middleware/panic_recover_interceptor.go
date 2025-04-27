// internal/middleware/panic_recover_interceptor.go

package middleware

import (
	"context"
	"runtime/debug"

	"github.com/hohotang/shortlink-core/internal/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PanicRecoveryInterceptor creates a gRPC interceptor that recovers from panics
// and returns a gRPC error with code Internal
func PanicRecoveryInterceptor(baseLogger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		// Get the request-scoped logger if it exists
		log := logger.FromContext(ctx)
		if log == nil {
			log = baseLogger
		}

		// Recover from any panic
		defer func() {
			if r := recover(); r != nil {
				// Log the panic with stack trace
				stackTrace := string(debug.Stack())
				log.Error("Panic recovered in gRPC handler",
					zap.Any("panic", r),
					zap.String("method", info.FullMethod),
					zap.String("stack", stackTrace),
				)

				// Create an error that will be returned to the client
				err = status.Errorf(
					codes.Internal,
					"Internal server error: %s",
					"an unexpected error occurred, please contact support if the issue persists",
				)
			}
		}()

		// Call the handler
		return handler(ctx, req)
	}
}

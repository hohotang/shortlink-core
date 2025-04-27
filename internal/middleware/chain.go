// internal/middleware/chain.go

package middleware

import (
	"context"

	"google.golang.org/grpc"
)

// ChainUnaryInterceptors creates a single interceptor from multiple interceptors.
// The first interceptor will be the outer-most, while the last interceptor will
// be the inner-most wrapper around the real call.
func ChainUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Build the chain of interceptors
		chainedHandler := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			index := i // Create a copy of i to avoid closure issues
			chainedHandler = func(currentCtx context.Context, currentReq interface{}) (interface{}, error) {
				return interceptors[index](currentCtx, currentReq, info, chainedHandler)
			}
		}

		// Execute the chain
		return chainedHandler(ctx, req)
	}
}

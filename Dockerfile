FROM golang:1.24.1 as builder

WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/shortlink-core ./cmd/server

# Create a minimal image
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/shortlink-core .
COPY config.yaml .

# Expose the gRPC port
EXPOSE 50051

# Run the service
CMD ["/app/shortlink-core"] 
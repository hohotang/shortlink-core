# 📎 shortlink-core

A URL shortener core service that provides URL shortening and expansion functionality via gRPC.

This is the **Core Service** for the distributed URL shortener system. It handles URL shortening and expansion requests from the API Gateway.

## 📌 Features

- Exposes a **gRPC API** for:
  - Shortening URLs
  - Expanding shortened URLs
- Uses **Snowflake algorithm** with **Base62 encoding** for generating short IDs
- Supports multiple storage options:
  - In-memory storage
  - Redis cache
  - PostgreSQL database
  - Combined PostgreSQL + Redis for optimal performance
- Configuration using **Viper** with YAML and environment variables

## 🧱 Project Structure

```
shortlink-core/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/                  # Configuration loader with Viper
│   ├── service/                 # Service implementation
│   │   └── url_service.go       # URLService implementation
│   ├── storage/                 # Storage interfaces and implementations
│   │   ├── storage.go           # URLStorage interface
│   │   ├── memory.go            # In-memory storage implementation
│   │   ├── redis.go             # Redis storage implementation
│   │   ├── postgres.go          # PostgreSQL storage implementation
│   │   └── combined.go          # Combined Redis + PostgreSQL implementation
│   └── utils/                   # Utility functions
│       └── id_generator.go      # Snowflake ID generator with Base62 encoding
├── proto/                       # Protocol Buffers definitions
│   ├── shortlink.proto          # Service and message definitions
│   ├── shortlink.pb.go          # Generated proto code
│   └── shortlink_grpc.pb.go     # Generated gRPC code
├── config.yaml                  # Application configuration
├── Dockerfile                   # Docker build file
├── go.mod / go.sum              # Go module dependencies
└── README.md                    # Project documentation
```

## 🚀 Getting Started

### Prerequisites

- Go 1.18+
- PostgreSQL (optional)
- Redis (optional)
- Docker (optional)

### Configuration Options

The service can be configured via `config.yaml` or environment variables with `SHORTLINK_` prefix:

```yaml
server:
  port: 50051

storage:
  # Available options: memory, redis, postgres, both (both redis and postgres)
  type: postgres
  redis_url: redis://localhost:6379
  postgres_url: postgres://postgres:postgres@localhost:5432/shortlink?sslmode=disable
  cache_ttl: 3600 # seconds

snowflake:
  machine_id: 1
```

### Run locally

```bash
go run ./cmd/server
```

### Run with Docker

```bash
docker build -t shortlink-core .
docker run -p 50051:50051 shortlink-core
```

## 🧬 gRPC API

Defined in `proto/shortlink.proto`.

```proto
service URLService {
  // ShortenURL creates a short URL from the original URL
  rpc ShortenURL(ShortenURLRequest) returns (ShortenURLResponse);
  
  // ExpandURL resolves a short URL to its original URL
  rpc ExpandURL(ExpandURLRequest) returns (ExpandURLResponse);
}
```

## About the ID Generation

The service uses Twitter's Snowflake algorithm to generate IDs:
- 41 bits for timestamp (milliseconds since epoch) - provides ~69 years of unique IDs
- 10 bits for machine ID - supports up to 1024 different nodes
- 12 bits for sequence - up to 4096 IDs per millisecond per node

These numeric IDs are then encoded to Base62 (0-9, a-z, A-Z) for shorter representation.

## 📦 TODO (Next Steps)

- [ ] Add unit tests
- [ ] Add integration tests with the API Gateway
- [ ] Add OpenTelemetry tracing
- [ ] Add metrics collection
- [ ] Use pod IP for machine ID in Kubernetes environments 
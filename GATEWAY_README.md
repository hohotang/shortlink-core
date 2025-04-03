# 📎 shortlink-api-gateway

A side project for experimenting with distributed architecture, observability (OpenTelemetry), and service communication (gRPC).

This is the **API Gateway** service for the distributed URL shortener system.  
It handles client requests, forwards them to internal services (via gRPC), and returns the results.

---

## 📌 Features

- Accepts **HTTP REST API** from clients
- Forwards requests to `shortlink-url-service` via **gRPC**
- Provides endpoints to:
  - Shorten a URL
  - Expand a short link
- Integrates with **OpenTelemetry** and **Grafana Tempo** for distributed tracing

---

## 🧱 Project Structure

```
shortlink-gateway/
├── cmd/
│   └── gateway/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/                  # Configuration loader
│   ├── engine/                  # Gin engine setup
│   ├── handler/                 # HTTP handlers
│   │   ├── expand.go            # URL expansion handler
│   │   └── shorten.go           # URL shortening handler
│   ├── logger/                  # Zap logger integration
│   ├── middleware/              # HTTP middleware
│   ├── otel/                    # OpenTelemetry setup
│   ├── server/                  # Server and router
│   └── service/                 # Service layer implementation
│       ├── url_service.go       # URLService interface and Mock implementation
│       └── url_grpc_client.go   # gRPC client implementation
├── proto/                       # Protocol Buffers definitions
│   ├── shortlink.proto          # Service and message definitions
│   ├── shortlink.pb.go          # Generated proto code
│   └── shortlink_grpc.pb.go     # Generated gRPC code
├── tempo/                       # Tempo distributed tracing config
├── grafana/                     # Grafana visualization config
├── .env                         # Environment variables
├── .env_example                 # Environment variables example
├── config.yaml                  # Application configuration
├── docker-compose.yml           # Docker Compose configuration
├── Dockerfile                   # Docker build file
├── go.mod / go.sum              # Go module dependencies
├── Makefile                     # Project build and management
└── README.md                    # Project documentation
```

---

## 🚀 Getting Started

### Prerequisites

- Go 1.24+
- Docker + Docker Compose

### Run locally with Docker Compose

```bash
docker-compose up --build
```

This will launch:
- `gateway` on `http://localhost:8080`
- `Grafana` on `http://localhost:3000` (user: `admin`, pass: `admin`)
- `Tempo` OTLP receiver on port `4318`

### Run locally (without Docker)

```bash
go run ./cmd/gateway
```

---

## 🧪 API Endpoints

| Method | Path         | Description           |
|--------|--------------|-----------------------|
| POST   | `/shorten`   | Shortens a long URL   |
| GET    | `/:shortID`  | Redirects to original |

---

## 🧬 gRPC Public API

Defined in `proto/shortlink.proto`.

```proto
service UrlPublicAPI {
  rpc ShortenUrl(ShortenRequest) returns (ShortenResponse);
  rpc ExpandUrl(ExpandRequest) returns (ExpandResponse);
}
```

---

## 📦 TODO (Next Steps)

- [x] Add OpenTelemetry tracing via stdout
- [x] Replace stdout exporter with OTLP exporter
- [x] Dockerize Gateway + Tempo + Grafana stack
- [x] Implement gRPC client to URL service
- [ ] Unit testing and integration tests
- [x] Inject handler
- [ ] Add RateLimiter
- [ ] Error Handle improvement

---

### View Traces in Grafana

1. Visit [http://localhost:3000](http://localhost:3000)
2. Go to **Explore > Tempo**
3. Use `service.name = "api-gateway"` to filter traces
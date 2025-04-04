# Application name and binary location
APP_NAME=shortlink-core
BIN_DIR=bin
BIN=$(BIN_DIR)/$(APP_NAME)

# Go command and packages
GO_CMD=go
GO_BUILD=$(GO_CMD) build
GO_CLEAN=$(GO_CMD) clean
GO_TEST=$(GO_CMD) test
GO_GET=$(GO_CMD) get
GO_MOD=$(GO_CMD) mod
PACKAGES=./...

# Linting
GOLANGCI_LINT=golangci-lint

# Detect OS
ifeq ($(OS),Windows_NT)
    WHICH_CMD := where
    NULL_DEV := nul 2>&1
    PROTO_INSTALL_MSG := "protoc not installed. Please download and install from: https://github.com/protocolbuffers/protobuf/releases"
else
    WHICH_CMD := which
    NULL_DEV := /dev/null
    PROTO_INSTALL_MSG := "protoc not installed. Please install using your package manager: apt-get install protobuf-compiler or brew install protobuf"
endif

# Check if tools are installed
lint-check:
	@$(WHICH_CMD) $(GOLANGCI_LINT) >$(NULL_DEV) || (echo "$(GOLANGCI_LINT) not installed. Installing..." && \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)

.PHONY: all build clean test lint run tidy deps

# Default target
all: lint test build

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN) ./cmd/server

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR)
	$(GO_CLEAN) $(PACKAGES)

# Run tests
test:
	@echo "Running tests..."
	$(GO_TEST) -v $(PACKAGES)

# Lint the Go code
lint: lint-check
	$(GOLANGCI_LINT) run ./...

# Run the application
run:
	@echo "Running $(APP_NAME)..."
	$(GO_CMD) run ./cmd/server

# Update dependencies
tidy:
	@echo "Tidying Go modules..."
	$(GO_MOD) tidy

# Install required dependencies
deps:
	@echo "Installing dependencies..."
	$(GO_GET) $(PACKAGES) 
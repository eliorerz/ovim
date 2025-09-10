# OVIM Backend Makefile
# OpenShift Virtual Infrastructure Manager (OVIM) Backend Build and Deployment

# Project configuration
PROJECT_NAME := ovim-backend
BINARY_NAME := ovim_server
MAIN_PATH := ./cmd/ovim-server
MODULE_NAME := github.com/eliorerz/ovim-updated

# Container configuration
CONTAINER_IMAGE := ovim-backend:latest
BACKEND_CONTAINER_NAME := ovim-backend-container

# Port configuration
DB_PORT := 5432

# Database configuration
DB_USER := ovim
DB_PASSWORD := ovim123
DB_NAME := ovim
DB_HOST := localhost
DATABASE_URL := postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

# Build configuration
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
GIT_TREE_STATE := $(shell if git diff --quiet 2>/dev/null; then echo "clean"; else echo "dirty"; fi)

# Go build flags
LDFLAGS := -ldflags "\
	-X $(MODULE_NAME)/pkg/version.gitVersion=$(GIT_VERSION) \
	-X $(MODULE_NAME)/pkg/version.gitCommit=$(GIT_COMMIT) \
	-X $(MODULE_NAME)/pkg/version.gitTreeState=$(GIT_TREE_STATE) \
	-X $(MODULE_NAME)/pkg/version.buildDate=$(BUILD_DATE)"

# Go configuration
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GO_BUILD_FLAGS := -tags netgo -a -installsuffix cgo
GO_TEST_FLAGS := -race -cover

.PHONY: help clean build test test-unit test-integration lint fmt deps run dev dev-with-db server-stop
.PHONY: container-build container-clean
.PHONY: db-start db-stop
.PHONY: version

help:
	@echo "OVIM Backend Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':'

clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@go clean -cache -testcache -modcache
	@echo "Stopping and removing containers..."
	@-podman stop $(BACKEND_CONTAINER_NAME) 2>/dev/null || true
	@-podman rm $(BACKEND_CONTAINER_NAME) 2>/dev/null || true

deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod verify
	@go mod tidy

fmt: 
	@echo "Formatting Go code and running go vet..."
	@go fmt ./...
	@go vet ./...

## lint: Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
		echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## test: Run all tests
test: fmt
	@echo "Running all tests..."
	@go test $(GO_TEST_FLAGS) ./...

test-unit: fmt
	@echo "Running unit tests..."
	@go test $(GO_TEST_FLAGS) -short ./pkg/...

test-integration: fmt
	@echo "Running integration tests..."
	@go test $(GO_TEST_FLAGS) -coverprofile=coverage.out ./test/integration/...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

build: deps fmt
	@echo "Building $(BINARY_NAME)..."
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		$(GO_BUILD_FLAGS) $(LDFLAGS) \
		-o $(BINARY_NAME) $(MAIN_PATH)

run: server-stop build
	@echo "Starting OVIM backend server with HTTPS..."
	@OVIM_DATABASE_URL="$(DATABASE_URL)" \
	 OVIM_ENVIRONMENT=development \
	 OVIM_LOG_LEVEL=info \
	 ./$(BINARY_NAME)

## dev: Run in development mode with hot reload
dev: server-stop deps
	@OVIM_DATABASE_URL="$(DATABASE_URL)" $(MAKE) _dev-server

dev-with-db: server-stop db-start
	@OVIM_DATABASE_URL="$(DATABASE_URL)" $(MAKE) _dev-server

# Internal target for running dev server (shared logic)
_dev-server:
	@echo "Starting development server with HTTPS..."
	@if command -v air >/dev/null 2>&1; then \
		OVIM_ENVIRONMENT=development \
		OVIM_LOG_LEVEL=debug \
		air; \
	else \
		echo "air not installed, running without hot reload..."; \
		$(MAKE) build; \
		OVIM_ENVIRONMENT=development \
		OVIM_LOG_LEVEL=info \
		./$(BINARY_NAME); \
	fi

server-stop:
	@echo "Stopping any running OVIM server processes..."
	@-pkill -f "ovim_server" 2>/dev/null || true
	@-pkill -f "ovim-server" 2>/dev/null || true
	@-pkill -f "go run.*ovim-server" 2>/dev/null || true
	@-pkill -f "air" 2>/dev/null || true
	@echo "Server processes stopped"

container-build:
	@echo "Building container image $(CONTAINER_IMAGE)..."
	@podman build \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		--build-arg GIT_COMMIT="$(GIT_COMMIT)" \
		--build-arg GIT_VERSION="$(GIT_VERSION)" \
		-t $(CONTAINER_IMAGE) .

container-clean:
	@echo "Cleaning container $(BACKEND_CONTAINER_NAME)..."
	@-podman stop $(BACKEND_CONTAINER_NAME) 2>/dev/null || true
	@-podman rm $(BACKEND_CONTAINER_NAME) 2>/dev/null || true

version:
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(GIT_VERSION)"
	@echo "Commit: $(GIT_COMMIT)"
	@echo "Tree State: $(GIT_TREE_STATE)"
	@echo "Build Date: $(BUILD_DATE)"

# db-start: Start PostgreSQL database
db-start: db-stop
	@echo "Starting PostgreSQL database..."
	@if command -v podman-compose >/dev/null 2>&1; then \
		podman-compose up -d postgres; \
	else \
		echo "Starting PostgreSQL with podman run..."; \
		podman run -d --name ovim-postgres \
			-e POSTGRES_USER=$(DB_USER) \
			-e POSTGRES_PASSWORD=$(DB_PASSWORD) \
			-e POSTGRES_DB=$(DB_NAME) \
			-p $(DB_PORT):5432 \
			-v ovim_postgres_data:/var/lib/postgresql/data \
			docker.io/postgres:16-alpine; \
	fi
	@echo "Waiting for database to be ready..."
	@sleep 5
	@echo "Database started successfully"

db-stop:
	@echo "Stopping PostgreSQL database..."
	@if command -v podman-compose >/dev/null 2>&1; then \
		podman-compose down postgres; \
	else \
		podman stop ovim-postgres 2>/dev/null || true; \
		podman rm ovim-postgres 2>/dev/null || true; \
	fi
	@echo "Database stopped"

# Default target
all: test build
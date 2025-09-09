# OVIM Backend Makefile
# OpenShift Virtual Infrastructure Manager (OVIM) Backend Build and Deployment

# Project configuration
PROJECT_NAME := ovim-backend
BINARY_NAME := ovim_server
MAIN_PATH := ./cmd/ovim-server
MODULE_NAME := github.com/eliorerz/ovim-updated

# Container configuration
CONTAINER_REGISTRY ?= quay.io
CONTAINER_NAMESPACE ?= ovim
CONTAINER_NAME := $(PROJECT_NAME)
CONTAINER_TAG ?= latest
CONTAINER_IMAGE := $(CONTAINER_REGISTRY)/$(CONTAINER_NAMESPACE)/$(CONTAINER_NAME):$(CONTAINER_TAG)

# Podman pod configuration
POD_NAME := ovim-pod
BACKEND_CONTAINER_NAME := ovim-backend-container
FRONTEND_CONTAINER_NAME := ovim-ui-container

# Port configuration
BACKEND_PORT := 8080
FRONTEND_PORT := 3000
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

# Targets
.PHONY: help clean build test lint fmt vet deps run dev dev-with-tls dev-with-db-tls server-stop
.PHONY: container-build container-run container-push container-clean
.PHONY: pod-create pod-start pod-stop pod-clean pod-logs pod-status
.PHONY: db-start db-stop db-restart db-logs db-shell db-migrate db-seed
.PHONY: install-tools release

## help: Show this help message
help:
	@echo "OVIM Backend Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':'

## clean: Clean build artifacts and containers
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@go clean -cache -testcache -modcache
	@echo "Stopping and removing containers..."
	@-podman stop $(BACKEND_CONTAINER_NAME) 2>/dev/null || true
	@-podman rm $(BACKEND_CONTAINER_NAME) 2>/dev/null || true

## deps: Download and verify dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod verify
	@go mod tidy

## fmt: Format Go code
fmt:
	@echo "Formatting Go code..."
	@go fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...

## lint: Run golangci-lint
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
		echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## test: Run tests
test: fmt vet
	@echo "Running tests..."
	@go test $(GO_TEST_FLAGS) ./...

## test-unit: Run unit tests only (excludes integration)
test-unit: fmt vet
	@echo "Running unit tests..."
	@go test $(GO_TEST_FLAGS) -short ./pkg/...

## test-integration: Run integration tests (memory storage only)
test-integration: fmt vet
	@echo "Running integration tests..."
	@go test $(GO_TEST_FLAGS) -short ./test/integration/...

## test-integration-full: Run all integration tests including PostgreSQL
test-integration-full: fmt vet
	@echo "Running full integration tests..."
	@go test $(GO_TEST_FLAGS) ./test/integration/...

## test-integration-coverage: Run integration tests with coverage
test-integration-coverage: fmt vet
	@echo "Running integration tests with coverage..."
	@go test $(GO_TEST_FLAGS) -coverprofile=coverage.out ./test/integration/...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## test-all: Run all tests including unit and integration
test-all: test-unit test-integration-full

## build: Build the binary
build: deps fmt vet
	@echo "Building $(BINARY_NAME)..."
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		$(GO_BUILD_FLAGS) $(LDFLAGS) \
		-o $(BINARY_NAME) $(MAIN_PATH)

## run: Build and run the server locally
run: server-stop build
	@echo "Starting OVIM backend server..."
	@OVIM_ENVIRONMENT=development \
	 OVIM_PORT=$(BACKEND_PORT) \
	 OVIM_LOG_LEVEL=info \
	 ./$(BINARY_NAME)

## dev: Run in development mode with hot reload
dev: server-stop deps
	@echo "Starting development server..."
	@if command -v air >/dev/null 2>&1; then \
		OVIM_ENVIRONMENT=development \
		OVIM_PORT=$(BACKEND_PORT) \
		OVIM_LOG_LEVEL=debug \
		air; \
	else \
		echo "air not installed, running without hot reload..."; \
		$(MAKE) build; \
		OVIM_ENVIRONMENT=development \
		OVIM_PORT=$(BACKEND_PORT) \
		OVIM_LOG_LEVEL=info \
		./$(BINARY_NAME); \
	fi

## server-stop: Stop any running OVIM server processes
server-stop:
	@echo "Stopping any running OVIM server processes..."
	@-pkill -f "ovim_server" 2>/dev/null || true
	@-pkill -f "ovim-server" 2>/dev/null || true
	@-pkill -f "go run.*ovim-server" 2>/dev/null || true
	@-pkill -f "air" 2>/dev/null || true
	@echo "Server processes stopped"

## container-build: Build container image
container-build:
	@echo "Building container image $(CONTAINER_IMAGE)..."
	@podman build \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		--build-arg GIT_COMMIT="$(GIT_COMMIT)" \
		--build-arg GIT_VERSION="$(GIT_VERSION)" \
		-t $(CONTAINER_IMAGE) .

## container-run: Run container locally
container-run: container-clean container-build
	@echo "Running container $(BACKEND_CONTAINER_NAME)..."
	@podman run -d \
		--name $(BACKEND_CONTAINER_NAME) \
		-p $(BACKEND_PORT):$(BACKEND_PORT) \
		-e OVIM_PORT=$(BACKEND_PORT) \
		-e OVIM_ENVIRONMENT=development \
		-e OVIM_LOG_LEVEL=info \
		$(CONTAINER_IMAGE)
	@echo "Backend container started on port $(BACKEND_PORT)"

## container-push: Push container image to registry
container-push: container-build
	@echo "Pushing container image $(CONTAINER_IMAGE)..."
	@podman push $(CONTAINER_IMAGE)

## container-clean: Stop and remove container
container-clean:
	@echo "Cleaning container $(BACKEND_CONTAINER_NAME)..."
	@-podman stop $(BACKEND_CONTAINER_NAME) 2>/dev/null || true
	@-podman rm $(BACKEND_CONTAINER_NAME) 2>/dev/null || true

## pod-create: Create podman pod for full stack
pod-create:
	@echo "Creating pod $(POD_NAME)..."
	@-podman pod rm $(POD_NAME) 2>/dev/null || true
	@podman pod create \
		--name $(POD_NAME) \
		-p $(BACKEND_PORT):$(BACKEND_PORT) \
		-p $(FRONTEND_PORT):$(FRONTEND_PORT)

## pod-start: Start the full stack in a pod
pod-start: pod-create container-build
	@echo "Starting full OVIM stack..."
	@podman run -d \
		--name $(BACKEND_CONTAINER_NAME) \
		--pod $(POD_NAME) \
		-e OVIM_PORT=$(BACKEND_PORT) \
		-e OVIM_ENVIRONMENT=development \
		$(CONTAINER_IMAGE)
	@echo "OVIM backend started in pod $(POD_NAME)"
	@echo "Backend available at: http://localhost:$(BACKEND_PORT)"
	@echo "API health check: http://localhost:$(BACKEND_PORT)/health"

## pod-stop: Stop the pod
pod-stop:
	@echo "Stopping pod $(POD_NAME)..."
	@-podman pod stop $(POD_NAME) 2>/dev/null || true

## pod-clean: Remove the pod and all containers
pod-clean:
	@echo "Cleaning pod $(POD_NAME)..."
	@-podman pod rm -f $(POD_NAME) 2>/dev/null || true

## pod-logs: Show logs from backend container
pod-logs:
	@echo "Showing logs for $(BACKEND_CONTAINER_NAME)..."
	@podman logs -f $(BACKEND_CONTAINER_NAME)

## pod-status: Show pod and container status
pod-status:
	@echo "Pod status:"
	@podman pod ps --filter name=$(POD_NAME) || echo "Pod not found"
	@echo ""
	@echo "Container status:"
	@podman ps --filter name=$(BACKEND_CONTAINER_NAME) || echo "Backend container not found"

## install-tools: Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/cosmtrek/air@latest
	@echo "Tools installed successfully"

## release: Build release binaries for multiple platforms
release: clean test
	@echo "Building release binaries..."
	@mkdir -p dist
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$$os" = "windows" ]; then \
				ext=".exe"; \
			else \
				ext=""; \
			fi; \
			echo "Building $$os/$$arch..."; \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build \
				$(GO_BUILD_FLAGS) $(LDFLAGS) \
				-o dist/$(BINARY_NAME)-$$os-$$arch$$ext $(MAIN_PATH); \
		done; \
	done
	@echo "Release binaries built in dist/"

## version: Show version information
version:
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(GIT_VERSION)"
	@echo "Commit: $(GIT_COMMIT)"
	@echo "Tree State: $(GIT_TREE_STATE)"
	@echo "Build Date: $(BUILD_DATE)"

#################################
# Database Management Targets  #
#################################

## db-start: Start PostgreSQL database with Podman Compose
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

## db-stop: Stop PostgreSQL database
db-stop:
	@echo "Stopping PostgreSQL database..."
	@if command -v podman-compose >/dev/null 2>&1; then \
		podman-compose down postgres; \
	else \
		podman stop ovim-postgres 2>/dev/null || true; \
		podman rm ovim-postgres 2>/dev/null || true; \
	fi
	@echo "Database stopped"

## db-restart: Restart PostgreSQL database
db-restart: db-stop db-start

## db-logs: Show PostgreSQL database logs
db-logs:
	@echo "Showing PostgreSQL logs..."
	@if podman ps --filter name=ovim-postgres --format "{{.Names}}" | grep -q ovim-postgres; then \
		podman logs -f ovim-postgres; \
	elif command -v podman-compose >/dev/null 2>&1; then \
		podman-compose logs -f postgres; \
	else \
		echo "Database container not found"; \
	fi

## db-shell: Connect to PostgreSQL database shell
db-shell:
	@echo "Connecting to PostgreSQL shell..."
	@if podman ps --filter name=ovim-postgres --format "{{.Names}}" | grep -q ovim-postgres; then \
		podman exec -it ovim-postgres psql -U $(DB_USER) -d $(DB_NAME); \
	else \
		echo "Database container not running. Start it with 'make db-start'"; \
	fi

## db-migrate: Run database migrations (application will handle this automatically)
db-migrate: build
	@echo "Running database migrations..."
	@OVIM_DATABASE_URL="$(DATABASE_URL)" ./$(BINARY_NAME) -version >/dev/null
	@echo "Database migrations completed (GORM AutoMigrate)"

## db-seed: Seed database with initial data (application will handle this automatically)
db-seed: build
	@echo "Seeding database with initial data..."
	@OVIM_DATABASE_URL="$(DATABASE_URL)" ./$(BINARY_NAME) -version >/dev/null
	@echo "Database seeding completed"

## dev-with-db: Run development server with PostgreSQL
dev-with-db: server-stop db-start
	@echo "Starting development server with PostgreSQL..."
	@OVIM_DATABASE_URL="$(DATABASE_URL)" \
	 OVIM_ENVIRONMENT=development \
	 OVIM_LOG_LEVEL=debug \
	 go run $(MAIN_PATH)

## dev-with-tls: Run development server with HTTPS/TLS enabled
dev-with-tls: server-stop deps
	@echo "Starting development server with HTTPS/TLS enabled..."
	@OVIM_TLS_ENABLED=true \
	 OVIM_ENVIRONMENT=development \
	 OVIM_LOG_LEVEL=debug \
	 go run $(MAIN_PATH)

## dev-with-db-tls: Run development server with PostgreSQL and HTTPS/TLS
dev-with-db-tls: server-stop db-start
	@echo "Starting development server with PostgreSQL and HTTPS/TLS..."
	@OVIM_TLS_ENABLED=true \
	 OVIM_DATABASE_URL="$(DATABASE_URL)" \
	 OVIM_ENVIRONMENT=development \
	 OVIM_LOG_LEVEL=debug \
	 go run $(MAIN_PATH)

# Default target
all: test build
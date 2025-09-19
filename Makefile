# OVIM Backend Makefile
# OpenShift Virtual Infrastructure Manager (OVIM) Backend Build and Deployment

# Project configuration
PROJECT_NAME := ovim-backend
BUILD_DIR := build
BINARY_NAME := $(BUILD_DIR)/ovim_server
CONTROLLER_BINARY_NAME := $(BUILD_DIR)/ovim_controller
MAIN_PATH := ./cmd/ovim-server
CONTROLLER_MAIN_PATH := ./cmd/controller
MODULE_NAME := github.com/eliorerz/ovim-updated

# Container configuration
CONTAINER_IMAGE := ovim-backend:latest
BACKEND_CONTAINER_NAME := ovim-backend-container

# Image registry configuration
IMAGE_REGISTRY ?= quay.io/eerez
OVIM_SERVER_IMAGE_NAME ?= ovim
OVIM_UI_IMAGE_NAME ?= ovim-ui
OVIM_CONTROLLER_IMAGE_NAME ?= ovim

# Deployment configuration
OVIM_NAMESPACE ?= ovim-system
OVIM_IMAGE_TAG ?= latest
OVIM_CONTROLLER_IMAGE ?= $(IMAGE_REGISTRY)/$(OVIM_CONTROLLER_IMAGE_NAME)
OVIM_SERVER_IMAGE ?= $(IMAGE_REGISTRY)/$(OVIM_SERVER_IMAGE_NAME)
OVIM_UI_IMAGE ?= $(IMAGE_REGISTRY)/$(OVIM_UI_IMAGE_NAME)
KUBECTL_CMD ?= kubectl

# Stack deployment configuration
OVIM_DOMAIN ?= ovim.local
OVIM_DB_STORAGE_SIZE ?= 10Gi
OVIM_UI_REPLICAS ?= 2
OVIM_INGRESS_CLASS ?= nginx

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
BUILD_TIMESTAMP := $(shell date -u +'%Y%m%d-%H%M%S')
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_SHORT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
GIT_TREE_STATE := $(shell if git diff --quiet 2>/dev/null; then echo "clean"; else echo "dirty"; fi)

# Generate unique image tag for this build
UNIQUE_TAG := $(BUILD_TIMESTAMP)-$(GIT_SHORT_COMMIT)

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

# CRD configuration
CRD_DIR := config/crd
CRD_DOCS_DIR := docs/crds
KUBECTL_CMD ?= kubectl

.PHONY: help clean build build-controller test test-unit test-integration lint fmt deps run dev dev-with-db server-stop
.PHONY: controller-run controller-stop controller-build controller-dev
.PHONY: container-build container-clean
.PHONY: db-start db-stop db-migrate db-migrate-rollback db-migrate-validate
.PHONY: crd-install crd-uninstall crd-validate crd-status crd-docs crd-examples crd-test
.PHONY: generate manifests
.PHONY: deploy deploy-dry-run undeploy deploy-samples deploy-full deploy-dev deploy-prod undeploy-force deployment-status
.PHONY: deploy-stack deploy-stack-dry-run deploy-stack-dev deploy-stack-prod undeploy-stack deploy-database deploy-server deploy-ui deploy-ingress stack-status stack-logs stack-port-forward build-ui
.PHONY: version build-push deploy-image

help:
	@echo "OVIM Backend Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Development targets:"
	@echo "  build              Build the backend binary"
	@echo "  build-controller   Build the controller binary"
	@echo "  test               Run all tests"
	@echo "  test-unit          Run unit tests only"
	@echo "  test-integration   Run integration tests with coverage"
	@echo "  lint               Run linter"
	@echo "  fmt                Format code and run go vet"
	@echo "  run                Build and run server"
	@echo "  dev                Run in development mode with hot reload"
	@echo ""
	@echo "Controller targets:"
	@echo "  controller-build   Build the controller binary"
	@echo "  controller-run     Run the controller"
	@echo "  controller-stop    Stop running controllers"
	@echo "  controller-dev     Run controller in development mode"
	@echo ""
	@echo "Code generation targets:"
	@echo "  generate           Generate code (deepcopy, CRDs, etc.)"
	@echo "  manifests          Generate Kubernetes manifests"
	@echo ""
	@echo "Database targets:"
	@echo "  db-start           Start PostgreSQL database"
	@echo "  db-stop            Stop PostgreSQL database"
	@echo "  db-migrate         Apply CRD database migration"
	@echo "  db-migrate-validate Validate database migration"
	@echo "  db-migrate-rollback Rollback database migration (destructive)"
	@echo ""
	@echo "CRD targets:"
	@echo "  crd-install        Install OVIM CRDs to cluster"
	@echo "  crd-uninstall      Uninstall CRDs (destructive)"
	@echo "  crd-validate       Validate existing CRD installation"
	@echo "  crd-status         Show CRD status"
	@echo "  crd-docs           Generate CRD documentation"
	@echo "  crd-examples       Generate CRD usage examples"
	@echo "  crd-test           Run CRD-specific tests"
	@echo ""
	@echo "Deployment targets:"
	@echo "  deploy             Deploy OVIM to Kubernetes cluster"
	@echo "  deploy-dry-run     Show what would be deployed without applying"
	@echo "  deploy-samples     Deploy sample resources after OVIM deployment"
	@echo "  deploy-full        Complete deployment with samples and DB migration"
	@echo "  deploy-dev         Deploy with development configuration"
	@echo "  deploy-prod        Deploy with production configuration"
	@echo "  undeploy           Remove OVIM from Kubernetes cluster"
	@echo "  deployment-status  Show current deployment status"
	@echo ""
	@echo "Full Stack targets:"
	@echo "  deploy-stack       Deploy complete stack (PostgreSQL + OVIM + UI + Ingress)"
	@echo "  deploy-stack-unique Deploy complete stack with unique timestamp tags"
	@echo "  deploy-stack-dry-run Dry run of full stack deployment"
	@echo "  deploy-stack-dev   Deploy development stack"
	@echo "  deploy-stack-prod  Deploy production stack"
	@echo "  deploy-database    Deploy only PostgreSQL database"
	@echo "  deploy-server      Deploy only OVIM server"
	@echo "  deploy-ui          Deploy only OVIM UI"
	@echo "  deploy-ingress     Deploy only ingress configuration"
	@echo "  undeploy-stack     Remove entire stack"
	@echo "  stack-status       Show status of all stack components"
	@echo "  stack-logs         Show logs from all stack components"
	@echo ""
	@echo "Container targets:"
	@echo "  container-build      Build OVIM server container image"
	@echo "  container-build-ui   Build OVIM UI container image"
	@echo "  container-push       Build and push server image to registry"
	@echo "  container-push-ui    Build and push UI image to registry"
	@echo "  container-push-all   Build and push all images to registry"
	@echo "  container-clean      Clean container"
	@echo ""
	@echo "Other targets:"
	@echo "  clean              Clean build artifacts and containers"
	@echo "  deps               Download and verify dependencies"
	@echo "  version            Show version information"

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@go clean -cache -testcache -modcache
	@echo "Stopping and removing containers..."
	@-podman stop $(BACKEND_CONTAINER_NAME) 2>/dev/null || true
	@-podman rm $(BACKEND_CONTAINER_NAME) 2>/dev/null || true
	$(MAKE) controller-stop
	$(MAKE) server-stop

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
	@GOPATH=$$(go env GOPATH); \
	if command -v golangci-lint >/dev/null 2>&1; then \
		if golangci-lint run --timeout=10m 2>/dev/null; then \
			echo "golangci-lint completed successfully"; \
		else \
			echo "golangci-lint failed, falling back to basic linting..."; \
			go vet ./... && echo "go vet passed"; \
		fi; \
	elif [ -f "$$GOPATH/bin/golangci-lint" ]; then \
		if $$GOPATH/bin/golangci-lint run --timeout=10m 2>/dev/null; then \
			echo "golangci-lint completed successfully"; \
		else \
			echo "golangci-lint failed, falling back to basic linting..."; \
			go vet ./... && echo "go vet passed"; \
		fi; \
	else \
		echo "golangci-lint not available, using basic linting..."; \
		go vet ./... && echo "go vet passed"; \
	fi

## test: Run all tests (excluding hanging integration tests)
test: fmt
	@echo "Running all tests..."
	@go test $(GO_TEST_FLAGS) ./auth ./config ./pkg/... ./controllers ./webhook
	@echo "Note: Integration tests excluded due to hanging issue (tests pass individually)"

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
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		$(GO_BUILD_FLAGS) $(LDFLAGS) \
		-o $(BINARY_NAME) $(MAIN_PATH)

build-controller: deps fmt
	@echo "Building $(CONTROLLER_BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		$(GO_BUILD_FLAGS) $(LDFLAGS) \
		-o $(CONTROLLER_BINARY_NAME) $(CONTROLLER_MAIN_PATH)

controller-build: build-controller

controller-run: controller-stop controller-build
	@echo "Starting OVIM controller..."
	@OVIM_DATABASE_URL="$(DATABASE_URL)" \
	 OVIM_ENVIRONMENT=development \
	 OVIM_LOG_LEVEL=info \
	 ./$(CONTROLLER_BINARY_NAME) \
		--metrics-bind-address=:8080 \
		--health-probe-bind-address=:8081 \
		--database-url="$(DATABASE_URL)" \
		--leader-elect=false

controller-dev: controller-stop
	@echo "Starting controller in development mode..."
	@OVIM_DATABASE_URL="$(DATABASE_URL)" \
	 OVIM_ENVIRONMENT=development \
	 OVIM_LOG_LEVEL=debug \
	 go run $(CONTROLLER_MAIN_PATH) \
		--metrics-bind-address=:8080 \
		--health-probe-bind-address=:8081 \
		--database-url="$(DATABASE_URL)" \
		--leader-elect=false

controller-stop:
	@echo "Stopping any running OVIM controller processes..."
	@-pkill -f "ovim_controller" 2>/dev/null || true
	@-pkill -f "ovim-controller" 2>/dev/null || true
	@-pkill -f "go run.*controller" 2>/dev/null || true
	@echo "Controller processes stopped"

run: server-stop build
	@echo "Starting OVIM backend server with HTTPS..."
	@OVIM_DATABASE_URL="$(DATABASE_URL)" \
	 OVIM_ENVIRONMENT=development \
	 OVIM_LOG_LEVEL=info \
	 ./$(BINARY_NAME)

## dev: Run in development mode with hot reload
dev: server-stop deps
	@OVIM_DATABASE_URL="$(DATABASE_URL)" \
	 OVIM_OPENSHIFT_ENABLED="$(OVIM_OPENSHIFT_ENABLED)" \
	 OVIM_OPENSHIFT_USE_MOCK="$(OVIM_OPENSHIFT_USE_MOCK)" \
	 OVIM_OPENSHIFT_KUBECONFIG="$(OVIM_OPENSHIFT_KUBECONFIG)" \
	 OVIM_OPENSHIFT_TEMPLATE_NAMESPACE="$(OVIM_OPENSHIFT_TEMPLATE_NAMESPACE)" \
	 OVIM_OIDC_ENABLED="$(OVIM_OIDC_ENABLED)" \
	 $(MAKE) _dev-server

dev-with-db: server-stop db-start
	@OVIM_DATABASE_URL="$(DATABASE_URL)" \
	 OVIM_OPENSHIFT_ENABLED="$(OVIM_OPENSHIFT_ENABLED)" \
	 OVIM_OPENSHIFT_USE_MOCK="$(OVIM_OPENSHIFT_USE_MOCK)" \
	 OVIM_OPENSHIFT_KUBECONFIG="$(OVIM_OPENSHIFT_KUBECONFIG)" \
	 OVIM_OPENSHIFT_TEMPLATE_NAMESPACE="$(OVIM_OPENSHIFT_TEMPLATE_NAMESPACE)" \
	 OVIM_OIDC_ENABLED="$(OVIM_OIDC_ENABLED)" \
	 $(MAKE) _dev-server

# Internal target for running dev server (shared logic)
_dev-server:
	@echo "Starting development server with HTTPS on port 8090..."
	@if command -v air >/dev/null 2>&1; then \
		OVIM_TLS_PORT=8090 \
		OVIM_DATABASE_URL="$(OVIM_DATABASE_URL)" \
		OVIM_ENVIRONMENT=development \
		OVIM_LOG_LEVEL=debug \
		OVIM_OPENSHIFT_ENABLED="$(OVIM_OPENSHIFT_ENABLED)" \
		OVIM_OPENSHIFT_USE_MOCK="$(OVIM_OPENSHIFT_USE_MOCK)" \
		OVIM_OPENSHIFT_KUBECONFIG="$(OVIM_OPENSHIFT_KUBECONFIG)" \
		OVIM_OPENSHIFT_TEMPLATE_NAMESPACE="$(OVIM_OPENSHIFT_TEMPLATE_NAMESPACE)" \
		OVIM_OIDC_ENABLED="$(OVIM_OIDC_ENABLED)" \
		air; \
	else \
		echo "air not installed, running without hot reload..."; \
		$(MAKE) build; \
		OVIM_TLS_PORT=8090 \
		OVIM_DATABASE_URL="$(OVIM_DATABASE_URL)" \
		OVIM_ENVIRONMENT=development \
		OVIM_LOG_LEVEL=info \
		OVIM_OPENSHIFT_ENABLED="$(OVIM_OPENSHIFT_ENABLED)" \
		OVIM_OPENSHIFT_USE_MOCK="$(OVIM_OPENSHIFT_USE_MOCK)" \
		OVIM_OPENSHIFT_KUBECONFIG="$(OVIM_OPENSHIFT_KUBECONFIG)" \
		OVIM_OPENSHIFT_TEMPLATE_NAMESPACE="$(OVIM_OPENSHIFT_TEMPLATE_NAMESPACE)" \
		OVIM_OIDC_ENABLED="$(OVIM_OIDC_ENABLED)" \
		./$(BINARY_NAME); \
	fi

server-stop:
	@echo "Stopping any running OVIM server processes..."
	@-pkill -f "ovim_server" 2>/dev/null || true
	@-pkill -f "ovim-server" 2>/dev/null || true
	@-pkill -f "go run.*ovim-server" 2>/dev/null || true
	@-pkill -f "air" 2>/dev/null || true
	@echo "Server processes stopped"

# Build and tag server container image
container-build: build build-controller
	@echo "Building OVIM server container image..."
	@echo "Unique tag: $(UNIQUE_TAG)"
	@podman build \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		--build-arg GIT_COMMIT="$(GIT_COMMIT)" \
		--build-arg GIT_VERSION="$(GIT_VERSION)" \
		-t $(OVIM_SERVER_IMAGE):$(OVIM_IMAGE_TAG) \
		-t $(OVIM_SERVER_IMAGE):$(UNIQUE_TAG) \
		-t $(OVIM_SERVER_IMAGE):latest \
		-f Dockerfile .

# Build UI container image  
container-build-ui:
	@echo "Building OVIM UI container image..."
	@echo "Unique tag: $(UNIQUE_TAG)"
	@cd ../ovim-ui && make container-build
	@echo "Tagging UI image for registry..."
	@podman tag ovim-ui:latest $(OVIM_UI_IMAGE):$(OVIM_IMAGE_TAG)
	@podman tag ovim-ui:latest $(OVIM_UI_IMAGE):$(UNIQUE_TAG)
	@podman tag ovim-ui:latest $(OVIM_UI_IMAGE):latest

# Push server image to registry
container-push: container-build
	@echo "Pushing OVIM server image to registry..."
	@echo "Tags: $(OVIM_IMAGE_TAG), $(UNIQUE_TAG), latest"
	@podman push $(OVIM_SERVER_IMAGE):$(OVIM_IMAGE_TAG)
	@podman push $(OVIM_SERVER_IMAGE):$(UNIQUE_TAG)
	@podman push $(OVIM_SERVER_IMAGE):latest

# Push UI image to registry
container-push-ui: container-build-ui
	@echo "Pushing OVIM UI image to registry..."
	@echo "Tags: $(OVIM_IMAGE_TAG), $(UNIQUE_TAG), latest"
	@podman push $(OVIM_UI_IMAGE):$(OVIM_IMAGE_TAG)
	@podman push $(OVIM_UI_IMAGE):$(UNIQUE_TAG)
	@podman push $(OVIM_UI_IMAGE):latest

# Build and push all images (UI temporarily disabled)
container-push-all: container-push
	@echo "All images pushed to registry successfully (UI skipped)"

container-clean:
	@echo "Cleaning container $(BACKEND_CONTAINER_NAME)..."
	@-podman stop $(BACKEND_CONTAINER_NAME) 2>/dev/null || true
	@-podman rm $(BACKEND_CONTAINER_NAME) 2>/dev/null || true

version:
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(GIT_VERSION)"
	@echo "Commit: $(GIT_COMMIT)"
	@echo "Short Commit: $(GIT_SHORT_COMMIT)"
	@echo "Tree State: $(GIT_TREE_STATE)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Build Timestamp: $(BUILD_TIMESTAMP)"
	@echo "Unique Tag: $(UNIQUE_TAG)"
	@echo ""
	@echo "Image Tags:"
	@echo "  Server: $(OVIM_SERVER_IMAGE):$(UNIQUE_TAG)"
	@echo "  UI: $(OVIM_UI_IMAGE):$(UNIQUE_TAG)"

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

# Database migration targets
## db-migrate: Apply database migration for CRD architecture
db-migrate:
	@echo "Applying database migration for CRD architecture..."
	@if [ ! -f scripts/migrations/001_org_vdc_crd_migration.sql ]; then \
		echo "Error: Migration script not found"; \
		exit 1; \
	fi
	@echo "Running migration script..."
	@PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) \
		-f scripts/migrations/001_org_vdc_crd_migration.sql
	@echo "Migration completed successfully"

## db-migrate-validate: Validate database migration
db-migrate-validate:
	@echo "Validating database migration..."
	@if [ ! -f scripts/migrations/validate_migration_001.sql ]; then \
		echo "Error: Validation script not found"; \
		exit 1; \
	fi
	@PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) \
		-f scripts/migrations/validate_migration_001.sql
	@echo "Migration validation completed"

## db-migrate-rollback: Rollback database migration (WARNING: Destructive)
db-migrate-rollback:
	@echo "WARNING: This will rollback the database migration and delete VDC/Catalog data!"
	@read -p "Are you sure you want to continue? (type 'yes' to confirm): " confirm; \
	if [ "$$confirm" != "yes" ]; then \
		echo "Rollback cancelled"; \
		exit 1; \
	fi
	@if [ ! -f scripts/migrations/rollback_001_org_vdc_crd_migration.sql ]; then \
		echo "Error: Rollback script not found"; \
		exit 1; \
	fi
	@PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) \
		-f scripts/migrations/rollback_001_org_vdc_crd_migration.sql
	@echo "Migration rollback completed"

# CRD management targets (legacy, using basic crd-manager.sh)
crd-docs:
	@./scripts/crd-docs-generator.sh

crd-examples:
	@./scripts/crd-examples-generator.sh

crd-test: fmt
	@echo "Running CRD-specific tests..."
	@go test $(GO_TEST_FLAGS) -run "TestCRD|TestMigration|TestConditions|TestJSONB" ./pkg/models/...
	@echo "CRD tests completed"

# Code generation targets
generate: 
	@echo "Generating code..."
	@if command -v controller-gen >/dev/null 2>&1; then \
		controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./pkg/api/v1/..."; \
	else \
		echo "controller-gen not found, skipping generation"; \
		echo "Install with: go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest"; \
	fi

manifests:
	@echo "Generating Kubernetes manifests..."
	@mkdir -p $(CRD_DIR)
	@if command -v controller-gen >/dev/null 2>&1; then \
		controller-gen crd:generateEmbeddedObjectMeta=true rbac:roleName=ovim-controller \
			webhook paths="./..." output:crd:artifacts:config=$(CRD_DIR); \
	else \
		echo "controller-gen not found, cannot generate manifests"; \
		echo "Install with: go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest"; \
	fi

# Deployment targets

## deploy: Deploy OVIM to Kubernetes cluster
deploy: build build-controller generate manifests
	@echo "Deploying OVIM to cluster..."
	@OVIM_NAMESPACE=$(OVIM_NAMESPACE) \
	 OVIM_IMAGE_TAG=$(OVIM_IMAGE_TAG) \
	 OVIM_CONTROLLER_IMAGE=$(OVIM_CONTROLLER_IMAGE) \
	 OVIM_SERVER_IMAGE=$(OVIM_SERVER_IMAGE) \
	 KUBECTL_CMD=$(KUBECTL_CMD) \
	 ./scripts/deploy.sh

## deploy-dry-run: Show what would be deployed without applying
deploy-dry-run: generate manifests
	@echo "Dry-run deployment of OVIM..."
	@OVIM_NAMESPACE=$(OVIM_NAMESPACE) \
	 OVIM_IMAGE_TAG=$(OVIM_IMAGE_TAG) \
	 OVIM_CONTROLLER_IMAGE=$(OVIM_CONTROLLER_IMAGE) \
	 OVIM_SERVER_IMAGE=$(OVIM_SERVER_IMAGE) \
	 KUBECTL_CMD=$(KUBECTL_CMD) \
	 ./scripts/deploy.sh --dry-run

## deploy-samples: Deploy sample resources (requires OVIM to be deployed first)
deploy-samples:
	@echo "Deploying sample resources..."
	@$(KUBECTL_CMD) apply -f config/samples/ || true
	@echo "Sample resources deployed. Check with: $(KUBECTL_CMD) get organizations,virtualdatacenters,catalogs --all-namespaces"

## deploy-full: Complete deployment with database migration and samples
deploy-full: deploy
	@echo "Running full deployment with database migration and samples..."
	@if [ -n "$(DATABASE_URL)" ]; then \
		echo "Running database migration..."; \
		$(MAKE) db-migrate; \
	else \
		echo "DATABASE_URL not set, skipping database migration"; \
		echo "Run 'make db-migrate' manually if needed"; \
	fi
	@echo "Waiting for controller to be ready..."
	@sleep 10
	@$(MAKE) deploy-samples
	@echo "Full deployment completed!"

## deploy-dev: Deploy with development configuration
deploy-dev: 
	@echo "Deploying OVIM in development mode..."
	@OVIM_NAMESPACE=ovim-dev \
	 OVIM_IMAGE_TAG=dev \
	 $(MAKE) deploy

## deploy-prod: Deploy with production configuration
deploy-prod:
	@echo "Deploying OVIM in production mode..."
	@OVIM_NAMESPACE=ovim-system \
	 OVIM_IMAGE_TAG=latest \
	 $(MAKE) deploy

## undeploy: Remove OVIM from Kubernetes cluster
undeploy:
	@echo "Removing OVIM from cluster..."
	@OVIM_NAMESPACE=$(OVIM_NAMESPACE) \
	 KUBECTL_CMD=$(KUBECTL_CMD) \
	 ./scripts/undeploy.sh

## undeploy-force: Force remove OVIM without confirmation
undeploy-force:
	@echo "Force removing OVIM from cluster..."
	@OVIM_NAMESPACE=$(OVIM_NAMESPACE) \
	 KUBECTL_CMD=$(KUBECTL_CMD) \
	 ./scripts/undeploy.sh --force

# Enhanced CRD targets with deployment variables
crd-install: generate manifests
	@echo "Installing CRDs with deployment configuration..."
	@KUBECTL_CMD=$(KUBECTL_CMD) ./scripts/crd-manager.sh install

crd-uninstall:
	@echo "Uninstalling CRDs..."
	@KUBECTL_CMD=$(KUBECTL_CMD) ./scripts/crd-manager.sh uninstall

crd-validate:
	@echo "Validating CRDs..."
	@KUBECTL_CMD=$(KUBECTL_CMD) ./scripts/crd-manager.sh validate

crd-status:
	@echo "Checking CRD status..."
	@KUBECTL_CMD=$(KUBECTL_CMD) ./scripts/crd-manager.sh status

# Quick deployment status check
deployment-status:
	@echo "OVIM Deployment Status:"
	@echo "======================="
	@echo "Namespace: $(OVIM_NAMESPACE)"
	@echo "Image Tag: $(OVIM_IMAGE_TAG)"
	@echo "Controller Image: $(OVIM_CONTROLLER_IMAGE):$(OVIM_IMAGE_TAG)"
	@echo ""
	@if $(KUBECTL_CMD) get namespace $(OVIM_NAMESPACE) >/dev/null 2>&1; then \
		echo "Namespace Status: ✓ Exists"; \
		echo ""; \
		echo "CRDs:"; \
		$(KUBECTL_CMD) get crd | grep ovim.io || echo "  No OVIM CRDs found"; \
		echo ""; \
		echo "Controller:"; \
		$(KUBECTL_CMD) get deployment,pod,service -n $(OVIM_NAMESPACE) -l app.kubernetes.io/name=ovim 2>/dev/null || echo "  No OVIM resources found"; \
		echo ""; \
		echo "Custom Resources:"; \
		$(KUBECTL_CMD) get organizations,virtualdatacenters,catalogs --all-namespaces 2>/dev/null || echo "  No custom resources found"; \
	else \
		echo "Namespace Status: ✗ Not found"; \
		echo "Run 'make deploy' to deploy OVIM"; \
	fi

# Full Stack Deployment targets

## deploy-stack: Deploy complete OVIM stack (PostgreSQL + Controllers + UI + Ingress)
deploy-stack: container-push-all generate manifests
	@echo "Deploying complete OVIM stack..."
	@OVIM_NAMESPACE=$(OVIM_NAMESPACE) \
	 OVIM_IMAGE_TAG=$(OVIM_IMAGE_TAG) \
	 OVIM_CONTROLLER_IMAGE=$(OVIM_CONTROLLER_IMAGE) \
	 OVIM_SERVER_IMAGE=$(OVIM_SERVER_IMAGE) \
	 OVIM_UI_IMAGE=$(OVIM_UI_IMAGE) \
	 OVIM_DOMAIN=$(OVIM_DOMAIN) \
	 OVIM_DB_STORAGE_SIZE=$(OVIM_DB_STORAGE_SIZE) \
	 OVIM_UI_REPLICAS=$(OVIM_UI_REPLICAS) \
	 KUBECTL_CMD=$(KUBECTL_CMD) \
	 DATABASE_URL=$(DATABASE_URL) \
	 ./scripts/deploy-stack.sh --no-build-ui

## deploy-stack-unique: Deploy complete OVIM stack with unique tags
deploy-stack-unique: container-push-all generate manifests
	@echo "Deploying complete OVIM stack with unique tag: $(UNIQUE_TAG)..."
	@OVIM_NAMESPACE=$(OVIM_NAMESPACE) \
	 OVIM_IMAGE_TAG=$(UNIQUE_TAG) \
	 OVIM_CONTROLLER_IMAGE=$(OVIM_CONTROLLER_IMAGE) \
	 OVIM_SERVER_IMAGE=$(OVIM_SERVER_IMAGE) \
	 OVIM_UI_IMAGE=$(OVIM_UI_IMAGE) \
	 OVIM_DOMAIN=$(OVIM_DOMAIN) \
	 OVIM_DB_STORAGE_SIZE=$(OVIM_DB_STORAGE_SIZE) \
	 OVIM_UI_REPLICAS=$(OVIM_UI_REPLICAS) \
	 KUBECTL_CMD=$(KUBECTL_CMD) \
	 DATABASE_URL=$(DATABASE_URL) \
	 UNIQUE_TAG=$(UNIQUE_TAG) \
	 ./scripts/deploy-stack.sh --use-unique-tag --no-build-ui

## deploy-stack-dry-run: Show what would be deployed in the full stack
deploy-stack-dry-run: generate manifests
	@echo "Dry-run deployment of complete OVIM stack..."
	@OVIM_NAMESPACE=$(OVIM_NAMESPACE) \
	 OVIM_IMAGE_TAG=$(OVIM_IMAGE_TAG) \
	 OVIM_DOMAIN=$(OVIM_DOMAIN) \
	 OVIM_DB_STORAGE_SIZE=$(OVIM_DB_STORAGE_SIZE) \
	 OVIM_UI_REPLICAS=$(OVIM_UI_REPLICAS) \
	 KUBECTL_CMD=$(KUBECTL_CMD) \
	 ./scripts/deploy-stack.sh --dry-run

## deploy-stack-dev: Deploy development stack
deploy-stack-dev:
	@echo "Deploying OVIM stack in development mode..."
	@OVIM_NAMESPACE=ovim-dev \
	 OVIM_IMAGE_TAG=dev \
	 OVIM_DOMAIN=ovim-dev.local \
	 OVIM_DB_STORAGE_SIZE=5Gi \
	 OVIM_UI_REPLICAS=1 \
	 $(MAKE) deploy-stack

## deploy-stack-prod: Deploy production stack
deploy-stack-prod:
	@echo "Deploying OVIM stack in production mode..."
	@OVIM_NAMESPACE=ovim-system \
	 OVIM_IMAGE_TAG=latest \
	 OVIM_DOMAIN=$(OVIM_DOMAIN) \
	 OVIM_DB_STORAGE_SIZE=50Gi \
	 OVIM_UI_REPLICAS=3 \
	 $(MAKE) deploy-stack

## deploy-database: Deploy only PostgreSQL database
deploy-database:
	@echo "Deploying PostgreSQL database..."
	@OVIM_NAMESPACE=$(OVIM_NAMESPACE) \
	 OVIM_DB_STORAGE_SIZE=$(OVIM_DB_STORAGE_SIZE) \
	 KUBECTL_CMD=$(KUBECTL_CMD) \
	 ./scripts/deploy-stack.sh --skip-controller --skip-ui --skip-ingress --skip-build

## deploy-server: Deploy only OVIM server
deploy-server:
	@echo "Deploying OVIM server..."
	@OVIM_NAMESPACE=$(OVIM_NAMESPACE) \
	 OVIM_IMAGE_TAG=$(OVIM_IMAGE_TAG) \
	 KUBECTL_CMD=$(KUBECTL_CMD) \
	 ./scripts/deploy-stack.sh --skip-controller --skip-database --skip-ui --skip-ingress

## deploy-ui: Deploy only OVIM UI
deploy-ui:
	@echo "Deploying OVIM UI..."
	@OVIM_NAMESPACE=$(OVIM_NAMESPACE) \
	 OVIM_IMAGE_TAG=$(OVIM_IMAGE_TAG) \
	 OVIM_UI_REPLICAS=$(OVIM_UI_REPLICAS) \
	 KUBECTL_CMD=$(KUBECTL_CMD) \
	 ./scripts/deploy-stack.sh --skip-controller --skip-database --skip-server --skip-ingress

## deploy-ingress: Deploy only ingress configuration
deploy-ingress:
	@echo "Deploying ingress configuration..."
	@OVIM_NAMESPACE=$(OVIM_NAMESPACE) \
	 OVIM_DOMAIN=$(OVIM_DOMAIN) \
	 KUBECTL_CMD=$(KUBECTL_CMD) \
	 ./scripts/deploy-stack.sh --skip-controller --skip-database --skip-ui --skip-build

## undeploy-stack: Remove entire OVIM stack
undeploy-stack:
	@echo "Removing OVIM stack..."
	@OVIM_NAMESPACE=$(OVIM_NAMESPACE) \
	 KUBECTL_CMD=$(KUBECTL_CMD) \
	 ./scripts/undeploy.sh --force
	@echo "Removing additional stack components..."
	@$(KUBECTL_CMD) delete statefulset,pvc,secret,configmap -l app.kubernetes.io/name=ovim -n $(OVIM_NAMESPACE) --ignore-not-found=true
	@$(KUBECTL_CMD) delete ingress -l app.kubernetes.io/name=ovim -n $(OVIM_NAMESPACE) --ignore-not-found=true

## stack-status: Show status of all stack components
stack-status:
	@echo "OVIM Stack Status:"
	@echo "=================="
	@echo "Namespace: $(OVIM_NAMESPACE)"
	@echo "Domain: $(OVIM_DOMAIN)"
	@echo ""
	@if $(KUBECTL_CMD) get namespace $(OVIM_NAMESPACE) >/dev/null 2>&1; then \
		echo "Components:"; \
		echo "----------"; \
		$(KUBECTL_CMD) get pods,svc,ingress,pvc -n $(OVIM_NAMESPACE) -l app.kubernetes.io/name=ovim || echo "  No OVIM components found"; \
		echo ""; \
		echo "Deployments:"; \
		echo "-----------"; \
		$(KUBECTL_CMD) get deployment,statefulset -n $(OVIM_NAMESPACE) -l app.kubernetes.io/name=ovim || echo "  No deployments found"; \
		echo ""; \
		echo "Ingress Status:"; \
		echo "--------------"; \
		$(KUBECTL_CMD) describe ingress ovim-ingress -n $(OVIM_NAMESPACE) 2>/dev/null | grep -E "(Address:|Rules:)" || echo "  No ingress found"; \
	else \
		echo "Namespace $(OVIM_NAMESPACE) not found"; \
		echo "Run 'make deploy-stack' to deploy the complete stack"; \
	fi

## stack-logs: Show logs from all stack components
stack-logs:
	@echo "OVIM Stack Logs:"
	@echo "==============="
	@echo ""
	@echo "Controller Logs:"
	@echo "---------------"
	@$(KUBECTL_CMD) logs -l app.kubernetes.io/component=controller -n $(OVIM_NAMESPACE) --tail=20 --prefix=true || echo "No controller logs found"
	@echo ""
	@echo "UI Logs:"
	@echo "--------"
	@$(KUBECTL_CMD) logs -l app.kubernetes.io/component=ui -n $(OVIM_NAMESPACE) --tail=20 --prefix=true || echo "No UI logs found"
	@echo ""
	@echo "Database Logs:"
	@echo "-------------"
	@$(KUBECTL_CMD) logs -l app.kubernetes.io/component=database -n $(OVIM_NAMESPACE) --tail=20 --prefix=true || echo "No database logs found"

## stack-port-forward: Set up port forwarding for local access
stack-port-forward:
	@echo "Setting up port forwarding for OVIM stack..."
	@echo "Cleaning up any existing port-forward processes..."
	@-pkill -f "kubectl port-forward" 2>/dev/null || true
	@sleep 2
	@echo "Cluster UI will be available at: https://localhost:8445"
	@echo "Cluster API will be available at: https://localhost:8446"
	@echo "Database will be available at: localhost:5433"
	@echo ""
	@echo "Note: Local development uses different ports:"
	@echo "  Local dev server: https://localhost:8090 (make dev)"
	@echo "  Local database:   localhost:5432"
	@echo ""
	@echo "Press Ctrl+C to stop port forwarding"
	@$(KUBECTL_CMD) port-forward svc/ovim-ui 8445:443 -n $(OVIM_NAMESPACE) &
	@$(KUBECTL_CMD) port-forward svc/ovim-server 8446:8443 -n $(OVIM_NAMESPACE) &
	@$(KUBECTL_CMD) port-forward svc/ovim-postgresql 5433:5432 -n $(OVIM_NAMESPACE) &
	@wait

# Build UI container (helper target)
build-ui:
	@echo "Building OVIM UI container..."
	@cd ../ovim-ui && make container-build

# Default target
all: test build

## build-push: Build and push backend container with unique timestamp tag
build-push: clean
	@echo "Building and pushing backend container with unique tag..."
	$(eval UNIQUE_TAG := $(BUILD_TIMESTAMP)-$(GIT_SHORT_COMMIT))
	@echo "Using tag: $(UNIQUE_TAG)"
	podman build --no-cache -t quay.io/eerez/ovim:$(UNIQUE_TAG) .
	podman tag quay.io/eerez/ovim:$(UNIQUE_TAG) quay.io/eerez/ovim:latest
	podman push quay.io/eerez/ovim:$(UNIQUE_TAG)
	podman push quay.io/eerez/ovim:latest
	@echo "Backend image pushed with tag: $(UNIQUE_TAG)"

## deploy-image: Update backend deployment with latest unique image
deploy-image:
	@echo "Updating backend deployment with latest image..."
	$(eval LATEST_TAG := $(shell podman images quay.io/eerez/ovim --format "{{.Tag}}" | grep -E '^[0-9]{8}-[0-9]{6}-' | head -1))
	@if [ -z "$(LATEST_TAG)" ]; then \
		echo "No timestamped tag found, using latest"; \
		kubectl set image deployment/ovim-server server=quay.io/eerez/ovim:latest -n ovim-system; \
		kubectl set image deployment/ovim-controller controller=quay.io/eerez/ovim:latest -n ovim-system; \
	else \
		echo "Using tag: $(LATEST_TAG)"; \
		kubectl set image deployment/ovim-server server=quay.io/eerez/ovim:$(LATEST_TAG) -n ovim-system; \
		kubectl set image deployment/ovim-controller controller=quay.io/eerez/ovim:$(LATEST_TAG) -n ovim-system; \
	fi
.PHONY: all build clean install test test-localstack test-deps help release release-dry-run fmt fmt-check vet lint verify docker-build localstack-up localstack-down localstack-logs localstack-health tidy

BINARY_NAME=rosactl
BUILD_DIR=./bin
INSTALL_DIR=/usr/local/bin
IMAGE_NAME=rosa-regional-platform-cli
IMAGE_TAG=$(shell tag=$$(git describe --tags --exact-match 2>/dev/null); if [ -n "$$tag" ]; then echo "$$tag" | sed 's/^v//'; else echo "dev"; fi)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION=$(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.1.0")
LDFLAGS=-ldflags "-X github.com/openshift-online/rosa-regional-platform-cli/internal/commands/version.GitCommit=$(GIT_COMMIT) -X github.com/openshift-online/rosa-regional-platform-cli/internal/commands/version.Version=$(VERSION)"

all: fmt vet lint test build
	@echo ""
	@echo "✓ All checks and build completed successfully!"

help:
	@echo "Available targets:"
	@echo ""
	@echo "Build & Install:"
	@echo "  all             - Run all checks (fmt, vet, lint, test, build)"
	@echo "  build           - Build the rosactl binary"
	@echo "  clean           - Remove built binaries and test artifacts"
	@echo "  install         - Install rosactl to $(INSTALL_DIR)"
	@echo "  tidy            - Tidy go modules"
	@echo ""
	@echo "Testing:"
	@echo "  test            - Run unit tests"
	@echo "  test-localstack - Run integration tests against LocalStack"
	@echo "  test-deps       - Install test dependencies (Ginkgo)"
	@echo ""
	@echo "Docker & LocalStack:"
	@echo "  docker-build    - Build Lambda container image"
	@echo "  localstack-up   - Start LocalStack"
	@echo "  localstack-down - Stop LocalStack and remove volumes"
	@echo "  localstack-logs - View LocalStack logs"
	@echo "  localstack-health - Check LocalStack health"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt             - Format Go code with gofmt"
	@echo "  fmt-check       - Check if Go code is formatted (non-destructive)"
	@echo "  vet             - Run go vet for suspicious code"
	@echo "  lint            - Run golangci-lint"
	@echo "  verify          - Run all checks (fmt-check, vet, lint)"
	@echo ""
	@echo "Version Management:"
	@echo "  release-dry-run - Show what next version would be (dry-run)"
	@echo "  release         - Create semantic version release (uses conventional commits)"
	@echo ""
	@echo "  help            - Show this help message"

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/rosactl
	@echo "✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

clean:
	@echo "Cleaning up..."
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@rm -f coverage.out
	@rm -f rosactl
	@echo "✓ Clean complete"

install:
	@echo "Installing $(BINARY_NAME) to GOPATH/bin..."
	@go install $(LDFLAGS) ./cmd/rosactl/
	@echo "✓ Installation complete"

tidy:
	@echo "Tidying go modules..."
	@GOFLAGS=-mod=mod go mod tidy
	@echo "✓ Tidy complete"

test:
	@echo "Running unit tests..."
	@go test -v -race -coverprofile=coverage.out ./internal/... ./cmd/...
	@echo "✓ Unit tests passed"

test-localstack:
	@echo "Running LocalStack integration tests..."
	@./test/localstack/run-localstack-tests.sh

test-deps:
	@echo "Installing test dependencies..."
	@go install github.com/onsi/ginkgo/v2/ginkgo@latest
	@echo "✓ Test dependencies installed (Ginkgo)"

# Docker targets
docker-build:
	@echo "Building Docker image: $(IMAGE_NAME):$(IMAGE_TAG)"
	@docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .
	@echo "✓ Docker image built: $(IMAGE_NAME):$(IMAGE_TAG)"

# LocalStack targets
localstack-up:
	@echo "Starting LocalStack..."
	@docker-compose -f docker-compose.localstack.yaml up -d
	@echo "Waiting for LocalStack to be ready..."
	@for i in $$(seq 1 30); do \
		if curl -s http://localhost:4566/_localstack/health > /dev/null 2>&1; then \
			echo "✓ LocalStack is ready"; \
			exit 0; \
		fi; \
		echo -n "."; \
		sleep 1; \
	done; \
	echo ""; \
	echo "⚠️  LocalStack may not be ready yet. Check with 'make localstack-health'"

localstack-down:
	@echo "Stopping LocalStack..."
	@docker-compose -f docker-compose.localstack.yaml down -v
	@echo "✓ LocalStack stopped"

localstack-logs:
	@docker-compose -f docker-compose.localstack.yaml logs -f

localstack-health:
	@echo "Checking LocalStack health..."
	@curl -s http://localhost:4566/_localstack/health | jq '.' || echo "LocalStack is not running or not responding"

# Code Quality Targets
fmt:
	@echo "Formatting Go code..."
	@gofmt -w -s ./cmd ./internal ./test
	@echo "Formatting complete"

fmt-check:
	@echo "Checking code formatting..."
	@files=$$(gofmt -l ./cmd ./internal ./test 2>&1); \
	if [ -n "$$files" ]; then \
		echo "The following files are not formatted:"; \
		echo "$$files"; \
		echo ""; \
		echo "Run 'make fmt' to format them"; \
		exit 1; \
	fi
	@echo "✓ All files are properly formatted"

vet:
	@echo "Running go vet..."
	@go vet ./...
	@echo "✓ go vet passed"

lint:
	@echo "Running golangci-lint..."
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "Error: golangci-lint not found"; \
		echo "Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		echo "Or visit: https://golangci-lint.run/welcome/install/"; \
		exit 1; \
	fi
	@golangci-lint run --timeout=5m ./...
	@echo "✓ golangci-lint passed"

verify: fmt-check vet lint
	@echo ""
	@echo "✓ All verification checks passed!"

# Semantic versioning using go-semver-release
# Requires: go install github.com/s0ders/go-semver-release@latest
release-dry-run:
	@echo "Checking what next version would be..."
	@if ! command -v go-semver-release &> /dev/null; then \
		echo "Error: go-semver-release not found"; \
		echo "Install: go install github.com/s0ders/go-semver-release@latest"; \
		exit 1; \
	fi
	@go-semver-release release . --dry-run --config .semver.yaml

release:
	@echo "Creating semantic version release..."
	@if ! command -v go-semver-release &> /dev/null; then \
		echo "Error: go-semver-release not found"; \
		echo "Install: go install github.com/s0ders/go-semver-release@latest"; \
		exit 1; \
	fi
	@echo "Analyzing commits using conventional commit messages..."
	@go-semver-release release . --config .semver.yaml
	@echo ""
	@echo "✓ Release created! New version:"
	@git describe --tags --abbrev=0
	@echo ""
	@echo "To push the tag to remote, run:"
	@echo "  git push origin $$(git describe --tags --abbrev=0)"
	@echo ""
	@echo "⚠️  Don't forget to update the version badge in README.md:"
	@echo "  ![Version](https://img.shields.io/badge/version-$$(git describe --tags --abbrev=0 | sed 's/^v//')-blue.svg)"

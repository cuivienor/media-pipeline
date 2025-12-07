.PHONY: build build-local build-all build-mock-makemkv build-ripper deploy run-remote clean test test-contracts test-e2e test-all fmt vet deploy-dev dev

# Build for Linux (production target)
build:
	GOOS=linux GOARCH=amd64 go build -o bin/media-pipeline ./cmd/media-pipeline

# Build for local machine (development)
build-local:
	go build -o bin/media-pipeline ./cmd/media-pipeline

# Build mock-makemkv (for testing)
build-mock-makemkv:
	go build -o bin/mock-makemkv ./cmd/mock-makemkv

# Build ripper CLI
build-ripper:
	go build -o bin/ripper ./cmd/ripper

# Build all binaries for local development
build-all: build-local build-mock-makemkv build-ripper

# Deploy to analyzer container
deploy: build
	scp bin/media-pipeline analyzer:/home/media/bin/

# Run on analyzer container (interactive)
run-remote:
	ssh -t analyzer '/home/media/bin/media-pipeline'

# Build, deploy, and run in one command
run: deploy run-remote

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test ./...

# Run contract tests (validates bash scripts produce scanner-compatible state)
test-contracts: bin/validate-state
	./test/test-contracts.sh

# Build state validator
bin/validate-state:
	go build -o bin/validate-state ./test/validate-state

# Run E2E tests (requires mock-makemkv and ripper)
test-e2e: build-mock-makemkv build-ripper
	go test ./tests/e2e/... -v

# Run all tests
test-all: test test-contracts test-e2e

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Test container targets
TEST_HOST := pipeline-test
DEV_BIN := /home/media/bin/dev

# Build all for Linux and deploy to test container dev directory
deploy-dev:
	GOOS=linux GOARCH=amd64 go build -o bin/media-pipeline ./cmd/media-pipeline
	GOOS=linux GOARCH=amd64 go build -o bin/ripper ./cmd/ripper
	GOOS=linux GOARCH=amd64 go build -o bin/mock-makemkv ./cmd/mock-makemkv
	ssh $(TEST_HOST) 'mkdir -p $(DEV_BIN) && rm -f $(DEV_BIN)/media-pipeline $(DEV_BIN)/ripper $(DEV_BIN)/mock-makemkv'
	scp bin/media-pipeline bin/ripper bin/mock-makemkv $(TEST_HOST):$(DEV_BIN)/

# Deploy and run TUI interactively on test container
dev: deploy-dev
	ssh -t $(TEST_HOST) 'PATH=$(DEV_BIN):$$PATH media-pipeline'

.PHONY: build build-local deploy run-remote clean

# Build for Linux (production target)
build:
	GOOS=linux GOARCH=amd64 go build -o bin/media-pipeline ./cmd/media-pipeline

# Build for local machine (development)
build-local:
	go build -o bin/media-pipeline ./cmd/media-pipeline

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

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

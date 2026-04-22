.PHONY: run build lint tidy

## run: Start the development server (loads .env automatically)
run:
	go run ./cmd/api/

## build: Compile the binary to bin/api
build:
	go build -o bin/api ./cmd/api/

## tidy: Tidy and verify Go module dependencies
tidy:
	go mod tidy
	go mod verify

## lint: Run golangci-lint (must be installed separately)
lint:
	golangci-lint run ./...

## help: Print this help message
help:
	@echo "Available targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST)

BINARY_NAME=hoptag
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_DATE?=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-X github.com/hoopoelabs/hoptag/cmd.Version=$(VERSION) -X github.com/hoopoelabs/hoptag/cmd.Commit=$(COMMIT) -X github.com/hoopoelabs/hoptag/cmd.BuildDate=$(BUILD_DATE)"

.PHONY: all build test clean fmt vet install run-fixtures

all: test build

build:
	@echo "==> Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) .

install:
	@echo "==> Installing $(BINARY_NAME) to \$$GOPATH/bin..."
	go install $(LDFLAGS) .

test:
	@echo "==> Running tests..."
	go test -v -race ./...

test-coverage:
	@echo "==> Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

fmt:
	@echo "==> Formatting code..."
	go fmt ./...

vet:
	@echo "==> Vetting code..."
	go vet ./...

clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf bin/ coverage.out
	find . -type f -name "_hoptag_override.tf" -delete
	find . -type f -name "*.hoptag.tf" -delete
	find . -type f -name "*.bak" -delete

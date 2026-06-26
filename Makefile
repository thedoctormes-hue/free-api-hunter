.PHONY: build test lint docker run clean

VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -X main.Version=$(VERSION) -s -w
BINARY  := hunter
PKG     := ./cmd/hunter

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

test:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint:
	which golangci-lint || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./...

docker:
	docker build -t free-api-hunter:$(VERSION) .

run:
	./$(BINARY) --api :8090

clean:
	rm -f $(BINARY) coverage.out
	go clean -cache

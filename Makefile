BINARY_CLI = ryobi
BINARY_SERVER = ryobid
VERSION ?= 0.1.0-dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
CHANNEL ?= dev

GOFLAGS = -ldflags "-X github.com/ryobi-project/ryobi/pkg/version.Version=$(VERSION) \
	-X github.com/ryobi-project/ryobi/pkg/version.Commit=$(COMMIT) \
	-X github.com/ryobi-project/ryobi/pkg/version.Channel=$(CHANNEL)"

DIST_DIR = dist

.PHONY: all build build-cli build-server test lint clean install

all: build

build: build-cli build-server

build-cli:
	@echo "Building $(BINARY_CLI)..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 go build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_CLI) ./cmd/ryobi/

build-server:
	@echo "Building $(BINARY_SERVER)..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 go build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_SERVER) ./cmd/ryobid/

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(DIST_DIR)

install: build
	cp $(DIST_DIR)/$(BINARY_CLI) /usr/local/bin/$(BINARY_CLI)
	@echo "Installed $(BINARY_CLI) to /usr/local/bin/"

BINARY_CLI = ryobi
BINARY_SERVER = ryobid
BINARY_ENV = ryobi-env
VERSION ?= 0.1.0-dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
CHANNEL ?= dev

GOFLAGS = -ldflags "-X github.com/ryobi-project/ryobi/pkg/version.Version=$(VERSION) \
	-X github.com/ryobi-project/ryobi/pkg/version.Commit=$(COMMIT) \
	-X github.com/ryobi-project/ryobi/pkg/version.Channel=$(CHANNEL)"

DIST_DIR = dist

IMAGE_REGISTRY ?= ghcr.io/ryobi-project
IMAGE_TAG ?= $(VERSION)

.PHONY: all build build-cli build-server build-env build-ui test lint clean install proto image-env image-server

all: build

build: build-ui build-cli build-server build-env

build-ui:
	@echo "Building Portal UI..."
	cd ui/portal && npm run build
	@echo "Building Admin UI..."
	cd ui/admin && npm run build

build-cli:
	@echo "Building $(BINARY_CLI)..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 go build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_CLI) ./cmd/ryobi/

build-server:
	@echo "Building $(BINARY_SERVER)..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 go build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_SERVER) ./cmd/ryobid/

build-env:
	@echo "Building $(BINARY_ENV)..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 go build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_ENV) ./cmd/ryobi-env/

proto:
	protoc --go_out=. --go-grpc_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		proto/environment.proto
	mv proto/environment.pb.go pkg/grpcapi/
	mv proto/environment_grpc.pb.go pkg/grpcapi/

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(DIST_DIR)

install: build
	cp $(DIST_DIR)/$(BINARY_CLI) /usr/local/bin/$(BINARY_CLI)
	cp $(DIST_DIR)/$(BINARY_ENV) /usr/local/bin/$(BINARY_ENV)
	@echo "Installed $(BINARY_CLI) and $(BINARY_ENV) to /usr/local/bin/"

image-env:
	podman build -t $(IMAGE_REGISTRY)/ryobi-env:$(IMAGE_TAG) \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) \
		-f deploy/images/ryobi-env/Containerfile .

image-server:
	podman build -t $(IMAGE_REGISTRY)/ryobid:$(IMAGE_TAG) \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) \
		-f deploy/Containerfile .

images: image-env image-server

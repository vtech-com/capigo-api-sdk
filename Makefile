MODULE := github.com/vtech-com/capigo-api-sdk
VERSION_PKG := $(MODULE)/internal/version

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -ldflags "\
  -X $(VERSION_PKG).Version=$(VERSION) \
  -X $(VERSION_PKG).Commit=$(COMMIT) \
  -X $(VERSION_PKG).Date=$(DATE)"

DIST_DIR := dist
BINARY   := $(DIST_DIR)/capigo

.PHONY: build test lint release-snapshot install clean update-spec

## update-spec: Fetch latest OpenAPI spec from Capigo platform
update-spec:
	curl -fsSL https://platform.capigo.app/api/openapi -o api/openapi.json
	@echo "Updated api/openapi.json from https://platform.capigo.app/api/openapi"

build:
	@mkdir -p $(DIST_DIR)
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test ./... -count=1 -race -coverprofile=coverage.out

lint:
	golangci-lint run ./...

release-snapshot:
	goreleaser release --snapshot --clean

install:
	go install $(LDFLAGS) .

clean:
	rm -rf $(DIST_DIR) coverage.out

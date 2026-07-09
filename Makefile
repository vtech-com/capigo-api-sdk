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
SKILL_ZIP := $(DIST_DIR)/capigo-api-skill.zip

# Tấm openclaw host — override on the command line if the SSH alias/path differ,
# e.g. `make skill-install-tam TAM_HOST=other-host`.
TAM_HOST       ?= vtech:tam
TAM_SKILLS_DIR ?= ~/.openclaw/plugin-skills

.PHONY: build test lint release-snapshot install clean update-spec verify-api skill-package skill-install-tam

## update-spec: Fetch latest OpenAPI spec from Capigo platform
## Prod serves minified JSON on one line; pretty-print it to 2-space indent so
## the committed spec stays reviewable and successive runs produce stable diffs.
update-spec:
	curl -fsSL https://platform.capigo.app/api/openapi | \
		python3 -c "import json,sys; json.dump(json.load(sys.stdin), open('api/openapi.json','w'), indent=2, ensure_ascii=False); open('api/openapi.json','a').write(chr(10))"
	@echo "Updated api/openapi.json from https://platform.capigo.app/api/openapi (pretty-printed)"

## verify-api: Call a running API and compare its answers with what we claim
## Every other guard in this repo checks the CLI against api/openapi.json. This one
## checks the document — and the help pages — against the server, which is the only
## party that has never lied. Read-only: it makes no writes and creates nothing.
##   CAPIGO_API_URL=http://127.0.0.1:3999/api/v1 CAPIGO_API_KEY=csk_... \
##   CAPIGO_TENANT=acme-corp make verify-api
## Builds the binary first, so each help page's OUTPUT sample is checked to name
## every field the API actually returns.
verify-api: build
	@python3 scripts/verify_api.py --cli $(BINARY)

## skill-package: Zip the bundled agent skill for distribution (openclaw / other hosts)
skill-package:
	@mkdir -p $(DIST_DIR)
	@rm -f $(SKILL_ZIP)
	cd skills && zip -r ../$(SKILL_ZIP) capigo-api -x '*.DS_Store'
	@echo "Packaged skill at $(SKILL_ZIP)"

## skill-install-tam: Package and install the skill onto the Tấm openclaw host (idempotent)
skill-install-tam: skill-package
	ssh $(TAM_HOST) 'cat > /tmp/capigo-api-skill.zip' < $(SKILL_ZIP)
	ssh $(TAM_HOST) 'rm -rf $(TAM_SKILLS_DIR)/capigo-api && mkdir -p $(TAM_SKILLS_DIR) && unzip -oq /tmp/capigo-api-skill.zip -d $(TAM_SKILLS_DIR) && rm -f /tmp/capigo-api-skill.zip'
	@echo "Installed capigo-api skill to $(TAM_HOST):$(TAM_SKILLS_DIR)/capigo-api"

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

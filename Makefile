.PHONY: build build-server build-agent dev-server dev-web test lint lint-web typecheck-web proto package clean migrate-create migrate-up migrate-down frontend

GOCMD=go
GOFLAGS=-trimpath
LDFLAGS=-s -w

# Version info
DG_VERSION := $(shell date +'%Y.%-m.%-d')_$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DG_BUILD_DATE := $(shell date +'%Y-%m-%dT%H:%M:%S')
DG_GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION_LDFLAGS := -X github.com/michael/device_grid/internal/version.Version=$(DG_VERSION) -X github.com/michael/device_grid/internal/version.BuildDate=$(DG_BUILD_DATE) -X github.com/michael/device_grid/internal/version.GitCommit=$(DG_GIT_COMMIT)

SERVER_BIN=bin/devicegrid-server
AGENT_BIN=bin/devicegrid-agent
WEB_DIR=web

# ============================================================
# Build
# ============================================================

build: build-server build-agent

build-server:
	$(GOCMD) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(SERVER_BIN) ./cmd/server

build-agent:
	$(GOCMD) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(AGENT_BIN) ./cmd/agent

# Cross-compile agent for remote deployment
build-agent-all:
	GOOS=linux GOARCH=amd64 $(GOCMD) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/agent-linux-amd64 ./cmd/agent
	GOOS=linux GOARCH=arm64 $(GOCMD) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/agent-linux-arm64 ./cmd/agent

# ============================================================
# Development
# ============================================================

dev-server:
	$(GOCMD) run ./cmd/server

dev-web:
	cd $(WEB_DIR) && npm run dev

# ============================================================
# Testing & Linting
# ============================================================

test:
	$(GOCMD) test -v -race -timeout 120s ./...

test-cover:
	$(GOCMD) test -race -coverprofile=coverage.out ./...

lint:
	$(GOCMD) vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

lint-web:
	cd $(WEB_DIR) && npm run lint

typecheck-web:
	cd $(WEB_DIR) && npm run typecheck

# ============================================================
# Protobuf
# ============================================================

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/agent/proto/agent.proto

# ============================================================
# Frontend
# ============================================================

frontend:
	cd $(WEB_DIR) && npm install && npm run build

embed-frontend: frontend
	rm -rf internal/web/dist
	cp -r $(WEB_DIR)/dist internal/web/dist

# ============================================================
# Package (single binary with embedded frontend)
# ============================================================

package: embed-frontend build-server build-agent-all
	@echo "Package complete:"
	@echo "  Server: $(SERVER_BIN)"
	@echo "  Agent:  dist/agent-linux-amd64, dist/agent-linux-arm64"

# ============================================================
# Electron Desktop App
# ============================================================

electron: build-server
	cp $(SERVER_BIN) electron/bin/devicegrid-server 2>/dev/null || mkdir -p electron/bin && cp $(SERVER_BIN) electron/bin/devicegrid-server
	cd electron && npm run dist

electron-dev: build-server
	mkdir -p electron/bin && cp $(SERVER_BIN) electron/bin/devicegrid-server
	cd electron && npm start

# ============================================================
# Database Migrations
# ============================================================

migrate-create:
	@read -p "Migration name: " name; \
	ts=$$(date +%Y%m%d%H%M%S); \
	touch migrations/$${ts}_$${name}.up.sql; \
	touch migrations/$${ts}_$${name}.down.sql; \
	echo "Created migration: $${ts}_$${name}"

migrate-up:
	$(GOCMD) run ./cmd/server --migrate up

migrate-down:
	$(GOCMD) run ./cmd/server --migrate down

# ============================================================
# Misc
# ============================================================

clean:
	rm -rf bin/ dist/ web/dist/ web/node_modules/ coverage.out
	$(GOCMD) clean -cache

tidy:
	$(GOCMD) mod tidy

deps:
	$(GOCMD) mod download

help:
	@echo "DeviceGrid Makefile targets:"
	@echo "  build          - Build server + agent binaries"
	@echo "  build-agent-all - Cross-compile agent for amd64 + arm64"
	@echo "  dev-server     - Run backend with hot config"
	@echo "  dev-web        - Run Vue dev server"
	@echo "  test           - Run all Go tests"
	@echo "  lint           - Run go vet + golangci-lint"
	@echo "  proto          - Generate gRPC code from proto files"
	@echo "  frontend       - Build frontend for production"
	@echo "  package        - Full release build (frontend + embed + cross-compile)"
	@echo "  electron       - Build Electron desktop app"
	@echo "  electron-dev   - Run Electron in dev mode"
	@echo "  migrate-up     - Apply database migrations"
	@echo "  migrate-down   - Rollback last migration"
	@echo "  clean          - Remove build artifacts"

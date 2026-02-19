.DEFAULT_GOAL := help

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X main.version=$(VERSION) \
           -X main.commit=$(COMMIT) \
           -X main.buildDate=$(BUILD_DATE)

LDFLAGS_RELEASE := $(LDFLAGS) -s -w

.PHONY: build build-release install frontend frontend-dev dev test test-short e2e vet lint tidy clean release release-darwin-arm64 release-darwin-amd64 release-linux-amd64 help

# Build the binary (debug, with embedded frontend)
build: frontend
	CGO_ENABLED=1 go build -tags fts5 -ldflags="$(LDFLAGS)" -o agentsv ./cmd/agentsv
	@chmod +x agentsv

# Build with optimizations (release)
build-release: frontend
	CGO_ENABLED=1 go build -tags fts5 -ldflags="$(LDFLAGS_RELEASE)" -trimpath -o agentsv ./cmd/agentsv
	@chmod +x agentsv

# Install to ~/.local/bin, $GOBIN, or $GOPATH/bin
install: build-release
	@if [ -d "$(HOME)/.local/bin" ]; then \
		echo "Installing to ~/.local/bin/agentsv"; \
		cp agentsv "$(HOME)/.local/bin/agentsv"; \
	else \
		INSTALL_DIR="$${GOBIN:-$$(go env GOBIN)}"; \
		if [ -z "$$INSTALL_DIR" ]; then \
			GOPATH_FIRST="$$(go env GOPATH | cut -d: -f1)"; \
			INSTALL_DIR="$$GOPATH_FIRST/bin"; \
		fi; \
		mkdir -p "$$INSTALL_DIR"; \
		echo "Installing to $$INSTALL_DIR/agentsv"; \
		cp agentsv "$$INSTALL_DIR/agentsv"; \
	fi

# Build frontend SPA and copy into embed directory
frontend:
	cd frontend && npm install && npm run build
	rm -rf internal/web/dist
	cp -r frontend/dist internal/web/dist

# Run Vite dev server (use alongside `make dev`)
frontend-dev:
	cd frontend && npm run dev

# Run Go server in dev mode (no embedded frontend)
dev:
	go run -tags fts5 -ldflags="$(LDFLAGS)" ./cmd/agentsv $(ARGS)

# Run tests
test:
	go test -tags fts5 ./... -v -count=1

# Run fast tests only
test-short:
	go test -tags fts5 ./... -short -count=1

# Run Playwright E2E tests
e2e:
	cd frontend && npx playwright test

# Vet
vet:
	go vet -tags fts5 ./...

# Lint Go code with project defaults
lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1" >&2; \
		exit 1; \
	fi
	golangci-lint run ./...

# Tidy dependencies
tidy:
	go mod tidy

# Clean build artifacts
clean:
	rm -f agentsv
	rm -rf internal/web/dist dist/

# Build release binary for current platform (CGO required for sqlite3)
release: frontend
	mkdir -p dist
	CGO_ENABLED=1 go build -tags fts5 \
		-ldflags="$(LDFLAGS_RELEASE)" -trimpath \
		-o dist/agentsv-$$(go env GOOS)-$$(go env GOARCH) ./cmd/agentsv

# Cross-compile targets (require CC set to target cross-compiler)
release-darwin-arm64: frontend
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -tags fts5 \
		-ldflags="$(LDFLAGS_RELEASE)" -trimpath \
		-o dist/agentsv-darwin-arm64 ./cmd/agentsv

release-darwin-amd64: frontend
	mkdir -p dist
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -tags fts5 \
		-ldflags="$(LDFLAGS_RELEASE)" -trimpath \
		-o dist/agentsv-darwin-amd64 ./cmd/agentsv

release-linux-amd64: frontend
	mkdir -p dist
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -tags fts5 \
		-ldflags="$(LDFLAGS_RELEASE)" -trimpath \
		-o dist/agentsv-linux-amd64 ./cmd/agentsv

# Show help
help:
	@echo "agentsv build targets:"
	@echo ""
	@echo "  build          - Build with embedded frontend"
	@echo "  build-release  - Release build (optimized, stripped)"
	@echo "  install        - Build and install to ~/.local/bin or GOPATH"
	@echo ""
	@echo "  dev            - Run Go server (use with frontend-dev)"
	@echo "  frontend       - Build frontend SPA"
	@echo "  frontend-dev   - Run Vite dev server"
	@echo ""
	@echo "  test           - Run all tests"
	@echo "  test-short     - Run fast tests only"
	@echo "  e2e            - Run Playwright E2E tests"
	@echo "  vet            - Run go vet"
	@echo "  lint           - Run golangci-lint"
	@echo "  tidy           - Tidy go.mod"
	@echo ""
	@echo "  release        - Release build for current platform"
	@echo "  clean          - Remove build artifacts"

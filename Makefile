SHELL := /bin/bash

BINARY      := muxmaster-website
BIN_DIR     := bin
PKG         := ./cmd/muxmaster-website

# Tailwind v4 standalone CLI — pinned for reproducible builds.
TAILWIND_VERSION := v4.0.6
TAILWIND_BIN     := $(BIN_DIR)/tailwindcss
TAILWIND_OS      := $(shell uname -s | tr '[:upper:]' '[:lower:]')
TAILWIND_ARCH    := $(shell uname -m)
ifeq ($(TAILWIND_ARCH),x86_64)
	TAILWIND_ARCH := x64
endif
ifeq ($(TAILWIND_ARCH),aarch64)
	TAILWIND_ARCH := arm64
endif
TAILWIND_URL := https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$(TAILWIND_OS)-$(TAILWIND_ARCH)

CSS_SRC := static/css/app.css
CSS_DIR := static/css

GO_LDFLAGS := -s -w -X main.buildID=$(shell date -u +%Y%m%dT%H%M%SZ)
GO_FLAGS   := -trimpath -ldflags='$(GO_LDFLAGS)'

.PHONY: all build run dev test vet lint tidy css css-watch tailwind-install logo assets static-perms docker-build clean

# Normalise filesystem permissions across the entire static/ tree: every
# directory becomes 0755 and every file becomes 0644. Invoked as the
# final step of every recipe that creates or modifies files under
# static/ so the production Docker image — which runs as the distroless
# nonroot UID 65532 (since v1.0.2, commit 4c5a1f6) — can always read
# every static asset. See the v1.0.4 entry in CHANGELOG.md for the
# original bug (mktemp 0600 + mv preserving mode → 403 Forbidden on the
# CSS bundle). The sweep is intentionally tree-wide rather than
# per-file: any future tool that lands content under static/ is covered
# without needing to remember to chmod its own outputs.
NORMALIZE_STATIC_PERMS = find static -type d -exec chmod 0755 {} + && find static -type f -exec chmod 0644 {} +

# Upstream logo source. The website vendors a single canonical PNG copy at
# tools/imagegen/source.png (a build input, not a runtime asset, hence
# outside static/); every other image artefact (sized header logos,
# favicons, apple-touch-icons, the 1200x630 Open Graph composition) is
# produced from it deterministically by `make assets`.
UPSTREAM_LOGO := ../MuxMaster/assets/logo-muxmaster.png

ASSETS_TOOL := $(BIN_DIR)/imagegen

all: build

tailwind-install:
	@if [ ! -x "$(TAILWIND_BIN)" ]; then \
		mkdir -p $(BIN_DIR); \
		echo "Downloading Tailwind CSS standalone $(TAILWIND_VERSION) ($(TAILWIND_OS)-$(TAILWIND_ARCH))"; \
		curl -fsSL -o $(TAILWIND_BIN) $(TAILWIND_URL); \
		chmod +x $(TAILWIND_BIN); \
	fi
	@$(TAILWIND_BIN) --help > /dev/null

# Build the CSS bundle once and write it to a content-hashed filename.
# The Go server discovers the hashed filename at startup (see internal/render).
# The final permission sweep is handled by NORMALIZE_STATIC_PERMS — see
# the comment on that variable for the rationale.
css: tailwind-install
	@mkdir -p $(CSS_DIR)
	@rm -f $(CSS_DIR)/app.*.css
	@TMP_OUT=$$(mktemp); \
		$(TAILWIND_BIN) -i $(CSS_SRC) -o $$TMP_OUT --minify >/dev/null && \
		HASH=$$(sha256sum $$TMP_OUT | cut -c1-12) && \
		mv $$TMP_OUT $(CSS_DIR)/app.$$HASH.css && \
		echo "wrote $(CSS_DIR)/app.$$HASH.css"
	@$(NORMALIZE_STATIC_PERMS)

css-watch: tailwind-install
	@mkdir -p $(CSS_DIR)
	@$(TAILWIND_BIN) -i $(CSS_SRC) -o $(CSS_DIR)/app.dev.css --watch

# Vendor the upstream logo into the build-input tree. The source is the
# canonical 1024x1024 RGBA PNG in ../MuxMaster/assets/, copied to
# tools/imagegen/source.png. `make assets` then derives every sized
# variant under static/. The source PNG deliberately does NOT live under
# static/ because it is a build input, not a runtime asset: serving it
# publicly would expose a 1.6 MB blob that no page references.
logo:
	@if [ ! -f "$(UPSTREAM_LOGO)" ]; then \
		echo "missing $(UPSTREAM_LOGO); clone the MuxMaster repo next to this one" >&2; \
		exit 1; \
	fi
	@mkdir -p tools/imagegen
	@cp $(UPSTREAM_LOGO) tools/imagegen/source.png
	@chmod 0644 tools/imagegen/source.png
	@echo "vendored $(UPSTREAM_LOGO) into tools/imagegen/source.png"

# Build the build-time image generator and run it against the build-input
# source PNG. Produces: logo-{32,64,80,128,192,256,384}.png, favicon-{32,
# 192,512}.png, apple-touch-icon-180.png, and og-image.png (1200x630).
# All deterministic.
$(ASSETS_TOOL): tools/imagegen/main.go
	@mkdir -p $(BIN_DIR)
	@go build -o $(ASSETS_TOOL) ./tools/imagegen

assets: $(ASSETS_TOOL) tools/imagegen/source.png
	@./$(ASSETS_TOOL)
	@$(NORMALIZE_STATIC_PERMS)

# Manual sweep, exposed for debugging and for ad-hoc invocation after any
# operation that touches static/ outside the standard recipes.
static-perms:
	@$(NORMALIZE_STATIC_PERMS)
	@echo "normalised permissions under static/ (dirs 0755, files 0644)"

build: css assets
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 go build $(GO_FLAGS) -o $(BIN_DIR)/$(BINARY) $(PKG)

run: build
	@./$(BIN_DIR)/$(BINARY)

dev: css
	@go run $(PKG)

test:
	@go test -race ./...

vet:
	@go vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed; running 'go vet' instead"; \
		go vet ./...; \
	fi

tidy:
	@go mod tidy

docker-build:
	@docker build -t $(BINARY):dev .

clean:
	@rm -rf $(BIN_DIR)
	@rm -f $(CSS_DIR)/app.*.css $(CSS_DIR)/app.dev.css
	@rm -f static/img/logo-*.png static/img/og-image.png
	@rm -f static/favicon/favicon-*.png static/favicon/apple-touch-icon-*.png

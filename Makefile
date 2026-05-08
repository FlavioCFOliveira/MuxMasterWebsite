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

.PHONY: all build run dev test vet lint tidy css css-watch tailwind-install logo docker-build clean

# Upstream logo source. The website vendors a single PNG copy at
# static/img/logo.png. TODO(spec): the asset-generation pipeline will
# additionally produce sized variants (32, 80, 192, 512), AVIF/WebP, and
# a 1200x630 Open Graph composition; until then we serve the source PNG
# at every size.
UPSTREAM_LOGO := ../MuxMaster/assets/logo-muxmaster.png

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
css: tailwind-install
	@mkdir -p $(CSS_DIR)
	@rm -f $(CSS_DIR)/app.*.css
	@TMP_OUT=$$(mktemp); \
		$(TAILWIND_BIN) -i $(CSS_SRC) -o $$TMP_OUT --minify >/dev/null && \
		HASH=$$(sha256sum $$TMP_OUT | cut -c1-12) && \
		mv $$TMP_OUT $(CSS_DIR)/app.$$HASH.css && \
		echo "wrote $(CSS_DIR)/app.$$HASH.css"

css-watch: tailwind-install
	@mkdir -p $(CSS_DIR)
	@$(TAILWIND_BIN) -i $(CSS_SRC) -o $(CSS_DIR)/app.dev.css --watch

# Vendor the upstream logo into the static tree. The source is the
# canonical 1024x1024 RGBA PNG in ../MuxMaster/assets/. Three copies are
# produced today: header logo, OG image, favicon. TODO(spec): replace
# with proper sized variants and a 1200x630 OG composition once the
# asset-generation pipeline lands.
logo:
	@if [ ! -f "$(UPSTREAM_LOGO)" ]; then \
		echo "missing $(UPSTREAM_LOGO); clone the MuxMaster repo next to this one" >&2; \
		exit 1; \
	fi
	@mkdir -p static/img static/favicon
	@cp $(UPSTREAM_LOGO) static/img/logo.png
	@cp $(UPSTREAM_LOGO) static/img/og-image.png
	@cp $(UPSTREAM_LOGO) static/favicon/favicon.png
	@echo "vendored $(UPSTREAM_LOGO) into static/img and static/favicon"

build: css
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

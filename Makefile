# cc-deck Makefile
#
# Two components:
#   cc-zellij-plugin/  Rust WASM plugin for Zellij (controller + sidebar, single binary)
#   cc-deck/           Go CLI that embeds the plugin and provides hook integration
#
# Quick start:
#   make build          Build everything (WASM + CLI)
#   make install        Build and install plugin into Zellij
#   make test           Run all tests
#   make dev            Start Zellij with cc-deck layout for development

VERSION    ?= 0.12.0
REGISTRY   ?= quay.io/cc-deck
WASM_TARGET = wasm32-wasip1
WASM_SRC    = cc-zellij-plugin/target/$(WASM_TARGET)/release/cc_deck.wasm
WASM_DBG    = cc-zellij-plugin/target/$(WASM_TARGET)/dev-opt/cc_deck.wasm
WASM_DST    = cc-deck/internal/plugin/cc_deck.wasm
CLI_BIN     = cc-deck/cc-deck

BASE_IMAGE  = $(REGISTRY)/cc-deck-base

GIT_COMMIT  = $(shell git rev-parse --short=8 HEAD 2>/dev/null || echo "unknown")
GIT_DIRTY   = $(shell git diff --quiet 2>/dev/null && echo "" || echo "-dirty")
BUILD_DATE  = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

CLI_LDFLAGS = -X github.com/cc-deck/cc-deck/internal/cmd.Version=$(VERSION)+$(GIT_COMMIT)$(GIT_DIRTY) \
              -X github.com/cc-deck/cc-deck/internal/cmd.Commit=$(GIT_COMMIT)$(GIT_DIRTY) \
              -X github.com/cc-deck/cc-deck/internal/cmd.Date=$(BUILD_DATE) \
              -X github.com/cc-deck/cc-deck/internal/cmd.ImageRegistry=$(REGISTRY)

.PHONY: build build-wasm build-wasm-debug copy-wasm build-cli cross-cli \
        test test-go test-rust test-e2e test-compose test-integration smoke lint lint-go lint-rust \
        deploy-ssh install uninstall status \
        test-image demo-image demo-image-push base-image base-image-push \
        demo-setup demo-record demo-gif demo-mp4 demo-clean \
        dev reload clean help

## -- Build -------------------------------------------------

build: build-wasm copy-wasm build-cli  ## Build everything (release WASM + CLI)

build-wasm:  ## Build WASM plugin (release, wasm-opt if available)
	cd cc-zellij-plugin && cargo build --target $(WASM_TARGET) --release
	@if command -v wasm-opt >/dev/null 2>&1; then \
		echo "Running wasm-opt on binary..."; \
		wasm-opt -Oz --zero-filled-memory --enable-bulk-memory-opt $(WASM_SRC) -o $(WASM_SRC); \
	fi

build-wasm-debug:  ## Build WASM plugin (dev-opt: opt-level=1 for wasmi performance)
	cd cc-zellij-plugin && cargo build --target $(WASM_TARGET) --profile dev-opt

copy-wasm: $(WASM_SRC)  ## Copy WASM binary to Go embed location
	mkdir -p cc-deck/internal/plugin
	cp $(WASM_SRC) $(WASM_DST)

build-cli: $(WASM_DST)  ## Build Go CLI (requires WASM to be copied first)
	cd cc-deck && go build -ldflags "$(CLI_LDFLAGS)" -o cc-deck ./cmd/cc-deck

cross-cli: $(WASM_DST)  ## Cross-compile CLI for linux/amd64 and linux/arm64
	cd cc-deck && GOOS=linux GOARCH=amd64 go build -ldflags "$(CLI_LDFLAGS)" -o cc-deck-linux-amd64 ./cmd/cc-deck
	cd cc-deck && GOOS=linux GOARCH=arm64 go build -ldflags "$(CLI_LDFLAGS)" -o cc-deck-linux-arm64 ./cmd/cc-deck

$(WASM_DST):
	@echo "WASM binary not found at $(WASM_DST)"
	@echo "Run 'make build-wasm copy-wasm' first"
	@exit 1

## -- Test --------------------------------------------------

test: test-go test-rust  ## Run all tests

test-go: $(WASM_DST)  ## Run Go tests
	cd cc-deck && go test ./...

test-rust:  ## Run Rust tests (native, not WASM)
	cd cc-zellij-plugin && cargo test

test-e2e: build  ## Run E2E tests (builds binary, runs as subprocess)
	cd cc-deck && go test -tags e2e -v -count=1 ./internal/e2e/

test-compose:  ## Run compose smoke tests (requires podman + podman-compose)
	cd cc-deck && go test -run "TestComposeSmoke" -v -count=1 -timeout 300s ./internal/cmd/

test-integration:  ## Run K8s integration tests (requires kind cluster named cc-deck-test)
	cd cc-deck && go test -tags integration -v -timeout 5m -count=1 ./internal/integration/

smoke: build  ## Run smoke test script against compiled binary
	./scripts/smoke-test-env.sh

## -- Lint --------------------------------------------------

lint: lint-go lint-rust  ## Run all linters

lint-go:  ## Run Go linter
	cd cc-deck && go vet ./...

lint-rust:  ## Run Rust linter
	cd cc-zellij-plugin && cargo clippy -- -D warnings

## -- Remote Deploy ----------------------------------------

DEPLOY_HOST   ?=
DEPLOY_ARCH   ?= amd64
DEPLOY_BIN    ?= cc-deck/cc-deck-linux-$(DEPLOY_ARCH)
DEPLOY_WASM   ?= $(WASM_SRC)

deploy-ssh: build-wasm cross-cli  ## Deploy cc-deck to a remote SSH host (DEPLOY_HOST=marovo)
	@if [ -z "$(DEPLOY_HOST)" ]; then \
		echo "Usage: make deploy-ssh DEPLOY_HOST=<hostname> [DEPLOY_ARCH=amd64|arm64]"; \
		exit 1; \
	fi
	scp $(DEPLOY_WASM) $(DEPLOY_HOST):~/.config/zellij/plugins/cc_deck.wasm
	scp $(DEPLOY_BIN)  $(DEPLOY_HOST):~/bin/cc-deck
	@echo "Deployed to $(DEPLOY_HOST) ($(DEPLOY_ARCH)). Restart Zellij session to load new plugin."

## -- Plugin Management ------------------------------------

install: build  ## Build and install plugin into Zellij
	$(CLI_BIN) config plugin install --force
	@# Remove legacy two-binary files from previous installations
	@rm -f $(HOME)/.config/zellij/plugins/cc_deck_controller.wasm 2>/dev/null || true
	@rm -f $(HOME)/.config/zellij/plugins/cc_deck_sidebar.wasm 2>/dev/null || true

uninstall:  ## Remove plugin from Zellij
	$(CLI_BIN) config plugin remove

status:  ## Show plugin installation status
	$(CLI_BIN) config plugin status

## -- Image Testing ----------------------------------------

TEST_IMAGE_DIR ?= /tmp/cc-deck-test-image

test-image: build cross-cli  ## Build cc-deck, cross-compile, init test dir, and open in Claude Code
	rm -rf $(TEST_IMAGE_DIR)
	$(CLI_BIN) image init $(TEST_IMAGE_DIR)
	@echo ""
	@echo "Test directory ready: $(TEST_IMAGE_DIR)"
	@echo ""
	@echo "  Dev builds: cross-compiled binaries placed in build-context/ automatically"
	@echo "  Released builds: /cc-deck.build downloads binaries from GitHub Releases"
	@echo ""
	@echo "Next: cd $(TEST_IMAGE_DIR) && claude"
	@echo "Then: /cc-deck.build"
	@# For dev builds, pre-place cross-compiled binaries so /cc-deck.build finds them
	mkdir -p $(TEST_IMAGE_DIR)/build-context
	cp cc-deck/cc-deck-linux-* $(TEST_IMAGE_DIR)/build-context/

## -- Container Images --------------------------------------

PLATFORMS  ?= linux/arm64,linux/amd64
DEMO_IMAGE = $(REGISTRY)/cc-deck-demo

demo-image: cross-cli  ## Build the cc-deck demo container image (multi-arch manifest)
	mkdir -p demo-image/build-context
	cp cc-deck/cc-deck-linux-* demo-image/build-context/
	@podman rmi $(DEMO_IMAGE):latest 2>/dev/null || true
	@podman manifest rm $(DEMO_IMAGE):latest 2>/dev/null || true
	podman manifest create $(DEMO_IMAGE):latest
	@for arch in arm64 amd64; do \
		podman build --platform linux/$${arch} -t $(DEMO_IMAGE):latest-$${arch} demo-image/; \
		podman manifest add $(DEMO_IMAGE):latest $(DEMO_IMAGE):latest-$${arch}; \
	done

demo-image-push: demo-image  ## Build and push the demo image (multi-arch)
	podman manifest push --all $(DEMO_IMAGE):latest docker://$(DEMO_IMAGE):latest

base-image:  ## Build the cc-deck base container image (multi-arch manifest)
	@podman rmi $(BASE_IMAGE):latest 2>/dev/null || true
	@podman manifest rm $(BASE_IMAGE):latest 2>/dev/null || true
	podman manifest create $(BASE_IMAGE):latest
	@for arch in arm64 amd64; do \
		podman build --platform linux/$${arch} -t $(BASE_IMAGE):latest-$${arch} base-image/; \
		podman manifest add $(BASE_IMAGE):latest $(BASE_IMAGE):latest-$${arch}; \
	done

base-image-push: base-image  ## Build and push the base image (multi-arch)
	podman manifest push --all $(BASE_IMAGE):latest docker://$(BASE_IMAGE):latest

## -- Demo Recording ----------------------------------------

DEMO ?= plugin

demo-setup:  ## Set up demo projects in /tmp/cc-deck-demo
	demos/projects/setup.sh

demo-record: demo-setup  ## Record a demo (DEMO=plugin|deploy|image)
	RECORD=1 demos/scripts/$(DEMO)-demo.sh

demo-gif:  ## Convert recording to GIF (DEMO=plugin|deploy|image)
	@test -f demos/recordings/$(DEMO)-demo.cast || (echo "No recording found. Run: make demo-record DEMO=$(DEMO)" && exit 1)
	agg --cols 200 --rows 50 --idle-time-limit 3 --last-frame-duration 5 \
		demos/recordings/$(DEMO)-demo.cast demos/recordings/$(DEMO)-demo.gif
	@echo "GIF saved: demos/recordings/$(DEMO)-demo.gif"

demo-mp4:  ## Convert recording to MP4 with voiceover (DEMO=plugin|deploy|image)
	@test -f demos/recordings/$(DEMO)-demo.cast || (echo "No recording found. Run: make demo-record DEMO=$(DEMO)" && exit 1)
	@if [ -f demos/recordings/$(DEMO)-demo-voiceover.mp3 ]; then \
		ffmpeg -y -i demos/recordings/$(DEMO)-demo.gif -i demos/recordings/$(DEMO)-demo-voiceover.mp3 \
			-c:v libx264 -c:a aac -shortest demos/recordings/$(DEMO)-demo.mp4; \
	else \
		ffmpeg -y -i demos/recordings/$(DEMO)-demo.gif -c:v libx264 demos/recordings/$(DEMO)-demo.mp4; \
	fi
	@echo "MP4 saved: demos/recordings/$(DEMO)-demo.mp4"

demo-voiceover:  ## Generate voiceover audio (DEMO=plugin|deploy|image, requires OPENAI_API_KEY)
	demos/voiceover.sh demos/narration/$(DEMO)-demo.txt

demo-clean:  ## Remove demo projects and recordings
	demos/projects/cleanup.sh
	rm -f demos/recordings/*.cast demos/recordings/*.gif demos/recordings/*.mp4

## -- Development ------------------------------------------

ZELLIJ_PLUGINS_DIR ?= $(HOME)/.config/zellij/plugins
ZELLIJ_CACHE_DIR   ?= $(HOME)/Library/Caches/org.Zellij-Contributors.Zellij

dev: dev-install  ## Build debug WASM, install to Zellij plugins dir, clear cache
	@echo "Ready. Start Zellij with: zellij --layout cc-deck"

dev-install: build-wasm-debug  ## Quick install: copy dev-opt WASM to Zellij plugins dir
	mkdir -p $(ZELLIJ_PLUGINS_DIR)
	cp $(WASM_DBG) $(ZELLIJ_PLUGINS_DIR)/cc_deck.wasm
	@# Clear compiled WASM caches (per-session UUID dirs) and stale permissions
	@# so Zellij re-compiles the plugin and re-prompts for any new permissions
	rm -rf $(ZELLIJ_CACHE_DIR)/*/file: 2>/dev/null || true
	rm -f $(ZELLIJ_CACHE_DIR)/permissions.kdl 2>/dev/null || true
	@echo "Installed cc_deck.wasm to $(ZELLIJ_PLUGINS_DIR)/ and cleared cache"

reload: build-wasm-debug  ## Rebuild dev-opt WASM and hot-reload in running Zellij
	zellij action start-or-reload-plugin "file:$(WASM_DBG)"

## -- Clean ------------------------------------------------

clean:  ## Remove all build artifacts
	cd cc-zellij-plugin && cargo clean
	rm -f $(WASM_DST)
	rm -f $(CLI_BIN)
	rm -f cc-deck/cc-deck-linux-*

## -- Help -------------------------------------------------

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

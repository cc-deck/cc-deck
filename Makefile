# cc-deck Makefile
#
# Two components:
#   cc-zellij-plugin/  Rust WASM plugin for Zellij (sidebar + picker)
#   cc-deck/           Go CLI that embeds the plugin and provides hook integration
#
# Quick start:
#   make build          Build everything (WASM + CLI)
#   make install        Build and install plugin into Zellij
#   make test           Run all tests
#   make dev            Start Zellij with cc-deck layout for development

VERSION    ?= 0.6.0
REGISTRY   ?= quay.io/cc-deck
WASM_TARGET = wasm32-wasip1
WASM_SRC    = cc-zellij-plugin/target/$(WASM_TARGET)/release/cc_deck.wasm
WASM_DBG    = cc-zellij-plugin/target/$(WASM_TARGET)/debug/cc_deck.wasm
WASM_DST    = cc-deck/internal/plugin/cc_deck.wasm
CLI_BIN     = cc-deck/cc-deck

BASE_IMAGE  = $(REGISTRY)/cc-deck-base

CLI_LDFLAGS = -X github.com/cc-deck/cc-deck/internal/cmd.Version=$(VERSION) \
              -X github.com/cc-deck/cc-deck/internal/cmd.ImageRegistry=$(REGISTRY)

.PHONY: build build-wasm build-wasm-debug copy-wasm build-cli cross-cli \
        test test-go test-rust lint lint-go lint-rust \
        install uninstall status \
        test-image demo-image demo-image-push base-image base-image-push \
        demo-setup demo-record demo-gif demo-mp4 demo-clean \
        dev reload clean help

## -- Build -------------------------------------------------

build: build-wasm copy-wasm build-cli  ## Build everything (release WASM + CLI)

build-wasm:  ## Build WASM plugin (release)
	cd cc-zellij-plugin && cargo build --target $(WASM_TARGET) --release

build-wasm-debug:  ## Build WASM plugin (debug, faster)
	cd cc-zellij-plugin && cargo build --target $(WASM_TARGET)

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

## -- Lint --------------------------------------------------

lint: lint-go lint-rust  ## Run all linters

lint-go:  ## Run Go linter
	cd cc-deck && go vet ./...

lint-rust:  ## Run Rust linter
	cd cc-zellij-plugin && cargo clippy -- -D warnings

## -- Plugin Management ------------------------------------

install: build  ## Build and install plugin into Zellij
	$(CLI_BIN) plugin install --force

uninstall:  ## Remove plugin from Zellij
	$(CLI_BIN) plugin remove

status:  ## Show plugin installation status
	$(CLI_BIN) plugin status

## -- Image Testing ----------------------------------------

TEST_IMAGE_DIR ?= /tmp/cc-deck-test-image

test-image: build cross-cli  ## Build cc-deck, cross-compile, init test dir, and open in Claude Code
	rm -rf $(TEST_IMAGE_DIR)
	$(CLI_BIN) image init $(TEST_IMAGE_DIR)
	mkdir -p $(TEST_IMAGE_DIR)/.build-context
	cp cc-deck/cc-deck-linux-* $(TEST_IMAGE_DIR)/.build-context/
	@echo ""
	@echo "Test directory ready: $(TEST_IMAGE_DIR)"
	@echo ""
	@echo "Next: cd $(TEST_IMAGE_DIR) && claude"
	@echo "Then: /cc-deck.build"

## -- Container Images --------------------------------------

PLATFORMS  ?= linux/arm64,linux/amd64
DEMO_IMAGE = $(REGISTRY)/cc-deck-demo

demo-image: cross-cli  ## Build the cc-deck demo container image (multi-arch manifest)
	mkdir -p demo-image/.build-context
	cp cc-deck/cc-deck-linux-* demo-image/.build-context/
	@podman manifest rm $(DEMO_IMAGE):latest 2>/dev/null || true
	podman manifest create $(DEMO_IMAGE):latest
	@for arch in arm64 amd64; do \
		podman build --platform linux/$${arch} -t $(DEMO_IMAGE):latest-$${arch} demo-image/; \
		podman manifest add $(DEMO_IMAGE):latest $(DEMO_IMAGE):latest-$${arch}; \
	done

demo-image-push: demo-image  ## Build and push the demo image (multi-arch)
	podman manifest push --all $(DEMO_IMAGE):latest docker://$(DEMO_IMAGE):latest

base-image:  ## Build the cc-deck base container image (multi-arch manifest)
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

dev-install: build-wasm-debug  ## Quick install: copy debug WASM to Zellij plugins dir
	mkdir -p $(ZELLIJ_PLUGINS_DIR)
	cp cc-zellij-plugin/target/$(WASM_TARGET)/debug/cc_deck.wasm $(ZELLIJ_PLUGINS_DIR)/cc_deck.wasm
	@# Clear compiled WASM cache so Zellij picks up the new binary
	rm -f $(ZELLIJ_CACHE_DIR)/0.43.1/[0-9]* 2>/dev/null || true
	@echo "Installed cc_deck.wasm to $(ZELLIJ_PLUGINS_DIR)/ and cleared cache"

reload: build-wasm-debug  ## Rebuild debug WASM and hot-reload in running Zellij
	zellij action start-or-reload-plugin "file:cc-zellij-plugin/target/$(WASM_TARGET)/debug/cc_deck.wasm"

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

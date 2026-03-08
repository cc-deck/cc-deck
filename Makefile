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

VERSION    ?= 0.2.0
WASM_TARGET = wasm32-wasip1
WASM_SRC    = cc-zellij-plugin/target/$(WASM_TARGET)/release/cc_deck.wasm
WASM_DBG    = cc-zellij-plugin/target/$(WASM_TARGET)/debug/cc_deck.wasm
WASM_DST    = cc-deck/internal/plugin/cc_deck.wasm
CLI_BIN     = cc-deck/cc-deck

.PHONY: build build-wasm build-wasm-debug copy-wasm build-cli \
        test test-go test-rust lint lint-go lint-rust \
        install uninstall status \
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
	cd cc-deck && go build \
		-ldflags "-X main.Version=$(VERSION)" \
		-o cc-deck ./cmd/cc-deck

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

## -- Help -------------------------------------------------

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

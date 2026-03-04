.PHONY: build build-wasm copy-wasm build-cli clean

WASM_TARGET := wasm32-wasip1
WASM_SRC := cc-zellij-plugin/target/$(WASM_TARGET)/release/cc_deck.wasm
WASM_DST := cc-deck/internal/plugin/cc_deck.wasm

build: build-wasm copy-wasm build-cli

build-wasm:
	cd cc-zellij-plugin && cargo build --target $(WASM_TARGET) --release

copy-wasm:
	cp $(WASM_SRC) $(WASM_DST)

build-cli:
	cd cc-deck && go build -o cc-deck ./cmd/cc-deck

clean:
	cd cc-zellij-plugin && cargo clean
	rm -f $(WASM_DST)
	rm -f cc-deck/cc-deck

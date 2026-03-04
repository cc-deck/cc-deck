package plugin

import (
	_ "embed"
)

//go:embed cc_deck.wasm
var wasmBinary []byte

// PluginInfo describes the embedded WASM plugin binary.
type PluginInfo struct {
	Version    string // plugin version, e.g. "0.1.0"
	SDKVersion string // zellij-tile SDK version, e.g. "0.43"
	MinZellij  string // minimum supported Zellij version, e.g. "0.40"
	BinarySize int64
	Binary     []byte
}

// EmbeddedPlugin returns info about the embedded WASM plugin.
func EmbeddedPlugin() PluginInfo {
	return PluginInfo{
		Version:    "0.1.0",
		SDKVersion: "0.43",
		MinZellij:  "0.40",
		BinarySize: int64(len(wasmBinary)),
		Binary:     wasmBinary,
	}
}

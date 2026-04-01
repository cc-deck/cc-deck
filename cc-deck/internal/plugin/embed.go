package plugin

import (
	_ "embed"
)

// Legacy single-binary embed (kept for backward compatibility during transition)
//go:embed cc_deck.wasm
var wasmBinary []byte

// Two-binary architecture (030-single-instance-arch)
//go:embed cc_deck_controller.wasm
var controllerWasm []byte

//go:embed cc_deck_sidebar.wasm
var sidebarWasm []byte

// PluginInfo describes the embedded WASM plugin binary.
type PluginInfo struct {
	Version    string // plugin version, e.g. "0.1.0"
	SDKVersion string // zellij-tile SDK version, e.g. "0.43"
	MinZellij  string // minimum supported Zellij version, e.g. "0.40"
	BinarySize       int64
	ControllerSize   int64
	SidebarSize      int64

	// Legacy single binary (deprecated, kept for transition)
	Binary []byte

	// Two-binary architecture
	ControllerBinary []byte
	SidebarBinary    []byte
}

// EmbeddedPlugin returns info about the embedded WASM plugins.
func EmbeddedPlugin() PluginInfo {
	return PluginInfo{
		Version:          "0.8.0",
		SDKVersion:       "0.43",
		MinZellij:        "0.40",
		BinarySize:       int64(len(wasmBinary)),
		ControllerSize:   int64(len(controllerWasm)),
		SidebarSize:      int64(len(sidebarWasm)),
		Binary:           wasmBinary,
		ControllerBinary: controllerWasm,
		SidebarBinary:    sidebarWasm,
	}
}

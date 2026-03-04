package plugin

import (
	"os"
	"path/filepath"
	"strings"
)

// InstallState describes the current installation state of the cc-deck plugin.
type InstallState struct {
	PluginInstalled   bool
	PluginPath        string
	PluginSize        int64
	LayoutInstalled   bool
	LayoutPath        string
	LayoutType        string // "minimal" or "full"
	DefaultInjected   bool
	DefaultLayoutPath string
	Compatibility     string // "compatible", "untested", "incompatible"
}

// DetectInstallState checks the filesystem to determine what is currently installed.
// It looks for the WASM binary in the plugins directory, a cc-deck layout file,
// and whether the default layout has been injected with the plugin block.
func DetectInstallState(zInfo ZellijInfo, pInfo PluginInfo) InstallState {
	state := InstallState{}

	if !zInfo.Installed {
		state.Compatibility = "incompatible"
		return state
	}

	state.Compatibility = CheckCompatibility(zInfo.Version, pInfo.SDKVersion)

	// Check if the WASM plugin binary is installed
	pluginPath := filepath.Join(zInfo.PluginsDir, "cc_deck.wasm")
	if fi, err := os.Stat(pluginPath); err == nil {
		state.PluginInstalled = true
		state.PluginPath = pluginPath
		state.PluginSize = fi.Size()
	}

	// Check for a cc-deck layout file
	checkLayout(&state, zInfo.LayoutsDir, "cc-deck.kdl")

	// Check if the default layout has our injection
	defaultLayoutPath := filepath.Join(zInfo.LayoutsDir, "default.kdl")
	if content, err := os.ReadFile(defaultLayoutPath); err == nil {
		state.DefaultLayoutPath = defaultLayoutPath
		if HasInjection(string(content)) {
			state.DefaultInjected = true
		}
	}

	return state
}

// checkLayout looks for a cc-deck layout file and determines its type.
func checkLayout(state *InstallState, layoutsDir, filename string) {
	layoutPath := filepath.Join(layoutsDir, filename)
	content, err := os.ReadFile(layoutPath)
	if err != nil {
		return
	}

	state.LayoutInstalled = true
	state.LayoutPath = layoutPath

	if strings.Contains(string(content), "default_tab_template") {
		state.LayoutType = "full"
	} else {
		state.LayoutType = "minimal"
	}
}

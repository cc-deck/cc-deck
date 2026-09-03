package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckLayout_NoFile(t *testing.T) {
	dir := t.TempDir()
	state := InstallState{}
	checkLayout(&state, dir, "cc-deck.kdl")

	if state.LayoutInstalled {
		t.Error("expected LayoutInstalled to be false when no layout file exists")
	}
	if state.LayoutPath != "" {
		t.Errorf("expected empty LayoutPath, got %q", state.LayoutPath)
	}
}

func TestCheckLayout_FullLayout(t *testing.T) {
	dir := t.TempDir()
	content := GenerateLayout("/plugins", LayoutStandard)
	if err := os.WriteFile(filepath.Join(dir, "cc-deck.kdl"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write layout file: %v", err)
	}

	state := InstallState{}
	checkLayout(&state, dir, "cc-deck.kdl")

	if !state.LayoutInstalled {
		t.Error("expected LayoutInstalled to be true")
	}
	if state.LayoutType != "full" {
		t.Errorf("expected LayoutType %q, got %q", "full", state.LayoutType)
	}
	if state.LayoutPath != filepath.Join(dir, "cc-deck.kdl") {
		t.Errorf("expected LayoutPath %q, got %q", filepath.Join(dir, "cc-deck.kdl"), state.LayoutPath)
	}
}

func TestCheckLayout_MinimalLayout(t *testing.T) {
	dir := t.TempDir()
	// A layout without a default_tab_template block is classified as "minimal".
	content := "layout {\n    tab name=\"main\" focus=true {\n        pane\n    }\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "cc-deck.kdl"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write layout file: %v", err)
	}

	state := InstallState{}
	checkLayout(&state, dir, "cc-deck.kdl")

	if !state.LayoutInstalled {
		t.Error("expected LayoutInstalled to be true")
	}
	if state.LayoutType != "minimal" {
		t.Errorf("expected LayoutType %q, got %q", "minimal", state.LayoutType)
	}
}

func TestDetectInstallState_ZellijNotInstalled(t *testing.T) {
	zInfo := ZellijInfo{Installed: false}
	pInfo := PluginInfo{SDKVersion: "0.44"}

	state := DetectInstallState(zInfo, pInfo)

	if state.Compatibility != "incompatible" {
		t.Errorf("expected Compatibility %q, got %q", "incompatible", state.Compatibility)
	}
	if state.PluginInstalled {
		t.Error("expected PluginInstalled to be false when zellij is not installed")
	}
}

func TestDetectInstallState_FullyInstalled(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	layoutsDir := filepath.Join(dir, "layouts")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}
	if err := os.MkdirAll(layoutsDir, 0755); err != nil {
		t.Fatalf("failed to create layouts dir: %v", err)
	}

	pluginBinary := []byte("fake wasm binary")
	if err := os.WriteFile(filepath.Join(pluginsDir, "cc_deck.wasm"), pluginBinary, 0644); err != nil {
		t.Fatalf("failed to write plugin binary: %v", err)
	}

	layoutContent := GenerateLayout(pluginsDir, LayoutMinimal)
	if err := os.WriteFile(filepath.Join(layoutsDir, "cc-deck.kdl"), []byte(layoutContent), 0644); err != nil {
		t.Fatalf("failed to write layout file: %v", err)
	}

	defaultContent := InjectionStart + "\nsome injected block\n" + InjectionEnd
	if err := os.WriteFile(filepath.Join(layoutsDir, "default.kdl"), []byte(defaultContent), 0644); err != nil {
		t.Fatalf("failed to write default layout: %v", err)
	}

	zInfo := ZellijInfo{Installed: true, Version: "0.43.0", PluginsDir: pluginsDir, LayoutsDir: layoutsDir}
	pInfo := PluginInfo{SDKVersion: "0.44"}

	state := DetectInstallState(zInfo, pInfo)

	if !state.PluginInstalled {
		t.Error("expected PluginInstalled to be true")
	}
	if state.PluginSize != int64(len(pluginBinary)) {
		t.Errorf("expected PluginSize %d, got %d", len(pluginBinary), state.PluginSize)
	}
	if !state.LayoutInstalled {
		t.Error("expected LayoutInstalled to be true")
	}
	if !state.DefaultInjected {
		t.Error("expected DefaultInjected to be true")
	}
	if state.Compatibility != "compatible" {
		t.Errorf("expected Compatibility %q, got %q", "compatible", state.Compatibility)
	}
}

func TestDetectInstallState_NothingInstalled(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	layoutsDir := filepath.Join(dir, "layouts")

	zInfo := ZellijInfo{Installed: true, Version: "0.43.0", PluginsDir: pluginsDir, LayoutsDir: layoutsDir}
	pInfo := PluginInfo{SDKVersion: "0.44"}

	state := DetectInstallState(zInfo, pInfo)

	if state.PluginInstalled {
		t.Error("expected PluginInstalled to be false when binary is absent")
	}
	if state.LayoutInstalled {
		t.Error("expected LayoutInstalled to be false when layout file is absent")
	}
	if state.DefaultInjected {
		t.Error("expected DefaultInjected to be false when default layout is absent")
	}
}

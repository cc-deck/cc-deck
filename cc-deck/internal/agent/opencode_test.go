package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeAgentIdentity(t *testing.T) {
	a := &OpenCodeAgent{}
	if a.Name() != "opencode" {
		t.Errorf("Name() = %q, want %q", a.Name(), "opencode")
	}
	if a.DisplayName() != "OpenCode" {
		t.Errorf("DisplayName() = %q, want %q", a.DisplayName(), "OpenCode")
	}
	if a.Indicator() != "OC" {
		t.Errorf("Indicator() = %q, want %q", a.Indicator(), "OC")
	}
}

func TestOpenCodeAgentTranslateEvent(t *testing.T) {
	a := &OpenCodeAgent{}

	tests := []struct {
		name      string
		input     string
		wantEvent string
		wantTool  string
	}{
		{
			name:      "SessionStart",
			input:     `{"hook_event_name":"SessionStart","session_id":"oc-1"}`,
			wantEvent: "SessionStart",
		},
		{
			name:      "Stop",
			input:     `{"hook_event_name":"Stop","session_id":"oc-1"}`,
			wantEvent: "Stop",
		},
		{
			name:      "PreToolUse",
			input:     `{"hook_event_name":"PreToolUse","tool_name":"file_edit","session_id":"oc-1"}`,
			wantEvent: "PreToolUse",
			wantTool:  "file_edit",
		},
		{
			name:      "PostToolUse",
			input:     `{"hook_event_name":"PostToolUse","tool_name":"file_edit","session_id":"oc-1"}`,
			wantEvent: "PostToolUse",
			wantTool:  "file_edit",
		},
		{
			name:      "PermissionRequest",
			input:     `{"hook_event_name":"PermissionRequest","session_id":"oc-1"}`,
			wantEvent: "PermissionRequest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := a.TranslateEvent([]byte(tt.input))
			if err != nil {
				t.Fatalf("TranslateEvent() error: %v", err)
			}
			if payload.Agent != "opencode" {
				t.Errorf("Agent = %q, want %q", payload.Agent, "opencode")
			}
			if payload.HookEvent != tt.wantEvent {
				t.Errorf("HookEvent = %q, want %q", payload.HookEvent, tt.wantEvent)
			}
			if tt.wantTool != "" && payload.ToolName != tt.wantTool {
				t.Errorf("ToolName = %q, want %q", payload.ToolName, tt.wantTool)
			}
		})
	}
}

func TestOpenCodeAgentTranslateEventMalformed(t *testing.T) {
	a := &OpenCodeAgent{}

	tests := []struct {
		name  string
		input string
	}{
		{"empty JSON", `{}`},
		{"invalid JSON", `not json`},
		{"missing event", `{"session_id":"abc"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := a.TranslateEvent([]byte(tt.input))
			if err == nil {
				t.Error("expected error for malformed input, got nil")
			}
		})
	}
}

func setupOpenCodeTestDir(t *testing.T) (cleanup func()) {
	t.Helper()
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "plugins", "cc-deck.ts")
	configPath := filepath.Join(dir, "opencode.json")

	origPlugin := opencodePluginPathFunc
	origConfig := opencodeConfigPathFunc
	opencodePluginPathFunc = func() string { return pluginPath }
	opencodeConfigPathFunc = func() string { return configPath }

	return func() {
		opencodePluginPathFunc = origPlugin
		opencodeConfigPathFunc = origConfig
	}
}

func TestOpenCodeAgentInstallHooks(t *testing.T) {
	cleanup := setupOpenCodeTestDir(t)
	defer cleanup()

	a := &OpenCodeAgent{}

	if err := a.InstallHooks(); err != nil {
		t.Fatalf("InstallHooks() error: %v", err)
	}

	if !a.HooksInstalled() {
		t.Error("HooksInstalled() = false after InstallHooks()")
	}

	content, err := os.ReadFile(opencodePluginPath())
	if err != nil {
		t.Fatalf("reading plugin file: %v", err)
	}

	for _, want := range []string{
		"@opencode-ai/plugin",
		"cc-deck hook --agent opencode",
		"session.created",
		"session.idle",
		"tool.execute.before",
		"tool.execute.after",
		"permission.asked",
	} {
		if !strings.Contains(string(content), want) {
			t.Errorf("plugin file missing %q", want)
		}
	}
}

func TestOpenCodeAgentInstallHooksRegistersInConfig(t *testing.T) {
	cleanup := setupOpenCodeTestDir(t)
	defer cleanup()

	a := &OpenCodeAgent{}

	if err := a.InstallHooks(); err != nil {
		t.Fatalf("InstallHooks() error: %v", err)
	}

	data, err := os.ReadFile(opencodeConfigPath())
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parsing config: %v", err)
	}

	plugins, _ := config["plugin"].([]any)
	found := false
	for _, p := range plugins {
		if s, ok := p.(string); ok && s == pluginEntry {
			found = true
		}
	}
	if !found {
		t.Errorf("plugin entry %q not found in config plugins: %v", pluginEntry, plugins)
	}
}

func TestOpenCodeAgentInstallHooksIdempotent(t *testing.T) {
	cleanup := setupOpenCodeTestDir(t)
	defer cleanup()

	a := &OpenCodeAgent{}

	if err := a.InstallHooks(); err != nil {
		t.Fatalf("first InstallHooks() error: %v", err)
	}
	if err := a.InstallHooks(); err != nil {
		t.Fatalf("second InstallHooks() error: %v", err)
	}

	if !a.HooksInstalled() {
		t.Error("HooksInstalled() = false after second InstallHooks()")
	}

	data, err := os.ReadFile(opencodeConfigPath())
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	var config map[string]any
	json.Unmarshal(data, &config)
	plugins, _ := config["plugin"].([]any)
	count := 0
	for _, p := range plugins {
		if s, ok := p.(string); ok && s == pluginEntry {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 plugin entry, got %d", count)
	}
}

func TestOpenCodeAgentInstallHooksPreservesExistingConfig(t *testing.T) {
	cleanup := setupOpenCodeTestDir(t)
	defer cleanup()

	existing := map[string]any{
		"$schema":  "https://opencode.ai/config.json",
		"model":    "google-vertex/claude-opus-4-6@20250610",
		"plugin":   []any{"some-other-plugin@latest"},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	configPath := opencodeConfigPath()
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, data, 0o644)

	a := &OpenCodeAgent{}
	if err := a.InstallHooks(); err != nil {
		t.Fatalf("InstallHooks() error: %v", err)
	}

	updated, _ := os.ReadFile(configPath)
	var config map[string]any
	json.Unmarshal(updated, &config)

	if config["model"] != "google-vertex/claude-opus-4-6@20250610" {
		t.Error("existing model config was overwritten")
	}

	plugins, _ := config["plugin"].([]any)
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d: %v", len(plugins), plugins)
	}
}

func TestOpenCodeAgentUninstallHooks(t *testing.T) {
	cleanup := setupOpenCodeTestDir(t)
	defer cleanup()

	a := &OpenCodeAgent{}

	if err := a.UninstallHooks(); err != nil {
		t.Fatalf("UninstallHooks() on nonexistent file: %v", err)
	}

	if err := a.InstallHooks(); err != nil {
		t.Fatalf("InstallHooks() error: %v", err)
	}
	if err := a.UninstallHooks(); err != nil {
		t.Fatalf("UninstallHooks() error: %v", err)
	}

	if a.HooksInstalled() {
		t.Error("HooksInstalled() = true after UninstallHooks()")
	}

	data, err := os.ReadFile(opencodeConfigPath())
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	var config map[string]any
	json.Unmarshal(data, &config)
	plugins, _ := config["plugin"].([]any)
	for _, p := range plugins {
		if s, ok := p.(string); ok && s == pluginEntry {
			t.Error("plugin entry still present in config after UninstallHooks()")
		}
	}
}

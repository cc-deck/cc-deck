package agent

import (
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

func TestOpenCodeAgentInstallHooks(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "plugins", "cc-deck.ts")

	origFunc := opencodePluginPathFunc
	opencodePluginPathFunc = func() string { return pluginPath }
	defer func() { opencodePluginPathFunc = origFunc }()

	a := &OpenCodeAgent{}

	if err := a.InstallHooks(); err != nil {
		t.Fatalf("InstallHooks() error: %v", err)
	}

	if !a.HooksInstalled() {
		t.Error("HooksInstalled() = false after InstallHooks()")
	}

	content, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("reading plugin file: %v", err)
	}

	if !strings.Contains(string(content), "@opencode-ai/plugin") {
		t.Error("plugin file missing @opencode-ai/plugin import")
	}
	if !strings.Contains(string(content), "cc-deck hook --agent opencode") {
		t.Error("plugin file missing cc-deck hook command")
	}
	if !strings.Contains(string(content), "session.next.step.started") {
		t.Error("plugin file missing session.next.step.started handler")
	}
	if !strings.Contains(string(content), "session.next.step.ended") {
		t.Error("plugin file missing session.next.step.ended handler")
	}
	if !strings.Contains(string(content), "tool.execute.before") {
		t.Error("plugin file missing tool.execute.before handler")
	}
	if !strings.Contains(string(content), "tool.execute.after") {
		t.Error("plugin file missing tool.execute.after handler")
	}
	if !strings.Contains(string(content), "permission.ask") {
		t.Error("plugin file missing permission.ask handler")
	}
}

func TestOpenCodeAgentInstallHooksIdempotent(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "plugins", "cc-deck.ts")

	origFunc := opencodePluginPathFunc
	opencodePluginPathFunc = func() string { return pluginPath }
	defer func() { opencodePluginPathFunc = origFunc }()

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
}

func TestOpenCodeAgentUninstallHooks(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "plugins", "cc-deck.ts")

	origFunc := opencodePluginPathFunc
	opencodePluginPathFunc = func() string { return pluginPath }
	defer func() { opencodePluginPathFunc = origFunc }()

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
}

package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeAgentIdentity(t *testing.T) {
	a := &ClaudeAgent{}
	if a.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", a.Name(), "claude")
	}
	if a.DisplayName() != "Claude Code" {
		t.Errorf("DisplayName() = %q, want %q", a.DisplayName(), "Claude Code")
	}
	if a.Indicator() != "✳" {
		t.Errorf("Indicator() = %q, want %q", a.Indicator(), "✳")
	}
}

func TestClaudeAgentTranslateEvent(t *testing.T) {
	a := &ClaudeAgent{}

	tests := []struct {
		name      string
		input     string
		wantEvent string
		wantTool  string
		wantAgent string
	}{
		{
			name:      "SessionStart",
			input:     `{"hook_event_name":"SessionStart","session_id":"abc"}`,
			wantEvent: "SessionStart",
			wantAgent: "claude",
		},
		{
			name:      "PreToolUse",
			input:     `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"abc"}`,
			wantEvent: "PreToolUse",
			wantTool:  "Bash",
			wantAgent: "claude",
		},
		{
			name:      "PostToolUse",
			input:     `{"hook_event_name":"PostToolUse","session_id":"abc"}`,
			wantEvent: "PostToolUse",
			wantAgent: "claude",
		},
		{
			name:      "PostToolUseFailure",
			input:     `{"hook_event_name":"PostToolUseFailure","session_id":"abc"}`,
			wantEvent: "PostToolUseFailure",
			wantAgent: "claude",
		},
		{
			name:      "UserPromptSubmit",
			input:     `{"hook_event_name":"UserPromptSubmit","session_id":"abc"}`,
			wantEvent: "UserPromptSubmit",
			wantAgent: "claude",
		},
		{
			name:      "PermissionRequest",
			input:     `{"hook_event_name":"PermissionRequest","session_id":"abc"}`,
			wantEvent: "PermissionRequest",
			wantAgent: "claude",
		},
		{
			name:      "Notification",
			input:     `{"hook_event_name":"Notification","session_id":"abc"}`,
			wantEvent: "Notification",
			wantAgent: "claude",
		},
		{
			name:      "Stop",
			input:     `{"hook_event_name":"Stop","session_id":"abc"}`,
			wantEvent: "Stop",
			wantAgent: "claude",
		},
		{
			name:      "SubagentStart",
			input:     `{"hook_event_name":"SubagentStart","session_id":"abc","agent_id":"sub-1"}`,
			wantEvent: "SubagentStart",
			wantAgent: "claude",
		},
		{
			name:      "SubagentStop",
			input:     `{"hook_event_name":"SubagentStop","session_id":"abc","agent_id":"sub-1"}`,
			wantEvent: "SubagentStop",
			wantAgent: "claude",
		},
		{
			name:      "SessionEnd",
			input:     `{"hook_event_name":"SessionEnd","session_id":"abc"}`,
			wantEvent: "SessionEnd",
			wantAgent: "claude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := a.TranslateEvent([]byte(tt.input))
			if err != nil {
				t.Fatalf("TranslateEvent() error: %v", err)
			}
			if payload.HookEvent != tt.wantEvent {
				t.Errorf("HookEvent = %q, want %q", payload.HookEvent, tt.wantEvent)
			}
			if payload.Agent != tt.wantAgent {
				t.Errorf("Agent = %q, want %q", payload.Agent, tt.wantAgent)
			}
			if tt.wantTool != "" && payload.ToolName != tt.wantTool {
				t.Errorf("ToolName = %q, want %q", payload.ToolName, tt.wantTool)
			}
		})
	}
}

func TestClaudeAgentTranslateEventMalformed(t *testing.T) {
	a := &ClaudeAgent{}

	tests := []struct {
		name  string
		input string
	}{
		{"empty JSON", `{}`},
		{"invalid JSON", `not json`},
		{"missing event name", `{"session_id":"abc"}`},
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

func TestClaudeAgentTranslateEventPreservesFields(t *testing.T) {
	a := &ClaudeAgent{}
	input := `{"hook_event_name":"PreToolUse","session_id":"sess-123","tool_name":"Read","cwd":"/home/user","agent_id":"sub-42"}`

	payload, err := a.TranslateEvent([]byte(input))
	if err != nil {
		t.Fatalf("TranslateEvent() error: %v", err)
	}
	if payload.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", payload.SessionID, "sess-123")
	}
	if payload.Cwd != "/home/user" {
		t.Errorf("Cwd = %q, want %q", payload.Cwd, "/home/user")
	}
	if payload.AgentID != "sub-42" {
		t.Errorf("AgentID = %q, want %q", payload.AgentID, "sub-42")
	}
	if payload.PaneID != 0 {
		t.Errorf("PaneID = %d, want 0 (set by hook command, not adapter)", payload.PaneID)
	}
}

func TestClaudeAgentInstallHooksIdempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")

	origFunc := claudeSettingsPathFunc
	claudeSettingsPathFunc = func() string { return settingsPath }
	defer func() { claudeSettingsPathFunc = origFunc }()

	a := &ClaudeAgent{}

	if err := a.InstallHooks(); err != nil {
		t.Fatalf("first InstallHooks() error: %v", err)
	}

	if !a.HooksInstalled() {
		t.Error("HooksInstalled() = false after InstallHooks()")
	}

	if err := a.InstallHooks(); err != nil {
		t.Fatalf("second InstallHooks() error: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parsing settings: %v", err)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	for _, event := range claudeHookEvents {
		eventHooks, _ := hooks[event].([]any)
		count := 0
		for _, h := range eventHooks {
			if isCCDeckEntry(h) {
				count++
			}
		}
		if count != 1 {
			t.Errorf("event %s has %d cc-deck entries, want 1", event, count)
		}
	}
}

func TestClaudeAgentUninstallHooksSafety(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")

	origFunc := claudeSettingsPathFunc
	claudeSettingsPathFunc = func() string { return settingsPath }
	defer func() { claudeSettingsPathFunc = origFunc }()

	a := &ClaudeAgent{}

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

func TestClaudeAgentHookEventCount(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")

	origFunc := claudeSettingsPathFunc
	claudeSettingsPathFunc = func() string { return settingsPath }
	defer func() { claudeSettingsPathFunc = origFunc }()

	a := &ClaudeAgent{}

	if count := a.HookEventCount(); count != 0 {
		t.Errorf("HookEventCount() = %d before install, want 0", count)
	}

	if err := a.InstallHooks(); err != nil {
		t.Fatalf("InstallHooks() error: %v", err)
	}

	if count := a.HookEventCount(); count != len(claudeHookEvents) {
		t.Errorf("HookEventCount() = %d, want %d", count, len(claudeHookEvents))
	}
}

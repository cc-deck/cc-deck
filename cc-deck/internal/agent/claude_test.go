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

func TestClaudeAgentCredentialSpecs(t *testing.T) {
	a := &ClaudeAgent{}
	specs := a.CredentialSpecs()
	if len(specs) < 3 {
		t.Fatalf("CredentialSpecs() returned %d specs, want at least 3", len(specs))
	}
	names := make(map[string]bool)
	for _, s := range specs {
		names[s.Name] = true
	}
	for _, required := range []string{"api", "vertex", "bedrock"} {
		if !names[required] {
			t.Errorf("missing %q credential spec", required)
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

func TestClaudeAgentInstallHooksPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")

	origFunc := claudeSettingsPathFunc
	claudeSettingsPathFunc = func() string { return settingsPath }
	defer func() { claudeSettingsPathFunc = origFunc }()

	// Pre-populate with a non-cc-deck hook on SessionEnd
	initial := map[string]any{
		"hooks": map[string]any{
			"SessionEnd": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "~/.claude/hooks/sync-memory-to-obsidian.sh",
						},
					},
				},
			},
		},
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}

	a := &ClaudeAgent{}
	if err := a.InstallHooks(); err != nil {
		t.Fatalf("InstallHooks() error: %v", err)
	}

	result, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(result, &settings); err != nil {
		t.Fatalf("parsing settings: %v", err)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	sessionEnd, _ := hooks["SessionEnd"].([]any)

	// Must have exactly 2 entries: the obsidian hook + cc-deck hook
	if len(sessionEnd) != 2 {
		t.Fatalf("SessionEnd has %d entries, want 2; got: %v", len(sessionEnd), sessionEnd)
	}

	// Verify the obsidian hook survived
	foundObsidian := false
	for _, entry := range sessionEnd {
		m, _ := entry.(map[string]any)
		hooksArr, _ := m["hooks"].([]any)
		for _, h := range hooksArr {
			action, _ := h.(map[string]any)
			if cmd, _ := action["command"].(string); cmd == "~/.claude/hooks/sync-memory-to-obsidian.sh" {
				foundObsidian = true
			}
		}
	}
	if !foundObsidian {
		t.Error("obsidian sync hook was lost after InstallHooks()")
	}

	// Verify PreToolUse has the rtk hook + cc-deck hook (not just cc-deck)
	preToolUse, _ := hooks["PreToolUse"].([]any)
	ccDeckCount := 0
	for _, entry := range preToolUse {
		if isCCDeckEntry(entry) {
			ccDeckCount++
		}
	}
	if ccDeckCount != 1 {
		t.Errorf("PreToolUse has %d cc-deck entries, want 1", ccDeckCount)
	}

	// Run again to verify idempotency with existing hooks
	if err := a.InstallHooks(); err != nil {
		t.Fatalf("second InstallHooks() error: %v", err)
	}

	result, _ = os.ReadFile(settingsPath)
	json.Unmarshal(result, &settings) //nolint:errcheck
	hooks, _ = settings["hooks"].(map[string]any)
	sessionEnd, _ = hooks["SessionEnd"].([]any)

	if len(sessionEnd) != 2 {
		t.Fatalf("after second install, SessionEnd has %d entries, want 2", len(sessionEnd))
	}

	// Verify other settings keys are preserved
	if _, exists := settings["hooks"]; !exists {
		t.Error("hooks key missing after install")
	}
}

func TestClaudeAgentInstallHooksPreservesTopLevelKeys(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")

	origFunc := claudeSettingsPathFunc
	claudeSettingsPathFunc = func() string { return settingsPath }
	defer func() { claudeSettingsPathFunc = origFunc }()

	// Pre-populate with permissions and allowedTools alongside hooks
	initial := map[string]any{
		"permissions": map[string]any{
			"allow": []any{"Bash(*)", "Read(*)"},
		},
		"allowedTools": []any{"Bash", "Read"},
		"hooks": map[string]any{
			"SessionEnd": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "~/.claude/hooks/sync-memory-to-obsidian.sh",
						},
					},
				},
			},
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "rtk hook claude",
						},
					},
				},
			},
		},
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	a := &ClaudeAgent{}
	if err := a.InstallHooks(); err != nil {
		t.Fatalf("InstallHooks() error: %v", err)
	}

	result, _ := os.ReadFile(settingsPath)
	var settings map[string]any
	json.Unmarshal(result, &settings) //nolint:errcheck

	// permissions must survive
	perms, _ := settings["permissions"].(map[string]any)
	if perms == nil {
		t.Fatal("permissions key was lost")
	}
	allow, _ := perms["allow"].([]any)
	if len(allow) != 2 {
		t.Errorf("permissions.allow has %d entries, want 2", len(allow))
	}

	// allowedTools must survive
	tools, _ := settings["allowedTools"].([]any)
	if len(tools) != 2 {
		t.Errorf("allowedTools has %d entries, want 2", len(tools))
	}

	// rtk hook on PreToolUse must survive alongside cc-deck hook
	hooks, _ := settings["hooks"].(map[string]any)
	preToolUse, _ := hooks["PreToolUse"].([]any)
	foundRTK := false
	for _, entry := range preToolUse {
		m, _ := entry.(map[string]any)
		hooksArr, _ := m["hooks"].([]any)
		for _, h := range hooksArr {
			action, _ := h.(map[string]any)
			if cmd, _ := action["command"].(string); cmd == "rtk hook claude" {
				foundRTK = true
			}
		}
	}
	if !foundRTK {
		t.Error("rtk hook was lost from PreToolUse after InstallHooks()")
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

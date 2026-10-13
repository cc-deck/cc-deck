package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCodexAgentIdentity(t *testing.T) {
	a := &CodexAgent{}
	if a.Name() != "codex" {
		t.Errorf("Name() = %q, want %q", a.Name(), "codex")
	}
	if a.DisplayName() != "Codex CLI" {
		t.Errorf("DisplayName() = %q, want %q", a.DisplayName(), "Codex CLI")
	}
	if a.Indicator() != "◆" {
		t.Errorf("Indicator() = %q, want %q", a.Indicator(), "◆")
	}
}

func TestCodexAgentInstallHooks(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".codex")
	hooksPath := filepath.Join(configDir, "hooks.json")

	origFunc := codexHooksPathFunc
	codexHooksPathFunc = func() string { return hooksPath }
	defer func() { codexHooksPathFunc = origFunc }()

	a := &CodexAgent{}

	if err := a.InstallHooksAt(configDir); err != nil {
		t.Fatalf("InstallHooksAt() error: %v", err)
	}

	if !a.HooksInstalled() {
		t.Error("HooksInstalled() = false after InstallHooks()")
	}

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("reading hooks.json: %v", err)
	}

	var hooksFile map[string]any
	if err := json.Unmarshal(data, &hooksFile); err != nil {
		t.Fatalf("parsing hooks.json: %v", err)
	}

	hooks, _ := hooksFile["hooks"].(map[string]any)
	if hooks == nil {
		t.Fatal("hooks.json missing 'hooks' wrapper key")
	}

	for _, event := range codexHookEvents {
		eventHooks, ok := hooks[event].([]any)
		if !ok {
			t.Errorf("event %s not found in hooks.json", event)
			continue
		}
		if len(eventHooks) != 1 {
			t.Errorf("event %s has %d entries, want 1", event, len(eventHooks))
			continue
		}
		entry, _ := eventHooks[0].(map[string]any)
		hooksArr, _ := entry["hooks"].([]any)
		if len(hooksArr) != 1 {
			t.Errorf("event %s hook entry has %d actions, want 1", event, len(hooksArr))
			continue
		}
		action, _ := hooksArr[0].(map[string]any)
		if cmd, _ := action["command"].(string); cmd != codexHookCommand() {
			t.Errorf("event %s command = %q, want %q", event, cmd, codexHookCommand())
		}
	}

	if len(hooks) != len(codexHookEvents) {
		t.Errorf("hooks.json has %d events, want %d", len(hooks), len(codexHookEvents))
	}
}

func TestCodexAgentInstallHooksIdempotent(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".codex")
	hooksPath := filepath.Join(configDir, "hooks.json")

	origFunc := codexHooksPathFunc
	codexHooksPathFunc = func() string { return hooksPath }
	defer func() { codexHooksPathFunc = origFunc }()

	a := &CodexAgent{}

	if err := a.InstallHooksAt(configDir); err != nil {
		t.Fatalf("first InstallHooksAt() error: %v", err)
	}

	if err := a.InstallHooksAt(configDir); err != nil {
		t.Fatalf("second InstallHooksAt() error: %v", err)
	}

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("reading hooks.json: %v", err)
	}

	var hooksFile map[string]any
	if err := json.Unmarshal(data, &hooksFile); err != nil {
		t.Fatalf("parsing hooks.json: %v", err)
	}
	hooks, _ := hooksFile["hooks"].(map[string]any)
	for _, event := range codexHookEvents {
		eventHooks, _ := hooks[event].([]any)
		count := 0
		for _, h := range eventHooks {
			if isCCDeckEntry(h) {
				count++
			}
		}
		if count != 1 {
			t.Errorf("event %s has %d cc-deck entries after double install, want 1", event, count)
		}
	}
}

func TestCodexAgentInstallHooksPreservesOtherHooks(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".codex")
	hooksPath := filepath.Join(configDir, "hooks.json")

	origFunc := codexHooksPathFunc
	codexHooksPathFunc = func() string { return hooksPath }
	defer func() { codexHooksPathFunc = origFunc }()

	initial := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "my-custom-tool notify",
						},
					},
				},
			},
		},
	}
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(hooksPath, data, 0o644); err != nil {
		t.Fatalf("write initial hooks.json: %v", err)
	}

	a := &CodexAgent{}
	if err := a.InstallHooksAt(configDir); err != nil {
		t.Fatalf("InstallHooksAt() error: %v", err)
	}

	result, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("reading hooks.json: %v", err)
	}
	var hooksFile map[string]any
	if err := json.Unmarshal(result, &hooksFile); err != nil {
		t.Fatalf("parsing hooks.json: %v", err)
	}
	hooks, _ := hooksFile["hooks"].(map[string]any)
	sessionStart, _ := hooks["SessionStart"].([]any)

	if len(sessionStart) != 2 {
		t.Fatalf("SessionStart has %d entries, want 2 (custom + cc-deck)", len(sessionStart))
	}

	foundCustom := false
	for _, entry := range sessionStart {
		m, _ := entry.(map[string]any)
		hooksArr, _ := m["hooks"].([]any)
		for _, h := range hooksArr {
			action, _ := h.(map[string]any)
			if cmd, _ := action["command"].(string); cmd == "my-custom-tool notify" {
				foundCustom = true
			}
		}
	}
	if !foundCustom {
		t.Error("custom hook was lost after InstallHooks()")
	}
}

func TestCodexAgentInstallHooksCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".codex")
	hooksPath := filepath.Join(configDir, "hooks.json")

	origFunc := codexHooksPathFunc
	codexHooksPathFunc = func() string { return hooksPath }
	defer func() { codexHooksPathFunc = origFunc }()

	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(hooksPath, []byte("not valid json{{{"), 0o644); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}

	a := &CodexAgent{}
	err := a.InstallHooksAt(configDir)
	if err == nil {
		t.Fatal("expected error for corrupted hooks.json, got nil")
	}

	data, _ := os.ReadFile(hooksPath)
	if string(data) != "not valid json{{{" {
		t.Error("corrupted file was modified; InstallHooks must not overwrite invalid files")
	}
}

func TestCodexAgentUninstallHooks(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".codex")
	hooksPath := filepath.Join(configDir, "hooks.json")

	origFunc := codexHooksPathFunc
	codexHooksPathFunc = func() string { return hooksPath }
	defer func() { codexHooksPathFunc = origFunc }()

	a := &CodexAgent{}

	if err := a.InstallHooksAt(configDir); err != nil {
		t.Fatalf("InstallHooksAt() error: %v", err)
	}
	if err := a.UninstallHooks(); err != nil {
		t.Fatalf("UninstallHooks() error: %v", err)
	}
	if a.HooksInstalled() {
		t.Error("HooksInstalled() = true after UninstallHooks()")
	}

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("reading hooks.json after uninstall: %v", err)
	}
	var hooksFile map[string]any
	if err := json.Unmarshal(data, &hooksFile); err != nil {
		t.Fatalf("parsing hooks.json after uninstall: %v", err)
	}
	if hooks, ok := hooksFile["hooks"]; ok {
		hooksMap, _ := hooks.(map[string]any)
		if len(hooksMap) > 0 {
			t.Errorf("hooks key should be empty or absent after uninstall, got %d events", len(hooksMap))
		}
	}
}

func TestCodexAgentUninstallHooksPreservesOther(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, ".codex", "hooks.json")

	origFunc := codexHooksPathFunc
	codexHooksPathFunc = func() string { return hooksPath }
	defer func() { codexHooksPathFunc = origFunc }()

	initial := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "my-custom-tool notify",
						},
					},
				},
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "cc-deck hook --agent codex --pane-id \"$ZELLIJ_PANE_ID\"",
						},
					},
				},
			},
		},
	}
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(hooksPath, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	a := &CodexAgent{}
	if err := a.UninstallHooks(); err != nil {
		t.Fatalf("UninstallHooks() error: %v", err)
	}

	result, _ := os.ReadFile(hooksPath)
	var hooksFile map[string]any
	json.Unmarshal(result, &hooksFile) //nolint:errcheck
	hooks, _ := hooksFile["hooks"].(map[string]any)
	sessionStart, _ := hooks["SessionStart"].([]any)

	if len(sessionStart) != 1 {
		t.Fatalf("SessionStart has %d entries, want 1 (custom only)", len(sessionStart))
	}

	m, _ := sessionStart[0].(map[string]any)
	hooksArr, _ := m["hooks"].([]any)
	action, _ := hooksArr[0].(map[string]any)
	if cmd, _ := action["command"].(string); cmd != "my-custom-tool notify" {
		t.Errorf("remaining hook command = %q, want custom tool", cmd)
	}
}

func TestCodexAgentUninstallHooksNoFile(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, ".codex", "hooks.json")

	origFunc := codexHooksPathFunc
	codexHooksPathFunc = func() string { return hooksPath }
	defer func() { codexHooksPathFunc = origFunc }()

	a := &CodexAgent{}
	if err := a.UninstallHooks(); err != nil {
		t.Fatalf("UninstallHooks() on nonexistent file: %v", err)
	}
}

func TestCodexAgentHooksInstalled(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".codex")
	hooksPath := filepath.Join(configDir, "hooks.json")

	origFunc := codexHooksPathFunc
	codexHooksPathFunc = func() string { return hooksPath }
	defer func() { codexHooksPathFunc = origFunc }()

	a := &CodexAgent{}

	if a.HooksInstalled() {
		t.Error("HooksInstalled() = true before install")
	}

	if err := a.InstallHooksAt(configDir); err != nil {
		t.Fatalf("InstallHooksAt() error: %v", err)
	}

	if !a.HooksInstalled() {
		t.Error("HooksInstalled() = false after install")
	}
}

func TestCodexAgentTranslateEvent(t *testing.T) {
	a := &CodexAgent{}

	tests := []struct {
		name      string
		input     string
		wantEvent string
		wantTool  string
	}{
		{
			name:      "SessionStart",
			input:     `{"hook_event_name":"SessionStart","session_id":"abc"}`,
			wantEvent: "SessionStart",
		},
		{
			name:      "PreToolUse",
			input:     `{"hook_event_name":"PreToolUse","tool_name":"apply_patch","session_id":"abc"}`,
			wantEvent: "PreToolUse",
			wantTool:  "apply_patch",
		},
		{
			name:      "PostToolUse",
			input:     `{"hook_event_name":"PostToolUse","session_id":"abc"}`,
			wantEvent: "PostToolUse",
		},
		{
			name:      "PermissionRequest",
			input:     `{"hook_event_name":"PermissionRequest","session_id":"abc"}`,
			wantEvent: "PermissionRequest",
		},
		{
			name:      "UserPromptSubmit",
			input:     `{"hook_event_name":"UserPromptSubmit","session_id":"abc"}`,
			wantEvent: "UserPromptSubmit",
		},
		{
			name:      "Stop",
			input:     `{"hook_event_name":"Stop","session_id":"abc"}`,
			wantEvent: "Stop",
		},
		{
			name:      "SubagentStart",
			input:     `{"hook_event_name":"SubagentStart","session_id":"abc"}`,
			wantEvent: "SubagentStart",
		},
		{
			name:      "SubagentStop",
			input:     `{"hook_event_name":"SubagentStop","session_id":"abc"}`,
			wantEvent: "SubagentStop",
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
			if payload.Agent != "codex" {
				t.Errorf("Agent = %q, want %q", payload.Agent, "codex")
			}
			if payload.ToolName != tt.wantTool {
				t.Errorf("ToolName = %q, want %q", payload.ToolName, tt.wantTool)
			}
		})
	}
}

func TestCodexAgentTranslateEventPreservesFields(t *testing.T) {
	a := &CodexAgent{}
	input := `{"hook_event_name":"PreToolUse","session_id":"sess-456","tool_name":"shell","cwd":"/workspace"}`

	payload, err := a.TranslateEvent([]byte(input))
	if err != nil {
		t.Fatalf("TranslateEvent() error: %v", err)
	}
	if payload.SessionID != "sess-456" {
		t.Errorf("SessionID = %q, want %q", payload.SessionID, "sess-456")
	}
	if payload.Cwd != "/workspace" {
		t.Errorf("Cwd = %q, want %q", payload.Cwd, "/workspace")
	}
	if payload.PaneID != 0 {
		t.Errorf("PaneID = %d, want 0 (set by hook command, not adapter)", payload.PaneID)
	}
}

func TestCodexAgentTranslateEventIgnoresExtraFields(t *testing.T) {
	a := &CodexAgent{}
	input := `{"hook_event_name":"SessionStart","session_id":"abc","model":"o3-pro","extra_field":"ignored"}`

	payload, err := a.TranslateEvent([]byte(input))
	if err != nil {
		t.Fatalf("TranslateEvent() error: %v", err)
	}
	if payload.HookEvent != "SessionStart" {
		t.Errorf("HookEvent = %q, want %q", payload.HookEvent, "SessionStart")
	}
}

func TestCodexAgentTranslateEventMalformed(t *testing.T) {
	a := &CodexAgent{}

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

func TestCodexAgentCredentialSpecs(t *testing.T) {
	a := &CodexAgent{}
	specs := a.CredentialSpecs()
	if len(specs) != 1 {
		t.Fatalf("CredentialSpecs() returned %d specs, want 1", len(specs))
	}
	if specs[0].Name != "openai" {
		t.Errorf("spec name = %q, want %q", specs[0].Name, "openai")
	}

	foundKey := false
	for _, env := range specs[0].EnvVars {
		if env.Name == "OPENAI_API_KEY" && env.Required {
			foundKey = true
		}
	}
	if !foundKey {
		t.Error("OPENAI_API_KEY not found as required env var in openai spec")
	}
}

func TestCodexAgentRequiredDomainGroups(t *testing.T) {
	a := &CodexAgent{}
	groups := a.RequiredDomainGroups()
	if len(groups) != 1 {
		t.Fatalf("RequiredDomainGroups() returned %d groups, want 1", len(groups))
	}
	if groups[0] != "openai" {
		t.Errorf("RequiredDomainGroups()[0] = %q, want %q", groups[0], "openai")
	}
}

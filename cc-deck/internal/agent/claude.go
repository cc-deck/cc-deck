package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ClaudeAgent implements the Agent interface for Claude Code.
type ClaudeAgent struct{}

func init() {
	Register(&ClaudeAgent{})
}

func (c *ClaudeAgent) Name() string        { return "claude" }
func (c *ClaudeAgent) DisplayName() string { return "Claude Code" }
func (c *ClaudeAgent) Indicator() string   { return "CC" }

func (c *ClaudeAgent) IsInstalled() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

func (c *ClaudeAgent) DetectConfig() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".claude")
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	return dir
}

func (c *ClaudeAgent) InstallHooks() error {
	settingsPath := claudeSettingsPath()
	settings, err := readClaudeSettings(settingsPath)
	if err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	for _, event := range claudeHookEvents {
		entry := claudeHookEntry(event)
		entryMap := structToMap(entry)

		eventHooks, _ := hooks[event].([]any)
		eventHooks = removeCCDeckHooks(eventHooks)
		eventHooks = append(eventHooks, entryMap)
		hooks[event] = eventHooks
	}

	settings["hooks"] = hooks
	return writeClaudeSettings(settingsPath, settings)
}

func (c *ClaudeAgent) UninstallHooks() error {
	settingsPath := claudeSettingsPath()
	settings, err := readClaudeSettings(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}

	changed := false
	for event, val := range hooks {
		eventHooks, ok := val.([]any)
		if !ok {
			continue
		}
		filtered := removeCCDeckHooks(eventHooks)
		if len(filtered) != len(eventHooks) {
			changed = true
			if len(filtered) == 0 {
				delete(hooks, event)
			} else {
				hooks[event] = filtered
			}
		}
	}

	if !changed {
		return nil
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}

	return writeClaudeSettings(settingsPath, settings)
}

func (c *ClaudeAgent) HooksInstalled() bool {
	settingsPath := claudeSettingsPath()
	settings, err := readClaudeSettings(settingsPath)
	if err != nil {
		return false
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return false
	}
	for _, val := range hooks {
		eventHooks, ok := val.([]any)
		if !ok {
			continue
		}
		if containsCCDeckHook(eventHooks) {
			return true
		}
	}
	return false
}

// claudeHookPayload is the JSON structure from Claude Code hook events.
type claudeHookPayload struct {
	SessionID string `json:"session_id,omitempty"`
	HookEvent string `json:"hook_event_name"`
	ToolName  string `json:"tool_name,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
}

func (c *ClaudeAgent) TranslateEvent(input []byte) (*NormalizedPayload, error) {
	var hook claudeHookPayload
	if err := json.Unmarshal(input, &hook); err != nil {
		return nil, fmt.Errorf("parsing Claude Code hook payload: %w", err)
	}
	if hook.HookEvent == "" {
		return nil, fmt.Errorf("missing hook_event_name in Claude Code payload")
	}
	return &NormalizedPayload{
		Agent:     c.Name(),
		SessionID: hook.SessionID,
		HookEvent: hook.HookEvent,
		ToolName:  hook.ToolName,
		Cwd:       hook.CWD,
		AgentID:   hook.AgentID,
	}, nil
}

// HookEventCount returns how many hook events are registered for cc-deck.
func (c *ClaudeAgent) HookEventCount() int {
	settingsPath := claudeSettingsPath()
	settings, err := readClaudeSettings(settingsPath)
	if err != nil {
		return 0
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return 0
	}
	count := 0
	for _, val := range hooks {
		eventHooks, ok := val.([]any)
		if !ok {
			continue
		}
		if containsCCDeckHook(eventHooks) {
			count++
		}
	}
	return count
}

// --- Claude Code settings.json management ---

var claudeHookEvents = []string{
	"SessionStart",
	"PreToolUse",
	"PostToolUse",
	"PostToolUseFailure",
	"UserPromptSubmit",
	"PermissionRequest",
	"Notification",
	"Stop",
	"SubagentStop",
	"SubagentStart",
	"SessionEnd",
}

// hookEventsWithMatcher lists events that support the matcher field.
var hookEventsWithMatcher = map[string]bool{
	"PermissionRequest": true,
	"Notification":      true,
}

type hookEntry struct {
	Matcher *string      `json:"matcher,omitempty"`
	Hooks   []hookAction `json:"hooks"`
}

type hookAction struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

func ccDeckHookCommand() string {
	return "cc-deck hook --agent claude --pane-id \"$ZELLIJ_PANE_ID\""
}

func claudeHookEntry(event string) hookEntry {
	entry := hookEntry{
		Hooks: []hookAction{{Type: "command", Command: ccDeckHookCommand()}},
	}
	if hookEventsWithMatcher[event] {
		m := ""
		entry.Matcher = &m
	}
	return entry
}

var claudeSettingsPathFunc = defaultClaudeSettingsPath

func claudeSettingsPath() string {
	return claudeSettingsPathFunc()
}

func defaultClaudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func containsCCDeckHook(hooks []any) bool {
	for _, h := range hooks {
		if isCCDeckEntry(h) {
			return true
		}
	}
	return false
}

func removeCCDeckHooks(hooks []any) []any {
	var result []any
	for _, h := range hooks {
		if !isCCDeckEntry(h) {
			result = append(result, h)
		}
	}
	return result
}

func isCCDeckEntry(entry any) bool {
	const prefix = "cc-deck hook"
	if m, ok := entry.(map[string]any); ok {
		hooksArr, _ := m["hooks"].([]any)
		for _, h := range hooksArr {
			if action, ok := h.(map[string]any); ok {
				if cmd, ok := action["command"].(string); ok && strings.HasPrefix(cmd, prefix) {
					return true
				}
			}
		}
	}
	if s, ok := entry.(string); ok && strings.HasPrefix(s, prefix) {
		return true
	}
	return false
}

func structToMap(v any) map[string]any {
	data, _ := json.Marshal(v)
	var result map[string]any
	json.Unmarshal(data, &result) //nolint:errcheck
	return result
}

func readClaudeSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, fmt.Errorf("reading settings: %w", err)
	}
	if len(data) == 0 {
		return make(map[string]any), nil
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing settings.json: %w", err)
	}
	return settings, nil
}

func writeClaudeSettings(path string, settings map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}
	data = append(data, '\n')
	return atomicWriteFile(path, data, 0o644)
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".cc-deck-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

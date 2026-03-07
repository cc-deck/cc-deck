// T019: settings.json hook management (add/remove cc-deck hooks)
// Uses the new matcher-based hook format required by Claude Code 2.1+

package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeSettingsPath returns the path to ~/.claude/settings.json.
func ClaudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func ccDeckHookCommand() string {
	return "cc-deck hook --pane-id \"$ZELLIJ_PANE_ID\""
}

var hookEvents = []string{
	"SessionStart",
	"PreToolUse",
	"PostToolUse",
	"Notification",
	"Stop",
	"SubagentStop",
	"SessionEnd",
}

type hookEntry struct {
	Matcher *string      `json:"matcher,omitempty"`
	Hooks   []hookAction `json:"hooks"`
}

type hookAction struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// hookEventsWithMatcher lists events that support the matcher field.
var hookEventsWithMatcher = map[string]bool{
	"PreToolUse":  true,
	"PostToolUse": true,
	"Notification": true,
}

func ccDeckHookEntry(event string) hookEntry {
	entry := hookEntry{
		Hooks: []hookAction{{Type: "command", Command: ccDeckHookCommand()}},
	}
	if hookEventsWithMatcher[event] {
		m := ""
		entry.Matcher = &m
	}
	return entry
}

// RegisterHooks adds cc-deck hook entries to settings.json.
// Removes any old-format entries and adds new matcher-based format.
func RegisterHooks(settingsPath string) error {
	settings, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	for _, event := range hookEvents {
		entry := ccDeckHookEntry(event)
		entryMap := structToMap(entry)

		eventHooks, _ := hooks[event].([]any)
		// Remove old-format or duplicate entries, then add fresh
		eventHooks = removeCCDeckHooks(eventHooks)
		eventHooks = append(eventHooks, entryMap)
		hooks[event] = eventHooks
	}

	settings["hooks"] = hooks
	return writeSettings(settingsPath, settings)
}

// RemoveHooks removes only cc-deck hook entries from settings.json.
func RemoveHooks(settingsPath string) error {
	settings, err := readSettings(settingsPath)
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

	return writeSettings(settingsPath, settings)
}

// HasHooks returns true if settings.json contains cc-deck hooks.
func HasHooks(settingsPath string) bool {
	settings, err := readSettings(settingsPath)
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

// HookEventCount returns how many hook events are registered for cc-deck.
func HookEventCount(settingsPath string) int {
	settings, err := readSettings(settingsPath)
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

func containsCCDeckHook(hooks []any) bool {
	cmd := ccDeckHookCommand()
	for _, h := range hooks {
		if isCCDeckEntry(h, cmd) {
			return true
		}
	}
	return false
}

func removeCCDeckHooks(hooks []any) []any {
	cmd := ccDeckHookCommand()
	var result []any
	for _, h := range hooks {
		if !isCCDeckEntry(h, cmd) {
			result = append(result, h)
		}
	}
	return result
}

// isCCDeckEntry checks both new format (object) and old format (string).
// Matches any command starting with "cc-deck hook" to catch old and new variants.
func isCCDeckEntry(entry any, _ string) bool {
	const prefix = "cc-deck hook"
	// New format: {"matcher": "", "hooks": [{"type": "command", "command": "..."}]}
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
	// Old format: plain string
	if s, ok := entry.(string); ok && strings.HasPrefix(s, prefix) {
		return true
	}
	return false
}

func structToMap(v any) map[string]any {
	data, _ := json.Marshal(v)
	var result map[string]any
	json.Unmarshal(data, &result)
	return result
}

func readSettings(path string) (map[string]any, error) {
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

func writeSettings(path string, settings map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}
	data = append(data, '\n')
	return atomicWrite(path, data, 0o644)
}

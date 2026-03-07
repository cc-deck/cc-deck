// T019: settings.json hook management (add/remove cc-deck hooks)

package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeSettingsPath returns the path to ~/.claude/settings.json.
func ClaudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

// ccDeckHookCommand returns the hook command to register.
func ccDeckHookCommand() string {
	return "cc-deck hook"
}

// hookEvents lists all Claude Code hook event types to register.
var hookEvents = []string{
	"PreToolUse",
	"PostToolUse",
	"Notification",
	"Stop",
	"SubagentStop",
}

// RegisterHooks adds cc-deck hook entries to settings.json.
// Creates the file if it doesn't exist.
// Preserves all existing content.
func RegisterHooks(settingsPath string) error {
	settings, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	hookCmd := ccDeckHookCommand()

	for _, event := range hookEvents {
		eventHooks, _ := hooks[event].([]interface{})
		if !containsHook(eventHooks, hookCmd) {
			eventHooks = append(eventHooks, hookCmd)
			hooks[event] = eventHooks
		}
	}

	settings["hooks"] = hooks
	return writeSettings(settingsPath, settings)
}

// RemoveHooks removes only cc-deck hook entries from settings.json.
// Preserves all other hooks and settings.
func RemoveHooks(settingsPath string) error {
	settings, err := readSettings(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to remove
		}
		return err
	}

	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		return nil
	}

	hookCmd := ccDeckHookCommand()
	changed := false

	for event, val := range hooks {
		eventHooks, ok := val.([]interface{})
		if !ok {
			continue
		}
		filtered := removeHook(eventHooks, hookCmd)
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

	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		return false
	}

	hookCmd := ccDeckHookCommand()
	for _, val := range hooks {
		eventHooks, ok := val.([]interface{})
		if !ok {
			continue
		}
		if containsHook(eventHooks, hookCmd) {
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

	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		return 0
	}

	hookCmd := ccDeckHookCommand()
	count := 0
	for _, val := range hooks {
		eventHooks, ok := val.([]interface{})
		if !ok {
			continue
		}
		if containsHook(eventHooks, hookCmd) {
			count++
		}
	}
	return count
}

func readSettings(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("reading settings: %w", err)
	}

	if len(data) == 0 {
		return make(map[string]interface{}), nil
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing settings.json: %w", err)
	}
	return settings, nil
}

func writeSettings(path string, settings map[string]interface{}) error {
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

func containsHook(hooks []interface{}, cmd string) bool {
	for _, h := range hooks {
		if s, ok := h.(string); ok && s == cmd {
			return true
		}
	}
	return false
}

func removeHook(hooks []interface{}, cmd string) []interface{} {
	var result []interface{}
	for _, h := range hooks {
		if s, ok := h.(string); ok && s == cmd {
			continue
		}
		result = append(result, h)
	}
	return result
}

package plugin

import (
	"github.com/cc-deck/cc-deck/internal/agent"
)

// ClaudeSettingsPath returns the path to ~/.claude/settings.json.
// Delegates to the ClaudeAgent adapter.
func ClaudeSettingsPath() string {
	a := agent.Get("claude")
	if a == nil {
		return ""
	}
	dir := a.DetectConfig()
	if dir == "" {
		return ""
	}
	return dir + "/settings.json"
}

// RegisterHooks adds cc-deck hook entries to settings.json.
// Delegates to the ClaudeAgent adapter.
func RegisterHooks(settingsPath string) error {
	a := agent.Get("claude")
	if a == nil {
		return nil
	}
	return a.InstallHooks()
}

// RemoveHooks removes only cc-deck hook entries from settings.json.
// Delegates to the ClaudeAgent adapter.
func RemoveHooks(settingsPath string) error {
	a := agent.Get("claude")
	if a == nil {
		return nil
	}
	return a.UninstallHooks()
}

// HasHooks returns true if settings.json contains cc-deck hooks.
// Delegates to the ClaudeAgent adapter.
func HasHooks(settingsPath string) bool {
	a := agent.Get("claude")
	if a == nil {
		return false
	}
	return a.HooksInstalled()
}

// HookEventCount returns how many hook events are registered for cc-deck.
func HookEventCount(settingsPath string) int {
	a := agent.Get("claude")
	if a == nil {
		return 0
	}
	if ca, ok := a.(*agent.ClaudeAgent); ok {
		return ca.HookEventCount()
	}
	return 0
}

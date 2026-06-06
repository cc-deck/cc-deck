package agent

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed opencode_plugin.ts
var opencodePluginTemplate []byte

// OpenCodeAgent implements the Agent interface for OpenCode.
type OpenCodeAgent struct{}

func init() {
	Register(&OpenCodeAgent{})
}

func (o *OpenCodeAgent) Name() string        { return "opencode" }
func (o *OpenCodeAgent) DisplayName() string { return "OpenCode" }
func (o *OpenCodeAgent) Indicator() string   { return "OC" }

func (o *OpenCodeAgent) IsInstalled() bool {
	_, err := exec.LookPath("opencode")
	return err == nil
}

func (o *OpenCodeAgent) DetectConfig() string {
	dir := opencodeConfigDir()
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	return dir
}

func (o *OpenCodeAgent) InstallHooks() error {
	pluginPath := opencodePluginPath()
	dir := filepath.Dir(pluginPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating OpenCode plugins directory %s: %w", dir, err)
	}
	if err := atomicWriteFile(pluginPath, opencodePluginTemplate, 0o644); err != nil {
		return fmt.Errorf("writing OpenCode plugin: %w", err)
	}
	return nil
}

func (o *OpenCodeAgent) UninstallHooks() error {
	pluginPath := opencodePluginPath()
	if err := os.Remove(pluginPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("removing OpenCode plugin: %w", err)
	}
	return nil
}

func (o *OpenCodeAgent) HooksInstalled() bool {
	pluginPath := opencodePluginPath()
	_, err := os.Stat(pluginPath)
	return err == nil
}

// opencodeHookPayload represents the input format when OpenCode calls
// cc-deck hook --agent opencode. The TypeScript plugin sends pre-mapped
// event names, so TranslateEvent just wraps them.
type opencodeHookPayload struct {
	HookEvent string `json:"hook_event_name"`
	SessionID string `json:"session_id,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
}

func (o *OpenCodeAgent) TranslateEvent(input []byte) (*NormalizedPayload, error) {
	var hook opencodeHookPayload
	if err := json.Unmarshal(input, &hook); err != nil {
		return nil, fmt.Errorf("parsing OpenCode hook payload: %w", err)
	}
	if hook.HookEvent == "" {
		return nil, fmt.Errorf("missing hook_event_name in OpenCode payload")
	}
	return &NormalizedPayload{
		Agent:     o.Name(),
		SessionID: hook.SessionID,
		HookEvent: hook.HookEvent,
		ToolName:  hook.ToolName,
	}, nil
}

var opencodeConfigDirFunc = defaultOpencodeConfigDir

func opencodeConfigDir() string {
	return opencodeConfigDirFunc()
}

func defaultOpencodeConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode")
}

var opencodePluginPathFunc = defaultOpencodePluginPath

func opencodePluginPath() string {
	return opencodePluginPathFunc()
}

func defaultOpencodePluginPath() string {
	return filepath.Join(opencodeConfigDir(), "plugins", "cc-deck.ts")
}

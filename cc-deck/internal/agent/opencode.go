package agent

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cc-deck/cc-deck/internal/fileutil"
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
func (o *OpenCodeAgent) Indicator() string   { return "▶" } // ▶

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
	if err := fileutil.AtomicWrite(pluginPath, opencodePluginTemplate, 0o644); err != nil {
		return fmt.Errorf("writing OpenCode plugin: %w", err)
	}
	if err := registerPluginInConfig(pluginPath); err != nil {
		return fmt.Errorf("registering plugin in opencode config: %w", err)
	}
	return nil
}

func (o *OpenCodeAgent) UninstallHooks() error {
	pluginPath := opencodePluginPath()
	_ = unregisterPluginFromConfig(pluginPath)
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
	Cwd       string `json:"cwd,omitempty"`
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
		Cwd:       hook.Cwd,
	}, nil
}

func (o *OpenCodeAgent) CredentialSpecs() []CredentialSpec {
	return []CredentialSpec{
		{
			Name:     "openai",
			Priority: 10,
			EnvVars: []EnvVarSpec{
				{Name: "OPENAI_API_KEY", Required: true},
			},
		},
		{
			Name:     "anthropic",
			Priority: 20,
			EnvVars: []EnvVarSpec{
				{Name: "ANTHROPIC_API_KEY", Required: true},
			},
		},
	}
}

// --- OpenCode config (opencode.json) management ---

const pluginEntry = "~/.config/opencode/plugins/cc-deck.ts"

// registerPluginInConfig adds the cc-deck plugin to the "plugin" array
// in opencode.json if not already present.
func registerPluginInConfig(pluginPath string) error {
	configPath := opencodeConfigPath()
	config, err := readOpencodeConfig(configPath)
	if err != nil {
		return err
	}

	plugins, _ := config["plugin"].([]any)

	for _, p := range plugins {
		if s, ok := p.(string); ok && (s == pluginEntry || s == pluginPath) {
			return nil
		}
	}

	plugins = append(plugins, pluginEntry)
	config["plugin"] = plugins
	return writeOpencodeConfig(configPath, config)
}

// unregisterPluginFromConfig removes the cc-deck plugin entry from
// the "plugin" array in opencode.json.
func unregisterPluginFromConfig(pluginPath string) error {
	configPath := opencodeConfigPath()
	config, err := readOpencodeConfig(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	plugins, _ := config["plugin"].([]any)
	if len(plugins) == 0 {
		return nil
	}

	var filtered []any
	for _, p := range plugins {
		s, ok := p.(string)
		if ok && (s == pluginEntry || s == pluginPath) {
			continue
		}
		filtered = append(filtered, p)
	}

	if len(filtered) == len(plugins) {
		return nil
	}

	if filtered == nil {
		filtered = []any{}
	}
	config["plugin"] = filtered
	return writeOpencodeConfig(configPath, config)
}

func readOpencodeConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{
				"$schema": "https://opencode.ai/config.json",
			}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]any{
			"$schema": "https://opencode.ai/config.json",
		}, nil
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing opencode.json: %w", err)
	}
	return config, nil
}

func writeOpencodeConfig(path string, config map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding opencode.json: %w", err)
	}
	data = append(data, '\n')
	return fileutil.AtomicWrite(path, data, 0o644)
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

var opencodeConfigPathFunc = defaultOpencodeConfigPath

func opencodeConfigPath() string {
	return opencodeConfigPathFunc()
}

func defaultOpencodeConfigPath() string {
	return filepath.Join(opencodeConfigDir(), "opencode.json")
}

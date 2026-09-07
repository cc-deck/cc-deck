package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cc-deck/cc-deck/internal/fileutil"
)

// CodexAgent implements the Agent interface for Codex CLI.
type CodexAgent struct{}

func init() {
	Register(&CodexAgent{})
}

func (c *CodexAgent) Name() string        { return "codex" }
func (c *CodexAgent) DisplayName() string { return "Codex CLI" }
func (c *CodexAgent) Indicator() string   { return "◆" }

func (c *CodexAgent) IsInstalled() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}

func (c *CodexAgent) DetectConfig() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".codex")
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	return dir
}

func (c *CodexAgent) InstallHooks() error {
	hooksPath := codexHooksPath()
	hooksFile, err := readCodexHooks(hooksPath)
	if err != nil {
		return err
	}

	hooks, _ := hooksFile["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	for _, event := range codexHookEvents {
		entry := codexHookEntry()
		entryMap := structToMap(entry)

		eventHooks, _ := hooks[event].([]any)
		eventHooks = removeCCDeckHooks(eventHooks)
		eventHooks = append(eventHooks, entryMap)
		hooks[event] = eventHooks
	}

	hooksFile["hooks"] = hooks
	return writeCodexHooks(hooksPath, hooksFile)
}

func (c *CodexAgent) UninstallHooks() error {
	hooksPath := codexHooksPath()
	hooksFile, err := readCodexHooks(hooksPath)
	if err != nil {
		return err
	}

	hooks, _ := hooksFile["hooks"].(map[string]any)
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
		delete(hooksFile, "hooks")
	} else {
		hooksFile["hooks"] = hooks
	}

	return writeCodexHooks(hooksPath, hooksFile)
}

func (c *CodexAgent) HooksInstalled() bool {
	hooksPath := codexHooksPath()
	hooksFile, err := readCodexHooks(hooksPath)
	if err != nil {
		return false
	}
	hooks, _ := hooksFile["hooks"].(map[string]any)
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

type codexHookPayload struct {
	SessionID string `json:"session_id,omitempty"`
	HookEvent string `json:"hook_event_name"`
	ToolName  string `json:"tool_name,omitempty"`
	CWD       string `json:"cwd,omitempty"`
}

func (c *CodexAgent) TranslateEvent(input []byte) (*NormalizedPayload, error) {
	var hook codexHookPayload
	if err := json.Unmarshal(input, &hook); err != nil {
		return nil, fmt.Errorf("parsing Codex hook payload: %w", err)
	}
	if hook.HookEvent == "" {
		return nil, fmt.Errorf("missing hook_event_name in Codex payload")
	}
	return &NormalizedPayload{
		Agent:     c.Name(),
		SessionID: hook.SessionID,
		HookEvent: hook.HookEvent,
		ToolName:  hook.ToolName,
		Cwd:       hook.CWD,
	}, nil
}

func (c *CodexAgent) CredentialSpecs() []CredentialSpec {
	return []CredentialSpec{
		{
			Name:     "openai",
			Priority: 10,
			EnvVars: []EnvVarSpec{
				{Name: "OPENAI_API_KEY", Required: true},
			},
		},
	}
}

func (c *CodexAgent) RequiredDomainGroups() []string {
	return []string{"openai"}
}

// --- Codex hooks.json management ---

// PreToolUse is deliberately absent; see claudeHookEvents.
var codexHookEvents = []string{
	"SessionStart",
	"PostToolUse",
	"PermissionRequest",
	"UserPromptSubmit",
	"Stop",
	"SubagentStart",
	"SubagentStop",
}

func codexHookCommand() string {
	return "cc-deck hook --agent codex --pane-id \"$ZELLIJ_PANE_ID\""
}

func codexHookEntry() hookEntry {
	return hookEntry{
		Hooks: []hookAction{{Type: "command", Command: codexHookCommand()}},
	}
}

var codexHooksPathFunc = defaultCodexHooksPath

func codexHooksPath() string {
	return codexHooksPathFunc()
}

func defaultCodexHooksPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "hooks.json")
}

func readCodexHooks(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, fmt.Errorf("reading hooks.json: %w", err)
	}
	if len(data) == 0 {
		return make(map[string]any), nil
	}
	var hooksFile map[string]any
	if err := json.Unmarshal(data, &hooksFile); err != nil {
		return nil, fmt.Errorf("parsing hooks.json: %w", err)
	}
	if hooksFile == nil {
		hooksFile = make(map[string]any)
	}
	return hooksFile, nil
}

func writeCodexHooks(path string, hooksFile map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(hooksFile, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding hooks.json: %w", err)
	}
	data = append(data, '\n')
	return fileutil.AtomicWrite(path, data, 0o644)
}

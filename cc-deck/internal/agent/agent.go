package agent

import (
	"fmt"
	"sort"
	"sync"
)

// Agent defines the behavioral contract for an AI coding agent integration.
type Agent interface {
	// Name returns the machine-readable identifier (e.g., "claude", "opencode").
	Name() string
	// DisplayName returns the human-readable name (e.g., "Claude Code", "OpenCode").
	DisplayName() string
	// Indicator returns a 1-3 character sidebar indicator (e.g., "CC", "OC").
	// Must be unique across all registered agents.
	Indicator() string

	// IsInstalled returns true if the agent binary is available in PATH.
	IsInstalled() bool
	// DetectConfig returns the filesystem path to the agent's config directory.
	// Returns an empty string if the config directory does not exist.
	DetectConfig() string

	// InstallHooks writes hook artifacts to the agent's config directory.
	// Must be idempotent: calling when hooks exist updates them without duplication.
	InstallHooks() error
	// UninstallHooks removes cc-deck hook artifacts from the agent's config.
	// Returns nil if no hooks are installed.
	UninstallHooks() error
	// HooksInstalled returns true if cc-deck hooks are currently active.
	HooksInstalled() bool

	// TranslateEvent parses agent-specific JSON input and produces a NormalizedPayload.
	// Sets the agent field to Name(). Returns an error for malformed input.
	TranslateEvent(input []byte) (*NormalizedPayload, error)
}

// NormalizedPayload is the common format sent to the Zellij plugin via pipe message.
// The PaneID and Badges fields are populated by the hook command after TranslateEvent returns.
type NormalizedPayload struct {
	Agent     string   `json:"agent"`
	SessionID string   `json:"session_id,omitempty"`
	PaneID    uint32   `json:"pane_id"`
	HookEvent string   `json:"hook_event_name"`
	ToolName  string   `json:"tool_name,omitempty"`
	Cwd       string   `json:"cwd,omitempty"`
	AgentID   string   `json:"agent_id,omitempty"`
	Badges    []string `json:"badges,omitempty"`
}

var (
	mu         sync.Mutex
	agents     = map[string]Agent{}
	indicators = map[string]string{} // indicator -> agent name
)

// Register adds an agent to the global registry.
// Panics on duplicate name or duplicate indicator.
func Register(a Agent) {
	mu.Lock()
	defer mu.Unlock()

	name := a.Name()
	if _, exists := agents[name]; exists {
		panic(fmt.Sprintf("agent: duplicate name %q", name))
	}

	ind := a.Indicator()
	if existing, exists := indicators[ind]; exists {
		panic(fmt.Sprintf("agent: indicator %q already registered by %q (conflict with %q)", ind, existing, name))
	}

	agents[name] = a
	indicators[ind] = name
}

// Get returns the agent with the given name, or nil if not found.
func Get(name string) Agent {
	mu.Lock()
	defer mu.Unlock()
	return agents[name]
}

// All returns all registered agents in stable alphabetical order by name.
func All() []Agent {
	mu.Lock()
	defer mu.Unlock()

	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]Agent, len(names))
	for i, name := range names {
		result[i] = agents[name]
	}
	return result
}

// Reset clears the registry. Only for use in tests.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	agents = map[string]Agent{}
	indicators = map[string]string{}
}

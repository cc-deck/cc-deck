package setup

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Manifest represents the cc-deck-setup.yaml file.
type Manifest struct {
	Version     int              `yaml:"version"`
	Tools       []string         `yaml:"tools,omitempty"`
	Sources     []SourceEntry    `yaml:"sources,omitempty"`
	Plugins     []PluginEntry    `yaml:"plugins,omitempty"`
	MCP         []MCPEntry       `yaml:"mcp,omitempty"`
	GithubTools []GithubTool     `yaml:"github_tools,omitempty"`
	Settings    *SettingsConfig  `yaml:"settings,omitempty"`
	Network     *NetworkConfig   `yaml:"network,omitempty"`
	Targets     *TargetsConfig   `yaml:"targets,omitempty"`
}

// TargetsConfig holds per-target configuration.
type TargetsConfig struct {
	Container *ContainerTarget `yaml:"container,omitempty"`
	SSH       *SSHTarget       `yaml:"ssh,omitempty"`
}

// ContainerTarget describes the container image to build.
type ContainerTarget struct {
	Name     string `yaml:"name"`
	Tag      string `yaml:"tag,omitempty"`
	Base     string `yaml:"base,omitempty"`
	Registry string `yaml:"registry,omitempty"`
}

// SSHTarget describes the SSH provisioning target.
type SSHTarget struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port,omitempty"`
	IdentityFile string `yaml:"identity_file,omitempty"`
	CreateUser   bool   `yaml:"create_user,omitempty"`
	User         string `yaml:"user,omitempty"`
	Workspace    string `yaml:"workspace,omitempty"`
}

// NetworkConfig describes network filtering for containerized sessions.
type NetworkConfig struct {
	AllowedDomains []string `yaml:"allowed_domains,omitempty"`
}

// SourceEntry tracks a repository analyzed for tool discovery.
type SourceEntry struct {
	URL           string   `yaml:"url"`
	Ref           string   `yaml:"ref,omitempty"`
	Path          string   `yaml:"path,omitempty"`
	DetectedTools []string `yaml:"detected_tools,omitempty"`
	DetectedFrom  []string `yaml:"detected_from,omitempty"`
}

// PluginEntry describes a Claude Code plugin to install.
type PluginEntry struct {
	Name   string `yaml:"name"`
	Source string `yaml:"source"`
}

// MCPEntry describes an MCP server sidecar container.
type MCPEntry struct {
	Name        string  `yaml:"name"`
	Image       string  `yaml:"image"`
	Transport   string  `yaml:"transport,omitempty"`
	Port        int     `yaml:"port,omitempty"`
	Auth        MCPAuth `yaml:"auth,omitempty"`
	Description string  `yaml:"description,omitempty"`
}

// MCPAuth describes authentication requirements for an MCP server.
type MCPAuth struct {
	Type    string   `yaml:"type,omitempty"`
	EnvVars []string `yaml:"env_vars,omitempty"`
}

// GithubTool describes a tool to download from GitHub releases.
type GithubTool struct {
	Repo   string `yaml:"repo"`
	Binary string `yaml:"binary"`
}

// SettingsConfig describes user configuration to apply to the target.
type SettingsConfig struct {
	Shell          string `yaml:"shell,omitempty"`
	ShellRC        string `yaml:"shell_rc,omitempty"`
	ZellijConfig   string `yaml:"zellij_config,omitempty"`
	ClaudeMD       string `yaml:"claude_md,omitempty"`
	ClaudeSettings string `yaml:"claude_settings,omitempty"`
	Hooks          string `yaml:"hooks,omitempty"`
	MCPSettings    string `yaml:"mcp_settings,omitempty"`
	CCSetupMCP     string `yaml:"cc_setup_mcp,omitempty"`
}

// LoadManifest reads and parses a cc-deck-setup.yaml file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	return &m, nil
}

// Validate checks the manifest for required fields.
func (m *Manifest) Validate() error {
	if m.Version < 1 {
		return fmt.Errorf("manifest version must be >= 1, got %d", m.Version)
	}
	if m.Targets != nil {
		if ct := m.Targets.Container; ct != nil {
			if ct.Name == "" {
				return fmt.Errorf("targets.container.name is required")
			}
		}
		if st := m.Targets.SSH; st != nil {
			if st.Host == "" {
				return fmt.Errorf("targets.ssh.host is required")
			}
			if st.CreateUser && st.User == "" {
				return fmt.Errorf("targets.ssh.user is required when create_user is true")
			}
		}
	}
	return nil
}

// ImageRef returns the full container image reference (name:tag).
// Returns empty string if no container target is configured.
func (m *Manifest) ImageRef() string {
	if m.Targets == nil || m.Targets.Container == nil {
		return ""
	}
	ct := m.Targets.Container
	tag := ct.Tag
	if tag == "" {
		tag = "latest"
	}
	return ct.Name + ":" + tag
}

// DefaultBaseImage is the fallback base image reference.
// The registry prefix is set at build time via ldflags.
var DefaultBaseImage = "quay.io/cc-deck/cc-deck-base:latest"

// BaseImage returns the base image reference, with default.
func (m *Manifest) BaseImage() string {
	if m.Targets != nil && m.Targets.Container != nil && m.Targets.Container.Base != "" {
		return m.Targets.Container.Base
	}
	return DefaultBaseImage
}

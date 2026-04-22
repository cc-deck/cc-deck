package build

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest represents the build.yaml file.
type Manifest struct {
	Version     int              `yaml:"version"`
	Tools       []ToolEntry      `yaml:"tools,omitempty"`
	Sources     []SourceEntry    `yaml:"sources,omitempty"`
	Plugins     []PluginEntry    `yaml:"plugins,omitempty"`
	MCP         []MCPEntry       `yaml:"mcp,omitempty"`
	Settings    *SettingsConfig  `yaml:"settings,omitempty"`
	Network     *NetworkConfig   `yaml:"network,omitempty"`
	Targets     *TargetsConfig   `yaml:"targets,omitempty"`
}

// ToolEntry describes a tool to install, either via package manager or GitHub release.
type ToolEntry struct {
	Name         string `yaml:"name"`
	Install      string `yaml:"install,omitempty"`
	Repo         string `yaml:"repo,omitempty"`
	AssetPattern string `yaml:"asset_pattern,omitempty"`
	InstallPath  string `yaml:"install_path,omitempty"`
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
	Clone         bool     `yaml:"clone,omitempty"`
	Path          string   `yaml:"path,omitempty"`
	DetectedTools []string `yaml:"detected_tools,omitempty"`
	DetectedFrom  []string `yaml:"detected_from,omitempty"`
}

// PluginEntry describes a Claude Code plugin to install.
type PluginEntry struct {
	Name        string `yaml:"name"`
	Source      string `yaml:"source"`
	Marketplace string `yaml:"marketplace,omitempty"`
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

// PackageTools returns tools that should be installed via package manager.
func (m *Manifest) PackageTools() []ToolEntry {
	var result []ToolEntry
	for _, t := range m.Tools {
		if t.Install == "" || t.Install == "package" {
			result = append(result, t)
		}
	}
	return result
}

// GithubReleaseTools returns tools that should be installed from GitHub releases.
func (m *Manifest) GithubReleaseTools() []ToolEntry {
	var result []ToolEntry
	for _, t := range m.Tools {
		if t.Install == "github-release" {
			result = append(result, t)
		}
	}
	return result
}

// SettingsConfig describes user configuration to apply to the target.
type SettingsConfig struct {
	Shell          string            `yaml:"shell,omitempty"`
	ShellRC        string            `yaml:"shell_rc,omitempty"`
	ZellijConfig   string            `yaml:"zellij_config,omitempty"`
	ClaudeMD       string            `yaml:"claude_md,omitempty"`
	ClaudeSettings string            `yaml:"claude_settings,omitempty"`
	Hooks          string            `yaml:"hooks,omitempty"`
	MCPSettings    string            `yaml:"mcp_settings,omitempty"`
	CCSetupMCP     string            `yaml:"cc_setup_mcp,omitempty"`
	RemoteBG       string            `yaml:"remote_bg,omitempty"`
	GitConfig      map[string]string `yaml:"git_config,omitempty"`
	ToolConfigs    []ToolConfig      `yaml:"tool_configs,omitempty"`
}

// ToolConfig describes a tool's configuration file to deploy to the target.
type ToolConfig struct {
	Tool   string `yaml:"tool"`
	Source string `yaml:"source"`
	Target string `yaml:"target,omitempty"`
}

// LoadManifest reads and parses a build.yaml file.
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
	for i, p := range m.Plugins {
		if p.Name == "" {
			return fmt.Errorf("plugins[%d].name is required", i)
		}
		if p.Source == "" {
			return fmt.Errorf("plugins[%d].source is required", i)
		}
	}
	for i, mcp := range m.MCP {
		if mcp.Name == "" {
			return fmt.Errorf("mcp[%d].name is required", i)
		}
	}
	for i, t := range m.Tools {
		if t.Name == "" {
			return fmt.Errorf("tools[%d].name is required", i)
		}
		if t.Install == "github-release" {
			if t.Repo == "" || !strings.Contains(t.Repo, "/") {
				return fmt.Errorf("tools[%d].repo must be in owner/repo format", i)
			}
		}
	}
	if m.Settings != nil {
		for i, tc := range m.Settings.ToolConfigs {
			if tc.Tool == "" {
				return fmt.Errorf("settings.tool_configs[%d].tool is required", i)
			}
			if tc.Source == "" {
				return fmt.Errorf("settings.tool_configs[%d].source is required", i)
			}
		}
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

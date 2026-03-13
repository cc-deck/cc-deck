package build

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Manifest represents the cc-deck-build.yaml file.
type Manifest struct {
	Version     int              `yaml:"version"`
	Image       ImageConfig      `yaml:"image"`
	Tools       []string         `yaml:"tools,omitempty"`
	Sources     []SourceEntry    `yaml:"sources,omitempty"`
	Plugins     []PluginEntry    `yaml:"plugins,omitempty"`
	MCP         []MCPEntry       `yaml:"mcp,omitempty"`
	GithubTools []GithubTool     `yaml:"github_tools,omitempty"`
	Settings    *SettingsConfig  `yaml:"settings,omitempty"`
}

// ImageConfig describes the container image to build.
type ImageConfig struct {
	Name string `yaml:"name"`
	Tag  string `yaml:"tag,omitempty"`
	Base string `yaml:"base,omitempty"`
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
	Name        string   `yaml:"name"`
	Image       string   `yaml:"image"`
	Transport   string   `yaml:"transport,omitempty"`
	Port        int      `yaml:"port,omitempty"`
	Auth        MCPAuth  `yaml:"auth,omitempty"`
	Description string   `yaml:"description,omitempty"`
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

// SettingsConfig describes user configuration to bake into the image.
type SettingsConfig struct {
	ClaudeMD       string `yaml:"claude_md,omitempty"`
	ClaudeSettings string `yaml:"claude_settings,omitempty"`
	Hooks          string `yaml:"hooks,omitempty"`
	ZellijConfig   string `yaml:"zellij_config,omitempty"`
}

// LoadManifest reads and parses a cc-deck-build.yaml file.
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
	if m.Image.Name == "" {
		return fmt.Errorf("image.name is required")
	}
	return nil
}

// ImageRef returns the full image reference (name:tag).
func (m *Manifest) ImageRef() string {
	tag := m.Image.Tag
	if tag == "" {
		tag = "latest"
	}
	return m.Image.Name + ":" + tag
}

// DefaultBaseImage is the fallback base image reference.
// The registry prefix is set at build time via ldflags.
var DefaultBaseImage = "quay.io/rhuss/cc-deck-base:latest"

// BaseImage returns the base image reference, with default.
func (m *Manifest) BaseImage() string {
	if m.Image.Base != "" {
		return m.Image.Base
	}
	return DefaultBaseImage
}

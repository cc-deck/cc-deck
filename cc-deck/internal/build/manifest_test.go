package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManifest_WithContainerTarget(t *testing.T) {
	content := `
version: 2
targets:
  container:
    name: my-image
    tag: latest
    base: quay.io/cc-deck/cc-deck-base:latest
    registry: quay.io/cc-deck
tools:
  - "Go 1.25"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "cc-deck-build.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	m, err := LoadManifest(path)
	require.NoError(t, err)

	require.NotNil(t, m.Targets)
	require.NotNil(t, m.Targets.Container)
	assert.Equal(t, "my-image", m.Targets.Container.Name)
	assert.Equal(t, "latest", m.Targets.Container.Tag)
	assert.Equal(t, "quay.io/cc-deck/cc-deck-base:latest", m.Targets.Container.Base)
	assert.Equal(t, "quay.io/cc-deck", m.Targets.Container.Registry)
	assert.Nil(t, m.Targets.SSH)
	assert.Equal(t, []string{"Go 1.25"}, m.Tools)
}

func TestLoadManifest_WithSSHTarget(t *testing.T) {
	content := `
version: 2
targets:
  ssh:
    host: dev@marovo
    port: 22
    identity_file: ~/.ssh/id_ed25519
    create_user: true
    user: dev
    workspace: ~/workspace
`
	dir := t.TempDir()
	path := filepath.Join(dir, "cc-deck-build.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	m, err := LoadManifest(path)
	require.NoError(t, err)

	require.NotNil(t, m.Targets)
	require.NotNil(t, m.Targets.SSH)
	assert.Equal(t, "dev@marovo", m.Targets.SSH.Host)
	assert.Equal(t, 22, m.Targets.SSH.Port)
	assert.Equal(t, "~/.ssh/id_ed25519", m.Targets.SSH.IdentityFile)
	assert.True(t, m.Targets.SSH.CreateUser)
	assert.Equal(t, "dev", m.Targets.SSH.User)
	assert.Equal(t, "~/workspace", m.Targets.SSH.Workspace)
	assert.Nil(t, m.Targets.Container)
}

func TestLoadManifest_WithBothTargets(t *testing.T) {
	content := `
version: 2
targets:
  container:
    name: my-image
  ssh:
    host: dev@marovo
`
	dir := t.TempDir()
	path := filepath.Join(dir, "cc-deck-build.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	m, err := LoadManifest(path)
	require.NoError(t, err)

	require.NotNil(t, m.Targets)
	require.NotNil(t, m.Targets.Container)
	require.NotNil(t, m.Targets.SSH)
	assert.Equal(t, "my-image", m.Targets.Container.Name)
	assert.Equal(t, "dev@marovo", m.Targets.SSH.Host)
}

func TestLoadManifest_WithoutTargets(t *testing.T) {
	content := `
version: 2
tools:
  - ripgrep
`
	dir := t.TempDir()
	path := filepath.Join(dir, "cc-deck-build.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	m, err := LoadManifest(path)
	require.NoError(t, err)
	assert.Nil(t, m.Targets)
}

func TestLoadManifest_WithNetworkSection(t *testing.T) {
	content := `
version: 2
targets:
  container:
    name: my-image
network:
  allowed_domains:
    - python
    - golang
    - custom.example.com
`
	dir := t.TempDir()
	path := filepath.Join(dir, "cc-deck-build.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	m, err := LoadManifest(path)
	require.NoError(t, err)

	require.NotNil(t, m.Network)
	assert.Equal(t, []string{"python", "golang", "custom.example.com"}, m.Network.AllowedDomains)
}

func TestManifest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		m       *Manifest
		wantErr string
	}{
		{
			name: "valid with no targets",
			m:    &Manifest{Version: 2},
		},
		{
			name: "valid with container target",
			m: &Manifest{
				Version: 2,
				Targets: &TargetsConfig{
					Container: &ContainerTarget{Name: "test"},
				},
			},
		},
		{
			name: "valid with SSH target",
			m: &Manifest{
				Version: 2,
				Targets: &TargetsConfig{
					SSH: &SSHTarget{Host: "dev@server"},
				},
			},
		},
		{
			name:    "invalid version",
			m:       &Manifest{Version: 0},
			wantErr: "manifest version must be >= 1",
		},
		{
			name: "container target missing name",
			m: &Manifest{
				Version: 2,
				Targets: &TargetsConfig{
					Container: &ContainerTarget{},
				},
			},
			wantErr: "targets.container.name is required",
		},
		{
			name: "SSH target missing host",
			m: &Manifest{
				Version: 2,
				Targets: &TargetsConfig{
					SSH: &SSHTarget{},
				},
			},
			wantErr: "targets.ssh.host is required",
		},
		{
			name: "SSH target create_user without user",
			m: &Manifest{
				Version: 2,
				Targets: &TargetsConfig{
					SSH: &SSHTarget{Host: "dev@server", CreateUser: true},
				},
			},
			wantErr: "targets.ssh.user is required when create_user is true",
		},
		{
			name: "SSH target create_user with user",
			m: &Manifest{
				Version: 2,
				Targets: &TargetsConfig{
					SSH: &SSHTarget{Host: "dev@server", CreateUser: true, User: "dev"},
				},
			},
		},
		{
			name: "plugin missing name",
			m: &Manifest{
				Version: 2,
				Plugins: []PluginEntry{{Source: "marketplace"}},
			},
			wantErr: "plugins[0].name is required",
		},
		{
			name: "plugin missing source",
			m: &Manifest{
				Version: 2,
				Plugins: []PluginEntry{{Name: "test"}},
			},
			wantErr: "plugins[0].source is required",
		},
		{
			name: "mcp missing name",
			m: &Manifest{
				Version: 2,
				MCP:     []MCPEntry{{Image: "test:latest"}},
			},
			wantErr: "mcp[0].name is required",
		},
		{
			name: "github_tools invalid repo format",
			m: &Manifest{
				Version: 2,
				GithubTools: []GithubTool{{Repo: "invalid", Binary: "test"}},
			},
			wantErr: "github_tools[0].repo must be in owner/repo format",
		},
		{
			name: "valid with plugins and github_tools",
			m: &Manifest{
				Version:     2,
				Plugins:     []PluginEntry{{Name: "test", Source: "marketplace"}},
				GithubTools: []GithubTool{{Repo: "org/tool", Binary: "tool"}},
				MCP:         []MCPEntry{{Name: "mcp-test", Image: "test:latest"}},
			},
		},
		{
			name: "valid tool_configs",
			m: &Manifest{
				Version: 2,
				Settings: &SettingsConfig{
					ToolConfigs: []ToolConfig{
						{Tool: "starship", Source: "./starship.toml", Target: "starship.toml"},
					},
				},
			},
		},
		{
			name: "tool_configs missing tool",
			m: &Manifest{
				Version: 2,
				Settings: &SettingsConfig{
					ToolConfigs: []ToolConfig{{Source: "./config.toml"}},
				},
			},
			wantErr: "settings.tool_configs[0].tool is required",
		},
		{
			name: "tool_configs missing source",
			m: &Manifest{
				Version: 2,
				Settings: &SettingsConfig{
					ToolConfigs: []ToolConfig{{Tool: "helix"}},
				},
			},
			wantErr: "settings.tool_configs[0].source is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManifest_ImageRef(t *testing.T) {
	tests := []struct {
		name string
		m    *Manifest
		want string
	}{
		{
			name: "no targets",
			m:    &Manifest{},
			want: "",
		},
		{
			name: "container with tag",
			m: &Manifest{Targets: &TargetsConfig{
				Container: &ContainerTarget{Name: "my-image", Tag: "v1"},
			}},
			want: "my-image:v1",
		},
		{
			name: "container default tag",
			m: &Manifest{Targets: &TargetsConfig{
				Container: &ContainerTarget{Name: "my-image"},
			}},
			want: "my-image:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.m.ImageRef())
		})
	}
}

func TestManifest_BaseImage(t *testing.T) {
	tests := []struct {
		name string
		m    *Manifest
		want string
	}{
		{
			name: "no targets uses default",
			m:    &Manifest{},
			want: DefaultBaseImage,
		},
		{
			name: "container with custom base",
			m: &Manifest{Targets: &TargetsConfig{
				Container: &ContainerTarget{Name: "test", Base: "fedora:41"},
			}},
			want: "fedora:41",
		},
		{
			name: "container without base uses default",
			m: &Manifest{Targets: &TargetsConfig{
				Container: &ContainerTarget{Name: "test"},
			}},
			want: DefaultBaseImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.m.BaseImage())
		})
	}
}

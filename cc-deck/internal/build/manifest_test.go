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
  - name: "Go 1.25"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "build.yaml")
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
	require.Len(t, m.Tools, 1)
	assert.Equal(t, "Go 1.25", m.Tools[0].Name)
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
	path := filepath.Join(dir, "build.yaml")
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
	path := filepath.Join(dir, "build.yaml")
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
  - name: ripgrep
`
	dir := t.TempDir()
	path := filepath.Join(dir, "build.yaml")
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
	path := filepath.Join(dir, "build.yaml")
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
			name: "tool missing name",
			m: &Manifest{
				Version: 2,
				Tools:   []ToolEntry{{Install: "package"}},
			},
			wantErr: "tools[0].name is required",
		},
		{
			name: "github-release tool invalid repo format",
			m: &Manifest{
				Version: 2,
				Tools:   []ToolEntry{{Name: "test", Install: "github-release", Repo: "invalid"}},
			},
			wantErr: "tools[0].repo must be in owner/repo format",
		},
		{
			name: "valid with plugins and github-release tools",
			m: &Manifest{
				Version: 2,
				Tools:   []ToolEntry{{Name: "tool", Install: "github-release", Repo: "org/tool"}},
				Plugins: []PluginEntry{{Name: "test", Source: "marketplace"}},
				MCP:     []MCPEntry{{Name: "mcp-test", Image: "test:latest"}},
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

func TestLoadManifest_WithGitConfig(t *testing.T) {
	content := `
version: 3
settings:
  git_config:
    user.name: "Roland Huß"
    user.email: "roland@example.com"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "build.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	m, err := LoadManifest(path)
	require.NoError(t, err)

	require.NotNil(t, m.Settings)
	require.NotNil(t, m.Settings.GitConfig)
	assert.Equal(t, "Roland Huß", m.Settings.GitConfig["user.name"])
	assert.Equal(t, "roland@example.com", m.Settings.GitConfig["user.email"])
}

func TestLoadManifest_WithOpenShellTarget(t *testing.T) {
	content := `
version: 3
targets:
  openshell:
    name: my-sandbox
    tag: v2
    base: ghcr.io/custom/base:latest
    registry: ghcr.io/myorg
network:
  allowed_domains:
    - api.anthropic.com
    - github.com
`
	dir := t.TempDir()
	path := filepath.Join(dir, "build.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	m, err := LoadManifest(path)
	require.NoError(t, err)

	require.NotNil(t, m.Targets)
	require.NotNil(t, m.Targets.OpenShell)
	assert.Equal(t, "my-sandbox", m.Targets.OpenShell.Name)
	assert.Equal(t, "v2", m.Targets.OpenShell.Tag)
	assert.Equal(t, "ghcr.io/custom/base:latest", m.Targets.OpenShell.Base)
	assert.Equal(t, "ghcr.io/myorg", m.Targets.OpenShell.Registry)
	assert.Nil(t, m.Targets.Container)
	assert.Nil(t, m.Targets.SSH)
}

func TestLoadManifest_WithOpenShellPolicy(t *testing.T) {
	content := `
version: 3
targets:
  openshell:
    name: my-sandbox
    policy:
      filesystem_policy:
        include_workdir: true
        read_only:
          - /usr
        read_write:
          - /sandbox
      landlock:
        compatibility: best_effort
      process:
        run_as_user: sandbox
        run_as_group: sandbox
      network_policies:
        git_hosting:
          name: git-hosting
          endpoints:
            - host: github.com
              port: 443
          binaries:
            - path: /usr/bin/git
`
	dir := t.TempDir()
	path := filepath.Join(dir, "build.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	m, err := LoadManifest(path)
	require.NoError(t, err)

	require.NotNil(t, m.Targets.OpenShell.Policy)
	p := m.Targets.OpenShell.Policy

	require.NotNil(t, p.FilesystemPolicy)
	assert.True(t, p.FilesystemPolicy.IncludeWorkdir)
	assert.Equal(t, []string{"/usr"}, p.FilesystemPolicy.ReadOnly)

	require.NotNil(t, p.Landlock)
	assert.Equal(t, "best_effort", p.Landlock.Compatibility)

	require.NotNil(t, p.Process)
	assert.Equal(t, "sandbox", p.Process.RunAsUser)

	require.Len(t, p.NetworkPolicies, 1)
	np := p.NetworkPolicies["git_hosting"]
	assert.Equal(t, "git-hosting", np.Name)
	require.Len(t, np.Endpoints, 1)
	assert.Equal(t, "github.com", np.Endpoints[0].Host)
	assert.Equal(t, 443, np.Endpoints[0].Port)
	require.Len(t, np.Binaries, 1)
	assert.Equal(t, "/usr/bin/git", np.Binaries[0].Path)
}

func TestManifest_Validate_OpenShellMissingName(t *testing.T) {
	m := &Manifest{
		Version: 3,
		Targets: &TargetsConfig{
			OpenShell: &OpenShellTarget{},
		},
	}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "targets.openshell.name is required")
}

func TestManifest_Validate_OpenShellValid(t *testing.T) {
	m := &Manifest{
		Version: 3,
		Targets: &TargetsConfig{
			OpenShell: &OpenShellTarget{Name: "test-sandbox"},
		},
	}
	assert.NoError(t, m.Validate())
}

func TestManifest_OpenShellImageRef(t *testing.T) {
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
			name: "openshell with tag",
			m: &Manifest{Targets: &TargetsConfig{
				OpenShell: &OpenShellTarget{Name: "my-sandbox", Tag: "v1"},
			}},
			want: "my-sandbox:v1",
		},
		{
			name: "openshell default tag",
			m: &Manifest{Targets: &TargetsConfig{
				OpenShell: &OpenShellTarget{Name: "my-sandbox"},
			}},
			want: "my-sandbox:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.m.OpenShellImageRef())
		})
	}
}

func TestManifest_OpenShellBaseImage(t *testing.T) {
	tests := []struct {
		name string
		m    *Manifest
		want string
	}{
		{
			name: "no targets uses default",
			m:    &Manifest{},
			want: DefaultOpenShellBaseImage,
		},
		{
			name: "openshell with custom base",
			m: &Manifest{Targets: &TargetsConfig{
				OpenShell: &OpenShellTarget{Name: "test", Base: "custom:latest"},
			}},
			want: "custom:latest",
		},
		{
			name: "openshell without base uses default",
			m: &Manifest{Targets: &TargetsConfig{
				OpenShell: &OpenShellTarget{Name: "test"},
			}},
			want: DefaultOpenShellBaseImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.m.OpenShellBaseImage())
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

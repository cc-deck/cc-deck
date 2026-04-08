# Data Model: Unified Setup Command

**Feature**: 034-unified-setup-command
**Date**: 2026-04-08

## Entity: Setup Manifest (`cc-deck-setup.yaml`)

The manifest is the source of truth for what to install on a target machine. It contains shared sections (target-agnostic) and per-target sections.

### Schema (v2)

```yaml
version: 2

# ---- Shared sections (populated by /cc-deck.capture) ----

tools:                        # Free-form tool descriptions
  - "go 1.25"
  - "ripgrep"
  - "jq"

sources:                      # Repository provenance
  - url: "https://github.com/user/project"
    ref: "main"               # optional
    path: "/local/path"       # optional
    detected_tools:           # optional
      - "go 1.25"
    detected_from:            # optional
      - "go.mod"

plugins:                      # Claude Code plugins
  - name: "superpowers"
    source: "marketplace"
  - name: "copyedit"
    source: "git:https://github.com/org/copyedit.git"

mcp:                          # MCP server configurations
  - name: "github-mcp"
    image: "ghcr.io/modelcontextprotocol/github-mcp:latest"
    transport: "sse"          # optional
    port: 8000                # optional
    auth:                     # optional
      type: "token"
      env_vars: ["GITHUB_TOKEN"]
    description: ""           # optional

github_tools:                 # GitHub release downloads
  - repo: "starship/starship"
    binary: "starship"
    asset_pattern: "starship-{arch}-unknown-linux-gnu.tar.gz"  # optional

settings:
  shell: "zsh"                # "zsh" or "bash"
  shell_rc: "./build-context/zshrc"  # path to curated shell config
  zellij_config: "current"    # "current", "vanilla", or path
  claude_md: "~/.claude/CLAUDE.md"
  claude_settings: "~/.claude/settings.json"
  hooks: "~/.claude/hooks.json"
  mcp_settings: "~/.config/cc-setup/mcp.json"
  cc_setup_mcp: "~/.config/cc-setup/mcp.json"

network:                      # optional, container-only
  allowed_domains:
    - "golang"
    - "github"

# ---- Target sections ----

targets:
  container:                  # omitted if not needed
    name: "my-dev"
    tag: "latest"
    base: "quay.io/cc-deck/cc-deck-base:latest"
    registry: "quay.io/cc-deck"  # optional, for --push

  ssh:                        # omitted if not needed
    host: "dev@marovo"
    port: 22                  # optional, default 22
    identity_file: "~/.ssh/id_ed25519"  # optional
    create_user: true         # optional, default false
    user: "dev"               # required if create_user is true
    workspace: "~/workspace"  # optional, default ~/workspace
```

### Go Struct (replaces current `Manifest`)

```go
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

// ContainerTarget replaces the old ImageConfig.
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
```

### Validation Rules

| Field | Rule |
|-------|------|
| `version` | Must be >= 1 |
| `targets.container.name` | Required if container target present |
| `targets.ssh.host` | Required if SSH target present |
| `targets.ssh.user` | Required if `create_user` is true |
| `tools` | Free-form strings, no format enforcement |
| `plugins[].name` | Required, non-empty |
| `plugins[].source` | Required, non-empty |
| `mcp[].name` | Required, non-empty |
| `github_tools[].repo` | Required, format: `owner/repo` |

### State Transitions

The manifest itself is stateless (it is a declarative profile). State transitions occur in the generated artifacts:

```
[No setup dir] --init--> [Manifest template + Claude commands]
[Manifest template] --capture--> [Populated manifest]
[Populated manifest] --build container--> [Containerfile + image]
[Populated manifest] --build ssh--> [Ansible playbooks + provisioned host]
```

---

## Entity: Container Target Artifacts

Generated by `/cc-deck.build --target container`. Same as current `cc-deck image` behavior.

```
.cc-deck/setup/
├── cc-deck-setup.yaml          # manifest
├── Containerfile               # generated
├── build-context/              # binaries, config files
│   ├── cc-deck-linux-amd64
│   ├── cc-deck-linux-arm64
│   └── zshrc
├── build.sh                    # standalone rebuild script
└── .gitignore
```

---

## Entity: SSH Target Artifacts

Generated by `/cc-deck.build --target ssh`.

```
.cc-deck/setup/
├── cc-deck-setup.yaml          # manifest
├── inventory.ini               # generated from targets.ssh
├── site.yml                    # main playbook entry point
├── group_vars/
│   └── all.yml                 # variables from manifest
├── roles/
│   ├── base/
│   │   ├── tasks/main.yml      # user creation, SSH keys, core packages
│   │   └── defaults/main.yml
│   ├── tools/
│   │   ├── tasks/main.yml      # system packages + github releases
│   │   └── defaults/main.yml
│   ├── zellij/
│   │   ├── tasks/main.yml      # Zellij binary download
│   │   └── defaults/main.yml
│   ├── claude/
│   │   ├── tasks/main.yml      # Claude Code via official installer
│   │   └── defaults/main.yml
│   ├── cc_deck/
│   │   ├── tasks/main.yml      # cc-deck CLI + plugin install
│   │   └── defaults/main.yml
│   ├── shell_config/
│   │   ├── tasks/main.yml      # zshrc, starship, credential sourcing
│   │   ├── templates/
│   │   │   └── zshrc.j2
│   │   └── defaults/main.yml
│   └── mcp/
│       ├── tasks/main.yml      # MCP server setup
│       └── defaults/main.yml
├── README.md                   # standalone usage instructions
└── .gitignore
```

### Inventory Format

Generated `inventory.ini`:
```ini
[setup_targets]
target ansible_host=marovo ansible_user=dev ansible_port=22 ansible_ssh_private_key_file=~/.ssh/id_ed25519
```

### Group Variables

Generated `group_vars/all.yml` (extracted from manifest):
```yaml
# Generated by cc-deck setup - do not edit manually
cc_deck_version: "0.12.0"
zellij_version: "0.43.1"
create_user: true
target_user: dev
workspace: ~/workspace
shell: zsh
tools:
  - go 1.25
  - ripgrep
  - jq
plugins:
  - name: superpowers
    source: marketplace
github_tools:
  - repo: starship/starship
    binary: starship
```

---

## Entity: Ansible Role Details

### base role
- **Purpose**: System preparation and user creation
- **Inputs**: `create_user`, `target_user`, `shell`
- **Tasks**: Detect OS family (set_fact), install core packages (git, curl, tar, unzip, zsh), create user with sudo if `create_user` is true, set default shell, create workspace directory

### tools role
- **Purpose**: Install development tools from manifest
- **Inputs**: `tools`, `github_tools`
- **Tasks**: Map tool descriptions to package names, install via dnf, download GitHub release binaries with architecture mapping (aarch64/x86_64)

### zellij role
- **Purpose**: Install Zellij terminal multiplexer
- **Inputs**: `zellij_version`
- **Tasks**: Download Zellij release binary for target architecture, install to `/usr/local/bin/zellij`, verify installation

### claude role
- **Purpose**: Install Claude Code
- **Inputs**: none (uses official installer)
- **Tasks**: Run `curl -fsSL https://claude.ai/install.sh | bash` as the target user

### cc_deck role
- **Purpose**: Install cc-deck CLI and plugin
- **Inputs**: `cc_deck_version`
- **Tasks**: Download cc-deck release binary from GitHub Releases, install to `/usr/local/bin/cc-deck`, run `cc-deck plugin install` as target user (installs WASM plugin, layout files, controller config, Claude Code hooks)

### shell_config role
- **Purpose**: Configure shell environment
- **Inputs**: `shell`, curated shell config
- **Tasks**: Install curated shell RC from template, add credential sourcing snippet (`[ -f ~/.config/cc-deck/credentials.env ] && source ...`), install starship config if present

### mcp role
- **Purpose**: Configure MCP servers
- **Inputs**: `plugins` (MCP entries from manifest)
- **Tasks**: Set up MCP config files in appropriate locations

---

## Entity: Lightweight Probe (replaces pre-flight bootstrap)

A single SSH command that replaces the interactive bootstrap in `internal/ssh/bootstrap.go`.

```go
// Probe checks whether an SSH host has been provisioned by cc-deck setup.
func Probe(client *ssh.Client) error {
    output, err := client.Run("which zellij && which cc-deck && which claude")
    if err != nil {
        return fmt.Errorf("host appears unprovisioned (missing tools): %s\n"+
            "Run 'cc-deck setup' to provision the host first", output)
    }
    return nil
}
```

The probe replaces the full bootstrap check sequence. It runs during `cc-deck env create --type ssh` after SSH connectivity is confirmed.

---

## Changes to Existing Entities

### EnvironmentDefinition (no changes)
The SSH fields in `EnvironmentDefinition` remain unchanged. The setup command is independent of environment definitions.

### EnvironmentInstance (no changes)
SSH environment instances are unaffected. The setup command provisions the host; environment creation registers a session.

### FileStateStore (no changes)
State management is unaffected by the setup command.

### Build Package Rename
- `internal/build` -> `internal/setup`
- `Manifest.Image` -> `Manifest.Targets`
- `cc-deck-image.yaml` -> `cc-deck-setup.yaml`
- `ImageConfig` -> `ContainerTarget`
- All imports and references updated accordingly

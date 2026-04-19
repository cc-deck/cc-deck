---
description: Discover tools and settings from your local environment for the setup manifest
---

## User Input

$ARGUMENTS

## Setup directory

All setup artifacts live in `.cc-deck/setup/` relative to the project root (the git root or the directory containing `.cc-deck/`). Resolve the setup directory first:

1. Find the project root (look for `.cc-deck/` directory or git root, walking up from the current working directory)
2. The setup directory is `<project-root>/.cc-deck/setup/`
3. The manifest is at `<setup-dir>/build.yaml`

All file references in this command (manifest, config files) are relative to the setup directory unless stated otherwise.

## Interaction Model

This command runs as a **step-by-step wizard**. There are 9 steps total.

**Rules:**
- Present ONE step at a time
- Show a progress header: `## Step N/9: Title`
- After presenting each step's findings, use `AskUserQuestion` to collect the user's decision
- Never present the next step until the user has responded to the current one
- Use `AskUserQuestion` for ALL user interactions (never ask inline text questions)

**AskUserQuestion patterns:**

For **short choice lists** (2-4 options), use `AskUserQuestion` directly with the options.

For **long toggle lists** (tools, configs, plugins, MCP servers with more than 4 items), first show the detected items as a text list, then use `AskUserQuestion` with these standard options:

```
options:
  - label: "Accept all"
    description: "Include all items shown above"
  - label: "Exclude some"
    description: "Tell me which items to remove from the list"
  - label: "None"
    description: "Skip this section entirely"
```

If the user selects "Exclude some", ask in a follow-up text message which items to remove (by number or name). If the list has 4 or fewer items, use `AskUserQuestion` with `multiSelect: true` directly instead of the accept/exclude pattern.

---

## Preparation (silent)

Before presenting Step 1, silently perform all analysis:
- Identify repositories (from user input or current directory scan)
- Analyze build files, CI configs, tool version files, container files
- Deduplicate and resolve version conflicts
- Detect network domain groups
- Detect tool configuration files
- For each repository, run `git -C <path> remote get-url origin` to capture the clone URL

Then present Step 1 with the results.

---

## Step 1/9: Detected Tools

Analyze each repository for these files (if present):

**Build files**: `go.mod`, `package.json`, `Cargo.toml`, `pyproject.toml`, `Gemfile`, `pom.xml`, `build.gradle`, `build.gradle.kts`, `settings.gradle`, `Makefile`, `CMakeLists.txt`

**CI configs**: `.github/workflows/*.yml`, `.gitlab-ci.yml`, `Jenkinsfile`, `.circleci/config.yml`

**Tool version files**: `.tool-versions`, `.nvmrc`, `.python-version`, `.sdkmanrc`, `.go-version`, `.ruby-version`, `.java-version`, `rust-toolchain.toml`

**Java-specific**: `mvnw` / `gradlew` (detect Maven/Gradle version from wrapper properties), `jvm.config`, `.mvn/jvm.config` (JVM flags), `pom.xml` `<maven.compiler.source>` / `<maven.compiler.target>` / `<java.version>` properties

**Container files**: `Dockerfile`, `Containerfile` (for system package hints)

For each file found, extract:
- Programming language and version requirements
- Build tools and their versions
- System-level dependencies (compilers, libraries)
- Runtime requirements

Merge findings across all repositories, deduplicate, resolve version conflicts (pick highest compatible version).

**Present** the detected tools as a numbered text list:

```
## Step 1/9: Detected Tools

Scanned N repositories.

  1. Rust stable (edition 2021)         (from repo1/Cargo.toml)
  2. wasm32-wasip1 target               (from repo1/Makefile)
  3. Go >= 1.25                         (from repo2/go.mod)
  4. Node.js >= 18.17.1                 (from repo3/package.json)
  5. npm (bundled with Node)            (from repo3/package.json)
  6. Make (system)                      (from multiple Makefiles)
```

Then use `AskUserQuestion`:

```json
{
  "questions": [{
    "question": "Which detected tools should be added to the manifest?",
    "header": "Tools",
    "multiSelect": false,
    "options": [
      {"label": "Accept all", "description": "Include all 6 detected tools as shown"},
      {"label": "Exclude some", "description": "I'll tell you which tools to remove"},
      {"label": "None", "description": "Don't add any tools to the manifest"}
    ]
  }]
}
```

If user selects "Exclude some", ask which items to remove by number or name, then confirm.

**STOP. Wait for user response before proceeding.**

---

## Step 2/9: cc-deck Companion Tools

Present optional tools from the cc-deck ecosystem and community that enhance the development environment. These are not detected from repositories but offered as curated recommendations.

**Tool catalog** (update this list as the ecosystem grows):

| Tool | Source | Description |
|------|--------|-------------|
| cc-session | `github.com/cc-deck/cc-session` | Terminal session recorder and replayer for Claude Code sessions |
| cc-setup | `github.com/cc-deck/cc-setup` | Environment bootstrapper, manages MCP servers and credentials |
| abtop | `github.com/graykode/abtop` | AI-powered terminal system monitor with natural language queries |

**Present** the catalog as text, then use `AskUserQuestion` with `multiSelect: true`:

```
## Step 2/9: cc-deck Companion Tools

Optional tools that integrate with your cc-deck environment:
```

```json
{
  "questions": [{
    "question": "Which companion tools should be included?",
    "header": "Companion",
    "multiSelect": true,
    "options": [
      {"label": "cc-session", "description": "Terminal session recorder/replayer for Claude Code (github.com/cc-deck/cc-session)"},
      {"label": "cc-setup", "description": "Environment bootstrapper, manages MCP servers and credentials (github.com/cc-deck/cc-setup)"},
      {"label": "abtop", "description": "AI-powered terminal system monitor with natural language queries (github.com/graykode/abtop)"}
    ]
  }]
}
```

**STOP. Wait for user response before proceeding.**

**Action**: For each selected companion tool, add it to the manifest's `github_tools` section with the install metadata:

```yaml
github_tools:
  - name: cc-session
    repo: cc-deck/cc-session
    asset_pattern: "cc-session-{arch}-unknown-linux-gnu.tar.xz"
    install_path: /usr/local/bin/cc-session
  - name: cc-setup
    repo: cc-deck/cc-setup
    asset_pattern: "cc-setup-{arch}-unknown-linux-gnu.tar.xz"
    install_path: /usr/local/bin/cc-setup
  - name: abtop
    repo: graykode/abtop
    asset_pattern: "abtop-{arch}-unknown-linux-gnu.tar.gz"
    install_path: /usr/local/bin/abtop
```

The `{arch}` placeholder is resolved during build to the target architecture (e.g., `x86_64`, `aarch64`).

---

## Step 3/9: Network Domain Groups

Based on the ecosystem files found in Step 1, determine which network domain groups to add to the manifest's `network.allowed_domains` section:

| Ecosystem file | Domain group |
|----------------|-------------|
| `go.mod` | `golang` |
| `pyproject.toml`, `requirements.txt`, `.python-version` | `python` |
| `package.json`, `.nvmrc` | `nodejs` |
| `Cargo.toml`, `rust-toolchain.toml` | `rust` |

Always include `github` if a `.git` directory or `.github/` directory is found.

**Present** the detected groups as text, then use `AskUserQuestion`.

Since domain groups are typically 4 or fewer, use `multiSelect: true` directly:

```json
{
  "questions": [{
    "question": "Which network domain groups should be allowed in the container?",
    "header": "Domains",
    "multiSelect": true,
    "options": [
      {"label": "golang", "description": "Go module proxy, sum database (from go.mod)"},
      {"label": "rust", "description": "crates.io, docs.rs (from Cargo.toml)"},
      {"label": "nodejs", "description": "npm registry (from package.json)"},
      {"label": "github", "description": "GitHub API, raw content (from .git/)"}
    ]
  }]
}
```

If more than 4 groups are detected, fall back to the accept/exclude pattern.

**STOP. Wait for user response before proceeding.**

---

## Step 4/9: Tool Configuration Files

For each tool accepted in Steps 1-2 (and any tools already in the manifest), check whether a corresponding config file exists locally under `~/.config/`.

**Skip list**: `zellij` (handled separately in Step 6), `git` (global gitconfig is not XDG by default).

**Name aliases** (tool binary name differs from config directory):

| Binary name | Config directory name |
|---|---|
| `hx` | `helix` |
| `btm` | `bottom` |

For all other tools, the config directory name matches the tool name.

**Discovery procedure** for each tool (using the resolved config name):

1. Check `~/.config/<name>.toml` (top-level file, e.g., `starship.toml`)
2. Check `~/.config/<name>/config.*` (any extension: `.toml`, `.yaml`, `.yml`, `.json`, `.kdl`, `.ron`, no extension)
3. Check `~/.config/<name>/<name>.*` (self-named config, e.g., `yazi/yazi.toml`, `alacritty/alacritty.toml`)
4. Check `~/.config/<name>/` for any config-like files (`*.toml`, `*.yaml`, `*.yml`, `*.json`, `*.ron`, `*.kdl`)

Stop at the first match. If multiple files are found in step 4, prefer `config.*` over others, then pick the largest file. Record the full path of the discovered config file.

**Present** the findings as a numbered text list showing which tools have configs and which don't:

```
## Step 4/9: Tool Configuration Files

  1. starship   ~/.config/starship.toml (42 lines)
  2. helix      ~/.config/helix/config.toml (78 lines)
  3. k9s        ~/.config/k9s/config.yaml (15 lines)
  4. ripgrep    (no config found)
  5. bat        (no config found)
```

Then use `AskUserQuestion`. If 4 or fewer tools have configs, use `multiSelect: true` with only the tools that have configs. If more than 4, use the accept/exclude pattern.

**STOP. Wait for user response before proceeding.**

**Action** (after user confirms): For each accepted tool config:
1. Copy the config file to `<setup-dir>/config/` with a descriptive name (e.g., `starship.toml`, `helix-config.toml`)
2. Add an entry to `settings.tool_configs` in the manifest, always including `target` with the path relative to `~/.config/` on the target machine:

```yaml
settings:
  tool_configs:
    - tool: starship
      source: ./config/starship.toml
      target: starship.toml
    - tool: helix
      source: ./config/helix-config.toml
      target: helix/config.toml
```

The `source` field is relative to the setup directory. The `target` field is relative to `~/.config/` and tells the build command exactly where to place the file.

---

## Step 5/9: Shell Configuration

**Step 4a: Ask which shell to use**

Use `AskUserQuestion`:

```json
{
  "questions": [{
    "question": "Which shell should the container use?",
    "header": "Shell",
    "multiSelect": false,
    "options": [
      {"label": "zsh (Recommended)", "description": "Includes starship prompt, fzf, zoxide out of the box"},
      {"label": "bash", "description": "Standard bash shell"},
      {"label": "Base image default", "description": "Keep whatever the base image provides (zsh)"}
    ]
  }]
}
```

**STOP. Wait for user response.**

**Step 4b: Analyze and present curated config**

Based on the chosen shell, read the corresponding config file (`~/.zshrc` or `~/.bashrc`).

**Curate a container-ready config**: Generate a new startup file for the container by analyzing the local one. The base image already provides starship, zoxide, fzf, bat, lsd, git-delta (for zsh), so do not duplicate those.

**Extract** (include in curated config):
- Custom aliases (but skip macOS-specific ones like `pbcopy`, `open`, `brew`)
- Custom functions
- Environment variables (PATH additions, EDITOR, GOPATH, etc.)
- Git configuration aliases
- Custom keybindings (`bindkey` for zsh, `bind` for bash)
- Plugin manager config (oh-my-zsh, zinit for zsh; bash-it for bash) if portable
- Custom prompt settings (but note starship is available in the base image)
- Tool initializations that work on Linux (pyenv, rbenv, nvm, sdkman, etc.)

**Strip out** (exclude from curated config):
- macOS-specific: `pbcopy`, `pbpaste`, `open`, `osascript`, Homebrew paths (`/opt/homebrew/`, `/usr/local/Cellar/`)
- macOS conditionals: `[[ "$OSTYPE" == "darwin"* ]]` blocks
- Hardware-specific: display/audio settings, Bluetooth, macOS defaults
- Local tool paths: `/Users/<username>/`, `/Applications/`
- Desktop app integrations: iTerm2 shell integration, VS Code shell integration
- Duplicate of base image defaults (for zsh): starship init, zoxide init, fzf source, bat/lsd aliases

**Guard unresolved commands**: After curating, scan the proposed config for commands referenced in `alias`, `compdef`, and function bodies. Cross-reference against:
- Tools accepted in Steps 1-2
- Tools in the base image (git, curl, tar, zsh, bat, lsd, starship, fzf, zoxide, jq, rg)
- Standard system commands (ls, cd, grep, etc.)

For any command not in these lists, **wrap it with an availability check** rather than removing it:
- `compdef X=Y` -> `(( $+commands[X] )) && compdef X=Y`
- `alias foo="bar"` -> `(( $+commands[bar] )) && alias foo="bar"` (only if `bar` is the unresolved command)

Show the guarded lines in the "Stripped" summary so the user sees what was wrapped.

**Present** the curated config as text:

```
## Step 5/9: Shell Configuration

Analyzed: ~/.zshrc (85 lines)

Proposed curated .zshrc for container:
---
# Custom aliases
alias k='kubectl'
alias kx='kubectx'

# Environment
export EDITOR="hx"
export GOPATH="$HOME/go"
export PATH="$HOME/go/bin:$PATH"

# Git aliases
alias gs='git status'
---

Stripped (macOS/local-specific):
  - 3 Homebrew path entries
  - 2 pbcopy/pbpaste aliases
  - 1 iTerm2 shell integration block
  - 5 lines already in base image .zshrc
```

Then use `AskUserQuestion`:

```json
{
  "questions": [{
    "question": "Accept the proposed shell configuration?",
    "header": "Shell RC",
    "multiSelect": false,
    "options": [
      {"label": "Accept", "description": "Use the curated shell config as shown above"},
      {"label": "Edit", "description": "I'll make changes to the proposed config"},
      {"label": "Skip", "description": "Don't include custom shell configuration"}
    ]
  }]
}
```

**STOP. Wait for user response before proceeding.**

**Action**: Write the curated config to `<setup-dir>/config/` and update manifest:

For **zsh**:
```yaml
settings:
  shell: zsh
  shell_rc: ./config/zshrc
```
The Containerfile appends the curated content to the base image's `.zshrc` (do not replace it):
```dockerfile
COPY --chown=dev:dev zshrc /home/dev/.zshrc.custom
RUN cat /home/dev/.zshrc.custom >> /home/dev/.zshrc && rm /home/dev/.zshrc.custom
```

For **bash**:
```yaml
settings:
  shell: bash
  shell_rc: ./config/bashrc
```
The Containerfile sets bash as the default shell and appends the curated config:
```dockerfile
RUN chsh -s /bin/bash dev 2>/dev/null || usermod -s /bin/bash dev
COPY --chown=dev:dev bashrc /home/dev/.bashrc.custom
RUN cat /home/dev/.bashrc.custom >> /home/dev/.bashrc && rm /home/dev/.bashrc.custom
```

The Zellij `config.kdl` `default_shell` is set to match the chosen shell.

---

## Step 6/9: Zellij Configuration

**Scan**: Check `~/.config/zellij/` for:
- `config.kdl` (main config with keybindings, theme, etc.)
- Any `.kdl` files in `themes/` subdirectory

**Exclude** (managed by cc-deck, never copy):
- `plugins/cc_deck.wasm`
- `layouts/cc-deck.kdl`
- `layouts/cc-deck-minimal.kdl`
- `layouts/cc-deck-standard.kdl`
- `layouts/cc-deck-clean.kdl`
- `layouts/cc-deck-personal.kdl`

**Present** findings as text, then use `AskUserQuestion` with `multiSelect: true` (typically 2-4 items):

```
## Step 6/9: Zellij Configuration

  Excluded (managed by cc-deck): plugins/cc_deck.wasm, layouts/cc-deck*.kdl
```

```json
{
  "questions": [{
    "question": "Which Zellij config files should be included?",
    "header": "Zellij",
    "multiSelect": true,
    "options": [
      {"label": "config.kdl", "description": "Main config: keybindings, options, theme (570 lines)"},
      {"label": "themes/claude-dark.kdl", "description": "Custom theme file"}
    ]
  }]
}
```

**STOP. Wait for user response before proceeding.**

If the user selected a theme file, ask about a remote background color for visual distinction:

```json
{
  "questions": [{
    "question": "Use a distinct background color for remote sessions (SSH/container)?",
    "header": "Remote BG",
    "multiSelect": false,
    "options": [
      {"label": "Dark blue (#1a2a3a)", "description": "Subtle blue tint, easy to distinguish from local"},
      {"label": "Dark green (#1a2a1a)", "description": "Subtle green tint"},
      {"label": "Dark purple (#2a1a2a)", "description": "Subtle purple tint"},
      {"label": "No change", "description": "Same background as local sessions"}
    ]
  }]
}
```

**STOP. Wait for user response before proceeding.**

**Action**: Copy selected files to `<setup-dir>/config/`. If the user's `config.kdl` does not contain `default_shell`, append `default_shell "zsh"` to ensure Zellij panes start with zsh (required for starship prompt and shell integrations).

If the user chose a remote background color, add it to the manifest:
```yaml
settings:
  remote_theme_bg: "<chosen color>"
```

Update manifest:
```yaml
settings:
  zellij_config: current
```

---

## Step 7/9: Claude Configuration

**Scan** `~/.claude/` for:
- `CLAUDE.md` (global user instructions)
- `settings.json` (user preferences, hooks)
- `todos/` directory (persistent todos)

Use `AskUserQuestion` with `multiSelect: true`:

```json
{
  "questions": [{
    "question": "Which Claude configuration files should be included?",
    "header": "Claude",
    "multiSelect": true,
    "options": [
      {"label": "CLAUDE.md", "description": "Global user instructions (180 lines)"},
      {"label": "settings.json", "description": "Model preferences, hooks, permissions (will merge with cc-deck hooks, not overwrite)"}
    ]
  }]
}
```

**STOP. Wait for user response before proceeding.**

**Action**: Copy selected files and update manifest:
```yaml
settings:
  claude_md: ./config/CLAUDE.md
  claude_settings: ./config/claude-settings.json
```

For `settings.json`, extract only portable preferences (model, theme, permissions) and omit machine-specific paths. Save as `claude-settings.json` in `<setup-dir>/config/`.

---

## Step 8/9: Claude Code Plugins

**Scan**: Discover installed plugins and their marketplace sources:
1. Run `claude plugins list` to get all installed plugins
2. Run `claude plugins marketplace list` to get all registered marketplaces
3. Read `~/.claude/settings.json` for `enabledPlugins` entries (format: `pluginName@marketplaceName`)

For each installed plugin, determine the marketplace source type:
- **Official marketplace** (`anthropics/claude-code` or `anthropics/claude-plugins-official`): record `source: marketplace`
- **GitHub-based marketplace** (custom GitHub repo): record `source: "github:<owner>/<repo>"` and `marketplace: <marketplace-name>`
- **Directory-based marketplace** (local development path): record `source: directory`

**Present** as a categorized text list:

```
## Step 8/9: Claude Code Plugins

  Marketplace plugins:
    1. superpowers (official)
    2. gopls-lsp (official)
    3. rust-analyzer-lsp (official)

  GitHub plugins:
    4. copyedit (github:org/cc-copyedit)
    5. cc-rosa (github:cc-deck/cc-rosa-rhoai)

  Local plugins (cannot install remotely):
    6. prose (directory, local dev)
    7. blog (directory, local dev)

  Disabled:
    8. hugo (directory)
    9. playwright (marketplace)
```

Then use `AskUserQuestion`:

```json
{
  "questions": [{
    "question": "Which Claude Code plugins should be included in the manifest?",
    "header": "Plugins",
    "multiSelect": false,
    "options": [
      {"label": "All active", "description": "Include all currently enabled plugins (items 1-7)"},
      {"label": "Marketplace only", "description": "Only include marketplace and GitHub plugins (skip local directory plugins)"},
      {"label": "Customize", "description": "I'll tell you exactly which plugins to include or exclude"},
      {"label": "None", "description": "Don't include any plugins"}
    ]
  }]
}
```

If user selects "Customize", ask which items to include/exclude by number or name.

**STOP. Wait for user response before proceeding.**

**Action**: Update the manifest plugins section:
```yaml
plugins:
  - name: sdd
    source: marketplace
  - name: cc-rosa
    source: "github:cc-deck/cc-rosa-rhoai"
    marketplace: cc-rosa-dev-marketplace
  - name: copyedit
    source: "github:org/cc-copyedit"
    marketplace: cc-rhuss-marketplace
```

Directory-based plugins are included in the manifest with `source: directory` so the user sees them, but the build command will skip them with a warning.

---

## Step 9/9: MCP Servers

**Scan** MCP configuration from multiple sources:
1. `~/.config/cc-setup/mcp.json` (cc-setup cached MCP configs)
2. `~/.claude/settings.json` (Claude Code MCP settings under `mcpServers`)
3. Project-level `.claude/settings.json` if present

**Present** as a categorized text list:

```
## Step 9/9: MCP Servers

  From cc-setup cache:
    1. github-mcp (container, ghcr.io/..., sse, port 8000)
    2. filesystem-mcp (stdio)

  From Claude settings:
    3. playwright (npx @playwright/mcp@latest, stdio)
    4. readwise (http, mcp-readwise.int-tichny.org)
    5. google-private (http, mcp-google-private.int-tichny.org)
    6. slack-redhat (sse, mcp-slack.int-tichny.org)
```

Then use `AskUserQuestion`:

```json
{
  "questions": [{
    "question": "Which MCP servers should be included?",
    "header": "MCP",
    "multiSelect": false,
    "options": [
      {"label": "Accept all", "description": "Include all 6 MCP servers shown above"},
      {"label": "Exclude some", "description": "I'll tell you which MCP servers to remove"},
      {"label": "None", "description": "Don't include any MCP servers"}
    ]
  }]
}
```

If user selects "Exclude some", ask which items to remove by number or name.

**STOP. Wait for user response before proceeding.**

**Action**: For selected MCP servers:

- **Container-based MCP** (from cc-setup with image references): Add to manifest `mcp` section with full config (image, transport, port, auth):
  ```yaml
  mcp:
    - name: github-mcp
      image: ghcr.io/modelcontextprotocol/github-mcp:latest
      transport: sse
      port: 8000
      auth:
        type: token
        env_vars: [GITHUB_TOKEN]
  ```

- **npx-based MCP** (from Claude settings): Copy the MCP server config to a `mcp-settings.json` file that gets merged into the container's Claude settings:
  ```json
  {
    "mcpServers": {
      "playwright": {
        "command": "npx",
        "args": ["@anthropic-ai/mcp-playwright"]
      }
    }
  }
  ```

- **cc-setup cache**: If the user selects MCP servers from cc-setup, copy the relevant entries from `~/.config/cc-setup/mcp.json` to `<setup-dir>/config/` as `cc-setup-mcp.json`. The Containerfile will place it at `/home/dev/.config/cc-setup/mcp.json`.

---

## Summary and Manifest Update

After all 9 steps are complete, show a final text summary:

```
## Capture Complete

  Tools:       Rust stable, wasm32-wasip1, Go >= 1.25, Make (4 selected)
  Network:     golang, rust, github (3 groups)
  Tool configs: starship, helix (2 selected)
  Shell:       zsh (42 custom lines)
  Zellij:      config.kdl + themes/claude-dark.kdl
  Claude:      CLAUDE.md + settings (merged)
  Plugins:     superpowers, gopls-lsp, copyedit (3 selected)
  MCP:         playwright, readwise (2 selected)
```

Then use `AskUserQuestion`:

```json
{
  "questions": [{
    "question": "Write these selections to the manifest?",
    "header": "Confirm",
    "multiSelect": false,
    "options": [
      {"label": "Write manifest", "description": "Save all selections to build.yaml and copy config files to <setup-dir>/config/"},
      {"label": "Go back", "description": "Review or change selections for a specific step"},
      {"label": "Cancel", "description": "Discard all selections without writing"}
    ]
  }]
}
```

**STOP. Wait for user confirmation.**

Then perform the write:

1. Update `tools` section with accepted tool entries (as free-form text)
2. Update `sources` section with repository provenance. For each repo, include:
   - `path`: local directory name
   - `url`: git remote origin URL (from `git remote get-url origin`)
   - `detected_tools`: tools found in this repo
   - `detected_from`: files that were analyzed
   The `url` field is used by `ws new --repo` to clone repos on remote targets.
3. Update `network.allowed_domains` with detected domain groups
4. Update `settings` section with shell, Zellij, Claude selections
5. Update `plugins` section with selected plugins
6. Update `mcp` section with selected MCP servers
7. Copy selected config files to `<setup-dir>/config/`
8. Report what was written

---

## Key Rules

- Never modify tools or settings the user explicitly rejected
- Keep tool descriptions human-readable (e.g., "Rust stable >= 1.80", not "rustc-1.80.0")
- Record which files each tool was detected from (for provenance)
- If re-running on an already-analyzed repo, update existing entries and highlight changes
- **NEVER include container runtimes** (podman, docker, buildah, skopeo) as detected tools. These are host build tools, not image dependencies. The base image does not need them. If found in CI configs or Containerfiles, silently exclude them from the results.
- For settings.json, always MERGE (preserve cc-deck hooks)
- Exclude cc-deck managed files from Zellij config (layouts, plugin)
- For MCP auth, only store env var NAMES, never actual credentials
- Show file sizes and line counts to help users decide what to include
- If a file or directory does not exist, silently skip that step (do not error, adjust step count accordingly)
- Re-running this command should show current selections and allow modifications

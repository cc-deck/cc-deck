---
description: Select local settings, plugins, and MCP servers to bake into the container image
---

## User Input

$ARGUMENTS

## Outline

Interactive wizard that discovers local settings and lets you choose what to transport into the container image. Updates `cc-deck-build.yaml` and copies selected files to the build directory.

### Step 1: Discover local settings

Scan the local system and present findings organized by section. For each section, show what was found and let the user select what to include.

---

### Section A: Shell Configuration

**Step A.1: Ask which shell to use**

```
Which shell should the container use?
  1. zsh (default, includes starship prompt, fzf, zoxide)
  2. bash
  3. Keep base image default (zsh)
```

Record the choice. This determines:
- Which startup file to analyze (`.zshrc` or `.bashrc`)
- The `default_shell` setting in Zellij's `config.kdl`
- Whether to set `chsh` in the Containerfile

**Step A.2: Analyze the shell config file**

Based on the chosen shell, read the corresponding config file:
- **zsh**: `~/.zshrc`
- **bash**: `~/.bashrc`

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

**Present**:
```
Zsh Configuration:
  Analyzed: ~/.zshrc (85 lines)

  Proposed curated .zshrc for container:
  ─────────────────────────────────────
  # Custom aliases
  alias k='kubectl'
  alias kx='kubectx'
  alias tf='terraform'

  # Environment
  export EDITOR="hx"
  export GOPATH="$HOME/go"
  export PATH="$HOME/go/bin:$PATH"

  # Git aliases
  alias gs='git status'
  alias gd='git diff'

  # Custom functions
  mkcd() { mkdir -p "$1" && cd "$1" }
  ─────────────────────────────────────

  Stripped (macOS/local-specific):
    - 3 Homebrew path entries
    - 2 pbcopy/pbpaste aliases
    - 1 iTerm2 shell integration block
    - 5 lines already in base image .zshrc

  Accept proposed .zshrc? [y/edit/skip]
    y    = use the proposed curated .zshrc
    edit = open for manual editing before saving
    skip = don't include custom zsh config
```

Let the user review and edit before accepting. If they choose "edit", show the content and let them modify it.

**Action**: Write the curated config to the build directory and update manifest:

For **zsh**:
```yaml
settings:
  shell: zsh
  shell_rc: ./zshrc
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
  shell_rc: ./bashrc
```
The Containerfile sets bash as the default shell and appends the curated config:
```dockerfile
RUN chsh -s /bin/bash dev 2>/dev/null || usermod -s /bin/bash dev
COPY --chown=dev:dev bashrc /home/dev/.bashrc.custom
RUN cat /home/dev/.bashrc.custom >> /home/dev/.bashrc && rm /home/dev/.bashrc.custom
```

The Zellij `config.kdl` `default_shell` is set to match the chosen shell.

---

### Section B: Zellij Configuration

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

**Present**:
```
Zellij Configuration:
  Found: ~/.config/zellij/config.kdl (keybindings, options, theme)
  Found: ~/.config/zellij/themes/catppuccin.kdl

  Excluded (managed by cc-deck):
    plugins/cc_deck.wasm, layouts/cc-deck*.kdl

  Include config.kdl? [y/n]
  Include themes? [y/n]
```

**Action**: Copy selected files to the build directory. If the user's `config.kdl` does not contain `default_shell`, append `default_shell "zsh"` to ensure Zellij panes start with zsh (required for starship prompt and shell integrations).

Update manifest:
```yaml
settings:
  zellij_config: current
```

---

### Section C: Claude Configuration

**Scan** `~/.claude/` for:
- `CLAUDE.md` (global user instructions)
- `settings.json` (user preferences, hooks)
- `todos/` directory (persistent todos)

**Present**:
```
Claude Configuration:
  Found: ~/.claude/CLAUDE.md (180 lines)
  Found: ~/.claude/settings.json (model preferences, hooks, permissions)

  Include CLAUDE.md? [y/n]
  Include settings.json? [y/n] (will merge with cc-deck hooks, not overwrite)
```

**Action**: Copy selected files and update manifest:
```yaml
settings:
  claude_md: ./CLAUDE.md
  claude_settings: ./claude-settings.json
```

For `settings.json`, extract only portable preferences (model, theme, permissions) and omit machine-specific paths. Save as `claude-settings.json` in the build directory.

---

### Section D: Claude Code Plugins

**Scan**: Discover installed plugins by reading:
- `~/.claude/settings.json` (look for `plugins` or `pluginDirs` entries)
- `~/.claude/plugins/` directory
- Claude Code plugin cache locations

**Present**:
```
Claude Code Plugins:
  [x] sdd (marketplace)
  [x] cc-rosa (marketplace)
  [ ] prose (marketplace)
  [x] copyedit (git:https://github.com/org/copyedit.git)

  Toggle with numbers, 'a' for all, 'n' for none, Enter to confirm:
```

Show all discovered plugins with checkboxes. Pre-select all that are currently active.

**Action**: Update the manifest plugins section:
```yaml
plugins:
  - name: sdd
    source: marketplace
  - name: cc-rosa
    source: marketplace
  - name: copyedit
    source: "git:https://github.com/org/copyedit.git"
```

For marketplace plugins, also check if there are cached plugin directories that should be copied. Copy the plugin's marketplace metadata so `cc-deck plugin install` can restore them inside the container.

---

### Section E: MCP Servers

**Scan** MCP configuration from multiple sources:
1. `~/.config/cc-setup/mcp.json` (cc-setup cached MCP configs)
2. `~/.claude/settings.json` (Claude Code MCP settings under `mcpServers`)
3. Project-level `.claude/settings.json` if present

**Present**:
```
MCP Servers:
  From cc-setup cache (~/.config/cc-setup/mcp.json):
    [x] github-mcp (ghcr.io/modelcontextprotocol/github-mcp:latest, sse, port 8000)
    [ ] filesystem-mcp (stdio)
    [x] postgres-mcp (ghcr.io/org/postgres-mcp:latest, sse, port 9000)

  From Claude settings:
    [x] playwright (npx @anthropic-ai/mcp-playwright)
    [ ] memory (npx @anthropic-ai/mcp-memory)

  Toggle with numbers, 'a' for all, 'n' for none, Enter to confirm:
```

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

- **cc-setup cache**: If the user selects MCP servers from cc-setup, copy the relevant entries from `~/.config/cc-setup/mcp.json` to the build directory as `cc-setup-mcp.json`. The Containerfile will place it at `/home/dev/.config/cc-setup/mcp.json`.

---

### Step 2: Summary and confirmation

Show a summary of all selections:

```
Settings to bake into image:

  Zsh:     ~/.zshrc (42 lines)
  Zellij:  config.kdl + themes/catppuccin.kdl
  Claude:  CLAUDE.md + settings (merged)
  Plugins: sdd, cc-rosa, copyedit (3 selected)
  MCP:     github-mcp, postgres-mcp, playwright (3 selected)

Proceed? [y/n/edit]
```

### Step 3: Write files and update manifest

1. Copy all selected files to the build directory
2. Update `cc-deck-build.yaml` with the selections
3. Report what was written

### Key Rules

- Never overwrite existing manifest entries without asking
- For settings.json, always MERGE (preserve cc-deck hooks)
- Exclude cc-deck managed files from Zellij config (layouts, plugin)
- For MCP auth, only store env var NAMES, never actual credentials
- Show file sizes and line counts to help users decide what to include
- If a file doesn't exist, silently skip it (don't error)
- Re-running this command should show current selections and allow modifications

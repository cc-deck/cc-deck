---
description: "Build workspace: --target ssh | --target container | --target openshell [--push]"
---

## User Input

$ARGUMENTS

**Usage**: `/cc-deck.build --target ssh` or `/cc-deck.build --target container [--push]` or `/cc-deck.build --target openshell [--push]`

## Setup directory

All setup artifacts live in `.cc-deck/setup/` relative to the project root (the git root or the directory containing `.cc-deck/`). Resolve the setup directory first:

1. Find the project root (look for `.cc-deck/` directory or git root, walking up from the current working directory)
2. The setup directory is `<project-root>/.cc-deck/setup/`
3. The manifest is at `<setup-dir>/build.yaml`

All file references in this command (manifest, Containerfile, container/context/, Ansible artifacts) are relative to the setup directory unless stated otherwise.

## Outline

Build a target workspace from the `build.yaml` manifest. Requires `--target container` or `--target ssh` in the arguments. Optionally accepts `--push` (container only).

### Step 0: Target dispatch

Parse `$ARGUMENTS` for the `--target` flag.

Read `<setup-dir>/build.yaml` and validate it has `version >= 1`.

Determine which targets are configured in the manifest:
- `targets.container` is configured if it has a `name` field
- `targets.ssh` is configured if it has a `host` field
- `targets.openshell` is configured if it has a `name` field

| Condition | Action |
|-----------|--------|
| `--target container` | Go to **Section A: Container Build** |
| `--target ssh` | Go to **Section B: SSH Provisioning** |
| `--target openshell` | Go to **Section C: OpenShell Build** |
| `--target` not provided, only one configured | Auto-select that target |
| `--target` not provided, multiple configured | Error: "Multiple targets configured. Specify --target container, --target ssh, or --target openshell" |
| `--target` not provided, none configured | Error: "No targets configured in manifest. Add a targets section." |
| `--push` with `--target ssh` | Error: "--push is only valid with --target container or --target openshell" |

If the selected target section is missing from the manifest, error: "No [target] section in manifest. Run `cc-deck build init --target [target]` or add it manually."

---

## Section A: Container Build

### A1: Read and validate

Read `build.yaml`. Extract from `targets.container`:
- `name` (required)
- `tag` (default: `latest`)
- `base` (default: `quay.io/cc-deck/cc-deck-base:latest`)
- `registry` (optional, enables `--push`)

### A2: Generate the Containerfile

Assemble the Containerfile by composing pre-rendered snippet files with generated sections.

**Snippet files**: Located in `container/snippets/`. These are pre-rendered Containerfile fragments with all paths and variables resolved. Copy their content EXACTLY as-is into the Containerfile. Do NOT modify snippet content.

**Base image note**: The snippet `01-header.txt` already contains the correct FROM line. The base image is Fedora and already includes common developer tools (git, jq, zsh, nodejs, npm, python3, uv, ripgrep, bat, lsd, starship, etc.), so do NOT reinstall packages that the base image already provides.

**Tool resolution**: Tools are read from the unified `tools` section and dispatched by `install` field:
- `install: package` (or omitted): resolved to `dnf install -y` commands for Fedora repos, or language-specific installers for tools not in Fedora repos
- `install: github-release`: downloaded from GitHub Releases using the `repo`, `asset_pattern`, and `install_path` fields. Asset pattern placeholders: `{arch}` (x86_64/aarch64), `{goarch}` (amd64/arm64), `{version}` (latest release tag from GitHub API)
- Use `${TARGETARCH}` for multi-arch GitHub release downloads in Containerfile layers

**Assembly order** (generate the Containerfile by following these steps in sequence):

1. Read `container/snippets/01-header.txt` and copy its content verbatim
2. Read `container/snippets/02-user-setup.txt` and copy its content verbatim
3. **GENERATE**: System packages layer (`dnf install -y` for `PackageTools()` not in base image)
4. **GENERATE**: Language-specific tools layer (version-specific installs)
5. **GENERATE**: GitHub release tools layer (curl downloads for `GithubReleaseTools()`)
6. Read `container/snippets/03-mandatory-stack.txt` and copy its content verbatim
7. **GENERATE**: Plugin install commands (see Plugin handling below)
8. **GENERATE**: User configuration layers (see Settings handling below)
9. Read `container/snippets/06-footer.txt` and copy its content verbatim

**Plugin handling**: For each plugin entry in the manifest:
- `source: marketplace` -> `claude plugins install <name>`
- `source: github:<owner/repo>` -> `claude plugins marketplace add <owner/repo>` then `claude plugins install <name>@<marketplace>`
- `source: directory` -> skip (local dev only)

Wrap plugin commands in `USER dev` / `USER root` blocks.

**Settings handling** (read the `settings` section from the manifest):

For each setting, copy the source file to `container/context/` during Step A4, then add the matching COPY instruction to the Containerfile:

| Manifest field | Source | Container destination | Notes |
|---|---|---|---|
| `settings.shell` | `zsh` or `bash` | Sets `default_shell` in config.kdl and `chsh` | Default: `zsh` |
| `settings.shell_rc` | The specified path | Appended to shell rc file (`.zshrc` or `.bashrc`) | Curated additions (base image rc preserved) |
| `settings.zellij_config: current` | `~/.config/zellij/config.kdl` | `/home/dev/.config/zellij/config.kdl` | Strip controller block before copying (see below) |
| `settings.zellij_config: <path>` | The specified path | `/home/dev/.config/zellij/config.kdl` | Strip controller block before copying (see below) |
| `settings.zellij_config: vanilla` | (skip) | (nothing) | Use cc-deck defaults only |
| `settings.remote_bg` | Hex color (e.g., `#0d1b2a`) | Added as `set-bg` call in shell RC | Terminal background for remote sessions |
| `settings.claude_md` | The specified path | `/home/dev/.claude/CLAUDE.md` | Global user instructions for Claude |
| `settings.claude_settings` | The specified path | Merge into `/home/dev/.claude/settings.json` | Merge user preferences with existing settings |
| `settings.hooks` | The specified path | Merge into `/home/dev/.claude/settings.json` | Merge with cc-deck hooks, do not overwrite |
| `settings.mcp_settings` | The specified path | Merge into `/home/dev/.claude/settings.json` | npx-based MCP server configs |
| `settings.cc_setup_mcp` | The specified path | `/home/dev/.config/cc-setup/mcp.json` | cc-setup MCP server cache |
| `settings.git_config` | Map of git config keys/values | `git config --global` in RUN layer | Sets git identity (user.name, user.email) |
| `settings.tool_configs[]` | Each entry's `source` path | `/home/dev/.config/<target>` | One COPY per tool config entry |

**Tool config destination**: Each `tool_configs` entry has a `target` field that specifies the path relative to `~/.config/` (e.g., `starship.toml`, `helix/config.toml`). The container destination is `/home/dev/.config/<target>`. If an entry lacks a `target` field, fall back to `/home/dev/.config/<tool>/<source-filename>`.

**Zellij config.kdl sanitization**: When copying `config.kdl` to the build context, **always strip the cc-deck controller block** (the lines between `// cc-deck-controller-start` and `// cc-deck-controller-end` inclusive). This block contains absolute paths from the host machine that will be wrong on the target. The `cc-deck config plugin install` command (run later in the mandatory layers) re-injects the controller block with the correct target paths.

Use `/cc-deck.capture` to interactively select what to include before building.

**Containerfile COPY examples** (add these to the "User configuration" layer):

```dockerfile
# Git identity (if settings.git_config is set)
USER dev
RUN git config --global user.name "Roland Huß" && \
    git config --global user.email "roland@example.com"
USER root

# Shell config (if settings.shell_rc is set, append to base image rc file)
COPY --chown=dev:dev <shell_rc_file> /home/dev/.<shell>rc.custom
RUN cat /home/dev/.<shell>rc.custom >> /home/dev/.<shell>rc && rm /home/dev/.<shell>rc.custom

# Zellij user config (if settings.zellij_config is set)
# NOTE: container/context/zellij-config.kdl must have the cc-deck controller block stripped first (see below)
COPY --chown=dev:dev container/context/zellij-config.kdl /home/dev/.config/zellij/config.kdl
RUN grep -qE '^default_shell' /home/dev/.config/zellij/config.kdl || \
    echo 'default_shell "<chosen-shell>"' >> /home/dev/.config/zellij/config.kdl

# Claude global instructions (if settings.claude_md is set)
COPY --chown=dev:dev container/context/CLAUDE.md /home/dev/.claude/CLAUDE.md

# Claude settings merge (if settings.claude_settings, hooks, or mcp_settings is set)
COPY --chown=dev:dev container/context/settings.json /home/dev/.claude/settings.json

# cc-setup MCP cache (if settings.cc_setup_mcp is set)
COPY --chown=dev:dev container/context/cc-setup-mcp.json /home/dev/.config/cc-setup/mcp.json

# Tool configs (for each entry in settings.tool_configs)
COPY --chown=dev:dev container/context/starship.toml /home/dev/.config/starship.toml
COPY --chown=dev:dev container/context/helix-config.toml /home/dev/.config/helix/config.toml

# Starship prompt init (if starship is available in the image)
# Guard with interactive check to prevent issues in non-interactive sessions
RUN SHELL_RC="/home/dev/.$(basename $(getent passwd dev | cut -d: -f7))rc"; \
    if command -v starship >/dev/null 2>&1 && ! grep -q 'starship init' "$SHELL_RC" 2>/dev/null; then \
      SHELL_NAME=$(basename $(getent passwd dev | cut -d: -f7)); \
      echo '[[ $- == *i* ]] && eval "$(starship init '"$SHELL_NAME"')"' >> "$SHELL_RC"; \
    fi

# Re-inject cc-deck controller and hooks after config copies overwrite them.
# Only needed when zellij_config or claude_settings are deployed above.
USER dev
RUN cc-deck config plugin install --force --skip-backup
USER root
```

**Merge strategy for settings.json**: Read the existing `/home/dev/.claude/settings.json` (created by cc-deck config plugin install with hooks), merge in user preferences from the specified file, write the merged result to `container/context/settings.json`. Never overwrite cc-deck hooks.

### A3: Check for existing Containerfile

If a `container/Containerfile` already exists:

1. Generate the new Containerfile content to a temporary variable
2. Compare old and new content
3. If they differ, show the diff to the user and ask:
   - **"Use new Containerfile"**: overwrite with the generated version
   - **"Keep existing Containerfile"**: use the existing file as-is
   - **"Stop"**: abort the build entirely
4. If they are identical, proceed silently

If no Containerfile exists, write the generated one directly.

### A4: Prepare the build context

1. Create `container/context/` directory
2. Determine the cc-deck version by running `cc-deck version -o json` and extracting the `version` field
3. Download Linux binaries for both architectures from GitHub Releases. Skip any architecture whose binary already exists in `container/context/`:
   ```bash
   mkdir -p container/context
   VERSION=$(cc-deck version -o json | jq -r '.version')
   for ARCH in amd64 arm64; do
     if [ ! -f "container/context/cc-deck-linux-${ARCH}" ]; then
       curl -fsSL "https://github.com/cc-deck/cc-deck/releases/download/v${VERSION}/cc-deck_${VERSION}_linux_${ARCH}.tar.gz" \
         | tar xz -C container/context/ cc-deck
       mv container/context/cc-deck "container/context/cc-deck-linux-${ARCH}"
     fi
   done
   ```
4. If the download fails (e.g., version is `dev` or the release does not exist), stop and tell the user:
   - For development builds: run `make cross-cli` from the cc-deck source repo, then copy the binaries to `container/context/`
   - For released versions: check that the version tag exists at `https://github.com/cc-deck/cc-deck/releases`

### A5: Build the image

Detect the container runtime (prefer `podman`, fall back to `docker`).

**IMPORTANT**: Use a 10-minute timeout (600000ms) for the build command. Container builds are slow.

**Default platforms**: `linux/arm64,linux/amd64`. The user can override via input (e.g., "build for linux/amd64 only").

```bash
podman build --platform linux/arm64,linux/amd64 -t <image-name>:<tag> -f container/Containerfile .
```

If the user specified specific platforms in their input, use those instead of the defaults.

### A6: Handle build failures (self-correction loop)

If the build fails:

1. **Read the error output** carefully
2. **Identify the failing step** (which RUN instruction, which layer)
3. **Diagnose the root cause**. Common issues:
   - Package not found: wrong package name, use `dnf search` to find the right one
   - Download failures: wrong URL or architecture, use `${TARGETARCH}`
   - Permission errors: missing `USER root`
   - Binary not found: wrong PATH, need symlinks
4. **Fix the Containerfile** with the corrected commands
5. **Retry the build** (cached layers are reused, only failed steps re-run)
6. **Repeat** until success or 3 fix attempts

After 3 failed attempts, stop and present the remaining error with your analysis.

### A7: Generate build.sh

After a successful build, generate a `container/build.sh` script that lets the user rebuild the image from the command line without Claude Code:

```bash
#!/bin/bash
# Rebuild the container image from the existing Containerfile.
# Generated by /cc-deck.build --target container - regenerate with: claude /cc-deck.build --target container
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

IMAGE_NAME="<image-name>"
IMAGE_TAG="<image-tag>"
PLATFORMS="${PLATFORMS:-linux/arm64,linux/amd64}"

# Detect container runtime
if command -v podman >/dev/null 2>&1; then
  RUNTIME=podman
elif command -v docker >/dev/null 2>&1; then
  RUNTIME=docker
else
  echo "Error: neither podman nor docker found" >&2
  exit 1
fi

echo "Building ${IMAGE_NAME}:${IMAGE_TAG} (platforms: ${PLATFORMS})"
$RUNTIME build --platform "$PLATFORMS" -t "${IMAGE_NAME}:${IMAGE_TAG}" -f Containerfile .
echo "Done: ${IMAGE_NAME}:${IMAGE_TAG}"
```

Fill in the actual `IMAGE_NAME` and `IMAGE_TAG` from the manifest. Make the script executable (`chmod +x container/build.sh`).

### A8: Push (if --push)

If `--push` was specified in the arguments:

1. Check that `targets.container.registry` is set in the manifest. If not, error: "No registry configured. Add `registry` to `targets.container` in build.yaml."
2. Tag the image with the full registry reference: `<registry>/<name>:<tag>`
3. Push the image:
   ```bash
   podman push <registry>/<name>:<tag>
   ```
4. **IMPORTANT**: Use a 10-minute timeout (600000ms) for the push command.
5. If auth fails, suggest `podman login <registry>` and retry.

### A9: Report results

On success, show:
- Image name:tag
- Image size (`podman images <name>:<tag> --format '{{.Size}}'`)
- Number of retry attempts (if any)
- Summary of Containerfile fixes made (if any)
- Note that `container/build.sh` was generated for CLI rebuilds
- If pushed: the full registry reference
- Usage hint: `Create a workspace: cc-deck ws new <name> --type container --image <name>:<tag>`

---

## Section B: SSH Provisioning

### B1: Read and validate

Read `build.yaml`. Extract from `targets.ssh`:
- `host` (required)
- `port` (default: 22)
- `identity_file` (optional)
- `create_user` (default: false)
- `user` (required if `create_user` is true)
- `workspace` (default: `~/workspace`)

### B2: Check Ansible availability

```bash
which ansible-playbook
```

If not available, error: "Ansible is required for SSH provisioning. Install with: brew install ansible (macOS) or pip install ansible"

### B3: Generate Ansible artifacts

Generate the following files from the manifest:

#### ansible.cfg

```ini
[defaults]
deprecation_warnings = false

[ssh_connection]
ssh_args = -o SendEnv=TERM -o SetEnv=TERM=dumb
```

This suppresses deprecation warnings and prevents OSC escape sequences from the remote shell (e.g., starship prompt) that cause "junk after JSON data" warnings.

#### inventory.ini

```ini
[setup_targets]
target ansible_host=<host> ansible_user=<user-from-host> ansible_port=<port> ansible_ssh_private_key_file=<identity_file> ansible_python_interpreter=auto_silent
```

Parse the `host` field: if it contains `@`, split into `user@hostname`. The `ansible_user` comes from the user part. If no `@`, use the current local username.

**Note:** Always include `ansible_python_interpreter=auto_silent` to suppress Python interpreter discovery warnings.

#### group_vars/all.yml

```yaml
# Generated by cc-deck.build --target ssh - do not edit manually
cc_deck_version: "<output of cc-deck version -o json | jq -r .version>"
zellij_version: "<output of zellij --version | awk '{print $2}'>"
create_user: <targets.ssh.create_user>
target_user: <targets.ssh.user>
workspace: <targets.ssh.workspace>
shell: <settings.shell or "zsh">
tools:
  <manifest tools list (unified format with name, install, repo, asset_pattern, install_path fields)>
plugins:
  <manifest plugins list>
git_config:
  <map from settings.git_config, e.g. user.name: "Roland Huß", user.email: "roland@example.com">
tool_configs:
  <list from settings.tool_configs, each with tool, source, and dest_path resolved from the XDG mapping>
```

#### site.yml

```yaml
---
- hosts: setup_targets
  become: true
  roles:
    - base
    - tools
    - zellij
    - claude
    - cc_deck
    - plugins
    - shell_config
    - mcp
```

#### Role task files (roles/*/tasks/main.yml)

Generate task content for each role from the manifest data. Each role is idempotent.

**Ansible best practices (apply to ALL generated roles):**
- Use `ansible_facts['architecture']` instead of `ansible_architecture` (avoids INJECT_FACTS_AS_VARS deprecation)
- Use `ansible_facts['os_family']` instead of `ansible_os_family`
- Never use `{{ item }}` or `{{ item.name }}` in task `name:` fields (causes template errors in Ansible 2.18+). Use static names; the `loop_control.label` handles display.
- Always add `executable: /bin/bash` to `ansible.builtin.shell` tasks (prevents "junk after JSON data" warnings)
- Prefer `ansible.builtin.command` over `ansible.builtin.shell` when no shell features are needed
- Add `export TERM=dumb` at the start of shell tasks that run interactive tools (rustup, claude, cc-deck) to suppress OSC escape sequences that cause "junk after JSON data" warnings

**base role** (`roles/base/tasks/main.yml`):
- Detect OS family (`ansible_facts['os_family']` fact)
- Install core packages: git, curl, tar, unzip, zsh (via `dnf`)
- If `create_user` is true:
  - Create user with `useradd -m -s /bin/zsh <target_user>`
  - Grant sudo access (add to wheel group or create sudoers entry)
  - Install SSH authorized key from the `.pub` counterpart of `identity_file`
- Set default shell to the configured shell
- Add GitHub SSH host key to `known_hosts` (enables `git clone` over SSH without interactive host verification)
- If running from Ghostty terminal (`$TERM_PROGRAM == ghostty`), export and install `xterm-ghostty` terminfo on the remote (via `infocmp -x xterm-ghostty` locally, then `tic -x -` on remote)
- Set timezone to match the local machine. Detect via `readlink /etc/localtime` (macOS/Linux) or `$TZ` env var. Add `timezone` variable to `group_vars/all.yml` and use `community.general.timezone` module in the base role.
- Create workspace directory

**tools role** (`roles/tools/tasks/main.yml`):
- For tools where `install` is `package` (or omitted): map tool descriptions to package names and install via `dnf install -y`
- For tools where `install` is `github-release`: download release binaries from GitHub using `repo`, `asset_pattern`, and `install_path` fields. Resolve `{arch}` to system architecture, `{goarch}` to Go convention (amd64/arm64), `{version}` to latest release tag via GitHub API. Use `curl -fsSL` with error checking and `|| true` fallback.

**zellij role** (`roles/zellij/tasks/main.yml`):
- Download Zellij release binary for target architecture
- Install to `/usr/local/bin/zellij`
- Verify with `zellij --version`

**claude role** (`roles/claude/tasks/main.yml`):
- Run `curl -fsSL https://claude.ai/install.sh | bash` as the target user
- Verify with `claude --version`

**cc_deck role** (`roles/cc_deck/tasks/main.yml`):
- Download cc-deck release binary from GitHub Releases (using the version from `cc_deck_version`)
- Install to `/usr/local/bin/cc-deck`
- Run `cc-deck config plugin install` as the target user (installs WASM plugin, layout files, controller config, Claude Code hooks)

**plugins role** (`roles/plugins/tasks/main.yml`):
- Install Claude Code plugins from the manifest `plugins` section using the `claude` CLI as the target user
- For each plugin entry:
  - `source: marketplace` -> run `claude plugins install <name>` (installs from official Anthropic marketplace)
  - `source: "github:<owner/repo>"` -> run `claude plugins marketplace add <owner/repo>` first, then `claude plugins install <name>@<marketplace>`
  - `source: directory` -> skip with a debug message ("plugin X is a local development plugin, skipping remote install")
- All commands run as the target user (`become_user: {{ target_user }}`)
- Requires Claude Code to already be installed (depends on `claude` role)
- Example Ansible task:
  ```yaml
  - name: Add custom marketplace
    ansible.builtin.shell: |
      export PATH="/home/{{ target_user }}/.local/bin:$PATH"
      claude plugins marketplace add {{ item.source | regex_replace('^github:', '') }}
    args:
      executable: /bin/bash
    become_user: "{{ target_user }}"
    when: item.source is match('^github:')
    loop: "{{ plugins }}"
    loop_control:
      label: "{{ item.name }}"

  - name: Install plugin
    ansible.builtin.shell: |
      export PATH="/home/{{ target_user }}/.local/bin:$PATH"
      claude plugins install {{ item.name }}
    args:
      executable: /bin/bash
    become_user: "{{ target_user }}"
    when: item.source != 'directory'
    loop: "{{ plugins }}"
    loop_control:
      label: "{{ item.name }}"
  ```

**shell_config role** (`roles/shell_config/tasks/main.yml`):
- If `git_config` is defined in `group_vars/all.yml`, set each key via `git config --global` as the target user:
  ```yaml
  - name: Set git config
    ansible.builtin.command:
      cmd: "git config --global {{ item.key }} '{{ item.value }}'"
    become_user: "{{ target_user }}"
    loop: "{{ git_config | dict2items }}"
    loop_control:
      label: "{{ item.key }}"
    when: git_config is defined
  ```
- Ensure `~/.local/bin` is on PATH by adding `export PATH="$HOME/.local/bin:$PATH"` to the shell RC file (required for `claude` which installs there)
- Ensure `TERM` is set: add `export TERM="${TERM:-xterm-256color}"` to the shell RC file (SSH sessions through Zellij may not propagate TERM)
- Deploy curated shell RC as `~/.zshrc.custom` (if `settings.shell_rc` is set), then add `[ -f ~/.zshrc.custom ] && source ~/.zshrc.custom` to `~/.zshrc` via `lineinfile` (idempotent, does not inline the content)
- Add credential sourcing snippet: `[ -f ~/.config/cc-deck/credentials.env ] && source ~/.config/cc-deck/credentials.env`
- Install starship config if present
- Guard starship init with interactive shell check: `[[ $- == *i* ]] && eval "$(starship init zsh)"` (prevents OSC escape sequences in non-interactive sessions like Ansible)
- **Zellij config.kdl**: If `settings.zellij_config` is set:
  1. Strip the cc-deck controller block locally (use `sed` with `delegate_to: localhost` and `become: false`) before copying
  2. Deploy the stripped config to the target
  3. Ensure `default_shell` is set in the deployed config
  The stripping is critical because the source config contains absolute paths from the host machine (e.g., `/Users/rhuss/...`) that are wrong on the target.
- **Zellij env block**: Add an `env` block to config.kdl with PATH (including `~/.local/bin` and `~/.cargo/bin`) and `TERM=xterm-256color`. This ensures all Zellij panes have correct PATH regardless of shell initialization.
- If `settings.remote_bg` is set, the curated shell RC (`.zshrc.custom`) should include `set-bg`/`reset-bg` functions and auto-set the background on interactive shell start with `trap reset-bg EXIT` for cleanup.
- **IMPORTANT ordering**: `cc-deck config plugin install` must run as the **last task** in the shell_config role, AFTER the Claude settings merge. This ensures hooks are not overwritten by the settings merge. The plugin install re-injects the controller block in config.kdl AND registers hooks in settings.json.
- For each entry in `settings.tool_configs`: copy the source file to `/home/<target_user>/.config/<target>` (using the `target` field from the manifest entry; fall back to `<tool>/<source-filename>` if `target` is not set). Create parent directories as needed. Example Ansible task:
  ```yaml
  - name: Deploy tool config
    ansible.builtin.copy:
      src: "{{ item.source }}"
      dest: "/home/{{ target_user }}/.config/{{ item.target }}"
      owner: "{{ target_user }}"
      mode: '0644'
    loop: "{{ tool_configs }}"
  ```

**mcp role** (`roles/mcp/tasks/main.yml`):
- For each MCP entry in the manifest, set up the appropriate config
- Copy MCP settings to the target user's config directory

### B4: Handle existing playbooks

If role task files already have content (not just the skeleton from init):

1. Show a diff of what would change in each modified role
2. Ask the user:
   - **"Use new roles"**: overwrite with generated content
   - **"Keep existing roles"**: use the existing files as-is
   - **"Stop"**: abort the build entirely

### B5: Run Ansible playbook

Ansible playbooks are in the `ssh/` subdirectory: `cd <setup-dir>/ssh && ansible-playbook -i inventory.ini site.yml`

```bash
cd <setup-dir>/ssh
ansible-playbook -i inventory.ini site.yml
```

**IMPORTANT**: Use a 10-minute timeout (600000ms) for the playbook execution.

### B6: Handle playbook failures (self-correction loop)

If the playbook fails:

1. **Read the Ansible error output** carefully
2. **Identify the failing task** (which role, which task)
3. **Diagnose the root cause**. Common issues:
   - Package not found: wrong package name for the distro
   - Permission denied: missing `become: true`
   - Download failure: wrong URL or architecture mapping
   - SSH key issues: wrong key path or permissions
4. **Fix the relevant role task file** with corrected tasks
5. **Retry the playbook** (Ansible idempotency ensures already-succeeded tasks are skipped)
6. **Repeat** until success or 3 fix attempts

After 3 failed attempts, stop and present the remaining error with your analysis.

### B7: Generate README

After a successful run, generate a `README.md` in the setup directory with standalone usage instructions:

```markdown
# SSH Provisioning - Standalone Usage

This directory contains Ansible playbooks generated by `cc-deck build`.

## Re-run the playbook

```bash
cd <setup-dir>/ssh
ansible-playbook -i inventory.ini site.yml
```

## Run specific roles

```bash
ansible-playbook -i inventory.ini site.yml --tags tools
```

## Target: <host>
```

### B8: Report results

On success, show:
- Host provisioned
- Roles applied (list all 7)
- Number of retry attempts (if any)
- Summary of role fixes made (if any)
- Note that playbooks can be re-run standalone

### B9: Register workspace

After SSH provisioning succeeds, automatically register the provisioned host as a workspace so `cc-deck attach` works immediately.

1. **Derive workspace name** from the SSH host:
   - Parse `targets.ssh.host` (e.g., `root@marovo` -> hostname `marovo`)
   - If the host is an IP address, use `ssh-<ip-with-dashes>` (e.g., `10.0.1.5` -> `ssh-10-0-1-5`)
   - If the host has no `@`, use the hostname directly

2. **Determine effective host** for the workspace:
   - If `create_user` is true and `targets.ssh.user` is set, the workspace host is `<user>@<hostname>` (developers SSH as the created user, not root)
   - Otherwise, use `targets.ssh.host` as-is

3. **Register the workspace** using `--update` for idempotency.

   **IMPORTANT**: Run from the **project root** (not from `.cc-deck/setup/`), otherwise cc-deck refuses to create a workspace inside a `.cc-deck/` directory.

   **IMPORTANT**: If the workspace path contains `~`, expand it to the absolute path on the remote (e.g., `~/workspace` -> `/home/<user>/workspace`). The `~` must NOT be passed to the shell because it would expand to the local home directory.

   ```bash
   cd <project-root> && \
   cc-deck ws new <name> --type ssh \
     --host <effective-host> \
     [--ssh-port <port>] \
     [--identity-file <identity_file>] \
     [--workspace <absolute-remote-path>] \
     [--repo <url> ...] \
     --update
   ```
   Only include optional flags whose values are set in the manifest (not commented out).
   For `--repo` flags: include one `--repo <url>` for each entry in the `sources` section that has a `url` field.

   Also write the cloned repos to `.cc-deck/workspace.yaml` so `cc-deck ws update --sync-repos` can sync them later.

4. **Report** the registration:
   ```
   Workspace "<name>" registered. Attach with:
     cc-deck attach <name>
   ```

---

## Section C: OpenShell Build

### C1: Read and validate

Read `build.yaml`. Extract from `targets.openshell`:
- `name` (required)
- `tag` (default: `latest`)
- `base` (default: `ghcr.io/nvidia/openshell-community/sandboxes/base:latest`)
- `registry` (optional, enables `--push`)
- `policy` (optional, explicit overrides)

### C2: Generate the Containerfile

Assemble the Containerfile by composing pre-rendered snippet files with generated sections.

**Snippet files**: Located in `openshell/snippets/`. These are pre-rendered Containerfile fragments with all paths and variables resolved for the sandbox user. Copy their content EXACTLY as-is into the Containerfile. Do NOT modify snippet content.

**Base image probing**: Before generating the variable sections, inspect the base image to determine:
1. **OS family and package manager**: Run `podman run --rm --entrypoint "" <base-image> cat /etc/os-release` to detect the distro. Use `apt-get` for Debian/Ubuntu, `dnf` for Fedora/RHEL, `apk` for Alpine. The default OpenShell base image is Ubuntu-based, so do NOT assume `dnf`.
2. **Pre-installed tools**: Run `podman run --rm --entrypoint "" <base-image> sh -c "which git node python3 npm curl rg 2>/dev/null"` to discover what is already available. Skip installing tools that are already present. Also check binary paths (e.g., python3 may be at `/sandbox/.venv/bin/python3` rather than `/usr/bin/python3`). Use the discovered paths when generating `policy.yaml` (step C4).

**Tool resolution**: Tools from the unified `tools` section, dispatched by `install` field:
- `install: package` (or omitted): resolved to the appropriate package manager command detected from the base image (e.g., `apt-get install -y` for Ubuntu, `dnf install -y` for Fedora)
- `install: github-release`: same as Section A (downloaded from GitHub Releases)

**Binary path tracking**: As you write install instructions, track which binary path each tool installs to. This mapping is used when generating `policy.yaml` (step C4):
- **Pre-installed tools**: Use the actual paths discovered during base image probing
- `install: package` installs to `/usr/bin/<binary>` (typical for apt/dnf)
- `install: github-release` uses the `install_path` field, or `/usr/local/bin/<name>`
- npm global packages go to `/usr/local/bin/<name>`
- Well-known defaults: Claude Code at `/sandbox/.local/bin/claude`, git at `/usr/bin/git`, node at `/usr/bin/node`

**Assembly order** (generate the Containerfile by following these steps in sequence):

1. Read `openshell/snippets/01-header.txt` and copy its content verbatim
2. *(snippet 02-user-setup.txt is empty for openshell, skip it)*
3. **GENERATE**: System packages layer (use probed package manager for tools not in base image)
4. **GENERATE**: Language-specific tools layer
5. **GENERATE**: GitHub release tools layer
6. Read `openshell/snippets/03-mandatory-stack.txt` and copy its content verbatim
7. **GENERATE**: Plugin install commands (same as Section A plugin handling)
8. Read `openshell/snippets/04-openshell-extras.txt` and copy its content verbatim
9. **GENERATE**: User configuration layers (see Settings handling below)
10. Read `openshell/snippets/05-shell-finalize.txt` and copy its content verbatim
11. Read `openshell/snippets/06-footer.txt` and copy its content verbatim

**CRITICAL ordering**: Steps 10 and 11 (shell-finalize and footer) MUST come AFTER step 9 (user config). The shell-finalize snippet appends starship init and Zellij auto-start to `.bashrc`/`.zshrc`. If user config layers come after, they can overwrite these additions.

**Settings handling**: Same rules as Section A, but all paths use `/sandbox/` instead of `/home/dev/`, and all `COPY --chown=dev:dev` become `COPY --chown=sandbox:sandbox`, and all `USER dev` become `USER sandbox`.

| Setting | OpenShell path |
|---|---|
| `settings.shell_rc` | `/sandbox/.<shell>rc` |
| `settings.zellij_config` | `/sandbox/.config/zellij/config.kdl` |
| `settings.claude_md` | `/sandbox/.claude/CLAUDE.md` |
| `settings.claude_settings` | `/sandbox/.claude/settings.json` |
| `settings.hooks` | `/sandbox/.claude/settings.json` |
| `settings.mcp_settings` | `/sandbox/.claude/settings.json` |
| `settings.cc_setup_mcp` | `/sandbox/.config/cc-setup/mcp.json` |
| `settings.tool_configs[]` | `/sandbox/.config/<target>` |
| `settings.git_config` | `git config --global` as `sandbox` |

### C3: Check for existing Containerfile

Same pattern as A3. If `openshell/Containerfile` already exists, show diff and ask the user whether to overwrite, keep, or stop.

### C3b: Prepare the build context

Same as Section A step A4, but using `openshell/context/` instead of `container/context/`:

1. Create `openshell/context/` directory
2. Determine the cc-deck version by running `cc-deck version -o json` and extracting the `version` field
3. Download Linux binaries for both architectures from GitHub Releases. Skip any architecture whose binary already exists in `openshell/context/`:
   ```bash
   mkdir -p openshell/context
   VERSION=$(cc-deck version -o json | jq -r '.version')
   for ARCH in amd64 arm64; do
     if [ ! -f "openshell/context/cc-deck-linux-${ARCH}" ]; then
       curl -fsSL "https://github.com/cc-deck/cc-deck/releases/download/v${VERSION}/cc-deck_${VERSION}_linux_${ARCH}.tar.gz" \
         | tar xz -C openshell/context/ cc-deck
       mv openshell/context/cc-deck "openshell/context/cc-deck-linux-${ARCH}"
     fi
   done
   ```
4. If the download fails (e.g., version is `dev` or the release does not exist), stop and tell the user:
   - For development builds: run `make cross-cli` from the cc-deck source repo, then copy the binaries to `openshell/context/`
   - For released versions: check that the version tag exists at `https://github.com/cc-deck/cc-deck/releases`

Copy settings files into `openshell/context/` using the same logic as Section A step A4 (shell_rc, zellij_config, claude_md, etc.), adapting destination paths for `/sandbox/` instead of `/home/dev/`.

### C4: Generate openshell/policy.yaml

Generate the OpenShell policy from `network.allowed_domains` with the default policy structure:

1. Start with the default policy (see `cc-deck/internal/openshell/default-policy.yaml` for reference):
   - `version: 1`
   - `filesystem_policy`: `include_workdir: true`, read_only paths (`/usr`, `/lib`, `/proc`, `/etc`, `/var/log`), read_write paths (`/sandbox`, `/tmp`, `/dev/null`, `/dev/urandom`, `/dev/random`, `/dev/pts`)
   - `landlock.compatibility: best_effort`
   - `process.run_as_user: sandbox`, `process.run_as_group: sandbox`

2. Auto-generate `network_policies` entries from `network.allowed_domains`:
   - For each domain, create an entry with slug key, name, endpoint (host:443)
   - Associate discovered binary paths with the appropriate entries based on which tools logically use which domains (inferred from the tool install instructions you wrote in C2)

   **MANDATORY policy rules (OpenShell 0.0.46+)**:

   a. **Binary glob for Claude Code**: Claude Code installs a versioned binary at `/sandbox/.local/share/claude/versions/<ver>`. The wrapper at `/usr/local/bin/claude` is NOT the process that makes network calls. EVERY policy entry that Claude Code uses MUST include this glob:
      ```yaml
      binaries:
        - { path: /usr/local/bin/claude }
        - { path: /sandbox/.local/bin/claude }
        - { path: "/sandbox/.local/share/claude/**" }
        - { path: /usr/bin/node }
      ```

   b. **`access` on REST endpoints**: Any endpoint with `protocol: rest` MUST include either `access: full` or explicit `rules`. Without one, the supervisor rejects the policy. Use `access: full` for API endpoints. Use `rules` with method/path patterns for scoped access (e.g., git-over-HTTPS). Endpoints without `protocol` do not need `access`.

   c. **Do NOT use deprecated fields**: `tls: terminate` and `enforcement: enforce` are deprecated in 0.0.46. Omit them.

   d. **Required Claude Code endpoints** (always include in `claude_code` policy):
      - `api.anthropic.com:443` (protocol: rest, access: full)
      - `statsig.anthropic.com:443`
      - `sentry.io:443`
      - `downloads.claude.ai:443` (auto-update)
      - `raw.githubusercontent.com:443` (skill/plugin fetching)

   e. **Vertex AI endpoints** (include when credentials contain `claude-vertex` or `vertex`):
      - `aiplatform.googleapis.com:443` (bare, used as CONNECT target)
      - `global-aiplatform.googleapis.com:443` and regional endpoints
      - `oauth2.googleapis.com:443` (token refresh)
      - `www.googleapis.com:443` (token info)
      - `accounts.google.com:443` (auth)
      All with the Claude Code binary glob from (a).

3. If `targets.openshell.policy` is defined, apply merge semantics:
   - If overrides has `filesystem_policy`, `landlock`, or `process`: replace the default section entirely
   - For `network_policies`: match explicit entries against auto-generated entries by endpoint host. Explicit entries replace matched auto-generated entries. Entries for hosts not in `allowed_domains` are additive. Auto-generated entries for non-overridden hosts are preserved.

Write the resulting policy YAML to `openshell/policy.yaml`.

### C5: Build the image

Detect the container runtime (prefer `podman`, fall back to `docker`).

**IMPORTANT**: Use a 10-minute timeout (600000ms) for the build command.

```bash
podman build -t <name>:<tag> -f openshell/Containerfile .
```

### C6: Handle build failures (self-correction loop)

Same pattern as A6. Read error output, identify failing step, diagnose root cause, fix Containerfile, retry. Up to 3 attempts.

### C7: Generate openshell/build.sh

After a successful build, generate a build script (same pattern as A7):

```bash
#!/bin/bash
# Rebuild the OpenShell sandbox image from the existing Containerfile.
# Generated by /cc-deck.build --target openshell - regenerate with: claude /cc-deck.build --target openshell
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

IMAGE_NAME="<image-name>"
IMAGE_TAG="<image-tag>"

# Detect container runtime
if command -v podman >/dev/null 2>&1; then
  RUNTIME=podman
elif command -v docker >/dev/null 2>&1; then
  RUNTIME=docker
else
  echo "Error: neither podman nor docker found" >&2
  exit 1
fi

echo "Building ${IMAGE_NAME}:${IMAGE_TAG}"
$RUNTIME build -t "${IMAGE_NAME}:${IMAGE_TAG}" -f Containerfile .
echo "Done: ${IMAGE_NAME}:${IMAGE_TAG}"
```

Make the script executable.

### C8: Push (if --push)

Same pattern as A8. Check `targets.openshell.registry` is set, tag with full registry reference, push.

### C9: Register workspace

After a successful build, register an OpenShell workspace so the user can immediately attach.

1. **Derive workspace name** from the image name (e.g., `cc-deck` -> `cc-deck-openshell`, or use the project directory name with `-openshell` suffix).

2. **Build the `ws new` command** with repos from the manifest:

   **IMPORTANT**: Run from the **project root** (not from `.cc-deck/setup/`).

   ```bash
   cd <project-root> && \
   cc-deck ws new <name> --type openshell \
     --image <image-name>:<tag> \
     [--repo <url> ...] \
     --update
   ```
   For `--repo` flags: include one `--repo <url>` for each entry in the `sources` section that has a `url` field.

3. **Report** the registration:
   ```
   Workspace "<name>" registered. Attach with:
     cc-deck attach <name>
   ```

### C10: Report results

On success, show:
- Image name:tag
- Image size
- Number of retry attempts (if any)
- Summary of Containerfile fixes made (if any)
- Note that `openshell/build.sh` was generated for CLI rebuilds
- If pushed: the full registry reference
- Policy summary: number of network_policies entries, whether explicit overrides were applied
- Workspace name and attach command

---

## Key Rules (all targets)

- Never modify `build.yaml` (the manifest is the source of truth)
- All generated files include a "GENERATED BY cc-deck.build" header
- The self-correction loop pattern is the same for all targets: run, read error, fix artifact, retry, up to 3 attempts
- **Container-specific**: Never omit the 3 mandatory layers. Always use `container/context/cc-deck-linux-${TARGETARCH}` as COPY source.
- **SSH-specific**: All roles must be idempotent. The playbook must be runnable standalone without cc-deck or Claude Code.
- **OpenShell-specific**: Always include the mandatory cc-deck/Zellij/cc-session layers (same as container target). Always embed `openshell/policy.yaml` at `/etc/openshell/policy.yaml`. Use `sandbox` user and `/sandbox` workdir. Final `chown -R sandbox:sandbox /sandbox` before `USER sandbox`.
- Combine related package install calls into a single task/RUN for efficiency
- Clean package caches after installs

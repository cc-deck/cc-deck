# Manual Test: Build Pipeline (018)

Walkthrough for verifying the container image build pipeline end-to-end.
Covers `cc-deck image init`, the AI-driven commands (`/cc-deck.capture`,
`/cc-deck.build`, `/cc-deck.push`), and the CLI verify/diff subcommands.

## Prerequisites

```bash
# Verify podman is installed and running
podman info --format '{{.Host.Security.Rootless}}'

# Verify cc-deck is installed and shows a release version (not "dev")
cc-deck version
# Expected: cc-deck version 0.8.0 (or a released version)
# If version is "dev", binary downloads will fail. Use 'make cross-cli'
# and copy binaries to build-context/ manually instead.
alias ccd=cc-deck

# Verify Claude Code is available (needed for AI commands)
claude --version

# Create a scratch project to test against
mkdir -p ~/cc-deck-build-test/myproject
cd ~/cc-deck-build-test/myproject
git init
cat > go.mod << 'GO'
module example.com/test-project

go 1.25
GO
echo 'package main; func main() {}' > main.go
```

---

## 1. Initialize a Build Directory (US1)

### 1a. Init in project directory (default, uses .cc-deck/image/)

```bash
cd ~/cc-deck-build-test/myproject
ccd image init
# Expected:
#   Build directory initialized: .cc-deck/image
#   Manifest:  .cc-deck/image/cc-deck-image.yaml
#   Commands:  .claude/commands/cc-deck.*.md

# Verify directory structure
ls .cc-deck/image/
# Expected: cc-deck-image.yaml  .gitignore

ls .claude/commands/cc-deck.*.md
# Expected: cc-deck.build.md  cc-deck.capture.md  cc-deck.push.md
```

### 1b. Verify manifest is valid YAML with examples

```bash
yq '.' .cc-deck/image/cc-deck-image.yaml
# Expected: valid YAML with commented examples for image, tools, sources,
#           plugins, mcp, github_tools, settings sections

# Check image name defaults to project directory name
yq '.image.name' .cc-deck/image/cc-deck-image.yaml
# Expected: myproject
```

### 1c. Init refuses to overwrite existing directory

```bash
ccd image init
# Expected: error "build directory already initialized: .cc-deck/image/cc-deck-image.yaml exists"
```

### 1d. Init with --force overwrites

```bash
ccd image init --force
# Expected: success, directory re-scaffolded
```

### 1e. Init with explicit directory (alternative)

```bash
cd ~/cc-deck-build-test
ccd image init custom-image-dir
# Expected: initializes custom-image-dir/ with manifest
ls custom-image-dir/cc-deck-image.yaml
# Expected: file exists
rm -rf custom-image-dir
```

---

## 2. Configure the Manifest with /cc-deck.capture (US2)

This step requires running Claude Code interactively.

### 2a. Launch Claude Code and run capture

```bash
cd ~/cc-deck-build-test/myproject
claude
# Inside Claude Code:
#   /cc-deck.capture .
```

**Expected capture flow:**

1. **Repository analysis**: Detects `go.mod`, reports Go >= 1.25
2. **Network domains**: Suggests `golang`, `github` domain groups
3. **Shell config**: Asks which shell (zsh/bash), analyzes `~/.zshrc`
4. **Zellij config**: Scans `~/.config/zellij/config.kdl`
5. **Claude config**: Scans `~/.claude/CLAUDE.md` and `settings.json`
6. **Plugins**: Lists discovered Claude Code plugins
7. **MCP servers**: Lists discovered MCP server configs
8. **Summary**: Shows all findings, asks to write to manifest

### 2b. Verify manifest was updated

```bash
# Check tools section
yq '.tools' .cc-deck/image/cc-deck-image.yaml
# Expected: contains "Go compiler >= 1.25" or similar

# Check sources section has provenance
yq '.sources' .cc-deck/image/cc-deck-image.yaml
# Expected: entry with path to myproject, detected_tools, detected_from

# Check network domains (if accepted)
yq '.network.allowed_domains' .cc-deck/image/cc-deck-image.yaml
# Expected: golang, github

# Check settings (if accepted)
yq '.settings' .cc-deck/image/cc-deck-image.yaml
# Expected: shell, zellij_config, claude_md entries (depending on selections)
```

### 2c. Re-run capture (should update, not duplicate)

```bash
# Inside Claude Code:
#   /cc-deck.capture .
# Expected: shows current selections, highlights changes
# Existing entries are updated, not duplicated
```

---

## 3. Populate Manifest Manually (alternative to capture)

If testing without Claude Code, populate the manifest by hand.

```bash
cat > .cc-deck/image/cc-deck-image.yaml << 'YAML'
version: "1"

image:
  name: myproject
  tag: latest
  base: ghcr.io/rhuss/cc-deck-base:latest

tools:
  - "Go compiler >= 1.25"

settings:
  shell: zsh
YAML
```

---

## 4. Generate Containerfile and Build with /cc-deck.build (US3 + US4)

### 4a. Run the combined build command

```bash
cd ~/cc-deck-build-test/myproject
claude
# Inside Claude Code:
#   /cc-deck.build
```

**Expected build flow:**

1. **Read manifest**: Validates `cc-deck-image.yaml` has `version` and `image.name`
2. **Generate Containerfile**: Resolves tools to install commands, includes mandatory layers
3. **Check existing Containerfile**: If exists, shows diff and asks what to do
4. **Prepare build context**: Downloads cc-deck Linux binaries (amd64 + arm64) from GitHub Releases matching the installed version
5. **Build image**: Runs `podman build --platform linux/arm64,linux/amd64`
6. **Self-correction**: If build fails, fixes Containerfile and retries (up to 3 times)
7. **Generate build.sh**: Creates a CLI rebuild script
8. **Report**: Shows image name, size, retry count

### 4b. Verify Containerfile was generated

```bash
head -5 .cc-deck/image/Containerfile
# Expected:
#   # GENERATED BY cc-deck.build - DO NOT EDIT MANUALLY
#   # Regenerate with: claude /cc-deck.build
#   FROM ghcr.io/rhuss/cc-deck-base:latest
#   ARG TARGETARCH
```

### 4c. Verify mandatory layers are present

```bash
# cc-deck self-install layer
grep 'cc-deck config plugin install' .cc-deck/image/Containerfile
# Expected: RUN ... cc-deck config plugin install --install-zellij --force --skip-backup

# Claude Code install layer
grep 'claude.ai/install.sh' .cc-deck/image/Containerfile
# Expected: RUN curl -fsSL https://claude.ai/install.sh | sh

# cc-session and cc-setup layer
grep 'cc-session' .cc-deck/image/Containerfile
# Expected: curl download for cc-session and cc-setup
```

### 4d. Verify build context (binary download)

```bash
ls .cc-deck/image/build-context/
# Expected: cc-deck-linux-amd64  cc-deck-linux-arm64
# (and possibly settings files: CLAUDE.md, zellij-config.kdl, etc.)

# Verify binaries match the installed cc-deck version
cc-deck version -o json | jq -r '.version'
# Expected: 0.8.0 (or current version)

# Verify the downloaded binaries are Linux ELF
file .cc-deck/image/build-context/cc-deck-linux-amd64
# Expected: ELF 64-bit LSB executable, x86-64
file .cc-deck/image/build-context/cc-deck-linux-arm64
# Expected: ELF 64-bit LSB executable, ARM aarch64
```

### 4d-err. Download fails for dev builds

```bash
# If cc-deck was built from source with version "dev":
cc-deck version
# Shows: cc-deck version dev (commit: ..., built: ...)

# /cc-deck.build should fail at Step 4 and instruct:
#   "Version is 'dev'. Run 'make cross-cli' from the cc-deck source repo,
#    then copy binaries to build-context/"
```

### 4e. Verify build.sh was generated

```bash
ls -l .cc-deck/image/build.sh
# Expected: executable script

head -10 .cc-deck/image/build.sh
# Expected: shebang, IMAGE_NAME, IMAGE_TAG, PLATFORMS, runtime detection

grep 'IMAGE_NAME=' .cc-deck/image/build.sh
# Expected: IMAGE_NAME="myproject"
```

### 4f. Verify the image was built

```bash
podman images myproject:latest --format '{{.Repository}}:{{.Tag}} {{.Size}}'
# Expected: myproject:latest <size>
```

### 4g. Rebuild using build.sh (no Claude Code needed)

```bash
cd .cc-deck/image
./build.sh
# Expected: builds successfully, reuses cached layers
cd ../..
```

---

## 5. Build without Containerfile (error case)

```bash
ccd image init error-test --force
rm -f error-test/Containerfile 2>/dev/null
# Inside Claude Code, /cc-deck.build should generate a new Containerfile
# The CLI `cc-deck build` (if it existed) would error without one
```

---

## 6. Build with Unreleased Version (error case)

```bash
# If cc-deck version is "dev" (built from source without version tag),
# the GitHub Release download will fail.
# /cc-deck.build should stop with a clear message:
#   "Version is 'dev' — no GitHub Release available.
#    Run 'make cross-cli' from the source repo, then copy binaries to build-context/"

# Workaround for development builds:
cd ~/cc-deck-build-test/myproject
mkdir -p .cc-deck/image/build-context
# (from cc-deck source repo)
# make cross-cli
# cp cc-deck/cc-deck-linux-* .cc-deck/image/build-context/
```

---

## 7. Push the Image with /cc-deck.push (US4)

### 7a. Push to registry

```bash
cd ~/cc-deck-build-test/myproject
claude
# Inside Claude Code:
#   /cc-deck.push
```

**Expected push flow:**

1. **Read manifest**: Extracts `image.name` and `image.tag`
2. **Verify image exists**: Checks `podman images`
3. **Push**: Runs `podman push <image-name>:<tag>`
4. **Report**: Shows pushed image reference

### 7b. Push without built image (error case)

```bash
# Remove the local image first
podman rmi myproject:latest 2>/dev/null
# Inside Claude Code:
#   /cc-deck.push
# Expected: error suggesting to run /cc-deck.build first
```

### 7c. Push without registry login (error case)

```bash
# If not logged in to the target registry:
# Expected: suggests 'podman login <registry>' and instructions
```

---

## 8. Add Plugins with /cc-deck.plugin (US5)

```bash
cd ~/cc-deck-build-test/myproject
claude
# Inside Claude Code:
#   /cc-deck.plugin
```

### 8a. Add a marketplace plugin

```bash
# When prompted, add "sdd" from marketplace
# Expected: manifest updated with plugin entry

yq '.plugins' .cc-deck/image/cc-deck-image.yaml
# Expected:
#   - name: sdd
#     source: marketplace
```

### 8b. Add a git-based plugin

```bash
# Inside Claude Code:
#   /cc-deck.plugin
# Add plugin from git URL, e.g., "git:https://github.com/org/my-plugin.git"

yq '.plugins[-1]' .cc-deck/image/cc-deck-image.yaml
# Expected:
#   name: my-plugin
#   source: "git:https://github.com/org/my-plugin.git"
```

---

## 9. Add MCP Servers with /cc-deck.mcp (US5)

### 9a. MCP server with image labels

```bash
# Inside Claude Code:
#   /cc-deck.mcp ghcr.io/modelcontextprotocol/github-mcp:latest

# Expected: reads cc-deck.mcp/* labels from image, auto-populates entry
yq '.mcp' .cc-deck/image/cc-deck-image.yaml
# Expected: entry with name, image, transport, port, auth
```

### 9b. MCP server without labels (interactive)

```bash
# Inside Claude Code:
#   /cc-deck.mcp some-unlabeled-image:latest
# Expected: asks for transport (sse/stdio), port, auth details interactively
```

---

## 10. Verify Built Image (US6)

```bash
# Rebuild image first if removed
cd ~/cc-deck-build-test/myproject

ccd image verify
# Expected:
#   Verifying image: myproject:latest
#
#   PASS  cc-deck version: v0.8.0
#   PASS  Claude Code available: 1.x.x
#   PASS  Go compiler: go version go1.25 ...
#
#   3 passed, 0 failed
```

### 10a. Verify with missing tool (expected failure)

```bash
# Add a tool to manifest that was not installed
yq -i '.tools += ["nonexistent-tool >= 99.0"]' .cc-deck/image/cc-deck-image.yaml
# Note: verify checks are heuristic (maps tool names to commands),
# so this may not produce a FAIL unless the tool name maps to a known check
```

---

## 11. Diff Manifest vs Containerfile (US6)

### 11a. Diff when in sync

```bash
ccd image diff
# Expected:
#   Tools:
#   Plugins:
#   MCP Servers:
#   GitHub Tools:
#   No differences detected. Manifest and Containerfile appear in sync.
```

### 11b. Diff after manifest change

```bash
# Add a new tool without regenerating the Containerfile
yq -i '.tools += ["Python >= 3.12"]' .cc-deck/image/cc-deck-image.yaml

ccd image diff
# Expected:
#   Tools:
#     + Python >= 3.12 (in manifest, not in Containerfile)
#   ...
#   Regenerate with: claude /cc-deck.containerfile
```

### 11c. Diff without Containerfile

```bash
ccd image init diff-test --force
ccd image diff diff-test
# Expected: error "no Containerfile found, run /cc-deck.containerfile first"
rm -rf diff-test
```

---

## 12. Settings Handling in Containerfile

These checks verify that settings from the manifest are correctly handled
in the generated Containerfile.

### 12a. Shell configuration (settings.shell_rc)

```bash
grep 'zshrc' .cc-deck/image/Containerfile
# Expected: COPY --chown=dev:dev ... and append to .zshrc
# (only if settings.shell_rc was set during capture)
```

### 12b. Zellij config (settings.zellij_config: current)

```bash
grep 'config.kdl' .cc-deck/image/Containerfile
# Expected: COPY --chown=dev:dev build-context/zellij-config.kdl ...
# (only if settings.zellij_config was set)
```

### 12c. Claude CLAUDE.md (settings.claude_md)

```bash
grep 'CLAUDE.md' .cc-deck/image/Containerfile
# Expected: COPY --chown=dev:dev build-context/CLAUDE.md /home/dev/.claude/CLAUDE.md
# (only if settings.claude_md was set)
```

### 12d. Claude settings merge (settings.claude_settings)

```bash
grep 'settings.json' .cc-deck/image/Containerfile
# Expected: COPY --chown=dev:dev build-context/settings.json /home/dev/.claude/settings.json
# Verify cc-deck hooks are preserved (merged, not overwritten)
```

---

## 13. Error Cases

### No container runtime

```bash
PATH=/usr/bin ccd image verify
# Expected: error about podman/docker not found
```

### Invalid manifest YAML

```bash
echo "invalid: yaml: [broken" > /tmp/bad-manifest/cc-deck-image.yaml 2>/dev/null
mkdir -p /tmp/bad-manifest
echo "invalid: yaml: [broken" > /tmp/bad-manifest/cc-deck-image.yaml
ccd image verify /tmp/bad-manifest
# Expected: YAML parse error with details
rm -rf /tmp/bad-manifest
```

### Missing required fields

```bash
mkdir -p /tmp/empty-manifest
echo "version: '1'" > /tmp/empty-manifest/cc-deck-image.yaml
ccd image verify /tmp/empty-manifest
# Expected: validation error about missing image.name
rm -rf /tmp/empty-manifest
```

---

## Cleanup

```bash
# Remove test images
podman rmi myproject:latest 2>/dev/null

# Remove test directories
rm -rf ~/cc-deck-build-test

# Unset alias
unalias ccd 2>/dev/null
```

---

## Verification Summary

| Test | US | FR | Result |
|------|----|----|--------|
| Init default .cc-deck/image/ | US1 | FR-007 | |
| Manifest has valid YAML | US1 | FR-001, FR-002 | |
| Commands extracted | US1 | FR-009 | |
| Init refuses overwrite | US1 | FR-008 | |
| Init with --force | US1 | FR-008 | |
| Init with explicit dir | US1 | FR-007 | |
| Capture detects tools | US2 | FR-015 | |
| Capture deduplicates | US2 | FR-017 | |
| Capture user review | US2 | FR-016 | |
| Capture network domains | US2 | FR-015 | |
| Capture re-run updates | US2 | FR-017 | |
| Manual manifest population | US3 | FR-003 | |
| Containerfile generated | US3 | FR-018, FR-019 | |
| Mandatory layers present | US3 | FR-020 | |
| Build context prepared | US4 | FR-010 | |
| build.sh generated | US4 | FR-010 | |
| Image built successfully | US4 | FR-010, FR-011 | |
| Rebuild with build.sh | US4 | FR-010 | |
| Push image | US4 | FR-012 | |
| Push without image (error) | US4 | FR-012 | |
| Push auth failure guidance | US4 | FR-012 | |
| Plugin add (marketplace) | US5 | FR-021 | |
| Plugin add (git) | US5 | FR-021 | |
| MCP add (with labels) | US5 | FR-022 | |
| MCP add (interactive) | US5 | FR-022 | |
| Verify passes | US6 | FR-013 | |
| Diff in sync | US6 | FR-014 | |
| Diff after change | US6 | FR-014 | |
| Diff without Containerfile | US6 | FR-014 | |
| Shell config in Containerfile | - | FR-005 | |
| Zellij config in Containerfile | - | FR-005 | |
| Claude config in Containerfile | - | FR-005 | |
| Settings merge (hooks preserved) | - | FR-005 | |
| No container runtime error | - | FR-011 | |
| Invalid manifest error | - | FR-010 | |
| Missing required fields error | - | FR-010 | |

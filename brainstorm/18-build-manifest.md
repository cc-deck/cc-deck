# 18: Build Manifest & Scaffold

## Problem

Users need to create customized container images that mirror their local Claude Code setup,
including project-specific tool dependencies, plugins, and MCP server configuration.
There is no tooling for this today. Image building was explicitly deferred in brainstorm 04
(session flavors) as a "future separate project." This is that project.

## Relationship to Existing Work

- **Brainstorm 04 (Flavors)**: Handles image **selection** at deploy time. A flavor references
  a pre-built image. This feature handles image **creation**.
- **Brainstorm 02 (K8s CLI)**: Defined the deployment model (StatefulSet, PVC, env vars).
  This feature produces the images that deployments consume.

## The Build Directory

`cc-deck build init <dir>` creates a self-contained build directory:

```
my-cc-image/
  cc-deck-build.yaml          # The manifest (source of truth)
  Containerfile                # Generated, never manually edited
  compose.yaml                # Generated for local MCP sidecars
  k8s/                        # Generated Kubernetes manifests
    kustomization.yaml
    statefulset.yaml
    mcp-sidecars.yaml
  .claude/
    commands/                  # AI-driven build commands
      cc-deck.extract.md
      cc-deck.plugin.md
      cc-deck.mcp.md
      cc-deck.containerfile.md
      cc-deck.publish.md
    scripts/                   # Helper scripts called by commands
      validate-manifest.sh
      list-plugins.sh
```

## Manifest Schema: `cc-deck-build.yaml`

```yaml
# cc-deck-build.yaml
version: 1

image:
  name: my-team/cc-deck-dev     # Required: image name
  tag: latest                    # Default tag
  base: ghcr.io/rhuss/cc-deck-base:latest  # Base image (default: cc-deck base)

# Free-form tool requirements, resolved by LLM during containerfile generation.
# Each entry is human-readable text evaluated during `cc-deck.containerfile`.
# Conflicts (e.g., two Go versions) are detected and resolved by the LLM.
tools:
  - "Go compiler >= 1.22"
  - "Python 3.12 with uv package manager"
  - "protobuf compiler (protoc) and buf CLI"
  - "ripgrep, fd-find, jq, yq"

# Repositories that were analyzed for tool discovery.
# NOT included in the image. Kept for provenance and re-extraction.
sources:
  - url: https://github.com/org/api-service
    ref: main
    path: /local/path/to/api-service    # Local checkout used during extract
    detected_tools:
      - "Go 1.23"
      - "protoc >= 25.0"
      - "buf CLI"
    detected_from:
      - go.mod
      - buf.yaml
      - .github/workflows/ci.yml
  - url: https://github.com/org/web-frontend
    ref: main
    path: /local/path/to/web-frontend
    detected_tools:
      - "Node.js 22"
      - "pnpm >= 9"
    detected_from:
      - package.json
      - .nvmrc

# Claude Code plugins to install in the image.
plugins:
  - name: cc-rosa
    source: marketplace
  - name: sdd
    source: marketplace
  - name: custom-plugin
    source: "git:https://github.com/org/custom-plugin.git"

# MCP servers as sidecar containers.
# These are NOT baked into the main image but generate compose/k8s manifests.
mcp:
  - name: github
    image: ghcr.io/modelcontextprotocol/github-mcp:latest
    transport: sse
    port: 8000
    auth:
      type: token
      env_vars:
        - GITHUB_TOKEN
    description: "GitHub repository management"
  - name: slack
    image: ghcr.io/org/slack-mcp:latest
    transport: sse
    port: 3001
    auth:
      type: basic
      env_vars:
        - SLACK_TOKEN
        - SLACK_TEAM_ID
    description: "Slack workspace integration"

# Additional tools downloaded from GitHub releases.
# Each entry specifies a repo and binary name. Multi-arch download is automatic.
github_tools:
  - repo: rhuss/cc-setup
    binary: cc-setup
  - repo: rhuss/cc-session
    binary: cc-session

# Optional: CC configuration to bake into image
settings:
  claude_md: ./project-claude.md    # Project-level CLAUDE.md
  hooks: ./hooks.json               # Claude Code hooks config
  zellij_config: current            # "current" = copy ~/.config/zellij/
                                    # "vanilla" = only cc-deck defaults
                                    # or a path to a custom directory
```

## Schema Principles

1. **Human-readable**: YAML, no binary formats. Tools are free-form text, not package manager commands.
2. **Declarative**: Describes desired state, not build steps. The Containerfile is derived.
3. **Provenance-aware**: Tracks which repos contributed which tools (for re-extraction).
4. **Separation of concerns**: Main image vs MCP sidecars are distinct sections.
5. **No credentials**: Auth section describes what env vars are needed, never contains actual values.

## CLI Commands

### `cc-deck build init <dir>`

Creates the build directory with:
- Empty `cc-deck-build.yaml` (with commented-out examples)
- `.claude/commands/` with all build commands
- `.claude/scripts/` with helper scripts
- `.gitignore` (ignoring generated files)

All commands and scripts are embedded in the `cc-deck` Go binary using `go:embed`
(same pattern as the plugin WASM and layout files).

### `cc-deck build <dir>`

Deterministic build step:
1. Validates `cc-deck-build.yaml` schema
2. Validates `Containerfile` exists (must run `cc-deck.containerfile` first)
3. Runs `podman build` (or `docker build`) with appropriate tags
4. Reports image size and layer breakdown

### `cc-deck push <dir> [--registry <url>]`

Pushes the built image:
1. Reads image name/tag from manifest
2. Runs `podman push` (or `docker push`)
3. Optionally tags with git SHA for traceability

### `cc-deck build verify <dir>`

Dry-run validation:
1. Builds the image
2. Starts a container from it
3. Checks: Claude Code starts, Zellij starts, tools are available
4. Reports pass/fail with details

### `cc-deck build diff <dir>`

Shows what changed since last build:
- New tools detected
- Plugin updates available
- MCP image updates available
- Useful for CI/CD pipelines

## Open Questions

- Should `cc-deck build init` auto-detect `podman` vs `docker` and configure accordingly?
  Or require the user to specify?
- Lock file (`cc-deck-build.lock`) for pinning exact tool/plugin/MCP versions?
  Useful for reproducibility but adds complexity. Consider for v2.
- Should the manifest support "profiles" (e.g., dev vs CI) or keep it single-purpose?

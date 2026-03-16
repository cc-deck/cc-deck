# 24: Multi-Agent Support

**Date**: 2026-03-16
**Status**: brainstorm
**Feature**: Support for multiple AI coding agents beyond Claude Code
**Inspired by**: paude project (Ben Browning)

## Problem

cc-deck is currently built exclusively around Claude Code. The hook system (`cc-deck hook`) parses Claude Code's specific output format for status detection. The container image pipeline assumes Claude Code installation. As the AI coding agent landscape expands (Cursor CLI, Gemini CLI, Codex CLI, etc.), cc-deck should support multiple agents to remain relevant and useful. The core session management, sidebar rendering, and deployment infrastructure are already agent-agnostic in design, but the integration points are tightly coupled to Claude Code.

## paude's Approach

The paude project (by Ben Browning) provides a reference implementation for multi-agent support in containerized environments. Key architectural elements:

### Agent Protocol

paude defines a standardized `Agent` interface with these responsibilities:

- **Configuration**: agent-specific config directory paths, process names, session identifiers
- **Installation**: per-agent install commands for container environments
- **Sandboxing**: isolation and security settings (yolo flags, permission modes)
- **Launch**: agent-specific invocation patterns and CLI arguments
- **Mounts**: config directories and file mappings for container volumes
- **Environment**: credential handling, auth tokens, API keys, environment variables

### AgentConfig Dataclass

A shared data structure captures agent-specific metadata:

- `process_name`: process identifier for monitoring (e.g., "claude", "cursor", "gemini")
- `session_name`: session identifier format for status tracking
- `config_dir`: agent configuration directory (e.g., ".claude", ".cursor", ".gemini")
- `yolo_flag`: permission bypass flag (e.g., "--dangerously-skip-permissions", "--yolo")
- `domain_aliases`: allowlist of agent-specific domains for network access
- `activity_files`: file paths for activity detection (alternative to process monitoring)

### Implementations

paude implements three agents as reference examples:

**Claude Agent**:
- Config dir: `.claude`
- Yolo flag: `--dangerously-skip-permissions`
- Domain aliases: `claude.ai`
- Auth: `ANTHROPIC_API_KEY` environment variable or OAuth passthrough

**Cursor Agent**:
- Config dir: `.cursor`
- Yolo flag: `--yolo`
- Domain aliases: `cursor.com`, `cursorapi.com`
- Auth: API key stored in config, passed via environment variable

**Gemini Agent**:
- Config dir: `.gemini`
- Yolo flag: `--yolo`
- Domain aliases: `googleapis.com`, `generativelanguage.googleapis.com`
- Auth: gcloud application default credentials (ADC) via `GOOGLE_APPLICATION_CREDENTIALS`

### Agent Registry

A factory pattern provides agent lookup and instantiation:

```python
def get_agent(name: str) -> Agent:
    """Return agent instance by name, raise error if unknown."""
    agents = {
        "claude": ClaudeAgent(),
        "cursor": CursorAgent(),
        "gemini": GeminiAgent(),
    }
    return agents[name]
```

### Credential Handling

paude distinguishes three credential categories per agent:

1. **Passthrough vars**: environment variables inherited from host (e.g., `ANTHROPIC_API_KEY`)
2. **Secret vars**: injected credentials from secrets management (e.g., Kubernetes Secrets, podman secrets)
3. **Config excludes**: config files not copied from host (e.g., API keys stored in local config)

### Domain Aliases

Each agent declares network domains required for operation. In containerized environments, network policies or allow-lists use these aliases to permit agent-specific traffic:

- Claude: `claude.ai`
- Cursor: `cursor.com`, `cursorapi.com`
- Gemini: `googleapis.com`, `generativelanguage.googleapis.com`

## Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Agent abstraction | Go interface in cc-deck CLI, trait-based detection in plugin | Clean separation, each agent implements a standard contract |
| First additional agent | Cursor CLI | Similar terminal UX to Claude Code, growing adoption, straightforward integration |
| Status detection | Agent-specific hook implementations | Each agent has different output formats; cannot use a single parser |
| Plugin changes | Minimal, keep sidebar agent-agnostic | Smart attend and rendering already work with generic status values |
| Image pipeline | Per-agent install commands in cc-deck-build.yaml | Manifest already supports custom install steps |
| Default agent | Claude Code | Backwards compatibility, primary use case |

## Agent Interface Design

Define a Go interface mirroring paude's Agent protocol:

```go
// pkg/agent/agent.go

package agent

type Agent interface {
    // Identity
    Name() string              // "claude", "cursor", "gemini"
    ProcessName() string       // Process name for monitoring

    // Configuration
    ConfigDir() string         // Local config directory (e.g., ".claude")
    YoloFlag() string          // Permission bypass flag

    // Network
    DomainAliases() []string   // Allowed domains for network policies

    // Installation
    InstallCommand() string    // Container install command

    // Integration
    HookParser() HookParser    // Agent-specific status parser

    // Credentials
    CredentialVars() []string  // Required environment variables
}

// HookParser extracts status from agent output
type HookParser interface {
    ParseStatus(output string) (Status, error)
}

// Status represents current agent state (matches plugin enum)
type Status int

const (
    StatusIdle Status = iota
    StatusWorking
    StatusPermission
    StatusDone
    StatusNotification
    StatusPaused
)
```

## Agent Registry

Centralized agent registry with factory pattern:

```go
// pkg/agent/registry.go

package agent

import "fmt"

var registry = map[string]Agent{
    "claude": &ClaudeAgent{},
    "cursor": &CursorAgent{},
    "gemini": &GeminiAgent{},
}

func GetAgent(name string) (Agent, error) {
    agent, ok := registry[name]
    if !ok {
        return nil, fmt.Errorf("unknown agent: %s", name)
    }
    return agent, nil
}

func ListAgents() []string {
    names := make([]string, 0, len(registry))
    for name := range registry {
        names = append(names, name)
    }
    return names
}
```

## Adaptation: Plugin (All Deployment Targets)

The Rust plugin in `cc-zellij-plugin/` requires minimal changes to support multiple agents. The current architecture is already agent-agnostic in rendering and state management.

### Current Coupling Points

The plugin hook integration (`pipe_handler.rs`) expects Claude Code-specific status messages from `cc-deck hook`. This is the primary coupling point.

### Proposed Changes

1. **Agent-agnostic activity detection**: Instead of parsing agent-specific output, use multiple detection strategies:
   - **Process monitoring**: detect if agent process is running and consuming CPU
   - **File watchers**: monitor activity files that agents write to (e.g., `.cursor/activity.json`)
   - **Generic pipe protocol**: agent-specific hook binary sends standardized JSON messages

2. **Hook message format**: Standardize the pipe message protocol across all agents:
   ```json
   {
     "type": "cc-deck:hook",
     "pane_id": "12345",
     "status": "Working",
     "session_name": "project-main",
     "agent": "cursor"
   }
   ```

3. **Sidebar enhancements**: Add optional agent indicator per session:
   - Small icon or letter prefix (e.g., `[C]` for Claude, `[K]` for Cursor)
   - Color-coded status bar per agent
   - Configurable via `show_agent_indicator` in plugin config

4. **No changes needed**: Smart attend algorithm, navigation mode, and core rendering remain unchanged. They operate on generic status values (Working, Idle, Permission, Done) regardless of agent.

## Adaptation: Podman

Container deployment gains per-agent image variants and credential handling.

### Per-Agent Images

Each agent gets its own base image layer:

```
quay.io/cc-deck/cc-deck-base:latest           # Common tools, shell, Zellij
quay.io/cc-deck/cc-deck-claude:latest         # + Claude Code installation
quay.io/cc-deck/cc-deck-cursor:latest         # + Cursor CLI installation
quay.io/cc-deck/cc-deck-gemini:latest         # + Gemini CLI installation
```

Shared infrastructure (Zellij, shell, development tools) remains identical across all images. Only the agent installation layer differs.

### Credential Injection

Podman secrets and environment variables adapt per agent:

**Claude Code**:
```bash
podman secret create anthropic-key <(echo "$ANTHROPIC_API_KEY")
podman run --secret anthropic-key -e ANTHROPIC_API_KEY=/run/secrets/anthropic-key ...
```

**Cursor**:
```bash
podman secret create cursor-key <(echo "$CURSOR_API_KEY")
podman run --secret cursor-key -e CURSOR_API_KEY=/run/secrets/cursor-key ...
```

**Gemini**:
```bash
podman secret create gcloud-adc ~/.config/gcloud/application_default_credentials.json
podman run --secret gcloud-adc -e GOOGLE_APPLICATION_CREDENTIALS=/run/secrets/gcloud-adc ...
```

### compose.yaml Generation

`cc-deck deploy --compose` gains an `--agent` flag:

```bash
cc-deck deploy --compose <build-dir> --agent cursor --output ./deploy
```

Generated `compose.yaml` references the agent-specific image:

```yaml
services:
  cc-deck:
    image: quay.io/cc-deck/cc-deck-cursor:latest
    env_file:
      - .env
    environment:
      - CURSOR_API_KEY
    volumes:
      - cc-deck-data:/home/dev
```

Generated `.env.example` includes agent-specific variables:

```
# .env.example for Cursor
CURSOR_API_KEY=
```

## Adaptation: Kubernetes and OpenShift

Kubernetes deployment manifests adapt to agent selection via StatefulSet environment variables and Secrets.

### Per-Agent StatefulSets

`cc-deck deploy --k8s` gains an `--agent` flag:

```bash
cc-deck deploy --k8s <build-dir> --agent gemini --namespace ai-dev --output ./k8s
```

Generated StatefulSet references agent-specific image and secrets:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: cc-deck
spec:
  template:
    spec:
      containers:
        - name: cc-deck
          image: quay.io/cc-deck/cc-deck-gemini:latest
          env:
            - name: GOOGLE_APPLICATION_CREDENTIALS
              value: /var/secrets/gcloud/adc.json
          volumeMounts:
            - name: gcloud-adc
              mountPath: /var/secrets/gcloud
              readOnly: true
      volumes:
        - name: gcloud-adc
          secret:
            secretName: gemini-gcloud-adc
```

### Secret Mappings

Each agent requires different Kubernetes Secrets:

**Claude Code**:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: claude-api-key
stringData:
  api-key: "sk-ant-..."
```

**Cursor**:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cursor-api-key
stringData:
  api-key: "cursor_..."
```

**Gemini**:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gemini-gcloud-adc
data:
  adc.json: <base64-encoded credentials>
```

### Profile Configuration

User profiles gain an agent field:

```yaml
# ~/.config/cc-deck/config.yaml
profiles:
  default:
    agent: claude
    kubeconfig: ~/.kube/config
    namespace: cc-deck

  cursor-dev:
    agent: cursor
    kubeconfig: ~/.kube/home-config
    namespace: ai-dev
```

CLI commands respect profile agent setting:

```bash
cc-deck profile set cursor-dev --agent cursor
cc-deck deploy --k8s <build-dir> --profile cursor-dev
```

## Container Image Pipeline

The `cc-deck-build.yaml` manifest gains an `agent` field to specify which agent to install.

### Manifest Changes

```yaml
# cc-deck-build.yaml
metadata:
  name: my-cursor-image
  version: 1.0.0
  agent: cursor              # NEW: agent selector

base_image:
  registry: quay.io/cc-deck
  name: cc-deck-base
  tag: latest

agent_install:
  method: script             # NEW: per-agent installation
  script: |
    curl -fsSL https://cursor.sh/install.sh | sh
    cursor --version

credentials:
  required:
    - CURSOR_API_KEY         # NEW: agent-specific env vars
  optional:
    - CURSOR_WORKSPACE_ID
```

### AI Command Adaptation

Claude Code commands adapt based on agent field:

**`/cc-deck.extract`**: Recognizes agent-specific binaries and config directories
- Claude Code: looks for `claude` binary, `.claude` config
- Cursor: looks for `cursor` binary, `.cursor` config
- Gemini: looks for `gemini` binary, `.gemini` config

**`/cc-deck.settings`**: Generates agent-specific Containerfile snippets
- Claude: native installer invocation
- Cursor: Cursor installer script
- Gemini: gcloud SDK + Gemini CLI installation

**`/cc-deck.build`**: Self-corrects agent-specific installation errors
- Retries up to 3 times with Containerfile fixes
- Agent-specific error detection (e.g., missing API key, auth failure)

**`/cc-deck.push`**: Tags images with agent suffix
- `quay.io/myteam/cc-deck-cursor:1.0.0`
- `quay.io/myteam/cc-deck-gemini:1.0.0`

### Multi-Agent Images

Support installing multiple agents in a single image:

```yaml
metadata:
  agent: multi               # Install all registered agents

agent_install:
  agents:
    - claude
    - cursor
    - gemini

  default: claude            # Default agent for launch
```

Runtime agent selection via environment variable:

```bash
podman run -e CC_DECK_AGENT=cursor quay.io/myteam/cc-deck-multi:latest
```

## Open Questions

- Should cc-deck support running different agents in different tabs of the same Zellij session?
  - **Implication**: Would require per-tab agent tracking in plugin state, per-tab hook parsers
  - **Use case**: Compare agent responses side-by-side, test multi-agent workflows
  - **Complexity**: Medium (plugin state changes, hook routing)

- How to handle agents that do not have a CLI mode (IDE-only agents)?
  - **Examples**: GitHub Copilot (VS Code only), Amazon CodeWhisperer (IDE plugins)
  - **Options**: Skip IDE-only agents, or support headless IDE servers (e.g., code-server)
  - **Recommendation**: Focus on terminal-native agents for v1

- Should the hook protocol be standardized across agents, or should each agent have its own hook binary?
  - **Standardized protocol**: Single hook binary, agent-specific parsing logic
  - **Per-agent binaries**: `cc-deck-hook-claude`, `cc-deck-hook-cursor`, `cc-deck-hook-gemini`
  - **Trade-off**: Standardized = simpler deployment, Per-agent = easier to extend
  - **Recommendation**: Standardized protocol with agent field in messages

- Priority: is Cursor CLI mature enough for production container use?
  - **Current state**: Cursor CLI is in active development, API may change
  - **Risk**: Breaking changes in Cursor updates could require cc-deck adaptations
  - **Mitigation**: Pin Cursor version in Containerfile, document known-good versions
  - **Recommendation**: Mark Cursor support as experimental in v1

- How to detect agent capabilities (MCP support, tool use, etc.) for feature gating?
  - **Approach**: Agent interface gains `Capabilities()` method returning feature flags
  - **Use cases**: Disable MCP sidebar if agent lacks MCP support, hide tools panel if no tool use
  - **Example**: `type Capabilities struct { MCP bool; Tools bool; Streaming bool }`

- Should `cc-deck image` commands auto-detect which agent to configure based on installed binaries?
  - **Scenario**: User runs `cc-deck image init` in environment with Cursor already installed
  - **Auto-detection**: Scan for agent binaries, default to detected agent in manifest
  - **Trade-off**: Convenience vs. explicit configuration
  - **Recommendation**: Auto-detect and suggest, but require explicit confirmation

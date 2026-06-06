# Brainstorm: Multi-Agent Support for cc-deck

**Date:** 2026-03-19 (updated 2026-06-06)
**Status:** active
**Trigger:** Competitive analysis of [cmux](https://github.com/manaflow-ai/cmux) and its agent-agnostic notification approach
**Updated:** Comparative analysis with [lince](https://github.com/RisorseArtificiali/lince), a Zellij-based multi-agent dashboard with sandboxing

## Problem Statement

cc-deck currently only works with Claude Code via its hooks system. Other AI coding agents (OpenAI Codex CLI, Google Gemini CLI, Cursor CLI) have similar lifecycle events and hook mechanisms. Supporting multiple agents would significantly broaden cc-deck's audience and strengthen its position against macOS-only competitors like cmux.

## Lessons from lince (April 2026 Analysis)

[lince](https://github.com/RisorseArtificiali/lince) is a parallel effort that launched the same week as cc-deck (2026-03-02). It provides a Zellij dashboard for spawning and monitoring multiple AI agents (Claude, Codex, Gemini, OpenCode, Aider) with bubblewrap sandboxing. Comparing implementations reveals patterns we should adopt, validate, or deliberately skip.

### What lince validates in our approach

- **Hook-based status tracking is correct.** lince also uses Zellij pipes for status messages. Their dual-pipe design (`claude-status` for native hooks, `lince-status` for wrapped agents) confirms pipes are the right transport.
- **Normalized event format works.** Our `HookPayload` is cleaner than their split-pipe approach, but both projects converge on the same idea: normalize agent events into a common format before the plugin consumes them.
- **Go interface for behavioral logic.** lince's pure-config approach (TOML) cannot express hook installation, credential detection, or agent installation. Our Go `Agent` interface is more principled for these concerns.

### Patterns to adopt from lince

**1. Agent wrapper for hookless agents.** lince ships `lince-agent-wrapper`, a thin shell script that wraps any CLI command and sends start/stop events via Zellij pipe. This means any tool becomes a "managed session" in the sidebar with minimal status. We should build an equivalent `cc-deck-agent-wrapper` that emits our normalized `HookPayload` format. This is the fastest path to multi-agent MVP.

**2. Config-driven display properties.** lince defines per-agent display metadata in TOML (label, color, pipe name) separate from behavioral logic. Our hybrid approach should be: define display properties (short label, color, indicator character) in config, keep behavioral logic (hook installation, event translation, credential transport) in Go agent adapters.

**3. Event mapping tables per agent type.** lince lets each agent type define custom `event_map` entries that translate agent-specific event names to status states. This avoids hardcoding agent event vocabularies into the plugin. We should add an `EventMap map[string]string` field to our agent config, with Go adapters providing defaults that users can override.

**4. Pane title matching as fallback identification.** lince matches panes to agents using `pane_title_pattern` per agent type, not just hook-reported session IDs. Belt and suspenders. Worth adding to our reconciliation logic.

**5. Graceful degradation by hook richness.** Agents with native hooks (Claude) get full status (Working, Permission, tool names, subagent counts). Agents without hooks (wrapped with the agent wrapper) get only start/stop. The sidebar still works, just shows less detail. This two-tier approach avoids blocking multi-agent support on every agent having rich hooks.

### Patterns we should skip

**Bubblewrap sandboxing.** lince uses Linux namespaces (bubblewrap) and macOS Seatbelt (nono) to isolate agent processes. This is clever for local execution, but our Podman containers already provide stronger isolation (full container boundary, network namespace, cgroup limits). Adding bubblewrap would duplicate what Podman gives us and wouldn't help SSH or K8s environments. Skip.

**Data-only agent definitions.** lince defines agents entirely in TOML config. This works for simple spawning but breaks down for hook installation, credential detection, and install scripts. Our Go interface approach is better for behavioral concerns.

### Patterns worth exploring independently

**Credential proxy.** lince's `agent-sandbox` includes a localhost HTTP proxy that intercepts API calls and injects credentials, so API keys never enter the agent's environment. This would strengthen our container environments where we currently pass `ANTHROPIC_API_KEY` as an env var. Worth considering as a security hardening feature for our credential transport design.

**Git-push restriction.** lince blocks `git push` three ways: a wrapper script in `$PATH`, sanitized `.gitconfig` (no credential helpers), and no SSH keys in the sandbox. We could offer a simpler version: a `restrict-push: true` flag on workspace definitions that installs a git wrapper blocking push operations.

### Comparative stats (as of April 2026)

| Metric | cc-deck | lince |
|---|---|---|
| Created | 2026-03-02 | 2026-03-02 |
| Commits | 475 | 76 |
| Contributors | 1 | 3 |
| LOC (total) | ~36,000 | ~7,650 |
| LOC (Rust plugin) | 13,314 | 3,427 |
| Test functions | 429+ | 0 |
| CI pipelines | 4 | 0 |
| Releases | 9 | 0 |
| Agent support | Claude Code | Claude, Codex, Gemini, OpenCode, Aider |
| Sandboxing | Podman containers | bubblewrap / nono |
| Environment types | 6 (local, compose, SSH, K8s deploy, K8s sandbox, container) | 1 (local only) |

## Design Principle: Agent as an Interface

An "agent" in cc-deck should be a well-defined interface, not hardcoded Claude Code logic. This interface governs three concerns:

1. **Hook integration**: how to install hooks into the agent's configuration, how to translate the agent's events into cc-deck's normalized state model
2. **Installation**: how to install the agent binary itself (relevant for container images and remote environments)
3. **Credential transport**: how to securely provide API keys and auth tokens to the agent at runtime

This interface must work identically across all workspace types (local, Podman container, Kubernetes, SSH). The workspace abstraction is a separate concern. This document focuses on the agent interface itself.

## Agent Hook Landscape

### Claude Code (current, fully supported)

Hook events: `SessionStart`, `PreToolUse`, `PostToolUse`, `UserPromptSubmit`, `PermissionRequest`, `Notification`, `Stop`, `SubagentStop`, `SessionEnd`

Protocol: JSON on stdin to shell command, configured in `~/.claude/settings.json`

Config location: `~/.claude/settings.json`

Installation: `curl -fsSL https://claude.ai/install.sh | sh` (native installer, bundles own Node.js)

Credentials: `ANTHROPIC_API_KEY` env var, or OAuth via `~/.claude/.credentials.json`

State mapping already implemented in `cc-zellij-plugin/src/pipe_handler.rs`.

### OpenAI Codex CLI

Hook events: `sessionStart`, `userPromptSubmit`, `stop`

Thread status notifications with explicit flags: `waitingOnApproval`, `waitingOnUserInput`, `active`, `idle`, `systemError`

Protocol: JSON on stdin to shell command, configured in project/user settings. Hook types: `command`, `prompt`, `agent`. Execution: `sync` or `async`.

Config location: TBD (project or user settings file)

Installation: `npm install -g @openai/codex` or similar

Credentials: `OPENAI_API_KEY` env var

Key insight: Codex's `ThreadStatus` model maps cleanly to cc-deck's activity states:
- `active` (no flags) -> `Working`
- `active` + `waitingOnApproval` -> `Waiting(Permission)`
- `active` + `waitingOnUserInput` -> `Waiting(Notification)`
- `idle` -> `Idle`
- `notLoaded` -> `Init`

### Google Gemini CLI

Hook events (11 total): `SessionStart`, `SessionEnd`, `BeforeAgent`, `AfterAgent`, `BeforeModel`, `AfterModel`, `BeforeToolSelection`, `BeforeTool`, `AfterTool`, `PreCompress`, `Notification`

Protocol: JSON on stdin, JSON on stdout (hooks can modify behavior). Configured in `settings.json` (project, user, or system level). Environment vars: `GEMINI_PROJECT_DIR`, `GEMINI_SESSION_ID`, `GEMINI_CWD`.

Config location: `~/.gemini/settings.json` (user level)

Installation: `npm install -g @anthropic-ai/gemini-cli` or similar

Credentials: `GEMINI_API_KEY` env var, or Google Cloud ADC

Universal input fields: `session_id`, `transcript_path`, `cwd`, `hook_event_name`, `timestamp`.

Key insight: Gemini's event set is richer than Claude Code's. The `Notification` event carries `notification_type`, `message`, and `details`, which could drive more informative sidebar displays.

Proposed state mapping:
- `SessionStart` -> `Init`
- `BeforeAgent`, `BeforeTool`, `BeforeModel` -> `Working`
- `AfterAgent` -> `Done`
- `Notification` (type: permission) -> `Waiting(Permission)`
- `Notification` (type: other) -> `Waiting(Notification)`
- `SessionEnd` -> remove session

### Cursor CLI

Headless mode (`--print`) with stream-JSON output. No documented hooks system (as of March 2026, hooks docs returned 404).

Stream-JSON events include: system events, assistant messages, tool calls (with paths), result events.

Key limitation: No hook mechanism means cc-deck would need to parse stdout stream, which is fragile and couples tightly to Cursor's output format. Not recommended for initial implementation.

Alternative: Monitor the stream-JSON output via a wrapper script that translates events to `cc-deck hook` calls. This is a community contribution opportunity rather than first-party support.

## Why Not OSC Escape Sequences

An alternative approach would be to monitor terminal escape sequences (OSC 9, OSC 99, OSC 777) instead of requiring agent-specific hooks. This was investigated and ruled out for two reasons:

1. **Zellij does not support OSC notification sequences.** OSC 99 support is an open issue ([zellij#3451](https://github.com/zellij-org/zellij/issues/3451)) since June 2024 with no resolution. OSC 9 and OSC 777 are similarly unsupported.

2. **Zellij's plugin API cannot observe pane output.** Even if Zellij added OSC passthrough, the `zellij-tile 0.43` plugin API provides no event for reading raw terminal data from other panes. A plugin can only observe its own input. There is no `PaneOutput` or `TerminalData` event.

These are structural limitations that would require Zellij upstream changes. The hook-based approach, where the Go CLI translates agent events into Zellij pipe messages, remains the correct architecture. If Zellij adds an `OscNotification` plugin event in the future, this decision can be revisited.

## Architecture: Unified Adapter Protocol

Define a normalized intermediate format that any agent adapter must produce:

```json
{
  "version": 1,
  "agent": "codex",
  "event": "Working|Permission|Notification|Done|AgentDone|Init|Idle|End",
  "session_id": "optional-id",
  "pane_id": 42,
  "cwd": "/path/to/project",
  "tool_name": "optional",
  "message": "optional notification text",
  "metadata": {}
}
```

Ship built-in adapters for Claude Code, Codex, and Gemini. Allow external adapters via `cc-deck hook --raw` that accepts the normalized format directly.

The Rust plugin stays unchanged. It already consumes a normalized `HookPayload` via the `cc-deck:hook` pipe message. The adapter logic lives entirely in the Go CLI.

### Agent Wrapper for Hookless Agents

Inspired by lince's `lince-agent-wrapper`, ship a `cc-deck-agent-wrapper` script that wraps any CLI command and emits start/stop events in our normalized format:

```bash
#!/bin/sh
# cc-deck-agent-wrapper: minimal status for any CLI agent
AGENT_ID="${CC_DECK_AGENT_ID:-$(basename "$1")-$$}"
PANE_ID="${ZELLIJ_PANE_ID:-0}"

# Emit start event
echo '{"version":1,"agent":"'$1'","event":"Init","session_id":"'$AGENT_ID'","pane_id":'$PANE_ID'}' \
  | cc-deck hook --raw

# Run the wrapped command
"$@"
EXIT_CODE=$?

# Emit stop event
echo '{"version":1,"agent":"'$1'","event":"End","session_id":"'$AGENT_ID'","pane_id":'$PANE_ID'}' \
  | cc-deck hook --raw

exit $EXIT_CODE
```

This gives any agent basic sidebar presence (Init, then End) without requiring the agent to support hooks. Agents with native hooks (Claude, Codex, Gemini) bypass the wrapper and send rich events directly.

### Hybrid Config + Code Architecture

Following analysis of lince's pure-config approach, adopt a hybrid model:

**Config-driven (user-customizable):**
- Display properties: `short_label`, `color`, `indicator` character
- Event mapping overrides: custom event name to status translations
- Pane title matching pattern for identification
- Credential env var names

**Code-driven (Go interface):**
- Hook installation/uninstallation logic
- Event translation with semantic understanding
- Agent binary detection and config file discovery
- Install scripts for container images
- Credential file path resolution

```yaml
# Example: ~/.config/cc-deck/agents.yaml
agents:
  claude:
    short_label: "C"
    color: blue
    # event_map not needed: Go adapter handles natively
  codex:
    short_label: "X"
    color: cyan
    event_map:
      active: Working
      waitingOnApproval: Permission
      waitingOnUserInput: Notification
      idle: Idle
  custom-agent:
    short_label: "?"
    color: yellow
    wrapper: true  # Use cc-deck-agent-wrapper
    command: ["my-agent", "--headless"]
    pane_title_pattern: "my-agent"
```

This lets users add completely custom agents via config (with wrapper-level status), while built-in agents get full behavioral support via Go adapters.

## The Agent Interface

Each supported agent implements a Go interface. The interface focuses on behavioral concerns that cannot be expressed in config alone:

```go
type Agent interface {
    // Identity (defaults can be overridden in agents.yaml)
    Name() string           // "claude", "codex", "gemini"
    DisplayName() string    // "Claude Code", "OpenAI Codex", "Gemini CLI"
    Indicator() string      // "C", "X", "G" (for sidebar display)

    // Detection
    IsInstalled() bool      // check if agent binary exists
    DetectConfig() string   // find agent's config file path

    // Hook lifecycle
    InstallHooks(paneIDExpr string) error   // write hooks to agent's config
    UninstallHooks() error                  // remove cc-deck hooks
    HooksInstalled() bool                   // check if hooks are already present
    HasNativeHooks() bool                   // true if agent supports rich events

    // Event translation
    TranslateEvent(stdin []byte) (*HookPayload, error)  // agent JSON -> normalized
    DefaultEventMap() map[string]string                  // fallback event mappings

    // Image build (for container environments)
    InstallScript() string          // shell commands to install the agent binary
    CredentialEnvVars() []string    // env vars needed at runtime (e.g., ANTHROPIC_API_KEY)
    ConfigPaths() []string          // paths to copy/mount for configuration

    // Pane identification (belt and suspenders with hook-based detection)
    PaneTitlePattern() string       // match pane titles for this agent type
}
```

Agents without a Go adapter (custom agents defined only in `agents.yaml`) use `cc-deck-agent-wrapper` and get wrapper-level status (Init/End only). The wrapper is the default, native hooks are the upgrade path.

### Agent Registry

Agents register themselves in a central registry:

```go
var agents = map[string]Agent{
    "claude": &ClaudeAgent{},
    "codex":  &CodexAgent{},
    "gemini": &GeminiAgent{},
}
```

All CLI commands that interact with agents use this registry. Adding a new agent means implementing the interface and registering it.

## Hook Installation

### CLI Commands

```bash
# Install plugin + detect and hook all installed agents
cc-deck plugin install

# Install plugin + hook specific agents only
cc-deck plugin install --agents claude,codex

# Manage hooks separately from plugin install
cc-deck hooks install              # detect + install all
cc-deck hooks install --agents codex,gemini
cc-deck hooks uninstall            # remove all
cc-deck hooks uninstall --agents codex
cc-deck hooks status               # show which agents have hooks
```

### Auto-Detection

`cc-deck plugin install` (without `--agents`) should:
1. Scan for installed agents by checking binary existence and config file presence
2. Report what it found: "Detected: Claude Code, Codex CLI"
3. Install hooks for all detected agents
4. On subsequent runs, detect newly installed agents and update hooks for all

### Hook Update on `cc-deck plugin install`

When re-running `cc-deck plugin install` (e.g., after upgrading cc-deck), all previously installed hooks should be updated to the current version. The install command should:
1. Check which agents have cc-deck hooks installed (`HooksInstalled()`)
2. Detect any newly installed agents
3. Reinstall/update hooks for the union of both sets
4. Report changes: "Updated hooks for Claude Code, added hooks for Codex"

## Image Build Integration

The `build.yaml` manifest (source of truth for image builds) needs an `agents` section:

```yaml
# build.yaml
version: 3
agents:
  - claude
  - codex
targets:
  container:
    name: my-workspace
    base: quay.io/cc-deck/cc-deck-base:latest
```

### Build Stages per Agent

Each agent contributes to the Containerfile through its interface:

1. **Binary installation** (`InstallScript()`): Each agent knows how to install itself. Claude Code uses `curl -fsSL https://claude.ai/install.sh | sh`, Codex uses npm, etc.

2. **Hook registration**: After all agents are installed, `cc-deck plugin install` runs inside the image and auto-detects + hooks all of them.

3. **Config scaffolding** (`ConfigPaths()`): Agent config directories that need to exist or be pre-populated.

### Multi-Agent Containerfile Pattern

```dockerfile
# Stage: Install agents
# (generated from manifest agents list)
RUN curl -fsSL https://claude.ai/install.sh | sh           # claude
RUN npm install -g @openai/codex                            # codex

# Stage: Install cc-deck + hooks for all agents
COPY cc-deck /usr/local/bin/cc-deck
RUN cc-deck plugin install --install-zellij
# Auto-detects claude + codex, installs hooks for both
```

## Credential Transport (Open Question)

This is the hardest unsolved problem for container and remote environments. Each agent needs API keys at runtime, and these must not be baked into images.

### Current Approach (Claude Code only)

- Local: User has `ANTHROPIC_API_KEY` in shell env or OAuth credentials in `~/.claude/`
- Podman: `podman secret create` + `--secret` flag + env var pointing to `/run/secrets/`
- K8s: Kubernetes Secrets mounted as env vars or files

### Multi-Agent Credential Matrix

| Agent | Primary credential | Alt credential | Config files |
|---|---|---|---|
| Claude Code | `ANTHROPIC_API_KEY` | OAuth (`~/.claude/.credentials.json`) | `~/.claude/settings.json` |
| Codex CLI | `OPENAI_API_KEY` | | TBD |
| Gemini CLI | `GEMINI_API_KEY` | Google Cloud ADC (`~/.config/gcloud/application_default_credentials.json`) | `~/.gemini/settings.json` |

### Approaches Under Consideration

**A. Environment variables only:** Simple, works everywhere. Each agent declares its `CredentialEnvVars()`. The workspace is responsible for injecting them.

**B. Secret mount convention:** Define a standard mount point (`/run/secrets/cc-deck/`) where each agent's credentials are placed by name (`claude-api-key`, `codex-api-key`). Agents read from env vars that point to these files.

**C. cc-deck credential broker:** A `cc-deck credentials` command that reads from the workspace's secret store (env vars, Podman secrets, K8s Secrets) and exports them as the agent-specific env vars. Run as an entrypoint wrapper.

**D. Credential proxy (inspired by lince).** A localhost HTTP proxy that intercepts API calls and injects credentials on the fly. Agent environments never see the actual API keys. Strongest security posture, but adds complexity.

**Decision deferred.** This intersects heavily with workspace credential transport. The credential proxy approach (D) is worth prototyping for container environments where env var exposure is the primary risk.

## Sidebar Display Changes

### Agent Indicator

When multiple agents are in use, show an agent icon or abbreviation:

```
[C] project-backend    ● Working     main
[X] api-refactor       ⚠ Permission  feat/api
[G] docs-update        ✓ Done        docs
```

Where: `[C]` = Claude, `[X]` = Codex, `[G]` = Gemini

When only one agent type is in use, hide the prefix to save space.

### Two-Tier Status Display

Following lince's graceful degradation pattern, the sidebar adapts to the richness of available status information:

**Tier 1: Native hooks (Claude, Codex, Gemini).** Full status with activity state, tool names, subagent counts, elapsed time. This is what cc-deck shows today for Claude Code.

**Tier 2: Wrapper-only agents.** Minimal status showing Init/Running/Stopped. No tool names, no subagent counts. The sidebar entry still works for navigation and smart attend, just with less detail.

The visual distinction should be subtle. A wrapper-only agent simply shows fewer details, not a degraded UI.

## Smart Attend Across Agents

The current smart attend algorithm (priority tiers: Permission > Notification > Done > Idle, skip Working/Paused) works agent-agnostically. No changes needed for the core algorithm.

Optional enhancement: per-agent attend priority. Deferred unless users request it.

## Competitive Positioning vs cmux and lince

| Capability | cc-deck (current) | cc-deck (with multi-agent) | cmux | lince |
|---|---|---|---|---|
| Agent support | Claude Code only | Claude, Codex, Gemini, extensible | Claude Code, OpenCode | Claude, Codex, Gemini, OpenCode, Aider |
| Platform | Linux + macOS | Linux + macOS | macOS only | Linux + macOS (experimental) |
| Remote execution | Containers, SSH, K8s | Same, with per-agent install | None (Cloud VMs planned) | None (local only) |
| Session states | 7 granular states | Same | Binary (attention/not) | 6 states |
| Smart attend | Priority-based round-robin | Same, works across agents | Jump to latest unread | Manual selection |
| Sandboxing | Podman containers | Same | None | bubblewrap / nono per agent |
| Environment mgmt | 6 workspace types | Same | None | None |
| Notification protocol | Custom hooks | Custom hooks (OSC not viable) | OSC 9/99/777 + CLI | Custom hooks + agent wrapper |
| Agent wrapper | None | `cc-deck-agent-wrapper` | None | `lince-agent-wrapper` |

## Dependencies and Sequencing

This feature depends on the **execution environment abstraction**, which defines how cc-deck manages sessions across workspace types. The agent interface (especially `InstallScript()`, `CredentialEnvVars()`, and credential transport) must align with the workspace abstraction. **Prerequisite is DONE.**

Recommended order:
1. **Agent wrapper script** (cc-deck-agent-wrapper, enables any CLI as a managed session)
2. **Agent interface definition** (Go interface + registry + config)
3. **Codex adapter** (first non-Claude agent, validates the interface)
4. **Gemini adapter** (richer event model, stress-tests the interface)
5. **Image build integration** (multi-agent manifests)
6. **Credential transport** (consider proxy approach)

## Lessons from paude (May 2026 Analysis)

[paude](https://github.com/bbrowning/paude) is a Python CLI for running AI coding agents in secure containers. It supports Claude Code, Cursor CLI, Gas City, Gemini CLI, and OpenClaw. Unlike cc-deck's interactive terminal approach, paude uses a fire-and-forget model where agents work in containers and users harvest results via git. Its agent abstraction is mature (v0.15.0, 592 commits) and worth studying.

### Patterns to adopt from paude

**1. AgentConfig dataclass with rich metadata.** paude's `AgentConfig` captures 20+ fields per agent in a single dataclass: identity (`name`, `display_name`, `process_name`), installation (`install_script`, `install_dir`), environment (`env_vars`, `passthrough_env_vars`, `secret_env_vars`, `passthrough_env_prefixes`), container settings (`config_dir_name`, `config_file_name`, `exposed_ports`, `default_base_image`), and interaction flags (`yolo_flag`, `clear_command`, `args_env_var`, `session_name`).

Our Go `Agent` interface already covers most of these concerns but distributes them across method returns rather than a single struct. Consider adding an `AgentConfig` struct that consolidates static agent metadata, keeping the Go interface for behavioral methods only.

**2. Provider/Credential separation.** paude separates agent identity from provider credentials via `ProviderCredentials`. The same agent (e.g., Claude Code) can use different providers (Anthropic direct, Vertex AI), and each provider declares its own credential requirements. The `build_provider_credentials()` function resolves the correct credential set based on agent + provider combination.

This matters for cc-deck because our credential transport design (see Open Question 6) currently ties credentials to agents. A provider abstraction would let users run Claude Code via Vertex AI with different credentials than Anthropic direct, without changing the agent adapter.

```go
type ProviderConfig struct {
    Name                 string
    PassthroughEnvVars   []string
    SecretEnvVars        []string
    PassthroughPrefixes  []string
    ExtraEnvVars         map[string]string
    ModelConfig          map[string]string
}
```

**3. Trust/onboarding suppression per agent.** paude ships `claude_trust_script()` and `gemini_trust_script()` functions that generate shell snippets to pre-configure agents for non-interactive use. These use `jq` to safely manipulate JSON config files, setting `hasCompletedOnboarding`, `hasTrustDialogAccepted`, and `trustedFolders`.

Our `apply_sandbox_config()` equivalent should be part of the Agent interface, called during container setup:

```go
type Agent interface {
    // ... existing methods ...
    SandboxConfigScript(home, workspace, args string, yolo bool) string
}
```

**4. Per-agent domain aliases.** Each paude agent declares `extra_domain_aliases` (e.g., Claude adds `["claude"]`, Cursor adds `["cursor"]`, Gemini adds `["gemini", "nodejs"]`). When the user requests `"default"` domains, the expansion merges `BASE_ALIASES` (vertexai, python, github) with the agent's extras. This means the network filter adapts automatically to the agent type.

cc-deck's domain group system (brainstorm 22) should adopt this. The agent adapter declares which domain groups it needs, and the workspace definition merges them with user-specified groups.

**5. Exposed ports for web-based agents.** paude's `exposed_ports` field (list of host/container port tuples) supports agents like OpenClaw that have web UIs rather than CLI-only interfaces. This is forward-looking for cc-deck if we ever support web-based agents or agents with debug UIs.

**6. Pipefail-safe installation scripts.** paude wraps agent install commands in `SHELL ["/bin/bash", "-o", "pipefail", "-c"]` Dockerfile directives and verifies the binary exists after installation. This prevents silent `curl | sh` failures during image builds. Our `InstallScript()` should include similar verification.

### Patterns that validate our approach

**Agent Protocol (Python Protocol class).** paude uses a Python `Protocol` (structural typing) with methods: `config` (property), `dockerfile_install_lines()`, `apply_sandbox_config()`, `launch_command()`, `host_config_mounts()`, `build_environment()`. This is structurally similar to our Go `Agent` interface, confirming the interface approach is correct.

**Tmux session names per agent.** paude assigns each agent a `session_name` for tmux management (e.g., `"claude"`, `"cursor"`). Our Zellij pane-based approach is analogous but more native to our terminal multiplexer.

### Comparative analysis

| Aspect | paude | cc-deck (current) | cc-deck (proposed) |
|---|---|---|---|
| Agent config model | Single `AgentConfig` dataclass | Go interface methods | Hybrid: `AgentConfig` struct + interface |
| Credential model | Provider-level separation | Agent-level only | Add provider abstraction |
| Trust suppression | Shell script generators | Hook-based config | Add `SandboxConfigScript()` |
| Domain filtering | Per-agent `extra_domain_aliases` | Global domain groups | Per-agent domain group declarations |
| Installation safety | Pipefail + binary verification | Basic install scripts | Add pipefail + verification |
| Web agent support | `exposed_ports` field | Not supported | Add port exposure to config |

## Related Brainstorms

- **042 (voice-relay)**: Voice relay via PipeChannel, agent-agnostic, focus-based routing
- **043 (clipboard-bridge)**: Clipboard image bridging via DataChannel
- **025 (security-model)**: Credential proxy approach intersects with security model

## Open Questions

1. **Event schema stability:** How stable are Codex and Gemini's hook schemas? Breaking changes would require adapter updates. Should we version the adapter configs?

2. **Mixed-agent tabs:** Can a single Zellij tab run both Claude Code and Codex panes? Yes, each pane has one agent, pane_id disambiguates. But layout templates would need to support heterogeneous agents per tab.

3. **Agent auto-detection reliability:** What if an agent is installed but not configured (no API key)? Should `cc-deck plugin install` still hook it? Probably yes, with a warning.

4. **Cursor CLI evolution:** Monitor for a proper hooks API before investing in Cursor support.

5. **Aider, Continue, other agents:** The adapter protocol should be generic enough that adding new agents is trivial. The `--raw` mode handles this.

6. **Credential rotation in long-running containers:** If API keys expire or rotate, how do running containers pick up new credentials? This is a workspace concern.

7. **Agent-specific permissions models:** Claude Code has YOLO mode, Codex has permission levels. Should cc-deck normalize these or expose them? Probably expose as metadata.

8. **lince interoperability:** lince uses `claude-status` as its pipe name for Claude Code hooks. If a user has both lince and cc-deck installed, hook conflicts could arise. Our pipe namespace (`cc-deck:hook`) avoids this, but worth documenting.

9. **Git-push restriction as workspace flag:** Should `restrict-push: true` be a workspace-level setting or an agent-level setting? Workspace-level makes more sense (you restrict the execution context, not the agent type).

---

## Revisit: 2026-06-06

### Updated Hook Ecosystem Research

The hook landscape has expanded since the original brainstorm. Three additional tools now have documented hook systems:

**Cline (v3.36+):** 6 events (PreToolUse, PostToolUse, UserPromptSubmit, TaskStart, TaskResume, TaskCancel). JSON-on-stdin, exit-code blocking. Scripts live in `~/Documents/Cline/Rules/Hooks/` or `.clinerules/hooks/`. macOS/Linux only. Follows the same JSON-stdin pattern as Claude/Codex/Gemini.

**OpenCode:** 20+ events via TypeScript plugin system. In-process execution (not shell-invocable). Plugins register in `.opencode/plugin/` or `~/.config/opencode/plugin/`. Event model is the most granular (LSP diagnostics, message parts). Completely different architecture from the JSON-stdin family. This makes it the ideal stress-test canary for the abstraction.

**OpenClaw:** 12 events via hybrid HOOK.md + plugin API. In-process TypeScript for the plugin API, YAML frontmatter-based HOOK.md for simpler hooks. Multi-channel aware (Telegram, Discord, Slack, WhatsApp). Security note: CVE-2026-41336 and CVE-2026-25253 disclosed in early 2026.

**No common standard exists.** A March 2026 cross-ecosystem analysis concludes that "any abstraction spanning multiple platforms must adapt to each model rather than assume a common interface." However, de facto convergence is clear: PreToolUse/PostToolUse, SessionStart, and Stop appear in nearly every implementation. JSON-on-stdin with exit-code blocking (exit 2 = block) is the dominant out-of-process protocol.

### Updated Codebase Coupling Analysis

A thorough scan of the cc-deck codebase identified 70+ Claude Code coupling points:

**Hooks (very tight):** 11 hardcoded Claude event names in `internal/plugin/hooks.go:24-36`. `ClaudeSettingsPath()` hardcoded to `~/.claude/settings.json`. All 6 plugin package files reference this path.

**Credentials (very tight):** `ANTHROPIC_API_KEY` referenced in 9+ files. `CLAUDE_CODE_USE_VERTEX` and `CLAUDE_CODE_USE_BEDROCK` flags in 5+ files. Claude-specific credential profiles in `internal/openshell/credentials.go` and `internal/ssh/credentials.go`.

**Network policies (very tight):** `claude_code` component in `internal/build/policies/claude-code.yaml` with `match: always: true`. Hardcoded anthropic domains in `internal/network/builtin.go`. Explicit `if comp.Key == "claude_code"` check in `internal/build/policy.go:251`.

**Binary discovery (tight):** Hardcoded paths `/usr/local/bin/claude`, `/sandbox/.local/bin/claude`. Probe check `claude --version` in `internal/cmd/build.go:703`.

**Rust plugin (tight):** `HookPayload` struct and `hook_event_to_activity()` mapping in `cc-zellij-plugin/src/pipe_handler.rs`. Hardcoded "Claude Code" UI text in `cc-zellij-plugin/src/sidebar_plugin/render.rs`.

**Branding:** "Manage Claude Code workspaces" in `cmd/cc-deck/main.go:30`.

### Architecture Decision: Pure Go Interface (No Config File)

The original brainstorm proposed a hybrid config + interface approach (agents.yaml for display, Go interface for behavior). After analysis, we decided on a **pure Go interface** with no agents.yaml config file.

**Rationale:**
- The config file solves a flexibility problem that doesn't exist for a curated product with first-class agent support
- Config files add failure modes (malformed YAML, schema skew, silent misconfiguration) without adding functionality for built-in agents
- All agent properties (events, config paths, display labels, credential env vars) are static and well-defined
- The `cc-deck-agent-wrapper` script serves as the extension mechanism for arbitrary/unknown agents without requiring any config
- Pure Go structs give type safety, easier testing (no config file fixtures), and compiler-enforced interface compliance

**What stays from the original design:**
- The `Agent` interface concept (identity, detection, hook lifecycle, event translation)
- The agent registry (Go map)
- The two-tier status model (Tier 1: native hooks, Tier 2: wrapper only)
- The `cc-deck-agent-wrapper` script concept
- The `cc-deck hook --raw` command for normalized payloads

**What changes:**
- No `agents.yaml` config file
- Display properties (label, color, indicator) are methods on the Go struct, not config values
- Event mapping is code, not config overrides
- Runtime configurability is deferred until a real need emerges

### First Spec Scope: Core Abstraction Layer

The first specification from this brainstorm covers:

1. **Go `Agent` interface** definition with methods for identity, detection, hook lifecycle, event translation, pane identification
2. **Agent registry** (Go map, built-in agents register at init)
3. **`ClaudeAgent` adapter** (refactor existing hardcoded Claude logic into an interface implementation)
4. **`OpenCodeAgent` adapter** (stress-test canary, validates the abstraction handles in-process/TypeScript hook agents via wrapper approach)
5. **`cc-deck-agent-wrapper` script** for hookless or differently-hooked agents
6. **`cc-deck hook --raw` command** to accept normalized payloads from wrappers
7. **Rust plugin generalization:** `agent` field in `HookPayload`, agent indicator `[C]`/`[O]` in sidebar, remove hardcoded "Claude Code" text

**Why OpenCode as the canary (not Codex or Gemini):** OpenCode's in-process TypeScript plugin system is architecturally the furthest from Claude Code's JSON-stdin shell hooks. Codex and Gemini use the same JSON-stdin pattern as Claude, so they'd test "similar but different," not "genuinely different." OpenCode forces the abstraction to handle the wrapper/bridge path from day one, proving the two-tier model works. If the abstraction handles both Claude (full hooks, Tier 1) and OpenCode (wrapper, Tier 2), adding Codex and Gemini becomes straightforward.

### Deferred to Follow-Up Specs

These areas are explicitly out of scope for the first spec:

1. **Network policy generalization** (per-agent domain declarations, removing `match: always: true` for claude_code)
2. **Credential transport abstraction** (per-agent/per-provider credential model, provider separation inspired by paude)
3. **Build system multi-agent support** (manifest `agents` section, multi-agent Containerfile generation, per-agent install scripts)
4. **Additional agent adapters** (Codex, Gemini, Cline, OpenClaw)
5. **Naming/branding** is kept as "cc-deck"; just clean up help text where it implies Claude Code exclusivity

### Updated Open Questions

10. **OpenCode bridge mechanism:** How does cc-deck get lifecycle events from OpenCode? Options: (a) wrapper script emitting Init/End only, (b) a TypeScript OpenCode plugin that calls `cc-deck hook --raw` on relevant events, giving richer status. The first spec should support both.

11. **Agent detection order:** When multiple agents are installed, should `cc-deck plugin install` hook all of them or ask? Original decision: hook all, show a report. Still valid.

12. **Wrapper PID tracking:** The agent wrapper needs to track the child process PID for clean shutdown. If the wrapped agent is killed, the wrapper must emit the End event. Signal handling in the wrapper script.

13. **Pane-to-agent association persistence:** When a session starts, cc-deck learns which pane runs which agent via hook events. If the plugin restarts (Zellij reload), this association is lost. Should it be persisted to WASI `/cache/` alongside sessions.json?

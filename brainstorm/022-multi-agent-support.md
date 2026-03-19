# Brainstorm: Multi-Agent Support for cc-deck

**Date:** 2026-03-19
**Status:** Brainstorm
**Trigger:** Competitive analysis of [cmux](https://github.com/manaflow-ai/cmux) and its agent-agnostic notification approach

## Problem Statement

cc-deck currently only works with Claude Code via its hooks system. Other AI coding agents (OpenAI Codex CLI, Google Gemini CLI, Cursor CLI) have similar lifecycle events and hook mechanisms. Supporting multiple agents would significantly broaden cc-deck's audience and strengthen its position against macOS-only competitors like cmux.

## Design Principle: Agent as an Interface

An "agent" in cc-deck should be a well-defined interface, not hardcoded Claude Code logic. This interface governs three concerns:

1. **Hook integration** - how to install hooks into the agent's configuration, how to translate the agent's events into cc-deck's normalized state model
2. **Installation** - how to install the agent binary itself (relevant for container images and remote environments)
3. **Credential transport** - how to securely provide API keys and auth tokens to the agent at runtime

This interface must work identically across all execution environments (local, Podman container, Kubernetes Deployment, Kubernetes sandbox). The execution environment abstraction is a separate concern and will be designed in a dedicated session. This document focuses on the agent interface itself.

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

## The Agent Interface

Each supported agent implements a Go interface:

```go
type Agent interface {
    // Identity
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

    // Event translation
    TranslateEvent(stdin []byte) (*HookPayload, error)  // agent JSON -> normalized

    // Image build (for container environments)
    InstallScript() string          // shell commands to install the agent binary
    CredentialEnvVars() []string    // env vars needed at runtime (e.g., ANTHROPIC_API_KEY)
    ConfigPaths() []string          // paths to copy/mount for configuration
}
```

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

The `cc-deck-build.yaml` manifest (source of truth for image builds) needs an `agents` section:

```yaml
# cc-deck-build.yaml
version: 1
agents:
  - claude
  - codex
image:
  base: quay.io/cc-deck/cc-deck-base:latest
  # ...
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

**A. Environment variables only:** Simple, works everywhere. Each agent declares its `CredentialEnvVars()`. The execution environment is responsible for injecting them.

**B. Secret mount convention:** Define a standard mount point (`/run/secrets/cc-deck/`) where each agent's credentials are placed by name (`claude-api-key`, `codex-api-key`). Agents read from env vars that point to these files.

**C. cc-deck credential broker:** A `cc-deck credentials` command that reads from the execution environment's secret store (env vars, Podman secrets, K8s Secrets) and exports them as the agent-specific env vars. Run as an entrypoint wrapper.

**Decision deferred.** This intersects heavily with the execution environment abstraction (local vs Podman vs K8s). The credential transport mechanism should be designed as part of that work.

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

### Configurable Sidebar Fields

Possible fields (configurability is a separate feature):
- Agent type indicator (new)
- Session name (existing)
- Activity indicator (existing)
- Git branch (existing)
- Working directory (existing, used for auto-naming)
- Listening ports (new, from cmux inspiration, separate feature)

## Smart Attend Across Agents

The current smart attend algorithm (priority tiers: Permission > Notification > Done > Idle, skip Working/Paused) works agent-agnostically. No changes needed for the core algorithm.

Optional enhancement: per-agent attend priority. Deferred unless users request it.

## Competitive Positioning vs cmux

| Capability | cc-deck (current) | cc-deck (with multi-agent) | cmux |
|---|---|---|---|
| Agent support | Claude Code only | Claude, Codex, Gemini, extensible | Claude Code, OpenCode |
| Platform | Linux + macOS | Linux + macOS | macOS only |
| Remote execution | Containers, K8s planned | Same, with per-agent install | None (Cloud VMs planned) |
| Session states | 7 granular states | Same | Binary (attention/not) |
| Smart attend | Priority-based round-robin | Same, works across agents | Jump to latest unread |
| Browser integration | None | Future (MCP-based) | Built-in scriptable browser |
| Notification protocol | Custom hooks | Custom hooks (OSC not viable) | OSC 9/99/777 + CLI |

## Dependencies and Sequencing

This feature depends on the **execution environment abstraction**, which will define how cc-deck manages sessions across local, Podman, K8s Deployment, and K8s sandbox environments. The agent interface (especially `InstallScript()`, `CredentialEnvVars()`, and credential transport) must align with whatever abstraction emerges from that work.

Recommended order:
1. **Execution environment abstraction** (separate session, prerequisite)
2. **Agent interface definition** (this feature, Go interface + registry)
3. **Codex adapter** (first non-Claude agent, validates the interface)
4. **Gemini adapter** (richer event model, stress-tests the interface)
5. **Image build integration** (multi-agent manifests)
6. **Credential transport** (after execution environments are defined)

## Open Questions

1. **Event schema stability:** How stable are Codex and Gemini's hook schemas? Breaking changes would require adapter updates. Should we version the adapter configs?

2. **Mixed-agent tabs:** Can a single Zellij tab run both Claude Code and Codex panes? Yes, each pane has one agent, pane_id disambiguates. But layout templates would need to support heterogeneous agents per tab.

3. **Agent auto-detection reliability:** What if an agent is installed but not configured (no API key)? Should `cc-deck plugin install` still hook it? Probably yes, with a warning.

4. **Cursor CLI evolution:** Monitor for a proper hooks API before investing in Cursor support.

5. **Aider, Continue, other agents:** The adapter protocol should be generic enough that adding new agents is trivial. The `--raw` mode handles this.

6. **Credential rotation in long-running containers:** If API keys expire or rotate, how do running containers pick up new credentials? This is an execution environment concern.

7. **Agent-specific permissions models:** Claude Code has YOLO mode, Codex has permission levels. Should cc-deck normalize these or expose them? Probably expose as metadata.

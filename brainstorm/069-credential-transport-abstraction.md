# Brainstorm: Credential Transport Abstraction for Multi-Agent Support

**Date:** 2026-06-06
**Status:** active
**Depends on:** 066-agent-abstraction (core Agent interface must land first)
**Extracted from:** 022-multi-agent-support

## Problem Framing

Credential handling is currently hardcoded for Claude Code across the entire codebase. `ANTHROPIC_API_KEY` is referenced in 9+ files. `CLAUDE_CODE_USE_VERTEX` and `CLAUDE_CODE_USE_BEDROCK` mode flags appear in 5+ files. Credential profiles in `internal/openshell/credentials.go` define "claude" and "claude-vertex" profiles. The SSH credential logic in `internal/ssh/credentials.go` has a hardcoded priority order: "ANTHROPIC_API_KEY first, then Vertex, then Bedrock."

When multiple agents are supported, each agent needs different credentials (OpenAI API key for Codex, Google Cloud ADC for Gemini, etc.), and the credential transport must work across all workspace types (local, Podman, K8s, SSH, Compose, OpenShell).

## Prior Analysis (from brainstorm 022)

### Multi-Agent Credential Matrix

| Agent | Primary credential | Alt credential | Config files |
|---|---|---|---|
| Claude Code | `ANTHROPIC_API_KEY` | OAuth (`~/.claude/.credentials.json`) | `~/.claude/settings.json` |
| Codex CLI | `OPENAI_API_KEY` | | TBD |
| Gemini CLI | `GEMINI_API_KEY` | Google Cloud ADC (`~/.config/gcloud/application_default_credentials.json`) | `~/.gemini/settings.json` |
| OpenCode | `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` | Multiple provider support | `~/.config/opencode/config.json` |

### Provider/Credential Separation (from paude analysis)

paude separates agent identity from provider credentials via `ProviderCredentials`. The same agent (e.g., Claude Code) can use different providers (Anthropic direct, Vertex AI), and each provider declares its own credential requirements. The `build_provider_credentials()` function resolves the correct credential set based on agent + provider combination.

This matters because our credential transport currently ties credentials to agents. A provider abstraction would let users run Claude Code via Vertex AI with different credentials than Anthropic direct, without changing the agent adapter.

### Trust/Onboarding Suppression (from paude analysis)

paude ships trust scripts that pre-configure agents for non-interactive use (`hasCompletedOnboarding`, `hasTrustDialogAccepted`, `trustedFolders`). The Agent interface should include a `SandboxConfigScript()` method for this.

## Approaches Considered

### A: Environment Variables Only

Each agent declares its `CredentialEnvVars()`. The workspace is responsible for injecting them. Simple and works everywhere.

- Pros: Simplest. Works across all workspace types. No new infrastructure.
- Cons: Env vars are visible in process listings. No provider-level separation. Users must manually set the right env vars for each agent.

### B: Secret Mount Convention

Define a standard mount point (`/run/secrets/cc-deck/`) where each agent's credentials are placed by name (`claude-api-key`, `codex-api-key`). Agents read from env vars that point to these files.

- Pros: Credentials not visible in process listings. Consistent location across workspaces.
- Cons: Requires mount support in all workspace types. More setup complexity.

### C: cc-deck Credential Broker

A `cc-deck credentials` command that reads from the workspace's secret store (env vars, Podman secrets, K8s Secrets) and exports them as agent-specific env vars. Runs as an entrypoint wrapper.

- Pros: Single entry point for credential resolution. Workspace-agnostic.
- Cons: Another moving part in the startup chain. Must handle all workspace secret backends.

### D: Credential Proxy (inspired by lince)

A localhost HTTP proxy that intercepts API calls and injects credentials on the fly. Agent environments never see the actual API keys.

- Pros: Strongest security posture. Keys never enter agent environment.
- Cons: Highest complexity. Must handle HTTPS interception. Agent-specific API endpoint knowledge needed.

## Decision

Not yet decided. Approach A (env vars) is the pragmatic starting point. Approach C (credential broker) is the likely evolution. Approach D (proxy) is the aspirational goal for high-security environments. To be finalized during specification.

## Key Requirements

- Agent interface must declare credential requirements (env vars, config file paths, secret names)
- Provider-level separation: same agent can use different providers with different credentials
- All 6 workspace types (local, Podman, K8s, SSH, Compose, OpenShell) must support multi-agent credentials
- Existing Claude Code credential flows must not regress
- Trust/onboarding suppression scripts should be part of the Agent interface
- `internal/openshell/credentials.go` and `internal/ssh/credentials.go` must be generalized

## Open Questions

- Should there be a `cc-deck credentials check` command that verifies all required credentials are available before starting a workspace? (May fall out naturally from eager validation.)
- Credential rotation in long-running containers: if API keys expire, how do running containers pick up new credentials? (Claude Code has `gcpAuthRefresh` but this is agent-specific.)

---

## Revisit: 2026-06-07

### Updated Problem Framing

Research into actual agent credential requirements revealed that env var names are **not universal across agents** for the same provider. Vertex AI is the clearest example: Claude Code uses `ANTHROPIC_VERTEX_PROJECT_ID` + `CLOUD_ML_REGION`, Gemini CLI uses `GOOGLE_CLOUD_PROJECT` + `GOOGLE_CLOUD_LOCATION`, and OpenCode uses `GOOGLE_CLOUD_PROJECT` or `ANTHROPIC_VERTEX_PROJECT_ID` + `ANTHROPIC_VERTEX_REGION`. The JSON credential file (`GOOGLE_APPLICATION_CREDENTIALS`) is shared, but the companion env vars differ per agent. Codex CLI adds another wrinkle: its `env_key` config field makes the API key env var name itself configurable.

This means a centralized "provider profile" registry cannot own the env var mapping. Each agent must declare its own credential shape.

### Agent Credential Matrix (Researched)

| Provider | Claude Code | OpenCode | Codex CLI | Gemini CLI |
|----------|------------|----------|-----------|------------|
| Anthropic API | `ANTHROPIC_API_KEY` | `ANTHROPIC_API_KEY` | N/A | N/A |
| OpenAI API | N/A | `OPENAI_API_KEY` | configurable via `env_key` (defaults to `OPENAI_API_KEY`) | N/A |
| Google/Gemini | N/A | `GOOGLE_API_KEY` | N/A | `GEMINI_API_KEY` or `GOOGLE_API_KEY` |
| Vertex AI project | `ANTHROPIC_VERTEX_PROJECT_ID` | `GOOGLE_CLOUD_PROJECT` | N/A | `GOOGLE_CLOUD_PROJECT` |
| Vertex AI region | `CLOUD_ML_REGION` | `ANTHROPIC_VERTEX_REGION` | N/A | `GOOGLE_CLOUD_LOCATION` |
| Vertex AI file | `GOOGLE_APPLICATION_CREDENTIALS` | `GOOGLE_APPLICATION_CREDENTIALS` | N/A | `GOOGLE_APPLICATION_CREDENTIALS` |
| Mode flag | `CLAUDE_CODE_USE_VERTEX=1` | N/A | N/A | must *unset* `GEMINI_API_KEY` |

### New Approaches Considered

The original four approaches (A: env vars only, B: secret mount, C: broker, D: proxy) remain valid as transport mechanisms. The key new insight is about **ownership**: who declares what credentials an agent needs.

#### E: Hybrid with Agent-Heavy Ownership (chosen)

Agent interface declares full credential specs per auth mode. A thin shared package handles only transport (file copying, env injection, path remapping). No centralized profile registry.

- Pros: Adding a new agent means implementing `CredentialSpecs()` only, no touching registry or workspace types. Handles divergent env var names naturally. File credentials (Vertex JSON) are first-class.
- Cons: Some duplication if two agents happen to use identical specs. Agents must be kept up to date when upstream agents change their env var expectations.

### Updated Decision

**Chosen: Approach E (hybrid with agent-heavy ownership)**

The Agent interface gets `CredentialSpecs() []CredentialSpec`. Each CredentialSpec represents one auth mode and declares:

- **Name**: human-readable mode identifier (e.g., "api", "vertex", "bedrock")
- **EnvVars**: env var names the agent expects, with optional fixed values (e.g., `CLAUDE_CODE_USE_VERTEX=1`)
- **FileCredential**: env var pointing to a file + default path on host (e.g., `GOOGLE_APPLICATION_CREDENTIALS` with default `~/.config/gcloud/application_default_credentials.json`)
- **Endpoints**: host:port pairs for network policy generation (e.g., `oauth2.googleapis.com:443`)
- **UnsetVars**: env vars that must be unset to avoid conflicts (e.g., Gemini CLI needs `GEMINI_API_KEY` unset when using Vertex)
- **Priority**: ordering for auto-selection when user doesn't specify

**Auth mode selection:**
- `cc-deck ws new` detects which modes have credentials available on the host
- If multiple match, prompt user to pick (or accept `--auth-mode` flag)
- Chosen mode stored in workspace definition
- `cc-deck ws ls` shows active auth mode per workspace (e.g., "claude/vertex", "opencode/api")
- If no explicit choice, auto-select using agent-defined priority order

**Credential validation:**
- Eager by default: validate at workspace start that required env vars and files exist
- Workspace definition can mark credentials as "externally provided" (skip host-side validation for K8s Secrets, OpenShell providers, etc.)

**Shared package (`internal/credential`):**
- File transport: copy credential files into containers, set permissions, remap paths
- Env var injection: resolve from host env, inject into workspace
- Existence checks: verify env vars are set, files exist at expected paths
- No profile knowledge: doesn't know what "vertex" means, just moves data

**Code changes:**
- `openshell/credentials.go`: `KnownProviderProfiles` map replaced by agent-declared specs. `ResolveCredentials` and `DetectCredentials` move into shared credential package using agent specs as input.
- `ssh/credentials.go`: `detectAuthMode()` and hardcoded switch statement removed. `BuildCredentialSet` iterates over active agent's credential specs instead.
- `agent/agent.go`: Agent interface gains `CredentialSpecs() []CredentialSpec` method.
- `agent/claude.go`: Declares API, Vertex, Bedrock credential specs.
- `agent/opencode.go`: Declares OpenAI, Anthropic credential specs.

### Open Threads
- Credential sharing falls out naturally: if both Claude and OpenCode declare `ANTHROPIC_API_KEY`, the workspace resolves it once from the host env and injects for both.
- `cc-deck credentials check` could be the eager validation extracted into a standalone command.
- Credential rotation remains agent-specific (Claude has `gcpAuthRefresh`, others do not). May need a workspace-level refresh hook in the future.

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

- Should credential resolution be eager (fail fast at workspace start) or lazy (fail when agent tries to use API)?
- How do we handle agents that support multiple providers (e.g., OpenCode can use OpenAI or Anthropic)?
- Should there be a `cc-deck credentials check` command that verifies all required credentials are available before starting a workspace?
- Credential rotation in long-running containers: if API keys expire, how do running containers pick up new credentials?
- Should the credential broker support credential sharing (one Anthropic key used by both Claude Code and OpenCode)?

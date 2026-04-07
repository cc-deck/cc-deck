# Research: SSH Remote Execution Environment

**Branch**: `033-ssh-environment` | **Date**: 2026-04-07

## Research Basis

Four parallel research agents explored the codebase:
1. Environment interface, implementations, and factory pattern
2. State management, definition system, and type-specific fields
3. CLI command structure, flag patterns, and dispatch
4. External tool invocation, process replacement, and credential patterns

## Decisions

### D-001: SSH Client Architecture
**Decision**: Create `internal/ssh/` package wrapping the system `ssh` binary.
**Rationale**: The codebase uses dedicated packages per external tool (`internal/podman/`, `internal/compose/`). SSH needs the same pattern. Using the system `ssh` binary (FR-027) ensures compatibility with user SSH configs, agents, and jump hosts.
**Alternatives**: Go SSH library (x/crypto/ssh) rejected per spec requirement FR-027.

### D-002: State Storage Model
**Decision**: Add `SSHFields` struct to `EnvironmentInstance` (v2 instances), following the `ContainerFields`/`ComposeFields` pattern.
**Rationale**: All newer environment types use v2 instances with type-specific pointer fields. SSH should follow the same pattern.
**Alternatives**: v1 EnvironmentRecord (legacy, only for local environments).

### D-003: Definition Extension
**Decision**: Add SSH-specific fields (`Host`, `Port`, `IdentityFile`, `JumpHost`, `SSHConfig`, `Workspace`) to `EnvironmentDefinition`.
**Rationale**: Existing definition struct has type-agnostic fields (Auth, Credentials, Env) that SSH reuses, but needs host/connection fields that are SSH-specific.
**Alternatives**: Using only the `Env` map for SSH config (rejected: poor UX, no validation).

### D-004: Process Replacement for Attach
**Decision**: Use `syscall.Exec()` to replace the cc-deck process with `ssh -t <host> zellij attach cc-deck-<name>`.
**Rationale**: Both `LocalEnvironment` and `ContainerEnvironment` use `syscall.Exec()` for attach. This gives the user a direct terminal connection.
**Alternatives**: Spawning SSH as subprocess (rejected: breaks terminal handling).

### D-005: Credential File Integration
**Decision**: Write credentials to `~/.config/cc-deck/credentials.env` on remote (mode 600). Zellij layout sources this file via ENV directive.
**Rationale**: Clarified in spec. Avoids modifying user shell config. Zellij layout controls the pane startup environment.
**Alternatives**: Modifying .bashrc (rejected: invasive), env-var-only injection (rejected: new panes after reattach lose credentials).

### D-006: Pre-flight Check Architecture
**Decision**: Implement as a list of `PreflightCheck` objects with `Run()` and optional `Remedy()` methods. Interactive prompts via `bufio.Scanner`/`fmt.Fprintf` (existing pattern from `config.PromptProfile()`).
**Rationale**: Each check is independent and has a clear pass/fail result. Some checks offer automated remediation (tool installation).
**Alternatives**: Single monolithic check function (rejected: not extensible, poor UX).

### D-007: Harvest via Temporary Git Remote
**Decision**: Add temporary git remote pointing to remote workspace via SSH, `git fetch`, then remove remote.
**Rationale**: Clarified in spec. Leverages git's native SSH transport. No extra tools needed beyond git and ssh.
**Alternatives**: git bundle (extra step), rsync of .git objects (fragile).

### D-008: Refresh-Creds as New Subcommand
**Decision**: Add `env refresh-creds [name]` subcommand to the lifecycle group.
**Rationale**: No existing command handles credential refresh. This is SSH-specific but could be extended to other types later.
**Alternatives**: Adding a method to the Environment interface (rejected: only SSH needs it currently).

### D-009: Auto Auth Detection Order
**Decision**: `ANTHROPIC_API_KEY` first, then Vertex, then Bedrock. First match wins.
**Rationale**: Clarified in spec. Matches the priority order in existing `DetectAuthMode()` function in `auth.go`.
**Alternatives**: Union of all found credentials (rejected per spec clarification).

### D-010: Default Workspace
**Decision**: Default remote workspace is `~/workspace` when not specified.
**Rationale**: Clarified in spec. Avoids polluting home directory.
**Alternatives**: Home directory (rejected: cluttered), `/workspace` (rejected: requires root).

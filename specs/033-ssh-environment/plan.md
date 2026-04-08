# Implementation Plan: SSH Remote Execution Environment

**Branch**: `033-ssh-environment` | **Date**: 2026-04-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/033-ssh-environment/spec.md`

## Summary

Implement a new `ssh` environment type for cc-deck that manages Zellij sessions on persistent remote machines over SSH connections. The implementation adds an `internal/ssh` package for SSH operations, an `SSHEnvironment` type implementing the `Environment` interface, pre-flight bootstrap with interactive tool installation, credential forwarding with persistent file-based storage, and CLI extensions including a `refresh-creds` subcommand. All SSH operations use the system `ssh` binary for full compatibility with user configurations.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML), adrg/xdg v0.5.3 (XDG paths via internal/xdg wrapper)
**Storage**: YAML files at `$XDG_CONFIG_HOME/cc-deck/environments.yaml` (definitions) and `$XDG_STATE_HOME/cc-deck/state.yaml` (runtime state)
**Testing**: `go test` via `make test`
**Target Platform**: darwin/linux (CLI runs locally, targets linux/amd64 and linux/arm64 remotes)
**Project Type**: CLI tool
**Performance Goals**: Pre-flight < 30s, status < 10s, credential refresh < 5s (from spec SC-003/004/008)
**Constraints**: Must use system `ssh` binary (FR-027), must use `make install`/`make test`/`make lint` (constitution VI)
**Scale/Scope**: Single-user CLI managing 1-10 remote environments

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | PASS | SSH is CLI-only (Go). No WASM plugin changes needed. |
| II. Plugin Installation | N/A | No plugin changes. |
| III. WASM Filename Convention | N/A | No WASM changes. |
| IV. WASM Host Function Gating | N/A | No WASM changes. |
| V. Zellij API Research Order | PASS | SSH attach uses remote Zellij via standard CLI commands. |
| VI. Build via Makefile Only | PASS | All builds via `make install`, `make test`, `make lint`. |
| VII. Interface Behavioral Contracts | PASS | SSH environment contract documented in contracts/ssh-environment.md. Spec references interface contract (FR-025). |
| VIII. Simplicity | PASS | Minimal new abstractions: SSHClient, PreflightCheck interface. No speculative features. |
| IX. Documentation Freshness | PENDING | Must update README.md, CLI reference, and Antora docs upon completion. |
| X. Spec Tracking in README | PENDING | Must add 033-ssh-environment to README spec table. |
| XI. Release Process | N/A | Not a release. |
| XII. Prose Plugin for Documentation | PENDING | Must use prose plugin for all documentation. |
| XIII. XDG Paths on All Platforms | PASS | Uses internal/xdg package. Remote credential file at ~/.config/cc-deck/. |
| XIV. No Dotfile Nesting | PASS | No dot-prefixed files inside dot directories. |

## Project Structure

### Documentation (this feature)

```text
specs/033-ssh-environment/
├── spec.md
├── plan.md              # This file
├── research.md          # Research findings
├── data-model.md        # Entity definitions
├── quickstart.md        # Usage examples
├── contracts/
│   └── ssh-environment.md
├── checklists/
│   └── requirements.md
└── tasks.md             # Task breakdown (generated separately)
```

### Source Code (repository root)

```text
cc-deck/internal/
├── ssh/                      # NEW: SSH client package
│   ├── client.go             # SSH command builder and executor
│   ├── client_test.go
│   ├── bootstrap.go          # Pre-flight checks and tool installation
│   ├── bootstrap_test.go
│   ├── credentials.go        # Credential file management on remote
│   └── credentials_test.go
├── env/
│   ├── ssh.go                # NEW: SSHEnvironment implementing Environment
│   ├── ssh_test.go           # NEW: Unit tests
│   ├── types.go              # MODIFY: Add EnvironmentTypeSSH, SSHFields
│   ├── factory.go            # MODIFY: Add SSH case
│   ├── definition.go         # MODIFY: Add SSH fields to EnvironmentDefinition
│   ├── errors.go             # MODIFY: Add ErrSSHNotFound
│   ├── interface.go          # NO CHANGE
│   ├── auth.go               # NO CHANGE (reuse existing)
│   ├── state.go              # NO CHANGE (v2 instances work as-is)
│   └── validate.go           # NO CHANGE
├── cmd/
│   └── env.go                # MODIFY: Add SSH flags, refresh-creds subcommand
```

**Structure Decision**: Single package for SSH client (`internal/ssh/`), environment implementation in existing `internal/env/` package. Follows the `internal/podman/` and `internal/compose/` patterns.

## Implementation Phases

### Phase 1: Foundation (Types, Client, Factory)

**Goal**: SSH type registered, client package operational, basic Create/Delete working.

**Tasks**:
1. Add `EnvironmentTypeSSH`, `SSHFields` to `types.go`; add `SSH *SSHFields` to `EnvironmentInstance`
2. Add SSH fields (`Host`, `Port`, `IdentityFile`, `JumpHost`, `SSHConfig`, `Workspace`) to `EnvironmentDefinition` in `definition.go`
3. Add `ErrSSHNotFound` to `errors.go`
4. Create `internal/ssh/client.go` with SSH command builder:
   - `NewClient(host, port, identityFile, jumpHost, sshConfig)` constructor
   - `Run(ctx, cmd string) (string, error)` for non-interactive command execution
   - `RunInteractive(cmd string) error` for process replacement via `syscall.Exec`
   - `Check(ctx) error` for connectivity test
   - `RemoteInfo(ctx) (os, arch string, error)` for OS/arch detection
   - `buildArgs(extraArgs ...string) []string` internal helper for SSH arg construction
5. Create `internal/env/ssh.go` with `SSHEnvironment` struct and basic methods:
   - `Type()`, `Name()` identity methods
   - `Create()` with name validation, conflict check, SSH connectivity test, state recording
   - `Delete()` with running check, best-effort remote session kill, state removal
   - `Start()`, `Stop()` returning `ErrNotSupported`
   - Stub implementations for remaining methods (returning `ErrNotSupported` temporarily)
6. Add SSH case to `factory.go`
7. Add SSH flags to `createFlags` in `cmd/env.go` and wire them into `runEnvCreate()`
8. Unit tests for client.go and ssh.go

### Phase 2: Attach and Status

**Goal**: Users can attach to remote Zellij sessions and check environment status.

**Tasks**:
1. Implement `SSHEnvironment.Attach()`:
   - Nested Zellij detection (`$ZELLIJ` env var)
   - Load definition for SSH params
   - Update `LastAttached` timestamp
   - Check if remote Zellij session exists (`zellij list-sessions -n` via SSH)
   - Create session if missing (`zellij attach --create-background cc-deck-<name> --layout cc-deck`)
   - `syscall.Exec` to replace process with SSH connection to `zellij attach cc-deck-<name>`
2. Implement `SSHEnvironment.Status()`:
   - Query remote via SSH with timeout
   - Parse `zellij list-sessions -n` output for `cc-deck-<name>`
   - Return running/stopped/error states
3. Add `Upload(ctx, localPath, remotePath string) error` and `Download(ctx, remotePath, localPath string) error` to SSH client (via `scp` for now, rsync in Phase 4)
4. Unit tests

### Phase 3: Pre-flight Bootstrap

**Goal**: Interactive tool installation during Create.

**Tasks**:
1. Create `internal/ssh/bootstrap.go` with `PreflightCheck` interface and implementations:
   - `ConnectivityCheck` (SSH connectivity)
   - `OSDetectionCheck` (OS/arch detection)
   - `ZellijCheck` (check + install offer)
   - `ClaudeCodeCheck` (check + install offer)
   - `CcDeckCheck` (check + install offer)
   - `PluginCheck` (check + install offer)
   - `CredentialCheck` (verify auth mode can be satisfied)
2. Implement `RunPreflightChecks(ctx, client, stdin, stdout)` orchestrator:
   - Run each check sequentially
   - For failures with remedy: prompt user (install/skip/manual)
   - For unsupported platforms: warn and skip
3. Integrate pre-flight checks into `SSHEnvironment.Create()` (replace simple connectivity test)
4. Unit tests with mock SSH responses

### Phase 4: Credentials and Refresh

**Goal**: Credential forwarding on attach, credential refresh without attaching.

**Tasks**:
1. Create `internal/ssh/credentials.go`:
   - `WriteCredentialFile(ctx, client, creds map[string]string) error` writes env file on remote
   - `CopyCredentialFile(ctx, client, localPath, remoteName string) error` for file-based creds (GCP JSON)
   - `BuildCredentialSet(def *EnvironmentDefinition) (map[string]string, error)` resolves credentials from definition + local env
2. Integrate credential writing into `SSHEnvironment.Attach()` (before session creation/attach)
3. Implement `refresh-creds` subcommand:
   - Add `newEnvRefreshCredsCmd()` to `cmd/env.go`
   - Resolve environment, load definition, build credential set, write to remote
   - Handle auth=none (report disabled)
4. Unit tests

### Phase 5: Data Transfer (Push, Pull, Harvest, Exec)

**Goal**: File sync, remote command execution, and git commit harvesting.

**Tasks**:
1. Add rsync support to SSH client:
   - `Rsync(ctx, src, dst string, excludes []string, push bool) error`
   - Fallback to scp if rsync unavailable on remote
2. Implement `SSHEnvironment.Push()` and `Pull()`:
   - Use rsync with SSH transport
   - Respect exclusion patterns from SyncOpts
   - Default remote path: configured workspace
3. Implement `SSHEnvironment.Exec()`:
   - Run command via SSH in workspace directory (`cd <workspace> && <cmd>`)
   - Return output to caller
4. Implement `SSHEnvironment.Harvest()`:
   - Add temporary git remote: `git remote add cc-deck-<name> ssh://<host>/<workspace>`
   - `git fetch cc-deck-<name>`
   - Remove temporary remote
   - If CreatePR: create PR via `gh` CLI
5. Unit tests

### Phase 6: Parallel Status and Integration

**Goal**: Multi-environment status, reconciliation, and end-to-end polish.

**Tasks**:
1. Add SSH reconciliation function (following `ReconcileContainerEnvs` pattern):
   - Query each SSH environment in parallel with per-host timeout
   - Update state store with live status
2. Wire SSH reconciliation into `runEnvList()` for parallel status display
3. Update `resolveEnvironment()` in `cmd/env.go` to handle SSH instances
4. Integration testing with a real remote (manual test plan)
5. Edge case handling:
   - Unreachable hosts (timeout handling)
   - Deleted remote sessions (status reporting)
   - Invalid SSH configurations (clear error messages)

### Phase 7: Documentation

**Goal**: Complete documentation per constitution principles IX, X, XII.

**Tasks**:
1. Update README.md:
   - Add SSH environment description and usage examples
   - Add 033-ssh-environment to Feature Specifications table
2. Update CLI reference (`docs/modules/reference/pages/cli.adoc`):
   - Document `env create` SSH flags
   - Document `env refresh-creds` command
   - Document SSH-specific behavior for attach, status, push, pull, harvest
3. Create Antora guide page for SSH environments (`docs/modules/running/pages/ssh-environments.adoc`)
4. Run `/prose:check` on all documentation
5. Update landing page if applicable

## Complexity Tracking

No constitution violations. No complexity justifications needed.

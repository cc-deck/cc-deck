# Research: Environment Interface and CLI

**Feature**: 023-env-interface | **Date**: 2026-03-20

## R1: State Management Approach

**Decision**: New `state.yaml` file at `$XDG_STATE_HOME/cc-deck/state.yaml` using `adrg/xdg` package's `xdg.StateHome` path.

**Rationale**: The existing `config.yaml` stores profiles and session records together. Environment state (timestamps, container IDs, lifecycle states) changes frequently and pollutes user-edited config. XDG_STATE_HOME is the correct XDG directory for application state that should persist across reboots but is not user-editable configuration. The `adrg/xdg` package (already a dependency at v0.5.3) provides `xdg.StateHome`.

**Alternatives considered**:
- Keep in `config.yaml`: Rejected because frequent writes to config file risk corruption of user-edited profiles.
- Use `$XDG_DATA_HOME`: Valid but XDG_STATE_HOME is semantically more correct for ephemeral-ish state.
- SQLite: Overkill for tracking a handful of environments.

## R2: Atomic File Writes

**Decision**: Use write-to-temp-then-rename pattern, matching the existing `SaveSnapshot()` implementation in `session/snapshot.go`.

**Rationale**: The codebase already uses this pattern for snapshots. The state file will be written atomically: write to `state.yaml.tmp`, then `os.Rename()` to `state.yaml`. This prevents corruption from crashes or concurrent access.

**Alternatives considered**:
- File locking (flock): Adds complexity for a single-user tool. Lost updates are acceptable per spec clarification.
- Advisory lock file: Same conclusion, unnecessary complexity.

## R3: Environment Interface Design

**Decision**: Go interface with per-type implementations registered via a factory function. The interface methods map directly to CLI subcommands.

**Rationale**: The existing codebase separates CLI commands (`internal/cmd/`) from business logic (`internal/session/`, `internal/k8s/`). The Environment interface lives in a new `internal/env/` package. Each type (Local, Podman, K8s) gets its own file implementing the interface. A `NewEnvironment(envType, name, stateStore)` factory creates the correct implementation.

**Alternatives considered**:
- Single mega-struct with type switch: Violates Open/Closed principle, forces touching existing code for each new type.
- Plugin system (hashicorp/go-plugin): Overkill for 4 known types.

## R4: CLI Command Group Pattern

**Decision**: Follow the existing command group pattern used by `plugin`, `snapshot`, `profile`, and `image` commands. Public `NewEnvCmd(gf)` parent with private `newEnvCreateCmd(gf)`, `newEnvListCmd(gf)`, etc.

**Rationale**: Exact match with existing codebase conventions. The parent command has no `RunE`, subcommands are added via `AddCommand()`. Registration in `main.go` via `rootCmd.AddCommand(cmd.NewEnvCmd(gf))`.

**Alternatives considered**: None. The pattern is established and consistent.

## R5: Zellij Session Detection for Local Environments

**Decision**: Use `zellij list-sessions` (parsed from stdout) to detect running sessions. Zellij binary detection uses existing `plugin/zellij.go` utilities (`exec.LookPath`, version parsing).

**Rationale**: The codebase already has Zellij binary detection and version checking in `plugin/zellij.go`. For local environment reconciliation, `zellij list-sessions` outputs one session name per line. The expected session name for a local environment is `cc-deck-<env-name>`.

**Alternatives considered**:
- `pgrep -x zellij`: Already used in `plugin/remove.go` but only detects if any Zellij is running, not specific sessions.
- Read Zellij's internal state files: Fragile, undocumented, version-dependent.

## R6: Output Format

**Decision**: Reuse existing global `-o/--output` flag (already in `GlobalFlags` struct) supporting text/json/yaml. The `env list` and `env status` commands use the same pattern as `session/list.go`.

**Rationale**: The existing `List()` function in `session/list.go` already implements text/json/yaml output formatting. The `env list` command can follow the same pattern with struct tags.

**Alternatives considered**: None. The pattern is established.

## R7: Name Validation

**Decision**: Add a `ValidateEnvName(name string) error` function in the new `internal/env/` package. Pattern: `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, max 40 chars.

**Rationale**: Currently no name validation exists (validation happens at K8s API level for K8s sessions). The environment interface serves all types, so validation must satisfy the most restrictive naming rules: K8s DNS subdomain (lowercase alphanumeric + hyphens, start/end with alphanumeric). Max 40 chars leaves room for `cc-deck-` prefix (48 total, well under K8s 63-char limit).

**Alternatives considered**:
- No validation (defer to runtime): Creates confusing errors when K8s rejects names.
- Broader allowed chars: Would fail at K8s resource creation time.

## R8: Config Migration

**Decision**: On first state file load, if state file does not exist but `config.yaml` has sessions, migrate them as K8s-type environment records. Remove sessions from config.yaml after successful migration.

**Rationale**: The existing `config.yaml` has a `Sessions []Session` field tracking K8s deployments. These should appear in the new `state.yaml` as K8s-type environments. One-time migration on first access prevents data loss.

**Alternatives considered**:
- Manual migration: Poor UX, users would lose visibility of existing sessions.
- Keep sessions in both: Creates inconsistency.

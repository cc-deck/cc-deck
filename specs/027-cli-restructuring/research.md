# Research: CLI Command Restructuring

**Feature**: 027-cli-restructuring
**Date**: 2026-03-22

## Decision 1: Cobra Command Group Mechanism

**Decision**: Use `rootCmd.AddGroup()` with insertion-order control for help output grouping.

**Rationale**: Cobra's `AddGroup()` already works at the root command level (no restriction to subcommands). Groups display in the order they are added via `AddGroup()`. The existing `env.go` already uses this pattern for env subcommand groups (lifecycle, info, data, maintenance). Commands not assigned to a group appear under a default "Additional Commands" section.

**Alternatives considered**:
- Custom help template: More control but higher maintenance cost and fragile across Cobra upgrades.
- No grouping (flat list): Current state, does not solve the discoverability problem.

## Decision 2: Sharing RunE Between Top-Level and env Subcommands

**Decision**: Create shared constructor functions that return fresh `*cobra.Command` instances. Both the top-level and env-level commands call the same constructor, getting independent Command objects that delegate to the same `runXxx` business logic functions.

**Rationale**: A single `*cobra.Command` cannot be added to two parents (Cobra uses parent references internally). The existing code already separates business logic into `runXxx` functions (e.g., `runEnvList`, `runEnvStatus`, `runEnvAttach`), so the RunE closures are thin wrappers. Creating two Command objects from a shared constructor is clean and idiomatic.

**Pattern**:
```
newAttachCmdCore(gf) → *cobra.Command  (shared constructor, registers flags)
newEnvAttachCmd(gf)  → calls newAttachCmdCore(gf)  (added to env)
NewAttachCmd(gf)     → calls newAttachCmdCore(gf)  (added to root)
```

**Alternatives considered**:
- Thin wrapper commands that manually invoke env subcommands: Adds indirection, flag registration duplication, and potential for drift.
- Using Cobra aliases at root level: Cobra aliases only work within a single command (e.g., `ls` as alias for `list`), not cross-command.

## Decision 3: Legacy K8s Command Removal Scope

**Decision**: Remove 25 files across 4 packages: 6 command files, 6 session K8s files, 8 k8s package files, 2 sync package files, 2 integration test files. Modify `profile.go` to remove K8s Secret validation. Remove `k8s.io/client-go` from go.mod.

**Rationale**: All K8s-specific code is cleanly isolated. The `internal/k8s/`, `internal/sync/`, and K8s-facing `internal/session/` functions are only used by the legacy commands. The `internal/session/` package retains non-K8s functions (autosave, snapshot, save, restore). The `profile.go` command currently validates K8s Secrets, which must be removed or made optional.

**Alternatives considered**:
- Keep K8s packages as dead code for future use: Violates YAGNI. When K8s env types are implemented, the code will be redesigned for the unified env interface.
- Deprecation period with warnings: Unnecessary since K8s commands are not in active use (pre-release project).

## Decision 4: Profile Command K8s Dependency

**Decision**: Remove K8s Secret validation from `profile.go`. Profile management (add, list, use, show) remains functional for credential file management without K8s validation.

**Rationale**: Profile credentials are used by container environments (podman secrets) and will be used by future K8s env types. The K8s Secret validation was specific to the legacy deploy workflow. When K8s env types are implemented, validation will be handled by the env interface.

## Decision 5: Global Flags Retention

**Decision**: Keep `--kubeconfig` and `--namespace` global flags on the root command even after removing K8s commands.

**Rationale**: These flags will be needed when K8s environment types are implemented in the env system. Removing and re-adding them creates churn. They are harmless when unused.

## Decision 6: Help Group Naming and Order

**Decision**: Four groups in this display order: "Daily" (attach, list, status, start, stop, logs), "Session" (snapshot), "Environment" (env), "Setup" (plugin, profile, domains, image). Utility commands (hook, version, completion) remain ungrouped.

**Rationale**: "Daily" first because those are the most frequently used commands. "Session" second as it is used regularly but less often. "Environment" third for the full env namespace. "Setup" last as it is one-time configuration. This order matches the user's mental model of frequency.

# Research: 039-cli-rename-ws-build

**Date**: 2026-04-18
**Method**: Parallel agent research (4 agents)

## Decision Summary

| Decision | Rationale | Alternatives Considered |
|----------|-----------|------------------------|
| Use Cobra `Aliases` for `ws`/`workspace` | Already used in 3 places in codebase; simplest approach | Separate command registration (rejected: duplication) |
| Extract `newExecCmdCore` for promotion | Consistent with existing promotion pattern (attach, list use `*CmdCore`) | Direct duplication (rejected: violates DRY) |
| Move `completion` from main.go to `internal/cmd/` | Needed for `config` parent grouping; consistent with plugin/profile/domains | Keep in main.go with wrapper (rejected: inconsistent) |
| No backward-compat shims for old names | Pre-1.0, small user base, Cobra's "Did you mean?" is sufficient | Hidden aliases (rejected per clarification Q1) |
| Rename `internal/setup/` package to `internal/build/` | Aligns with CLI rename; manifest filename changes too | Keep internal name (rejected: confusing divergence) |
| Rename `cc-deck-setup.yaml` to `cc-deck-build.yaml` | User-facing manifest file should match CLI command | Keep old filename (rejected: inconsistent) |

## Research Findings

### 1. Command Registration Structure

**Current groups** (main.go:47-52):
- `daily` (promoted): attach, list, status, start, stop, logs
- `session`: snapshot
- `environment`: env (parent)
- `setup`: plugin, profile, domains, setup (parent)
- Ungrouped: hook (hidden), version, completion

**Target groups**:
- `workspace` (promoted): attach, ls, exec
- `session`: snapshot
- `build`: build (parent)
- `config`: config (parent for plugin, profile, domains, completion)
- Ungrouped: hook (hidden), version

**Promotion pattern** (env_promote.go): Thin exported wrappers call `*CmdCore` factory functions. Both env subcommand and top-level command get independent `*cobra.Command` instances from same core. Key constraint: `exec` has no CmdCore function yet, needs extraction.

**Key files to modify**:
- `cc-deck/cmd/cc-deck/main.go` (group definitions, command registration)
- `cc-deck/internal/cmd/env.go` (1833 lines, rename to ws.go)
- `cc-deck/internal/cmd/env_promote.go` (rename to ws_promote.go, change exports)
- `cc-deck/internal/cmd/setup.go` (rename to build.go)
- New: `cc-deck/internal/cmd/config.go`

### 2. Test Impact

**8 test files need renaming and content updates (~150+ string replacements)**:

| Current File | New File | Impact |
|-------------|----------|--------|
| `env_promote_test.go` | `ws_promote_test.go` | CRITICAL: group names, constructor names, command assertions |
| `env_integration_test.go` | `ws_integration_test.go` | CRITICAL: ~30 `"env"` string args |
| `compose_smoke_test.go` | (keep name) | CRITICAL: ~20 `"env"` string args, builds binary |
| `e2e/env_test.go` | `e2e/ws_test.go` | CRITICAL: ~25 `"env"` string args |
| `env_create_test.go` | `ws_new_test.go` | HIGH: function names, ~12 create references |
| `env_prune_test.go` | `ws_prune_test.go` | MEDIUM: 3 call sites |
| `env_resolve_test.go` | `ws_resolve_test.go` | MEDIUM: 3 call sites |
| `setup_test.go` | `build_test.go` | HIGH: function references |

**Internal package tests** (`internal/setup/init_test.go`, `internal/setup/manifest_test.go`): 7 references to `cc-deck-setup.yaml` filename.

### 3. Documentation Impact

**~353+ references across 14 files**:

| Category | Files | References |
|----------|-------|------------|
| CLI reference (`cli.adoc`) | 1 | ~79 |
| README | 1 | ~28 |
| Antora guides (running/) | 5 | ~96 |
| Antora guides (using/) | 1 | ~24 |
| Walkthroughs | 3 | ~120+ |
| Legacy quickstart | 1 | ~6 |

**Key renames in docs**: `cc-deck env` → `cc-deck ws`, `env create` → `ws new`, `env delete` → `ws delete`, `cc-deck setup` → `cc-deck build`, section headings ("Environment Management" → "Workspace Management").

### 4. Cobra Patterns Available

| Pattern | Already Used | Location |
|---------|-------------|----------|
| `Aliases` field | Yes | env.go:869 (`ls`), snapshot.go:85 (`rm`), plugin.go:90 (`uninstall`) |
| `Hidden: true` | Yes | hook.go:57 |
| `AddGroup`/`GroupID` | Yes | main.go:47, env.go:45 |
| `addToGroup` helper | Yes (duplicated) | main.go:141, env.go:1794 |

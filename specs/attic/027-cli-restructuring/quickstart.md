# Quickstart: CLI Command Restructuring

**Feature**: 027-cli-restructuring
**Date**: 2026-03-22

## Implementation Phases

### Phase 1: Remove Legacy K8s Commands

Remove the six legacy top-level commands and their backing packages. This is the safest starting point because it reduces the codebase before adding new structure.

1. Remove command files: `deploy.go`, `connect.go`, `list.go`, `delete.go`, `logs.go`, `sync.go`
2. Remove command registrations from `main.go`
3. Remove backing packages: `internal/k8s/` (entire directory), `internal/sync/` (entire directory)
4. Remove K8s-specific session functions: `session/deploy.go`, `session/connect.go`, `session/list.go`, `session/delete.go`, `session/logs.go`, `session/validate.go`
5. Update `profile.go` to remove K8s Secret validation
6. Remove `internal/integration/` test directory
7. Run `go mod tidy` to clean up unused dependencies (removes `k8s.io/client-go`)
8. Verify: `make test` and `make lint` pass

### Phase 2: Promote Commands to Top Level

Extract shared command constructors and register them at both root and env levels.

1. For each of the six commands (attach, list, status, start, stop, logs):
   - Extract a `newXxxCmdCore(gf)` shared constructor from the existing `newEnvXxxCmd(gf)`
   - `newEnvXxxCmd(gf)` calls `newXxxCmdCore(gf)` (env path)
   - Create exported `NewXxxCmd(gf)` that calls `newXxxCmdCore(gf)` (root path)
2. Register the six new top-level commands in `main.go`
3. Verify: Both `cc-deck attach` and `cc-deck env attach` work identically

### Phase 3: Add Help Output Groups

Organize commands into named groups in the root help output.

1. Define four groups on the root command: Daily, Session, Environment, Setup
2. Assign each command to its group via `GroupID`
3. Verify: `cc-deck --help` shows commands under correct group headings

### Phase 4: Tests and Documentation

1. Add tests verifying promoted commands exist and produce expected output
2. Add tests verifying help output contains correct groups
3. Add tests verifying removed commands are gone
4. Update README.md with new command structure
5. Update CLI reference documentation (Antora)

## Verification

After all phases:
- `make test` passes
- `make lint` passes
- `cc-deck --help` shows four command groups
- `cc-deck attach` and `cc-deck env attach` behave identically
- `cc-deck deploy` returns "unknown command" error
- Shell completion includes both top-level and env subcommands

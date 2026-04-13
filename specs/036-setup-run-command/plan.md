# Implementation Plan: cc-deck setup run

**Branch**: `036-setup-run-command` | **Date**: 2026-04-13 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/036-setup-run-command/spec.md`

## Summary

Add `cc-deck setup run` subcommand that executes pre-generated build artifacts directly from the CLI, without Claude Code involvement. The command auto-detects whether to run a container build (via podman/docker) or SSH provisioning (via ansible-playbook) based on the artifacts present in the setup directory.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), os/exec (stdlib), existing `internal/setup` package
**Storage**: N/A (stateless, reads manifest only)
**Testing**: `go test` via `make test`
**Target Platform**: CLI tool (Linux, macOS)
**Project Type**: CLI
**Performance Goals**: N/A (thin wrapper around external tools)
**Constraints**: Must use `make install` / `make test` / `make lint` (constitution Principle VI)
**Scale/Scope**: Single new subcommand, ~150-200 lines of Go code

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | PASS | Go CLI only, no plugin changes |
| II. Plugin Installation | N/A | No plugin changes |
| III. WASM Filename | N/A | No WASM changes |
| IV. WASM Host Function Gating | N/A | No WASM changes |
| V. Zellij API Research | N/A | No Zellij API usage |
| VI. Build via Makefile | PASS | Will use `make test`, `make lint`, `make install` |
| VII. Interface Behavioral Contracts | N/A | No new interface implementations |
| VIII. Simplicity | PASS | Reuses existing code, no new abstractions |
| IX. Documentation Freshness | REQUIRED | Must update README, CLI reference |
| X. Spec Tracking in README | REQUIRED | Add spec 036 to table |
| XI. Release Process | N/A | No release steps |
| XII. Prose Plugin | REQUIRED | For documentation content |
| XIII. XDG Paths | N/A | No new paths, uses existing `resolveSetupDir()` |
| XIV. No Dotfile Nesting | N/A | No new dotfiles |

## Project Structure

### Documentation (this feature)

```text
specs/036-setup-run-command/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── cli-commands.md  # CLI contract
└── tasks.md             # Phase 2 output (from /speckit-tasks)
```

### Source Code (repository root)

```text
cc-deck/internal/cmd/setup.go       # Add newSetupRunCmd(), wire into NewSetupCmd
cc-deck/internal/cmd/setup_test.go   # Unit tests for auto-detection and flag validation
```

**Structure Decision**: All new code goes into the existing `setup.go` file following the established pattern where all setup subcommands live in a single file. The run command logic is simple enough (auto-detect, exec, stream) that a separate `internal/setup/run.go` file would be over-abstraction.

## Implementation Phases

### Phase 1: Core Command (FR-001, FR-002, FR-003, FR-004, FR-005, FR-009, FR-010)

Add `newSetupRunCmd()` to `internal/cmd/setup.go` with:

1. **Command registration**: Wire into `NewSetupCmd()` alongside init/verify/diff
2. **Target auto-detection**: Check for `Containerfile` and `site.yml`+`inventory.ini` in setup dir
3. **Container execution**: `<runtime> build -t <imageRef> -f Containerfile .` with stdout/stderr piped to terminal
4. **SSH execution**: `ansible-playbook -i inventory.ini site.yml` with stdout/stderr piped to terminal
5. **Exit code passthrough**: Return build tool's exit code

Key implementation details:
- Use `exec.Command()` with `Cmd.Stdout = os.Stdout`, `Cmd.Stderr = os.Stderr`, `Cmd.Stdin = os.Stdin`
- Set `Cmd.Dir` to the setup directory so relative paths in Containerfile/playbook work
- Extract exit code from `exec.ExitError` for passthrough

### Phase 2: Push Support (FR-006, FR-007, FR-008)

Add `--push` flag handling:

1. **Flag definition**: `cmd.Flags().BoolVar(&push, "push", false, "Push image after build (container only)")`
2. **Validation**: Reject `--push` with SSH target, require `targets.container.registry` in manifest
3. **Push execution**: After successful build, run `<runtime> push <registry>/<name>:<tag>`

### Phase 3: Tests

Unit tests in `internal/cmd/setup_test.go`:

1. **Auto-detection logic**: Test all four artifact combinations (container only, ssh only, both, neither)
2. **Flag validation**: Test `--push` rejection with SSH, `--push` without registry
3. **Image reference construction**: Test push reference format `registry/name:tag`

### Phase 4: Documentation

1. **README.md**: Add spec 036 to feature table, update setup workflow section
2. **CLI reference** (`docs/modules/reference/pages/cli.adoc`): Add `setup run` command with flags and examples
3. Use prose plugin for all documentation content

## Complexity Tracking

No constitution violations. No complexity justifications needed.

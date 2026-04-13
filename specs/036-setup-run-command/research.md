# Research: cc-deck setup run

**Feature Branch**: `036-setup-run-command`
**Date**: 2026-04-13

## Existing Code Patterns

### Decision: Reuse existing setup infrastructure

**Rationale**: All required building blocks already exist in the codebase:

- `setup.LoadManifest()` at `internal/setup/manifest.go:112` parses `cc-deck-setup.yaml`
- `setup.DetectRuntime()` at `internal/setup/runtime.go:10` finds podman/docker
- `Manifest.ImageRef()` at `internal/setup/manifest.go:176` constructs `name:tag`
- `resolveSetupDir()` at `internal/cmd/setup.go:270` resolves the setup directory
- `fileExists()` at `internal/cmd/setup.go:468` checks for artifact presence

**Alternatives considered**: None. Creating new abstractions would violate constitution Principle VIII (Simplicity/YAGNI).

### Decision: Command registration follows existing pattern

**Rationale**: The `newSetupRunCmd()` function follows the same pattern as `newSetupInitCmd()`, `newSetupVerifyCmd()`, and `newSetupDiffCmd()` in `internal/cmd/setup.go`. All use cobra with `MaximumNArgs(1)` for optional dir argument and string flags for target selection.

### Decision: Use os/exec with Cmd.Stdin/Stdout/Stderr piped to os.Std*

**Rationale**: The spec requires real-time streaming of build output (FR-004). Unlike `runContainerVerify()` which captures output via `CombinedOutput()`, the run command must pipe directly to the terminal. The standard Go pattern is:

```go
cmd := exec.Command(runtime, args...)
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
cmd.Stdin = os.Stdin
```

This also naturally preserves exit codes via `cmd.Run()` and `exec.ExitError` (FR-005).

**Alternatives considered**: Using `CombinedOutput()` and printing after completion. Rejected because it violates FR-004 (real-time streaming) and would buffer potentially large build output in memory.

### Decision: Target auto-detection by artifact presence

**Rationale**: The `runDiff()` function at `internal/cmd/setup.go:302` already implements this exact pattern: check for `Containerfile` and `roles/` directory to determine target type. The run command uses the same detection but checks for `Containerfile` and `site.yml` + `inventory.ini` (the Ansible entry points rather than the role directories).

### Decision: Ansible detection uses site.yml + inventory.ini

**Rationale**: While `runDiff()` checks for the `roles/` directory, the run command checks for `site.yml` and `inventory.ini` because these are the actual execution entry points that `ansible-playbook` needs. A `roles/` directory could exist from `setup init` scaffolding without build artifacts being generated yet.

## Ansible Prerequisites

### Decision: Check ansible-playbook via exec.LookPath

**Rationale**: Same pattern as `DetectRuntime()`. If not found, the error message includes install instructions: `"ansible-playbook not found in PATH; install with: pip install ansible"`.

## Push Workflow

### Decision: Sequential build-then-push

**Rationale**: The `--push` flag triggers a two-step workflow: build first, then push only on success. The push image reference is `<registry>/<name>:<tag>` constructed from manifest fields. This is the standard container workflow and matches what users expect from tools like `podman build && podman push`.

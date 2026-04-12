# 035: `cc-deck setup run` Command

## Summary

Add `cc-deck setup run` subcommand that executes pre-generated build artifacts (Containerfile or Ansible playbooks) directly, without Claude Code involvement. This is the "execute" step after `/cc-deck.build` has generated the artifacts.

## Motivation

Currently, the entire build workflow goes through the `/cc-deck.build` Claude Code command, which both generates artifacts and executes them. Users need a way to re-run builds directly (e.g., after fixing a shell config issue) without invoking Claude Code again.

## CLI Interface

```
cc-deck setup run [dir] [--target container|ssh] [--push]
```

### Arguments

- **`dir`** (optional): Setup directory path. Auto-resolved using the existing `resolveSetupDir()` logic (walks up from current directory looking for `.cc-deck/setup/`).

### Flags

- **`--target`** (optional): Force `container` or `ssh`. Auto-detected from artifacts if omitted.
- **`--push`** (optional): Container target only. Push the built image to the registry after a successful build. Requires `targets.container.registry` in the manifest.

## Target Auto-Detection

When `--target` is not specified, detect from generated artifacts in the setup directory:

| Artifacts found | Detected target |
|---|---|
| `Containerfile` only | `container` |
| `site.yml` + `inventory.ini` only | `ssh` |
| Both present | Error: `--target` flag required to disambiguate |
| Neither present | Error: no build artifacts found, run `/cc-deck.build` first |

## Container Target

1. Load manifest (`cc-deck-setup.yaml`) to read `targets.container.name` and `targets.container.tag` for the image reference.
2. Detect container runtime via existing `setup.DetectRuntime()` (prefers podman, falls back to docker).
3. Execute: `<runtime> build -t <name>:<tag> -f Containerfile .` from the setup directory.
4. Stream stdout/stderr to the terminal in real time.
5. If `--push`:
   - Validate that `targets.container.registry` is set in the manifest. Error if missing.
   - Execute: `<runtime> push <registry>/<name>:<tag>`.

## SSH Target

1. Verify `ansible-playbook` is on PATH. Error with install instructions if missing.
2. Execute: `ansible-playbook -i inventory.ini site.yml` from the setup directory.
3. Stream stdout/stderr to the terminal in real time.

## Error Handling

- Non-zero exit codes from the build tool (podman, ansible-playbook) are passed through as the cc-deck exit code.
- No automatic retry loops. Retries with self-correction remain Claude Code's responsibility during `/cc-deck.build`.
- Missing prerequisites produce clear error messages:
  - No container runtime: "neither podman nor docker found in PATH"
  - No ansible-playbook: "ansible-playbook not found in PATH; install with: pip install ansible"
  - Missing artifacts: "no build artifacts found in <dir>; run /cc-deck.build to generate them"
  - `--push` without registry: "targets.container.registry not set in manifest"

## Implementation

### Files

- `cc-deck/internal/cmd/setup.go`: Add `newSetupRunCmd()`, wire into setup parent command.
- `cc-deck/internal/setup/run.go` (new): Core logic for target detection, container build, SSH playbook execution.
- `cc-deck/internal/setup/run_test.go` (new): Unit tests for auto-detection logic and flag validation.

### Dependencies

Uses existing code only:
- `setup.LoadManifest()` for reading the manifest
- `setup.DetectRuntime()` for container runtime detection
- `resolveSetupDir()` for directory resolution
- `os/exec` with stdout/stderr piped to terminal

### Integration with Existing Commands

The `setup run` command slots into the existing workflow:

```
cc-deck setup init      # scaffold setup directory
/cc-deck.capture        # discover tools and config (Claude Code)
/cc-deck.build          # generate artifacts (Claude Code)
cc-deck setup run       # execute the build (direct CLI)
cc-deck setup verify    # smoke-test the result
cc-deck setup diff      # check for manifest drift
```

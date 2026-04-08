# SSH Environment Behavioral Contract

**Date**: 2026-04-07 | **Extends**: Environment Interface Contract

This contract documents the behavioral requirements for the SSH environment type beyond what the method signatures specify.

## Identity

- `Type()` returns `EnvironmentTypeSSH` (`"ssh"`)
- `Name()` returns the user-provided environment name

## Create

1. MUST call `ValidateEnvName(name)` before any resource operations
2. MUST check for name conflicts via `store.FindInstanceByName(name)`
3. MUST verify `ssh` binary is available locally via `exec.LookPath("ssh")`
4. MUST load definition from DefinitionStore to get SSH connection parameters
5. MUST run pre-flight checks in order:
   a. SSH connectivity test
   b. Remote OS/architecture detection
   c. Zellij availability (offer install if missing)
   d. Claude Code availability (offer install if missing)
   e. cc-deck CLI availability (offer install if missing)
   f. cc-deck plugin status (offer install if missing)
   g. Credential verification
6. For each missing tool on a supported platform: MUST offer interactive installation
7. For unsupported platforms: MUST warn and skip installation offers
8. On success: MUST add `EnvironmentInstance` with `SSH` fields and state `running`
9. On failure: MUST NOT leave partial state records

## Attach

1. MUST check `$ZELLIJ` env var; if set, print warning and return nil (no error)
2. MUST load definition to get current SSH connection parameters
3. MUST write credentials to remote file before attaching (unless auth=none)
4. MUST update `LastAttached` timestamp in state store
5. MUST check if remote Zellij session `cc-deck-<name>` exists
6. If session does not exist: MUST create it with `--layout cc-deck`
7. MUST use `syscall.Exec()` to replace the local process with SSH connection
8. The user MUST interact directly with the remote Zellij session

## Status

1. MUST query the remote machine via SSH (no caching)
2. MUST check if Zellij session `cc-deck-<name>` exists on remote
3. If remote is unreachable: MUST return state=error with diagnostic message
4. If session exists: MUST return state=running with session details
5. If session not found: MUST return state=stopped
6. MUST respect per-host timeout to prevent hanging on unreachable hosts

## Start / Stop

1. MUST return `ErrNotSupported` (SSH environments have externally managed lifecycle)

## Delete

1. MUST refuse deletion of running environments unless `force=true` (return `ErrRunning`)
2. With `force=true`: MUST attempt to kill remote Zellij session (best-effort)
3. MUST remove `EnvironmentInstance` from state store
4. Cleanup failures: log as warnings, do not return errors

## Exec

1. MUST run commands on remote via SSH in the configured workspace directory
2. MUST return stdout/stderr to the caller
3. MUST use `exec.CommandContext()` with timeout for non-interactive execution

## Push / Pull

1. MUST use rsync over SSH for file transfer
2. MUST respect configured exclusion patterns
3. If rsync unavailable on remote: MUST fall back to scp with warning
4. MUST use the configured workspace as the remote target directory

## Harvest

1. MUST add a temporary git remote pointing to remote workspace via SSH
2. MUST `git fetch` from the temporary remote
3. MUST remove the temporary remote after fetch
4. If `CreatePR` is true: MUST create a PR from the fetched branch

## Credential Refresh (SSH-specific, not part of Environment interface)

1. MUST write fresh credentials to remote credential file without attaching
2. MUST support all auth modes (auto, api, vertex, bedrock)
3. For auth=none: MUST report that credential management is disabled
4. MUST NOT disrupt the running remote Zellij session

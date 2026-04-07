# Data Model: SSH Remote Execution Environment

**Branch**: `033-ssh-environment` | **Date**: 2026-04-07

## Entities

### SSHFields (Runtime State)

Stored in `EnvironmentInstance.SSH` within `state.yaml`.

| Field | Type | YAML Tag | Description |
|-------|------|----------|-------------|
| Host | string | `host` | SSH target in user@host format |
| Port | int | `port,omitempty` | SSH port (0 means use default/config) |
| IdentityFile | string | `identity_file,omitempty` | Path to SSH private key |
| JumpHost | string | `jump_host,omitempty` | Bastion/jump host |
| SSHConfig | string | `ssh_config,omitempty` | Custom SSH config file path |
| Workspace | string | `workspace,omitempty` | Remote working directory |

### EnvironmentDefinition Extensions

New fields added to `EnvironmentDefinition` for SSH type:

| Field | Type | YAML Tag | Description |
|-------|------|----------|-------------|
| Host | string | `host,omitempty` | SSH target (user@host) |
| Port | int | `port,omitempty` | SSH port |
| IdentityFile | string | `identity-file,omitempty` | Path to private key |
| JumpHost | string | `jump-host,omitempty` | Bastion host |
| SSHConfig | string | `ssh-config,omitempty` | Custom SSH config |
| Workspace | string | `workspace,omitempty` | Remote workspace dir (default: ~/workspace) |

Note: `Auth`, `Credentials`, `Env` fields already exist on `EnvironmentDefinition` and are reused.

### SSH Client

Internal helper (not persisted). Wraps SSH operations for a specific remote host.

| Field | Type | Description |
|-------|------|-------------|
| Host | string | user@host target |
| Port | int | SSH port (0 = default) |
| IdentityFile | string | Private key path |
| JumpHost | string | Bastion host |
| SSHConfig | string | Custom config path |

### PreflightCheck

Interface for pre-flight verification steps during Create.

| Method | Signature | Description |
|--------|-----------|-------------|
| Name | `() string` | Human-readable check name |
| Run | `(ctx) error` | Execute the check |
| HasRemedy | `() bool` | Whether auto-fix is available |
| Remedy | `(ctx) error` | Attempt automated fix |
| ManualInstructions | `() string` | Instructions for manual fix |

### Credential File

Written to remote at `~/.config/cc-deck/credentials.env` (mode 600).

Format: Shell-sourceable env file:
```
export ANTHROPIC_API_KEY="sk-..."
export CLAUDE_CODE_USE_VERTEX="1"
```

## State Transitions

```
[not exists] --Create--> running
running --Delete(force)--> [removed]
running --Detach--> running (session persists)
running --Reboot--> stopped (session lost)
stopped --Attach--> running (new session created)
[any] --Status--> [queried live, no state change]
```

Note: SSH environments do not support Start/Stop (returns ErrNotSupported). The `running` state means the state record exists and was created successfully. Actual session status is always queried live via SSH.

## Relationships

```
EnvironmentDefinition (environments.yaml)
  └── defines SSH connection parameters
  └── references auth mode, credentials, env vars

EnvironmentInstance (state.yaml)
  └── contains SSHFields (runtime snapshot)
  └── tracks CreatedAt, LastAttached, State

SSHEnvironment (runtime)
  ├── reads EnvironmentDefinition for config
  ├── reads/writes EnvironmentInstance for state
  └── uses SSHClient for all remote operations

SSHClient (runtime)
  └── wraps system ssh binary
  └── used by SSHEnvironment for Run, Attach, Upload, Download
```

## Validation Rules

- Environment name: `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, max 40 chars (existing)
- Host field: Required, must contain `@` or be a hostname (validated during Create)
- Port: 0-65535 (0 means use SSH defaults)
- Workspace: Defaults to `~/workspace` if empty
- Auth mode: Must be one of: auto, api, vertex, bedrock, none
- Zellij session name: `cc-deck-<envname>` (derived, not user-configurable)

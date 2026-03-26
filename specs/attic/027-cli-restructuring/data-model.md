# Data Model: CLI Command Restructuring

**Feature**: 027-cli-restructuring
**Date**: 2026-03-22

## Entities

This feature does not introduce new data entities, persistent state, or storage changes. It restructures the command hierarchy (an in-memory concern) and modifies help output formatting.

### Command Group (conceptual, not persisted)

A grouping label applied to commands for help output organization.

| Attribute | Description |
|-----------|-------------|
| ID        | Internal identifier (e.g., "daily", "session", "env", "setup") |
| Title     | Display label in help output (e.g., "Daily:", "Session:", "Environment:", "Setup:") |
| Commands  | Ordered list of commands assigned to this group |

### Promoted Command Mapping (conceptual)

| Top-Level Command | env Subcommand | Shared Constructor |
|-------------------|----------------|-------------------|
| `attach`          | `env attach`   | `newAttachCmdCore` |
| `list` (alias `ls`) | `env list`  | `newListCmdCore`   |
| `status`          | `env status`   | `newStatusCmdCore` |
| `start`           | `env start`    | `newStartCmdCore`  |
| `stop`            | `env stop`     | `newStopCmdCore`   |
| `logs`            | `env logs`     | `newLogsCmdCore`   |

### Removed Commands

| Command   | Backing Package Functions | Files |
|-----------|--------------------------|-------|
| `deploy`  | `session.Deploy()`       | `cmd/deploy.go`, `session/deploy.go` |
| `connect` | `session.Connect()`      | `cmd/connect.go`, `session/connect.go` |
| `list`    | `session.List()`         | `cmd/list.go`, `session/list.go` |
| `delete`  | `session.Delete()`       | `cmd/delete.go`, `session/delete.go` |
| `logs`    | `session.Logs()`         | `cmd/logs.go`, `session/logs.go` |
| `sync`    | `sync.Sync()`            | `cmd/sync.go`, `sync/sync.go` |

## State Changes

None. No configuration files, state files, or persistent data are modified by this feature.

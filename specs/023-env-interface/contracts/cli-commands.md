# Contract: CLI Commands

**Feature**: 023-env-interface | **Date**: 2026-03-20

## Command Tree

```
cc-deck env
  create <name> --type <local|podman|k8s|sandbox> [type-specific flags]
  attach <name>
  start <name>
  stop <name>
  delete <name> [--force]
  list [--type <type>] [-o json|yaml]
  status <name> [-o json|yaml]
  exec <name> -- <cmd...>       (stub: "not yet implemented")
  push <name> [local-path]      (stub: "not yet implemented")
  pull <name> [remote-path]     (stub: "not yet implemented")
  harvest <name> [-b branch]    (stub: "not yet implemented")
  logs <name>                   (stub: "not yet implemented")
```

## Backward-Compatible Aliases

Existing top-level commands become hidden aliases:

| Old command | Delegates to | Notes |
|-------------|-------------|-------|
| `cc-deck deploy <name>` | `cc-deck env create <name> --type k8s` | Maps deploy flags to env create flags |
| `cc-deck connect <name>` | `cc-deck env attach <name>` | |
| `cc-deck delete <name>` | `cc-deck env delete <name>` | |
| `cc-deck list` | `cc-deck env list` | |
| `cc-deck logs <name>` | `cc-deck env logs <name>` | |

All aliases remain `Hidden: true` in cobra.

## Output Formats

### `env list` (table, default)

```
NAME            TYPE      STATUS    STORAGE     LAST ATTACHED    AGE
mydev           local     running   host        5m ago           3d
my-project      podman    running   volume      2h ago           1d
backend-work    k8s       running   pvc/10Gi    30m ago          5d
```

### `env list -o json`

```json
[
  {
    "name": "mydev",
    "type": "local",
    "state": "running",
    "storage": "host",
    "last_attached": "2026-03-20T15:30:00Z",
    "age": "3d",
    "created_at": "2026-03-17T10:00:00Z"
  }
]
```

### `env status <name>` (table, default)

```
Environment: mydev
Type:        local
Status:      Running
Storage:     Host filesystem
Uptime:      3d 5h
Attached:    5m ago

Agent Sessions:
  NAME              STATUS        BRANCH          LAST EVENT
  api-refactor      Permission    feat/api-v2     2m ago
  docs-update       Working       docs/quickstart 1m ago
```

### `env status <name> -o json`

```json
{
  "name": "mydev",
  "type": "local",
  "state": "running",
  "storage": {"type": "host-path"},
  "created_at": "2026-03-17T10:00:00Z",
  "last_attached": "2026-03-20T15:30:00Z",
  "sessions": [
    {
      "name": "api-refactor",
      "activity": "Permission",
      "branch": "feat/api-v2",
      "last_event": "2026-03-20T15:33:00Z"
    }
  ]
}
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 5 | Resource conflict (name already exists) |

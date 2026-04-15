# Data Model: Environment Lifecycle Fixes

**Branch**: `037-env-lifecycle-fixes` | **Date**: 2026-04-14

## Changed Entities

### envListEntry (structured output for `cc-deck ls`)

**File**: `cc-deck/internal/cmd/env.go`

| Field | Type | JSON/YAML key | Change |
|-------|------|---------------|--------|
| Name | string | `name` | Existing |
| Type | string | `type` | Existing |
| State | string | `state` | Existing |
| Storage | string | `storage` | Existing |
| Image | string | `image` | Existing |
| LastAttached | string | `last_attached` | Existing |
| Age | string | `age` | Existing |
| **Source** | **string** | **`source`** | **New**: values `global`, `project`, or empty |

### envStatusOutput (structured output for `cc-deck status`)

**File**: `cc-deck/internal/cmd/env.go`

| Field | Type | JSON/YAML key | Change |
|-------|------|---------------|--------|
| Name | string | `name` | Existing |
| Type | EnvironmentType | `type` | Existing |
| State | EnvironmentState | `state` | Existing |
| Storage | string | `storage` | Existing |
| Uptime | string | `uptime` | Existing |
| LastAttached | string | `last_attached` | Existing |
| Sessions | []SessionInfo | `sessions` | Existing |
| Image | string | `image` | Existing |
| **ProjectPath** | **string** | **`project_path`** | **New**: project directory path for project-local envs |

### createFlags (CLI flags for `env create`)

**File**: `cc-deck/internal/cmd/env.go`

| Field | Type | Flag | Change |
|-------|------|------|--------|
| **global** | **bool** | **`--global`** | **New**: force global definition resolution |
| **local** | **bool** | **`--local`** | **New**: force project-local definition resolution |

## Unchanged Entities

- `EnvironmentDefinition`: No schema changes
- `EnvironmentInstance`: No schema changes
- `StateFile`: No schema changes
- `ProjectEntry`: No schema changes
- `DefinitionStore`: No API changes (existing `FindByName`, `Remove` used as-is)

## State Transitions

No new state transitions. The lifecycle states (not created, running, stopped, error) remain unchanged. The changes affect how environments enter the lifecycle (type resolution at create time) and how they exit (definition cleanup at delete time).

# Contract: Environment Interface

**Feature**: 023-env-interface | **Date**: 2026-03-20
**Updated**: 2026-03-21 (added behavioral requirements)

## Go Interface

```go
package env

// Environment manages the lifecycle of a Zellij instance
// running in a specific execution context.
type Environment interface {
    // Identity
    Type() EnvironmentType
    Name() string

    // Lifecycle
    Create(ctx context.Context, opts CreateOpts) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Delete(ctx context.Context, force bool) error
    Status(ctx context.Context) (*EnvironmentStatus, error)

    // Interaction
    Attach(ctx context.Context) error
    Exec(ctx context.Context, cmd []string) error

    // Data transfer
    Push(ctx context.Context, opts SyncOpts) error
    Pull(ctx context.Context, opts SyncOpts) error
    Harvest(ctx context.Context, opts HarvestOpts) error
}
```

## Behavioral Requirements

All implementations MUST satisfy these behaviors regardless of environment type.
These are cross-cutting concerns that ensure a consistent user experience.

### Attach

1. **Nested Zellij check**: MUST detect if the host terminal is already inside Zellij (`$ZELLIJ` env var). If so, print a warning and return without attaching.
2. **Session creation with layout**: MUST create the Zellij session with `--layout cc-deck` if the session does not already exist. This ensures the cc-deck sidebar plugin is loaded. Use `--create-background` for initial creation, then attach separately.
3. **Auto-start**: If the environment is stopped, MUST start it before attaching (FR-018).
4. **Timestamp update**: MUST update `LastAttached` in the state store.
5. **Session naming**: The Zellij session name MUST follow the environment's naming convention (local: `cc-deck-<name>`, container: always `cc-deck` per container isolation).

### Create

1. **Name validation**: MUST call `ValidateEnvName()`.
2. **Tool availability**: MUST check that the required tool is available (Zellij for local, podman for container) and return the appropriate error.
3. **State recording**: MUST write to the state store on success.
4. **Cleanup on failure**: MUST clean up partially created resources if creation fails partway through.

### Delete

1. **Running check**: MUST refuse to delete a running environment unless `force` is true.
2. **Best-effort cleanup**: MUST attempt to clean up all associated resources. Partial cleanup failures MUST be reported as warnings (log), not errors (return).
3. **State removal**: MUST remove the record from the state store.

### Status

1. **Reconciliation**: MUST reconcile stored state against actual runtime state (Zellij sessions for local, podman inspect for container).
2. **Session info**: SHOULD populate session information for running environments when available.

## CreateOpts

```go
type CreateOpts struct {
    Image   string        // OCI image (remote environments only)
    Storage StorageConfig // Storage backend configuration
    Sync    SyncConfig    // Sync strategy configuration
}
```

Type-specific options are passed via type-specific extension structs
embedded in the concrete implementation, not in the interface.

## EnvironmentStatus

```go
type EnvironmentStatus struct {
    State       EnvironmentState
    Since       time.Time
    Message     string          // Error details if State == Error
    Sessions    []SessionInfo   // Agent sessions (populated for running envs)
}

type SessionInfo struct {
    Name     string
    Activity string    // "Working", "Permission", "Done", etc.
    Branch   string
    LastEvent time.Time
}
```

## SyncOpts

```go
type SyncOpts struct {
    LocalPath  string   // Local directory
    RemotePath string   // Remote directory (default: /workspace)
    Excludes   []string // Exclusion patterns
    UseGit     bool     // Use git strategy instead of copy
}
```

## HarvestOpts

```go
type HarvestOpts struct {
    Branch   string // Local branch name for harvested commits
    CreatePR bool   // Create PR after harvest
}
```

## State Store Interface

```go
// StateStore manages persistent environment records.
type StateStore interface {
    Load() (*StateFile, error)
    Save(state *StateFile) error
    FindByName(name string) (*EnvironmentRecord, error)
    Add(record *EnvironmentRecord) error
    Update(record *EnvironmentRecord) error
    Remove(name string) error
    List(filter *ListFilter) ([]*EnvironmentRecord, error)
}

type ListFilter struct {
    Type *EnvironmentType // Filter by type (nil = all)
}
```

## Factory

```go
// NewEnvironment creates an Environment implementation for the given type.
// Returns an error if the type is not yet implemented.
func NewEnvironment(envType EnvironmentType, name string, store StateStore) (Environment, error)
```

## Error Types

```go
var (
    ErrNotSupported     = errors.New("operation not supported for this environment type")
    ErrNotImplemented   = errors.New("environment type not yet implemented")
    ErrNameConflict     = errors.New("environment with this name already exists")
    ErrNotFound         = errors.New("environment not found")
    ErrInvalidName      = errors.New("invalid environment name")
    ErrZellijNotFound   = errors.New("zellij binary not found in PATH")
    ErrRunning          = errors.New("environment is running; use --force to delete")
)
```

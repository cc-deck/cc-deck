# Contract: Environment Interface

**Feature**: 023-env-interface | **Date**: 2026-03-20

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

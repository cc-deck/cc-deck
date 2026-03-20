package env

import (
	"context"
	"time"
)

// Environment is the core abstraction for all environment types.
// Each backend (local, podman, k8s-deploy, k8s-sandbox) implements
// this interface to provide a uniform management API.
type Environment interface {
	// Type returns the environment type identifier.
	Type() EnvironmentType

	// Name returns the human-readable environment name.
	Name() string

	// Create provisions a new environment with the given options.
	Create(ctx context.Context, opts CreateOpts) error

	// Start brings a stopped environment back to a running state.
	Start(ctx context.Context) error

	// Stop gracefully stops a running environment.
	Stop(ctx context.Context) error

	// Delete removes the environment and its resources.
	// If force is true, a running environment is stopped first.
	Delete(ctx context.Context, force bool) error

	// Status returns the current state and metadata for the environment.
	Status(ctx context.Context) (*EnvironmentStatus, error)

	// Attach opens an interactive session into the environment.
	Attach(ctx context.Context) error

	// Exec runs a command inside the environment.
	Exec(ctx context.Context, cmd []string) error

	// Push synchronizes local files into the environment.
	Push(ctx context.Context, opts SyncOpts) error

	// Pull synchronizes files from the environment to local storage.
	Pull(ctx context.Context, opts SyncOpts) error

	// Harvest extracts work products (e.g., git commits) from the environment.
	Harvest(ctx context.Context, opts HarvestOpts) error
}

// CreateOpts holds options for creating a new environment.
type CreateOpts struct {
	Image   string        `yaml:"image,omitempty"`
	Storage StorageConfig `yaml:"storage,omitempty"`
	Sync    SyncConfig    `yaml:"sync,omitempty"`
}

// SyncOpts holds options for push/pull file synchronization.
type SyncOpts struct {
	LocalPath  string   `yaml:"local_path,omitempty"`
	RemotePath string   `yaml:"remote_path,omitempty"`
	Excludes   []string `yaml:"excludes,omitempty"`
	UseGit     bool     `yaml:"use_git,omitempty"`
}

// HarvestOpts holds options for the harvest operation.
type HarvestOpts struct {
	Branch   string `yaml:"branch,omitempty"`
	CreatePR bool   `yaml:"create_pr,omitempty"`
}

// EnvironmentStatus represents the runtime status of an environment.
type EnvironmentStatus struct {
	State    EnvironmentState `json:"state"`
	Since    time.Time        `json:"since"`
	Message  string           `json:"message,omitempty"`
	Sessions []SessionInfo    `json:"sessions,omitempty"`
}

// SessionInfo describes a Claude Code session inside an environment.
type SessionInfo struct {
	Name      string    `json:"name"`
	Activity  string    `json:"activity"`
	Branch    string    `json:"branch"`
	LastEvent time.Time `json:"last_event"`
}

// ListFilter constrains which environments are returned by a list operation.
type ListFilter struct {
	Type *EnvironmentType `json:"type,omitempty"`
}

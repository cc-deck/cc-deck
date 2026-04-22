package ws

import (
	"context"
	"time"
)

// Workspace is the core abstraction for all workspace types.
// Each backend (local, podman, k8s-deploy, k8s-sandbox) implements
// this interface to provide a uniform management API.
type Workspace interface {
	// Type returns the workspace type identifier.
	Type() WorkspaceType

	// Name returns the human-readable workspace name.
	Name() string

	// Create provisions a new workspace with the given options.
	Create(ctx context.Context, opts CreateOpts) error

	// Start brings a stopped workspace back to a running state.
	Start(ctx context.Context) error

	// Stop gracefully stops a running workspace.
	Stop(ctx context.Context) error

	// Delete removes the workspace and its resources.
	// If force is true, a running workspace is stopped first.
	Delete(ctx context.Context, force bool) error

	// Status returns the current state and metadata for the workspace.
	Status(ctx context.Context) (*WorkspaceStatus, error)

	// Attach opens an interactive session into the workspace.
	Attach(ctx context.Context) error

	// Exec runs a command inside the workspace.
	Exec(ctx context.Context, cmd []string) error

	// ExecOutput runs a command inside the workspace and returns stdout.
	ExecOutput(ctx context.Context, cmd []string) (string, error)

	// Push synchronizes local files into the workspace.
	Push(ctx context.Context, opts SyncOpts) error

	// Pull synchronizes files from the workspace to local storage.
	Pull(ctx context.Context, opts SyncOpts) error

	// Harvest extracts work products (e.g., git commits) from the workspace.
	Harvest(ctx context.Context, opts HarvestOpts) error

	// PipeChannel returns the pipe channel for this workspace.
	PipeChannel(ctx context.Context) (PipeChannel, error)

	// DataChannel returns the data channel for this workspace.
	DataChannel(ctx context.Context) (DataChannel, error)

	// GitChannel returns the git channel for this workspace.
	GitChannel(ctx context.Context) (GitChannel, error)
}

// CreateOpts holds options for creating a new workspace.
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
	Path     string `yaml:"path,omitempty"`
}

// WorkspaceStatus represents the runtime status of a workspace.
type WorkspaceStatus struct {
	State    WorkspaceState `json:"state"`
	Since    time.Time        `json:"since"`
	Message  string           `json:"message,omitempty"`
	Sessions []SessionInfo    `json:"sessions,omitempty"`
}

// SessionInfo describes a Claude Code session inside a workspace.
type SessionInfo struct {
	Name      string    `json:"name"`
	Activity  string    `json:"activity"`
	Branch    string    `json:"branch"`
	LastEvent time.Time `json:"last_event"`
}

// ListFilter constrains which workspaces are returned by a list operation.
type ListFilter struct {
	Type *WorkspaceType `json:"type,omitempty"`
}

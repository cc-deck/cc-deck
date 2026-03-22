# Contract: Project Discovery

**Feature**: 026-project-local-config | **Date**: 2026-03-22

## Package: `cc-deck/internal/project`

### Functions

```go
package project

// FindGitRoot returns the git root directory for the given start path.
// Uses `git rev-parse --show-toplevel` for reliable detection.
// Returns the canonical (symlink-resolved) path.
// Returns an error if startDir is not inside a git repository.
func FindGitRoot(startDir string) (string, error)

// FindProjectConfig looks for .cc-deck/environment.yaml at the git root
// of the given start directory. Returns the project root path (git root)
// and nil error if found. Returns ErrNoProjectConfig if .cc-deck/ does
// not exist or has no environment.yaml.
func FindProjectConfig(startDir string) (projectRoot string, err error)

// CanonicalPath returns the symlink-resolved absolute path.
// Falls back to the original path if resolution fails.
func CanonicalPath(path string) string

// ProjectName returns the directory basename of the given path,
// suitable for use as a default environment name.
func ProjectName(projectRoot string) string
```

### Worktree Functions

```go
// WorktreeInfo describes a single git worktree.
type WorktreeInfo struct {
    Path   string // Absolute path to the worktree
    Branch string // Branch name (empty for detached HEAD)
    Bare   bool   // Whether this is a bare worktree
}

// ListWorktrees returns all worktrees for the git repository at gitRoot.
// Parses output of `git worktree list --porcelain`.
func ListWorktrees(gitRoot string) ([]WorktreeInfo, error)
```

### Error Types

```go
var (
    ErrNotGitRepo       = errors.New("not inside a git repository")
    ErrNoProjectConfig  = errors.New("no .cc-deck/environment.yaml found at git root")
)
```

### Behavioral Requirements

1. `FindGitRoot` MUST return canonical (symlink-resolved) paths.
2. `FindGitRoot` MUST correctly handle git worktrees (where `.git` is a file, not a directory).
3. `FindProjectConfig` MUST only check the git root, never intermediate directories.
4. `CanonicalPath` MUST be idempotent: calling it on an already-canonical path returns the same path.
5. `ListWorktrees` MUST handle the case where `git worktree list` is not available (old git versions) by returning an empty list without error.

## Package: `cc-deck/internal/env` (Extensions)

### ProjectStatusStore

```go
// ProjectStatusStore manages per-project status files at .cc-deck/status.yaml.
type ProjectStatusStore struct {
    projectRoot string
}

// NewProjectStatusStore creates a store for the given project root directory.
func NewProjectStatusStore(projectRoot string) *ProjectStatusStore

// Load reads the project status file. Returns empty status if file does not exist.
func (s *ProjectStatusStore) Load() (*ProjectStatusFile, error)

// Save writes the status file atomically (write-to-tmp + rename).
func (s *ProjectStatusStore) Save(status *ProjectStatusFile) error

// Remove deletes the status file.
func (s *ProjectStatusStore) Remove() error
```

### ProjectDefinitionLoader

```go
// LoadProjectDefinition reads the environment definition from
// .cc-deck/environment.yaml in the given project root.
// Returns ErrNoProjectConfig if the file does not exist.
func LoadProjectDefinition(projectRoot string) (*EnvironmentDefinition, error)

// SaveProjectDefinition writes an environment definition to
// .cc-deck/environment.yaml, creating the directory if needed.
func SaveProjectDefinition(projectRoot string, def *EnvironmentDefinition) error
```

### Registry Extensions to FileStateStore

```go
// RegisterProject adds or updates a project entry in the global registry.
// Uses canonical (symlink-resolved) paths. Updates last_seen if already registered.
func (s *FileStateStore) RegisterProject(path string) error

// UnregisterProject removes a project entry from the global registry.
func (s *FileStateStore) UnregisterProject(path string) error

// ListProjects returns all project entries from the global registry.
func (s *FileStateStore) ListProjects() ([]ProjectEntry, error)

// PruneStaleProjects removes entries whose paths no longer exist.
// Returns the count of removed entries.
func (s *FileStateStore) PruneStaleProjects() (int, error)
```

### Behavioral Requirements

1. `RegisterProject` MUST resolve symlinks before storing the path.
2. `RegisterProject` MUST update `last_seen` if the project is already registered.
3. `PruneStaleProjects` MUST use `os.Stat()` to check path existence.
4. `LoadProjectDefinition` MUST parse a bare `EnvironmentDefinition` (not wrapped in `DefinitionFile`).
5. `SaveProjectDefinition` MUST create `.cc-deck/` directory and `.cc-deck/.gitignore` if they do not exist.
6. Any environment operation (`Create`, `Start`, `Attach`) on a project-local environment MUST idempotently ensure `.cc-deck/.gitignore` exists with `status.yaml` and `run/` entries before proceeding (FR-030).

## CLI Contract: `cc-deck env prune`

```
Usage: cc-deck env prune
```

### Behavioral Requirements

1. MUST check each project registry entry for path existence.
2. MUST remove entries whose paths no longer exist.
3. MUST report the count of removed entries.
4. MUST be safe to run repeatedly (idempotent).

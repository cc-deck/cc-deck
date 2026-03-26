# Contract: internal/podman Package

**Feature**: 024-container-env | **Date**: 2026-03-20

## Package API

```go
package podman

import "context"

// --- Detection ---

// Available returns true if podman is installed and in PATH.
func Available() bool

// IsRootless returns true if podman is running in rootless mode.
func IsRootless(ctx context.Context) (bool, error)

// --- Container Lifecycle ---

// RunOpts configures a new container.
type RunOpts struct {
    Name    string            // Container name (required)
    Image   string            // OCI image reference (required)
    Volumes []string          // Volume mounts in "name:/path" or "/host:/container" format
    Secrets []SecretMount     // Secrets to inject
    Ports   []string          // Port mappings in "host:container" format
    AllPorts bool             // Publish all exposed ports (-P)
    Cmd     []string          // Command to run (default: ["sleep", "infinity"])
}

// SecretMount describes how a secret is injected into a container.
type SecretMount struct {
    Name   string // Podman secret name
    Target string // Environment variable name inside container
}

// Run creates and starts a new container. Returns the container ID.
func Run(ctx context.Context, opts RunOpts) (string, error)

// Start starts a stopped container by name or ID.
func Start(ctx context.Context, nameOrID string) error

// Stop stops a running container by name or ID.
func Stop(ctx context.Context, nameOrID string) error

// Remove removes a container. If force is true, stops it first.
func Remove(ctx context.Context, nameOrID string, force bool) error

// --- Inspection ---

// ContainerInfo holds the state of a container as reported by podman inspect.
type ContainerInfo struct {
    ID      string // Full container ID
    Name    string // Container name
    State   string // running, exited, stopped, paused
    Running bool   // True if container is actively running
}

// Inspect returns information about a container. Returns nil, nil if
// the container does not exist.
func Inspect(ctx context.Context, nameOrID string) (*ContainerInfo, error)

// --- Exec ---

// Exec runs a command inside a container. If interactive is true,
// stdin/stdout/stderr are connected to the terminal.
func Exec(ctx context.Context, nameOrID string, cmd []string, interactive bool) error

// --- File Transfer ---

// Cp copies files between host and container using podman cp.
// Source and destination use the format "[container:]path".
func Cp(ctx context.Context, src, dst string) error

// --- Volumes ---

// VolumeCreate creates a named volume. Returns nil if already exists.
func VolumeCreate(ctx context.Context, name string) error

// VolumeRemove removes a named volume.
func VolumeRemove(ctx context.Context, name string) error

// VolumeExists returns true if the named volume exists.
func VolumeExists(ctx context.Context, name string) bool

// --- Secrets ---

// SecretCreate creates a podman secret from the given data.
// If the secret already exists, it is replaced.
func SecretCreate(ctx context.Context, name string, data []byte) error

// SecretRemove removes a podman secret.
func SecretRemove(ctx context.Context, name string) error

// SecretExists returns true if the named secret exists.
func SecretExists(ctx context.Context, name string) bool
```

## Error Handling

All functions return descriptive errors wrapping the podman CLI output:

```go
// Example error format:
// "podman run: exit status 125 (Error: image not found)"

var ErrPodmanNotFound = errors.New("podman binary not found in PATH")
```

- Functions that check existence (Inspect, VolumeExists, SecretExists) return nil/false for missing resources, not errors.
- Remove operations are idempotent: removing a non-existent resource is not an error.

## Internal Implementation

All functions use a shared command runner:

```go
func run(ctx context.Context, args ...string) (string, error) {
    cmd := exec.CommandContext(ctx, "podman", args...)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("podman %s: %w (%s)", args[0], err, strings.TrimSpace(string(out)))
    }
    return strings.TrimSpace(string(out)), nil
}
```

Interactive commands (Exec with `interactive=true`, Attach) use `syscall.Exec` to replace the process, consistent with the `LocalEnvironment.Attach` pattern.

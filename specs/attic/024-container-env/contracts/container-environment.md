# Contract: ContainerEnvironment

**Feature**: 024-container-env | **Date**: 2026-03-20

## Struct Definition

```go
package env

// ContainerEnvironment manages a container-based environment
// using podman for single-container lifecycle.
type ContainerEnvironment struct {
    name  string
    store *FileStateStore
    defs  *DefinitionStore

    // Type-specific options set by CLI before lifecycle calls.
    Ports       []string          // Port mappings (host:container)
    AllPorts    bool              // Publish all exposed ports
    Credentials map[string]string // Credential key=value pairs
    KeepVolumes bool              // Preserve volumes on delete
}
```

## Interface Implementation

| Method | Behavior |
|--------|----------|
| `Type()` | Returns `EnvironmentTypeContainer` |
| `Name()` | Returns the environment name |
| `Create(ctx, opts)` | Validates name, checks podman available, creates volume, creates secrets, runs container, writes definition + state |
| `Start(ctx)` | Runs `podman start cc-deck-<name>`, updates state |
| `Stop(ctx)` | Runs `podman stop cc-deck-<name>`, updates state |
| `Delete(ctx, force)` | Stops container (if force), removes container, removes volume (unless KeepVolumes), removes secrets, removes definition + state records |
| `Status(ctx)` | Runs `podman inspect` for container state, reconciles with state store |
| `Attach(ctx)` | Auto-starts if stopped, runs `podman exec -it cc-deck-<name> zellij attach cc-deck --create` via syscall.Exec |
| `Exec(ctx, cmd)` | Runs `podman exec cc-deck-<name> <cmd...>` |
| `Push(ctx, opts)` | Runs `podman cp <local> cc-deck-<name>:/workspace/<path>` |
| `Pull(ctx, opts)` | Runs `podman cp cc-deck-<name>:<remote> <local>` |
| `Harvest(ctx, opts)` | Returns `ErrNotSupported` with message suggesting push/pull |

## Create Flow

```
1. ValidateEnvName(name)
2. Check podman.Available()
3. Resolve image (flag → config default → demo image fallback with warning)
4. Resolve storage (flag → named-volume default)
5. If named-volume: podman.VolumeCreate("cc-deck-<name>-data")
6. For each credential:
   a. Resolve value (explicit → host env → skip with warning)
   b. podman.SecretCreate("cc-deck-<name>-<key>", value)
7. podman.Run(RunOpts{
     Name:    "cc-deck-<name>",
     Image:   resolvedImage,
     Volumes: ["cc-deck-<name>-data:/workspace"] or ["/path:/workspace"],
     Secrets: resolved secrets,
     Ports:   resolved ports,
     Cmd:     ["sleep", "infinity"],
   })
8. Write EnvironmentDefinition to DefinitionStore
9. Write EnvironmentInstance to StateStore (state: running)
```

## Delete Flow

```
1. Load record from StateStore
2. If running and !force: return ErrRunning
3. podman.Remove("cc-deck-<name>", force)       [best-effort]
4. For each secret: podman.SecretRemove(...)     [best-effort, warn on error]
5. If !KeepVolumes: podman.VolumeRemove(...)     [best-effort, warn on error]
6. Remove from StateStore
7. Remove from DefinitionStore
```

## Reconciliation

Called during `cc-deck env list` and `cc-deck env status`:

```
1. Load all container-type instances from StateStore
2. For each instance:
   a. info = podman.Inspect("cc-deck-<name>")
   b. If info == nil: state = error (container deleted externally)
   c. If info.Running: state = running
   d. Else: state = stopped
   e. Update StateStore if state changed
```

## Naming Conventions

```go
func containerName(envName string) string { return "cc-deck-" + envName }
func volumeName(envName string) string    { return "cc-deck-" + envName + "-data" }
func secretName(envName, key string) string {
    return "cc-deck-" + envName + "-" + strings.ToLower(strings.ReplaceAll(key, "_", "-"))
}
```

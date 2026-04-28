# Brainstorm: Compose Environment

**Date:** 2026-03-21
**Status:** Brainstorm (ready for spec)
**Depends on:** 023-env-interface (Environment interface), 024-container-env (reference implementation, shared podman layer)
**Related:** 028-project-config (deferred, layers on top of compose)

## Summary

The compose environment is the second container-based environment type, adding multi-container orchestration via `podman-compose`. Its primary differentiator from the `container` type is support for sidecar containers: a tinyproxy sidecar for network filtering today, and MCP server sidecars in the future. It is **project-local**, meaning the generated compose files live in the project directory alongside the code.

## Design Decisions

All decisions below were made during interactive brainstorming.

### D1: Code Reuse Strategy

**Decision:** B/C hybrid. Shared helper functions + compose uses `internal/podman` directly.

- Extract `detectAuthMode()` and `detectAuthCredentials()` from `container.go` into shared helpers (exported functions or a separate file in `internal/env/`).
- Compose uses `internal/podman` for secrets and volumes.
- Compose uses the compose CLI (`podman-compose`) for lifecycle (up, down, start, stop).
- No embedding or wrapping of `ContainerEnvironment`.

### D2: Project-Local Compose Directory

**Decision:** Compose environments are project-local. Generated files live in `.cc-deck/` within the project directory.

```
~/projects/my-api/                    # project directory (base dir)
  cc-deck.yaml                        # project config (committed, DEFERRED to 028)
  .cc-deck/                           # generated artifacts (GITIGNORED)
    compose.yaml                      # generated
    .env                              # generated credentials
    proxy/                            # generated (if network filtering)
      tinyproxy.conf
      whitelist
  src/
  go.mod
```

Key principle: **committed config and generated artifacts live in separate locations.** The `.cc-deck/` directory is entirely gitignored. The optional `cc-deck.yaml` at project root is user-authored and committed (deferred to brainstorm 028).

The `--path` flag specifies the project directory. Defaults to cwd.

### D3: Allowed Domains Optional

**Decision:** `--allowed-domains` is optional for compose.

If specified, a tinyproxy sidecar is added to the compose project. If omitted, the compose project has only the session container (no proxy, no network filtering). This is intentional because compose will later support MCP sidecars, making it useful even without network filtering.

### D4: Default Storage is Bind Mount

**Decision:** Host-path bind mount is the default storage for project-local compose.

The project directory is mounted directly into the container at `/workspace`. Changes are bidirectional and immediate. This matches the project-local model where the user is already working in the project directory.

Named volume is available via `--storage named-volume` for when isolation is desired.

### D5: Gitignore Handling

**Decision:** Warn the user and offer an automatic option.

On `cc-deck env create --type compose`, if `.cc-deck/` is not in `.gitignore`:
1. Print: "Add `.cc-deck/` to your .gitignore to avoid committing generated files."
2. If `--gitignore` flag is passed, automatically append `.cc-deck/` to `.gitignore`.

The compose files are fully managed (generated, never hand-edited).

### D6: Existing `.cc-deck/` Directory

**Decision:** Regenerate with warning.

If `.cc-deck/` already exists when running `cc-deck env create --type compose`, regenerate all files from the definition, print a warning: "Regenerating compose files in .cc-deck/". The `.env` file is also regenerated (credentials come from the definition or host env, not from hand-edited .env files).

### D7: Delete Removes `.cc-deck/`

**Decision:** `cc-deck env delete` for compose removes the `.cc-deck/` directory entirely.

Since all files are generated and trivially reproducible, clean deletion is preferred. The project directory itself is never touched, only the `.cc-deck/` subdirectory.

### D8: CLI Surface

Compose shares all flags with the container type plus compose-specific additions:

```bash
cc-deck env create mydev --type compose \
  --image quay.io/cc-deck/cc-deck-demo:latest \
  --allowed-domains anthropic,github,npm \
  --port 8082:8082 \
  --path /path/to/project \
  --auth auto \
  --credential KEY=VALUE \
  --gitignore
```

New compose-specific flags:
- `--allowed-domains` (comma-separated or repeatable): domain groups/literals for proxy sidecar
- `--path` (optional): project directory, defaults to cwd
- `--gitignore` (optional): auto-add `.cc-deck/` to `.gitignore`

### D9: Definition and State Schema

**EnvironmentDefinition** additions:
```go
type EnvironmentDefinition struct {
    // ... existing fields ...
    AllowedDomains []string `yaml:"allowed-domains,omitempty"`  // compose only
    ProjectDir     string   `yaml:"project-dir,omitempty"`      // compose only
}
```

**ComposeFields** in state.yaml:
```go
type ComposeFields struct {
    ProjectDir    string `yaml:"project_dir"`
    ContainerName string `yaml:"container_name"`
    ProxyName     string `yaml:"proxy_name,omitempty"`
}
```

No `ContainerID` since compose manages the containers by name.

### D10: Compose Runtime Detection

Default to `podman-compose`. Auto-detection order if not configured:
1. `podman-compose` (preferred)
2. `docker compose` (v2 plugin)
3. `docker-compose` (legacy standalone)

Override via `$XDG_CONFIG_HOME/cc-deck/config.yaml`:
```yaml
defaults:
  compose-runtime: podman-compose
```

## Lifecycle Implementation

| Method | Implementation |
|--------|---------------|
| Create | Validate name, detect auth, create secrets via `internal/podman`, generate compose.yaml + proxy configs in `.cc-deck/`, run `podman-compose up -d` |
| Attach | `podman exec -it cc-deck-<name> zellij ...` (identical to container type) |
| Start | `podman-compose -f .cc-deck/compose.yaml start` |
| Stop | `podman-compose -f .cc-deck/compose.yaml stop` |
| Delete | `podman-compose -f .cc-deck/compose.yaml down`, remove secrets, remove `.cc-deck/` dir |
| Status | `podman inspect cc-deck-<name>` (same as container, uses container_name) |
| Exec | `podman exec cc-deck-<name> <cmd>` (same as container) |
| Push | `podman exec` + tar pipe (compose does not support `podman cp` on service names) |
| Pull | `podman exec` + tar pipe |
| Harvest | Not supported (return error, same as container) |

## Existing Code to Reuse

| Component | Package | What to reuse |
|-----------|---------|---------------|
| Compose YAML generator | `internal/compose/generate.go` | `Generate()` produces compose.yaml, tinyproxy.conf, whitelist |
| Tinyproxy config | `internal/compose/proxy.go` | `GenerateTinyproxyConf()`, `GenerateWhitelist()` |
| Domain resolver | `internal/network/domains.go` | `Resolver.ExpandAll()` resolves group names to domain lists |
| Podman secrets | `internal/podman/secret.go` | `SecretCreate()`, `SecretRemove()` |
| Podman exec | `internal/podman/exec.go` | `Exec()` for attach and command execution |
| Podman inspect | `internal/podman/container.go` | `Inspect()` for status reconciliation |
| Auth detection | `internal/env/container.go` | `detectAuthMode()`, `detectAuthCredentials()` (to be extracted) |
| Zellij session check | `internal/env/container.go` | `containerHasZellijSession()` (to be extracted) |

## Changes Needed in Existing Code

1. **`internal/env/types.go`**: Add `EnvironmentTypeCompose` constant, add `ComposeFields` struct
2. **`internal/env/factory.go`**: Add `EnvironmentTypeCompose` case
3. **`internal/env/definition.go`**: Add `AllowedDomains` and `ProjectDir` fields to `EnvironmentDefinition`
4. **`internal/env/container.go`**: Extract `detectAuthMode()`, `detectAuthCredentials()`, `containerHasZellijSession()` into shared helpers
5. **`internal/compose/generate.go`**: May need minor updates (volume mounts, secret integration)
6. **CLI commands**: Add `--allowed-domains`, `--path`, `--gitignore` flags to `env create`

## Compose YAML Generation Updates

The existing generator needs these adjustments for the compose environment:

1. **Volumes**: Add workspace bind mount (`./..:/workspace` relative to `.cc-deck/`)
2. **Secrets**: Integrate with podman secrets (currently uses `.env` file)
3. **Container name**: Already supported via `ContainerName` field
4. **Stdin/TTY**: Set `stdin_open: true` and `tty: true` for the session service (needed for interactive attach)

## Push/Pull via Exec

Since `podman cp` does not work with compose service names, push and pull use exec + tar:

```bash
# Push
tar cf - -C <local-path> . | podman exec -i cc-deck-<name> tar xf - -C /workspace

# Pull
podman exec cc-deck-<name> tar cf - -C /workspace <path> | tar xf - -C <local-path>
```

This is the same mechanism used by K8s environments and is reliable across all container runtimes.

## Network Filtering Architecture

When `--allowed-domains` is specified:

```
┌─ Compose Project ──────────────────────────────┐
│                                                 │
│  ┌─ session ──────────┐  ┌─ proxy ──────────┐  │
│  │  cc-deck-<name>    │  │  tinyproxy       │  │
│  │                    │  │                   │  │
│  │  HTTP_PROXY=proxy  │──│  allowlist filter │──│── internet
│  │  HTTPS_PROXY=proxy │  │                   │  │
│  │                    │  │  (internal +      │  │
│  │  (internal network │  │   default network)│  │
│  │   only)            │  │                   │  │
│  └────────────────────┘  └───────────────────┘  │
│                                                 │
│  Networks:                                      │
│    internal (bridge, no external access)         │
│    default  (bridge, internet access)            │
│                                                 │
│  Session container: internal only                │
│  Proxy container: internal + default             │
│  Result: session can only reach internet         │
│          through the proxy allowlist             │
└─────────────────────────────────────────────────┘
```

## Deferred Items

- **Project-local config** (`cc-deck.yaml`): Brainstorm 028. Layers on top of compose as an ergonomic enhancement.
- **MCP sidecars**: Future spec. Compose provides the foundation for adding MCP server containers alongside the session container.
- **Git harvesting**: Cross-environment feature, separate brainstorm (026-git-harvest-sync).

## Open Questions (resolved)

All questions resolved during brainstorming. No remaining open items.

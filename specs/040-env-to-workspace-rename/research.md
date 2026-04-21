# Research: Environment-to-Workspace Internal Rename

**Date**: 2026-04-21  
**Method**: Parallel codebase exploration (3 research agents)

## Decision 1: Rename Scope

**Decision**: Rename all `Environment`-prefixed Go identifiers (types, constants, functions) and abbreviated forms (`Env`, `Envs`) that refer to cc-deck workspaces. Preserve identifiers referring to OS environment variables or Docker Compose environment.

**Rationale**: The spec requires zero occurrences of `Environment`-prefixed types (FR-001). Abbreviated forms like `ReconcileContainerEnvs` and `ValidateEnvName` also refer to cc-deck workspaces and would create terminology inconsistency if left.

**Alternatives considered**: Renaming only full `Environment` prefix (leaving `Env` abbreviations) was rejected because it creates a mixed vocabulary.

## Decision 2: Package Move Strategy

**Decision**: Move `internal/env/` to `internal/ws/` as a directory rename. All 41 files change package declaration from `package env` to `package ws`.

**Rationale**: Go enforces that the package name matches the directory name. The spec requires `internal/ws/` (FR-002).

**Alternatives considered**: Creating a new package and migrating files one by one was rejected as unnecessary complexity for a mechanical rename.

## Decision 3: No Backward Compatibility

**Decision**: No fallback for old filenames, YAML keys, or environment variable names.

**Rationale**: Per clarification session, the user explicitly decided against backward compatibility. Old `environments.yaml`, `environments:` YAML key, and `CC_DECK_DEFINITIONS_FILE` are simply not recognized.

## Research Findings

### Identifiers to Rename (internal/env/ package)

**Types (10):**
- `EnvironmentType` -> `WorkspaceType`
- `EnvironmentState` -> `WorkspaceState`
- `EnvironmentInstance` -> `WorkspaceInstance`
- `EnvironmentDefinition` -> `WorkspaceDefinition`
- `EnvironmentStatus` -> `WorkspaceStatus`
- `LocalEnvironment` -> `LocalWorkspace`
- `ContainerEnvironment` -> `ContainerWorkspace`
- `ComposeEnvironment` -> `ComposeWorkspace`
- `SSHEnvironment` -> `SSHWorkspace`
- `K8sDeployEnvironment` -> `K8sDeployWorkspace`

**Interface (1):**
- `Environment` -> `Workspace`

**Constants (12):**
- `EnvironmentTypeLocal` -> `WorkspaceTypeLocal`
- `EnvironmentTypeContainer` -> `WorkspaceTypeContainer`
- `EnvironmentTypeCompose` -> `WorkspaceTypeCompose`
- `EnvironmentTypeK8sDeploy` -> `WorkspaceTypeK8sDeploy`
- `EnvironmentTypeK8sSandbox` -> `WorkspaceTypeK8sSandbox`
- `EnvironmentTypeSSH` -> `WorkspaceTypeSSH`
- `EnvironmentStateRunning` -> `WorkspaceStateRunning`
- `EnvironmentStateStopped` -> `WorkspaceStateStopped`
- `EnvironmentStateAvailable` -> `WorkspaceStateAvailable`
- `EnvironmentStateCreating` -> `WorkspaceStateCreating`
- `EnvironmentStateError` -> `WorkspaceStateError`
- `EnvironmentStateUnknown` -> `WorkspaceStateUnknown`

**Functions/Methods (8):**
- `NewEnvironment` -> `NewWorkspace`
- `ReconcileContainerEnvs` -> `ReconcileContainerWorkspaces`
- `ReconcileLocalEnvs` -> `ReconcileLocalWorkspaces`
- `ReconcileComposeEnvs` -> `ReconcileComposeWorkspaces`
- `ReconcileSSHEnvs` -> `ReconcileSSHWorkspaces`
- `ReconcileK8sDeployEnvs` -> `ReconcileK8sDeployWorkspaces`
- `AllProjectEnvironmentNames` -> `AllProjectWorkspaceNames`
- `ValidateEnvName` -> `ValidateWsName`

**Unexported identifiers:**
- `envNameRegex` -> `wsNameRegex`
- `maxEnvNameLength` -> `maxWsNameLength`
- `definitionFileName` -> value changes to `"workspaces.yaml"`

**Struct field + YAML tag:**
- `DefinitionFile.Environments` -> `DefinitionFile.Workspaces` (tag: `yaml:"workspaces"`)

### DO NOT Rename

- `EnvironmentDefinition.Env` field (`yaml:"env"`) - OS environment variables
- `compose/generate.go:Environment` field - Docker Compose YAML key
- `composeEnvFile = "env"` - Docker Compose env file
- Any `os.Getenv()` calls or OS env var references
- `auth.go` environment detection (OS env)
- `repos.go:134` "Resolve from environment variable"

### Consumer Files (5 files, 218 env. references)

| File | Type | References |
|------|------|-----------|
| `internal/cmd/ws.go` | Production | 120 |
| `internal/cmd/ws_new_test.go` | Test | 68 |
| `internal/cmd/ws_resolve_test.go` | Test | 5 |
| `internal/cmd/ws_prune_test.go` | Test | 1 |
| `internal/integration/k8s_deploy_test.go` | Integration | 24 |

### User-Facing Strings to Update

**Error messages (~25):** Found in `definition.go`, `state.go`, `ssh.go`, `compose.go`, `k8s_deploy.go`, `container.go`, `local.go`, `ws.go`

**CLI help text (~5):** Found in `ws.go` lines 29, 38, 155, 1740

**Build command descriptions (~10):** Found in `cc-deck.build.md`, `cc-deck.capture.md`, `build.yaml.tmpl`, `build.go`

**Config references:**
- `definitionFileName = "environments.yaml"` (definition.go:14)
- `CC_DECK_DEFINITIONS_FILE` env var (definition.go:62)
- ~20 test references to `CC_DECK_DEFINITIONS_FILE`
- ~8 test references to `environments.yaml`
- ~3 test inline YAML strings with `environments:` key

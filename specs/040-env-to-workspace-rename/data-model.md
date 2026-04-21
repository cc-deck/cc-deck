# Data Model: Environment-to-Workspace Rename

**Date**: 2026-04-21

## Entity Renames

This feature performs no structural data model changes. All entities retain the same fields, relationships, and validation rules. Only names change.

| Current Name | New Name | File |
|---|---|---|
| `EnvironmentDefinition` | `WorkspaceDefinition` | `definition.go` |
| `EnvironmentInstance` | `WorkspaceInstance` | `types.go` |
| `EnvironmentType` | `WorkspaceType` | `types.go` |
| `EnvironmentState` | `WorkspaceState` | `types.go` |
| `EnvironmentStatus` | `WorkspaceStatus` | `interface.go` |
| `Environment` (interface) | `Workspace` | `interface.go` |
| `LocalEnvironment` | `LocalWorkspace` | `local.go` |
| `ContainerEnvironment` | `ContainerWorkspace` | `container.go` |
| `ComposeEnvironment` | `ComposeWorkspace` | `compose.go` |
| `SSHEnvironment` | `SSHWorkspace` | `ssh.go` |
| `K8sDeployEnvironment` | `K8sDeployWorkspace` | `k8s_deploy.go` |
| `DefinitionFile.Environments` | `DefinitionFile.Workspaces` | `definition.go` |

## YAML Serialization

YAML tags in `state.yaml` are unchanged (e.g., `yaml:"type"`, `yaml:"instances"`). No state file migration.

The `DefinitionFile` YAML tag changes from `yaml:"environments"` to `yaml:"workspaces"`. The config filename changes from `environments.yaml` to `workspaces.yaml`.

## Config File Format

Before:
```yaml
version: 1
environments:
  - name: myws
    type: container
    image: quay.io/cc-deck/cc-deck:latest
```

After:
```yaml
version: 1
workspaces:
  - name: myws
    type: container
    image: quay.io/cc-deck/cc-deck:latest
```

## Relationships (unchanged)

- `WorkspaceDefinition` 1:1 `WorkspaceInstance` (by name)
- `WorkspaceInstance` 1:N `SessionInfo` (attached sessions)
- `Workspace` interface implemented by 5 concrete types
- `DefinitionStore` manages `WorkspaceDefinition` persistence
- `FileStateStore` manages `WorkspaceInstance` persistence

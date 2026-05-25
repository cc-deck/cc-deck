# Data Model: Tool PATH Restoration

## Modified Entities

### ContainerfileData (containerfile.go)

Existing struct with one new field:

| Field | Type | Description |
|-------|------|-------------|
| Target | string | "container" or "openshell" (existing) |
| User | string | "dev" or "sandbox" (existing) |
| HomeDir | string | "/home/dev" or "/sandbox" (existing) |
| ContextDir | string | "container" or "openshell" (existing) |
| BaseImage | string | From manifest targets (existing) |
| Shell | string | From settings or "zsh" (existing) |
| **ToolPaths** | **[]string** | **Resolved tool install paths for PATH restoration (NEW)** |

## New Constants

### toolPathRegistry (containerfile.go)

A Go map constant mapping tool name keywords to their install path templates:

```
Key: "go"     → Value: "/usr/local/go/bin"
Key: "cargo"  → Value: "{home}/.cargo/bin"
Key: "rust"   → Value: "{home}/.cargo/bin"
```

The `{home}` placeholder is replaced with `ContainerfileData.HomeDir` during resolution.

## New Functions

### ResolveToolPaths (containerfile.go)

Takes a `*Manifest` and a `homeDir string`, returns `[]string`.

Iterates `manifest.Tools`, checks each tool name against the registry keys using case-insensitive substring matching. Collects matching paths, resolves `{home}` placeholders, and deduplicates.

## Data Flow

```
Manifest.Tools
  → iterate tool names
  → for each tool, check against toolPathRegistry keys (case-insensitive substring)
  → collect matched paths
  → replace {home} with ContainerfileData.HomeDir
  → deduplicate
  → set ContainerfileData.ToolPaths

ContainerfileData.ToolPaths
  → rendered in 05-shell-finalize.tmpl
  → generates: RUN sed -i '1i export PATH="/usr/local/go/bin:/sandbox/.cargo/bin:$PATH"' .zshrc .bashrc
```

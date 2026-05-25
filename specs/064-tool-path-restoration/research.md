# Research: Tool PATH Restoration

## Decision 1: Registry data structure

**Decision**: Go map constant (`map[string]string`) in `containerfile.go`.

**Rationale**: The registry is small (2-3 entries initially), read-only, and only used at build time. A Go map constant is the simplest structure, requires no file I/O, and is trivially testable. A YAML config file would add file discovery, parsing, and error handling complexity for no benefit at this scale.

**Alternatives considered**:
- YAML config file: Rejected because it adds I/O and parsing for a table that changes only when the codebase adds new tool install patterns.
- Embedded YAML: Better than external YAML but still adds parsing overhead for no benefit.

## Decision 2: Tool name matching strategy

**Decision**: Case-insensitive substring matching with explicit registry keys.

**Rationale**: Manifest tool names are human-readable strings like "Go >= 1.25.0", "Rust stable (edition 2021)", "cargo". The registry needs to match "go" against "Go >= 1.25.0" and "cargo" or "rust" against Rust entries. Case-insensitive `strings.Contains` achieves this. Multiple keys can map to the same path (e.g., both "cargo" and "rust" map to `{home}/.cargo/bin`).

**Alternatives considered**:
- Exact match: Too rigid for human-readable tool names.
- Regex: Overkill for simple keyword matching.

## Decision 3: Home directory placeholder

**Decision**: Use `{home}` as a placeholder in registry values, replaced with `ContainerfileData.HomeDir` during resolution.

**Rationale**: `ContainerfileData` already has a `HomeDir` field (`/sandbox` for openshell, `/home/dev` for container). Using a simple string replacement avoids Go template parsing complexity in the registry values.

## Decision 4: Template gate for both targets

**Decision**: Remove the `{{- if eq .Target "openshell"}}` gate from the new PATH prepend block so it runs for both openshell and container targets.

**Rationale**: The spec requires FR-009 (both targets). The existing `05-shell-finalize.tmpl` gates starship and Zellij setup to openshell only, but PATH restoration is needed for both. The new block should be ungated (or gated to exclude SSH, but SSH never uses this template).

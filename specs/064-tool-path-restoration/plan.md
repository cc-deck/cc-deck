# Implementation Plan: Tool PATH Restoration

**Branch**: `064-tool-path-restoration` | **Date**: 2026-05-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/064-tool-path-restoration/spec.md`

## Summary

Add a tool path registry to the build package that maps manifest tools to their non-standard install paths. During Containerfile generation, resolve the registry against the manifest and prepend the matched paths to shell rc files in the shell-finalize template. This ensures tools like `go` and `cargo` are available in interactive shell sessions even when login initialization resets the Docker `ENV PATH`.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: text/template (Go stdlib), gopkg.in/yaml.v3 (YAML), testify v1.11.1 (testing)
**Storage**: N/A (build-time only, no runtime state)
**Testing**: `go test ./...` via `make test`
**Target Platform**: Linux containers (OpenShell sandboxes, container targets)
**Project Type**: CLI tool + build system
**Constraints**: Must use `make test`, `make lint` (never `go build` directly)
**Scale/Scope**: 2-3 initial registry entries, extensible

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| Tests for new code | PASS | Unit tests for registry resolution, template rendering |
| README.md updated | PASS | Will document tool PATH restoration |
| CLI reference updated | N/A | No new CLI commands or flags |
| Antora docs guide page | N/A | Build internals, not a user-facing feature |
| Configuration reference | N/A | No new config options (registry is internal) |
| Prose plugin for docs | PASS | Will use cc-deck voice profile for any doc updates |
| Build rules (make only) | PASS | All builds via `make test`, `make lint` |

## Project Structure

### Documentation (this feature)

```text
specs/064-tool-path-restoration/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
cc-deck/internal/build/
├── containerfile.go     # Add ToolPaths field, toolPathRegistry, ResolveToolPaths()
├── containerfile_test.go # Tests for registry resolution
└── templates/containerfile/
    └── 05-shell-finalize.tmpl  # Add PATH prepend block
```

**Structure Decision**: All changes are in the existing `cc-deck/internal/build/` package. No new packages or files needed (only modifications to existing files plus one new function and one new constant).

## Implementation Phases

### Phase 1: Registry and Resolution (P1 - User Story 1 + 3)

**Files**: `containerfile.go`, `containerfile_test.go`

1. Add `toolPathRegistry` map constant mapping tool keywords to install path templates:
   - `"go"` -> `"/usr/local/go/bin"`
   - `"cargo"` -> `"{home}/.cargo/bin"`
   - `"rust"` -> `"{home}/.cargo/bin"`
2. Add `ResolveToolPaths(m *Manifest, homeDir string) []string` function:
   - Iterate `m.Tools`, check each `Name` against registry keys (case-insensitive substring via `strings.Contains`)
   - Replace `{home}` with `homeDir` in matched paths
   - Deduplicate results (preserve order, skip duplicates)
3. Add `ToolPaths []string` field to `ContainerfileData`
4. Update `ContainerDataForTarget()` to call `ResolveToolPaths()` and populate `ToolPaths`
5. Add tests: Go tool matches, Rust/cargo tool matches, no matches (empty result), deduplication, case-insensitive matching, home directory substitution

### Phase 2: Template Integration (P2 - User Story 2)

**Files**: `05-shell-finalize.tmpl`

1. Add a PATH prepend block at the top of the template (before the openshell-gated starship/Zellij blocks)
2. The block is conditional: only renders if `.ToolPaths` is non-empty
3. Generates a single `RUN` step that prepends all paths to both `.zshrc` and `.bashrc` using `sed -i '1i ...'`
4. No target gate (works for both openshell and container)
5. Add test for template rendering with and without tool paths

### Phase 3: Documentation and Cleanup

**Files**: `README.md`

1. Update README.md to mention tool PATH restoration behavior
2. Remove the manually added `/usr/local/go/bin` from the curated zshrc in `.cc-deck/setup/config/zshrc` (this was the manual workaround, now replaced by the automated solution)

# Implementation Plan: Two-Pass Binary Probing

**Branch**: `062-two-pass-binary-probing` | **Date**: 2026-05-24 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/062-two-pass-binary-probing/spec.md`

## Summary

Replace the static well-known paths table (`policy_binaries.go`) with a two-pass image build that probes the actual container for binary locations. The first pass builds the image without binary restrictions. A probe step discovers binary paths via `which`/`find` inside the built image. The second pass rebuilds with the corrected policy containing probed paths and runtime glob patterns. Component YAML files gain `probe_binaries` and `runtime_globs` fields, making each component self-contained.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML parsing), encoding/json (probe output parsing), os/exec (podman invocation), context (timeouts)
**Storage**: Filesystem (policy.yaml, component YAML files)
**Testing**: `go test ./...` via `make test`; `make lint` for static analysis
**Target Platform**: Linux/macOS (CLI runs locally, images target Linux containers)
**Project Type**: CLI tool
**Performance Goals**: Second-pass rebuild under 10 seconds on warm cache; total probe overhead under 30 seconds
**Constraints**: Must use `podman` exclusively (never Docker). Must use `make install`/`make test`/`make lint` (never `go build` directly). XDG paths via `internal/xdg` package.
**Scale/Scope**: 7 embedded policy component YAML files to update; 1 new Go file to create; 2 Go files to modify; 1 Go file to remove

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Plan includes unit tests for probe logic, integration tests for two-pass flow, and documentation updates (README, CLI reference). |
| II. Interface contracts | PASS | Component YAML schema contract defined in `contracts/component-schema.md`. No new interface backends. |
| III. Build and tool rules | PASS | All builds via `make install`/`make test`/`make lint`. Container runtime is `podman`. XDG paths not affected. |

## Project Structure

### Documentation (this feature)

```text
specs/062-two-pass-binary-probing/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── component-schema.md
└── tasks.md             # Phase 2 output (from /speckit.tasks)
```

### Source Code (repository root)

```text
cc-deck/internal/build/
├── probe.go                 # NEW: probe container logic, result parsing
├── probe_test.go            # NEW: unit tests for probe logic
├── component.go             # MODIFIED: add ProbeBinaries, RuntimeGlobs fields
├── component_test.go        # MODIFIED: test new field parsing/validation
├── policy.go                # MODIFIED: two-pass assembly mode, applyProbeResults()
├── policy_test.go           # MODIFIED: test two-pass assembly
├── policy_binaries.go       # REMOVED: well-known paths table and resolveBinaries()
├── policy_binaries_test.go  # REMOVED: tests for removed function
└── policies/
    ├── python.yaml          # MODIFIED: add probe_binaries, runtime_globs
    ├── rust.yaml             # MODIFIED: add probe_binaries, runtime_globs
    ├── node.yaml             # MODIFIED: add probe_binaries, runtime_globs
    ├── go.yaml               # MODIFIED: add probe_binaries, runtime_globs
    ├── claude-code.yaml      # UNCHANGED (explicit binaries)
    ├── git-hosting.yaml      # UNCHANGED (explicit binaries)
    └── vertex-ai.yaml        # UNCHANGED (explicit binaries)

cc-deck/internal/cmd/
└── build.go                 # MODIFIED: runOpenShellBuild() gains two-pass logic
```

**Structure Decision**: All changes fit within the existing `internal/build/` and `internal/cmd/` packages. A new `probe.go` file is added for the probe logic, keeping it separate from policy assembly. No new packages needed.

## Complexity Tracking

No constitution violations to justify.

## Implementation Approach

### Phase 1: Component Schema Extension

1. Add `ProbeBinaries []string` and `RuntimeGlobs []string` fields to `PolicyComponent` struct in `component.go`.
2. Extend `ValidateComponent()` to validate new fields (no path separators in probe_binaries, absolute paths in runtime_globs).
3. Update all 4 tool-matched component YAML files (python.yaml, rust.yaml, node.yaml, go.yaml) with appropriate `probe_binaries` and `runtime_globs` values.
4. Add tests for new field parsing and validation.

### Phase 2: Probe Logic

1. Create `probe.go` with:
   - `ProbeResult` and `ProbeReport` types
   - `generateProbeScript(components []PolicyComponent) string`: generates the shell script that runs `which`/`find` for each binary and outputs JSON results.
   - `ProbeBinaries(ctx context.Context, runtime string, imageRef string, components []PolicyComponent) (*ProbeReport, error)`: runs the probe container and parses results.
   - `collectProbeBinaries(comp PolicyComponent) []string`: returns `probe_binaries` if set, otherwise `match.tools`.
2. Create `probe_test.go` with unit tests for script generation and result parsing (mock the podman execution).

### Phase 3: Policy Assembly Refactor

1. Add `applyProbeResults(components []PolicyComponent, report *ProbeReport) []PolicyComponent`: replaces `resolveBinaries()`. For each component without explicit binaries, populates `Binaries` from probe results plus `runtime_globs`.
2. Add `AssemblePolicyFirstPass()` or a `stripBinaries` parameter to `AssemblePolicy()` that produces a policy with empty binaries on non-explicit components.
3. Remove `policy_binaries.go` (the `wellKnownPaths` table and `resolveBinaries()` function).
4. Remove `policy_binaries_test.go`.
5. Update `policy_test.go` to test the new assembly paths.

### Phase 4: Two-Pass Build Integration

1. Modify `runOpenShellBuild()` in `cmd/build.go`:
   - First pass: call `refreshOpenShellPolicy()` with binaries stripped, build image with tag `<name>:probe-build`.
   - Probe: call `build.ProbeBinaries()` with the first-pass image.
   - Second pass: call `refreshOpenShellPolicy()` with probe results applied, rebuild image with the final tag.
   - Stamp: call `oci.StampPolicyLabel()` on the final image.
   - Cleanup: remove the first-pass image on success; retag as `<name>:probe-debug` on failure.
2. Handle the skip case: when no tools need probing (no matched components with `match.tools`), skip the two-pass process entirely and build normally.

### Phase 5: Tests and Documentation

1. Add integration-style tests that verify the two-pass build flow end-to-end (mocking podman execution).
2. Update existing policy assembly tests to work without the well-known paths table.
3. Ensure `make test` and `make lint` pass.
4. Update README.md with the two-pass build behavior.
5. Update CLI reference docs if build command output changes.
6. Update configuration reference for the new component YAML fields.

## Key Files Reference

| File | Role | Change |
|------|------|--------|
| `cc-deck/internal/build/component.go` | Component struct & loading | Add ProbeBinaries, RuntimeGlobs fields + validation |
| `cc-deck/internal/build/probe.go` | Probe container logic | New file |
| `cc-deck/internal/build/policy.go` | Policy assembly | Add first-pass mode, applyProbeResults() |
| `cc-deck/internal/build/policy_binaries.go` | Well-known paths table | Remove entirely |
| `cc-deck/internal/build/policies/*.yaml` | Embedded components | Add probe_binaries, runtime_globs to 4 files |
| `cc-deck/internal/cmd/build.go` | Build command | Two-pass flow in runOpenShellBuild() |

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Probe container fails on some base images (missing `which`/`find`) | FR-004/FR-005: graceful fallback and warnings; edge case in spec |
| Second-pass cache miss (unexpected layer invalidation) | Containerfile structure already separates tool layers from policy COPY |
| Probe timeout on large images | 30s per-binary + 5min total timeout with clear error messages |
| Component YAML migration misses a glob pattern | Compare wellKnownPaths table entries against new runtime_globs before removing |

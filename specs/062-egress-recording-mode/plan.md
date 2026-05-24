# Implementation Plan: Egress Recording Mode

**Branch**: `062-egress-recording-mode` | **Date**: 2026-05-24 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/062-egress-recording-mode/spec.md`

## Summary

Add a `cc-deck build record` subcommand that launches an interactive Podman pod with the workspace image and a CoreDNS sidecar to capture all DNS queries. On session exit, the captured domains are deduplicated, filtered for noise, matched against existing catalog components, and new domains are appended to `build.yaml` `network.allowed_domains`. This eliminates manual endpoint research for workspace policy configuration.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (manifest I/O), internal/podman (container lifecycle), internal/build (manifest, policy, components)
**Storage**: `build.yaml` manifest file (read + write-back)
**Testing**: `go test` via `make test`, testify v1.11.1
**Target Platform**: Linux/macOS (wherever Podman runs)
**Project Type**: CLI tool
**Performance Goals**: Pod creation overhead < 30s (SC-002), post-processing < 10s for 500 domains (SC-003)
**Constraints**: Podman only (no Docker), `make install`/`make test`/`make lint` only (no direct `go build`)
**Scale/Scope**: Single interactive session, up to 500 unique domains

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests + documentation | PASS | Plan includes unit tests for all new packages, CLI reference update, Antora guide page |
| II. Interface contracts | N/A | No new interface implementations; uses existing `Manifest`, `PolicyComponent` types |
| III. Build and tool rules | PASS | Uses `make install`/`make test`/`make lint`, `internal/xdg` not needed (no new XDG paths), Podman only |

No violations. No Complexity Tracking entries needed.

## Project Structure

### Documentation (this feature)

```text
specs/062-egress-recording-mode/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (created by /speckit-tasks)
```

### Source Code (repository root)

```text
cc-deck/internal/
├── record/              # NEW: recording session package
│   ├── record.go        # Session orchestration (pod create, attach, teardown)
│   ├── record_test.go   # Unit tests for orchestration logic
│   ├── dns.go           # DNS log parsing, dedup, noise filtering
│   ├── dns_test.go      # Unit tests for parsing and filtering
│   ├── catalog.go       # Reverse catalog matching
│   └── catalog_test.go  # Unit tests for matching
├── podman/
│   ├── pod.go           # NEW: pod create/remove operations
│   └── pod_test.go      # NEW: pod operation tests
├── build/
│   └── manifest.go      # MODIFIED: add SaveManifest() function
├── cmd/
│   └── build.go         # MODIFIED: add newBuildRecordCmd()

docs/modules/
├── reference/pages/
│   └── cli.adoc         # MODIFIED: add build record reference
├── using/pages/
│   └── egress-recording.adoc  # NEW: guide page
```

**Structure Decision**: New `internal/record` package isolates all recording logic (session lifecycle, DNS parsing, catalog matching). Podman pod operations go in the existing `internal/podman` package. The CLI command is a thin wrapper in `cmd/build.go`.

## Implementation Phases

### Phase 1: Pod Infrastructure + DNS Parsing (Foundation)

Add Podman pod primitives and DNS log parser. These are the building blocks with no external dependencies.

**Deliverables**:
- `internal/podman/pod.go`: `PodCreate()`, `PodRemove()`, `PodExists()`
- `internal/record/dns.go`: `ParseDNSLog()`, `DeduplicateDomains()`, `FilterNoise()`
- Unit tests for both packages
- Tests use hardcoded CoreDNS log samples (no live Podman needed)

**FR coverage**: FR-002 (pod creation), FR-009 (dedup + noise filtering)

### Phase 2: Recording Session Orchestration (Core)

Wire up the full session lifecycle: create pod with workspace + sidecar, attach user, extract log on exit, clean up.

**Deliverables**:
- `internal/record/record.go`: `RunRecordingSession()` orchestrator
- CoreDNS Corefile generation (embedded string template)
- Pod lifecycle: create volume, create pod, start sidecar, start workspace, attach, wait for exit, copy log, remove pod + volume
- Error handling: image not found, sidecar failure, cleanup on interrupt (SIGINT/SIGTERM)

**FR coverage**: FR-001, FR-003, FR-004, FR-005, FR-006, FR-007, FR-008, FR-015

### Phase 3: Catalog Matching + Manifest Update (Output)

Post-session processing: match domains against catalog, update manifest, print summary.

**Deliverables**:
- `internal/record/catalog.go`: `MatchAgainstCatalog()` reverse matching
- `internal/build/manifest.go`: `SaveManifest()` function
- Manifest update logic: load, append new domains to `allowed_domains`, save
- Summary report output (total observed, covered, new, file path)
- Unit tests for matching and manifest round-trip

**FR coverage**: FR-010, FR-011, FR-012, FR-013, FR-014

### Phase 4: CLI Integration + Documentation

Register the command, add documentation, verify end-to-end.

**Deliverables**:
- `internal/cmd/build.go`: `newBuildRecordCmd()` with setup dir argument
- CLI reference in `docs/modules/reference/pages/cli.adoc`
- Guide page `docs/modules/using/pages/egress-recording.adoc`
- Configuration reference update if needed
- README.md update

**FR coverage**: FR-001 (CLI surface), Constitution I (docs)

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| CoreDNS log format changes across versions | Low | Medium | Pin to specific CoreDNS image tag, parse defensively |
| Podman pod API differences across versions | Medium | Medium | Test on Podman 4.x and 5.x, use basic pod create flags only |
| YAML write-back mangles user formatting | Medium | Low | Acceptable (same as capture command), document in guide |
| DNS sidecar misses queries during startup race | Low | Medium | Start sidecar first, wait for port 53 to be listening before starting workspace |
| Signal handling on Ctrl+C leaves orphan pods | Medium | High | Trap SIGINT/SIGTERM, always attempt pod cleanup in defer |

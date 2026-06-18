# Implementation Plan: Base Image Probe

**Branch**: `070-base-image-probe` | **Date**: 2026-06-17 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/070-base-image-probe/spec.md`

## Summary

Add a base image probe step to the build pipeline that inspects a container image before Containerfile generation. The probe discovers OS family, package manager, pre-installed tools with versions, user setup, and shell availability. Results are cached by image reference + digest so repeat builds skip the probe. The Containerfile generator uses probe data to select the correct package manager, skip already-installed tools, and shadow incompatible versions via PATH ordering.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML), encoding/json (stdlib), os/exec (stdlib for podman invocation)
**Storage**: JSON file (`<setup-dir>/probe-cache.json`) for probe result caching
**Testing**: `go test` via `make test`, testify v1.11.1 for assertions
**Target Platform**: macOS (build host), Linux containers (probe target)
**Project Type**: CLI tool (cc-deck) with Claude Code build skill integration
**Performance Goals**: Probe completes within 30 seconds; cache lookup under 1 second
**Constraints**: Single `podman run` invocation per probe; no cross-architecture probing
**Scale/Scope**: ~30 default tools to probe; 3 OS families (Fedora/dnf, Debian/apt-get, RHEL/dnf)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests + docs | PASS | Unit tests for probe script gen, parsing, version comparison, caching. Integration test against real images. CLI reference updated. Guide page for probe command. |
| II. Interface contracts | PASS | New package, no existing interface to implement. Contracts defined in `contracts/`. |
| III. Build/tool rules | PASS | Uses `make test`/`make lint`, `internal/xdg` not needed (setup-dir based), podman exclusively. |
| IV. Plugin debug logging | N/A | No plugin changes in this feature. |

## Project Structure

### Documentation (this feature)

```text
specs/070-base-image-probe/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   ├── probe-cli.md     # CLI contract
│   └── probe-script.md  # Probe script contract
└── tasks.md             # Phase 2 output (from /speckit.tasks)
```

### Source Code (repository root)

```text
cc-deck/
├── cmd/
│   └── build_probe.go          # NEW: `cc-deck build probe` subcommand
├── internal/build/
│   ├── imageprobe/             # NEW: base image probe package
│   │   ├── probe.go            # Script generation + podman execution
│   │   ├── parse.go            # JSON output parsing
│   │   ├── cache.go            # Probe cache read/write/invalidation
│   │   ├── version.go          # Semver parsing + compatibility check
│   │   ├── tools.go            # Default tool set + manifest merge
│   │   ├── diff.go             # ToolDiff: compare probe vs manifest
│   │   ├── probe_test.go       # Unit tests: script gen, parsing
│   │   ├── cache_test.go       # Unit tests: cache operations
│   │   ├── version_test.go     # Unit tests: version comparison
│   │   ├── diff_test.go        # Unit tests: tool diffing
│   │   └── integration_test.go # Integration test: probe real images
│   ├── containerfile.go        # MODIFIED: add ProbeResult to ContainerfileData
│   └── manifest.go             # MODIFIED: add ProbeTools field
├── internal/build/commands/
│   └── cc-deck.build.md        # MODIFIED: integrate probe step into A2/C2
docs/
├── modules/guide/pages/
│   └── base-image-probe.adoc   # NEW: guide page
├── modules/reference/pages/
│   ├── cli.adoc                # MODIFIED: add `build probe` command
│   └── configuration.adoc      # MODIFIED: add `probe_tools:` manifest key
```

**Structure Decision**: New `imageprobe` package under `internal/build/` keeps probe logic isolated from the existing policy probe in `probe.go`. The CLI subcommand follows the existing `build_*` pattern in `cmd/`.

## Implementation Architecture

### Data Flow

```
Manifest (build.yaml)
  ├── tools: [Go 1.25, Python 3, ...]
  ├── base image ref
  └── probe_tools: [...] (optional override)
         │
         ▼
  ┌─────────────────┐
  │  Probe Cache     │◄── probe-cache.json
  │  (check digest)  │
  └────────┬────────┘
           │ cache miss
           ▼
  ┌─────────────────┐
  │  podman run      │   Single container invocation
  │  --entrypoint sh │   with generated probe script
  │  <base-image>    │
  └────────┬────────┘
           │ JSON-per-line output
           ▼
  ┌─────────────────┐
  │  Parse + Store   │──► probe-cache.json (updated)
  └────────┬────────┘
           │
           ▼
  ┌─────────────────┐
  │  ToolDiff        │   Compare probe results vs manifest
  │  (present/       │   requirements using major.minor
  │   missing/       │   version comparison
  │   incompatible)  │
  └────────┬────────┘
           │
           ▼
  ┌─────────────────┐
  │  Containerfile   │   Use probed pkg mgr (dnf/apt-get)
  │  Generation      │   Skip present tools
  │                  │   Shadow incompatible via /usr/local/bin
  └─────────────────┘
```

### Version Comparison Logic

```
Required: Go 1.25
Installed: Go 1.22

Parse both → (major=1, minor=25) vs (major=1, minor=22)
Same major? YES
Installed minor >= required minor? NO (22 < 25)
Result: INCOMPATIBLE → shadow install to /usr/local/bin
```

### Probe Script Structure (generated by Go)

```sh
#!/bin/sh
# OS detection
cat /etc/os-release 2>/dev/null | while IFS='=' read key val; do
  case "$key" in ID|ID_LIKE|NAME|VERSION_ID) ... ;; esac
done
# Package manager detection
for pm in dnf apt-get apk yum; do which $pm 2>/dev/null && ...; done
# Tool detection (repeated for each tool)
p=$(which git 2>/dev/null) && v=$(git --version 2>&1 | head -1) && printf '{"type":"tool",...}\n'
# User info
printf '{"type":"user","name":"%s","uid":%d,...}\n' "$(whoami)" "$(id -u)"
# Shell availability
for s in bash zsh sh; do which $s 2>/dev/null; done
```

## Complexity Tracking

No constitution violations to justify.

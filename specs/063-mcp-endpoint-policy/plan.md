# Implementation Plan: MCP Endpoint Policy Integration

**Branch**: `063-mcp-endpoint-policy` | **Date**: 2026-05-24 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/063-mcp-endpoint-policy/spec.md`

## Summary

Add MCP server endpoint declarations to the build manifest and automatically generate network policy entries during policy assembly. This unblocks Claude Code MCP server connections in OpenShell sandboxes, which currently hang at startup because the supervisor denies all MCP traffic.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML), testify v1.11.1 (testing)
**Storage**: YAML manifest files (`build.yaml`), embedded YAML component files
**Testing**: `go test ./...` via `make test`
**Target Platform**: Linux (OpenShell sandboxes)
**Project Type**: CLI tool + build system
**Performance Goals**: No measurable regression in policy assembly time
**Constraints**: Must use `make test`, `make lint` (never `go build` directly)
**Scale/Scope**: Up to 10 MCP entries per manifest

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| Tests for new code | PASS | Unit tests for policy assembly, slugification, endpoint parsing |
| README.md updated | PASS | Will document MCP endpoint feature |
| CLI reference updated | N/A | No new CLI commands or flags |
| Antora docs guide page | N/A | Not a substantial standalone feature requiring its own guide |
| Configuration reference | PASS | Will document `endpoint` field in manifest schema |
| Prose plugin for docs | PASS | Will use cc-deck voice profile |
| Build rules (make only) | PASS | All builds via `make test`, `make lint` |
| XDG paths | N/A | No XDG paths involved |
| Container runtime | N/A | No container operations |

## Project Structure

### Documentation (this feature)

```text
specs/063-mcp-endpoint-policy/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (from /speckit.tasks)
```

### Source Code (repository root)

```text
cc-deck/internal/build/
├── manifest.go          # Add Endpoint field to MCPEntry struct
├── policy.go            # Add MCP policy generation + pkg_node augmentation + slugifyMCPName()
├── policy_test.go       # Tests for MCP policy entries, slugification, augmentation
└── commands/
    └── cc-deck.capture.md  # Extend Step 9 with endpoint extraction

docs/modules/reference/pages/
├── configuration.adoc   # Document endpoint field
└── manifest-schema.adoc # Document endpoint in schema reference (if exists)
```

**Structure Decision**: All changes are in the existing `cc-deck/internal/build/` package. No new packages or directories needed.

## Implementation Phases

### Phase 1: Core Policy Assembly (P1 - User Story 1)

**Files**: `manifest.go`, `policy.go`, `policy_test.go`

1. Add `Endpoint string` field with `yaml:"endpoint,omitempty"` to `MCPEntry` struct in `manifest.go`
2. Add `slugifyMCPName()` function in `policy.go` (replace hyphens/spaces/non-alphanumeric with underscores, lowercase)
3. Add `parseMCPEndpoint()` helper in `policy.go` to split `host:port` string, validate, return (host, port, error)
4. In `assemblePolicyCore()`, after the credentials block (~line 242), add MCP processing loop:
   - Find the `claude_code` component in `matched` to get its binaries
   - Iterate `manifest.MCP` entries with non-empty `Endpoint`
   - Parse endpoint, skip with warning on error
   - Generate `NetworkPolicy` keyed as `mcp_<slugifyMCPName(name)>`
   - Use `Description` as `Name` (fallback to `Name` if empty)
   - Set binaries from `claude_code` component
5. Add tests: MCP entry with endpoint, MCP entry without endpoint, malformed endpoint, multiple MCP entries, determinism with MCP entries, missing claude_code component (skip gracefully)

### Phase 2: pkg_node Binary Augmentation (P3 - User Story 3)

**Files**: `policy.go`, `policy_test.go`

1. After MCP processing in `assemblePolicyCore()`, check if `pkg_node` exists in `networkPolicies` AND manifest has any MCP entries with endpoints
2. If both conditions met, get `claude_code` binaries and append to `pkg_node` entry's binary list (dedup by path)
3. Add tests: augmentation when both conditions met, no augmentation when pkg_node absent, no augmentation when no MCP endpoints

### Phase 3: Capture Command Extension (P2 - User Story 2)

**Files**: `cc-deck.capture.md`

1. Extend Step 9 to extract endpoint URLs:
   - HTTP/SSE: parse `url` field from server config, extract host:port
   - Stdio with mcp-remote: scan `args` array for HTTPS URLs, extract host:port
   - Local stdio: no endpoint
2. Present extracted endpoints alongside MCP server info for user confirmation
3. Write confirmed endpoints to manifest's `mcp[].endpoint` field

### Phase 4: Documentation

**Files**: `docs/modules/reference/pages/configuration.adoc`

1. Document the `endpoint` field in the MCP entry section of the configuration reference
2. Add example showing MCP entries with and without endpoints
3. Use prose plugin with cc-deck voice profile

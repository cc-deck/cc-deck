# Research: Policy Binary Resolution

## R1: Insertion Point in AssemblePolicy

**Decision**: Add binary resolution after component matching and before building the networkPolicies map (between lines 122 and 126 in policy.go).

**Rationale**: At this point, `matched` contains all components that will become policy entries. We can iterate over them, check if they lack binaries, and resolve from the manifest. This is before the allowed_domains expansion, so catalog components get binaries first.

## R2: Well-Known Paths Table Design

**Decision**: Use a `map[string][]string` keyed by tool name. Each value is a list of additional paths beyond the default `/usr/bin/<name>`. The table is defined as a package-level variable in a new file `policy_binaries.go`.

**Rationale**: A simple map lookup per tool name. The table is small (fewer than 20 entries) and rarely changes. Keeping it in Go code rather than YAML means it compiles into the binary with no I/O.

**Alternatives considered**:
- YAML config file: Adds I/O and a deployment concern. Rejected.
- Per-component binaries in catalog YAML: Breaks installation independence. Rejected (this is the problem we're solving).

## R3: Resolution Logic

**Decision**: For each matched component that has NO explicit binaries:
1. Read `match.Tools` to get tool names
2. For each tool name, look up in manifest's `Tools` section
3. If found with `Install: "package"` or empty: add `/usr/bin/<name>`
4. If found with `Install: "github-release"`: add `InstallPath`
5. Always add well-known paths for the tool (from the table)
6. Deduplicate paths

**Rationale**: Covers all install methods. Well-known paths catch alternative locations (cargo in ~/.cargo/bin, go in /usr/local/go/bin). Deduplication prevents repeated entries.

## R4: Embedded Components to Modify

**Decision**: Remove `binaries` from go.yaml, rust.yaml, node.yaml, python.yaml. Keep `binaries` in claude-code.yaml, vertex-ai.yaml, git-hosting.yaml.

**Rationale**: The package registry components should use automatic resolution. The claude/vertex components have complex glob patterns (`/sandbox/.local/share/claude/**`) that cannot be derived from the manifest. git-hosting.yaml has `match.always: true` and specific binaries (`/usr/bin/git`, `/usr/bin/gh`) that are well-defined.

## R5: Key Files

- `cc-deck/internal/build/policy.go:89` - `AssemblePolicy()`, insertion point for resolution
- `cc-deck/internal/build/component.go:15` - `PolicyComponent` struct with `Binaries` field
- `cc-deck/internal/build/manifest.go:35` - `ToolEntry` struct with `Name`, `Install`, `InstallPath`
- `cc-deck/internal/build/policies/*.yaml` - embedded components
- `cc-deck/internal/build/policy_test.go` - existing tests
